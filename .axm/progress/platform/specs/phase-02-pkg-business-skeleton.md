<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: platform
related:
  - ../../../knowledge/pkg-infra/overview.md
-->

# Phase 02 · pkg 业务骨架补齐

## 验收标准

### AI 自动验收

- `cd pkg && go test ./...` 通过。
- `make test` 覆盖 config/errno/redis/middleware/jwt/registry/mq 相关测试并无新增失败。

### 人类验收

- 确认 pkg 横切基础设施的阶段记录与当前代码模块边界一致。

---

## 历史阶段记录

> **状态**: ✅ 已完成
> **时间**: 2026-05-07
> **范围**: 在 Phase 01 三件套基础上，补齐后续 idp/iam/billing/credits/notification 通用的横切基础设施；本阶段不做业务逻辑、不写 IDL、不接 Google
> **对应 SPEC 章节**: §5 errno · §8 redis · §9 middleware · §10 config · §三 仓库结构

---

## 一、阶段目标

原计划 Phase 02 是"idp 端到端最小链路（Google 登录）"。用户在澄清后决定**先把业务真正所需的 pkg 基础设施骨架一次性补齐**，后续业务阶段（Phase 03 起）可以直接面向完整的 pkg 编码，不再回头改。

## 二、关键决策（已与用户对齐）

| # | 决策项 | 选定方案 |
|---|--------|----------|
| 1 | idp/iam 拆分（为 Phase 03 预留） | **正式分层**：idp + iam 并行起，idp 通过 Kitex RPC 调 iam `UpsertUser` |
| 2 | 外部身份映射归属 | **放 idp**（`identities` 集合：google_sub ↔ user_id），iam 只管用户资料 |
| 3 | 自家 JWT 算法 | **先 HS256 跑通**，Phase 03/04 升级 RS256 —— pkg/jwt 用 Signer/Verifier 接口隔离 |
| 4 | Phase 02 模块档位 | config/errno/redis/middleware/jwt 做 L2（可用）；registry/mq 做 L1（接口 + 占位） |
| 5 | 脚手架附加项 | 清理 Phase 01 遗留占位（redis lock.go / mq producer & consumer / registry etcd.go）+ 补 Makefile lint/test；不做 .env.example / docker-compose / config yaml 模板（留 Phase 03） |

## 三、已完成

### 3.1 pkg/errno —— L2 统一错误码

- `code.go`：按 SPEC §5.1 一次性定义完整区段错误码（系统 10001-10999 / IDP 11001 / IAM 12001 / Billing 13001 / Credits 14001 / Notification 15001），新增 `ErrNotImplemented=10009` / `ErrDuplicateKey=10010`
- `errno.go`：`Errno{Code, Message}` 值类型、`Error/Is/WithMessage/WithMessagef`；`FromDBError` 与 `pkg/db.IsNotFound/IsDuplicateKey` 互转；`errors.Is` 按 Code 判定
- 单测：覆盖基本 Error、按 Code 判定 Is、WithMessage*、FromDBError 四种路径（含 wrapped / 已是 Errno / 未知错误兜底）、区段边界冒烟

### 3.2 pkg/config —— L2 配置加载

- `config.go`：基于 viper，`Load[T any](path, *out)` 一站式：yaml 读入 → `os.ExpandEnv` 展开 `${VAR}` → `AutomaticEnv + SetEnvKeyReplacer(".→_")` 环境变量覆盖
- `reflect.collectKeys` 扫描 out 的 `mapstructure` tag 显式 `BindEnv`，保证**无 yaml 文件、只用 env** 的场景也能读到
- `MustLoad` 启动期 panic 版；`RequireEnv` 强校验必填环境变量
- 单测：yaml-only / env 覆盖 / `${VAR}` 展开 / 文件不存在 / 空 path / nil out / 解析错误 / RequireEnv / MustLoad panic

### 3.3 pkg/redis —— L2 客户端 + 分布式锁（重写 Phase 01 占位）

