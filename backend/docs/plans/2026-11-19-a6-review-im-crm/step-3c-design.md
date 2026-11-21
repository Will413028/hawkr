# A6-3c 停用身分改由擁有者守門：實作設計 v1（A6-3c0 設計書，待開工 design-review）

上層：計畫 `../2026-11-19-a6-review-im-crm.md` 的「A6-3c」與 A6-D19；輸入簡報 `step-3c-brief.md`（Will 2026-11-21 選定方向）。路徑相對於 `backend/`；`M=internal/modules`、`P=internal/platform`。標記：【量】＝打開檔案、跑過指令或讀過官方文件；【推】＝讀程式推論；未查證＝沒有查到來源。核對時的 HEAD：dev `492151cd`。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

| 程式 | 排程 | 來源 | 分類 |
|---|---|---|---|
| crm 投影與 legacy consumer 端的守門呼叫 | ordering 搬遷時投影退場，守門跟著刪 | 計畫「延後」、`step-3-design.md` §0【量】 | SELF-IMPOSED（Java ordering 是我們的程式） |
| review、im、crm 的 `inactive_*` 複本與 advisory lock | 本步刪除 | A6-D19 | — |
| buyer、seller 的守門與狀態 `pub_*` | 長期：它們是「停用」事實的唯一來源 | A6-D19 | — |
| social 的 `inactive_authors` 與每日對帳 | 不動（存旗標，規則 2 例外） | 計畫 A6-3c「不能動」 | — |

結論：新增的 buyer、seller API 是長期介面，照長期標準設計；只有 crm 投影端那兩次守門呼叫是過渡程式（約 1–2 檔【推】）。

## 0.1 範圍

- buyer：`LockNotClosed`、`pub_closed_buyers()`；seller：`LockNotRetired`、`pub_retired_stores()`；各自的 migration、`pub.sql`、契約測試。
- review、im、crm：寫入路徑在 transaction 內取擁有者守門；讀取改 join `pub_*`；訂閱者只排抹除 job；保底掃描改讀 `pub_*`；刪三張複本表。
- sweep 的 River 唯一性選項抽成共用 helper（§5）。
- `internal/arch` 與 `platform/lockorder` 的登記檢查（§6）、測試與 mutation（§8）、各模組 `CARRYOVER.md`／`AGENTS.md`／`docs/modules/*.md` 更新。

## 0.2 不在範圍

- social 的 `inactive_authors`、每日對帳、`LockActive` 的社群呼叫端（只在 W3 選 (a) 時受 `LockActive` 鎖強度影響）。
- 各模組抹除 job 的刪除範圍與批次（review 每批發 `ReviewAggregateChanged`、im 標 `erased_at`、crm 分批刪）。
- im「刪帳申請中仍可聊天」語意（`im/CARRYOVER.md:19`；本步以守門只擋 CLOSED 保留它，§3.2）。
- accountdeletion 的依賴表（不認識 review、im、crm，A6-D9）；平台級 lifecycle 框架（簡報選項 3 已否決）。
- Connect handler、client、gateway（A6-5 之後）。

## 0.3 決策總表（只寫落點）

| 來源 | 決定 | 本檔落點 |
|---|---|---|
| A6-D19 | 事實與守門只在擁有者；消費者 `FOR KEY SHARE` 守門、擁有者轉移持 `FOR UPDATE` | §1、§2 |
| 簡報 §4 | 守門用列鎖，不用 advisory lock（lockorder 能機械涵蓋） | §6 |
| 簡報 §8 | review 重用 `pub_inactive_buyers`（驗證結果：語意相符，見 §1.3） | §1.3、§3.1 |
| 簡報 §5 | im 守門鎖在 `inTx` 內、RPC 之後取，取到後重查狀態 | §3.2 |
| 單體規則 2 | 跨模組讀取 API 優先，再 `pub_*`；`pub.sql` 只放形狀 | §1 |
| 單體規則 4 | 鎖由下往上：identity → buyer → seller → 上層 | §3、§6 |
| 單體規則 6 | migration 只往前；已進 dev 的不得改 | §4 |

## 0.4 查證過的現況【量】

| 事實 | 證據 |
|---|---|
| review、im、crm 已依賴 buyer、seller，依賴表不用改 | `internal/arch/modules.go:28-30` |
| `buyer.LockActive` 是 `FOR SHARE`，放行 ACTIVE；PENDING_DELETION 回 FailedPrecondition，CLOSED 與不存在回 NotFound | `buyer/buyer.go:219-225`；`buyer/internal/stint/stint.go:32-47`；`buyer/db/queries/buyers.sql:25-30` |
| 買家的業務寫入（收藏、追蹤、地址、profile、封鎖）先 `stint.Require`＝`LockBuyer`＝`FOR UPDATE` | `buyer/internal/stint/stint.go:11-27`；`buyers.sql:18-23`；呼叫處 `rg -n "stint.Require" buyer` |
| 買家轉移：`BeginDeletion`、`CloseForDeletion` 都先 `LockBuyer`（`FOR UPDATE`）再改狀態 | `buyer/internal/deletion/deletion.go:36-42,52-56,82-86` |
| 店家業務寫入先 `stint.Lock`＝`LockStore`＝`FOR UPDATE`（單列）；關店與刪帳路徑用 `LockAccountStores`（帳號全部 stint，`FOR UPDATE`） | `seller/internal/stint/stint.go:20-33,42-58`；`seller/db/queries/stores.sql:50-54,177-184` |
| `seller.LockForDeletion` 鎖帳號全部店家列 | `seller/seller.go:590-593`；`seller/internal/closing/closing.go:67-70` |
| 店家退場：一個 transaction 內 `RetireDueStores`（`FOR UPDATE OF s SKIP LOCKED`）→ 再判斷 → `RetireStores` → 發 `StoreRetired` | `seller/db/queries/purge.sql:28`；`seller/internal/purge/purge.go:107-175` |
| 恢復營業路徑也呼叫 `RetireDueStores`，但呼叫方已鎖帳號全部 stint，自己的鎖不會被 SKIP | `seller/internal/closing/closing.go:292,335`（docstring「呼叫方已鎖住帳號列與帳號全部的 stint」） |
| accountdeletion 在派發事件前先鎖買家、店家，再 `BeginDeletion`／`CloseForDeletion` | `accountdeletion/internal/request/request.go:365-392` |
| `pub_inactive_buyers()`：`buyer.buyers` 中 `status <> 'ACTIVE'`（PENDING_DELETION 加 CLOSED），無時間參數 | `buyer/db/objects.sql:9-14`；`buyer/db/pub.sql:19-21` |
| seller 沒有「已退場」的 pub 或守門；`pub_exposable_stores(at)` 是「可曝光」，不能重用 | `seller/db/pub.sql`（`rg -n "CREATE FUNCTION" seller/db/pub.sql`：`pub_exposable_stores`、`pub_review_items`、`pub_content_takedowns`、`pub_lock_content_takedowns`） |
| seller 已有帶鎖的 pub 先例：`pub_lock_content_takedowns`（VOLATILE，`FOR SHARE`） | `seller/db/pub.sql:69`；`docs/modules/seller.md` §21 |
| 退場的事實欄位是 `seller.stores.retired_at`；索引 `stores_retire_pending` | `seller/db/schema.sql:58,108` |
| 買家、店家列在 CLOSED／退場後仍保留（pub 函式依賴列存在；`ClearPII` 只更新欄位） | `buyer/db/queries/buyers.sql:128-130`；`seller/db/schema.sql:63-68`【推】 |
| im 的 `Send`：`guard`（含 `Orders.OpenOrderBetween` 網路 RPC）在 `send.go:45` 呼叫、`inTx` 在 `send.go:62` 才開；tx 內先 `LockParty` 雙方、再 `IsInactiveParty`、`AllocateSeq` | `im/internal/write/send.go:45,62-86,239` |
| im 買家只在 CLOSED 才算 inactive（`OnBuyerDeleted`，不訂 `BuyerDeletionRequested`）；店家在退場 | `im/im.go:254-264`；`cmd/hawkr/im.go:24-25` |
| review 買家在 `BuyerDeletionRequested` 與 `BuyerDeleted` 都算 inactive | `review/review.go:211-224`；`cmd/hawkr/review.go:28-29` |
| crm 買家 CLOSED、店家退場；投影先 `LockPartyShared`（買家、店家）再 `AnyInactiveParty` | `crm/crm.go:188-205`；`crm/internal/project/project.go:89-102` |
| crm 標籤與客戶寫入只讀 `inactive_parties`，不取身分鎖（只有 `LockStoreTags` 這把 advisory lock，鎖的是標籤命名空間，與 `Deactivate` 不互斥） | `crm/internal/write/write.go:100-136`；`crm/db/queries/tags.sql:3-7,34-35` |
| lockorder：`Waits` 的列鎖才算持有；`SKIP LOCKED`、`NOWAIT` 不算等待的一邊；advisory lock 不分類 | `P/lockorder/classify.go`（檔頭註解）；`P/lockorder/checker.go:114-166` |
| lockorder 檢查器對「測試檔開的 transaction」整個跳過 | `P/lockorder/checker.go:117-119`（`byTest`） |
| 整合測試的 DB 掛著 lockorder 檢查器，單體程式開的 transaction 違規會讓測試失敗 | `P/testdb/testdb.go:274-301` |
| `Modules` 順序：buyer、seller 在 review、im、crm 之前 | `P/lockorder/lockorder.go`（`Modules`） |
| 共用 helper 的位置：`P/jobs` 已 import `river`、`rivertype` | `P/jobs/jobs.go:12-21` |
| 三份 sweep 唯一性選項各寫一份；另有 im 存活標記清理與 media refsync | `review/internal/purge/purge.go:250-252`；`im/internal/purge/purge.go:203-205`；`crm/internal/purge/purge.go:203-205`；`im/internal/live/sweep.go:19-21`；`media/internal/refsync/refsync.go:325-327` |
| 同一份「未完成狀態」五個值的清單散在 10 個檔案 | `rg -c "rivertype.JobStateScheduled" --glob '!*_test.go' M P`：crm、im、media、payment、review、seller（兩處）、social（兩處）、`P/inbox` |
| Postgres 列鎖衝突表：`FOR KEY SHARE` 只與 `FOR UPDATE` 衝突；不衝突於 `FOR NO KEY UPDATE`；一般 `UPDATE`（不改被外鍵參照的唯一鍵欄位）取 `FOR NO KEY UPDATE`，改那類欄位或 `DELETE` 取 `FOR UPDATE` | 官方文件 explicit-locking §Row-Level Locks，WebFetch 2026-11-21【量】 |
| `SKIP LOCKED` 的語意（跳過無法立即鎖到的列） | 官方文件這一頁沒有寫；`SELECT` 頁沒有查。未查證，依 `FOR UPDATE` 與 `FOR KEY SHARE` 衝突【推】 |

