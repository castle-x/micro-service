<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: quality
related:
  - ../../project/code-review.md
  - ../../project/api-testing.md
-->

# 质量体系建设路线图

将 `project/code-review.md`、`project/api-testing.md` 与后续将立的测试规范，从文档落地为日常工程实践。

> 本路线含 15 个阶段（QUAL-01 ~ QUAL-15），按 **ROI（价值/投入比）+ 当前架构成熟度** 排序，不是严格串行。

## ROI 排序总表

> ROI 视角：本仓库当前是**纯后端微服务、未真正前端接入、混沌+性能能力来自团队自带专家**。"成本"指落地与维护投入，"价值"指能拦下的事故类别。

| 优先级 | ID | 阶段 | 状态 | 价值 | 成本 | 备注 |
|---|---|---|---|---|---|---|
| 🔴 P0 | QUAL-01 | 代码审查规范推广 | 🟡 进行中 | 高 | 低 | 已有规范，缺 CODEOWNERS + lint 增强 |
| 🔴 P0 | QUAL-03 | 契约 CI 卡口（IDL/OpenAPI）| 🟡 进行中 | 高 | 低 | 脚本已落地，差 CI 接入 |
| 🔴 P0 | QUAL-06 | 安全扫描流水线（SAST/SCA）| ⚪ 未开始 | 极高 | 低 | gosec + govulncheck 接 CI，半天工作量 |
| 🔴 P0 | QUAL-07 | 数据一致性测试 | ⚪ 未开始 | 极高 | 中 | 纯后端最易出事，billing→credits 链路优先 |
| 🟡 P1 | QUAL-02 | 测试金字塔补齐 | 🟡 进行中 | 高 | 中 | iam 模板已建，待复制到其他 7 个服务 |
| 🟡 P1 | QUAL-04 | E2E 关键链路 | 🟡 进行中 | 高 | 中 | 4/5 已落地，差 billing E2E |
| 🟡 P1 | QUAL-08 | 故障注入与混沌测试 | ⚪ 未开始 | 高 | 中 | 团队有专家，落地快 |
| 🟡 P1 | QUAL-09 | 配置与启动验证 | ⚪ 未开始 | 中-高 | 低 | 与 dev-ops/env-split 配合 |
| 🟡 P1 | QUAL-10 | 可观测性验证 | ⚪ 未开始 | 中-高 | 低 | 复用现有 OpenObserve，工具已有 |
| 🟢 P2 | QUAL-11 | 契约消费方测试 (CDC) | ⚪ 未开始 | 中 | 中 | IDL 兼容已部分覆盖，CDC 测语义层漂移 |
| 🟢 P2 | QUAL-05 | 度量与回顾 | ⚪ 未开始 | 中 | 中 | 依赖前序产生数据 |
| 🟢 P2 | QUAL-12 | 属性测试 (PBT) | ⚪ 未开始 | 中-高 | 高 | 写"好性质"是真功夫，从混沌延伸 |
| ⚪ P3 | QUAL-13 | 性能 baseline 与压测 | ⚪ 未开始 | 中 | 中 | 团队有专家，但无前端流量前不紧迫 |
| ⚪ P3 | QUAL-14 | 长跑（Soak）测试 | ⚪ 未开始 | 中 | 高 | NSQ/SSE/lease 长跑场景，时间成本高 |
| ⚪ P3 | QUAL-15 | 动态安全扫描 (DAST) | ⚪ 未开始 | 中 | 中 | edge-api 真正对外后再做 |

> 状态：⚪ 未开始 / 🟡 进行中 / 🟢 完成 / 🔴 阻塞

## 测试维度全景

按"测试解决的根本问题"分类（每类至少 1 个 QUAL-XX 覆盖）：

| 维度 | 解决的根本问题 | 关联阶段 |
|---|---|---|
| 功能正确性 | 代码符合预期行为 | QUAL-02 / QUAL-04 |
| 契约稳定性 | 服务间字节流"实际产生 ⊆ 实际期望" | QUAL-03 / QUAL-11 |
| 数据一致性 | 跨服务/异步事件下不变式不被破坏 | QUAL-07 |
| 可靠性 | 依赖故障下能优雅降级与恢复 | QUAL-08 |
| 安全性 | 代码漏洞 + 部署/配置漏洞 | QUAL-06 / QUAL-15 |
| 性能 | SLA 满足 + 找拐点 | QUAL-13 |
| 长跑稳定性 | 资源累积速度 ≤ 回收速度 | QUAL-14 |
| 隐藏边界 | 人没想到的输入 + 业务不变式 | QUAL-12 |
| 可观测性 | 排障工具链端到端真实可用 | QUAL-10 |
| 部署正确性 | 配置/启动顺序/manifest 正确 | QUAL-09 |
| 流程治理 | 审查 + 度量 + 持续改进 | QUAL-01 / QUAL-05 |

