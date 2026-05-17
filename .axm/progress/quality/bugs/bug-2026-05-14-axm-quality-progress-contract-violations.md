<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: bug
initiative: quality
related:
  - ../roadmap.md
-->

# bug-2026-05-14-axm-quality-progress-contract-violations — .axm/progress/quality/ 47 个契约违规

## 元信息

| 字段 | 值 |
|---|---|
| ID | `bug-2026-05-14-axm-quality-progress-contract-violations` |
| 所属 initiative | `quality` |
| 提交人 | review-agent（DevOps） |
| 提交时间 | 2026-05-14 |
| 优先级 | P3 |
| 严重度 | Minor |
| 当前状态 | `fixed` |
| 影响模块 | `.axm/progress/quality/`（roadmap.md + 15 个 specs） |
| 影响版本 | quality initiative 创建时 |
| 关联 PR / commit | 本地未提交 |
| 关联 spec / roadmap | `../roadmap.md` |

## 复现步骤

1. `cd /Users/castlexu/github/micro-service`
2. `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=. 2>&1 | tail -10`

## 期望表现

- `Summary: 0 error(s), 0 warning(s)` 或仅 warning

## 实际表现

```
Summary: 47 error(s), 0 warning(s)
```

错误全部集中在 `.axm/progress/quality/` 下的 16 个文件，主要类别：

- `progress-type` 字段缺失
- `initiative` 字段缺失
- `status: in-progress` 不在合法集 `active/draft/deprecated`

## 影响范围

- `axm validate` 无法 exit 0，CI 集成 axm 校验时整仓被卡
- AI 在跑 axm validate 看到 47 个 error 容易把注意力浪费在 quality 上而忽略其他真问题

## 根因分析

quality initiative 的 progress 文档创建时未遵循 `axm bug-doc-guide.md` / `progress-doc-guide.md` 的元数据契约：
- 把状态机词（`in-progress`）当作 axm `status` 字段值
- 漏写 `progress-type` 与 `initiative`

## 修复验收标准

### 修复约束

1. **必须**：把每份 quality progress 的 `status: in-progress` 改为合法值（应该是 `active`，因为是仍在推进的 progress）
2. **必须**：给每份补 `progress-type`（roadmap 或 spec）
3. **必须**：给每份补 `initiative: quality`
4. **不允许**绕过 validate（例如在脚本里加 quality/ 排除）
5. 修复后 `axm validate` 在本仓库整体应 `0 errors`
6. 只改元数据，不动正文

### AI 自动验收

- [x] `node /Users/castlexu/.codex/skills/axm/scripts/validate.mjs --target=/Users/castlexu/github/micro-service` 输出 `Summary: 0 error(s), 0 warning(s)`
- [x] `node /Users/castlexu/.codex/skills/axm/scripts/reindex.mjs --target=/Users/castlexu/github/micro-service --dry-run` 无意外变更
- [x] 回归：`bash scripts/dev/self_check.sh` 仍 ok
- [x] quality/ 目录文件数量与名字未变（只改元数据和验收结构）

### 人类验收

- [ ] 抽查 3 份 quality spec 的元数据合法性
- [ ] 不出现"为通过校验把状态从 in-progress 改成 closed"等语义漂移

## 时间线

| 时间 | 状态 | 操作人 | 说明 |
|---|---|---|---|
| 2026-05-14 | open | review-agent | 提交 BUG，与 dev-ops 解耦 |
| 2026-05-17 | in-progress | 主 agent / quality worker | 按最新版 axm 契约修复 quality roadmap/spec 元数据，并整理验收结构 |
| 2026-05-17 | fixed | 主 agent | `validate.mjs`、`reindex.mjs --dry-run` 与 `scripts/dev/self_check.sh` 通过；本地未提交 |
