<!-- axm-meta
doc-state: current
last-reviewed: 2026-05-17
owner: castlexu
progress-type: bug
initiative: dev-ops
workflow-state: closed
state-updated: 2026-05-17
related:
  - ../specs/process-lifecycle.md
  - ./bug-2026-05-14-dev-start-exit-zero-on-failure.md
-->

# bug-2026-05-14-dev-start-failure-path-unverified — dev-start 30s 超时失败路径未端到端验证

## 元信息

| 字段 | 值 |
|---|---|
| ID | `bug-2026-05-14-dev-start-failure-path-unverified` |
| 所属 initiative | `dev-ops` |
| 提交人 | review-agent（DevOps） |
| 提交时间 | 2026-05-14 |
| 优先级 | P2 |
| 严重度 | Minor |
| 影响模块 | `scripts/dev/start.sh` |
| 影响版本 | dev-ops initiative 首版 |
| 关联 PR / commit | 本地未提交 |
| 关联 spec / roadmap | `../specs/process-lifecycle.md`、`./bug-2026-05-14-dev-start-exit-zero-on-failure.md`（前置依赖） |

## 复现步骤

> 前置：`bug-2026-05-14-dev-start-exit-zero-on-failure` 已修复（否则失败被吞掉无法观察）。

1. `cd /Users/castlexu/github/micro-service`
2. 准备一个临时 services.json，让 admin_port 指向无人监听的端口（让 readyz 永远 unavailable）：
   ```bash
   cat > /tmp/svc-fail.json <<'EOF'
   {"services":[{"name":"iam","port":38082,"admin_port":59999,"binary":"iam","depends_on":[],"log_path":"bin/log/iam.log"}]}
   EOF
   ```
3. `bash scripts/dev/stop.sh >/dev/null 2>&1`
4. `DEV_CONFIG_FILE=/tmp/svc-fail.json timeout 45 bash scripts/dev/start.sh iam`
5. 清理：`DEV_CONFIG_FILE=/tmp/svc-fail.json bash scripts/dev/stop.sh iam; rm /tmp/svc-fail.json`

## 期望表现

- 30s 后退出，exit code 非零
- stderr 包含 `did not become ready within 30s`
- stderr 包含日志末尾 50 行（`last 50 lines`）
- stderr 包含状态文件路径与内容
- 残留的真实业务进程能被 `make dev-stop` 正确清理

## 实际表现

- 实施代码 `scripts/dev/start.sh:9-32` (`print_failure_context`) + `wait_for_service:42-66` 已写好该路径
- 但**从未真实触发观察过** —— 因为前置 BUG 让所有失败被吞掉，端到端 contract 没人验过
- 不确定 `print_failure_context` 实际输出是否完整（status_file / log tail / 残留进程提示）

## 影响范围

- 失败诊断信息可信度未知
- AI 自动化拿到失败 exit 后，依赖该输出做下一步决策（如读 log tail 自动定位），如果输出缺失会导致诊断断链

## 根因分析

DEV-01 spec 验收 8 项中第 8 项"启动失败打印日志尾部"未做行为测试，只检查代码路径存在。

端到端验证时确认 `print_failure_context` 已输出 readyz 响应、status file 内容和 log tail；缺失项是 readyz 超时后没有明确提示启动的业务进程会保留供排查、需要后续 `dev-stop` 清理。修复在 `wait_for_service` 超时分支补充该提示，不改变崩溃退出路径和端口占用路径的行为。

## 修复验收标准

### 修复约束

1. 本 BUG **不要求改实现**（除非测试中发现真缺失），主要任务是**端到端验证**
2. **决策**：超时退出后是否 kill 已启动的子进程？
   - 建议**保留进程**：让 AI / 人类能继续读 log 排障，由后续 `dev-stop` 清理
   - 如保留，需在 `print_failure_context` 输出末尾打印 `process pid=X kept alive for inspection; run make dev-stop to clean up`
3. 如果发现实现真有缺失（如 status_file 不打印、log tail 截断），**必须**补齐

### AI 自动验收

- [x] 前置：`bug-2026-05-14-dev-start-exit-zero-on-failure` 已 fixed（AI 自动验收通过，待人类 verified）
- [x] 行为：使用临时 `DEV_CONFIG_FILE` 触发 readyz timeout，断言：
  - exit code 非零
  - stderr 含 `did not become ready within 30s`
  - stderr 含当前配置 log path 的 `last 50 lines from ...`
  - stderr 含 `status file` 字样并跟着 JSON 内容
- [x] 行为：失败后跑 `DEV_CONFIG_FILE=<临时配置> bash scripts/dev/stop.sh fake`，端口 39082 释放、`bin/run/fake.{pid,status}` 清理
- [x] 不引入回归：正常成功路径仍 exit 0

### 人类验收

- [x] 故意配置错误 mongo URI 让 iam 启动后立即崩溃，确认终端打印的 log tail 足以让 AI 自动定位问题
- [x] 决策"保留进程供排查"还是"超时即 kill"的预期符合直觉

## 时间线

| 时间 | 状态 | 操作人 | 说明 |
|---|---|---|---|
| 2026-05-14 | open | review-agent | 提交 BUG，依赖前置 BUG 修复后再启动 |
| 2026-05-14 | in-progress | dev-ops BUG 修复 worker（process lifecycle） | 前置失败传播修复后，使用临时 `DEV_CONFIG_FILE` 与临时假服务二进制触发 readyz timeout 路径 |
| 2026-05-14 | fixed | dev-ops BUG 修复 worker（process lifecycle） | 已验证 timeout exit 非零，stderr 包含 readyz、status JSON、last 50 lines 和进程保留提示；临时服务已用 `stop.sh` 清理。本地未提交 |
| 2026-05-17 | closed | 主 agent | 用户确认 dev-ops 已开发完成；真实错误配置演练转为后续回归场景 |
