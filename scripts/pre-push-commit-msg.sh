#!/usr/bin/env bash
# pre-push：檢查這次要推的每個 commit 訊息。commit-msg hook 擋不到 git revert、
# git cherry-pick、git commit-tree 建的 commit（它們不跑 commit-msg），推之前在這裡補擋。
# 範圍推導與 pre-push-gitleaks.sh 相同；規則與 CI 同一支腳本。
# 已在 origin 任一分支上的 commit 不算「要推的」：merge 進來的 dev commit 不是這次推送的內容。
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
