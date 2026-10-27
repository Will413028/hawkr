#!/usr/bin/env bash
# 用消費端自己的 buf.gen.yaml 生成對外 API 的程式碼。在消費端的目錄執行：
#   <相對路徑>/api/generate.sh <buf.gen.yaml>
# 先確認 buf CLI 是 .buf-version 的版本（check-buf-version.sh）；清空輸出目錄由各 buf.gen.yaml 的 clean: true 負責。
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "用法：generate.sh <buf.gen.yaml>" >&2
  exit 2
fi

"$(dirname "$0")/check-buf-version.sh"

buf generate --template "$1"
