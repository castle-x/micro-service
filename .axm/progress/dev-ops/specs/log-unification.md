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
  - ../../../knowledge/pkg-infra/overview.md
  - ../../../project/observability.md
-->

# DEV-03：日志格式统一与 AI 查询入口

## 实施状态

已完成并闭合。stdlib log 接管、本地 `logs-query`、`dev-logs`、`lint-noprint` 和 JSON 日志查询入口均已落地。

闭合证据：

- 源码事实：`pkg/logger/logger.go`、`scripts/dev/logs-query.sh`、`Makefile` 的日志查询 / lint 入口已存在。
- 长期事实已同步：`../../../knowledge/pkg-infra/overview.md` 已记录 logger 和 trace/span 日志字段；OpenObserve 查询另见 OTel initiative。
- 人类确认：2026-05-17 用户确认 dev-ops 已开发完成。

## 背景

`pkg/logger` 已默认输出 JSON（zap production encoder），关键字段齐全（`time / level / service / msg / caller_file / trace_id / span_id / user_id / tenant_id`）。但实际开发中存在三类破坏者：

1. **第三方库自带日志**：Hertz 与 Kitex 的 access log、Mongo driver、go-redis、kitex registry 都用各自格式，混入 `bin/log/<svc>.log` 后行格式不统一，AI grep / jq 时容易踩坑
2. **遗留 `fmt.Println` / `log.Printf`**：业务代码里偶发出现，输出无 trace_id 无 service 无 time
3. **6 个服务 6 个日志文件**：当前 AI 排障要 `Read` 6 次或脚本拼接，没有统一查询入口；`make obs-*` 已有 OpenObserve 远端查询，但前提是 OpenObserve 在跑且接入正确，本地直接 grep 文件应该是 fallback

## 目标

- 严格保证 `bin/log/<svc>.log` 每行都是合法 JSON，至少含约定的最小字段
- 第三方库日志全部接管到 `pkg/logger`，统一格式
- 提供 AI 可调用的本地日志查询脚本（按 trace_id / service / level / 时间窗口过滤），输出 JSON 数组
- `make lint` 增加一条静态规则，禁止业务代码使用 `fmt.Println` / `log.Printf`

## 范围

- `pkg/logger`：新增 `IngestStdLog()`（重定向 `log.Default()` 到 zap）；新增 Hertz/Kitex access log adapter
- 各 `services/*/main.go`：调用接管函数
- `Makefile`：新增 `make dev-logs` 目标（聚合 tail）+ `make logs-query` 目标（调用查询脚本）+ `make lint` 追加禁用项检查
- `scripts/dev/logs-query.sh`：本地日志查询脚本

## 非目标

- 不引入额外日志采集 sidecar / fluentbit 等
- 不实现日志轮转（dev 阶段，真要轮转用 `> bin/log/<svc>.log` 手动截断即可）
- 不替代 OpenObserve；OpenObserve 走 `make obs-*` 现有命令，本 spec 只补本地文件维度
- 不重构 `pkg/logger` 的 API（已经够用）

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 日志最小字段 | `time(ISO8601) / level / service / msg`；建议字段 `trace_id / span_id / caller_file / user_id / tenant_id` |
| 第三方库接管 | Hertz access log → 自定义 middleware 调 `logger.Ctx(ctx).Info`；Kitex access middleware 同理；Mongo driver 通过 `options.Client().SetLoggerOptions()` 桥接到 zap |
| `fmt.Println` 禁用 | `make lint` 增加 `! grep -rn --include='*.go' -E '(^|[^a-zA-Z])(fmt\.Print|fmt\.Println|log\.Print)' services/ pkg/`，命中即失败；测试代码可白名单 |
| 查询脚本入参 | `--service=<name>` `--trace-id=<id>` `--level=error,warn` `--since=15m` `--limit=200` `--format=json` |
| 查询脚本实现 | bash 包装 `jq` 流式过滤 `bin/log/*.log`，输出合并 JSON 数组到 stdout |
| `make dev-logs` | `tail -F bin/log/iam.log bin/log/idp.log ...`，多文件混合输出（人类查看） |
| `make logs-query` | 透传参数到 `scripts/dev/logs-query.sh`，例如 `make logs-query ARGS="--trace-id=abc --since=5m"` |
| Caller 字段 | 已有 `caller_file`，本 spec 不改名（避免破坏现有 OpenObserve 仪表板） |

## 设计约束

- 日志接管不能产生递归（zap 内部错误不要再写回 zap）
- 查询脚本必须**容错坏行**（如某行不是 JSON），跳过并在 stderr 计数，不能整体失败——dev 阶段总会出现意外输出
- 查询脚本输出必须是合法 JSON 数组，方便 AI 直接 `jq` 后续处理
- `make lint` 检查的禁用规则要给业务一条逃生通道：`// nolint:noprint` 注释豁免（仅限确实需要 stdout 的场景，如 `cmd/bootstrap`）

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| 所有日志行都是 JSON | `for f in bin/log/*.log; do awk 'NR<=200' $f | jq -e . >/dev/null || echo "BAD: $f"; done` 无 BAD 输出 |
| 必含字段齐备 | 同上换成 `jq -e '.time and .level and .service and .msg'` |
| 第三方库日志被接管 | 调用一次访问日志触发的接口（如 `curl localhost:38080/healthz`），随后 `tail -1 bin/log/edge-api.log` 应是合法 JSON 且 `service=edge-api` |
| lint 禁用规则生效 | 故意在 `services/iam/biz/` 加一行 `fmt.Println("x")`，`make lint` 应失败 |
| 查询脚本输出合法 JSON 数组 | `make logs-query ARGS="--service=iam --since=1h" | jq -e 'type=="array"'` |
| 查询脚本支持 trace_id 过滤 | 用一个真实存在的 trace_id：`make logs-query ARGS="--trace-id=<id>" | jq -e 'all(.[]; .trace_id=="<id>")'` |
| 坏行容错 | 故意往 `bin/log/iam.log` 追加一行 `not json garbage`，再跑查询脚本，stdout 仍是合法 JSON 数组，stderr 提示 1 bad line |

## 人类验收

- `make dev-logs` 在终端混合输出 6 个服务日志，颜色/前缀清晰可读
- 排障一次实际故障：仅靠 `make logs-query` + trace_id 能定位问题，无需打开 OpenObserve UI
- 第三方库（Mongo / go-redis / kitex registry）的关键事件不丢失（启动连接、断线重连等都有 INFO/WARN 记录）

## 开发进度

- ✅ 2026-05-14：已新增 `logger.IngestStdLog()`，5 个 dev-start 服务入口已接管 stdlib `log.Default()` 到 JSON zap logger。
- ✅ 2026-05-14：已新增 `scripts/dev/logs-query.sh` 与 Makefile `dev-logs / logs-query / lint-noprint` 入口；bootstrap 命令式 stdout 已用 `nolint:noprint` 明确豁免。
- ✅ 2026-05-14：主 agent 已验证 logger 单测、日志查询临时夹具、`make lint-noprint`、`make lint`；tester agent focused verification 通过。
- ✅ 2026-05-17：用户确认 dev-ops 已开发完成；真实 trace_id 排障演练可作为后续回归，不阻塞本阶段闭合。

## 风险与回退

- 风险：第三方库 logger 接管可能漏掉某些版本 API，导致部分日志仍是旧格式；可分服务渐进接入，先 `edge-api` `iam` 两个最常被排障的
- 回退：`pkg/logger` 已默认 JSON，本 spec 失败不会让现状变差；查询脚本独立可单独回退
