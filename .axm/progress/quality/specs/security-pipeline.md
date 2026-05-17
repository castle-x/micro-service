<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
priority: P0
related:
  - ../roadmap.md
  - ../../../project/code-review.md
-->

# QUAL-06 安全扫描流水线（SAST / SCA）

## 实施进度

- 业务状态：`pending`

## 背景

SAST（静态扫描代码）和 SCA（依赖漏洞扫描）是成本最低、收益最高的安全测试。本仓库目前 `make lint` 仅 `go vet` + 可选 `golangci-lint`，未接安全维度。

## 解决的根本问题

- **代码层已知漏洞模式**：硬编码密钥、SQL 拼接、不安全随机数、TLS 配置错误等可由模式匹配检测的漏洞。
- **依赖供应链漏洞**：第三方库的 CVE（如 jwt-go 的算法绕过、yaml.v2 的拒绝服务）。

> 边界：不解决"代码对、配置错"的运行时漏洞（那是 QUAL-15 DAST），不解决业务逻辑漏洞（如折扣可叠加），不解决鉴权策略错误（需手工渗透）。

## 触发条件

- 每个 PR：增量代码 SAST + 完整依赖 SCA
- nightly：全量 SAST 复扫，捕捉新发布的 CVE

## 验收标准

### AI 自动验收

- [ ] `make lint` 集成 `gosec`、`staticcheck`、`govulncheck`
- [ ] `.github/workflows/ci.yml` PR job 加入安全扫描步骤
- [ ] 现存告警全部清零或 baseline 化

### 人类验收

- [ ] 文档：安全告警的分级与处理流程（高 = 阻塞、中 = 周内修、低 = 季度清理）

## 工具候选

| 类型 | 工具 | 备注 |
|---|---|---|
| SAST（Go） | `gosec` | 通用安全规则 |
| SAST（深度静态） | `staticcheck` | bug 模式 + 死代码 |
| SAST（多语言） | `semgrep` | 自写规则灵活性最高，YAML 模板 |
| SCA（Go 依赖 CVE） | `govulncheck`（官方） | 仅扫真实调用链上的漏洞 |
| SCA（容器/全栈） | `trivy fs` | 同时扫 Go/npm/Dockerfile |

## 待展开问题

- 现存告警基数？是否需要 baseline 抑制后逐步收紧？
- staticcheck 已在 QUAL-01 中规划接入，本阶段是否复用同一 lint target？
- 是否引入 SARIF 格式输出到 GitHub Security tab？
