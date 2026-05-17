<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-14
owner: castlexu
entries:
  - path: roadmap.md
    title: 本地开发运维改造路线
    when-to-read: 查看 dev 阶段进程管理、健康检查、日志、配置改造的整体路线与当前进度
  - path: specs/
    title: 阶段 specs
    when-to-read: 实施某一具体阶段（进程管理 / 健康检查 / 日志统一 / 配置拆分）时
  - path: bugs/
    title: dev-ops BUG 看板与单条 BUG
    when-to-read: 提交 / 分派 / 修复 / 验收 dev-ops 范围内的 BUG
-->
# dev-ops — 本地开发运维改造

针对本地 dev 阶段（`make dev-start` 全栈调试）的运维流程稳定性与 AI 可读性优化。改造范围限定在 Makefile / scripts / pkg/health / pkg/logger / deployments/env 这几条线，不涉及生产部署。

核心原则：**AI 可读优先**。所有产物（PID、状态、日志、配置错误）必须是机器友好的结构化文本（JSON 行 / 单行 KV），方便 AI 用 Read / grep / jq 直接消费；不引入需要终端 UI 才能用的工具。

## initiatives 范围

| 文件 | 内容 |
|---|---|
| `roadmap.md` | 4 个阶段拆分、依赖关系、当前事实进度 |
| `specs/process-lifecycle.md` | DEV-01：进程优雅启停 + 状态文件 |
| `specs/health-endpoints.md` | DEV-02：标准 /healthz /readyz /version 接口 |
| `specs/log-unification.md` | DEV-03：日志格式统一 + AI 查询入口 |
| `specs/env-split.md` | DEV-04：.env 拆分 + 校验脚本 |
| `bugs/` | 本 initiative 实施过程中发现的 BUG（看板 + 单条文档） |
