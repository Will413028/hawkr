# 後端改為 Go modular monolith：遷移計畫

**Goal：** 把 22 個 deployable（12 Go、10 Java）加 gateway，改成一個 Go module、一個 binary、一個資料庫的 modular monolith。完成後刪除內部 gRPC／proto、RabbitMQ、Flyway 與 Java toolchain。

**決策與理由：** `docs/adr/2026-09-24-go-modular-monolith.md`（D1–D9）。本檔只寫步驟、驗收與進度，不重複理由。

**盤點資料：** `2026-09-24-go-modular-monolith/`，快照為 monorepo `6c8bc0c15`，說明見該目錄的 `README.md`。

**在整體路線中的位置：** 第 3 步。第 4 步後端 CD 以本計畫完成後的單一 image 為對象；第 5 步前端 CD 與本計畫互相獨立。

> **階段 B 的改寫順序（2026-09-24 確認）**：依 D8「從依賴最少的往核心」，payment 排在 ordering 之前。這樣 ordering 改寫時，可以直接以 in-process 的方式呼叫 Go 版 payment，下單到付款當下就能用單一 transaction，不必先為遠端的 Java payment 寫一套過渡用的 outbox 或事件。金流「編排」仍然最後做：ordering（checkout 與付款結果）、aftersales（退款）、finance（撥款）。

> **對外契約定案（2026-09-26）**：Protobuf＋Connect 取代 D7 的 OpenAPI spec-first（ADR `2026-09-26-external-contract-protobuf-connect.md`；content 試點 `2026-09-25-connect-contract-pilot.md` 通過）。前端改用生成的 client 併進「模組完成的定義」，原本的階段 D 取消；0.6 的規格改為現有 API 清單，生成的 server 與 oapi-codegen 已刪除。A0 改為 gateway 當過渡入口、隨模組搬遷縮小（Will 決定）。

## 全域約束

- 系統從未上線：不做資料搬移、不雙跑、不切流量。並存只是為了讓開發環境一直能跑、CI 一直是綠的。
- 不得連到舊公司的任何端點，prod 與 dev 都算（ADR D9）。本機與 CI 只使用本機服務或測試替身。
- 暫時的轉接程式碼只能放在 `internal/legacy/`。模組不得 import 它，只有組裝點接線。
- 不搬設計書有寫、但程式沒接上的功能（ADR D8 的清單）。
- 每一步結束時，CI 必須全綠。已知的既有違規要和本次新增分開陳述。
- commit 與 push 依使用者授權；commit 訊息使用 Conventional Commits，subject 以小寫開頭。

## 模組完成的定義

每個模組都要逐項完成，才算搬遷完成，舊服務才能刪除。

- [ ] 目錄 `internal/modules/<名稱>/` 以 `make new-module` 建立（0.5）。`AGENTS.md` 寫明職責、擁有的資料、不變量、讀取模型、不負責什麼。
- [ ] **沿用盤點**：`CARRYOVER.md` 的「沿用盤點」表（決策紀錄不放進每次都載入的 `AGENTS.md`），舊服務被沿用的每個機制一行（ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例），欄位是必要條件、今天是否成立、業界在同樣條件下的做法、決定。舊 gateway 的 handler 另有「傳輸層的沿用盤點」表，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）：content 的這張表是設計審查追出來的，模板改成一開始就要求。從零設計會不同的項目，完成前提出來決定，不默默修補，也不只記待決。表存在而且每格有填由 `TestEveryMovedModuleHasACarryOverAudit` 檢查。
- [ ] **公司時期未完成項**：舊服務在公司時期留下的 open issue、未完成項與未合併分支（維護者的個人清單，依模組分組），逐項對照現在的程式碼決定：在模組裡修掉、因設計改變而不再成立，或刪除並寫理由。
- [ ] **獨立設計審查**：完成前由沒參與實作的 agent 只看設計做一次審查：照搬而沒重新評估的機制、在既有結構上將就的修補、只記待決的項目，並對照舊實作與業界做法。發現的項目處理完，或補進盤點表並有結論。
- [ ] schema 放在模組自己的 `db/`（Atlas），通過 arch 與 SQL 檢查：只碰自己的 schema，沒有跨 schema FK。
- [ ] 公開 API（`<名稱>.go`、`events.go`、`ports.go`）的依賴不超出 ADR D2 的表。
- [ ] **RPC 去向**：`rpc-servers.csv` 中這個服務的每個 RPC，`disposition` 欄都已填寫：模組 API、http handler，或刪除並註明理由。
- [ ] **事件去向**：`events.csv` 中這個服務產生或消費的每一筆，都已填寫：直接呼叫、同 transaction 事件、River job，或刪除。
- [ ] **副本去向**：`replicas.csv` 中這個服務持有或被複製的每一列，都已填寫：保留快照、刪除改查詢，或讀取模型。
- [ ] 外部 provider 呼叫都符合 D5：先寫 intent，再由 River job 呼叫。
- [ ] **代碼值與錯誤碼的去向**：舊服務用到的每個 CodeMeta 值域與錯誤碼逐一決定去向：proto enum、模組的 Go 型別與 CHECK、client 的呈現層、`ErrorReason`，或刪除。CodeMeta 那一段標上擁有者，結論寫進沿用盤點（ADR `2026-09-25-codemeta-retirement.md` 的過渡第 3 步）。
- [ ] 測試：unit 測試，加上跑在自己 schema 上的 integration 測試（testcontainers）。Java 改寫的模組，另有 integration 測試對照表，全數對上或標記退場理由。
- [ ] 特性測試：這個模組的對外 endpoint 情境寫一次、透過 driver 介面操作，REST driver 對舊 gateway、Connect driver 對單體都通過（ADR `2026-09-26-external-contract-protobuf-connect.md` D4）。骨架由 `make new-characterization` 產生，共用的部分在 `internal/httpapi/characterize`，經 gateway 的執行用 `scripts/characterize.sh <模組>`，在 CI 的 `backend-characterization.yml` 跑，以該 job 的結論為證據（本機不跑整套環境，A7 計畫 A7-2v）。
- [ ] **對外 API**：以根目錄 `api/` 的 proto 重新設計（〈API 設計規範（Connect）〉，不照搬舊的路徑與 envelope），單體以 Connect handler 提供，在單體的 `connectServices` 加一列（A2 起入口在單體，gateway 不再代理 Connect）。
- [ ] **client**：四個 client 中用到這個模組的，改用生成的 client，刪除該模組的手寫 REST 呼叫與型別；gateway 刪除該模組的 REST 路由，現有 API 清單（`api/openapi/openapi.yaml`）一起刪掉對應的端點。第二個讓 website 與 admin 都改用 Connect 的模組，把兩邊逐字重複的 Connect 錯誤分類器與重試做成 pnpm workspace 的共用套件（Will 2026-09-26；試點計畫的 client 沿用盤點）。
- [ ] 刪除舊服務的目錄、compose service、CI workflow 的路徑。proto 只在仍有 Java 呼叫方時保留，並移到 `internal/legacy/`。
- [ ] D9 gate 通過。
- [ ] 檢查這個模組相關的既有 ADR：仍然有效，或新增 ADR 取代，並標記 superseded。

## 階段 0：地基（不改變任何既有行為）

- [x] **0.1 單體骨架**
  - `backend/go.mod`：新的 module path。
  - `cmd/hawkr`：`serve`、`migrate` 子命令。`worker` 在第一個註冊 job worker 的模組搬進來時加入：River 要求至少一個 worker，沒有 worker 的角色只能空轉。
  - `internal/{platform,modules,httpapi,legacy,arch}`。對外入口叫 `httpapi`，避免套件名和標準庫的 `net/http` 撞名。
  - 過渡期把 `.` 加進 `go.work`。
