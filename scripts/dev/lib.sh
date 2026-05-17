#!/usr/bin/env bash

: "${LOG_PREFIX:=[dev]}"

DEV_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${DEV_SCRIPT_DIR}/../.." && pwd)"
CONFIG_FILE="${DEV_CONFIG_FILE:-${DEV_SCRIPT_DIR}/services.json}"
RUN_DIR="${REPO_ROOT}/bin/run"
LOG_DIR="${REPO_ROOT}/bin/log"

log() {
  printf '%s %s\n' "${LOG_PREFIX}" "$*" >&2
}

die() {
  log "$*"
  exit 1
}

require_jq() {
  command -v jq >/dev/null 2>&1 || die "jq is required to read ${CONFIG_FILE}"
}

require_lsof() {
  command -v lsof >/dev/null 2>&1 || die "lsof is required to inspect service ports"
}

ensure_run_dirs() {
  mkdir -p "${RUN_DIR}" "${LOG_DIR}"
}

service_names() {
  require_jq
  jq -r '.services[].name' "${CONFIG_FILE}"
}

service_exists() {
  local name="$1"
  require_jq
  [ "$(jq -r --arg name "${name}" '[.services[] | select(.name == $name)] | length' "${CONFIG_FILE}")" != "0" ]
}

service_field() {
  local name="$1"
  local field="$2"

  require_jq
  jq -r --arg name "${name}" --arg field "${field}" '.services[] | select(.name == $name) | .[$field] // empty' "${CONFIG_FILE}"
}

selected_services() {
  if [ "$#" -eq 0 ] || { [ "$#" -eq 1 ] && [ "$1" = "--all" ]; }; then
    service_names
    return
  fi

  local requested=("$@")
  local arg
  for arg in "${requested[@]}"; do
    service_exists "${arg}" || die "unknown service '${arg}'"
  done

  local name
  while IFS= read -r name; do
    for arg in "${requested[@]}"; do
      if [ "${name}" = "${arg}" ]; then
        printf '%s\n' "${name}"
        break
      fi
    done
  done < <(service_names)
}

pid_file_for() {
  printf '%s/%s.pid\n' "${RUN_DIR}" "$1"
}

status_file_for() {
  printf '%s/%s.status\n' "${RUN_DIR}" "$1"
}

read_pid_file() {
  local service="$1"
  local file
  local pid=""

  file="$(pid_file_for "${service}")"
  if [ -f "${file}" ]; then
    IFS= read -r pid < "${file}" || true
  fi

  if [[ "${pid}" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "${pid}"
  fi
}

process_alive() {
  local pid="$1"

  [[ "${pid}" =~ ^[0-9]+$ ]] && kill -0 "${pid}" 2>/dev/null
}

port_pids() {
  local port="$1"
  local output

  require_lsof
  output="$(lsof -nP -ti "tcp:${port}" 2>/dev/null || true)"
  if [ -n "${output}" ]; then
    printf '%s\n' "${output}" | awk 'NF && !seen[$0]++'
  fi
}

port_listened_by_pid() {
  local port="$1"
  local expected_pid="$2"
  local output
  local pid

  require_lsof
  output="$(lsof -nP -t -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)"
  for pid in ${output}; do
    if [ "${pid}" = "${expected_pid}" ]; then
      return 0
    fi
  done

  return 1
}

pids_only_match() {
  local pids="$1"
  local expected_pid="$2"
  local pid
  local seen=0

  for pid in ${pids}; do
    seen=1
    if [ "${pid}" != "${expected_pid}" ]; then
      return 1
    fi
  done

  [ "${seen}" -eq 1 ]
}

first_pid() {
  local pids="$1"
  local pid

  for pid in ${pids}; do
    printf '%s\n' "${pid}"
    return
  done
}

first_non_matching_pid() {
  local pids="$1"
  local expected_pid="${2:-}"
  local pid

  for pid in ${pids}; do
    if [ -z "${expected_pid}" ] || [ "${pid}" != "${expected_pid}" ]; then
      printf '%s\n' "${pid}"
      return
    fi
  done

  first_pid "${pids}"
}

load_env_files() {
  local env_dir="${DEV_ENV_DIR:-${REPO_ROOT}/deployments/env}"
  local files=(
    "${DEV_ENV_FILE:-${REPO_ROOT}/.env}"
    "${env_dir}/infra.env"
    "${env_dir}/observability.env"
    "${env_dir}/asset.env"
    "${env_dir}/model.env"
    "${env_dir}/secrets.env"
    "${env_dir}/overrides.env"
  )
  local env_file

  for env_file in "${files[@]}"; do
    if [ -f "${env_file}" ]; then
      set -a
      # shellcheck source=/dev/null
      . "${env_file}"
      set +a
    fi
  done
}

dev_env_args() {
  printf '%s\n' \
    "OTEL_ENABLED=${OTEL_ENABLED:-true}" \
    "OTEL_ENDPOINT=${OTEL_ENDPOINT:-localhost:4317}" \
    "OTEL_PROTOCOL=${OTEL_PROTOCOL:-grpc}" \
    "OTEL_ENVIRONMENT=${OTEL_ENVIRONMENT:-local}" \
    "OTEL_INSECURE=${OTEL_INSECURE:-true}" \
    "OTEL_STRICT=${OTEL_STRICT:-false}"
}

dev_cmd_label() {
  local binary="$1"
  local env_args=()
  local line

  while IFS= read -r line; do
    env_args+=("${line}")
  done < <(dev_env_args)

  printf 'env'
  for line in "${env_args[@]}"; do
    printf ' %s' "${line}"
  done
  printf ' ./bin/%s\n' "${binary}"
}

iso8601_now() {
  date -u '+%Y-%m-%dT%H:%M:%SZ'
}

write_status_file() {
  local service="$1"
  local pid="$2"
  local port="$3"
  local started_at="$4"
  local cmd="$5"
  local log_path="$6"
  local status_file
  local tmp_file

  status_file="$(status_file_for "${service}")"
  tmp_file="${status_file}.tmp.$$"
  jq -cn \
    --arg service "${service}" \
    --argjson pid "${pid}" \
    --argjson port "${port}" \
    --arg started_at "${started_at}" \
    --arg cmd "${cmd}" \
    --arg log_path "${log_path}" \
    '{service:$service,pid:$pid,port:$port,started_at:$started_at,cmd:$cmd,log_path:$log_path}' > "${tmp_file}"
  mv "${tmp_file}" "${status_file}"
}

read_status_field() {
  local service="$1"
  local field="$2"
  local file

  file="$(status_file_for "${service}")"
  if [ -f "${file}" ]; then
    jq -er --arg field "${field}" '.[$field] // empty' "${file}" 2>/dev/null || true
  fi
}

ready_probe_for_start() {
  local admin_port="$1"
  local code

  if [ -z "${admin_port}" ] || [ "${admin_port}" = "null" ]; then
    printf 'unknown\n'
    return
  fi
  if ! command -v curl >/dev/null 2>&1; then
    printf 'unknown\n'
    return
  fi

  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 1 "http://127.0.0.1:${admin_port}/readyz" 2>/dev/null || true)"
  case "${code}" in
    200)
      printf 'true\n'
      ;;
    ""|000|404)
      printf 'unavailable\n'
      ;;
    *)
      printf 'false\n'
      ;;
  esac
}

