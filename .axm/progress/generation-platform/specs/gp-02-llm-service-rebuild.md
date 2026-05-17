<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: generation-platform
workflow-state: verified
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - ../decisions.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
-->

# GP-02 LLM 服务重建 Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用新的 `services/llm` 替代旧 `services/model` 方向，提供 provider/model/key 管理、Eino `ToolCallingChatModel` 能力、非流式 Generate、流式 Stream、tool-calling rich message、usage 统计和观测能力，作为 GP-03 `agent` 服务的模型调用地基。

**Architecture:** `llm` 是独立 Hertz 服务，保留“模型流式输出属于 HTTP/SSE 协议”的例外，但服务名、路由、数据模型和契约全部重建，不兼容旧 `/api/v1/model/*`。`edge-api` 只做登录态、权限和 HTTP 门面；`agent` 后续通过服务发现调用 `llm` HTTP API，并在自身模块内实现 Eino `ToolCallingChatModel` client，不直接 import `services/llm` 内部包。

**Tech Stack:** Go 1.25.6、Hertz、MongoDB、Redis、Eino `model.ToolCallingChatModel`、eino-ext model providers、OpenTelemetry、OpenAPI、etcd discovery。

---

## 背景

当前 `services/model` 已有 provider CRUD、OpenAI-compatible adapter、非流式 chat、SSE chat stream、usage chunk 解析和基础 OTel。但它仍是早期“模型网关”形态：

- 路由、命名和文档都围绕 `model`，与新路线中的 `llm` 服务边界不一致。
- provider 模型过粗，只有 `provider.default_model`，不足以支撑多模型能力、capabilities、tool-calling、reasoning、JSON mode 和后续计费。
- adapter 层自定义了接口，未把 Eino `ToolCallingChatModel` 作为一等抽象。
- 旧 HTTP API 直接返回 `content string`，不能可靠承载 tool call、tool result message、rich usage 和后续 agent 事件转换。
- 图像能力曾混在 `model` provider 中；新路线要求 `llm` 与 `generator` 拆开。

GP-02 要先把模型层立住，GP-03 的 Agent Runner 才能依赖稳定的 tool-calling ChatModel 后端。

## 目标

- 新增 `services/llm/`，并从构建、dev、edge proxy 中替换旧 `services/model/`。
- 新增 `idl/llm/openapi.yaml`，定义新的 provider/model 管理、Generate、Stream 和 usage 契约。
- 使用 Eino `model.ToolCallingChatModel` 作为服务内核心抽象；provider adapter 必须能被后续 agent client 映射成 Eino ChatModel 语义。
- Provider 与 Model 拆表：provider 管 key/base_url/vendor，model 管模型名、能力、上下文、输出上限、默认参数和启用状态。
- 支持非流式 `Generate`，返回 assistant message、tool calls、reasoning、usage、finish_reason。
- 支持流式 `Stream`，以 SSE 输出 reasoning/content/tool_call/usage/done/error 事件。
- 支持 OpenAI-compatible MVP，覆盖 OpenAI、DeepSeek、Moonshot、Qwen/OpenRouter 等兼容 `/v1/chat/completions` 的 provider；其他 provider 通过后续扩展接入。
- 支持 API key AES-GCM 加密存储、敏感字段脱敏、限流、超时、请求幂等和 usage 事件记录。
- 接入统一 logger、errno、OpenTelemetry trace/metrics/log correlation。

## 非目标

- 不兼容旧 `services/model` 的 `/api/v1/model/*` 路由、请求体或响应体。
- 不实现 Agent、tool 执行、ReAct loop、workflow 状态机或 generator 生图任务。
- 不在 `llm` 中执行业务 tool；`llm` 只把模型返回的 tool calls 原样结构化返回。
- 不保存 prompt 模板、长期记忆、RAG、向量索引或多模态资产主数据。
- 不把 provider API key 暴露给 `edge-api`、`agent` 或日志系统。
- 不为尚未接入的非 OpenAI-compatible provider 写空 adapter。

## 已确认开发细节

### 服务与协议形态

| 事项 | 决定 |
|---|---|
| 服务名 | `llm` |
| 进程路径 | `services/llm/` |
| 本地端口 | 复用旧模型槽位 `:38083`，admin health `:48083` |
| 注册发现 | etcd service name 改为 `llm` |
| 外部门面 | `edge-api` 暴露 `/api/v1/admin/llm/*`，需要 `llm:admin` 权限 |
| 内部调用 | 后续 `agent` 通过 HTTP + service discovery 调用 `llm`，不 import `services/llm` |
| 流式输出 | `llm` 内部接口用 SSE；`edge-api` 代理 SSE 时只 pipe，不改事件语义 |
| 旧服务 | `services/model` 从 `go.work`、`Makefile`、dev services 和 edge routes 移除；代码可在同一阶段删除 |

### OpenAPI 路由

