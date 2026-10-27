# 模組內部的結構：根套件只當門面，實作依子領域放進 internal/

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-27 |
| 影響範圍 | 單體的每個模組（`internal/modules/<名稱>`）、`cmd/new-module` 的模板、`internal/arch` |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D2（模組只能 import 對方的根套件） |

## 脈絡

模組之間的邊界已經由依賴表、Go 的 `internal/` 與 schema 隔離強制，模組內部的結構沒有規範。A1、A2 的做法是把 use case 平鋪在根套件：每個 use case 在同一個函式裡開 transaction、呼叫 sqlc 的 `store`、排 job。identity 搬完後根套件有 15 個程式檔、6,285 行（含測試），帳號、認證、Apple 撤銷、代行混在一起。

根套件同時是「公開 API」與「實作」：規範說其他模組只能 import 根套件，但根套件裡所有 exported 的東西都可以被呼叫，哪些是刻意公開的只能靠 review 判斷。還有約 20 個模組要搬，平鋪會隨模組變大而惡化。

約束：

- 繼承：模組只能 import 對方的根套件（上層 ADR D2），由 `internal/arch` 的測試強制；仍成立。
- 繼承：transaction 由最上層的 use case 以 `db.InTx` 開，repository 用 `db.Conn(ctx)`（backend `AGENTS.md` 單體規則 3）；仍成立。
- 外部：Go 不允許 import 循環；根套件 import `internal/…` 之後，`internal/…` 不能 import 根套件。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：依功能（子領域）分套件，公開 API 與實作分開（modular monolith 的 package by feature；Spring Modulith 的 API／internal 分離，Kamil Grzybek 的 modular monolith 範例） | 對外型別要住在葉節點套件、根套件以別名公開；根套件的方法多一行轉呼叫；模組內要維護一張套件依賴圖 | 採用 |
| 維持平鋪（現況） | 公開 API 與實作混在一起，邊界無法機械檢查；大模組的根套件持續變大 | 否決 |
| 模組內依技術分層（usecase／service／repository／entities，clean architecture 的分層版） | 一個功能散在四、五個套件；sqlc 已經是有型別的查詢，再包 repository 介面多半只是轉呼叫；po／bo 互轉增加樣板；service 與 usecase 的分界要靠判斷 | 否決 |
| 每個模組都分 domain／app／adapters／ports（hexagonal） | 簡單模組的樣板比邏輯多；Go 社群普遍反對沒有理由的分層套件 | 否決（複雜規則以「抽純邏輯」處理，見決策 3） |

## 決策

1. **根套件只當門面**：`Module`、`New`（組裝）、對外的方法（只轉呼叫 `internal/`）、對外型別的別名、事件、錯誤。根套件不得 import `internal/store`，SQL 一律在 `internal/` 裡。
2. **實作依子領域放進 `internal/<子領域>/`**，例如 identity 的 `account`、`auth`、`applerevoke`、`delegation`。對外型別與事件定義在葉節點 `internal/model`，根套件以 `type X = model.X` 公開。`store`（sqlc）、外部整合（`clerk`、`apple`…）與 `model` 是葉節點。
3. **use case 預設是 transaction script**（一個函式：開 transaction、呼叫 store、排 job 或發事件）。有狀態機或複雜不變量的子領域，把規則抽成不碰 I/O 的純邏輯（例如代行的 `rules.go`），use case 只負責載入、呼叫規則、儲存。
4. **use case 的輸入**：參數超過兩個時用具名的 struct（`SuspendInput`），一兩個參數維持一般參數。
5. **模組內的套件依賴圖明確寫出並由測試強制**（同 `internal/arch` 的做法），不准反向或循環；子領域之間需要的東西以介面或 `model` 的型別傳遞。
6. **一律適用**：規則簡單、只有一個子領域的模組（content、media）同樣採用，結構一致比省一層轉呼叫重要。
7. 測試：整合測試留在根套件、只經公開 API（黑箱）；純邏輯的單元測試放在各自的套件；子領域內部的函式不直接測。

## 後果

- content、media、identity 依此重構；`make new-module` 的模板改成產生這個結構；`internal/arch` 加上「根套件不得 import `internal/store`」與模組內依賴圖的檢查。
- 代價：每個對外型別一行別名、每個對外方法一行轉呼叫。
- 代行留在 identity 或拆成獨立模組另外決定（依賴表已有 `delegation`，與 A2 的做法不一致）；拆出去時直接搬 `internal/delegation`。
- 重新評估的條件：別名與轉呼叫的樣板明顯超過實作本身（多數模組都只有一個 use case）；或 Go 出現讓公開 API 與實作分離更便宜的機制。

## 修訂：否決 repository 介面的理由與重新評估條件（2026-10-21，Will）

原決策段保留歷史。決策不變：模組內不加 repository 介面，use case 直接用 sqlc 的 `store`；測試是經根套件的黑箱整合測試（真 Postgres）加純邏輯的單元測試。本段只換掉否決理由，並補上這個否決自己的重新評估條件（原「重新評估的條件」只涵蓋門面的樣板）。

