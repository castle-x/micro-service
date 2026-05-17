<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
entries:
  - path: cr-rollout.md
    title: QUAL-01 代码审查规范推广
    when-to-read: 推 CR 流程、配置 CODEOWNERS、扩展 lint 工具链时
  - path: test-pyramid.md
    title: QUAL-02 测试金字塔补齐
    when-to-read: 给某个服务补集成测试、统计覆盖率基线时
  - path: contract-ci.md
    title: QUAL-03 契约 CI 卡口
    when-to-read: 配置 idl-compat / openapi-validate 在 CI 中的卡口时
  - path: e2e-suite.md
    title: QUAL-04 E2E 关键链路
    when-to-read: 实现某条新 E2E 链路或扩展 SSE 专项时
  - path: quality-metrics.md
    title: QUAL-05 度量与回顾
    when-to-read: 接入度量采集 / 准备季度回顾时
  - path: security-pipeline.md
    title: QUAL-06 安全扫描流水线（SAST/SCA）
    when-to-read: 接 gosec/govulncheck/staticcheck 到 CI、做安全攻防场景时
  - path: consistency-suite.md
    title: QUAL-07 数据一致性测试
    when-to-read: 验证 NSQ 幂等 / Saga 补偿 / 跨服务数据不变式时
  - path: chaos-suite.md
    title: QUAL-08 故障注入与混沌测试
    when-to-read: 设计 toxiproxy / docker pause 故障场景、验证降级与恢复时
  - path: config-startup.md
    title: QUAL-09 配置与启动验证
    when-to-read: 验证 .env / docker-compose / K8s manifest / 启动顺序与 health 行为时
  - path: observability-validation.md
    title: QUAL-10 可观测性验证
    when-to-read: 端到端验证 trace 贯通 / log correlation / metric 采集生效时
  - path: cdc-contract.md
    title: QUAL-11 契约消费方测试 (CDC)
    when-to-read: 解决 IDL 兼容但语义漂移 / 提供方不知道消费方真实期望的问题时
  - path: property-based-testing.md
    title: QUAL-12 属性测试 (PBT)
    when-to-read: 测试有数学性质的算法 / 状态机 / 序列化 / 业务不变式时
  - path: perf-baseline.md
    title: QUAL-13 性能 baseline 与压测
    when-to-read: 建立 SLA / 跑 k6 load / stress / spike 时（前端接入或预发环境后启动）
  - path: soak-test.md
    title: QUAL-14 长跑（Soak）测试
    when-to-read: 验证 NSQ 消费 / SSE 长连接 / etcd lease 等长跑稳定性时
  - path: dast-scan.md
    title: QUAL-15 动态安全扫描 (DAST)
    when-to-read: edge-api 对外暴露后，扫部署+配置类漏洞时
-->

# quality/specs — 阶段实施 spec

每个 spec 对应 `roadmap.md` 中的一个阶段（QUAL-XX），描述：

- 目标与验收标准
- 实施步骤
- 影响面与回滚方案
- 完成状态