- [ ] **0.2 platform**
  - [x] 核心機制：`config`（缺少或格式錯誤的設定一次回報，不退回預設值）、`db`（`InTx` 與從 ctx 取 transaction）、`events`（同 transaction 的 in-process 事件，transaction 外呼叫直接回錯誤，D4）、`jobs`（River，`Enqueue` 跟隨呼叫方的 transaction，D6）、`testdb`（integration 測試用的 Postgres）。
  - [ ] 其餘平台類套件在第一個用到它的模組搬進來時一起搬，不先搬沒有使用者、沒被測到的程式碼：logger、otel、errors、timeutil、pagination、httputil、httpclient、middleware、ratelimit、redis、crypto、phone。codemeta 不再擴充：凍結，隨模組搬遷退場（ADR `2026-09-25-codemeta-retirement.md`）。
    - 隨 A1 搬入（2026-09-25）：
      - `apperr`：取代 `pkg/errors` 的分類，模組只回報種類（目前只有 content 用到的 InvalidInput、NotFound）與訊息，對外的狀態碼由 http 層決定。業務碼等第一個需要它的模組再加。
      - `tsid` 不搬：實體的主鍵改成應用層以 Go 標準庫 `uuid.NewV7()` 產生的 UUIDv7，不另建 platform 套件；舊服務改寫前照舊使用 `pkg/tsid`。識別的規則見 ADR `2026-09-25-row-identity-by-data-kind.md` 與 PostgreSQL 設計規範 §二。
      - `revalidate`：從 goroutine 改成 River job，見責任邊界規範 §9.5 的單體註記。
      - log 用標準庫的 `log/slog`，不搬 `pkg/logger`（zap）。
      - `timeutil`、`pagination`、`mediaurl` 沒有搬：模組的 API 直接收 `time.Time`，分頁與 object key 的檢查各只有幾行，放在使用它的模組。
      - `--go-media` 另外輸出到 `internal/modules/media/ref_type.go`，CI 檢查 `--go`、`--go-media` 生成物的 drift。
    - [x] **ID 策略**（2026-09-25 決定）：`pkg/tsid` 沒有 node 位元，serve 的多個 replica 加上 worker 同一毫秒會撞號。改成依資料性質決定識別：實體用 UUIDv7，值的集合與關聯表用複合鍵，不需要任何部署協調（ADR `2026-09-25-row-identity-by-data-kind.md`）。content 已照做。
  - [ ] 帶業務語意的套件不進 platform，隨所屬模組搬：delegation、rolestate、payuni、moderation、blocklist、geocode、sms、mail。
  - [ ] 沒人使用的套件刪除：`membershipkernel`、`csvutil`、`cmd`。
  - 外部呼叫的 intent（D5）由 River job 承載：業務列與帶冪等鍵的 job 在同一個 transaction 內寫入，worker 在 transaction 外呼叫 provider。共用的 helper 在第一個外部整合搬進來時，依實際需要抽出。
- [ ] **0.3 arch 檢查**（Go test）
  - [x] 依賴表是單一檔案（`internal/arch/modules.go`），由 ADR D2 的表轉成。每個模組的實際 import 必須在表內。
  - [x] 模組不得 import `internal/legacy`；platform 不得 import 模組。
  - [x] migration 的隔離：每個模組的 migration 單獨套用在空的 database 上，參照別的模組（例如跨 schema FK）就套用失敗，把物件建在自己的 schema 以外就被列出來。由 Postgres 本身判斷，不另寫 SQL parser。
  - [x] query 的隔離：每個模組的 sqlc 只讀自己的 `db/schema.sql`，參照別的模組的 query 在 `sqlc generate` 時就失敗。`make sqlc` 以 `scripts/gen-sqlc.sh` 的同一個 image 生成；`backend-monolith` 重新生成並比對，並確認 `internal/arch/testdata/sqlc-foreign-schema` 這份故意違規的 fixture 因為 schema 不存在而失敗。
  - [x] 每條規則都附一份故意違規的 fixture，而且這份 fixture 必須讓檢查失敗。
- [ ] **0.4 Atlas 與 migration**
  - [x] `internal/platform/migrate`：migration 檔 embed 進 binary，`hawkr migrate` 依登記順序套用全部模組。套用前驗證 `atlas.sum`（以 Go 重新實作 Atlas 的演算法，已用 12 個服務的 `atlas.sum` 逐位元比對）；已套用的檔案不得修改或刪除；新檔案的版本不得早於已套用的版本；以 advisory lock 保證同時只有一個 migrate。執行期與 CI 都不需要 Atlas CLI。
  - [x] `cmd/hawkr/modules.go` 登記每個模組；測試保證 `internal/modules` 底下的每個模組都有登記。
  - [x] 開發時產生 migration 用的 Atlas 設定（每個模組自己的 `db/`：schema、migrations、`atlas.sum`），隨 A1 試點建立並實際跑一次。
    - Atlas 以 database 範圍執行，產生的 DDL 帶 schema 名稱；`schema.sql` 要寫 `CREATE SCHEMA IF NOT EXISTS public;`，否則 Atlas 會產生 `DROP SCHEMA public`。
    - 目前沒有模組使用 Postgres extension，platform 也沒有對應的機制。content 原本的 pg_trgm 索引拿掉了，理由見 content 模組的 `AGENTS.md`（兩個字的中文關鍵字用不到索引）。第一個真正需要 extension 的模組搬進來時（social 的 hashtag 搜尋用 `similarity()`，A7），照這個已經在 2026-09-25 實測過的做法加上：
      - extension 是整個 database 共用的，不屬於任何模組，裝在共用的 `extensions` schema，引用時寫完整名稱（例如 `extensions.gin_trgm_ops`）。
      - Atlas 社群版不管理 extension：模組 migration 目錄開頭放一個手寫的 migration，冪等地建立（`CREATE SCHEMA IF NOT EXISTS extensions` 加 `CREATE EXTENSION IF NOT EXISTS … WITH SCHEMA extensions`），`atlas migrate hash` 後再 diff；`schema.sql` 也寫同樣兩句。模組之間因此沒有套用順序的依賴。Atlas docker 的 `baseline` 需要登入 Pro，不採用。
      - `ForeignObjects` 接受 `extensions` schema，但要列出其中不屬於 extension 的物件（`pg_depend.deptype = 'e'`），並檢查 `pg_extension` 都裝在 `extensions`：裝在別的 schema 時，其他模組的 `CREATE EXTENSION IF NOT EXISTS … WITH SCHEMA extensions` 會變成 no-op，引用就找不到。
    - `TestSchemaFileMatchesMigrations` 把每個模組的 `schema.sql` 與 migration 分別套用在空的 database 上，比對 `DescribeSchema` 的結果（表、欄位、約束、索引、sequence、view、函式、enum、domain、trigger），sqlc 讀的檔與部署的 schema 因此不會分岔。`schema.sql` 本身也必須通過 `ForeignObjects`：它若建立別的模組的表，參照別的模組的 query 就能通過 sqlc 生成。CHECK 的 `IN` 清單要寫成 `col::text IN (…)`，否則 Atlas 產生的寫法經 Postgres 重新解析後文字不同（語意相同）。
  - `-- atlas:txmode none` 這類 directive 目前明確拒絕：新的 baseline 都是空表，不需要 `CREATE INDEX CONCURRENTLY`；真的需要時再實作。
- [x] **0.5 模組模板與 scaffold**（2026-09-27，從 A1 的 content 抽出）
  - `make new-module name=<x> from=<舊服務>`（`cmd/new-module`，開發工具、不部署）：`AGENTS.md`（各節）與 `CARRYOVER.md`（兩張沿用盤點表，空列讓 `TestEveryMovedModuleHasACarryOverAudit` 失敗到填好為止）、公開 API 的根套件（embed migration、`Deps`）、`db/` 的 `schema.sql`、`atlas.hcl`、`sqlc.yaml`、只套用自己 migration 的 integration 測試骨架；登記到 `cmd/hawkr/modules.go`，以 Atlas 產生建立 schema 的第一個 migration。名稱必須已在依賴表。`make sqlc` 略過還沒有 query 的模組。
  - `make new-characterization name=<x> legacy=<舊服務>`：特性測試骨架（driver、情境、Connect 與 REST driver、直接打單體與經 gateway 的執行、起舊服務的 `legacy.sh`）。共用的部分抽成 `internal/httpapi/characterize`，起服務的腳本是模組無關的 `scripts/characterize.sh`（設計審查的發現：刪 content-service 時一起刪掉的 harness 之後每個模組都要用）。`ch.Run` 在沒有情境時失敗。
  - 模板本身由 `cmd/new-module` 的測試守住：產生的 Go 檔合法而且 gofmt 過、`legacy.sh` 語法正確、登記加在清單最後。
