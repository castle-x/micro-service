<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: roadmap
initiative: opentelemetry
related:
  - ../../project/observability.md
  - ../../knowledge/observability/overview.md
  - ../../knowledge/pkg-infra/overview.md
  - ../../knowledge/services/overview.md
-->

# OpenTelemetry 接入路线图

> **状态**：基础闭环已完成（OTel-01 至 OTel-06 已落地；日志入库、metrics 友好展示和拓扑状态图后续按需补）
> **整理时间**：2026-05-12
> **定位**：记录 OpenTelemetry 接入的阶段路线、依赖关系、验收口径和当前进度。本文同时记录已落地能力与后续按需补强项。

## 一、背景与目标

项目最初只具备自研 trace 元数据基础：

- `pkg/middleware` 负责 HTTP/Kitex 的 `trace_id` 生成与透传。
- `pkg/logger.Ctx(ctx)` 能把 trace/user/tenant 元数据写入日志。
- `edge-api`、`model` 已使用 Hertz 中间件；`idp`、`iam` 等 Kitex 服务已挂载 trace/recovery/logging。

经过 OTel-01 至 OTel-05，当前已经具备标准 OpenTelemetry 应用侧能力：

- `pkg/otel` 已提供 tracer/meter provider、OTLP exporter、resource、shutdown 与禁用降级。
- Hertz / Kitex 已支持 W3C `traceparent` / `baggage` 传播，并兼容 `X-Trace-ID`。
- 日志可通过 OTel `trace_id` / `span_id` 与 span 树关联。
- Mongo、Redis、MQ、model/LLM provider 调用已形成低敏、低基数的可观测边界。
- `edge-api -> model -> LLM provider` 已具备连续 trace 传播和 provider span。

OTel-06 已基于 OpenObserve 补齐本地统一观测平台、OpenTelemetry Collector 和 AI 可读查询命令。当前目标已经从“继续搭基础设施”转为“在具体业务排障时使用这些入口，并按需补齐日志入库、metrics 展示和拓扑状态聚合”。

## 二、总体原则

| 原则 | 说明 |
|---|---|
| 渐进接入 | 先打 `pkg/otel` 基座，再逐层接 HTTP/RPC、日志、I/O、LLM、观测栈 |
| 不绑后端 | 代码只依赖 OTLP 和 OTel API；应用不直接绑定 OpenObserve、Jaeger/Tempo/Loki、Prometheus/Grafana 等实现 |
| 兼容现状 | 继续兼容 `X-Trace-ID`，内部逐步切到 W3C trace context |
| 默认可降级 | exporter/collector 不可用时，服务应继续启动，业务不依赖观测后端 |
| 敏感信息最小化 | span attributes、metrics labels、日志字段都不得记录 secret、token、API key、原始 prompt |
| AI 可读 | 每个阶段都要考虑机器可读输出，最终形成稳定排障入口 |

## 三、阶段路线

| 阶段 | 主题 | 状态 | 产物 |
|---|---|---|---|
| OTel-01 | `pkg/otel` 基座 | 已完成 | [`specs/otel-foundation.md`](specs/otel-foundation.md) |
| OTel-02 | Hertz + Kitex trace 注入 | 已完成 | [`specs/http-rpc-tracing.md`](specs/http-rpc-tracing.md) |
| OTel-03 | log correlation | 已完成 | [`specs/log-correlation.md`](specs/log-correlation.md) |
| OTel-04 | Mongo / Redis / MQ instrumentation | 已完成 | [`specs/data-mq-instrumentation.md`](specs/data-mq-instrumentation.md) |
| OTel-05 | model / LLM 可观测性 | 已完成 | [`specs/model-llm-observability.md`](specs/model-llm-observability.md) |
| OTel-06 | OpenObserve 本地观测平台与 AI 查询工具 | 已完成基础版 | [`specs/local-observability-ai-tools.md`](specs/local-observability-ai-tools.md) |

## 四、阶段依赖

