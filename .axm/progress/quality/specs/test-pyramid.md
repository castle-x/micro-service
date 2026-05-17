<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
workflow-state: in-progress
state-updated: 2026-05-17
related:
  - ../roadmap.md
  - ../../../project/api-testing.md
-->

# QUAL-02 测试金字塔补齐

## 实施进度

- 业务状态：`in-progress`

## 目标

把每个服务从"只有零散单测"提升到"unit + integration + 可被 e2e 覆盖"三层完整。

## 验收标准

### AI 自动验收

- [x] `Makefile` 提供 `test-unit / test-integration` target
- [x] `services/iam/test/integration/` 模板 + README 落地
- [ ] 其他 7 个服务（idp、iam、billing、credits、notification、asset、edge-api、model）每个至少 1 个集成测试 case
- [ ] `biz/` 单测覆盖率统计基线值入仓（`docs/coverage-baseline.md`）
- [ ] CI integration job 跑通且 < 10 分钟

### 人类验收

- [ ] 抽查每个服务首个集成测试 case 是否覆盖代表性业务路径

## 实施步骤

### Step 1 — 模板复制

按服务依次执行：

```bash
cp -r services/iam/test/integration services/<svc>/test/integration
# 修改 import path 与示例业务调用
cd services/<svc> && go test ./test/integration/... -count=1 -race -tags=integration
```

按依赖深度推进：`idp → iam`（已有）→ `notification → credits → billing → asset → edge-api`（最后，因为依赖前述）。

### Step 2 — testcontainers 依赖统一

在每个服务的 `go.mod` 加：

```
github.com/testcontainers/testcontainers-go v0.27.0
github.com/stretchr/testify v1.9.0
```

> 不要把 testcontainers 加到生产依赖；只在 `_test.go` 文件里 import。

### Step 3 — 覆盖率基线

```bash
make test-unit
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -1   # 总覆盖率
```

把当前数字记到 `docs/coverage-baseline.md`，作为后续不能下降的基线。**不要**强行追求高数字——`pkg/utils` 这种容易测的自然高，`services/edge-api/handler` 走真实链路的天然低。

### Step 4 — 各服务首个 integration case 建议

| 服务 | 推荐首个 case |
|---|---|
| idp | 邮箱+密码登录全链路：写库 → 读库 → 签 token |
| iam | 用户创建 → 角色绑定 → 权限校验 |
| billing | 订单创建 → 支付回调 → 状态机推进 |
| credits | 充值流水 → 扣费幂等 → 余额一致 |
| notification | 模板渲染 → 队列入队 → mock provider |
| asset | 上传记录 → OSS mock → 状态回调 |
| edge-api | 鉴权中间件 + 1 个代表性 RPC proxy |
| model | provider mock + SSE chunk 完整性（部分与 e2e 重叠，集成测试侧重纯本进程逻辑） |

## 影响面

- CI 时长增加 ~5-10 分钟（integration job）
- 本地开发：默认不跑 integration（build tag 隔离），无影响

## 回滚

- 删除 `_test.go` 即可，不影响生产代码
- Makefile target 用 `for ... if -d` 兜底，无目录的服务自动跳过
