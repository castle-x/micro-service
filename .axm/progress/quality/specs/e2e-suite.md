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

# QUAL-04 E2E 关键链路

## 实施进度

- 业务状态：`in-progress`

## 目标

覆盖 4 条 micro-service 最易出事的跨服务链路 + SSE 专项，提供"上线前最后一道闸"。

## 验收标准

### AI 自动验收

- [x] `scripts/e2e-all.sh` 调度器（支持 `E2E_ONLY` 选择性跑）
- [x] `scripts/e2e-google-auth.sh` 登录链路（已存量）
- [x] `scripts/e2e-asset-oss-upload.sh` 资产上传（已存量）
- [x] `scripts/e2e-model-sse.sh` SSE 专项（6 项断言）
- [ ] `scripts/e2e-billing.sh` 支付扣费 + NSQ 事件验证
- [ ] CI develop 分支 push 触发 `make test-e2e`

### 人类验收

- [ ] 4 条链路连续 7 天 0 flaky（每日 nightly 跑）

## 实施步骤

### Step 1 — billing E2E 脚本

`scripts/e2e-billing.sh`：

1. 登录拿 token
2. 创建订单（edge-api → billing）
3. 模拟支付回调（webhook 签名校验）
4. 轮询 credits 服务，断言积分到账
5. 轮询 notification 服务，断言事件被消费
6. **幂等验证**：重发回调 2 次，credits 余额不变（验证 NSQ at-least-once 处理）
7. **死信验证**（可选）：故意制造消费失败，断言消息进 `<topic>_dlq`

实现参考 `scripts/e2e-model-sse.sh` 风格：纯 curl + jq，单文件 < 200 行。

### Step 2 — CI 集成

`.github/workflows/ci.yml` e2e job：

```yaml
e2e:
  if: github.ref == 'refs/heads/develop' || github.ref == 'refs/heads/main'
  steps:
    - run: cp .env.example .env
    - run: make infra-up
    - run: make build
    - run: make test-e2e
    - if: failure()
      uses: actions/upload-artifact@v4
      with:
        name: e2e-logs
        path: bin/log/
```

### Step 3 — Flaky 治理

每条 E2E 必须：
- **幂等**：跑两遍结果一致
- **自带 cleanup**：失败后不留垃圾数据
- **超时明确**：每条 < 60s
- **失败可定位**：失败时 dump `bin/log/{service}.log`

发现 flaky 立即 quarantine（标记 `# FLAKY-<日期>`），1 周内修；超期 = 删除该 case。

## 影响面

- CI 时长：develop merge 多 ~15 分钟
- 本地：可单跑某一条（`E2E_ONLY=billing bash scripts/e2e-all.sh`）

## 回滚

- 单条 E2E 脚本独立，删除某条不影响其他
- `scripts/e2e-all.sh` 找不到脚本会 SKIP 而非 FAIL
