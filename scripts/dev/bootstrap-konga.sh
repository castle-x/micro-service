#!/usr/bin/env bash
set -euo pipefail

KONGA_URL="${KONGA_URL:-http://127.0.0.1:1337}"
KONGA_CONNECTION_NAME="${KONGA_CONNECTION_NAME:-local-kong}"
KONGA_KONG_ADMIN_URL="${KONGA_KONG_ADMIN_URL:-http://kong:8001}"
KONGA_BOOTSTRAP_TIMEOUT="${KONGA_BOOTSTRAP_TIMEOUT:-60}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "bootstrap-konga: $1 is required" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

nodes_json="$(mktemp "${TMPDIR:-/tmp}/konga-nodes.XXXXXX")"
cleanup() {
  rm -f "$nodes_json"
}
trap cleanup EXIT

deadline=$((SECONDS + KONGA_BOOTSTRAP_TIMEOUT))
until curl -fsS --max-time 2 "${KONGA_URL}/api/kongnode" -o "$nodes_json" \
  && jq -e 'type == "array"' "$nodes_json" >/dev/null; do
  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "bootstrap-konga: timed out waiting for ${KONGA_URL}" >&2
    exit 1
  fi
  sleep 1
done

payload="$(
  jq -cn \
    --arg name "$KONGA_CONNECTION_NAME" \
    --arg admin_url "$KONGA_KONG_ADMIN_URL" \
    '{
      name: $name,
      type: "default",
      kong_admin_url: $admin_url,
      active: true,
      health_checks: false
    }'
)"

connection_id="$(
  jq -r --arg name "$KONGA_CONNECTION_NAME" '
    [.[] | select(.name == $name) | .id][0] // empty
  ' "$nodes_json"
)"

if [ -z "$connection_id" ]; then
  response="$(
    curl -fsS --max-time 10 \
      -X POST "${KONGA_URL}/api/kongnode" \
      -H "Content-Type: application/json" \
      --data "$payload"
  )"
  connection_id="$(
    printf '%s' "$response" | jq -r '
      if type == "array" then .[0].id else .id end // empty
    '
  )"
else
  curl -fsS --max-time 10 \
    -X PUT "${KONGA_URL}/api/kongnode/${connection_id}" \
    -H "Content-Type: application/json" \
    --data "$payload" >/dev/null
fi

if [ -z "$connection_id" ]; then
  echo "bootstrap-konga: failed to resolve Konga connection id" >&2
  exit 1
fi

curl -fsS --max-time 2 "${KONGA_URL}/api/kongnode" -o "$nodes_json"
jq -e \
  --argjson id "$connection_id" \
  --arg name "$KONGA_CONNECTION_NAME" \
  --arg admin_url "$KONGA_KONG_ADMIN_URL" \
  '.[] | select(.id == $id and .name == $name and .kong_admin_url == $admin_url and .active == true)' \
  "$nodes_json" >/dev/null

echo ">>> Konga connection ready: ${KONGA_CONNECTION_NAME} -> ${KONGA_KONG_ADMIN_URL}"
