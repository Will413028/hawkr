.PHONY: build run dev test test-integration migrate-diff migrate-apply sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/im-service ./cmd/im-service/

run:
	go run ./cmd/im-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# testcontainers 重放 db/migrations 的 integration 測試（需要本機 Docker daemon）：
#   - internal/repository/postgres：CreateWithLock 的 advisory lock + transaction 行為
# 機制與 notification-service / ai-service procurement 的 dbtest_test.go 相同。
test-integration:
	go test -tags=integration ./... -count=1 -v

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
	cd ../.. && docker build -f services/im-service/Dockerfile --target prod -t hawkr/im-service .

docker-build-dev:
	cd ../.. && docker build -f services/im-service/Dockerfile --target dev -t hawkr/im-service:dev .

docker-run:
	docker run -p 9933:9933 hawkr/im-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build im-service

docker-compose-up:
	cd ../.. && docker compose up -d im-service

docker-compose-down:
	cd ../.. && docker compose stop im-service

docker-compose-logs:
	cd ../.. && docker compose logs -f im-service

docker-compose-restart:
	cd ../.. && docker compose restart im-service

# Docker Compose: Build and restart (useful after code changes)
docker-compose-rebuild:
	cd ../.. && docker compose up -d --build im-service

# Lint
lint:
	go vet ./...

# Clean
clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
