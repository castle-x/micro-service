<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
depth: overview
code-refs:
  - pkg/logger/logger.go
  - pkg/middleware/metainfo.go
  - pkg/middleware/hertz/hertz.go
  - pkg/middleware/kitex/kitex.go
  - pkg/otel/otel.go
  - pkg/db/doc.go
  - pkg/redis/client.go
  - pkg/mq/nsq/nsq.go
  - pkg/mq/instrumentation.go
  - services/edge-api/router.go
  - services/edge-api/handler/model_otel.go
  - services/model/router.go
  - services/model/adapter/adapter.go
  - deployments/docker-compose.observability.yml
  - deployments/observability/otel-collector.yaml
  - scripts/observability/openobserve-query.mjs
related:
  - ../../project/observability.md
  - ../../project/architecture.md
  - ../pkg-infra/overview.md
  - ../services/overview.md
-->


# 可观测性 — 速查

## 当前定位

本项目以 OpenTelemetry 作为 trace、metrics、log correlation 的统一标准。当前分支已经接入应用侧 OTel 基座，并以 OpenObserve 作为本地统一观测平台：服务通过 OTLP 发往 OpenTelemetry Collector，再由 Collector 写入 OpenObserve。`pkg/middleware` 保留兼容 trace 元数据，`pkg/otel` 负责标准 W3C trace context 与 provider 初始化，`pkg/logger.Ctx(ctx)` 写入 `trace_id` / `span_id` 用于日志关联。

## 核心链路

```text
Client/Kong
  -> edge-api/Hertz
    -> Kitex RPC: idp/iam/billing/credits/notification
      -> MongoDB / Redis
      -> MQ publish / consume
    -> model service/Hertz
      -> LLM or image provider
```

## 观测信号

| 信号 | 用途 | 典型问题 |
|---|---|---|
| Trace | 单次请求链路、错误 span、慢 span | 登录慢、RPC 超时、LLM provider 报错 |
| Metrics | 趋势、告警、容量 | P95/P99 升高、错误率升高、队列积压 |
| Logs | 结构化上下文和业务细节 | 业务错误码、panic stack、关键状态变更 |

三类信号必须通过 `trace_id` / `span_id` 关联，避免只靠时间戳拼接。

## 代码边界

| 边界 | 当前事实 | 说明 |
|---|---|---|
| `pkg/logger` | zap JSON logger，`Ctx(ctx)` 注入 trace/user/tenant | 增加 OTel trace/span id 关联字段 |
| `pkg/otel` | 初始化 tracer/meter provider、OTLP gRPC/HTTP exporter、resource、W3C propagator | 代码侧只依赖 OTLP，不绑定 OpenObserve |
| `pkg/middleware/hertz` | HTTP trace id 生成、header 响应、日志中间件 | 已接入 HTTP server span 与 W3C context 提取/注入 |
| `pkg/middleware/kitex` | Kitex trace id 透传、recovery、logging | 已接入 RPC server/client span 与 context propagation |
| `pkg/db` | Mongo Client/Repository/Transaction 封装 | 已有 Mongo operation span/metrics 边界 |
| `pkg/redis` | go-redis client 与 lock/key helper | 已有 Redis command span/metrics 边界 |
| `pkg/mq` | NSQ producer/consumer 抽象仍是占位 | 已有 publish/consume span 与 message context helper，真实 NSQ 后续跟业务接入 |
| `services/model` | Hertz 服务与 provider adapter | 已接入 LLM provider span、token/latency/error metrics |

## 人类观测入口

当前本地观测栈：

```text
services/* -> OTLP -> OpenTelemetry Collector
Collector -> OpenObserve
OpenObserve UI -> traces / metrics / logs
```

OpenObserve 是第一版聚合平台，Prometheus/Grafana/Jaeger 不再是本地开发必需组件。代码侧仍只依赖 OTLP，后续如果需要保留 Prometheus 生态，可在 Collector 增加并行出口。

## AI 观测入口

AI 排障需要稳定、机器可读的查询入口。建议后续提供：

| 命令 | 输出 |
|---|---|
| `make obs-trace TRACE_ID=<id>` | 查询 OpenObserve 中的 trace/span 数据 |
| `make obs-logs TRACE_ID=<id>` | 查询关联日志；当前日志入库链路仍需后续补强 |
| `make obs-metrics SERVICE=<name>` | 查询服务指标快照；当前输出仍偏原始 |
| `make obs-errors SINCE=15m` | 最近错误 trace 列表 |

输出应优先使用 JSON 或固定字段文本，方便 AI 读取后形成证据链。

## 分阶段接入路线

| 阶段 | 范围 | 验收 |
|---|---|---|
| 1 | `.axm` 规范与知识入口 | AI 和人类知道何时读取观测性规范 |
| 2 | `pkg/otel` 基座 | 服务可初始化/关闭 OTel，支持启用和禁用 |
| 3 | Hertz + Kitex trace | `edge-api -> idp/iam` 可看到完整 trace |
| 4 | log correlation | 同一请求日志可通过 trace id 查询 |
| 5 | Mongo/Redis/MQ | I/O 慢操作和错误可定位到具体 span |
| 6 | OpenObserve 本地观测栈与 AI 查询脚本 | OpenObserve UI 可查看 trace，`make obs-*` 提供机器可读入口 |

## 当前限制

- OpenObserve Trace UI 的内置字段可能会查询不存在的字段，例如 `llm_input`；本项目按规范不记录原始 prompt，因此这类字段缺失是预期的。
- 应用日志目前主要落在 `bin/log/*.log` 并带 `trace_id/span_id`；完整日志入 OpenObserve 的 pipeline 还不是本分支目标。
- metrics 已通过 OTel exporter 发出，但查询脚本和仪表盘仍是基础形态，先满足排障入口，不做重 UI。
- 服务节点拓扑和存活状态不能只靠 OTel trace 得到；需要结合 etcd 注册表、健康检查和进程/容器状态。
