# 跨模組讀取與跨模組寫入的治本做法（ADR 2026-09-24 D3、D4 的前提重審）v3

> 快照：monorepo `06b158957`。cwd 為 `backend/`，除非另註。
> 標記：【量】附指令或實驗、【推】推論、【猜】猜測、【外】外部來源已查證（附 URL）、【外?】外部來源沒有查證到。
> 取代 v1（`docs/plans/2026-10-18-a7-social/d3-read-model-brief.md`）與 v2。v2 的對抗式審查（2 HIGH、5 MED、4 LOW）逐條處理，見文末「審查處理」。
> 本版的實驗都在 scratchpad 內做：暫時的 Postgres 18.6（`initdb`，port 54999）、sqlc 1.31.1、Atlas v1.1.3 canary。指令與 SQL 摘要在附錄 B，暫存檔已刪。

## 結論（先讀這段）

1. **D3 的「每模組一個 schema、只有擁有者寫入、不建跨 schema FK」符合業界基準，保留。** D3 第 3 點把「不能讀別人的內部表」推成「跨模組篩選只能靠事件同步更新的副本」。這一步的理由（view 是拆庫期間的過渡做法、模組要能單獨搬移）以拆庫為前提，本專案沒有拆庫的目標（ADR D1）。
2. **跨模組篩選改用擁有者發布的唯讀 SQL 介面**：`<owner>.pub_*` view，需要「讀取當下的時間」時用 `pub_*(at)` SQL 函式。依賴表允許的上層模組可以在 SQL 裡 join 它。讀取模型降為效能手段，量測觸發才建，建的時候用 outbox 非同步投影。
3. **F1 的結論：曝光（exposure）不能只包一層 view 在 `seller.store_exposures` 外面。** 那張表存的是「最後公告的值」，不是「現在可不可以曝光」。建議做法是把曝光規則的每個述詞都搬進 seller 擁有的 SQL（`seller.pub_exposable_stores(at)`，時間由請求的 clock 傳入）。Go 的 `Classify`、`CapabilitiesOf` 改讀同一份 SQL，所有消費端在同一刻得到同一個答案。這要改 seller，seller 目前歸 10e session，約 1–1.5 個 session【猜】，必須排在 A7-1a 之前。
4. **強制機制**：擁有者提供 `db/pub.sql` 契約檔，只宣告 `pub_*` 的形狀；消費端的 sqlc 只讀這一份，所以 join 內部表在生成時就失敗【量】。另加一個整合測試，比對 `pub.sql` 與真實物件的欄位，它同時是消費者契約測試。不需要 `pg_query_go`。
5. **D4 保留，收窄三處**：
   - (a) 同 transaction 訂閱者只做兩種事：只寫自己的列、而且無法在讀取時推導的原子狀態變更；或者只排入 job、寫 inbox。
   - (b) 可以被別的模組呼叫的公開方法不得用 `InReadSnapshot`，撞唯一鍵後也不得在同一個 transaction 繼續下 SQL。`InTx` 可以重入，118 個呼叫點都不用改【量】。
   - (c) 全域鎖順序**由下往上**：identity 帳號列（錨點）→ buyer → seller → 上層模組 → 編排者自己的列。這和 accountdeletion 的現有順序一致，取代 A7 計畫 A7-1c (b)。
6. **事件的發布方式不變**（§6）。`ReportResolved` 已經存在。

---

## 0. 前提題：被改的規則與程式之後會不會被重寫、搬走或刪除？

### 術語

| 術語 | 本文的意思 |
|---|---|
| 內部表 | 模組 `schema.sql` 裡的表，只有擁有模組讀寫 |
| 發布的讀取介面（`pub_*`） | 擁有者在自己的 schema 定義、明文承諾形狀的唯讀 view 或 SQL 函式，等於 Go 公開 API 的 SQL 版 |
| `pub.sql` | 擁有者的契約檔，只宣告 `pub_*` 的欄位形狀，給消費端的 sqlc 讀 |
| 讀取模型 | 消費模組在自己 schema 裡保存的、別的模組資料的副本 |
| composition | 呼叫多個擁有者的 API，在記憶體依同一個全序合併 |
| 同 transaction 事件 | `events.Publish`：handler 在發布方的 transaction 內同步執行，失敗就 rollback 發布方 |
| outbox | 在業務 transaction 內 `Enqueue`（River `InsertTx`），commit 後由 worker 執行 |

### 量化

| 對象 | 現況 | 排程 | 判定 |
|---|---|---|---|
| 模組 | 規劃 24 個；已建 9 個，8 個有 `db/`【量】`ls internal/modules/*/db` | 其餘在 A、B 階段搬進 | 規則現在改，影響最大 |
| 表／SQL view／SQL 函式 | 71 張表（seller 37）／0／0【量】`rg -c "CREATE TABLE" internal/modules/*/db/schema.sql`；`rg -n "CREATE (OR REPLACE )?(VIEW\|FUNCTION)" internal/modules/*/db/schema.sql`（無輸出） | — | 新增 `pub_*` 不必改任何現有物件 |
| 模組間 import 邊 | 12 條【量】附錄 A1 | 隨搬遷成長 | — |
| 跨模組共用 transaction | accountdeletion：在自己的 `InTx` 內鎖 identity 帳號，再呼叫 identity、buyer、seller 的 `BeginDeletion`【量】`sed -n 225,262p internal/modules/accountdeletion/internal/request/request.go` | A7-1d：moderation → social 驗證 → seller 建立措施 | D4 已經在用 |
| 同 transaction 訂閱者 | 24 個：19 relay、1 inbox、4 原子狀態變更【量】`rg -n "events\.Subscribe\(" cmd -g '!*_test.go'` | relay 隨 `internal/legacy` 刪除 | 不需要改 |
| `events.Publish` 呼叫點 | 48【量】`rg -n "events\.Publish\(" internal -g '!*_test.go' \| wc -l` | — | 發布方寫法不變 |
| `.Enqueue(` 呼叫點 | 32，其中 16 在 `internal/legacy`【量】`rg -n '\.Enqueue\(' internal -g '!*_test.go' \| wc -l`（v2 寫 63，把 `jobs.Enqueuer` 型別也算進去了） | — | outbox 和同 transaction 事件並用，不是「主力」 |
| `InTx`／`InReadSnapshot`（模組內，非測試） | 118／10；`internal/httpapi` 是 0【量】`rg -o '\.InTx\(' internal/modules -g '!*_test.go' \| wc -l`，`InReadSnapshot\(` 同理 | — | 模組服務實際上就是最上層，見 §3 |
| 照 D3 第 3 點建的讀取模型 | **0**【量】v1 §0 | 計畫中 3 份：social 曝光、social 封鎖（A7 計畫:177-178）、catalog `store_state`（遷移計畫 B4:257） | **規則還沒有任何程式兌現，現在改只改文件** |
| 計畫中的同 transaction 反應 | social 訂閱 `MeasureCreated`／`MeasureRevoked`（A7-1c）；social 訂閱 `BuyerDeleted`／`StoreRetired`（`interface-inventory.md:89-90`）；moderation 訂閱 `MeasureRevoked`（A7-1d） | A7-1a、1c、1d | 本文改變它們的做法 |
| 曝光的現況 | 規則在 Go（`model.MissingExposure`）；`seller.store_exposures` 存**最後公告值**，由 13 個寫入點在同一個 transaction 呼叫 `Announce`，時間造成的變化由 job 補上【量】§4.1 | social、catalog 要用它篩選 | **F1：見 §4.1** |
| D3 的強制機制 | ADR:129 寫「CI 用 Postgres parser 解析每一條 query」。實際是 sqlc（內建 Postgres parser）只讀自己的 `schema.sql`，參照別的 schema 在生成時失敗，加上一個故意違規的 fixture 檢查（`backend-monolith.yml:133-150`、`internal/arch/testdata/sqlc-foreign-schema`）【量】 | — | 大致準確，差在措辭，修訂時順手改清楚 |

