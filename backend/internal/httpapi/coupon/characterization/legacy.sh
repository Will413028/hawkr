# shellcheck shell=bash
# maven-pl: coupon-service
#
# scripts/characterize.sh coupon 會 source 這個檔，在起 gateway 之前呼叫 legacy_start 起舊的 Java coupon-service。
# 可以用的函式與變數見 scripts/characterize.sh 的說明。coupon-service 刪除時連同這個檔一起刪除。
# 上面的 maven-pl 行是 CI（backend-characterization.yml）建 jar 用的 Maven 模組名。
#
# 先建好 jar（CI 與本機都一樣）：
#   (cd services/java && ./mvnw package -B -q -pl :coupon-service -am -DskipTests)
#
# coupon-service 呼叫 account、buyer、seller 的 gRPC 時連單體的 legacy listener（127.0.0.1:${LEGACY_PORT}）；order-service
# 不起（連不上時 MEMBER 型的買家資格降級為空，BD28）。Flyway 在啟動時套用 migration。RabbitMQ 與 Redis 用 infra 的那一組
# （docker-compose-infra.yml）。兩個排程（自動釋放、自動過期）關掉：情境不該被背景的狀態轉換影響。
#
# 身分：賣家與買家要有種子的店家與帳號，套用 infra/seed-data/01-account.sql（帳號 9、4 是賣家，店家 1009、1004；2001、2002
# 是買家）再跑 hawkr dev-seed，token 以 hawkr dev-token -account <帳號 ID> 簽。

JAVA_HTTP_PORT=19951
JAVA_GRPC_PORT=19952

legacy_start() {
  local jar
  local jars
  jars=$(find services/java/coupon-service/target -maxdepth 1 -name "coupon-service-*.jar" \
    ! -name "*-sources.jar" ! -name "*-javadoc.jar" ! -name "original-*" 2>/dev/null || true)
  if [ -z "$jars" ]; then
    echo "找不到 coupon-service 的 jar，先建：(cd services/java && ./mvnw package -pl :coupon-service -am -DskipTests)" >&2
    exit 1
  fi
  if [ "$(printf '%s\n' "$jars" | wc -l | tr -d ' ')" != 1 ]; then
    echo "coupon-service 的 jar 不只一個，清掉 target 重建：$jars" >&2
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
  CHARACTERIZE_BUYER2_TOKEN=$(dev_token -account 2002)
  CHARACTERIZE_SELLER_TOKEN=$(dev_token -account 9)
  CHARACTERIZE_SELLER2_TOKEN=$(dev_token -account 4)
  export CHARACTERIZE_BUYER2_TOKEN CHARACTERIZE_SELLER_TOKEN CHARACTERIZE_SELLER2_TOKEN

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
