# backend

後端正在從 `services/` 的微服務（Go 與 `services/java/`）加 gateway，遷移成一個 Go modular monolith。

- 決策與理由：`docs/adr/2026-09-24-go-modular-monolith.md`（D1–D9）
- 步驟、驗收與進度：`docs/plans/2026-09-24-go-modular-monolith.md`
- 搬遷清單：`docs/plans/2026-09-24-go-modular-monolith/`

遷移期間有兩套程式碼：

- 單體（新）：`go.mod`、`cmd/`、`internal/`，新功能寫在這裡，遵守下方「單體規則」。
- 舊服務：`gateway/`、`services/`、`pkg/`、`services/java/`，逐一搬進單體後刪除；只做讓它繼續運作所需的修改。

## 單體的結構

全文見 `docs/開發規範/單體開發規範.md` §1。

- `cmd/hawkr`：唯一部署的 binary，也是組裝點，模組之間的接線只在這裡做。`cmd/new-module` 是產生模組骨架的工具（`make new-module`），不部署。
- `internal/platform`：跨模組的基礎設施（`db`、`events`、`jobs`、`apperr`、`adminperm`、`twtime`、`uitext` 等），不得依賴任何模組。
- `internal/modules/<名稱>`：業務模組。根套件是公開 API，實作放在自己的 `internal/`；`db/` 放 `schema.sql`、`objects.sql`、`queries/`、`migrations/`，sqlc 生成到 `internal/store`，不得手改。
- `internal/httpapi`：對外的入口，只做傳輸與畫面需要的讀取組裝，還沒搬的路徑轉給 gateway。每支 `*AdminService` 方法與 `/api/files/admin/…` 路由都要宣告權限或明列豁免，缺漏時單體不能啟動（`adminauth`）；`gen` 由 `api/` 生成，不得手改。
- `internal/legacy`：遷移期轉接還在舊服務的功能，對應模組搬進單體後刪除。
- `internal/arch`：依賴方向檢查。

## 單體規則（都由測試或 CI 強制）

每條的全文與例外見 `docs/開發規範/單體開發規範.md` §2，編號相同。

1. 模組只能 import 依賴表允許的其他模組，而且只能 import 對方的根套件。依賴表在 `internal/arch/modules.go`；改它等於改架構，先說明理由並更新 ADR 或計畫。
2. SQL 只碰自己模組的 schema，不建跨 schema FK，表寫完整名稱（`content.banners`）。跨模組讀取先問資料是不是放錯了擁有者，再依序用：對方 API、httpapi 的 keyset composition、擁有者發布的唯讀 `<owner>.pub_*`；讀取模型只在量測觸發後才建。`pub.sql` 只放 `CREATE SCHEMA` 與 `pub_*` 的形狀（`internal/arch/sqlc_test.go`、`TestPubContractsMatchMigrations`）。現有的 `pub_*` 見擁有者的 `AGENTS.md`「發布的讀取介面」。
3. transaction 由最上層的 use case 用 `db.InTx` 開，需要一致快照的唯讀讀取用 `db.InReadSnapshot`（不能開在 `InTx` 內）；repository 一律用 `db.Conn(ctx)`。會被別的模組呼叫的方法不用 `InReadSnapshot`，撞唯一鍵後不在同一個 transaction 繼續下 SQL。
4. 下層通知上層、而且需要原子性時用 `events.Publish`（transaction 內）；同 transaction 的訂閱者只做原子的狀態變更（含把擁有者的事實複製進自己的表）或 `Enqueue` job／寫 inbox，其餘副作用用 `jobs.Enqueue`。跨模組 transaction 由下往上鎖列：identity 帳號 → buyer → seller → 上層模組 → 編排者自己的列。
5. 外部 provider（PayUni、AMEGO、Apple、R2、FCM、LLM）不在 DB transaction 內呼叫：transaction 內寫業務列並排入帶冪等鍵的 job，由 worker 在外面呼叫；webhook 先寫 inbox。
6. `db/schema.sql` 只放表、約束、索引與 enum（migration 由 `atlas migrate diff` 產生）；view、函式與複合型別（含 `pub_*`）放 `db/objects.sql`（`atlas migrate new` 手寫）。函式一律 `BEGIN ATOMIC`（觸發函式除外）、只參照自己的表；已套用的 migration 不得修改或刪除（`TestDesiredStateIsSplitBetweenSchemaAndObjects`、`TestSchemaFileMatchesMigrations`）。模組登記在 `cmd/hawkr/modules.go`；沒有自己資料的（`reviewqueue`）登記在 `withoutData`。
7. 不得出現舊公司的端點或帳號識別碼。
8. 刪除一個舊服務之前，它在搬遷清單裡的 RPC、事件、副本都要填好 `disposition`（`scripts/check-migration-inventory.py`）。
9. 頭尾空白只有 `internal/platform/trimtext` 一個定義：寫入端用 `trimtext.Trim` 或 `strings.TrimSpace`，DB 的 CHECK 用 `trimtext.Check` 並登記在 `cmd/hawkr` 的 `trimmedColumns`；SQL 不用 `btrim`／`trim`，也不自己定義空白（`TestTrimmedTextChecksUseTheCanonicalPattern`、`TestModuleSQLDoesNotTrim`）。

