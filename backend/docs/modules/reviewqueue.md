# reviewqueue 模組行為規格

本文是單體 `reviewqueue` 模組行為的唯一來源。後台待辦：所有模組中等待營運處理的事項，依期限由急到緩，keyset 分頁。

## 1. 範圍與公開介面

- 公開介面是 `Module.List(ctx, now, kinds, after, limit)`，與對外型別別名 `Kind`、`Item`、`Cursor`。
- 模組讀各來源發布的 `pub_*_review_items(at)`；現在只有 `seller.pub_review_items`。待辦的成員、開始與期限的規則在擁有者的 SQL（seller 的 `review_<種類>(at)`），這個模組只排序與分頁。
- 待辦是各模組的領域狀態推導出來的，不另存：模組沒有 schema、`db/schema.sql` 與 migration，登記在 `cmd/hawkr/modules.go` 的 `withoutData`，不在 `modules()`。`db/sqlc.yaml` 的 schema 只有 seller 的 `pub.sql`。
- 依賴圖的唯一來源是 `internal/arch/modules.go`：模組依賴 `seller`（讀它的 `pub.sql`），不 import seller 的 Go 套件。
- 加來源的做法：依賴表加一行、`sqlc.yaml` 加對方的 `pub.sql`、`ListItems` 的 `FROM` 加一支 `UNION ALL`、`model.Kinds` 加種類。

## 2. 種類

- `Kind` 是 UPPER_SNAKE，與來源 `kind` 欄、契約的 `ReviewQueueItemKind` 一一對應。seller 發布 10 種：`REPORT_TRIAGE`、`CASE_DECISION`、`CASE_INQUIRY_OVERDUE`、`AUTHORITY_ORDER`、`AUTHORITY_TARGET_NOTICE`、`MEASURE_APPEAL`、`MEASURE_LIFT_REQUEST`、`MEASURE_REVIEW`、`KYC_APPLICATION`、`PAYUNI_BIND_RETRY`。
- 查詢以模組認得的 `Kinds` 篩選：來源多出的種類不會出現在結果裡，執行期不報錯。seller 的 10 支 `review_<種類>(at)` 的 kind 字面值集合等於 `Kinds`，由測試鎖住。
- 空字串與不在 `Kinds` 裡的值都不是有效的種類。

## 3. 排序與游標

- 全序是 `due_at, start_at, kind COLLATE "C", item_id COLLATE "C"`；`ORDER BY` 與 keyset 的 row-value 比較寫同一組，所以翻頁不漏不重，與資料庫的預設 collation 無關。
- 第一頁沒有游標：四個 `after_*` 都是 NULL，查詢以 `sqlc.narg` 加 `IS NULL OR` 處理。
- 游標是上一頁最後一列的 `(Due, Start, Kind, ID)`。兩個時間有值且精確到 µs、種類有效、ID 非空，才是有效的游標（`Cursor.Valid`）。
- `kind` 的 `COLLATE "C"` 現在沒有測試能抓到拿掉它（現有種類值在預設與 C collation 下順序相同），保留是為了與 `item_id` 一致；新增的 kind 值若與既有值在 collation 下順序不同，ICU 測試要加入跨種類的列。

## 4. 讀取的快照與時間

- `List` 跑在 `db.Conn(ctx)` 上，一個 SQL 語句就是一個快照（READ COMMITTED 下每個語句一個快照），不開 transaction、不用唯讀快照。
- 整個請求一個 `now`，由呼叫端傳入，模組不讀時鐘。

## 5. 權限與輸入

- 模組不知道呼叫者。`internal/httpapi/reviewqueue` 把請求的種類與呼叫者對各來源的讀取權限取交集（種類到權限的表在 handler）；沒有任何種類的讀取權時回空清單。
- page_size 負數、token 與 kinds 不符、kinds 含 UNSPECIFIED 或未知值，是 INVALID_ARGUMENT。token 是 `rpc.TokenFor` 的 keyset，綁請求的 kinds。

## 6. 已知限制

- 翻頁途中被營運自己改過排序鍵的項目（改案件嚴重度、延後複審、暫不告知換成仍已到期的確認日期），可能在後面的頁重複或不出現，重新整理就正確。
- 每次請求都算出全部開著的待辦再排序，沒有索引能幫排序。