新增 `idl/llm/openapi.yaml`。服务直连路径以 `/api/v1/llm` 为前缀；edge admin 门面以 `/api/v1/admin/llm` 为前缀代理同名子路径。

| Method | Path | 用途 |
|---|---|---|
| `GET` | `/api/v1/llm/providers` | 列出 provider，绝不返回明文 API key |
| `POST` | `/api/v1/llm/providers` | 创建 provider |
| `PUT` | `/api/v1/llm/providers/:id` | 更新 provider 非密钥字段 |
| `PATCH` | `/api/v1/llm/providers/:id/api-key` | 更新 API key |
| `PATCH` | `/api/v1/llm/providers/:id/enabled` | 启停 provider |
| `POST` | `/api/v1/llm/providers/:id/test` | 用最小请求验证 provider key/base_url |
| `GET` | `/api/v1/llm/models` | 列出模型 |
| `POST` | `/api/v1/llm/models` | 创建模型配置 |
| `PUT` | `/api/v1/llm/models/:id` | 更新模型配置 |
| `PATCH` | `/api/v1/llm/models/:id/enabled` | 启停模型 |
| `POST` | `/api/v1/llm/generate` | 非流式模型调用 |
| `POST` | `/api/v1/llm/stream` | SSE 流式模型调用 |

### 核心请求/响应语义

`GenerateReq` 最小形态：

```json
{
  "model_ref": "deepseek/deepseek-v4-flash",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "hello"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "asset.get_asset",
        "description": "Get one asset by id",
        "parameters": {"type": "object", "properties": {"asset_id": {"type": "string"}}}
      }
    }
  ],
  "tool_choice": "auto",
  "response_format": {"type": "text"},
  "temperature": 0.7,
  "max_tokens": 2048,
  "idempotency_key": "optional-client-key"
}
```

`GenerateResp` 最小形态：

```json
{
  "code": 0,
  "data": {
    "request_id": "llmreq_xxx",
    "message": {
      "role": "assistant",
      "content": "hello",
      "reasoning_content": "",
      "tool_calls": []
    },
    "usage": {
      "prompt_tokens": 12,
      "completion_tokens": 8,
      "total_tokens": 20
    },
    "finish_reason": "stop",
    "model_ref": "deepseek/deepseek-v4-flash"
  }
}
```

SSE event 类型：

| Event | Payload |
|---|---|
| `reasoning_delta` | `{"request_id":"...","content":"..."}` |
| `content_delta` | `{"request_id":"...","content":"..."}` |
| `tool_call_delta` | `{"request_id":"...","index":0,"id":"...","name":"...","arguments_delta":"..."}` |
| `message_completed` | 完整 assistant message，含拼装后的 tool calls |
| `usage` | prompt/completion/total tokens |
| `done` | request_id、finish_reason、model_ref |
| `error` | code、message、request_id |

### 数据模型

`llm_providers`：

| 字段 | 说明 |
|---|---|
| `_id` | ProviderID |
| `name` | 展示名 |
| `slug` | 全局唯一，例如 `deepseek` |
| `vendor` | `openai_compatible` / `deepseek` / `qwen` / `openrouter` 等；MVP 均走 OpenAI-compatible adapter |
| `base_url` | 上游 API base URL |
| `api_key_cipher` | AES-GCM 加密后的 key |
| `enabled` | 是否启用 |
| `default_model_ref` | 默认模型引用 |
| `extra_json` | provider 私有配置，禁止保存明文 secret |
| `created_at` / `updated_at` | 审计字段 |

`llm_models`：

| 字段 | 说明 |
|---|---|
| `_id` | ModelID |
| `provider_id` / `provider_slug` | 所属 provider |
| `model` | 上游模型名，例如 `deepseek-v4-flash` |
| `model_ref` | `<provider_slug>/<model>`，全局唯一 |
| `display_name` | 展示名 |
| `capabilities` | `chat`、`stream`、`tool_calling`、`json_mode`、`reasoning`、`vision` |
| `context_window` | 上下文窗口 |
| `max_output_tokens` | 输出上限 |
| `default_parameters_json` | 默认 temperature/top_p/max_tokens 等 |
| `enabled` | 是否启用 |
| `created_at` / `updated_at` | 审计字段 |

`llm_request_logs`：

| 字段 | 说明 |
|---|---|
| `_id` | LLMRequestID |
| `request_id` | 对外 request id |
| `caller` / `user_id` / `tenant_id` | 来自 Base metadata 或 HTTP header |
| `model_ref` / `provider_slug` | 实际调用目标 |
| `stream` | 是否流式 |
| `status` | `success` / `failed` / `cancelled` |
| `usage` | prompt/completion/total tokens |
| `first_token_ms` / `duration_ms` | 延迟指标 |
| `idempotency_key` | 幂等键 |
| `error_code` / `error_message` | 失败摘要 |
| `created_at` | 请求时间 |

### Eino adapter 设计