### 0.4.1 與簡報不符之處

1. 簡報 §4 表格最後一列寫「`FOR KEY SHARE` 不擋店家或買家的資料更新；只擋 `FOR UPDATE` 的轉移」。**業務寫入本身就持 `FOR UPDATE`**（買家 `stint.Require`、店家 `stint.Lock`，見上表），所以守門的 `FOR KEY SHARE` 會與每一個買家或店家的業務寫入互相等待，不只和刪帳、退場。LockActive 現況（`FOR SHARE`）本來就有同樣的互等，但 im 的每則訊息與 crm 的每筆投影是新增的熱路徑。見 W4。
2. 簡報 §8 的 im 檔數 22 含 `LockPartyStreams`（im 自己的存活標記鎖，`live/streams.go`、`presence.sql`）的假命中。排除後 im 是 18（§7）。
3. `pub_inactive_buyers` 的現有契約寫「只給 social 每日對帳，不給求值或旗標呼叫」（`buyer/AGENTS.md:15`、`pub.sql:18`、`docs/modules/buyer.md:88`）。review 的每支公開讀取都 join 它，等於改契約，必須同一次變更改這些說明（§1.3）。

## 1. buyer、seller 新增的 API（完整清單）

### 1.1 buyer

**根套件（`buyer/buyer.go`，緊接 `LockActive` 之後）：**

```go
// LockNotClosed 在呼叫方的 transaction 內以 FOR KEY SHARE 確認買家尚未關閉（ACTIVE 與 PENDING_DELETION 都放行），鎖持有到 transaction 結束：
// accountdeletion 的 FOR UPDATE（BeginDeletion、CloseForDeletion）要等這個 transaction 提交。已關閉與不存在回 NotFound。必須在 transaction 內呼叫。
func (module *Module) LockNotClosed(ctx context.Context, buyerID int64) error
```

- 轉到 `profile.Service.LockNotClosed`（比照 `profile.go:122-132`：`model.Positive`、`db.TxFrom` 檢查、沒有 tx 回 `errors.New("buyer：LockNotClosed 必須在 transaction 內呼叫")`）→ `stint.RequireNotClosedShared`（比照 `stint.go:32-47`，只保留「CLOSED 或 `PiiClearedAt != nil` 回 NotFound」）。
- 錯誤分類：closed 與 missing 都是 `apperr.NotFound("買家已關閉")`，與 `LockActive` 相同；消費端以 `apperr.KindOf` 分辨「已停用」與「查詢失敗」（W1 討論 missing 的處理）。
- `LockActive` 簽名不變（review 使用，§3.1）。

**sqlc（`buyer/db/queries/buyers.sql`）：**

```sql
-- name: LockBuyerKeyShare :one
-- 以 KEY SHARE 讀取指定 stint：只與 FOR UPDATE（BeginDeletion、CloseForDeletion）互斥，與別的共享鎖、一般 UPDATE 不互斥。
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR KEY SHARE;
```

- 與 `LockBuyerShared`（`buyers.sql:25-30`，`FOR SHARE`，`stint.go:36` 與 `block/block.go:63` 使用）的關係見 §1.5 與 W3：若選 (a) 把 `LockBuyerShared` 本身改成 `FOR KEY SHARE`，就不新增 `LockBuyerKeyShare`，`LockNotClosed` 與 `LockActive` 共用一支查詢（一個定義）。

**pub：**

`buyer/db/pub.sql` 新增（形狀宣告）：

```sql
CREATE TABLE buyer.pub_closed_buyer (
    buyer_id BIGINT NOT NULL
);
-- 此刻已關閉（CLOSED）的買家，無時間參數；不含刪除中。買家關閉不可逆，列永久保留。
CREATE FUNCTION buyer.pub_closed_buyers() RETURNS SETOF buyer.pub_closed_buyer
LANGUAGE sql STABLE
AS $$ SELECT * FROM buyer.pub_closed_buyer $$;
```

`buyer/db/objects.sql` 與手寫 migration（定義，單體規則 6）：

```sql
CREATE TYPE buyer.pub_closed_buyer AS (buyer_id BIGINT);

CREATE FUNCTION buyer.pub_closed_buyers() RETURNS SETOF buyer.pub_closed_buyer
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT b.buyer_id
    FROM buyer.buyers b
    WHERE b.status = 'CLOSED';
END;
```

migration 名：`<時間戳>_buyer_pub_closed_buyers.sql`（`atlas migrate new buyer_pub_closed_buyers --env dev` 手寫、`atlas migrate hash`，比照 `buyer/db/migrations/20261022175334_buyer_pub_inactive_buyers.sql`）。

**契約測試：**

- `cmd/hawkr` 的 `TestPubContractsMatchMigrations`（`cmd/hawkr/main_integration_test.go:133`）自動涵蓋新函式（`pub.sql` 與 migration 雙向比對），不用改測試，只要兩邊都寫。
- buyer 新增 `TestClosedBuyersAndLockNotClosed`（`buyer/inactive_integration_test.go`，與 `TestInactiveBuyersAndDeletionRequestedEvent`、`TestLockActiveSerializesWithBeginDeletion` 並列）：ACTIVE、PENDING_DELETION、CLOSED、不存在各一，驗 `pub_closed_buyers()` 只含 CLOSED；`LockNotClosed` 在無 tx 時拒絕、ACTIVE 與 PENDING 放行、CLOSED 與不存在 NotFound。
- buyer 新增 `TestLockNotClosedBlocksBeginDeletionUntilCommit`：持 `LockNotClosed` 的 tx 未提交時，另一條 `BeginDeletion` 與 `CloseForDeletion` 的 tx 等待（比照 `TestLockActiveSerializesWithBeginDeletion` 的並發寫法）；提交後轉移完成，下一次 `LockNotClosed` 對 CLOSED 回 NotFound。

### 1.2 seller

**根套件（`seller/seller.go`，緊接 `LockForDeletion` 之後）：**

```go
// LockNotRetired 在呼叫方的 transaction 內以 FOR KEY SHARE 確認店家尚未退場（retired_at 為 NULL；營運中、關店、等待刪除都放行），鎖持有到 transaction 結束：
// 退場的 RetireDueStores 是 FOR UPDATE SKIP LOCKED，正被守門鎖住的店這一輪被跳過、下一輪再退場。已退場與不存在回 NotFound。必須在 transaction 內呼叫。
func (m *Module) LockNotRetired(ctx context.Context, storeID int64) error
```

- 實作放 `seller/internal/stint`（與 `Lock` 同層）：`stint.RequireNotRetiredShared(ctx, q, storeID)`，`model.Positive`、`db.TxFrom` 檢查同 buyer。
- 為什麼不重用 `stint.Lock`：它是 `FOR UPDATE` 且對 CLOSED 回 NotFound（`stint.go:20-33`），語意是「寫入需要店家營運中」，不是「尚未退場」；兩者不能共用。

**sqlc（`seller/db/queries/stores.sql`，緊接 `LockStore` 之後）：**

```sql
-- name: LockStoreKeyShare :one
-- 以 KEY SHARE 讀取指定 stint：只與 FOR UPDATE（RetireDueStores、LockAccountStores）互斥。
SELECT *
FROM seller.stores
WHERE store_id = $1
FOR KEY SHARE;
```

