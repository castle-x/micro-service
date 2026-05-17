<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: kong-etcd
related:
  - ../roadmap.md
  - ../../../project/architecture.md
  - ../../../project/coding.md
  - ../../../knowledge/services/overview.md
-->

# Kong Gateway JWT and Konga Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 接入 Kong 作为本地与后续部署链路的前置 API Gateway，负责统一路由与 JWT 通用认证，并接入 Konga 作为本地只读观察面板。

**Architecture:** Kong 继续使用 DB-less declarative configuration，配置事实以 `deployments/kong/declarative.yml` 及渲染后的本地配置为准。Kong 只做入口路由、JWT 签名/时效校验、基础边缘策略；`edge-api` 继续解析用户上下文、校验封禁/黑名单、执行 IAM 权限检查和业务鉴权。Konga 连接 Kong Admin API 观察 services/routes/plugins/consumers，不作为配置写入控制面。

**Tech Stack:** Kong Gateway DB-less、Kong JWT plugin、Docker Compose、Konga、Hertz `edge-api`、Kitex `idp/iam`、Go JWT HS256 当前实现。

---

## 实施进度

| 项 | 状态 |
|---|---|
| 计划确认 | 待确认 |
| 代码实施 | 未开始 |
| AI 自动验收 | 未开始 |
| 人类验收 | 未开始 |

## 背景

当前仓库已经有 Kong 容器与 DB-less 配置骨架，但还没有形成真实网关链路：

- `deployments/docker-compose.yml` 中存在 `kong` service，挂载 `deployments/kong/declarative.yml`。
- `deployments/kong/declarative.yml` 只有静态占位 route，且 upstream 写成 `http://edge-api:8888`。
- 当前 `make dev-start` 是在宿主机启动 `edge-api :38080`，compose 网络中没有名为 `edge-api` 的容器，因此 Kong 目前无法代理到真实 `edge-api`。
- `edge-api` 当前在 `services/edge-api/middleware/auth.go` 中完成 JWT 验签、封禁检查、上下文注入和权限检查。

本阶段把 Kong 从“占位”推进到“可用的前置网关”：

```text
Client
  -> Kong :8000
  -> edge-api :38080
  -> Kitex idp/iam/asset + model HTTP proxy
```

Kong 负责通用 JWT authentication，`edge-api` 继续负责业务 authorization。

## 目标

- Kong proxy `http://localhost:8000` 能稳定代理到本地 `edge-api`。
- Kong 将认证公开路由与受保护业务路由拆开。
- 受保护路由必须先经过 Kong JWT plugin。
- 现有 `edge-api` 鉴权中间件保留，继续做业务级用户状态、token 黑名单、role/permission 检查。
- Konga 在 `http://localhost:1337` 可打开，并能观察 Kong services、routes、plugins、consumers。
- 文档明确 Konga 在 DB-less 模式下只用于观察，不用于修改配置。

## 非目标

- 不接入 Kong AI Gateway。
- 不把 RBAC/ABAC、封禁、token blacklist、租户权限下放到 Kong。
- 不在本阶段实现 Kong 动态 etcd upstream 或自研 Kong 控制面。
- 不把真实用户一一同步为 Kong Consumer；Kong Consumer 表示 token issuer 或客户端身份，真实用户身份仍在 JWT claims 和 `edge-api/idp/iam` 中处理。
- 不在本阶段升级到 OIDC、JWKS 自动轮换或 RS256；RS256 作为后续安全增强独立拆分。
- 不用 Konga 写入 Kong 配置；DB-less 配置必须经 Git 与 declarative config 变更。

## 关键决策

| 决策 | 结论 | 原因 |
|---|---|---|
| Kong 配置模式 | 保持 DB-less | 当前 compose 已使用 DB-less，适合本地和 GitOps 风格；配置变更可审查。 |
| 本地 upstream | 第一阶段使用 `host.docker.internal:38080` | 当前服务由宿主机脚本启动，不在 compose 网络内。 |
| 认证边界 | Kong 做 authn，`edge-api` 做 authz | Kong JWT plugin 可做签名和时效校验，但不理解项目封禁、黑名单和 IAM 权限。 |
| JWT issuer | 短期沿用 `iss=idp` | 当前 `pkg/jwt` 签发器已设置 issuer 为 `idp`，可作为 Kong JWT credential key。 |
| JWT secret 注入 | 不把真实 `JWT_SECRET` 提交到 Git | 需要用本地渲染文件或等价机制生成 `*.local.yaml`，该模式已被 `.gitignore` 覆盖。 |
| Konga 定位 | 本地观察面板 | Konga 仓库已归档，且 DB-less Admin API 写操作受限；只用于人类可视化检查。 |