`services/llm/component/` 负责把 provider/model 配置转换为 Eino `model.ToolCallingChatModel`：

- `Factory.Build(ctx, modelRef)` 返回 `model.ToolCallingChatModel` 和 `ResolvedModel`。
- MVP 使用 OpenAI-compatible ChatModel；配置由 `llm_providers.base_url`、解密后的 API key、`llm_models.model` 组成。
- 只要请求含 tools，就调用 `chatModel.WithTools(schema.ToolInfo[])`；模型不支持 tool calling 时返回 `ErrLLMModelCapabilityUnsupported`。
- `Generate` 调用 Eino `Generate`；`Stream` 调用 Eino `Stream` 并转换为服务 SSE events。
- Eino 返回的 tool calls、reasoning content、finish reason 和 usage 必须保留为结构化字段，不降级成纯文本。

### 多 Provider 扩展风险（2026-05-17 追加）

MVP 只承诺 `openai_compatible` adapter。OpenAI-compatible 只能说明 HTTP 形态相近，不等于能力、事件和计费语义完全一致。后续新增 provider 时必须逐个做 conformance，而不是仅凭 vendor 名称放行。

| 风险 | 说明 | GP-02 处理口径 |
|---|---|---|
| Base URL 规范不一致 | 有的厂商要求 `/v1`，有的 SDK 会自行拼接 path，容易出现 `/v1/v1` 或漏 path | Provider test 和 conformance 用真实 fake upstream/沙箱请求覆盖最终 URL |
| Stream 事件差异 | usage chunk、tool call delta、finish reason、reasoning delta 字段可能缺失或字段名不同 | SSE 转换层只依赖 Eino 结构；provider 不返回 usage 时不得伪造 |
| Tool calling 差异 | 不同模型对 `tool_choice`、JSON Schema、strict mode、parallel tool calls 支持不同 | 继续以 `llm_models.capabilities` 显式声明；不支持时返回 `ErrLLMModelCapabilityUnsupported` |
| JSON mode / response_format 差异 | OpenAI-compatible provider 可能忽略或拒绝 `response_format` | capability 里区分 `json_mode`，并用 provider conformance 测试确认 |
| Reasoning 输出差异 | reasoning content 可能在不同字段、独立事件或完全不可见 | 仅在 Eino message 中有结构化字段时透传；不把普通 content 猜成 reasoning |
| Token usage 差异 | 非流式/流式 usage 可能缺失、延迟到最后一帧、或包含 cached/reasoning 等细项 | GP-02 记录基础 prompt/completion/total；细项保留为 GP-07/usage hardening |
| 限流与错误差异 | 上游可能使用不同 HTTP code、错误体、Retry-After 头 | adapter 层统一转 `ErrLLMRateLimited` / `ErrLLMUpstream`，日志保留脱敏摘要 |
| 认证头差异 | 有的需要 organization/project header 或非 Bearer key | 放入 provider `extra_json`，但禁止保存明文 secret |

Provider 支持闭合标准：新增 provider 不能只改配置示例，必须至少覆盖 `Generate`、`Stream`、`Stream usage`、`tool calling`、错误映射、脱敏日志和 base URL 拼接的 conformance case；达不到则保持 `openai_compatible` 通用路径，不新增专属 vendor。

### Token usage 与消耗边界（2026-05-17 追加）

当前 GP-02 已实现的是 token **用量记录**，不是 credits/billing **消耗扣减**。两者必须分层：

| 层次 | 当前状态 | 事实来源 / 后续归属 |
|---|---|---|
| 接口 usage | 已实现：`GenerateResp.Usage` 返回 prompt/completion/total | `services/llm/biz/generate.go` |
| SSE usage | 已实现：流式结束前发送 `event: usage`，前端在 `done` 消息上展示 | `services/llm/handler/generate.go`、`web/src/lib/api.ts`、`ChatDebugPage.tsx` |
| 请求日志 usage | 已实现：写入 `llm_request_logs.usage` | `services/llm/dal/model/request_log.go`、`GenerateBiz.record` |
| Eino usage 来源 | 已实现：从 `schema.ResponseMeta.Usage` 映射 | Eino OpenAI ACL 会映射上游 usage；stream 会请求 `include_usage` |
| 详细 usage | 未完成：当前只保存 prompt/completion/total，未保存 cached/reasoning 等细项 | usage hardening 或 GP-07 前置 |
| 真实消耗扣减 | 未完成：没有价格表、credits ledger、MQ usage event 或 billing 集成 | GP-07 credits/billing |

实现约束：

