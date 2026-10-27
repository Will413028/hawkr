.PHONY: build run dev test test-cover test-integration migrate-diff migrate-apply sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/crm-service ./cmd/crm-service/

run:
	go run ./cmd/crm-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 需要本機 Docker daemon（testcontainers 起一個 postgres:16-alpine 重放 db/migrations）。
# CI 由 .github/workflows/integration-tests.yml 的 crm-service job 呼叫。
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
	cd ../.. && docker build -f services/crm-service/Dockerfile --target prod -t hawkr/crm-service .

docker-build-dev:
	cd ../.. && docker build -f services/crm-service/Dockerfile --target dev -t hawkr/crm-service:dev .

docker-run:
	docker run -p 10030:10030 hawkr/crm-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build crm-service

docker-compose-up:
	cd ../.. && docker compose up -d crm-service

docker-compose-down:
	cd ../.. && docker compose stop crm-service

docker-compose-logs:
	cd ../.. && docker compose logs -f crm-service

docker-compose-restart:
	cd ../.. && docker compose restart crm-service

# Docker Compose: Build and restart (useful after code changes)
docker-compose-rebuild:
	cd ../.. && docker compose up -d --build crm-service

# Lint
lint:
	go vet ./...

# Clean
clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
