#!/usr/bin/env bash
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
REDIS_PASSWORD=${CHARACTERIZE_REDIS_PASSWORD-HawkrRedis2026}
MONOLITH_DB=characterize_monolith
MONOLITH_PORT=18080
LEGACY_PORT=19941
GATEWAY_PORT=19080
LOG_DIR=$(mktemp -d)
pids=()
dbs=()
gateway_env=()

psql_admin() {
  if [ -n "${CHARACTERIZE_PSQL_URL:-}" ]; then
    psql "$CHARACTERIZE_PSQL_URL" -q "$@"
  else
    docker exec hawkr-postgres psql -U "$PG_USER" -d postgres -q "$@"
  fi
}

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

clean_env() { exec env -i PATH="$PATH" HOME="$HOME" "$@"; }

start_bg() {
  local name=$1
  shift
  (clean_env "$@") >"${LOG_DIR}/${name}.log" 2>&1 &
  pids+=($!)
}

create_db() {
  psql_admin -c "DROP DATABASE IF EXISTS $1 WITH (FORCE)" -c "CREATE DATABASE $1" >/dev/null
  dbs+=("$1")
}

db_url() { echo "postgres://${PG_USER}:${PG_PASSWORD}@${PGHOST}:5432/$1?sslmode=disable"; }

require_free_port() {
  command -v lsof >/dev/null 2>&1 || return 0
  if lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $1 已經有程式在聽，先停掉它：lsof -nP -iTCP:$1 -sTCP:LISTEN" >&2
    exit 1
  fi
}

for port in "$MONOLITH_PORT" "$LEGACY_PORT" "$GATEWAY_PORT"; do
  require_free_port "$port"
done

legacy=false
if [ -f "${PKG_DIR}/legacy.sh" ]; then
  # shellcheck source=/dev/null
  source "${PKG_DIR}/legacy.sh"
  legacy=true
fi

(cd gateway && go build -o "${LOG_DIR}/gateway" ./cmd)
GOWORK=off go build -o "${LOG_DIR}/hawkr" ./cmd/hawkr

create_db "$MONOLITH_DB"
MONOLITH_DB_URL=$(db_url "$MONOLITH_DB")
(clean_env DATABASE_URL="$MONOLITH_DB_URL" "${LOG_DIR}/hawkr" migrate) >"${LOG_DIR}/monolith-migrate.log" 2>&1
DEV_AUTH_KEY_FILE="${LOG_DIR}/dev-auth.pem"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$DEV_AUTH_KEY_FILE" 2>/dev/null
KYC_HASH_KEYS="t1:$(openssl rand -base64 32)"
MEDIA_KEKS="k1:$(openssl rand -base64 32)"
start_bg monolith \
  DATABASE_URL="$MONOLITH_DB_URL" HTTP_ADDR="127.0.0.1:${MONOLITH_PORT}" LEGACY_GRPC_ADDR="127.0.0.1:${LEGACY_PORT}" \
  MEDIA_STORAGE=s3compat MEDIA_S3_ENDPOINT="${MEDIA_S3_ENDPOINT:?缺 MEDIA_S3_ENDPOINT}" MEDIA_S3_REGION="${MEDIA_S3_REGION:?缺 MEDIA_S3_REGION}" \
  MEDIA_S3_BUCKET="${MEDIA_S3_BUCKET:?缺 MEDIA_S3_BUCKET}" MEDIA_S3_ACCESS_KEY_ID="${MEDIA_S3_ACCESS_KEY_ID:?缺 MEDIA_S3_ACCESS_KEY_ID}" \
  MEDIA_S3_SECRET_ACCESS_KEY="${MEDIA_S3_SECRET_ACCESS_KEY:?缺 MEDIA_S3_SECRET_ACCESS_KEY}" MEDIA_KEKS="$MEDIA_KEKS" MEDIA_ORIGIN=https://media.invalid PUBLIC_WEB_ORIGIN=https://web.invalid \
  GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}" \
  PRODUCT_SERVICE_ADDR=127.0.0.1:1 ORDER_SERVICE_ADDR=127.0.0.1:1 INVOICE_SERVICE_ADDR=127.0.0.1:1 \
  CORS_ALLOWED_ORIGINS=http://localhost:3000 \
  DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" KYC_HASH_KEYS="$KYC_HASH_KEYS" \
  "${LOG_DIR}/hawkr" serve

dev_token() {
  (clean_env DATABASE_URL="$MONOLITH_DB_URL" DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" "${LOG_DIR}/hawkr" dev-token "$@")
}
BUYER_TOKEN=$(dev_token)
BUYER2_TOKEN=$(dev_token)
ADMIN_TOKEN=$(dev_token -admin -user dev_characterize_admin)

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
  MEDIA_BASE_URL=http://media.invalid \
  REDIS_HOST=127.0.0.1 REDIS_PORT=6379 REDIS_PASSWORD="$REDIS_PASSWORD" \
  ACCOUNT_SERVICE_HOST=127.0.0.1 ACCOUNT_SERVICE_GRPC_PORT="$LEGACY_PORT" \
  LEGACY_SELLER_GRPC_HOST=127.0.0.1 LEGACY_SELLER_GRPC_PORT="$LEGACY_PORT" \
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
CHARACTERIZE_BUYER_TOKEN="$BUYER_TOKEN" CHARACTERIZE_BUYER2_TOKEN="$BUYER2_TOKEN" CHARACTERIZE_ADMIN_TOKEN="$ADMIN_TOKEN" \
GOWORK=off go test -count=1 -tags gateway -v "./${PKG_DIR}/"
