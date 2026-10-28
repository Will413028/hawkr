#!/usr/bin/env bash
set -e

# ==============================================================================
# Hawkr - 資料庫初始化腳本
# ==============================================================================
# 用途: 在 PostgreSQL 中建立所有服務需要的資料庫
# 使用方式:
#   ./scripts/init-databases.sh [local|prod]
#
# 說明:
#   - local: 本地 PostgreSQL (localhost:5432)
#   - prod:  正式環境（host 一律由 DB_HOST 環境變數指定，不內建預設值，
#           避免遷移換 DB provider 後腳本悄悄連錯主機）
#           prod 強制 sslmode=require
#   - 預設為 local
# ==============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

ENV="${1:-local}"

if [[ "$ENV" != "local" && "$ENV" != "prod" ]]; then
  echo -e "${RED}錯誤: 環境參數必須是 'local' 或 'prod'${NC}"
  exit 1
fi

# Docker entrypoint context:
# 當本腳本被 mount 進 postgres image 的 /docker-entrypoint-initdb.d/ 時，
# postgres entrypoint 會設好 POSTGRES_USER / POSTGRES_PASSWORD。把它們 bridge
# 到 DB_USER / DB_PASSWORD，讓下面的環境變數驗證在 docker init 階段也能通過。
if [[ -z "${DB_PASSWORD:-}" && -n "${POSTGRES_PASSWORD:-}" ]]; then
  export DB_PASSWORD="$POSTGRES_PASSWORD"
fi
if [[ -z "${DB_USER:-}" && -n "${POSTGRES_USER:-}" ]]; then
  export DB_USER="$POSTGRES_USER"
fi
# 在 postgres container 裡（initdb.d 或 docker exec）走 unix socket。entrypoint 初始化時的暫時
# server 只聽 socket（listen_addresses=''），用預設的 -h localhost 走 TCP 會連不上：腳本失敗、
# container 退出，restart 後又因為資料目錄已存在而跳過初始化，結果一個 database 都沒建。
# 只限 local：prod 的 host 必須明確指定，不能因為環境剛好有 POSTGRES_PASSWORD 就改連本機 socket。
if [[ "$ENV" == "local" && -z "${DB_HOST:-}" && -n "${POSTGRES_PASSWORD:-}" \
      && -S "/var/run/postgresql/.s.PGSQL.${DB_PORT:-5432}" ]]; then
  export DB_HOST=/var/run/postgresql
fi

# 資料庫連線參數
if [[ "$ENV" == "local" ]]; then
  DB_HOST="${DB_HOST:-localhost}"
  DB_PORT="${DB_PORT:-5432}"
  DB_USER="${DB_USER:-dbadmin}"
  DB_PASSWORD="${DB_PASSWORD:?請設定 DB_PASSWORD 環境變數}"
elif [[ "$ENV" == "prod" ]]; then
  # prod 強制 SSL；host 沒有預設值，必須由呼叫端明確指定目前真正的 prod DB
  DB_HOST="${DB_HOST:?請設定 DB_HOST 環境變數（目前真正的 prod DB host）}"
  DB_PORT="${DB_PORT:-5432}"
  DB_USER="${DB_USER:-dbadmin}"
  DB_PASSWORD="${DB_PASSWORD:?請設定 DB_PASSWORD 環境變數（RDS master password）}"
  export PGSSLMODE="require"
