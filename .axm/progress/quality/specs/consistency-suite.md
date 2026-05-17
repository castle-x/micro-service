<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P0
related:
  - ../roadmap.md
  - ../../../project/api-testing.md
-->

# QUAL-07 数据一致性测试

## 实施进度

- 业务状态：`pending`

## 背景

纯后端微服务最容易出事的不是单服务 bug，而是**跨服务/异步事件**下数据不变式被悄悄破坏。本仓库典型链路：
- 支付：billing → NSQ → credits / notification
- 用户级联：iam.deleteUser → billing 订单引用 / credits 账户 / notification 偏好
- 并发扣费：同账户多并发 → 余额不能为负

`api-testing.md` 的 §3.6 仅覆盖 NSQ 幂等/死信的浅层断言。本阶段做**业务不变式级**的一致性验证。

## 解决的根本问题

- **at-least-once 消息语义下的幂等**：同一事件被消费 N 次 ≡ 消费 1 次的最终状态
- **跨服务 Saga 部分失败的补偿**：订单建了但扣费失败 → 订单应回滚或挂起，不能"半生不熟"
- **并发场景下的不变式**：余额非负、积分非负、订单状态机不能逆向迁移
- **删除/更新的级联完整性**：上游删除后下游引用不应悬空（要么级联清理要么 graceful fallback）

> 边界：不测性能（QUAL-13）、不测故障下行为（QUAL-08）、不测安全（QUAL-06）。一致性测试关注"**没有故障时**业务不变式是否成立"。结合 QUAL-08 时升级为"故障注入 + 不变式断言"。

## 触发条件

- 每条新增的跨服务事件链路：必须配 1 个一致性测试
- bugfix 涉及一致性：必须带回归测试（红线，沿用 universal/quality.md）
- nightly：跑全量一致性测试（耗时较高，不进 PR）

## 验收标准

### AI 自动验收

- [ ] `tests/consistency/` 目录约定建立
- [ ] billing→credits 链路一致性测试（首条样板）
- [ ] iam 用户删除级联完整性测试
- [ ] 并发扣费不变式测试（同账户 N goroutine）
- [ ] CI nightly 跑一致性测试套

### 人类验收

- [ ] 确认首批业务不变式清单覆盖高风险链路

## 工具候选

| 工具 | 用途 |
|---|---|
| `testcontainers` + 真 NSQ/Mongo | 复用 QUAL-02 集成测试基础 |
| `gopter` | 属性测试驱动（与 QUAL-12 协作） |
| `porcupine` | 高阶 linearizability check（如需） |
| 自写 `require.Eventually` | 异步事件最终一致性轮询断言 |

## 关键不变式清单（待补充）

- billing：`sum(credits.transactions) == sum(billing.payments where status=success)`
- credits：`account.balance == sum(transactions)` 且 `>= 0`
- 订单状态机：状态只能向前迁移，不可回退（除显式 cancel）
- 通知：每条已支付订单必有 1 条通知事件（at-least-once）

## 待展开问题

- 一致性测试用单元粒度（biz 层 mock）还是集成粒度（真容器）？
- 死信队列里的消息应该参与一致性断言吗？
- 时钟漂移场景如何模拟（NSQ 消息乱序）？
