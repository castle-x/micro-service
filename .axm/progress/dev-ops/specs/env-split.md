<!-- axm-meta
status: active
last-reviewed: 2026-05-14
owner: castlexu
progress-type: spec
initiative: dev-ops
related:
  - ../roadmap.md
  - process-lifecycle.md
-->

# DEV-04：.env 拆分与启动前校验

## 背景

当前根目录 `.env.example` 约 100 行，混杂 5 类不同生命周期的变量：

- **infra 连接信息**：基本不变，可提交模板（MONGO_URI / REDIS_ADDR / ETCD_ENDPOINT / 端口注册）
- **可观测**：OTEL_ * / OPENOBSERVE_*，基本不变
- **业务参数**：ASSET_* 等，按服务归类，会持续增加
- **第三方凭据**：JWT_SECRET / GOOGLE_* / ALIPAY_* / ALIYUN_OSS_*，敏感且变更频繁
- **个人覆盖**：调试时临时调整端口/日志级别

混在单文件时的问题：

- 单文件持续膨胀，AI Read 后期需要分页
- 占位符（`your-...` `change-me-...`）忘改要等服务运行时才报错
- 凭据与非凭据混排，团队成员复制 .env 时心理负担大

## 目标

- `.env` 拆成职责清晰的多个分组文件，每文件 ≤ 30 行
- 启动前由脚本机械合并加载，加载顺序明确（后者覆盖前者）
- 启动前自动校验：占位符未替换、关键变量缺失、文件间重复 KEY 冲突
- 校验输出结构化（JSON），AI 可直接消费

## 范围

- 新增 `deployments/env/` 目录，按职责拆分
- 模板文件 `*.example` 入仓；真实文件 `*.env`（除 `infra.env` `observability.env` 外）全部 gitignore
- 新增 `scripts/dev/check-env.sh`，被 `make dev-start` 前置依赖
- 改造 Makefile 的 `ENV_FILE_VARS` 逻辑：从单文件加载改为按顺序合并多文件

## 非目标

- 不引入 dotenv / direnv 等运行时依赖（保持 Makefile 自包含）
- 不接入 1Password / Vault 等密钥管理（本地 dev 不需要）
- 不强制变量命名重构（保持 `MONGO_URI` `JWT_SECRET` 等现名不变，避免代码大改）

## 已确认开发细节

| 主题 | 决策 |
|---|---|
| 目录布局 | `deployments/env/{infra,observability,asset,model,secrets,overrides}.env` |
| 加载顺序 | `infra → observability → 各业务（asset / model / ...）→ secrets → overrides`，后覆盖前 |
| 提交策略 | `infra.env.example` `observability.env.example` `asset.env.example` `model.env.example` `secrets.env.example` `overrides.env.example` 入仓；同名去掉 `.example` 后缀的真实文件全部 gitignore |
| 向后兼容 | 保留根目录 `.env` 加载（最低优先级），存在则警告"建议迁移到 deployments/env/" |
| 校验规则 1 | 关键变量必填：`MONGO_URI / REDIS_ADDR / ETCD_ENDPOINT / JWT_SECRET / GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / ALIYUN_OSS_ACCESS_KEY_ID / ALIYUN_OSS_ACCESS_KEY_SECRET / OPENOBSERVE_AUTH_HEADER` |
| 校验规则 2 | 占位符检测：值以 `your-` `change-me-` `replace-with-` 开头视为未替换 |
| 校验规则 3 | 重复 KEY：跨文件出现同名变量时给 warn（明确"被后者覆盖"） |
| 校验输出 | stdout JSON：`{"ok": false, "missing": [...], "placeholder": [...], "duplicates": [...]}`；exit 0 ok / exit 1 fail |
| dev-start 集成 | `dev-start` 前置依赖 `check-env`，失败直接 exit 1，stderr 提示如何修复 |
| Makefile 变量展开 | 用 `set -a; source deployments/env/<file>; set +a` 风格在子 shell 内合并，避免 `xargs` 处理引号特殊字符的坑 |

## 设计约束

- `secrets.env` 必须 gitignore，**绝对不能**入仓；CI 通过其他方式注入
- 校验脚本不依赖外部工具（只用 bash + awk + sort），保证全平台可跑
- 拆分迁移期允许"老 .env + 新 deployments/env" 并存，新的优先级更高
- 文件之间的覆盖关系应在 `deployments/env/README.md` 写清楚（写一次，不进 AGENTS.md 主路由）

## AI 自动验收

| 验收项 | 命令或检查 |
|---|---|
| 模板文件齐全 | `for f in infra observability asset model secrets overrides; do test -f deployments/env/$f.env.example; done` |
| 拆分后无变量丢失 | 用旧 `.env.example` 的变量名列表 diff 新拆分文件合集，差集为空（除明确删除的） |
| check-env 通过场景 | 复制全部 .example 为 .env、填好真实值，`make dev-check-env` 输出 `{"ok":true}` exit 0 |
| check-env 失败场景 1（占位） | 故意保留一个 `your-google-client-id...`，`make dev-check-env` exit 1，stdout JSON 含 `placeholder: ["GOOGLE_CLIENT_ID"]` |
| check-env 失败场景 2（缺失） | 删除某个必填变量，`make dev-check-env` exit 1，stdout JSON 含 `missing: ["..."]` |
| check-env 重复 KEY 警告 | 在两个文件里同时定义 `JWT_SECRET`，输出含 `duplicates: ["JWT_SECRET"]` |
| dev-start 集成生效 | 把 secrets.env 的 JWT_SECRET 清空，`make dev-start` 应在 check-env 阶段失败，不再继续 build 与启动 |
| 变量加载顺序符合预期 | 在 `overrides.env` 设 `LOG_LEVEL=debug`，启动后 `bin/log/iam.log` 第一行 level=debug 的日志存在 |

## 人类验收

- 新成员 onboard：clone 仓库后能看着 `deployments/env/README.md` 5 分钟内填好所有 .env
- 单文件长度均控制在 30 行以内，AI 一次 Read 不需要分页
- 误提交保护：`git status` 不会显示已修改的 secrets.env

## 开发进度

- ✅ 2026-05-14：已新增 `deployments/env/{infra,observability,asset,model,secrets,overrides}.env.example` 与 `deployments/env/README.md`，真实 `deployments/env/*.env` 已加入 gitignore。
- ✅ 2026-05-14：已新增 `scripts/dev/check-env.sh`，Makefile `dev-check-env` 已作为 `dev-start / dev-restart / model-* / asset-*` 前置检查。
- ✅ 2026-05-14：主 agent 已验证 env 临时夹具通过、缺失、占位符、重复 key 场景；tester agent focused verification 通过。
- ⏳ 人类验收待执行：按 README 从零复制并填写本地 env，确认 5 分钟内可完成且 `make dev-check-env` 输出可读。

## 风险与回退

- 风险：迁移期同时存在新旧 env 文件可能让加载顺序与预期不符；建议合并 PR 时一并删除根 `.env.example`，只保留 `deployments/env/*.example`
- 回退：保留根目录 `.env` 加载逻辑作为兜底，回退时仅删除 `deployments/env/` 目录与 check-env 集成
