.PHONY: build-all test-all vet-all lint-all lint-monolith lint-staged vet-tags-staged lint-fix-all deadcode proto-gen proto-lint proto-lint-numeric \
	db-init db-init-local atlas-validate-all atlas-validate-staged \
	infra-up infra-down migrate serve worker dev-key dev-token dev-seed dev-seed-check dev-seed-stores reannounce-exposures sqlc jet connect uitext new-module new-characterization

GO_SERVICES := \
	gateway \
	services/ai-service \
	services/crm-service \
	services/im-service \
	services/invoice-service \
	services/notification-service \
	services/review-service \
	services/seller-service \
	services/social-service

# Go services with their own Atlas-managed DB (all GO_SERVICES except gateway, which has none)
ATLAS_SERVICES := $(filter-out gateway,$(GO_SERVICES))

# Build all Go services
build-all:
	@for svc in $(GO_SERVICES); do \
		echo "=== Building $$svc ==="; \
		(cd $$svc && go build ./...) || exit 1; \
	done
	@echo "=== All services built successfully ==="

# Test all Go services
test-all:
	@for svc in $(GO_SERVICES); do \
		echo "=== Testing $$svc ==="; \
		(cd $$svc && go test ./...) || exit 1; \
	done
	@echo "=== All tests passed ==="

# Vet all Go services
vet-all:
	@for svc in $(GO_SERVICES); do \
		echo "=== Vetting $$svc ==="; \
		(cd $$svc && go vet ./...) || exit 1; \
	done
	@echo "=== All services vetted ==="

# Lint all Go services (requires golangci-lint: brew install golangci-lint)
lint-all:
	@for svc in $(GO_SERVICES); do \
		echo "=== Linting $$svc ==="; \
		(cd $$svc && golangci-lint run --allow-parallel-runners ./...) || exit 1; \
	done
	@echo "=== All services linted ==="

GOLANGCI_LINT_VERSION := $(shell cat .golangci-version)
GOLANGCI_LINT_MONOLITH := GOWORK=off GOTOOLCHAIN=go$(shell cat .go-version) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint-monolith:
	$(GOLANGCI_LINT_MONOLITH) run -c .golangci.monolith.yml ./cmd/... ./internal/...
	$(GOLANGCI_LINT_MONOLITH) run -c .golangci.monolith.yml --build-tags=integration ./cmd/... ./internal/...

# Lint only Go service modules that contain staged .go files (fast pre-commit check)
#
# gocritic is disabled here for every module except gateway itself: its only
# enabled check is the ruleguard rule in gateway/ruleguard/rules.go (banning
# direct gin JSON emitters), which imports github.com/quasilyte/go-ruleguard/dsl
# — a dependency declared only in gateway/go.mod. With GOWORK=off (added
# 2026-08-03, a632e2982, to isolate module discovery for scripts/), every other
# module's typechecker can't resolve that import and golangci-lint's whole run
# errors out (exit 3) regardless of the module's actual code — not a false
# positive, a hard crash. The rule itself is pkgpath-scoped to
# gateway/internal/(handler|middleware|router), so it can never match outside
# gateway anyway; disabling gocritic for non-gateway modules here loses no
# real coverage while keeping gateway's own commits gated on it.
# vet-tags-staged：對「有 staged .go 的模組」跑 go vet -tags integration。
#
# 🔴 為什麼需要獨立一道：`go build` 與 `go test` **看不見** //go:build integration
# 的檔案，所以改了介面簽章時，本地三件（gofmt／build／test）可以全綠而 CI 紅在
# 「帶 tag 的測試檔要編譯得過」——那些是唯一會呼叫真實 repository 方法的地方。
# 2026-09-04 實際踩到（改 NewCorrection 簽章漏了 void_reissue_integration_test.go），
# 而 §17.4.1 的第四件早就寫著這條，規則存在但沒有機制 ⇒ 照樣忘。
#
# 增量策略與 lint-staged 相同：只跑包含 staged .go 的模組。
vet-tags-staged:
	@STAGED_MODS=$$(git diff --name-only --cached --diff-filter=ACMR "*.go" | grep -v "^pkg/proto/" | while read -r file; do \
		dir="$$(dirname "$$file")"; \
		while [ "$$dir" != "." ] && [ "$$dir" != "/" ]; do \
			if [ -f "$$dir/go.mod" ]; then echo "$$dir"; break; fi; \
			dir="$$(dirname "$$dir")"; \
		done; \
	done | sort -u); \
	if [ -z "$$STAGED_MODS" ]; then \
		echo "=== No staged Go modules to vet ==="; \
	else \
		FAILED=0; \
		for mod in $$STAGED_MODS; do \
			echo "=== $$mod (vet -tags integration) ==="; \
			VET_OUT=$$(cd $$mod && GOWORK=off go vet -tags integration ./... 2>&1); \
			VET_CODE=$$?; \
			echo "$$VET_OUT"; \
			if [ $$VET_CODE -ne 0 ]; then \
				IS_GO_SERVICE=false; \
				for svc in $(GO_SERVICES); do \
					if [ "$$svc" = "$$mod" ]; then IS_GO_SERVICE=true; fi; \
				done; \
				if [ "$$IS_GO_SERVICE" = false ] && echo "$$VET_OUT" | grep -q "no packages to vet"; then \
					echo "(module $$mod is not a GO_SERVICES member — e.g. scripts/ one-off tools tagged //go:build ignore — skipping)"; \
				else \
					FAILED=1; \
				fi; \
			fi; \
		done; \
		if [ $$FAILED -ne 0 ]; then \
			echo ""; \
			echo "🔴 帶 build tag 的測試檔編譯不過。go build／go test 看不見它們，"; \
			echo "   所以本地其他檢查會全綠而 CI 紅。多半是改了介面簽章卻漏了 integration 測試的呼叫點。"; \
			exit 1; \
		fi; \
		echo "=== All staged Go modules pass vet -tags integration ==="; \
	fi

