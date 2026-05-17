<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: decision
initiative: generation-platform
related:
  - ./roadmap.md
  - ../platform/roadmap.md
  - ../asset/roadmap.md
-->

# AI 创作平台路线整合决策

> 本文记录已确认且影响后续 roadmap/spec 的阶段性决策。已落地为系统事实后，再同步到 `knowledge/` 或 `project/`。

## 2026-05-17：合并产品线，不合并平台地基

### 背景

原 `generation-platform` roadmap 已经承载通用生图平台、数字资产、工作流、生图任务、计费等核心业务方向，但它形成时尚未纳入 Eino。

后续讨论确认：

- 项目早期可以砍掉旧 `model service` 方向，不需要为历史 API 保持兼容。
- Eino 应作为 Go AI 应用框架被深度使用，尤其用于 Agent、tool calling、流式输出和局部编排。
- 创作平台的业务主线是用户通过线性工作流逐步和不同 Agent 交互，产出并沉淀资产内容。
- 顶层业务调用关系收敛为：

```text
workflow -> agent -> llm
                  -> tool -> generator
                  -> tool -> asset
```

### 决策

1. 保留 `.axm/progress/generation-platform/` 路径，不新建平行的 `eino-platform/`。
2. 将 `generation-platform` 标题升级为“AI 创作 / 生成资产平台”，作为核心产品总 roadmap。
3. Eino 创作平台计划并入 `generation-platform/roadmap.md`，不写入 `knowledge/` 作为已实现事实。
4. `platform/` 不并入本产品路线。它继续记录平台地基、认证链路、基础设施和历史决策。
5. `asset/` 继续独立跟踪。它是生成平台的核心依赖，但领域模型和验收足够重，不合回总 roadmap。
6. `knowledge/RAG` 暂不进入早期主线，直到创作流程明确需要知识库或长期记忆。

### 服务边界决策

| 服务 | 决策 |
|---|---|
| `workflow` | 合并工作流自定义、编排、执行、任务提交和人工确认；不拆 `workflow-control` / `workflow-runner` |
| `agent` | 合并 Agent 配置、运行、tool 注册和 tool 执行；不拆 `agent-registry` / `agent-runtime` / `tool-registry` / `tool-runtime` |
| `llm` | 独立服务，承载 provider/model/key、Generate/Stream、tool-calling ChatModel、usage 和观测 |
| `generator` | 独立服务，承载生图/编辑/批量任务；既能直接调用，也能作为 Agent tool 被调用 |
| `asset` | 继续作为独立 Kitex 服务，负责资产和媒体主数据 |

### 非目标

- 不把 Eino 的 Workflow 直接等同于业务工作流。
- 不让 `workflow` 直接执行 ReAct 循环。
- 不让 `agent` 持有 provider API key 或直接访问业务主存储。
- 不在早期建立独立 `knowledge` / RAG 服务。
- 不为了兼容旧 `model` API 设计迁移层。

### 后续影响

- 新增业务能力优先从 `generation-platform/roadmap.md` 进入，再拆阶段 spec。
- `platform/roadmap.md` 中未启动的 IAM RBAC、billing/credits、notification 等通用阶段不再自动作为主线推进；只有被创作平台阶段需要时再拉起。
- `AGENTS.md` Knowledge Index 需要把 AI 创作平台 / Eino 编排 / 工作流与生图服务设计路由到 `generation-platform`。
- 旧文档中 `model-service`、`prompt-service`、`generation-service` 的边界描述若与本决策冲突，以本决策和最新 roadmap 为准；生图服务名以 `generator` 为准。

## 2026-05-17：生图服务命名调整为 `generator`

### 背景

原路线中使用 `generation` 表示生图服务，但它容易和 `generation-platform` initiative、生成类业务动作、历史 `generation-service` 文档混淆。

### 决策

1. 生图服务正式命名为 `generator`。
2. `.axm/progress/generation-platform/` 路径保留不变，只表示产品路线名称。
3. 后续 spec、服务边界和 Agent tool 描述统一使用 `generator`。
4. 旧字段名或历史 DTO 中的 `GenerationJobID` / `AssetSource.GENERATION` 先保持代码契约兼容，不因文档命名调整立即重命名。
