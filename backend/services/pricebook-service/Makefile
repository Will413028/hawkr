.PHONY: build run dev test test-cover test-integration migrate-diff migrate-status sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/pricebook-service ./cmd/pricebook-service/

run:
	go run ./cmd/pricebook-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 需要本機 Docker daemon（testcontainers 會起一個一次性 postgres:16-alpine 重放 db/migrations）。
# CI 由 .github/workflows/integration-tests.yml 的 pricebook-service job 呼叫。
test-integration:
	go test -tags=integration ./... -v

# Database: Atlas migrations
# 刻意不提供 migrate-apply：`--env dev` 指向共用的 dev DB，本機套用會讓 schema 領先於
# 程式碼部署、並讓套用順序與 PR 合併時序脫鉤。套用一律走 CI/CD；驗證用隔離容器。
migrate-diff:
	@read -p "Migration name: " name; \
	cd db && atlas migrate diff $$name --env dev

migrate-status:
	cd db && atlas migrate status --env dev

# Code Generation: sqlc
sqlc-generate:
	../../scripts/gen-sqlc.sh $$(basename $$(pwd))

# Proto (run from monorepo root)
proto:
	@echo "Run 'make proto' from backend-payuni root"

# Docker: Standalone build (run from monorepo root context)
docker-build:
	cd ../.. && docker build -f services/pricebook-service/Dockerfile --target prod -t hawkr/pricebook-service .

docker-build-dev:
	cd ../.. && docker build -f services/pricebook-service/Dockerfile --target dev -t hawkr/pricebook-service:dev .

docker-run:
	docker run -p 10039:10039 hawkr/pricebook-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build pricebook-service

docker-compose-up:
	cd ../.. && docker compose up -d pricebook-service

docker-compose-down:
	cd ../.. && docker compose stop pricebook-service

docker-compose-logs:
	cd ../.. && docker compose logs -f pricebook-service

docker-compose-restart:
	cd ../.. && docker compose restart pricebook-service

docker-compose-rebuild:
	cd ../.. && docker compose up -d --build pricebook-service
