# CodeBuddy 项目规范入口

本项目的 AI 协作规范以根目录 [`AGENTS.md`](./AGENTS.md) 为唯一权威入口。

## 路由规则

CodeBuddy 在处理本仓库任务时，应直接读取并遵循：

- [`AGENTS.md`](./AGENTS.md) — 项目级 AI 协作入口、架构摘要、Knowledge Index 与通用编码规则
- `.axm/` — 由 `AGENTS.md` 路由到的项目规范与知识库

## 冲突处理

如本文件与 [`AGENTS.md`](./AGENTS.md)、`.axm/` 中的规则存在冲突，以 [`AGENTS.md`](./AGENTS.md) 为准。

本文件只作为 CodeBuddy 的轻量路由入口，不重复维护具体工程规范。
