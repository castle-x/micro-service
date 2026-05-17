<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: devloop.md
    title: DEVLOOP 状态机
    when-to-read: 每次任务开始时，确认 T0-T3 分级与分支流程
  - path: quality.md
    title: 质量保障规范
    when-to-read: 编码完成前、提交前的质量门禁
  - path: docs.md
    title: 文档规范
    when-to-read: 产出 .axm 下任何 .md 文档时
  - path: vcs.md
    title: 版本控制规范
    when-to-read: 代码提交/分支操作前
  - path: review.md
    title: 二审 Review 规范
    when-to-read: 用第二个 agent / 工具 / 模型做收尾 review 时
-->

# universal/ — 通用规范

跨项目通用的工程规范，内容换一个项目仍然适用。所有文件使用 axm metadata 骨架 A（`applies-to: [universal]`）。

## 索引

| 文件 | 内容 | 何时读取 |
|---|---|---|
| `devloop.md` | DEVLOOP 状态机（意图识别→分级→分支→验证→交付） | 每次任务开始时 |
| `quality.md` | 测试策略、质量门禁、回归防护 | 编码完成/提交前 |
| `docs.md` | 文档规范（axm metadata 骨架 / 命名 / 索引 / 审查） | 产出文档时 |
| `vcs.md` | 版本控制（分支策略 / 原子提交 / 需求生命周期） | 代码变更时 |
| `review.md` | 二审 Review 规范（七条契约 / 闭环循环 / 最终报告） | 用第二个 agent / 工具做收尾 review 时 |