else
  DB_HOST="${DB_HOST:?請設定 DB_HOST 環境變數}"
  DB_PORT="${DB_PORT:-5432}"
  DB_USER="${DB_USER:-dbadmin}"
  DB_PASSWORD="${DB_PASSWORD:?請設定 DB_PASSWORD 環境變數}"
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Hawkr 資料庫初始化 - 環境: ${ENV}${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 檢查 psql 是否安裝
if ! command -v psql &> /dev/null; then
  echo -e "${RED}錯誤: 找不到 psql 命令${NC}"
  echo -e "${YELLOW}請先安裝 PostgreSQL client${NC}"
  echo -e "${YELLOW}macOS: brew install postgresql${NC}"
  exit 1
fi

# 定義所有需要建立的資料庫
# hawkr 是單體的 database（每個模組一個 schema，由 hawkr migrate 建立）；
# 其餘是舊服務各自的 database，服務搬進單體並刪除後，從清單移除。
DATABASES=(
  "hawkr"
  "hawkr_seller"
  "hawkr_media"
  "hawkr_shipping"
  "hawkr_im"
  "hawkr_ai"
  "hawkr_payment"
  "hawkr_aftersales"
  "hawkr_order"
  "hawkr_product"
  "hawkr_product_import"
  "hawkr_social"
  "hawkr_cart"
  "hawkr_crm"
  "hawkr_review"
  "hawkr_webhook"
  "hawkr_notification"
  "hawkr_coupon"
  "hawkr_invoice"
)

# PostgreSQL 連線字串
PGPASSWORD="$DB_PASSWORD"
export PGPASSWORD

# 建立資料庫的函數
create_database() {
  local db_name=$1

  # 檢查資料庫是否已存在
  if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${db_name}'" | grep -q 1; then
    echo -e "${YELLOW}[${db_name}]${NC} 已存在，跳過建立"
    return 0
  fi

  # 建立資料庫
  if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE ${db_name};" > /dev/null 2>&1; then
    echo -e "${GREEN}[${db_name}]${NC} ✓ 建立成功"
    return 0
  else
    echo -e "${RED}[${db_name}]${NC} ✗ 建立失敗"
    return 1
  fi
}

# 測試連線
echo -e "${BLUE}測試資料庫連線...${NC}"
if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "SELECT version();" > /dev/null 2>&1; then
  echo -e "${RED}無法連線到 PostgreSQL${NC}"
  echo -e "${YELLOW}請確認:${NC}"
  echo -e "  1. PostgreSQL 服務是否執行"
  if [[ "$ENV" == "local" ]]; then
    echo -e "  2. 執行 'docker-compose up -d postgres'"
  else
    echo -e "  2. 網路連線是否正常"
  fi
  exit 1
fi
echo -e "${GREEN}✓ 連線成功${NC}"
echo ""

# 建立所有資料庫
SUCCESS=0
FAILED=0

for db in "${DATABASES[@]}"; do
  if create_database "$db"; then
    ((SUCCESS+=1))
  else
    ((FAILED+=1))
  fi
done

# 輸出摘要
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}初始化摘要${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "總共: ${#DATABASES[@]} 個資料庫"
echo -e "${GREEN}成功: ${SUCCESS}${NC}"
echo -e "${RED}失敗: ${FAILED}${NC}"

if [ ${FAILED} -gt 0 ]; then
  exit 1
fi

# ==============================================================================
# Setup required extensions
# ==============================================================================
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}設置必要的 extensions${NC}"
echo -e "${BLUE}========================================${NC}"

# pg_trgm extension 用途：
#   - hawkr_product：product / hashtag 的 GIN trgm 索引（原本由 product V1
#     CREATE EXTENSION 處理，自 38 號設計書起改由 init 階段建立，V1 凍結）
declare -A EXTENSIONS=(
  ["hawkr_product"]="pg_trgm"
)

for db in "${!EXTENSIONS[@]}"; do
  ext="${EXTENSIONS[$db]}"
  if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$db" \
       -c "CREATE EXTENSION IF NOT EXISTS ${ext};" > /dev/null 2>&1; then
    echo -e "${GREEN}[${db}]${NC} ✓ ${ext} extension 已啟用"
  else
    echo -e "${RED}[${db}]${NC} ✗ 建立 ${ext} extension 失敗"
    exit 1
  fi
done

echo -e "\n${GREEN}所有資料庫初始化完成! 🎉${NC}"
echo -e "${YELLOW}下一步: Java 服務由 Flyway 於啟動時自動套用 schema；Go 服務執行 ./dev.sh seed（Atlas migrate apply + 測試資料）${NC}"