lint-staged:
	@STAGED_MODS=$$(git diff --name-only --cached --diff-filter=ACMR "*.go" | grep -v "^pkg/proto/" | while read -r file; do \
		dir="$$(dirname "$$file")"; \
		while [ "$$dir" != "." ] && [ "$$dir" != "/" ]; do \
			if [ -f "$$dir/go.mod" ]; then \
				echo "$$dir"; \
				break; \
			fi; \
			dir="$$(dirname "$$dir")"; \
		done; \
	done | sort -u); \
	if [ -z "$$STAGED_MODS" ]; then \
		echo "=== No staged Go modules to lint ==="; \
	else \
		for mod in $$STAGED_MODS; do \
			echo "=== Linting staged module $$mod ==="; \
			GOCRITIC_FLAG=""; \
			if [ "$$mod" != "gateway" ]; then GOCRITIC_FLAG="--disable gocritic"; fi; \
			LINT_OUT=$$(cd $$mod && GOWORK=off golangci-lint run --allow-parallel-runners $$GOCRITIC_FLAG ./... 2>&1); \
			LINT_CODE=$$?; \
			echo "$$LINT_OUT"; \
			if [ $$LINT_CODE -ne 0 ]; then \
				IS_GO_SERVICE=false; \
				for svc in $(GO_SERVICES); do \
					if [ "$$svc" = "$$mod" ]; then IS_GO_SERVICE=true; fi; \
				done; \
				if [ "$$IS_GO_SERVICE" = false ] && echo "$$LINT_OUT" | grep -q "no go files to analyze"; then \
					echo "(module $$mod is not a GO_SERVICES member — e.g. scripts/ one-off tools tagged //go:build ignore — skipping)"; \
				else \
					exit 1; \
				fi; \
			fi; \
		done; \
		echo "=== All staged Go modules linted successfully ==="; \
	fi

# Auto-fix lint issues in all Go services
lint-fix-all:
	@for svc in $(GO_SERVICES); do \
		echo "=== Fixing $$svc ==="; \
		(cd $$svc && golangci-lint run --allow-parallel-runners --fix ./...) || true; \
	done
	@echo "=== All services fixed ==="

