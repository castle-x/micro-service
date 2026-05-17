<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P3
related:
  - ../roadmap.md
  - ./soak-test.md
-->

# QUAL-13 性能 baseline 与压测

## 实施进度

- 业务状态：`pending`

## 背景

团队自带性能测试专家。本阶段不是"教方法"，而是把性能测试**沉淀为可重复的 baseline + CI 集成**，并在本仓库技术栈（Hertz HTTP、Kitex Thrift、SSE、NSQ）上各落地一份代表性脚本。

> 当前无真实前端流量，性能测试 ROI 在 P3。一旦 edge-api 对外或预发环境就绪，升级到 P1。

## 解决的根本问题

- **SLA 验证**：在预期 QPS 下 p95/p99 是否满足约束
- **容量拐点**：throughput 升不上去的点在哪、瓶颈是 CPU/网络/连接池/锁竞争
- **降级行为验证**：限流/熔断/超时在压力下是否真生效
- **回归防护**：新版本性能是否相对 baseline 退化 > 10%

> 边界：本阶段是 **Load / Stress / Spike**；长跑稳定性是 QUAL-14 Soak。

## 四种子类型

| 类型 | 目的 | 时长 | 本仓库优先场景 |
|---|---|---|---|
| Load | 验证 SLA | 10 min | edge-api 主要接口 |
| Stress | 找拐点 | 渐增 5-30 min | model SSE 并发流 |
| Spike | 突发流量验证限流/熔断 | 10 倍突增 | Kong + edge-api |
| Soak | 长跑稳定性 | 4-24h | 见 QUAL-14（独立 spec） |

## 触发条件

- 每次发版前：跑 baseline，对比上版本，下降 > 10% 报警
- 重大架构变更（中间件/连接池/缓存策略）：必须重测 baseline
- 性能 PR：必跑相关场景

## 验收标准

### AI 自动验收

- [ ] `tests/perf/k6/` 目录约定
- [ ] 4 类场景脚本各至少 1 个
- [ ] baseline 入仓（`docs/perf-baseline.md`）
- [ ] CI 自动对比 baseline，回归报警
- [ ] 接 OpenObserve，性能指标可视化

### 人类验收

- [ ] 性能专家确认 baseline 环境与核心指标可作为后续对比依据

## 工具候选

| 工具 | 适用 | 备注 |
|---|---|---|
| `k6` | HTTP/SSE/gRPC，主力 | JS 脚本，OTel 集成 |
| `vegeta` | HTTP 极简 smoke | 命令行，配置极简 |
| `ghz` | gRPC（Thrift over TCP 可类比） | Kitex 压测候选 |
| `wrk` | 经典轻量 | 老牌但够用 |

## 关键观测指标

```
对外 HTTP:     p50/p95/p99 latency, RPS, 错误率, Kong 5xx, edge-api 5xx
内部 Kitex:    p99 server-side, RPC 错误率, 连接池利用率
SSE (model):   首 token 延迟分布, chunk 间隔 p99, 并发流上限, OOM 边界
NSQ:           生产/消费吞吐, 消息堆积深度, 重投率
DB:            Mongo 慢查询日志, Redis OPS, 连接池等待
```

## 待展开问题

- 性能测试环境标准化：用固定规格的 docker-compose 还是云上固定 instance？baseline 跨硬件无可比性。
- 测试数据：每次清库还是用 seed 数据池？
- 与 QUAL-08 混沌的合体：故障下性能（"灰色失败"）是否在本阶段还是混沌阶段覆盖？