**pub：**

```sql
-- pub.sql 形狀
CREATE TABLE seller.pub_retired_store (
    store_id BIGINT NOT NULL
);
CREATE FUNCTION seller.pub_retired_stores() RETURNS SETOF seller.pub_retired_store
LANGUAGE sql STABLE
AS $$ SELECT * FROM seller.pub_retired_store $$;
```

```sql
-- objects.sql 與 migration（定義）
CREATE TYPE seller.pub_retired_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_retired_stores() RETURNS SETOF seller.pub_retired_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT s.store_id
    FROM seller.stores s
    WHERE s.retired_at IS NOT NULL;
END;
```

- 語意：已退場（第一段清理完成、`StoreRetired` 已發）的店家；不含只是 CLOSED 但可恢復營業的店（`seller/db/queries/stores.sql` 恢復營業只作用於未退場列，`purge.sql` 註解）。與 `pub_exposable_stores`（可曝光）互不重用。
- migration 名：`<時間戳>_seller_pub_retired_stores.sql`。
- 索引：`stores_purge_pending (store_id) WHERE retired_at IS NOT NULL AND pii_purged_at IS NULL`（`seller/db/schema.sql:109`）只涵蓋第二段未完成的店；已全部清完的退場店不在索引裡。讀取是「依 `store_id` 查是否退場」走主鍵，不需要新索引；sweep 掃全部退場店見 W5。

**契約測試：** `TestPubContractsMatchMigrations` 自動涵蓋；seller 新增 `TestRetiredStoresAndLockNotRetired`（ACTIVE、CLOSED 未退場、PENDING_DELETION、已退場、不存在）與 `TestLockNotRetiredSkipsTheStoreInRetireBatch`（持守門的 tx 未提交時 `retireBatch` 不選那家店、也不阻塞，提交後下一輪退場並發 `StoreRetired`；比照 `seller/purge_integration_test.go` 現有的 `afterCandidates` hook 寫法）。

### 1.3 review 是否重用 `pub_inactive_buyers`：重用【量】

- 簡報 §8 的前提是 review 在申請刪除時就隱藏。核對：`OnBuyerDeletionRequested` 與 `OnBuyerDeleted` 都呼叫 `Deactivate`（`review/review.go:211-224`），所以 review 的「inactive」＝PENDING_DELETION 加 CLOSED＝`status <> 'ACTIVE'`，與 `pub_inactive_buyers()` 的定義逐字相同（`buyer/db/objects.sql:9-14`）。守門是 `LockActive`（只放行 ACTIVE）。不另建 pub。
- 同次變更要改的契約文字：`buyer/db/pub.sql:13-18` 的註解（把「給 social 對帳」改成「social 對帳；review 的讀取過濾」）、`buyer/db/objects.sql` 同段註解、`buyer/AGENTS.md:15`、`docs/modules/buyer.md:88`、`buyer/CARRYOVER.md:105`。`TestInactiveBuyersAndDeletionRequestedEvent` 不用改。
- im 與 crm 要的是 CLOSED（不含刪除中），不能重用它，所以才新增 `pub_closed_buyers`（W2 討論形狀）。

### 1.4 完整清單

| 模組 | 項目 | 檔案 |
|---|---|---|
| buyer | `Module.LockNotClosed`、`profile.Service.LockNotClosed`、`stint.RequireNotClosedShared` | `buyer/buyer.go`、`buyer/internal/profile/profile.go`、`buyer/internal/stint/stint.go` |
| buyer | `LockBuyerKeyShare`（或 W3 (a) 改 `LockBuyerShared`） | `buyer/db/queries/buyers.sql` 與生成的 `buyers.sql.go` |
| buyer | `pub_closed_buyers()` | `db/pub.sql`、`db/objects.sql`、新 migration、`atlas.sum` |
| seller | `Module.LockNotRetired`、`stint.RequireNotRetiredShared` | `seller/seller.go`、`seller/internal/stint/stint.go` |
| seller | `LockStoreKeyShare` | `seller/db/queries/stores.sql` 與生成的 `stores.sql.go` |
| seller | `pub_retired_stores()` | `db/pub.sql`、`db/objects.sql`、新 migration、`atlas.sum` |
| buyer、seller | 文件 | `AGENTS.md`「發布的讀取介面」、`docs/modules/buyer.md`、`docs/modules/seller.md` §21、鎖順序節 |

### 1.5 選做：`LockActive` 從 `FOR SHARE` 降成 `FOR KEY SHARE`——建議做（W3）

- 現況 `FOR SHARE` 與 `FOR NO KEY UPDATE` 也衝突（官方表【量】）。買家所有 `UPDATE buyer.buyers` 都在 `FOR UPDATE` 的列鎖之後（`UpdateProfile`、`UpdateAvatar`、`Pending`、`Close` 在 `LockBuyer` 之後；`ClearPII` 在 `CleanupBatch` 的 `FOR UPDATE SKIP LOCKED` 之後：`buyers.sql:38-60,102-112,119-132`），所以降成 `FOR KEY SHARE` 不改變任何現有 transaction 的等待集合【推：讀過全部 `UPDATE buyer.buyers` 語句】。
- 好處：buyer 只有一種共享守門查詢，`LockActive` 與 `LockNotClosed` 共用 `LockBuyerShared`（改名或不改名皆可），不留兩個強度。影響面：social 的 `LockActive` 呼叫端、`block.go:63`；守門測試 `TestLockActiveSerializesWithBeginDeletion`、social 的整合測試。建議在 3c1 以獨立 commit 做，社群測試整組重跑。

## 2. 守門與狀態的語意總表

| 模組 | 寫入守門（tx 內） | 讀取過濾 | 為什麼 |
|---|---|---|---|
| review | `buyer.LockActive`（作者） | `NOT EXISTS (… buyer.pub_inactive_buyers() …)` | 申請刪除起就隱藏 |
| im | 買家 `buyer.LockNotClosed`，再店家 `seller.LockNotRetired` | `pub_closed_buyers()`、`pub_retired_stores()` | W4（刪帳申請中仍可聊天）、店家退場才抹 |
| crm | 同 im | 同 im | 買家 CLOSED、店家退場 |

鎖序一律「買家先於店家」（`Modules` 順序 buyer < seller，`P/lockorder/lockorder.go`），之後才是自己模組的列。

## 3. 各模組改點

指令與計數【量】（排除 `LockPartyStreams` 假命中）：

```
cd backend/internal/modules
P='inactive_authors|inactive_parties|LockParty\b|LockPartyShared|LockAuthor|InactiveAuthor|InactiveParty|InactiveParties|IsInactive'
for m in review im crm; do echo "$m $(rg -l -i "$P" $m | wc -l)"; done
```

結果：review 21（生成 5、測試 3、文件／migration／testdata 5、手寫 8）；im 18（生成 3、測試 3、文件／migration 4、手寫 8）；crm 17（生成 4、測試 2、文件／migration 3、手寫 8）。用簡報的寬鬆 pattern（含 `LockParty` 子字串）im 是 22，其中 `live/streams.go`、`presence.sql(.go)`、`live/timesource_test.go` 是 `LockPartyStreams`，不動。

### 3.1 review

| 檔案 | 現況 | 改成 |
|---|---|---|
| `db/queries/public.sql:11,22,40,51,69,75,91,105,116` | 每支查詢 `NOT EXISTS (… review.inactive_authors …)` | `NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() ib WHERE ib.buyer_id = r.buyer_id)`；`sqlc.yaml` 已讀 `../../buyer/db/pub.sql`，不用改 |
| `db/queries/own.sql:37`、`purge.sql:33`（`IsReviewPublic`）、`testdata/public_predicate.sql:2` | 同上述述詞 | 同一句 join；`readpolicy_test.go` 讀 `testdata/public_predicate.sql` 作為唯一述詞，一起改 |
| `db/queries/inactive.sql` | `InsertInactiveAuthor`、`IsInactiveAuthor`、`LockAuthor` | 整檔刪除；提早檢查（`write.go:92`，tx 外，只為省轉正圖片）改成 `db/queries/own.sql` 的 `AuthorInactive :one`（`SELECT EXISTS(… pub_inactive_buyers() …)`）；tx 內不用它 |
| `internal/write/write.go:122-131` | `LockAuthor` + `IsInactiveAuthor` | 刪；tx 內第一句改 `s.d.Buyers.LockActive(ctx, in.BuyerID)`（`write.Buyers` 介面加 `LockActive`，`*buyer.Module` 已有）。FailedPrecondition 與 NotFound 都翻成 `text.ReviewAuthorInactive`（現行行為：`write.go:96-97,129-130`） |
| `internal/purge/purge.go:81-99` | `Deactivate` 持 `LockAuthor`、寫複本、排 job | 改名 `EnqueuePurge(ctx, buyerID)`：只排 job（`enqueuePurge` 已存在，`purge.go:96-99`），不再要求 transaction 內 |
| `internal/purge/purge.go:101-120` + `purge.sql:16-18` | `Sweep` 讀 `ListInactiveAuthorsWithReviews` | 查詢改為 `FROM buyer.pub_inactive_buyers() ib WHERE EXISTS (SELECT 1 FROM review.reviews r WHERE r.buyer_id = ib.buyer_id) ORDER BY ib.buyer_id LIMIT …`（`reviews_buyer` 索引，`review/db/schema.sql:47`）；規模見 W5 |
| `review.go:208-224` | 訂閱者呼叫 `Deactivate` | 改呼叫 `EnqueuePurge`；兩個事件都留（直接 CLOSED 只發 `BuyerDeleted`，`review.go:216-218`）；`SweepPurge` 說明更新 |
| `db/schema.sql` | `review.inactive_authors` | 刪表；`atlas migrate diff review_drop_inactive_authors --env dev` |
| 生成：`internal/store/{inactive,own,public,purge}.sql.go`、`models.go` | | `make sqlc` |
| `internal/text/zh-TW.json` | 命中文字 `inactive`？ | 只有文案 key 名 `ReviewAuthorInactive`，不動（`rg` 的命中來自這個 key） |
| 測試 | `review_integration_test.go`、`purge_integration_test.go`、`readpolicy_test.go`、`accountdeletion/accountdeletion_lockorder_integration_test.go:138` | §8 |

