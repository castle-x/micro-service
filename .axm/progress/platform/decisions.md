<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: decision
initiative: platform
related:
  - ../../project/architecture.md
-->

# 初始设计与历史决策归档

> **状态**：📦 历史参考 / 已归档
> **整理时间**：2026-05-07
> **来源**：根目录旧 `SPEC.md` 与 `初步设计参考.md`
> **定位**：只保留立项初期的有效设计意图，不再作为当前开发规范或进度事实源。

---

## 一、归档目的

立项初期的 `SPEC.md` 与 `初步设计参考.md` 曾用于快速统一架构方向和让 AI 生成初始骨架。随着 Phase 01 / Phase 02 已完成，这两个根目录文档中已经出现三类问题：

1. **实现已推进**：`pkg/logger`、`pkg/db`、`pkg/utils`、`pkg/config`、`pkg/errno`、`pkg/redis`、`pkg/jwt`、`pkg/middleware` 已完成，旧文档中的 TODO 或示例代码不再准确。
2. **路线已调整**：原计划优先做 idp 最小登录链路，后来调整为先补齐 pkg 业务骨架，再在 Phase 03 做 idp + iam + Google 登录。
3. **存在旧占位信息**：旧文档含 `github.com/yourco/platform`、Go 1.22、旧 pkg 文件名、旧 Makefile 示例等，不应继续被 AI 当作当前事实。

因此：

- 根目录 `SPEC.md` 与 `初步设计参考.md` 降级为**路由入口**。
- 本文件只保留有效的初始设计意图和已废弃信息清单。
- 当前进度以 `.axm/progress/platform/roadmap.md` 为准；当前 AI 协作规范以根目录 `AGENTS.md` 和 `.axm/` 为准。

---

## 二、事实源优先级

| 优先级 | 文件/目录 | 用途 |
|---|---|---|
| 1 | `.axm/progress/platform/roadmap.md` | 当前阶段、模块完成度、下一阶段路线图的唯一事实源 |
| 2 | `.axm/progress/<initiative>/specs/*.md` | 各阶段关键决策、完成项、延后项和验证结果 |
| 3 | `AGENTS.md` + `.axm/` | AI 协作入口、项目架构规范、编码规范、知识索引 |
| 4 | 实际源码与测试 | 实现细节的最终事实源 |
| 5 | 本文件 | 初始设计意图的历史参考 |
| 6 | 根目录 `SPEC.md` / `初步设计参考.md` | 仅作为路由入口，不承载具体规范 |

若任意历史内容与 `.axm/progress/platform/roadmap.md`、`.axm/` 或源码冲突，以更高优先级为准。

---

## 三、从旧文档继承的有效设计意图

### 3.1 总体架构

保留以下核心架构方向：

```text
Client → Kong(edge-gateway) → Hertz(edge-api) → Kitex RPC services
                                      ↓
                         MongoDB / Redis / etcd / NSQ
```

当前仍有效的定位：

- `edge-gateway`：Kong，负责 TLS、路由、粗粒度限流、JWT 签名/过期校验。
- `edge-api`：Hertz，负责 REST/WebSocket 门面、参数校验、协议转换和第三方回调。
- `idp`：Kitex，负责 OAuth/OIDC、登录注册、Token 颁发与刷新。
- `iam`：Kitex，负责用户资料、组织、RBAC/ABAC、资源授权。
- `billing`：Kitex，负责支付订单、渠道、对账、退款。
- `credits`：Kitex，负责积分账户、余额、流水、规则。
- `notification`：Kitex，负责短信、邮件、站内信、推送模板和多渠道发送。

### 3.2 Monorepo 结构

保留以下模块边界：

- `idl/`：全局 Thrift IDL 独立 module。
- `pkg/`：通用基础设施独立 module。
- `services/*`：业务服务，每个服务独立 module。
- `deployments/`：本地与部署配置。
- `.axm/progress/`：阶段进度与决策记录。
- `.axm/`：AI 可读的项目规范与知识库。

### 3.3 服务分层

服务内仍按以下职责分层：

