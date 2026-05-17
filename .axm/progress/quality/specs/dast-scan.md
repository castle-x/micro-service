<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
workflow-state: ready
state-updated: 2026-05-17
priority: P3
related:
  - ../roadmap.md
  - ./security-pipeline.md
-->

# QUAL-15 动态安全扫描 (DAST)

## 实施进度

- 业务状态：`pending`

## 背景

QUAL-06 SAST/SCA 扫**代码本身**和**依赖**。但很多漏洞**不在代码里**，而在**运行时的组合行为**——SAST 永远看不到。DAST 从攻击者视角扫**正在跑**的系统。

> 当前 edge-api 暴露面小，DAST ROI 在 P3。一旦 edge-api 真正对外（前端接入或上预发），升级到 P1。

## 解决的根本问题

DAST 真正的杀手锏不是"找代码 bug"，而是**"代码对、配置错"** 这一类问题：

| 真实场景 | SAST 能发现吗 | DAST 能发现 |
|---|---|---|
| 代码完美但 Kong 把响应缓存到 CDN（含敏感字段） | ❌ | ✅ |
| 代码有鉴权中间件但 K8s ingress 漏配某路径 | ❌ | ✅ |
| /metrics、/debug/env 暴露在公网 | ❌ | ✅ |
| TLS 配置允许 SSLv3 / 心血漏洞 | ❌ | ✅ |
| 用户 A token 访问 /api/v1/users/B 返回 200（IDOR） | 部分 | ✅ |
| 错误响应泄漏堆栈或 SQL | 部分 | ✅ |

**所有"代码层测试"都拦不住部署/配置层漏洞**。

> 边界：找不到业务逻辑漏洞（如"折扣可叠加 100 次"），那是手工渗透 + 业务测试的事。误报多，需要白名单调优。

## 触发条件

- 上线前置（任何对外环境 deploy 前）：必跑 DAST
- nightly：跑 nuclei 模板扫已知 CVE / exposures
- 每周：完整 ZAP 主动扫描

## 验收标准

### AI 自动验收

- [ ] DAST 测试环境就绪（独立 namespace，避开 dev）
- [ ] `scripts/dast/` 目录约定
- [ ] `nuclei` 模板扫接 nightly CI
- [ ] ZAP 主动扫接周度 cron

### 人类验收

- [ ] 漏洞分级与处理流程（高 = 阻塞、中 = 周内修、低 = 季度清理）

## 工具候选

| 工具 | 用途 |
|---|---|
| `nuclei` | YAML 模板表达攻击模式，CVE / exposures / misconfigurations |
| `OWASP ZAP` | 主动扫 + 被动扫，业界标准 |
| `Burp Suite` | 商业，手工渗透更顺手 |
| `wapiti` | 简单替代品 |

## 本仓库高优先级扫描场景

- JWT 攻击：`alg: none`、kid 注入、token 重放
- IDOR：A 租户 token 访问 B 租户资源
- 接口枚举：所有 `/api/v1/**` 是否都鉴权
- 错误信息泄漏：构造非法输入看响应是否泄漏堆栈/SQL
- 敏感端点：/metrics, /debug/pprof, /healthz 是否在公网且无鉴权
- 文件上传：MIME 伪造、路径穿越、未限大小（asset 服务）
- 第三方回调签名绕过（billing 支付回调、asset OSS 回调）

## 启动条件

DAST 真正有意义的前提：
1. 有独立的、与生产同构的测试环境（不能扫开发库）
2. 有"明确的攻击面定义"（哪些是对外的、哪些是内网的）
3. 有专人处理告警（避免变成"扫一堆没人看"）

## 待展开问题

- DAST 测试环境是否独立 K8s namespace 还是独立 docker-compose？
- 是否引入 SARIF 输出到 GitHub Security tab？
- nuclei 模板更新策略（社区模板每周更新，是否每次都拉最新）？
