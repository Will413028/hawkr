.PHONY: build test test-integration migrate-diff migrate-apply migrate-status sqlc-generate lint clean \
        docker-compose-build docker-compose-rebuild docker-compose-logs docker-compose-up docker-compose-down

build:
	go build -o bin/social-service ./cmd/social-service/

test:
	go test ./... -v

test-integration:
	go test -tags=integration ./... -v

migrate-diff:
	@read -p "Migration name: " name; \
	cd db && atlas migrate diff $$name --env dev

migrate-apply:
	cd db && atlas migrate apply --env dev

migrate-status:
	cd db && atlas migrate status --env dev

sqlc-generate:
	../../scripts/gen-sqlc.sh $$(basename $$(pwd))

docker-compose-build:
	cd ../.. && docker compose build social-service

docker-compose-rebuild:
	cd ../.. && docker compose up -d --build social-service

docker-compose-logs:
	cd ../.. && docker compose logs -f social-service

docker-compose-up:
	cd ../.. && docker compose up -d social-service

docker-compose-down:
	cd ../.. && docker compose stop social-service

lint:
	go vet ./...

clean:
	rm -rf bin/ tmp/ coverage.out
