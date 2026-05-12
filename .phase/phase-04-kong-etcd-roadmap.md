# Phase 04 · Kong + etcd 接入总体路线图

> **状态**：🧭 Roadmap / 待实施  
> **整理时间**：2026-05-11  
> **定位**：记录 Kong 网关与 etcd 服务发现的接入路线、边界和验收标准。本文是实施前的总体规划，不代表功能已完成。

---

## 一、背景与现状

项目架构目标一直是：

```text
Client -> Kong -> edge-api(Hertz) -> Kitex RPC services -> MongoDB/Redis/etcd/NSQ
```

但当前 `make dev-restart` 的真实开发链路仍是轻量直连模式：

```text
Browser -> Vite(:35173) -> edge-api(:38080)
edge-api -> idp(:38081)     # 静态 HostPorts
edge-api -> iam(:38082)     # 静态 HostPorts
edge-api -> model(:38083)   # 静态 HTTP proxy
```

当前事实：

- `deployments/docker-compose.dev.yml` 只启动 MongoDB + Redis，不包含 Kong / etcd。
- `deployments/docker-compose.yml` 有 Kong / etcd / NSQ，但未纳入 `make dev-restart` 默认链路。
- `deployments/kong/declarative.yml` 仍是占位配置，只声明了一个 edge-api 路由。
- `pkg/registry` 和 `pkg/registry/etcd` 目前是 L1 skeleton，真实 `Register` / `Resolve` 尚未实现。
- `edge-api` 当前通过 `client.WithHostPorts(...)` 调用 idp / iam，通过 `MODEL_ADDR` 直连 model service。

因此，本路线图的核心目标是把“架构目标”落到可运行、可验证、可渐进迁移的开发链路里。

---

## 二、目标

### 2.1 短期目标

1. 本地开发可通过 Kong 作为统一 API 入口。
2. idp / iam 等 Kitex 服务可注册到 etcd。
3. edge-api 可通过 etcd resolver 发现 idp / iam，而不是硬编码 `127.0.0.1:port`。
4. 保持当前轻量开发模式可用，避免所有开发任务都强制启动完整基础设施。
5. SSE 流式接口经 Kong / edge-api / model 后仍能正常实时输出，不出现缓冲或丢帧。

### 2.2 非目标

- 本阶段不把业务鉴权逻辑迁到 Kong；JWT 校验与权限判断仍由 edge-api/idp/iam 负责。
- 本阶段不强制把 model HTTP service 纳入 etcd 服务发现。
- 本阶段不接入生产级 TLS、证书自动续期、外部负载均衡或 K8s Ingress。
- 本阶段不一次性完成 billing / credits / notification 的完整业务 RPC 链路。

---

## 三、推荐方案

### 3.1 服务发现先走 Kitex 官方 registry/resolver

优先使用 Kitex 官方 etcd registry/resolver 接入 idp / iam：

- 服务端：Kitex server 启动时注册服务名、实例地址和 TTL。
- 调用端：edge-api 使用 resolver 按服务名发现实例。
- 配置：通过 `deployments/config/*.yaml` 和环境变量控制是否启用 registry。

原因：

- idp / iam 是 Kitex 服务，Kitex 官方组件能直接覆盖注册、发现、负载均衡和实例变化。
- 比先自研完整 `pkg/registry/etcd` 风险更低。
- 不阻塞未来统一抽象；后续仍可把 `pkg/registry` 升级为对 Kitex 和 HTTP service 都可用的薄封装。

### 3.2 Kong 先做入口代理，不做复杂策略

Kong 第一阶段只负责：

- `/api/*` 转发到 edge-api。
- 保持 CORS / preflight 可用。
- 保持 SSE 响应不被缓冲。

暂不启用：

- Kong JWT 插件。
- rate-limiting 插件。
- consumer 管理。
- 多 upstream 负载均衡。

原因：

- 当前鉴权和权限逻辑仍在 edge-api/idp/iam，职责迁移过早会制造双重鉴权。
- 先让网关成为稳定入口，再逐步增加边缘能力更容易验证和回滚。

---

## 四、分阶段路线

### Stage 1：基础设施入口分层

目标：区分轻量开发和完整基础设施开发。

交付：

- 保留 `make infra-up`：MongoDB + Redis。
- 新增或明确 `make infra-full-up`：MongoDB + Redis + etcd + NSQ + Kong。
- 新增或明确 `make infra-full-down` / `make infra-full-ps`。
- 文档说明：
  - 轻量链路用于普通业务开发。
  - full 链路用于服务发现、MQ、网关相关开发。

验收：

```text
make infra-up        # 只启动 MongoDB + Redis
make infra-full-up   # 启动 MongoDB + Redis + etcd + NSQ + Kong
```

### Stage 2：Kitex 服务注册到 etcd

目标：idp / iam 启动后自动注册到 etcd。

交付：

- idp main 初始化 etcd registry。
- iam main 初始化 etcd registry。
- 配置支持：
  - `registry.enabled`
  - `registry.type`
  - `registry.endpoints`
  - `registry.service_name`
  - `registry.addr`
- 默认仍允许静态端口启动，避免无 etcd 时服务无法本地调试。

验收：

```text
make infra-full-up
make dev-restart
etcdctl get --prefix /micro-service/
```

