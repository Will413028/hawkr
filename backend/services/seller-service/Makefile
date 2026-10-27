.PHONY: build run dev test test-cover test-integration migrate-diff migrate-apply sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/seller-service ./cmd/seller-service/

run:
	go run ./cmd/seller-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

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
	cd ../.. && docker build -f services/seller-service/Dockerfile --target prod -t hawkr/seller-service .

docker-build-dev:
	cd ../.. && docker build -f services/seller-service/Dockerfile --target dev -t hawkr/seller-service:dev .

docker-run:
	docker run -p 10020:10020 hawkr/seller-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build seller-service

docker-compose-up:
	cd ../.. && docker compose up -d seller-service

docker-compose-down:
	cd ../.. && docker compose stop seller-service

docker-compose-logs:
	cd ../.. && docker compose logs -f seller-service

docker-compose-restart:
	cd ../.. && docker compose restart seller-service

# Docker Compose: Build and restart (useful after code changes)
docker-compose-rebuild:
	cd ../.. && docker compose up -d --build seller-service

# Lint
lint:
	go vet ./...

# Clean
clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
