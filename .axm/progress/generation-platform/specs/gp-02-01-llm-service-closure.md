<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: generation-platform
workflow-state: verified
state-updated: 2026-05-17
related:
  - ./gp-02-llm-service-rebuild.md
  - ./gp-03-agent-service-mvp.md
  - ../roadmap.md
  - ../../../project/coding.md
  - ../../../project/api-testing.md
  - ../../../knowledge/services/overview.md
-->

# GP-02-01 LLM 服务收口 Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 收口 GP-02 已落地的 `llm` 服务，让 GP-03 `agent` 可以稳定依赖 Generate/Stream/tool-calling 契约，并让人类可以通过 Web 调试平台完成可重复的本地验证。

**Architecture:** 本 spec 不重建 GP-02 的服务边界，只补齐闭合缺口：provider 真实连通性、身份/幂等元数据、脱敏调用面、edge 到 llm 的真实 smoke，以及 Web 调试入口。能提升体验但不影响 GP-03 依赖面的功能默认 deferred，避免 GP-02 收口继续扩张。

**Tech Stack:** Go 1.25.6、Hertz、MongoDB、Redis、Eino OpenAI-compatible ChatModel、React/Vite、Playwright、Kong/edge-api、本地 fake OpenAI-compatible upstream。

---

## 背景

GP-02 主 spec 已完成大部分代码落地：`services/llm`、OpenAPI、provider/model 管理、Generate、Stream、SSE usage、edge proxy、旧 `model` 删除和基础 Web Chat 调试页都已经出现。

但 GP-02 仍有几类会影响后续 GP-03 的收尾缺口：

- Provider test 目前只证明配置可读，不能证明 base URL、Authorization、model 参数和上游错误映射真实可用。
- `llm_request_logs` 已有 `caller/user_id/tenant_id/idempotency_key` 字段，但请求链路仍需闭合可信来源，幂等查询不能继续使用空 userID。
- 脱敏工具已存在，但日志、错误、request log metadata、上游错误摘要等调用面需要统一接入。
- Web 端已有 provider 页面与 Chat 调试页，但人类要完成“配置 provider/model -> provider test -> stream 调试 -> 看 usage/error”的路径还不够顺。
- GP-03 需要的是一个稳定的 LLM 依赖面，不需要 GP-02 在此阶段继续展开计费、多 provider 矩阵、路由/fallback 或复杂控制台。

## 收口原则

- **只做阻塞 GP-03 的闭合项。** GP-03 需要稳定模型调用、流式事件、tool call 结构、usage、身份/审计和可诊断错误。
- **Web 调试平台是必须项。** 它不是产品化后台，而是人类验收和排障入口；必须能用本地 fake upstream 稳定复现成功、usage 和错误路径。
- **默认不做可后置项。** Redis 限流、真实厂商 conformance 矩阵、详细 token 计费、OpenObserve dashboard、前端高级体验都不阻塞 GP-03，除非用户单独提升优先级。
- **不引入新服务边界。** `llm` 仍只负责模型调用；不做 Agent、tool 执行、credits/billing 或 generator。

## 必须完成范围

| 项目 | 为什么必须做 | GP-03 依赖 |
|---|---|---|
| Provider test 真实最小 Generate | 快速区分“配置错 / key 错 / base URL 错 / 上游错 / llm 逻辑错” | Agent 配置模型前要能诊断模型不可用原因 |
| 请求身份与幂等闭合 | usage、审计、后续扣费、重复提交都依赖可信 `user_id` 与 `idempotency_key` | Agent run 重试不能把不同用户的请求混到同一幂等记录 |
| 脱敏调用面闭合 | 接真实 provider 后日志和错误不能泄漏 API key/JWT/Authorization | Agent 调试和运行日志会引用 LLM 错误摘要 |
| edge -> llm Generate/Stream smoke | 单测不能证明 Kong/edge proxy/SSE pipe 的真实门面链路 | Agent HTTP client 要依赖同一类 SSE/HTTP 语义 |
| Web 调试平台可用 | 人类需要稳定验证 provider/model、stream、usage、错误和 stop | GP-03 开发时可用它排查 llm 侧问题 |
| 最终质量门禁 | 确认旧 `model` 不回流，`llm` build/test/lint 可交付 | GP-03 可以从稳定分支继续开发 |

## 默认 Deferred 范围

这些可以做，但默认不做，不阻塞 GP-03：

