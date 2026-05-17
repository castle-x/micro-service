<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: generation-platform/
    title: AI 创作 / 生成资产平台
    when-to-read: 讨论 AI 创作平台、Eino Agent 编排、数字资产、工作流、生图任务、服务拆分和用户交互路线时
  - path: asset/
    title: 数字资产服务
    when-to-read: 设计或实现资产类型、资产实例、资产版本、资产部分、OSS/CDN 对接时
  - path: platform/
    title: 平台地基 / 整体演进
    when-to-read: 查看已完成平台地基、Phase 01-03、历史设计决策或通用支撑模块计划时
  - path: kong-etcd/
    title: Kong 与 etcd 接入
    when-to-read: 规划或实施 Kong 网关、etcd 服务注册发现、full infra 开发链路时
  - path: opentelemetry/
    title: OpenTelemetry 接入
    when-to-read: 规划或实施 trace、metrics、log correlation、观测栈和 AI 排障入口时
  - path: dev-ops/
    title: 本地开发运维改造
    when-to-read: 规划或实施本地 dev 阶段进程管理、健康检查、日志统一、env 拆分时
  - path: quality/
    title: 质量体系建设
    when-to-read: 推进代码审查推广、测试金字塔补齐、契约 CI、E2E 链路、度量回顾时
-->
# progress/ — 开发进度

管理阶段性开发上下文：roadmap、阶段 spec、验收状态与开发进展。这里记录“准备怎么做、做到哪里、如何验收”，不替代 `knowledge/` 中的系统事实。

## 当前 initiatives

| Initiative | 内容 |
|---|---|
| `generation-platform/` | AI 创作与生成资产平台的总体产品方向、Eino Agent 编排路线、服务边界、用户交互和阶段路线 |
| `asset/` | 数字资产服务的已确认设计、资产类型/部分/版本模型、OSS/CDN 接入路线 |
| `platform/` | micro-service 平台地基、已完成阶段与历史设计决策 |
| `kong-etcd/` | Kong 网关与 etcd 服务注册发现的路线图和实施 spec |
| `opentelemetry/` | OpenTelemetry trace、metrics、log correlation、观测栈和 AI 查询工具路线图 |
| `dev-ops/` | 本地 dev 阶段运维改造：进程生命周期、健康检查、日志统一、env 拆分 |
| `quality/` | 质量体系建设：代码审查推广、测试金字塔补齐、契约 CI、E2E 链路、度量回顾 |
