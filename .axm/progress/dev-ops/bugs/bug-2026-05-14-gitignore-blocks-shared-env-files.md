<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: bug
initiative: dev-ops
related:
  - ../specs/env-split.md
  - ../roadmap.md
-->

# bug-2026-05-14-gitignore-blocks-shared-env-files — .gitignore 误把 infra.env / observability.env 也排除

## 元信息

| 字段 | 值 |
|---|---|
| ID | `bug-2026-05-14-gitignore-blocks-shared-env-files` |
| 所属 initiative | `dev-ops` |
| 提交人 | review-agent（DevOps） |
| 提交时间 | 2026-05-14 |
| 优先级 | P1 |
| 严重度 | Major |
| 当前状态 | `closed` |
| 影响模块 | `.gitignore`、`deployments/env/` |
| 影响版本 | dev-ops initiative 首版 |
| 关联 PR / commit | 本地未提交 |
| 关联 spec / roadmap | `../specs/env-split.md`、`deployments/env/README.md` |

## 复现步骤

1. `cd /Users/castlexu/github/micro-service`
2. `touch deployments/env/infra.env deployments/env/observability.env deployments/env/secrets.env deployments/env/model.env`
3. `git check-ignore -v deployments/env/infra.env`
4. `git check-ignore -v deployments/env/observability.env`
5. 清理：`rm deployments/env/infra.env deployments/env/observability.env deployments/env/secrets.env deployments/env/model.env`

## 期望表现

- `infra.env` 与 `observability.env` **不被 ignore**（属于团队共享的稳定连接信息，spec 第 ④ 条 "infra/observability 入仓"）
- `secrets.env` `overrides.env` `asset.env` `model.env` 仍被 ignore（含密钥或个人覆盖）

## 实际表现

- 当前 `.gitignore`：
  ```
  deployments/env/*.env
  ```
- 通配过广，**所有** `*.env` 都被 ignore，包括 infra/observability
- 团队成员无法通过 git 拉取统一的本地连接信息，每人手填，易漂移

## 影响范围

- 与 spec DEV-04 与 `deployments/env/README.md` 的承诺直接矛盾
- 团队 onboarding 体验下降：需要口头/文档传播 infra.env 内容

## 根因分析

实施 env-split 时只考虑了"防止误提交密钥"，没考虑 spec 中"infra/observability 入仓"的双向约束；通配规则简单一刀切。

具体表现是 `.gitignore` 使用 `deployments/env/*.env` 后没有用 negation 白名单恢复共享文件，因此 Git 同时排除了 `infra.env` / `observability.env` 和真正不应提交的 `secrets.env` / `model.env` / `asset.env`。此外当前仓库只有对应 `.example`，没有可入仓的 shared 默认 `.env`，导致即使修正 ignore 规则，新成员仍无法直接拉到共享默认连接信息。

## 修复验收标准

### 修复约束

1. **必须**：让 `deployments/env/infra.env` 与 `deployments/env/observability.env` 可被 git track
2. **必须**：`secrets.env` `overrides.env` 继续 ignore
3. **决策**：`asset.env` `model.env` 倾向继续 ignore（含 OSS bucket / encrypt key 等敏感信息），强制走 `.example` + check-env 校验
4. **推荐写法**（白名单 negation）：
   ```gitignore
   deployments/env/*.env
   !deployments/env/infra.env
   !deployments/env/observability.env
   ```
5. 修改 `.gitignore` 后**必须**跑 `git status` 看是否有非预期文件进入跟踪

### AI 自动验收

- [x] 行为：执行复现步骤 1-4，`git check-ignore -q deployments/env/infra.env` 退出非零（不被 ignore）
- [x] 行为：`git check-ignore -q deployments/env/observability.env` 退出非零
- [x] 行为：`git check-ignore -q deployments/env/secrets.env` 退出 0（被 ignore）
- [x] 行为：`git check-ignore -q deployments/env/model.env` 退出 0
- [x] 行为：`git check-ignore -q deployments/env/asset.env` 退出 0
- [x] 回归：`git status` 不出现意外的 staged 文件
- [x] 回归：`scripts/dev/self_check.sh` 仍 ok

### 人类验收

- [x] 模拟新成员 clone 仓库，能在 `deployments/env/` 下直接看到 `infra.env`（含真实可工作的本地端口与 URI）
- [x] 故意 `git add deployments/env/secrets.env` 失败或被 ignore

## 时间线

| 时间 | 状态 | 操作人 | 说明 |
|---|---|---|---|
| 2026-05-14 | open | review-agent | 提交 BUG |
| 2026-05-14 | in-progress | dev-ops BUG 修复 worker | 接手修复，确认 `.gitignore` 通配缺少 infra/observability 白名单，且仓库仅有 `.example` 默认文件 |
| 2026-05-14 | fixed | dev-ops BUG 修复 worker | `.gitignore` 增加 infra/observability negation 白名单，复制对应 `.example` 为 shared 默认 `.env`；自动验收通过，关联 commit：本地未提交 |
| 2026-05-17 | closed | 主 agent | 用户确认 dev-ops 已开发完成；shared env 与 secrets ignore 行为已闭合 |