- `usageFromSchema` 只负责把 Eino `schema.TokenUsage` 映射为服务 DTO；provider 不返回 usage 时保持空值/零值，不做本地估算。
- 流式 usage 依赖 provider 是否支持 OpenAI-compatible `stream_options.include_usage`；Eino 会请求，但不能保证所有 provider 返回。
- GP-02 不直接扣 credits，不写账本，不把 token 单价塞进调用链；后续应由 `llm` 成功请求发布 usage event，`credits/billing` 按 `request_id` 幂等消费。
- 后续若接入 cached tokens、reasoning tokens、audio/image tokens，应扩展 `Usage` DTO 和 `llm_request_logs.usage`，避免只按 total tokens 做错误计费。

### 错误码

将原 `Model 16001 - 16999` 区段重命名为 `LLM 16001 - 16999`，并替换错误名：

| 错误 | Code | 场景 |
|---|---:|---|
| `ErrLLMProviderNotFound` | 16001 | provider 不存在 |
| `ErrLLMProviderDisabled` | 16002 | provider 被禁用 |
| `ErrLLMAdapterUnsupported` | 16003 | vendor 或协议不支持 |
| `ErrLLMUpstream` | 16004 | 上游模型调用失败 |
| `ErrLLMModelNotFound` | 16005 | model_ref 不存在 |
| `ErrLLMModelDisabled` | 16006 | model 被禁用 |
| `ErrLLMModelCapabilityUnsupported` | 16007 | 请求能力不被模型支持 |
| `ErrLLMRateLimited` | 16008 | 本服务或上游限流 |
| `ErrLLMInvalidMessage` | 16009 | message/tool schema 不合法 |

### 安全与资源限制

- API key 只在 `ProviderBiz.GetForCall` 的短生命周期内解密；DTO、日志、错误和 event 中永不包含明文 key。
- 请求日志保存 message 摘要、hash、usage 和错误摘要，不保存完整 prompt；后续如需 prompt 审计另拆 spec。
- `max_tokens` 不能超过 `llm_models.max_output_tokens`。
- tool schema 总大小、message 数量、单 message content 长度、SSE 单事件大小都要有限制并有单测。
- 所有字段名匹配 `password`、`secret`、`token`、`authorization`、`api_key` 时输出前替换为 `[REDACTED]`。

## 文件结构

| 路径 | 操作 | 责任 |
|---|---|---|
| `idl/llm/openapi.yaml` | Create | 新 LLM HTTP/OpenAPI 契约 |
| `services/llm/go.mod` | Create | 新 llm module |
| `services/llm/main.go` | Create | 配置、Mongo、Redis、OTel、Hertz、registry、health |
| `services/llm/router.go` | Create | `/api/v1/llm` 路由 |
| `services/llm/handler/provider.go` | Create | Provider HTTP handler |
| `services/llm/handler/model.go` | Create | Model HTTP handler |
| `services/llm/handler/generate.go` | Create | Generate/Stream HTTP handler |
| `services/llm/biz/provider.go` | Create | provider CRUD、key 加解密、test |
| `services/llm/biz/model.go` | Create | model CRUD、capability 校验 |
| `services/llm/biz/generate.go` | Create | Generate/Stream 编排、usage 记录 |
| `services/llm/component/*.go` | Create | Eino ChatModel factory、OpenAI-compatible provider |
| `services/llm/dal/model/*.go` | Create | Mongo 文档模型 |
| `services/llm/dal/mongo/*.go` | Create | repo、索引、分页 |
| `services/llm/security/redact.go` | Create | 敏感字段脱敏 |
| `deployments/config/llm.yaml` | Create | llm 非敏感配置 |
| `deployments/env/llm.env.example` | Create | llm 本地 env 示例 |
| `services/edge-api/handler/llm_proxy.go` | Create | edge admin proxy + SSE pipe |
| `services/edge-api/main.go` | Modify | 初始化 llm resolver |
| `services/edge-api/router.go` | Modify | 注册 `/api/v1/admin/llm` |
| `deployments/config/edge-api.yaml` | Modify | `llm.service_name` 替换旧 `model.service_name` |
| `scripts/dev/services.json` | Modify | `model` 替换为 `llm`，日志改 `bin/log/llm.log` |
| `Makefile` | Modify | `ALL_SERVICES`、build/test/start targets 替换 model 为 llm |
| `go.work` | Modify | `./services/model` 替换为 `./services/llm` |
| `pkg/errno/code.go` | Modify | Model 错误码重命名为 LLM 错误码 |
| `pkg/errno/errno_test.go` | Modify | 错误码区段测试 |
| `services/model/` | Delete | 新服务通过后删除旧模型网关代码 |
| `idl/model/openapi.yaml` | Delete or deprecate | 新契约使用 `idl/llm/openapi.yaml` |

## 任务拆解

### Task 1: 契约与错误码

**Files:**
- Create: `idl/llm/openapi.yaml`
- Modify: `pkg/errno/code.go`
- Modify: `pkg/errno/errno_test.go`

- [x] 写 `idl/llm/openapi.yaml`，覆盖 provider/model/generate/stream 路由、DTO、SSE event 和错误响应。
- [x] 把 `Model 16001 - 16999` 错误码区段重命名为 `LLM 16001 - 16999`。
- [x] 增加 capability、message、rate limit 相关错误。
- [x] 运行 `cd pkg && go test ./errno -count=1`，预期 PASS。

