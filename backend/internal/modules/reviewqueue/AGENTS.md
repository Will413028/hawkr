# reviewqueue 模組

後台待辦：所有模組中等待營運處理的事項，依期限由急到緩，keyset 分頁。新模組，沒有舊服務的來源；設計是 `docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-design.md`（v3.1）的 §1.3、§2.2、§3.2。

## 職責

- 讀各來源發布的 `pub_*_review_items(at)`（現在只有 `seller.pub_review_items`），以 `(due_at, start_at, kind, item_id)` 的全序排序並 keyset 分頁（E4）：`Module.List(ctx, now, kinds, after, limit)`。
- 種類（`Kind`，UPPER_SNAKE）與來源 `kind` 欄、契約的 `ReviewQueueItemKind` 一一對應；查詢以模組認得的 kinds 篩選，來源多出的種類不會出現在結果裡（執行期不報錯）；由測試 `TestKindsMatchTheSource` 鎖住 seller 的 12 支 `review_<種類>(at)` 的 kind 字面值集合等於模組的 `Kinds`。
- 待辦的成員、開始與期限的規則不在這裡：它們在擁有者的 SQL（seller 的 `review_<種類>(at)`，E10），這個模組只排序與分頁。

## 擁有的資料

沒有。待辦是各模組的領域狀態推導出來的，不另存（設計 §2.3），所以模組沒有 schema、沒有 `db/schema.sql` 與 migration，也不登記在 `cmd/hawkr/modules.go` 的 `modules()`（在 `withoutData`）；`db/sqlc.yaml` 只讀 seller 的 `pub.sql`。第一次要存自己的資料時再建 schema 與 migration（`make new-module` 的骨架）。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法與對外型別別名（`Kind`、`Item`、`Cursor`）。
- `internal/model`：種類、一列、游標與游標的檢查，不碰 I/O。`internal/items`：`List` 的實作。`internal/store`：sqlc 生成的資料存取葉節點，不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go`：模組依賴 `seller`（讀它的 `pub.sql`），不 import seller 的 Go 套件。
- 查詢在 `db/queries/items.sql`；`db/sqlc.yaml` 的 schema 只有 `../../seller/db/pub.sql`（沒有自己的 `schema.sql`）（`internal/arch` 的 `TestSQLCReadsOnlyItsOwnSchemaAndAllowedPubContracts` 檢查）。加來源時：依賴表加一行、`sqlc.yaml` 加對方的 `pub.sql`、`ListItems` 的 `FROM` 加一支 `UNION ALL`、`model.Kinds` 加種類。

## 不變量

- 全序是 `due_at, start_at, kind COLLATE "C", item_id COLLATE "C"`，`ORDER BY` 與 keyset 的 row-value 比較寫同一組，所以翻頁不漏不重（與資料庫的預設 collation 無關）；第一頁沒有游標（四個 `after_*` 都是 NULL，`sqlc.narg` 加 `IS NULL OR`，同 repo 慣例）。`TestListPagesWithoutGapsOrDuplicates` 鎖住翻頁不漏不重，`TestOrderingIsBytewiseNotLocale`（ICU database）鎖住 `item_id` 的 `COLLATE "C"`；`kind` 的 `COLLATE "C"` 現在沒有測試能抓到拿掉它（11 個值在預設與 C collation 下順序相同），保留是為了與 `item_id` 一致、不依賴資料庫的 collation；新增的 kind 值若與既有值在 collation 下順序不同，ICU 測試要加入跨種類的列。
- `List` 跑在 `db.Conn(ctx)` 上，一個 SQL 語句就是一個快照（READ COMMITTED 下每個語句一個快照），不開 transaction、不用唯讀快照。整個請求一個 `now`，由呼叫端傳入，模組不讀時鐘。
- 游標的時間精確到 µs，種類有效、ID 非空，才是有效的游標（`Cursor.Valid`）。

## 已知限制

- 翻頁途中被營運自己改過排序鍵的項目（改案件嚴重度、延後複審、暫不告知換成仍已到期的確認日期），可能在後面的頁重複或不出現，重新整理就正確（設計 E9，接受）。觸發：有第二位營運人員，或出現要凍結排序鍵的需求時改成實體化（設計 E12）。
- 每次請求都算出全部開著的待辦再排序，沒有索引能幫排序；超過 300 ms 是另存 SLA 紀錄的觸發（設計 §6.5）。

## 讀取模型

無。

## 不負責

- 哪些事項算待辦、開始與期限怎麼算：各來源模組（現在是 seller 的 `review_<種類>(at)`）。
- 權限：`internal/httpapi/reviewqueue` 把請求的種類與呼叫者對各來源的讀取權限取交集（種類 → 權限的表在 handler），模組不知道呼叫者。
- 處理待辦的動作：在各來源的頁面與指令。

## 決策紀錄

沿用的機制與當時的判斷在同目錄的 `CARRYOVER.md`。
