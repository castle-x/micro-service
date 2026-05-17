#!/usr/bin/env bash
set -euo pipefail

LOG_PREFIX="[dev-web]"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

wait_for_web() {
  local pid="$1"
  local port="$2"
  local deadline
  local code

  deadline=$((SECONDS + 30))
  while [ "${SECONDS}" -le "${deadline}" ]; do
    if ! process_alive "${pid}"; then
      log "web pid ${pid} exited before listening on port ${port}"
      return 1
    fi

    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 1 "http://127.0.0.1:${port}/" 2>/dev/null || true)"
    if [ "${code}" = "200" ]; then
      log "web pid ${pid} is ready on ${port}"
      return 0
    fi

    sleep 0.5
  done

  log "web pid ${pid} did not become ready within 30s"
  return 1
}

print_web_failure_context() {
  local log_path

  log_path="$(web_log_path)"
  if [ -f "${REPO_ROOT}/${log_path}" ]; then
    log "last 50 lines from ${log_path}:"
    tail -50 "${REPO_ROOT}/${log_path}" | sed "s/^/${LOG_PREFIX} /" >&2
  else
    log "log file ${log_path} is missing"
  fi
}

start_web() {
  local port
  local pid_file
  local status_file
  local existing_pid
  local pids
  local offender
  local log_path
  local vite_bin
  local pid
  local started_at

  require_lsof
  ensure_run_dirs

  port="$(web_port)"
  pid_file="$(pid_file_for web)"
  status_file="$(status_file_for web)"
  existing_pid="$(read_pid_file web)"
  pids="$(port_pids "${port}")"
  log_path="$(web_log_path)"
  vite_bin="${REPO_ROOT}/web/node_modules/.bin/vite"

  if [ -n "${pids}" ]; then
    if [ -n "${existing_pid}" ] && process_alive "${existing_pid}" && pids_only_match "${pids}" "${existing_pid}"; then
      if [ ! -f "${status_file}" ]; then
        write_status_file web "${existing_pid}" "${port}" "$(iso8601_now)" "$(web_cmd_label)" "${log_path}"
      fi
      log "web already running as pid ${existing_pid} on port ${port}; checking readiness"
      wait_for_web "${existing_pid}" "${port}" || return 1
      return 0
    fi

    offender="$(first_non_matching_pid "${pids}" "${existing_pid}")"
    log "port ${port} occupied by pid ${offender} (not web), refuse to start"
    return 1
  fi

  if [ -n "${existing_pid}" ]; then
    if process_alive "${existing_pid}"; then
      log "web pid file points to alive pid ${existing_pid}, but port ${port} is not listened; refuse to start"
      return 1
    fi

    log "removing stale state for web pid ${existing_pid}"
    rm -f "${pid_file}" "${status_file}"
  fi

  if [ ! -x "${vite_bin}" ]; then
    log "web dependencies are missing; run 'cd web && npm install' first"
    return 1
  fi

  mkdir -p "$(dirname "${REPO_ROOT}/${log_path}")"
  log "starting web on port ${port}; log ${log_path}"
  pid="$(
    cd "${REPO_ROOT}/web"
    nohup ./node_modules/.bin/vite --host 0.0.0.0 --port "${port}" > "${REPO_ROOT}/${log_path}" 2>&1 &
    printf '%s\n' "$!"
  )"

  printf '%s\n' "${pid}" > "${pid_file}"
  started_at="$(iso8601_now)"
  write_status_file web "${pid}" "${port}" "${started_at}" "$(web_cmd_label)" "${log_path}"

  if ! wait_for_web "${pid}" "${port}"; then
    print_web_failure_context
    return 1
  fi
}

stop_web() {
  local pid
  local pid_file
  local status_file
  local deadline

  pid_file="$(pid_file_for web)"
  status_file="$(status_file_for web)"
  pid="$(read_pid_file web)"

  if [ -z "${pid}" ]; then
    log "web has no valid pid file; removing stale status"
    rm -f "${pid_file}" "${status_file}"
    return 0
  fi

  if process_alive "${pid}"; then
    log "stopping web pid ${pid} with SIGTERM"
    kill -TERM "${pid}" 2>/dev/null || true
    deadline=$((SECONDS + 5))
    while [ "${SECONDS}" -le "${deadline}" ]; do
      process_alive "${pid}" || break
      sleep 1
    done

    if process_alive "${pid}"; then
      log "web pid ${pid} did not exit within 5s; sending SIGKILL"
      kill -KILL "${pid}" 2>/dev/null || true
    fi
  else
    log "web pid ${pid} is not alive"
  fi

  rm -f "${pid_file}" "${status_file}"
}

main() {
  local action="${1:-status}"

  require_jq
  case "${action}" in
    start)
      start_web
      ;;
    stop)
      stop_web
      ;;
    restart)
      stop_web
      start_web
      ;;
    status)
      web_status_item_json
      ;;
    *)
      die "usage: scripts/dev/web.sh {start|stop|restart|status}"
      ;;
  esac
}

main "$@"