**原否決理由的重新檢驗**（2026-10-21 的獨立審查與對抗性審查，對照當時的程式碼）：

| 原理由 | 判定 | 依據 |
|---|---|---|
| 一個功能散在四、五個套件 | 不成立 | 現行結構的 feat commit 也動到 5–6 個套件（handler、根套件、子領域、`model`、`store`、`db/queries`），與 go-clean-arch v4 相差頂多一個 |
| sqlc 已經是有型別的查詢，再包 repository 介面多半只是轉呼叫 | 成立 | 抽查 13 個 use case，7 個是驗證後呼叫一支 sqlc 方法；其餘 6 個有真正的邏輯，但每次資料存取仍是一支 sqlc 方法，repository 只會一對一轉呼叫 |
| po／bo 互轉增加樣板 | 不成立 | 決策 2 的門面規則本來就要把 sqlc 的列轉成 `model` 型別（約 34 個轉換函式），加 repository 只是把轉換搬到別處 |
| service 與 usecase 的分界要靠判斷 | 減弱 | 只適用四層的版本；go-clean-arch v4 已合成一個 `article/service.go` |

**決定性的理由（原文沒寫）**：這個系統的正確性有很大一部分在 SQL 與 transaction 裡。例如：
- 鎖順序：模組 `.sql` 有 36 處 `FOR UPDATE`，對應單體 ADR 的 D4(c)。
- 約束：CHECK 約束與 `ON CONFLICT`。
- 規則寫在資料庫裡：`pub_*` 與審閱政策函式。
- 執行計畫測試：`*_plan_integration_test.go`。
- transaction 跟著 `ctx` 走：`internal/platform/db/db.go` 的 `Conn`、`InTx`。

mock 的 repository 只能驗「呼叫了哪個方法」，驗不到這些，而這些正是主要風險。

**基準**：
- Microsoft EF Core 的測試指南（learn.microsoft.com/ef/core/testing/choosing-a-testing-strategy）建議先對正式使用的資料庫測，需要替身時才用 repository 模式，並提醒它成本高、包住的查詢仍要對真資料庫測。
- Go Code Review Comments（go.dev/wiki/CodeReviewComments「Interfaces」）規定介面放在使用端，不在實作端「為了 mock」定義，也不在有實際用途前定義。
- Software Engineering at Google 第 13 章：真實實作夠快、可重現時優先用；太慢時用 fake；mock 放最後。

**需要替身時的做法**：
- 在使用端定義窄介面，優先用 fake（有真實行為的替身）。
- 資料層真的需要介面時，用 sqlc 的 `emit_interface: true` 生成，不手寫 repository。
- 現有的使用端介面（Clerk、Apple、`Orders` 等外部系統與 port）就是這個形狀。

**重新評估的條件**（任一成立就重新評估，只在成立的地方加介面）：
- 真資料庫測試的回饋卡住開發：`testdb` 加速後，單一套件在容器與 template 都已存在時仍超過約 10 秒。
- 某個模組需要第二種儲存實作，或要換掉 sqlc。
- 整合測試打不到、需要故障注入的錯誤分支持續累積。
- 出現不碰 SQL、純粹由呼叫順序決定的編排（例如串接多個外部系統的流程）：在那個子領域加窄介面與 fake。

## 修訂：共用可見性述詞的讀取改用 go-jet，sqlc 仍是預設（2026-10-23，Will；A7-1a）

原決策段與上一個修訂保留。上面「重新評估的條件」第二項「要換掉 sqlc」在 social 成立了一部分，本段記結果：**sqlc 仍是預設**；一個模組的讀取在多支 query 共用同一個可見性或資格述詞時，那些讀取改用 go-jet 建在模組自己的 `internal/visibility` 上。依據：`docs/plans/2026-10-18-a7-social/step-1a-builder-baseline.md`（業界做法、spike 量測、約束分類）。

**脈絡**：social 有 24 支 query（公開讀取、佔位項目、標籤建議與熱門、重算）帶同一組規則（貼文已發布且店家可曝光、留言沒有刪除也沒有隱藏、買家作者沒有停用）。sqlc 沒有片段或巨集（sqlc #2244 自 2023-04 未解），規則只能在每支 query 重寫並用「次數」測試數、或推進資料庫物件。資料庫物件的路被單體規則擋住：view 與 `BEGIN ATOMIC` 函式在建立時就綁定，不能參照別的模組的 `pub_*`，而 migration 要能單獨套用在空 database 上（`TestEveryModuleStaysInsideItsOwnSchema`）。業界的做法（Mattermost、Gitea、Grafana、Lemmy、Discourse 等）是把可見性規則做成應用層 query 層的可組合值，單一入口加測試保證完整，沒有一個把「每個觀看者的可見性」放進資料庫 view 或函式。

