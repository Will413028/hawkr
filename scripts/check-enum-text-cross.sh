#!/usr/bin/env bash
# =============================================================================
# check-enum-text-cross.sh
#   同一個 proto enum 的「值到文字」對照表在兩個以上 client 各寫一份就失敗
# =============================================================================
# 把 Q3 的「兩端以上進共用文案目錄」機械化（設計 backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md §5.2，
# 規則 E 見 enum-text-guard-design.md §1）。各 client 的工具各自掃自己的原始碼，--report 寫出「此 client 有對照表的 enum 全名」，
# 這裡再交集：
#   TS：packages/hawkr-ui-text-ts/tools/check-enum-text.ts（website、admin、共用 TS 套件 packages/hawkr-api-ts）
#   Dart：packages/hawkr_ui_text/tool/check_enum_text.dart（seller-app、buyer-app、共用 Dart 套件）
# report 只含對照表的形式（case、key、pair：值對應文字的表）；條件形式（if、三元式、&&…）是行為上的分流，不是「兩端各寫一份
# 對照表」，不進 report，否則單端 enum 的分流會因 admin 與 website 各有一處而失敗（設計 K8）。
# 範圍：不在共用文案目錄、也不是伺服器擁有的 enum。目錄裡與伺服器擁有的 enum，由各 client 自己的規則 E 一律擋下
# （任何形式，不只對照表；website／admin 的 lint 與兩支 App 的 enum_text step 都已接上）；共用套件
# （packages/hawkr-api-ts/src 與三個 Dart 套件）沒有自己的 lint，由這支腳本擋：它們的工具 exit 1（受管 enum 的命中）就是失敗。
# 其餘只機械化「兩端以上的對照表就該進目錄」。單端的表（admin 單端 enum）允許。
#
# 相依：pnpm（已在根目錄 install）、dart、jq。用法：bash scripts/check-enum-text-cross.sh
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# 工具的結果只有兩種可信：exit 0（乾淨）或 exit 1（有規則 E 會擋的命中），而且都必須寫出可解析的 report。
# 四個 client 的 exit 1 由它們自己的 lint／CI step 負責，這裡照常取 report；共用套件沒有別的 step，exit 1 在這裡就是失敗。
# 其他 exit code、沒有 report、report 不是 JSON，一律是工具壞了：exit 2，不得當成通過（崩潰的 node 也是 exit 1，
# 所以不能只看 exit code）。
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

# 每個 client 都必須有 report，少一份就是工具壞了。
for client in "${expected_clients[@]}"; do
  if [ ! -s "$tmp/$client.json" ]; then
    echo "check-enum-text-cross：缺少 $client 的 report" >&2
    exit 2
  fi
done

# 各 client 自己的規則負責的 enum：在目錄的、伺服器擁有的。
jq -r '.enums[] | select(.inCatalog or .serverOwned) | .name' "$root/packages/hawkr-ui-text-ts/src/enum-manifest.gen.json" | sort >"$tmp/covered.txt"

# enum <TAB> client <TAB> 位置
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
