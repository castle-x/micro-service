<!-- axm-meta
status: active
last-reviewed: 2026-05-14
owner: castlexu
entries:
  - path: log.md
    title: dev-ops BUG 看板
    when-to-read: 查看 dev-ops 范围内 BUG 状态分布、优先级、负责人
  - path: bug-2026-05-14-dev-start-exit-zero-on-failure.md
    title: dev-start 子服务失败时整脚本仍 exit 0
    when-to-read: 修复 / 验收 scripts/dev/start.sh 的失败传播
  - path: bug-2026-05-14-edge-api-missing-etcd-readiness.md
    title: edge-api / model 缺 etcd readiness check
    when-to-read: 修复 / 验收 readyz 探测完整性
  - path: bug-2026-05-14-model-encrypt-key-missing-from-env-template.md
    title: MODEL_ENCRYPT_KEY 未纳入 model.env.example 与 check-env
    when-to-read: 修复 / 验收 model.env.example 与 check-env REQUIRED_KEYS
  - path: bug-2026-05-14-gitignore-blocks-shared-env-files.md
    title: .gitignore 误把 infra.env / observability.env 也排除
    when-to-read: 修复 / 验收 deployments/env 的 git 白名单
  - path: bug-2026-05-14-dev-start-failure-path-unverified.md
    title: dev-start 30s 超时失败路径未端到端验证
    when-to-read: 在前置 BUG 修复后端到端验证失败诊断输出
-->
# dev-ops/bugs — 本地开发运维改造 BUG

`progress/dev-ops/` 实施过程中暴露的 5 条 BUG。看板入口：[`log.md`](./log.md)。

事实唯一来源是单条 BUG 文档；与看板冲突时以单条文档为准。
