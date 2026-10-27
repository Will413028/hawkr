#!/usr/bin/env bash
# 確認 buf CLI 是 api/.buf-version 的版本。api/generate.sh 與 backend 的 `make uitext` 共用。
set -euo pipefail

want=$(cat "$(dirname "$0")/.buf-version")
got=$(buf --version)
if [[ "$got" != "$want" ]]; then
  echo "buf 是 ${got}，api/.buf-version 要求 $want" >&2
  exit 1
fi
