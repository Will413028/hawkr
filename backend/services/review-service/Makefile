.PHONY: build run dev test migrate-diff migrate-apply sqlc-generate docker-compose-build docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart

# Build
build:
	go build -o bin/review-service ./cmd/review-service/

run:
	go run ./cmd/review-service/

# Development (Air hot-reload)
dev:
	air -c .air.toml

# Test
test:
	go test ./... -v

test-cover:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

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
	cd ../.. && docker build -f services/review-service/Dockerfile --target prod -t hawkr/review-service .

docker-build-dev:
	cd ../.. && docker build -f services/review-service/Dockerfile --target dev -t hawkr/review-service:dev .

docker-run:
	docker run -p 10033:10033 hawkr/review-service

# Docker Compose: Service-specific commands (run from monorepo root)
docker-compose-build:
	cd ../.. && docker compose build review-service

docker-compose-up:
	cd ../.. && docker compose up -d review-service

docker-compose-down:
	cd ../.. && docker compose stop review-service

docker-compose-logs:
	cd ../.. && docker compose logs -f review-service

docker-compose-restart:
	cd ../.. && docker compose restart review-service

docker-compose-rebuild:
	cd ../.. && docker compose up -d --build review-service
