<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: kong-etcd
related:
  - ../../project/architecture.md
  - ../../knowledge/services/overview.md
-->

# Kong + etcd 接入路线图

> **状态**：已完成并闭合 / 后续扩展按需另拆
> **整理时间**：2026-05-12
> **定位**：记录本分支实际完成的 etcd 服务注册发现与 Kong 基础入口事实。本文以代码现状为准，不再描述“待实施”的旧轻量/full 双轨方案。

## 一、当前事实

当前开发链路已经从“静态端口直连为主”调整为“默认完整地基”：

```text
make infra-up
  -> deployments/docker-compose.yml
  -> MongoDB + Redis + etcd + NSQ + Kong

make dev-start / make dev-restart
  -> infra-up + obs-up + build
  -> edge-api / idp / iam / model
  -> REGISTRY_TYPE=etcd + DISCOVERY_TYPE=etcd + OTEL_ENABLED=true
```

服务注册发现真实路径：

```text
iam      --Kitex registry--> etcd
idp      --Kitex registry--> etcd
edge-api --Hertz registry--> etcd
model    --Hertz registry--> etcd

edge-api --Kitex resolver--> idp / iam
idp      --Kitex resolver--> iam
edge-api --HertzServiceResolver + HTTP proxy--> model
```

关键代码落点：

| 范围 | 文件 |
|---|---|
| CloudWeGo glue | `pkg/cloudwego/config.go`、`pkg/cloudwego/registry.go` |
| 服务配置 | `deployments/config/{edge-api,idp,iam,model}.yaml` |
| 服务入口 | `services/{edge-api,idp,iam,model}/main.go` |
| 本地基础设施 | `deployments/docker-compose.yml`、`Makefile` |
| Kong DB-less 配置 | `deployments/kong/declarative.yml` |

## 二、已完成范围

| 阶段 | 状态 | 当前结果 |
|---|---|---|
| Stage 1：完整基础设施入口 | 已完成 | `make infra-up` 使用 `deployments/docker-compose.yml` 启动 MongoDB、Redis、etcd、NSQ、Kong |
| Stage 2：Kitex 服务注册到 etcd | 已完成 | `idp`、`iam` 通过 `pkg/cloudwego.KitexRegistryOptions` 注册 |
| Stage 3：Kitex 客户端使用 etcd resolver | 已完成 | `edge-api` 发现 `idp/iam`，`idp` 发现 `iam` |
| Stage 4：Hertz 服务注册到 etcd | 已完成 | `edge-api`、`model` 通过 `pkg/cloudwego.HertzServerOptions` 注册 |
| Stage 5：model HTTP 服务发现 | 已完成基础版 | `edge-api` 使用 `NewHertzServiceResolver` 解析 `model` 后继续走原 HTTP/SSE proxy |
| Stage 6：Kong 入口 | 已完成基础版 | Kong DB-less 配置已接入 edge-api 路由、JWT 通用认证、Konga 本地观察面板和 OpenTelemetry trace；Web dev `/api` 默认走 Kong proxy |

## 闭合记录

- 2026-05-17：用户确认 `kong-etcd` 已开发完成，本文作为已完成历史上下文保留 `active`。
- 长期事实已同步到 `../../project/architecture.md`、`../../knowledge/services/overview.md`、`../../knowledge/observability/overview.md`。
- 本 initiative 没有 `bugs/` 目录；无未关闭 BUG 文档。
- 遗留项均移动到“后续可选项”，不再阻塞当前地基闭合。

## 三、边界与非目标

- 不继续实现自研 `pkg/registry/etcd`。当前 etcd 集成只通过 CloudWeGo 官方扩展的薄封装完成。
- 不在本分支把 Kong 改成动态服务发现控制面。`deployments/kong/declarative.yml` 仍是静态 DB-less 入口事实来源。
- 不把 RBAC/ABAC、封禁、token blacklist、租户权限等业务鉴权迁到 Kong；Kong 只负责前置路由与 JWT 通用认证，业务鉴权仍由 `edge-api/idp/iam` 负责。
- 不把 billing / credits / notification 的业务调用链路纳入本分支。
- 不继续扩展通用基础设施组件；地基够用，后续跟随具体业务需求补齐。

## 四、配置事实

配置中没有 `registry.enabled` / `discovery.enabled` 字段。是否启用由 `type/endpoints` 决定：

```yaml
registry:
  type: "${REGISTRY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${REGISTRY_PREFIX:micro-service}"

discovery:
  type: "${DISCOVERY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${DISCOVERY_PREFIX:micro-service}"
```

本地服务注册地址通过 `.env.example` 中的 `*_REGISTRY_ADDR` 控制，默认是 `127.0.0.1:3808x`。裸进程开发使用本机地址；未来如果切到全容器运行，需要改为容器网络可达地址。

## 五、验收口径

已经具备的验收方向：

```bash
make infra-up
make dev-start
docker exec platform-etcd etcdctl get --prefix micro-service
```

期望能看到 `idp`、`iam`、`edge-api`、`model` 的注册信息，并且不依赖 `IDP_ADDR/IAM_ADDR/MODEL_ADDR` 静态变量完成主链路调用。

本分支暂不要求：

- Kong 动态 upstream。
- 生产级 Kong 动态 upstream / 控制面。
- etcd watch/lease 状态图。
- 服务拓扑 UI。

## 六、后续可选项

只有在后续业务或运维确实需要时再启动：

1. **Kong 入口增强**：前置路由、JWT 通用认证、Konga 观察面板、Web 经 Kong 访问后台和 Kong OTel trace 已落地；限流策略仍待后续按业务需求拆分。
2. **服务状态拓扑**：从 etcd 注册表 + health endpoint + OTel trace 汇总服务节点和调用关系。
3. **容器化服务运行**：把当前裸进程 dev-start 演进为全容器开发链路。