# Validate Atlas migration directory integrity for every Go service (atlas.sum
# checksums match the actual migration files). Local-only, no DB connection needed.
# Catches: a migration .sql file added/edited without re-running `atlas migrate hash`
# to update atlas.sum — this passes `go build`/`go test` silently but fails
# `atlas migrate apply`'s directory integrity check at deploy time (see
# deploy-k8s.yml's migrate job). Added after this exact gap escaped code review once
# (notification-service, 2026-07-18) and was only caught by a whole-branch review, not CI.
atlas-validate-all:
	@for svc in $(ATLAS_SERVICES); do \
		if [ -f "$$svc/db/atlas.hcl" ]; then \
			echo "=== Validating $$svc migrations ==="; \
			(cd $$svc/db && atlas migrate validate --dir "file://migrations") || exit 1; \
		fi; \
	done
	@echo "=== All Atlas migration directories valid ==="

# Validate Atlas migration directory integrity ONLY for services with staged migration/schema files
atlas-validate-staged:
	@STAGED_SVCS=$$(git diff --name-only --cached --diff-filter=ACMR | grep -E '^services/[^/]+/db/' | cut -d/ -f1-2 | sort -u); \
	if [ -z "$$STAGED_SVCS" ]; then \
		echo "=== No staged DB migrations to validate ==="; \
	else \
		for svc in $$STAGED_SVCS; do \
			if [ -f "$$svc/db/atlas.hcl" ]; then \
				echo "=== Validating $$svc migrations ==="; \
				(cd $$svc/db && atlas migrate validate --dir "file://migrations") || exit 1; \
			fi; \
		done; \
		echo "=== All staged Atlas migration directories valid ==="; \
	fi

# Report unreachable functions across all Go services (call-graph dead code).
# golangci-lint 的 unused 只做 package 層級；deadcode 做跨 package call-graph 分析。
# Report-only：永遠 exit 0。已排除 generated code（*.pb.go、docs/docs.go）。
# 注意：經 gRPC handler / RabbitMQ consumer / DI 間接呼叫的 func 可能誤報為 dead，刪前請人工複核。
deadcode:
	@go install golang.org/x/tools/cmd/deadcode@latest
	@DC="$$(go env GOPATH)/bin/deadcode"; \
	for svc in $(GO_SERVICES); do \
		echo "=== deadcode $$svc ==="; \
		(cd $$svc && $$DC -test ./... 2>/dev/null | grep -vE '\.pb\.go|docs/docs\.go') || true; \
	done; \
	echo "=== deadcode scan done (report-only) ==="

# Generate protobuf code using Docker (pinned protoc)
proto-gen:
	./scripts/gen-go-proto.sh

# Lint protobuf files
proto-lint:
	buf lint api/proto

# Enforce Proto 管理規範 §6: no int64/int32/double on response-exposed fields
# (grandfathered exceptions in scripts/proto/numeric-allowlist.txt). See pkg/cmd/protonumericlint.
# Standalone from `proto-lint` (buf STANDARD lint has pre-existing violations, out of scope).
proto-lint-numeric:
	buf build api/proto -o - | go run github.com/Will413028/hawkr/go-pkg/cmd/protonumericlint -allowlist scripts/proto/numeric-allowlist.txt

# ==============================================================================
# Database Migration Commands
# ==============================================================================

# 初始化所有資料庫 (local)
db-init-local:
	@echo "=== 初始化本地資料庫 ==="
	./scripts/init-databases.sh local

# 預設使用 local
db-init: db-init-local

