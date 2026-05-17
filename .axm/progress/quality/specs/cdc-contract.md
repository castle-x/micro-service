<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
workflow-state: ready
state-updated: 2026-05-17
priority: P2
related:
  - ../roadmap.md
  - ../../../project/api-testing.md
-->

# QUAL-11 契约消费方测试 (CDC)

## 实施进度

- 业务状态：`pending`

## 背景

`QUAL-03 contract-ci` 测的是 IDL **静态结构兼容**（fid 复用、required 改动、enum 改值）。但 IDL 兼容 ≠ 语义兼容。典型漂移：
- iam 把 `status` 从 `"active"` 改成 `"ACTIVE"`，IDL 类型不变，消费方代码挂
- billing 的 `Order.amount` 单位从分变成元，类型仍是 int64
- 错误码从 1001 改成 1101，消费方 switch case 路径错

这些都是 IDL **表达力之外**的契约。

## 解决的根本问题

> **两个服务能成功通信的前提**：提供方实际产生的字节流 ⊆ 消费方实际期望的字节流空间。

IDL 只是这个期望的一种近似。CDC 让**消费方**显式写下"我假设响应长这样"，作为契约本体；**提供方**在 CI 上重放消费方契约，断言响应符合预期。

- 解决 IDL 表达力不足（语义、零值约定、枚举字面量）
- 解决提供方"不知道消费方真实使用了哪个子集"

> 边界：不测性能、不测并发，只测"语义契约"。是 IDL 结构兼容（QUAL-03）的**语义层补充**。

## 触发条件

- 新增/修改 IDL 字段语义（即使类型不变）：消费方需更新契约文件
- 提供方 PR：CI 自动拉所有消费方契约 replay
- 新增消费方：必须写自己的契约文件

## 验收标准

### AI 自动验收

- [ ] `services/<consumer>/test/contract/<provider>_test.go` 约定建立
- [ ] edge-api 对 iam 的契约样板（首条）
- [ ] CI：iam IDL PR 自动跑所有消费方契约

### 人类验收

- [ ] 文档：CDC 与 QUAL-03 idl-compat 的分工边界

## 工具候选

| 工具 | 适用 | 备注 |
|---|---|---|
| `Pact` Go | HTTP（edge-api → model） | 成熟，OpenAPI 友好 |
| 自写 contract test（基于 gomock + golden file） | Kitex Thrift | Pact 对 Thrift 不友好，简化版更适合本仓库 |

## 简化版方案草图

```go
// services/edge-api/test/contract/iam_test.go
//go:build contract

// 消费方写"我对 iam.GetUser 的期望"
// 用 mock client 跑业务调用，记录请求 + 期望响应到 fixture
// 提供方 CI 用真实 iam 代码 replay fixture 中的请求，断言响应满足期望
```

## 待展开问题

- 契约文件存消费方仓还是中央仓？（本仓库是 monorepo，统一放消费方目录最自然）
- 与 QUAL-03 idl-compat 的关系：static structural 兼容 vs semantic 契约，是否共享同一 CI job？
- Kitex 的 Pact-Go 不友好，是否值得自写 contract framework，还是用最小手写 fixture？
