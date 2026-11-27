#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

expected_clients=(website admin ts-packages seller-app buyer-app dart-packages)
strict_clients=" ts-packages dart-packages "

run_tool() {
  local name=$1
  shift
  local rc=0
  "$@" >/dev/null 2>"$tmp/$name.err" || rc=$?
  if [ "$rc" -eq 1 ] && [[ "$strict_clients" == *" $name "* ]]; then
    echo "check-enum-text-cross：$name 在受管 enum 成立的區域裡產生使用者面文字（共用套件沒有別的 lint，這裡擋）：" >&2
    sed 's/^/  /' "$tmp/$name.err" >&2 || true
    exit 1
  fi
  if [ "$rc" -gt 1 ] || [ ! -s "$tmp/$name.json" ] || ! jq -e '.enums | type == "object"' "$tmp/$name.json" >/dev/null 2>&1; then
    echo "check-enum-text-cross：$name 的掃描工具失敗（exit ${rc}，report 缺少或不是 JSON）：$*" >&2
    sed 's/^/  /' "$tmp/$name.err" >&2 || true
    exit 2
  fi
}

ts_client() {
  local name=$1
  shift
  run_tool "$name" pnpm --dir "$root/packages/hawkr-ui-text-ts" exec tsx tools/check-enum-text.ts --report "$tmp/$name.json" "$@"
}

dart_client() {
  local name=$1
  shift
  (cd "$root/packages/hawkr_ui_text" && run_tool "$name" dart run tool/check_enum_text.dart --report "$tmp/$name.json" "$@")
}

ts_client website "$root/website"
ts_client admin "$root/admin"
ts_client ts-packages "$root/packages/hawkr-api-ts/src"
(cd "$root/packages/hawkr_ui_text" && dart pub get >/dev/null) || { echo "check-enum-text-cross：dart pub get 失敗" >&2; exit 2; }
dart_client seller-app "$root/seller-app/lib"
dart_client buyer-app "$root/buyer-app/lib"
dart_client dart-packages "$root/packages/hawkr_api/lib" "$root/packages/hawkr_auth/lib" "$root/packages/hawkr_app_update/lib"

for client in "${expected_clients[@]}"; do
  if [ ! -s "$tmp/$client.json" ]; then
    echo "check-enum-text-cross：缺少 $client 的 report" >&2
    exit 2
  fi
done

jq -r '.enums[] | select(.inCatalog or .serverOwned) | .name' "$root/packages/hawkr-ui-text-ts/src/enum-manifest.gen.json" | sort >"$tmp/covered.txt"

for f in "$tmp"/*.json; do
  jq -r --arg client "$(basename "$f" .json)" '.enums | to_entries[] | [.key, $client, (.value | join(", "))] | @tsv' "$f"
done | sort >"$tmp/all.tsv"
awk -F'\t' 'NR == FNR { covered[$1] = 1; next } !($1 in covered)' "$tmp/covered.txt" "$tmp/all.tsv" >"$tmp/open.tsv"

fail=$(awk -F'\t' '
  { clients[$1] = clients[$1] + 1; where[$1] = where[$1] "\n    " $2 "：" $3 }
  END { for (e in clients) if (clients[e] >= 2) printf "%s（%d 個 client）%s\n", e, clients[e], where[e] }
' "$tmp/open.tsv")

if [ -n "$fail" ]; then
  echo "check-enum-text-cross：同一個 proto enum 的對照表在兩個以上 client 各寫一份：" >&2
  echo "$fail" >&2
  echo "改進共用文案目錄（backend/internal/platform/uitext/catalog；新文案先給 Will 核可），各端改用 @hawkr/ui-text／hawkr_ui_text。" >&2
  exit 1
fi
echo "check-enum-text-cross：沒有 enum 的對照表在兩個以上 client 各寫一份（目錄與伺服器擁有的 enum 由各 client 自己的規則負責；單端的表 $(wc -l <"$tmp/open.tsv" | tr -d ' ') 筆）"