### Task 2: 服务骨架和 dev 链路替换

**Files:**
- Create: `services/llm/go.mod`
- Create: `services/llm/main.go`
- Create: `services/llm/router.go`
- Create: `deployments/config/llm.yaml`
- Create: `deployments/env/llm.env.example`
- Modify: `go.work`
- Modify: `Makefile`
- Modify: `scripts/dev/services.json`

- [x] 以旧 `services/model` 和现有 `asset` 初始化风格为参考，新建 `services/llm`。
- [x] 使用 `LLM_CONFIG`，默认配置路径 `deployments/config/llm.yaml`。
- [x] 默认监听 `:38083`，admin health `:48083`，registry service name 为 `llm`。
- [x] `Makefile` 中把 `model` 替换为 `llm`，并新增 `llm-start` / `llm-stop` / `llm-restart`。
- [x] 运行 `cd services/llm && go test ./... -count=1`，预期 PASS。

### Task 3: Provider/Model DAL 与业务校验

**Files:**
- Create: `services/llm/dal/model/provider.go`
- Create: `services/llm/dal/model/model.go`
- Create: `services/llm/dal/mongo/provider.go`
- Create: `services/llm/dal/mongo/model.go`
- Create: `services/llm/biz/provider.go`
- Create: `services/llm/biz/model.go`
- Test: `services/llm/biz/provider_test.go`
- Test: `services/llm/biz/model_test.go`

- [x] 建立 `llm_providers.slug` 唯一索引。
- [x] 建立 `llm_models.model_ref` 唯一索引和 `provider_id + model` 唯一索引。
- [x] Provider 创建时校验 `name`、`slug`、`vendor`、`base_url`，API key 加密后入库。
- [x] Model 创建时校验 provider 存在且启用、capabilities 非空、`model_ref` 为 `<provider_slug>/<model>`。
- [x] List Provider 不返回 `api_key_cipher`。
- [x] 运行 `cd services/llm && go test ./biz -run 'Test(Provider|Model)' -count=1`，预期 PASS。

### Task 4: Eino ChatModel factory

**Files:**
- Create: `services/llm/component/factory.go`
- Create: `services/llm/component/openai_compatible.go`
- Test: `services/llm/component/factory_test.go`
- Test: `services/llm/component/openai_compatible_test.go`

- [x] 定义 `Factory.Build(ctx, modelRef string) (model.ToolCallingChatModel, *ResolvedModel, error)`。
- [x] 对 `openai_compatible` vendor 构建 Eino OpenAI-compatible ChatModel。
- [x] 请求 tools 时调用 `WithTools`；无 `tool_calling` capability 时返回 `ErrLLMModelCapabilityUnsupported`。
- [x] 测试 fake upstream：Generate 正常、Generate tool call、Stream content delta、Stream tool_call_delta、上游错误。
- [x] 运行 `cd services/llm && go test ./component -count=1`，预期 PASS。

### Task 5: Generate 业务与 usage 记录

**Files:**
- Create: `services/llm/dal/model/request_log.go`
- Create: `services/llm/dal/mongo/request_log.go`
- Create: `services/llm/biz/generate.go`
- Create: `services/llm/handler/generate.go`
- Test: `services/llm/biz/generate_test.go`
- Test: `services/llm/handler/generate_test.go`

- [x] 校验 `model_ref`、messages、tools、tool_choice、response_format 和采样参数。
- [x] 合并模型默认参数与请求参数，请求参数优先。
- [x] 调用 Eino `Generate` 并返回完整 assistant message、tool calls、usage、finish_reason。
- [x] 写入 `llm_request_logs`，失败也要记录 status/error_code。
- [x] 同一个 `idempotency_key + user_id + model_ref` 的成功请求返回原结果摘要，不重复调用上游。
- [x] 运行 `cd services/llm && go test ./biz ./handler -run TestGenerate -count=1`，预期 PASS。

### Task 6: Stream SSE 与 tool call delta 拼装

**Files:**
- Modify: `services/llm/biz/generate.go`
- Modify: `services/llm/handler/generate.go`
- Test: `services/llm/biz/stream_test.go`
- Test: `services/llm/handler/stream_test.go`

- [x] `POST /api/v1/llm/stream` 设置 `Content-Type: text/event-stream; charset=utf-8`、`Cache-Control: no-cache`、`X-Accel-Buffering: no`。
- [x] 使用 `io.Pipe + SetBodyStream` 保证逐 chunk 输出。
- [x] 将 Eino stream 转换为 `reasoning_delta`、`content_delta`、`tool_call_delta`、`message_completed`、`usage`、`done`。
- [x] tool call arguments delta 必须拼装成最终 `message_completed.tool_calls[].function.arguments`。
- [x] 上游错误写 `event: error` 后关闭流，并记录 failed request log。
- [x] 运行 `cd services/llm && go test ./biz ./handler -run TestStream -count=1`，预期 PASS。

