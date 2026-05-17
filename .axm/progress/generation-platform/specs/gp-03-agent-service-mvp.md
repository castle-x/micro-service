<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: generation-platform
workflow-state: ready
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - ../decisions.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
-->

# GP-03 Agent 服务 MVP Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建 `agent` Kitex 服务，提供 Agent profile 管理、基于 Eino ADK `ChatModelAgent`/`Runner` 的 ReAct 执行、tool 白名单、运行事件持久化，以及由 `edge-api` 暴露的 Agent SSE 事件流。

**Architecture:** `agent` 是独立业务服务，持有 Agent 配置、运行状态、事件日志、tool catalog 和 tool 执行审计；它通过 `llm` 服务取得 Eino `ToolCallingChatModel` 能力，通过 RPC/MQ 形式调用被允许的业务 tool，不直接访问其他服务主存储。SSE 属于 HTTP 门面，由 `edge-api` 使用普通 Kitex RPC 拉取 `agent` 事件并转成 `text/event-stream`，避免在服务间引入 HTTP 反向依赖。

**Tech Stack:** Go 1.25.6、Kitex、MongoDB、Redis、Eino ADK、OpenTelemetry、Hertz edge-api、Thrift IDL。

---

## 背景

`generation-platform` 已确认顶层调用链为：

```text
workflow -> agent -> llm
                  -> tool -> generator
                  -> tool -> asset
```

GP-03 位于 `llm` 服务重建之后、`workflow` 服务 MVP 之前。它需要先给后续 workflow 提供一个稳定的“可运行 Agent”能力：workflow 不执行 ReAct 循环，只提交任务、等待事件、处理人工确认；Agent 内部负责模型调用、工具循环、事件输出和工具权限。

本阶段不考虑旧 `services/model` API 兼容。若旧模型网关与新 `llm`/`agent` 边界冲突，优先按本 spec 与 `generation-platform/decisions.md` 重建。

## 目标

- 新增 `idl/agent/agent.thrift`，定义 Agent profile、tool spec、run、event、interrupt/resume、事件拉取等 RPC 契约。
- 新增 `services/agent/` Kitex 服务，并纳入 `go.work`、`Makefile`、本地 dev 配置、etcd 注册发现和 admin health。
- 支持系统默认 Agent 与用户自定义 Agent profile；第一版允许配置 `instruction`、`model_ref`、`tool_whitelist`、`max_iterations`、采样参数和可选 `output_schema`。
- 使用 Eino ADK `ChatModelAgent` + `Runner` 执行 ReAct 工具循环，生产环境只通过 `Runner` 运行，不直接调用 `agent.Run()`。
- 通过服务内 tool catalog 和 per-profile/per-run 白名单控制工具可见性与执行权限；工具执行必须有审计记录。
- 持久化 Agent run、event log、tool execution log；提供普通 RPC 的事件拉取接口，支持 edge-api SSE 门面。
- 支持 interrupt/resume MVP：需要用户确认或补充信息时，run 进入 `WAITING_USER`，用户 resume 后继续执行。
- 接入项目统一日志、错误码、OpenTelemetry trace/metrics、敏感字段脱敏。

## 非目标

- 不实现 `workflow` 服务、工作流模板、步骤状态机或步骤产物绑定；GP-04 负责。
- 不实现 `generator` 生图服务；GP-05 负责。GP-03 只保留 generator tool 接口位，默认不启用。
- 不建设 Knowledge/RAG、长期记忆、向量库或多 Agent DeepAgent 编排。
- 不开放任意用户代码、shell、filesystem middleware、MCP 任意工具或浏览器工具。
- 不兼容旧 `services/model` 的 `/api/v1/model/chat` 或 `/api/v1/model/chat/stream`。
- 不在 agent 服务中保存 provider API key；模型凭据只归属 GP-02 `llm` 服务。

## 已确认开发细节

### 服务边界