**重写清单**：
- ❌ 旧 `redis.go`（空 Client）+ `lock.go`（TODO 占位）删除
- ✅ `client.go`：`Config` 结构（Addr/Password/DB/PoolSize/MinIdleConns/超时）；`Init(cfg)` 初始化全局单例并 Ping；`InitWithClient` 供测试注入；`GetClient()` 未 Init 返回 nil（不 panic）；薄封装 Set/Get/Del/SetNX；`Key(parts...)` 按 SPEC §8.1 规范拼接
- ✅ `lock.go`：基于 `github.com/bsm/redislock`，`ObtainLock(ctx,key,ttl,opts...)` 支持可选 `LinearBackoff + LimitRetry`；Release 幂等（`ErrLockNotHeld` 吞错）；`TTL/Refresh`
- 错误归一：抢锁失败 → `ErrRateLimit`；参数非法 → `ErrInvalidParam`；其他 → `ErrInternal`
- 单测（miniredis）：Key 拼接 / Init 校验 / Set-Get-Del / Cache miss / SetNX / ObtainLock 竞争 & Release 幂等 / nil client 安全

### 3.4 pkg/jwt —— L2 HS256 实现（预留 RS256 扩展）

- `jwt.go`：`Signer/Verifier` 接口 + `Claims`（`UserID/TenantID` + 嵌入 `jwt.RegisteredClaims`）
- `NewHS256Signer(secret, ttl, issuer)`：secret **长度校验 ≥32 字节**；自动填充 iat/exp/jti（uuid v7）；issuer 可选
- `NewHS256Verifier(secret)`：Parse 时显式断言 `SigningMethodHMAC`，**拦截 alg=none / 算法切换攻击**；过期 → `ErrTokenExpired`，签名/格式错误 → `ErrTokenInvalid`
- 单测：round-trip / 空 token / malformed / 错误 secret / 过期 / alg=none 攻击 / 用户指定 jti & exp / 三段式格式冒烟

### 3.5 pkg/middleware —— L2 Kitex + Hertz 两套

**metainfo.go（共享）**：
- 常量 `MetaKey{TraceID,Caller,UserID,TenantID}`
- `RegisterLoggerExtractor()` 注册到 Phase 01 已预留的 `logger.SetMetaInfoExtractor` 钩子，让 `logger.Ctx(ctx)` 自动从 metainfo 读 trace_id
- `WithMeta` / `TraceIDFromContext` 辅助函数

**kitex 三件套**（`pkg/middleware/kitex`）：
- `Trace()` —— 服务端：若 metainfo 缺 trace_id 则生成 uuid；同步写入 logger ctx key
- `Recovery()` —— panic 记 stacktrace + 返回 `ErrInternal`
- `Logging()` —— 记录 `{service}/{method} + duration + errno.Code`，不记 req/resp body

**hertz 三件套**（`pkg/middleware/hertz`）：
- `Trace()` —— 从 `X-Trace-ID` header 取/生成；同时写 response header（便于客户端排障）+ metainfo + logger ctx
- `Recovery()` —— panic 返回 500 + `{code, message}` JSON
- `Logging()` —— 记录 `method/path/status/duration`；status≥500 用 Error，其他 Info

单测：kitex_test 用 endpoint 手写 mock；hertz_test 用 `ut.CreateUtRequestContext + SetHandlers + Next`

### 3.6 pkg/registry & pkg/mq —— L1 接口骨架

**registry**（清理旧 `etcd.go` 占位）：
- `registry.go`：`Registry{Register/Deregister/Close}` + `Resolver{Resolve/Close}` 接口；`Endpoint/ServiceInfo` 结构；`NotImplementedRegistry/Resolver` 默认实现
- `etcd/etcd.go`：`Config` 预留字段 + `NewRegistry/NewResolver` 构造；所有方法返回 `ErrNotImplemented`，**不引入 go.etcd.io/etcd 依赖**，保持 go.mod 洁净

