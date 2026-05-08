#!/usr/bin/env bash
# e2e-google-auth.sh — Phase 03 Google 登录端到端冒烟测试
# 前置条件：
#   1. make dev 已启动（MongoDB / Redis 运行中）
#   2. 三个服务已在本地运行（iam:8082, idp:8081, edge-api:8080）
#   3. 环境变量已设置（参见 .env.example）
#
# 用法：
#   export GOOGLE_CLIENT_ID=xxx GOOGLE_CLIENT_SECRET=xxx JWT_SECRET=xxx
#   bash scripts/e2e-google-auth.sh

set -euo pipefail

EDGE_BASE="${EDGE_BASE:-http://localhost:8080}"
PASS=0
FAIL=0

info()  { echo "[INFO]  $*"; }
ok()    { echo "[PASS]  $*"; PASS=$((PASS+1)); }
fail()  { echo "[FAIL]  $*"; FAIL=$((FAIL+1)); }

# ---- Step 1: 获取 Google 授权 URL ----
info "Step 1: GET /api/v1/auth/google/url"
RESP=$(curl -sf "${EDGE_BASE}/api/v1/auth/google/url" 2>/dev/null || echo '{}')
AUTH_URL=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('auth_url',''))" 2>/dev/null || true)
STATE=$(echo "$RESP"    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('state',''))"    2>/dev/null || true)

if [ -n "$AUTH_URL" ] && [ -n "$STATE" ]; then
  ok "Got auth_url (length=${#AUTH_URL}) and state (length=${#STATE})"
else
  fail "Missing auth_url or state — response: $RESP"
fi

info ""
info "===== 手动步骤 ====="
info "在浏览器中打开以下 URL 完成 Google 授权："
info "  $AUTH_URL"
info ""
info "Google 会重定向到 http://localhost:8080/api/v1/auth/google/callback?code=CODE&state=$STATE"
info "复制 code 参数后运行 Step 2："
info ""
info "  export GOOGLE_CODE=<your_code>"
info "  bash scripts/e2e-google-auth.sh --callback"
info "===== ===== ====="

# ---- Step 2: 模拟回调（需要真实 code，仅在 --callback 模式下执行）----
if [ "${1:-}" = "--callback" ]; then
  CODE="${GOOGLE_CODE:-}"
  if [ -z "$CODE" ]; then
    fail "GOOGLE_CODE not set. Export it first."
    exit 1
  fi

  info ""
  info "Step 2: GET /api/v1/auth/google/callback?code=\$CODE&state=$STATE"
  CB_RESP=$(curl -sf "${EDGE_BASE}/api/v1/auth/google/callback?code=${CODE}&state=${STATE}" 2>/dev/null || echo '{}')
  ACCESS_TOKEN=$(echo "$CB_RESP"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('access_token',''))"  2>/dev/null || true)
  REFRESH_TOKEN=$(echo "$CB_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('refresh_token',''))" 2>/dev/null || true)
  USER_ID=$(echo "$CB_RESP"       | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('user_id',''))"       2>/dev/null || true)

  if [ -n "$ACCESS_TOKEN" ]; then
    ok "Got access_token (length=${#ACCESS_TOKEN})"
  else
    fail "Missing access_token — response: $CB_RESP"
  fi
  if [ -n "$REFRESH_TOKEN" ]; then
    ok "Got refresh_token"
  else
    fail "Missing refresh_token"
  fi
  if [ -n "$USER_ID" ]; then
    ok "Got user_id: $USER_ID"
  else
    fail "Missing user_id"
  fi

  # ---- Step 3: 刷新 token ----
  if [ -n "$REFRESH_TOKEN" ]; then
    info ""
    info "Step 3: POST /api/v1/auth/token/refresh"
    REFRESH_RESP=$(curl -sf -X POST "${EDGE_BASE}/api/v1/auth/token/refresh" \
      -H "Content-Type: application/json" \
      -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}" 2>/dev/null || echo '{}')
    NEW_ACCESS=$(echo "$REFRESH_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('access_token',''))" 2>/dev/null || true)
    if [ -n "$NEW_ACCESS" ]; then
      ok "Token refresh successful (new token length=${#NEW_ACCESS})"
    else
      fail "Token refresh failed — response: $REFRESH_RESP"
    fi
  fi
fi

# ---- 汇总 ----
echo ""
echo "===== E2E 结果 ====="
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
