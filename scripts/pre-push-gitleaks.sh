#!/usr/bin/env bash
# pre-push：只掃這次要推的 commit 範圍（離線、通常不到一秒），與 CI 用同一份設定與基準。
# pre-commit 在 pre-push 階段提供 PRE_COMMIT_FROM_REF／PRE_COMMIT_TO_REF；
# 新分支沒有遠端基準時（FROM 是空的或全 0）改掃 origin/main..TO。
set -euo pipefail

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "gitleaks 沒有安裝，略過 pre-push 檢查（CI 仍會掃完整歷史）：brew install gitleaks" >&2
  exit 0
fi

to="${PRE_COMMIT_TO_REF:-HEAD}"
from="${PRE_COMMIT_FROM_REF:-}"
if [[ -z "$from" || "$from" =~ ^0+$ ]]; then
  from="$(git merge-base origin/main "$to" 2>/dev/null || true)"
fi
range="${from:+$from..}$to"

exec gitleaks git --config .gitleaks.toml --baseline-path .gitleaks-baseline.json \
  --redact --no-banner --log-opts="$range" .
