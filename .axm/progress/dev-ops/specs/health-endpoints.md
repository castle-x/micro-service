<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: dev-ops
workflow-state: closed
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - process-lifecycle.md
  - ../../../knowledge/pkg-infra/overview.md
-->

# DEV-02：标准健康检查接口

## 实施状态

已完成并闭合。`pkg/health`、`/healthz`、`/readyz`、`/version`、业务端口 + 10000 admin 端口规则、Mongo/Redis/etcd readiness check 和 5 个本地服务接入均已落地。

闭合证据：

- 源码事实：`pkg/health/{server,checks}.go`、`services/{edge-api,idp,iam,asset,model}/main.go`、`scripts/dev/services.json` 已存在。
- 关联 BUG 已关闭：`../bugs/bug-2026-05-14-edge-api-missing-etcd-readiness.md`。
- 人类确认：2026-05-17 用户确认 dev-ops 已开发完成。

## 背景

当前所有服务都没有统一的健康检查接口：

- `edge-api` / `model` 是 Hertz HTTP，**理论上**有自定义健康路由，但格式各异、未规范
- `iam` / `idp` / `asset` 是 Kitex RPC，根本没有 HTTP 端口对外暴露健康
- DEV-01 的 `dev-start` 想真正等服务"就绪"，没有标准化探针就只能 `sleep`
- 出现问题时 AI 想判断"是依赖（mongo/redis/etcd）挂了还是服务自身挂了"也无据可依

## 目标

为每个后端服务（包括 Kitex RPC 服务）暴露统一的 HTTP 健康检查端点，区分"存活"与"就绪"，输出固定 JSON 格式。

## 范围

- 新增 `pkg/health` 包，提供：
  - `Server` 结构体，可注册多个 `Check(name, fn)`
  - 内置三个 endpoint：`/healthz` `/readyz` `/version`
  - 启动一个独立的 admin HTTP 监听
- 接入 5 个服务：`edge-api / idp / iam / asset / model`
- 每个服务在 main 中调用 3-5 行代码完成接入；具体注册哪些 dep check（mongo / redis / etcd / oss）由各服务自行决定

## 非目标

- 不暴露 metrics endpoint（OTel collector 已经覆盖）
- 不接入业务深度健康（如"某队列堆积量"），只检查依赖连通性
- 不做认证；admin 端口默认只监听 127.0.0.1

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 包位置 | `pkg/health` |
| Admin 端口规则 | **业务端口 + 10000**，例如 iam 38082 → 48082。写死在 `services.json` 里 |
| 监听地址 | 默认 `127.0.0.1:<admin-port>`，避免对外暴露 |
| `/healthz` 语义 | 进程存活、HTTP 可响应。永远返回 `{"status":"ok"}` 200 |
| `/readyz` 语义 | 所有注册的 Check 通过才返回 200，否则 503，body 列出每个 Check 状态 |
| `/version` 字段 | `service / commit / built_at / go_version`；commit/built_at 通过 ldflags 注入 |
| Check 函数签名 | `func(ctx context.Context) error`，1 秒 ctx 超时 |
| 内置 Check helper | `health.MongoCheck(client)` `health.RedisCheck(client)` `health.EtcdCheck(client)`，业务自己 wire |
| 失败响应格式 | `{"status":"not_ready","deps":{"mongo":"ok","redis":"timeout","etcd":"ok"}}` |

## 设计约束

- `pkg/health` 不得 import `services/*` 或第三方业务库；mongo/redis/etcd helper 应放在各对应 pkg（如 `pkg/db/health.go`、`pkg/redis/health.go`），由 main 拼装
- Admin server 必须支持 graceful shutdown，由 service main 与 OTel shutdown 一起串联
- `/readyz` 不能在 Check 实现里发起重试或长 RPC，单次轻量探测即可
- 接入服务时必须把 admin 端口注册进 DEV-01 的 `services.json`

## 与 DEV-01 的协作

- DEV-01 的 `scripts/dev/start.sh` 在确认进程存活后，对每个服务的 admin 端口轮询 `/readyz`，全部 200 才认定启动完成
- 轮询参数：每 500ms 一次，单服务最多 30s，超时打印该服务最后一次 `/readyz` 响应 + log 尾部
- DEV-01 的 `dev-status` 输出每项追加一个 `ready: bool` 字段，由 `/readyz` 探测得到

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| pkg/health 单测通过 | `cd pkg && go test ./health/... -count=1 -race` |
| 5 个服务 build 通过 | `make build` |
| 启动后 healthz 全部返回 200 | `for p in 48080 48081 48082 48083 48084; do curl -fsS localhost:$p/healthz | jq -e '.status=="ok"'; done` |
| 启动后 readyz 全部返回 200 | `for p in 48080 48081 48082 48083 48084; do curl -fsS localhost:$p/readyz | jq -e '.status=="ready"'; done` |
| 故意停 mongo 容器后 readyz 变 503 | `docker stop platform-mongo && sleep 3 && curl -o /dev/null -w '%{http_code}' localhost:48082/readyz` 输出 `503` |
| version 含必需字段 | `curl -fsS localhost:48082/version | jq -e '.service and .commit and .built_at'` |
| dev-start 真正等到 readyz | DEV-01 脚本接入后，`time make dev-start` 在低性能机器与 sleep 2 版本相比应**更晚返回**，且返回时所有 readyz 立即 200 |

## 人类验收

- 接入 5 个服务的成本足够低（main 改动 ≤ 5 行）
- 故意制造 mongo 不可用、etcd 不可用、redis 不可用三种场景，readyz 都能正确反映依赖故障
- admin 端口偏移规则（+10000）在团队内可口头讲清，无需查文档

## 开发进度

- ✅ 2026-05-14：已新增 `pkg/health`，提供 `/healthz /readyz /version`、依赖 Check 注册、1s 超时、版本字段与 admin addr 环境变量覆盖。
- ✅ 2026-05-14：`edge-api / idp / iam / asset / model` 已接入 admin health server；服务注册了可轻量探测的 Mongo / Redis checks。
- ✅ 2026-05-14：主 agent 已验证 `pkg/health`、5 个接入服务测试、`make build` 与 `make lint`；tester agent focused verification 通过。
- ✅ 2026-05-17：用户确认 dev-ops 已开发完成；真实依赖 stop/start 场景保留为后续回归，不阻塞本阶段闭合。

## 风险与回退

- 风险：admin server 启动失败（端口冲突）若没处理好可能导致主服务无法启动；约定 admin server 失败只 log warn 不退出
- 回退：DEV-02 失败不影响 DEV-01；DEV-01 的就绪等待退化为"等 PID 存在 + 端口监听"
