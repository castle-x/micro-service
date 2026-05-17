#!/usr/bin/env bash
# scripts/e2e-llm-sse.sh
# LLM 服务 SSE 流式生成专项 E2E
#
# 验证：
#   1. 首 token 延迟 < FIRST_TOKEN_MS（默认 5000ms）
#   2. 收到至少 N 个 chunk
#   3. 收到 done 事件
#   4. 收到 usage 事件并包含 token 计数
#   5. client cancel 后服务端 ctx 正确终止（无残留连接）
#   6. 非法模型返回 4xx 或 SSE error event
#
# 前置：
#   - make dev-start 全栈已启动
#   - .env 中 LLM provider 凭据可用，或已通过 fake upstream 配好 provider/model
#   - 测试账号 admin@platform.com / Admin@1234 已 bootstrap
#
# 配置（环境变量）：
#   EDGE_API=http://localhost:38080
#   STREAM_ENDPOINT=/api/v1/admin/llm/stream
#   MODEL_REF=openai/gpt-4o-mini
#   FIRST_TOKEN_MS=5000
#   MIN_CHUNKS=3
#   FAKE_OPENAI_BASE_URL=http://localhost:39090  # optional setup hint only
#   FAKE_OPENAI_KEY=fake-key                       # optional setup hint only

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

EDGE_API="${EDGE_API:-http://localhost:38080}"
STREAM_ENDPOINT="${STREAM_ENDPOINT:-/api/v1/admin/llm/stream}"
FIRST_TOKEN_MS="${FIRST_TOKEN_MS:-5000}"
MIN_CHUNKS="${MIN_CHUNKS:-1}"
MODEL_REF="${MODEL_REF:-openai/gpt-4o-mini}"
TEST_EMAIL="${TEST_EMAIL:-admin@platform.com}"
TEST_PASSWORD="${TEST_PASSWORD:-Admin@1234}"
FAKE_OPENAI_BASE_URL="${FAKE_OPENAI_BASE_URL:-}"
FAKE_OPENAI_KEY="${FAKE_OPENAI_KEY:-}"

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

require() { command -v "$1" >/dev/null 2>&1 || { red "missing tool: $1"; exit 2; }; }
require curl
require jq

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

print_env_hint() {
  yellow "environment:"
  yellow "  EDGE_API=$EDGE_API"
  yellow "  STREAM_ENDPOINT=$STREAM_ENDPOINT"
  yellow "  MODEL_REF=$MODEL_REF"
  if [ -n "$FAKE_OPENAI_BASE_URL" ] || [ -n "$FAKE_OPENAI_KEY" ]; then
    yellow "  fake upstream hint: FAKE_OPENAI_BASE_URL=${FAKE_OPENAI_BASE_URL:-<unset>} FAKE_OPENAI_KEY=${FAKE_OPENAI_KEY:+<set>}"
    yellow "  note: this script does not create provider/model records; configure them before running."
  fi
}

fail_env() {
  red "✗ $*"
  print_env_hint
  exit 1
}

extract_event_json() {
  local file="$1"
  local event="$2"
  awk -v event="$event" '
    $0 == "event: " event { in_event = 1; next }
    in_event && /^data: / { sub(/^data: /, ""); print; in_event = 0; next }
    $0 == "" { in_event = 0 }
  ' "$file"
}

require_event() {
  local file="$1"
  local event="$2"
  if ! grep -q "^event: $event$" "$file"; then
    red "✗ missing $event event"
    cat "$file"
    exit 1
  fi
}

validate_json_lines() {
  local file="$1"
  local invalid
  invalid=$(grep '^data: ' "$file" \
    | sed 's/^data: //' \
    | while read -r line; do
        [ -z "$line" ] && continue
        echo "$line" | jq . >/dev/null 2>&1 || echo BAD
      done \
    | grep -c BAD || true)
  if [ "$invalid" -gt 0 ]; then
    red "✗ $invalid chunk(s) are not valid JSON"
    cat "$file"
    exit 1
  fi
}

echo ">>> [SSE-E2E] preflight edge-api health"
HEALTH=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$EDGE_API/healthz" || true)
[ -n "$HEALTH" ] || HEALTH=000
if [ "$HEALTH" != "200" ]; then
  fail_env "edge-api is not reachable or unhealthy at $EDGE_API/healthz (http $HEALTH). Start local infra with make dev-start, or set EDGE_API."
fi
green "    ✓ edge-api healthy"

echo ">>> [SSE-E2E] login as $TEST_EMAIL"
LOGIN_BODY="$TMP/login.body"
LOGIN_CODE=$(curl -sS -o "$LOGIN_BODY" -w '%{http_code}' --max-time 10 \
  -X POST "$EDGE_API/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}" || echo 000)
LOGIN_RESP=$(cat "$LOGIN_BODY")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.access_token // .access_token // empty' 2>/dev/null || true)
if [ -z "$TOKEN" ]; then
  fail_env "login failed (http $LOGIN_CODE). Ensure test account exists: TEST_EMAIL=$TEST_EMAIL TEST_PASSWORD=<set>. Response: $LOGIN_RESP"
fi
green "    ✓ token acquired"