| 项目 | 后续归属 | 默认不做原因 |
|---|---|---|
| Redis 限流 `llm:rate:{userID}:{modelRef}` | 真实用户开放前或独立 hardening | 内测/GP-03 开发不依赖；会引入 Redis 策略和配置面 |
| OpenObserve dashboard / 完整人工查询手册 | observability hardening | GP-02 只需 trace/log 字段和 smoke 结果；仪表盘可后置 |
| 多厂商真实 conformance 矩阵 | provider onboarding spec | MVP 只承诺 OpenAI-compatible；每个真实 provider 应按接入时逐个闭合 |
| cached/reasoning/audio/image token 细项 | usage hardening / GP-07 | 影响计费模型，需要和 credits/billing 一起设计 |
| credits/billing 扣费、MQ usage event、价格表 | GP-07 | 不属于 LLM 地基收口 |
| 模型路由、fallback、熔断 | agent/llm routing 后续 spec | GP-03 只需要单模型稳定调用 |
| 前端高级产品化 | 控制台后续迭代 | provider 编辑/删除、预设模板、历史会话、图表等不影响验收 |
| axm 全仓元数据迁移 | axm 元数据迁移独立任务 | 现有旧 `status` 元数据不是 GP-02 代码闭合 blocker |

## GP-03 可依赖的最小稳定面

GP-02-01 完成后，GP-03 只依赖以下能力：

- `POST /api/v1/admin/llm/generate` 与 `POST /api/v1/admin/llm/stream` 可通过 edge-api 调用。
- 请求体稳定支持 `model_ref`、`messages`、`tools`、`tool_choice`、`response_format`、采样参数和 `idempotency_key`。
- Generate 返回 assistant message、tool calls、usage、finish_reason、request_id。
- Stream 输出 `reasoning_delta`、`content_delta`、`tool_call_delta`、`message_completed`、`usage`、`done`、`error`，edge proxy 不改事件语义。
- `llm_request_logs` 记录可信 `caller/user_id/tenant_id/model_ref/provider_slug/stream/status/usage/idempotency_key/error`。
- API key、Authorization、JWT、token、password、secret 不出现在 DTO、日志、错误、request log metadata 或 SSE error payload 中。
- Provider/model 可以通过 Web 或 API 创建、启停、测试，且本地 fake upstream 可复现成功和错误路径。

## Web 调试平台目标

本阶段的 Web 平台是“调试台”，不是完整后台。验收路径必须完整：

```text
登录 admin
  -> 打开 /admin/llm
  -> 创建或查看 provider
  -> 创建或查看 model
  -> 点击 provider test
  -> 打开 /admin/chat-debug
  -> 选择 enabled model_ref
  -> 发送普通消息
  -> 看到 SSE 内容、usage、done
  -> 发送非法配置或停止流
  -> 看到可理解错误或停止状态
```

### Web 必须能力

| 页面 / 模块 | 必须能力 |
|---|---|
| `web/src/pages/admin/ModelProvidersPage.tsx` | 列 provider；创建/编辑/删除 provider；更新 key；启停 provider；列 model；创建/编辑/删除 model；启停 model；通过 Generate ping 默认模型/指定模型并展示脱敏结果 |
| `web/src/pages/admin/ChatDebugPage.tsx` | 从 `/api/v1/admin/llm/models` 读取 enabled models；选择真实 `model_ref`；展示 content/reasoning/usage；展示 SSE error；停止流后 UI 回到可发送状态 |
| `web/src/lib/api.ts` | 增加 provider/model list/create/update/delete/enable API wrapper；Generate parser 兼容 assistant/message/content 响应形态；Stream parser 保留 usage 并兼容 error/done |
| `web/tests/e2e/chat-debug-usage.spec.ts` | 覆盖登录、模型列表、SSE content、usage、done 展示 |
| 新增或扩展 E2E | 覆盖 provider/model 调试入口：Web 测试按钮走 Generate ping 成功/失败、model select 来源于 models 而非 provider `default_model_ref` |

### 本地 fake upstream

为了人类不依赖真实付费 key 也能测试，需要提供一个最小 OpenAI-compatible fake upstream：

| 文件 | 责任 |
|---|---|
| `scripts/dev/fake-openai-compatible.mjs` | 监听本地端口，模拟 `POST /v1/chat/completions` 的非流式和流式 OpenAI-compatible 响应 |
| `scripts/e2e-llm-sse.sh` | 支持通过环境变量指向 fake provider/model；校验 content_delta、usage、done、非法 model error |