| 模块 | 职责 | 约束 |
|---|---|---|
| `idl/agent/agent.thrift` | 跨服务 RPC 契约 | include `base.thrift`；枚举保留 `UNKNOWN = 0`；请求含 `base.BaseReq` |
| `services/agent` | Agent profile、run、event、tool catalog、Runner 执行 | Kitex 服务；不能 import 其他 `services/*` 内部包，只能用生成 client |
| `services/edge-api` | Agent HTTP API 与 SSE 门面 | 不直接访问 Agent Mongo/Redis；只调用 Agent RPC |
| `services/llm` | ToolCallingChatModel 能力 | GP-03 只依赖契约；测试使用 fake model |
| `services/asset` | 资产读写 tool 的后端 | Agent tool 通过 asset RPC 调用，不直接访问 asset 主存储 |
| `services/generator` | 生图 tool 的后端 | GP-05 前不启用真实 generator tool |

### IDL 契约

新增 `idl/agent/agent.thrift`，核心枚举与 DTO：

```thrift
enum AgentProfileScope {
    UNKNOWN = 0
    SYSTEM  = 1
    USER    = 2
}

enum AgentProfileStatus {
    UNKNOWN  = 0
    ENABLED  = 1
    DISABLED = 2
}

enum AgentRunStatus {
    UNKNOWN      = 0
    QUEUED       = 1
    RUNNING      = 2
    WAITING_USER = 3
    COMPLETED    = 4
    FAILED       = 5
    CANCELLED    = 6
}

enum AgentEventType {
    UNKNOWN             = 0
    RUN_STARTED         = 1
    ASSISTANT_DELTA     = 2
    ASSISTANT_MESSAGE   = 3
    REASONING_DELTA     = 4
    TOOL_CALL_STARTED   = 5
    TOOL_CALL_COMPLETED = 6
    INTERRUPTED         = 7
    RUN_COMPLETED       = 8
    RUN_FAILED          = 9
    RUN_CANCELLED       = 10
}
```

第一版 RPC：

| RPC | 用途 |
|---|---|
| `Health` | Kitex 探活 |
| `CreateAgentProfile` | 创建用户自定义 Agent |
| `UpdateAgentProfile` | 更新用户自定义 Agent；系统默认 Agent 只允许管理员更新 |
| `GetAgentProfile` | 查询 profile 详情 |
| `ListAgentProfiles` | 分页列出系统默认 + 当前用户 profile |
| `ListAgentTools` | 返回当前用户可见的 tool catalog |
| `StartAgentRun` | 创建 run，异步启动 Runner，返回 `RunID` |
| `AppendAgentRunMessage` | 给已有 run 追加用户消息，适用于多轮直接对话 |
| `ResumeAgentRun` | 对 `WAITING_USER` run 提交用户输入或审批结果 |
| `CancelAgentRun` | 请求取消仍在执行的 run |
| `GetAgentRun` | 查询 run 状态与 usage 摘要 |
| `PullAgentRunEvents` | 按 `after_seq` 拉取事件；支持 `limit` 与 `wait_ms` 长轮询 |

`PullAgentRunEvents` 是 edge-api SSE 的唯一事件来源。MVP 不使用 Kitex streaming；这样可以保持服务间协议简单，并让所有外部流式输出都收敛到 `edge-api`。

### Mongo 数据模型

`agent_profiles`：

| 字段 | 说明 |
|---|---|
| `_id` | AgentProfileID |
| `workspace_id` | 第一版可用 userID 作为默认 workspace；后续 IAM workspace 落地后迁移 |
| `owner_user_id` | 系统 profile 为空，用户 profile 为创建人 |
| `scope` | `SYSTEM` / `USER` |
| `name` / `description` | 展示与选择 |
| `instruction` | 系统提示词，禁止为空 |
| `model_ref` | 指向 `llm` 模型配置，例如 provider/model slug |
| `tool_whitelist` | `ToolBinding[]`，包含 name、enabled、requires_approval、config JSON |
| `run_config` | max_iterations、temperature、max_tokens、top_p |
| `output_schema` | 可选 JSON Schema 字符串 |
| `status` | enabled / disabled |
| `created_by` / `created_at` / `updated_at` | 审计字段 |

`agent_runs`：

| 字段 | 说明 |
|---|---|
| `_id` | RunID |
| `profile_id` / `profile_snapshot` | 运行时使用 profile 快照，避免后续修改影响历史 run |
| `workspace_id` / `user_id` | 权限隔离 |
| `source` | `direct` / `workflow`，GP-03 默认 `direct` |
| `external_ref` | workflowRunID、stepRunID 等后续引用 |
| `status` | queued/running/waiting_user/completed/failed/cancelled |
| `checkpoint_id` | Eino interrupt/resume checkpoint key |
| `last_event_seq` | 当前最大事件序号 |
| `usage` | prompt/completion/total tokens、tool_call_count、duration_ms |
| `idempotency_key` | 防重复提交 |
| `created_at` / `updated_at` / `completed_at` | 生命周期时间 |

