<!-- axm-meta
status: active
last-reviewed: 2026-05-12
owner: castlexu
depth: deep
code-refs:
  - services/edge-api/handler/auth.go
  - services/edge-api/middleware/auth.go
  - services/idp/biz/login.go
  - services/idp/biz/token.go
  - services/iam/biz/user.go
  - services/iam/biz/role.go
related:
  - ./overview.md
  - ../../project/architecture.md
-->

# 认证与权限系统交互时序图

> 覆盖 IDP（身份认证服务）和 IAM（用户与权限服务）与 Edge-API、前端 Web 之间的完整交互流程。

---

## 参与者说明

| 参与者 | 说明 |
|---|---|
| Web | 前端 React 应用（:35173） |
| Edge-API | Hertz HTTP 门面（:38080） |
| IDP | Kitex 身份认证服务（:38081） |
| IAM | Kitex 用户与权限服务（:38082） |
| MongoDB | 持久化存储 |
| Redis | Token 缓存、封禁标记、角色权限缓存 |

---

## 1. 账号密码注册

```
Web           Edge-API         IDP              IAM           MongoDB
 |                |              |                |               |
 |-- POST /auth/register ------->|              |               |
 |   {email, password, name}     |              |               |
 |                | validateEmail/Password()     |               |
 |                |              |                |               |
 |                |-UpsertUserByProvider()------->|               |
 |                |  {provider="password",        |               |
 |                |   sub=email, email}           |               |
 |                |              |-- FindByEmail()-------------->|
 |                |              |              |<-- null --------|
 |                |              |-- InsertUser(role="user")---->|
 |                |              |<-- userID, role, status ------|
 |                |<-- {userID, role="user", created=true} ------|
 |                |              |                |               |
 |                |-- bcrypt(password) ---------> |               |
 |                |-- InsertPasswordCredential() ---------->     |
 |                |              |-- Issue(userID, role) ------->|Redis|
 |                |              |  SaveRefreshToken(jti, uid, role)    |
 |                |<-- {access_token, refresh_token, expires_at}        |
 |<-- 200 {access_token, refresh_token, user_id} ----------------       |
```

---

## 2. 账号密码登录

```
Web           Edge-API         IDP              IAM           MongoDB/Redis
 |                |              |                |                |
 |-- POST /auth/login ---------->|                |                |
 |   {email, password}           |                |                |
 |                |-- FindByEmail(email) -------->|                |
 |                |<-- {userID, passwordHash} ----|                |
 |                |-- bcrypt.Compare()            |                |
 |                |              |                |                |
 |                |-- GetUser(userID) ----------->|                |
 |                |              |-- FindByID() ----------------->|
 |                |              |<-- {role, status} -------------|
 |                |<-- {role, status}             |                |
 |                |-- checkStatus(2/3 → reject)   |                |
 |                |              |                |                |
 |                |-- Issue(userID, role) ------->|                |
 |                |  SaveRefreshToken(jti,uid,role) ------------>|Redis|
 |                |<-- {access_token, refresh_token}              |
 |<-- 200 {access_token, refresh_token, user_id} ----------------|
```

---

## 3. Google OAuth2 登录

```
Web           Edge-API         IDP              IAM           Google/MongoDB/Redis
 |                |              |                |                |
 |-- GET /auth/google/url ------>|                |                |
 |                |-- GetGoogleAuthURL() -------->|                |
 |                |              |-- SaveState(state) ----------->|MongoDB|
 |                |<-- {auth_url, state} ---------|                |
 |<-- 200 {auth_url} --------------------------------             |
 |                |              |                |                |
 |-- 用户跳转 Google 授权页 ---->|                |                |
 |                |              |                |                |
 |<-- Google 回调 ?code=xxx&state=yyy            |                |
 |-- GET /auth/google/callback?code&state ------->|                |
 |                |-- LoginByGoogle(code,state) ->|                |
 |                |              |-- ConsumeState(state) -------->|MongoDB|
 |                |              |-- Google Token Exchange ------->|Google|
 |                |              |<-- {sub, email, name, avatar}--|
 |                |              |-- UpsertUserByProvider() ----->|IAM|
 |                |              |                |-- FindByEmail()|
 |                |              |                |-- Insert/Update|
 |                |              |<-- {userID, role, status} -----|
 |                |              |-- checkStatus(2/3 → reject)    |
 |                |              |-- Upsert Identity(provider,sub)|MongoDB|
 |                |              |-- Issue(userID, role) -------->|Redis|
 |                |<-- {tokens} -|                |                |
 |<-- 302 redirect /auth/callback?access_token=...               |
```

---

## 4. Token 刷新

```
Web           Edge-API         IDP                           Redis
 |                |              |                              |
 |-- POST /auth/token/refresh -->|                              |
 |   {refresh_token}             |                              |
 |                |-- Refresh(refreshToken) ------------------>|
 |                |              |-- verify JWT signature       |
 |                |              |-- GetRefreshToken(jti) ----->|
 |                |              |<-- {userID, role} -----------|
 |                |              |-- DeleteRefreshToken(jti) -->|
 |                |              |-- Issue(userID, role)        |
 |                |              |-- SaveRefreshToken(newJti) ->|
 |                |<-- {new_access_token, new_refresh_token}    |
 |<-- 200 {access_token, refresh_token, expires_at} ------------|
```

