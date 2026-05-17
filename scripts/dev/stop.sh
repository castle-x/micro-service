#!/usr/bin/env bash
set -euo pipefail

LOG_PREFIX="[dev-stop]"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

stop_service() {
  local service="$1"
  local pid
  local pid_file
  local status_file
  local deadline

  pid_file="$(pid_file_for "${service}")"
  status_file="$(status_file_for "${service}")"
  pid="$(read_pid_file "${service}")"

  if [ -z "${pid}" ]; then
    log "${service} has no valid pid file; removing stale status"
    rm -f "${pid_file}" "${status_file}"
    return 0
  fi

  if process_alive "${pid}"; then
    log "stopping ${service} pid ${pid} with SIGTERM"
    kill -TERM "${pid}" 2>/dev/null || true
    deadline=$((SECONDS + 5))
    while [ "${SECONDS}" -le "${deadline}" ]; do
      process_alive "${pid}" || break
      sleep 1
    done

    if process_alive "${pid}"; then
      log "${service} pid ${pid} did not exit within 5s; sending SIGKILL"
      kill -KILL "${pid}" 2>/dev/null || true
    fi
  else
    log "${service} pid ${pid} is not alive"
  fi

  rm -f "${pid_file}" "${status_file}"
}

stop_legacy_services_for() {
  local service="$1"
  local legacy

  while IFS= read -r legacy; do
    if [ -z "${legacy}" ]; then
      continue
    fi
    log "${service} replaces legacy ${legacy}; cleaning legacy process state"
    stop_service "${legacy}"
  done < <(legacy_services_for "${service}")
}

main() {
  local selection
  local services=()
  local service

  require_jq
  ensure_run_dirs
  selection="$(selected_services "$@")"
  while IFS= read -r service; do
    if [ -n "${service}" ]; then
      services+=("${service}")
    fi
  done <<< "${selection}"

  for service in "${services[@]}"; do
    stop_legacy_services_for "${service}"
    stop_service "${service}"
  done
}

main "$@"