## 依赖关系

```
[基础层]                         [纵深层]                      [外延层]
QUAL-01 CR 推广 ─┐
QUAL-02 金字塔 ──┼──→ QUAL-04 E2E ──┐
QUAL-03 契约 CI ─┘                  ├──→ QUAL-08 混沌 ──┐
QUAL-06 安全扫描 ────────────────────┤                  ├──→ QUAL-12 PBT
QUAL-07 一致性 ─────────────────────┤                  ├──→ QUAL-13 性能
QUAL-09 配置启动 ────────────────────┤                  ├──→ QUAL-14 Soak
QUAL-10 可观测性 ────────────────────┤                  └──→ QUAL-15 DAST
QUAL-11 CDC ────────────────────────┘
                                                              │
                              QUAL-05 度量回顾 ←───────────────┘
```

P0 + 部分 P1 是后续所有阶段的前提。QUAL-05 度量必须在前序阶段产出数据后才能做有意义的回顾。

## 当前事实进度（2026-05-14）

### 已落地
- `.axm/project/code-review.md`、`.axm/project/api-testing.md` 规范
- `Makefile`：`test-unit / test-integration / test-contract / test-e2e / test-all / idl-compat / openapi-validate`
- `scripts/idl-compat.sh`、`scripts/openapi-validate.sh`、`scripts/e2e-all.sh`、`scripts/e2e-model-sse.sh`
- `services/iam/test/integration/` testcontainers 模板 + README
- `.github/pull_request_template.md`、`.github/workflows/ci.yml`

### 已立项未实现
- QUAL-06 ~ QUAL-15（共 10 个阶段，本轮新增）：spec 已写最小骨架，等待逐步展开

## 阶段简介与 spec 索引

按 ROI 顺序：

### 🔴 P0 立刻做
- **QUAL-01** [`cr-rollout.md`](specs/cr-rollout.md) — 代码审查规范推广
- **QUAL-03** [`contract-ci.md`](specs/contract-ci.md) — 契约 CI 卡口
- **QUAL-06** [`security-pipeline.md`](specs/security-pipeline.md) — 安全扫描流水线
- **QUAL-07** [`consistency-suite.md`](specs/consistency-suite.md) — 数据一致性测试

### 🟡 P1 近期推进
- **QUAL-02** [`test-pyramid.md`](specs/test-pyramid.md) — 测试金字塔补齐
- **QUAL-04** [`e2e-suite.md`](specs/e2e-suite.md) — E2E 关键链路
- **QUAL-08** [`chaos-suite.md`](specs/chaos-suite.md) — 故障注入与混沌
- **QUAL-09** [`config-startup.md`](specs/config-startup.md) — 配置与启动验证
- **QUAL-10** [`observability-validation.md`](specs/observability-validation.md) — 可观测性验证

### 🟢 P2 中期推进
- **QUAL-11** [`cdc-contract.md`](specs/cdc-contract.md) — 契约消费方测试 (CDC)
- **QUAL-05** [`quality-metrics.md`](specs/quality-metrics.md) — 度量与回顾
- **QUAL-12** [`property-based-testing.md`](specs/property-based-testing.md) — 属性测试 (PBT)

### ⚪ P3 后期推进（待外部条件）
- **QUAL-13** [`perf-baseline.md`](specs/perf-baseline.md) — 性能 baseline（前端接入或预发环境后启动）
- **QUAL-14** [`soak-test.md`](specs/soak-test.md) — 长跑测试（性能 baseline 稳定后启动）
- **QUAL-15** [`dast-scan.md`](specs/dast-scan.md) — DAST（edge-api 真正对外后启动）

## 历史决策

- **2026-05-14 立项 5 阶段**：QUAL-01 ~ QUAL-05 初始路线，集中在审查 + 测试金字塔 + 契约 + E2E + 度量。
- **2026-05-14 边界净化**：将 `code-review.md §8/§9`、`api-testing.md §6/§8` 的度量与渐进路线，从 `project/` 迁出到本目录。理由：规范无时间维度，路线/度量带时间维度。
- **2026-05-14 ROI 重排 + 新增 10 阶段**：基于"纯后端微服务、未前端接入、团队自带性能/混沌专家"的现状，新增 QUAL-06 ~ QUAL-15。原 QUAL-05 度量回顾下移到 P2（依赖前序数据）。新阶段先立项不展开，每份 spec 仅含背景 + 解决问题 + 工具候选 + 触发条件，留待逐步展开。