fake upstream 的最小行为：

- 要求 `Authorization: Bearer fake-key`；错误 key 返回 401 JSON。
- 非流式返回 assistant content、finish_reason、usage。
- `stream: true` 时按 OpenAI-compatible SSE 输出 content delta、usage chunk 和 `[DONE]`。
- 支持一个固定 tool-call 测试模型或参数，用于后续手动验证 tool calls；不需要实现真实工具执行。

## 文件结构

| 路径 | 操作 | 责任 |
|---|---|---|
| `services/llm/biz/provider.go` | Modify | Provider test 从配置可读升级为最小 Generate 连通性检查 |
| `services/llm/biz/provider_test.go` | Modify | 覆盖 provider test 成功、401/403、404/path、5xx、脱敏错误 |
| `services/llm/biz/generate.go` | Modify | 从 context/header 读取可信 metadata，修正幂等查询维度，记录 request log |
| `services/llm/handler/generate.go` | Modify | 绑定 metadata/idempotency，错误与 SSE payload 脱敏 |
| `services/llm/handler/provider.go` | Modify | provider test 返回脱敏摘要 |
| `services/llm/security/redact.go` | Modify if needed | 补齐字符串、JSON、结构体或错误摘要脱敏入口 |
| `services/edge-api/handler/llm_proxy.go` | Modify | 透传可信用户/租户/caller metadata 到 llm；保留 SSE pipe |
| `services/edge-api/handler/llm_proxy_test.go` | Modify | 覆盖 metadata/header 透传与 SSE 不改写 |
| `web/src/lib/api.ts` | Modify | 增加 model/provider test API 与 stream parser 验证 |
| `web/src/pages/admin/ModelProvidersPage.tsx` | Modify | 补齐 model 管理与 provider test 调试入口 |
| `web/src/pages/admin/ChatDebugPage.tsx` | Modify | 使用 model 列表选择，保留 usage/error/stop 展示 |
| `web/tests/e2e/chat-debug-usage.spec.ts` | Modify | 更新为 models 来源，覆盖 usage/done |
| `web/tests/e2e/llm-debug-setup.spec.ts` | Create | 覆盖 provider/model/test 调试路径 |
| `scripts/dev/fake-openai-compatible.mjs` | Create | 本地 fake OpenAI-compatible upstream |
| `scripts/e2e-llm-sse.sh` | Modify | 支持 fake upstream/manual env 并检查 usage/done/error |
| `.axm/progress/generation-platform/specs/gp-02-llm-service-rebuild.md` | Modify after implementation | 记录 GP-02-01 验收结果或指向本 spec |
| `.axm/progress/generation-platform/roadmap.md` | Modify after implementation | GP-02-01 完成后更新 GP-02/GP-03 状态 |

## 任务拆解

### Task 1: Provider test 真实最小 Generate

**Files:**
- Modify: `services/llm/biz/provider.go`
- Modify: `services/llm/biz/provider_test.go`
- Create: `scripts/dev/fake-openai-compatible.mjs`

- [x] 将 `ProviderBiz.Test` 改为对 provider 的 `base_url + /v1/chat/completions` 发最小非流式 Generate 请求。
- [x] test 请求使用 provider 的 `default_model_ref` 解析出的上游 model；如果无法解析，返回可理解的脱敏失败摘要。
- [x] 覆盖 fake upstream 成功、401/403、404/path 错误、5xx、超时、无 usage 返回。
- [x] 失败 message 不包含 API key、Authorization header、完整 request body 或上游 secret 字段。
- [x] `scripts/dev/fake-openai-compatible.mjs` 可用 `node scripts/dev/fake-openai-compatible.mjs --port 39090 --key fake-key` 启动。
- [x] 运行 `cd services/llm && go test ./biz -run TestProvider -count=1`，预期 PASS。

### Task 2: 身份、租户、caller 与幂等闭合

**Files:**
- Modify: `services/edge-api/handler/llm_proxy.go`
- Modify: `services/edge-api/handler/llm_proxy_test.go`
- Modify: `services/llm/biz/generate.go`
- Modify: `services/llm/handler/generate.go`
- Modify: `services/llm/dal/mongo/request_log.go`
- Modify: `services/llm/dal/model/request_log.go` if needed

