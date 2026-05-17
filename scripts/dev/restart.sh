#!/usr/bin/env bash
set -euo pipefail

LOG_PREFIX="[dev-restart]"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/lib.sh
. "${SCRIPT_DIR}/lib.sh"

log "restarting selected dev services"
"${SCRIPT_DIR}/stop.sh" "$@"
"${SCRIPT_DIR}/start.sh" "$@"
