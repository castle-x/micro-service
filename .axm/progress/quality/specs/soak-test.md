<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P3
related:
  - ../roadmap.md
  - ./perf-baseline.md
  - ./chaos-suite.md
-->

# QUAL-14 长跑（Soak）测试

## 实施进度

- 业务状态：`pending`

## 背景

短测验证"功能对"，但有一类问题**只在长时间运行后才显形**——本质是"资源累积速度 > 回收速度"。在小时甚至天级别才会越过崩溃阈值，是其他测试都看不见的盲区。

## 解决的根本问题

- **goroutine 泄漏**：每次请求 `go func()` 但 ctx 没取消 → 100 QPS 跑 12h 后 OOM
- **连接/FD 泄漏**：HTTP client / DB conn 不复用，慢漏几小时 → `too many open files`
- **缓存/Map 无界增长**：map 只加不删、缓存无 TTL → 内存单调上升
- **GC 压力累积**：长寿对象越多，full GC 越频繁，p99 退化
- **协议层状态泄漏**：HTTP/2 stream 不关、Kafka offset 不提交、etcd lease 续约异常
- **业务热点漂移**：数据分布随时间变化，缓存命中率从 90% 跌到 30%

> 边界：与 QUAL-13 性能测试的差别——**Soak 不追性能极限**，用日常 QPS 看系统能不能保持一周稳定。是趋势观测而非压测。

## 触发条件

- 每个 release 候选版本：周度 soak（24h）作为发版前置门
- 重大架构变更（连接池、goroutine 池、缓存策略）：必须 soak 验证
- 已知曾经发生 goroutine 泄漏的服务：每个 PR nightly 短 soak（4h）

## 关键观测项（Soak 的"指标即测试"）

Soak 不靠 assert 而靠观测趋势：

```
- heap_alloc          单调上升 = 泄漏
- goroutine_count     稳态后应平稳，持续上升 = goroutine 泄漏
- open_fds            应稳定，上升 = 资源未关闭
- db_conn_used        应在池上限以内稳定
- gc_pause_ns         不应单调上升
- p99_latency         不应随时间退化
- rate(errors)        不应突然抬升
```

## 本仓库高优先级场景

| 服务/链路 | Soak 关注点 |
|---|---|
| NSQ 消费者（credits/notification） | goroutine + offset 长期稳定 |
| model SSE 长连接 | FD + goroutine 双高危 |
| etcd lease 续约 | 24h 看租约不漂移 |
| Mongo 连接池 | 慢查询导致连接慢漏 |
| Kitex 服务长跑 | 重试/熔断器内部状态不漂移 |

## 验收标准

### AI 自动验收

- [ ] `tests/soak/` 目录约定
- [ ] NSQ 消费者 24h soak（首条样板）
- [ ] model SSE 12h soak（验证 FD/goroutine）
- [ ] OpenObserve 指标趋势告警接入
- [ ] 周度 nightly cron 跑 release 前置 soak

### 人类验收

- [ ] 人工确认长跑趋势阈值与 release 前置门禁策略

## 工具候选

| 工具 | 用途 |
|---|---|
| `k6` constant arrival rate | 稳态流量长跑 |
| `pprof` 持续采样 | heap/goroutine 历史对比 |
| OpenObserve metrics 长期保留 | 趋势可视化 |
| `goleak` | 单测级 goroutine 泄漏检测，soak 入口前先过 |

## 与混沌的合体

Soak + **间歇性小故障** 比纯 Soak 更能暴露问题：
- "故障恢复后是否真把临时持有的资源全释放了"
- "重连/重试机制长期是否泄漏"

## 待展开问题

- 24h soak 占用一台机器，CI 资源如何调度（独立 runner？）
- 趋势线斜率告警阈值如何定？需要先建立 baseline
- soak 期间引入 chaos 是否在本阶段还是 QUAL-08？建议双方共享 infra
