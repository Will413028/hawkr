#!/usr/bin/env bash
# 用消費端自己的 buf.gen.yaml 生成對外 API 的程式碼。在消費端的目錄執行：
#   <相對路徑>/api/generate.sh <buf.gen.yaml>
# 先確認 buf CLI 是 .buf-version 的版本（check-buf-version.sh）。
#
# buf.gen.yaml 的 clean: true 會在呼叫外掛「之前」清空 out 目錄；外掛失敗（指令不在 PATH、沒先
# dart pub get 或 pnpm install）時生成物已經被刪光。所以先用 buf 的 --output 生成到暫存目錄，
# buf 成功後才把每個 out 目錄整個換成新的：失敗時原本的生成物不動，成功時刪掉的 proto 也不留孤兒檔。
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "用法：generate.sh <buf.gen.yaml>" >&2
  exit 2
fi
template="$1"

"$(dirname "$0")/check-buf-version.sh"

# 各外掛的 out 目錄（相對於目前目錄）。out 一律是單行純量，允許加引號。
outs=()
while IFS= read -r dir; do
  [[ -n "$dir" ]] && outs+=("${dir%/}")
done < <(sed -nE "s/^[[:space:]]*-?[[:space:]]*out:[[:space:]]*[\"']?([^\"'#[:space:]]+).*/\\1/p" "$template" | sort -u)
if [[ ${#outs[@]} -eq 0 ]]; then
  echo "$template 找不到任何外掛的 out；未動生成物" >&2
  exit 1
fi
for dir in "${outs[@]}"; do
  if [[ "$dir" == /* || "$dir" == *..* ]]; then
    echo "out 必須是目前目錄下的相對路徑：${dir}；未動生成物" >&2
    exit 1
  fi
done

staging="$(mktemp -d "${TMPDIR:-/tmp}/hawkr-generate.XXXXXX")"
trap 'rm -rf "$staging"' EXIT

if ! buf generate --template "$template" --output "$staging"; then
  echo "buf generate 失敗；未動生成物" >&2
  exit 1
fi

# 每個生成的檔案都要落在某個 out 目錄底下；有漏掉的代表上面沒讀到某個 out，換進去會少東西。
stray="$(cd "$staging" && find . -type f | sed 's|^\./||' | while IFS= read -r f; do
  ok=0
  for dir in "${outs[@]}"; do [[ "$f" == "$dir"/* ]] && { ok=1; break; }; done
  [[ "$ok" -eq 1 ]] || echo "$f"
done)"
if [[ -n "$stray" ]]; then
  printf '生成了不在已知 out 目錄下的檔案（%s 的 out 沒讀到？）；未動生成物：\n%s\n' "$template" "$stray" >&2
  exit 1
fi

for dir in "${outs[@]}"; do
  mkdir -p "$staging/$dir" "$(dirname "$dir")"
  rm -rf "$dir"
  mv "$staging/$dir" "$dir"
done
