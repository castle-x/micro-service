<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
depth: overview
code-refs:
  - services/edge-api/main.go
  - services/edge-api/handler/auth.go
  - services/edge-api/handler/asset.go
  - services/edge-api/router.go
  - services/idp/main.go
  - services/iam/main.go
  - services/asset/main.go
  - services/asset/handler.go
  - services/llm/main.go
  - services/billing/main.go
  - services/credits/main.go
  - services/notification/main.go
  - idl/asset/asset.thrift
  - deployments/config/edge-api.yaml
  - deployments/config/idp.yaml
  - deployments/config/iam.yaml
  - deployments/config/asset.yaml
  - deployments/config/llm.yaml
  - idl/llm/openapi.yaml
  - scripts/dev/services.json
  - idl/base.thrift
related:
  - ../../project/architecture.md
  - ../../project/coding.md
-->


# 服务拓扑 — 速查

## 定位

`services/` 是业务进程集合：`edge-api` 是公网 HTTP/WebSocket 的 Hertz 门面，`llm` 是独立 Hertz HTTP/SSE LLM 服务，`asset`、`idp`、`iam` 等业务服务以 Kitex RPC 为主。服务间通信应通过 IDL 生成的 RPC client、HTTP 契约或 MQ 事件完成，禁止直接 import 对方内部业务包。

## 模块清单

| 服务 | 路径 | 框架 | 职责 |
|---|---|---|---|
| edge-api | `services/edge-api/` | Hertz | REST/WebSocket 门面、参数校验、协议转换、回调接收 |
| idp | `services/idp/` | Kitex | OAuth2/OIDC、登录注册、Token 颁发与刷新、MFA |
| iam | `services/iam/` | Kitex | 用户资料、组织架构、角色权限 RBAC/ABAC、资源授权 |
| asset | `services/asset/` | Kitex | 个人资产类型、资产库、版本、媒体对象、上传会话、对象存储签名 URL |
| llm | `services/llm/` | Hertz | provider/model/key 管理、Generate/Stream、Eino ChatModel 适配；本地端口 `38083`，admin health 端口 `48083` |
| billing | `services/billing/` | Kitex | 支付订单、渠道对接、对账、退款 |
| credits | `services/credits/` | Kitex | 积分账户、余额校验、流水、积分规则引擎 |
| notification | `services/notification/` | Kitex | 短信、邮件、站内信、推送模板、多渠道发送 |

## 当前阶段事实

- `edge-api`、`idp`、`iam`、`asset`、`llm` 已具备可运行入口，并在启动时初始化 `pkg/otel`。
- `idp`、`iam`、`asset` 通过 `pkg/cloudwego` 注册 Kitex 服务到 etcd；`edge-api` 通过 etcd resolver 调用 `idp` / `iam` / `asset`，`idp` 通过 resolver 调用 `iam`。
- `edge-api` 和 `llm` 作为 Hertz 服务注册到 etcd；`edge-api -> llm` 使用 `HertzServiceResolver` 解析服务地址后再走 HTTP/SSE proxy。
- `edge-api -> llm` admin proxy 会覆盖客户端伪造的 `X-Caller`、`X-User-ID`、`X-Tenant-ID`，只透传登录态派生的可信 metadata；`llm` 将这些字段写入 request log 与 logger context。
- `llm` 的后端 provider test API 会对 OpenAI-compatible upstream 发最小非流式 Generate：在 provider `base_url` path 后追加 `/chat/completions`；如果 `base_url` 已含 `/v1`，实际请求为 `/v1/chat/completions`，不会重复拼接 `/v1/v1`。Web `/admin/llm` 页面上的 provider/model 测试按钮当前走 `/api/v1/admin/llm/generate` ping 默认模型或指定模型，使结果与 Chat Debug 使用同一条 Generate 链路。
- `llm_request_logs` 以 `user_id + model_ref + idempotency_key` 查询非流式成功幂等记录，不用空 userID 命中跨用户缓存。
- `llm` 的 provider test、Generate/Stream 上游错误、SSE error payload 和 request log error_message 输出前必须脱敏 API key、Authorization、JWT/token、password、secret。
- `edge-api` 已暴露 `/api/v1/assets` 资产、类型、分类、版本和媒体路由，并通过 `AssetService` Kitex client 调用 asset。
- `asset` 使用 MongoDB 保存资产主数据和上传会话，使用对象存储签名 URL 支持媒体上传/访问；进程当前初始化 Redis 并纳入 admin health。
- `billing`、`credits`、`notification` 仍是后续业务阶段；`pkg/mq/nsq` 真实收发尚未实现。
- 当前分支目标是 etcd + OpenTelemetry/OpenObserve 地基，暂不继续做通用组件扩展。

## 分层约定

| 层 | 常见路径 | 职责 |
|---|---|---|
| 入口 | `main.go` | 初始化配置、日志、中间件、RPC/HTTP server、注册发现 |
| Handler | `handler.go` 或 `handler/*.go` | 参数校验、调用 biz/RPC、组装响应 |
| Biz | `biz/*.go` | 业务编排和领域规则 |
| DAL Model | `dal/model/*.go` | MongoDB 文档模型 |
| DAL Mongo | `dal/mongo/*.go` | 集合访问、索引、Repository 封装 |
| Cache | `cache/*.go` | Redis 缓存、锁、幂等、临时状态 |
| MQ | `mq/*.go` | 生产/消费异步事件 |

## 核心链路

```text
Client → Kong → edge-api/Hertz → Kitex RPC → idp/iam/asset
edge-api/Hertz → HTTP proxy → llm/Hertz
billing → NSQ event → credits/notification
services/* → pkg/db + pkg/redis + pkg/logger + pkg/errno + pkg/middleware + pkg/cloudwego + pkg/otel
```

## edge-api Redis guard

- `edge-api` 不直接访问 MongoDB/Redis 业务主存储，业务数据仍通过 idp/iam/asset/llm 等后端服务读取或修改。
- 当前允许的 Redis 直读是鉴权前置 guard：`services/edge-api/middleware/auth.go` 读取 `idp:banned:{userID}`，用于本地 JWT 验签成功后的封禁拦截。
- 其他 Redis 业务缓存、主数据或权限数据仍归属后端服务；例如角色权限由 IAM 查询并在 IAM 侧缓存。

## Google 登录首发约束

- idp 负责 Google OAuth/OIDC 交互、identities 映射、JWT 签发。
- iam 负责用户资料主数据，idp 通过 RPC 调 iam 创建或读取用户。
- edge-api 只暴露 `/auth/google/url` 和 `/auth/google/callback` 等 HTTP 适配入口，不直接写身份或用户表。