`agent_events`：

| 字段 | 说明 |
|---|---|
| `_id` | EventID |
| `run_id` | 所属 run |
| `seq` | run 内严格递增，从 1 开始 |
| `type` | AgentEventType |
| `agent_name` | Eino event 的 AgentName |
| `role` | assistant/tool/system/user，可为空 |
| `tool_name` | tool event 时填写 |
| `content` | 已脱敏文本；delta event 可为空 |
| `payload_json` | 结构化 payload，禁止保存明文 secret/token |
| `created_at` | 事件时间 |

`agent_tool_executions`：

| 字段 | 说明 |
|---|---|
| `_id` | ToolExecutionID |
| `run_id` / `event_seq` | 对应 run 与 tool event |
| `tool_name` | catalog 中的工具名 |
| `args_hash` | 参数哈希；原始参数仅在脱敏后保存 |
| `status` | running/completed/failed/denied |
| `result_ref` | 大结果保存位置或截断引用 |
| `error_code` / `error_message` | 失败原因 |
| `duration_ms` | 执行耗时 |

### Tool catalog 与白名单

MVP 内置 tool catalog 放在 `services/agent/tool/`，不做用户动态上传工具。

| Tool | MVP 状态 | 权限 |
|---|---|---|
| `asset.get_asset` | 启用 | 只能读取当前用户可访问资产 |
| `asset.get_current_version` | 启用 | 只能读取当前用户可访问资产版本 |
| `asset.create_version` | 启用但默认 `requires_approval=true` | 通过 asset RPC 写版本，必须产生 tool execution 审计 |
| `generator.create_image_job` | 注册但默认禁用 | GP-05 接入后启用 |
| `ask_user` | 启用 | 触发 Eino interrupt，用于澄清或确认 |

工具执行规则：

- Runner 构建前按 profile/run 白名单过滤工具；模型看不到未授权工具。
- 服务端在 tool 执行入口再次校验白名单，防止模型伪造 tool name。
- `requires_approval=true` 的工具包装为 Eino approvable/interrupt tool；审批通过后才能执行。
- tool 结果超过阈值时只在 event 中写摘要，完整结果写入 tool execution 记录或后续对象存储引用。
- tool args/result 写日志与事件前必须做敏感字段扫描，字段名包含 `password`、`secret`、`token`、`authorization` 时一律替换为 `[REDACTED]`。

### Eino Runner 方案

- 默认使用 `adk.NewChatModelAgent`，不使用 DeepAgent。
- `ChatModelAgentConfig.Model` 使用 agent 服务内的 `llmclient.ToolCallingChatModel` 适配器；该适配器调用 GP-02 `llm` 服务，不持有 provider key。
- `MaxIterations` 来自 profile，默认 10，上限 20；超过上限时 run 标记 `FAILED`，错误码为 `ErrAgentMaxIterationsExceeded`。
- `RunnerConfig.EnableStreaming=true`，服务从 Eino `AsyncIterator[*AgentEvent]` 读取事件并转换成 `agent_events`。
- `RunnerConfig.CheckPointStore` 使用 Redis 实现；本地单测使用 in-memory store。
- middleware 顺序固定为：PatchToolCalls、tool result reduction、自定义 trace/audit middleware。MVP 不启用 filesystem、shell、tool search、skill middleware。
- 服务启动时发现 `RUNNING`/`QUEUED` 的历史 run，应标记为 `FAILED` 并写入 `RUN_FAILED` 事件；`WAITING_USER` run 保持可 resume。

### edge-api HTTP/SSE 契约

新增登录态路由：