# ==============================================================================
# 單體（cmd/hawkr）的本機環境
# ==============================================================================
# 單體的 config 對必填設定不給預設值（internal/platform/config），本機要用的值集中在
# 這裡，全部指向 docker-compose-infra.yml 起的服務；新增必填設定時，本機的值也加在這裡。
# 要改就在命令列覆寫，例如 make migrate DATABASE_URL=…。環境變數蓋不掉這裡的值
# （Makefile 的賦值優先），別的專案 export 的 DATABASE_URL 不會被帶進來。
INFRA_COMPOSE := docker compose -f docker-compose-infra.yml
DATABASE_URL = postgres://dbadmin:Hawkr2024Admin@localhost:5432/hawkr?sslmode=disable
# 遷移計畫 B1 之前，media 還是 Java 的 media-service（./dev.sh 的 compose 起在 9937）
MEDIA_SERVICE_ADDR = localhost:9937
# 對外圖片的網域：本機的 image gateway（workers/image-gateway 的 wrangler dev，預設聽 8787）
MEDIA_ORIGIN = http://localhost:8787
# 網站的網域：API 回給 client 的網站網址（入駐文件）由它組成絕對網址；本機是 website 的 dev server
PUBLIC_WEB_ORIGIN = http://localhost:3000
# 單體是對外的入口（遷移計畫 A2）：還沒搬的 REST、SSE、WebSocket 轉給 ./dev.sh 起的 gateway（9900）。
GATEWAY_URL = http://localhost:9900
PRODUCT_SERVICE_ADDR = localhost:9913
ORDER_SERVICE_ADDR = localhost:9915
CRM_SERVICE_ADDR = localhost:10030
# 推薦排行的評分統計（discovery 的 worker）：./dev.sh 的 review-service（10033），A6 搬進單體後刪除
REVIEW_SERVICE_ADDR = localhost:10033
# 單體替舊服務提供 account-service 的 gRPC（./dev.sh 的舊服務以 host.docker.internal:9941 呼叫）；worker 把事件送到
# 舊服務訂閱的 RabbitMQ
LEGACY_GRPC_ADDR = :9941
RABBITMQ_URL = amqp://guest:guest@localhost:5672/
# 瀏覽器直接呼叫單體的 origin（website 的 next dev）。admin 的瀏覽器只打自己的 BFF，由 BFF 以伺服器身分呼叫，不在這裡
CORS_ALLOWED_ORIGINS = http://localhost:3000
# 登入由 Clerk 處理（ADR 2026-09-27-identity-clerk）：development instance 的 key 在 backend/.env，由
# scripts/clerk-env.sh 帶上（不經過 make 的指令列）；web 取得的 token 帶 azp，要是這裡的 origin。
CLERK_AUTHORIZED_PARTIES ?= $(CORS_ALLOWED_ORIGINS)
# KYC 個資的金鑰（身分證號、銀行帳號的加密，與身分比對用的 HMAC；只給本機開發用，兩者必須不同，格式 id:base64，詳見
# cmd/hawkr/seller.go 的 readKYCKeys）。正式環境放部署設定，不進版控。
KYC_ENCRYPTION_KEYS ?= dev1:ZGV2LW9ubHkta3ljLWVuY3J5cHRpb24ta2V5LTAwMDE=
KYC_HASH_KEYS ?= dev1:ZGV2LW9ubHkta3ljLWhhc2gtaG1hYy1rZXktMDAwMDE=
# PayUni 代理商 API（子商店的綁定、同步與狀態查詢）：本機不設定，所以核准後的綁定會記為 PAYUNI_NOT_CONFIGURED（沒有 sandbox 金鑰，
# 也沒有預設網址）。要連 sandbox 時在指令列給，例如 make worker PAYUNI_BASE_URL=… PAYUNI_AGENT_ID=… PAYUNI_HASH_KEY=…
# PAYUNI_HASH_IV=…（四個一起設定，詳見 cmd/hawkr/seller.go 的 readPayUni）。
PAYUNI_BASE_URL ?=
PAYUNI_AGENT_ID ?=
PAYUNI_HASH_KEY ?=
PAYUNI_HASH_IV ?=
PAYUNI_STOREFRONT_BASE_URL ?=
# 本機開發的 dev token 的金鑰（make dev-key 產生，不進版控）。與 Access 正式設定互斥。
DEV_AUTH_KEY_FILE ?= $(CURDIR)/.dev/dev-auth.pem
MONOLITH_ENV = GOWORK=off DATABASE_URL='$(DATABASE_URL)' MEDIA_SERVICE_ADDR='$(MEDIA_SERVICE_ADDR)' MEDIA_ORIGIN='$(MEDIA_ORIGIN)' PUBLIC_WEB_ORIGIN='$(PUBLIC_WEB_ORIGIN)' \
	GATEWAY_URL='$(GATEWAY_URL)' \
	CORS_ALLOWED_ORIGINS='$(CORS_ALLOWED_ORIGINS)' CLERK_AUTHORIZED_PARTIES='$(CLERK_AUTHORIZED_PARTIES)' \
	DEV_AUTH_KEY_FILE='$(DEV_AUTH_KEY_FILE)' LEGACY_GRPC_ADDR='$(LEGACY_GRPC_ADDR)' \
	KYC_ENCRYPTION_KEYS='$(KYC_ENCRYPTION_KEYS)' KYC_HASH_KEYS='$(KYC_HASH_KEYS)' \
	PAYUNI_BASE_URL='$(PAYUNI_BASE_URL)' PAYUNI_AGENT_ID='$(PAYUNI_AGENT_ID)' PAYUNI_HASH_KEY='$(PAYUNI_HASH_KEY)' \
	PAYUNI_HASH_IV='$(PAYUNI_HASH_IV)' PAYUNI_STOREFRONT_BASE_URL='$(PAYUNI_STOREFRONT_BASE_URL)' \
	RABBITMQ_URL='$(RABBITMQ_URL)' \
	PRODUCT_SERVICE_ADDR='$(PRODUCT_SERVICE_ADDR)' ORDER_SERVICE_ADDR='$(ORDER_SERVICE_ADDR)' CRM_SERVICE_ADDR='$(CRM_SERVICE_ADDR)' \
	REVIEW_SERVICE_ADDR='$(REVIEW_SERVICE_ADDR)' \
	scripts/clerk-env.sh

