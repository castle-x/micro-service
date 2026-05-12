# Phase 目录说明（历史保留）

> 本目录已由 `.axm/progress/` 替代，不再作为新的阶段进度事实源。新的 roadmap、阶段 spec、验收状态与历史决策请写入 `.axm/progress/<initiative>/`。

> 本目录用于记录项目**阶段性工作进度**。每一个阶段（Phase）一个 Markdown 文件，按时间顺序编号。
>
> 目的：在长周期开发中保留上下文，让任何人（包括未来的自己 / AI 协作者）都能快速了解「现在做到哪里」「下一步做什么」「为什么这么做」。

## 迁移后事实源

1. `.axm/progress/platform/roadmap.md` 是当前阶段、模块完成度、下一步路线的事实源。
2. `.axm/progress/<initiative>/specs/*.md` 记录对应阶段的关键决策、完成项、延后项和验证结果。
3. 根目录 `SPEC.md`、`初步设计参考.md` 只保留为历史入口；旧设计意图已迁入 `.axm/progress/platform/decisions.md`。
4. 若历史 `.phase/` 文档与 `.axm/progress/` 或源码冲突，以 `.axm/progress/` 和源码为准。

## 历史文件命名约定

```
phase-<序号>-<slug>.md        # 阶段进度快照，例如 phase-01-pkg-infra.md
phase-00-initial-design-reference.md # 立项初始设计参考归档
STATUS.md                     # 历史总体进度索引；新事实源见 .axm/progress/platform/roadmap.md
```

## 每个 phase 文档应包含的章节

1. **概述**：本阶段目标与范围
2. **关键决策**：重要的取舍、方案选型及原因
3. **已完成**：按模块列出成果 + 验证结果
4. **未完成 / 延后**：本阶段未做、推迟到后续的事项
5. **下一阶段建议**：对下一个 Phase 的方向建议