---

## 5. Token 校验（鉴权中间件）

```
Web           Edge-API(Auth Middleware)          IDP         Redis
 |                |                               |              |
 |-- 任意需登录请求 Authorization: Bearer <token> |              |
 |                |-- JWT 本地验签                |              |
 |                |   (验签失败) ---- VerifyToken RPC ---------->|
 |                |              (验签成功)        |              |
 |                |-- GetClient().Get("idp:banned:{userID}") --->|
 |                |   key 存在 → 401 banned        |              |
 |                |   key 不存在 → 注入 userID/role ctx           |
 |                |-- RequirePermission(perm)      |              |
 |                |   role == "super_admin" → bypass              |
 |                |   其他 → GetRolePermissions(role) ---------->|IAM|
 |                |          IAM 查 Redis 角色权限缓存            |
 |                |          命中 → 返回 permissions[]            |
 |                |          未命中 → 查 MongoDB → 写 Redis(TTL=5min)|
 |                |   比对 permission → 403 or 放行              |
 |                |-- 执行业务 handler                           |
 |<-- 响应 --------|                               |              |
```

---

## 6. 用户封禁

```
Web(Admin)    Edge-API         IAM           IDP           MongoDB/Redis
 |                |              |              |                |
 |-- PUT /admin/users/:id/status {status:3} -->|                |
 |   Bearer: admin_token         |              |                |
 |                |-- Auth middleware: 验签 + 检查封禁标记       |
 |                |-- RequirePermission("user:status:update")    |
 |                |-- UpdateUserStatus(targetID, banned=3) ----->|
 |                |              |-- UpdateStatus(MongoDB) ----->|MongoDB|
 |                |              |-- 检查 super_admin 保护       |
 |                |<-- ok --------|              |                |
 |                |-- RevokeUserTokens(targetID) ------------->|IDP|
 |                |              |              |-- Del refresh Set →|Redis|
 |                |-- BanUser(targetID) ------------------------>|IDP|
 |                |              |              |-- Set "idp:banned:{id}" →|Redis|
 |<-- 200 ok -----|              |              |                |
 |                |              |              |                |
 |    (被封用户持有旧 token 再次请求)            |                |
 |-- GET /api/v1/user/me  Bearer: old_token ---->|                |
 |                |-- Auth middleware: JWT 本地验签 → ok         |
 |                |-- Get("idp:banned:{userID}") → key 存在      |Redis|
 |                |-- 401 account is banned                      |
 |<-- 401 --------|              |              |                |
```

---

## 7. 角色与权限管理

```
Web(Admin)    Edge-API         IAM               Redis/MongoDB
 |                |              |                    |
 |-- POST /admin/roles {name, displayName, perms} -->|
 |                |-- RequirePermission("role:write") |
 |                |-- CreateRole(name, perms) ------->|
 |                |              |-- ExistsByCodes(perms) → MongoDB
 |                |              |-- Insert Role → MongoDB
 |                |<-- RoleItem --|                    |
 |<-- 200 role ----|              |                    |
 |                |              |                    |
 |-- PUT /admin/roles/:id {displayName, perms} ------>|
 |                |-- RequirePermission("role:write") |
 |                |-- UpdateRole(id, perms) ---------->|
 |                |              |-- UpdatePermissions → MongoDB
 |                |              |-- Delete("iam:role:perms:{name}") → Redis
 |                |<-- ok --------|                    |
 |<-- 200 ok -----|              |                    |
 |                |              |                    |
 |-- PUT /admin/users/:id/role {role} -------------->|
 |                |-- RequirePermission("user:role:assign")
 |                |-- UpdateUserRole(targetID, role) ->|
 |                |              |-- 检查角色存在 / 保护 super_admin
 |                |              |-- UpdateRole → MongoDB
 |                |<-- ok --------|                    |
 |                |-- RevokeUserTokens(targetID) → IDP → Redis 删除 refresh token
 |<-- 200 ok -----|              |                    |
```

---

## 8. 获取当前用户信息（/user/me）

```
Web           Edge-API         IDP              IAM
 |                |              |                |
 |-- GET /user/me Authorization: Bearer token -->|
 |                |-- Auth middleware: 验签 + 封禁检查
 |                |-- VerifyToken(token) -------->|
 |                |<-- {userID} ------------------|
 |                |-- GetUser(userID) ----------->|
 |                |<-- {email, name, avatar, role, status}
 |<-- 200 {user_id, email, name, avatar_url, role}
```

---

## Redis Key 设计速查

| Key 格式 | 内容 | TTL | 用途 |
|---|---|---|---|
| `idp:refresh:{jti}` | `userID\|role` | 7天 | Refresh token 有效性 |
| `idp:user:refresh:{userID}` | Set of JTIs | 7天 | 反向索引，用于批量撤销 |
| `idp:blacklist:{jti}` | `"1"` | ~1h | Access token 黑名单 |
| `idp:banned:{userID}` | `"1"` | 永久 | 用户封禁标记 |
| `iam:role:perms:{roleName}` | `perm1,perm2,...` | 5min | 角色权限缓存 |
