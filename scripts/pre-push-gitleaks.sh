#!/usr/bin/env bash
set -euo pipefail

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "gitleaks 沒有安裝，略過 pre-push 檢查（CI 仍會掃完整歷史）：brew install gitleaks" >&2
  exit 0
fi

to="${PRE_COMMIT_TO_REF:-HEAD}"
from="${PRE_COMMIT_FROM_REF:-}"
if [[ -z "$from" || "$from" =~ ^0+$ ]]; then
  from="$(git merge-base origin/dev "$to" 2>/dev/null || true)"
fi
range="${from:+$from..}$to"

exec gitleaks git --config .gitleaks.toml --baseline-path .gitleaks-baseline.json \
  --redact --no-banner --log-opts="$range --not --remotes=origin" .
