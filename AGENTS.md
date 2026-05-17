# AGENTS.md

micro-service 项目的 AI 协作入口文档。本文是 AI 处理本仓库任务的**唯一入口规则文档**；其他工具文件（`CLAUDE.md` / `CODEBUDDY.md` 等）仅作摘要，冲突时以本文为准。

> 本文由 [axm skill](https://github.com/castle-x/axiom) 生成骨架，项目特有内容（架构、包边界、Knowledge Index）由 AI 按项目实际补全。

## .axm 召回声明

**本条优先级最高，高于其余所有规则。**

每轮若实际读取过 `.axm/` 下任何文件，**必须**在回答最开头输出：

> **.axm 召回**
>
>
> | 文件          | 读取原因  |
> | ----------- | ----- |
> | `.axm/<路径>` | <一句话> |
>

仅列实际读取的文件，按读取顺序排列；本轮未读则不输出此块；沿用上轮已读内容仍需列出。该表格必须是回答的第一块内容。

## Architecture

`micro-service` 是一个 Go 单仓库多模块微服务平台骨架，基于 `Kong + Hertz + Kitex + MongoDB + Redis + etcd + NSQ` 构建。

### 模块划分

- `idl/` — 全局 Thrift IDL 独立 module，定义跨服务 RPC 契约和 `base.thrift` 共享结构。
- `pkg/` — 通用基础设施独立 module，包含 `logger`、`db`、`utils`、`config`、`errno`、`redis`、`jwt`、`middleware`、`cloudwego`、`health`、`httpclient`、`otel`、`registry`、`mq`。
- `services/edge-api/` — Hertz HTTP/WebSocket 接入层，负责 REST 门面、参数校验、协议转换和回调接收。
- `services/idp/` — Kitex 身份认证服务，负责 OAuth/OIDC、登录注册、Token 签发与刷新。
- `services/iam/` — Kitex 用户与权限服务，负责用户资料、组织、RBAC/ABAC 和资源授权。
- `services/billing/` — Kitex 支付服务，负责订单、支付渠道、对账、退款。
- `services/credits/` — Kitex 积分服务，负责账户、余额、流水、规则引擎。
- `services/notification/` — Kitex 通知服务，负责短信、邮件、站内信、推送模板。
- `services/asset/` — Kitex 数字资产服务，负责个人资产类型、资产库、版本、媒体对象和上传会话。
- `services/model/` — Hertz HTTP AI 模型服务，负责 LLM/图像供应商配置管理、对话接口（非流式 + SSE 流式）。
- `deployments/` — Docker、Kong、K8s 等运行与部署配置。
- `.axm/progress/` — 阶段进度、路线图、spec 与验收状态记录；替代历史 `.phase/` 目录。

### 依赖方向

`services/* → pkg`；`services/* → idl/generated code`；`edge-api → idp/iam/asset Kitex RPC client`；`edge-api → model service (HTTP proxy)`；`billing → MQ event → credits/notification`；`pkg` 不依赖任何业务服务。

### 核心约束

- `pkg/` 禁止 import `services/*`。
- `services/*` 之间禁止直接 import 对方内部 Go 包；跨服务通信必须走 IDL + Kitex RPC 或 MQ。
- **例外**：`services/model/` 使用 Hertz HTTP（而非 Kitex RPC），因为 SSE 流式输出天然属于 HTTP 协议；`edge-api` 通过 HTTP proxy 调用，接口契约见 `idl/model/openapi.yaml`。
- `edge-api` 禁止直接访问业务主存储；业务数据必须走后端服务。允许读取鉴权/封禁类短期 Redis guard key（当前为 `idp:banned:{userID}`）以完成请求前置拦截。
- 所有日志走 `pkg/logger.Ctx(ctx)`，trace/user/tenant 元数据由 `pkg/middleware` 透传。
- 业务错误码统一走 `pkg/errno`，新增错误必须落在已实现的服务区段；历史来源见 `.axm/progress/platform/decisions.md`。

## Coding Rules

> 这些规则偏向谨慎而非速度。对于显然简单的任务，运用判断力即可。

### 1. 先思考，再动手

**不假设。不掩饰困惑。主动呈现权衡。**

动手前：

- 明确说出你的假设。不确定就问。
- 存在多种解读时，把它们列出来——不要沉默地选一个。
- 有更简单的方案就说出来。该推回就推回。
- 有不清楚的地方，停下来。说出困惑在哪。问。

### 2. 简单优先

**能解决问题的最少代码。不做任何投机性内容。**

- 不做超出要求的功能。
- 单次使用的代码不做抽象。
- 不做未被要求的"灵活性"或"可配置性"。
- 不为不可能发生的场景写错误处理。
- 写了 200 行、50 行就能解决的，重写。

自问："资深工程师会觉得这过度复杂吗？"如果是，简化。

### 3. 外科手术式修改

**只动必须动的。只清理自己制造的乱子。**

修改现有代码时：

- 不"改进"相邻代码、注释或格式。
- 不重构没有损坏的东西。
- 匹配已有风格，即使你会用不同写法。
- 注意到无关的死代码，说出来——不要删。

当你的改动产生孤儿时：

- 清理**因你的改动**而变成未使用的 import / 变量 / 函数。
- 不清理早于本次变更就已存在的死代码，除非被要求。

检验标准：每一行变更都应能直接追溯到用户的需求。

### 4. 目标驱动执行

**明确验收标准。循环直到验证通过。**

把任务转化为可验证的目标：

- "加校验" → "先写针对非法输入的测试，再让测试通过"
- "修 bug" → "先写能复现 bug 的测试，再让测试通过"
- "重构 X" → "确保重构前后测试都通过"

多步骤任务，先陈述简要计划：

```
1. [步骤] → 验证：[检查点]
2. [步骤] → 验证：[检查点]
3. [步骤] → 验证：[检查点]
```

清晰的验收标准让你能独立循环推进。模糊标准（"让它跑起来"）会导致反复澄清。

---

最终决定权在人。AI 不擅自做重大决策；有异议明确说出，但尊重人的选择。

**这些规则生效的标志：** diff 中不必要的改动减少；因过度复杂而返工的情况减少；澄清问题在动手前提出，而不是在出错后。

## Knowledge Index

<!-- Knowledge Index 是任务路由表；新增长期文档后同步维护这里的目标阅读路径。 -->

| 任务类型             | 读哪里                          |
| ---------------- | ---------------------------- |
| 每次任务开始 / 分级与流程   | `.axm/universal/devloop.md` |
| 编码完成 / 提交前质量门禁   | `.axm/universal/quality.md` |
| 提交 / 分支操作        | `.axm/universal/vcs.md`     |
| 写 `.axm` 文档       | `.axm/universal/docs.md`    |
| 第二个 agent / 工具收尾 review | `.axm/universal/review.md` |
| roadmap / spec / 开发进度 | `.axm/progress/index.md`    |
| 理解整体架构 / 新增模块   | `.axm/project/architecture.md` + `.axm/knowledge/services/overview.md` |
| 编写或修改 Go 代码      | `.axm/project/coding.md` + `.axm/knowledge/pkg-infra/overview.md` |
| 修改 pkg 基础设施       | `.axm/project/architecture.md` + `.axm/knowledge/pkg-infra/overview.md` |
| 修改服务骨架 / 新增服务逻辑 | `.axm/project/architecture.md` + `.axm/knowledge/services/overview.md` |
| 设计 IDL / RPC 接口   | `.axm/project/architecture.md` + `.axm/knowledge/services/overview.md` |
| model service / LLM 适配器 | `idl/model/openapi.yaml` + `services/model/adapter/adapter.go` |
| asset service / 数字资产库 | `idl/asset/asset.thrift` + `.axm/knowledge/services/overview.md` |
| idp / iam / Google 登录 | `.axm/knowledge/services/overview.md` + `.axm/project/architecture.md` |
| 调整测试 / lint / 构建命令 | `.axm/project/coding.md` |
| 排查 trace / 日志 / 中间件 | `.axm/knowledge/pkg-infra/overview.md` + `.axm/project/coding.md` |
| OpenTelemetry / 可观测性 / 后端排障 | `.axm/project/observability.md` + `.axm/knowledge/observability/overview.md` |
| 新增 DB / Redis / MQ / 外部 API / LLM 调用链路 | `.axm/project/observability.md` + `.axm/project/coding.md` |
| 做 code review / 写 PR / 定合并门禁 | `.axm/project/code-review.md` |
| 写 API 测试 / 契约 / E2E / SSE 专项 | `.axm/project/api-testing.md` |
| 推进质量体系（CR/测试/契约/E2E/度量）落地与进度 | `.axm/progress/quality/` |

`.axm/` 目录分区：`universal/`（通用流程）、`project/`（项目规范）、`knowledge/`（系统设计事实）、`progress/`（roadmap、阶段 spec、验收状态）。各目录有 `index.md` 总索引。
