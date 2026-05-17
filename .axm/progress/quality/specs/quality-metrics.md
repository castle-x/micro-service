<!-- axm-meta
status: active
last-reviewed: 2026-05-17
owner: castlexu
progress-type: spec
initiative: quality
related:
  - ../roadmap.md
  - ../../../project/code-review.md
  - ../../../project/api-testing.md
  - ../../opentelemetry/index.md
-->

# QUAL-05 度量与回顾

## 实施进度

- 业务状态：`pending`

## 目标

让质量改进有数据支撑，不靠感觉。把 11 个核心指标接入 OpenObserve（复用现有可观测性栈），季度回顾发现盲区。

## 验收标准

### AI 自动验收

- [ ] CI 输出指标到 OpenObserve（unit 覆盖率、套件耗时、Flaky 数、PR 周期等）
- [ ] `docs/quarterly-review-template.md` 模板入仓

### 人类验收

- [ ] 第一次回顾会议完成，输出至少 3 个调整项

## 核心指标

| 指标 | 目标 | 数据源 | 采集方式 |
|---|---|---|---|
| biz/ 单测覆盖率 | ≥ 70% | `go test -coverprofile` | CI 上传 |
| OpenAPI 路径覆盖率 | 100% | `openapi-validate.sh` | 脚本输出 |
| 关键链路 E2E 通过率 | 100% | CI 历史 | GitHub Actions API |
| 线上 P0/P1 测试可拦截率 | ≥ 80% | 故障复盘标注 | 人工填写 |
| 测试套件耗时 P95 | unit < 3min / integration < 10min | CI | Actions API |
| Flaky 测试数 | 0 | CI 重试统计 | 自写汇总脚本 |
| PR 首次响应时长 P50 | < 4h（工作日） | GitHub PR API | 自写汇总脚本 |
| PR 合并周期 P50 | < 24h | 同上 | 同上 |
| 单 PR 平均轮次 | < 3 轮 | 同上 | 同上 |
| 🔴 Blocker / PR | 趋势下降 | 评论扫描 | 自写脚本 grep 优先级标记 |
| 线上故障"review 可拦截"标注 | 趋势下降 | 故障复盘 | 人工 |

## 实施步骤

### Step 1 — CI 指标采集

每个 CI job 末尾上报 OTel：

```yaml
- name: Report metrics
  if: always()
  run: |
    curl -X POST $OBS_ENDPOINT/api/default/_json \
      -H "Authorization: Bearer $OBS_TOKEN" \
      -d "{
        \"metric\": \"ci.duration\",
        \"job\": \"${{ github.job }}\",
        \"status\": \"${{ job.status }}\",
        \"duration\": ${{ steps.timing.outputs.duration }},
        \"branch\": \"${{ github.ref_name }}\"
      }"
```

复用 `.axm/progress/opentelemetry/` 已建立的 OpenObserve 实例。

### Step 2 — PR 指标汇总脚本

`scripts/quality-metrics.sh`：

```bash
# 用 gh CLI 拉过去 90 天 PR
gh pr list --state merged --limit 200 --json number,createdAt,mergedAt,reviews,comments
# 计算：首响 P50、合并周期 P50、平均轮次、🔴 数
```

每周一 cron 跑一次，结果存 OpenObserve。

### Step 3 — 季度回顾模板

`docs/quarterly-review-template.md`：

```markdown
# Q? 质量回顾

## 数据快照
- 覆盖率：X% (上季 Y%)
- 套件耗时：unit ?s / integration ?s
- Flaky 数：?
- 线上故障：? 起，其中"测试可拦截" ? 起

## 发现的盲区
1.
2.

## 调整项（下季优先级）
- [ ] ...
- [ ] ...

## 不变的部分
（什么运行良好，保持不变）
```

### Step 4 — 回顾节奏

- **季度**：完整回顾会议（1.5 小时）
- **月度**：异常指标轻量同步（15 分钟）
- **触发式**：线上 P0/P1 故障复盘必须标注"测试是否可拦截"

## 原则

- 度量是**镜子**，不是**鞭子**。指标趋势异常时优先问"流程哪里卡住了"，而不是"谁没做"。
- 数据无法捕捉的事情依然重要（如代码可读性、设计简洁度），不要因为难度量就忽略。

## 影响面

- 团队工作量：每季 1.5 小时回顾会议
- 系统依赖：复用现有 OpenObserve，无新基础设施

## 回滚

- 关闭 cron 即停止采集，已有数据保留作为基线
