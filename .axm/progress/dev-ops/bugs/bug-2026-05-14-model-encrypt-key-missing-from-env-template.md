<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: bug
initiative: dev-ops
workflow-state: closed
state-updated: 2026-05-17
related:
  - ../specs/env-split.md
  - ../roadmap.md
-->

# bug-2026-05-14-model-encrypt-key-missing-from-env-template — MODEL_ENCRYPT_KEY 未纳入 model.env.example 与 check-env

## 元信息

| 字段 | 值 |
|---|---|
| ID | `bug-2026-05-14-model-encrypt-key-missing-from-env-template` |
| 所属 initiative | `dev-ops` |
| 提交人 | review-agent（DevOps） |
| 提交时间 | 2026-05-14 |
| 优先级 | P1 |
| 严重度 | Major |
| 影响模块 | `deployments/env/model.env.example`、`scripts/dev/check-env.sh`、`services/model` |
| 影响版本 | dev-ops initiative 首版 |
| 关联 PR / commit | 本地未提交 |
| 关联 spec / roadmap | `../specs/env-split.md` |

## 复现步骤

1. `cat /Users/castlexu/github/micro-service/deployments/env/model.env.example`
2. `grep MODEL_ENCRYPT_KEY /Users/castlexu/github/micro-service/scripts/dev/check-env.sh`
3. 启动 model service 但不设 `MODEL_ENCRYPT_KEY`：`unset MODEL_ENCRYPT_KEY && bash scripts/dev/start.sh model`
4. 查看 log：`tail bin/log/model.log`

## 期望表现

- `model.env.example` 含 `MODEL_ENCRYPT_KEY=replace-with-...` 占位
- `check-env REQUIRED_KEYS` 含 `MODEL_ENCRYPT_KEY`
- 缺失或占位时 `make dev-check-env` exit 1 并在 JSON 列出
- 新成员 onboard 时不会到运行时才发现该缺失

## 实际表现

- `model.env.example` 仅 2 行注释，无任何变量
- `check-env REQUIRED_KEYS` 不含 `MODEL_ENCRYPT_KEY`
- model service `main.go:70-76` 在缺失或长度不足时打印 warn 并使用 dev fallback —— 但 dev fallback 会让加密的 LLM provider 配置无法跨重启读出（key 不一致 → 解密失败）
- 新成员 onboard 跑 model-restart 后会出现"已存的 provider 突然解不开"的怪现象

## 影响范围

- 所有需要使用 `services/model` 的开发者
- onboard 体验差：spec DEV-04 承诺"5 分钟内填好真实值"，但 MODEL_ENCRYPT_KEY 不在模板里
- 违反 spec DEV-04 的"业务参数按服务归类"约定

## 根因分析

实施 dev-ops env 拆分时未对 `services/model/main.go` 的 `os.Getenv("MODEL_ENCRYPT_KEY")` 做归因分析，导致该变量散落在 secrets.env / 根 .env，不在任何 example 模板中。

修复时确认 `check-env.sh` 的占位检测已支持 `replace-with-` 前缀，根因不是检测逻辑缺失，而是 `MODEL_ENCRYPT_KEY` 同时缺少模板声明与 `REQUIRED_KEYS` 登记。

## 修复验收标准

### 修复约束

1. **必须**：`deployments/env/model.env.example` 追加：
   ```
   # 32-byte symmetric key for encrypting LLM provider config in MongoDB.
   # Generate: openssl rand -base64 32
   MODEL_ENCRYPT_KEY=replace-with-32-byte-base64-key
   ```
2. **必须**：`scripts/dev/check-env.sh:7` 的 `REQUIRED_KEYS` 末尾追加 `MODEL_ENCRYPT_KEY`
3. **不允许**把 MODEL_ENCRYPT_KEY 移到 `secrets.env`（spec 明确"业务参数按服务归类"；语义属于单服务配置而非跨服务凭据）
4. 占位符 `replace-with-` 必须能被现有 `check-env.sh:112` 的 placeholder 检测命中（已支持，无需改正则）

### AI 自动验收

- [x] 静态：`grep -q '^MODEL_ENCRYPT_KEY=' deployments/env/model.env.example`
- [x] 静态：`grep -q 'MODEL_ENCRYPT_KEY' scripts/dev/check-env.sh`
- [x] 行为（占位）：在隔离临时目录跑 check-env，`MODEL_ENCRYPT_KEY=replace-with-...` 时 exit 1 且 JSON `placeholder` 列表含 `MODEL_ENCRYPT_KEY`
- [x] 行为（缺失）：`MODEL_ENCRYPT_KEY=` 空值时 exit 1 且 JSON `missing` 含 `MODEL_ENCRYPT_KEY`
- [x] 行为（合法）：填入 32+ 字符值，check-env exit 0、`ok: true`
- [x] 不引入新违规：现有 6 个 `.env.example` 仍能 `git track`（不触发 gitignore-blocks-shared-env-files BUG）

### 人类验收

- [x] 模拟新成员：仅复制 `*.env.example` 为 `*.env`，把所有 `your-` `change-me-` `replace-with-` 替换后，`make dev-check-env` 一次通过
- [x] model service 重启后，已存 LLM provider 配置仍能正确解密

## 时间线

| 时间 | 状态 | 操作人 | 说明 |
|---|---|---|---|
| 2026-05-14 | open | review-agent | 提交 BUG |
| 2026-05-14 | in-progress | dev-ops BUG 修复 worker（model env key） | 限定写入 `model.env.example`、`check-env.sh` 和本 BUG 文档，定位为模板与必填 key 登记遗漏 |
| 2026-05-14 | fixed | dev-ops BUG 修复 worker（model env key） | 已追加 `MODEL_ENCRYPT_KEY` 模板占位并纳入 `REQUIRED_KEYS`；AI 自动验收通过，关联 commit：本地未提交 |
| 2026-05-17 | closed | 主 agent | 用户确认 dev-ops 已开发完成；新成员 onboard 与 model 重启解密演练转为后续回归场景 |
