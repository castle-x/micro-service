<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: pkg-infra/
    title: pkg 基础设施
    when-to-read: 理解或修改 logger/db/utils/config/errno/redis/jwt/middleware/registry/mq 时
  - path: services/
    title: 服务拓扑
    when-to-read: 理解或修改 edge-api、idp、iam、asset、llm、billing、credits、notification 服务时
  - path: observability/
    title: 可观测性
    when-to-read: 理解或排查 OpenTelemetry trace、metrics、log correlation 时
-->


# knowledge/ — 知识库

描述本项目各子系统"是什么"和"为什么这么设计"。采用**两层结构**：每个子系统一个目录，包含 `overview.md`（速查）+ 若干深度文档。

> **本 index 的 `entries` 由 AI 在 Phase 3 Author 阶段按项目实际子系统填充**，填充后可运行 `node scripts/reindex.mjs` 自动同步。

## 访问约定

1. **AI 初读**：优先读 `<system>/overview.md`（≤150 行速查）
2. **需要深度细节**：进入 `<system>/<topic>.md` 阅读
3. **频繁访问的事实**：都在 overview 里列出（模块清单、API 命令、DB 表、Store 清单等）

## 维护规则

- 代码变更影响设计时，同步更新对应知识文档，并重新走一轮 `last-reviewed` 校验
- 知识文档只记"是什么"和"为什么"，不记"应该怎么做"（规范走 `project/`）
- 每个文档的 `code-refs` 字段指向真实源码路径，作为事实锚点

## 与规范的区别

| | 规范（universal/project） | 知识库 |
|---|---|---|
| 内容 | "应该怎么做" | "是什么、为什么这么设计" |
| 时效 | 长期规则 | 随代码演进 |
| 触发 | DEVLOOP 引导 | 需要理解系统时主动查阅 |

参考 `references/knowledge-doc-guide.md` 获取知识文档的写作要点。
