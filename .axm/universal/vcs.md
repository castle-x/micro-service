<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
applies-to: [universal]
related:
  - ./devloop.md
-->

# 版本控制规范

## PR（Pull Request）在做什么

**PR 不是"多一个按钮"，而是合并前的协作界面**：把 `feat/xx` 上的一组提交，以**可审阅的 diff、CI 结果、讨论串、审批记录**的形式呈现，**合入目标分支**（本仓库里主要是合入 `develop`）时留痕、可回滚、可与 Issue/里程碑关联。

对多人开发来说，它的意义是：

- **隔离**：各人在 `feat/xx` 上开发，不直接污染集成分支，减少互相踩脚。
- **门控**：合并前跑 lint/test（及未来更多检查），问题拦在进 `develop` 之前。
- **可审计**：合入点清晰（谁、何时、因何合并、关联哪次讨论）。
- **小步集成**：多轮 `feat/xx` → `develop` 的 PR，对应**开发阶段小版本**的增量集成；`develop` 稳定后再**有序**合入 `main` 做**发布大版本**。

**日常习惯**：`feat/xx` 上仍鼓励**小步、可读的提交**；与 PR 是互补关系（PR 是"要合并到 develop 时的一扇门"，不是替代原子提交）。

## GitHub CLI（`gh`）与本地/AI 协作

在**本机/IDE** 里若要让 **AI 或终端脚本** 代劳「开 PR、查 PR、合并 PR、看 checks」等，除 `git` 与网络外，通常依赖 **GitHub 官方命令行** [`gh`](https://cli.github.com/)（`git` 只负责推代码，不自带 GitHub API 上的 PR 界面能力）。

- **安装**：以本机包管理器为准，例如 macOS 常用 `brew install gh`。
- **首次授权（必做一次）**：安装后须在本机执行 **`gh auth login`**，按提示用浏览器登录 GitHub 或粘贴 token。**未授权时** `gh pr create` / `gh pr merge` 等会失败（常见提示为需 `GH_TOKEN` 或 `gh auth login`）。**该步骤不能由 AI 代替完成**（涉及账号与浏览器或 token，必须由人在自己的环境里执行一次）。完成后凭据会保存在本机 keyring/配置中，日常可复用；**换机、重装、清理凭据**后需重新登录（`gh auth status` 可检查当前是否已登）。
- 与纯 `git push` 的关系：代码可先 `git push` 到远端分支，再用 `gh pr create` 从该分支向 `develop` 开 PR；或全程在 GitHub 网页上操作，**不强制**装 `gh`，但 **AI 自动代开/代合 PR 的路径依赖已安装且已登录的 `gh`**。

## 核心原则

**一个需求/主题 = 尽量一条线清晰的提交历史**。禁止在**同一功能分支的同一 PR 里**混进无关大杂烩。AI 编码 → 自测 → 审阅/CI → 再合并。

## 需求生命周期（本地/分支上）

```text
[需求提出] → [在 feat/xx 上研发] → [自测] → [推远端 + 开 PR] → [审阅/CI] → [合入 develop] → [需求在集成线关闭]
```

### 流程步骤

1. **需求提出** — 分配唯一 ID 或短描述，如 `PROJ-001` / `feat/user-login`。
2. **在 `feat/xx` 上研发** — 从**当前 `develop`** 拉出分支；不穿插无关需求，大块改动可拆成多个 `feat/xx` 或多个 PR。
3. **自测** — 运行测试/类型检查，未通过则循环修复，再开 PR。
4. **开 PR** — 指向 **`develop`**；说明范围、可截图/可验证步骤。
5. **审阅与合并** — 通过 CI 与必要审阅后合入 `develop`（`main` 仅按发布策略由 `develop` 合入，见下）。
6. **推送** — 功能分支在合并后可按需删除；本地 `git pull` 更新 `develop`。

## 分支策略（多人、双轨）

| 分支 | 角色 | 说明 |
|------|------|------|
| **`main`** | 生产/大版本基线 | **稳定、可发版**；不承载日常并行的多人开发。由 **`develop` 在发布节点合入**（大版本/正式发布）。 |
| **`develop`** | 日常集成分支 | 所有人基于它拉 `feat/xx`；**开发阶段小版本**的集成都先进这里（经 PR）。 |
| **`feat/xxx`** | 功能/主题分支 | 从 **`develop` 最新**切出，命名 kebab-case；**仅通过 PR 合回 `develop`**。 |
| **`hotfix/xxx`** | 紧急热修 | 视严重度基于 `main` 或 `develop`；修完要同时想清是否需合回另一轨，避免线分叉漂移。 |

### 工作流简图

```text
         feat/a ──PR──►
                        develop ──按发布计划──PR──► main
         feat/b ──PR──►
```

### 规则

- **`main` 与 `develop` 禁止 force push**（特殊仓库治理除外，须团队共识）。
- 功能分支命名 **kebab-case**（例：`feat/user-login`, `feat/data-export`）。
- 不修改他人环境的 git config、不代他人 `--no-verify` 绕过 hooks；合并以 **PR + CI 绿灯** 为常规门槛。
- **小版本** = `develop` 上持续合入的增量（可配合 milestone/tag 在团队内约定）；**大版本** = `develop` → `main` 的合入 + 发版/打 tag（具体节奏与版本号在发布流程里定，不绑死在本文件）。

## 与「原子提交、人类确认」的关系

- 本仓库**仍可**在单功能分支上坚持「一需求一 PR、PR 内提交尽量可 bisect」。
- **多人在 `develop` 上并行**时，以 **PR 合入** 作为「进入集成线」的确认点，替代「单人在 `main` 上每拍一次都人类 diff」的旧假设。

## 提交规范

### 格式

```text
[可选 PROJ-XXXX] <标签>: <简述>

- 要点1
- 要点2
```

（若团队仍要 `AI-Test` / `Human-Confirm` 元信息，可写在 PR 描述或 check list，不必强制进每条 commit。）

### 标签

| 标签 | 用途 | 示例 |
|------|------|------|
| `feat:` | 新功能 | `feat: add user login` |
| `bugfix:` | Bug 修复 | `bugfix: fix ime duplicate input` |
| `refactor:` | 重构 | `refactor: extract nav helpers` |
| `docs:` | 文档 | `docs: update devloop spec` |
| `test:` | 测试 | `test: add editor e2e` |
| `chore:` | 杂项 | `chore: update deps` |
| `wip:` | 临时保存 | `wip: mid-refactor stash` |

### 规则

- 描述用祈使句，不用过去时。
- 禁止无信息量的提交信息（如 "fix"、"update"）。
- `wip:` 仅用于**分支内**临时切上下文；**要合入 `develop` 的 PR 应整理为可 review 的提交**。

## 异常处理

| 异常 | 处理 |
|------|------|
| 需求过大 | 拆为多个小 `feat/xx` 或多 PR，各自可 review |
| 与 `develop` 冲突 | 在 `feat/xx` 上 `merge develop` 或 `rebase`（团队统一一种），解决后再推 |
| PR 不通过 | 在功能分支上继续修，**不**在目标分支上直接补同一功能 |
| 提交包含无关改动 | Review 时要求打回，用 `git add -p` 或另分支剥离 |

## 安全约束

- **高危 git 操作**（force push 到受保护分支、改 remote、批量删分支）**须人工确认**；日常 **PR 合并**在权限与规范允许下可由维护者执行。
- **不修改** 他人机器的 git config。
- **不**为省事滥用 `--no-verify`。
- **不**对 `main` / `develop` 做与团队规范冲突的 force push。
