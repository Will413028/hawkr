# shellcheck shell=bash

MAVEN_PL=coupon-service
JAVA_HTTP_PORT=19951
JAVA_GRPC_PORT=19952

legacy_start() {
  local jar
  local jars
  jars=$(find services/java/$MAVEN_PL/target -maxdepth 1 -name "$MAVEN_PL-*.jar" \
    ! -name "*-sources.jar" ! -name "*-javadoc.jar" ! -name "original-*" 2>/dev/null || true)
  if [ -z "$jars" ]; then
    echo "找不到 $MAVEN_PL 的 jar，先建：(cd services/java && ./mvnw package -pl :$MAVEN_PL -am -DskipTests)" >&2
    exit 1
  fi
  if [ "$(printf '%s\n' "$jars" | wc -l | tr -d ' ')" != 1 ]; then
    echo "$MAVEN_PL 的 jar 不只一個，清掉 target 重建：$jars" >&2
    exit 1
  fi
  jar=$jars
  require_free_port "$JAVA_HTTP_PORT"
  require_free_port "$JAVA_GRPC_PORT"

  docker exec -i hawkr-postgres psql -v ON_ERROR_STOP=1 -q -U "$PG_USER" -d "$MONOLITH_DB" \
    <infra/seed-data/01-account.sql >"${LOG_DIR}/seed-account.log" 2>&1
  (clean_env DATABASE_URL="$MONOLITH_DB_URL" DEV_AUTH_KEY_FILE="$DEV_AUTH_KEY_FILE" \
    KYC_HASH_KEYS="$KYC_HASH_KEYS" \
    MEDIA_STORAGE=s3compat MEDIA_S3_ENDPOINT="$MEDIA_S3_ENDPOINT" MEDIA_S3_REGION="$MEDIA_S3_REGION" MEDIA_S3_BUCKET="$MEDIA_S3_BUCKET" \
    MEDIA_S3_ACCESS_KEY_ID="$MEDIA_S3_ACCESS_KEY_ID" MEDIA_S3_SECRET_ACCESS_KEY="$MEDIA_S3_SECRET_ACCESS_KEY" MEDIA_KEKS="$MEDIA_KEKS" \
    MEDIA_ORIGIN=https://media.invalid PUBLIC_WEB_ORIGIN=https://web.invalid \
    GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}" LEGACY_GRPC_ADDR="127.0.0.1:${LEGACY_PORT}" \
    PRODUCT_SERVICE_ADDR=127.0.0.1:1 ORDER_SERVICE_ADDR=127.0.0.1:1 INVOICE_SERVICE_ADDR=127.0.0.1:1 \
    CORS_ALLOWED_ORIGINS=http://localhost:3000 \
    "${LOG_DIR}/hawkr" dev-seed) >"${LOG_DIR}/seed-stores.log" 2>&1
  BUYER_TOKEN=$(dev_token -account 2001)
  BUYER2_TOKEN=$(dev_token -account 2002)
  CHARACTERIZE_SELLER_TOKEN=$(dev_token -account 9)
  CHARACTERIZE_SELLER2_TOKEN=$(dev_token -account 4)
  export CHARACTERIZE_SELLER_TOKEN CHARACTERIZE_SELLER2_TOKEN

  create_db coupon_characterization
  start_bg coupon-service \
    TZ=UTC SERVER_PORT="$JAVA_HTTP_PORT" GRPC_PORT="$JAVA_GRPC_PORT" \
    DB_HOST="$PGHOST" DB_PORT=5432 DB_POOL_PORT=5432 DB_NAME=coupon_characterization DB_USER="$PG_USER" DB_PASSWORD="$PG_PASSWORD" DB_SSLMODE=disable \
    FLYWAY_ENABLED=true NACOS_ENABLED=false \
    REDIS_HOST=127.0.0.1 REDIS_PORT=6379 REDIS_PASSWORD=HawkrRedis2026 \
    RABBITMQ_HOST=127.0.0.1 RABBITMQ_PORT=5672 RABBITMQ_USERNAME=guest RABBITMQ_PASSWORD=guest \
    AUTO_RELEASE_ENABLED=false AUTO_EXPIRE_ENABLED=false \
    INTERNAL_AUTH_MODE=permissive SKIP_ACCOUNT_TYPE_CHECK=true \
    LEGACY_SELLER_GRPC_HOST=127.0.0.1 LEGACY_SELLER_GRPC_PORT="$LEGACY_PORT" \
    LEGACY_BUYER_GRPC_HOST=127.0.0.1 LEGACY_BUYER_GRPC_PORT="$LEGACY_PORT" \
    ACCOUNT_SERVICE_HOST=127.0.0.1 ACCOUNT_SERVICE_GRPC_PORT="$LEGACY_PORT" \
    ORDER_SERVICE_HOST=127.0.0.1 ORDER_SERVICE_GRPC_PORT=1 \
    OTEL_SDK_DISABLED=true \
    java -jar "$jar"

  echo "等 coupon-service 起來…"
  local up=false
  for _ in $(seq 1 120); do
    if lsof -nP -iTCP:"$JAVA_GRPC_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
      up=true
      break
    fi
    sleep 1
  done
  if [ "$up" != true ]; then
    echo "coupon-service 沒有在 120 秒內起來，log：${LOG_DIR}/coupon-service.log" >&2
    tail -n 40 "${LOG_DIR}/coupon-service.log" >&2 || true
    exit 1
  fi
  gateway_env+=(
    COUPON_SERVICE_HOST=127.0.0.1 COUPON_SERVICE_GRPC_PORT="$JAVA_GRPC_PORT"
    LEGACY_BUYER_GRPC_HOST=127.0.0.1 LEGACY_BUYER_GRPC_PORT="$LEGACY_PORT"
  )
}
