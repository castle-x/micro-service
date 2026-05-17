<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
applies-to: [project:micro-service]
related:
  - ../knowledge/pkg-infra/overview.md
  - ../knowledge/services/overview.md
  - ../knowledge/observability/overview.md
-->


# micro-service 架构规范

## 项目定位

`micro-service` 是一个 Go 单仓库多模块微服务平台骨架，核心链路是 `Kong → Hertz edge-api → Kitex RPC services / Hertz llm service → MongoDB/Redis/etcd/NSQ`。当前分支已把 etcd 服务发现、OpenTelemetry/OpenObserve 本地观测地基、asset 数字资产服务和 GP-02 `llm` 服务接入开发链路。

## 模块划分

| 模块 | 职责 | 约束 |
|---|---|---|
| `idl/` | 全局接口契约目录 | Thrift IDL 定义跨服务 RPC 契约，共享结构统一放 `idl/base.thrift`；`idl/llm/openapi.yaml` 定义 llm HTTP/SSE 契约 |
| `pkg/` | 通用基础设施独立 module | 可被 `services/*` 依赖；禁止反向依赖任何业务服务 |
| `services/edge-api/` | Hertz HTTP/WebSocket 接入层 | 只做协议适配、参数校验、调用 RPC、统一响应；禁止直接访问业务主存储 |
| `services/idp/` | Kitex 身份认证服务 | 负责 OAuth/OIDC、登录注册、Token 签发/刷新；外部身份映射归属 idp |
| `services/iam/` | Kitex 用户与权限服务 | 负责用户资料、组织、角色权限、资源授权；不处理 OAuth 协议细节 |
| `services/llm/` | Hertz HTTP LLM 服务 | 负责 provider/model/key 管理、非流式 Generate、SSE Stream；由 edge-api 通过 HTTP proxy 调用 |
| `services/billing/` | Kitex 支付服务 | 负责订单、支付渠道、对账、退款；资金相关逻辑只在本服务内收口 |
| `services/credits/` | Kitex 积分服务 | 负责积分账户、余额、流水、规则；消费 billing 事件 |
| `services/notification/` | Kitex 通知服务 | 负责短信、邮件、站内信、推送模板；偏异步事件消费 |
| `services/asset/` | Kitex 数字资产服务 | 负责个人资产类型、资产实例、版本、媒体对象、上传会话和对象存储签名 URL |
| `deployments/` | 本地与部署配置 | 放 Docker/Kong/K8s 等运行环境配置 |
| `.axm/progress/` | 阶段进度与计划 | 记录 roadmap、阶段 spec、验收状态和历史决策，不作为运行时输入 |

## 依赖方向

```text
services/* → pkg
services/* → idl/generated code（生成后）
edge-api → idp/iam/asset RPC client
edge-api → llm HTTP proxy
billing → mq event → credits/notification
pkg → 第三方库
idl → thrift 基础定义
```

硬约束：

- `pkg/` 禁止 import `services/*`，避免基础设施反向依赖业务。
- `services/*` 之间禁止直接 import 对方内部 Go 包；跨服务通信必须走 IDL + Kitex RPC 或 MQ 事件。
- `services/llm/` 是明确例外：它使用 Hertz HTTP 而非 Kitex RPC，因为 SSE 流式输出属于 HTTP 协议；契约见 `idl/llm/openapi.yaml`，本地端口事实以 `deployments/config/llm.yaml` 和 `scripts/dev/services.json` 为准。
- `edge-api` 禁止直接访问 MongoDB/Redis 业务主存储；需要业务数据时调用对应 Kitex 服务。允许读取鉴权/封禁类短期 Redis guard key（当前为 `idp:banned:{userID}`）以完成请求前置拦截。
- `idl/base.thrift` 是跨服务上下文、分页、基础响应的唯一共享定义，业务 thrift 必须 include 它。

## 新增能力放置规则

- 新增跨服务通用能力：优先放 `pkg/<capability>/`，并保持独立于业务服务。
- 新增服务私有业务逻辑：放对应 `services/<service>/biz/`。
- 新增服务私有数据模型：放 `services/<service>/dal/model/`，Mongo 访问放 `services/<service>/dal/mongo/`。
- 新增缓存、锁、限流等服务私有封装：放 `services/<service>/cache/`，底层复用 `pkg/redis`。
- 新增 HTTP API：先补 IDL 和服务 handler，再在 `services/edge-api/handler/` 暴露 REST 适配。

## IDL 与生成约束

- thrift 文件按 `idl/{service}/{service}.thrift` 组织。
- 请求命名使用 `XxxReq`，响应命名使用 `XxxResp`，服务命名使用 `XxxService`。
- 字段编号从 1 连续递增；枚举必须显式保留 `UNKNOWN = 0`。
- 所有服务请求应携带 `base.BaseReq`，响应应能表达 `base.BaseResp` 或等价错误码。

## 数据与横切关注点

- 主存储默认 MongoDB，通过 `pkg/db` 的 `Client` / `Repository[T]` / 事务封装访问。
- Redis 仅作为缓存、锁、幂等、临时 state/session，不作为主数据源。
- 日志统一通过 `pkg/logger.Ctx(ctx)`，trace/user/tenant 元数据由 `pkg/middleware` 透传。
- 服务注册发现当前通过 `pkg/cloudwego` 封装 CloudWeGo 官方 etcd 扩展；不要把新的服务发现逻辑写进业务服务内部。
- OpenTelemetry 通过 `pkg/otel` 初始化，开发链路默认接入 OpenTelemetry Collector + OpenObserve；应用代码只依赖 OTLP，不直接绑定观测后端。
- 错误码统一使用 `pkg/errno`；新增错误必须落在已实现的服务区段内，历史来源见 `../progress/platform/decisions.md`。
