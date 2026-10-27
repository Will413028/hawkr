# backend

後端正在從 `services/` 的微服務（Go 與 `services/java/`）加 gateway，遷移成一個 Go modular monolith。

- 決策與理由：`docs/adr/2026-09-24-go-modular-monolith.md`（D1–D9）
- 步驟、驗收與進度：`docs/plans/2026-09-24-go-modular-monolith.md`
- 搬遷清單：`docs/plans/2026-09-24-go-modular-monolith/`

遷移期間有兩套程式碼：

| 位置 | 是什麼 | 規則 |
|---|---|---|
| `go.mod`、`cmd/`、`internal/` | 單體（新） | 下方「單體規則」 |
| `gateway/`、`services/`、`pkg/`、`services/java/` | 舊服務，逐一搬進單體後刪除 | 新功能寫在單體；舊服務只做讓它繼續運作所需的修改 |

## 單體的結構

- `cmd/hawkr`：唯一部署的 binary，子命令 `serve`、`worker`、`migrate`、`dev-token`、`dev-seed`、`reannounce-exposures`、`delegation-eligibility`、`admin-role`。這裡是組裝點，模組之間的接線只在這裡做。`cmd/new-module` 是開發時產生模組骨架的工具（`make new-module`），不部署。
- `internal/platform`：`adminperm`（後台的權限目錄與角色對權限的對應）、`apperr`（模組錯誤的分類）、`codemeta`、`config`、`db`（`InTx`、`InReadSnapshot`）、`events`、`gcis`（經濟部商業司公開資料的統編查詢 client）、`jobs`（River）、`migrate`、`pagination`（AIP-158）、`payuni`（PayUni 子商店 API 的 client）、`phone`（台灣電話的正規化）、`policy`（平台的政策分類：檢舉人、平台判定與認定共用的 `Category` 與各入口的子集）、`revalidate`、`seal`（加密與 blind index 的金鑰環）、`testdb`、`twaddr`（內政部的縣市與鄉鎮市區代碼、郵遞區號）、`twtime`（台北時區，內嵌 tzdata；單體唯一的一份）、`uitext`（共用文案目錄的 Go 執行期：`Locale`、`Text`、請求語系的 context 縫；`catalog/` 是來源檔，`catalog_gen.go` 由 `cmd/uitext-gen` 生成；`cmd/uitext-gen` 是開發工具，不部署）。platform 不得依賴任何模組。
- `internal/modules/<名稱>`：業務模組。根套件是它的公開 API，實作放在自己的 `internal/`。`db/` 放 `schema.sql`（desired state，Atlas 與 sqlc 都讀它）、`queries/`、`migrations/` 與 `atlas.hcl`；sqlc 生成到 `internal/store`，不得手改；sqlc 是預設的 query 層，一個模組的讀取在多支 query 共用同一個可見性或資格述詞時，那些讀取改用 go-jet 建在模組自己的 `internal/visibility` 上（目前只有 `social`，規則 2、6）。第一個照這個結構搬進來的是 `content`；新模組以 `make new-module` 產生骨架（模板從 content 抽出）。
- `internal/httpapi`：對外 HTTP，也是對外的入口（遷移計畫 A2）：認證（顧客 Clerk 與後台 Cloudflare Access 的 token，`Authorization: Bearer` 為主、顧客 web 同源的 `__session` cookie 為後備，由 identity 驗證；ADR `docs/adr/2026-09-27-identity-clerk.md`；沒有帳號的供應商以 agent 的 session cookie（每張邀請一個，`__Host-hk_vp_<rfq_recipient_id>`，由請求指名要用哪一張，使用者類別 `*InviteeService`）進入，見 API 規範 §二）、CORS、CSRF、限流，只做傳輸與畫面需要的讀取組裝。還沒搬的路徑轉給 gateway（`internal/legacy/gatewayproxy`）。對外 API 是根目錄 `api/` 的 Connect（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md`，慣例在 `docs/開發規範/API 設計規範（Connect）.md`）：`rpc` 是共用的授權、錯誤轉換、逾時、log、分頁 token 與 field mask，各模組的 handler 在同名的子套件（例如 `content`），`server.go` 掛上每個 service。`adminauth` 執行後台的逐方法授權：每支 `*AdminService` 方法與每條 `/api/files/admin/…` 路由宣告一個權限（`platform/adminperm`）或明列的豁免，啟動時對照實際掛載的 admin service 的 descriptor（缺漏、多餘、權限不在目錄都不能啟動；interceptor 掛在所有 Connect service 共用的 opts，以 `rpc.AudienceOf` 只處理 admin，新增的 admin service 不必自己選用），未宣告的 procedure 一律拒絕，轉給舊 gateway 的請求也要管理員有 `legacy.gateway.forward`（ADR `docs/adr/2026-10-04-admin-roles.md`）。`internal/httpapi/gen` 是由 `api/` 生成的訊息與 handler 介面，不得手改。
- `internal/legacy`：遷移期的暫時轉接，每個子套件對應一個還在舊服務的功能（現有子套件以目錄為準），對應模組搬進單體後刪除。`gen/` 是 `make connect` 生成的舊服務 client。
- `internal/arch`：依賴方向檢查。

## 單體規則（都由測試或 CI 強制）

1. 模組只能 import 依賴表允許的其他模組，而且只能 import 對方的根套件。依賴表在 `internal/arch/modules.go`；改它等於改架構，先說明理由並更新 ADR 或計畫。
2. SQL 只碰自己模組的 schema，不建跨 schema FK；表一律寫完整名稱（`content.banners`）。sqlc 只讀自己的 `schema.sql`（與 `objects.sql`）加依賴表允許的模組的 `db/pub.sql`，參照別的模組的內部表在生成時就失敗。跨模組讀取，依序判斷：先檢查資料是不是放錯了擁有者；只是依 ID 補顯示欄位就呼叫對方 API；各來源都擁有全部篩選與排序鍵就在 httpapi 做 keyset composition；要用下層的屬性篩選或排序，就 join 擁有者發布的唯讀 `<owner>.pub_*`（view，或帶時間參數的 `BEGIN ATOMIC` 函式，時間由請求傳入），它必須算「現在的狀態」；讀取模型只在量測觸發後才建，由 outbox 投影。例外是資料庫端維護的旗標或計數所依賴的外部事實（social 的「計入」依賴買家刪帳）：函式與 trigger 只能參照模組自己的表，所以事實由同 transaction 的訂閱者複製進自己的表（規則 4），pub_* 留給對帳核對複本是否漏了（每日對帳的 Go 查詢），不給 trigger 或旗標的求值呼叫。擁有者的 `db/pub.sql` 只放 `CREATE SCHEMA <自己>` 與 `pub_*` 的形狀，函式宣告成 `RETURNS SETOF <自己>.pub_*`，由 arch 測試（`internal/arch/sqlc_test.go`：sqlc 的 schema 清單 × 依賴表、`pub.sql` 的內容）與契約測試（`cmd/hawkr` 的 `TestPubContractsMatchMigrations`：`pub.sql` 與 migration 套用後的真實物件雙向比對）強制。sqlc 是預設。模組的讀取在多支 query 共用同一個可見性或資格述詞時（目前只有 `social`；A6 的 review、im 是可能的下一批），那些讀取用 go-jet 在模組自己的 `internal/visibility` 上建，經 `db.Conn(ctx)` 執行、以 pgx 掃描；`make jet` 對只套用該模組 migration 的暫時 Postgres 生成表型別（`internal/jet`，只有模組自己的表），並由擁有者的 `pub.sql` 生成 `pub_*` 的型別化包裝（`internal/pubsql`，`cmd/jetgen`）。這些 query 沒有 sqlc 的 schema 清單擋跨模組參照，邊界改由守門測試強制：每支登記的 query 渲染出來的 SQL 以 pg_query 解析（`internal/platform/querycheck`：表只能是自己的 schema，函式只能是自己的、內建白名單或依賴表允許的 `<擁有者>.pub_*`）、原始 SQL 的逃生口（`CustomExpression`、`Token`、`Raw*`、`NewTable`）只許出現在 `pubsql` 的生成檔、公開表只經 `visibility` 的入口取得、每支 query 對測試資料庫 Describe 比對 row struct、渲染的 SQL 有 golden 檔（模板見 `social/internal/reads`）；理由與重新評估條件見 ADR `docs/adr/2026-09-27-module-internal-structure.md` 的修訂。現有的 `pub_*` 與用法見擁有者的 `AGENTS.md`「發布的讀取介面」（目前有 `seller.pub_exposable_stores(at)`、`seller.pub_review_items(at)`、`buyer.pub_inactive_buyers()`、`buyer.pub_blocked_buyers()`、`buyer.pub_blocked_stores()`）。
3. transaction 由最上層的 use case 用 `db.InTx` 開；需要一致快照的唯讀讀取（多個語句拼成一個結果、跨頁）由最上層用 `db.InReadSnapshot` 開（唯讀 `REPEATABLE READ`，不能開在 `InTx` 內，ctx 已有 transaction 時回錯誤）；repository 一律用 `db.Conn(ctx)`，不自己開 transaction。會被別的模組呼叫的方法不得用 `InReadSnapshot`（另給加入呼叫方 transaction 的入口），撞唯一鍵後不得在同一個 transaction 繼續下 SQL。
4. 下層通知上層、而且需要原子性時，用 `events.Publish`（必須在 transaction 內）；同 transaction 的訂閱者只做兩種事：原子的狀態變更（只寫自己的列、讀取時推導不出來；把擁有者的事實複製進自己的表，並讓依賴它的旗標與計數一起改，也算，例如 social 在買家刪帳的 transaction 記下 `inactive_authors` 並重新求值他的內容，因為旗標的維護必須與事實同一個 transaction 才沒有中間狀態），或只 `Enqueue` job／寫 inbox。其餘副作用與外部呼叫用 `jobs.Enqueue`。跨模組 transaction 內，可能被兩個以上模組鎖到的列由下往上鎖：identity 帳號列 → buyer → seller → 上層模組（依依賴表由下往上）→ 編排者自己的列。
5. 外部 provider（PayUni、AMEGO、Apple、R2、FCM、LLM）不在 DB transaction 內呼叫：先在 transaction 內寫業務列並排入帶冪等鍵的 job，由 worker 在 transaction 外呼叫。外部進來的 webhook 先寫 inbox。
6. 模組的 desired state 分兩個檔：`db/schema.sql` 只放表、約束、索引與 enum 型別，它們的 migration 由 Atlas（`atlas migrate diff`，只讀 `schema.sql`）產生並更新 `atlas.sum`；view、函式與複合型別（含 `pub_*`）放 `db/objects.sql`（Atlas 社群版在 desired state 裡遇到 view 就報錯），用 `atlas migrate new` 手寫 migration 再 `atlas migrate hash`，放錯檔由 `internal/arch` 的 `TestDesiredStateIsSplitBetweenSchemaAndObjects` 擋；改到底層欄位時在同一份 migration 內 drop 再建，函式一律 `BEGIN ATOMIC`（唯一的例外：觸發函式 `RETURNS trigger`，Postgres 只允許以 plpgsql 等程序語言寫它；函式只能參照模組自己的表，別的模組擁有的事實要由訂閱者複製進自己的表，見規則 2 與 4）、回傳型別不用表或 view 的整列（表加欄位時內聯會失敗；兩者由 `TestSchemaFileMatchesMigrations` 對每個模組檢查）；已套用的 migration 不得修改或刪除，要改就新增一個。`schema.sql` 加 `objects.sql` 與 migration 套用後的結果必須相同（`TestSchemaFileMatchesMigrations`；sqlc 也依序讀兩個檔）。模組要登記在 `cmd/hawkr/modules.go`。沒有自己資料的模組（目前只有 `reviewqueue`）不建 `db/schema.sql` 與 migration，sqlc 只讀依賴表允許的 `pub.sql`，不登記在 migration 清單（`modules()`；改登記在同檔的 `withoutData`）；第一次要存自己的資料時再建（`make new-module` 的骨架）。
7. 不得出現舊公司的端點或帳號識別碼（根目錄 `scripts/check-legacy-endpoints.py`，本機 pre-push hook 跑）。
8. 刪除一個舊服務之前，它在搬遷清單裡的 RPC、事件、副本都要填好 `disposition`（`scripts/check-migration-inventory.py`）。
9. 頭尾空白只有 `internal/platform/trimtext` 一個定義（Unicode White_Space，與 `strings.TrimSpace` 同一個集合）：寫入端用 `trimtext.Trim` 或 `strings.TrimSpace`（同一個集合，`trimtext` 的 `TestTrimAndStringsTrimSpaceRemoveTheSameSetOverAllRunes` 逐碼位斷言），DB 的 CHECK 寫 `trimtext.Check(欄位)` 的展開並登記在 `cmd/hawkr` 的 `trimmedColumns`；schema 與 query 不用只去 U+0020 的 `btrim`／`trim`，也不自己定義空白（指定空白字元的 trim、比對樣式裡的空白、`\s`／`[:space:]` 這類正則類別）（`TestTrimmedTextChecksUseTheCanonicalPattern`、`internal/arch` 的 `TestModuleSQLDoesNotTrim`）。

## 指令

```bash
cd backend
GOWORK=off go test -race ./...                      # unit，含依賴方向檢查
GOWORK=off go test -race -tags integration ./...    # integration，testcontainers，需要 Docker
make lint-monolith                                  # 單體 golangci-lint：一般與 integration tag，僅掃 cmd、internal
make infra-up                                       # 本機的 Postgres 18、Redis、RabbitMQ（docker-compose-infra.yml）
make migrate                                        # 單體連本機的 hawkr database
make serve                                          # :8080，/healthz、/readyz
make worker                                         # River job；沒設定 WEB_REVALIDATE_* 時不送 website 的快取失效
make dev-token args='-user dev_account_2001'        # 本機的登入：以本機的 dev 金鑰簽一個 token 印出（不連 Clerk；管理員：args='-admin -user dev_ops'）
make dev-seed                                       # 本機測試資料的重置（infra/seed-test-data.sh：前提檢查 → 01 帳號與買家 → seller 種子店家 upsert 回 fixture（明文 fixture 在 cmd/hawkr/dev_seed.go，KYC 識別欄位以執行中的金鑰加密；不碰種子以外的店家；停權、關閉、等待刪除、刪帳申請這些有下游消費者的生命週期狀態不還原，前提檢查發現就拒絕）→ 04–06 → 全量重新公告）；01 重建的帳號（9、4、2001~2099、3001~3099）若擁有種子以外的未關閉店家會在 01 之前中止，要重建本機 DB，手動測試用全新的 dev_<名稱> 帳號；要先 make migrate，只在有 DEV_AUTH_KEY_FILE 的本機開發環境執行；Makefile 的 dev-seed-check、dev-seed-stores 只給這支腳本呼叫，不單獨執行
make reannounce-exposures                           # seller 全量重新公告可曝光（不比對差異，版本照常加一）：種子重寫之後、Java 的 store_exposure 重建或死信處理之後；事件由 make worker 送出
GOWORK=off go run ./cmd/hawkr delegation-eligibility disable -admin-id 123 -operator-admin-id 456  # 營運撤回代行資格；enable 開通；需 DATABASE_URL
GOWORK=off go run ./cmd/hawkr admin-role grant -admin ops@example.com -role OWNER -reason '…'  # 授予管理員角色（-admin 是 ID 或 email，要先以 Access 登入過一次）；revoke 撤銷並撤銷進行中的代行；需 DATABASE_URL
make sqlc                                           # 改完模組的 db/schema.sql、db/objects.sql 或 db/queries/ 後重新生成 internal/store（docker，版本同 scripts/gen-sqlc.sh）
make jet                                            # 改完用 go-jet 的模組（social）的 migration 或擁有者的 pub.sql 後重新生成 internal/jet 與 internal/pubsql（docker；本機帶 DOCKER_CONTEXT 與 DOCKER_HOST，見 AGENTS.local.md）
(cd internal/modules/<名稱>/db && atlas migrate diff <名稱> --env dev)   # 改完 schema.sql 後產生 migration（只在本機需要 Atlas CLI）；改 objects.sql 用 atlas migrate new <名稱> --env dev 手寫、再 atlas migrate hash --env dev
make connect                                        # 改完根目錄 api/ 的 proto 後重新生成 internal/httpapi/gen，以及 internal/legacy/gen（buf 的版本見 ../api/.buf-version）
make uitext                                         # 改完共用文案目錄（internal/platform/uitext/catalog）、模組的 internal/text/*.json 或 api/ 的 proto 註解後重新生成 Go、TS、Dart 與 golden，並把來源檔寫成正規形式（buf 的版本見 ../api/.buf-version）
make new-module name=<模組> from=<舊服務>             # 模組骨架：AGENTS.md、CARRYOVER.md（沿用盤點）、db/、公開 API、integration 測試，並登記到 cmd/hawkr；需要 Atlas CLI
make new-characterization name=<模組> legacy=<舊服務> # 特性測試骨架：driver、情境、REST 與 Connect driver、legacy.sh
scripts/characterize.sh <模組>                      # 特性測試經本機的 gateway 打單體（與模組的舊服務，見 legacy.sh）；先 make infra-up
python3 scripts/check-migration-inventory.py --assume-gone <服務>   # 刪除前列出要補的去向
```

`GOWORK=off` 確保單體不依賴 `go.work` 裡的舊服務 module。單體在本機用的設定值集中在 `Makefile`，全部指向 `docker-compose-infra.yml` 的服務；連別的 database 用 `make migrate DATABASE_URL=…`。舊服務的整套本機環境是 `./dev.sh`（`docker-compose-local.yml`，需要 `.env`），它 include 同一份 infra。單體是對外的入口：client 連在本機跑的單體（`make serve`，:8080），還沒搬的路徑由單體轉給 `./dev.sh` 的 gateway（`GATEWAY_URL`，:9900）；顧客登入由 Clerk 處理；後台部署由 Cloudflare Access 保護。`backend/.env` 只放本機 Clerk development key，`make serve`／`make worker` 以 `scripts/clerk-env.sh` 帶上；後台本機用 `make dev-token args='-admin -user dev_ops'` 簽的 token。部署設定 `CF_ACCESS_TEAM_DOMAIN` 與 `CF_ACCESS_ADMIN_AUD` 時須停用 dev key；啟動時兩者互斥。舊服務：Go 在各自的 module 目錄跑 `go test ./...`；Java 從 `services/java/` 用 Maven reactor。

## 契約規則（單體與舊服務都適用）

- **SSOT 的優先序**：契約的唯一來源（`docs/開發規範/`、根目錄 `api/` 與 `api/proto/`、DB schema，以及舊契約的 CodeMeta）→ 對應的設計書（`docs/詳細設計書/`）→ 現有實作 → issue、畫面與口頭建議。唯一來源互相矛盾時，先指出矛盾，不要做會讓契約漂移的修改。
- **對外 API**：根目錄 `api/` 的 proto 是唯一來源，慣例依 `docs/開發規範/API 設計規範（Connect）.md`；改 API 先改 proto，再跑 `make connect`，四個消費端各自重新生成（根目錄 `AGENTS.md` 的跨專案契約表）。上線前的 package 是 `v1alpha1`，契約可以直接改；上線時改成 `v1`，之後 `buf breaking`（`WIRE_JSON`）生效。
- **舊 gateway 的 REST**（`/api/v1`，`docs/開發規範/API 設計規範.md`）：只維護到該模組搬進單體。namespace 只有 `public`、`app`、`admin`；URL path 與 query 用 `snake_case`，JSON 用 `camelCase`；統一 envelope；時間是 ISO 8601 UTC。
- **現有 API 清單**：`api/openapi/openapi.yaml` 記錄舊 gateway 還在服務的 REST，模組搬遷、重新設計 API 時對照（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md` D5）。gateway 的 swag 註解不再維護；改 gateway 的輸出或刪路由時一起改清單，`scripts/gateway-json-contract-check.py`（Check B：欄位）與 `gateway/internal/router/openapi_coverage_test.go`（路由，雙向）會檢查。
- **型別、金額與時間**：Connect 依〈API 設計規範（Connect）〉§四（proto3 JSON：`int64` 是字串、`int32` 是數字；ID 是不透明的字串）；舊 REST 的 ID、金額、數量、計數用字串。內部用語意正確的原生型別，只在邊界轉換。金額是新台幣「元」的整數，禁止浮點。時間以 UTC 為準；測試注入 clock，不用 sleep 或本機時區。
- **身分**：`account_id` 是登入 principal；`buyer_id`、`store_id` 是業務鍵，不可互換，業務表一律用業務鍵。
- **CodeMeta**：凍結，只服務還沒搬進單體的舊契約（ADR `docs/adr/2026-09-25-codemeta-retirement.md`）。單體與新契約的列舉用 proto enum 加模組的 Go 型別與 CHECK，錯誤原因用 `api/` 的 `ErrorReason`；舊契約要改值時仍先改 `.md` 再跑 `generate.sh`，不手改生成檔，也不新增值域。
- **重複實作**：新增 helper、enum、validator、mapper 前先搜尋既有的擁有者，擴充它，不要為單一呼叫方另建一份。
- **設定與 secret**：secret 不進 repo、log、測試 fixture 或 commit 訊息；timeout、retry、連線池與 shutdown 都要明確，沒有 deadline 的外部呼叫不允許。
- **測試**：bugfix 先有會失敗的測試；fixture 要 deterministic、可重跑、不含真實個資；不為了讓測試通過而放寬契約、關掉 assertion 或永久 skip。完成前跑與 CI 相同的命令並看 exit code，既有的警告與本次新增的分開說明。integration 測試的 schema 由 `pg.NewDB(t, testdb.Jobs(), testdb.Migrations(…))` 給，不在測試裡套 migration（測 migration 本身的除外），獨立的測試加 `t.Parallel()`（`internal/platform/testdb`）。

## 文件

- 設計書：`docs/詳細設計書/`；工程規範：`docs/開發規範/`；ADR：`docs/adr/`（格式見 `docs/adr/README.md`）；計畫：`docs/plans/`。
- `docs/開發規範/` 裡只適用舊服務、或已不再適用的規範（Proto、gRPC 錯誤處理、MQ、Java、多副本排程、Gateway Swagger、Git 分支、新增微服務），檔頭都有「適用範圍」標註與退場時間。
- 舊的程式註解與文件常引用「`CLAUDE.md` §N」，指的是公司時期的 backend 指令檔，已經移除；原文（`backend/CLAUDE.md`）在私人封存。其中仍然有效的規則已經併入本檔。