## 文件结构

| 文件 | 操作 | 职责 |
|---|---|---|
| `deployments/kong/declarative.yml` | 修改 | 提供无敏感值的声明式配置模板或可审查基线。 |
| `deployments/kong/declarative.local.yaml` | 生成，不提交 | 注入本地 `JWT_SECRET` 后供 Kong 容器加载。 |
| `scripts/dev/render-kong-config.sh` | 新增 | 从 env 文件读取 `JWT_SECRET`，生成 `declarative.local.yaml`。 |
| `deployments/docker-compose.yml` | 修改 | 修正 Kong 配置挂载，增加 Konga service 和本地端口。 |
| `deployments/env/infra.env.example` | 修改 | 记录 Kong/Konga 本地端口与可选开关。 |
| `deployments/env/README.md` | 修改 | 说明 Konga 只读观察、Kong DB-less 配置来源。 |
| `Makefile` | 修改 | 在 `infra-up` 输出 Kong proxy/admin 与 Konga URL，必要时先渲染 Kong 配置。 |
| `scripts/dev/check-env.sh` | 按需修改 | 确保 `JWT_SECRET` 缺失时阻止启动需要 JWT 的 Kong 配置。 |

## 设计细节

### 路由拆分

Kong 只暴露明确路径，不保留无鉴权 `/` catch-all 作为业务入口。

| Kong route | Upstream | JWT plugin | 用途 |
|---|---|---|---|
| `/api/v1/auth` | `edge-api` | 否 | 登录、注册、OAuth 回调、refresh、logout。 |
| `/api/v1/user` | `edge-api` | 是 | 登录后用户接口。 |
| `/api/v1/assets` | `edge-api` | 是 | 资产库接口。 |
| `/api/v1/admin/models/chat/stream` | `edge-api` stream service | 是 | SSE 流式模型代理，单独设置长超时。 |
| `/api/v1/admin` | `edge-api` | 是 | 管理后台与模型非流式代理。 |

### JWT plugin 配置

当前 access token 由 `idp` 以 HS256 签发，claims 中包含 `iss=idp`、`exp`、`iat`、`jti`、`user_id`、`role`。Kong JWT credential 使用 `key=idp` 匹配 `iss`：

```yaml
consumers:
  - username: platform-idp
    custom_id: platform-idp
    jwt_secrets:
      - key: idp
        algorithm: HS256
        secret: "<rendered from JWT_SECRET>"
```

受保护 route 挂载：

```yaml
plugins:
  - name: jwt
    config:
      header_names:
        - authorization
      key_claim_name: iss
      claims_to_verify:
        - exp
        - nbf
      run_on_preflight: false
```

`run_on_preflight: false` 是必须项；否则浏览器 CORS preflight 没有 Bearer token 时会被 Kong 拒绝。

### SSE 超时

`/api/v1/admin/models/chat/stream` 必须使用独立 service 或独立 timeout 配置：

```yaml
services:
  - name: edge-api-stream
    url: http://host.docker.internal:38080
    connect_timeout: 60000
    read_timeout: 300000
    write_timeout: 300000
```

不要给 SSE route 添加会聚合、缓存、压缩或改写 response body 的插件。

### Konga 接入约束

Konga 连接地址使用 compose 网络内的 Kong Admin API：

```text
http://kong:8001
```

本地访问地址：

```text
http://localhost:1337
```

Konga 只用于查看：

- Services
- Routes
- Plugins
- Consumers
- JWT credentials

Konga 不用于新增、修改、删除 Kong 配置。DB-less 模式下 Kong Admin API 对实体 CRUD 写操作会返回受限结果，配置变更必须改 declarative config 后重启或 reload Kong。

## 任务计划

### Task 1: 渲染 Kong 本地配置

**Files:**

