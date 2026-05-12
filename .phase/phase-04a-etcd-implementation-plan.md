# Phase 04a · etcd 服务注册与发现详细实现方案

> **状态**：方案已确认 / 待实施  
> **整理时间**：2026-05-11  
> **范围**：只覆盖 etcd 服务注册与服务发现；Kong 网关落地仍以 [`phase-04-kong-etcd-roadmap.md`](./phase-04-kong-etcd-roadmap.md) 为上层路线图。  
> **原则**：优先使用 CloudWeGo 官方扩展：Kitex 用 `github.com/kitex-contrib/registry-etcd`，Hertz 用 `github.com/hertz-contrib/registry/etcd`。

---

## 一、目标与边界

### 1.1 本方案要解决什么

当前开发链路中，服务调用仍依赖静态地址：

```text
edge-api -> idp  127.0.0.1:38081
edge-api -> iam  127.0.0.1:38082
idp      -> iam  127.0.0.1:38082
edge-api -> model 127.0.0.1:38083
```

etcd 接入后的目标链路：

```text
iam      --register--> etcd
idp      --register--> etcd
edge-api --register--> etcd   # Hertz HTTP 服务注册，供后续网关/HTTP 客户端发现
model    --register--> etcd   # Hertz HTTP 服务注册，供后续 model proxy 发现

edge-api --Kitex resolver--> idp / iam
idp      --Kitex resolver--> iam
```

### 1.2 本阶段不做什么

- 不自研完整 `pkg/registry/etcd` lease/watch/load-balance。`pkg/registry` 继续作为后续 HTTP 通用发现抽象的预留。
- 不把 Kong upstream 直接改成动态 etcd 发现。Kong DB-less 模式先保留静态 upstream，后续可评估 Kong + DNS 或控制面同步。
- 不强制所有本地开发都启动 etcd。默认仍走静态地址，full 模式显式启用 etcd。
- 不把业务鉴权、JWT、RBAC 下放到 Kong。

---

## 二、官方能力确认

CloudWeGo 官方文档给出的接入点如下：

- Kitex etcd：安装 `github.com/kitex-contrib/registry-etcd`；服务端用 `server.WithRegistry(r)` 和 `server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: ...})`；客户端用 `client.WithResolver(r)`。
- Hertz etcd：安装 `github.com/hertz-contrib/registry/etcd`；服务端用 `server.WithRegistry(r, &registry.Info{ServiceName, Addr, Weight, Tags})`；客户端可用 Hertz client + `sd.Discovery(r)`。

参考：

