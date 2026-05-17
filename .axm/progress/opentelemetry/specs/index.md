<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: otel-foundation.md
    title: OTel 基座
    when-to-read: 理解已完成 pkg/otel、配置、resource、exporter、shutdown 和禁用降级时
  - path: http-rpc-tracing.md
    title: HTTP/RPC trace 注入
    when-to-read: 理解已完成 Hertz、Kitex、W3C trace context 与 X-Trace-ID 兼容时
  - path: log-correlation.md
    title: 日志关联
    when-to-read: 理解已完成 trace_id/span_id 日志字段、panic/error 关联与日志验收时
  - path: data-mq-instrumentation.md
    title: DB/Redis/MQ instrumentation
    when-to-read: 理解已完成 Mongo、Redis、NSQ 的 span、metrics 和 message context 时
  - path: model-llm-observability.md
    title: model/LLM 可观测性（旧 model 历史记录）
    when-to-read: 仅在追溯旧 services/model 观测实现历史时读取；当前 LLM 链路改读 generation-platform GP-02 与 knowledge/observability
  - path: local-observability-ai-tools.md
    title: 本地观测栈与 AI 查询工具
    when-to-read: 理解已完成 Collector、OpenObserve、make obs-*、AI 排障入口时
-->
# specs/ — OpenTelemetry 阶段 specs

OpenTelemetry 第一版已闭合。这里保留 OTel-01 至 OTel-06 的已完成实施记录、验收口径和后续按需增强项。