- Create: `scripts/dev/render-kong-config.sh`
- Modify: `deployments/kong/declarative.yml`
- Generate: `deployments/kong/declarative.local.yaml`

- [ ] **Step 1: 新增渲染脚本**

脚本读取现有 env 加载逻辑，并拒绝使用占位 `JWT_SECRET`：

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

load_env_files

if [ -z "${JWT_SECRET:-}" ]; then
  die "JWT_SECRET is required to render Kong declarative config"
fi
if [ "${#JWT_SECRET}" -lt 32 ]; then
  die "JWT_SECRET must be at least 32 bytes"
fi
case "${JWT_SECRET}" in
  your-*|change-me-*|replace-with-*)
    die "JWT_SECRET still uses a placeholder value"
    ;;
esac

src="${REPO_ROOT}/deployments/kong/declarative.yml"
dst="${REPO_ROOT}/deployments/kong/declarative.local.yaml"

KONG_JWT_SECRET="${JWT_SECRET}" python3 - "$src" "$dst" <<'PY'
import os
import pathlib
import sys

src = pathlib.Path(sys.argv[1])
dst = pathlib.Path(sys.argv[2])
secret = os.environ["KONG_JWT_SECRET"]

raw = src.read_text(encoding="utf-8")
if "__JWT_SECRET__" not in raw:
    raise SystemExit("missing __JWT_SECRET__ placeholder")

rendered = raw.replace("__JWT_SECRET__", secret)
if "__JWT_SECRET__" in rendered:
    raise SystemExit("failed to replace __JWT_SECRET__")

dst.write_text(rendered, encoding="utf-8")
dst.chmod(0o600)
PY

printf 'rendered %s\n' "${dst}"
```

- [ ] **Step 2: 让声明式配置保留 secret placeholder**

`deployments/kong/declarative.yml` 中只允许出现：

```yaml
secret: "__JWT_SECRET__"
```

不能出现真实开发或生产 secret。

- [ ] **Step 3: 运行脚本验证生成**

Run:

```bash
bash scripts/dev/render-kong-config.sh
test -f deployments/kong/declarative.local.yaml
grep -q "__JWT_SECRET__" deployments/kong/declarative.local.yaml && exit 1 || true
```

Expected:

```text
rendered /Users/castlexu/github/micro-service/deployments/kong/declarative.local.yaml
```

并且 `declarative.local.yaml` 不包含 `__JWT_SECRET__`。

### Task 2: 配置 Kong routes、services、consumers 和 JWT plugin

**Files:**

- Modify: `deployments/kong/declarative.yml`

- [ ] **Step 1: 写入 edge-api service**

本地裸进程开发使用宿主机 upstream：

```yaml
services:
  - name: edge-api
    url: http://host.docker.internal:38080
    connect_timeout: 60000
    read_timeout: 150000
    write_timeout: 150000
    routes:
      - name: edge-api-auth-public
        paths:
          - /api/v1/auth
        protocols:
          - http
          - https
        strip_path: false
      - name: edge-api-user-protected
        paths:
          - /api/v1/user
        protocols:
          - http
          - https
        strip_path: false
        plugins:
          - name: jwt
            config:
              header_names:
                - authorization
              key_claim_name: iss
              claims_to_verify:
                - exp
                - nbf
              run_on_preflight: false
      - name: edge-api-assets-protected
        paths:
          - /api/v1/assets
        protocols:
          - http
          - https
        strip_path: false
        plugins:
          - name: jwt
            config:
              header_names:
                - authorization
              key_claim_name: iss
              claims_to_verify:
                - exp
                - nbf
              run_on_preflight: false
      - name: edge-api-admin-protected
        paths:
          - /api/v1/admin
        protocols:
          - http
          - https
        strip_path: false
        plugins:
          - name: jwt
            config:
              header_names:
                - authorization
              key_claim_name: iss
              claims_to_verify:
                - exp
                - nbf
              run_on_preflight: false
```

- [ ] **Step 2: 写入 SSE 专用 service**

将 stream route 放在更长路径上，避免 60s 默认 upstream read timeout：

```yaml
  - name: edge-api-stream
    url: http://host.docker.internal:38080
    connect_timeout: 60000
    read_timeout: 300000
    write_timeout: 300000
    routes:
      - name: edge-api-admin-model-stream-protected
        paths:
          - /api/v1/admin/models/chat/stream
        protocols:
          - http
          - https
        strip_path: false
        plugins:
          - name: jwt
            config:
              header_names:
                - authorization
              key_claim_name: iss
              claims_to_verify:
                - exp
                - nbf
              run_on_preflight: false