## 指令

```bash
cd backend
GOWORK=off go test -race ./...  # unit，含依賴方向檢查
GOWORK=off go test -race -tags integration ./...  # integration，需要 Docker
make lint-monolith
make infra-up && make migrate && make serve  # 本機環境（:8080）；make worker 跑 River job
make dev-token args='-user dev_account_2001'  # 本機登入；管理員用 args='-admin -user dev_ops'
make dev-seed  # 重置測試資料（前提見 Makefile 註解）
make sqlc  # 改 db/schema.sql、objects.sql、queries/ 後
make jet  # 改 social 的 migration 或擁有者的 pub.sql 後
make connect  # 改 api/ 的 proto 後
make uitext  # 改文案目錄、模組的 text/*.json 或 proto 註解後
(cd internal/modules/<名稱>/db && atlas migrate diff <名稱> --env dev)  # 改 schema.sql 後；objects.sql 用 atlas migrate new 手寫再 atlas migrate hash
```

營運子命令：`go run ./cmd/hawkr` 不帶參數會列出全部（營運用的需要 `DATABASE_URL`）。搬模組時的工具（`make new-module`、`make new-characterization`、`scripts/characterize.sh`）見遷移計畫。

`GOWORK=off` 確保單體不依賴 `go.work` 裡的舊服務 module。單體的本機設定集中在 `Makefile`；`backend/.env` 只放本機 Clerk development key，與 `CF_ACCESS_*` 互斥。舊服務的整套本機環境是 `./dev.sh`，還沒搬的路徑由單體轉給它的 gateway（:9900）；細節見 `docs/開發規範/單體開發規範.md` §3。

## 契約規則（單體與舊服務都適用）

- **SSOT 的優先序**：契約的唯一來源（`docs/開發規範/`、根目錄 `api/` 與 `api/proto/`、DB schema，以及舊契約的 CodeMeta）→ 對應的設計書（單體模組是 `docs/modules/<模組>.md`，其餘是 `docs/詳細設計書/`）→ 現有實作 → issue、畫面與口頭建議。唯一來源互相矛盾時，先指出矛盾，不要做會讓契約漂移的修改。
- **對外 API**：根目錄 `api/` 的 proto 是唯一來源，慣例依 `docs/開發規範/API 設計規範（Connect）.md`；改 API 先改 proto 再 `make connect`。上線前的 package 是 `v1alpha1`，契約可以直接改；上線時改成 `v1`，之後 `buf breaking`（`WIRE_JSON`）生效。
- **舊 gateway 的 REST**（`/api/v1`）：只維護到該模組搬進單體，慣例見 `docs/開發規範/API 設計規範.md`。
- **現有 API 清單**：`api/openapi/openapi.yaml` 記錄舊 gateway 還在服務的 REST（ADR `docs/adr/2026-09-26-external-contract-protobuf-connect.md` D5）；改 gateway 的輸出或刪路由時一起改它（`scripts/gateway-json-contract-check.py`、`gateway/internal/router/openapi_coverage_test.go`），swag 註解不再維護。
- **型別、金額與時間**：Connect 依〈API 設計規範（Connect）〉§四（proto3 JSON：`int64` 是字串、`int32` 是數字；ID 是不透明的字串）；舊 REST 的 ID、金額、數量、計數用字串。內部用語意正確的原生型別，只在邊界轉換。金額是新台幣「元」的整數，禁止浮點。時間以 UTC 為準；測試注入 clock，不用 sleep 或本機時區。
- **身分**：`account_id` 是登入 principal；`buyer_id`、`store_id` 是業務鍵，不可互換，業務表一律用業務鍵。
- **CodeMeta**：凍結，只服務還沒搬進單體的舊契約（ADR `docs/adr/2026-09-25-codemeta-retirement.md`）；新契約的列舉用 proto enum 加模組的 Go 型別與 CHECK，錯誤原因用 `api/` 的 `ErrorReason`。改舊契約的值照 `docs/代碼管理（CodeMeta）/README.md`，不新增值域。
- **重複實作**：新增 helper、enum、validator、mapper 前先搜尋既有的擁有者，擴充它，不要為單一呼叫方另建一份。
- **設定與 secret**：secret 不進 repo、log、測試 fixture 或 commit 訊息；timeout、retry、連線池與 shutdown 都要明確，沒有 deadline 的外部呼叫不允許。
- **測試**：bugfix 先有會失敗的測試；fixture 要 deterministic、不含真實個資；不為了讓測試通過而放寬契約、關掉 assertion 或永久 skip。integration 測試的 schema 由 `pg.NewDB` 給，不在測試裡套 migration（`internal/platform/testdb`，§3）。

## 文件

- 模組規格：`docs/modules/`；設計書：`docs/詳細設計書/`；工程規範：`docs/開發規範/`；ADR：`docs/adr/`（格式見 `docs/adr/README.md`）；計畫：`docs/plans/`。
- `docs/開發規範/` 裡只適用舊服務或已不適用的規範，檔頭有「適用範圍」標註與退場時間。
- 舊註解與文件裡的「`CLAUDE.md` §N」指已移除的公司時期指令檔；仍有效的規則已併入本檔。
