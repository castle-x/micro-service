<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P2
related:
  - ../roadmap.md
  - ./consistency-suite.md
  - ./chaos-suite.md
-->

# QUAL-12 属性测试 (PBT)

## 实施进度

- 业务状态：`pending`

## 背景

例子测试有**确认偏误**——你测的是你想到的场景。属性测试反过来问"无论输入是什么，应该满足什么性质？"，由框架自动生成几百上千个随机输入并 shrinking 到最小复现。

## 解决的根本问题

- **隐藏边界**：人没想到的边界用例（嵌套 nil、NaN、并发乱序、超长字符串等）
- **业务不变式**：在随机操作序列下，"余额非负"、"订单总额 = 支付总额"、"已发货必有支付"等不变式是否被破坏
- **分布式系统中的"灰色 bug"**：与 QUAL-08 混沌结合 = "随机故障序列 + 随机操作序列 → 业务不变式仍成立"

> 边界：不替代例子测试（具体 bug 用例仍写明确 case），不适合 CRUD 业务代码（无可表达数学性质）。适合：**序列化/反序列化、状态机、协议、算法、并发不变式**。

## 触发条件

- 新增有数学性质的算法或状态机：必须配 PBT
- 关键业务不变式（余额、订单状态机）：必须有 PBT 覆盖
- nightly 或周度跑（不进 PR，因耗时）

## 验收标准

### AI 自动验收

- [ ] `tests/property/` 目录约定
- [ ] credits 账户余额不变式 PBT（首条样板）
- [ ] 订单状态机迁移合法性 PBT
- [ ] 序列化对称性 PBT（`Unmarshal(Marshal(x)) == x` for all x）
- [ ] CI nightly 跑 PBT 套件

### 人类验收

- [ ] 确认首条 PBT 选择的业务性质足够稳定且有价值

## 工具候选

| 工具 | 用途 |
|---|---|
| `gopter` | Go 主流 PBT 框架，支持 shrinking |
| `rapid` | 现代 PBT，API 更人性化 |
| `porcupine` | 高阶 linearizability check（强一致性算法验证） |

## 与混沌的合体（关键升级）

```
传统 PBT:           随机生成事件序列 → 断言不变式
PBT + 混沌:         随机生成（事件序列 + 故障序列） → 断言不变式
                                    ↓
                            "故障恢复后系统状态依然合法"
                            （这是混沌测试的天花板）
```

## 待展开问题

- gopter vs rapid 选型？rapid 更现代但生态较新
- 首条 PBT 选什么场景？建议 credits 余额（业务核心、不变式清晰）
- 如何把 PBT 失败用例自动归档到 fixture，作为后续回归用例？
- 跑多久算够？1000 次随机迭代还是基于时间预算？
