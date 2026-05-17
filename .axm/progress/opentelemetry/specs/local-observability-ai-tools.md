<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: opentelemetry
workflow-state: closed
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - ../../../project/observability.md
  - ../../../knowledge/observability/overview.md
-->

# OTel-06：OpenObserve 本地观测平台与 AI 查询工具

> **实施状态**：已完成并闭合（本地 compose、Collector 配置、Make target、AI 查询脚本已落地；基础 trace 链路已验证）
> **最近更新**：2026-05-17

## 背景

代码侧接入 OTel 之后，还需要人类和 AI 都能稳定读取数据。该阶段把本地统一观测平台、dashboard/search 入口和机器可读查询命令补齐。

经当前讨论确认，本阶段优先接入 self-host OpenObserve，作为 traces、metrics、logs 的统一查看与存储平台；Prometheus/Grafana/Jaeger 不作为第一版必需组件。OpenTelemetry Collector 仍建议保留，作为应用与观测后端之间的 OTLP 解耦层。

## 目标

- 提供本地 OpenTelemetry Collector 配置。
- 提供 OpenObserve 本地部署入口，用于接收、存储和查询 traces、metrics、logs。
- 提供 OpenObserve dashboard/search 入口，避免第一版同时维护 Grafana、Jaeger、Loki、Prometheus 多组件栈。
- 提供 `make obs-*` 或等价脚本，输出 AI 可读的 trace/log/metrics 摘要。
- 文档说明如何启动、如何制造测试链路、如何排障。

## 范围

- `deployments/observability/`
- `deployments/docker-compose*.yml`
- `Makefile`
- `scripts/observability/`
- 相关 README 或 `.axm/knowledge/observability/overview.md` 更新

## 非目标

- 不在本阶段承诺生产级 HA。
- 不强制接入云厂商托管观测平台。
- 不把 Prometheus/Grafana/Jaeger/Loki 作为第一版必需组件；已有 Prometheus 体系后续可作为 Collector 的可选并行出口。
- 不把 AI 查询工具做成复杂服务，先用脚本和 Make target。

## 已确认开发细节

| 入口 | 推荐默认 |
|---|---|
| Collector | OpenTelemetry Collector |
| 观测平台 | OpenObserve self-host Open Source Edition |
| 数据链路 | `services/* -> OTLP -> Collector -> OpenObserve` |
| Trace backend | OpenObserve traces；不再默认单独部署 Jaeger |
| Metrics backend | OpenObserve metrics；Prometheus 仅作为可选并行出口 |
| Logs backend | OpenObserve logs；本地文件日志保留为开发兜底 |
| Dashboard / UI | OpenObserve Web UI |
| AI scripts | JSON 或固定字段文本输出 |

建议命令：

```text
make obs-up
make obs-down
make obs-trace TRACE_ID=<id>
make obs-logs TRACE_ID=<id>
make obs-metrics SERVICE=<service>
make obs-errors SINCE=15m
```

## 设计约束

- AI 工具输出必须稳定，避免依赖人类 UI 文本。
- 观测栈启动失败不得影响普通 `make infra-up` 轻量开发链路。
- 观测后端 endpoint 和端口需要写入文档，避免靠记忆排障。
- 应用侧只配置 OTLP endpoint，不直接依赖 OpenObserve SDK 或专有协议。
- OpenObserve Open Source Edition 使用 AGPL-3.0；本阶段只接入和部署，不修改 OpenObserve 源码。生产或对外服务化前需要复核 license 与商业使用边界。
- Collector pipeline 要保留后续扩展空间：可追加 Prometheus remote write、Tempo、Loki 或托管 APM exporter，而不改业务服务。

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| 观测栈启动 | `make obs-up` 返回成功 |
| OpenObserve 可访问 | OpenObserve Web UI 可打开，默认账号和端口写入文档 |
| OTLP 链路 | 服务配置 OTLP endpoint 后，Collector 能把 telemetry 转发到 OpenObserve |
| trace 查询 | `make obs-trace TRACE_ID=<id>` 输出 root span、error span、slowest span |
| metrics 查询 | `make obs-metrics SERVICE=edge-api` 输出延迟、错误率、吞吐 |
| logs 查询 | `make obs-logs TRACE_ID=<id>` 输出关联结构化日志或明确报告当前日志采集未接入 |
| 不影响轻量链路 | `make infra-up` 仍只启动基础依赖 |

## 人类验收

- 人类能打开 OpenObserve 查看一次 `edge-api -> idp/iam` 请求的 trace。
- 人类能在 OpenObserve 中查看服务延迟、错误率和吞吐趋势。
- 人类能按 trace id 在 OpenObserve 中查到关联日志，或确认日志采集作为后续小阶段补齐。
- 人类能给 AI 一个 trace id，AI 能通过 `make obs-*` 输出形成排障结论。

## 实施记录

- 新增 `deployments/docker-compose.observability.yml`，本地启动 OpenObserve + OpenTelemetry Collector。
- 新增 `deployments/observability/otel-collector.yaml`，接收 OTLP gRPC/HTTP 并转发到 OpenObserve gRPC OTLP 入口。
- 新增 `scripts/observability/openobserve-query.mjs` 与 node:test 单测，提供 JSON 输出的 trace/logs/metrics/errors 查询入口。
- 新增 `make obs-up`、`make obs-down`、`make obs-ps`、`make obs-trace`、`make obs-logs`、`make obs-metrics`、`make obs-errors`。
- 更新 `make dev-start` / `make dev-restart` / `make model-start` / `make model-restart`，开发阶段默认启动观测栈并以 `OTEL_ENABLED=true`、`OTEL_ENDPOINT=localhost:4317` 启动服务。
- 新增 `deployments/observability/README.md`，记录本地端口、默认账号、服务 OTel 环境变量和查询命令。

## 闭合记录

- 2026-05-17：用户确认 OTel 已开发完成，OTel-06 已闭合为完成。
- 源码事实：`deployments/docker-compose.observability.yml`、`deployments/observability/otel-collector.yaml`、`scripts/observability/openobserve-query.mjs`、`scripts/observability/openobserve-query.test.mjs`、`deployments/observability/README.md` 已存在。
- 长期事实已同步到 `../../../knowledge/observability/overview.md` 和 `../../../project/observability.md`。
- 完整日志入库、metrics 展示美化、服务拓扑状态图仍是按需增强项，不作为 OTel-06 未完成项。
