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
- `internal/platform`：跨模組的基礎設施（`db` 的 `InTx`／`InReadSnapshot`、`events`、`jobs`（River）、`apperr`、`adminperm`、`seal`、`twtime`（單體唯一的台北時區）、`uitext`（共用文案目錄的 Go 執行期，`catalog/` 是來源、`catalog_gen.go` 由 `cmd/uitext-gen` 生成）等，完整清單看目錄）。platform 不得依賴任何模組。
- `internal/modules/<名稱>`：業務模組。根套件是它的公開 API，實作放在自己的 `internal/`。`db/` 放 `schema.sql`、`objects.sql`、`queries/`、`migrations/` 與 `atlas.hcl`；sqlc 生成到 `internal/store`，不得手改（go-jet 的例外見規則 2）。新模組以 `make new-module` 產生骨架。
- `internal/httpapi`：對外的入口（遷移計畫 A2）：認證（顧客 Clerk、後台 Cloudflare Access，見根目錄 `AGENTS.md` 與 ADR `docs/adr/2026-09-27-identity-clerk.md`；沒有帳號的供應商用 agent 邀請的 session cookie，見 API 規範 §二）、CORS、CSRF、限流，只做傳輸與畫面需要的讀取組裝；還沒搬的路徑轉給 gateway（`internal/legacy/gatewayproxy`）。`rpc` 是共用的授權、錯誤轉換、逾時、log、分頁 token 與 field mask，各模組的 handler 在同名子套件，`server.go` 掛上每個 service。`adminauth`：每支 `*AdminService` 方法與 `/api/files/admin/…` 路由都要宣告一個權限（`platform/adminperm`）或明列豁免，缺漏時單體不能啟動，未宣告的一律拒絕（ADR `docs/adr/2026-10-04-admin-roles.md`）。`internal/httpapi/gen` 由 `api/` 生成，不得手改。
- `internal/legacy`：遷移期的暫時轉接，每個子套件對應一個還在舊服務的功能（現有子套件以目錄為準），對應模組搬進單體後刪除。`gen/` 是 `make connect` 生成的舊服務 client。
- `internal/arch`：依賴方向檢查。

## 單體規則（都由測試或 CI 強制）

1. 模組只能 import 依賴表允許的其他模組，而且只能 import 對方的根套件。依賴表在 `internal/arch/modules.go`；改它等於改架構，先說明理由並更新 ADR 或計畫。
2. SQL 只碰自己模組的 schema，不建跨 schema FK；表一律寫完整名稱（`content.banners`）。sqlc 只讀自己的 `schema.sql`（與 `objects.sql`）加依賴表允許的模組的 `db/pub.sql`。跨模組讀取依序判斷：資料是不是放錯了擁有者；只是依 ID 補顯示欄位就呼叫對方 API；各來源都擁有全部篩選與排序鍵就在 httpapi 做 keyset composition；要用下層的屬性篩選或排序，就 join 擁有者發布的唯讀 `<owner>.pub_*`（view，或時間由請求傳入的 `BEGIN ATOMIC` 函式），算「現在的狀態」；讀取模型只在量測觸發後才建，由 outbox 投影。資料庫端的旗標或計數依賴別的模組的事實時，由同 transaction 的訂閱者把事實複製進自己的表（規則 4），`pub_*` 只給對帳用。`pub.sql` 只放 `CREATE SCHEMA` 與 `pub_*` 的形狀，函式宣告成 `RETURNS SETOF <自己>.pub_*`（`internal/arch/sqlc_test.go`、`cmd/hawkr` 的 `TestPubContractsMatchMigrations`）。多支 query 共用同一個可見性述詞時改用 go-jet（目前只有 `social`，`make jet`），邊界由 `internal/platform/querycheck` 等守門測試強制，細節與重新評估條件見 ADR `docs/adr/2026-09-27-module-internal-structure.md` 的修訂。現有的 `pub_*` 見擁有者的 `AGENTS.md`「發布的讀取介面」。
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
GOWORK=off go test -race -tags integration ./...    # integration，需要 Docker
make lint-monolith
make infra-up && make migrate && make serve         # 本機環境（:8080）；make worker 跑 River job
make dev-token args='-user dev_account_2001'        # 本機登入；管理員用 args='-admin -user dev_ops'
make dev-seed                                       # 重置測試資料（前提見 Makefile 註解）
make sqlc                                           # 改 db/schema.sql、objects.sql、queries/ 後
make jet                                            # 改 social 的 migration 或擁有者的 pub.sql 後
make connect                                        # 改 api/ 的 proto 後
make uitext                                         # 改文案目錄、模組的 text/*.json 或 proto 註解後
(cd internal/modules/<名稱>/db && atlas migrate diff <名稱> --env dev)  # 改 schema.sql 後；objects.sql 用 atlas migrate new 手寫再 atlas migrate hash
```

營運子命令（`go run ./cmd/hawkr <子命令>`，需要 `DATABASE_URL`）：`reannounce-exposures`、`delegation-eligibility`、`admin-role`、`social-recount`。搬模組時的工具（`make new-module`、`make new-characterization`、`scripts/characterize.sh`、`scripts/check-migration-inventory.py`）見遷移計畫。

`GOWORK=off` 確保單體不依賴 `go.work` 裡的舊服務 module。單體的本機設定值集中在 `Makefile`，指向 `docker-compose-infra.yml`；連別的 database 用 `make migrate DATABASE_URL=…`。舊服務的整套本機環境是 `./dev.sh`（`docker-compose-local.yml`，需要 `.env`），還沒搬的路徑由單體轉給它的 gateway（`GATEWAY_URL`，:9900）。`backend/.env` 只放本機 Clerk development key，`make serve`／`make worker` 以 `scripts/clerk-env.sh` 帶上；dev key 與 `CF_ACCESS_*` 啟動時互斥。舊服務：Go 在各自的 module 目錄跑 `go test ./...`；Java 從 `services/java/` 用 Maven reactor。

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
- 舊註解與文件裡的「`CLAUDE.md` §N」指公司時期的 backend 指令檔，已移除；仍有效的規則已併入本檔。
