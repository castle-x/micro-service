#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

fail() {
  printf 'self_check: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local expected="$1"
  local actual="$2"
  local label="$3"

  if [ "${expected}" != "${actual}" ]; then
    fail "${label}: expected '${expected}', got '${actual}'"
  fi
}

assert_json_expr() {
  local expr="$1"
  local file="$2"
  local label="$3"

  jq -e "${expr}" "${file}" >/dev/null || fail "${label}"
}

assert_grep() {
  local pattern="$1"
  local file="$2"
  local label="$3"

  grep -Eq "${pattern}" "${file}" || fail "${label}"
}

assert_eq "iam idp asset edge-api llm" "$(service_names | tr '\n' ' ' | sed 's/ $//')" "service order"
assert_eq "iam idp asset edge-api llm" "$(selected_services | tr '\n' ' ' | sed 's/ $//')" "default selection"
assert_eq "asset llm" "$(selected_services llm asset | tr '\n' ' ' | sed 's/ $//')" "selection preserves config order"
assert_eq "model" "$(legacy_services_for llm | tr '\n' ' ' | sed 's/ $//')" "llm must clean up legacy model service"
assert_eq "" "$(legacy_services_for asset | tr '\n' ' ' | sed 's/ $//')" "asset must not have legacy cleanup"
assert_eq "38080" "$(service_field edge-api port)" "edge-api port"
assert_eq "48083" "$(service_field llm admin_port)" "llm admin port"
assert_eq "bin/log/idp.log" "$(service_field idp log_path)" "idp log path"
assert_eq "35173" "$(web_port)" "web port"
assert_eq "bin/log/web.log" "$(web_log_path)" "web log path"

assert_json_expr '.services | length == 5' "${CONFIG_FILE}" "expected five services"
assert_json_expr '[.services[].name] == ["iam","idp","asset","edge-api","llm"]' "${CONFIG_FILE}" "expected service order"
assert_json_expr 'all(.services[]; (.name and .port and .admin_port and .binary and .depends_on and .log_path))' "${CONFIG_FILE}" "expected required fields"

assert_grep 'start_service "\$\{service\}" \|\| return 1' "${SCRIPT_DIR}/start.sh" "start.sh main must propagate start_service failure"
assert_grep 'scripts/dev/web.sh start' "${REPO_ROOT}/Makefile" "Makefile dev-start must start web"
assert_grep 'scripts/dev/web.sh stop' "${REPO_ROOT}/Makefile" "Makefile dev-stop must stop web"
assert_grep 'scripts/dev/web.sh restart' "${REPO_ROOT}/Makefile" "Makefile dev-restart must restart web"
assert_grep 'legacy_services_for "\$\{service\}"' "${SCRIPT_DIR}/stop.sh" "stop.sh must clean legacy services during restarts"
assert_grep 'legacy_service_for_pid' "${SCRIPT_DIR}/start.sh" "start.sh must explain legacy port blockers"
assert_grep 'LLM_ENCRYPT_KEY' "${SCRIPT_DIR}/check-env.sh" "check-env must require LLM_ENCRYPT_KEY"
assert_grep '^LLM_ENCRYPT_KEY=' "${REPO_ROOT}/deployments/env/llm.env.example" "llm.env.example must declare LLM_ENCRYPT_KEY"
assert_grep 'openssl rand -base64 32' "${REPO_ROOT}/deployments/env/README.md" "env README must document LLM_ENCRYPT_KEY generation"

item="$(status_item_json iam)"
printf '%s\n' "${item}" | jq -e '.service == "iam" and .port == 38082 and (.alive | type == "boolean") and has("ready")' >/dev/null \
  || fail "status item JSON shape"

web_item="$(web_status_item_json)"
printf '%s\n' "${web_item}" | jq -e '.service == "web" and .port == 35173 and (.alive | type == "boolean") and has("ready")' >/dev/null \
  || fail "web status item JSON shape"

assert_grep "target: 'http://localhost:8000'" "${REPO_ROOT}/web/vite.config.ts" "web dev proxy must route /api through Kong"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/dev-self-check.XXXXXX")"
trap 'rm -rf "${tmp_dir}"' EXIT
mkdir -p "${tmp_dir}/deployments/env"
cat > "${tmp_dir}/deployments/env/infra.env" <<'EOF'
MONGO_URI=mongodb://localhost:27017/platform
REDIS_ADDR=localhost:6379
ETCD_ENDPOINT=127.0.0.1:2379
EOF
cat > "${tmp_dir}/deployments/env/observability.env" <<'EOF'
OPENOBSERVE_AUTH_HEADER=Basic local-local-local-local
EOF
cat > "${tmp_dir}/deployments/env/asset.env" <<'EOF'
ALIYUN_OSS_ACCESS_KEY_ID=local-access-key-id
ALIYUN_OSS_ACCESS_KEY_SECRET=local-access-key-secret
EOF
cat > "${tmp_dir}/deployments/env/secrets.env" <<'EOF'
JWT_SECRET=local-jwt-secret-at-least-thirty-two-bytes
GOOGLE_CLIENT_ID=local-google-client-id
GOOGLE_CLIENT_SECRET=local-google-client-secret
EOF

set +e
ROOT_DIR="${tmp_dir}" ENV_DIR="${tmp_dir}/deployments/env" bash "${SCRIPT_DIR}/check-env.sh" > "${tmp_dir}/check-env.out" 2> "${tmp_dir}/check-env.err"
check_env_status=$?
set -e

assert_eq "1" "${check_env_status}" "check-env must fail when LLM_ENCRYPT_KEY is missing"
assert_json_expr '.missing == ["LLM_ENCRYPT_KEY"]' "${tmp_dir}/check-env.out" "check-env must report only missing LLM_ENCRYPT_KEY"
assert_grep 'llm\.env' "${tmp_dir}/check-env.err" "check-env missing LLM_ENCRYPT_KEY hint must name llm.env"
assert_grep 'openssl rand -base64 32' "${tmp_dir}/check-env.err" "check-env missing LLM_ENCRYPT_KEY hint must include key generation command"

printf 'self_check: ok\n'
