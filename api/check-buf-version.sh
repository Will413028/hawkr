#!/usr/bin/env bash
set -euo pipefail

want=$(cat "$(dirname "$0")/.buf-version")
got=$(buf --version)
if [[ "$got" != "$want" ]]; then
  echo "buf 是 ${got}，api/.buf-version 要求 $want" >&2
  exit 1
fi
