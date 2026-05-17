<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: generation-platform
related:
  - ./decisions.md
  - ../asset/roadmap.md
  - ../platform/roadmap.md
  - ../../project/architecture.md
  - ../../knowledge/services/overview.md
-->

# AI 创作 / 生成资产平台总体路线图

> 当前文档是核心业务总 roadmap。它承载 AI 创作平台的产品目标、服务边界、阶段路线和依赖关系；平台地基、资产服务、质量体系等细节继续保留在各自 initiative 中。
>
> 最后更新：2026-05-17

## 背景与目标

平台目标不是做一个纯聊天式生图工具，而是让用户把创作经验沉淀为可复用的数字资产，并通过可配置的工作流持续创作、确认、保存和复用。

核心用户包括：

- 自运营虚拟 IP 账号，需要快速稳定地产出角色相关图片。
- 写真、影楼等商业出图场景，需要复用风格、人物、套系和参考图。
- 后续可扩展到宠物、汽车、电商产品等任意可结构化描述的资产类型。

核心价值：

- 用户可以自定义资产类型和资产组成部分。
- 用户可以把提示词、图片、结构化数据沉淀到资产中。
- 用户可以通过线性工作流逐步和不同 Agent 交互，创建每个步骤的具体资产内容。
- 用户可以直接调用生图能力，也可以让 Agent 把生图服务作为 tool 调用。
- 用户可以复用已有资产组合生成新内容，并继续微调、编辑、沉淀。

## 整合结论

本路线合并原 `generation-platform` 与 Eino 创作平台计划；不再新建平行的 `eino-platform` initiative。

`platform/` 不并入本路线。它继续作为已完成平台地基和历史决策入口，记录 `pkg`、`idp/iam`、etcd、OpenTelemetry、本地开发链路等通用能力。本路线只引用这些地基，不复制其阶段细节。

## 已完成平台地基

以下能力已作为本路线的前置条件存在，细节见对应 initiative：

| 地基 | 当前作用 | 事实源 |
|---|---|---|
| `pkg` 基础设施 | logger、db、config、errno、redis、jwt、middleware、cloudwego、otel | [`../platform/roadmap.md`](../platform/roadmap.md) |
| 身份与权限基础 | idp、iam、登录态、JWT、基础角色权限 | [`../platform/roadmap.md`](../platform/roadmap.md) |
| etcd 服务发现 | Kitex/Hertz 服务注册发现，供新增服务复用 | [`../kong-etcd/roadmap.md`](../kong-etcd/roadmap.md) |
| OpenTelemetry | trace、metrics、log correlation，AI 排障入口 | [`../opentelemetry/roadmap.md`](../opentelemetry/roadmap.md) |
| 本地 dev 链路 | dev-start/status/check-env、health、日志统一 | [`../dev-ops/roadmap.md`](../dev-ops/roadmap.md) |
| 质量门禁 | code review、契约、E2E、可观测性验证路线 | [`../quality/roadmap.md`](../quality/roadmap.md) |

## 当前设计判断

- `Asset` 是平台的核心沉淀物。第一版不做复杂资产状态机，使用 `savedToLibrary` 区分资产库内容和历史产物。
- 业务工作流不是 Eino Workflow 的直接映射，而是长生命周期的用户创作状态机：模板、运行实例、步骤状态、人工确认、资产写入规则都由 `workflow` 服务管理。
- Eino 的核心价值放在 Agent 执行层：每个工作流步骤可绑定不同类型的 Agent；Agent 基于 ReAct 循环调用 LLM 和 tools，产出步骤结果。
- 顶层调用链按业务边界收敛为：

```text
workflow -> agent -> llm
                  -> tool -> generator
                  -> tool -> asset
```

- `llm` 与 `generator` 必须拆开：`llm` 负责底层文本/多模态模型调用、模型供应商、流式输出和 usage；`generator` 负责生图/编辑/批量任务、重试、结果入库和生产溯源。
- `agent` 合并 Agent 配置、Agent 运行和 tool 注册/执行；早期不拆 `agent-registry` / `agent-runtime` / `tool-registry` / `tool-runtime`。
- `workflow` 合并工作流自定义、编排、执行和任务提交；早期不拆单独的 `workflow-control` / `workflow-runner`。
- `knowledge/RAG` 暂不进入第一阶段主线。平台早期优先完成创作工作流、Agent、生图和资产沉淀闭环。
- 当前 `services/model` 属于早期旧模型网关方向。后续不以兼容旧 API 为目标；根据新路线用 `llm` 服务重建模型调用层。
- 生图服务正式命名为 `generator`；`generation-platform` 只保留为产品路线 initiative 名称，不代表服务名。

