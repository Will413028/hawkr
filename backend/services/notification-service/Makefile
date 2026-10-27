.PHONY: build run test test-integration migrate-diff migrate-apply sqlc

build:
	go build -o bin/notification-service ./cmd/notification-service

run:
	go run ./cmd/notification-service

test:
	go test ./... -v

# testcontainers 重放 migration 的整包 integration 測試（需要本機 Docker daemon）：
#   - internal/worker：issue #1176 契約 guardrail，template ↔ publisher payload key 對照（13 號設計書 §14）
#   - internal/repository/postgres：system_notification_log 的 DB 行為
# 原本只跑 worker 的 TestTemplateContract，repository 那 2 支因此從未被任何 CI 執行過。
# 呼叫者：contract-tests.yml（notification db/worker 路徑）與
# integration-tests.yml 的 notification-service job（repository 路徑）。
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

# sqlc codegen (run from notification-service directory)
sqlc:
	../../scripts/gen-sqlc.sh $$(basename $$(pwd))

clean:
	rm -rf bin/ coverage.out coverage.html