# 起 Postgres、Redis、RabbitMQ、OTel collector，等到 healthcheck 通過
infra-up:
	$(INFRA_COMPOSE) up -d --wait

infra-down:
	$(INFRA_COMPOSE) down

migrate:
	$(MONOLITH_ENV) go run ./cmd/hawkr migrate

# HTTP 預設聽 :8080（HTTP_ADDR），/healthz、/readyz
serve: dev-key
	$(MONOLITH_ENV) go run ./cmd/hawkr serve

# 執行非同步 job（River）；revalidate 沒設定 WEB_REVALIDATE_* 時不送出
worker: dev-key
	$(MONOLITH_ENV) go run ./cmd/hawkr worker

# 本機開發的 dev token 的 RSA 私鑰：沒有時產生一把，只在這台機器上用
dev-key:
	@[ -z '$(DEV_AUTH_KEY_FILE)' ] || [ -f '$(DEV_AUTH_KEY_FILE)' ] || { mkdir -p '$(dir $(DEV_AUTH_KEY_FILE))' && \
		openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out '$(DEV_AUTH_KEY_FILE)' 2>/dev/null && \
		chmod 600 '$(DEV_AUTH_KEY_FILE)' && echo "產生 $(DEV_AUTH_KEY_FILE)"; }

# 本機的登入：以 dev 金鑰簽一個 token 印出（24 小時，不連 Clerk），第一個請求時建立帳號或管理員。
# 例：make dev-token args='-user dev_account_2001'（seed 的帳號）、make dev-token args='-admin -user dev_ops'
dev-token: dev-key
	@DEV_AUTH_KEY_FILE='$(DEV_AUTH_KEY_FILE)' GOWORK=off go run ./cmd/hawkr dev-token $(args)

# 本機的測試資料重置（唯一支援的路徑）：infra/seed-test-data.sh 先 dev-seed-check，再依序跑 01 帳號與買家 → dev-seed-stores → 04–06 → 全量重新公告。
# 單獨寫 seller 的店家會讓 identity 與 Java 的 store_exposure 不一致，所以不提供單獨的目標給人用。要先 make migrate，
# 只在有 DEV_AUTH_KEY_FILE 的本機開發環境執行；要連別的單體資料庫時帶 DATABASE_URL=…（傳給腳本的 MONOLITH_DATABASE_URL）。
dev-seed: dev-key
	MONOLITH_DATABASE_URL='$(DATABASE_URL)' bash infra/seed-test-data.sh

# 前提檢查（只給 infra/seed-test-data.sh 呼叫，在 01 之前）：01 重建的帳號區間若擁有種子以外的店家就失敗，不寫任何資料。
dev-seed-check: dev-key
	$(MONOLITH_ENV) go run ./cmd/hawkr dev-seed -check

# 重置的一個步驟（只給 infra/seed-test-data.sh 呼叫）：seller 模組把種子店家還原成 fixture（upsert，不碰種子以外的店家），明文 fixture 在
# cmd/hawkr/dev_seed.go，以執行中的 KYC 金鑰加密後寫入。
dev-seed-stores: dev-key
	$(MONOLITH_ENV) go run ./cmd/hawkr dev-seed