## 服务边界

| 服务 | 职责 | Eino 使用点 | 非目标 |
|---|---|---|---|
| `workflow` | 工作流模板、步骤定义、运行实例、当前步骤、任务提交、人工确认、步骤产物与资产部分绑定 | 可在单步内部使用 Chain/Workflow 辅助结构化，但不把 Eino Workflow 当顶层业务状态机 | 不直接执行 ReAct 循环；不直接调用外部模型供应商 |
| `agent` | Agent 配置、默认 Agent、用户自定义 Agent、Agent run、事件流、tool 白名单、tool 执行、Interrupt/Resume | Eino ADK `ChatModelAgent`、`Runner`、Tool、middleware、checkpoint | 不持有 provider API key；不直接写资产主存储；不开放任意用户代码 |
| `llm` | provider/model/key 管理，Generate/Stream，tool-calling ChatModel 能力，模型路由、usage、限流、OTel | Eino `ChatModel` / `ToolCallingChatModel` 组件或 provider adapter | 不保存资产；不执行业务 tool；不承担生图任务状态机 |
| `generator` | 文生图、图生图、编辑、批量生成任务、重试、幂等、结果标准化、媒体入库、生成溯源 | 可作为 Eino Tool 被 `agent` 调用；自身不强依赖 Eino | 不承担通用 LLM 聊天；不管理工作流步骤状态 |
| `asset` | 资产类型、资产实例、版本、资产部分、媒体对象、OSS/CDN、生产溯源 | 被 `workflow`、`agent` tools、`generator` 调用 | 不执行模型调用或 Agent 决策 |
| `credits/billing` | 额度、扣费、订单、资金相关流程 | 消费 usage / generator 事件 | 不阻塞第一阶段创作闭环 |

## 用户交互主线

### 创建资产类型

用户定义一种资产类型，例如“角色资产”：

```text
角色资产
├─ 背景
├─ DNA
├─ 脸
├─ 身体
└─ 风格
```

每个部分声明允许保存的内容形态：

```text
背景：文本 + 结构化 JSON
DNA：结构化 JSON
脸：图片 + 文本 + 结构化 JSON
身体：图片 + 文本 + 结构化 JSON
风格：文本 + 图片
```

### 创建资产

用户基于资产类型创建资产实例，例如“角色 A”。工作流逐步引导用户完成：

```text
背景讨论 -> DNA 结构化 -> 脸部生图 -> 身体生图 -> 风格确认 -> 保存到资产库
```

每一步可绑定不同 Agent：

| 步骤 | 示例 Agent | 主要工具 |
|---|---|---|
| 背景讨论 | 背景设定 Agent | asset 读写草稿 |
| DNA 结构化 | 结构化抽取 Agent | asset 写入 JSON part |
| 脸部生图 | 脸部生成 Agent | generator 生图、asset 媒体入库 |
| 身体生图 | 身体生成 Agent | generator 生图、asset 媒体入库 |
| 风格确认 | 风格整理 Agent | asset 版本写入 |

### 复用资产生图

用户选择已有资产、提示词、风格和参考图，启动生产型工作流：

```text
选择资产 -> 选择目标 -> 组合上下文 -> 批量生图 -> 选择结果 -> 微调/编辑 -> 保存或保留历史
```

产物可以：

- 写入当前资产某个部分。
- 作为历史产物保留。
- 被用户保存到资产库。
- 生成新资产或新资产版本。

## 阶段路线图

