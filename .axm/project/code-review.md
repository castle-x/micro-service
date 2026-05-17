<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-14
owner: castlexu
applies-to: [project:micro-service]
related:
  - ./coding.md
  - ./architecture.md
  - ./observability.md
  - ../universal/quality.md
  - ../universal/vcs.md
-->

# micro-service 代码审查规范

> 目标：让 review 真正提升代码质量与团队水平，而不是变成"风格警察"。本文与 `universal/quality.md` 的"代码审查要点"互补：universal 给最小通用门禁，本文给 micro-service 项目落地细则。

## 1. 何时需要 Review

| 变更类型 | 是否必须 Review | 最少 reviewer 数 |
|---|---|---|
| `pkg/` 公共基础设施 | 必须 | 2（其中 1 名 owner） |
| `idl/` thrift / openapi 契约 | 必须 | 2（消费方 + 提供方各 1） |
| `services/*` 业务逻辑 | 必须 | 1 |
| `services/*` 跨服务调用链路新增 | 必须 | 2（调用方 + 被调方） |
| `deployments/`、Kong、K8s、CI | 必须 | 1（运维 owner） |
| 仅文档 / `.axm/` / 注释 | 建议 | 1 |
| 自动化生成代码（kitex_gen、thrift gen） | 不审查代码本身，只审查触发它的 IDL 变更 | — |

**作者自审**：开 PR 前先按 §4 清单自检一遍，不要把脏活推给 reviewer。

## 2. 优先级标记（强制使用）

每条评论必须带前缀，便于作者一眼分类、便于 PR 合并门禁判断：

| 标记 | 含义 | 是否阻塞合并 |
|---|---|---|
| 🔴 **Blocker** | 正确性 / 安全 / 数据丢失 / 破坏契约 | 是，必须修 |
| 🟡 **Suggestion** | 可维护性 / 性能 / 缺测试 / 重复代码 | 否，但需作者明确回应 |
| 💭 **Nit** | 命名、注释、风格 | 否，作者可选 |
| ❓ **Question** | 不确定意图，先问再下结论 | 否 |
| 👍 **Praise** | 好的设计或写法 | 否，鼓励而非凑数 |

只有 🔴 是合并阻塞项；🟡 至少需要作者一句"采纳/不采纳 + 理由"。

## 3. 审查维度（按优先级）

### 3.1 🔴 正确性与契约（Correctness）
- 业务逻辑是否真的实现了 PR 描述 / spec 中的验收标准？
- IDL 变更是否向后兼容？字段编号是否复用？枚举是否新增而非修改？
- 跨服务调用是否正确处理超时、重试、幂等？
- DB 写路径是否正确处理事务边界、并发与重复消息（NSQ at-least-once）？

### 3.2 🔴 安全（Security）
- 入参是否校验？（HTTP 走 hertz binding/validate，RPC 在 biz 层显式校验）
- 是否引入 SQL/NoSQL/命令注入、SSRF、路径穿越？
- 鉴权：edge-api 入口是否经过 `pkg/middleware` 的 auth；`iam` 是否做了资源级 RBAC/ABAC 检查？
- 敏感字段（password / secret / token / authorization / 手机号 / 身份证）**禁止**进日志、错误 message、metadata、trace span attribute。
- 第三方 LLM / 支付回调签名是否校验？回调是否做幂等？

### 3.3 🟡 可观测性（Observability）
- 新增 HTTP/RPC/DB/Redis/MQ/外部 API/LLM 链路是否符合 `project/observability.md`：起 span、记录关键 attribute、错误打 status=Error？
- 日志是否走 `logger.Ctx(ctx)`？是否带 trace_id/user_id/tenant_id？
- 是否在循环里打了 INFO 日志？（高 QPS 下会刷屏）

### 3.4 🟡 错误处理（Error Handling）
- 业务错误是否走 `pkg/errno`，落在已实现服务区段？
- DB 错误是否经 `errno.FromDBError` 转换？
- 是否吞异常（`_ = err` / 空 catch）？
- 错误是否携带可定位的上下文（资源 ID、租户、操作类型）？