**決策**：
1. sqlc 是預設，靜態的 query（寫入、鎖、清除、媒體引用、作者與店主自己的讀取）照舊。
2. 共用述詞的讀取用 go-jet 在 `internal/visibility` 的述詞上建（`ReadablePosts`、`Comments`／`ForumEntries` 的 `Counted`／`Viewable`），在 `db.Conn(ctx)` 上執行，以 pgx 的 `RowToStructByName` 掃描（jet 的 QRM 需要 `database/sql`，不用）。選 go-jet 不選 bob：運算式有型別、改欄位名稱在編譯期就壞、生成器只看你給的 schema（bob 的生成器對著 live DB 型別化，不擋跨 schema）、採用面較大，bob 的 `.With().All()` 在 v0.50.0 有掉參數的 bug。
3. 保住 sqlc 原本給的保證，改由測試給：邊界（`internal/platform/querycheck` 對渲染出來的 SQL 以 pg_query 解析，取代 sqlc 的 schema 清單）、結果形狀（對測試資料庫 Describe，不需要資料，比對 row struct）、golden SQL 快照（review 看得到 SQL 的差異）、單一入口型別加 AST 測試（取代 `readpolicy_test.go` 數條件次數）、原始 SQL 的逃生口只許出現在由擁有者 `pub.sql` 生成的包裝。
4. 「計入」只有一個定義，在 Go：`internal/counted` 把它生成成 plpgsql 函式 `reevaluate_*`（給 trigger、外部事實改變的 job、每日對帳用），golden 測試比對；讀取與重算直接內聯同一個 Go 述詞。reevaluate 不能被讀取當作逐列函式呼叫（量測 8,676 ms 對內聯的 53 ms）。
5. 生成的 plpgsql 函式是單體規則 6 的第二個例外（`migrate.Module.Generated`）。

**約束的分類**：「sqlc 是唯一的 query 層」是自己訂的（`AGENTS.md`「單體的結構」與本 ADR 原決策），不是外部約束；真約束是 sqlc 沒有片段、Postgres 對 view 與 `BEGIN ATOMIC` 的建立時綁定、migration 單獨套用、generic plan 無法用 `$1` 證明 partial index 的述詞（固定代碼因此一律內聯成常數，`TestRuleCodesAreInlineConstants`）。

**成本與重新評估**：go-jet 與 pg_query_go（cgo）進 `backend/go.mod`，生成需要 Docker（與 sqlc 同）。重新評估條件：go-jet 12 個月沒有新版本或與 pgx、PG 版本不相容 → 同一套述詞改用 bob 的 builder；公開讀取在正式資料量下 p95 超過 300 ms → 讀取模型（outbox 投影）；模組拆庫 → 跨 schema 的 `pub_*` join 變成 API 呼叫或投影；sqlc 出片段或巨集 → 評估把 builder 的 query 併回 sqlc。

## 修訂：「計入」改成 social 自己的表加一組 `BEGIN ATOMIC` 函式，不再生成 plpgsql（2026-10-23，Will 選 D-sync；A7-1a 最終審查）

上一則修訂的決策 4、5（`internal/counted` 生成 plpgsql 函式、規則 6 的第二個例外 `migrate.Module.Generated`）被取代。讀取改用 go-jet 建在 `internal/visibility` 上（決策 1 至 3）不變。

- **新做法**：「計入」的輸入全部屬於 social：內容的 `deleted_at`、`hides`，以及買家刪帳的事實在 social 的複本 `social.inactive_authors`（同 transaction 的訂閱者寫入，見 `2026-10-23-social-counts-in-stats-tables.md` 修訂二）。求值寫成 `LANGUAGE sql BEGIN ATOMIC` 的 `reevaluate_comments`、`reevaluate_forum_entries`（只參照 social 自己的表，migration 單獨套用在空 database 上成功），結果存成內容列的旗標；讀取直接用旗標，重算是旗標的加總。Go 不再有「計入」的述詞，`visibility.Counted` 只是這個旗標。
- **約束的重新分類**：「migration 單獨套用在空 database 上必須成功」原本寫成 REAL，理由是它保證模組可獨立搬動。查過之後：拆庫已不是目標（`2026-09-24-go-modular-monolith.md` 撤回 view 否決時的前提），這條規則是**自己訂的、為了在不拆庫時仍然機械地守住模組邊界**；本次沒有推翻它，而是讓資料模型滿足它（事實複本），所以它現在在執行期也成立：social 的函式不依賴任何別的模組的物件。需要例外時應先問資料放對擁有者沒有，不是加豁免。
- **生成器與例外刪除**：`internal/counted`、生成區塊與 golden 測試、`migrate.Module.Generated` 與規則 6 的第二個例外全部刪除；規則 6 只剩 `RETURNS trigger` 一個 plpgsql 的例外。
- **重新評估**：若 `reevaluate_*` 改成不存旗標（讀取時推導），複本與同步寫入的必要條件消失，回到上一則修訂以前的取捨。
