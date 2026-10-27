# shellcheck shell=bash
# scripts/characterize.sh social 會 source 這個檔，在起 gateway 之前呼叫 legacy_start 起舊的 social-service。
# 可以用的函式與變數見 scripts/characterize.sh 的說明。social-service 刪除時連同這個檔一起刪除。
#
# social-service 呼叫 account-service、buyer-service、seller-service 的 gRPC 時連單體的 legacy listener
# （127.0.0.1:${LEGACY_PORT}）；product-service 不起（情境不帶 product_id）；RabbitMQ 連不上時 social-service 自己停用事件
# 發布（internal/mq/di.go），所以不用起 MQ。賣家需要店家：套用 infra/seed-data/01-account.sql（帳號 9、4 是賣家，2001、2002
# 是買家）再跑 hawkr dev-seed，token 以 hawkr dev-token 簽 dev_account_<帳號 ID>。

legacy_start() {
  local social_port=19918
  require_free_port "$social_port"

  docker exec -i hawkr-postgres psql -v ON_ERROR_STOP=1 -q -U "$PG_USER" -d "$MONOLITH_DB" \
    <infra/seed-data/01-account.sql >"${LOG_DIR}/seed-account.log" 2>&1
  (clean_env DATABASE_URL="$MONOLITH_DB_URL" DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" \
    KYC_ENCRYPTION_KEYS="$KYC_ENCRYPTION_KEYS" KYC_HASH_KEYS="$KYC_HASH_KEYS" \
    MEDIA_SERVICE_ADDR="127.0.0.1:${FAKE_MEDIA_PORT}" MEDIA_ORIGIN=https://media.invalid PUBLIC_WEB_ORIGIN=https://web.invalid \
    GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}" LEGACY_GRPC_ADDR="127.0.0.1:${LEGACY_PORT}" \
    PRODUCT_SERVICE_ADDR=127.0.0.1:1 ORDER_SERVICE_ADDR=127.0.0.1:1 CRM_SERVICE_ADDR=127.0.0.1:1 REVIEW_SERVICE_ADDR=127.0.0.1:1 \
    CORS_ALLOWED_ORIGINS=http://localhost:3000 \
    "${LOG_DIR}/hawkr" dev-seed) >"${LOG_DIR}/seed-stores.log" 2>&1
  CHARACTERIZE_SELLER_TOKEN=$(dev_token -user dev_account_9)
  CHARACTERIZE_SELLER2_TOKEN=$(dev_token -user dev_account_4)
  CHARACTERIZE_BUYER2_TOKEN=$(dev_token -user dev_account_2002)
  export CHARACTERIZE_SELLER_TOKEN CHARACTERIZE_SELLER2_TOKEN CHARACTERIZE_BUYER2_TOKEN

  create_db social_characterization
  (cd services/social-service/db && atlas migrate apply --dir file://migrations \
    --url "$(db_url social_characterization)&search_path=public") >"${LOG_DIR}/legacy-migrate.log" 2>&1
  (cd services/social-service && go build -o "${LOG_DIR}/social-service" ./cmd/social-service)
  start_bg social-service ENV=development INTERNAL_AUTH_MODE=off \
    DB_HOST="$PGHOST" DB_PORT=5432 DB_USER="$PG_USER" DB_PASSWORD="$PG_PASSWORD" DB_NAME=social_characterization DB_SSLMODE=disable \
    GRPC_PORT="$social_port" MEDIA_BASE_URL=http://media.invalid \
    ACCOUNT_SERVICE_HOST=127.0.0.1 ACCOUNT_SERVICE_GRPC_PORT="$LEGACY_PORT" \
    BUYER_SERVICE_HOST=127.0.0.1 BUYER_SERVICE_GRPC_PORT="$LEGACY_PORT" \
    SELLER_SERVICE_HOST=127.0.0.1 SELLER_SERVICE_GRPC_PORT="$LEGACY_PORT" \
    MEDIA_SERVICE_HOST=127.0.0.1 MEDIA_SERVICE_GRPC_PORT="$FAKE_MEDIA_PORT" \
    PRODUCT_SERVICE_HOST=127.0.0.1 PRODUCT_SERVICE_GRPC_PORT=1 \
    RABBITMQ_URL=amqp://guest:guest@127.0.0.1:1/ \
    REDIS_HOST=127.0.0.1 REDIS_PORT=6379 REDIS_PASSWORD=HawkrRedis2026 \
    "${LOG_DIR}/social-service"
  for _ in $(seq 1 30); do
    if lsof -nP -iTCP:"$social_port" -sTCP:LISTEN >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  gateway_env+=(SOCIAL_SERVICE_HOST=127.0.0.1 SOCIAL_SERVICE_GRPC_PORT="$social_port")
}