寫入路徑的鎖順序：`buyer.LockActive`（buyer 列）→ `InsertReview`（review 列）。`Submit` 現有的唯一 RPC（`OrderForReview`，`write.go:77`）在 tx 之前，維持。

### 3.2 im

| 檔案 | 現況 | 改成 |
|---|---|---|
| `internal/write/send.go:62-86` | tx 內先 `LockParty` 雙方、`IsInactiveParty` 寄件人與收件人 | tx 內第一組語句：`Buyers.LockNotClosed(ctx, t.buyerID)`，再 `Stores.LockNotRetired(ctx, t.store.StoreID)`（鎖序買家先於店家，與 accountdeletion 的 `lockParticipants` 同序，`request.go:365-375`），**在 `guard` 之後（RPC 已完成）**。取得後依寄件人／收件人角色翻譯：NotFound → 寄件人 `ImSenderInactive`、收件人 `ReasonReceiverDeleted`（現行訊息與 reason 不變，`send.go:75-86`）；非 NotFound 錯誤原樣回傳（fail-closed） |
| `internal/write/write.go:28-45,62-72` | `Buyers`、`Stores` 介面；`inactive` 測試縫 | 介面加 `LockNotClosed`、`LockNotRetired`；`inactive` 縫（G4 tx 外的提早檢查）改成 pub 讀取（`messages.sql` 新增 `PartyInactive`，見下）；單元測試替身補方法 |
| `internal/write/send.go`（G4，`guard` 內） | `s.inactive` | 保留為省 Finalize 的提早檢查（正確性由 tx 內守門保證，註解不變） |
| `db/queries/messages.sql:30-42` | relay 讀取的 `receiver_inactive` 子查詢讀 `im.inactive_parties` | `CASE WHEN m.sender_kind='BUYER' THEN EXISTS(SELECT 1 FROM seller.pub_retired_stores() r WHERE r.store_id = c.store_id) ELSE EXISTS(SELECT 1 FROM buyer.pub_closed_buyers() b WHERE b.buyer_id = c.buyer_id) END` |
| `db/queries/messages.sql:66-76`（`ListInactivePartiesWithMessages`） | 讀複本 | 兩段 `UNION ALL`：`SELECT 'BUYER', b.buyer_id FROM buyer.pub_closed_buyers() b WHERE EXISTS(…conversations JOIN messages…)`、`SELECT 'STORE', s.store_id FROM seller.pub_retired_stores() s WHERE EXISTS(…)`，外層 `ORDER BY kind, party_id LIMIT` |
| `internal/read/read.go:239` | `peerOnline` 讀 `IsInactiveParty` | 新增 `PartyInactive :one`（單句 `EXISTS` 兩側 pub），讀取不取鎖 |
| `db/queries/inactive.sql` | 三支查詢 | 整檔刪除（`PartyInactive` 放 `messages.sql`） |
| `internal/purge/purge.go:76-97` | `Deactivate` 持 `LockParty`、寫複本、排 job | 改名 `EnqueueErase(ctx, kind, id)`，只排 job；`ErasePartyArgs` 的註解（`purge.go:55-61`）改寫：「進入刪除後沒有新訊息」的理由改成「`Send` 在 tx 內持擁有者的 `FOR KEY SHARE`，擁有者轉移的 `FOR UPDATE` 與它互斥」 |
| `im.go:254-264` | `OnBuyerDeleted`、`OnStoreRetired` | 只呼叫 `EnqueueErase`；`cmd/hawkr/im.go:24-25` 訂閱不變（仍只訂 `BuyerDeleted`，不訂 `BuyerDeletionRequested`） |
| `im/db/sqlc.yaml` | 只讀 `schema.sql` | `schema` 加 `"../../buyer/db/pub.sql"`、`"../../seller/db/pub.sql"`（`internal/arch/sqlc.go:18` 的 `pubSchemaPath` 規則；依賴表已允許） |
| `db/schema.sql:88-96` | `im.inactive_parties` | 刪表；`atlas migrate diff im_drop_inactive_parties --env dev` |

**守門鎖與 RPC 的順序（計畫「不能動」）**【量】：`guard`（含 `Orders.OpenOrderBetween`）在 `send.go:45`，`inTx` 在 `send.go:62`，所以守門鎖取在 RPC 之後已是現況，新增的守門只是放在 `inTx` 的第一組語句。實作規定：(1) 兩把守門鎖必須在 `inTx` 內、`AllocateSeq` 之前；(2) 取到鎖之後才算「這次寫入看到的狀態」，不重用 `guard` 內 tx 外的讀取結果（G4 的提早檢查只是優化）；(3) 之後不得把 `guard` 或任何 legacy 呼叫搬進 `inTx`（否則列鎖跨網路呼叫，退場與業務寫入被拖住）。第 3 點的機械化見 W7。

**保留「刪帳申請中的買家仍可傳訊息」（`im/CARRYOVER.md:19`，W4）**：守門用 `LockNotClosed`，PENDING_DELETION 放行；`cmd/hawkr/im.go` 仍不訂 `BuyerDeletionRequested`；讀取過濾用 `pub_closed_buyers()`（不含刪除中）。三處一致，否則等待刪除的買家會被擋。回歸測試見 §8（`TestPendingDeletionBuyerCanStillSend`）。

**抹除與寫入的先後保證**：closing tx 持買家列 `FOR UPDATE` 直到 commit（`request.go:365-392`、`deletion.go:84`），`Send` 持 `FOR KEY SHARE`，兩者互斥。所以 `Send` 若先取得守門，訊息在 closing 之前提交，抹除 job 在 closing 提交之後才可能開始、會涵蓋它；`Send` 若後到，看到 CLOSED 被拒。店家端：退場 tx 對被守門鎖住的店 `SKIP LOCKED`（`purge.sql:28`），這一輪跳過、下一輪再退場，不漏退場（`SKIP LOCKED` 語意未查證，見 §0.4）。

### 3.3 crm

