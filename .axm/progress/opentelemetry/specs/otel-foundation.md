<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: opentelemetry
related:
  - ../roadmap.md
  - ../../../project/observability.md
  - ../../../knowledge/pkg-infra/overview.md
-->

# OTel-01：pkg/otel 基座

> **实施状态**：已完成
> **完成时间**：2026-05-12

## 背景

后续 HTTP/RPC、DB、Redis、MQ、LLM 的遥测都需要统一初始化和关闭逻辑。该阶段先建立最小公共基座，不改变业务行为。

## 目标

- 新增 `pkg/otel`，集中管理 tracer provider、meter provider、resource、OTLP exporter 和 shutdown。
- 支持通过配置或环境变量启用/禁用 OTel。
- 服务启动时可初始化 OTel，退出时可安全 shutdown。
- exporter 不可用时不影响业务进程启动。

## 范围

- `pkg/otel` 公共包。
- `pkg/config` 或服务配置结构中的 OTel 配置字段。
- 选取 `edge-api`、`idp`、`iam`、`model` 作为首批启动接入点。

## 非目标

- 本阶段不改 Hertz/Kitex span 逻辑。
- 本阶段不接入 DB/Redis/MQ/LLM instrumentation。
- 本阶段不搭建本地 Jaeger/Grafana 栈。

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 包位置 | `pkg/otel` |
| 导出协议 | OTLP，优先 gRPC；必要时支持 HTTP |
| Resource | `service.name`、`service.version`、`deployment.environment` |
| 默认行为 | 未配置 endpoint 时可禁用或 no-op |
| 采样 | local/staging 默认全采样；prod 预留 ratio 配置 |

## 设计约束

- `pkg/otel` 不得 import `services/*`。
- OTel 初始化失败不得导致业务服务不可用，除非显式配置为 strict。
- 禁止在配置、日志、span 中输出 collector credential。

## 实施结果

- 新增 `pkg/otel`，集中初始化 tracer provider、meter provider、W3C propagation、resource、OTLP exporter 与 shutdown。
- 新增 `otel` 配置结构，支持 `enabled`、`endpoint`、`protocol`、`environment`、`service_version`、`sample_ratio`、`insecure`、`strict`。
- `edge-api`、`idp`、`iam`、`model` 启动时读取 OTel 配置并注册退出 shutdown。
- 默认未启用或未配置 endpoint 时返回 no-op shutdown，不影响服务启动。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| pkg 编译测试 | 已通过：`cd pkg && go test ./... -count=1` |
| 服务编译 | 已通过：`make build` |
| 禁用 OTel 可启动 | 已覆盖：`pkg/otel` 单测验证 disabled/no endpoint 返回 no-op shutdown |
| 配置解析 | 已覆盖：`pkg/otel` 单测覆盖 enabled/endpoint/environment/sample ratio |

## 人类验收

- 人类确认服务启动配置足够简单，不要求业务开发者理解 Collector 细节。
- 人类确认配置命名能长期稳定使用。
