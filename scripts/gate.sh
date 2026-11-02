#!/usr/bin/env bash
# Run declarative lot criteria and retain evidence even when a criterion fails.
set -euo pipefail
cd "$(dirname "$0")/.."
exec python3 scripts/gate.py "$@"
