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

assert_eq "iam idp asset edge-api model" "$(service_names | tr '\n' ' ' | sed 's/ $//')" "service order"
assert_eq "iam idp asset edge-api model" "$(selected_services | tr '\n' ' ' | sed 's/ $//')" "default selection"
assert_eq "asset model" "$(selected_services model asset | tr '\n' ' ' | sed 's/ $//')" "selection preserves config order"
assert_eq "38080" "$(service_field edge-api port)" "edge-api port"
assert_eq "48083" "$(service_field model admin_port)" "model admin port"
assert_eq "bin/log/idp.log" "$(service_field idp log_path)" "idp log path"
assert_eq "35173" "$(web_port)" "web port"
assert_eq "bin/log/web.log" "$(web_log_path)" "web log path"

assert_json_expr '.services | length == 5' "${CONFIG_FILE}" "expected five services"
assert_json_expr '[.services[].name] == ["iam","idp","asset","edge-api","model"]' "${CONFIG_FILE}" "expected service order"
assert_json_expr 'all(.services[]; (.name and .port and .admin_port and .binary and .depends_on and .log_path))' "${CONFIG_FILE}" "expected required fields"

grep -Eq 'start_service "\$\{service\}" \|\| return 1' "${SCRIPT_DIR}/start.sh" \
  || fail "start.sh main must propagate start_service failure"
grep -Eq 'scripts/dev/web.sh start' "${REPO_ROOT}/Makefile" \
  || fail "Makefile dev-start must start web"
grep -Eq 'scripts/dev/web.sh stop' "${REPO_ROOT}/Makefile" \
  || fail "Makefile dev-stop must stop web"
grep -Eq 'scripts/dev/web.sh restart' "${REPO_ROOT}/Makefile" \
  || fail "Makefile dev-restart must restart web"

item="$(status_item_json iam)"
printf '%s\n' "${item}" | jq -e '.service == "iam" and .port == 38082 and (.alive | type == "boolean") and has("ready")' >/dev/null \
  || fail "status item JSON shape"

web_item="$(web_status_item_json)"
printf '%s\n' "${web_item}" | jq -e '.service == "web" and .port == 35173 and (.alive | type == "boolean") and has("ready")' >/dev/null \
  || fail "web status item JSON shape"

grep -q "target: 'http://localhost:8000'" "${REPO_ROOT}/web/vite.config.ts" \
  || fail "web dev proxy must route /api through Kong"

printf 'self_check: ok\n'