- [x] **0.13 單體的 golangci-lint 進 CI**（Will 2026-09-27；排在 A2 的 C10 之後、0.12 之前，搬程式時就受 lint 約束）
  - 單體另寫一份設定（只掃 `cmd`、`internal`）：standard（errcheck、govet、staticcheck、unused、ineffassign）、errorlint、nilerr、gocritic 的 diagnostic 類、bodyclose、noctx、gosec、exhaustive；forbidigo 在模組裡禁止直接呼叫 `time.Now()`（時間由 Deps 注入）；depguard 禁用 `log`（用 `slog`）與正式程式的 `math/rand`。不用 prealloc。舊服務沿用 `backend/.golangci.yml`（公司時期為 gateway 寫的，ruleguard 指向 `gateway/ruleguard`，對單體跑不起來），隨遷移刪除。
  - 零問題為門檻，現有的問題一次修完、不留基準線（2026-09-27 以暫時設定量到 77 個：errcheck 44、prealloc 19、errorlint 10、其他 4；正式設定的數字以實跑為準）。
  - `backend-monolith.yml` 加 lint job（golangci-lint 的版本鎖定）；`backend/AGENTS.md` 的指令段加上本機的跑法。
  - 驗收：CI 的 lint job 通過；突變：在模組裡加一個 `time.Now()`、把一個 `%w` 改成 `%v`，各自要被擋。
  - 結果（2026-09-27）：設定是 `backend/.golangci.monolith.yml`，本機跑 `make lint-monolith`（一般與 `integration` tag 各一次），CI 的 lint job 釘 golangci-lint v2.13.2。修掉的問題：一般 122 個、integration 143 個，最多的是 errcheck 34／46、noctx 30／31、gosec 23／24。`//nolint` 都是單行、附理由：gosec 是整數原位元轉換與 `new-module` 產生檔的權限；errorlint 是下游錯誤只留在 log 文字的地方。設定層的排除只有三條：forbidigo 只管 `internal/modules/`、測試檔的特定 gosec 規則、`convert.go` 的 G101。突變：加 `time.Now()` 被 forbidigo 擋，`%w` 改 `%v` 被 errorlint 擋。修 errorlint 時把下游錯誤改成 `%w`，下游帶的 `*apperr.Error` 就蓋過了外層的欄位錯誤；integration 測試與 code review 抓到後，一律改成下游只當文字，並加上單元測試。race、integration 都通過。
- [x] **0.12 模組內部的結構**（ADR `2026-09-27-module-internal-structure.md`，Will 2026-09-27；排在 A2 的 C10 之後）
  - `internal/arch` 加上檢查：根套件不得 import `internal/store`；每個模組的 `internal/` 套件依賴圖（寫在模組的 `AGENTS.md` 或測試的表），不准反向與循環。
  - 重構 content、media、identity：use case 搬進 `internal/<子領域>/`，對外型別移到 `internal/model` 並在根套件以別名公開，根套件只轉呼叫；整合測試維持經公開 API。
  - `cmd/new-module` 的模板改成產生這個結構，模板的測試同步。
  - 代行拆成 `delegation` 模組（ADR `2026-09-27-delegation-module.md`）：新 schema 重建三張表、程式從 identity 搬進 delegation 的 `internal/`；identity 在管理員登出、移除時發事件，delegation 在同一個 transaction 撤銷；`internal/legacy/delegationservice` 改接 delegation；依賴表的說明同步。
  - 驗收：unit 與 integration 測試、`internal/arch` 的新檢查（突變：在根套件加一個 `store` 的 import 要失敗）、`make new-module` 產生的骨架可以編譯。
  - 結果（2026-09-28，commit 分三段）：
    - `internal/arch` 的依賴圖寫在 `modules.go`，是唯一來源，模組的 `AGENTS.md` 只指向它。檢查擋下這些情況：
      - 根套件 import `store`，沒有例外；
      - 未宣告的套件；
      - 反向或循環的依賴；
      - `internal/` 以外的子套件；
      - 圖上有、磁碟上沒有的節點。
    - content、media、identity、delegation 都是門面加 `internal/<子領域>`。`Deps` 由根套件定義，各子領域宣告自己需要的最小依賴，`model` 保持葉節點。`Jobs` 介面只有一個擁有者（`internal/platform`），AIP-158 的每頁筆數正規化也收成一份。
    - 模板產生同樣的結構，模板測試會對產生的骨架跑依賴圖檢查，也會編譯特性測試的骨架。
    - delegation 是獨立模組（schema `delegation`），四張表不對 identity 下 FK；管理員名稱由 identity 的 `Admins` 以每頁一次的批次查詢取得。
    - design-review 的三項由 Will 決定（2026-09-28，寫在 delegation ADR 文末的修訂）：
      - 代行資格（allowlist）搬進 delegation：停用資格時在同一個 transaction 撤銷進行中的代行；營運以 `hawkr delegation-eligibility` 操作。
      - 後台登出由 httpapi 直接呼叫 delegation 撤銷，不經事件：改用 Access 後，identity 在登出時沒有要寫的資料。
      - 代行 credential 只在兌換 handoff code 時以亂數簽發、只存 hash：create 只回 handoff code，admin BFF 以管理員身分讀 session 摘要對帳；HMAC 金鑰與輪替、`nextval` 預取、`OVERRIDING SYSTEM VALUE` 都刪除。gateway、admin BFF、舊契約 proto、現有 API 清單與設計書 56 同步。
    - 驗收：unit、integration（testcontainers）、`make lint-monolith` 0 個問題；gateway 全套測試；admin 566 項測試；proto 生成物比對；gateway JSON 契約 gate。突變：在根套件加 `store` 的 import，arch 測試失敗。
    - 已知限制（不在 0.12 範圍）：media 換圖時 release 的競態，留給 B1 放進同一個 transaction 根治。
