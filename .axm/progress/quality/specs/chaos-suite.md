<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P1
related:
  - ../roadmap.md
  - ../../../project/observability.md
-->

# QUAL-08 故障注入与混沌测试

## 实施进度

- 业务状态：`pending`

## 背景

团队自带混沌工程专家，本阶段重点不是"教方法"，而是**把高危故障场景沉淀为可重复执行的脚本**，并把"业务不变式"作为断言（与 QUAL-07 一致性测试协作）。

## 解决的根本问题

- **依赖故障下的优雅降级**：Mongo/Redis/etcd/NSQ/LLM 任一挂掉，系统是雪崩还是降级？
- **gray failure（灰色失败）**：依赖没挂但慢，上游是堆积、熔断还是超时正确传播？
- **故障恢复的状态正确性**：故障期间的请求最终是否落库？连接池是否回收？租约是否续约？
- **级联失败防护**：单服务故障不应触发级联（限流/熔断/超时是否到位）

> 边界：不测纯性能极限（QUAL-13）、不测长跑资源累积（QUAL-14）。混沌测试关注"**事件式故障**下的瞬态/恢复行为"。

## 触发条件

- 新增依赖（DB/MQ/外部 API）：必须配对应混沌场景
- 重大架构变更（熔断器/限流器/超时配置）：必须配混沌验证
- nightly：跑完整混沌套件

## 高危故障场景清单（本仓库）

| 场景 | 工具 | 关键断言 |
|---|---|---|
| Mongo primary 切换 | `docker pause` | 写入失败但服务不崩，恢复后写入成功 |
| Redis 连接断 | `docker network disconnect` | 缓存击穿不打挂 DB，限流降级正确 |
| etcd 不可达 | `docker stop` | Kitex 服务发现降级到 last-known，不全失败 |
| NSQ lookupd 挂 | `toxiproxy` 丢包 | billing 事件堆积可恢复，无重复消费 |
| LLM provider 慢响应 | `toxiproxy` latency 注入 | model SSE 客户端超时正确触发，goroutine 不泄漏 |
| Kitex 单 pod 慢响应（gray failure） | `toxiproxy` 注入单实例 | 上游熔断 / 重试 / 降级正确 |
| Kong 5xx 突发 | k6 spike | edge-api 不雪崩，限流生效 |

## 验收标准

### AI 自动验收

- [ ] `scripts/chaos/` 目录约定
- [ ] 至少 5 个高危场景脚本可重复运行
- [ ] 每条断言：服务不 panic、不 OOM、/healthz 反映状态、故障恢复后无需手动重启
- [ ] 混沌套件接 nightly CI

### 人类验收

- [ ] 专家抽查高危场景覆盖是否足够

## 工具候选

| 工具 | 适用 |
|---|---|
| `docker pause/kill/stop` | 容器级别，本地+CI 易用 |
| `toxiproxy` | 网络层注入：延迟、丢包、截断、带宽限制 |
| `chaos-mesh` | K8s 环境（生产前演练） |
| `failpoint`（gofail） | 代码级故障点（精细但侵入） |

## 待展开问题

- 是否在 dev infra-up 流程之外起独立的"chaos infra"，避免污染开发环境？
- 与 QUAL-07 一致性测试的边界：一致性 = "没故障时不变式成立"，混沌 = "故障下恢复后不变式仍成立"，是否双方共享同一断言代码？
- gofail 注入需修改源码加 failpoint，与 AGENTS §3 外科手术原则的权衡？
