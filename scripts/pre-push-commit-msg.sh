#!/usr/bin/env bash
set -euo pipefail

to="${PRE_COMMIT_TO_REF:-HEAD}"
from="${PRE_COMMIT_FROM_REF:-}"
if [[ -z "$from" || "$from" =~ ^0+$ ]]; then
  from="$(git merge-base origin/dev "$to" 2>/dev/null || true)"
fi
if [[ -z "$from" ]]; then
  exec python3 scripts/check-commit-msg.py --range "$to^!" --not --remotes=origin
fi
exec python3 scripts/check-commit-msg.py --range "$from..$to" --not --remotes=origin
