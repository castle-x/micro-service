#!/usr/bin/env bash
# scripts/e2e-all.sh
# E2E 测试调度器：依次跑 4 条关键链路
#
# 前置：
#   - make dev-start 已启动全栈（infra + services + edge-api）
#   - .env 已配置
#
# 用法：
#   bash scripts/e2e-all.sh                 # 跑所有
#   E2E_ONLY=auth,sse bash scripts/e2e-all.sh  # 只跑指定
#
# 退出码：0 全绿；非 0 任一失败

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

declare -a SUITES=(
  "auth:scripts/e2e-google-auth.sh:登录/鉴权链路 (edge-api → idp → iam)"
  "asset:scripts/e2e-asset-oss-upload.sh:资产上传链路 (edge-api → asset → OSS)"
  "sse:scripts/e2e-model-sse.sh:模型 SSE 流式对话 (edge-api → model)"
  # billing 链路待 scripts/e2e-billing.sh 落地后启用
  # "billing:scripts/e2e-billing.sh:支付扣费链路 (edge-api → billing → MQ → credits/notification)"
)

ONLY="${E2E_ONLY:-}"

PASS=0
FAIL=0
SKIP=0
declare -a FAILED=()

for entry in "${SUITES[@]}"; do
  IFS=':' read -r key path desc <<< "$entry"

  if [ -n "$ONLY" ] && ! echo ",$ONLY," | grep -q ",$key,"; then
    continue
  fi

  bold "================================================================"
  bold "[$key] $desc"
  bold "  → $path"
  bold "================================================================"

  if [ ! -f "$path" ]; then
    yellow "    SKIP: $path not found (not yet implemented)"
    SKIP=$((SKIP + 1))
    continue
  fi

  if bash "$path"; then
    green "[$key] PASS"
    PASS=$((PASS + 1))
  else
    red "[$key] FAIL"
    FAIL=$((FAIL + 1))
    FAILED+=("$key")
  fi
  echo
done

bold "================================================================"
bold "E2E summary: $PASS pass / $FAIL fail / $SKIP skip"
bold "================================================================"

if [ "$FAIL" -gt 0 ]; then
  red "Failed suites: ${FAILED[*]}"
  exit 1
fi
green "✓ All e2e suites passed"
