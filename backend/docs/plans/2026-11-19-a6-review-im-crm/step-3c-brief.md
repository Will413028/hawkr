# 簡報：一方（買家／店家）停用後，上層模組怎麼「立即隱藏、擋新寫入與重放、抹除自己的資料」

證據等級：【實測】跑過指令或讀過程式；【推論】由程式與文件推得；【推測】依記憶的業界資料，沒有上網查證。
修訂 v2（依獨立審查）：守門機制改成列鎖，補上 lockorder 涵蓋、im 的 RPC、CLOSED 時由誰拿獨占鎖，crm 重放改標 SELF-IMPOSED，檔數更正為約 70。

## 結論先講

review、im、crm 三份複本的必要條件**不成立**：它們只在查詢時過濾、在寫入時把關，沒有在資料庫端存旗標。repo 規則 2 對這種情況的規定是 join 擁有者的 `pub_*`。只有 social 的「買家」複本有必要條件（存起來的旗標加上「函式只能參照自己的表」）。social 處理「店家」時本來就沒有複本，讀取時用 `seller.pub_exposable_stores` 擋（`social/social.go:672`）。建議如下：

- 事實與串行化收回擁有者。守門用**擁有者表上的列鎖**：消費者取 `FOR KEY SHARE`，擁有者的狀態轉移持 `FOR UPDATE`。另外發布 `pub_*` 狀態函式給查詢過濾用。
- review、im、crm 刪掉 `inactive_*` 表與 advisory lock。
- 抹除 job 留在各模組。
- social 不動。
- 不做平台框架。

## 0. 前提問題：這些程式有排定要重寫、搬移或刪除嗎？

| 程式 | 排程 | 來源 | 分類 |
|---|---|---|---|
| crm 投影、ledger、legacy consumer | ordering 搬遷時改讀 ordering 的 `pub_*` | `backend/docs/plans/2026-11-19-a6-review-im-crm.md:75,79,125` | SELF-IMPOSED（Java ordering 是我們的程式） |
| crm 複本的「擋刪後重放」用途 | 投影退場時跟著消失 | 同上【推論】 | SELF-IMPOSED |
| review、im、social 的停用機制 | 沒有排程 | `rg -n "退場\|ordering 搬遷" backend/docs/plans/2026-11-19-a6-review-im-crm.md`【實測】 | — |

現在改的話，ordering 搬遷時只會丟掉 crm 投影端那一次守門呼叫，約 1–2 個檔【推論】。

## 1. 業界基準【推測，依記憶】

- Shopify 的 componentization 在同一個 DB 裡運作，元件之間經公開 API 讀取，不複製資料。
- Kamil Grzybek 的 modular-monolith-with-ddd 與 Microsoft eShop 會複製資料，但它們的前提是「模擬或已經拆庫」。Fowler 的 Event-Carried State Transfer 也說，複製換到的是自主性，代價是一致性與重複。
- GDPR：持有資料的系統各自刪除。crypto-shredding 用在不能改寫的儲存，我們的資料不在那種地方。
- Postgres 列鎖相容表【推測，應以官方文件 explicit-locking 確認】：`FOR KEY SHARE` 只和 `FOR UPDATE` 衝突，不和 `FOR NO KEY UPDATE`（一般的 UPDATE）衝突。多個交易同時持有共享列鎖時，Postgres 會建 multixact（記錄「同一列被多個交易鎖住」的內部結構）。
- 基準形狀：事實只有擁有者一份。消費者在自己的 transaction 內讀擁有者的現況並取擁有者給的共享守門，擁有者轉移時取獨占鎖並發事件，消費者收到事件只排自己的抹除 job。

## 2. 現有機制為什麼存在

- **social 買家複本**：ADR `docs/adr/2026-10-23-social-counts-in-stats-tables.md` 修訂二寫了必要條件：資料庫端存的旗標依賴別的模組的事實。這個條件**仍成立**。
- **規則 2**（`backend/docs/dev-guidelines/monolith-development-guidelines.md:17`）：要用下層的屬性過濾時 join `pub_*`；只有資料庫端旗標或計數依賴別的模組時才複製。
- **review**：`inactive_authors` 只用在 `public.sql` 和 `own.sql` 的 `NOT EXISTS` 過濾【實測】。`LockAuthor` 的理由「Submit 沒有可鎖的列」不成立：buyer 已經提供 `LockActive`，它鎖的是買家列，social 正在用。
- **im**（`im/CARRYOVER.md:35`）：理由一是避開跨模組鎖序；理由二是等待刪除的買家仍可傳訊息（W4），而 `LockActive` 只放行營運中的買家。兩者都是擁有者的 API 少一種形狀造成的。
- **crm**（`crm/CARRYOVER.md:28`）：投影端取共享鎖、`Deactivate` 取獨占鎖。這是基準形狀，只是放錯了擁有者。
- **seller**：沒有「已退場」的 `pub_*` 或守門【實測：`rg -n "CREATE FUNCTION" seller/db/pub.sql`】。`pub_exposable_stores(at)` 不能拿來重用：它是「可曝光」，條件包含營運中、上架必要欄位、未受措施限制（`seller/db/objects.sql`）。CLOSED、停權、資料不全的店都不在裡面，但它們還沒退場，im、crm 不能因此把它們當成已停用。

