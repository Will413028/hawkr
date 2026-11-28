# catalog 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/catalog.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

商品、規格（選項軸與 SKU）、庫存與平台分類樹；行為規格唯一來源 `backend/docs/modules/catalog.md`，決策 `backend/docs/plans/2026-11-24-b4-catalog.md` 的 D1–D26。

## 擁有的資料

schema `catalog`（`db/schema.sql`）：`categories`、`products`、`product_option_axes`、`product_option_values`、`product_skus`、`stock_operations`、`stock_movements`、`product_images`、`product_hashtags`、`sales_orders`、`restock_queue`、`restock_purge_queue`。讀取模型：無。依賴擴充 `pg_trgm`（平台安裝）。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只有 `Module`、`New`、轉呼叫方法、型別別名與錯誤。
- `internal/model` 放對外型別與錯誤；`internal/stock` 是庫存的冪等扣減與歸還；`internal/taxonomy` 讀分類樹；`internal/counter` 是評分與銷量；`internal/restock` 是補貨佇列與 River 週期 worker（不經 `platform/jobs`）；`internal/product` 是商品的寫入、讀取與兩種列表；`internal/description` 是描述區塊的純驗證與編碼；`internal/store` 是 sqlc 的查詢套件。
- 依賴 media（圖片 `Confirm`／`MarkDeleted`）與 seller 的 `pub_exposable_stores`、`pub_content_takedowns`（只有可見性函式讀，`db/sqlc.yaml` 讀 seller 的 `pub.sql`）；hashtag 規則在 `platform/hashtag`，與 social 共用。
- 依賴圖唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；各子領域宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

- 庫存只由 `stock` 改：扣減是 `UPDATE … WHERE stock_qty >= n`，先插入 `stock_operations`（冪等鍵唯一；結帳的操作全域唯一，賣家調整的鍵依 `store_id` 只在店家內唯一）再扣，不「先查再扣」；SKU 列依 ID 升序鎖定；庫存不為負。守門：`TestConcurrentSameKeyDeductsOnce`、`TestMultiItemDeductsInOppositeOrderDoNotDeadlock`、`TestAdjustKeyIsScopedPerStore`。
- 歸還依原扣減，只認原扣減的冪等鍵；先 `LockStockOperation` 鎖原扣減列，累計不超過原扣減量，原扣減沒有的 SKU 被拒。守門：`TestRestoreFollowsTheOriginalDeduction`。
- 同一個冪等鍵以不同內容重送回 `ErrIdempotencyKeyReused`，內容相同回成功且不改庫存。`DeductStock`／`RestoreStock` 在呼叫方的 transaction 內失敗時，呼叫方必須整個回滾。
- 分類只在 migration 變動，兩層；商品只掛葉節點（`IsLeafCategory`）；名稱 `LocalizedText`（必有 zh-TW）。守門：`TestSeedTaxonomyShape`、`TestOnlyLeafCategoriesTakeProducts`、`categories_name_check`。
- 商品狀態是 `DRAFT`／`ACTIVE`／`ARCHIVED`，`ACTIVE` 必有 `first_published_at`；刪除是 `deleted_at`，列保留。可見性只有 `db/objects.sql` 的 `buyer_visible_products(at)`／`buyer_viewable_products(at)`（含 ARCHIVED，單筆頁）：狀態 ∧ 店家可曝光 ∧ 無生效中的 `PRODUCT` 措施，不寫進狀態欄；買家面 query 走函式，其餘登記在 `arch.CatalogNonBuyerProductQueries`；`products` 加欄位時重建函式。守門：`TestProductUnderMeasureIsNotVisible`。
- 商品寫入在新圖 `Confirm` 之後從 `LockOwnProduct`（以 `storeID` 限定）的列鎖開始，既有 SKU 列接著依 ID 升序鎖住（`LockLiveSkus`，與 `DeductStock` 同序）。守門：`TestUpdateLocksExistingSkusInAscendingIdOrderWhateverTheInputOrder`、`TestUpdateConfirmsNewImagesBeforeLockingTheProductRow`。
- 讀取：商品列與子表一句 SQL 讀出（一份快照；會被別模組呼叫，規則 3 不開 `InReadSnapshot`）。守門：`TestReadIsOneSnapshotWhileAWriteCommitsBetweenItsParts`。
- 圖片：新 token 在鎖商品列之前 `Confirm`，退場資產在鎖住之後 `MarkDeleted`，都在寫入的 transaction 內，失敗整筆回滾；描述圖片區塊只存 `asset_id`。守門：`TestImagesConfirmedInTheWriteAndMarkedDeletedWhenRemoved`、`TestMediaFailuresRollBackTheProductWrite`。
- 狀態（D22）任兩個都能互切（無效目標回 `InvalidInputError`）；ACTIVE 的商品在切成 ACTIVE 與每次寫入時至少 1 張主圖；`first_published_at` 只在第一次 ACTIVE 填，不清除。刪除把還在的 SKU 一併標刪除（扣減與調整找不到，歸還仍可）。守門：`TestDeleteProductMakesItsSkusUndeductibleButRestorable`、`TestProductLifecycleAndFirstPublishedAt`。
- 購物車 `Active` 與扣庫存（`ErrProductNotPurchasable`）用同一個可見性，扣減的 `UPDATE` 內再判一次。守門：`TestSuspendedStoreSkuIsNotPurchasableAtCheckout`。
- SKU 組合、價格區間、etag（`content_version`）、請求冪等、`SetStock`、低庫存、搜尋與列表篩選的不變量在 `internal/product/AGENTS.md`。
- 補貨通知（D26）：不可買變可買時同 transaction 寫佇列表，週期 worker 在外經 `RestockNotifier` 通知；不變量在 `internal/restock/AGENTS.md`。

## 不負責

- 運費與配送方式（D10）、運費設定與報價：seller（B3）。
- 結帳的扣庫存呼叫與訂單快照保存：ordering（B8）；catalog 只提供 `DeductStock`／`RestoreStock` 與結帳商品快照。
- 評分與則數：review，經 `SetRating` 寫絕對值（B4-4e）；收藏：buyer。
- 商品限制措施（平台下架）的事實：seller 擁有措施，catalog 讀取（B4-5）。
- 媒體檔案的儲存與刪除：media；catalog 只存引用並 `Confirm`／`MarkDeleted`。

## 決策紀錄

從舊服務沿用的每個機制、當時的決定與理由，以及與舊服務的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關決策：`backend/docs/plans/2026-11-24-b4-catalog.md` 的 D2、D6、D8、D14、D16–D26。