| Method | Path | 用途 |
|---|---|---|
| `POST` | `/api/v1/agents/profiles` | 创建用户 Agent |
| `GET` | `/api/v1/agents/profiles` | 列出系统 + 用户 Agent |
| `GET` | `/api/v1/agents/profiles/:id` | 查询 Agent |
| `PUT` | `/api/v1/agents/profiles/:id` | 更新 Agent |
| `GET` | `/api/v1/agents/tools` | 查看可用工具 |
| `POST` | `/api/v1/agents/runs` | 启动直接 Agent run |
| `POST` | `/api/v1/agents/runs/:id/messages` | 追加用户消息 |
| `POST` | `/api/v1/agents/runs/:id/resume` | 提交审批/补充信息 |
| `POST` | `/api/v1/agents/runs/:id/cancel` | 取消 run |
| `GET` | `/api/v1/agents/runs/:id` | 查询 run |
| `GET` | `/api/v1/agents/runs/:id/events` | SSE 事件流 |

SSE event payload：

```json
{
  "run_id": "string",
  "seq": 12,
  "type": "assistant_delta",
  "agent_name": "CharacterFaceAgent",
  "role": "assistant",
  "tool_name": "",
  "content": "text delta",
  "payload": {},
  "created_at": 1779014400000
}
```

SSE 断线重连：

- 客户端可用 query `?after_seq=<n>` 或 `Last-Event-ID` 继续。
- edge-api 循环调用 `PullAgentRunEvents(wait_ms=25000, limit=100)`。
- `RUN_COMPLETED`、`RUN_FAILED`、`RUN_CANCELLED` 后 edge-api 发送最终事件并关闭 SSE。

### 错误码

在 `pkg/errno` 新增 Agent 区段 `18001 - 18999`：

| 错误 | Code | 场景 |
|---|---:|---|
| `ErrAgentProfileNotFound` | 18001 | profile 不存在或无权限 |
| `ErrAgentProfileDisabled` | 18002 | profile 禁用 |
| `ErrAgentRunNotFound` | 18003 | run 不存在或无权限 |
| `ErrAgentRunStateConflict` | 18004 | 状态不允许当前操作 |
| `ErrAgentToolNotAllowed` | 18005 | tool 不在白名单 |
| `ErrAgentToolExecutionFailed` | 18006 | tool 执行失败 |
| `ErrAgentCheckpointNotFound` | 18007 | resume checkpoint 缺失 |
| `ErrAgentMaxIterationsExceeded` | 18008 | ReAct 循环超过上限 |

## 文件结构

| 路径 | 操作 | 责任 |
|---|---|---|
| `idl/agent/agent.thrift` | Create | Agent RPC 契约 |
| `go.work` | Modify | 加入 `./services/agent` |
| `Makefile` | Modify | `SERVICES` / `ALL_SERVICES` / `gen` 纳入 agent |
| `deployments/config/agent.yaml` | Create | agent 非敏感配置 |
| `deployments/env/agent.env.example` | Create | agent 本地 env 示例 |
| `scripts/dev/services.json` | Modify | agent 端口 `38085`、admin `48085`，依赖 `iam`、`asset`、`llm` |
| `services/edge-api/main.go` | Modify | 初始化 agent Kitex client |
| `services/edge-api/router.go` | Modify | 注册 `/api/v1/agents` 路由 |
| `services/edge-api/handler/agent.go` | Create | Agent REST/SSE adapter |
| `services/agent/go.mod` | Create | agent module |
| `services/agent/main.go` | Create | 配置、Mongo/Redis、OTel、Kitex、health、registry |
| `services/agent/handler.go` | Create | Kitex handler |
| `services/agent/biz/profile.go` | Create | profile 校验与 CRUD |
| `services/agent/biz/run.go` | Create | run 状态机与提交 |
| `services/agent/biz/event.go` | Create | event append/pull/long-poll |
| `services/agent/runner/runner.go` | Create | Eino Runner 构建与事件转换 |
| `services/agent/llmclient/chat_model.go` | Create | Eino ToolCallingChatModel -> llm RPC 适配 |
| `services/agent/tool/catalog.go` | Create | 内置 tool catalog 与白名单过滤 |
| `services/agent/tool/asset.go` | Create | asset RPC tools |
| `services/agent/tool/ask_user.go` | Create | interrupt/resume 工具 |
| `services/agent/dal/model/*.go` | Create | Mongo 文档模型 |
| `services/agent/dal/mongo/*.go` | Create | repo、索引、分页、事件游标 |
| `pkg/errno/code.go` | Modify | Agent 错误码 |
| `pkg/errno/errno_test.go` | Modify | 错误码区段测试 |