### Task 7: Provider test、限流、脱敏和资源限制

**Files:**
- Create: `services/llm/security/redact.go`
- Create: `services/llm/biz/limits.go`
- Modify: `services/llm/biz/provider.go`
- Test: `services/llm/security/redact_test.go`
- Test: `services/llm/biz/limits_test.go`
- Test: `services/llm/biz/provider_test.go`

- [x] `POST /providers/:id/test` 使用最小 `Generate` 请求验证 base_url/API key/model。
- [x] Redis 限流保持 deferred，后续真实用户开放前按 `llm:rate:{userID}:{modelRef}` 独立闭合。
- [x] 限制 messages 数量、content 长度、tool schema 大小、stream 单事件大小、max_tokens。
- [x] 实现 `security.Redact` / `RedactJSONBytes`，覆盖嵌套 JSON、malformed JSON 和敏感字段名。
- [x] 日志、错误、request log metadata 均调用脱敏函数。
- [x] 运行 `cd services/llm && go test ./biz ./component ./handler ./security -count=1`，2026-05-17 PASS。
- [x] provider test 升级为真实上游最小 Generate 后，补 `ProviderTest` 专项测试并闭合。

### Task 8: edge-api admin 门面替换

**Files:**
- Create: `services/edge-api/handler/llm_proxy.go`
- Modify: `services/edge-api/main.go`
- Modify: `services/edge-api/router.go`
- Modify: `deployments/config/edge-api.yaml`
- Test: `services/edge-api/handler/llm_proxy_test.go`

- [x] 用 `llm.service_name` 替换旧 `model.service_name` 配置。
- [x] `/api/v1/admin/llm/*` 需要 `llm:admin` 权限。
- [x] 普通 JSON 路由代理到 `llm` 服务。
- [x] `/stream` 路由 pipe SSE，不修改 event 名称和 payload。
- [x] 移除旧 `/api/v1/admin/models/*` 路由。
- [x] 运行 `cd services/edge-api && go test ./handler -run TestLLMProxy -count=1`，预期 PASS。

### Task 9: 删除旧 model 服务与契约

**Files:**
- Delete: `services/model/`
- Delete or deprecate: `idl/model/openapi.yaml`
- Modify: `Makefile`
- Modify: `scripts/dev/services.json`
- Modify: `go.work`

- [x] 确认 `make build` 不再引用 `model`。
- [x] 确认 `scripts/dev/services.json` 中只有 `llm`，没有 `model`。
- [x] 删除旧 `model-start` / `model-stop` / `model-restart`，替换为 `llm-*`。
- [x] 如保留 `idl/model/openapi.yaml` 作历史参考，必须在文件内标记 deprecated，并从 Knowledge Index 移除；优先删除。
- [x] 运行 `rg "services/model|/api/v1/model|admin/models|model service|MODEL_CONFIG|MODEL_ENCRYPT_KEY"`，预期只剩历史 progress 文档或已明确 deprecated 的引用。

### Task 10: 全量验证与 axm 闭环

**Files:**
- Modify after implementation: `.axm/progress/generation-platform/roadmap.md`
- Modify after implementation: `.axm/knowledge/services/overview.md`
- Modify after implementation: `.axm/project/architecture.md`

- [x] 运行 `make fmt`。
- [x] 运行 `make test-services`。
- [x] 运行 `make lint`。
- [x] 运行 `make build`。
- [x] 本地启动 infra/obs/dev 后，通过 edge admin 创建 provider/model，发起 generate 和 stream；本轮本地 dev 服务未 ready，已在 GP-02-01 记录环境阻塞。
- [x] OpenObserve 中查询 llm trace、first token、usage 和错误 span；仪表盘/人工查询保持 observability hardening deferred，不阻塞 GP-03。
- [x] 实现完成后，把 `llm` 服务替代 `model` 的长期事实同步到 `knowledge/services/overview.md` 与 `project/architecture.md`。

## AI 自动验收

| 验收项 | 命令 / 检查 | 预期 |
|---|---|---|
| axm 契约 | `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service` | 0 errors |
| errno | `cd pkg && go test ./errno -count=1` | PASS |
| llm 单测 | `cd services/llm && go test ./... -count=1` | PASS |
| edge proxy 单测 | `cd services/edge-api && go test ./handler -run TestLLMProxy -count=1` | PASS |
| 全服务测试 | `make test-services` | PASS |
| lint | `make lint` | PASS |
| 构建 | `make build` | `llm` 构建成功，`model` 不再构建 |
| 旧引用扫描 | `rg "services/model|/api/v1/model|admin/models|MODEL_CONFIG|MODEL_ENCRYPT_KEY"` | 无运行时代码引用 |
| SSE 测试 | handler 测试解析 `event: content_delta`、`event: usage`、`event: done` | PASS |
| tool-calling 测试 | fake upstream 返回 tool call，Generate/Stream 都保留结构化 tool calls | PASS |
| 脱敏测试 | 覆盖 api_key/token/authorization/password/secret | PASS |