能看到 idp / iam 的服务实例注册信息。

### Stage 3：edge-api 使用 etcd resolver 调 idp / iam

目标：edge-api 不再依赖硬编码 HostPorts 调用核心 Kitex 服务。

交付：

- edge-api 配置支持：
  - `discovery.enabled`
  - `discovery.type`
  - `discovery.endpoints`
- discovery 开启时：
  - idp client 使用服务名 `idp` + etcd resolver。
  - iam client 使用服务名 `iam` + etcd resolver。
- discovery 关闭时：
  - 保持当前 `client.WithHostPorts(idpAddr/iamAddr)` 行为。

验收：

```text
IDP_ADDR / IAM_ADDR 不配置
discovery.enabled=true
edge-api 登录 / 用户接口仍可用
```

### Stage 4：Kong 纳入本地入口

目标：本地浏览器可通过 Kong 访问后端 API。

交付：

- 完善 `deployments/kong/declarative.yml`：
  - service: edge-api
  - route: `/api`
  - CORS plugin
  - SSE 相关响应头透传
- Vite proxy 可配置目标：
  - 默认仍指向 `http://localhost:38080`
  - gateway 模式指向 `http://localhost:8000`
- 增加 dev 文档说明。

验收：

```text
curl http://localhost:8000/api/v1/health
curl http://localhost:8000/api/v1/admin/models/providers
```

从 Kong 入口访问 API 成功。

### Stage 5：E2E 验证切到 Kong 入口

目标：关键用户路径在 Kong 入口下可验证。

交付：

- Playwright E2E 增加 gateway baseURL 或独立 project。
- 覆盖：
  - 登录。
  - 用户信息读取。
  - chatdebug SSE。
  - chatdebug token usage done 事件。

验收：

```text
npm run e2e -- tests/e2e/chat-debug-usage.spec.ts
```

在 Kong 入口模式下通过，且 SSE 最终事件包含 token usage。

### Stage 6：后续扩展到 HTTP service / pkg registry

目标：在 Kitex 链路稳定后，再决定是否把 model HTTP service 也纳入服务发现。

可选方向：

- 让 model service 注册到 etcd，edge-api 的 `ModelProxy` 支持 resolver。
- 完善 `pkg/registry/etcd` 为 HTTP 服务发现提供通用能力。
- 给 Kong upstream 接入服务发现或通过部署层注入 upstream。

延后原因：

- model 是 Hertz HTTP + SSE，不是 Kitex RPC。
- SSE 对代理缓冲更敏感，应在 Kong 基础链路稳定后单独验证。

---

## 五、配置原则

### 5.1 默认兼容轻量开发

任何新配置都应有保守默认值：

```yaml
registry:
  enabled: false

discovery:
  enabled: false
```

这样无 etcd 时仍能使用当前静态端口开发链路。

### 5.2 full 模式显式启用

full infra 模式通过配置或环境变量显式打开：

```yaml
registry:
  enabled: true
  type: "etcd"
  endpoints:
    - "localhost:2379"

discovery:
  enabled: true
  type: "etcd"
  endpoints:
    - "localhost:2379"
```

### 5.3 服务名稳定

服务发现使用稳定服务名：

| 服务 | service name |
|---|---|
| idp | `idp` |
| iam | `iam` |
| billing | `billing` |
| credits | `credits` |
| notification | `notification` |

---

## 六、风险与注意事项

| 风险 | 说明 | 缓解 |
|---|---|---|
| etcd 不可用导致服务启动失败 | 如果 registry 强制开启，服务启动依赖 etcd | 默认关闭；full 模式才开启 |
| 本机注册地址错误 | Kitex 注册 `:38081` 可能无法被其他进程解析 | 配置显式 `registry.addr=127.0.0.1:38081` |
| Kong 破坏 SSE 实时性 | 网关或上游 header 配置不当可能缓冲响应 | 专门用 chatdebug SSE E2E 验证 |
| 双重鉴权 | Kong JWT 与 edge-api JWT 同时启用会增加排查复杂度 | 第一阶段 Kong 不做 JWT |
| pkg/registry 抽象过早复杂化 | 自研 lease/watch/load-balance 成本高 | 先用 Kitex 官方 registry/resolver |

---

## 七、推荐实施顺序

```text
1. Makefile + docker compose：补 full infra 命令
2. idp / iam：接 Kitex etcd registry，默认关闭
3. edge-api：接 Kitex etcd resolver，默认关闭
4. Kong：完善 declarative route + CORS + SSE 验证
5. E2E：增加 gateway 模式验证
6. pkg/registry：按需升级为 HTTP service discovery 通用抽象
```

---

## 八、与现有 Phase 的关系

这份 roadmap 可以作为 Phase 04 的基础设施前置工作，也可以拆成独立的 “Phase 04a · service discovery + gateway”。

建议排序：

1. 先完成 Kong + etcd 接入的最小闭环。
2. 再继续 iam RBAC / 权限。
3. RBAC 完成后再考虑把 Kong JWT / rate limit 等边缘策略纳入网关。

原因：

- RBAC 会依赖 edge-api 稳定调用 iam。
- 服务发现先就位，可以避免后续每个服务都继续复制静态地址模式。
- Kong 先只做代理，等权限模型稳定后再承接更多边缘策略。
