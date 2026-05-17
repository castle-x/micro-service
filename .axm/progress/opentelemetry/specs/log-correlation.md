<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: opentelemetry
workflow-state: closed
state-updated: 2026-05-12
related:
  - ../roadmap.md
  - ../../../project/observability.md
  - ../../../knowledge/pkg-infra/overview.md
-->

# OTel-03：Log Correlation

> **实施状态**：已完成
> **完成时间**：2026-05-12

## 背景

项目日志已通过 `logger.Ctx(ctx)` 注入 `trace_id/user_id/tenant_id`。进入 OTel 后，日志还需要稳定关联当前 span，形成 `trace_id + span_id + service` 的排障闭环。

## 目标

- `logger.Ctx(ctx)` 自动注入 OTel trace id 和 span id。
- 保留现有 user/tenant/caller 字段。
- recovery 和业务错误日志能关联当前 span。
- 错误日志中的 `errno` code 可与 span attribute 对齐。

## 范围

- `pkg/logger`
- `pkg/middleware/hertz` recovery/logging
- `pkg/middleware/kitex` recovery/logging

## 非目标

- 本阶段不引入日志后端。
- 本阶段不改变日志输出格式以外的业务行为。

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 日志字段 | `trace_id`、`span_id`、`service`、`user_id`、`tenant_id` |
| 兼容策略 | 无 OTel span 时继续使用现有 `trace_id` |
| panic | 日志写 stack，span 记录 error event |
| errno | 写 `error_code` 字段，避免敏感 message |

## 设计约束

- 禁止记录 password、secret、token、authorization、cookie、API key。
- 禁止记录原始 prompt、模型响应正文、请求/响应 body。

## 实施结果

- `logger.Ctx(ctx)` 已优先从当前 OTel span context 注入 `trace_id` / `span_id`。
- 无 OTel span 时，`logger.Ctx(ctx)` 继续使用现有 metainfo/context key 中的 `trace_id`。
- Hertz recovery 已在当前 span 上记录 panic error event、设置 error status，并写入 `error.code` span attribute 与 `error_code` 日志字段。
- Kitex recovery 已在当前 span 上记录 panic error event、设置 error status，并写入 `error.code` span attribute 与 `error_code` 日志字段。
- Kitex Trace / ClientTrace 已在返回错误时设置 span error status，并写入 `error.code` attribute。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| logger 单测 | 已通过：`cd pkg && go test ./logger ./middleware/... -count=1` |
| 日志字段检查 | 已覆盖：构造带 span 的 ctx，日志输出包含 `trace_id` 和 `span_id` |
| 降级检查 | 已覆盖：无 span 的 ctx 仍输出现有 `trace_id` |

## 人类验收

- 人类从一条 Trace UI 的 span 复制 trace id 后，可以在日志输出中查到同一请求日志。
- 人类确认日志字段足够排障，且没有敏感信息泄漏。