- [x] edge-api 从登录态读取 userID，并以内部 header 或 metadata 透传到 llm；客户端传入的同名 header 不可信。
- [x] llm handler 将可信 `caller/user_id/tenant_id` 写入 `GenerateReq` 或 request context。
- [x] `FindSuccessful` 使用 `user_id + model_ref + idempotency_key`，禁止继续用空 userID 查询成功记录。
- [x] 非流式幂等命中返回原成功摘要，不重复调用 upstream；不同 userID 的相同 idempotency_key 不互相命中。
- [x] request log 成功和失败都记录 `caller/user_id/tenant_id/model_ref/provider_slug/stream/status/idempotency_key`。
- [x] 运行 `cd services/llm && go test ./biz ./handler -run 'TestGenerate|TestStream|TestIdempotency|TestRequestLog' -count=1`，预期 PASS。
- [x] 运行 `cd services/edge-api && go test ./handler -run TestLLMProxy -count=1`，预期 PASS。

### Task 3: 脱敏调用面闭合

**Files:**
- Modify: `services/llm/security/redact.go`
- Modify: `services/llm/security/redact_test.go`
- Modify: `services/llm/biz/provider.go`
- Modify: `services/llm/biz/generate.go`
- Modify: `services/llm/handler/provider.go`
- Modify: `services/llm/handler/generate.go`
- Modify: `services/edge-api/handler/llm_proxy.go`

- [x] 日志输出上游错误前调用 `security.Redact` 或 `RedactJSONBytes`。
- [x] provider test、Generate、Stream error event 的 message 只保留脱敏摘要。
- [x] request log metadata/error_message 不保存明文 key、Authorization、JWT、token、password、secret。
- [x] edge proxy 记录 upstream error 时不输出 Authorization、请求体或 provider key。
- [x] 测试覆盖 nested JSON、malformed JSON、Go struct、plain error string 中的敏感字段。
- [x] 运行 `cd services/llm && go test ./security ./biz ./handler -run 'TestRedact|TestProvider|TestGenerate|TestStream' -count=1`，预期 PASS。

