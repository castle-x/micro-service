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

# QUAL-03 契约 CI 卡口

## 实施进度

- 业务状态：`in-progress`

## 目标

让 Thrift IDL 不兼容变更与 OpenAPI 漂移在 PR 阶段被拦截，不进 develop。

## 验收标准

### AI 自动验收

- [x] `scripts/idl-compat.sh` 落地、bash -n / HEAD smoke 通过
- [x] `scripts/openapi-validate.sh` 落地
- [x] `Makefile idl-compat / openapi-validate / test-contract` target
- [ ] CI 中 `redocly` 自动安装（npm i -g @redocly/cli）
- [ ] PR 必跑 `make test-contract`，failed 阻塞合并
- [ ] 至少 1 次主动验证：手动制造 IDL 不兼容变更 → CI 应该红

### 人类验收

- [ ] major 版本 bump 的 bypass 说明与消费方升级计划由 reviewer 确认

## 实施步骤

### Step 1 — CI 安装 redocly

`.github/workflows/ci.yml` 已含 contract job，确认 redocly 安装步骤：

```yaml
contract:
  steps:
    - uses: actions/setup-node@v4
      with: { node-version: '20' }
    - run: npm i -g @redocly/cli
    - run: make test-contract
      env:
        BASE_REF: origin/${{ github.base_ref || 'develop' }}
```

### Step 2 — 主动失败演练

在 feature 分支制造一个故意不兼容的 IDL 变更：

```thrift
// 原：1: required string email
// 改：1: required string username   // fid 复用，应该被拦截
```

跑 `make idl-compat`，预期：

```
✗ [fid reused] idl/iam/iam.thrift: User.1 was 'email', now 'username'
```

确认拦截后还原。

### Step 3 — 设置例外通道

major 版本 bump 时允许显式 bypass：

```bash
IDL_COMPAT_ALLOW_BREAKING=1 make idl-compat
```

约束：bypass 必须在 PR 描述中说明 **为什么** 以及 **所有消费方升级计划**。

## 影响面

- 所有 IDL 变更 PR：CI 多一项 30 秒检查
- 所有 `services/llm` 路由变更：必须同步更新 `idl/llm/openapi.yaml`

## 回滚

- 注释 `.github/workflows/ci.yml` 中 contract job 即可关闭
- 脚本本身可独立运行，与生产代码无耦合