- [x] **0.6 OpenAPI**（2026-09-26 由 Connect 取代，ADR `2026-09-26-external-contract-protobuf-connect.md`：規格保留為現有 API 清單，`internal/httpapi/openapi`、規格 lint 與 oapi-codegen 刪除；以下是當時的做法）
  - 選定生成工具並做小規模驗證：Go strict server 介面、TS client、Dart client 各一個。
  - 從 swag 轉出初始規格（約 600 個 endpoint），以 `scripts/gateway-json-contract-check.py` 的 baseline 驗證 wire 格式一致。
  - CI 加入規格 lint 與生成物 drift 檢查。
  - 生成工具：Go 用 oapi-codegen 的 strict server，TS 用 openapi-typescript 加 openapi-fetch，Dart 用 openapi-generator 的 dart-dio。選擇理由與被否決的方案見 ADR `2026-09-25-openapi-toolchain.md`。
  - 規格：`api/openapi/openapi.yaml`，OpenAPI 3.0.3，612 個 operation。swag 的 Swagger 2.0 以 kin-openapi 轉成 3.0 後正規化：
    - operationId 取 handler 函式名。
    - path 依 gin 路由表加上 `/api/v1`；PayUni 回跳與 Apple 網域驗證等 7 條本來就在根目錄。
    - schema 名稱去掉 Go package 前綴。
    - 錯誤回應收斂成 `ErrorEnvelope`，成功回應的 allOf 攤平成 envelope。
    - tag 改成唯一的 ASCII 名稱，中文放進 description。
    - 補上 web 用的 cookie 認證。
    - 註解本身寫錯的地方，以 handler 為準修正：與 path 樣板對不上的參數、缺少的 200、`*/*`、SSE 與 `c.String` 回應的 content type。path 參數的名稱不影響 wire，其餘沿用註解；`/app/auth/providers/{id}` 的 POST 與 DELETE 因為 OpenAPI 不允許同一層只差參數名，統一叫 `id`。
  - wire 驗證：Check B 改成比對規格與 handler 的實際輸出（原本比的是 `@Success` 註解），解析器也補上 `m["key"] = …` 這種條件式欄位。共比對 116 支 handler，找到的落差都以 wire 為準改進規格：baseline 原本登記的 3 支、`listAvailableCoupons` 的 records 形狀，以及 coupon 模板漏掉的 `claimRequired`、`claimed`、`targetAccounts[].phone`。baseline 的 Check B 條目歸零。Check A（舊身分名）也改為掃描規格，規格裡既有的 17 個舊名與程式碼端的條目同源登記。
  - 路由覆蓋：gateway 的路由表與規格逐條比對（`gateway/internal/router/openapi_coverage_test.go`），也檢查 operationId 等於 handler 方法名，因為 Check B 靠這個對應找 handler。還沒寫進規格的 44 條登記在該測試裡：代行營運 24、binary 串流 proxy 9、沒寫註解 4、swag 漏收 1、PayUni webhook 1、本機工具 5。所屬模組搬遷時補齊（模組完成的定義：對外 endpoint 都在 OpenAPI 規格內）。
  - lint：`internal/httpapi/openapi/spec_test.go`，檢查結構、operationId、envelope、content type 與命名規則。v1 既有的 149 個命名例外登記在 `api/openapi/lint-baseline.json`，只准變短。
  - 生成物：`internal/httpapi/openapi/openapi.gen.go`，以 `make openapi` 生成，`backend-monolith` workflow 比對 drift；`strict_test.go` 驗證只註冊部分 operation、query 綁定、envelope 的 wire 形狀，以及錯誤一律是 `ErrorEnvelope`。TS 與 Dart 的生成物在階段 D 接進各前端，drift 檢查放在各前端的 CI。
  - 已知的精度缺口，在模組搬遷、handler 改由規格生成時逐一收緊：
    - swag 沒有 required 與 nullable 的資訊，所以 DTO 欄位全是選填，也沒有 nullable。
    - 110 支 handler 的成功回應 data 是自由物件或沒有 data，無法比對。
    - 4 處陣列沒有元素型別。
  - 規格成為唯一來源後，gateway 的 swag 註解不再維護；改 gateway 的輸出時改規格，Check B 與路由覆蓋會檢查。
- [x] **0.7 D9 gate**（`scripts/check-legacy-endpoints.py`、`legacy-endpoints` workflow）
  - denylist：舊公司網域的 host、公司 infra IP、PlanetScale host、公司 R2 帳號、舊公司的 ghcr.io 組織、公司的 Workers 子網域與 OSS bucket，再加上盤點時找到的公司第三方帳號：Sentry org、Firebase 專案、Google OAuth client（3 個 GCP 專案）。Apple 的 `DEVELOPMENT_TEAM` 是打包簽章身分，不是執行期連線，歸第 5 步前端 CD。
  - 新單體的路徑（`backend/cmd/`、`backend/internal/`）零容忍；其他路徑以 `scripts/legacy-endpoints-baseline.json` 起算（建立時 476 個檔案、2322 個命中），只能減少、不能增加；減少時要求 `--update` 鎖住新低點。Markdown 與 `docs/` 是歷史紀錄，不在範圍內。
- [x] **0.8 盤點 gate**（`backend/scripts/check-migration-inventory.py`、`backend-migration-inventory` workflow）
  - `rpc-servers.csv`、`events.csv`、`replicas.csv` 加上 `disposition` 欄，格式為 `<種類>: <說明>`：RPC 是 `api`、`http`、`removed`；事件是 `call`、`in-tx-event`、`job`、`removed`；副本是 `snapshot`、`query`、`read-model`、`removed`。
  - 何時必須填：RPC 在實作它的服務目錄刪除時；事件在 producer 與 consumer 都刪除時（只刪一邊時，事件還經由 legacy adapter 流動）；副本在持有它的服務刪除時。只存在於設計書的事件（doc-only）不需要填。
  - 刪除某個服務之前，先跑 `python3 backend/scripts/check-migration-inventory.py --assume-gone <服務>`，列出要補的項目。
- [x] **0.9 本機環境**
  - compose 裡一個 Postgres，同時放單體的 database 與 Java 服務各自的 database；加上 Redis，以及過渡期仍需要的 RabbitMQ。
  - 單體 config 的預設值全部指向本機。
  - 做法：基礎設施拆到 `docker-compose-infra.yml`（Postgres 18、Redis、RabbitMQ、OTel collector），不需要 `.env` 就能起；`docker-compose-local.yml` 以 `include` 引入同一份。18 的 image 拒絕沿用 17 的 volume，volume 改名為 `postgres18_data`。
  - config 對必填設定仍不給預設值（0.2）；本機的值集中在 `Makefile`（`make infra-up`、`migrate`、`serve`），全部指向本機，環境變數蓋不掉、命令列可以覆寫。
  - `scripts/init-databases.sh`：加入單體的 `hawkr`，清單等於單體加上每個舊服務的 `DB_NAME`（補上一直缺少的 `hawkr_product_import`，移除沒有服務使用的 `hawkr_marketing`）。修正既有 bug：initdb 階段的暫時 server 只聽 unix socket，腳本用 TCP 連不上，首次啟動時一個 database 都沒建（過去靠 `./dev.sh up` 事後補建才沒被發現）。
  - gateway 的 host port 8080 沒有程式在聽，拿掉，讓在本機跑的單體（`:8080`）可以和舊服務同時跑。
  - 驗證（2026-09-25）：空的 volume 首次啟動就建好 23 個 database；12 個 Go 服務的 Atlas migration、10 個 Java 服務的 Flyway migration（9.22.3，只警告未測試 18）都在 18 上套用成功；`make migrate` 可重跑，`make serve` 的 `/healthz`、`/readyz` 回 204，停掉 Postgres 後 `/readyz` 回 503。
- [x] **0.10 指令檔**
  - 指令檔一律是 `AGENTS.md`，所有 `CLAUDE.md` 移除（2026-09-25 使用者決定）：backend、admin、website 的內容併入各自的 `AGENTS.md`；兩支 App 的 `CLAUDE.md` 改名為 `AGENTS.md`。website 保留 Next.js 管理的規則區塊，`next dev` 因此只更新 `AGENTS.md`、不會再建立 `CLAUDE.md`（已用 Next.js 自己的函式驗證）。
  - 根目錄 `AGENTS.md`：monorepo 地圖、與 CI 相同的檢查、跨專案契約、共通規則。原本分散在各子專案「跨 Repo 共識」裡仍然有效的規則集中到這裡；`dev` 分支、promote、跨 repo session 這些公司時期的流程移除。
  - `backend/AGENTS.md` 改寫：單體的結構與規則、指令、仍然有效的契約規則。舊程式註解引用的「`CLAUDE.md` §N」指向私人封存的 `backend/CLAUDE.md`。
  - `docs/開發規範/` 裡只適用舊服務或已不再適用的 9 份規範，檔頭加上適用範圍與退場時間；責任邊界規範 §七 加註單體內 saga 與 outbox 的例外已不成立。
- [ ] **0.11 CI**：新增 `backend-monolith` workflow，涵蓋 build、vet、`test -race`、arch、SQL、Atlas、Connect 生成物 drift、D9 gate、盤點 gate、integration 測試（OpenAPI drift 於 2026-09-26 隨 Connect 定案移除）。

**驗收**

- 在 backend 根 module 執行 `go build ./... && go test ./...`，全部通過。
- arch、SQL、D9、盤點四個 gate 的故意違規 fixture，都會讓檢查失敗。
- 在空的本機 Postgres 上，`hawkr migrate` 成功；`hawkr serve` 能啟動並回應 health check。
- 新的 workflow 在 GitHub 上是綠燈。

## 階段 A：Go 服務與 gateway 併進單體