### Task 4: Web 调试平台可用

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/pages/admin/ModelProvidersPage.tsx`
- Modify: `web/src/pages/admin/ChatDebugPage.tsx`
- Create: `web/tests/e2e/llm-debug-setup.spec.ts`
- Modify: `web/tests/e2e/chat-debug-usage.spec.ts`

- [x] `api.ts` 增加 `modelListModels`、`modelCreateModel`、`modelSetModelEnabled`、`modelTestProvider`。
- [x] `/admin/llm` 页面在 provider 表下方或旁侧展示 model 列表，支持创建 model、启停 model。
- [x] `/admin/llm` 页面提供 provider test 按钮，展示 `ok/message`，message 必须来自脱敏结果。
- [x] `/admin/chat-debug` 从 model list 选择 enabled `model_ref`，不再只从 provider `default_model_ref` 猜模型。
- [x] Chat 调试页继续展示 reasoning/content/usage/error，停止流后输入框和发送按钮恢复。
- [x] Playwright mock 覆盖 provider/model/test 成功路径、provider test 失败摘要、chat usage/done 展示。
- [x] 运行 `cd web && npm run build`，预期 PASS。
- [x] 运行 `cd web && npm run e2e -- chat-debug-usage.spec.ts llm-debug-setup.spec.ts`，预期 PASS。

### Task 5: edge -> llm smoke 与脚本化人类验收

**Files:**
- Modify: `scripts/e2e-llm-sse.sh`
- Modify: `scripts/e2e-all.sh` if needed
- Modify: `scripts/dev/self_check.sh` if needed

- [x] `scripts/e2e-llm-sse.sh` 记录并校验 `content_delta`、`usage`、`done`、非法 model error。
- [x] 脚本支持 `MODEL_REF`、`EDGE_API`、`STREAM_ENDPOINT`、fake upstream 相关环境变量。
- [x] `make dev-status` 必须能看到 `llm` 与 `web`；不得出现旧 `model` 服务。
- [x] 使用 fake upstream 完成一次 edge admin generate/stream smoke，并记录输出摘要；本轮本地 `edge-api :38080` 未启动，`bash scripts/e2e-llm-sse.sh` preflight 返回 HTTP 000，未伪造通过。
- [x] 运行 `bash scripts/dev/self_check.sh`，预期 PASS。
- [x] 在全栈已启动时运行 `bash scripts/e2e-llm-sse.sh`，预期 PASS；本轮本地 infra 未启动，已记录为环境阻塞。

### Task 6: 最终质量门禁与文档闭合

**Files:**
- Modify: `.axm/progress/generation-platform/specs/gp-02-llm-service-rebuild.md`
- Modify: `.axm/progress/generation-platform/specs/gp-02-01-llm-service-closure.md`
- Modify: `.axm/progress/generation-platform/roadmap.md`
- Modify: `.axm/knowledge/services/overview.md` if implementation changes long-term service facts
- Modify: `.axm/project/architecture.md` if service boundary facts change

- [x] 运行 `make fmt`。
- [x] 运行 `make test-services`。
- [x] 运行 `make lint`.
- [x] 运行 `make build`.
- [x] 运行 `rg "services/model|/api/v1/model|admin/models|MODEL_CONFIG|MODEL_ENCRYPT_KEY"`，预期只剩历史 progress 或明确 deprecated 文档引用。
- [x] 运行 `node /Users/castlexu/.codex/skills/axm/scripts/reindex.mjs --target=/Users/castlexu/github/micro-service --dry-run`，确认索引没有新增孤儿。
- [x] 运行 `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service`；若仍只失败于既有 `status` 元数据迁移，记录为 deferred，不阻塞 GP-03。
- [x] GP-02-01 全部 must 项通过后，把本 spec `workflow-state` 改为 `verified`，并在 GP-02 主 spec 记录收口验收摘要。

## AI 自动验收

| 验收项 | 命令 / 检查 | 预期 |
|---|---|---|
| Provider test | `cd services/llm && go test ./biz -run TestProvider -count=1` | PASS，覆盖成功和上游错误 |
| LLM 业务/handler | `cd services/llm && go test ./biz ./handler ./security -count=1` | PASS |
| Component | `cd services/llm && go test ./component -count=1` | PASS |
| edge proxy | `cd services/edge-api && go test ./handler -run TestLLMProxy -count=1` | PASS |
| Web build | `cd web && npm run build` | PASS |
| Web E2E | `cd web && npm run e2e -- chat-debug-usage.spec.ts llm-debug-setup.spec.ts` | PASS |
| dev 脚本自检 | `bash scripts/dev/self_check.sh` | PASS |
| 服务测试 | `make test-services` | PASS |
| lint | `make lint` | PASS |
| 构建 | `make build` | PASS，构建 `llm`，不构建旧 `model` |
| 旧引用扫描 | `rg "services/model|/api/v1/model|admin/models|MODEL_CONFIG|MODEL_ENCRYPT_KEY"` | 无运行时代码引用 |
| axm 索引 | `node /Users/castlexu/.codex/skills/axm/scripts/reindex.mjs --target=/Users/castlexu/github/micro-service --dry-run` | 无需落盘或仅显示预期变更 |

## 人类验收

本地验收优先使用 fake upstream，避免真实 provider key 影响结论：

1. 启动 fake upstream：`node scripts/dev/fake-openai-compatible.mjs --port 39090 --key fake-key`。
2. 启动全栈：`make dev-start`。
3. 打开 `http://localhost:35173/admin/llm`，登录 `admin@platform.com / Admin@1234`。
4. 创建 provider：`vendor=openai_compatible`，`base_url=http://127.0.0.1:39090/v1`，`api_key=fake-key`。
5. 创建 enabled model：`provider_slug=<provider>`，`model=fake-chat`，capabilities 至少包含 `chat`、`stream`、`tool_calling`。
6. 点击 provider test，看到成功摘要；把 key 改错后再测，看到脱敏失败摘要。
7. 打开 `http://localhost:35173/admin/chat-debug`，选择刚创建的 `model_ref`。
8. 发送普通消息，看到流式内容、`usage` 和完成状态。
9. 发送非法 model 或关闭 fake upstream，看到 Web 错误提示，且日志/SSE payload 不泄漏 key。
10. 点击停止，流中断后输入区恢复可发送状态。

## 完成判定

GP-02-01 视为完成需要同时满足：

- 必须完成范围全部通过 AI 自动验收或有明确、可复现的环境阻塞记录。
- 人类能用 Web 调试平台完成 fake upstream 的 provider/model/test/stream/usage/error 路径。
- GP-03 依赖的 Generate/Stream/tool-calling/usage/metadata 契约没有未决破坏性变更。
- Deferred 表中的项目没有被偷偷塞进本阶段；如必须提升，需要先更新本 spec。

## 实施与验收记录（2026-05-17）

### 已落地

