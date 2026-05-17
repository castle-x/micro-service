<!--
PR 模板 — micro-service
依据：.axm/project/code-review.md §4 + .axm/project/api-testing.md §7
请填空，不要删模板。
-->

## 动机（Why）

<!-- 为什么做这个改动？关联 issue / spec / .axm/progress/ 路径 -->

## 方案（What）

<!-- 改了什么？拆 1-3 个 bullet 即可，长说明放到 .axm/progress/ 文档 -->

-

## 影响面 (Blast Radius)

<!-- 列出改到的服务 / 接口 / 数据表 / 配置项 -->

- 服务：
- 接口：
- DB schema：
- 配置：

## 验收方式 (How to Verify)

<!-- reviewer 怎么本地验证？粘命令或步骤 -->

```bash
# 例：
# make test-unit
# bash scripts/e2e-model-sse.sh
```

## 回滚方式 (Rollback Plan)

<!-- 出问题怎么 1 步回滚？revert commit / 还原配置 / 跑 migration down -->

---

## 作者自审 Checklist

提交前请逐项确认（依据 `.axm/project/code-review.md §4`、`.axm/project/api-testing.md §7`）：

- [ ] 已本地跑：`make fmt && make lint && make test`（按分级补 integration / e2e）
- [ ] diff 中无"顺手改"的无关变更（AGENTS.md §3 外科手术式修改）
- [ ] **测试**：新增 / 修改的 API 都有对应层级的测试（unit / integration / contract / e2e）
- [ ] **bugfix**：带可复现 bug 的回归测试（红线，无测试不算修完）
- [ ] **IDL 变更**：跑过 `make idl-compat`，已通知所有消费方 owner
- [ ] **OpenAPI 变更**：跑过 `make openapi-validate`，spec 与代码同步
- [ ] **DB schema 变更**：附 migration up/down 脚本
- [ ] **新依赖**：跑过 `go mod tidy`，已说明引入理由
- [ ] **敏感字段**：未把 password/secret/token/authorization/手机号等写进日志/错误/trace
- [ ] **新 I/O 链路**：按 `.axm/project/observability.md` 起 span、加 metric
- [ ] **race**：`go test -race` 无报警
- [ ] 已删除 `fmt.Println`、`TODO`、注释掉的死代码

## Reviewer 注意事项

> reviewer 用 `.axm/project/code-review.md §2` 的优先级标记评论：
> 🔴 Blocker（阻塞合并） / 🟡 Suggestion（需作者回应） / 💭 Nit / ❓ Question / 👍 Praise

## 关联

- Issue:
- .axm/progress:
- 设计文档:
