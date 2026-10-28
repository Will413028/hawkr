# reviewqueue 模組

## 職責

後台待辦：所有模組中等待營運處理的事項，依期限由急到緩，keyset 分頁，給後台的待辦頁用。只排序與分頁；待辦的成員、開始與期限的規則在各來源模組的 SQL（現在是 seller 的 `review_<種類>(at)`）。行為規格的唯一來源是 `docs/modules/reviewqueue.md`。

## 擁有的資料

無 schema，沒有 `db/schema.sql` 與 migration：待辦由各模組的領域狀態推導，不另存。登記在 `cmd/hawkr/modules.go` 的 `withoutData`，不在 `modules()`。讀取模型：無。

## 發布的讀取介面

無。本模組不發布 `pub_*`，只讀 seller 發布的 `seller.pub_review_items(at)`（契約在 seller 的 `db/pub.sql`）。對外是 `Module.List(ctx, now, kinds, after, limit)`，形狀見 `docs/modules/reviewqueue.md` §1。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法與對外型別別名（`Kind`、`Item`、`Cursor`）。
- `internal/model`：種類、一列、游標與游標的檢查，不碰 I/O。`internal/items`：`List` 的實作。`internal/store`：sqlc 生成的資料存取葉節點，不得手改。
- 依賴圖的唯一來源是 `internal/arch/modules.go`：模組依賴 `seller`，不 import seller 的 Go 套件。查詢在 `db/queries/items.sql`，`db/sqlc.yaml` 的 schema 只有 `../../seller/db/pub.sql`。加來源的步驟見 `docs/modules/reviewqueue.md` §1。

## 不變量

- 全序是 `due_at, start_at, kind COLLATE "C", item_id COLLATE "C"`，翻頁不漏不重。守門：`TestListPagesWithoutGapsOrDuplicates`、`TestOrderingIsBytewiseNotLocale`（ICU database，鎖 `item_id`）；`internal/arch` 的 `TestSQLCReadsOnlyItsOwnSchemaAndAllowedPubContracts` 守 sqlc 只讀允許的 `pub.sql`。規格：`docs/modules/reviewqueue.md` §3。
- `kind` 的 `COLLATE "C"` 守門：無（只有規格）。規格：§3。
- 模組認得的種類等於來源發布的種類，查詢以 `Kinds` 篩選。守門：`TestKindsMatchTheSource`、`TestListFiltersByKinds`。規格：§2。
- `List` 一個語句一個快照，不開 transaction；`now` 由呼叫端傳入。守門：無（只有規格）。規格：§4。
- 游標的時間精確到 µs、種類有效、ID 非空才有效；無效輸入被拒絕。守門：`TestListRejectsInvalidInput`。規格：§3、§5。

## 不負責

- 哪些事項算待辦、開始與期限怎麼算：各來源模組（現在是 seller 的 `review_<種類>(at)`）。
- 權限：`internal/httpapi/reviewqueue` 取種類與呼叫者讀取權限的交集，模組不知道呼叫者。
- 處理待辦的動作：在各來源的頁面與指令。

## 決策紀錄

沿用機制的判斷在同目錄的 `CARRYOVER.md`。相關 ADR：`docs/adr/2026-10-20-admin-review-queue.md`（待辦是對擁有者事實的查詢，不另存；含已知限制的接受與改成另存 SLA 紀錄的觸發）、`docs/adr/2026-09-24-go-modular-monolith.md`（`pub_*`）。
