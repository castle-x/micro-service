<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: roadmap
initiative: dev-ops
-->

# dev-ops BUG 看板

> 单条 BUG 文档为事实来源；本看板与单条文档冲突时以单条文档为准。

## 状态分布

| 状态 | 数量 |
|---|---|
| open | 0 |
| in-progress | 0 |
| fixed | 5 |
| verified | 0 |
| closed | 0 |
| reopened | 0 |
| wont-fix | 0 |
| duplicate | 0 |

## 未关闭 BUG

| ID | 标题 | 优先级 | 状态 | 提交日 | 负责人 |
|---|---|---|---|---|---|
| [bug-2026-05-14-dev-start-exit-zero-on-failure](./bug-2026-05-14-dev-start-exit-zero-on-failure.md) | dev-start 子服务失败时整脚本仍 exit 0 | P0 | fixed | 2026-05-14 | 主 agent / process worker |
| [bug-2026-05-14-edge-api-missing-etcd-readiness](./bug-2026-05-14-edge-api-missing-etcd-readiness.md) | edge-api / model 缺 etcd readiness check | P1 | fixed | 2026-05-14 | 主 agent / health worker |
| [bug-2026-05-14-model-encrypt-key-missing-from-env-template](./bug-2026-05-14-model-encrypt-key-missing-from-env-template.md) | MODEL_ENCRYPT_KEY 未纳入 model.env.example 与 check-env | P1 | fixed | 2026-05-14 | 主 agent / env worker |
| [bug-2026-05-14-gitignore-blocks-shared-env-files](./bug-2026-05-14-gitignore-blocks-shared-env-files.md) | .gitignore 误把 infra.env / observability.env 也排除 | P1 | fixed | 2026-05-14 | 主 agent / gitignore worker |
| [bug-2026-05-14-dev-start-failure-path-unverified](./bug-2026-05-14-dev-start-failure-path-unverified.md) | dev-start 30s 超时失败路径未端到端验证 | P2 | fixed | 2026-05-14 | 主 agent / process worker |

## 最近关闭

（暂无）
