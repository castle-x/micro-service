<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: platform
related:
  - ../../../knowledge/pkg-infra/overview.md
-->

# Phase 01 · pkg 基础设施三件套

## 验收标准

### AI 自动验收

- `cd pkg && go test ./...` 通过。
- `make test` 覆盖 pkg 相关测试并无新增失败。

### 人类验收

- 确认 logger/db/utils 的阶段记录与当前 pkg 基础设施知识库一致。

---

## 历史阶段记录

> **状态**: ✅ 已完成
> **时间**: 2026-05-07
> **范围**: `pkg/logger`、`pkg/db`、`pkg/utils` 裁剪与现代化重构
> **对应 SPEC 章节**: §7 MongoDB、§10.2 密码与 ID、§11 Logger

---

## 一、阶段目标

用户带来了自积累的三个轮子（`ablogger` / `mongodb` / `tools`），需要结合当前 Monorepo 项目裁剪、改包名、现代化重构，为首个业务服务 `idp` 打下可靠底座。

## 二、关键决策（已与用户对齐）

| # | 决策项 | 选定方案 |
|---|--------|----------|
| 1 | logger 实现 | **A**：用 zap 重写，JSON 结构化 + `Ctx(ctx)` 自动注入 trace_id，完全替换老 ablogger；默认 stdout，不做文件轮转（容器化平台统一收集） |
| 2 | mongodb 封装 | **A**：推翻重写——强类型 `FindOptions/UpdateOptions` + 泛型 `Repository[T]`，自动软删除过滤；保留 `IndexOptions` |
| 3 | tools/utils 定位 | **A**：`pkg/tools` 合并到 `pkg/utils`，按职责拆文件；删除 `pkg/tools` 目录 |
| 4 | 本阶段范围 | logger + mongodb + utils 三件套（config / redis / errno / mq / middleware 推迟） |

## 三、已完成

### 3.1 pkg/logger — zap 重写

- `logger.go`：`Init(Options) / L() / Sync()`；默认 JSON encoder 输出 stdout；dev 模式切 console encoder；`ErrorLevel` 以上自动打 stacktrace
- `Ctx(ctx) *Logger`：自动注入 `trace_id / caller / user_id / tenant_id`，为 nil ctx 安全降级
- `context.go`：私有 `ctxKey` 类型，防止外部伪造；提供 `WithTraceID/WithCaller/WithUserID/WithTenantID` 和对应读取器
- `metainfo.go`：`SetMetaInfoExtractor(fn)` 钩子，后续 Kitex metainfo 接入只需一行注册
- Printf 兼容 API（`Infof/Errorf` 等）以便老风格平滑迁移
- 单测覆盖：级别过滤、元数据注入、extractor 优先级、nil ctx、Printf API

### 3.2 pkg/utils — 合并 tools，按职责拆分

| 文件 | 职责 |
|------|------|
| `time.go` | `NowUnix / NowUnixMilli / NowUnixNano`（UTC 秒级，符合 SPEC §7 禁 `time.Time`） |
| `id.go` | `NewID()` 使用 UUID v7（时间有序，利于 MongoDB 索引） |
| `crypto.go` | `HashPassword / VerifyPassword` bcrypt cost=10 |
| `json.go` | `ToJSON / FromJSON / MustJSON / ToStableString`（稳定序列化，含嵌套 map） |
| `convert.go` | `Atoi / Atoi64 / Atof64 / Int32ToStr` 等安全转换 |
| `slice.go` | 泛型 `SliceDedup[T] / SliceContains[T] / SliceMap / SliceFilter` |
| `net.go` | `GetLocalIP / GetAllLocalIPs / GetHostname` |
| `file.go` | `IsFileExist / IsDirExist / EnsureDir` |
| `context.go` | `CheckContext` 取消检测辅助 |

**清理掉**：`tzChina="Asia/Shanghai"` 硬编码、`rand.Seed` 反模式、`SafeValue` 字符串参数反模式、XML 支持、字节序转换、`DeepCopyMap`、`Queue`、`sonic` / `boost` 依赖。

### 3.3 pkg/db — 泛型 Repository 重写