- Provider test 已升级为真实最小 OpenAI-compatible Generate，请求使用脱密后的 Bearer key、`default_model_ref` 的上游 model，并兼容 `base_url` 已含 `/v1` 的本地 fake upstream 配置。
- `scripts/dev/fake-openai-compatible.mjs` 已新增，可模拟非流式、流式 usage、错误 key、基础 tool-call chunk。
- edge-api 已覆盖客户端伪造的 `X-Caller` / `X-User-ID` / `X-Tenant-ID`，改为透传可信登录态 metadata。
- llm Generate/Stream handler 已把可信 metadata 绑定进 `GenerateReq` 和 logger context；幂等查询改为 `user_id + model_ref + idempotency_key`。
- Generate/Stream 上游错误、SSE error payload、provider test message 与 request log error_message 已接入脱敏摘要。
- Web `/admin/llm` 已支持 provider 列表/创建/编辑/删除/key 更新/启停、model 列表/创建/编辑/删除/启停；页面上的 provider/model 测试按钮统一走 `/api/v1/admin/llm/generate` ping 默认模型或指定模型，避免与 Chat Debug 的真实 Generate 链路不一致。后端 `/providers/{id}/test` API 仍作为直接 provider 诊断入口保留。
- `scripts/e2e-llm-sse.sh` 已校验 `content_delta`、`usage`、`done`、非法 model 错误路径，并在本地 infra 未启动时给出明确 preflight 失败原因。

### AI 自动验收结果

| 项目 | 结果 |
|---|---|
| `GOCACHE=/private/tmp/go-build-cache go test ./biz ./handler ./security -run 'TestProvider\|TestGenerate\|TestStream\|TestIdempotency\|TestRequestLog\|TestRedact' -count=1`（`services/llm`） | PASS；sandbox 内因 `httptest` 监听被拒，升级权限后通过 |
| `GOCACHE=/private/tmp/go-build-cache go test ./component -count=1`（`services/llm`） | PASS |
| `GOCACHE=/private/tmp/go-build-cache go test ./handler -run TestLLMProxy -count=1`（`services/edge-api`） | PASS |
| `cd web && npm run build` | PASS |
| `cd web && npm run lint` | PASS |
| `cd web && npm run e2e -- chat-debug-usage.spec.ts llm-debug-setup.spec.ts` | PASS；sandbox 内因 Vite 监听被拒，升级权限后 2/2 通过 |
| `node --check scripts/dev/fake-openai-compatible.mjs` | PASS |
| `bash -n scripts/e2e-llm-sse.sh` | PASS |
| `bash scripts/dev/self_check.sh` | PASS |
| `make dev-status` | PASS；输出包含 `llm` 与 `web`，不包含旧 `model` 服务 |
| `GOCACHE=/private/tmp/go-build-cache make test-services` | PASS；sandbox 内本地 listener 被拒，升级权限后通过 |
| `GOCACHE=/private/tmp/go-build-cache make lint` | PASS；`golangci-lint` 未安装，按 Makefile 跳过 |
| `GOCACHE=/private/tmp/go-build-cache make build` | PASS |
| `rg "services/model\|/api/v1/model\|admin/models\|MODEL_CONFIG\|MODEL_ENCRYPT_KEY"` | PASS；无匹配 |
| `node /Users/castlexu/.codex/skills/axm/scripts/reindex.mjs --target=/Users/castlexu/github/micro-service --dry-run` | PASS；changed=0 |
| `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service` | FAIL：278 errors，均为既有 axm 元数据迁移问题（88 个旧 `status` / 缺 `doc-state`，51 个旧 progress 缺 `workflow-state/state-updated`）；GP-02-01 与 GP-02 文档已使用新 `doc-state`/`workflow-state` |
| `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service`（2026-05-17 axm 迁移后复跑） | PASS；0 errors, 0 warnings |
| `bash scripts/e2e-llm-sse.sh` | ENV-BLOCKED；本地 `http://localhost:38080/healthz` 不可达，脚本明确提示 `make dev-start` / `EDGE_API` |

### 人类验收状态

- Web mock E2E 已覆盖 provider/model 测试按钮通过 Generate ping 成功与失败摘要、Chat Debug usage/done 展示。
- 真实全栈 fake upstream 人类路径尚未在本轮执行，原因是本地 dev 服务均未处于 ready 状态；后续人类可按本文“人类验收”步骤启动 fake upstream 与 `make dev-start` 后复验。
- Deferred 范围保持不变：Redis 限流、真实多 provider conformance、详细 token 计费、credits/billing、模型路由/fallback 与控制台高级体验未塞入本阶段。
