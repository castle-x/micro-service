<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
workflow-state: ready
state-updated: 2026-05-17
priority: P1
related:
  - ../roadmap.md
  - ../../dev-ops/specs/env-split.md
  - ../../dev-ops/specs/health-endpoints.md
-->

# QUAL-09 配置与启动验证

## 实施进度

- 业务状态：`pending`

## 背景

很多生产事故的根因不在代码，而在**部署 + 配置**：`.env` 漏字段、Kong 路由漂移、K8s 资源限制、启动顺序错乱。本阶段提供"上线前最后一道闸"。

## 解决的根本问题

- **fail-fast 缺失**：配置错误延迟到运行时第一次调用才暴露 → 应在启动期立刻 panic
- **依赖未就绪而启动成功**：`make dev-start` 只 `sleep 2` 就报"全部就绪"，实际依赖未健康
- **manifest 漂移**：K8s/Helm/Kong 声明式配置与代码期望不一致
- **启动顺序耦合**：edge-api 在 iam 起来前调用导致冷启失败

> 边界：不测代码逻辑（其他 QUAL 覆盖）、不测性能（QUAL-13）。本阶段只测"配置能让系统正确启动到可用状态"。

## 触发条件

- 每次 `.env.example`、`docker-compose.yml`、K8s manifest、Kong config 变更：必须跑配置 smoke
- 部署到任何环境前：必跑 startup 验证

## 验收标准

### AI 自动验收

- [ ] `.env` 启动期 schema 校验（dev-ops/env-split 已立项，本阶段补集成测试）
- [ ] `scripts/smoke-startup.sh`：`make dev-start` 后 90s 内全 `/healthz` 200
- [ ] 故意 kill 一个依赖（mongo）→ 对应服务 `/readyz` 变 503，`/healthz` 仍 200，恢复后 readyz 回 200
- [ ] K8s manifest 校验：`kubeval` + `kube-score` 在 CI 跑
- [ ] 启动顺序无依赖耦合：随机顺序启动 6 个服务，最终全部 ready

### 人类验收

- [ ] 确认部署目标中哪些 manifest / Kong 配置需要纳入验证

## 工具候选

| 工具 | 用途 |
|---|---|
| 自写 `scripts/smoke-startup.sh` | docker-compose 全量启动 + health 轮询 |
| `kubeval` | K8s manifest schema 校验 |
| `kube-score` | K8s 最佳实践检查 |
| `deck validate` | Kong declarative config 校验 |
| `helm template` + `kubectl apply --dry-run` | Helm chart 渲染验证 |

## 与其他阶段协作

- 依赖 dev-ops/DEV-02（health-endpoints）提供 `/healthz` `/readyz` 标准
- 依赖 dev-ops/DEV-04（env-split）提供 `.env` schema 校验脚本
- 与 QUAL-08 混沌共享 docker compose 故障注入手法

## 待展开问题

- K8s manifest 是否真存在生产部署需要验证？还是只验证 docker-compose？
- Kong declarative config 是否在仓库内？需要确认 `deployments/` 现状
- /readyz 与 /healthz 的语义如何区分（前者 = 流量准备好，后者 = 进程活着）？
