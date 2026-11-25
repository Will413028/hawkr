# catalog 模組行為規格

本文是單體 `catalog` 模組行為的唯一來源：商品、規格（選項軸與 SKU）、庫存與平台分類樹。步驟與決策在 `backend/docs/plans/2026-11-24-b4-catalog.md`（D1–D20），Java product-service 的盤點在同名目錄。實作隨 B4 各子步驟增加；本文只寫已實作的部分，其餘標明步驟。

## 1. 範圍與公開介面

- 實作在 `internal/modules/catalog`，自己的 `catalog` schema 與 migration，登記在 `cmd/hawkr/modules.go` 的 `modules()`。不依賴 fulfillment（D10：運費與配送方式歸 seller 的運費設定，B3）。
- 根套件的方法：`DeductStock`、`RestoreStock`、`Categories`、`IsLeafCategory`；商品讀寫與公開列表隨 B4-2b，對外 Connect API 隨 B4-3，`legacy/catalogserver` 與單體內改接隨 B4-4。
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
- 輸入：key 1–200 字元、頭尾無空白；品項 1–100 個、數量為正、同一個 SKU 不重複；違反回 `ErrInvalidStockInput`。
- transaction：全部寫入在呼叫方的 transaction 內，沒有時自己開一個；呼叫方 transaction 內失敗時必須整個回滾。
- 扣減時點是「下單即扣、逾時未付歸還」（D2）；歸還的觸發在 B8。

## 4. 分類

- 種子在 migration `catalog_taxonomy_seed`：10 個頂層分組、52 個葉節點，來源與對照見 `backend/docs/plans/2026-11-24-b4-catalog/taxonomy-draft.md`。ID 由 Shopify ID 決定（頂層由 `top:<zh-TW 名稱>`），各環境相同。
- 沒有執行期寫入與管理畫面；要改分類就加 migration。新增子層或受管制品類前先做資格審核（ADR general-ecommerce-scope-cut）。
- `IsLeafCategory`：分類存在、有上層、沒有子節點。商品只能掛葉節點。
