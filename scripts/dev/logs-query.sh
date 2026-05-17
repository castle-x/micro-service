#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: logs-query.sh [--service=<name>] [--trace-id=<id>] [--level=error,warn] [--since=15m] [--limit=200] [--format=json]

Environment:
  LOG_DIR  Log directory to read. Defaults to <repo>/bin/log.
EOF
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    printf '[]\n'
    echo "logs-query: jq is required" >&2
    exit 127
  fi
}

since_to_seconds() {
  local value="$1"
  if [[ -z "$value" ]]; then
    printf '0\n'
    return
  fi
  if [[ "$value" =~ ^([0-9]+)([smhd])$ ]]; then
    local amount="${BASH_REMATCH[1]}"
    local unit="${BASH_REMATCH[2]}"
    case "$unit" in
      s) printf '%s\n' "$amount" ;;
      m) printf '%s\n' "$((amount * 60))" ;;
      h) printf '%s\n' "$((amount * 3600))" ;;
      d) printf '%s\n' "$((amount * 86400))" ;;
    esac
    return
  fi
  echo "logs-query: invalid --since value: $value (expected e.g. 15m, 1h, 30s)" >&2
  return 2
}

service=""
trace_id=""
levels=""
since=""
limit="200"
format="json"

for arg in "$@"; do
  case "$arg" in
    --service=*) service="${arg#*=}" ;;
    --trace-id=*) trace_id="${arg#*=}" ;;
    --level=*) levels="${arg#*=}" ;;
    --since=*) since="${arg#*=}" ;;
    --limit=*) limit="${arg#*=}" ;;
    --format=*) format="${arg#*=}" ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "logs-query: unknown argument: $arg" >&2
      usage
      printf '[]\n'
      exit 2
      ;;
  esac
done

if [[ "$format" != "json" ]]; then
  echo "logs-query: only --format=json is supported" >&2
  printf '[]\n'
  exit 2
fi

if ! [[ "$limit" =~ ^[0-9]+$ ]] || [[ "$limit" -lt 1 ]]; then
  echo "logs-query: --limit must be a positive integer" >&2
  printf '[]\n'
  exit 2
fi

require_jq

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
log_dir="${LOG_DIR:-$repo_root/bin/log}"
if ! since_seconds="$(since_to_seconds "$since")"; then
  printf '[]\n'
  exit 2
fi

shopt -s nullglob
files=()
if [[ -n "$service" ]]; then
  if [[ -f "$log_dir/$service.log" ]]; then
    files=("$log_dir/$service.log")
  fi
else
  files=("$log_dir"/*.log)
fi
shopt -u nullglob

if [[ "${#files[@]}" -eq 0 ]]; then
  printf '[]\n'
  echo "logs-query: bad_json_lines=0" >&2
  exit 0
fi

tmp_result="$(mktemp)"
trap 'rm -f "$tmp_result"' EXIT

if ! jq -R -s \
  --arg service "$service" \
  --arg trace_id "$trace_id" \
  --arg levels "$levels" \
  --argjson since_seconds "$since_seconds" \
  --argjson limit "$limit" \
  '
  def parsed_line:
    try {ok: true, value: fromjson} catch {ok: false};

  def level_allowed:
    . as $record
    |
    if $levels == "" then
      true
    else
      ($levels | split(",") | map(gsub("^\\s+|\\s+$"; "") | ascii_downcase) | map(select(length > 0))) as $wanted
      | $wanted | index(($record.level // "" | tostring | ascii_downcase)) != null
    end;

  def parsed_time:
    if (.time? | type) == "string" then
      ((.time | fromdateiso8601?)
        // (.time | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601?))
    else
      null
    end;

  def time_allowed:
    if $since_seconds <= 0 then
      true
    else
      parsed_time as $ts
      | if $ts == null then true else $ts >= (now - $since_seconds) end
    end;

  def record_allowed:
    ($service == "" or (.service // "") == $service)
    and ($trace_id == "" or (.trace_id // "") == $trace_id)
    and level_allowed
    and time_allowed;

  reduce (split("\n")[] | select(length > 0)) as $line
    ({bad: 0, items: []};
      ($line | parsed_line) as $parsed
      | if ($parsed.ok | not) or (($parsed.value | type) != "object") then
          .bad += 1
        elif ($parsed.value | record_allowed) then
          .items += [$parsed.value]
        else
          .
        end
    )
  | .items |= (if length > $limit then .[-$limit:] else . end)
  ' "${files[@]}" >"$tmp_result"; then
  printf '[]\n'
  echo "logs-query: jq query failed" >&2
  exit 1
fi

jq '.items' "$tmp_result"
bad_lines="$(jq -r '.bad' "$tmp_result")"
echo "logs-query: bad_json_lines=$bad_lines" >&2