- [CloudWeGo Kitex etcd service discovery](https://www.cloudwego.io/docs/kitex/tutorials/third-party/service_discovery/etcd/)
- [CloudWeGo Hertz etcd service discovery](https://www.cloudwego.io/docs/hertz/tutorials/third-party/service_discovery/etcd/)

---

## 三、服务名、前缀和地址规范

### 3.1 服务名

| 服务 | 框架 | 注册名 | 当前监听 |
|---|---|---|---|
| `iam` | Kitex | `iam` | `:38082` |
| `idp` | Kitex | `idp` | `:38081` |
| `edge-api` | Hertz | `edge-api` | `:38080` |
| `model` | Hertz | `model` | `:38083` |

### 3.2 etcd key 前缀

建议统一配置前缀为：

```yaml
prefix: "micro-service"
```

实现上分两类：

- Kitex：通过 `registry-etcd` 的 `WithEtcdServicePrefix("micro-service/kitex")`。
- Hertz：若 hertz-contrib 当前版本支持 service prefix option，则使用 `micro-service/hertz`；若不支持，则先接受扩展默认 key 格式，验收时用实际 key 前缀查询。

这里不强行让 Kitex/Hertz 共用同一 key schema。原因是两个官方扩展内部 value 格式不同，统一 schema 的收益小，风险高。

### 3.3 注册地址

禁止把 `:38081` 这类无 host 地址直接注册到 etcd。注册到 etcd 的地址必须是其他进程可拨通的地址：

```yaml
registry:
  addr: "${IDP_REGISTRY_ADDR:127.0.0.1:38081}"
```

本地裸进程 full 模式：

```text
127.0.0.1:38081 / 127.0.0.1:38082 / 127.0.0.1:38080 / 127.0.0.1:38083
```

Docker Compose 全容器模式后续可改为：

```text
idp:38081 / iam:38082 / edge-api:38080 / model:38083
```

---

## 四、配置设计

### 4.1 通用配置结构

每个需要注册自己的服务都增加 `registry` 配置：

```yaml
registry:
  enabled: "${REGISTRY_ENABLED:false}"
  type: "${REGISTRY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${REGISTRY_PREFIX:micro-service}"
  service_name: "idp"
  addr: "${IDP_REGISTRY_ADDR:127.0.0.1:38081}"
  weight: 10
  tags:
    env: "${APP_ENV:local}"
```

每个需要发现依赖服务的调用方增加 `discovery` 配置：

```yaml
discovery:
  enabled: "${DISCOVERY_ENABLED:false}"
  type: "${DISCOVERY_TYPE:etcd}"
  endpoints:
    - "${ETCD_ENDPOINT:127.0.0.1:2379}"
  prefix: "${DISCOVERY_PREFIX:micro-service}"
```

### 4.2 文件落点

| 文件 | 变化 |
|---|---|
| `deployments/config/iam.yaml` | 增加 `registry` |
| `deployments/config/idp.yaml` | 增加 `registry` + `discovery` |
| `deployments/config/edge-api.yaml` | 补齐当前缺失的 `iam/model/jwt/redis` 配置，并增加 `registry` + `discovery` |
| `deployments/config/model.yaml` | 增加 `registry` |
| `.env.example` | 增加 full 模式的 `REGISTRY_ENABLED`、`DISCOVERY_ENABLED`、`ETCD_ENDPOINT`、各服务 `*_REGISTRY_ADDR` 示例 |

### 4.3 默认值策略

所有新能力默认关闭：

```yaml
registry:
  enabled: false

discovery:
  enabled: false
```

这样 `make dev-restart` 不启动 etcd 也不会失败。需要 full 模式时显式设置：

```env
REGISTRY_ENABLED=true
DISCOVERY_ENABLED=true
ETCD_ENDPOINT=127.0.0.1:2379
```

---

## 五、代码设计

### 5.1 新增 pkg 级 CloudWeGo glue 包

建议新增一个很薄的包：

```text
pkg/cloudwego/
  discovery.go
  registry.go
```

职责：

- 只封装配置结构、默认值、option 组装和错误信息。
- 不自己实现 registry/resolver。
- 不依赖任何 `services/*`。
- 为各服务减少重复的 `if enabled { ... } else { ... }` 拼装代码。

核心类型建议：

```go
package cloudwego

type RegistryConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Type        string            `mapstructure:"type"`
	Endpoints   []string          `mapstructure:"endpoints"`
	Prefix      string            `mapstructure:"prefix"`
	ServiceName string            `mapstructure:"service_name"`
	Addr        string            `mapstructure:"addr"`
	Weight      int               `mapstructure:"weight"`
	Tags        map[string]string `mapstructure:"tags"`
}

type DiscoveryConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Type      string   `mapstructure:"type"`
	Endpoints []string `mapstructure:"endpoints"`
	Prefix    string   `mapstructure:"prefix"`
}
```

建议函数边界：

```go
func KitexRegistryOptions(cfg RegistryConfig) ([]server.Option, error)
func KitexClientOptions(cfg DiscoveryConfig, staticAddr string) ([]client.Option, error)
func HertzServerOptions(cfg RegistryConfig, listenAddr string) ([]server.Option, error)
```

注意：上面是设计边界，实际实现时因 Kitex 和 Hertz 都有 `server.Option` 命名冲突，建议用别名导入：

```go
kitexserver "github.com/cloudwego/kitex/server"
hertzserver "github.com/cloudwego/hertz/pkg/app/server"
```

### 5.2 Kitex 服务端注册

`services/iam/main.go`：

当前：

```go
svr := iamservice.NewServer(handler,
	server.WithServiceAddr(tcpAddr),
	server.WithMiddleware(mwkitex.Trace()),
	server.WithMiddleware(mwkitex.Recovery()),
	server.WithMiddleware(mwkitex.Logging()),
)
```

目标：

```go
opts := []server.Option{
	server.WithServiceAddr(tcpAddr),
	server.WithMiddleware(mwkitex.Trace()),
	server.WithMiddleware(mwkitex.Recovery()),
	server.WithMiddleware(mwkitex.Logging()),
}

registryOpts, err := cloudwego.KitexRegistryOptions(cfg.Registry)
if err != nil {
	logger.L().Fatal("kitex registry init failed", zap.Error(err))
}
opts = append(opts, registryOpts...)

svr := iamservice.NewServer(handler, opts...)
```

`KitexRegistryOptions` 在 `enabled=true` 时等价于：

```go
r, err := etcd.NewEtcdRegistry(
	cfg.Endpoints,
	etcd.WithEtcdServicePrefix(cfg.Prefix+"/kitex"),
)
if err != nil {
	return nil, err
}
return []server.Option{
	server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: cfg.ServiceName}),
	server.WithRegistry(r),
}, nil
```

`services/idp/main.go` 同理注册 `idp`。

### 5.3 Kitex 客户端发现

需要改两处：

1. `services/edge-api/main.go` 发现 `idp` 和 `iam`。
2. `services/idp/main.go` 发现 `iam`。

当前：

```go
idpCli, err := idpclient.NewClient("idp", client.WithHostPorts(idpAddr))
```

目标：

```go
idpClientOpts, err := cloudwego.KitexClientOptions(cfg.Discovery, idpAddr)
if err != nil {
	logger.L().Fatal("idp resolver init failed", zap.Error(err))
}
idpCli, err := idpclient.NewClient("idp", idpClientOpts...)
```

`KitexClientOptions` 逻辑：

```go
if !cfg.Enabled {
	return []client.Option{client.WithHostPorts(staticAddr)}, nil
}
r, err := etcd.NewEtcdResolver(
	cfg.Endpoints,
	etcd.WithEtcdServicePrefix(cfg.Prefix+"/kitex"),
)
if err != nil {
	return nil, err
}
return []client.Option{client.WithResolver(r)}, nil
```

服务名仍由 `NewClient("idp", ...)` / `NewClient("iam", ...)` 传入，不额外在配置里重复。

### 5.4 Hertz 服务端注册

`services/edge-api/main.go` 当前：

```go
h := server.Default(server.WithHostPorts(addr))
```

目标：

```go
hertzOpts := []server.Option{
	server.WithHostPorts(addr),
}
registryOpts, err := cloudwego.HertzServerOptions(cfg.Registry, addr)
if err != nil {
	logger.L().Fatal("hertz registry init failed", zap.Error(err))
}
hertzOpts = append(hertzOpts, registryOpts...)

h := server.Default(hertzOpts...)
```

`HertzServerOptions` 在 `enabled=true` 时等价于：

```go
r, err := etcd.NewEtcdRegistry(cfg.Endpoints)
if err != nil {
	return nil, err
}
return []server.Option{
	server.WithRegistry(r, &registry.Info{
		ServiceName: cfg.ServiceName,
		Addr:        utils.NewNetAddr("tcp", firstNonEmpty(cfg.Addr, listenAddr)),
		Weight:      firstNonZero(cfg.Weight, 10),
		Tags:        cfg.Tags,
	}),
}, nil
```

`services/model/main.go` 同理注册 `model`。这一步只做注册，不改变 `edge-api -> model` 的 HTTP proxy 逻辑，避免一次性触碰 SSE 代理路径。

### 5.5 Hertz 客户端发现暂不接入主链路

Hertz 官方也支持 client discovery：

```go
cli, err := client.NewClient()
r, err := etcd.NewEtcdResolver([]string{"127.0.0.1:2379"})
cli.Use(sd.Discovery(r))
status, body, err := cli.Get(ctx, nil, "http://model/...", config.WithSD(true))
```

但当前 `services/edge-api/handler/model_proxy.go` 是面向 SSE 的反向代理实现，直接替换为 Hertz client 容易引入流式缓冲和 header 透传问题。因此本阶段先让 `model` 注册到 etcd，`ModelProxy` 仍用静态 `MODEL_ADDR`。后续单独做 “HTTP/SSE service discovery” 小阶段。

---

## 六、基础设施与 Makefile

### 6.1 docker compose

`deployments/docker-compose.yml` 已包含 etcd。建议补强：

```yaml
etcd:
  image: bitnami/etcd:3.5
  environment:
    ALLOW_NONE_AUTHENTICATION: "yes"
    ETCD_ADVERTISE_CLIENT_URLS: http://etcd:2379
    ETCD_LISTEN_CLIENT_URLS: http://0.0.0.0:2379
  ports:
    - "2379:2379"
```

本地裸进程访问用 `127.0.0.1:2379`，容器内部访问用 `etcd:2379`。

### 6.2 Makefile

建议新增明确目标：

```makefile
infra-full-up:
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml up -d

infra-full-down:
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml down

infra-full-ps:
	$(DOCKER_COMPOSE) -f deployments/docker-compose.yml ps
```

保留：

```makefile
infra-up       # MongoDB + Redis
dev-restart    # 默认静态地址链路
```

新增 full 模式启动建议：

```makefile
dev-full-restart: infra-full-up dev-stop build
	REGISTRY_ENABLED=true DISCOVERY_ENABLED=true ETCD_ENDPOINT=127.0.0.1:2379 ...
```

已确认把 full 模式做成单独 target，避免日常 `make dev-restart` 被 etcd 依赖污染。

---

## 七、测试与验收

### 7.1 单元测试

新增 `pkg/cloudwego` 单测，重点不连真实 etcd，只测配置分支：

| 测试 | 断言 |
|---|---|
| registry disabled | 返回空 options，不报错 |
| discovery disabled | 返回 `client.WithHostPorts` options |
| registry enabled but endpoints empty | 返回明确错误 |
| discovery enabled but endpoints empty | 返回明确错误 |
| registry enabled but service_name empty | 返回明确错误 |
| registry enabled but addr empty | 对 Hertz 返回明确错误；Kitex 允许 server addr 作为监听地址但注册地址仍建议配置 |

说明：Kitex/Hertz option 类型内部不可轻易 introspect，不强行测试 option 内容；真实注册发现用集成测试验证。

### 7.2 集成验证

启动 etcd：

```bash
make infra-full-up
```

启动服务：

```bash
REGISTRY_ENABLED=true \
DISCOVERY_ENABLED=true \
ETCD_ENDPOINT=127.0.0.1:2379 \
make dev-restart
```

检查注册 key：

```bash
docker exec platform-etcd etcdctl get --prefix micro-service
```

预期至少看到：

```text
micro-service/kitex/idp...
micro-service/kitex/iam...
```

如果 Hertz 扩展支持 prefix，也应看到：

```text
micro-service/hertz/edge-api...
micro-service/hertz/model...
```

### 7.3 功能验收

在 `DISCOVERY_ENABLED=true` 且不依赖 `IDP_ADDR/IAM_ADDR` 的情况下：

```bash
curl http://127.0.0.1:38080/api/v1/user/me
```

未登录应得到认证错误，而不是连接失败。

管理员登录路径：

```bash
curl -X POST http://127.0.0.1:38080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@platform.com","password":"Admin@1234"}'
```

预期返回 access token。若失败，优先看：

```bash
tail -n 100 bin/log/edge-api.log
tail -n 100 bin/log/idp.log
tail -n 100 bin/log/iam.log
```

### 7.4 回归测试

至少跑：

```bash
make build
make test-pkg
cd services/iam && go test ./... -count=1
cd services/idp && go test ./... -count=1
cd services/edge-api && go test ./... -count=1
cd services/model && go test ./... -count=1
```

如果本阶段同时改到 web 或 Kong 入口，再追加 Playwright E2E；纯 etcd 注册发现阶段不强制跑 web E2E。

---

## 八、实施任务拆分

### Task 1：补 full infra Makefile 入口

修改：

- `Makefile`

验收：

```bash
make infra-full-up
make infra-full-ps
make infra-full-down
```

### Task 2：新增 CloudWeGo registry/discovery glue

修改：

- `pkg/cloudwego/registry.go`
- `pkg/cloudwego/discovery.go`
- `pkg/cloudwego/registry_test.go`
- `pkg/go.mod`

依赖：

```bash
cd pkg && go get github.com/kitex-contrib/registry-etcd github.com/hertz-contrib/registry/etcd
```

说明：服务入口只 import `github.com/castlexu/micro-service/pkg/cloudwego`，不直接 import contrib 包；因此 contrib 依赖集中落在 `pkg/go.mod`，各 `services/*/go.mod` 不需要重复声明，除非执行时决定不做 `pkg/cloudwego` 薄封装。

验收：

```bash
cd pkg && go test ./cloudwego -count=1
```

### Task 3：iam 注册到 etcd

修改：

- `services/iam/main.go`
- `deployments/config/iam.yaml`

验收：

```bash
cd services/iam && go test ./... -count=1
go build -o ../../bin/iam .
```

### Task 4：idp 注册到 etcd，并通过 resolver 调 iam

修改：

- `services/idp/main.go`
- `deployments/config/idp.yaml`

验收：

```bash
cd services/idp && go test ./... -count=1
go build -o ../../bin/idp .
```

### Task 5：edge-api 通过 resolver 调 idp/iam，并注册 Hertz 服务

修改：

- `services/edge-api/main.go`
- `deployments/config/edge-api.yaml`

验收：

```bash
cd services/edge-api && go test ./... -count=1
go build -o ../../bin/edge-api .
```

### Task 6：model 注册 Hertz 服务

修改：

- `services/model/main.go`
- `deployments/config/model.yaml`

验收：

```bash
cd services/model && go test ./... -count=1
go build -o ../../bin/model .
```

### Task 7：full 模式端到端验证

命令：

```bash
make infra-full-up
REGISTRY_ENABLED=true DISCOVERY_ENABLED=true ETCD_ENDPOINT=127.0.0.1:2379 make dev-restart
docker exec platform-etcd etcdctl get --prefix micro-service
```

验收：

- etcd 中可见 `idp` / `iam` Kitex 实例。
- `edge-api` 不设置 `IDP_ADDR/IAM_ADDR` 仍能调通登录链路。
- `idp` 不设置 `IAM_ADDR` 仍能调通 `iam`。
- `model` 和 `edge-api` 可注册 Hertz 实例；当前不要求通过 Hertz resolver 调用它们。

---

## 九、已确认的实施决策

1. **新增 `pkg/cloudwego`。**  
   它不是自研 registry，只是 CloudWeGo 官方扩展的薄 glue，用来避免四个服务重复写 option 拼装。

2. **Hertz 注册本阶段覆盖 `model`。**  
   本阶段只覆盖“注册”，不覆盖“发现调用”。这样 etcd 服务目录先完整，SSE 代理风险留到下一步单独处理。

3. **新增 `make dev-full-restart`。**  
   `make dev-restart` 保持轻量静态链路，`make dev-full-restart` 才打开 etcd 注册发现。

4. **service prefix 使用 `micro-service/kitex` 与 `micro-service/hertz` 分开。**  
   避免两个扩展内部 value/schema 混在一起，后续排查 etcd key 也更直观。

---

## 十、推荐结论

推荐按下面顺序实施：

```text
Makefile full infra
-> pkg/cloudwego glue
-> iam Kitex registry
-> idp Kitex registry + iam resolver
-> edge-api idp/iam resolver + Hertz registry
-> model Hertz registry
-> full 模式集成验证
```

这条路径能先把核心 RPC 服务发现闭环打通，同时不冒进改动 SSE HTTP proxy，也不会破坏现有轻量开发体验。
