#!/usr/bin/env bash
set -u

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
ENV_DIR="${ENV_DIR:-$ROOT_DIR/deployments/env}"

REQUIRED_KEYS="MONGO_URI REDIS_ADDR ETCD_ENDPOINT JWT_SECRET GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET ALIYUN_OSS_ACCESS_KEY_ID ALIYUN_OSS_ACCESS_KEY_SECRET OPENOBSERVE_AUTH_HEADER MODEL_ENCRYPT_KEY"

ENV_FILES=(
  "$ROOT_DIR/.env"
  "$ENV_DIR/infra.env"
  "$ENV_DIR/observability.env"
  "$ENV_DIR/asset.env"
  "$ENV_DIR/model.env"
  "$ENV_DIR/secrets.env"
  "$ENV_DIR/overrides.env"
)

existing_files=()
for file in "${ENV_FILES[@]}"; do
  if [ -f "$file" ]; then
    existing_files+=("$file")
  fi
done

if [ -f "$ROOT_DIR/.env" ]; then
  printf '%s\n' "warning: root .env is loaded at lowest priority; migrate values to deployments/env/." >&2
fi

raw="$(mktemp "${TMPDIR:-/tmp}/check-env.raw.XXXXXX")"
missing="$(mktemp "${TMPDIR:-/tmp}/check-env.missing.XXXXXX")"
placeholder="$(mktemp "${TMPDIR:-/tmp}/check-env.placeholder.XXXXXX")"
duplicates="$(mktemp "${TMPDIR:-/tmp}/check-env.duplicates.XXXXXX")"

cleanup() {
  rm -f "$raw" "$missing" "$placeholder" "$duplicates"
}
trap cleanup EXIT

: > "$raw"

run_validation() {
  awk -v required="$REQUIRED_KEYS" '
    function trim(s) {
      sub(/^[[:space:]]+/, "", s)
      sub(/[[:space:]]+$/, "", s)
      return s
    }

    function unquote(s) {
      s = trim(s)
      if (length(s) >= 2) {
        first = substr(s, 1, 1)
        last = substr(s, length(s), 1)
        if ((first == "\"" && last == "\"") || (first == "'\''" && last == "'\''")) {
          s = substr(s, 2, length(s) - 2)
        }
      }
      return s
    }

    BEGIN {
      split(required, req, " ")
    }

    {
      line = $0
      sub(/\r$/, "", line)
      line = trim(line)

      if (line == "" || substr(line, 1, 1) == "#") {
        next
      }

      if (line ~ /^export[[:space:]]+/) {
        sub(/^export[[:space:]]+/, "", line)
      }

      if (line !~ /^[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=/) {
        next
      }

      key = line
      sub(/[[:space:]]*=.*/, "", key)
      key = trim(key)

      value = line
      sub(/^[^=]*=/, "", value)
      value = unquote(value)

      pair = key SUBSEP FILENAME
      if (!(pair in seen_file)) {
        if (key in seen_any) {
          duplicate[key] = 1
        }
        seen_file[pair] = 1
        seen_any[key] = 1
      }

      values[key] = value
    }

    END {
      for (i in req) {
        key = req[i]
        if (!(key in values) || values[key] == "") {
          print "MISSING\t" key
        }
      }

      for (key in values) {
        if (values[key] ~ /^(your-|change-me-|replace-with-)/) {
          print "PLACEHOLDER\t" key
        }
      }

      for (key in duplicate) {
        print "DUPLICATE\t" key
      }
    }
  ' "$@" /dev/null > "$raw"
}

if [ "${#existing_files[@]}" -gt 0 ]; then
  run_validation "${existing_files[@]}"
else
  run_validation
fi

awk -F '\t' '$1 == "MISSING" { print $2 }' "$raw" | sort -u > "$missing"
awk -F '\t' '$1 == "PLACEHOLDER" { print $2 }' "$raw" | sort -u > "$placeholder"
awk -F '\t' '$1 == "DUPLICATE" { print $2 }' "$raw" | sort -u > "$duplicates"

json_array_file() {
  file="$1"
  first=1
  printf '['
  while IFS= read -r item; do
    if [ -z "$item" ]; then
      continue
    fi
    escaped="${item//\\/\\\\}"
    escaped="${escaped//\"/\\\"}"
    if [ "$first" -eq 1 ]; then
      first=0
    else
      printf ','
    fi
    printf '"%s"' "$escaped"
  done < "$file"
  printf ']'
}

if [ -s "$missing" ] || [ -s "$placeholder" ]; then
  ok=false
  exit_code=1
else
  ok=true
  exit_code=0
fi

printf '{"ok":%s,"missing":' "$ok"
json_array_file "$missing"
printf ',"placeholder":'
json_array_file "$placeholder"
printf ',"duplicates":'
json_array_file "$duplicates"
printf '}\n'

exit "$exit_code"