# seller 全量重新公告可曝光（不比對差異，版本照常加一）：種子重寫之後，以及 Java 的 store_exposure 重建或死信處理之後。事件由 make worker 送出。
reannounce-exposures:
	$(MONOLITH_ENV) go run ./cmd/hawkr reannounce-exposures

# 改完模組的 db/schema.sql、db/objects.sql 或 db/queries/ 後，重新生成 internal/modules/<名稱>/internal/store
# （每個模組的 sqlc.yaml 都輸出到這裡；先清空，刪掉的 query 才不會留下孤兒檔）。
# sqlc 的版本與舊服務相同，只宣告在 scripts/gen-sqlc.sh 的 IMAGE。
# 產生 migration 另外在模組的 db/ 跑 atlas migrate diff（見該目錄的 atlas.hcl）。
# 還沒有任何 query 的模組（剛由 make new-module 產生）略過：sqlc 在沒有 query 時直接失敗。
SQLC_IMAGE = $(shell sed -n 's/^IMAGE="\(.*\)"$$/\1/p' scripts/gen-sqlc.sh)
sqlc:
	@bash scripts/gen-monolith-sqlc.sh "$(SQLC_IMAGE)"

# 用 go-jet 建 query 的模組（目前 social，ADR 2026-09-27-module-internal-structure 的修訂）：重新生成
#   - internal/jet：模組自己的表型別（起暫時的 Postgres container、只套用該模組的 migration，需要 Docker；本機帶 DOCKER_CONTEXT 與 DOCKER_HOST，見 AGENTS.local.md）；
#   - internal/pubsql：依賴表允許的模組發布的 pub_* 的型別化包裝（由擁有者的 db/pub.sql 生成，原始 SQL 只出現在這裡）。
# 生成檔不得手改；CI 重新生成並比對（backend-monolith.yml 的「jet 生成物 drift」）。
jet:
	GOWORK=off go run ./cmd/jetgen tables -module social -out internal/modules/social/internal/jet
	GOWORK=off go run ./cmd/jetgen pub -module social -out internal/modules/social/internal/pubsql -use seller:pub_exposable_stores -use buyer:pub_blocked_buyers -use buyer:pub_blocked_stores

# 產生模組的骨架（遷移計畫 0.5）：internal/modules/<name>，登記到 cmd/hawkr/modules.go，並以 Atlas
# 產生第一個 migration（建立 schema；需要 Atlas CLI 與 docker）。from 是舊服務的名稱，寫進 AGENTS.md。
new-module:
	@test -n "$(name)" || { echo "用法：make new-module name=<模組> [from=<舊服務>]" >&2; exit 2; }
	GOWORK=off go run ./cmd/new-module module $(name) $(if $(from),-from $(from))
	cd internal/modules/$(name)/db && atlas migrate diff $(name)_schema --env dev

# 產生模組特性測試的骨架（internal/httpapi/<name>/characterization）；legacy 是舊服務的名稱，有的話另外產生
# REST driver 與起舊服務的 legacy.sh（scripts/characterize.sh 用）。
new-characterization:
	@test -n "$(name)" || { echo "用法：make new-characterization name=<模組> [legacy=<舊服務>]" >&2; exit 2; }
	GOWORK=off go run ./cmd/new-module characterization $(name) $(if $(legacy),-legacy $(legacy))

# 改完根目錄 api/ 的 proto 後，重新生成 Go 的訊息型別與 Connect 的 handler 介面（internal/httpapi/gen）；
# 同時生成遷移期呼叫 Java 服務的 stub（internal/legacy/gen，來源是 api/proto）。
# 外掛由 go.mod 的 tool 固定；buf CLI 的版本檢查在 ../api/generate.sh。
connect:
	GOWORK=off ../api/generate.sh buf.gen.connect.yaml
	GOWORK=off ../api/generate.sh buf.gen.legacy.yaml

uitext: ## 共用文案目錄與模組句子：重新生成 Go、TS、Dart，並把來源檔寫成正規形式
	../api/check-buf-version.sh
	buf build ../api -o - | GOWORK=off go run ./cmd/uitext-gen -image - -root .. -write
