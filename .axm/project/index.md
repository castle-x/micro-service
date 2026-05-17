<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-12
owner: castlexu
entries:
  - path: architecture.md
    title: 项目架构规范
    when-to-read: 涉及模块边界、服务调用、IDL、数据访问或新增模块时
  - path: coding.md
    title: 项目编码规范
    when-to-read: 编写 Go 代码、调整测试/lint/build 命令或引入依赖时
  - path: observability.md
    title: OpenTelemetry 可观测性规范
    when-to-read: 接入 trace/metrics/log、排查后端问题或新增 I/O 链路时
  - path: code-review.md
    title: 代码审查规范
    when-to-read: 提交 PR、做 reviewer、定义合并门禁或度量审查质量时
  - path: api-testing.md
    title: API 测试体系
    when-to-read: 设计或编写 HTTP/RPC/SSE/MQ 接口测试、补充契约测试或配置 CI 测试门禁时
-->


# project/ — 项目规范

绑定本项目的具体工程规范。所有文件使用 axm metadata 骨架 A（`applies-to: [project:<name>, ...]`）。

> **本 index 的 `entries` 由 AI 在 Phase 3 Author 阶段按项目实际情况填充**，填充后可运行 `node scripts/reindex.mjs` 自动同步。

## 建议包含的规范文件

按项目实际需要选择性创建：

| 文件 | 内容 | 何时读取 |
|---|---|---|
| `architecture.md` | 模块划分、依赖方向、IPC/API 契约、数据库 Schema | 涉及模块间交互或新增模块时 |
| `coding.md` | 语言风格、lint 规则、命名约定、路径别名 | 编写或修改代码时 |
| `observability.md` | OpenTelemetry trace/metrics/log 规范、准入准出与排障流程 | 新增链路或排查后端问题时 |
| `design.md` | 设计系统（配色、字体、组件规范） | UI 项目需要时 |

参考 `references/project-spec-guide.md` 获取各技术栈的 project 规范写作要点。