## 任务拆解

### Task 1: IDL、错误码与生成链路

**Files:**
- Create: `idl/agent/agent.thrift`
- Modify: `Makefile`
- Modify: `go.work`
- Modify: `pkg/errno/code.go`
- Modify: `pkg/errno/errno_test.go`

- [ ] 写 `agent.thrift`，包含 profile/tool/run/event DTO 与 11 个 RPC。
- [ ] 新增 Agent 错误码区段 `18001 - 18999`。
- [ ] 更新 `Makefile` 的 `SERVICES`，让 `make gen` 生成 `services/agent/kitex_gen`，并生成 `edge-api` 的 agent client。
- [ ] 运行 `make gen`。
- [ ] 运行 `cd pkg && go test ./errno -count=1`，预期 PASS。

### Task 2: agent 服务骨架与本地 dev 接入

**Files:**
- Create: `services/agent/go.mod`
- Create: `services/agent/main.go`
- Create: `services/agent/handler.go`
- Create: `deployments/config/agent.yaml`
- Create: `deployments/env/agent.env.example`
- Modify: `scripts/dev/services.json`

- [ ] 复制 asset 服务的 Kitex 初始化模式，但服务名、端口、配置改为 `agent`。
- [ ] 初始化 Mongo、Redis、OTel、Kitex registry、admin health。
- [ ] `scripts/dev/services.json` 中 agent 使用 `38085` / `48085`，依赖 `iam`、`asset`、`llm`；如果 GP-02 尚未落地，执行时先使用 fake llm 配置。
- [ ] 运行 `cd services/agent && go test ./... -count=1`，预期 PASS。

### Task 3: Profile CRUD 与默认 Agent seed

**Files:**
- Create: `services/agent/dal/model/profile.go`
- Create: `services/agent/dal/mongo/profile.go`
- Create: `services/agent/biz/profile.go`
- Modify: `services/agent/handler.go`
- Test: `services/agent/biz/profile_test.go`
- Test: `services/agent/handler_test.go`

- [ ] 为 `agent_profiles` 建唯一索引：`workspace_id + owner_user_id + name`、`scope + name`。
- [ ] 实现 profile 校验：instruction 非空、max_iterations 范围 1-20、tool_whitelist 中 tool 必须存在。
- [ ] 服务启动时 seed 三个系统 Agent：`BackgroundAgent`、`StructExtractorAgent`、`ImagePromptAgent`；重复启动保持幂等。
- [ ] Handler 对 nil request、无 userID、无权限、禁用 profile 返回统一 BaseResp 错误。
- [ ] 运行 `cd services/agent && go test ./biz -run TestProfile -count=1`，预期 PASS。

### Task 4: Tool catalog、白名单与审计

**Files:**
- Create: `services/agent/tool/catalog.go`
- Create: `services/agent/tool/asset.go`
- Create: `services/agent/tool/ask_user.go`
- Create: `services/agent/dal/model/tool_execution.go`
- Create: `services/agent/dal/mongo/tool_execution.go`
- Test: `services/agent/tool/catalog_test.go`
- Test: `services/agent/tool/asset_test.go`

- [ ] 注册 `asset.get_asset`、`asset.get_current_version`、`asset.create_version`、`generator.create_image_job`、`ask_user`。
- [ ] `generator.create_image_job` 默认 disabled；被请求时返回 `ErrAgentToolNotAllowed`。
- [ ] `asset.create_version` 默认 requires approval；未审批时通过 Eino interrupt 暂停。
- [ ] 每次 tool 执行写入 `agent_tool_executions`，包括 denied、failed、completed。
- [ ] 增加敏感字段脱敏函数和测试，覆盖 password/secret/token/authorization。
- [ ] 运行 `cd services/agent && go test ./tool ./dal/mongo -count=1`，预期 PASS。

### Task 5: Eino ToolCallingChatModel 适配与 fake model

**Files:**
- Create: `services/agent/llmclient/chat_model.go`
- Create: `services/agent/llmclient/fake_chat_model_test.go`
- Create: `services/agent/runner/model_test.go`