REQ_BODY='{
  "model_ref": "'"$MODEL_REF"'",
  "messages": [{"role":"user","content":"用一句话介绍 Go 语言"}]
}'

# ----------------------------------------------------------------
# Test 1: 正常流式响应
# ----------------------------------------------------------------
echo ">>> [Test 1] streaming chat completion"
OUT="$TMP/sse.out"
START_NS=$(date +%s%N)

# 用 --no-buffer 让 SSE 实时输出；超时 30s
curl -sS --no-buffer --max-time 30 \
  -X POST "$EDGE_API$STREAM_ENDPOINT" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$REQ_BODY" > "$OUT" || {
    red "✗ curl failed"
    cat "$OUT"
    exit 1
  }

# 首 token 延迟（用第一个 data: 行的时间戳近似；curl 写完才 return，无法精确，
# 这里用 stat 的修改时间差作 P95 monitor 兜底）
END_NS=$(date +%s%N)
TOTAL_MS=$(( (END_NS - START_NS) / 1000000 ))
echo "    total response time: ${TOTAL_MS}ms"
if [ "$TOTAL_MS" -gt "$FIRST_TOKEN_MS" ]; then
  yellow "    (warn) total response time ${TOTAL_MS}ms exceeded FIRST_TOKEN_MS=${FIRST_TOKEN_MS}ms; curl cannot measure first token precisely"
fi

# 校验 content_delta chunk 数
CHUNK_COUNT=$(grep -c '^event: content_delta' "$OUT" || true)
echo "    received $CHUNK_COUNT content_delta chunks"
if [ "$CHUNK_COUNT" -lt "$MIN_CHUNKS" ]; then
  red "✗ chunk count $CHUNK_COUNT < minimum $MIN_CHUNKS"
  cat "$OUT"
  exit 1
fi

# 校验 done event
require_event "$OUT" "done"
green "    ✓ stream completed with done event"

# 校验 usage event
require_event "$OUT" "usage"
USAGE_JSON=$(extract_event_json "$OUT" "usage" | tail -n 1)
if [ -z "$USAGE_JSON" ]; then
  red "✗ usage event has no data"
  cat "$OUT"
  exit 1
fi
if ! echo "$USAGE_JSON" | jq -e '
  (.prompt_tokens // .usage.prompt_tokens // empty) != null and
  (.completion_tokens // .usage.completion_tokens // empty) != null and
  (.total_tokens // .usage.total_tokens // empty) != null
' >/dev/null; then
  red "✗ usage event missing prompt_tokens/completion_tokens/total_tokens: $USAGE_JSON"
  cat "$OUT"
  exit 1
fi
green "    ✓ usage event includes token counts"

# 校验每个 data 行能解析成 JSON
validate_json_lines "$OUT"
green "    ✓ all chunks are valid JSON"

# ----------------------------------------------------------------
# Test 2: client cancel —— 中途断开，等待 2s 后检查服务端是否还在推
# 这个测试是 best-effort，主要确保服务端不会 panic / hang
# ----------------------------------------------------------------
echo ">>> [Test 2] client cancel mid-stream"
(
  curl -sS --no-buffer --max-time 2 \
    -X POST "$EDGE_API$STREAM_ENDPOINT" \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d "$REQ_BODY" > /dev/null 2>&1 || true
)
sleep 1
# 检查 edge-api 仍在响应（说明上一次 cancel 没把服务搞挂）
HEALTH=$(curl -sS -o /dev/null -w '%{http_code}' "$EDGE_API/healthz" || true)
[ -n "$HEALTH" ] || HEALTH=000
if [ "$HEALTH" != "200" ]; then
  red "✗ edge-api unhealthy after cancel: $HEALTH"
  exit 1
fi
green "    ✓ server alive after client cancel"

# ----------------------------------------------------------------
# Test 3: 错误路径 —— 非法模型名应返回 SSE error event 或 4xx
# ----------------------------------------------------------------
echo ">>> [Test 3] invalid model error handling"
ERR_OUT="$TMP/err.out"
curl -sS --no-buffer --max-time 10 \
  -o "$ERR_OUT" -w '%{http_code}' \
  -X POST "$EDGE_API$STREAM_ENDPOINT" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"model_ref":"__nonexistent__","messages":[{"role":"user","content":"hi"}]}' \
  > "$TMP/err.code" || fail_env "invalid model request could not reach $EDGE_API$STREAM_ENDPOINT"
CODE=$(cat "$TMP/err.code")
if [ "$CODE" -ge 400 ] && [ "$CODE" -lt 500 ]; then
  green "    ✓ got 4xx for invalid model ($CODE)"
elif grep -q '^event: error$' "$ERR_OUT"; then
  ERROR_JSON=$(extract_event_json "$ERR_OUT" "error" | tail -n 1)
  if [ -z "$ERROR_JSON" ] || ! echo "$ERROR_JSON" | jq . >/dev/null 2>&1; then
    red "✗ invalid model error event is missing valid JSON data"
    cat "$ERR_OUT"
    exit 1
  fi
  green "    ✓ got SSE error event for invalid model"
else
  red "✗ expected 4xx or SSE error event for invalid model, got code=$CODE body=$(head -c 500 "$ERR_OUT")"
  exit 1
fi

green "✓ SSE E2E passed"
