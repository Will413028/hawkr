#!/usr/bin/env bash
set -e


RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SEED_DIR="${SCRIPT_DIR}/seed-data"
BACKEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Hawkr 測試資料初始化${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if ! docker inspect hawkr-postgres &>/dev/null; then
  echo -e "${RED}錯誤: 找不到 hawkr-postgres 容器${NC}"
  echo "請先啟動 infra: cd infra && docker compose up -d"
  exit 1
fi

if [ "$(docker exec hawkr-postgres psql -U dbadmin -d hawkr -tAc "SELECT to_regclass('identity.accounts') IS NOT NULL AND to_regclass('buyer.buyers') IS NOT NULL AND to_regclass('seller.stores') IS NOT NULL AND to_regclass('seller.store_exposures') IS NOT NULL" 2>/dev/null)" != "t" ]; then
  echo -e "${RED}錯誤: 單體的 identity、buyer 或 seller schema 不存在${NC}"
  echo "請先在 backend 跑 make migrate"
  exit 1
fi

declare -a SEEDS=(
  "01-account.sql:hawkr:帳號與買家"
  "@dev-seed-stores:hawkr:賣家（seller 模組）"
  "04-product.sql:hawkr_product:商品"
  "05-coupon.sql:hawkr_coupon:優惠券"
  "06-shipping.sql:hawkr_shipping:配送設定"
  "@reannounce-exposures:hawkr:全量重新公告可曝光"
)

declare -a MAKE_ARGS=()
if [ -n "${MONOLITH_DATABASE_URL:-}" ]; then
  MAKE_ARGS+=("DATABASE_URL=${MONOLITH_DATABASE_URL}")
fi

echo -n -e "${BLUE}[前提檢查]${NC} make dev-seed-check..."
if output=$(make -C "$BACKEND_DIR" dev-seed-check ${MAKE_ARGS[@]+"${MAKE_ARGS[@]}"} 2>&1); then
  echo -e " ${GREEN}成功${NC}"
else
  echo -e " ${RED}失敗，未寫入任何資料${NC}"
  echo "$output" | tail -30
  exit 1
fi

SUCCESS=0
FAILED=0

for entry in "${SEEDS[@]}"; do
  IFS=':' read -r file db label <<< "$entry"
  filepath="${SEED_DIR}/${file}"

  if [[ "$file" == @* ]]; then
    target="${file#@}"
    echo -n -e "${BLUE}[${label}]${NC} make ${target}..."
    if output=$(make -C "$BACKEND_DIR" "$target" ${MAKE_ARGS[@]+"${MAKE_ARGS[@]}"} 2>&1); then
      echo -e " ${GREEN}成功${NC}"
      ((SUCCESS+=1))
    else
      echo -e " ${RED}失敗${NC}"
      echo -e "${RED}  錯誤詳情:${NC}"
      echo "$output" | tail -20
      ((FAILED+=1))
    fi
    continue
  fi

  if [ ! -f "$filepath" ]; then
    echo -e "${RED}[${label}] 跳過 - 找不到 ${file}${NC}"
    ((FAILED+=1))
    continue
  fi

  echo -n -e "${BLUE}[${label}]${NC} 匯入 ${db}..."

  if docker exec -i hawkr-postgres psql -v ON_ERROR_STOP=1 -U dbadmin -d "$db" < "$filepath" > /dev/null 2>&1; then
    echo -e " ${GREEN}成功${NC}"
    ((SUCCESS+=1))
  else
    echo -e " ${RED}失敗${NC}"
    echo -e "${RED}  錯誤詳情:${NC}"
    docker exec -i hawkr-postgres psql -v ON_ERROR_STOP=1 -U dbadmin -d "$db" < "$filepath" 2>&1 | tail -20
    ((FAILED+=1))
  fi
done

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "成功: ${GREEN}${SUCCESS}${NC} / 失敗: ${RED}${FAILED}${NC}"
echo -e "${BLUE}========================================${NC}"

if [ ${FAILED} -gt 0 ]; then
  exit 1
fi

echo ""
echo -e "${GREEN}測試資料初始化完成!${NC}"
echo ""
echo "測試帳號:"
echo "  賣家帳號 9 → 店家 1009 (翠綠有機農場, merId=HAWK9)"
echo "  賣家帳號 4 → 店家 1004 (澎湖鮮味水產, merId=HAWK4)"
echo "  買家 2001 (王小明)"
echo "  買家 2002 (林美玲)"
echo "  買家 3001 (陳大偉，還沒開店)"
echo ""
echo "商品: 10001~10008 (8 件商品, 10 個 SKU)"
echo "優惠券: 50001~50004 (4 張模板, 4 張實例)"
