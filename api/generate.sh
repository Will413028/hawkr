#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "用法：generate.sh <buf.gen.yaml>" >&2
  exit 2
fi
template="$1"

"$(dirname "$0")/check-buf-version.sh"

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

stray="$(cd "$staging" && find . -type f | sed 's|^\./||' | while IFS= read -r f; do
  ok=0
  for dir in "${tops[@]}"; do [[ "$f" == "$dir"/* ]] && { ok=1; break; }; done
  [[ "$ok" -eq 1 ]] || echo "$f"
done)"
if [[ -n "$stray" ]]; then
  printf '生成了不在已知 out 目錄下的檔案（%s 的 out 沒讀到？）；未動生成物：\n%s\n' "$template" "$stray" >&2
  exit 1
fi

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