- [ ] 定义 `llmclient.Client` 接口，让生产实现调用 GP-02 `llm` RPC，让测试实现返回脚本化 tool calls。
- [ ] `chat_model.go` 实现 Eino `ToolCallingChatModel`，包括 `Generate`、`Stream`、`WithTools`。
- [ ] `WithTools` 只把白名单过滤后的 `schema.ToolInfo` 传给 llm。
- [ ] fake model 测试覆盖：无 tool 单轮回复、一次 tool call、流式 assistant delta、模型错误。
- [ ] 运行 `cd services/agent && go test ./llmclient ./runner -run TestChatModel -count=1`，预期 PASS。

### Task 6: Run 状态机、Runner 执行与事件日志

**Files:**
- Create: `services/agent/dal/model/run.go`
- Create: `services/agent/dal/model/event.go`
- Create: `services/agent/dal/mongo/run.go`
- Create: `services/agent/dal/mongo/event.go`
- Create: `services/agent/biz/run.go`
- Create: `services/agent/biz/event.go`
- Create: `services/agent/runner/runner.go`
- Modify: `services/agent/handler.go`
- Test: `services/agent/biz/run_test.go`
- Test: `services/agent/runner/runner_test.go`

- [ ] `StartAgentRun` 创建 `QUEUED` run 与 `RUN_STARTED` 事件后，异步切换到 `RUNNING`。
- [ ] Runner 从 Eino iterator 读取 event，转换为 `agent_events`，并更新 `last_event_seq`。
- [ ] 事件 seq 在同一 run 内严格递增；并发 append 使用 Mongo 原子更新或事务保证。
- [ ] Run 成功写 `ASSISTANT_MESSAGE` 与 `RUN_COMPLETED`；失败写 `RUN_FAILED`；取消写 `RUN_CANCELLED`。
- [ ] `PullAgentRunEvents(after_seq, limit, wait_ms)` 支持无新事件时等待，超时返回空列表。
- [ ] 运行 `cd services/agent && go test ./biz ./runner -run TestRun -count=1`，预期 PASS。

### Task 7: Interrupt、Resume 与 Cancel

**Files:**
- Create: `services/agent/runner/checkpoint.go`
- Modify: `services/agent/biz/run.go`
- Modify: `services/agent/tool/ask_user.go`
- Test: `services/agent/runner/checkpoint_test.go`
- Test: `services/agent/biz/resume_test.go`

- [ ] 实现 Redis-backed Eino CheckPointStore；测试使用 in-memory store。
- [ ] `ask_user` 与 approval tool 触发 interrupt 后，run 状态变为 `WAITING_USER`，事件类型为 `INTERRUPTED`。
- [ ] `ResumeAgentRun` 只允许 `WAITING_USER` 状态，使用 checkpointID 与 interruptID 继续 Runner。
- [ ] `CancelAgentRun` 只允许 queued/running/waiting_user；取消后后续 resume/message 返回 `ErrAgentRunStateConflict`。
- [ ] 运行 `cd services/agent && go test ./biz ./runner -run 'Test(Interrupt|Resume|Cancel)' -count=1`，预期 PASS。

### Task 8: edge-api REST 与 SSE 门面

**Files:**
- Modify: `services/edge-api/main.go`
- Modify: `services/edge-api/router.go`
- Create: `services/edge-api/handler/agent.go`
- Test: `services/edge-api/handler/agent_test.go`

- [ ] 初始化 agent Kitex client，service name 默认 `agent`。
- [ ] 注册 `/api/v1/agents` 登录态路由。
- [ ] REST handler 只做参数校验、BaseReq 注入、RPC 调用和统一 JSON 响应。
- [ ] SSE handler 支持 `after_seq` 与 `Last-Event-ID`，循环调用 `PullAgentRunEvents`，输出 `id: <seq>` 与 `data: <json>`。
- [ ] SSE handler 在 terminal event 后关闭连接；RPC 错误写 `event: error` 后关闭。
- [ ] 运行 `cd services/edge-api && go test ./handler -run TestAgent -count=1`，预期 PASS。

### Task 9: 观测、安全与资源限制

**Files:**
- Modify: `services/agent/runner/runner.go`
- Modify: `services/agent/tool/catalog.go`
- Modify: `services/agent/biz/event.go`
- Modify: `pkg/otel` only if existing helper cannot express required spans
- Test: `services/agent/runner/observability_test.go`