| 檔案 | 現況 | 改成 |
|---|---|---|
| `internal/project/project.go:89-102` | 兩次 `LockPartyShared`、`AnyInactiveParty` | tx 第一組語句：`Buyers.LockNotClosed(ctx, buyerID)`、`Stores.LockNotRetired(ctx, storeID)`（買家先於店家），任一回 NotFound 則 `return nil`（commit 空 tx，行為同現行 `if inactive { return nil }`）並記 log；非 NotFound 回錯誤。其後才是 `LockOrderNet`（crm 列，排在 buyer、seller 之後，lockorder 通過） |
| `crm.go:121-124`、`crm.Deps` | `project.Deps` 只有 `DB`、`Now` | `crm.Deps` 加 `Buyers`、`Stores`（介面：`LockNotClosed`／`LockNotRetired`），`project.Deps` 與 `write.Deps` 各宣告最小介面；`cmd/hawkr` 的組裝與 `crm/fixture_integration_test.go`、`internal/legacy/crmconsumer/consumer_integration_test.go`、`accountdeletion/accountdeletion_lockorder_integration_test.go` 的 `crm.New` 呼叫補參數 |
| `internal/write/write.go:100-136`（`CreateTag`、`RenameTag`、`DeleteTag`）、`AddCustomerTags`、`RemoveCustomerTag`、`SetCustomerType` | 讀 `IsStoreInactive`／`LockCustomer` 內的 `NOT EXISTS (inactive_parties)`，**沒有取身分鎖** | 店家層寫入（`CreateTag`、`RenameTag`、`DeleteTag`）的 tx 第一組語句只取 `LockNotRetired(storeID)`；客戶層寫入（`AddCustomerTags`、`RemoveCustomerTag`、`SetCustomerType`）依序先取 `LockNotClosed(buyerID)`、再取 `LockNotRetired(storeID)`（規則 4：buyer 先於 seller）；之後才是 `LockStoreTags`（advisory）、`LockCustomer`（`FOR UPDATE OF c`，crm 列）。`requireActiveStore` 與 `IsStoreInactive` 刪除，由守門的 NotFound 對應 `tagNotFound`／客戶 NotFound。這同時補上現況的缺口：`tags.sql:3-7` 的 `LockStoreTags` 與 `Deactivate` 不互斥【量：`Deactivate` 只取 `LockParty`，`purge.go:86`】，退場提交後、抹除跑完前，進行中的標籤寫入可能留下資料 |
| `db/queries/customers.sql:9,25,41,57,73,89,105,113,122`、`tags.sql:31,46`（`LockCustomer`） | `NOT EXISTS (… crm.inactive_parties …)` | 買家側 `NOT EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() cb WHERE cb.buyer_id = c.buyer_id)`；店家側 `NOT EXISTS (SELECT 1 FROM seller.pub_retired_stores() rs WHERE rs.store_id = c.store_id)`（列表查詢的 `store_id` 是參數，可寫成對參數的 `EXISTS`，planner 內聯後走主鍵，見 §0.4 未查證） |
| `db/queries/inactive.sql` | 五支查詢 | 整檔刪除 |
| `internal/purge/purge.go:76-111` + `inactive.sql`（`ListInactivePartiesWithData`） | `Deactivate`、`Sweep` 以 `(kind, party_id)` keyset 讀複本 | `Deactivate` 改 `EnqueueErase`；`Sweep` 的查詢改 `UNION ALL`（買家 `pub_closed_buyers()`、店家 `pub_retired_stores()`，各自 `EXISTS (customers／order_nets／tags)`，`customers_by_buyer`、`order_nets_by_buyer` 索引已有，`crm/db/schema.sql:35,77`），外層保留 `(kind, party_id) > cursor` keyset |
| `crm.go:188-205` | 訂閱者與 `SweepErase` 說明 | 只排 job；說明更新 |
| `crm/db/sqlc.yaml` | 只讀 `schema.sql` | 加兩份 `pub.sql`（同 im） |
| `db/schema.sql:90-96` | `crm.inactive_parties` | 刪表；`atlas migrate diff crm_drop_inactive_parties --env dev` |

**重放不重建**：legacy 事件在買家 CLOSED 之後才送到 `RecordOrder`／`RecordRefund` 時，`project.record` 第一句 `LockNotClosed` 對 CLOSED 回 NotFound → 提前 `return nil`，不寫 ledger、`order_nets`、`customers`。這依賴「買家與店家列在關閉與退場後仍保留」（§0.4）：關閉的買家列在，狀態是 CLOSED，所以重放永遠被擋，與抹除 job 是否已跑完無關。重放的是從未存在的買家 ID 時也是 NotFound，處理見 W1。

**投影與退場的互等**：舊設計靠「Deactivate 的呼叫者只拿一把 BUYER 獨占、retireBatch 拿多把 STORE 獨占」論證沒有死結（`crm/db/queries/inactive.sql` 的 `LockPartyShared` 註解）。新設計不需要這個論證：retireBatch 對店家列是 `SKIP LOCKED`（不等待，不進 lockorder 的等待集合，`checker.go:130-160` 的 `Waits`），投影只持兩個 `FOR KEY SHARE` 與自己模組的列。`TestRetiringManyStoresWhileProjectingDoesNotDeadlock`（`crm/inactive_integration_test.go:129`）保留並改成對新機制驗證。

### 3.4 三模組共同

- 刪三張表（`review.inactive_authors`、`im.inactive_parties`、`crm.inactive_parties`）的 migration：三份 baseline 都已進 dev（`git log -- M/review/db/migrations` 見 `398956b9`、`858d1706`、`8526c9b8`），單體規則 6 不許改，所以各加一支往前的 migration：`<時間戳>_review_drop_inactive_authors.sql`、`<時間戳>_im_drop_inactive_parties.sql`、`<時間戳>_crm_drop_inactive_parties.sql`，內容是 `DROP TABLE`（`atlas migrate diff` 產生）加 `atlas.sum`。系統從未上線，複本表沒有需要保留的資料（根目錄 `AGENTS.md`「上線前不受相容性約束」）。
- 訂閱者的 transaction 內工作量只剩一次 `Enqueue`（單體規則 4）。三個訂閱者簽名不變，`cmd/hawkr` 的訂閱不變。
- `cmd/hawkr` 裡「訂閱不得多訂」的測試（`cmd/hawkr/im_integration_test.go:48-59`）斷言「BuyerDeleted 之後才寫複本」，要改成斷言「BuyerDeleted 之後才有抹除 job、之後 `Send` 被擋」，並保留「`BuyerDeletionRequested` 後仍可送出、無 job」。

## 4. 要刪的表與 migration 清單

| 項目 | 名稱 | 說明 |
|---|---|---|
| 刪表 | `review.inactive_authors`、`im.inactive_parties`、`crm.inactive_parties` | 各一支 migration |
| 新 migration（owner） | `<ts>_buyer_pub_closed_buyers.sql`、`<ts>_seller_pub_retired_stores.sql` | 手寫（`atlas migrate new`＋`atlas migrate hash`），Atlas 社群版略過型別與函式（`20261022175334_buyer_pub_inactive_buyers.sql` 檔頭） |
| `atlas.sum` | buyer、seller、review、im、crm | 5 份 |
| 不動 | `social.inactive_authors` | 存旗標 |

migration 順序要求：owner 的 pub migration 先於消費者的讀取（同一次部署都在，順序由 `cmd/hawkr/modules.go` 的模組順序決定【推】）。消費者的 `DROP TABLE` 與消費者查詢的改寫在同一次變更。

## 5. sweep 的 River 唯一性：共用 helper

現況重複【量】：

- 「同週期合併」：`river.InsertOpts{MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByPeriod: SweepInterval}}`，review（`purge.go:250-252`）、im（`purge.go:203-205`）、crm（`purge.go:203-205`）各一份；`SweepInterval = 15 * time.Minute` 也各一份（`review/internal/purge/purge.go:36-37`、`im/…/purge.go:29-30`、`crm/…/purge.go:40-41`）；im 的存活標記清理又一份（`im/internal/live/sweep.go:19-21`）、`media/internal/refsync/refsync.go:325-327` 又一份。
- 「同鍵未完成合併」：`ByArgs: true, ByState: [available, pending, running, retryable, scheduled]`，review（`purge.go:75`）、im（`purge.go:62`）、crm（`purge.go:70`）、social（`post/schedule.go:40`、`purge/purge.go:62,82` 用 `unfinishedStates()`）、payment（`account/jobs.go:16` 用 `openStates()`）、media refsync、seller（兩處，狀態集合可能不同）、`P/inbox`。

**建議位置：`P/jobs`（新檔 `P/jobs/unique.go`）**。理由：`P/jobs` 已 import `river`、`rivertype`（`jobs.go:12-21`）；platform 不依賴任何模組（`internal/arch/arch_test.go:161`），helper 只用 River 型別，不違反依賴規則；模組 import platform 是允許的。不放模組內或 `internal/arch`（arch 不得被模組 import）。

介面草案（簽名不決定細節，3c1 實作時以現有呼叫為準）：

```go
// UnfinishedStates 是 River 唯一鍵要涵蓋的「還沒結束」的狀態（含執行中；River 的規定）。
func UnfinishedStates() []rivertype.JobState
// UniqueByArgs 回傳「同一組參數、尚未結束的 job 合併」的 InsertOpts。
func UniqueByArgs(maxAttempts int) river.InsertOpts
// SweepOpts 回傳「同一個 interval 內的排程與重啟合併」的 InsertOpts（預設 ByState 含 completed，會讓掃描被擋，所以不能只寫 ByArgs）。
func SweepOpts(interval time.Duration) river.InsertOpts
```

範圍（W6）：本步必做 review、im、crm 三份 purge 與 im 的 `live/sweep.go`；social、payment、media、seller 的相同清單是否一併改，是 W6。`P/jobs` 的 helper 要有單元測試（狀態集合必須含 running，因為 River 要求；`ByPeriod` 選項不含 completed）。

## 6. `internal/arch` 與 lockorder 登記

**依賴表（`internal/arch/modules.go`）**【量】：review、im、crm 已列 buyer、seller（第 28-30 行），buyer、seller 不新增依賴。`InternalDependencies`：review、im、crm 的內部套件不 import buyer、seller 根套件（介面在 `write`、`project` 等套件本地宣告，根套件組裝），所以不改；若實作時要在內部套件 import platform 的新 helper，platform 一律允許。**結論：依賴表不改。**

**`sqlc.yaml` 檢查（`internal/arch/sqlc.go:18-97`）**：im、crm 的 `sqlc.yaml` 新增 `../../buyer/db/pub.sql`、`../../seller/db/pub.sql` 必須與依賴表一致（已允許）；review 已讀 buyer 的，不新增 seller。`CheckPubShapeUse`（`sqlc.go:171-180`）要求消費端只呼叫函式、不直接參照 `buyer.pub_closed_buyer` 形狀。

**lockorder（`P/lockorder`）**：

- `Modules` 順序不改（buyer、seller 本來就在 review、im、crm 之前）。
- 新的守門查詢 `LockBuyerKeyShare`、`LockStoreKeyShare` 是 `SELECT … FROM buyer.buyers … FOR KEY SHARE`，`Classify`（`classify.go`，`strengths` 表含 `LCS_FORKEYSHARE`）把它分類成 `{Schema: buyer, Table: buyers, Kind: FOR KEY SHARE, Waits: true}`，不用新增分類器。
- 以 `FOR KEY SHARE` 為準的好處：advisory lock 不被分類（`classify.go` 檔頭），舊的 `LockParty`、`LockPartyShared`、`LockAuthor` 全程在 lockorder 視野之外；新守門是列鎖，反序會被擋。
- 不涵蓋的（`lockorder.go` 檔頭「不在檢查範圍」）：測試檔自己開的 transaction（`checker.go:117-119`）、trigger 與函式內部的 SQL。**含意**：「反序必須失敗」不能用測試檔開 tx 去證明，見 §8.1。

**`restricted_calls.go` 涵蓋什麼**【量】：`RestrictedCalls{Names, AllowedDirs, Why}` 只做「某些方法名只能出現在某些目錄的非測試程式」（`restricted_calls.go:10-20`），現有一條規則 `SuspensionEntries`（`SuspendAccount`、`UnsuspendAccount`）。它是檔案層級、依名稱的 AST 比對，**不看呼叫是否在 transaction 內**，也不看型別。

**「legacy client 在 tx 內被呼叫」檢查可行性**（建議，不決定，W7）：

- (a) AST 版：在 `restricted_calls` 的同一套掃描上新增「`InTx`／`inTx` 的 func literal 內不得出現對 `internal/legacy/...` 型別欄位的方法呼叫」。可行，但只抓得到直接寫在 closure 裡的呼叫；經 helper 函式、介面（im 的 `Orders` 是 `model.Orders` 介面，AST 看不出它是 legacy 實作）間接呼叫抓不到；名稱規則（`OpenOrderBetween` 等）要逐一列。
- (b) 執行期版：整合測試把 `Orders` 換成斷言 `db.TxFrom(ctx)` 為 false 的替身（`im` 現有 `imNoOrders`、`imTestStores` 等替身在 `cmd/hawkr`，`im/fixture_integration_test.go`）。涵蓋間接呼叫，成本低，但只守有測試的路徑。
- 建議：本步做 (b)（im 的 `Send` 路徑，§8.3 `TestSendCallsOrdersOutsideTheWriteTransaction`）；(a) 另案，因為它的偽陽性與漏網都需要判準。

## 7. 檔數估計

指令（【量】，在 `backend/internal/modules`）：

```
P='inactive_authors|inactive_parties|LockParty\b|LockPartyShared|LockAuthor|InactiveAuthor|InactiveParty|InactiveParties|IsInactive'
for m in review im crm; do rg -l -i "$P" $m | wc -l; done    # 21 / 18 / 17
rg -l -i "$P" accountdeletion | wc -l                         # 1（lockorder 整合測試）
rg -l -i "$P" buyer seller | wc -l                             # buyer、seller 本身沒有複本，命中 0 才是預期；buyer 4 檔命中是 pub_inactive_buyers 與 LockActive 的文字，不是複本
```

| 項目 | 檔數 | 備註 |
|---|---|---|
| review、im、crm 命中的檔（含生成、測試、文件） | 56（21＋18＋17） | 簡報寫 60（im 22 含假命中） |
| 其中生成（`make sqlc`，不手改） | 12 | 5＋3＋4 |
| 新增：三份 `crm`／`im` 的 `sqlc.yaml`（im、crm 各 1）、`crm.go`／`crm.Deps` 組裝、`cmd/hawkr` 組裝與測試 | 約 6 | `cmd/hawkr/im.go`、`im_integration_test.go`、`review.go`、`review_wiring_test.go`、crm 的 `fixture_integration_test.go`、`crmconsumer/consumer_integration_test.go` |
| `accountdeletion/accountdeletion_lockorder_integration_test.go` | 1 | 斷言複本列改成斷言 job |
| buyer（`buyer.go`、`profile.go`、`stint.go`、`buyers.sql`＋生成、`objects.sql`、`pub.sql`、migration＋`atlas.sum`、測試、`AGENTS.md`、`CARRYOVER.md`、`docs/modules/buyer.md`） | 約 12 |  |
| seller（`seller.go`、`stint.go`、`stores.sql`＋生成、`objects.sql`、`pub.sql`、migration＋`atlas.sum`、測試、`AGENTS.md`、`docs/modules/seller.md`） | 約 11 |  |
| 三個刪表 migration＋`atlas.sum` | 6 | 每模組 2 |
| `P/jobs/unique.go`＋測試 | 2 | W6 擴大範圍時加 social、payment、media、seller 約 +8 |
| `internal/arch` 與 `P/lockorder` | 0–2 | §8.1 的守門測試放哪裡見該節 |
| 各模組 `CARRYOVER.md`、`AGENTS.md`（review、im、crm） | 6 | §9 |
| 合計 | **約 80（含 12 個生成檔）** | 簡報估約 70；差在 cmd/hawkr、accountdeletion 測試、`P/jobs` 與 seller、buyer 文件 |

social 的 34 檔不動（`rg -l -i "$P" social | wc -l`＝34【量，簡報】）。

## 8. 測試與 mutation

### 8.1 lockorder

| 測試 | 檔案 | 驗什麼 |
|---|---|---|
| `TestGuardQueriesClassifyAsOwnerRowLocks` | `internal/arch`（新檔 `guard_lock_test.go`；讀 `M/buyer/db/queries/buyers.sql`、`M/seller/db/queries/stores.sql`，對 `LockBuyerKeyShare`、`LockStoreKeyShare` 呼叫 `lockorder.Classify`） | 結果恰為 `{buyer, buyers, FOR KEY SHARE, Waits: true}`／`{seller, stores, …}`；有人把它改成 advisory lock 或拿掉 `FOR KEY SHARE`（沒有列鎖）就失敗 |
| `TestCheckerFlagsOwnerGuardAfterConsumerRowLock` | `P/lockorder/lockorder_test.go`（包內，可呼叫未匯出的 `observe`，比照 `TestCheckerFlagsWaitingForALowerModuleAfterLockingAHigherOne`，第 71 行） | 序列：`begin` → `UPDATE im.conversations …`（或 `crm.order_nets`）→ buyer 的 `LockBuyerKeyShare` SQL → 產生一筆 violation（Held 是 im、Then 是 buyer）。**這是「反序要失敗」的證明**：因為檢查器對測試檔開的 tx 整個跳過（`checker.go:117-119`），整合測試無法用測試檔的 tx 製造反序 |
| 每個整合測試的 DB 自動掛檢查器 | `P/testdb/testdb.go:274-301` | 正序（守門在前）通過；mutation「守門移到 `AllocateSeq` 之後」（im）或「移到 `LockOrderNet` 之後」（crm）會在 cleanup 回報 violation，讓 `Send`、`record` 的整合測試失敗 |
| `TestLockOrderIsBottomUpOverTheDependencyTable` | `internal/arch/lockorder_test.go:12` | 不改，仍通過（`Modules` 與依賴表不動） |

### 8.2 每模組：停用後立即看不到、寫入被擋、重放不重建