## 3. 現有的獨占鎖在哪裡【實測】

- **買家 PENDING_DELETION 與 CLOSED**：兩條路徑都由 accountdeletion 在同一個 transaction 內先 `LockForDeletion`（`buyers.sql:20-23` 的 `FOR UPDATE`），再 `BeginDeletion` 或 `CloseForDeletion`（`accountdeletion/internal/request/request.go:365-392`）。
- **店家退場**：`RetireDueStores` 用 `FOR UPDATE OF s SKIP LOCKED`（`seller/db/queries/purge.sql:28`）。正被守門鎖住的店，這一輪會被跳過，下一輪再退場；退場的 transaction 不等待，也不會漏退場。

所以 `LockNotClosed`、`LockNotRetired` 不需要另外加獨占鎖：兩個轉移本來就持 `FOR UPDATE`，而 `FOR UPDATE` 和 `FOR KEY SHARE` 衝突。

## 4. 守門機制：選列鎖，不選 advisory lock

| 面向 | 列鎖（`FOR KEY SHARE`／`FOR UPDATE`） | advisory lock（`_shared`／exclusive） |
|---|---|---|
| lockorder 機械檢查 | **有**：`platform/lockorder/classify.go` 依 schema 與表分類列鎖，checker 依依賴表排名（`checker.go:143`）。im 或 crm 的 transaction 先鎖 buyer／seller 的列、再鎖自己的列，就符合由下往上的順序；反了，整合測試會擋 | **沒有**：`classify.go` 的註解寫明「advisory lock 沒有表…也不算」。要涵蓋得另外寫一個分類器，把鍵的前綴對應到模組 |
| 和現有程式一致 | buyer 的 `LockActive` 已經是列鎖（`buyers.sql:25-30` 的 `FOR SHARE`），轉移端也已經持 `FOR UPDATE` | buyer 的 `LockActive`、`LockForDeletion` 要改；seller 退場要加 advisory lock |
| 熱門店家 | 同一間店同時有多個 transaction 持共享鎖時會建 multixact。量級：每則訊息與每筆投影各一個極短的 transaction。系統還沒上線，沒有量測數字；multixact 在每秒數千個同列共享鎖時才是已知的病灶【推測】 | 不建 multixact |
| 和一般更新互相等待 | 用 `FOR KEY SHARE`，不擋店家或買家的資料更新；只擋 `FOR UPDATE` 的轉移 | 不互相影響 |

決定用列鎖。理由是它被 lockorder 機械涵蓋，而且 buyer 現有的程式已經是這個形狀。`LockActive` 可以從 `FOR SHARE` 降成 `FOR KEY SHARE`，這是選做項，一行。多建 multixact 的代價先接受，觸發條件寫在 §8。

## 5. im 的 legacy RPC

守門 `guard` 會呼叫 `Orders.OpenOrderBetween`（legacy orderservice 的網路 RPC，`im/internal/write/send.go:239`），但 `guard` 在 transaction 之外執行（`send.go:45`），transaction 在 `send.go:62` 才開始【實測】。所以擁有者的列鎖只會在 `inTx` 裡面取，不會在持鎖期間打 RPC。

**風險**：日後若把守門移進 transaction（例如為了消除「檢查到寫入之間」的空窗），列鎖就會跨過網路呼叫，熱門店家的退場與其他持鎖者會被拖住。實作時要規定：守門鎖在 `inTx` 內、在 RPC 之後取，取到之後重新檢查狀態。arch 有限制呼叫的檢查（`internal/arch/restricted_calls.go`），可以評估把「legacy client 在 tx 內被呼叫」也加進去【推論，沒有查它現在涵蓋了什麼】。

## 6. 選項

| 選項 | 失敗條件 | 什麼事實會推翻 | 可逆性與成本 |
|---|---|---|---|
| (1) 現狀：各模組一份複本，外加各自的鎖、job、sweep | 每加一個消費者就再做一份；同一個 River 唯一性 bug 要逐份修（A6-2a1 修過 review 那份） | 拆庫重新成為目標 | 不用改；成本隨模組數線性增加 |
| **(2) 擁有者是唯一來源，用列鎖守門（建議）** | 量到熱門店家的 multixact 或鎖等待有問題 | 某個消費者需要資料庫端存的旗標（那就走 social 的形狀） | 可逆：加回表與訂閱者即可。約 70 檔（§7） |
| (3) 平台級 lifecycle 框架 | 每個消費者在不同時點觸發，抹除的形狀也各不相同，框架會長出一堆參數；而且它還是一份複本 | 出現 ≥3 個觸發時點與抹除形狀都相同的消費者 | 最貴 |
| (4) crypto-shredding；soft-delete 加 RLS | 前者不適用（資料不在不能改寫的儲存）。後者的 policy 一樣要參照擁有者狀態，抹除還是得做 | — | — |