- [x] **A0 gateway 當過渡入口，隨模組搬遷縮小**（2026-09-26 重新定義，Will 決定）
  - 原做法（gateway 的 REST handler 經 `internal/legacy` 整批搬進單體、單體取代 gateway）取消：對外契約改為 Connect（ADR `2026-09-26-external-contract-protobuf-connect.md`），每個模組搬遷時 API 都要重新設計，整批搬進來的約 600 支 REST handler 與測試都會再刪一次。
  - 做法（strangler facade）：gateway 繼續負責認證、CSRF、CORS、限流與還沒搬的模組的 REST，並依 service 名稱把 Connect 代理給單體（試點的「過渡期架構」，`gateway/internal/router/routes_connect.go`）。gateway 不再加功能，只做搬遷需要的改動。
  - 每個模組搬遷：gateway 刪掉該模組的 REST 路由、handler 與 gRPC client，單體的 `connectServices` 加一列（模組完成的定義）。
  - A2 identity 搬進單體時，入口移到單體：認證、session、CSRF、CORS、限流成為 `internal/httpapi` 的中介層，還沒搬的 REST 由單體轉給 gateway，gateway 改成信任單體帶的身分 header（與現在單體信任 gateway 的方式相反）。這樣 gateway 不必回頭呼叫單體驗證 session。
  - 最後一個模組（階段 B 的 Java 改寫）搬完時刪除 gateway（階段 C）。
  - 完成（2026-09-26）：Connect 代理與依 service 名稱的授權已在試點步驟 4 建好；之後的義務寫在模組完成的定義、A2 與階段 C。
- [x] **A1 content**（試點，2026-09-27 完成）
  - 用它驗證整套做法：模板、schema、Connect 的 handler，以及 media 的公開 API 加上呼叫 Java 的暫時實作。
  - 試點結束後，把學到的調整回寫模板與本計畫：模板見 0.5，本計畫的模組完成定義加上傳輸層的沿用盤點與特性測試的骨架。
  - 對外契約由 Connect 試點驗證並通過（`2026-09-25-connect-contract-pilot.md`，ADR `2026-09-26-external-contract-protobuf-connect.md`）。
  - [x] 模組本體（2026-09-25）：`internal/modules/content` 的 schema 與新 baseline、sqlc、公開 API、unit 與 integration 測試；與 content-service 的差異寫在模組的 `CARRYOVER.md`。media 只有公開 API（`internal/modules/media`，登記在 `apiOnly`）。
  - [x] worker 角色：`hawkr worker`，第一個 job 是 website 的快取失效。
  - [x] http 層與 `internal/legacy` 裡呼叫 Java media-service 的 client（2026-09-25，Connect 試點的步驟 4）：`internal/httpapi/{rpc,content}`、`internal/legacy/mediaservice`，gateway 以 Connect 代理給單體。
  - [x] buyer-app 的 `recipe_model.dart` 把食譜 ID 解析成 `int`（`int.tryParse(...) ?? 0`），UUID 會被靜默變成 0：隨試點步驟 6.6 改成字串。
  - [x] 去向與刪除 content-service（2026-09-26）。刪除前的最後一次特性測試（試點步驟 8）同一份情境對舊的 gateway＋content-service 與單體都通過，四個 client 已換成 Connect（試點步驟 6）。
    - `rpc-servers.csv` 的 21 個 RPC 都填了去向：站點文案 3 支與 `GetIngredientSuggestions` 刪除，其餘對應到 Connect 的方法；content-service 沒有事件與副本。
    - gateway 刪掉 content 的 REST 路由、handler 與 gRPC client，現有 API 清單刪掉對應的 20 個端點與只被它們引用的 schema（`maxRESTOperations` 612 → 592）。`api/proto/content` 沒有 Java 呼叫方，連同 `pkg/proto/content` 與 Java 的副本刪除。
    - 盤點漏列的站點文案讀取方：gateway 的退貨文案 SSE 先讀 key `return-policy`、沒有值才退回 ai-service 的固定文案；後台頁刪除後已經寫不進值，改成一律由 ai-service 回固定文案。
    - 特性測試的 REST driver 與經 gateway 的執行（`scripts/characterize-legacy-content.sh`）隨 content-service 刪除；情境與 Connect driver 留在 CI 的 integration 測試，gateway 的 Connect 代理由 gateway 自己的 router 測試涵蓋。
    - `./dev.sh` 的 gateway 補上 `MONOLITH_HOST`（預設 `host.docker.internal`）：已搬進單體的模組在 compose 環境裡由 gateway 代理給本機跑的單體（0.9 讓兩者並存，但一直沒接上）。
  - [x] 模組完成的定義逐項核對（2026-09-27）
    - 公司時期未完成項：快照（wiki 的 legacy-backlog）沒有 A1 的項目。
    - 相關 ADR：版本門檻 ADR 的 D2、D7 加註；D10 的前提（兩個 repo、沒有共用套件）在 monorepo 不成立，由 `2026-09-27-app-version-gate-shared-package.md` 取代，檢查邏輯抽成 `packages/hawkr_app_update`（Will 決定）；首頁動態排序的 ADR 加註從未實作、依 D8 不搬。
    - D9：baseline 鎖在 317 個檔案、1210 個命中。
    - 獨立設計審查：9 項發現，沒有駁回。改 4：排程的 CHECK 與寫入端不一致（新 migration 與測試）、A7 與 B4 記下 banner 點擊行為要確認商品與貼文存在、設計書 25 與 50 加上單體註記、A1 逐項寫出結論。記 2：盤點表補上 CodeMeta 的 `AccountType`、`AssetReferenceType`；刪掉的兩條告警沒有替代，單體的監控（River job 的狀態、存活）記進 wiki 的 CD 待辦。交給 Will 決定 3 項並照決定做完：快取失效的重試改成退避（§9.5 的單體註記）、特性測試的共用部分抽成 `internal/httpapi/characterize` 並保留經 gateway 的執行到 A2（另一項「只剩一個 driver 的特性測試與 handler 測試重複」併入這個決定：A2 時一起退役）。
- [x] **A2 identity**（2026-09-29 完成；交給 Will 的 Clerk、Apple 設定與 App 實機驗收在 A2 計畫的 C9 段）：來自 account-service，不含代行編排；入口從 gateway 移到單體（見 A0），IM 的 WebSocket 與還沒搬的 SSE、REST 經單體轉給 gateway，直到所屬模組搬遷。步驟、驗收與進度在 `2026-09-27-a2-identity.md`；身分驗證改用 Clerk（ADR `2026-09-27-identity-clerk.md`，Will 2026-09-27），它取代 ADR `2026-09-27-identity-opaque-sessions.md` 的憑證與 OTP 決策，步驟在 A2 計畫的〈改用 Clerk〉。
- [ ] **A3 buyer、seller**：seller 不含推薦排行；buyer 往上的補資料移到 http。步驟、決定與進度在 `2026-09-29-a3-buyer-seller.md`（盤點在同名目錄）；下面兩個子項已併入該檔的步驟 7。
  - seller 第一個 `*AppService`：實作錯誤模型 ADR `2026-09-25-api-error-model.md` D10 的賣家停權（`permission_denied` 加 `ErrorInfo.reason` `SELLER_SUSPENDED`，取代 9240 加 HTTP 409），在同一個變更加入 `ErrorReason` 的值、單體裡能力檢查的產生、client 把它轉成賣家身分的停權狀態，並端到端驗證（Connect 試點的通過條件 7）。能力檢查的做法（逐方法宣告、Redis 快取、fail-open）依模組完成的定義重新評估 ADR `2026-09-18-seller-capability-authorization.md`。
  - 申請成為商家：舊 gateway 的 `POST /app/sellers` 在 seller-service 建店並授予 SELLER 之後，才驗證城市代碼，驗證失敗回 400 而商店與角色已經寫入（A2 的 compose 驗收，2026-09-29）。搬進 seller 模組時，輸入驗證放在建店之前，建店與授予角色同一個 transaction。

