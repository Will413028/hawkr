#!/usr/bin/env bash
# 在本機起單體（入口）與 gateway，經入口跑一個模組的特性測試（internal/httpapi/<模組>/characterization 裡
# build tag gateway 的測試；ADR 2026-09-24-go-modular-monolith D8、ADR 2026-09-26-external-contract-protobuf-connect
# D4）。搬遷中的模組另外起它的舊服務，同一份情境以 REST 經入口與 gateway 打舊服務、以 Connect 打單體。呼叫者的身分
# 是以 hawkr dev-token 簽的 token（本機的暫時金鑰，不連任何外部 provider）。
#
# 只用 docker-compose-infra.yml 的 Postgres 與 Redis（先 make infra-up）；舊服務與單體呼叫的 media-service 是
# 測試自己聽的假服務（characterize.ServeFakeMedia）。服務以 env -i 啟動，不讀 .env、不繼承 shell 的環境變數，
# 不會連到任何外部服務。在 backend/ 執行：
#
#   scripts/characterize.sh <模組>
#
# 本機原生的 Postgres 佔著 loopback 的 5432 時（症狀是 database 不存在），用 PGHOST 指定 docker 的位址：
#
#   PGHOST=$(ipconfig getifaddr en0) scripts/characterize.sh <模組>
#
# 舊服務：模組的 characterization 目錄有 legacy.sh 時，這支腳本 source 它，並在起 gateway 之前呼叫它定義的
# legacy_start。legacy_start 可以用下面的 create_db、db_url、start_bg、require_free_port、clean_env 與 PGHOST、
# PG_USER、PG_PASSWORD、LOG_DIR、FAKE_MEDIA_PORT，要做的事：
#   - 建自己的 database、套用舊服務的 migration、起舊服務（start_bg）；
#   - 把 gateway 連到舊服務要的設定加進 gateway_env 陣列，例如 gateway_env+=(X_SERVICE_HOST=127.0.0.1)。
#   舊服務呼叫 account-service 與 seller-service 的 gRPC 時，連單體的 legacy listener（127.0.0.1:${LEGACY_PORT}）。
# 範本在 make new-module 產生的 legacy.sh。舊服務刪除時連同 legacy.sh 一起刪除。
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -ne 1 ]; then
  echo "用法：scripts/characterize.sh <模組>" >&2
  exit 2
fi
MODULE=$1
PKG_DIR="internal/httpapi/${MODULE}/characterization"
if [ ! -d "$PKG_DIR" ]; then
  echo "找不到 ${PKG_DIR}" >&2
  exit 2
fi

PGHOST=${PGHOST:-localhost}
PG_USER=dbadmin
PG_PASSWORD=Hawkr2024Admin
MONOLITH_DB=characterize_monolith
FAKE_MEDIA_PORT=19937
MONOLITH_PORT=18080
LEGACY_PORT=19941
GATEWAY_PORT=19080
# gateway 的聊天 WebSocket ticket 用 JWT_SECRET。
JWT_SECRET=$(openssl rand -hex 32)
# gateway 開機時要求 IM 串流的服務間密鑰（與特性測試無關），給一個只在這次用的值。
IM_SECRET=$(openssl rand -hex 32)
LOG_DIR=$(mktemp -d)
pids=()
dbs=()
gateway_env=()

psql_admin() { docker exec hawkr-postgres psql -U "$PG_USER" -d postgres -q "$@"; }

cleanup() {
  for pid in ${pids[@]+"${pids[@]}"}; do
    kill "$pid" 2>/dev/null || true
  done
  for db in ${dbs[@]+"${dbs[@]}"}; do
    psql_admin -c "DROP DATABASE IF EXISTS ${db} WITH (FORCE)" >/dev/null || true
  done
  echo "服務的 log：${LOG_DIR}"
}
trap cleanup EXIT

# 以乾淨的環境執行：只帶 PATH、HOME，其餘一律由參數指定。在 subshell 裡以 exec 取代自己，背景執行時
# $! 就是服務本身，結束時才殺得到它（殺 subshell 殺不到它的子行程）。
clean_env() { exec env -i PATH="$PATH" HOME="$HOME" "$@"; }

# start_bg <名稱> <env 與指令…>：以乾淨的環境在背景起服務，log 寫到 LOG_DIR/<名稱>.log。
start_bg() {
  local name=$1
  shift
  (clean_env "$@") >"${LOG_DIR}/${name}.log" 2>&1 &
  pids+=($!)
}

# create_db <名稱>：建一個空的 database，結束時刪除。
create_db() {
  psql_admin -c "DROP DATABASE IF EXISTS $1 WITH (FORCE)" -c "CREATE DATABASE $1" >/dev/null
  dbs+=("$1")
}

# db_url <名稱>：database 的連線字串。
db_url() { echo "postgres://${PG_USER}:${PG_PASSWORD}@${PGHOST}:5432/$1?sslmode=disable"; }

# 上一次沒有結束乾淨的服務還佔著 port 時，新的服務起不來，測試卻會打到舊的那一個。
require_free_port() {
  if lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $1 已經有程式在聽，先停掉它：lsof -nP -iTCP:$1 -sTCP:LISTEN" >&2
    exit 1
  fi
}

for port in "$FAKE_MEDIA_PORT" "$MONOLITH_PORT" "$LEGACY_PORT" "$GATEWAY_PORT"; do
  require_free_port "$port"
