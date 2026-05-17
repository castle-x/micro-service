<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: opentelemetry
related:
  - ../roadmap.md
  - ../../../project/observability.md
  - ../../../knowledge/observability/overview.md
  - ../../../knowledge/services/overview.md
-->

# OTel-05：model / LLM 可观测性

> **状态**：已完成
> **完成时间**：2026-05-12

## 背景

`services/model` 是 HTTP AI 模型服务，包含非流式和 SSE 流式调用。LLM provider 往往是高延迟、高错误率、高成本边界，需要单独观测。

## 目标

- model service HTTP 入口纳入 trace。
- provider adapter 调用形成 LLM child span。
- 记录 provider、model、stream、duration、status、token usage。
- SSE 流式输出能观测首 token 延迟、总耗时、异常终止。

## 范围

- `services/model/handler`
- `services/model/biz`
- `services/model/adapter`
- `edge-api -> model` HTTP proxy 链路

## 非目标

- 不记录原始 prompt。
- 不记录原始模型响应正文。
- 不在本阶段实现成本结算。

## 已确认开发细节

| 指标或属性 | 说明 |
|---|---|
| `gen_ai.system` | provider，例如 openai、anthropic 或其他供应商 |
| `gen_ai.request.model` | 模型名 |
| `llm.request.duration` | 请求总耗时 |
| `llm.first_token.duration` | SSE 首 token 延迟，若可采集 |
| `llm.token.count` | input/output token，若 provider 返回 |
| `stream` | true/false |

## 设计约束

- API key、prompt、completion、用户上传内容不得进入 logs/span/metrics。
- 流式接口不能因为观测逻辑引入明显缓冲。
- provider 不返回 token usage 时，指标缺失应可接受，不做猜测估算。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| model 测试 | `cd services/model && go test ./... -count=1` |
| 非流式 trace | `/api/v1/model/chat` trace 中可见 provider span |
| 流式 trace | `/api/v1/model/chat/stream` trace 中可见 stream span，SSE 正常输出 |
| 敏感字段检查 | trace/logs 不包含 prompt、completion、API key |

## 人类验收

- 人类可在 Trace UI 中判断一次模型调用慢在 edge proxy、model service，还是 provider。
- 人类可在 dashboard 中看到 provider/model 维度的延迟、错误率和 token usage。

## 实施记录

- `services/model/adapter`：
  - OpenAI-compatible adapter 在 `Chat` / `ChatStream` 中创建 `LLM chat.completions` client span。
  - span attributes 只记录 `gen_ai.system`、`gen_ai.request.model`、`gen_ai.operation.name`、`stream`、`status`、token usage 与首 token 延迟，不记录 prompt、completion、API key。
  - 记录 `llm.request.duration`、`llm.request.count`、`llm.first_token.duration`、`llm.token.count` metrics。
  - 流式 span 延续到 SSE upstream 被读完，避免只覆盖初始化请求。
- `services/edge-api/handler`：
  - model proxy 出站 HTTP 调用创建 client span。
  - 显式注入 W3C `traceparent` / `baggage` 到 model service 请求头，保证 `edge-api -> model -> provider` trace tree 连续。
  - SSE proxy span 通过包装 upstream body 在 EOF/Close 时结束，不引入额外缓冲。

## 自动验收记录

- `cd services/model && go test ./... -count=1`
- `cd services/edge-api && go test ./... -count=1`
