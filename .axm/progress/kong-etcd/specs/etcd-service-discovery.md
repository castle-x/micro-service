<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
progress-type: spec
initiative: kong-etcd
workflow-state: closed
state-updated: 2026-05-12
related:
  - ../roadmap.md
  - ../../../knowledge/services/overview.md
-->

# etcd 服务注册与发现实现记录

## 状态

已完成基础闭环。本文从“实施方案”更新为“代码事实记录”，用于后续 AI 快速理解当前 etcd 服务发现的边界。

## 验收标准

### AI 自动验收

- `make build` 通过。
- `make test-pkg` 通过。
- `cd services/{edge-api,idp,iam,model} && go test ./... -count=1` 按需通过。
- `make infra-up` 后，`docker exec platform-etcd etcdctl get --prefix micro-service` 能看到服务实例。

### 人类验收

- `make dev-start` / `make dev-restart` 能一键启动 infra、observability 和本地服务。
- 不依赖 `IDP_ADDR/IAM_ADDR/MODEL_ADDR` 静态地址变量，`edge-api` 能通过 etcd 发现 `idp`、`iam` 和 `model`。
- model SSE 路径仍走现有 HTTP proxy，服务发现只负责解析 upstream 地址，不改变流式转发语义。

## 一、目标与边界

当前真实链路：

```text
iam      --register--> etcd
idp      --register--> etcd
edge-api --register--> etcd
model    --register--> etcd

edge-api --Kitex resolver--> idp / iam
idp      --Kitex resolver--> iam
edge-api --HertzServiceResolver--> model --HTTP/SSE proxy--> model
```

本阶段不做：

- 不自研完整 `pkg/registry/etcd` lease/watch/load-balance。
- 不把 Kong upstream 改成动态 etcd 发现。
- 不把业务鉴权、JWT、RBAC 下放到 Kong。
- 不继续扩展通用组件；地基已满足当前开发。

## 二、实现方式

优先使用 CloudWeGo 官方扩展：

| 框架 | 依赖 | 封装 |
|---|---|---|
| Kitex | `github.com/kitex-contrib/registry-etcd` | `pkg/cloudwego.KitexRegistryOptions`、`pkg/cloudwego.KitexClientOptions` |
| Hertz | `github.com/hertz-contrib/registry/etcd` | `pkg/cloudwego.HertzServerOptions`、`pkg/cloudwego.NewHertzServiceResolver` |

`pkg/cloudwego` 只做配置、默认值、option 组装和错误信息封装，不实现自己的服务注册协议。

## 三、配置事实

配置结构没有 `enabled` 字段。开启方式由 `type/endpoints` 决定，默认就是本地 etcd：

```yaml
registry:
  type: "${REGISTRY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${REGISTRY_PREFIX:micro-service}"
  service_name: "<service>"
  addr: "${<SERVICE>_REGISTRY_ADDR:127.0.0.1:<port>}"
  weight: 10
  tags:
    env: "${APP_ENV:local}"

discovery:
  type: "${DISCOVERY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${DISCOVERY_PREFIX:micro-service}"
```

配置落点：

| 服务 | registry | discovery |
|---|---|---|
| `edge-api` | 有，注册 Hertz `edge-api` | 有，发现 Kitex `idp/iam` 与 Hertz `model` |
| `idp` | 有，注册 Kitex `idp` | 有，发现 Kitex `iam` |
| `iam` | 有，注册 Kitex `iam` | 无 |
| `model` | 有，注册 Hertz `model` | 无 |

## 四、服务名与地址

| 服务 | 框架 | 注册名 | 默认注册地址 |
|---|---|---|---|
| `edge-api` | Hertz | `edge-api` | `127.0.0.1:38080` |
| `idp` | Kitex | `idp` | `127.0.0.1:38081` |
| `iam` | Kitex | `iam` | `127.0.0.1:38082` |
| `model` | Hertz | `model` | `127.0.0.1:38083` |

不要把 `:38081` 这类无 host 地址注册进 etcd。裸进程开发使用 `127.0.0.1:<port>`；未来全容器运行时要改为容器网络地址。

## 五、代码落点

| 任务 | 文件 |
|---|---|
| CloudWeGo glue | `pkg/cloudwego/config.go`、`pkg/cloudwego/registry.go` |
| edge-api 注册和发现 | `services/edge-api/main.go`、`deployments/config/edge-api.yaml` |
| idp 注册和发现 | `services/idp/main.go`、`deployments/config/idp.yaml` |
| iam 注册 | `services/iam/main.go`、`deployments/config/iam.yaml` |
| model 注册 | `services/model/main.go`、`deployments/config/model.yaml` |
| 本地启动 | `Makefile`、`.env.example`、`deployments/docker-compose.yml` |

## 六、验证命令

基础验证：

```bash
make infra-up
make dev-start
docker exec platform-etcd etcdctl get --prefix micro-service
```

代码验证：

```bash
make build
make test-pkg
cd services/edge-api && go test ./... -count=1
cd services/idp && go test ./... -count=1
cd services/iam && go test ./... -count=1
cd services/model && go test ./... -count=1
```

功能验证建议：

```bash
curl http://127.0.0.1:38080/api/v1/health
curl -X POST http://127.0.0.1:38080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@platform.com","password":"Admin@1234"}'
```

失败时先看：

```bash
tail -n 100 bin/log/edge-api.log
tail -n 100 bin/log/idp.log
tail -n 100 bin/log/iam.log
tail -n 100 bin/log/model.log
```

## 七、后续说明

etcd 能提供“当前注册实例”事实，但不是完整拓扑观测平台。服务节点实时状态拓扑需要把以下信息聚合：

- etcd 注册 key：有哪些实例声明自己在线。
- health endpoint：实例是否真的可用。
- OTel trace：近期真实调用关系和错误率。
- 进程/容器状态：本机或部署平台的运行状态。

本分支只完成 etcd 注册发现地基，不实现拓扑 UI。