print_readyz_context() {
  local service="$1"
  local admin_port="$2"

  if [ -z "${admin_port}" ] || [ "${admin_port}" = "null" ]; then
    return
  fi
  if ! command -v curl >/dev/null 2>&1; then
    log "curl is missing; cannot fetch ${service} readyz response"
    return
  fi

  log "last readyz response from ${service} admin port ${admin_port}:"
  curl -sS -i --max-time 2 "http://127.0.0.1:${admin_port}/readyz" 2>&1 | sed "s/^/${LOG_PREFIX} /" >&2 || true
}

ready_json() {
  local admin_port="$1"
  local code

  if [ -z "${admin_port}" ] || [ "${admin_port}" = "null" ]; then
    printf 'null\n'
    return
  fi
  if ! command -v curl >/dev/null 2>&1; then
    printf 'null\n'
    return
  fi

  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 1 "http://127.0.0.1:${admin_port}/readyz" 2>/dev/null || true)"
  case "${code}" in
    200)
      printf 'true\n'
      ;;
    *)
      printf 'false\n'
      ;;
  esac
}

status_item_json() {
  local service="$1"
  local port
  local admin_port
  local pid
  local pid_json="null"
  local alive_json="false"
  local ready_value
  local started_at
  local log_path

  port="$(service_field "${service}" port)"
  admin_port="$(service_field "${service}" admin_port)"
  pid="$(read_pid_file "${service}")"
  if [ -n "${pid}" ]; then
    pid_json="${pid}"
    if process_alive "${pid}"; then
      alive_json="true"
    fi
  fi

  ready_value="$(ready_json "${admin_port}")"
  started_at="$(read_status_field "${service}" started_at)"
  log_path="$(read_status_field "${service}" log_path)"
  if [ -z "${log_path}" ]; then
    log_path="$(service_field "${service}" log_path)"
  fi

  jq -cn \
    --arg service "${service}" \
    --argjson pid "${pid_json}" \
    --argjson port "${port}" \
    --argjson alive "${alive_json}" \
    --argjson ready "${ready_value}" \
    --arg started_at "${started_at}" \
    --arg log_path "${log_path}" \
    '{service:$service,pid:$pid,port:$port,alive:$alive,ready:$ready,started_at:$started_at,log_path:$log_path}'
}
