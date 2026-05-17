<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
entries:
  - path: universal/
    title: 通用规范
    when-to-read: 流程/质量/文档/VCS 相关规则（跨项目通用）
  - path: project/
    title: 项目规范
    when-to-read: 架构/设计/编码相关规则（绑定本项目）
  - path: knowledge/
    title: 知识库
    when-to-read: 理解系统设计、模块结构、设计决策
  - path: progress/
    title: 开发进度
    when-to-read: 查看 roadmap、阶段 spec、验收状态与开发进展
-->

# .axm — AI 开发上下文入口

`.axm/` 是本仓库给 AI 的唯一上下文目录，分为四个一级分区。根入口 `AGENTS.md` 通过 Knowledge Index 将不同需求路由到本目录对应子分区。

## 一级分区

| 目录 | 职责 | 生命周期 |
|---|---|---|
| `universal/` | 跨项目通用的流程/质量/文档/VCS 规范 | 长期 |
| `project/` | 绑定本项目的架构/设计/编码规范 | 长期 |
| `knowledge/` | 当前系统的设计事实，随代码演进而更新 | 中长期 |
| `progress/` | roadmap、阶段 spec、验收状态与开发进展 | 阶段性 |

## 文档规则

所有 `.axm/**/*.md` 必须遵循 `universal/docs.md` 定义的四套 axm metadata 骨架（A 规范 / B 知识 / C 索引 / D 进度）与命名/索引/审查规则。

## 访问链路

AI 查找规范或知识的标准路径：

```
AGENTS.md（根入口·Knowledge Index）
    └→ .axm/index.md（本文件·一级分区）
        └→ <dir>/index.md（子分区索引）
            └→ 具体 .md 文件
```
