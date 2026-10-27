#!/usr/bin/env bash
# 本機跑單體時帶上 Clerk 的 development instance 的 key（ADR 2026-09-27-identity-clerk）：只從 backend/.env 讀
# Clerk 的設定放進子行程的環境，再執行後面的指令。key 不經過 make 的指令列，也不印出。
# .env 沒有設定時照樣執行：單體只接受本機的 dev token（make dev-token）。
#
#   scripts/clerk-env.sh <指令…>
set -euo pipefail
ENV_FILE="$(cd "$(dirname "$0")/.." && pwd)/.env"
if [ -f "$ENV_FILE" ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      CLERK_PUBLISHABLE_KEY=*|CLERK_SECRET_KEY=*|CLERK_JWT_KEY=*|CLERK_AUTHORIZED_PARTIES=*)
        name=${line%%=*}
        value=${line#*=}
        value=${value%\"}; value=${value#\"}; value=${value%\'}; value=${value#\'}
        if [ -n "$value" ]; then
          export "${name}=${value}"
        fi
        ;;
    esac
  done < "$ENV_FILE"
fi
exec "$@"