| Phase | 主题 | 状态 | 产物 |
|---|---|---|---|
| GP-00 | Eino 创作平台边界决策 | 已确认，已记录决策 | [`decisions.md`](decisions.md)：明确 `generation-platform` 是核心产品路线，`platform` 是地基；确定 `workflow -> agent -> (llm, tool -> generator)` |
| GP-01 | Asset 基础能力对齐与闭合 | 已完成 | 资产类型、资产实例、版本、媒体对象、OSS/CDN、生成入库前置；见 [`../asset/roadmap.md`](../asset/roadmap.md) |
| GP-02 | `llm` 服务重建 | 未拆 spec | 删除旧 `model` 方向，基于 Eino ChatModel/ToolCallingChatModel 重建 provider、key、Generate/Stream、usage、OTel |
| GP-03 | `agent` 服务 MVP | 未拆 spec | Agent profile、默认 Agent、用户自定义 Agent、Runner、ReAct 工具循环、Agent SSE 事件流、tool 白名单 |
| GP-04 | `workflow` 服务 MVP | 未拆 spec | 工作流模板、步骤定义、运行实例、任务提交、人工确认、步骤产物与 asset part 绑定 |
| GP-05 | `generator` 服务与 tool 化 | 未拆 spec | 生图/编辑/批量任务、重试、幂等、结果写入 asset；同时支持直接调用和 Agent tool 调用 |
| GP-06 | 资产复用生产流 | 未拆 spec | 消费已有资产组合上下文，批量生成历史产物或资产版本，支持选择、微调、保存 |
| GP-07 | 额度、计费与商业化 | 未拆 spec | credits/billing 接入，按 LLM usage、tool 调用、generator job 和存储资源计量 |
| GP-08 | Knowledge/RAG 预留 | deferred | 仅当创作流程需要外部知识库或长期记忆时再启动 |

## 阶段依赖

```text
平台地基（platform / kong-etcd / opentelemetry / dev-ops）
  -> GP-01 asset 基础能力
    -> GP-02 llm
      -> GP-03 agent
        -> GP-04 workflow
          -> GP-05 generator
            -> GP-06 资产复用生产流
              -> GP-07 credits/billing
```

说明：

- `GP-02` 与 `GP-05` 可以部分并行，但 `agent` 需要一个可用的 `llm` ChatModel 后端。
- `workflow` 可以先接 mock agent，但正式闭环依赖 `agent` 事件流与步骤结果协议。
- `generator` 的结果入库依赖 asset 媒体与版本能力。
- 商业化不阻塞早期创作体验，但 usage 事件和幂等键应在 `llm` / `agent` / `generator` 设计时预留。

## 当前事实进度

| 事项 | 状态 |
|---|---|
| 平台地基 | 已具备基础闭环，作为本路线前置能力 |
| 资产服务 | AS-01 至 AS-04 第一版能力已完成；本路线只引用，不复制细节 |
| 生图服务命名 | 已确认服务名为 `generator`，不再使用 `generation` 作为服务名 |
| Eino 引入 | 已完成架构讨论，尚未落代码 |
| 旧 `model` 服务 | 属于早期模型网关方向，后续由 `llm` 服务替代，不作为未来主线扩展 |
| Knowledge/RAG | 暂不进入早期主线 |

## 验收口径

每个阶段拆 spec 时必须写清两类验收：

| 类别 | 要求 |
|---|---|
| AI 自动验收 | Go 单测、服务构建、契约检查、SSE 事件解析、OTel trace/metrics 检查、敏感字段扫描 |
| 人类验收 | 通过 UI/API 完成一条资产创作工作流：选择资产类型、逐步与 Agent 交互、生图、确认、保存到资产库 |

跨阶段核心验收：

```text
用户创建角色资产
  -> workflow 进入背景步骤
  -> agent 与用户多轮对话
  -> agent 调 llm 和 generator tool
  -> generator 写入 asset 媒体与版本
  -> workflow 等待用户确认并推进下一步
  -> 最终资产保存到资产库
```

## 当前未确认问题

- `workflow` 第一版允许用户自定义到什么程度：仅编辑模板参数，还是允许新增/删除步骤。
- `agent` 用户自定义范围：instruction、model、tools、output schema、max iterations、memory 策略分别开放到什么级别。
- `agent` 与 `llm` 的协议形态：内部 Kitex 还是 HTTP/SSE；如何承载 tool-calling rich message。
- `generator` 第一批供应商与图像任务模型：同步返回、异步 job、回调和重试策略。
- 生成历史产物保留策略、清理策略和额度计费策略。
- 是否需要在 GP-02 前直接删除旧 `services/model`，还是在代码重构时自然替换。

## 下一步

优先拆 `GP-02 llm 服务重建` 和 `GP-03 agent 服务 MVP` 的 spec。`asset` 当前已具备媒体与版本写入前置条件；后续拆 `GP-05 generator` 时只需核对 generator 到 asset 的入库协议与幂等边界。
