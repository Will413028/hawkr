#!/usr/bin/env bash
set -e

# ==============================================================================
# Hawkr - 測試資料初始化腳本
# ==============================================================================
# 用途: 重建 DB 後一鍵匯入測試資料
# 使用方式:
#   ./infra/seed-test-data.sh
#
# 前置條件:
#   1. Docker 容器 hawkr-postgres 正在運行
#   2. 所有服務的 migration 已完成（Go: migrate-all.sh, Java: Flyway）
#   3. 以下資料庫已建立且有表：
#      - hawkr（單體；identity.accounts、account_roles、buyer.buyers、shipping_addresses 與 seller.stores，先跑 backend 的 make migrate）
#      - hawkr_product (category, product, product_sku, store_exposure)
#      - hawkr_coupon (coupon_templates, coupon_instances, seller_coupon_quotas)
#      - hawkr_shipping (seller_shipment_settings)
#
# 順序: 前提檢查（dev-seed-check，01 重建的帳號擁有種子以外的店家就中止）→ 01 帳號與買家 → 賣家（backend 的 make dev-seed-stores：seller 模組把種子店家還原成 fixture（upsert，不碰種子以外的店家），KYC 識別欄位以執行中的金鑰加密）
#       → 04 商品 → 05 優惠券 → 06 配送設定 → 全量重新公告可曝光（make reannounce-exposures）。
#       事件由 make worker 送到 RabbitMQ；沒有跑 worker 時事件留在資料庫，worker 啟動後送出。
#       dev-seed-stores 與 reannounce-exposures 要連單體的資料庫：預設是 backend/Makefile 的 DATABASE_URL；要覆寫（例如本機的 Homebrew
#       postgres 蓋過 docker 的 5432）時設定 MONOLITH_DATABASE_URL。
#
# 測試帳號與店家 ID 約定:
#   賣家帳號: 9, 4；店家: store_id 1009, 1004（與 account_id 刻意不同），store_code 9, 4 → PayUni merId HAWK9, HAWK4
#   買家: 2001~2099
#   還沒開店的買家: 3001~3099（沒有 SELLER 角色、沒有店；開店時 seller 模組才在同一個 transaction 建店並授予角色）
#   buyer_id: account_id + 50000（與 account_id 刻意不同，見 01-account.sql）
#   商品: 10001~10099
#   優惠券模板: 50001~50099
#   優惠券實例: 60001~60099
# ==============================================================================

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

# 檢查 postgres 容器
if ! docker inspect hawkr-postgres &>/dev/null; then
  echo -e "${RED}錯誤: 找不到 hawkr-postgres 容器${NC}"
  echo "請先啟動 infra: cd infra && docker compose up -d"
  exit 1
fi

# 賣家在單體的 seller schema（seed 不代跑單體的 migration）：沒有就明確失敗，不往下寫
if [ "$(docker exec hawkr-postgres psql -U dbadmin -d hawkr -tAc "SELECT to_regclass('identity.accounts') IS NOT NULL AND to_regclass('buyer.buyers') IS NOT NULL AND to_regclass('seller.stores') IS NOT NULL AND to_regclass('seller.store_exposures') IS NOT NULL" 2>/dev/null)" != "t" ]; then
  echo -e "${RED}錯誤: 單體的 identity、buyer 或 seller schema 不存在${NC}"
  echo "請先在 backend 跑 make migrate"
  exit 1
fi

# 定義 seed 檔案與對應 DB；@ 開頭是 backend 的 make 目標（seller 模組的種子與全量重新公告），不是 SQL 檔
declare -a SEEDS=(
  "01-account.sql:hawkr:帳號與買家"
  "@dev-seed-stores:hawkr:賣家（seller 模組）"
  "04-product.sql:hawkr_product:商品"
  "05-coupon.sql:hawkr_coupon:優惠券"
  "06-shipping.sql:hawkr_shipping:配送設定"
  "@reannounce-exposures:hawkr:全量重新公告可曝光"
)

# make 目標的 DATABASE_URL 覆寫（Makefile 的賦值優先於環境變數，只能放在命令列）
declare -a MAKE_ARGS=()
if [ -n "${MONOLITH_DATABASE_URL:-}" ]; then
  MAKE_ARGS+=("DATABASE_URL=${MONOLITH_DATABASE_URL}")
fi

# 前提檢查（fail fast，在 01 之前）：01 會刪除重建種子帳號（9、4、2001~2099、3001~3099）；這些帳號若擁有種子店家以外的店家，重建後店家會失去
# SELLER 角色。這裡擋下，不寫任何資料，也不自動刪店。
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
