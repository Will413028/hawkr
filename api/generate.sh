#!/usr/bin/env bash
# 用消費端自己的 buf.gen.yaml 生成對外 API 的程式碼。在消費端的目錄執行：
#   <相對路徑>/api/generate.sh <buf.gen.yaml>
# 先確認 buf CLI 是 .buf-version 的版本（check-buf-version.sh）。
#
# buf.gen.yaml 的 clean: true 會在呼叫外掛「之前」清空 out 目錄；外掛失敗（指令不在 PATH、沒先
# dart pub get 或 pnpm install）時生成物已經被刪光。所以先用 buf 的 --output 生成到目前目錄下的
# 暫存目錄（同一個檔案系統，mv 是 rename），buf 成功後才把每個最上層的 out 目錄換成新的：
# 失敗時原本的生成物不動，成功時刪掉的 proto 也不留孤兒檔。交換是「舊的改名成 .gen-old → 新的
# 搬進來 → 刪 .gen-old」，中途失敗或被中斷時 EXIT trap 會把 .gen-old 搬回去，任何時間點不是舊的
# 就是新的生成物在原位。
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
  while [[ "$dir" == ./* ]]; do dir="${dir#./}"; done
  while [[ "$dir" == */ ]]; do dir="${dir%/}"; done
  [[ -n "$dir" ]] || continue
  outs+=("$dir")
done < <(sed -nE "s/^[[:space:]]*-?[[:space:]]*out:[[:space:]]*[\"']?([^\"'#[:space:]]+).*/\\1/p" "$template" | sort -u)
if [[ ${#outs[@]} -eq 0 ]]; then
  echo "$template 找不到任何外掛的 out；未動生成物" >&2
  exit 1
fi
for dir in "${outs[@]}"; do
  bad=0
  [[ "$dir" == /* || "$dir" == . ]] && bad=1
  IFS=/ read -r -a parts <<< "$dir"
  for p in "${parts[@]}"; do [[ "$p" == .. || "$p" == . ]] && bad=1; done
  if [[ "$bad" -eq 1 ]]; then
    echo "out 必須是目前目錄下的相對路徑（不得含 . 或 .. 段）：${dir}；未動生成物" >&2
    exit 1
  fi
done

# 巢狀的 out（lib/gen 與 lib/gen/connect）只交換最上層的那個。
tops=()
for dir in "${outs[@]}"; do
  nested=0
  for other in "${outs[@]}"; do
    [[ "$other" != "$dir" && "$dir" == "$other"/* ]] && { nested=1; break; }
  done
  [[ "$nested" -eq 1 ]] || tops+=("$dir")
done

staging="$(mktemp -d ./.gen-staging.XXXXXX)"
pending_dir=""
pending_old=""
cleanup() {
  # 交換做到一半（舊的已改名、新的還沒到位）時把舊的搬回去。
  if [[ -n "$pending_old" && -e "$pending_old" && ! -e "$pending_dir" ]]; then
    mv "$pending_old" "$pending_dir" && echo "交換中斷；已還原 ${pending_dir}" >&2
  fi
  rm -rf "$staging"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP

if ! buf generate --template "$template" --output "$staging"; then
  echo "buf generate 失敗；未動生成物" >&2
  exit 1
fi

# 每個生成的檔案都要落在某個 out 目錄底下；有漏掉的代表上面沒讀到某個 out，換進去會少東西。
stray="$(cd "$staging" && find . -type f | sed 's|^\./||' | while IFS= read -r f; do
  ok=0
  for dir in "${tops[@]}"; do [[ "$f" == "$dir"/* ]] && { ok=1; break; }; done
  [[ "$ok" -eq 1 ]] || echo "$f"
done)"
if [[ -n "$stray" ]]; then
  printf '生成了不在已知 out 目錄下的檔案（%s 的 out 沒讀到？）；未動生成物：\n%s\n' "$template" "$stray" >&2
  exit 1
fi

# buf 沒在某個 out 寫出任何檔案時不換：換進去等於把真的目錄刪光。
for dir in "${tops[@]}"; do
  if [[ -z "$(find "$staging/$dir" -type f 2>/dev/null | head -n 1)" ]]; then
    echo "buf 沒有在 ${dir} 生成任何檔案；未動生成物" >&2
    exit 1
  fi
  if [[ -e "$dir.gen-old" ]]; then
    echo "${dir}.gen-old 已存在（上次中斷留下的？確認後刪掉再跑）；未動生成物" >&2
    exit 1
  fi
done

for dir in "${tops[@]}"; do
  mkdir -p "$(dirname "$dir")"
  if [[ -e "$dir" ]]; then
    pending_dir="$dir"
    pending_old="$dir.gen-old"
    mv "$dir" "$pending_old"
    if ! mv "$staging/$dir" "$dir"; then
      echo "搬入新的 ${dir} 失敗" >&2
      exit 1
    fi
    pending_dir=""
    pending_old=""
    rm -rf "$dir.gen-old"
  else
    mv "$staging/$dir" "$dir"
  fi
done