**架构**：
```
Client (InitMongo / Transaction / Ping / Close)
  └── BaseDocument (_id/created_at/updated_at/deleted_at) ← BaseDoc 嵌入
  └── Repository[T BaseDocument] (强类型 + 软删除自动注入)
       ├── FindOptions / UpdateOptions / FindAndUpdateOptions（编译期校验）
       ├── CRUD: FindOne/FindByID/Find/Count/Exists/InsertOne/InsertMany
       │         UpdateOne/UpdateMany/FindOneAndUpdate
       │         DeleteOne/DeleteMany（软删除）/ HardDeleteOne/Many
       ├── 软删除：applySoftDeleteFilter 用 $and 合并，与业务 filter 不冲突
       └── updated_at 自动注入 $set（支持 bson.D/bson.M，显式指定不覆盖）
Index: IndexOptions / CreateIndexesWithOptions / HasIndex / HasIndexesPrefixMatch
Errors: IsDuplicateKey（用 mongo.IsDuplicateKeyError）/ IsNotFound（errors.Is 支持 %w）
```

**修复的老代码 bug**：
- `Ping` 用 `context.TODO()` 绕过超时
- `MultiResult.Unmarshal` 用 `context.TODO()`
- `bson.M + interface{}` 的类型断言失败会 panic
- `IsDupKeyErr` 用字符串匹配脆弱
- `CreateShardKey` 命令拼写错误

**删除的冗余**：旧 `SingleResult/MultiResult` 包装、`FindDocsOrdered`、`QueryBuilder`、4 个 `ExampleXxx`、300+ 行索引进度监控（`MonitorIndexBuild/GetIndexBuildProgress/KillIndexBuild/GetIndexBuildStats` 等）。

### 3.4 项目结构整顿

- 删除 `pkg/logger/go.mod`、`pkg/db/go.mod`、`pkg/tools/go.mod`（合并到 `pkg/go.mod`）
- 删除 `pkg/tools/` 整个目录
- `pkg/go.mod` 直接依赖收敛到 6 个：`google/uuid` / `testify` / `mongo-driver` / `zap` / `x/crypto` / `yaml.v3`
- 所有子包 import path 统一为 `github.com/castlexu/micro-service/pkg/{logger,db,utils}`
- `go.work` 的 `use ./pkg` 已覆盖，无需改动

### 3.5 验证

```
go vet   ./...   ✅ 通过
go build ./...   ✅ 通过
go test  ./...   ✅ db / logger / utils 全通过
```

## 四、未完成 / 延后

以下 pkg 子模块目前仅有空目录（`pkg/config`、`pkg/errno`、`pkg/middleware`、`pkg/mq`、`pkg/registry`）或最小占位（`pkg/redis/lock.go` 仅骨架），**留到后续阶段**：

- **pkg/config**：viper，支持 yaml + 环境变量覆盖
- **pkg/errno**：统一 Code + Message 错误定义，对接 db.errors 与 gRPC status
- **pkg/redis**：完整 Client + 分布式锁（`bsm/redislock`）+ 幂等 key
- **pkg/mq**：NSQ 生产/消费封装
- **pkg/registry**：etcd 服务注册发现 + 配置监听
- **pkg/middleware**：Kitex/Hertz 的 trace / recover / metrics 拦截器

业务服务 `services/{idp,iam,billing,credits,edge-api,notification}` 目前全是占位，尚未实现。

## 五、下一阶段建议

### Phase 02（推荐）：idp 端到端最小链路

目标：打通 `edge-api → idp.Login(Kitex) → MongoDB → 返回 Token` 一条可运行的完整链路。

前置需要补齐的 pkg：
1. **pkg/config**（必须）—— idp 要读 MongoDB URI / JWT 密钥
2. **pkg/errno**（必须）—— 统一 gRPC 错误码返回
3. **pkg/middleware/trace**（推荐）—— 让 `logger.Ctx(ctx)` 能真正取到 trace_id

交付物：
- `services/idp/dal/mongo/user_repo.go`：基于 `db.Repository[*User]` 实现
- `services/idp/biz/login.go`：校验密码（utils.VerifyPassword）+ 签发 JWT
- `services/idp/handler.go`：实现 Kitex handler
- `services/edge-api/handler/login.go`：HTTP → Kitex 协议转换
- 本地跑通：`curl -X POST /api/v1/login -d '{...}'` 返回 JWT

### Phase 03+：iam / billing / redis / mq / 可观测性

按 SPEC 路线图推进。
