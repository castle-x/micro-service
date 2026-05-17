<!-- axm-meta
status: active
last-reviewed: 2026-05-14
owner: castlexu
entries:
  - path: roadmap.md
    title: 质量体系建设路线
    when-to-read: 查看代码审查 + API 测试体系的整体落地路线、阶段拆分与当前进度
  - path: specs/
    title: 阶段 specs
    when-to-read: 实施某一具体阶段（CR 推广 / 测试金字塔补齐 / 契约 CI / E2E 链路 / 度量回顾）时
  - path: bugs/
    title: quality 范围 BUG 看板与单条 BUG
    when-to-read: 跟踪跨模块质量 / axm 元数据 / 测试基础设施缺陷
-->

# quality — 质量体系建设

将 `project/code-review.md`（审查规范）与 `project/api-testing.md`（测试规范）落地到工程实践的进度跟踪与时间线。

> **范畴边界**：本目录只放"做到哪一步、下一步做什么、何时回顾"。规范本身（"应该怎么做"）不在这里，在 `project/` 下两份对应文档。规范是无时间维度的事实；本目录是带时间维度的计划。

## initiatives 范围

| 文件 | 内容 |
|---|---|
| `roadmap.md` | 15 阶段（QUAL-01~15）、ROI 排序、依赖图、当前事实进度 |
| `specs/cr-rollout.md` | QUAL-01 代码审查规范推广 |
| `specs/test-pyramid.md` | QUAL-02 测试金字塔补齐 |
| `specs/contract-ci.md` | QUAL-03 契约 CI 卡口（IDL/OpenAPI） |
| `specs/e2e-suite.md` | QUAL-04 E2E 关键链路 |
| `specs/quality-metrics.md` | QUAL-05 度量与回顾 |
| `specs/security-pipeline.md` | QUAL-06 安全扫描流水线（SAST/SCA） |
| `specs/consistency-suite.md` | QUAL-07 数据一致性测试 |
| `specs/chaos-suite.md` | QUAL-08 故障注入与混沌测试 |
| `specs/config-startup.md` | QUAL-09 配置与启动验证 |
| `specs/observability-validation.md` | QUAL-10 可观测性验证 |
| `specs/cdc-contract.md` | QUAL-11 契约消费方测试 (CDC) |
| `specs/property-based-testing.md` | QUAL-12 属性测试 (PBT) |
| `specs/perf-baseline.md` | QUAL-13 性能 baseline 与压测 |
| `specs/soak-test.md` | QUAL-14 长跑（Soak）测试 |
| `specs/dast-scan.md` | QUAL-15 动态安全扫描 (DAST) |
| `bugs/` | quality 范围（含 axm 元数据 / 测试基础设施）BUG 看板与单条文档 |

## 与规范的关系

```
.axm/project/code-review.md      规范（norm）：何时触发、必须遵守什么
.axm/project/api-testing.md      规范（norm）：测试金字塔、各层方案、反模式

         ↓ 实施

.axm/progress/quality/           计划（plan）：先做 A 再做 B、阶段验收、度量回顾
```

修改规范不需要改本目录；推进进度也不应反过来污染规范。