**結論**：要改的規則（D3 第 3 點、單體規則 2 的後半、A7 與 B4 計畫裡的讀取模型）都還沒有程式；D4 已在用而且保留。在 A7-1a 開工前改最便宜。唯一需要改既有程式的是 seller 的曝光規則（§4.1），以及 A7-1c 本來就排定的 `measure.Create`。

### 拆服務是不是目標？

- ADR D1 否決微服務，理由是「多個團隊要各自部署」的組織問題在這裡不存在【量】ADR:29。重評條件是獨立擴展、開發人數增加、co-change 偏高（ADR:287-291），今天都不成立【量】根目錄 `AGENTS.md`。
- 計畫與 ADR 裡提到「單獨搬移」的只有 ADR:155 一處【量】`rg -n -i "獨立部署|單獨搬移|拆出服務" docs/adr docs/plans`。
- Grzybek 的 README 只寫「Module data could be moved into separate databases if desired」【外】https://github.com/kgrzybek/modular-monolith-with-ddd 。把他的全非同步設計歸因於「保留拆庫能力」是我的推論【推】。
- 【推】D3 裡「為了將來能拆」的理由前提不成立；「為了改動隔離、資料擁有權、不變量只有一個家」的理由和拆不拆無關，仍然成立。

---

## 1. 決策 A：跨模組讀取的媒介

### 業界基準

| 系統／作者 | 資料怎麼分 | 跨模組讀取 | 來源 |
|---|---|---|---|
| Kamil Grzybek | 「Each Module has it's own data in a separate schema - shared data is not allowed」 | 只用 integration event（outbox／inbox）；「it is not possible nor desirable to create a transaction which spans more than one module」 | 【外】https://github.com/kgrzybek/modular-monolith-with-ddd |
| Milan Jovanović（資料隔離） | 「I always go for logical isolation using schemas」；「Joins are only allowed between tables of the same module」 | 這篇沒有談 view | 【外】https://www.milanjovanovic.tech/blog/modular-monolith-data-isolation |
| Milan Jovanović（資料邊界） | 「a screen that spans multiple modules」時，在 **public schema** 建唯讀 view，以 **`GRANT SELECT` 給特定 role** 強制（「Consumers query the view but cannot access the underlying tables directly」）；「This view acts like a controlled public API」；用途舉的是「dashboards or admin screens」 | 與「event 餵的讀取模型」並列為兩個選項，後者「works well when modules might be extracted later」；拆庫時「the view will have to be replaced with a service call」。沒有談跨模組 transaction | 【外】https://milanjovanovic.tech/blog/how-to-keep-your-data-boundaries-intact-in-a-modular-monolith |
| Shopify（Packwerk） | model「restrict their usage to the component that owns them」 | 稱跨元件查詢是「the second big blocker」 | 【外】https://shopify.engineering/shopify-monolith |
| GitLab | 為了規模把 DB 拆成 main／ci／sec，之後用 `gitlab_schema` 擋跨庫 join | 拆庫後 join 不可能 | 【外】https://docs.gitlab.com/development/database/multiple_databases/ |
| Derek Comartin | 反對跨模組 join：「if all the coupling is happening at the database level, what you still have is a big ball of mud」 | API 契約，或把概念放回邊界內 | 【外】https://codeopinion.com/stop-joining-tables-in-your-modular-monolith/ |
| 37signals（Majestic Monolith） | 一個整合的應用，不談內部資料邊界 | 自由 join | 【外】https://signalvnoise.com/svn3/the-majestic-monolith/ |
| Spring Modulith、Simon Brown、Microsoft | 沒有找到資料擁有權的規定 | — | 【外?】不拿來當依據 |

**基準（綜合）**【推】：
- 普遍一致：資料歸一個擁有者，只有擁有者寫，別人不讀內部表。
- 分歧在跨模組讀取的媒介。保留拆庫能力的做法只用事件與副本。共用 DB 的做法裡，Jovanović 給了「擁有者以外的唯讀 view＋權限控管」，但他的範圍是跨模組的畫面（dashboard、admin）。
- 本文把它延伸到「面向買家、每次列表都要套的可見性述詞」，view 放在擁有者 schema，強制方式改用 sqlc 契約檔而不是 DB 權限。**這是超出來源的延伸**，理由在下面的約束帳；它不是 Jovanović 的原話。

### 現有機制為什麼是那樣

- 否決「join 對方的表」（ADR:153）：Integration Database，GitLab 拆庫時的障礙。針對內部表，成立，保留。
- 否決「join 對方公開的 view」（ADR:154）有兩條理由：
  1. Newman 把 view 定位為拆庫期間的過渡做法。本專案不拆庫，前提不成立。【外?】Newman 原文沒有查到，照 ADR 的轉述。
  2. Atlas 社群版靜默略過 view。事實成立，本版也再量過，而且函式也一樣被略過【量】附錄 B3。不過 Atlas 是我們自己選的工具，處理見決策 B。
- 否決跨 schema FK（ADR:155）：migration 順序依賴這條理由成立，所以仍不建 FK。
- 第 3 點（讀取模型同 transaction 更新）之所以是「唯一的路」，是因為上面把 view 否決了。

### 選項

| 選項 | 一致性 | 改動隔離 | 主要代價 |
|---|---|---|---|
| A1 維持 D3（Go API／composition／讀取模型） | 讀取模型有延遲或同步風險（v1 全篇） | 好 | 3 份副本，加上事件、投影、重建、去重、sweep |
| **A2 擁有者發布 `pub_*`，上層在 SQL 裡 join（建議）** | 同一條 SQL、同一個 snapshot；**前提是 `pub_*` 算的是現在的狀態，不是公告值**（§4.1） | 好：消費端只依賴 `pub.sql` 的形狀 | `pub_*` 是契約；消費端的查詢計畫會用到擁有者的索引 |
| A4 允許 join 內部表 | 強一致 | 差 | 違反擁有權基準，不採 |
| A5 擁有者 Go API 回傳 ID 集合，消費端 `= ANY($1)` | 在同一個 snapshot 內強一致 | 好 | 集合要小（觀看者自己的封鎖名單）；不適合「全部可曝光的店」 |

v2 有一個 A3「SQL 函式，會擋住 planner 內聯」。**這個推論是錯的**：`LANGUAGE sql STABLE` 的 set-returning 函式會被內聯，帶參數的 generic plan 也一樣，`BEGIN ATOMIC` 的寫法也一樣【量】附錄 B1。所以函式不另列為一個選項，而是 A2 在需要時間參數時的形式。

### A2 的設計要點

- **命名與擁有**：`pub_*` 定義在擁有者的 `schema.sql`，同時宣告在擁有者的 `db/pub.sql`。只放消費端需要的欄位。
- **函式一律寫成 `BEGIN ATOMIC`**：這樣 Postgres 會追蹤它依賴的欄位，底層欄位被改型別或刪除時直接拒絕。用 `$$` 字串寫的函式體不追蹤依賴，改欄位名會留到執行時才壞【量】附錄 B2。
- **誰可以用**：照 `internal/arch/modules.go` 的依賴表，和 Go import 用同一張表。
- **強制（F5，取代 v2 的 parser 方案）**：
  - 消費端 `sqlc.yaml` 的 `schema:` 只列自己的 `schema.sql`，以及依賴模組的 `pub.sql`。sqlc 型別檢查 `pub_*` 的 join；`JOIN seller.measures` 會在生成時失敗（「relation "measures" does not exist」）【量】審查 `sqlc-exp/social2`，本版附錄 B4 重做。
  - sqlc 1.31.1 的限制【量】附錄 B4：`RETURNS TABLE (...)` 函式的輸出欄位**不會**被型別化，select 會報「column does not exist」，`SELECT *` 只得到 `interface{}`，放在 EXISTS 裡面則**完全不檢查欄位**。`RETURNS SETOF <表或 view>` 會被完整型別化並檢查欄位。所以 `pub.sql` 裡的函式宣告成 `RETURNS SETOF seller.pub_<名>`，並把 `pub_<名>` 宣告成一張只有欄位的表。真實 schema 裡的回傳型別可以不同，由下面的契約測試對齊。
  - 新增一個 arch 測試，在 `internal/arch` 內：
    - 每個模組 `sqlc.yaml` 的 schema 清單只能是自己的 `schema.sql`，加上依賴表允許的模組的 `pub.sql`。
    - `pub.sql` 裡每個 `CREATE` 的名字都必須是 `<自己的 schema>.pub_*`。
  - 新增契約測試（整合）：把 `pub.sql` 的每個物件，和套用 migration 後真實物件的欄位名稱、型別、參數比對（`information_schema.columns`；函式用 `pg_get_function_arguments`／`pg_get_function_result`）。這就是 v2 說缺少的消費者契約測試。
  - 現有的 `sqlc-foreign-schema` fixture 照用，它仍然證明「沒列在 schema 清單的 schema 會失敗」。
  - `scripts/gen-monolith-sqlc.sh` 要改成先把所有模組複製進 stage，再逐一 `generate`。現在是複製一個、跑一個，照字母順序；catalog 排在 seller 前面，生成時會讀不到 `seller/db/pub.sql`【量】`sed -n 1,25p scripts/gen-monolith-sqlc.sh`。
- **另一種強制方式：DB role＋`GRANT`**（Jovanović 的做法）。**不採**。要做到它，每個模組要有自己的 role。但 D4 的共用 transaction 跑在同一條連線、同一個 role 上，每次跨模組呼叫都得 `SET ROLE`。現在沒有任何 `GRANT`，也只有一個連線池【量】`rg -l -i 'GRANT ' internal/modules/*/db/migrations | wc -l` 為 0；`platform/db/db.go` 只有一個 `pool`。這條約束來自 D4，D4 是我們自己的決定（SELF-IMPOSED），但它基於自身理由保留（§3）。
- **效能**：在合成資料上（50 萬貼文、2 萬店家、2 千筆生效中的措施），完整的曝光述詞以 EXISTS 內聯，執行 0.77 ms、890 buffers【量】附錄 B1。這不是正式資料量，要求見 §5 的 EXPLAIN 驗收。

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| D3 否決 view 的文字 | SELF-IMPOSED（我們的 ADR） | ADR:154 |
| Newman「view 是拆庫期間的過渡」 | 前提不成立，不是約束 | ADR:154、D1 |
| Atlas 社群版略過 view 與函式 | 授權是事實，但「必須由 Atlas 產生 migration」是我們選的 → SELF-IMPOSED | 【外】https://atlasgo.io/features；【量】附錄 B3 |
| sqlc 不型別化 `RETURNS TABLE` 函式 | 工具行為是事實，但 sqlc 是我們選的 → SELF-IMPOSED；用 `SETOF <relation>` 繞過，不另付代價 | 【量】附錄 B4 |
| D4 的單一 role 共用 transaction（讓 GRANT 不可行） | SELF-IMPOSED，基於自身理由保留 | ADR D4 |
| 模組要能單獨搬移 | SELF-IMPOSED，前提不成立 | ADR:155 |
| Postgres 是唯一的 DB | **SELF-IMPOSED**（ADR D3 的選擇）。不在本題範圍，本文不重議 | ADR D3 |
| 一個開發者、從未上線 | REAL（事實） | 根目錄 `AGENTS.md` |

**沒有任何 REAL 約束把我們推離「擁有者發布唯讀介面」的基準。**

---

## 2. 決策 B：`pub_*` 的 migration 怎麼產生

### 業界基準

- 宣告式 diff 工具普遍把 view 與函式當一級物件（Atlas Pro 等）【外】https://atlasgo.io/features。
- 只寫版本化 SQL 的工具（goose、Flyway 等）對 view 沒有限制【推】，沒有逐一查文件。

### 現有機制與實測（F8）

- Atlas 在本 repo 只在本機跑 `migrate diff`。套用、`atlas.sum`、schema 一致性檢查（`TestSchemaFileMatchesMigrations`，用 `DescribeSchema`）都是我們自己的程式。`DescribeSchema` 已經涵蓋 view（`pg_get_viewdef`）與函式（`pg_get_functiondef`）【量】`sed -n 320,330p internal/platform/migrate/migrate.go`。
- 實測（Atlas v1.1.3 canary，seller schema 副本）：
  - 在 `schema.sql` 加 view，diff 回報「synced」，靜默略過。手寫 view migration 之後，diff 也**不會**產生 DROP VIEW【量】審查 `atlas-exp`。
  - 在 `schema.sql` 加 `BEGIN ATOMIC` 函式，diff 產生的 migration 只有表的變更，**沒有 CREATE FUNCTION**【量】附錄 B3。
  - 改一個被 view 用到的欄位型別：Atlas 照樣產生 `ALTER COLUMN … TYPE`，不警告【量】審查 `atlas-exp`。Postgres 在套用時拒絕：「cannot alter type of a column used by a view or rule」。`BEGIN ATOMIC` 函式也一樣：「… used by a function or procedure」；刪欄位也被拒絕【量】附錄 B1、B2。

### 選項

| 選項 | 內容 | 取捨 |
|---|---|---|
| **B1 表照用 Atlas，`pub_*` 手寫（建議）** | 含 `pub_*` 的變更用 `atlas migrate new` 建空檔手寫。改到 `pub_*` 底層欄位時，在**同一份**手寫 migration 內 drop 再建 | 漏寫 `pub_*` → `TestSchemaFileMatchesMigrations` 擋；底層欄位改型別卻沒重建 → Postgres 在套用時拒絕（本機 `hawkr migrate` 與 integration CI 都會失敗） |
| B2 拿掉 Atlas，全部手寫 | — | 多工作，沒有多得到正確性 |
| B3 Atlas Pro | 付費加憑證 | 為這一項不值得 |

**結構性守門**：每一種錯誤都是「大聲失敗」，沒有靜默壞掉的路徑。漏建由 schema 一致性測試擋，改型別由 Postgres 的依賴追蹤擋（前提是函式一律寫 `BEGIN ATOMIC`）。所以 **integration CI 就是守門**，不另做文字比對式的檢查；那種檢查脆弱，而且擋的是已經會大聲失敗的情況（already protected）。規則 6 改成「表由 Atlas 產生；`pub_*` 手寫，改到底層欄位時在同一份 migration 內重建；函式一律 `BEGIN ATOMIC`」。

---

## 3. 決策 C：D4——跨模組共用 transaction 與同 transaction 事件

### 先問：要解的是正確性還是併發瓶頸？

A7-1c 找到的三個問題都是**正確性**問題：`create.go` 的 `InReadSnapshot` 在 transaction 內失敗、反向鎖序、訂閱者失敗會 rollback 發布方。單一 DB 裡最便宜的解法是同一個 transaction 或同一個 snapshot。非同步是為了失敗隔離與擴展，代價是一致性。所以規則是：能在讀取時推導的就推導；需要原子性的就共用 transaction。

### 業界基準

| 來源 | 跨模組 transaction | 同步 in-tx 事件 |
|---|---|---|
| Grzybek | 禁止 | 不用，只有 outbox／inbox |
| Spring Modulith | 同步發布是預設，好處是「a very simple consistency model (either both … succeed or none of them does)」；壞處是「widen the transaction boundary and potentially cause the entire transaction to fail, even if the functionality that is causing the error is not crucial」 | 模組整合的預設是 `@ApplicationModuleListener`（非同步、`REQUIRES_NEW`），發布紀錄在原 transaction 內寫入 |
| Jovanović | 兩篇都沒談 | — |

來源：【外】https://docs.spring.io/spring-modulith/reference/events.html 、Grzybek 同前。

**基準**【推】：核心不變量要原子時，共用 transaction；次要的衍生反應放到 commit 之後的持久紀錄上非同步執行。

### 現有機制為什麼是那樣

- D4 否決「各開 transaction 再串 saga」，理由是四個實際缺口（ADR:175-180）。這是正確性證據，對「由上往下、不變量跨模組」的寫入成立。
- `InTx` 已經可以重入：ctx 有 transaction 就直接加入【量】`platform/db/db.go:112-114`。`InReadSnapshot` 在 transaction 內回 `ErrSnapshotInTx`【量】`db.go:149-151`，失敗是大聲的。
- PostgreSQL 設計規範 §十二已經規定冪等寫入用 `INSERT … ON CONFLICT`，撞 23505 之後不得在同一個 transaction 繼續下 SQL【量】`sed -n 287,310p "docs/開發規範/PostgreSQL 設計規範與最佳實踐.md"`。

### 暴露面逐一判定

| 暴露面 | 建議 | 理由與證據 |
|---|---|---|
| accountdeletion → identity、buyer、seller | **保留** | 不變量跨三個模組 |
| A7-1d moderation → social 驗證 → seller 建立措施 | **保留共用 transaction**；seller 提供 `CreateInTx` 入口 | 處置與措施要同時成立（A7-1d） |
| 公開方法自己開 transaction（**F2，取代 v2 的規則**） | 規則：①公開方法可以用 `InTx`（可重入）；②**會被別的模組呼叫的方法**不得用 `InReadSnapshot`，要另給一個加入呼叫方 transaction 的入口；③撞唯一鍵後不得在同一個 transaction 繼續下 SQL（就是 §十二） | 影響面【量】：<br>- `InTx` 118 處，不用改。<br>- `InReadSnapshot` 10 處（seller measure 9、discovery 1）：現在**沒有一處**在跨模組的呼叫路徑上（`rg -n "Measure\|Notes\(" internal/modules/{accountdeletion,agent,delegation,discovery}` 無命中；discovery 沒有任何模組依賴它）。計畫中只有 1 處會進入跨模組路徑：`create.go:278` 的 replay，經 A7-1d。<br>- catch 23505：2 處。`prospect.go:67` 分類後直接 return，合規。`create.go:259` 之後在**新的** snapshot 重播，單獨呼叫時合規，加入呼叫方 transaction 後會變成 25P02。<br>所以實際要改的是 1 個入口，而且 A7-1c 本來就排了 |
| 冪等（**F3**） | 跨模組入口 `CreateInTx` **不收 request_id**。冪等由呼叫方的狀態轉移擁有（A7-D7 第 10 點：處置的條件式 UPDATE，plan:291）。seller 自己的 handler 路徑保留 request_id，改成同一個 transaction 內「先鎖目標 stint，再查 (admin, request_id)」：同一個請求的並行呼叫會在 stint 鎖上排隊，後到的在 READ COMMITTED 下看得到先 commit 的列。插入用 `ON CONFLICT … DO NOTHING` 收掉剩下的競態，**不用 `SELECT … FOR UPDATE`**（Postgres 對不存在的列不鎖），不 catch 23505 後繼續 | §十二；`create.go:236-300` |
| 鎖順序（**F4，取代 A7-1c (b)**） | 見下方條文 | accountdeletion 先鎖 identity 帳號（`request.go:227`）；A7-1d 寫「最後寫處置」 |
| social 對內容下架措施的隱藏（A7-1c） | **讀取時推導**：join `seller.pub_content_takedowns`，不訂閱、不寫 social 的表。**依賴 A7-D7（狀態：未決，plan:475）的措施內容模型**，view 的定義等 A7-D7 定案後才寫 | 強一致，撤銷自動還原，沒有 rollback 連鎖 |
| moderation 訂閱 `MeasureRevoked` 更新處置狀態（A7-1d） | **改為讀取時推導**：處置頁讀 `seller.pub_*` 的措施現況；這和 646da604d「案件頁讀取時查措施現況」同方向。如果真的要存，就用 outbox | 同上 |
| social 訂閱 `BuyerDeleted`／`StoreRetired` 刪內容 | **改為 outbox**：relay 式訂閱者只 `Enqueue` 清除 job；可見性由 `pub_*`（已刪除、已退場）在讀取時擋住 | 刪資料是清理，可見性已經有保證 |
| **對賣家的「內容被平台隱藏」通知（F7，interface-inventory 第 11 列）** | 原觸發點是 social 的 `post.hidden_by_platform`。改成讀取推導後 social 不再寫，**觸發點改為 seller 的 `MeasureCreated`**（種類＝內容下架）：A8 的 notification 以 relay 形狀訂閱、只 `Enqueue`。現在 `MeasureCreated` 在 cmd 裡沒有任何訂閱者【量】`rg -n "MeasureCreated" cmd internal/legacy` 無命中。要不要和既有的措施通知合併成一則，是產品決定（P3） | `interface-inventory.md:87` |
| 4 個原子訂閱者 | 保留，不改程式 | 【推】只依註解判斷 |
| 19 relay＋1 inbox | 保留 | 這就是 outbox |

### 鎖順序條文（F4）

> 在跨模組的 transaction 內，**可能被兩個以上模組的路徑鎖到的列**，一律依下列層級由下往上鎖：
> 1. identity 帳號列：錨點。任何涉及一個帳號、跨模組的流程，第一個鎖它（`Identity.LockAccount`）。
> 2. buyer 的列。
> 3. seller 的列，照 seller 內部既有順序：店家／stint → 申請 → 註冊 → 公告列；措施與通報在店家列之後（`create.go`、`exposure.go` 註解）。
> 4. 上層模組的列，依 `modules.go` 依賴表由下往上（例如 social 內容在 seller 之後）。
> 5. 編排者（moderation、accountdeletion）自己的列最後。
>
> 「鎖」指 `FOR UPDATE`／`FOR SHARE`、`UPDATE`、`DELETE`、會衝突的 `INSERT`。READ COMMITTED 下的一般 `SELECT` 不鎖列，驗證型讀取（例如 moderation 先確認內容屬於該店）不受這條限制。
> 上層模組不得在呼叫下層**之前**鎖住自己會被下層同步訂閱者碰到的列。

核對：
- accountdeletion：identity → buyer → seller →（自己的列）✓
- A7-1d：讀 social 驗證（不鎖）→ seller 建措施 → 最後寫處置 ✓
- A7-D9 BAN：`identity.LockAccount` → `seller.CurrentStore` ✓
- A7-1c (b) 原本寫「moderation 的處置 → identity → seller → social」：處置在最前，和 A7-1d「最後寫處置」矛盾，也和第 5 層衝突。**本條取代它。**
- v2 寫「呼叫順序＝依賴方向由上往下」，這是錯的：鎖的方向是由下往上。

### 選項（同 transaction 事件的定位）

| 選項 | 取捨 |
|---|---|
| C1 照現行文字，不加規則 | social、moderation 的反應會繼續被設計成訂閱，每次重新分析鎖序 |
| **C2 收窄＋優先推導＋上面三條規則（建議）** | 反向鎖序與 rollback 連鎖在結構上不出現 |
| C3 模組間一律非同步 | 重新引入 D4 列的四類缺口 |

### 約束帳

| 約束 | 判定 | 來源 |
|---|---|---|
| `seller.Create` 自己開 `InReadSnapshot` | SELF-IMPOSED（我們的程式） | `create.go:246-249,278` |
| D4 handler 失敗就 rollback | SELF-IMPOSED；對原子狀態變更是對的，保留 | ADR:164 |
| 措施是店家處分的唯一來源、處分要可申覆（賣家條款 §18.4） | **SELF-IMPOSED**：條款從未生效（從未上線），是 Will 擁有的產品承諾，不是外部 REAL。本建議不改它 | A7 計畫:66 |
| 下架後「立即」不可見的法定時限 | 【猜】沒有查到條文；推導做法是零延遲 | — |

---

## 4. 具體消費端套用後的樣子

### 4.1 曝光（F1）：現況量測與路徑

**輸入怎麼變**【量】：

| 規則的輸入（`model.MissingExposure`、`exposure.go`） | 怎麼變 | 公告值什麼時候跟上 |
|---|---|---|
| 店家狀態（關閉、等待刪除）、店名、logo、鄉鎮代碼 | 指令（closing、profile） | 同一個 transaction 呼叫 `Announce`，零延遲 |
| KYC 流程 = APPROVED（申請狀態＋PayUni `bind_status`＋`outcome` 欄位） | 指令或 PayUni 觀測 job 寫入；`DeriveKYCFlow` 沒有時間輸入 | 同上（`kyc.go`、`kyc/admin.go`、`payuni/observe.go`） |
| 停權（措施在 `at` 是否生效：`starts_at <= at AND (ends_at IS NULL OR ends_at > at)`） | **時間**：停權結束（`ends_at`）、預告終止生效（`starts_at = created_at + 720h`） | 到期 job 排在 `ends_at`（`suspension/jobs.go:29-33`），終止生效 job 排在 `starts_at`（`closing/termination.go:44-45`），兩者都經 `Recompute` → `Announce`。正常延遲是 River 排程器加取件：預設 5 s＋1 s（River v0.47.0 `JobSchedulerIntervalDefault`、`FetchPollIntervalDefault`）。保底：`seller.reconcile_exposure` 每 15 分鐘、`reconcile_suspensions` 每小時 |

指令：
- `rg -n "\.Announce\(|\.Reannounce\(" internal/modules/seller -g '!*_test.go'`：13 個寫入點加 2 個對帳點。
- `sed -n 1,80p internal/modules/seller/internal/suspension/jobs.go`
- `sed -n 1,60p internal/modules/seller/internal/profile/reconcile.go`
- River 預設值：`rg -n "JobSchedulerIntervalDefault\s*=" $(go env GOMODCACHE)/github.com/riverqueue/river@v0.47.0`

**公告值和現在的規則什麼時候不一致**【推】，依上表推出：
- 只有在時間輸入的空窗。方向不安全的只有一種：預告終止已經生效，公告值還是可曝光。
- 停權結束後還沒公告的空窗，方向是安全的：店家多被藏一陣子。
- 空窗期間，以 Go 現算規則讀的消費端（discovery 的 `ExposableStores`）和讀公告值的消費端會得到**兩個不同的答案**。

**公告值今天的消費者**【量】：
- legacy relay（`cmd/hawkr/seller_events.go:34` → `internal/legacy/sellerrelay`），餵還沒搬進單體的 product。
- legacy 的縣市篩選 `ExposableStoreIDsByCounty`（`seller.go:502` 自己寫了「落後最多一個對帳週期」）。
- discovery 讀的是 Go 現算規則（`ranking.go:124`、`recompute.go:132`），不是公告值。

**路徑**：

| 路徑 | 內容 | 一致性 | 代價與擁有者 |
|---|---|---|---|
| **(i) 規則唯一的家在 seller 的 SQL（建議）** | 每個述詞都寫進 seller 擁有的 SQL：<br>- `seller.store_requirement_flags` view：各個與時間無關的條件，一欄一個布林。<br>- `seller.effective_measures(at) RETURNS SETOF seller.measures`：生效中的唯一定義；`EffectiveMeasures`／`EffectiveMeasuresAll` 改成從它選。<br>- `seller.pub_exposable_stores(at)`：flags 全真，而且帳號在 `at` 沒有生效中的停權。<br>Go 的 `Classify`、`CapabilitiesOf` 改讀 flags 加上 `effective_measures(at)`，只負責把布林對應到 `Requirement` 的順序與錯誤 metadata；「全部條件的 AND」在 Go 與 SQL 各寫一次，用資料集測試鎖住（repo 對停權推導已經用這個模式：`suspensions.sql.go:85`）。時間一律是請求的 clock，以參數傳入，不用 SQL 的 `now()` | 所有消費端在同一刻得到同一個答案，零延遲 | seller：schema、手寫 migration、`exposure.go`、`model/exposure.go`、`model/capability.go`、`stint`（生效述詞）、資料集測試、`pub.sql`。約 8–12 個檔、1–1.5 個 session【猜】。**擁有者是 seller，目前由 10e session 在改**，要排在 10e 落地之後、A7-1a 之前 |
| (ii) view 包在公告值外面，接受延遲 | `seller.pub_store_exposure` = `store_exposures` | 指令造成的變化零延遲；時間造成的變化正常約 6 s、最壞 15 分鐘；空窗期間和 discovery 答案不同 | seller 幾乎不用改。要你決定 P1 的延遲上界 |
| (iii) 由 Go 的 `ExposableStores` 補 ID 集合（A5） | social 先取一頁，再過濾，不夠就補抓 | 強一致 | 每頁筆數不固定，補抓迴圈讓 keyset 分頁變複雜；`MaxBatchSize` 100。適合 discovery 這種批次，不適合列表 |
| (iv) SQL 再寫一份，資料集測試鎖住 Go 規則 | 同 (i) 的 SQL，但 Go 規則不動 | 同 (i) | 便宜一點，約 0.5 session。但一個不變量有兩個家，違反「不留技術債」 |

**建議 (i)**。原因：
- 它是唯一同時做到「一個家」、「零延遲」、「所有消費端同一個答案」的路徑。
- 函式會被內聯【量】附錄 B1，`SETOF <relation>` 讓 sqlc 完整型別化【量】附錄 B4，所以沒有工具上的阻礙。
- (ii) 留下的是 v2 宣稱已經解決、其實沒解決的延遲。
- (iv) 留下兩個家。

**(i) 的終局**：
- `store_exposures`、`StoreExposureChanged` 與 15 分鐘對帳，只剩「通知行程外的 legacy 消費者」這一個用途。
- 觸發條件：B4 把 product 搬進單體，`rg -n "OnStoreExposureChanged" cmd` 歸零。那時一起刪除，`first_exposed_at` 的記錄改由週期 job 讀 `pub_exposable_stores(now)` 寫入。

### 4.2 各消費端

| 消費端 | 原計畫 | 新做法 | 一致性 |
|---|---|---|---|
| 後台待辦佇列（10e D′1、A7-1e） | composition | **不變**；改寫成 D3 判斷順序的正式一步 | 每個來源在同一個 `InReadSnapshot` 內讀【推】 |
| social 曝光（A7-1a） | 訂閱 `StoreExposureChanged` 維護讀取模型 | `EXISTS (SELECT 1 FROM seller.pub_exposable_stores($now) …)`，前提是 §4.1 (i) | 零延遲（走 (i)）；走 (ii) 就是 P1 的延遲 |
| social 封鎖過濾（A7-1a、1b） | 同 transaction 更新讀取模型 | join `buyer.pub_blocks`／`seller.pub_blocks`；觀看者名單小時用 A5。封鎖表在 A7 搬進 buyer 與 seller（A7-D2），現在單體裡還沒有【量】`rg -n -i block internal/modules/{buyer,seller}/db/schema.sql` 無命中 | 強一致 |
| SD3 寫入端（被封鎖的買家留言被拒） | 讀副本 | 在 social 自己的 transaction 內讀 `seller.pub_blocks`，或呼叫 seller API | 強一致 |
| social 對內容下架的隱藏（A7-1c） | 訂閱 | join `seller.pub_content_takedowns`（**等 A7-D7**） | 強一致 |
| catalog `store_state`（B4） | 讀取模型 | 用 `seller.pub_exposable_stores($now)`，必要時加上 seller 的其他 `pub_*`；不建 `store_state` | 同 social 曝光 |
| 已刪帳號、已退場店家的內容 | in-tx-event | relay 只排入清除 job；可見性由 `pub_*` 擋住 | 可見性強一致，清理最終一致 |
| 對賣家的內容隱藏通知（第 11 列） | social 發 `post.hidden_by_platform` | A8 訂閱 `MeasureCreated`（內容下架）只 `Enqueue` | 通知最終一致 |

---

## 5. 建議、終局與觸發條件

### 建議（每題一句）

- **A**：保留 schema per module、單一寫入者、不建 FK；新增擁有者發布的 `pub_*`（view，或帶時間參數的 `BEGIN ATOMIC` 函式），依賴表允許的上層可以 join；強制靠 `pub.sql` 契約檔加 arch 測試加契約測試。
- **A′（F1）**：曝光走 §4.1 (i)，規則唯一的家在 seller 的 SQL，時間由請求傳入。
  - 2026-10-19 完成：`seller.store_requirement_flags`（view）、`seller.effective_measures(at)`、`seller.pub_exposable_stores(at)`，Go 的 `Classify` 讀函式、`CapabilitiesOf` 讀條件。與 §4.1 (i) 的差異：`effective_measures` 回傳窄的 `seller.effective_measure (id, account_id)` 而不是 `SETOF seller.measures`（實測 measures 加欄位後回傳整列的函式在內聯時失敗）。
- **B**：表照用 Atlas，`pub_*` 手寫；守門是 schema 一致性測試加上 Postgres 的依賴追蹤（integration CI）。
- **C**：D4 保留；同 transaction 訂閱者收窄；三條規則（跨模組入口不得用 `InReadSnapshot`、撞鍵後不得繼續下 SQL、由下往上的全域鎖順序）。
- **D3 判斷順序**：
  1. 擁有者對嗎？
  2. 只是依 ID 補顯示 → 呼叫 Go API。
  3. 各來源自己有全部篩選鍵 → keyset composition。
  4. 要用下層的屬性篩選或排序 → join 下層的 `pub_*`，前提是它算的是現在的狀態。
  5. 只有在觸發條件成立時才建讀取模型，以 outbox 投影。

### A7-1a 的驗收要加（F7）

social 的公開可讀述詞是多個 `pub_*` 的組合：買家與店家的刪除、曝光、內容下架、兩種封鎖，再加上 SD16 的 hashtag 計數。A7-1a 的驗收要加：
- 在播種到預估正式量（至少 10⁵ 貼文、10⁴ 店家）的 DB 上，對動態牆、店家頁、hashtag 列表與計數各跑一次 `EXPLAIN (ANALYZE, BUFFERS)`，結果貼進 PR。
- 每個 `pub_*` 函式都要被內聯（計畫裡沒有 `Function Scan`），每個 join 都要有 Index Cond。
- p95 ≤ 300 ms @ page_size 50（沿用 10e D′1 的門檻）。
- 不達標時先加索引；加了還不行，才觸發讀取模型。

### 終局

上面就是終局，不是過渡。讀取模型只在下列量測觸發時才建：
- 某支讀取在正式資料量下 p95 超過門檻，`EXPLAIN` 顯示瓶頸在跨 `pub_*` 的 join，而且加索引解決不了。
- 或者 ADR D1 的重評條件成立，要拆某個模組。這時它的 `pub_*` 消費端逐一換成讀取模型或 API。消費端數量可以從各模組 `sqlc.yaml` 的 `pub.sql` 清單直接列出。

觸發之後的讀取模型寫法（保留 v1 與其審查的修正）：
- 在來源的 transaction 內 `InsertTx` 排入投影 job，只帶 ID 與版本；worker 讀來源的現在狀態再 upsert。
- 唯一 job 的狀態集合不含 Completed，結算時比對來源版本（照 `media/internal/refsync/refsync.go:311-321`【量】）。
- 加週期性 Sweep 與全量重建指令。
- 指令端的不變量一律同步問擁有者，不讀副本。

### 成本與受影響的檔案

