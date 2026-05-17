<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
depth: overview
code-refs:
  - pkg/go.mod
  - pkg/logger/logger.go
  - pkg/db/doc.go
  - pkg/config/config.go
  - pkg/errno/errno.go
  - pkg/redis/client.go
  - pkg/jwt/jwt.go
  - pkg/middleware/metainfo.go
  - pkg/cloudwego/config.go
  - pkg/cloudwego/registry.go
  - pkg/health/server.go
  - pkg/httpclient/client.go
  - pkg/otel/otel.go
  - pkg/mq/instrumentation.go
related:
  - ../../project/architecture.md
  - ../../project/coding.md
-->


# pkg 基础设施 — 速查

## 定位

`pkg/` 是微服务共享基础设施 module，承载日志、数据库、配置、错误码、Redis、JWT、中间件、CloudWeGo 注册发现 glue、admin health、HTTP client、OpenTelemetry 基座和 MQ 抽象。它只能依赖第三方库和自身子包，不能依赖任何 `services/*`。

## 模块清单

| 模块 | 路径 | 当前定位 |
|---|---|---|
| logger | `pkg/logger/` | zap 结构化日志封装，`logger.Ctx(ctx)` 注入 trace/user/tenant 元数据 |
| db | `pkg/db/` | MongoDB client、泛型 Repository、软删除、索引、事务封装 |
| utils | `pkg/utils/` | ID、时间、加密、JSON、文件、网络、context 等零内部依赖工具 |
| config | `pkg/config/` | viper + yaml + env + `${VAR}` 展开，支持泛型 `Load[T]` |
| errno | `pkg/errno/` | 全服务错误码体系，支持与 `pkg/db` 错误互转 |
| redis | `pkg/redis/` | go-redis v9 client、redislock 分布式锁、键名辅助 |
| jwt | `pkg/jwt/` | HS256 JWT 签发/校验接口，预留 RS256/JWKS 扩展 |
| middleware | `pkg/middleware/` | Hertz/Kitex trace、recovery、logging 以及 metainfo 透传 |
| cloudwego | `pkg/cloudwego/` | Kitex/Hertz etcd registry/resolver 的薄 glue，封装 CloudWeGo 官方扩展 option |
| health | `pkg/health/` | admin health server 与 Mongo/Redis/etcd 探活检查 |
| httpclient | `pkg/httpclient/` | 共享 HTTP client 基础封装，供需要外部 HTTP 调用的服务复用 |
| otel | `pkg/otel/` | 进程级 OpenTelemetry 初始化，提供 tracer/meter provider、OTLP exporter、resource、shutdown |
| registry | `pkg/registry/` | 通用 Registry/Resolver L1 接口骨架；当前真实 etcd 接入不走这里，而走 `pkg/cloudwego` |
| mq | `pkg/mq/` | Producer/Consumer L1 接口骨架；已有 message context/span helper，NSQ 真实收发仍是占位 |

## 关键概念

- **L2 可用包**：`logger`、`db`、`utils`、`config`、`errno`、`redis`、`jwt`、`middleware`、`cloudwego`、`health`、`httpclient`、`otel` 已具备可运行实现。
- **L1/预留包**：`registry` 仍是通用接口骨架；`mq` 的真实 NSQ publish/consume 仍未实现，但已经有 OTel message context 与 span helper。
- **etcd 服务发现**：当前分支的真实路径是 `services/* -> pkg/cloudwego -> kitex-contrib/hertz-contrib registry-etcd`，不是 `pkg/registry/etcd`。
- **trace 元数据**：`pkg/middleware` 负责 HTTP header / Kitex metainfo 透传，`pkg/otel` 安装 W3C propagator，`pkg/logger.Ctx(ctx)` 负责写入 `trace_id` / `span_id`。
- **错误码边界**：系统 10001-10999，idp 11001-11999，iam 12001-12999，billing 13001-13999，credits 14001-14999，notification 15001-15999，llm 16001-16999，asset 17001-17999。
- **当前边界**：etcd + OTel/OpenObserve 地基已够用；暂不继续扩展通用组件，后续优先跟随具体业务链路补齐。

## 修改入口

- 新增基础设施能力时，先判断是否被多个服务复用；否则留在 `services/<service>/`。
- 修改 `pkg` 后优先跑 `cd pkg && go vet ./... && go build ./... && go test ./... -count=1`。
- 新增第三方依赖后更新 `pkg/go.mod`，不要在 `pkg` 子目录再开 module。