| 层 | 路径 | 职责 |
|---|---|---|
| 入口 | `main.go` | 初始化配置、日志、中间件、server、注册发现 |
| Handler | `handler.go` / `handler/*.go` | 参数校验、调用 biz/RPC、组装响应 |
| Biz | `biz/*.go` | 业务编排和领域规则 |
| DAL Model | `dal/model/*.go` | MongoDB 文档模型 |
| DAL Mongo | `dal/mongo/*.go` | 集合访问、索引、Repository 封装 |
| Cache | `cache/*.go` | Redis 缓存、锁、幂等、临时状态 |
| MQ | `mq/*.go` | 生产/消费异步事件 |

### 3.4 IDL 规范

继续保留：

- 共享结构放 `idl/base.thrift`。
- 服务 IDL 放 `idl/{service}/{service}.thrift`。
- 请求命名 `XxxReq`，响应命名 `XxxResp`，服务命名 `XxxService`。
- 字段编号从 1 连续递增。
- 时间字段使用 `i64` Unix 秒。
- 金额字段使用 `i64` 分。
- 枚举显式保留 `UNKNOWN = 0`。
- 请求中透传 `base.BaseReq`。

### 3.5 横切规范

继续保留：

- 错误码按服务区段分配，当前实现见 `pkg/errno`。
- Redis 键名遵循 `{service}:{resource}:{id}:{action}`。
- 日志统一通过 `pkg/logger.Ctx(ctx)`。
- trace/caller/user/tenant 元数据由 `pkg/middleware` 透传。
- 配置文件只放非敏感配置；密钥走环境变量。
- `edge-api` 禁止直接访问业务数据库。
- `services/*` 之间禁止直接 import 对方内部 Go 包，跨服务走 IDL + RPC 或 MQ。

---

## 四、已废弃 / 已修正的信息

以下旧信息不得继续作为当前事实：

| 旧信息 | 当前事实 |
|---|---|
| `SPEC.md` 是最高优先级约束 | 当前以 `AGENTS.md` + `.axm/` 为 AI 规范入口，以 `.axm/progress/platform/roadmap.md` 为进度事实源 |
| `github.com/yourco/platform` 示例路径 | 当前 module 路径为 `github.com/castlexu/micro-service` |
| Go 1.22 示例 | 当前 `go.work` / 各 module 使用 Go `1.25.6` |
| 旧 `pkg/redis/redis.go`、`pkg/mq/producer.go`、`pkg/registry/etcd.go` 占位 | Phase 02 已清理并重写为当前实现/接口骨架 |
| Phase 02 做 idp 最小登录链路 | Phase 02 已改为 pkg 业务骨架；Phase 03 才做 idp + iam + Google 登录 |
| `pkg/registry` / `pkg/mq` 已接真实 etcd/NSQ | 当前为 L1 接口骨架，真实接入延后 |
| 旧文档中的长代码模板 | 仅作历史参考，实际写法以当前源码和 `.axm/project/coding.md` 为准 |
| 旧 `初步设计参考.md` 中的大量空行、占位、拼写错误 | 不再引用，已由本归档文件提炼 |

---

## 五、旧文档到当前文档的映射

| 旧文档内容 | 当前应读取 |
|---|---|
| 当前阶段 / 下一步 | `.axm/progress/platform/roadmap.md` |
| Phase 01 产物 | `.axm/progress/platform/specs/phase-01-pkg-infra.md` |
| Phase 02 产物 | `.axm/progress/platform/specs/phase-02-pkg-business-skeleton.md` |
| 架构和包边界 | `.axm/project/architecture.md` |
| 编码、测试、lint、module 规则 | `.axm/project/coding.md` |
| pkg 基础设施事实 | `.axm/knowledge/pkg-infra/overview.md` |
| 服务拓扑事实 | `.axm/knowledge/services/overview.md` |
| AI 协作入口 | `AGENTS.md` |

---

## 六、维护规则

1. 不再扩写根目录 `SPEC.md` 或 `初步设计参考.md`。
2. 阶段进度变化只更新 `.axm/progress/platform/roadmap.md` 和对应 `.axm/progress/<initiative>/specs/*.md`。
3. 稳态架构/编码规范变化更新 `.axm/project/*.md`。
4. 代码事实变化更新 `.axm/knowledge/**/*.md`。
5. 若历史参考内容被证明过时，不修改旧原文，直接在本文件追加“已废弃 / 已修正”说明。