A2、A3 的帳號、買家、賣家 ID 仍被 Java 服務存成 `BIGINT`，主鍵照 ID 策略的過渡規則暫用 `bigint GENERATED ALWAYS AS IDENTITY`，不用 TSID；最後一個 Java 消費者改寫後，以一個 migration 換成 uuid。
- [x] **A4 pricebook**：取消，不搬。pricebook 整個刪除（ADR `2026-10-27-general-ecommerce-scope-cut.md` D1），執行在 `2026-10-27-ecommerce-scope-cut.md` 步驟 5、6；完成時勾掉這一項。
- [ ] **A5 invoice**：先不接手發票快照，等 B8 的 ordering。
  - 機敏檔的後台讀取（A3 計畫 8f-1 留下的終點）：稅籍佐證與發票 PDF 改由 invoice 模組的 `/api/files/admin/invoice/...` 端點提供，網址由擁有者的 Connect 方法回傳，讀取經 `media.Media.ReadSecure`，存取紀錄進 invoice 模組（先寫再讀，寫不進去就不送），admin 的檢視不帶 asset ID。舊 invoice-service 的 admin 發票 PDF 下載目前不記讀取者，搬遷時一併補上。
  - 發票申訴的違規累計交給 seller（Will 2026-10-28，ADR `2026-09-18-invoice-service-architecture.md` 文末修訂）：invoice 搬進單體時刪掉 `seller_tax_profiles` 的 `violation_window_days`、`violation_threshold`、`suspension_days` 三欄與 `recommendation`、`suggestedSuspendDays`（含 proto、openapi、gateway DTO，以及 admin 申訴頁與建立處分對話框對它們的使用）；admin 改為顯示去重後的已確認申訴作為依據，由管理員經 seller 的 `CreateMeasure` 建立認定；決定發票是否新增認定類別（58A X-a、X3）。
- [ ] **A6 review、im、crm**（緊接 A7，排在 A4、A5、A8 之前：A7 不寫往舊 im／review 的橋（S3），聊天與評價的封鎖、檢舉在 A7-4e 到 A6 的 client 步驟之間本機會失敗，Will 2026-10-18 接受（A7 計畫 Q1）；步驟草案見 `2026-10-18-a7-social/decision-brief.md` §4 的 A6-0 到 A6-7，A6-0 照 A7-0 的格式重做盤點與計畫）
  - 封鎖（A7-D2 留下）：聊天與評價的封鎖、檢舉入口改用對話或評價的參照，im、review 以 in-process 呼叫 buyer、seller 的「是否封鎖」模組 API（A7-1b 建好，A7 不做 legacy gRPC）；moderation 加聊天與評價對象（快照、隱藏與還原 in-process）；CRM「黑名單」併入店家封鎖，分頁改名「封鎖名單」或拿掉；店家封鎖後訂單相關聯絡是否例外在這裡設計。完成時刪 `pkg/blocklist`、`pkg/moderation`。
  - A6 自己的橋（不是 S3 造成的）：review relay（給 Java order、product 與 notification）、crm 的 `order.completed`／`aftersales.refund.completed` consumer、coupon legacy client、orderservice 補 3 支方法；刪 legacy `reviewservice`、`crmservice` client（決定簡報 §1.2）。
  - IM 的即時通道依 ADR `2026-09-26-external-contract-protobuf-connect.md` D7 決定：預設是 server streaming 收訊息、unary 送出；需要瀏覽器上的雙向即時而保留 WebSocket 時，明列為第二套契約。扇出和 A7 拿掉的留言即時推一起設計（A7-D10）。
- [ ] **A7 social**（2026-10-17 提前，排在 A6 之前；原本排在 A6 之後只是計畫分段）：步驟、決定與進度都在階段計畫 `2026-10-18-a7-social.md`，本段只記結論。決定：A7-D1 到 A7-D5′ 見 `2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md`（Will 2026-10-17）；A7-D6 到 A7-D13、S1、S3、Q1–Q6 見 `2026-10-18-a7-social/decision-brief.md`（Will 2026-10-18）。規模：社群 REST 40 條（brief 口徑 42 含 2 條商品 hashtag）加代行貼文 6 條、RPC 34 支、表 10 張、事件 14 列；約 15.5 session【推】。
  - 新模組 social 與 moderation；封鎖在 buyer（個人）與 seller（店家）；店家內容被平台下架是 seller 的新措施種類「內容下架」；處分紀錄歸執行效果的模組（買家的社群處分在 social）；後台待辦加 moderation 的 `pub_*` 來源。
  - 排程發布歸 social；delegation 撤銷 session 時在同一個 transaction 取消，並以同步訂閱者寫稽核（A7-D6 選 D）。代行貼文 6 條路由在 A7 改走單體 Connect（S1），legacy SocialService 不寫，`api/proto/social`、`pkg/proto/social` 在 A7 收尾刪除。
  - 留言不即時推（A7-D10 不做），gateway 的留言 SSE 刪除；瀏覽數含匿名（每日換 salt 的雜湊去重）；計數同 transaction。
  - 店主 Connect Get（與 List 回同一個 Post 訊息），並刪除 `GET /app/sellers/posts/:id` 與 ADR `2026-09-26-external-contract-protobuf-connect.md` 文末修訂登記的 D5 例外（A3b 步驟 10d 第 3a-① 刀新增）。
  - 讚與留言的互動者身分：照 ADR `2026-10-17-social-actor-identity.md`（M1 (kind, id)、S2、I3）；`actor_id` 隨 buyer、store 換 uuid 時換型別。
  - content 的 banner 點擊行為（`post`）改成經 social 的公開 API 確認貼文存在，ID 格式同時換成 UUID：依賴表的 content 加上 social，並更新 ADR D2 的表（content 模組 `CARRYOVER.md` 的沿用盤點；現在只檢查非空）。
  - 賣家中心的動態數徽章改由伺服器提供數量：website 現在只從前 50 筆貼文算（`useSellerPosts`），超過就少算（A3 計畫第 4C 刀的決定）。
- [ ] **A8 notification**：依 ADR `2026-10-05-notification-module.md`：通知型別定義在程式碼、逐一明確訂閱，不照搬 wildcard。
  - 發票寄送的買家 email 來源：單體 buyer 沒有 email，email 只在 identity（`accounts.email`）；A8 定 identity 的 email 同步時，一併決定發票（invoice-service）寄送地址取自 identity 的哪個欄位與介面（設計書 58 U5，Will 2026-10-28）。
  - 通知的維運重送：舊的 outbox 手動補列已隨 outbox 刪除，設計書 56 的 runbook 已無重送步驟；A8 設計投遞紀錄時一併定義重送手段（例如重排永久失敗的投遞紀錄）與 runbook（Will 2026-10-28）。
  - identity 接 Clerk 的 `user.updated` webhook（驗證簽章）同步已驗證的主要 email，作為 email 渠道的寄件地址；不在寄信時呼叫 Clerk Backend API（ADR `2026-09-27-identity-clerk.md` 文末〈修訂：email 作為通知的寄件地址〉）。完成時 `docs/modules/identity.md` 的 email 從「建帳時的快照」改成同步後的規格。
  - `notifications/stream` 的 SSE 依 D7 改成 server streaming，續傳用游標欄位取代 `Last-Event-ID`，保留每個帳號的串流上限。
  - 治理的通知（A3b `step10e-reports-brief.md` R4、R7、R9，Will 2026-10-18）：訂閱 seller 的 `ReportReceived`、`ReportResolved`（通報人：登入的用站內通知、匿名但留了 email 的用 email，確認信依收件 email 節流），以及在 seller 加上詢問的事件並訂閱（賣家；3c-b 預先發出的 `CaseInquiryCreated` 因沒有消費者已刪除，A8 依「消費者出現時再加事件」在這一刀加）；食安事件通知某個商品過去 6 個月的購買者，前提是查得到購買者（B8）。
  - **確認信地址不得留在 job 參數裡**（A3b 3c-a 設計審查 #10，Will 2026-10-18）：`ReportReceived.ConfirmationEmail`（匿名送出留的 email）只在事件與 A8 同一個 transaction 排入的 job 參數裡，seller 不落表（`seller/events.go` 的註解寫同一條）。River v0.47.0（`backend/go.mod`）預設保留完成的 job 24 小時、丟棄（寄送失敗用盡重試）的 job 7 天（`rivershared/riversharedmaintenance/river_shared_maintenance.go:35-37`；專案沒有覆寫 `*JobRetentionPeriod`），參數裡的地址會跟著留那麼久。所以寄送 job 必須在完成或丟棄時清掉參數裡的地址，或參數只放 report／supplement 的參照、執行時再讀（地址就落在 seller，回到 365 天的聯絡方式保留規則管得到的範圍）。觸發：A8 訂閱 `ReportReceived` 的那一刀；驗收要有測試證明 job 完成與丟棄之後參數不含地址。
