<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: dev-ops
related:
  - ../roadmap.md
  - ../../../project/observability.md
-->

# DEV-01：进程生命周期改造

## 实施状态

已完成并闭合。PID 文件、状态 JSON、优雅停、端口占用保护、`dev-status` JSON、失败日志上下文和 Makefile 入口均已落地。

闭合证据：

- 源码事实：`scripts/dev/{start,stop,status,restart,self_check}.sh`、`scripts/dev/services.json` 已存在。
- 关联 BUG 已关闭：`../bugs/bug-2026-05-14-dev-start-exit-zero-on-failure.md`、`../bugs/bug-2026-05-14-dev-start-failure-path-unverified.md`。
- 人类确认：2026-05-17 用户确认 dev-ops 已开发完成。

## 背景

当前 `make dev-start / dev-stop / dev-restart` 通过 `nohup ./bin/<svc> &` 启动、`lsof -ti tcp:<port> | xargs kill -9` 停止。该方案有三个本质缺陷：

1. **强杀**：服务进程没机会执行 OTel shutdown / etcd 注销 / DB 连接 close → 重启时容易在 etcd 上残留旧节点几十秒
2. **端口误杀**：mac 本地 38080-38084 端口可能被无关进程（Chrome、AirPlay）临时占用，被无差别 kill -9
3. **无可观测**：当前 dev 阶段没有"哪些服务现在在跑、PID 是多少、什么时候启动的"这种问询入口，AI 排障要靠 `ps -ef | grep` 拼凑

`dev-start` 用 `sleep 2` 等服务就绪也是粗糙做法，本 spec 一并解决。

## 目标

- 启停脚本可控、可被 AI 程序化判断结果
- 每个服务启动时落 **PID 文件 + 状态 JSON 文件**，AI 通过 Read 即可掌握全局
- 停止默认 SIGTERM，5s 兜底 SIGKILL
- 启动时主动校验端口占用方是否就是同名服务（防误杀，也防重复启动）
- `make dev-start` 真正等到目标进程在跑（DEV-01 范围）；等到 `/readyz` 通过留给 DEV-02 接入

## 范围

- 新增 `scripts/dev/start.sh` `scripts/dev/stop.sh` `scripts/dev/status.sh` `scripts/dev/restart.sh`
- 改造 `Makefile` 的 `dev-start / dev-stop / dev-restart / model-* / asset-*` 目标，把 inline shell 改为调用上述脚本
- 新增 `bin/run/` 目录约定（gitignore）
- 输出统一的"服务清单"配置源（避免 6 个目标里硬编码 6 次端口）

## 非目标

- 不改变服务本身代码（健康检查接入留给 DEV-02）
- 不引入 overmind / hivemind / supervisord 等外部依赖
- 不替换 `nohup`（仍需后台运行）；只补强生命周期管理
- 不解决 docker compose 容器层的健康等待（另一个 cleanup PR）

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 服务清单单一来源 | 新增 `scripts/dev/services.json`，记录 `name / port / binary / depends_on`，启停脚本读它 |
| PID 文件路径 | `bin/run/<service>.pid`（仅 PID 数字一行） |
| 状态文件路径 | `bin/run/<service>.status`（单行 JSON） |
| 状态文件字段 | `service / pid / port / started_at(ISO8601) / cmd / log_path` |
| 优雅停超时 | 5 秒，超时后 SIGKILL；超时与兜底动作均输出到 stdout |
| 端口占用处理 | 启动前 `lsof -ti tcp:$port` 查端口；占用方 PID 与 `<svc>.pid` 一致则跳过；否则**报错退出**而非盲杀 |
| `dev-status` 输出 | 单条 JSON 数组到 stdout，每项含 `service / pid / port / alive(bool) / started_at / log_path` |
| 启动顺序 | iam → idp → asset → edge-api → model；并发启动也允许（不强依赖顺序），但等待时按依赖顺序检查 |
| 启动等待策略 | DEV-01 仅等"PID 存在 + 端口被本进程监听"；DEV-02 接入后再升级为 `/readyz` |
| 启动失败动作 | 30s 超时未就绪 → 打印对应 `bin/log/<svc>.log` 末尾 50 行 + 状态文件 + `exit 1` |
| Makefile 目标兼容 | 旧的 `make dev-start / dev-stop / dev-restart` 命名不变，行为升级 |

