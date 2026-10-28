#!/usr/bin/env bash
# 盤點：每個刪除單元在各目錄的引用檔數。從 repo 根目錄執行：bash backend/docs/plans/2026-10-27-ecommerce-scope-cut/inventory.sh [單元]
# 輸出的計數是計畫「規模」表的來源；每一步收尾時再跑一次，該單元應只剩「不能動」列出的命中。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# 排除：文件（*.md）、計畫目錄（本計畫與其他計畫的附件，是歷史盤點）、已套用 migration 的原樣樣本（migrate 測試用，不得改）、
# 素材刪除紀錄（*.tsv）、coverage 基線。
EXCL=(-g '!**/node_modules/**' -g '!*.md' -g '!**/coverage_baseline.json' -g '!*.tsv'
	-g '!backend/docs/plans/**' -g '!backend/internal/platform/migrate/testdata/**')
ROOTS=(backend api packages admin website buyer-app seller-app scripts .github)

count() {
	local name="$1" pattern="$2"
	shift 2
	echo "## ${name}"
	rg -l -i -e "${pattern}" "${EXCL[@]}" "$@" "${ROOTS[@]}" 2>/dev/null |
		awk -F/ '{ if ($1=="backend" && ($2=="services" || $2=="internal")) print $1"/"$2"/"$3; else if ($1=="backend") print $1"/"$2; else print $1"/"$2 }' |
		sort | uniq -c | sort -rn || true
}

unit="${1:-all}"

[[ "${unit}" == all || "${unit}" == recipe ]] &&
	count recipe 'recipe'
[[ "${unit}" == all || "${unit}" == origin ]] &&
	count origin 'OriginPicker|OriginSeller|TaiwanMap|LocationPickerModal|market/origins'
[[ "${unit}" == all || "${unit}" == seasonal ]] &&
	count seasonal 'HomeSeasonal|FruitBurst|seasonal-gradient'
[[ "${unit}" == all || "${unit}" == welfare ]] &&
	count welfare 'corporate-welfare|CorporateWelfare'
[[ "${unit}" == all || "${unit}" == foodreg ]] &&
	count foodreg 'foodreg|food_registration|FoodRegistration|food-registration|食品業者'
[[ "${unit}" == all || "${unit}" == fda ]] &&
	count fda 'fda_?referral|FdaReferral|FDAReferral|case_referrals|食安'
[[ "${unit}" == all || "${unit}" == uber ]] &&
	count uber 'uber_?delivery|UberDelivery|uber-delivery|"UBER|UBER_|provider\.uber|service\.uber'
[[ "${unit}" == all || "${unit}" == pricebook ]] &&
	count pricebook 'pricebook'
[[ "${unit}" == all || "${unit}" == clientfields ]] &&
	count clientfields '(?<!color)temperature|溫層|CHILLED|(?<![-_a-z])FROZEN(?![-_])|deliveryTemp|delivery_temp|goodsType|goods_type|traceab|產銷履歷|organic|有機(?!會|械)|ProductCertification|product[-_]?certification|certification_types' --pcre2
exit 0