### 3.5 🟡 架构边界（Architecture）
- `pkg/` 是否 import 了 `services/*`？（禁止）
- `services/*` 之间是否直接 import 对方内部包？（禁止，必须走 IDL/RPC 或 MQ）
- `edge-api` 是否绕过后端服务直接访问业务 DB？（禁止）
- 新功能是否放错了层（biz 逻辑塞进了 handler / dal）？

### 3.6 🟡 性能（Performance）
- N+1 查询：循环里调 DB / RPC？批量化或 dataloader 化。
- 是否在热路径反复 new logger / new client / 解析 config？
- Redis：key 是否带租户 / 服务前缀？是否设置 TTL？
- Goroutine 泄漏：是否所有 goroutine 都受 ctx 控制？是否有出口？
- 大对象拷贝、json.Marshal 大 struct、未分页的 list 接口。

### 3.7 🟡 可维护性（Maintainability）
- 命名是否准确（避免 `data`, `info`, `tmp`, `Manager`, `Helper`）？
- 一个函数 > 80 行 / 一个文件 > 600 行先质疑必要性。
- 是否过度抽象（参考 AGENTS.md §2 简单优先：单次使用不抽象、不投机）。
- 重复代码（出现 3 次以上才考虑提取，遵循 rule of three）。
- 死代码、未使用 import / var / func 是否清理（仅清理本次改动产生的）。

### 3.8 🟡 测试（Testing）
- bugfix：**必须**带可复现 bug 的回归测试（红线，参见 `universal/quality.md`）。
- 新功能：核心路径单测覆盖；T2 以上分级需 unit + 手动验证；T3 需 E2E。
- 测试是否真的断言了行为，而不是断言"调用过 mock"？
- 边界用例：空、零、负数、超长、并发、超时、网络错误。

### 3.9 💭 风格（Style）
- 已由 `make fmt` / `make lint` 兜底的，不要手动评论。
- 如 lint 未覆盖到的（命名语义、注释清晰度），可提 💭 nit。

## 4. 作者自审 Checklist（开 PR 前）

```
□ 已本地跑过：make fmt && make lint && make test（或对应分级门禁）
□ 已自审 diff，没有"顺手改"的无关变更（参见 AGENTS.md §3）
□ PR 描述写明：动机 / 方案 / 影响面 / 验收方式 / 回滚方式
□ 关联 .axm/progress/ 的 spec 或 issue
□ IDL 变更：列出所有消费方并通知 owner
□ DB schema 变更：附 migration 脚本 + 回滚脚本
□ 新依赖：已跑 go mod tidy；说明引入理由
□ 涉及敏感字段：确认未进日志/错误/trace
□ 新增 I/O 链路：已按 observability.md 起 span、加 metric
□ 已删除调试用 fmt.Println / TODO / 注释掉的代码
```

## 5. Reviewer 行为准则

1. **24 小时内首轮响应**（工作日）。超过则作者可在群内 @ catch up。
2. **一次给完意见**，不要分多轮挤牙膏。
3. **解释 why，不只是 what**——"改成 X" → "改成 X，因为 Y 在并发下会 Z"。
4. **建议而非命令**——"建议考虑 X" 优于 "必须改成 X"，除非是 🔴。
5. **对事不对人**——评论代码而不是评论作者；用 "this function" 而非 "you"。
6. **看到好代码就夸**——👍 是低成本的高回报。
7. **不确定就问**——❓ 比错误的 🔴 更受欢迎。
8. **尊重作者判断**——AGENTS.md 已写"最终决定权在人"；reviewer 不擅自做架构决策，提出异议但接受作者反驳。

## 6. 评论模板

````markdown
🔴 **安全：SQL 注入风险**
`services/iam/dal/mongo/user.go:142`：`name` 直接拼进 BSON 查询字符串。

**Why**：攻击者可以通过 `{"$ne": null}` 这种结构绕过校验，泄露全部用户。

**建议**：
- 用结构化 query：`bson.M{"name": name}`
- 在 biz 层用 `validator` tag 限制 name 字段为字母数字
````

````markdown
🟡 **性能：N+1 RPC 调用**
`services/edge-api/biz/order/list.go:88`：循环里对每个订单都调用 `iam.GetUser`。

