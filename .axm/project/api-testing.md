<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
applies-to: [project:micro-service]
related:
  - ./coding.md
  - ./architecture.md
  - ./observability.md
  - ./code-review.md
  - ../universal/quality.md
-->

# micro-service API 测试体系

> 目标：把"上线后才发现 API 坏了"变成"提交前就能拦截"。本文与 `universal/quality.md`（通用 T0–T3 测试分级）互补，给本项目 HTTP/RPC/SSE/MQ 四种接口的具体测试方案与可执行命令。

## 1. API 接口分类与测试责任

本仓库存在 4 类对外/对内 API，测试策略不同：

| 接口类型 | 协议 | 典型位置 | 主要风险 | 责任分级 |
|---|---|---|---|---|
| 对外 HTTP（REST） | Hertz HTTP/JSON | `services/edge-api/` | 入参校验、鉴权、协议转换 | T2+ |
| 对外 HTTP（SSE 流式） | Hertz HTTP/SSE | `services/model/` | 流断、首 token 延迟、心跳 | T3 |
| 内部 RPC | Kitex Thrift | `services/{idp,iam,billing,credits,notification,asset}` | 契约兼容、超时、幂等 | T2+ |
| 异步事件 | NSQ Topic | `billing → credits/notification` | 重复消费、顺序、死信 | T2+ |

> Asset 服务通过 OSS / 第三方回调链路也算对外 HTTP，归 edge-api 类。

## 2. 测试金字塔（本项目落地版）

```
        ┌──────────────────┐
        │   E2E 跨服务      │  ~5%   关键业务链路（登录、下单、扣费、通知）
        │   shell + curl    │
        ├──────────────────┤
        │   契约测试        │  ~15%  IDL 兼容、OpenAPI 校验
        │   pact / openapi  │
        ├──────────────────┤
        │   集成测试        │  ~30%  单服务 + 真实 Mongo/Redis (testcontainers)
        │   go test +tags   │
        ├──────────────────┤
        │   单元测试        │  ~50%  biz / dal / handler 纯逻辑
        │   go test         │
        └──────────────────┘
```

数字是健康分布参考，不是 KPI。**底层薄、中间空、顶层重** 是反模式（"冰淇淋甜筒"）。

## 3. 各层测试方案

### 3.1 单元测试（Layer 1）

**对象**：`biz/`、`dal/mongo/`、`dal/model/`、`handler/`、`pkg/` 工具函数。

**约束**：
- 不依赖网络、不依赖真实 DB；用 mock interface（gomock）或 in-memory fake。
- `dal/mongo/` 的 mock：抽 `Repository` interface，单测注入 fake 实现；不直接 mock `*mongo.Client`。
- 表驱动测试是默认风格：`tests := []struct{ name, in, want, wantErr }{...}`。
- 测试文件与源文件同包（白盒）；只在测公共 API 时用 `_test` 包（黑盒）。

**门禁**：
```bash
make test            # 仓库级
cd pkg && go test ./... -count=1 -race
cd services/iam && go test ./... -count=1 -race
```

`-race` **必须开**：本项目重度使用 goroutine（NSQ 消费、SSE 推流、Kitex handler）。

### 3.2 集成测试（Layer 2）

**对象**：单个服务 + 它依赖的真实中间件（Mongo、Redis、NSQ、etcd）。

**实现**：
- 用 build tag 隔离：`//go:build integration` + `make test-integration`。
- 中间件用 `testcontainers-go` 起 ephemeral 容器，不依赖本机服务。
- 测试 fixture：每个 case 用独立 db name / redis prefix / topic name，跑完销毁。

**模板路径**：`services/<svc>/test/integration/`，新建服务时复制。

**门禁**：
```bash
make test-integration
```

CI 上必跑；本地默认跳过（用 build tag），需要时显式开。

### 3.3 契约测试（Layer 3）