| 模組 | 測試（新增或改寫） | 檔案 |
|---|---|---|
| review | `TestDeactivationHidesReviewsImmediatelyViaThePubJoin`（改寫 `TestRD4DeletionRequestUncountsTheReviewsWithoutDeletingRows`：`BeginDeletion` 提交的同一刻公開讀取、`StoreRatingStats`、`own` 讀取都看不到；列還在） | `review/purge_integration_test.go` |
| review | `TestSubmitIsRejectedAfterDeletionRequestAndWaitsForTheGuard`（改寫 `TestSubmitWaitsForDeactivationAndIsRejected`、刪除 `TestDeactivateWaitsForAuthorLock`：持 `buyer.LockActive` 的 tx 與 `Submit` 的互斥改由 buyer 列鎖提供；`Submit` 在 `BeginDeletion` 之後回 `ReviewAuthorInactive`） | `review/purge_integration_test.go` |
| review | `TestSubscribersEnqueueOnePurgeJob`（改寫 `TestDeactivationSubscribersInsertOnce`：兩個事件只排一個 job，沒有複本表） | 同上 |
| review | `TestSweepPurgeEnqueuesOnlyInactiveBuyersWithReviewsLeft`（改寫 `TestSweepPurgeEnqueuesOnlyInactiveAuthorsWithReviewsLeft`，來源是 `pub_inactive_buyers`） | 同上 |
| review | `readpolicy_test.go`：公開述詞改成 pub join，仍從 `testdata/public_predicate.sql` 讀唯一一份；`TestViewerBlockPredicateInlinesThePubFunctions`（`review_integration_test.go:93`）加上 `pub_inactive_buyers` 內聯的 `EXPLAIN` 斷言 | `review/readpolicy_test.go`、`review_integration_test.go` |
| im | `TestPartiesCannotSendOrReceiveAfterClosedOrRetired`（改寫 `TestInactivePartiesCannotSendOrReceive`，`im_integration_test.go:401`：買家 CLOSED、店家退場後寄件與收件都被擋，reason 不變；讀取的 `receiver_inactive` 與 `peerOnline` 也要對） | `im/im_integration_test.go` |
| im | `TestSendWaitsForClosingAndIsRejected`、`TestRetireSkipsAStoreHeldByASend`（取代 `TestDeactivateWaitsForPartyLock`，第 578 行：持守門鎖的 `Send` 未提交時，buyer 關閉的 tx 等待；店家 `retireBatch` 跳過它，下一輪退場） | `im/im_integration_test.go`、`seller/purge_integration_test.go` |
| im | `TestPendingDeletionBuyerCanStillSend`（**回歸**，計畫「不能動」：`BeginDeletion` 後買家與店家雙向仍可送、訊息不被抹除、沒有抹除 job；`CloseForDeletion` 之後才擋、才有 job） | `im/im_integration_test.go`；正式組裝版 `cmd/hawkr/im_integration_test.go:48-59`（斷言改成不再查複本） |
| im | `TestSendCallsOrdersOutsideTheWriteTransaction`（`Orders` 替身斷言 `db.TxFrom(ctx)` 為 false，對應 §6 (b)） | `im/im_integration_test.go` |
| im | `TestSweepRequeuesDroppedEraseJobs`（`im_integration_test.go:483`，來源改 pub） | 同上 |
| crm | `TestInactiveBuyerIsInvisibleAndReadOnlyToSellers`、`TestInactiveStoreIsInvisibleAndReadOnlyToSellers`（`inactive_integration_test.go:27,70`，改以 `CloseForDeletion`、退場觸發，不再看複本；**新增**：標籤寫入與 `SetCustomerType` 在退場提交後被擋，補上現況缺口） | `crm/inactive_integration_test.go` |
| crm | `TestReplayAfterBuyerDeletionDoesNotRecreateRows`（`purge_integration_test.go:87`，改成不依賴複本：關閉買家後重放 `order.completed`、退款，ledger、`order_nets`、`customers` 都不重建；抹除 job 尚未跑與已跑完兩種都測） | `crm/purge_integration_test.go` |
| crm | `TestProjectionWaitsForConcurrentClosing`（取代 `TestProjectionWaitsForConcurrentDeactivation`，`purge_integration_test.go:164`） | 同上 |
| crm | `TestRetiringManyStoresWhileProjectingDoesNotDeadlock`、`TestSweepReachesPartiesBeyondOneBatch` | `crm/inactive_integration_test.go:129,172`；來源改 pub、論證改寫 |
| accountdeletion | 三個測試（`accountdeletion_lockorder_integration_test.go:104,167,217`）改斷言「抹除 job 已排」而不是「複本列 = 1」（第 138、205、236 行）；`TestDeletionLocksBuyerAndSellerBeforeSocialRows` 不動 | `accountdeletion/accountdeletion_lockorder_integration_test.go` |

### 8.3 mutation（在複本跑，`scripts/red-proof.py`，不得在工作目錄編輯後還原）

| # | 改回舊寫法 | 預期失敗的測試 |
|---|---|---|
| M1 | review `Submit` 拿掉 `Buyers.LockActive` | `TestSubmitIsRejectedAfterDeletionRequestAndWaitsForTheGuard` |
| M2 | review 公開讀取拿掉 `pub_inactive_buyers` join（任一支查詢） | `TestDeactivationHidesReviewsImmediatelyViaThePubJoin`；`readpolicy_test.go` 的述詞一致性 |
| M3 | im `Send` 拿掉兩把守門鎖 | `TestSendWaitsForClosingAndIsRejected`、`TestRetireSkipsAStoreHeldByASend` |
| M4 | im `Send` 守門移到 `AllocateSeq` 之後 | 所有走 `Send` 的整合測試（lockorder 檢查器 violation，`testdb.go:274-301`） |
| M5 | im 讀取拿掉 `pub_closed_buyers` 或 `pub_retired_stores` join | `TestPartiesCannotSendOrReceiveAfterClosedOrRetired` |
| M6 | im 守門改成 `LockActive`（PENDING_DELETION 被擋） | `TestPendingDeletionBuyerCanStillSend`、`cmd/hawkr` 的 newIM 測試 |
| M7 | crm `record` 拿掉守門（回到不取身分鎖） | `TestProjectionWaitsForConcurrentClosing`、`TestReplayAfterBuyerDeletionDoesNotRecreateRows` |
| M8 | crm 標籤寫入拿掉 `LockNotRetired` | 新增的「退場提交後標籤寫入被擋」 |
| M9 | crm 讀取拿掉 join | 兩個 invisible 測試 |
| M10 | `LockNotClosed` 改成 `FOR SHARE`／`FOR NO KEY UPDATE` 以外的弱鎖（拿掉鎖子句） | buyer `TestLockNotClosedBlocksBeginDeletionUntilCommit` |
| M11 | `LockNotRetired` 拿掉 `FOR KEY SHARE` | seller `TestLockNotRetiredSkipsTheStoreInRetireBatch` |
| M12 | `Send` 把 `guard` 搬進 `inTx` | `TestSendCallsOrdersOutsideTheWriteTransaction` |
| M13 | 守門查詢改 advisory lock | `TestGuardQueriesClassifyAsOwnerRowLocks` |

計畫的三個 mutation（拿掉守門鎖、拿掉讀取 join、im 刪帳申請中送訊息）分別對應 M3、M2／M5／M9、M6。mutation 結果只在複本跑，CI 不跑（比照 A6-3b 的註記）。

## 9. 沿用盤點（寫進各模組 `CARRYOVER.md`）

### 9.1 review

| 照搬的機制 | 先問「從零設計會怎麼做」 | 結論 |
|---|---|---|
| `inactive_authors` 複本表（A6-1c） | 複本的必要條件是「資料庫端存旗標」且「函式只能參照自己的表」；review 只在查詢過濾與寫入把關，沒有存旗標 | 不成立；join `buyer.pub_inactive_buyers()`，刪表 |
| `LockAuthor`（`Submit` 沒有可鎖的列，所以用 advisory lock） | buyer 已有 `LockActive`，鎖的是買家列 | 不成立；改 `buyer.LockActive`（`FOR KEY SHARE`，W3） |
| 刪帳訂閱者只排 job、抹除 job、保底掃描 | 抹除的形狀在 review 自己 | 成立，保留；訂閱者不再寫複本，保底掃描讀 `pub_inactive_buyers` |
| 公開述詞只寫一份（`testdata/public_predicate.sql`） | 一條規則一個定義 | 成立，述詞內容改 join |

### 9.2 im

| 照搬的機制 | 從零設計 | 結論 |
|---|---|---|
| `inactive_parties` 複本與 `LockParty`（`im/CARRYOVER.md:35` 的兩個理由：避開跨模組鎖序、W4 讓等待刪除的買家仍可聊天） | 理由一：擁有者的 API 只有一種形狀才逼出複本；理由二：同一原因 | 兩個理由都不成立：`LockNotClosed` 提供「放行 PENDING」的形狀，鎖序由 lockorder 機械檢查（buyer、seller 在 im 之前） |
| 前提「`Send` 不先持買家或店家的列鎖，改了要重看」 | 本步改了，已重看 | 不成立：`Send` 在 tx 內先持買家、店家的 `FOR KEY SHARE`，再寫自己的列；鎖序與 accountdeletion 的 `lockParticipants` 同序 |
| 抹除 job 與 `im.erase_sweep` | 抹除形狀在 im 自己 | 成立，保留；掃描來源改 pub |
| `guard` 在 tx 外 | 守門鎖不得跨 RPC | 成立且新增規定：鎖取在 `inTx` 內、RPC 之後，取到後重查 |
| 抹除保證「進入刪除後沒有新訊息」 | | 理由改為擁有者列鎖互斥 |

### 9.3 crm

| 照搬的機制 | 從零設計 | 結論 |
|---|---|---|
| `inactive_parties`、`LockParty`／`LockPartyShared`（基準形狀但放錯擁有者，簡報 §2） | 事實與守門應在擁有者 | 不成立；改 `LockNotClosed`＋`LockNotRetired`，刪表與 advisory lock |
| 「Deactivate 與 retireBatch 的死結論證」 | 新設計的 retireBatch 是 `SKIP LOCKED` | 論證不再需要，刪除該段說明 |
| 標籤與客戶寫入只讀複本、不取鎖 | 寫入要與轉移串行 | 缺口，已補（§3.3） |
| 抹除 job、保底掃描 | | 成立，保留 |
| 投影端的守門 | ordering 搬遷時投影退場 | 過渡程式；退場點與 `step-3-design.md` §0 一致 |

### 9.4 buyer、seller