## 设计约束

- 脚本只用 bash + 标准 Unix 工具（lsof / kill / cat / jq 可选）；不引入新依赖
- 所有 stderr 输出都要带 `[dev-start]` `[dev-stop]` 等前缀，便于 AI 区分脚本输出与服务输出
- 状态文件写入必须**先写 .tmp 再 mv**，避免 AI 读到半行 JSON
- `services.json` 必须可被人类直接编辑（JSON 不带注释也能接受），新增服务时只改这一个文件

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| 启停脚本存在 | `test -x scripts/dev/start.sh && test -x scripts/dev/stop.sh && test -x scripts/dev/status.sh` |
| 服务清单可被 jq 解析 | `jq . scripts/dev/services.json >/dev/null` |
| 启动后 PID 文件存在且有效 | `make dev-start` 后 `for s in iam idp asset edge-api model; do kill -0 $(cat bin/run/$s.pid); done` 全部 0 |
| 状态文件为合法单行 JSON | `for f in bin/run/*.status; do jq -e '.service and .pid and .port' $f; done` |
| `dev-status` 输出合法 JSON 数组 | `make dev-status | jq -e 'type=="array" and length>=5'` |
| 优雅停可工作 | `make dev-stop` 后 `lsof -ti tcp:38080 tcp:38081 tcp:38082 tcp:38083 tcp:38084` 无输出，且 `bin/run/` 下 .pid/.status 文件被清理 |
| 端口被无关进程占用时报错而非杀 | 模拟用 `nc -l 38082` 占端口，运行 `make dev-start` 应在 stderr 输出 "port 38082 occupied by pid X (not iam), refuse to start" 并 exit 1 |
| 启动失败打印日志尾部 | 故意改坏 `.env` 的 `MONGO_URI`，`make dev-start` 应在 30s 内退出并打印 iam.log 尾部内容 |

## 人类验收

- 在低性能机器（或冷启动 Docker 后立即 `make dev-start`）观察是否还会出现"看似启动成功但前端首请求失败"
- 重启 5 次后检查 etcd（`etcdctl get --prefix /micro-service/`），确认无僵尸节点堆积
- 误操作（如手动 `kill -9` 某服务后再 `make dev-start`）能正确处理
- 输出可读性：`make dev-status` 单条命令的输出，AI 与人类都能秒懂当前状态

## 开发进度

- ✅ 2026-05-14：已实现 `scripts/dev/services.json` 与 `start.sh / stop.sh / status.sh / restart.sh`，Makefile 的 `dev-* / model-* / asset-*` 目标已切换到脚本入口。
- ✅ 2026-05-14：主 agent 已验证脚本语法、`services.json` 可解析、`dev-status` 输出 JSON 数组、`make build` 与 `make lint` 通过。
- ✅ 2026-05-14：tester agent 初检发现已运行服务跳过 `/readyz` 与 3xx 被误判 ready 两个问题；主 agent 修复后 tester 复检通过。
- ✅ 2026-05-17：用户确认 dev-ops 已开发完成；真实 `make dev-start / dev-stop / dev-restart`、冷启动 Docker、多次重启与 etcd 僵尸节点检查不再阻塞本阶段闭合，可作为后续回归场景。

## 风险与回退

- 风险：现有 `make model-start / asset-start` 等子目标若行为不一致会困扰习惯用户；改造时这些目标需同步走新脚本
- 回退：保留旧 Makefile 目标实现到 git 历史，必要时 revert
