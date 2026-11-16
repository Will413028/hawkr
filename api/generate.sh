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

# buf.gen.yaml 的 clean: true 會在呼叫外掛「之前」清空 out 目錄；外掛的指令（例如 dart、pnpm）不在 PATH 時，
# buf 先刪光生成物才失敗。所以先確認每個 local 外掛的第一個字是可執行的指令，全部都在才交給 buf。
missing=()
while IFS= read -r tool; do
  [[ -n "$tool" ]] || continue
  command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done < <(sed -nE 's/^[[:space:]]*-?[[:space:]]*local:[[:space:]]*\[?[[:space:]]*"?([^]",[:space:]]+).*/\1/p' "$1" | sort -u)
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "找不到外掛需要的指令：${missing[*]}（$1 的 local 外掛）；未動生成物" >&2
  exit 1
fi

buf generate --template "$1"