選項 (3) 的否決證據：「觸發時點各不相同」是看各模組的訂閱者得出的【實測：`rg -n "BuyerDeletionRequested|BuyerDeleted\b|StoreRetired" {social/social.go,review/review.go,im/im.go,crm/crm.go}` 以及各 CARRYOVER】。

- social、review：買家在申請刪除時就隱藏（訂 `BuyerDeletionRequested`）；
- im、crm：買家到 CLOSED 才算（`im/im.go:255`）；
- 店家：im、crm 在退場時處理，social 只排抹除 job，可見性靠 `pub_exposable_stores`。

抹除的形狀也不同：

- social 要依序鎖 stats 再重新求值；
- review 每批要發 `ReviewAggregateChanged`；
- im 是把訊息標成抹除（`erased_at`），不刪列；
- crm 是分批刪除。

## 7. 約束帳

| 約束 | 分類 | 來源 |
|---|---|---|
| 函式不能參照別的模組 | SELF-IMPOSED（已重新分類），只影響 social，social 的複本滿足它 | `docs/adr/2026-09-27-module-internal-structure.md:109` |
| 跨模組鎖序 identity → buyer → seller → 上層 | 正確性要求，不推離基準：擁有者的列鎖本來就排在上層之前，lockorder 會檢查 | 規則 4、`platform/lockorder` |
| 等待刪除的買家仍可用 IM（W4） | REAL（產品決定）→ 擁有者要多一種「還沒關閉」的守門 | `im/CARRYOVER.md:19` |
| legacy 事件在刪除之後才送到（重放） | **SELF-IMPOSED**：這是 Java ordering 還沒搬的後果，Java 是我們的程式。在它搬之前，擁有者的狀態檢查一樣擋得住 | plan :125 |
| 「和 social 一致」「已經做完」「之後再說」 | SELF-IMPOSED | — |
| 相容性 | 不是約束（系統從未上線） | 根目錄 `AGENTS.md` |

## 8. 建議與改動量

- **buyer**：新增 `LockNotClosed`（`FOR KEY SHARE`，放行 ACTIVE 與 PENDING_DELETION），以及 `pub_closed_buyers()`（或者讓 review 直接用既有的 `pub_inactive_buyers`，因為 review 本來就在申請刪除時隱藏）。`LockActive` 降成 `FOR KEY SHARE` 是選做。
- **seller**：新增 `LockNotRetired`（`FOR KEY SHARE` 加 `retired_at IS NULL`）與 `pub_retired_stores()`。轉移端不用改（§3）。
- **review**：`Submit` 改用 `buyer.LockActive`；查詢 join `pub_inactive_buyers`；刪掉 `inactive_authors` 與 `LockAuthor`。
- **im**：`Send` 在 `inTx` 內依買家、店家的順序取兩個守門；查詢 join 兩個 `pub_*`；刪掉 `inactive_parties` 與 `LockParty`。
- **crm**：投影改取兩個守門；刪掉複本與 advisory lock。
- **三個模組**：抹除 job 保留，保底掃描改讀 `pub_*`；保底掃描的 River 唯一性設定抽成共用 helper。

**改動量**【實測：`rg -l -i "inactive_(authors|parties)|InactiveAuthor|InactiveParty|LockParty|LockAuthor" <模組> | wc -l`，含測試】：

| 項目 | 檔數 |
|---|---|
| review、im、crm（21、22、17） | 60 |
| buyer、seller 的 API、`pub.sql`、`objects.sql`、migration、`pub` 契約測試 | 約 8 |
| 三份刪表的 migration 與 `atlas.sum` | 約 3–6 |
| 合計 | **約 70** |

social 的 34 檔不動。lockorder 不用新增分類器。

## 9. 終態與觸發條件

- **終態**：「停用」的事實與守門只在 buyer、seller。上層用 join `pub_*` 過濾，寫入時取擁有者的 `FOR KEY SHARE` 列鎖（由 lockorder 機械檢查），收到事件只排自己的抹除 job。只有存旗標的 social 保留買家複本並做每日對帳。
- **觸發條件**：
  - 量到同一列的 multixact 增長或守門等待有問題 → 對那個擁有者改用 advisory lock，並同時為 lockorder 加 advisory 鍵的分類器；
  - 第 5 個消費者，而且形狀和既有的相同 → 重新評估平台框架；
  - ordering 搬遷 → 拿掉 crm 投影端的守門。

## 未查證

- 業界資料與 Postgres 鎖相容表是依記憶寫的。
- multixact 沒有量測。
- `LockNotClosed`、`LockNotRetired` 是草案名稱。
- `restricted_calls` 現在涵蓋了什麼沒有查。
