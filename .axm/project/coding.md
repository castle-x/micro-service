<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
applies-to: [project:micro-service]
related:
  - ./architecture.md
  - ./observability.md
-->


# micro-service 编码规范

## 工具链命令

在仓库根目录执行：

| 场景 | 命令 |
|---|---|
| 格式化 | `make fmt` |
| 静态检查 | `make lint` |
| 全量测试 | `make test` |
| 仅测 pkg | `make test-pkg` |
| 仅测 services | `make test-services` |
| 构建服务 | `make build` |
| 生成 Kitex 代码 | `make gen` |

在 `pkg/` 内局部验证：

```bash
cd pkg && go vet ./... && go build ./... && go test ./... -count=1
```

## Go module 规则

- 根目录使用 `go.work` 串联 `idl/`、`pkg/`、`services/*`。
- 不为 `pkg` 子包新增独立 `go.mod`；`pkg` 下所有子包共用 `pkg/go.mod`。
- 每个 `services/<service>/` 是独立 module，服务之间不直接 import 内部包。
- import 路径统一使用 `github.com/castlexu/micro-service/<module>` 对应 module 路径。

## 包与命名

- Go package 名称短、全小写、无下划线。
- 文件按职责命名，例如 `client.go`、`lock.go`、`handler.go`、`model/*.go`。
- 服务内分层目录保持一致：`biz/`、`dal/model/`、`dal/mongo/`、`cache/`、`mq/`。
- 新增公共 API 前先确认是否属于 `pkg`；只被单服务使用的代码不要放入 `pkg`。

## 错误处理

- 业务错误统一使用 `pkg/errno`，禁止在业务代码中散落裸字符串错误码。
- `pkg/db` 相关错误进入业务层时通过 `errno.FromDBError` 转换。
- 不要把 password、secret、token、authorization 等敏感字段写入错误 message、metadata 或日志。

## 日志与上下文

- 日志统一使用 `logger.Ctx(ctx)`，不要直接新建 zap logger。
- Hertz/Kitex 入口必须挂载 `pkg/middleware` 的 trace/recovery/logging 中间件。
- 跨服务调用需要透传 `trace_id`、`caller`、`user_id`、`tenant_id`。
- 新增 HTTP/RPC/DB/Redis/MQ/外部 API/LLM 调用链路时，必须遵守 `observability.md` 的 OpenTelemetry trace、metrics、log correlation 规范。

## 测试要求

- `pkg` 的 L2 可用模块必须配单测；新依赖引入后需跑 `cd pkg && go mod tidy`。
- 修改 `pkg` 后至少跑 `cd pkg && go test ./... -count=1`。
- 修改服务骨架后至少跑对应 `services/<service>` 的 `go test ./... -count=1`。
- 阶段交付前跑 `make lint && make test`。

## 本地集成测试账号

涉及登录态、管理员权限、跨服务链路或日志链路验证时，优先使用以下固定测试账号：

| 字段 | 值 |
|---|---|
| Email | `admin@platform.com` |
| Password | `Admin@1234` |

本地环境若账号不存在，先运行 `ADMIN_EMAIL=admin@platform.com ADMIN_PASSWORD='Admin@1234' ./bin/iam-bootstrap` 初始化，再执行登录或管理员接口测试。