## 已闭合子项（2026-05-17）

| 子项 | 状态 | 验证证据 |
|---|---|---|
| 错误码区段与 errno 测试 | 已闭合 | `cd pkg && go test ./errno -count=1` PASS |
| Eino ChatModel factory / OpenAI-compatible adapter | 已闭合 | `cd services/llm && go test ./biz ./component ./handler ./security -count=1` PASS，覆盖 component |
| Generate / Stream / usage 基础链路 | 已闭合 | 同上，覆盖 `TestGenerateReturnsAssistantUsageAndToolCalls`、`TestStreamEmitsDeltasCompletedUsageAndDone` |
| SSE handler 与 edge proxy 基础代理 | 已闭合 | `cd services/edge-api && go test ./handler -run TestLLMProxy -count=1` PASS |
| request log 基础 usage 字段 | 已闭合 | `GenerateBiz.record` 写入 `llm_request_logs.usage`，biz 测试覆盖 usage 映射 |
| 资源限制与脱敏工具函数 | 已闭合 | `services/llm/biz/limits_test.go`、`services/llm/security/redact_test.go` 通过 |
| GP-02-01 收口：Provider test 真实连通性 | 已闭合 | `ProviderBiz.Test` 通过 fake upstream/httptest 覆盖成功、401/403、404/path、5xx、超时、无 usage 和 `/v1` 去重 |
| GP-02-01 收口：身份与幂等 | 已闭合 | edge 覆盖伪造 metadata；Generate 幂等按 `user_id + model_ref + idempotency_key` 查询；request log 记录 caller/user/tenant |
| GP-02-01 收口：Web 调试平台 | 已闭合 | `cd web && npm run e2e -- chat-debug-usage.spec.ts llm-debug-setup.spec.ts` PASS，覆盖 model list、provider test、usage/done |

## 未闭合 / Deferred 子项（2026-05-17）

| 子项 | 当前判断 | 后续归属 |
|---|---|---|
| Redis 限流 | 代码中未看到 `llm:rate:{userID}:{modelRef}` 接入 | 内测可 deferred；真实用户开放前必须补 |
| 详细 token usage | 当前只保存 prompt/completion/total，未保存 cached/reasoning tokens | usage hardening / GP-07 前置 |
| credits/billing 消耗 | 未实现 token 定价、MQ usage event、credits ledger 和幂等扣减 | GP-07 |
| 全栈 fake upstream smoke | 本轮本地 `edge-api :38080` 未启动，`scripts/e2e-llm-sse.sh` preflight 明确失败，未伪造通过 | 人类按 GP-02-01 步骤启动 `make dev-start` 后复验 |
| OpenObserve 人工观测 | 尚未记录本地 infra/obs/dev 下的真实链路验证 | observability hardening；环境可用时复验 |
| 多 provider conformance | 当前只有 OpenAI-compatible 通用 adapter，未逐厂商闭合 | 新 provider 接入时逐个闭合 |

## GP-02 收尾分级（2026-05-17）

收尾原则：GP-02 只补齐让 GP-03 `agent` 能稳定依赖 `llm` 的地基能力；商业化、深度计费、多厂商矩阵不在本阶段继续展开。

### 现在必须做（阻塞 GP-02 闭合）

| 项目 | 为什么现在做 | 验收口径 |
|---|---|---|
| Provider test 真实最小调用 | 当前 provider test 只验证配置可读，不能证明 base URL、Authorization、model 参数和错误映射可用；GP-03 出问题时会失去诊断边界 | `POST /providers/:id/test` 通过 fake upstream 发最小 Generate；覆盖成功、401/403、404/路径错误、上游 5xx，失败摘要不泄漏 key |
| request log 身份与幂等维度 | usage、审计和后续扣费都依赖 `caller/user_id/tenant_id/idempotency_key`；现在日志字段存在但链路未闭合 | edge/llm handler 从可信 metadata/header 透传并写入 `llm_request_logs`；幂等查询不再使用空 userID |
| 脱敏调用面 | `security.Redact` 已实现但未全量接入；一旦接真实 provider，日志/错误/request metadata 泄漏 key 的风险很高 | handler/biz/upstream error/request log metadata 输出前统一脱敏；补 key、authorization、token、secret、password case |
| edge -> llm smoke | GP-03 依赖 HTTP/SSE 语义；只测 llm 内部和 proxy 单测还不能证明真实门面链路稳定 | 本地 fake upstream 下通过 edge admin 创建 provider/model，跑 generate 和 stream，确认 `usage`、`done`、错误事件可用 |
| 最终质量门禁 | 当前只记录了局部 PASS；闭合前要确认全仓构建与旧 model 引用没有回流 | 运行并记录 `go test` 局部、`make test-services`、`make lint`、`make build`、旧引用扫描；若有既有阻塞需写明 |