done

legacy=false
if [ -f "${PKG_DIR}/legacy.sh" ]; then
  # shellcheck source=/dev/null
  source "${PKG_DIR}/legacy.sh"
  legacy=true
fi

# 先建好執行檔再啟動：pids 就是服務本身，結束時才停得乾淨。
(cd gateway && go build -o "${LOG_DIR}/gateway" ./cmd)
GOWORK=off go build -o "${LOG_DIR}/hawkr" ./cmd/hawkr

create_db "$MONOLITH_DB"
MONOLITH_DB_URL=$(db_url "$MONOLITH_DB")
(clean_env DATABASE_URL="$MONOLITH_DB_URL" "${LOG_DIR}/hawkr" migrate) >"${LOG_DIR}/monolith-migrate.log" 2>&1
# 身分是本機的暫時金鑰簽的 dev token（不連任何外部 provider）；代行讀店家狀態在行程內
# 讀 seller 模組，不需要另外的服務位址。
DEV_AUTH_KEY_FILE="${LOG_DIR}/dev-auth.pem"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$DEV_AUTH_KEY_FILE" 2>/dev/null
# KYC 的加密與比對金鑰是必填，每次執行產生新的（兩把必須不同）。
KYC_ENCRYPTION_KEYS="t1:$(openssl rand -base64 32)"
KYC_HASH_KEYS="t1:$(openssl rand -base64 32)"
start_bg monolith \
  DATABASE_URL="$MONOLITH_DB_URL" HTTP_ADDR="127.0.0.1:${MONOLITH_PORT}" LEGACY_GRPC_ADDR="127.0.0.1:${LEGACY_PORT}" \
  MEDIA_SERVICE_ADDR="127.0.0.1:${FAKE_MEDIA_PORT}" MEDIA_ORIGIN=https://media.invalid PUBLIC_WEB_ORIGIN=https://web.invalid \
  GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}" \
  PRODUCT_SERVICE_ADDR=127.0.0.1:1 ORDER_SERVICE_ADDR=127.0.0.1:1 CRM_SERVICE_ADDR=127.0.0.1:1 REVIEW_SERVICE_ADDR=127.0.0.1:1 \
  CORS_ALLOWED_ORIGINS=http://localhost:3000 \
  DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" KYC_ENCRYPTION_KEYS="$KYC_ENCRYPTION_KEYS" KYC_HASH_KEYS="$KYC_HASH_KEYS" \
  "${LOG_DIR}/hawkr" serve

# dev_token <參數…>：以 hawkr dev-token 簽一個 token，印出。
dev_token() {
  (clean_env DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" "${LOG_DIR}/hawkr" dev-token "$@")
}
BUYER_TOKEN=$(dev_token -user dev_characterize_buyer)
ADMIN_TOKEN=$(dev_token -admin -user dev_characterize_admin)

# 管理員第一次認證時建立、而且沒有任何角色（ADR 2026-10-04-admin-roles）：GetSession 不需要權限，用它建立管理員，再以 CLI 授予 OWNER。
echo "等單體起來…"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${MONOLITH_PORT}/readyz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer ${ADMIN_TOKEN}" -d '{}' \
  "http://127.0.0.1:${MONOLITH_PORT}/hawkr.admin.identity.v1alpha1.IdentityAdminService/GetSession" >/dev/null
(clean_env DATABASE_URL="$MONOLITH_DB_URL" "${LOG_DIR}/hawkr" admin-role grant \
  -admin dev_characterize_admin@dev.localhost -role OWNER -reason "特性測試的管理員") >/dev/null

if [ "$legacy" = true ]; then
  legacy_start
fi

start_bg gateway \
  APP_ENV=local GIN_MODE=release INTERNAL_AUTH_MODE=off \
  PORT="$GATEWAY_PORT" DEBUG_PORT=19901 METRICS_PORT=19902 \
  JWT_SECRET="$JWT_SECRET" IM_CHAT_STREAM_JWT_SECRET="$IM_SECRET" MEDIA_BASE_URL=http://media.invalid \
  REDIS_HOST=127.0.0.1 REDIS_PORT=6379 REDIS_PASSWORD=HawkrRedis2026 \
  ACCOUNT_SERVICE_HOST=127.0.0.1 ACCOUNT_SERVICE_GRPC_PORT="$LEGACY_PORT" \
  SELLER_SERVICE_HOST=127.0.0.1 SELLER_SERVICE_GRPC_PORT="$LEGACY_PORT" \
  ${gateway_env[@]+"${gateway_env[@]}"} \
  "${LOG_DIR}/gateway"

echo "等 gateway 起來…"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${GATEWAY_PORT}/api/v1/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${GATEWAY_PORT}/api/v1/health" >/dev/null
curl -fsS "http://127.0.0.1:${MONOLITH_PORT}/readyz" >/dev/null

ENTRY_URL="http://127.0.0.1:${MONOLITH_PORT}" \
CHARACTERIZE_BUYER_TOKEN="$BUYER_TOKEN" CHARACTERIZE_ADMIN_TOKEN="$ADMIN_TOKEN" \
FAKE_MEDIA_ADDR="127.0.0.1:${FAKE_MEDIA_PORT}" \
GOWORK=off go test -count=1 -tags gateway -v "./${PKG_DIR}/"
