#!/usr/bin/env bash
set -euo pipefail

LOG_PREFIX="[dev-start]"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

print_failure_context() {
  local service="$1"
  local log_path="$2"
  local admin_port
  local status_file

  admin_port="$(service_field "${service}" admin_port)"
  print_readyz_context "${service}" "${admin_port}"

  status_file="$(status_file_for "${service}")"
  if [ -f "${status_file}" ]; then
    log "status file ${status_file}:"
    sed "s/^/${LOG_PREFIX} /" "${status_file}" >&2
  else
    log "status file ${status_file} is missing"
  fi

  if [ -f "${REPO_ROOT}/${log_path}" ]; then
    log "last 50 lines from ${log_path}:"
    tail -50 "${REPO_ROOT}/${log_path}" | sed "s/^/${LOG_PREFIX} /" >&2
  else
    log "log file ${log_path} is missing"
  fi
}

wait_for_service() {
  local service="$1"
  local pid="$2"
  local port="$3"
  local admin_port="$4"
  local deadline
  local ready_state

  deadline=$((SECONDS + 30))
  while [ "${SECONDS}" -le "${deadline}" ]; do
    if ! process_alive "${pid}"; then
      log "${service} pid ${pid} exited before listening on port ${port}"
      return 1
    fi

    if port_listened_by_pid "${port}" "${pid}"; then
      if [ -z "${admin_port}" ] || [ "${admin_port}" = "null" ]; then
        log "${service} pid ${pid} is listening on ${port}"
        return 0
      fi

      ready_state="$(ready_probe_for_start "${admin_port}")"
      if [ "${ready_state}" = "true" ]; then
        log "${service} pid ${pid} is listening on ${port} and readyz is healthy"
        return 0
      fi
    fi

    sleep 0.5
  done

  log "${service} pid ${pid} did not become ready within 30s"
  log "${service} process pid ${pid} kept alive for inspection; run make dev-stop to clean up"
  return 1
}

start_service() {
  local service="$1"
  local port
  local admin_port
  local binary
  local log_path
  local pid_file
  local status_file
  local existing_pid
  local pids
  local offender
  local legacy_offender
  local env_args=()
  local line
  local pid
  local started_at
  local cmd

  port="$(service_field "${service}" port)"
  admin_port="$(service_field "${service}" admin_port)"
  binary="$(service_field "${service}" binary)"
  log_path="$(service_field "${service}" log_path)"
  pid_file="$(pid_file_for "${service}")"
  status_file="$(status_file_for "${service}")"

  pids="$(port_pids "${port}")"
  existing_pid="$(read_pid_file "${service}")"
  if [ -n "${pids}" ]; then
    if [ -n "${existing_pid}" ] && process_alive "${existing_pid}" && pids_only_match "${pids}" "${existing_pid}"; then
      if [ ! -f "${status_file}" ]; then
        cmd="$(dev_cmd_label "${binary}")"
        write_status_file "${service}" "${existing_pid}" "${port}" "$(iso8601_now)" "${cmd}" "${log_path}"
      fi
      log "${service} already running as pid ${existing_pid} on port ${port}; checking readiness"
      if ! wait_for_service "${service}" "${existing_pid}" "${port}" "${admin_port}"; then
        print_failure_context "${service}" "${log_path}"
        return 1
      fi
      return 0
    fi

    offender="$(first_non_matching_pid "${pids}" "${existing_pid}")"
    legacy_offender="$(legacy_service_for_pid "${service}" "${offender}")"
    if [ -n "${legacy_offender}" ]; then
      log "port ${port} occupied by legacy ${legacy_offender} pid ${offender}; run make ${service}-restart or make dev-restart to migrate"
    else
      log "port ${port} occupied by pid ${offender} (not ${service}), refuse to start"
    fi
    return 1
  fi

  if [ -n "${existing_pid}" ]; then
    if process_alive "${existing_pid}"; then
      log "${service} pid file points to alive pid ${existing_pid}, but port ${port} is not listened; refuse to start"
      return 1
    fi

    log "removing stale state for ${service} pid ${existing_pid}"
    rm -f "${pid_file}" "${status_file}"
  fi

  if [ ! -x "${REPO_ROOT}/bin/${binary}" ]; then
    log "binary ./bin/${binary} is not executable; run make build first"
    return 1
  fi

  mkdir -p "$(dirname "${REPO_ROOT}/${log_path}")"
  while IFS= read -r line; do
    env_args+=("${line}")
  done < <(dev_env_args)

  log "starting ${service} on port ${port}; log ${log_path}"
  pid="$(
    cd "${REPO_ROOT}"
    nohup env "${env_args[@]}" "./bin/${binary}" > "${log_path}" 2>&1 &
    printf '%s\n' "$!"
  )"

  printf '%s\n' "${pid}" > "${pid_file}"
  started_at="$(iso8601_now)"
  cmd="$(dev_cmd_label "${binary}")"
  write_status_file "${service}" "${pid}" "${port}" "${started_at}" "${cmd}" "${log_path}"

  if ! wait_for_service "${service}" "${pid}" "${port}" "${admin_port}"; then
    print_failure_context "${service}" "${log_path}"
    return 1
  fi
}

main() {
  local selection
  local services=()
  local service

  require_jq
  require_lsof
  ensure_run_dirs
  load_env_files

  selection="$(selected_services "$@")"
  while IFS= read -r service; do
    if [ -n "${service}" ]; then
      services+=("${service}")
    fi
  done <<< "${selection}"

  for service in "${services[@]}"; do
    start_service "${service}" || return 1
  done
}

main "$@"
