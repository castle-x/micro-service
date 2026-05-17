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
  - ../../../knowledge/services/overview.md
-->

# OTel-02：Hertz + Kitex Trace 注入

> **实施状态**：已完成
> **完成时间**：2026-05-12

## 背景

当前 `pkg/middleware/hertz` 和 `pkg/middleware/kitex` 已能生成并透传自研 `trace_id`，但尚未形成 OTel span 树。该阶段把入口和跨服务调用升级为标准 trace。

## 目标

- Hertz 入口生成 HTTP server span。
- Kitex server/client 生成 RPC server/client span。
- 支持 W3C `traceparent` / `baggage` 传播。
- 继续兼容 `X-Trace-ID`，方便客户端报障和现有日志查询。
- `edge-api -> idp/iam` 能在 trace backend 中看到同一条完整链路。

## 范围

- `pkg/middleware/hertz`
- `pkg/middleware/kitex`
- 首批验证链路：`edge-api -> idp -> iam`、`edge-api -> iam`

## 非目标

- 本阶段不覆盖 Mongo/Redis/MQ。
- 本阶段不修改业务 handler 的内部逻辑。

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| HTTP route label | 使用模板化 route，禁止完整 URL 作为 label |
| RPC span name | `<service>/<method>` 或 `RPC <service>/<method>` |
| 错误处理 | handler 返回错误时 span status 置为 error |
| 兼容 header | 响应保留 `X-Trace-ID` |

## 设计约束

- span name 必须低基数。
- 不记录 request/response body。
- 不把 user input、token、authorization 写进 attributes。

## 实施结果

- `pkg/middleware/hertz.Trace()` 已生成 HTTP server span，从 `traceparent` / `baggage` 提取 W3C context，并继续响应 `X-Trace-ID`。
- `pkg/middleware/kitex.Trace()` 已生成 Kitex server span，从 metainfo 中提取 W3C context，并保留现有 `trace_id` metainfo/log context。
- 新增 `pkg/middleware/kitex.ClientTrace()`，生成 Kitex client span，并将 W3C `traceparent` / `baggage` 注入 metainfo。
- `pkg/cloudwego.KitexClientOptions()` 已默认挂载 `ClientTrace()`，首批 `edge-api -> idp/iam`、`idp -> iam` 客户端会走 OTel client span。
- `edge-api`、`model` 的 Hertz middleware 顺序调整为 `Trace -> Recovery -> Logging`，让 HTTP span 在响应状态确定后结束。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| pkg 中间件单测 | 已通过：`cd pkg && go test ./middleware/... -count=1` |
| 首批服务编译 | 已通过：`make build` |
| trace 传播 | 已覆盖：单测验证 Hertz 提取 `traceparent`，Kitex client 注入 `traceparent` 到 metainfo，并串联到 Kitex server |
| span 树 | 已覆盖代码侧链路：单测验证 `HTTP server -> Kitex client -> Kitex server` 三段 span 同 trace 且父子关系正确；Trace UI 形态留给人类验收 |

## 人类验收

- 人类用 Trace UI 打开一次登录或用户信息请求，能看清 `edge-api -> idp/iam` 的调用顺序和耗时。
- 人类确认旧的 `X-Trace-ID` 排障习惯没有被破坏。