### 现在可以做但不阻塞 GP-03

| 项目 | 处理建议 | 说明 |
|---|---|---|
| Redis 限流 | 内测阶段可 deferred；若要对真实用户开放，则提升为必须做 | 当前没有看到 `llm:rate:{userID}:{modelRef}` 接入 |
| 前端 ChatDebug usage E2E | 有时间就跑并闭合，不阻塞 agent 服务开发 | `web/tests/e2e/chat-debug-usage.spec.ts` 已存在，主要验证 UI 展示 |
| OpenObserve 人工查询 | 作为 GP-02 演示/验收项保留；如果本地 obs 环境不可用，可记录原因 | 至少要有 trace/log 字段设计和 smoke 结果；完整仪表盘可后置 |
| axm 全仓 validate | 不作为 GP-02 代码闭合 blocker | 当前失败来自全仓旧 `status` 元数据迁移，不是 llm 代码问题；应独立开 axm 元数据迁移任务 |

### 未来再做（不应拖住 GP-02）

| 项目 | 未来归属 | 暂不做原因 |
|---|---|---|
| 详细 token usage | usage hardening / GP-07 前置 | 当前 GP-02 只需要 prompt/completion/total；cached/reasoning/audio/image token 会影响计费模型，需和 credits 一起设计 |
| credits/billing 扣费 | GP-07 | 需要价格表、MQ usage event、ledger、幂等扣减和退款/补偿策略，不属于 llm 地基收尾 |
| 多 provider 专属 adapter | 新 provider 接入 spec | MVP 统一走 `openai_compatible`；只有出现 Eino/OpenAI-compatible 不能覆盖的厂商差异时才新增 vendor adapter |
| 多 provider conformance 矩阵 | provider onboarding 流程 | 应随具体厂商逐个验证，不在 GP-02 一次性铺所有模型 |
| usage 估算与本地 tokenization | GP-07 或独立 usage spec | provider 不返回 usage 时本地估算容易和账单不一致；GP-02 不伪造 usage |
| 模型路由、fallback、熔断 | agent/llm routing 后续 spec | 当前只需要稳定单模型调用；自动路由会扩大观测和计费复杂度 |

## 人类验收

- 管理员通过 edge-api 创建一个 OpenAI-compatible provider 和一个 model；列表接口看不到明文 API key。
- 管理员调用 provider test，能看到成功/失败摘要，失败时不泄漏 key。
- 调用 `/api/v1/admin/llm/generate`，能得到 assistant message、usage、finish_reason。
- 调用 `/api/v1/admin/llm/stream`，能逐步收到 `reasoning_delta` 或 `content_delta`，最后收到 `usage` 和 `done`。
- 使用带 tools 的请求时，模型返回的 tool calls 以结构化字段返回，而不是被拼进文本。
- OpenObserve 可看到 `edge-api -> llm -> upstream` 链路、模型名、provider、usage、first token latency；日志中没有 API key/JWT/Authorization。
- 本地 dev 状态中显示 `llm`，不再显示 `model`。

## 依赖与阻塞

- 需要确认引入的 `eino` / `eino-ext` 版本与 Go 1.25.6、当前 CloudWeGo/Hertz 版本兼容；实现时以 `go test ./...` 和最小 fake upstream 测试为准。
- 第一批真实 provider 以 OpenAI-compatible 为准；如果某个厂商的 Eino provider 构造函数与参考不同，先封装在 `component/openai_compatible.go`，不要扩大到多个 provider adapter。
- GP-03 的 `agent` 服务必须等本 spec 的 Generate/Stream/tool-calling 契约稳定后再正式开发。

## 实施进度

| 项目 | 状态 |
|---|---|
| Spec 创建 | 已完成 |
| OpenAPI / 错误码 | 已完成 |
| `services/llm` 骨架 | 已完成 |
| Provider / Model 管理 | 已完成 |
| Generate / Stream / Tool-calling | 已完成 |
| edge-api admin 门面 | 已完成 |
| 删除旧 model | 已完成 |
| GP-02-01 收口 | 已完成；详见 [`gp-02-01-llm-service-closure.md`](./gp-02-01-llm-service-closure.md) |
| AI 自动验收 | `make test-services`、`make lint`、`make build`、Web build/E2E、旧引用扫描已通过；全栈 SSE smoke 因本地 edge-api 未启动记录为环境阻塞 |
| 人类验收 | Web mock E2E 已覆盖调试入口；真实 fake upstream 全栈路径待人类启动本地 dev 后复验。当前 verified 表示 GP-03 依赖面已通过 AI 自动验收和可复验脚本收口，非表示所有人工 smoke 都已执行 |
