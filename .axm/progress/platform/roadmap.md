<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
progress-type: roadmap
initiative: platform
workflow-state: closed
state-updated: 2026-05-12
related:
  - ../../knowledge/services/overview.md
  - ../../knowledge/pkg-infra/overview.md
-->

# 平台整体路线图

> 始终反映**当前最新状态**。每个 Phase 结束时更新此文件。
>
> 最后更新：2026-05-12

---

## 当前阶段

**基础设施地基分支 · etcd + OpenTelemetry/OpenObserve** — ✅ 已完成基础闭环

本分支在 Phase 03 之后补齐开发地基：etcd 服务注册发现、`pkg/cloudwego`、`pkg/otel`、OpenTelemetry Collector、OpenObserve 和 `make dev-start/dev-stop/dev-restart` 一键开发链路。暂不继续扩展通用组件，后续优先回到具体业务能力。

## 事实源说明

- 当前进度、路线图和模块状态以本文件为准。
- 各阶段细节以对应 `.axm/progress/<initiative>/specs/*.md` 为准。
- 根目录 `SPEC.md` 与 `初步设计参考.md` 已降级为历史入口；旧设计意图已融合到 [`decisions.md`](decisions.md)。
- 当前 AI 协作与项目规范以根目录 `AGENTS.md` 和 `.axm/` 为入口。

## 阶段路线图

| Phase | 主题 | 状态 | 产物 |
|-------|------|------|------|
| 01 | pkg 基础设施（logger / db / utils） | ✅ 已完成 | `pkg/logger` · `pkg/db` · `pkg/utils`，全量 vet/build/test 通过 |
| 02 | pkg 业务骨架（config / errno / redis / middleware / jwt + registry/mq 占位） | ✅ 已完成 | 13 个 pkg 子包全绿，Makefile lint/test 真实命令就位 |
| 03 | idp + iam（含 Google 登录端到端） | ✅ 已完成 | IDL 补齐、Kitex 桩生成、identities/oauth_states、edge-api 回调、JWT 签发、单测全绿 |
| 04a | Kong + etcd 服务发现地基 | ✅ 已完成基础闭环 | [`../kong-etcd/roadmap.md`](../kong-etcd/roadmap.md)，[`../kong-etcd/specs/etcd-service-discovery.md`](../kong-etcd/specs/etcd-service-discovery.md) |
| 04 | iam RBAC / 权限 | ⏸ 待启动 | `iam.CheckPermission`，缓存到 Redis |
| 05 | billing + credits + mq 异步事件 | ⏳ 计划中 | `pkg/mq` L2 升级 + 事件驱动 |
| OTel | OpenTelemetry + OpenObserve 观测地基 | ✅ 已完成基础版 | [`../opentelemetry/roadmap.md`](../opentelemetry/roadmap.md)，dev 默认启用 OTel，OpenObserve 作为本地聚合平台 |
| 06 | notification + 部署深化 | ⏳ 计划中 | notification、生产化部署、告警与保留策略按需补 |

## 模块状态速览

### pkg（全部完成）

| 模块 | 状态 | 说明 |
|------|------|------|
| `pkg/logger` | ✅ 完成（P01） | zap 封装 + `Ctx(ctx)` 自动注入 trace_id |
| `pkg/db` | ✅ 完成（P01） | 泛型 `Repository[T]` + 软删除 + 强类型 Options |
| `pkg/utils` | ✅ 完成（P01） | time/id/crypto/json/convert/slice/net/file/context |
| `pkg/config` | ✅ 完成（P02） | viper + yaml + env + `${VAR}` 展开 + 泛型 `Load[T]` |
| `pkg/errno` | ✅ 完成（P02） | 全区段错误码 + `FromDBError` 与 pkg/db 互转 |
| `pkg/redis` | ✅ 完成（P02） | go-redis v9 单例 Client + redislock 分布式锁 + Key 辅助 |
| `pkg/jwt` | ✅ 完成（P02） | HS256 签发/校验，Signer/Verifier 接口隔离（预留 RS256） |
| `pkg/middleware` | ✅ 完成（P02 + OTel） | Kitex + Hertz 两套 trace/recovery/logging，已接入 W3C trace context |
| `pkg/cloudwego` | ✅ 完成（04a） | Kitex/Hertz etcd registry/resolver 薄 glue |
| `pkg/otel` | ✅ 完成（OTel） | OTel tracer/meter provider、OTLP exporter、resource、shutdown |
| `pkg/registry` | 🔶 L1 骨架（P02） | 通用 registry 预留；当前真实 etcd 路径是 `pkg/cloudwego` |
| `pkg/mq` | 🔶 L1 骨架（P02 + OTel helper） | NSQ 收发占位，已有 message context/span helper，Phase 05 再做业务事件 |

### services

| 服务 | 状态 | 说明 |
|------|------|------|
| `services/edge-api` | ✅ 完成（P03 + 04a + OTel） | Hertz HTTP 门面，注册到 etcd，发现 idp/iam/model，默认启用 OTel |
| `services/idp` | ✅ 完成（P03 + 04a + OTel） | Google/支付宝/密码登录能力，注册到 etcd，发现 iam，默认启用 OTel |
| `services/iam` | ✅ 完成（P03 + 04a + OTel） | User 主数据、角色/权限骨架，注册到 etcd，默认启用 OTel |
| `services/model` | ✅ 完成（model + 04a + OTel） | Hertz HTTP/SSE 模型服务，注册到 etcd，LLM provider spans/metrics |
| `services/billing` | ⬜ 占位（Phase 05） | |
| `services/credits` | ⬜ 占位（Phase 05） | |
| `services/notification` | ⬜ 占位（Phase 06） | |

### idl

| 文件 | 状态 |
|------|------|
| `idl/base.thrift` | ✅ 完成（P01） |
| `idl/idp/idp.thrift` | ✅ 完成（P03） |
| `idl/iam/iam.thrift` | ✅ 完成（P03） |
| `idl/model/openapi.yaml` | ✅ 完成（model HTTP 契约） |
| `idl/billing/billing.thrift` | ⬜ 骨架（Phase 05） |
| `idl/credits/credits.thrift` | ⬜ 骨架（Phase 05） |
| `idl/notification/notification.thrift` | ⬜ 骨架（Phase 06） |

## 当前建议验证命令

```
make build
make test-pkg
cd services/edge-api && go test ./... -count=1
cd services/idp && go test ./... -count=1
cd services/iam && go test ./... -count=1
cd services/model && go test ./... -count=1
```

历史单元测试覆盖：
- `services/idp/biz`：TokenBiz Issue/Verify/Refresh、短 secret 拒绝、id_token 解码
- `services/idp/cache`：RefreshToken 存取删、Blacklist 写入/过期/零TTL
- `services/iam/biz`：参数校验（空 email、无效 ObjectID）
- `services/iam/dal/mongo`：User model 字段、软删除、Touch、ObjectID 解析

端到端测试：需要真实 Google OAuth2 凭据，见 `scripts/e2e-google-auth.sh`。
