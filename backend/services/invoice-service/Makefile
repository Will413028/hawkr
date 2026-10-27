.PHONY: build run dev test test-cover test-integration migrate-diff migrate-apply sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/invoice-service ./cmd/invoice-service/

run:
	go run ./cmd/invoice-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 帶 //go:build integration 的測試（真 PostgreSQL / testcontainers）。
# CI 由 .github/workflows/integration-tests.yml 的 invoice-service job 呼叫。
test-integration:
	go test -tags=integration ./... -v

# Database: Atlas migrations
migrate-diff:
	@read -p "Migration name: " name; \
	cd db && atlas migrate diff $$name --env dev

migrate-apply:
	cd db && atlas migrate apply --env dev

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
	cd ../.. && docker build -f services/invoice-service/Dockerfile --target prod -t hawkr/invoice-service .

docker-build-dev:
	cd ../.. && docker build -f services/invoice-service/Dockerfile --target dev -t hawkr/invoice-service:dev .

docker-run:
	docker run -p 10040:10040 -p 10041:10041 -p 10042:10042 hawkr/invoice-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build invoice-service

docker-compose-up:
	cd ../.. && docker compose up -d invoice-service

docker-compose-down:
	cd ../.. && docker compose stop invoice-service

docker-compose-logs:
	cd ../.. && docker compose logs -f invoice-service

docker-compose-restart:
	cd ../.. && docker compose restart invoice-service

docker-compose-rebuild:
	cd ../.. && docker compose up -d --build invoice-service
