<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
applies-to: [project:micro-service, observability]
related:
  - ../knowledge/observability/overview.md
  - ../knowledge/pkg-infra/overview.md
  - ../knowledge/services/overview.md
-->


# OpenTelemetry 可观测性规范

## 目标

`micro-service` 的后端链路必须以 OpenTelemetry 为统一遥测标准，覆盖 trace、metrics、log correlation。目标不是记录更多日志，而是让人类和 AI 在后端异常时优先从遥测数据定位问题，再回到代码验证根因。

## 准入规则

新增或修改以下链路时，必须同步考虑 OTel 注入：

| 链路类型 | 必须具备 |
|---|---|
| Hertz HTTP 入口 | server span、route/method/status/duration、trace context 提取与响应 trace id |
| Kitex RPC 服务端 | server span、service/method/status/duration、panic/error 记录 |
| Kitex RPC 客户端 | client span、下游 service/method、trace context 透传 |
| MongoDB | operation/collection/duration/error span 或等价 instrumentation |
| Redis | command/duration/error span 或等价 instrumentation，禁止记录 value |
| MQ publish/consume | topic/message id/status/retry/lag，publish 与 consume 通过 message context 串联 |
| 外部 HTTP/API | peer/provider/endpoint/duration/status，禁止记录 credential |
| LLM/图像供应商调用 | provider/model/stream/status/duration/token usage，禁止记录原始 prompt、token、API key |

只新增业务逻辑但不新增 I/O 边界时，不强制新增 span；应复用入口 ctx，并保证日志继续使用 `logger.Ctx(ctx)`。

## Trace 规范

- 进程间传播优先使用 W3C `traceparent` / `baggage`。
- 对外兼容 `X-Trace-ID`：HTTP 响应应返回可用于报障的 trace id；内部链路以 OTel trace id 为准。
- span 命名使用稳定低基数名称，禁止把用户输入、资源 id、订单号直接放进 span name。
- 推荐命名：

| 类型 | span name 示例 |
|---|---|
| HTTP server | `HTTP GET /api/v1/user/me` |
| Kitex server | `iam.IAMService/GetUser` |
| Kitex client | `RPC idp.IDPService/Login` |
| MongoDB | `MongoDB users.findOne` |
| Redis | `Redis GET` |
| MQ publish | `NSQ publish billing.order_paid` |
| MQ consume | `NSQ consume billing.order_paid` |
| LLM | `LLM chat.completions` |

## Attribute 规范

允许写入低敏、低基数、可排障的属性：

| 属性 | 示例 | 说明 |
|---|---|---|
| `service.name` | `edge-api` | OTel resource 属性 |
| `deployment.environment` | `local` / `staging` / `prod` | 环境 |
| `enduser.id` | `user_123` | 可选，禁止写邮箱/手机号 |
| `tenant.id` | `tenant_abc` | 多租户定位 |
| `error.code` | `11001` | 对应 `pkg/errno` |
| `rpc.service` / `rpc.method` | `iam.IAMService` / `GetUser` | RPC 定位 |
| `db.system` / `db.collection.name` | `mongodb` / `users` | DB 定位 |
| `messaging.destination.name` | `billing.order_paid` | MQ topic |
| `gen_ai.system` / `gen_ai.request.model` | `openai` / `gpt-4.1` | LLM 调用定位 |

禁止写入以下内容：

- password、secret、token、authorization、cookie、API key
- 原始 prompt、原始模型响应、用户上传文件正文
- 身份证、银行卡、手机号、邮箱等高敏 PII
- 高基数字段作为 metrics label，例如完整 URL、订单号、用户输入

## Metrics 规范

metrics 用于趋势、告警和容量判断；trace 用于单次请求定位。新增 metrics 时必须控制 label 基数。

基础指标集：

| 指标 | 必要维度 |
|---|---|
| `http.server.duration` | service, route, method, status |
| `rpc.server.duration` | service, rpc.service, rpc.method, status |
| `rpc.client.duration` | service, peer.service, rpc.method, status |
| `db.client.duration` | service, db.system, db.collection.name, db.operation |
| `redis.client.duration` | service, db.operation |
| `mq.publish.count` / `mq.consume.duration` | service, messaging.destination.name, status |
| `llm.request.duration` / `llm.request.count` | service, provider, model, stream, status |
| `llm.token.count` | service, provider, model, token.type |

## Log 关联规范

- 业务日志必须使用 `logger.Ctx(ctx)`。
- 日志至少应可关联 `trace_id`；进入 OTel 后应同时关联 `span_id`。
- panic/recovery 必须同时写日志并在当前 span 上 `RecordError`、设置 error status。
- 业务错误应把 `errno` code 写入 span attribute；错误 message 仍需遵守敏感信息禁令。

## AI 排障流程

当后端出现异常、慢请求、错误码升高或用户提供 trace id 时，AI 必须优先按以下顺序排查：

1. 获取 `trace_id`；若没有，则使用时间窗、service、route、user/tenant 低敏标识缩小范围。
2. 查询 trace 拓扑，定位 root span、error span、slowest span。
3. 查看失败 span 的 status、events、attributes，确认错误码、下游 service、DB/Redis/MQ/LLM 边界。
4. 使用 `trace_id` 查询关联日志，只读取必要日志上下文。
5. 使用 metrics 验证是否为单次异常还是系统性问题，例如 P95/P99、错误率、队列积压、LLM provider 错误率。
6. 带着遥测证据回到代码，提出最小修复或下一步验证。

禁止在有可用遥测入口时仅凭代码猜测根因。

## 人类观测入口

本地开发已选择 OpenObserve 作为第一版聚合入口；服务通过 OTLP 到 OpenTelemetry Collector，再写入 OpenObserve。Prometheus/Grafana/Jaeger 不是当前必需组件，后续只在指标生态或告警需求明确时再并行接入。

本地和部署环境应提供稳定入口：

| 入口 | 用途 |
|---|---|
| Trace UI | 按 trace id、service、route、error 查询单次链路 |
| Metrics dashboard | 服务延迟、错误率、吞吐、DB/Redis/MQ/LLM 趋势 |
| Log query | 按 trace id / service / error code 查询结构化日志 |
| AI-friendly scripts | 输出 JSON 或紧凑文本，供 AI 稳定读取 trace、metrics、logs |

本地开发优先保证 `edge-api -> idp/iam -> MongoDB/Redis` 与 `edge-api -> llm -> LLM provider` 链路可被观察；`billing -> MQ -> credits/notification` 在对应功能实现时接入。

## 验收标准

新增 OTel 能力的验收必须可判定：

- 至少一条本地请求能在 Trace UI 中看到完整入口 span 和下游 span。
- 同一请求的日志可通过 trace id 查到。
- 核心 latency/error metrics 能在 dashboard 或查询接口中看到。
- 敏感字段不会出现在 span attributes、metrics labels、日志字段中。
- 禁用 exporter 时服务仍可启动，业务逻辑不依赖观测后端可用性。