| 項目 | 結論 |
|---|---|
| `pub_inactive_buyers` 「只給 social 對帳」的契約（`buyer/CARRYOVER.md:105`） | 改寫：review 的讀取過濾也用它；social 仍只用它對帳（不 join） |
| `LockActive`（`FOR SHARE`） | W3 建議降為 `FOR KEY SHARE` |
| seller 沒有「已退場」的 pub（`seller/db/pub.sql`） | 新增 `pub_retired_stores`，與 `pub_exposable_stores` 分開 |

### 9.5 開工 design-review 補列

| 機制 | 結論 |
|---|---|
| review 的 `AuthorInactive`、im 的 G4 `s.inactive`：tx 外的提早檢查 | 保留，必要條件是「被擋的請求不做 Finalize 轉正圖片」；tx 內的守門才是正確性保證，提早檢查只省成本 |
| crm 刪 `requireActiveStore`，錯誤對應改由守門的 NotFound | 行為不變：對外仍是 `tagNotFound`／客戶 NotFound，§8.2 的寫入被擋測試斷言錯誤碼 |
| crm 投影遇到不存在的買家或店家（W1） | 行為變更：原本照寫，改後跳過並記 error log；依 W1 拍板 |
| 店家退場靠 `RetireDueStores` 的 `SKIP LOCKED` 跳過被守門鎖住的店 | 接受跳過；熱門店家可能連續多輪被跳過。觸發條件：店家 `retired_at` 落後於排定時點超過 1 小時（4 輪），改為該店在下一輪不用 `SKIP LOCKED`、等待取鎖；門檻是推測 |
| `P/jobs` 的 sweep helper 與 `SweepInterval` 常數 | 依 W6 拍板 |
| W3 選 (a) 時會改到 social 使用的 `LockActive`／`LockBuyerShared` 鎖強度 | §0.2 的「social 不動」指 social 的複本與對帳；W3 (a) 時 social 的整組 unit＋integration 要重跑 |

## 10. 待決題

### W1 守門對「不存在」的處理：crm 重放到不存在的買家或店家

`LockNotClosed`、`LockNotRetired` 對已關閉與不存在都回 NotFound（比照 `LockActive`）。crm 的投影現況：不在複本的買家 ID 一律當營運中，照寫（`project.go:97-102`）。換成守門後，從未存在的 ID 會被當成「已停用」。

- (a) 同已停用：`return nil`、記 **error** log（上游壞資料要看得見），不重試。
- (b) 守門回可區分的原因（例如 NotFound 帶 reason）；已關閉靜默跳過，不存在回錯誤，由 River 重試到上限後人工處理。
- (c) 守門不分，crm 另以 `buyer.ProfileByBuyerID` 先確認存在（多一次讀取）。

建議 (a)：buyer、店家列不會被刪（§0.4），不存在只可能是壞資料，重試不會讓它存在；(b) 會讓守門的契約多一個分類，所有消費端都要處理。證據等級：Java ordering 只對真實訂單發事件【推，未對 Java 端核對】。狀態：未決。

### W2 `pub_closed_buyers()` 的形狀

- (a) 新增獨立函式 `pub_closed_buyers()`（本檔採用）。
- (b) 把 `pub_inactive_buyer` 加一欄 `status` 或 `closed boolean`，同一個函式服務兩種消費者；social 的 sqlc 與對帳查詢、契約測試都受影響。

建議 (a)：不改既有契約的形狀、不牽動 social；代價是 buyer 多一個函式。`pub_inactive_buyers` 與 `pub_closed_buyers` 是同一張表的兩個謂詞，沒有第三個消費者形狀時不抽象。狀態：未決。

### W3 `LockActive`／`LockBuyerShared` 是否同步降為 `FOR KEY SHARE`

- (a) 降，`LockNotClosed` 與 `LockActive` 共用一支查詢（本檔 §1.5 建議）。
- (b) 不降，新增 `LockBuyerKeyShare`，買家有兩種共享強度。

建議 (a)，3c1 獨立 commit，social 與 buyer 的守門測試整組重跑。證據：讀過全部 `UPDATE buyer.buyers` 都在 `FOR UPDATE` 之後，等待集合不變【推】。風險：social 的行為面（`block.go:63` 也用 `LockBuyerShared`）需要整組測試證明。狀態：未決。

### W4 守門與業務寫入的互等（簡報 §4 的修正，見 §0.4.1 第 1 點）

買家的業務寫入持 `FOR UPDATE`（`stint.go:11-27`），店家的持 `FOR UPDATE`（`seller/internal/stint/stint.go:20-33`），所以每則 im 訊息、每筆 crm 投影的 `FOR KEY SHARE` 會與它們互等；單則 tx 都短，但沒有量測。

- (a) 接受，量測觸發條件（守門等待時間、`pg_locks` 等待）超過門檻再處理。
- (b) 把業務寫入的列鎖降成 `FOR NO KEY UPDATE`（它們不改被外鍵參照的鍵），只有轉移（`BeginDeletion`、`CloseForDeletion`、`LockAccountStores`、`RetireDueStores`）保留 `FOR UPDATE`。買家要拆 `LockBuyer`（`Require` 與轉移共用）成兩支；店家的 `LockStore` 只被 `stint.Lock` 使用，改一行；24 個 seller 呼叫點不動。串行化不變（官方表：`FOR UPDATE` 與 `FOR NO KEY UPDATE` 衝突）。

建議 (a) 先上，(b) 是一行到兩行的後續修，觸發條件寫進 CARRYOVER；理由：系統未上線、沒有量測數字，先不改兩個擁有者的寫入鎖強度。狀態：未決。

### W5 sweep 的掃描範圍與索引

新 sweep 從擁有者的 pub 函式出發（買家 `status <> 'ACTIVE'`、店家 `retired_at IS NOT NULL`）逐一 `EXISTS` 消費者資料，而舊版從消費者自己的小複本表出發。pub 函式的結果集隨時間單調增加（列永久保留），buyer 沒有 `status` 的索引（`buyer/db/schema.sql` 的索引清單）。

- (a) 不加索引，每 15 分鐘掃一次，`LIMIT` 取滿才停（review、im）；crm 保留 keyset 游標。
- (b) buyer 加 `WHERE status <> 'ACTIVE'` 的部分索引、seller 的 `stores_purge_pending` 不涵蓋已完成第二段的店，另加 `WHERE retired_at IS NOT NULL` 的索引。
- (c) 從消費者端出發（`SELECT DISTINCT buyer_id FROM review.reviews`），再 join pub；全表掃描 reviews，不可取。

建議 (a)，改 (b) 的觸發條件寫進各模組 CARRYOVER：sweep 單輪耗時超過 30 秒（River job 的執行時間）或 closed 買家數超過 10 萬列（`SELECT count(*) FROM buyer.buyers WHERE status <> 'ACTIVE'`）；門檻數值是推測，沒有量測。未量測：closed 買家數、單次耗時。狀態：未決。

### W6 共用 sweep helper 的範圍

- (a) 只改 review、im、crm 與 im 的 `live/sweep.go`（本步需要的 4 處）。
- (b) 同時改 social、payment、media、seller、`P/inbox` 共 10 處相同的五狀態清單與其他 `ByPeriod` 選項。

建議 (a)：(b) 各處的狀態集合（`terminationStates`、`registryOpenStates`）可能不同，要逐一核對，超出本步；但 `P/jobs` helper 的命名與位置照 (b) 的終態設計，並在 `AGENTS.md`「重複實作」的欄位登記還沒搬的 10 處。狀態：未決。

### W7 「legacy client 在 tx 內被呼叫」的機械檢查形狀

見 §6：(a) `internal/arch` 的 AST 規則（只抓 closure 內直接呼叫）；(b) 整合測試替身斷言沒有 tx（只守有測試的路徑）；(c) 兩者都做。建議 (b)，本步 im 一個測試；(a) 另案。狀態：未決。

## 11. 未查證與限制

- `SKIP LOCKED` 的官方語意沒有查（只查了 explicit-locking 頁）；`RetireDueStores` 對被守門鎖住的店會跳過是依「`FOR UPDATE` 與 `FOR KEY SHARE` 衝突」推論。3c1 以 `TestLockNotRetiredSkipsTheStoreInRetireBatch` 在真實 Postgres 驗證。
- `BEGIN ATOMIC` 的 SQL 函式被 planner 內聯成單列查詢：現有先例 `TestViewerBlockPredicateInlinesThePubFunctions`（`review_integration_test.go:93`）驗證 `pub_blocked_*`；新函式要補同樣的 `EXPLAIN` 斷言（§8.2），本檔沒有跑。
- 熱門店家的 multixact 與鎖等待沒有量測（簡報 §4 的結論沿用）；觸發條件照簡報 §9。
- crm 現況「標籤寫入與退場的缺口」是讀碼推論（`Deactivate` 只取 `LockParty`、`LockStoreTags` 是另一個命名空間）；3c1 先寫會失敗的整合測試再確認。
- Java ordering 是否會對不存在的買家發 `order.completed`（W1）未對 Java 端核對。
- 本檔沒有跑任何測試、容器或指令以外的生成（唯讀指令與官方文件查閱）。
