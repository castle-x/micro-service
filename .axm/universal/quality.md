<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
applies-to: [universal]
related:
  - ./devloop.md
  - ./review.md
-->

# 质量保障规范

## 测试策略

| 级别 | 测试要求 |
|------|----------|
| T0 | 不需要测试 |
| T1 | 至少验证功能正常（手动或自动化） |
| T2 | 单元测试 + 手动验证核心路径 |
| T3 | 单元测试 + E2E + 回归测试 |

## 质量门禁

提交前必须通过以下检查（具体命令按项目技术栈替换）：

| 检查项 | 命令 | 要求 |
|--------|------|------|
| 类型检查 | `<项目 typecheck 命令>` | 零错误 |
| Lint | `<项目 lint 命令>` | 零错误 |
| 编译 | `<项目 build 命令>` | 零错误 |
| 单元测试 | `<项目 test 命令>` | 全部通过 |

> 初始化后由 AI 根据 Phase 1 Discover 的项目画像填入具体命令（例如 Node 项目 `pnpm typecheck` / `pnpm lint`；Rust 项目 `cargo check` / `cargo clippy`；Python `mypy` / `ruff` / `pytest`）。

### 按级别的门禁要求

| 级别 | 必须通过 |
|------|----------|
| T0 | 相关文件 lint 无新增错误 |
| T1 | lint + typecheck |
| T2 | lint + typecheck + 单元测试 |
| T3 | 全量门禁 + E2E |

## 回归防护

1. **修 bug 必须同步写测试**——无测试的 bugfix 不算完成
2. 测试必须覆盖 bug 的触发条件
3. 回归测试放置在对应的测试目录中

## 质量红线

以下情况**禁止提交**：

- 引入新的 typecheck 错误
- 引入新的 lint 错误
- 破坏已有测试
- 未通过对应级别的质量门禁

## 代码审查要点

| 维度 | 检查内容 |
|------|----------|
| 正确性 | 逻辑是否符合意图 |
| 最小性 | 是否存在未请求的变更 |
| 一致性 | 是否匹配现有代码风格 |
| 安全性 | 是否引入安全隐患 |

> 用第二个 agent / 工具做收尾 review 时（Codex review、Claude review、CI 静态检查、人工二审等），按 `review.md` 的七条契约执行；本节定义"看什么"，`review.md` 定义"怎么对待 review 结果"。
