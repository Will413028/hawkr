# shellcheck shell=bash
# scripts/characterize.sh media 會 source 這個檔，在起 gateway 之前呼叫 legacy_start 起舊的 Java media-service。
# 可以用的函式與變數見 scripts/characterize.sh 的說明。media-service 刪除時連同這個檔一起刪除。
#
# 先建好 jar（CI 的 backend-characterization.yml 與本機都一樣）：
#   (cd services/java && ./mvnw package -B -q -pl :media-service -am -DskipTests)
#
# 不接物件儲存（B1-D12）：不設任何 R2_* 與 ENCRYPTION_MASTER_KEY，Java 在 R2 設定不完整時照常啟動
# （R2StorageService.java:95-107 只警告），建立會話回模擬網址、完成時不回查物件。TOKEN_SECRET 是只給這次用的值。
# Java 不需要 Redis、RabbitMQ、Nacos（NACOS_ENABLED=false）。

JAVA_HTTP_PORT=19939
JAVA_GRPC_PORT=19938

legacy_start() {
  local jar
  jar=$(find services/java/media-service/target -maxdepth 1 -name "media-service-*.jar" 2>/dev/null | head -n 1 || true)
  if [ -z "$jar" ]; then
    echo "找不到 media-service 的 jar，先建：(cd services/java && ./mvnw package -pl :media-service -am -DskipTests)" >&2
    exit 1
  fi
  require_free_port "$JAVA_HTTP_PORT"
  require_free_port "$JAVA_GRPC_PORT"
  create_db media_characterization

  start_bg media-service \
    TZ=UTC SERVER_PORT="$JAVA_HTTP_PORT" GRPC_PORT="$JAVA_GRPC_PORT" \
    DB_HOST="$PGHOST" DB_PORT=5432 DB_NAME=media_characterization DB_USER="$PG_USER" DB_PASSWORD="$PG_PASSWORD" DB_SSLMODE=disable \
    FLYWAY_ENABLED=true NACOS_ENABLED=false GC_ENABLED=false \
    TOKEN_SECRET="$(openssl rand -hex 32)" TOKEN_ALLOW_EPHEMERAL=false \
    INTERNAL_AUTH_MODE=permissive SKIP_ACCOUNT_TYPE_CHECK=true \
    MEDIA_BASE_URL=https://media.invalid \
    SELLER_SERVICE_HOST=127.0.0.1 SELLER_SERVICE_GRPC_PORT="$LEGACY_PORT" \
    OTEL_SDK_DISABLED=true \
    java -jar "$jar"

  echo "等 media-service 起來…"
  local up=false
  for _ in $(seq 1 90); do
    if curl -fsS "http://127.0.0.1:${JAVA_HTTP_PORT}/actuator/health" >/dev/null 2>&1; then
      up=true
      break
    fi
    sleep 1
  done
  if [ "$up" != true ]; then
    echo "media-service 沒有在 90 秒內起來，log：${LOG_DIR}/media-service.log" >&2
    tail -n 40 "${LOG_DIR}/media-service.log" >&2 || true
    exit 1
  fi
  gateway_env+=(MEDIA_SERVICE_HOST=127.0.0.1 MEDIA_SERVICE_PORT="$JAVA_HTTP_PORT" MEDIA_SERVICE_GRPC_PORT="$JAVA_GRPC_PORT")
}