**Why**：100 条订单 = 100 次 RPC，p99 会爆炸。

**建议**：用 IDL 里已有的 `iam.BatchGetUser`，一次拿全。
````

## 7. 评审意见处理契约（作者侧）

> 本节针对**意见的消费侧**：作者拿到 review 评论后如何验证、采纳、拒绝、闭环。
> 适用于**人类 reviewer 与 AI/Agent reviewer（Codex / Claude / 任何 LLM 审查工具）混合**的所有场景。
> 与 §5 reviewer 行为准则形成"两端契约"——产生侧讲生成纪律，本节讲消费侧纪律。

### 7.1 三层契约

```
┌──────────────────────────────────────────────────────────┐
│ 认知层 — 不盲信任何 reviewer（含 LLM）                    │
│   1. 评审输出是【建议】，永远不要盲目执行                │
│   2. 每条 finding 必须读真实代码路径与相邻文件做验证      │
│   3. 涉及外部行为时，读依赖的文档/源码/类型做仲裁         │
├──────────────────────────────────────────────────────────┤
│ 决策层 — 拒绝噪音与过度建议                              │
│   4. 拒绝不切实际的边界情况、投机性风险、大范围重写、    │
│      让代码变复杂的修复                                  │
│   5. 修复优先在【正确的所有权边界】做小修；               │
│      除非明显改善一类 bug，否则不重构                    │
├──────────────────────────────────────────────────────────┤
│ 工程层 — 闭环纪律                                         │
│   6. 持续审查，直到不再产生【被接受 / 可执行】的 finding │
│   7. 修复改了代码 ⇒ 重跑相关测试 + 重跑 review            │
│   8. 拒绝某条 finding 时，仅当它解释【真实不变量或所有   │
│      权决策】时，加一条简短 inline 注释                  │
│   9. 不为审查而 push；仅在用户明确要求 push / ship /     │
│      PR 更新时 push                                       │
└──────────────────────────────────────────────────────────┘
```

### 7.2 每条契约的处理动作

| # | 契约 | 作者具体动作 |
|---|---|---|
| 1 | 评审是建议 | 拿到评论后**不直接 `git apply`**；先进入第 2 条流程 |
| 2 | 用代码事实仲裁 | 打开被指出的文件 + 相邻文件 + 调用方/被调方，确认指控成立 |
| 3 | 用依赖事实仲裁 | 若 finding 提到 `Hertz/Kitex/Mongo driver/jwt` 等外部行为，读对应库的当前版本文档或源码；不靠记忆 |
| 4 | 拒绝噪音类 | 见 §7.3 噪音模式清单，命中即驳回 |
| 5 | 小修优先 | 选择影响面最小的修复点；越过边界（如 reviewer 让你改 pkg/，但 bug 在 service/）要拒绝并解释 |
| 6 | 终止条件 | 当一轮 review 全部为 ❓/💭/👍 或仅剩 §7.3 噪音类时收手；不强求 0 评论 |
| 7 | 修复后重跑 | 任何代码改动 ⇒ `make lint && make test`（或对应分级）+ 触发新一轮 review |
| 8 | 拒绝沉淀为注释 | 见 §7.4 inline 注释模板 |
| 9 | 不为审查而 push | 仅在明确"push / ship / 更新 PR"指令下推送；review 过程中的临时 commit 留在本地或 stash |

### 7.3 必须驳回的"噪音模式"

reviewer（尤其 LLM）高频产出，作者**必须拒绝**：

- ❌ "如果有人传入 10GB 的 string 会怎样" — 不切实际的边界
- ❌ "理论上 race 可能发生" — 无具体路径的投机风险
- ❌ "建议把这部分抽成单独模块/服务" — 与当前 bug 无关的大范围重写
- ❌ "把这个 if 链改成策略模式" — 不改善 bug 类的炫技重构
- ❌ "加一个 mutex 防御一下" — 没证明真存在竞态就加锁
- ❌ "建议补充 5 个未来可能出错的 case" — 过度防御
- ❌ "命名建议从 `userId` 改成 `userID`" — 与本 PR 无关的格式偏好（lint 该管的不归 reviewer）