micro-service 的契约面有两个：**IDL Thrift（内部 RPC）** 和 **OpenAPI（对外 HTTP / model 服务）**。

#### 3.3.1 Thrift IDL 兼容性测试

**风险**：字段编号复用、required/optional 误改、枚举改值——会导致老版本 client crash。

**方案**：在 CI 增加 `make idl-compat`（或聚合目标 `make test-contract`）：
- 用 `thrift --gen` 比较 PR base 与 head 的 IDL 文件结构；
- 检测：字段编号重复使用、required 字段新增/删除、enum 值变更、struct/service 删除。
- 实现脚本放 `scripts/idl-compat.sh`，参考 [thrift-compat](https://github.com/cloudwego/thriftgo) 或自写 AST diff。

**契约消费方测试**：每次 IDL 变更 PR 必须列出所有 import 该 IDL 的 services 并跑这些服务的测试。

#### 3.3.2 OpenAPI 契约测试

**对象**：`idl/model/openapi.yaml` 与 `services/model/`、`services/edge-api/` 暴露的 HTTP 接口。

**双向校验**：
1. **请求/响应 schema 校验**：在集成测试里用 `kin-openapi` 对每个 case 的 req/resp 做 OpenAPI schema 验证；任何字段不符即 fail。
2. **路由覆盖率检查**：CI 跑完测试后用 coverage 工具列出 OpenAPI 里所有路径，未被任何测试 hit 的路径 → 阻塞合并。
3. **Mock server**：前端联调期用 `prism mock idl/model/openapi.yaml` 起 mock；前后端共享同一份契约。

### 3.4 E2E 测试（Layer 4）

**关键链路**（必须 E2E）：
- 登录注册：`edge-api → idp → iam`（已有 `scripts/e2e-google-auth.sh`，作为模板）
- 资产上传：`edge-api → asset → OSS 回调`（已有 `scripts/e2e-asset-oss-upload.sh`）
- 支付扣费：`edge-api → billing → MQ → credits/notification`
- 模型对话（SSE）：`edge-api → model → LLM provider`，验证流式 chunk、心跳、断流恢复

**实现**：
- 用 shell + curl + jq；不引重型框架，保持 5 分钟内可读懂。
- 每个 E2E 必须**幂等**：跑两遍结果一致，自带 cleanup。
- 固定测试账号：`admin@platform.com / Admin@1234`（见 `coding.md §本地集成测试账号`）。
- E2E 在 CI 用 docker-compose 起全栈（参考 `deployments/`）。

**门禁**：merge 到 `main` 前必须 E2E 全绿；feature 分支可跳过。

### 3.5 SSE 流式接口专项

`services/model/` 的对话流是 micro-service 最容易出事的接口，单独立专项：

| 测试点 | 方法 |
|---|---|
| 首 token 延迟 P95 | `time` + 第一行输出时间戳，断言 < 阈值 |
| chunk 完整性 | 收完所有 chunk 拼接 == provider raw response |
| 心跳 | 长连接（>30s）期间收到 `:keepalive\n\n` |
| 客户端断开 | client close 后服务端 ctx cancel、span end、provider 调用 abort（不漏 token 计费） |
| Upstream 错误 | LLM 返回 4xx/5xx 时 SSE 应推 `event: error\ndata: {...}` 而非直接断 |
| Backpressure | client 慢消费时不 OOM（buffer 上限） |

工具：`scripts/e2e-model-sse.sh`，通过 edge-api 的 `/api/v1/admin/models/chat/stream` 代理入口，用 curl `--no-buffer` + 自写解析。

### 3.6 异步事件（NSQ）测试

**对象**：`billing → credits/notification` 的事件流。

**关键测试**：
1. **生产端**：单元测试 mock NSQ producer，断言 message body 符合 schema。
2. **消费端**：集成测试用真 NSQ container，发送一条消息，断言 DB 副作用。
3. **幂等**：同一消息发送 N 次，DB 状态应等价于发送 1 次（用 message_id + dedup 表或 upsert）。
4. **死信**：消费失败 N 次后必须进入 `<topic>_dlq`，不能无限重试堵塞队列。
5. **顺序**：如果业务依赖顺序（如同一订单状态机），必须用单一 channel + 串行消费，并加测试覆盖。

## 4. 测试数据管理

| 类型 | 位置 | 规则 |
|---|---|---|
| Fixture（静态） | `services/<svc>/test/fixtures/*.json` | 版本入仓；新增字段需向后兼容 |
| Factory（动态构造） | `services/<svc>/test/factory/` | 用 builder pattern，避免 fixture 爆炸 |
| 数据库初始化 | testcontainers + `init.sql` / `init.js` | 每个测试独立 DB，不共享 |
| 敏感数据 | **禁止入仓** | 用 env / `.env.test`（gitignore） |

**禁忌**：
- ❌ 在共享 dev DB 上跑测试
- ❌ 测试之间依赖执行顺序
- ❌ 测试里 `time.Sleep` 等异步（用 `eventually` 轮询 + 超时）

## 5. CI 集成

**分级触发**：

| 触发时机 | 跑什么 | 目标耗时 |
|---|---|---|
| 每次 push（feature 分支） | unit + lint + idl-compat | < 3 min |
| PR open / update | + integration + openapi schema | < 10 min |
| Merge to develop | + E2E 关键 4 链路 | < 20 min |
| Nightly | + E2E 全集 + 性能 smoke | < 60 min |

**Makefile target**（建议新增）：
```makefile
test-unit:          go test ./... -count=1 -race -short
test-integration:   go test ./... -count=1 -race -tags=integration
test-e2e:           bash scripts/e2e-all.sh
test-contract:      bash scripts/idl-compat.sh && bash scripts/openapi-validate.sh
test-all:           test-unit test-integration test-contract test-e2e
```

## 6. 测试编写 Checklist（PR 提交前）

```
□ 新增 / 修改 API：是否补了对应层级的测试？
□ bugfix：是否带可复现 bug 的回归测试？（红线）
□ 是否覆盖：正常路径 + 边界（空/零/超长） + 错误路径 + 并发？
□ 测试是否独立：能单独跑、能任意顺序跑、能并行跑？
□ 是否清理：DB 记录、Redis key、上传文件、goroutine？
□ 是否使用了 -race？
□ 是否有 time.Sleep（应改为 eventually 轮询）？
□ Mock 是否真的验证了行为，而非"调用过 mock"？
□ IDL 变更：是否跑了 make idl-compat / make test-contract？是否通知所有消费方？
□ 新 HTTP 接口：是否在 OpenAPI 里登记？是否被 schema 校验覆盖？
```

## 7. 反模式（明确禁止）

- ❌ 把 E2E 当主力测试（金字塔倒置，慢 + flaky）
- ❌ Mock 一切（测了寂寞，连 SQL/HTTP 协议错都测不出）
- ❌ 测试与生产共享 DB / Redis / NSQ
- ❌ 测试里 `time.Sleep(5 * time.Second)`（用 `assert.Eventually`）
- ❌ Skip / `t.Skip` 不修就不修，挂半年
- ❌ 用日志输出代替 assert（"我看到日志说成功了"）
- ❌ IDL 改完不跑消费方测试

## 8. 与现有规范的关系

- `universal/quality.md` 定义通用 T0–T3 分级与门禁，本文落地为本项目的具体命令与协议测试方案。
- `project/coding.md` 给基础测试要求（race、count=1、go mod tidy），本文延伸到 API 协议层。
- `project/code-review.md §3.8` 测试维度，reviewer 用本文作为"测试是否充分"的判断依据。
- `project/observability.md` 的 trace/metrics 也是 SSE / 异步链路 e2e 的验证手段。
- 落地路线、度量指标与季度回顾计划见 `.axm/progress/quality/`（progress 范畴，非规范本身）。
