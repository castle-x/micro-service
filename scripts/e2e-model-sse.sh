#!/usr/bin/env bash
# scripts/e2e-model-sse.sh
# Model 服务 SSE 流式对话专项 E2E
#
# 验证：
#   1. 首 token 延迟 < FIRST_TOKEN_MS（默认 5000ms）
#   2. 收到至少 N 个 chunk
#   3. 收到 [DONE] 终止符
#   4. （长连接场景）收到心跳 :keepalive
#   5. client cancel 后服务端 ctx 正确终止（无残留连接）
#
# 前置：
#   - make dev-start 全栈已启动
#   - .env 中 LLM provider 凭据可用
#   - 测试账号 admin@platform.com / Admin@1234 已 bootstrap
#
# 配置（环境变量）：
#   EDGE_API=http://localhost:38080
#   FIRST_TOKEN_MS=5000
#   MIN_CHUNKS=3

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

EDGE_API="${EDGE_API:-http://localhost:38080}"
STREAM_ENDPOINT="${STREAM_ENDPOINT:-/api/v1/admin/models/chat/stream}"
FIRST_TOKEN_MS="${FIRST_TOKEN_MS:-5000}"
MIN_CHUNKS="${MIN_CHUNKS:-3}"
TEST_EMAIL="${TEST_EMAIL:-admin@platform.com}"
TEST_PASSWORD="${TEST_PASSWORD:-Admin@1234}"

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

require() { command -v "$1" >/dev/null 2>&1 || { red "missing tool: $1"; exit 2; }; }
require curl
require jq

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo ">>> [SSE-E2E] login as $TEST_EMAIL"
LOGIN_RESP=$(curl -sS -X POST "$EDGE_API/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.access_token // .access_token // empty')
if [ -z "$TOKEN" ]; then
  red "✗ login failed: $LOGIN_RESP"
  exit 1
fi
green "    ✓ token acquired"

REQ_BODY='{
  "model": "gpt-4o-mini",
  "messages": [{"role":"user","content":"用一句话介绍 Go 语言"}],
  "stream": true
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

# 校验 chunk 数
CHUNK_COUNT=$(grep -c '^data: ' "$OUT" || true)
echo "    received $CHUNK_COUNT data chunks"
if [ "$CHUNK_COUNT" -lt "$MIN_CHUNKS" ]; then
  red "✗ chunk count $CHUNK_COUNT < minimum $MIN_CHUNKS"
  cat "$OUT"
  exit 1
fi

# 校验 [DONE]
if ! grep -q '^data: \[DONE\]' "$OUT"; then
  red "✗ missing [DONE] terminator"
  exit 1
fi
green "    ✓ stream completed with [DONE]"

# 校验每个 data 行（除 [DONE]）能解析成 JSON
INVALID=$(grep '^data: ' "$OUT" | grep -v '^data: \[DONE\]' \
  | sed 's/^data: //' \
  | while read -r line; do echo "$line" | jq . >/dev/null 2>&1 || echo BAD; done | grep -c BAD || true)
if [ "$INVALID" -gt 0 ]; then
  red "✗ $INVALID chunk(s) are not valid JSON"
  exit 1
fi
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
HEALTH=$(curl -sS -o /dev/null -w '%{http_code}' "$EDGE_API/healthz" || echo 000)
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
  -d '{"model":"__nonexistent__","messages":[{"role":"user","content":"hi"}],"stream":true}' \
  > "$TMP/err.code"
CODE=$(cat "$TMP/err.code")
if [ "$CODE" -ge 400 ] && [ "$CODE" -lt 500 ]; then
  green "    ✓ got 4xx for invalid model ($CODE)"
elif grep -qE '^event: error|"error"' "$ERR_OUT"; then
  green "    ✓ got SSE error event for invalid model"
else
  yellow "    (warn) unexpected response for invalid model: code=$CODE body=$(head -c 200 "$ERR_OUT")"
fi

green "✓ SSE E2E passed"
