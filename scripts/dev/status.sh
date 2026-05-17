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
  local backend_args=()
  local include_web=false
  local service
  local arg
  local sep=""
  local item

  require_jq
  if [ "$#" -eq 0 ] || { [ "$#" -eq 1 ] && [ "$1" = "--all" ]; }; then
    include_web=true
    selection="$(selected_services)"
  else
    for arg in "$@"; do
      if [ "${arg}" = "web" ]; then
        include_web=true
      else
        backend_args+=("${arg}")
      fi
    done

    if [ "${#backend_args[@]}" -gt 0 ]; then
      selection="$(selected_services "${backend_args[@]}")"
    else
      selection=""
    fi
  fi

  if [ -n "${selection}" ]; then
    while IFS= read -r service; do
      if [ -n "${service}" ]; then
        services+=("${service}")
      fi
    done <<< "${selection}"
  fi

  if [ -n "${selection}" ]; then
    for service in "${services[@]}"; do
      items+=("$(status_item_json "${service}")")
    done
  fi
  if [ "${include_web}" = "true" ]; then
    items+=("$(web_status_item_json)")
  fi

  printf '['
  for item in "${items[@]}"; do
    printf '%s%s' "${sep}" "${item}"
    sep=","
  done
  printf ']\n'
}

main "$@"
