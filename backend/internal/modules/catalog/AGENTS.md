# catalog 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/catalog.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

商品、規格（選項軸與 SKU）、庫存與平台分類樹，給賣家、買家與後台用。從 Java product-service 重寫；行為規格的唯一來源是 `backend/docs/modules/catalog.md`，決策是 `backend/docs/plans/2026-11-24-b4-catalog.md` 的 D1–D20。

## 擁有的資料

schema `catalog`（`db/schema.sql`）：`categories`、`products`、`product_option_axes`、`product_option_values`、`product_skus`、`stock_operations`、`stock_movements`、`product_images`、`product_hashtags`。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與錯誤。
- `internal/model` 放對外型別與錯誤；`internal/stock` 是庫存的冪等扣減與歸還；`internal/taxonomy` 讀分類樹；`internal/product` 是商品的寫入、讀取與兩種列表；`internal/description` 是描述區塊的純驗證與編碼；`internal/store` 是 sqlc 的查詢套件。
- 依賴 media（圖片 `Confirm`／`MarkDeleted`）與 seller 的 `pub_exposable_stores`（公開列表的 join，`db/sqlc.yaml` 讀 seller 的 `pub.sql`）；hashtag 規則在 `platform/hashtag`，與 social 共用。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

- 庫存只由 `stock` 改：扣減是 `UPDATE … WHERE stock_qty >= n`，先插入 `stock_operations`（冪等鍵 UNIQUE，`ON CONFLICT DO NOTHING`）再扣，不「先查再扣」；SKU 列依 ID 升序鎖定；庫存不為負（`skus_stock_check`）。守門：`TestConcurrentSameKeyDeductsOnce`、`TestConcurrentDifferentKeysNeverOversell`、`TestMultiItemDeductsInOppositeOrderDoNotDeadlock`。規格 §庫存。
- 歸還依原扣減，只認原扣減的冪等鍵；先 `LockStockOperation` 鎖原扣減列，累計不超過原扣減量，原扣減沒有的 SKU 被拒。守門：`TestRestoreFollowsTheOriginalDeduction`、`TestConcurrentRestoresNeverExceedTheDeduction`。
- 同一個冪等鍵以不同內容重送回 `ErrIdempotencyKeyReused`，內容相同回成功且不改庫存。`DeductStock`／`RestoreStock` 在呼叫方的 transaction 內失敗時，呼叫方必須整個回滾。
- 分類只在 migration 變動，兩層；商品只能掛葉節點（`IsLeafCategory`）；名稱是 `LocalizedText`（必有 zh-TW）。守門：`TestSeedTaxonomyShape`、`TestOnlyLeafCategoriesTakeProducts`、`categories_name_check`。
- 商品狀態是 `DRAFT`／`ACTIVE`／`ARCHIVED`，`ACTIVE` 必有 `first_published_at`；刪除是 `deleted_at`，列保留。可見性是狀態 ∧ 店家可曝光（join `seller.pub_exposable_stores`）∧ 沒有有效限制措施，不寫進狀態欄。
- 商品寫入都從 `LockOwnProduct`（以呼叫端推導的 `storeID` 限定）的列鎖開始；SKU 以選項值的組合辨識，每次 SKU 寫入同 transaction 重算 `min_price`／`max_price`（D19）。組合必須落在選項值內、不重複，沒有軸的商品剛好一個預設 SKU。庫存不在商品寫入裡改（既有組合不動，新組合才給初始值）。守門：`TestSkuCombinationsMustBeUniqueAndWithinOptionValues`、`TestPriceRangeFollowsSkuChanges`。
- 圖片：新 token 在寫入的 transaction 內 `Confirm`，退場的資產同 transaction `MarkDeleted`，兩者失敗整筆回滾；描述的圖片區塊只存 `asset_id`，alt 與 key 在 `product_images`。守門：`TestImagesConfirmedInTheWriteAndMarkedDeletedWhenRemoved`、`TestMediaFailuresRollBackTheProductWrite`。
- 狀態切換只有 DRAFT→ACTIVE、ACTIVE→ARCHIVED、ARCHIVED→ACTIVE；`first_published_at` 只在第一次 ACTIVE 填。刪除不碰圖片引用。公開列表與公開單筆只含 ACTIVE、沒刪除、店家可曝光的商品；賣家清單不過濾狀態與曝光。守門：`TestProductLifecycleAndFirstPublishedAt`、`TestPublicListOnlyActiveProductsOfExposableStores`、`TestSellerListIsUnfilteredByStatusAndExposure`。
- 運費與配送方式不在 catalog（D10）；不依賴 fulfillment。

## 不負責

- 運費設定與報價：seller（B3）。
- 結帳時的扣庫存呼叫與訂單快照的保存：ordering（B8）；catalog 只提供 `DeductStock`／`RestoreStock` 與結帳用的商品快照（B4-4）。
- 評分與則數的事實：review，經 `SetRating` 傳絕對值寫進商品（B4-3）；收藏的事實：buyer。
- 商品限制措施（平台下架）的事實：seller 擁有措施，catalog 讀取（B4-5）。
- 媒體檔案的儲存與刪除：media；catalog 只存引用並在自己的 transaction 內 `Confirm`／`MarkDeleted`。

## 決策紀錄

從舊服務沿用的每個機制、當時的決定與理由，以及與舊服務的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關決策：`backend/docs/plans/2026-11-24-b4-catalog.md` 的 D2、D6、D8、D14、D16–D19。
