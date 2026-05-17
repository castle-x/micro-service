<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
related:
  - ../roadmap.md
  - ../../../project/code-review.md
-->

# QUAL-01 代码审查规范推广

## 实施进度

- 业务状态：`in-progress`

## 目标

将 `project/code-review.md` 从文档转化为团队默认行为：每个 PR 都按规范走，🔴/🟡/💭/❓/👍 标记成为评论默认形式，pkg/idl 变更强制双 reviewer，风格类问题完全自动化。

## 验收标准

### AI 自动验收

- [x] `.github/pull_request_template.md` 落地，新 PR 自动渲染自审 Checklist
- [ ] `.github/CODEOWNERS` 落地，`pkg/`、`idl/` 路径强制双 reviewer
- [ ] `make lint` 集成 `gosec`、`staticcheck`，零新增告警

### 人类验收

- [ ] 团队内 1 轮宣讲：优先级标记、合并门禁、reviewer 行为准则
- [ ] 前 4 周抽查 10 个 PR，其中 ≥ 8 个评论使用了优先级标记

## 实施步骤

### Step 1 — CODEOWNERS

创建 `.github/CODEOWNERS`：

```
# pkg/ 任何变更需要 2 人 review
/pkg/                @castlexu @<owner2>

# IDL 变更需要消费方 + 提供方共同 review
/idl/                @castlexu @<owner2>

# 部署配置需运维 review
/deployments/        @<ops-owner>
/.github/workflows/  @<ops-owner>
```

> 实际 owner 名单待团队确认后填入。

### Step 2 — lint 工具链增强

修改 `Makefile lint` target：

```makefile
lint:
	@cd pkg && go vet ./...
	@for svc in $(ALL_SERVICES); do cd services/$$svc && go vet ./... && cd ../..; done
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... ; \
	fi
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./pkg/... ./services/... ; \
	fi
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./pkg/... ./services/... ; \
	fi
```

`.golangci.yml`（仓库根）启用：`gosec`、`staticcheck`、`errcheck`、`govet`、`ineffassign`、`unused`。

### Step 3 — 宣讲与抽查

- 1 次 30 分钟内部分享：优先级标记的语义、典型评论模板（见 `code-review.md §6`）
- 4 周后随机抽查 10 个 PR，统计：
  - 评论是否带优先级前缀
  - 🔴 是否真的阻塞合并
  - 🟡 是否被作者明确回应

## 影响面

- 所有新开 PR：必须填 PR 模板
- `pkg/`、`idl/` 变更：流程变慢（需 2 人 review）
- lint：可能涌出存量 gosec/staticcheck 告警，需要 1 次集中清零

## 回滚

- 移除 `.github/CODEOWNERS` 即关闭强制双 reviewer
- `make lint` 中 gosec/staticcheck 用 `command -v` 兜底，未安装不报错
