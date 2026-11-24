# shellcheck shell=bash
# maven-pl: product-service
#
# scripts/characterize.sh catalog 會 source 這個檔，在起 gateway 之前呼叫 legacy_start 起舊的 Java product-service。
# 可以用的函式與變數見 scripts/characterize.sh 的說明。product-service 刪除時連同這個檔一起刪除。
# 上面的 maven-pl 行是 CI（backend-characterization.yml）建 jar 用的 Maven 模組名。
#
# 先建好 jar（CI 與本機都一樣）：
#   (cd services/java && ./mvnw package -B -q -pl :product-service -am -DskipTests)
#
# product-service 呼叫 seller、buyer 的 gRPC 時連單體的 legacy listener（127.0.0.1:${LEGACY_PORT}）；shipping 與 media 不起
# （情境不送配送方式與圖片，兩條呼叫都不會發生；公開商品頁取 shipping 設定失敗時降級，ProductService.java:1016）。
# Flyway 在啟動時套用 migration；V1 的 trigram 索引要 pg_trgm，建 database 後先建 extension。RabbitMQ 與 Redis 用 infra
# 的那一組（docker-compose-infra.yml）。兩個清理排程關掉。
#
# 店家可曝光：公開讀取以 store_exposure 判斷（沒有列＝不可曝光），平常由單體 sellerrelay 經 RabbitMQ 送
# seller.exposure.changed 餵入，但這支腳本不起 River worker。所以 product-service 起來後直接寫入兩家種子店可曝光的列，
# 情境不驗停權與不可曝光（B4-3 integration 覆蓋）。
#
# 分類：V1 的種子只有第一層，建立商品要第二層（「分類 ID 無對應的第二層分類」），所以同一步寫入三個第二層分類。
#
# 身分：CHARACTERIZE_SELLER_TOKEN、CHARACTERIZE_SELLER2_TOKEN 是種子帳號 9、4（店家 1009、1004）的 token；買家用腳本
# 新建的帳號（沒有賣家角色），管理員是腳本簽的。套用 infra/seed-data/01-account.sql 再跑 hawkr dev-seed 建店家。

JAVA_HTTP_PORT=19961
JAVA_GRPC_PORT=19962
PRODUCT_DB=product_characterization

legacy_start() {
  local jar
  local jars
  jars=$(find services/java/product-service/target -maxdepth 1 -name "product-service-*.jar" \
    ! -name "*-sources.jar" ! -name "*-javadoc.jar" ! -name "original-*" 2>/dev/null || true)
  if [ -z "$jars" ]; then
    echo "找不到 product-service 的 jar，先建：(cd services/java && ./mvnw package -pl :product-service -am -DskipTests)" >&2
    exit 1
  fi
  if [ "$(printf '%s\n' "$jars" | wc -l | tr -d ' ')" != 1 ]; then
    echo "product-service 的 jar 不只一個，清掉 target 重建：$jars" >&2
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
  CHARACTERIZE_SELLER_TOKEN=$(dev_token -account 9)
  CHARACTERIZE_SELLER2_TOKEN=$(dev_token -account 4)
  export CHARACTERIZE_SELLER_TOKEN CHARACTERIZE_SELLER2_TOKEN

  create_db "$PRODUCT_DB"
  docker exec hawkr-postgres psql -v ON_ERROR_STOP=1 -q -U "$PG_USER" -d "$PRODUCT_DB" -c "CREATE EXTENSION IF NOT EXISTS pg_trgm" >/dev/null
  start_bg product-service \
    TZ=UTC SERVER_PORT="$JAVA_HTTP_PORT" GRPC_PORT="$JAVA_GRPC_PORT" \
    DB_HOST="$PGHOST" DB_PORT=5432 DB_POOL_PORT=5432 DB_NAME="$PRODUCT_DB" DB_USER="$PG_USER" DB_PASSWORD="$PG_PASSWORD" DB_SSLMODE=disable \
    FLYWAY_ENABLED=true NACOS_ENABLED=false CLEANUP_IDEMPOTENCY_ENABLED=false \
    REDIS_HOST=127.0.0.1 REDIS_PORT=6379 REDIS_PASSWORD="$REDIS_PASSWORD" \
    RABBITMQ_HOST=127.0.0.1 RABBITMQ_PORT=5672 RABBITMQ_USERNAME=guest RABBITMQ_PASSWORD=guest \
    INTERNAL_AUTH_MODE=permissive \
    LEGACY_SELLER_GRPC_HOST=127.0.0.1 LEGACY_SELLER_GRPC_PORT="$LEGACY_PORT" \
    LEGACY_BUYER_GRPC_HOST=127.0.0.1 LEGACY_BUYER_GRPC_PORT="$LEGACY_PORT" \
    SHIPPING_SERVICE_HOST=127.0.0.1 SHIPPING_SERVICE_GRPC_PORT=1 \
    MEDIA_SERVICE_HOST=127.0.0.1 MEDIA_SERVICE_GRPC_PORT=1 MEDIA_BASE_URL=http://media.invalid \
    OTEL_SDK_DISABLED=true \
    java -jar "$jar"

  echo "等 product-service 起來…"
  local up=false
  for _ in $(seq 1 120); do
    if lsof -nP -iTCP:"$JAVA_GRPC_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
      up=true
      break
    fi
    sleep 1
  done
  if [ "$up" != true ]; then
    echo "product-service 沒有在 120 秒內起來，log：${LOG_DIR}/product-service.log" >&2
    tail -n 40 "${LOG_DIR}/product-service.log" >&2 || true
    exit 1
  fi
  docker exec -i hawkr-postgres psql -v ON_ERROR_STOP=1 -q -U "$PG_USER" -d "$PRODUCT_DB" >/dev/null <<SQL
INSERT INTO store_exposure (store_id, exposable, version, updated_at)
VALUES (1009, true, 1, now()), (1004, true, 1, now())
ON CONFLICT (store_id) DO UPDATE SET exposable = true;
INSERT INTO category (name, parent_id, status)
SELECT child, p.id, '9004'
FROM category p
JOIN (VALUES ('葉菜', '蔬菜'), ('根莖', '蔬菜'), ('柑橘', '水果')) AS c(child, parent) ON c.parent = p.name AND p.parent_id IS NULL
ON CONFLICT DO NOTHING;
SQL
  gateway_env+=(
    PRODUCT_SERVICE_HOST=127.0.0.1 PRODUCT_SERVICE_GRPC_PORT="$JAVA_GRPC_PORT" PRODUCT_SERVICE_PORT="$JAVA_HTTP_PORT"
    LEGACY_BUYER_GRPC_HOST=127.0.0.1 LEGACY_BUYER_GRPC_PORT="$LEGACY_PORT"
  )
}
