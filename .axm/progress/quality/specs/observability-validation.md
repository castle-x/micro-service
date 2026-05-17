<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
workflow-state: ready
state-updated: 2026-05-17
priority: P1
related:
  - ../roadmap.md
  - ../../../project/observability.md
  - ../../opentelemetry/index.md
-->

# QUAL-10 可观测性验证

## 实施进度

- 业务状态：`pending`

## 背景

`project/observability.md` 规定了"必须起 span / 必须打 metric / log 必带 trace_id"，但没人测**实际是否生效**。常见漂移：新加的 Kitex 调用忘了埋点、log 没带 ctx、metric label 拼错等，都要等线上排障失败才发现。

## 解决的根本问题

- **trace 贯通性**：一次 edge-api 请求是否能在 OpenObserve 看到完整链路（edge-api → iam → mongo）
- **log correlation**：同一 trace 的 log 是否都带相同 trace_id，能按 trace_id 检索
- **metric 完整性**：所有规范要求的 metric（RPC 计数、DB 延迟、错误率）是否真在采集
- **error 路径标注**：错误时 span status 是否设为 Error、metric 是否+1

> 边界：不测排障人的能力，只测"工具链是否真实可用"。可观测性的价值在排障，本阶段确保排障工具不在事故时才发现是坏的。

## 触发条件

- 每次新增 HTTP/RPC/DB/Redis/MQ/外部 API/LLM 链路：必须配可观测性断言
- 每周 nightly：跑全链路可观测性验证
- 改 observability 中间件 / collector 配置：必须验证回归

## 验收标准

### AI 自动验收

- [ ] `scripts/e2e-observability.sh`：发起一次 E2E 请求 → 用 OpenObserve API 查询 trace
- [ ] 断言：trace 含 ≥ 4 个 span（edge-api → iam → mongo → ...）
- [ ] 断言：每个 span 含规范要求的 attribute（service.name、http.method、db.system 等）
- [ ] 断言：同 trace_id 的 log 数量 ≥ N
- [ ] 断言：错误注入后 span status = Error，error metric +1
- [ ] 接 nightly CI

### 人类验收

- [ ] 人工确认查询结果能支持一次端到端排障演示

## 工具候选

| 工具 | 用途 |
|---|---|
| `scripts/observability/openobserve-query.mjs` | 已有，复用查询封装 |
| `Makefile obs-trace / obs-logs / obs-metrics` | 已有，封装成断言 |
| 自写 bash + jq | 解析 OpenObserve API 响应做断言 |

## 与其他阶段协作

- 复用 `opentelemetry/` initiative 已建立的 OpenObserve 实例和查询工具
- 与 QUAL-08 混沌测试结合：故障注入后验证错误能被采集到

## 待展开问题

- 测试环境的 OTel collector 是否独立于 dev 环境？
- 验证延迟（OTel batch 上报）：是否等多久后查询？
- metric cardinality 是否需要校验（避免 label 爆炸）？
