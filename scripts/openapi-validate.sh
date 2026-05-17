#!/usr/bin/env bash
# scripts/openapi-validate.sh
# OpenAPI 契约校验
#
# 检查项：
#   1. spec 本身合法（结构、ref 解析）
#   2. 所有路径在代码中至少被一处实现引用（粗粒度，避免"接口存在但没人写"）
#   3. （可选）请求/响应 schema 校验由集成测试中的 kin-openapi 验证，本脚本只做静态检查
#
# 工具：
#   - redocly CLI（首选，功能完整）：npm i -g @redocly/cli
#   - swagger-cli（次选）：npm i -g @apidevtools/swagger-cli
#   - 都没装则只做基础 grep 校验
#
# 用法：
#   bash scripts/openapi-validate.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

OPENAPI_SPEC="idl/llm/openapi.yaml"
SERVICES_DIR="services"
SERVICE_IMPL_DIR="$SERVICES_DIR/llm"

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

if [ ! -f "$OPENAPI_SPEC" ]; then
  red "Error: $OPENAPI_SPEC not found"
  exit 1
fi

ERRORS=0
report_error() { red "✗ $1"; ERRORS=$((ERRORS + 1)); }

# 1. 结构校验
echo ">>> Step 1: validating $OPENAPI_SPEC structure"
if command -v redocly >/dev/null 2>&1; then
  if redocly lint "$OPENAPI_SPEC"; then
    green "    ✓ redocly lint passed"
  else
    report_error "redocly lint failed"
  fi
elif command -v swagger-cli >/dev/null 2>&1; then
  if swagger-cli validate "$OPENAPI_SPEC"; then
    green "    ✓ swagger-cli validate passed"
  else
    report_error "swagger-cli validate failed"
  fi
else
  yellow "    (skip) Neither redocly nor swagger-cli installed; skipping structure validation."
  yellow "    Recommend: npm i -g @redocly/cli"
fi

# 2. 路径覆盖率：spec 中每个 path 至少在代码里出现一次
echo
echo ">>> Step 2: route coverage (each path must appear in code)"

# 提取 spec 里所有路径（顶层 paths: 下的 key）
PATHS=$(awk '
  /^paths:/ { in_paths=1; next }
  in_paths && /^[a-zA-Z]/ { in_paths=0 }
  in_paths && /^[[:space:]]{2}\// {
    sub(/:.*$/, "")
    sub(/^[[:space:]]+/, "")
    print
  }
' "$OPENAPI_SPEC")

if [ -z "$PATHS" ]; then
  yellow "    (warn) No paths extracted from spec — check yaml structure"
else
  MISSING=0
  while IFS= read -r p; do
    [ -z "$p" ] && continue
    # 把 {param} 转成正则 [^/]+
    pat=$(echo "$p" | sed -E 's@\{[^}]+\}@[^/]+@g; s@/@\\/@g')
    colon=$(echo "$p" | sed -E 's@\{([a-zA-Z_][a-zA-Z0-9_]*)\}@:\1@g')
    if ! grep -rE "$pat" "$SERVICE_IMPL_DIR" --include='*.go' -l >/dev/null 2>&1 \
      && ! grep -rF "$colon" "$SERVICE_IMPL_DIR" --include='*.go' -l >/dev/null 2>&1; then
      report_error "uncovered path: $p (no implementation found in $SERVICE_IMPL_DIR/)"
      MISSING=$((MISSING + 1))
    fi
  done <<< "$PATHS"
  TOTAL=$(echo "$PATHS" | wc -l | tr -d ' ')
  COVERED=$((TOTAL - MISSING))
  if [ "$MISSING" -eq 0 ]; then
    green "    ✓ All $TOTAL paths covered"
  else
    yellow "    coverage: $COVERED / $TOTAL"
  fi
fi

# 3. 反向检查：代码里 router 注册的路径，是否都登记在 spec？（best effort）
echo
echo ">>> Step 3: reverse coverage (routes in code should be in spec)"
# 抓取 hertz 风格路由：h.GET / h.POST / h.PUT / h.DELETE / h.PATCH
CODE_ROUTES=$(grep -rEh '\.(GET|POST|PUT|DELETE|PATCH)\("/' "$SERVICE_IMPL_DIR" --include='*.go' 2>/dev/null \
  | sed -E 's@.*\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)".*@\2@' \
  | sort -u || true)

if [ -n "$CODE_ROUTES" ]; then
  MISSING_IN_SPEC=0
  while IFS= read -r route; do
    [ -z "$route" ] && continue
    # spec 里的 path 用 {var}，把代码里的 :param 转成 {param} 后再比
    norm=$(echo "$route" | sed -E 's@:([a-zA-Z_]+)@{\1}@g')
    if ! echo "$PATHS" | grep -qx "$norm"; then
      yellow "    (warn) route '$route' not in OpenAPI spec"
      MISSING_IN_SPEC=$((MISSING_IN_SPEC + 1))
    fi
  done <<< "$CODE_ROUTES"
  if [ "$MISSING_IN_SPEC" -eq 0 ]; then
    green "    ✓ All code routes documented"
  fi
else
  yellow "    (skip) No absolute hertz routes found under $SERVICE_IMPL_DIR"
fi

echo
if [ "$ERRORS" -gt 0 ]; then
  red "✗ $ERRORS error(s)"
  exit 1
fi
green "✓ OpenAPI contract OK"