驳回时使用 §7.4 的 "intentional / scope" 模板，不必逐条辩论。

### 7.4 拒绝时的 inline 注释模板

**仅当**拒绝原因属于"真实不变量或所有权决策"时才写。普通的"这次不修"不必污染代码。

判断标准：未来的 reviewer（人或 LLM）看到这段代码，是否**仍然会提出同一条 finding**？是 → 写注释；否 → 不写。

```go
// invariant: balance allowed to be negative within the same txn;
// the saga step in services/billing reconciles it before commit.
// See .axm/progress/quality/specs/consistency-suite.md.
balance := account.balance - amount
```

```go
// ownership: validation lives in biz layer, NOT here. dal must stay
// thin and protocol-only. Do not add input checks at this boundary.
func (r *repo) Insert(ctx context.Context, u *User) error {
```

```go
// intentional: we DO want a nil-deref panic here if cfg is nil;
// it signals a misconfigured deployment and should crash fast,
// not silently fall back to defaults.
return cfg.Database.URI
```

反例（**不要**写这种）：

```go
// reviewer said this might be slow but it's fine for now  ← 噪音
// TODO: refactor someday                                  ← 与本次审查无关
// codex flagged this, ignored                             ← 没有信息量
```

### 7.5 与现有规范的衔接

- §2 优先级标记 ↔ 本节 §7.3 噪音清单：reviewer 应该用 🟡/💭，作者用 §7.3 驳回，标记体系闭环。
- §5 reviewer 行为准则 ↔ 本节 §7.1：reviewer 端"解释 why / 建议而非命令"，作者端"建议而非命令 → 我有权拒绝"，权责对称。
- §8 合并门禁第 4 条"所有 🟡 已被作者明确回应"：本节给出"明确回应"的具体形态（采纳 / §7.3 驳回 / §7.4 inline 注释）。

### 7.6 适用于 AI / Agent reviewer 的额外提醒

当 reviewer 是 LLM（Codex review、Claude review、本仓库未来引入的任何 AI review 工具）时：

- **它的"权威感"更强**——措辞自信、引用规范、看起来无懈可击。**这本身是缺陷而非优势**，更需要按 §7.1 第 1-3 条仲裁。
- **它倾向输出"看起来很专业"的过度建议**——§7.3 噪音清单几乎都是 LLM 高频产出，作者必须主动驳回。
- **它没有项目所有权概念**——经常越过 §7.1 第 5 条的边界（让你改 pkg/ 来修一个 service/ 的 bug）；这种 finding **默认拒绝**。
- **它不记得上次为什么拒绝**——§7.1 第 8 条 inline 注释是唯一让"重复审查的边际成本递减"的机制；务必在判断为真实不变量/所有权决策时落注释。

## 8. 合并条件（Merge Gate）

PR 可合并当且仅当：

1. CI 通过：`make lint && make test`（或对应分级要求，见 `universal/quality.md`）。
2. 至少 1 名 reviewer **Approve**（pkg/idl 需 2 名）。
3. 所有 🔴 **Blocker** 已解决（修复或经讨论降级为 🟡/💭）。
4. 所有 🟡 **Suggestion** 已被作者明确回应（采纳 / 不采纳 + 理由）。
5. 分支已 rebase 到目标分支最新 commit，无冲突。
6. PR 描述 + commit message 符合 `universal/vcs.md`。

## 9. 与现有规范的关系

- 本文是项目级 review 标准，**不替代** `universal/quality.md` 的通用质量门禁。
- 编码规范（命名、错误、日志、模块）见 `project/coding.md`，本文不重复，只负责"如何审查这些规范是否被遵守"。
- 提交、分支、PR 描述格式见 `universal/vcs.md`。
- 落地路线、度量与季度回顾计划见 `.axm/progress/quality/`（progress 范畴，非规范本身）。
- §7 评审意见处理契约改编自 `skills/codex-review/SKILL.md § Contract`（9 条原文），融合为本项目"两端契约"（reviewer 侧 + 作者侧）。
- 任何冲突以 `AGENTS.md` 为最终入口规则。