```

- [ ] **Step 3: 写入 Consumer 和 JWT credential**

```yaml
consumers:
  - username: platform-idp
    custom_id: platform-idp
    jwt_secrets:
      - key: idp
        algorithm: HS256
        secret: "__JWT_SECRET__"
```

- [ ] **Step 4: 检查 Kong 配置语法**

Run:

```bash
bash scripts/dev/render-kong-config.sh
docker run --rm \
  -e KONG_DATABASE=off \
  -v "$PWD/deployments/kong/declarative.local.yaml:/tmp/kong.yml:ro" \
  kong:3.9.1 kong config parse /tmp/kong.yml
```

Expected:

```text
valid declarative configuration
```

如果 `kong:3.9.1` 镜像不可用，则使用项目当前 `kong:3.7` 跑同一条 `kong config parse`，并在实施记录中写明版本差异。

### Task 3: 接入 Konga 本地观察面板

**Files:**

- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: 将 Kong 挂载改为本地渲染文件**

Kong service 使用生成文件：

```yaml
kong:
  image: kong:3.9.1
  container_name: platform-kong
  environment:
    KONG_DATABASE: "off"
    KONG_DECLARATIVE_CONFIG: /etc/kong/declarative.local.yaml
    KONG_PROXY_ACCESS_LOG: /dev/stdout
    KONG_ADMIN_ACCESS_LOG: /dev/stdout
    KONG_PROXY_ERROR_LOG: /dev/stderr
    KONG_ADMIN_ERROR_LOG: /dev/stderr
    KONG_ADMIN_LISTEN: 0.0.0.0:8001
  volumes:
    - ./kong/declarative.local.yaml:/etc/kong/declarative.local.yaml:ro
  ports:
    - "8000:8000"
    - "8001:8001"
  restart: unless-stopped
```

- [ ] **Step 2: 新增 Konga service**

本地开发允许 `NO_AUTH=true`，因为只绑定本机端口且不作为生产控制面：

```yaml
konga:
  image: pantsel/konga:0.14.9
  container_name: platform-konga
  depends_on:
    - kong
  environment:
    NODE_ENV: production
    TOKEN_SECRET: "local-konga-token-secret-change-me"
    NO_AUTH: "true"
  ports:
    - "1337:1337"
  volumes:
    - konga_data:/app/kongadata
  restart: unless-stopped
```

并在 `volumes` 中加入：

```yaml
konga_data:
```

- [ ] **Step 3: 验证容器配置**

Run:

```bash
bash scripts/dev/render-kong-config.sh
docker compose -f deployments/docker-compose.yml config >/tmp/micro-service-compose.yml
```

Expected: command exits 0。

### Task 4: 调整本地启动提示和环境校验

**Files:**

- Modify: `Makefile`
- Modify: `scripts/dev/check-env.sh`
- Modify: `deployments/env/README.md`
- Modify: `deployments/env/infra.env.example`

- [ ] **Step 1: 让 infra-up 先渲染 Kong 配置**

`infra-up` 在 `docker compose up -d` 前执行：

```makefile
	@bash scripts/dev/render-kong-config.sh