```text
OTel-01 pkg/otel
  -> OTel-02 HTTP/RPC trace
    -> OTel-03 log correlation
    -> OTel-04 DB/Redis/MQ
    -> OTel-05 model/LLM
      -> OTel-06 OpenObserve local observability + AI tools
```

`OTel-04` 和 `OTel-05` 可在 `OTel-02` 完成后并行推进；`OTel-06` 依赖前面至少有一条完整链路可观测。

## 五、当前事实进度

| 能力 | 当前状态 |
|---|---|
| `.axm` OTel 规范 | 已建立：`project/observability.md` |
| `.axm` OTel 知识入口 | 已建立：`knowledge/observability/overview.md` |
| OTel runtime 基座 | 已实现：`pkg/otel` 提供 tracer/meter provider、resource、OTLP exporter、shutdown 与禁用降级 |
| W3C trace context | 已实现：Hertz 从 `traceparent` / `baggage` 提取，Kitex 通过 metainfo 提取/注入 W3C context |
| OTLP exporter | 已实现：支持 OTLP gRPC / HTTP exporter 配置 |
| logs 关联 span id | 已实现：`logger.Ctx(ctx)` 自动输出 OTel `trace_id` / `span_id`，无 span 时保留旧 trace_id |
| DB/Redis/MQ instrumentation | 已实现：`pkg/db` Repository、`pkg/redis` Client/Lock、`pkg/mq` message context helper 与 NSQ placeholder span |
| LLM provider instrumentation | 已实现：model adapter 非流式/流式调用创建 `LLM chat.completions` span，记录 provider/model/stream/status/usage/首 token 延迟并避免写入 prompt/response/API key |
| 本地观测平台选型 | 已确认：OTel-06 优先接入 self-host OpenObserve，统一查看 traces、metrics、logs；Prometheus/Grafana/Jaeger 不作为第一版必需组件 |
| OpenTelemetry Collector | 决策保留：作为应用与 OpenObserve 之间的 OTLP 解耦层，便于后续替换或并行转发后端 |
| 本地观测栈 | 已实现并验证：`deployments/docker-compose.observability.yml` 启动 OpenObserve + OpenTelemetry Collector；OpenObserve UI 可打开 |
| dev 默认 OTel | 已实现：`make dev-start` / `make dev-restart` 默认执行 `obs-up` 并注入 `OTEL_ENABLED=true`、`OTEL_ENDPOINT=localhost:4317` |
| AI 查询脚本 | 已实现入口：`scripts/observability/openobserve-query.mjs` + `make obs-trace/obs-logs/obs-metrics/obs-errors` |
| 真实 trace 验证 | 已验证基础链路：本地请求可在 OpenObserve 中看到 `edge-api -> idp` 等 trace/span |

## 六、开放问题

| 问题 | 当前结论 | 决策时机 |
|---|---|---|
| OpenObserve 部署形态 | self-host Open Source Edition，本地 Docker 单节点 | 生产化前再确认数据目录、鉴权和保留策略 |
| Collector 是否必经 | 必经：`services/* -> Collector -> OpenObserve` | 若未来要并行转发 Prometheus/Tempo/Loki 时扩展 Collector |
| Prometheus/Grafana 是否保留 | 第一版不保留为必需组件；用户已接受 OpenObserve 聚合平台 | 只有已有指标生态或告警需求出现时再接 |
| 日志采集方式 | 本地开发先保留 `bin/log/*.log`；完整日志入 OpenObserve 后续按需做 | 需要跨服务日志检索或告警时 |
| 采样策略 | local/staging 全采样，prod 父采样 + ratio | 生产部署前确认 |
| 服务拓扑和存活状态 | OTel trace 只能反映调用关系；实时节点状态需聚合 etcd + health + 进程/容器状态 | 需要拓扑 UI 时单独立项 |
| AI 查询入口形态 | `make obs-*` 包装脚本输出 JSON 或固定字段文本 | 查询字段和输出格式随真实排障场景打磨 |
