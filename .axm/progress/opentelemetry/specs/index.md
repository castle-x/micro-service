<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
entries:
  - path: otel-foundation.md
    title: OTel 基座
    when-to-read: 实施 pkg/otel、配置、resource、exporter、shutdown 和禁用降级时
  - path: http-rpc-tracing.md
    title: HTTP/RPC trace 注入
    when-to-read: 实施 Hertz、Kitex、W3C trace context 与 X-Trace-ID 兼容时
  - path: log-correlation.md
    title: 日志关联
    when-to-read: 实施 trace_id/span_id 日志字段、panic/error 关联与日志验收时
  - path: data-mq-instrumentation.md
    title: DB/Redis/MQ instrumentation
    when-to-read: 实施 Mongo、Redis、NSQ 的 span、metrics 和 message context 时
  - path: model-llm-observability.md
    title: model/LLM 可观测性
    when-to-read: 实施 model service、provider adapter、stream 和 token usage 遥测时
  - path: local-observability-ai-tools.md
    title: 本地观测栈与 AI 查询工具
    when-to-read: 实施 Collector、Jaeger/Grafana、make obs-*、AI 排障入口时
-->
# specs/ — OpenTelemetry 阶段 specs

OpenTelemetry 接入的可执行阶段计划。每份 spec 必须包含 AI 自动验收和人类验收。
