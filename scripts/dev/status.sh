#!/usr/bin/env bash
set -euo pipefail

LOG_PREFIX="[dev-status]"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

main() {
  local selection
  local services=()
  local items=()
  local service
  local sep=""
  local item

  require_jq
  selection="$(selected_services "$@")"
  while IFS= read -r service; do
    if [ -n "${service}" ]; then
      services+=("${service}")
    fi
  done <<< "${selection}"

  for service in "${services[@]}"; do
    items+=("$(status_item_json "${service}")")
  done

  printf '['
  for item in "${items[@]}"; do
    printf '%s%s' "${sep}" "${item}"
    sep=","
  done
  printf ']\n'
}

main "$@"