- [ ] **A9 agent**
  - AI 助理與 procurement 的串流（AG-UI，12 支 SSE）：依 ADR `2026-09-26-external-contract-protobuf-connect.md` D7 在這一步決定改成 proto 的 server streaming，或保留 AG-UI 的 SSE 並明列為第二套契約（AG-UI 是外部的開放格式）。
- [ ] **A10 delegation**：從 gateway 搬出代行編排；session 與稽核資料留在 identity。
  - 代行寫入搬進單體時，在領域寫入的同一個 transaction 以 `FOR SHARE` 讀 session 列檢查狀態，再寫入與寫稽核（撤銷與寫入序列化）。理由：撤銷以 session 為準，gateway 與領域服務不同 DB 時無法保證撤銷前後的順序，同 DB 後成本只是一次加鎖讀取。
  - 代行貼文的 6 條路由排在 A7-2d 改走單體 Connect（A7 計畫 S1），A10 只剩商品與訂單的路由；A7-2d 完成後，代行 Connect 的第一版形狀（credential、能力、`request_id`、限流）記在 ADR `2026-09-27-delegation-module.md` 的修訂段，A10 沿用或修訂。
  - 代行改店面（gateway 店面四條路由目前固定回 503）：設計書 56 §6.1.7 與 §2.2 第 2 列附近仍描述 gateway 呼叫 seller-service `UpdateSeller`；A10 依 seller 模組的欄位與限制重新設計店面代行，並改寫這些段落（Will 2026-10-28）。
  - 代行營運有 24 條路由不在現有 API 清單裡（gateway 的 `routesNotInSpec`，另有 binary 代理 9 條等），搬遷時以路由表為準，不能只看 `openapi.yaml`。
  - website 的代行 client（`lib/api/delegated.ts`、`app/(delegated)/adm/api/_upstream.ts`）還送 `X-Requested-With`，理由是 gateway 的 CSRF guard；A2 之後 gateway 沒有這道檢查，入口的 CSRF 看 `Sec-Fetch-Site`／Origin。它在 56 號設計書的 header 允許清單裡，改 client 時一起改清單。
- [ ] **A11 discovery**：從 seller 搬出推薦賣家排行。

每一步還要處理兩件事：
- 這個服務原本提供給 Java 呼叫方的 gRPC server，移進單體的 `internal/legacy/`。
- 這個服務原本消費的 Java 事件，改由 legacy 的 RabbitMQ adapter 接收。

另外，服務原本的樂觀鎖（seller、social、delegation 的 `expectedVersion` 與各領域的 `*_VERSION_CONFLICT` 碼；invoice 在 A5）改成 ADR `2026-09-25-optimistic-concurrency-etag.md` 的做法，列進該模組的沿用盤點。

**階段驗收**

- 12 支 Go 服務的目錄都已刪除；入口是單體，gateway 只剩還沒改寫的 Java 模組的 REST 路由（階段 B 逐一刪除）。
- 單體對 Java 的所有依賴都只經過 `internal/legacy/`。
- 舊的 Go CI workflow 已併入 `backend-monolith`。

## 階段 B：Java 改寫成 Go

每個模組開工前，先完成兩件事：
1. 這個模組對外 endpoint 的特性測試，已經以 REST driver 經舊 gateway 對 Java 跑過並通過；改寫後以 Connect driver 對單體跑同一份情境（ADR `2026-09-26-external-contract-protobuf-connect.md` D4）。
2. 列出這個 Java 服務的 integration 測試對照表。

順序依 ADR D8，由下而上：

- [ ] **B1 media**
  - media 改成不認識業務（ADR `2026-10-03-media-references-owner-side.md`，Will 2026-10-03）：只存資產與生命週期（暫存→確認→刪除），引用只在擁有者的表；刪除是擁有者經 outbox 送出的明確指令，GC 只清過期沒確認的暫存檔；機敏下載由擁有者授權後向 media 要短效的簽章網址；拿掉 `asset_reference` 與 media 端的 `(ref_type, ref_id)`。B1 之前的做法（每個 ref 只有一個同步 job 寫引用、只精準解除退場的圖）在 B1 時換成確認／刪除的介面。
- [ ] **B2 payment**：物流財務與責任判定先留在 Java，B10 再處理。
  - `createRefund` 依 D5 修正。
- [ ] **B3 fulfillment**
  - 刪除 Uber 即時配送（ADR `2026-10-27-general-ecommerce-scope-cut.md` D4）：Java shipping 的 Uber 程式、payuni-webhook-service 的 Uber webhook、只服務它們的 proto RPC 與生成物、CodeMeta 的 Uber 值域、`.env.template` 與 `docker-compose-local.yml` 的 Uber 設定、`docs/runbooks/uber-quote-retention/`，4 份 Uber ADR 標 superseded；設計書 03、06、09、58 描述 3003（Uber）的段落與 `docs/runbooks/uber-quote-retention.md` 一併刪除；清單見 `docs/plans/2026-10-27-ecommerce-scope-cut.md`「延後」（Will 2026-10-29）。
- [ ] **B4 catalog**
  - 不建 `store_state` 讀取模型：catalog 的列表在 SQL 內 join `seller.pub_exposable_stores(at)`（時間用請求的 clock），取代停權快照（ADR 2026-09-24 修訂段，2026-10-18；簡報 `docs/plans/2026-10-18-a7-social/d3-read-model-brief.md` §4.2）。
    - 2026-10-19：`seller.pub_exposable_stores(at)` 已建（A′(i)）。B4 把 product 搬進單體、`rg -n "OnStoreExposureChanged" cmd` 歸零時一起刪除 `seller.store_exposures`、`StoreExposureChanged`、`seller.reconcile_exposure` 與 `make reannounce-exposures`，`ListPublicStores` 改以 `pub_exposable_stores($now)` 做 keyset，`first_exposed_at` 改由週期 job 讀 `pub_exposable_stores(now)` 寫入（seller `CARRYOVER.md`「A′(i)」）。
  - 銷量、評分、收藏數歸 catalog，由事實發生的模組呼叫 API 更新。
  - content 的 banner 點擊行為（`product`）改成經 catalog 的公開 API 確認商品存在，ID 格式同時換成 UUID：依賴表的 content 加上 catalog，並更新 ADR D2 的表（同上）。
  - 賣家中心的低庫存徽章改由伺服器提供數量：website 現在只從前 100 筆商品算（`SellerCenterShell`），超過就少算（A3 計畫第 4C 刀的決定）。
  - 商品限制措施（A3b `step10e-reports-brief.md` R6、`step10e-measure-brief.md` D7，Will 2026-10-18）：seller 的措施對象加商品（`measure_contents.content_kind` 加 PRODUCT），有理由、可以申覆、擋住重新上架；admin 的強制下架對話框改成建立這個措施；主管機關來函指名的商品頁資料依保存期限保存（R5，食安法修正草案第 43-1 條）。第一次部署前一定要有。