**mq**（清理旧 `producer.go` / `consumer.go` 占位）：
- `mq.go`：`Producer{Publish/Close}` + `Consumer{Subscribe/Close}` 接口；`Message/HandlerFunc`；NotImplemented 占位
- `nsq/nsq.go`：同构，**不引入 go-nsq 依赖**

### 3.7 Makefile —— lint / test 真实命令

- `make lint`：`go vet ./pkg/... + ./services/*/...`，检测到 `golangci-lint` 则追加运行
- `make test`：拆分为 `test-pkg` + `test-services`，各自 `go test ./... -count=1`
- `make help` 列出全部可用目标

### 3.8 依赖变更（pkg/go.mod）

**新增直接依赖**（Phase 01 从 6 个 → Phase 02 共 13 个）：
- `github.com/spf13/viper v1.21.0`
- `github.com/redis/go-redis/v9 v9.19.0`
- `github.com/bsm/redislock v0.9.4`
- `github.com/alicebob/miniredis/v2 v2.37.0`（test-only）
- `github.com/golang-jwt/jwt/v5 v5.3.1`
- `github.com/cloudwego/kitex v0.16.1`
- `github.com/cloudwego/hertz v0.10.4`
- `github.com/bytedance/gopkg v0.1.4`（metainfo）

**stretchr/testify** 从 v1.10.0 顺带升级到 v1.11.1（viper 传递依赖）。

### 3.9 验证

```
cd pkg && go vet ./... && go build ./... && go test ./... -count=1
```

全部 14 个包（含 Phase 01 的 db/logger/utils）绿：

| 包 | 结果 |
|---|---|
| config / errno / jwt / redis / middleware / middleware/kitex / middleware/hertz / registry / registry/etcd / mq / mq/nsq | ok |
| db / logger / utils（Phase 01） | ok（未回归） |

`make lint` / `make test` 在仓库根目录亦通过（services 各模块目前无测试，显示 `[no test files]`）。

## 四、未完成 / 延后

- `pkg/registry` 真实 etcd 接入（Phase 03/04 业务服务需跨进程调用时）
- `pkg/mq` 真实 NSQ 接入（Phase 05 billing↔credits 事件驱动）
- `pkg/jwt` RS256 / JWKS（Phase 03/04 按需升级，接口已隔离，业务代码零改动）
- 配置样例（`deployments/config/base.yaml`、`.env.example`）—— Phase 03 起 idp 时随首个真实配置一起落地

## 五、下一阶段建议

### Phase 03（推荐）：idp + iam 端到端 + Google 登录

前置就绪情况：
- ✅ config / errno / redis / middleware / jwt 已可用
- ✅ logger 的 trace_id 只需 main.go 调 `middleware.RegisterLoggerExtractor()` + 链中放 `mwkitex.Trace()` / `mwhertz.Trace()` 即可全链路打通

交付目标（可参考以下顺序）：
1. **IDL**：补 `idl/idp/idp.thrift`（GetGoogleAuthURL / LoginByGoogle / RefreshToken / VerifyToken）与 `idl/iam/iam.thrift`（UpsertUserByProvider / GetUser）
2. **iam 服务**：User 模型 + `biz.UpsertUserByProvider` + Kitex handler + `EnsureIndex`
3. **idp 服务**：
   - `identities` 集合（`(provider, provider_sub)` 唯一索引，映射到 iam user_id）
   - `oauth_states` 集合（MongoDB TTL 索引，Phase 04 再切 Redis）
   - OAuth client：用 `golang.org/x/oauth2/google` 换 code / 验 id_token
   - 登录 biz：state 校验 → 换 token → 解析 id_token → 调 iam.UpsertUserByProvider → 签发本家 JWT（`pkg/jwt` HS256）
4. **edge-api**：`/api/v1/auth/google/url` + `/api/v1/auth/google/callback` 两个 Hertz handler
5. **配置样例**：`deployments/config/{base,idp,iam}.yaml` 模板 + `.env.example`（JWT_SECRET / GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET）
6. **本地联调**：`curl` 走通回调 → 拿到自家 access/refresh token
