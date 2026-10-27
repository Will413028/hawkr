.PHONY: build run test test-integration sqlc migrate-diff migrate-apply migrate-status

build:
	go build -o bin/ai-service ./cmd/ai-service

run:
	go run ./cmd/ai-service

# Test
test:
	go test ./... -v

test-integration:
	go test -tags=integration ./... -v

# Database: sqlc
sqlc:
	../../scripts/gen-sqlc.sh $$(basename $$(pwd))

# Database: Atlas migrations
migrate-diff:
	@read -p "Migration name: " name; \
	cd db && atlas migrate diff $$name --env dev

migrate-apply:
	cd db && atlas migrate apply --env dev

migrate-status:
	cd db && atlas migrate status --env dev