| 項目 | 檔案 | 時機 | 估計 |
|---|---|---|---|
| ADR 修訂段：D3 判斷順序、撤回否決 view 的理由並說明、更正強制機制的措辭、D4 收窄與三條規則、規則 6 的 `pub_*` 例外 | `docs/adr/2026-09-24-go-modular-monolith.md` | 現在 | 這一列加下面四列共 0.5 session |
| 單體規則 2、3、4、6 | `backend/AGENTS.md` | 現在 | 同上 |
| A7 計畫：:71、:177-178、A7-1a 驗收（EXPLAIN）、A7-1c (b) 改為上面的鎖順序條文、(c) 定為推導、A7-1c 驗收的「social 訂閱」與 mutation 測試改寫、A7-1d 的 `MeasureRevoked` 訂閱改為推導；`interface-inventory.md` 第 11、13、14 列 | `docs/plans/2026-10-18-a7-social.md`、`interface-inventory.md` | 現在 | 同上 |
| 遷移計畫 B4：刪掉 `store_state` 讀取模型 | `docs/plans/2026-09-24-go-modular-monolith.md:257` | 現在 | 同上 |
| v1 簡報標記被取代；10e D′1 的例外文字由第 3 步取代 | `d3-read-model-brief.md`、`step10e-3cd-brief.md` | 現在 | 同上 |
| **曝光規則搬進 seller 的 SQL（§4.1 (i)）** | `seller/db/{schema.sql,pub.sql,migrations/*,queries/*}`、`seller/internal/{exposure,model,stint}/*`、資料集測試 | 10e 落地後、A7-1a 前 | 1–1.5 session【猜】，seller 擁有者 |
| `pub.sql` 機制：`gen-monolith-sqlc.sh` 改成先全部 stage；arch 測試（sqlc 清單 × 依賴表、`pub.sql` 只放 `pub_*`）；契約測試 helper | `scripts/gen-monolith-sqlc.sh`、`internal/arch/*`、`internal/platform/testdb` 或 `migrate` | A7-1a 開工時 | 0.5 session【猜】 |
| social 的 `sqlc.yaml`、第一批 `pub_*`（刪除、封鎖），內容下架等 A7-D7 | `internal/modules/{social,buyer,seller}/db/*` | A7-1a／1c | 含在各步 |
| `seller.CreateInTx` 入口與 request_id 路徑改成「先鎖再查」 | `seller/internal/measure/create.go`、`seller/measure*.go` | A7-1c（已在計畫） | 不增加 |

**計畫的淨變化**：
- A7 少兩份投影（曝光、封鎖）和它們的重建、去重、sweep；A7-1c、1d 少兩個同步訂閱者；B4 少一張表。
- 多出一個 seller 的前置工作（曝光規則搬進 SQL），它擋住 A7-1a。
- v2 說「程式現在沒有任何東西要重寫」，這句話撤回。

---

## 6. 事件的發布方式會不會變？（給正在加 seller 通報事件的 session）

**不變。** 規則：

1. 事件型別定義在 seller 的 `events.go`（別名到 `internal/model`），在產生該事實的指令的 `db.InTx` 內呼叫 `events.Publish`。不要在 transaction 外發，不要自建 outbox 表，seller 也不要直接 `Enqueue` 通知用的 job。
   【量】48 個 `events.Publish` 呼叫點都是這個形狀。**`ReportAcknowledged`、`ReportResolved` 已經存在**（`internal/modules/seller/events.go:34-37`，在 `internal/governance/report.go:256` 發出）。`ReportReceived` 目前 0 個命中，新增前先確認語意不和既有的重複。
2. 沒有訂閱者時 `Publish` 只做 transaction 檢查（`internal/platform/events/bus.go:50-63`），先加事件不影響行為。
3. 本文改的是**訂閱端**：A8 的 notification 訂閱這些事件時，handler 只能在同一個 transaction 內 `Enqueue`（和 19 個 relay 同形），不做業務或外部呼叫。
4. payload 帶 ID 與當下的事實，不帶之後要重讀才正確的衍生狀態【推】。
5. 不要為了讓別的模組知道結案，就讓上層同步訂閱來改自己的表或決定可見性；那屬於讀取時推導（`seller.pub_*`）。

---

## 7. 要你決定

### 技術決策

| 題 | 建議 |
|---|---|
| A：跨模組篩選的媒介 | 擁有者發布 `pub_*`，上層 join；讀取模型降為量測觸發的效能手段 |
| A′：曝光規則放哪裡（F1） | (i) seller 的 SQL 是唯一的家，Go 讀它；要排 seller 的 1–1.5 session，和 10e 協調 |
| B：`pub_*` 的 migration | Atlas 照用，`pub_*` 手寫，函式一律 `BEGIN ATOMIC`，integration CI 守門 |
| C：D4 | 保留；收窄同 transaction 訂閱者；三條規則（含取代 A7-1c (b) 的鎖順序） |

### 產品決策（與技術分開）

| 題 | 說明 | 建議 |
|---|---|---|
| P1 停權、終止或封鎖生效後，第三方多久內不能再看到店家或內容 | 走 A′(i)：曝光、封鎖、下架都是零延遲，這題不用答。**只有你改選 A′(ii) 時才要答**：預告終止生效後店家仍可見的上界（正常約 6 s、最壞 15 分鐘），以及停權結束後多藏一陣子（方向安全） | 選 (i)，這題就消失 |
| P2 已刪帳號、已退場店家的內容實體刪除時限 | 可見性立即消失；實體刪除由 job 執行，秒到分鐘級【猜】 | 接受；有法定時限的話寫進 social `AGENTS.md` |
| P3 內容被下架時，賣家收到幾則通知 | 措施本身可能已有對賣家的通知；第 11 列的「內容被隱藏」通知要不要另發一則，或併入措施通知 | 併入措施通知（一個事實一則），和 10e 對齊【推】 |

---

## 8. 沒有驗證的部分

- `pub_*` 在正式資料量下的效能：只量過合成資料（附錄 B1），正式量由 A7-1a 的 EXPLAIN 驗收補上。
- §4.1 (i) 的工作量（1–1.5 session）是猜的；`CapabilitiesOf` 與 `model.Authorize` 的改動範圍沒有逐行盤點。
- `ON CONFLICT DO NOTHING` 碰上並行、還沒 commit 的衝突列時會不會等待：§十二說不等待，本文沒有實測。§3 的建議靠「先鎖 stint」排隊，不依賴這一點。
- Newman 原文【外?】；Spring Modulith、Simon Brown、Microsoft 對資料擁有權的立場【外?】，都沒有拿來當依據。
- 4 個原子訂閱者能不能改成推導【推】，本文不動它們。
- `pub_content_takedowns` 的定義取決於 A7-D7（未決）。

---

## 附錄 A：量測指令

- A1 模組間 import 邊：
  ```bash
  cd backend; for m in $(ls internal/modules); do rg -l --type go -g '!*_test.go' 'internal/modules/' internal/modules/$m | xargs -r rg -o --no-filename 'internal/modules/\w+"' | sort -u | sed 's|internal/modules/||;s|"||' | grep -v "^$m$" | sed "s|^|$m -> |"; done
  ```
  結果 12 條：accountdeletion→{buyer,identity,seller}、agent→seller、buyer→{identity,media}、content→media、delegation→{identity,seller}、discovery→seller、seller→{identity,media}。