```

- [ ] **Step 2: 补充启动输出**

`infra-up` 完成后输出：

```text
>>> Kong    : http://localhost:8000 (proxy), http://localhost:8001 (admin)
>>> Konga   : http://localhost:1337 (local observer)
```

- [ ] **Step 3: 扩展 env 文档**

`deployments/env/README.md` 必须写清：

```md
Kong DB-less loads `deployments/kong/declarative.local.yaml`, generated from
`deployments/kong/declarative.yml` and local `JWT_SECRET`. Do not commit the
generated file. Konga is a local observer for Kong Admin API; do not use it as
the source of truth for configuration changes.
```

- [ ] **Step 4: 环境校验覆盖 Kong**

`scripts/dev/check-env.sh` 已经校验 `JWT_SECRET` 时，确认错误信息能覆盖 Kong 渲染需求；若当前脚本未校验长度和占位值，则补齐：

```text
JWT_SECRET must be set, must not be placeholder, and must be at least 32 bytes.
```

### Task 5: 保持 edge-api 业务鉴权不变

**Files:**

- Inspect: `services/edge-api/router.go`
- Inspect: `services/edge-api/middleware/auth.go`

- [ ] **Step 1: 确认 protected route 仍挂 edge-api Auth**

确认以下路由组仍使用 `authMw`：

```go
user := v1.Group("/user", authMw)
assets := v1.Group("/assets", authMw)
admin := v1.Group("/admin", authMw)
```

- [ ] **Step 2: 确认业务权限仍查 IAM**

确认 admin route 仍使用：

```go
edgemw.RequirePermission("<permission>", iamCli)
```

- [ ] **Step 3: 不新增信任 Kong header 的业务鉴权**

本阶段不读取 `X-Consumer-*` 作为用户身份来源。`edge-api` 仍从 Bearer token 中解析 `user_id` 和 `role`。

### Task 6: 网关链路自动验收

**Files:**

- No production file changes

- [ ] **Step 1: 启动完整本地链路**

Run:

```bash
make dev-start
```

Expected:

```text
Backend -> http://localhost:38080
Kong    -> http://localhost:8000
Konga   -> http://localhost:1337
```

实际输出格式可不同，但必须包含 Kong/Konga 地址。

- [ ] **Step 2: 验证 Kong Admin API 可读**

Run:

```bash
curl -fsS http://localhost:8001/services | jq '.data[].name'
curl -fsS http://localhost:8001/routes | jq '.data[].name'
curl -fsS http://localhost:8001/plugins | jq '.data[].name'
curl -fsS http://localhost:8001/consumers | jq '.data[].username'
```

Expected includes:

```text
"edge-api"
"edge-api-stream"
"edge-api-auth-public"
"edge-api-user-protected"
"edge-api-assets-protected"
"edge-api-admin-protected"
"edge-api-admin-model-stream-protected"
"jwt"
"platform-idp"
```

- [ ] **Step 3: 验证公开登录路由不需要 JWT**

Run:

```bash
curl -i http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@platform.com","password":"Admin@1234"}'
```

Expected: HTTP status is not Kong `401 Unauthorized` from missing JWT. If admin user does not exist, run:

```bash
ADMIN_EMAIL=admin@platform.com ADMIN_PASSWORD='Admin@1234' ./bin/iam-bootstrap
```

Then retry login and expect `HTTP/1.1 200` with `data.access_token`.

- [ ] **Step 4: 验证受保护路由无 token 被 Kong 拒绝**

Run:

```bash
curl -i http://localhost:8000/api/v1/user/me
```

Expected:

```text
HTTP/1.1 401
```

Response should be Kong JWT plugin 的未认证响应，而不是 `edge-api` 业务响应。

- [ ] **Step 5: 验证受保护路由有 token 后进入 edge-api**

Run:

```bash
TOKEN="$(
  curl -fsS http://localhost:8000/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"admin@platform.com","password":"Admin@1234"}' \
  | jq -r '.data.access_token'
)"

curl -i http://localhost:8000/api/v1/user/me \
  -H "Authorization: Bearer ${TOKEN}"
```

Expected:

```text
HTTP/1.1 200
```

并且响应来自 `edge-api` 用户接口。

- [ ] **Step 6: 验证业务鉴权仍由 edge-api 处理**

Run:

```bash
curl -i http://localhost:8000/api/v1/admin/users \
  -H "Authorization: Bearer ${TOKEN}"
