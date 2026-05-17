<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: dev-ops
workflow-state: in-progress
state-updated: 2026-05-17
related:
  - ../../knowledge/pkg-infra/overview.md
  - ../../project/observability.md
  - ../platform/roadmap.md
-->

# 本地开发运维改造路线图

> 反映**当前最新状态**。每个阶段验收完成后更新此文件。
>
> 最后更新：2026-05-17

---

## 背景

`make dev-start / dev-restart / dev-stop` 已成型，但当前实现在**稳定性**与**AI 可读性**两个维度存在系统性缺陷：

- **进程管理**：`lsof -ti tcp:<port> | xargs kill -9` 风格——强杀、可能误杀同端口无关进程、etcd 注册的服务租约不能优雅注销，重启容易留残影。
- **就绪等待**：`dev-start` 用 `sleep 2/3` 假装等服务就绪，没有真实健康探测，低性能机器或冷启动时 web 起来时后端未注册完成 → 出现"启动看似成功、首个请求 404"。
- **健康检查**：服务未提供统一的 `/healthz /readyz`，AI 无法用通用方式判断服务真实状态。
- **日志**：`pkg/logger` 已经默认 JSON，但未强制全链路使用，第三方库（Hertz/Kitex access log、Mongo driver）格式各异；6 个服务 6 个日志文件无聚合查询入口。
- **配置**：单一 `.env` 已上百行（infra + secrets + asset + observability 混在一起），新增 service 后会持续膨胀；占位符（`your-...` / `change-me-`）忘改要等运行时才报错。

本 initiative 不追求"线上级别"的 DevOps，只为本地 dev 阶段的**稳定性**与**AI 自助排障能力**做一轮针对性改造。

## 总目标

完成本路线后，AI / 人类在本地任意时刻执行下列动作都能拿到稳定、机器可读的反馈：

1. `make dev-start` —— 真正等到所有服务 `/readyz` 通过才返回；任何服务起不来直接打印对应 log 尾部并 exit 1
2. `make dev-stop` —— 优雅 SIGTERM，5s 兜底 SIGKILL；etcd 上的服务注册即时注销
3. `make dev-status` —— 输出 JSON 数组（每个服务的 pid / port / 健康状态 / 启动时间），AI 一次 Read 即可掌握全局
4. `cat bin/log/<svc>.log` —— 严格统一的 JSON 行格式，每行至少含 `time / level / service / msg / trace_id`
5. `make dev-check-env` —— 启动前校验，缺失或占位的关键变量以结构化输出列出
6. 任意服务异常退出，状态文件 + 日志末尾 30 行能让 AI 自洽诊断，无需人工介入

## 阶段路线图

| Phase | 主题 | 状态 | 产物 |
|-------|------|------|------|
| DEV-01 | 进程生命周期：PID 文件 + 优雅停 + 状态查询 | ✅ 已完成并闭合 | [`specs/process-lifecycle.md`](specs/process-lifecycle.md) |
| DEV-02 | 标准健康检查接口（/healthz /readyz /version） | ✅ 已完成并闭合 | [`specs/health-endpoints.md`](specs/health-endpoints.md) |
| DEV-03 | 日志格式统一 + AI 可调用查询入口 | ✅ 已完成并闭合 | [`specs/log-unification.md`](specs/log-unification.md) |
| DEV-04 | .env 拆分 + 启动前校验脚本 | ✅ 已完成并闭合 | [`specs/env-split.md`](specs/env-split.md) |

## 阶段依赖

- **DEV-02 依赖 DEV-01**：`dev-start` 的就绪等待逻辑需要 PID 文件确认进程在跑、再轮询 `/readyz` 才有意义。
- **DEV-03 不依赖其他**，可与 DEV-01/02 并行；但 DEV-01 的 `dev-status` 输出格式应与 DEV-03 的日志格式约定一致，便于 AI 统一处理。
- **DEV-04 不依赖其他**，可独立推进；建议在 DEV-01 之前或同步进行，因为新的 dev-start 脚本需要明确 env 加载顺序。

## 不在范围内

- 不引入 overmind / mprocs 等需要终端 UI 的进程管理工具（人类 UI 优先级低）
- 不做 Go 热重载（air 等），保留全量 build 流程
- 不改造 docker-compose 拓扑（mq profile 化等已确认但放在另一个 cleanup PR）
- 不涉及 CI / 生产部署的 DevOps，只覆盖本地 dev

## 当前事实进度

- ✅ 与用户对齐 4 个阶段拆分（2026-05-14）
- ✅ DEV-01 ~ DEV-04 spec 已落地并完成实现（2026-05-14）
- ✅ 主 agent 已完成交叉审核：脚本语法、`services.json`、`dev-status` JSON、`pkg/health`、`pkg/logger`、5 个接入服务测试、`make build`、`make lint` 均通过（2026-05-14）
- ✅ tester agent 已完成统一验收复核：DEV-01 初检发现 2 个 readiness 缺口，修复后复检通过；DEV-02/03/04 focused verification 通过（2026-05-14）
- ✅ dev-ops BUG 修复波次完成：5 条 `open` BUG 均进入 `fixed`，覆盖 `start.sh` 失败传播、readyz 超时诊断、edge-api/model etcd readiness、`MODEL_ENCRYPT_KEY` env 模板与 gitignore env 白名单（2026-05-14）
- ✅ 主 agent 与验收 agent 交叉复核通过：脚本语法、自检、start 失败路径、env fixture、gitignore 行为、`pkg/cloudwego` + `pkg/health` 测试、edge-api/model 测试、`make build`、`make lint`、`git diff --check` 均通过（2026-05-14）
- ✅ 2026-05-17 用户确认 dev-ops 已开发完成；DEV-01 ~ DEV-04 闭合为历史实施记录
- ⚠️ 2026-05-17 复核发现 `.gitignore` 仍会 ignore `deployments/env/infra.env` 与 `deployments/env/observability.env`，`bug-2026-05-14-gitignore-blocks-shared-env-files` 已重开；因此本 roadmap 的 workflow-state 暂回到 `in-progress`，直到该 BUG 再次修复/验证

## 闭合记录

- 长期事实已同步到 `../../knowledge/pkg-infra/overview.md`、`../../knowledge/services/overview.md`、`../../project/observability.md`：`pkg/health`、dev-start/status/check-env、日志查询、env 拆分均已成为当前系统事实。
- 本 initiative 的 4 条 BUG 已关闭，1 条 `.gitignore` 共享 env 白名单 BUG 处于 reopened；单条 BUG 文档为事实来源。
- 真实依赖异常演练、新成员 onboard 体验和 OpenObserve 日志入库等可作为后续回归或质量项，不再阻塞 dev-ops 第一版闭合。

## 尚未确认的问题

- 当前阻塞项：`bug-2026-05-14-gitignore-blocks-shared-env-files` 需重新修复 `.gitignore` 白名单并复跑验收。
- 已知限制：edge-api/model 的 etcd readiness 通过 `pkg/cloudwego.SharedEtcdClient` 集中缓存 health client；upstream Hertz/Kitex etcd registry/resolver 不暴露内部 client，因此没有直接复用 registry 私有 client。
- 日志查询当前只做本地文件版；OpenObserve 仍走现有 `make obs-*`。完整日志入库如有需要另拆质量或 OTel 小阶段。
