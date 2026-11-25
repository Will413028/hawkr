# catalog 模組行為規格

本文是單體 `catalog` 模組行為的唯一來源：商品、規格（選項軸與 SKU）、庫存與平台分類樹。步驟與決策在 `backend/docs/plans/2026-11-24-b4-catalog.md`（D1–D20），Java product-service 的盤點在同名目錄。實作隨 B4 各子步驟增加；本文只寫已實作的部分，其餘標明步驟。

## 1. 範圍與公開介面

- 實作在 `internal/modules/catalog`，自己的 `catalog` schema 與 migration，登記在 `cmd/hawkr/modules.go` 的 `modules()`。不依賴 fulfillment（D10：運費與配送方式歸 seller 的運費設定，B3）。
- 根套件的方法：`DeductStock`、`RestoreStock`、`AdjustStock`、`Categories`、`IsLeafCategory`、`CreateProduct`、`UpdateProduct`、`SetProductStatus`、`DeleteProduct`、`Product`、`PublicProduct`、`ListSellerProducts`、`ListPublicProducts`；對外 Connect API 隨 B4-3，`legacy/catalogserver` 與單體內改接隨 B4-4。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 資料

| 表 | 內容 |
|---|---|
| `categories` | 兩層分類樹；名稱 `LocalizedText`（JSONB，必有 zh-TW）；葉節點有 `shopify_taxonomy_id` |
| `products` | 商品：`DRAFT`／`ACTIVE`／`ARCHIVED`、`deleted_at`、價格區間、反正規化計數、描述區塊 JSON、`searchable_text` |
| `product_option_axes`、`product_option_values` | 0–2 個選項軸與有序的值 |
| `product_skus` | 選項值的組合；`price`、`compare_at_price`、`stock_qty`、`deleted_at` |
| `stock_operations`、`stock_movements` | 庫存帳本：一次操作一列（冪等鍵 UNIQUE），明細是有號 delta |
| `product_images` | 圖庫與描述圖片的資產引用 |
| `product_hashtags` | 商品的 hashtag 值集合 |

## 3. 庫存

- 扣減（`DeductStock(ctx, key, items)`）：先插入 `stock_operations`（`ON CONFLICT (idempotency_key) DO NOTHING`），再依 SKU ID 升序逐一 `UPDATE … WHERE stock_qty >= n AND deleted_at IS NULL`，每個寫一筆 `stock_movements`（delta 為負）。任何一個 SKU 不足整次回滾，回 `*InsufficientStockError`（第一個不足的 SKU）；SKU 不存在或已刪除回 `ErrSkuNotFound`。
- 重送：同一個 key 內容相同（品項與數量，不論順序）回成功且不改庫存；內容或種類不同回 `ErrIdempotencyKeyReused`。併發的同一個 key 在 UNIQUE 上排隊，後到的看到 0 列後走重送路徑。
- 歸還（`RestoreStock(ctx, key, deductKey, items)`）：依 `deductKey` 找原扣減（不存在回 `ErrDeductionNotFound`），先鎖原扣減列，再檢查每個 SKU 的累計歸還量不超過原扣減量（`ErrRestoreExceedsDeduction`）；`items` 為空表示歸還剩下的全部（沒有剩下的也回 `ErrRestoreExceedsDeduction`）。歸還不檢查 SKU 是否已刪除。
- 賣家調整（`AdjustStock(ctx, storeID, key, skuID, delta)`）：賣家改自己店家某個既有 SKU 的庫存，`storeID` 由呼叫端推導。用有號變動量而不是絕對值，因為 `stock_movements` 存的是有號 delta（`delta <> 0` 的只增不改帳本），變動量重送時內容固定可比對。先插入 `kind = 'ADJUST'` 的 `stock_operations`，再 `UPDATE … FROM products WHERE store_id = n AND 結果 BETWEEN 0 AND 2147483647`，寫一筆 movement。SKU 不存在、不屬於該店家或已刪除（含商品已刪除）回 `ErrSkuNotFound`；結果小於 0 回 `*InsufficientStockError`；結果超過 int32 上限回 `ErrInvalidStockInput`；失敗整筆回滾（含操作列）。重送規則同扣減，與扣減或歸還共用 key 回 `ErrIdempotencyKeyReused`。
- 輸入：key 1–200 字元、頭尾無空白；品項 1–100 個、數量為正、同一個 SKU 不重複；違反回 `ErrInvalidStockInput`。
- transaction：全部寫入在呼叫方的 transaction 內，沒有時自己開一個；呼叫方 transaction 內失敗時必須整個回滾。
- 扣減時點是「下單即扣、逾時未付歸還」（D2）；歸還的觸發在 B8。

## 4. 分類

