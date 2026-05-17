<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: platform
related:
  - ../../../knowledge/services/overview.md
  - ../../../knowledge/services/auth-sequence.md
-->

# Phase 03 · idp + iam + Google 登录端到端

## 验收标准

### AI 自动验收

- `make test` 通过 idp/iam 相关单元测试。
- `make build` 可编译 edge-api / idp / iam。
- `scripts/e2e-google-auth.sh` 在真实 Google OAuth2 凭据下可执行端到端冒烟。

### 人类验收

- 通过浏览器或 API 手动确认 Google 登录回调能签发 token，并能获取当前用户信息。

---

## 历史阶段记录

> 开始：2026-05-08　完成：2026-05-08

---

## 一、目标

在 Phase 02 就绪的 `pkg/` 基础上，实现 Google OAuth2 登录的完整链路：

```
浏览器
  └─► GET /api/v1/auth/google/url      (edge-api)
  └─► 重定向到 Google 授权页
  └─► Google 回调 /api/v1/auth/google/callback
        └─► idp: 验证 state → 换 code → 解析 id_token
              └─► iam: UpsertUserByProvider（幂等创建/更新用户）
              └─► idp: 写 identity 映射 → 签发 JWT
        └─► 返回 access_token + refresh_token
```

---

## 二、交付产物

### IDL
- `idl/idp/idp.thrift`：`IDPService`（GetGoogleAuthURL / LoginByGoogle / RefreshToken / VerifyToken）
- `idl/iam/iam.thrift`：`IAMService`（UpsertUserByProvider / GetUser）

### Kitex 生成代码
- `services/idp/kitex_gen/`：idp server + iam client
- `services/iam/kitex_gen/`：iam server
- `services/edge-api/kitex_gen/`：idp client

### iam 服务
- `dal/model/user.go`：User 文档（内嵌 `db.BaseDoc`，软删除支持）
- `dal/mongo/user.go`：UserRepo（Insert/FindByID/FindByEmail/UpdateProfile）+ email 唯一索引
- `biz/user.go`：UpsertByProvider（幂等）/ GetUser
- `handler.go`：实现 IAMService 接口
- `main.go`：Kitex server 启动，端口 `:8082`

### idp 服务
- `dal/model/user.go`：Identity + OAuthState 文档
- `dal/mongo/user.go`：IdentityRepo（FindByProvider/Upsert）+ OAuthStateRepo（Save/ConsumeAndDelete）
- `biz/oauth.go`：OAuthBiz（GetAuthURL / ExchangeCode，state 生成/验证）
- `biz/idtoken.go`：Google id_token base64 解码（不额外验签，Exchange 已保证）
- `biz/token.go`：TokenBiz（Issue / Refresh / Verify，Redis 存储 refresh JTI，黑名单）
- `biz/login.go`：LoginBiz（LoginByGoogle 主链路，调 iam RPC）
- `cache/token.go`：TokenCache（SaveRefreshToken / GetRefreshToken / DeleteRefreshToken / BlacklistAccessToken / IsBlacklisted）
- `handler.go`：实现 IDPService 接口
- `main.go`：Kitex server 启动，端口 `:8081`

### edge-api 服务
- `handler/auth.go`：AuthHandler（GetGoogleAuthURL / GoogleCallback / RefreshToken）
- `router.go`：Hertz 路由注册（/api/v1/auth/*）
- `main.go`：Hertz server 启动，端口 `:8080`

### 配置
- `deployments/config/idp.yaml`
- `deployments/config/iam.yaml`
- `deployments/config/edge-api.yaml`
- `.env.example`

### 测试
- `services/idp/biz/token_test.go`：TokenBiz 8 个用例
- `services/idp/biz/oauth_test.go`：id_token 解码 3 个用例
- `services/idp/cache/token_test.go`：TokenCache 6 个用例（含 miniredis 时间快进）
- `services/iam/biz/user_test.go`：参数校验 5 个用例
- `services/iam/dal/mongo/user_test.go`：User model 7 个用例
- `scripts/e2e-google-auth.sh`：端到端冒烟脚本（需真实 Google 凭据）

---

## 三、核心设计决策

| 决策 | 选择 | 原因 |
|------|------|------|
| idp identity 存储 | MongoDB `identities` 集合 | 与主数据同库，Phase 04 迁 Redis 接口已隔离 |
| oauth_states 过期 | MongoDB TTL 索引（10 分钟）+ 双重校验 | 无需引入额外 Redis 键，TTL 异步删除 + 业务层兜底 |
| refresh token 存储 | Redis（JTI → userID，TTL 7 天） | 可立即撤销，滚动刷新后旧 JTI 删除防重放 |
| id_token 验签 | 依赖 oauth2.Exchange 返回可信 | Exchange 只在合法 client secret 下成功，避免 JWKS 维护 |
| 跨服务通信 | Kitex RPC（idp → iam） | 遵循架构约束：服务间不直接 import，走 IDL |

---

## 四、未完成 / 延后

- `pkg/registry` etcd 真实接入（服务注册与发现，Phase 04 需要）
- RS256 JWT 升级（接口已隔离，Phase 04/05 按需）
- oauth_states 迁移到 Redis（Phase 04，消除 MongoDB TTL 异步清理的极端竞态）
- iam 用户 status 管理 API（禁用/封禁，Phase 04）

---

## 五、下一阶段建议

### Phase 04（推荐）：iam RBAC / 权限

前置就绪：
- ✅ User 主数据（iam）已可用
- ✅ Kitex RPC 链路已打通
- ✅ pkg/redis 分布式锁 + 缓存可用

交付目标：
1. `idl/iam/iam.thrift` 补充：Role / Permission / CheckPermission 接口
2. iam 服务：Role 模型、RBAC 策略、`CheckPermission`
3. Redis 缓存权限结果（`iam:perm:{user_id}` key）
4. edge-api：JWT 鉴权中间件（VerifyToken → CheckPermission）
5. 服务注册（pkg/registry etcd L2 升级）
