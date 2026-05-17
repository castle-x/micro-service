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

# OTel-04：Mongo / Redis / MQ Instrumentation

> **实施状态**：已完成
> **完成时间**：2026-05-12

## 背景

服务慢请求最常见根因通常在 DB、缓存或消息队列边界。该阶段把 `pkg/db`、`pkg/redis`、`pkg/mq` 纳入统一 trace/metrics。

## 目标

- MongoDB 操作有 span 和 duration metrics。
- Redis command 有 span 和 duration metrics。
- MQ publish/consume 有 span，message context 可在生产者和消费者之间传递。
- 慢操作和错误能定位到 collection、operation、topic、command。

## 范围

- `pkg/db`
- `pkg/redis`
- `pkg/mq`
- 首批业务链路：idp/iam 的 Mongo/Redis；billing/credits/notification 在 MQ L2 化时接入。

## 非目标

- 不记录 DB query body 或 Redis value。
- 不为每个业务 repository 手写重复埋点，优先在公共封装层完成。
- 不在本阶段实现完整 MQ 业务流。

## 已确认开发细节

| 边界 | 必要 attributes |
|---|---|
| MongoDB | `db.system=mongodb`、`db.collection.name`、`db.operation` |
| Redis | `db.system=redis`、`db.operation` |
| NSQ publish | `messaging.system=nsq`、`messaging.destination.name` |
| NSQ consume | `messaging.system=nsq`、`messaging.destination.name`、retry/attempt |

## 设计约束

- labels/attributes 必须低基数。
- DB filter、Redis key value、message payload 默认不写入 span。
- MQ context 注入失败不得阻塞消息发送。

## 实施结果

- `pkg/db` 新增 MongoDB instrumentation helper，通用 Repository 方法会生成 `MongoDB <collection>.<operation>` client span，并记录 `db.client.duration`。
- `pkg/redis` 新增 Redis instrumentation helper，常用 Client/Lock 方法会生成 `Redis <COMMAND>` client span，并记录 `redis.client.duration`。
- `pkg/mq` 新增 W3C message context 注入/提取、publish span、consume handler 包装与 `mq.consume.duration`。
- `pkg/mq/nsq` 占位 Producer/Consumer 已接入 publish span 和 handler 包装；真实 NSQ 发送/消费实现可复用同一 helper。
- 自动测试确认 span attributes 不包含 DB query body、Redis value、message payload。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| pkg 测试 | 已通过：`cd pkg && go test ./db ./redis ./mq/... -count=1` |
| 慢操作定位 | 已覆盖代码侧：Mongo/Redis 单测验证 child span 与低基数 attributes |
| MQ context | 已覆盖：publish span 与 consume span 通过 message headers 串联为父子关系 |
| 敏感字段检查 | 已覆盖：span attributes 不包含 query body、Redis value、message payload |

## 人类验收

- 人类能从一次登录或权限请求 trace 中看出 Mongo/Redis 耗时占比。
- MQ 功能可用后，人类能看到 billing event 从 publish 到 consume 的链路。