```

Expected:

- `super_admin` token 返回 `HTTP/1.1 200`。
- 非管理员 token 返回 `HTTP/1.1 403`，响应 message 来自 `edge-api` 权限检查。

### Task 7: Konga 人类验收

**Files:**

- No production file changes

- [ ] **Step 1: 打开 Konga**

Open:

```text
http://localhost:1337
```

Expected: Konga UI loads.

- [ ] **Step 2: 连接 Kong Admin API**

在 Konga 中添加 Kong node：

```text
Name: local-kong
Kong Admin URL: http://kong:8001
```

Expected: Konga 能显示 local-kong 节点状态。

- [ ] **Step 3: 查看配置对象**

在 Konga UI 中确认能看到：

- `edge-api`
- `edge-api-stream`
- protected routes
- `jwt` plugins
- `platform-idp` consumer

- [ ] **Step 4: 验证 Konga 不作为写入面**

尝试在 Konga 修改 route 或 plugin 时，如果 UI 报错或 Kong 返回 405，验收结果记录为符合预期：

```text
Konga can observe DB-less Kong config, but writes are not supported and must not be used.
```

## AI 自动验收

实施完成后必须运行：

```bash
bash scripts/dev/render-kong-config.sh
docker run --rm \
  -e KONG_DATABASE=off \
  -v "$PWD/deployments/kong/declarative.local.yaml:/tmp/kong.yml:ro" \
  kong:3.9.1 kong config parse /tmp/kong.yml
docker compose -f deployments/docker-compose.yml config >/tmp/micro-service-compose.yml
make build
make test-pkg
make dev-start
curl -fsS http://localhost:8001/services | jq '.data | length'
curl -i http://localhost:8000/api/v1/user/me
```

判定标准：

- 渲染脚本退出码为 0。
- Kong declarative config parse 通过。
- Docker Compose config 通过。
- `make build` 通过。
- `make test-pkg` 通过。
- `make dev-start` 通过。
- Kong Admin API 能读取 services。
- 未带 token 访问 `/api/v1/user/me` 返回 `401`。

## 人类验收

- 浏览器访问 `http://localhost:1337` 能打开 Konga。
- Konga 连接 `http://kong:8001` 后能看到 Kong services、routes、plugins、consumers。
- 浏览器或 curl 通过 `http://localhost:8000/api/v1/auth/login` 能登录。
- 未登录访问 `http://localhost:8000/api/v1/user/me` 被 Kong 拒绝。
- 登录后访问 `http://localhost:8000/api/v1/user/me` 成功进入 `edge-api`。
- 管理后台权限仍由 `edge-api`/IAM 判定，非管理员不会因为通过 Kong JWT 就获得管理权限。
- 人类确认 Konga 仅观察，不通过 UI 改配置。

## 风险与处理

| 风险 | 影响 | 处理 |
|---|---|---|
| Konga 仓库已归档 | 可能与 Kong 3.x 部分 API/schema 不兼容 | 使用固定镜像 `pantsel/konga:0.14.9` 作为本地观察面板；若无法稳定读取，记录为 deferred，不阻塞 Kong 网关接入。 |
| DB-less Admin API 写操作受限 | Konga 无法创建/修改配置 | 配置事实坚持 Git + declarative config；Konga 写失败视为符合预期。 |
| HS256 secret 进入 Kong | Kong 需要持有共享密钥 | 仅本阶段沿用；后续拆 RS256/JWKS spec，让 Kong 只持公钥。 |
| 本地 edge-api 仍暴露 38080 | 开发机上可绕过 Kong | 本阶段接受；生产或全容器环境必须让 edge-api 只在内网可达。 |
| CORS preflight 被 JWT 拦截 | 浏览器无法访问 protected API | JWT plugin 必须配置 `run_on_preflight: false`，CORS 仍由 `edge-api` 处理。 |
| SSE 超时 | 长模型流式响应中断 | Stream route 使用单独 service timeout，不叠 response 改写类插件。 |

## 后续增强

- 拆分 RS256/JWKS spec：`idp` 私钥签发，Kong 公钥验签，`edge-api` 同步支持 RS256 verifier。
- 全容器 dev 链路：`edge-api`、`idp`、`iam`、`asset`、`model` 进入 compose 网络后，把 Kong upstream 从 `host.docker.internal` 改为服务名。
- 网关级限流：在受保护 route 加 `rate-limiting`，但不替代 credits/billing。
- 网关访问日志与 trace correlation：把 Kong access log 与 OpenObserve 查询入口串起来。

## 参考资料

- Kong Gateway DB-less mode: https://developer.konghq.com/gateway/db-less-mode/
- Kong JWT plugin: https://developer.konghq.com/plugins/jwt/
- Kong JWT configuration reference: https://docs.konghq.com/hub/kong-inc/jwt/configuration/
- Konga repository: https://github.com/pantsel/konga
- Konga DB-less limitation example: https://github.com/pantsel/konga/issues/676