- 種子在 migration `catalog_taxonomy_seed`：10 個頂層分組、52 個葉節點，來源與對照見 `backend/docs/plans/2026-11-24-b4-catalog/taxonomy-draft.md`。ID 由 Shopify ID 決定（頂層由 `top:<zh-TW 名稱>`），各環境相同。
- 沒有執行期寫入與管理畫面；要改分類就加 migration。新增子層或受管制品類前先做資格審核（ADR general-ecommerce-scope-cut）。
- `IsLeafCategory`：分類存在、有上層、沒有子節點。商品只能掛葉節點。

## 5. 商品寫入（B4-2b）

呼叫端（B4-3 的 handler）從登入身分推導 `storeID` 與 `media.Uploader`（店家所屬帳號），不信任 client 送的店家；這一層每個寫入與賣家讀取都以 `storeID` 限定，商品不是這家店的或已刪除回 `ErrProductNotFound`。輸入不合法回 `*InvalidInputError{Field, Reason}`（`errors.Is(err, ErrInvalidInput)`），使用者面文字由呼叫端依欄位與原因產生。

- `CreateProduct` 建立草稿；`UpdateProduct` 整體取代內容（狀態不變）；`SetProductStatus` 切換狀態；`DeleteProduct` 設 `deleted_at`（不可還原，列與圖片引用保留）。全部在一個 transaction，並以 `LockOwnProduct` 鎖商品列。
- 狀態（D16）：`DRAFT`→`ACTIVE`、`ACTIVE`→`ARCHIVED`、`ARCHIVED`→`ACTIVE`；其他切換回 `ErrInvalidTransition`，切成目前狀態是 no-op。`first_published_at` 在第一次設成 `ACTIVE` 時填，之後不變。分類必須是葉節點。
- 選項與 SKU（D18）：0–2 個軸，軸名稱不重複，每軸 1–50 個值且不重複；SKU 以組合辨識，每個位置的值必須在該軸的值內、沒有軸的位置為空、組合不重複；沒有軸的商品剛好一個預設 SKU；SKU 最多 100 個。價格為正整數新台幣元，`compare_at_price` 必須大於售價。更新時組合還在就更新價格（庫存不動）、新組合新增（庫存給初始值）、不在輸入的組合以 `deleted_at` 刪除。
- 價格區間（D19）：每次 SKU 寫入的同 transaction 以沒刪除 SKU 的最小與最大價重算 `min_price`／`max_price`。
- 描述（D17 A）：`paragraph`（純文字）、`image`（新 token 或現有 `AssetID`，alt）、`list`（項目）、`table`（列的儲存格）。上限：區塊 100、描述圖片 20、段落 5000 字、清單 50 項、表格 50 列 × 10 欄、清單項與儲存格 200 字（表格與清單照 seller-app 編輯器的常數，其餘待確認）。JSON 存在 `products.description`，圖片區塊只含 `asset_id`。
- 圖片：圖庫最多 9 張（待確認）與描述圖片都在 `product_images`（`GALLERY`／`DESCRIPTION`，各自從 0 起排序）。新 token 在同 transaction 內 `media.ConfirmFields`（公開圖片），既有的圖以 `AssetID` 引用且必須屬於這個商品；沒有被引用的現有資產同 transaction `MarkDeleted`。Confirm 或 MarkDeleted 失敗整筆回滾。同一個 token 或資產不得出現兩次。
- hashtag（D14）：明確輸入，正規化與驗證是 `platform/hashtag`（與 social 同一份：去頭尾空白與 `#`、NFC、ASCII 小寫；字元集 a-z、0-9、漢字；1–40 字；最多 10 個）；寫入 `product_hashtags(name, display_name, position)`。
- `searchable_text`：標題、描述文字（段落、清單項、儲存格）、hashtag 名稱、分類名稱（en，缺時 zh-TW），以單一空格相接。

## 6. 讀取與列表

- `Product`（賣家，任何狀態、沒刪除）與 `PublicProduct`（ACTIVE、沒刪除、店家在 `at` 可曝光）回完整內容：描述區塊、圖庫、選項軸、SKU、hashtag。
- `ListSellerProducts(storeID)`：店家自己所有沒刪除的商品，不看狀態與曝光，最新建立的在前。
- `ListPublicProducts(at, categoryID?)`：只含 ACTIVE、沒刪除、`store_id IN (seller.pub_exposable_stores(at))` 的商品，最新上架的在前；`at` 是請求的 clock。
- 分頁是 keyset（`ListCursor{At, ID}`，賣家用建立時間、公開用首次上架時間，同時間依 ID 遞減）；頁大小預設與上限 100。列帶主圖庫第一張的 object key。
- 還沒有：有效限制措施（D7）的排除（B4-5）、搜尋與 hashtag 彙總（B4-3）。