- A2 訂閱者：`rg -n "events\.Subscribe\(" cmd -g '!*_test.go' | grep -v relay`
- A3 曝光：`sed -n 123,139p internal/modules/seller/db/schema.sql`；`sed -n 80,127p internal/modules/seller/internal/model/exposure.go`；`sed -n 80,180p internal/modules/seller/internal/exposure/exposure.go`
- A4 transaction 邊界：`sed -n 236,300p internal/modules/seller/internal/measure/create.go`；`rg -n 'InReadSnapshot\(' internal/modules -g '!*_test.go'`；`rg -n '23505' internal/modules -g '!*_test.go'`

## 附錄 B：實驗（scratchpad，已清除；可依下列步驟重現）

- **B1 函式內聯與效能**（Postgres 18.6，`initdb` 起在 127.0.0.1:54999）：
  - 資料：表 `seller.stores`、`measures`（部分索引同 repo 的 `measures_enforced_suspension`）、`kyc`、`payuni`、`social.posts`；20,000 店、2,000 筆生效中的停權、500,000 貼文。
  - 函式：`CREATE FUNCTION seller.pub_exposable_stores(at timestamptz) RETURNS TABLE(store_id bigint) LANGUAGE sql STABLE AS $$ …全部述詞…, NOT EXISTS (生效中措施 at) $$`。
  - 查詢：`PREPARE feed(timestamptz)`，以 `WHERE EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1) e WHERE e.store_id = p.store_id) ORDER BY created_at DESC, post_id DESC LIMIT 50` 取資料。
  - 結果：計畫裡沒有 Function Scan，展開成 stores、kyc、payuni 的 Index Scan 與 measures 的 Index Only Scan（`starts_at <= $1` 是 Index Cond）；`plan_cache_mode = force_generic_plan` 時相同；`EXPLAIN ANALYZE` 執行 0.765 ms、shared hit 890。
  - view 版（`pub_store_exposure` 包在 `store_exposures` 外）同樣被內聯（Memoize 加 Index Scan）。
  - 改 view 底層欄位的型別：`ERROR: cannot alter type of a column used by a view or rule`。
- **B2 函式的依賴追蹤**：
  - `$$` 寫的函式：改欄位型別成功；改欄位名稱也成功，下次呼叫才在內聯時報 `column p.outcome does not exist`，所以沒有依賴追蹤。
  - `BEGIN ATOMIC` 寫的函式：同樣被內聯；改欄位型別報 `cannot alter type of a column used by a function or procedure`，刪欄位報 `cannot drop column … because other objects depend on it`。
- **B3 Atlas 與函式**：
  - 複製審查的 `atlas-exp/db`（seller schema 副本），在 `schema.sql` 尾端加一個 `BEGIN ATOMIC` 函式，跑 `atlas migrate diff fn_test --env dev`。
  - 產生的檔案只有既有的表差異（`ALTER TABLE "seller"."store_exposures" …`），沒有 `CREATE FUNCTION`，函式被靜默略過。
- **B4 sqlc 1.31.1 與 `pub.sql`**：
  - 消費端 `schema: ["schema.sql", "../../seller/db/pub.sql"]`，加上 `omit_unused_structs: true`。
  - `RETURNS TABLE(store_id, published_at)`：select `e.published_at` 報 `column "published_at" does not exist`；`SELECT *` 產生 `[]interface{}`；在 EXISTS／IN 子查詢裡引用 `e.bogus` 也生成成功，欄位完全不檢查。
  - `RETURNS SETOF <composite type>`：同樣失敗。
  - `RETURNS SETOF seller.pub_exposable_store`（`pub.sql` 宣告成表）：生成 `FeedJoinRow{PostID int64; PublishedAt pgtype.Timestamptz}`，欄位被檢查（`r.bogus` 報錯）。
  - 同一個設定下 `JOIN seller.store_exposures` 報 `relation "store_exposures" does not exist`。

---

## 審查處理（critique-v2）

| 發現 | 處理 |
|---|---|
| F1 HIGH 曝光與 store_state 不是強一致 | 採納。量測輸入哪些靠指令、哪些靠時間，以及公告與對帳的機制、延遲上界、目前的消費者（§4.1）。列出四條路徑，建議 (i)「seller 的 SQL 是規則唯一的家，Go 讀它，時間由請求傳入」，附代價與擁有者（10e）。P1 依路徑回到產品決策（§7）。§0、§1、§4.2 的「強一致」改成有條件 |
| F2 HIGH「公開方法不得自己開 transaction」與 128 處矛盾 | 採納。換成窄規則：`InTx` 可重入可用；跨模組入口不得用 `InReadSnapshot`；撞鍵後不得繼續下 SQL。量化：118／10，現在跨模組路徑上 0 處，計畫中 1 處（`create.go:278`），23505 兩處中一處要改（§3）。撤回「沒有程式要重寫」 |
| F3 MED `SELECT FOR UPDATE` 冪等不成立 | 採納。刪掉 FOR UPDATE；跨模組入口不收 request_id，冪等由呼叫方的狀態轉移擁有（plan:291）；seller 自己的路徑「先鎖 stint 再查」，插入用 `ON CONFLICT`，依 §十二（§3） |
| F4 MED 鎖順序定義不清 | 採納。改成依資源層級**由下往上**：identity 帳號是錨點，編排者的列最後；附條文與三個流程的核對，明文取代 A7-1c (b)（§3） |
| F5 MED 強制機制有更便宜的做法 | 採納。改用 `pub.sql` 契約檔、arch 測試、欄位一致的契約測試，不要 parser；補上 sqlc 對 `RETURNS TABLE` 函式的限制與 `SETOF <relation>` 的解法（實測）；`gen-monolith-sqlc.sh` 先全部 stage 列入成本（§1、§5） |
| F6 MED 誇大 Jovanović | 採納。如實引用（public schema、GRANT、dashboard／admin 範圍、與讀取模型並列），標明本文是延伸；GRANT 列為選項，以 D4 單一 role 的理由否決（§1） |
| F7 MED 三個缺口 | 採納。第 11 列改由 `MeasureCreated` 觸發 A8 通知（附 P3）；`pub_content_takedowns` 標明依賴 A7-D7（未決）；A7-1a 驗收加 EXPLAIN 條件（§3、§4.2、§5） |
| F8 LOW Atlas 事實 | 採納。納入審查的 view 實測，另補函式被略過的實測；守門明定為 schema 一致性測試加上 Postgres 依賴追蹤（函式一律 `BEGIN ATOMIC`，實測），integration CI 就是守門，不另做文字比對（§2） |
| F9 LOW Enqueue 數字 | 採納。改成 32（16 在 legacy），刪掉「主力」（§0） |
| F10 LOW 約束標籤 | 採納。Postgres 唯一 DB → SELF-IMPOSED（不在範圍）；賣家條款 §18.4 → SELF-IMPOSED，是產品承諾（§1、§3） |
| F11 LOW parser 措辭與 Grzybek 動機 | 採納。ADR:129 改稱「大致準確、差在措辭」；Grzybek 的拆庫動機標【推】並引用 README 原句（§0） |
| v2 的 A3「函式擋住內聯」（本版自查） | 撤回。實測函式會被內聯（附錄 B1） |