- [ ] **B5 productimport**
- [ ] **B6 cart**
- [ ] **B7 coupon**
- [ ] **B8 ordering**
  - checkout 在單一 transaction 內完成：鎖購物車、扣庫存、建單、鎖定優惠券。
  - 發票快照移交給 invoice。
  - 訂單上的物流投影欄與付款副本刪除。
  - 訂單號重新設計，不照搬 `OrderNoGenerator`：`"O" + TSID % 10^9` 加上 `UNIQUE(order_no)`，約 3.7 萬筆訂單就有一半機率撞號（生日界）。訂單號是給人看的編號，另開 `UNIQUE` 欄，不由 ID 截斷或取模（PostgreSQL 設計規範 §二）。
  - agent 的「接受報價 → 建單」改在同一個 transaction 內完成，並標記 RFQ ADR 為 superseded。
- [ ] **B9 aftersales**
  - 取消、退款任務、優惠券補償在同一個 transaction 內完成。
  - 設定「進行中售後」旗標。
  - `refund_tasks`、`auto_refund_tasks`、`payment_refunds` 收斂成一張退款命令表。
- [ ] **B10 finance**
  - 賣家撥款從 ordering 移入。
  - 物流財務與責任判定從 Java payment-service 移入，Java payment-service 就此退場。

**階段驗收**

- `services/java/` 已刪除，Java 的 CI workflow 也已刪除。
- 205 個 Java integration 測試全數對應到 Go 測試，或已標記退場理由。

## 階段 C：收尾

- [ ] 過渡期用 `bigint GENERATED ALWAYS AS IDENTITY` 當主鍵的模組（identity、buyer、seller 等），在最後一個 Java 消費者改寫後以一個 migration 換成 uuid（ADR `2026-09-25-row-identity-by-data-kind.md` D1）。必須在後端 CD（第 4 步）之前完成：上線前這個 migration 沒有資料成本，上線後連號 ID 就對外可列舉。
- [ ] 刪除 `internal/legacy/`、`api/proto/`、`pkg/proto/`、`services/java/shared/java-proto`，以及內部 gRPC 的 buf 設定（`buf.gen.yaml`、`buf.gen.legacy.yaml`）與 gRPC 相關依賴。對外契約的 buf（根目錄 `api/`、`buf.gen.connect.yaml`、各消費端的 `buf.gen.yaml`）保留。
- [ ] 前端：最後一個模組搬完時，刪除兩支 App 的 Dio 與手寫的 transport、web 的手寫 REST client；前端的 API base URL 與設定通過 D9 gate。
- [ ] 最後一個模組搬完時刪除 gateway 目錄，連同現有 API 清單（`api/openapi/openapi.yaml`）、gateway JSON contract gate 與路由覆蓋測試。
- [ ] 刪除 `infra/mq/`（含 Terraform topology）、`pkg/mq`、compose 裡的 RabbitMQ。
- [ ] 刪除 `pkg/`（剩下的內容此時應已全部歸入 platform 或各模組）。若 `go.work` 只剩單體，也一併刪除。
- [ ] CI：刪除 proto 生成檔比對、Java、以 module 為單位的 integration matrix；notification 模板契約測試改成每個通知型別的比對測試（ADR `2026-10-05-notification-module.md`）。
- [ ] 對外 API 的 package 從 `v1alpha1` 一次改名成 `v1`（ADR `2026-09-26-external-contract-protobuf-connect.md` D2）：proto 的 package 與目錄、單體的 `connectServices`、四端的生成物與引用；改名之後 `buf breaking`（`WIRE_JSON`）才開始擋破壞性變更。必須在後端 CD（第 4 步）第一次部署之前完成：上線後改名本身就是破壞性變更，也會換掉 CDN 的快取鍵。
- [ ] 各模組上線前累積的 migration 壓成一個 baseline（Will 2026-09-25 決定）：系統還沒上線，已套用的只有本機與測試的 database，壓掉沒有成本；第一次部署之後才開始累積。必須在後端 CD（第 4 步）第一次部署之前完成。
  - identity 的 `20260927071536_clerk_identity.sql` 為了既有列把 `accounts.clerk_user_id` 與 `admins.clerk_user_id`（之後改名為 `access_subject`）填成 `unlinked:<id>`：壓成 baseline 時這兩句一起消失，兩個欄位直接是 NOT NULL，沒有佔位值（A2 C9 的 design-review）。
- [ ] CodeMeta 生成器（`docs/代碼管理（CodeMeta）/scripts/generate.sh`）的 Go 輸出改成先寫出、再交給 `gofmt`，不再自己計算對齊（Will 2026-09-25 決定）：自算對齊是為了在沒有 Go 的 runner 上重跑生成比對，monorepo 的 CI 跑生成器時都有 Go。
- [ ] 刪除 CodeMeta：沒有任何值域的擁有者是 CodeMeta、也沒有舊的消費端時，刪除 `docs/代碼管理（CodeMeta）/`、生成器、`internal/platform/codemeta`、`pkg/codemeta`、`pkg/errors/bizcode`、gateway 的鏡像與各 client 的 CodeMeta 生成物（ADR `2026-09-25-codemeta-retirement.md` 過渡第 4 步）。
- [ ] 賣家入駐文件改版時的重新同意（`pending_agreements`、`AcceptSellerAgreements`）：上線之前，或版本第一次在有真實賣家之後改版時加上，處理已入駐的賣家與換申請主體的情況（ADR `2026-10-19-seller-kyc-payuni-consolidated.md`「入駐同意紀錄」）。
- [ ] 賣家入駐文件上線前的條文工作（ADR `2026-10-19-seller-kyc-payuni-consolidated.md`「入駐同意紀錄」）：
  - 律師確認三份條文（賣家契約、KYC 個資蒐集告知、PayUni 條款的取得方式）。
  - 填入營運主體、聯絡信箱、平台網址等佔位符（`〔營運者名稱〕`、`〔聯絡信箱〕`、`〔平台網址〕` 等）：這是條文改變，正本的版本行要遞增，再跑 `pnpm gen:legal`。
  - 依代理商契約更正 PAYUNI_TERMS：取得條款正本存進 `website/docs/legal/`，manifest 改用 `doc`，版本改用正本的日期。
- [ ] 文件：
  - MQ 事件總覽（36 號）、Proto 管理規範封存。
  - 設計書與模組的對應寫進各模組的 `AGENTS.md`。

**驗收**

- repo 裡不再有 `.proto`、`pom.xml` 和 RabbitMQ 設定。
- backend 的 D9 baseline 為 0。
- 單體的實體表沒有 IDENTITY 主鍵。

## 階段 D：前端改用生成的 client（2026-09-26 取消）

對外契約改為 Protobuf＋Connect 後，前端改用生成的 client 併進「模組完成的定義」（每個模組搬遷時一起做），收尾（刪 Dio 與手寫的 client、D9）移到階段 C。

## 進度

| 階段 | 狀態 | 備註 |
|---|---|---|
| 0 地基 | 進行中 | 已完成：0.1；0.2 的核心機制；0.3 的依賴方向、migration 與 query 隔離檢查；0.4 的 migration 套用器與 Atlas 開發設定；0.6 的 OpenAPI 規格與生成工具（2026-09-26 由 Connect 取代，規格改為現有 API 清單）；0.7 的 D9 gate；0.8 的盤點 gate；0.9 的本機環境；0.10 的指令檔；0.11 的 `backend-monolith` workflow（含 integration 測試）；0.5 的模組模板與 scaffold（2026-09-27，從 content 抽出）。單體統一使用 Postgres 18。前置修正：CodeMeta 的 Go 輸出從 `docs/` 移到 `internal/platform/codemeta`，因為中文目錄名不是合法的 import path，會讓根 module 無法建置 |
| A Go 併入 | 進行中 | 對外契約定案為 Connect（2026-09-26），A0 重新定義為 gateway 當過渡入口並已完成；A1 content 完成（2026-09-27）：content-service 已刪除，模板見 0.5。A2 identity 完成（2026-09-29，`2026-09-27-a2-identity.md`）。A3 buyer、seller 的決定完成（2026-09-29），下一步是步驟 1（`2026-09-29-a3-buyer-seller.md`） |
| B Java 改寫 | 未開始 | |
| C 收尾 | 未開始 | |
| D 前端 client | 取消（2026-09-26） | 併進模組完成的定義，收尾在階段 C |