- [ ] 为 `agent.run`、`agent.model`、`agent.tool`、`agent.event_pull` 建 span，透传 trace/user/tenant metadata。
- [ ] 指标至少包含 run count/status、tool execution count/status、run duration、event lag。
- [ ] 限制单次 run 最大输入消息数、单事件 payload 大小、单 tool result 摘要大小。
- [ ] 日志和事件中不出现 API key、Authorization、JWT、password、secret。
- [ ] 运行 `cd services/agent && go test ./... -run 'Test.*Redact|Test.*Limit|Test.*Trace' -count=1`，预期 PASS。

### Task 10: 全链路验证与文档闭环

**Files:**
- Modify: `.axm/progress/generation-platform/roadmap.md`
- Modify: `.axm/knowledge/services/overview.md` after implementation completes

- [ ] 运行 `make fmt`。
- [ ] 运行 `make gen`。
- [ ] 运行 `make test-services`。
- [ ] 运行 `make lint`。
- [ ] 启动本地 dev 链路后，用固定测试账号创建 profile、启动 run、打开 SSE、触发一次 tool call、完成一次 interrupt/resume。
- [ ] 实现完成并通过验收后，把已落地的 `agent` 服务事实同步到 `knowledge/services/overview.md`，再闭合本 spec。

## AI 自动验收

| 验收项 | 命令 / 检查 | 预期 |
|---|---|---|
| IDL 生成 | `make gen` | 生成 `services/agent/kitex_gen` 与 `services/edge-api/kitex_gen/agent`，无错误 |
| agent 单测 | `cd services/agent && go test ./... -count=1` | PASS |
| edge handler 单测 | `cd services/edge-api && go test ./handler -run TestAgent -count=1` | PASS |
| 全服务测试 | `make test-services` | PASS |
| lint | `make lint` | PASS，且无直接 `fmt.Print/log.Print` |
| SSE 解析 | handler 测试断言 SSE 包含 `id:` 与 `data:`，terminal event 后关闭 | PASS |
| tool 白名单 | 单测覆盖未授权 tool 返回 `ErrAgentToolNotAllowed` | PASS |
| interrupt/resume | 单测覆盖 run 进入 `WAITING_USER` 并 resume 到 terminal 状态 | PASS |
| 敏感字段扫描 | 单测覆盖 secret/token/password/authorization 脱敏 | PASS |
| axm 契约 | `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service` | 0 errors |

## 人类验收

- 使用 `admin@platform.com` 登录后，可以在 HTTP API 创建一个用户 Agent profile，选择系统默认 tool 白名单。
- 调用 `POST /api/v1/agents/runs` 后，`GET /api/v1/agents/runs/:id/events` 能持续收到 SSE 事件：`run_started`、assistant delta/message、tool call started/completed、terminal event。
- 使用包含 `ask_user` 或 `asset.create_version` 的请求时，前端能看到 `interrupted` 事件；提交 `resume` 后同一个 run 继续执行并最终完成或失败。
- 未授权 tool 不会出现在模型可见 tools 中；即使模型伪造 tool name，服务也返回权限错误并写入审计事件。
- OpenObserve 中能用同一个 trace 看到 edge-api -> agent -> llm/tool 的链路；日志中没有 provider key、JWT、Authorization 或用户密码。
- 重启 agent 服务后，历史 completed/failed/cancelled run 可查询；历史 running run 被标记为 failed；waiting_user run 可继续 resume。

## 依赖与阻塞

- GP-03 正式执行依赖 GP-02 给出最小 `llm` RPC 契约，尤其是 tool-calling message、stream chunk、usage 和模型错误格式。
- 若 GP-02 尚未完成，可以先用 `llmclient.Client` fake 完成 agent 服务内的 TDD 与 edge SSE 验收；真实联调必须等 GP-02。
- `asset.create_version` 依赖 asset 当前版本写入能力；若权限模型不足，MVP 只允许当前用户自己的资产。
- `generator.create_image_job` 只做 catalog 占位，真实 tool 在 GP-05 接入。

## 实施进度

| 项目 | 状态 |
|---|---|
| Spec 创建 | 已完成 |
| IDL / 服务骨架 | 未开始 |
| Profile / Tool / Runner / Event | 未开始 |
| edge-api SSE | 未开始 |
| AI 自动验收 | 未开始 |
| 人类验收 | 未开始 |
