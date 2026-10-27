# Hawkr aftersales-service Dockerfile (layered)
# NOTE: Build from project root. 預編 jar 由 (cd services/java && ./mvnw install -pl :aftersales-service -am) 產出。
#   docker build -f services/java/aftersales-service/Dockerfile -t aftersales-service .
#
# Spring Boot layered jar 拆 4 層（dependencies / spring-boot-loader / snapshot-dependencies / application）
# 每層獨立 Docker layer hash：deps 不變時 GHCR 不重 push，每 PR 通常只推 ~15 MB application 層（fat jar 是每次 ~100 MB）
# 詳見 docs/基礎建設/CI_CD 自動化部署（K8s + Helm）.md §4.3。

# === Stage 1: 從 fat jar 拆出 layered 目錄 ===
FROM eclipse-temurin:21-jre-alpine@sha256:974b08960c5d96694c780e65b2d5705268ab1e1ca1a0dd0caf4ba6c3fe34d699 AS extractor
WORKDIR /extract
COPY services/java/aftersales-service/target/*.jar app.jar
RUN java -Djarmode=layertools -jar app.jar extract && rm app.jar

# === Stage 2: runtime ===
FROM eclipse-temurin:21-jre-alpine@sha256:974b08960c5d96694c780e65b2d5705268ab1e1ca1a0dd0caf4ba6c3fe34d699

LABEL maintainer="Hawkr Team"
LABEL service="aftersales-service"

RUN addgroup -S hawkr && adduser -S hawkr -G hawkr
WORKDIR /app

# 順序：deps（rarely change）→ loader → snapshot → application（changes every PR）
# 用 --chown 一次設好權限，避免 RUN chown 產生額外 ~100 MB layer
COPY --chown=hawkr:hawkr --from=extractor /extract/dependencies/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/spring-boot-loader/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/snapshot-dependencies/ ./
COPY --chown=hawkr:hawkr --from=extractor /extract/application/ ./
USER hawkr

ENV JAVA_OPTS="-Xms256m -Xmx512m -XX:+UseG1GC -XX:MaxGCPauseMillis=100"
ENV SERVER_PORT=9922
ENV GRPC_PORT=9923

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${SERVER_PORT}/actuator/health || exit 1

EXPOSE ${SERVER_PORT} ${GRPC_PORT}
ENTRYPOINT ["sh", "-c", "exec java ${JAVA_OPTS} org.springframework.boot.loader.launch.JarLauncher"]
