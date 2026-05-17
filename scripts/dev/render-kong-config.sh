#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

load_env_files

if [ -z "${JWT_SECRET:-}" ]; then
  die "JWT_SECRET is required to render Kong declarative config"
fi
if [ "${#JWT_SECRET}" -lt 32 ]; then
  die "JWT_SECRET must be at least 32 bytes"
fi
case "${JWT_SECRET}" in
  your-*|change-me-*|replace-with-*)
    die "JWT_SECRET still uses a placeholder value"
    ;;
esac

src="${REPO_ROOT}/deployments/kong/declarative.yml"
dst="${REPO_ROOT}/deployments/kong/declarative.local.yaml"

KONG_JWT_SECRET="${JWT_SECRET}" python3 - "$src" "$dst" <<'PY'
import os
import pathlib
import sys

src = pathlib.Path(sys.argv[1])
dst = pathlib.Path(sys.argv[2])
secret = os.environ["KONG_JWT_SECRET"]

raw = src.read_text(encoding="utf-8")
if "__JWT_SECRET__" not in raw:
    raise SystemExit("missing __JWT_SECRET__ placeholder")

rendered = raw.replace("__JWT_SECRET__", secret)
if "__JWT_SECRET__" in rendered:
    raise SystemExit("failed to replace __JWT_SECRET__")

dst.write_text(rendered, encoding="utf-8")
dst.chmod(0o600)
PY

printf 'rendered %s\n' "${dst}"
