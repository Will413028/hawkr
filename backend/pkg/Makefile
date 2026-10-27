.PHONY: test test-integration

# pkg 是共用套件模組，不是服務——沒有 build／run／migrate 目標，只有測試。
test:
	go test ./...

# 帶 tag 的測試需要真 Postgres（testcontainers）：pkg/database 的 savepoint 隔離
# 語意只有對真實交易才驗得出來——sub-transaction abort 之後外層還能不能 commit，
# 是 Postgres 的行為，mock 無法重現。
#
# 這個 target 的存在本身是 scripts/check-test-reachability.py 的要求：帶 build tag
# 的測試若沒有 CI 執行入口，它們不會被編譯、不會出現在任何報告裡，報告會永遠是綠的。
# 對應的 CI job 在 .github/workflows/integration-tests.yml。
test-integration:
	go test -tags=integration ./... -v
