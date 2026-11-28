# catalog 模組行為規格

本文是單體 `catalog` 模組行為的唯一來源：商品、規格（選項軸與 SKU）、庫存與平台分類樹。步驟與決策在 `backend/docs/plans/2026-11-24-b4-catalog.md`（D1–D20），Java product-service 的盤點在同名目錄。實作隨 B4 各子步驟增加；本文只寫已實作的部分，其餘標明步驟。

## 1. 範圍與公開介面

- 實作在 `internal/modules/catalog`，自己的 `catalog` schema 與 migration，登記在 `cmd/hawkr/modules.go` 的 `modules()`。不依賴 fulfillment（D10：運費與配送方式歸 seller 的運費設定，B3）。
- 根套件的方法：`DeductStock`、`RestoreStock`、`AdjustStock`、`SetStock`、`SetRating`、`RecordSales`、`Categories`、`IsLeafCategory`、`CreateProduct`、`UpdateProduct`、`SetProductStatus`、`DeleteProduct`、`Product`、`PublicProduct`、`AdminProduct`、`ListSellerProducts`、`CountSellerProducts`、`ListPublicProducts`、`ListPublicProductsOfStores`、`ListAdminProducts`、`SuggestProductNames`、`SuggestHashtags`、`TrendingHashtags`、`ProductCards`、`ProductStoreIDs`、`StoresWithPublicProducts`、`CheckoutSnapshot`、`SkuCartInfo`、`AddFavorite`／`RemoveFavorite`、`Etag`／`ParseEtag`、`SuggestedSafetyStock`。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 資料

| 表 | 內容 |
|---|---|
| `categories` | 兩層分類樹；名稱 `LocalizedText`（JSONB，必有 zh-TW）；葉節點有 `shopify_taxonomy_id` |
| `products` | 商品：`DRAFT`／`ACTIVE`／`ARCHIVED`、`deleted_at`、價格區間、反正規化計數、描述區塊 JSON、`searchable_text`（含選項軸名稱與值，pg_trgm GIN 索引）、`content_version`（etag）、`low_stock_threshold`（選填，預設 5）、`request_id`＋`request_hash`（建立的冪等） |
| `product_option_axes`、`product_option_values` | 0–2 個選項軸與有序的值 |
| `product_skus` | 選項值的組合；`price`、`compare_at_price`、`stock_qty`、`deleted_at` |
| `stock_operations`、`stock_movements` | 庫存帳本：一次操作一列（冪等鍵：結帳的操作全域唯一、賣家調整依 `store_id` 在店家內唯一，各一個部分唯一索引），明細是有號 delta；賣家的 `ADJUST`／`SET` 操作記 `request_hash` |
| `product_images` | 圖庫與描述圖片的資產引用 |
| `product_hashtags` | 商品的 hashtag 值集合 |
| `sales_orders` | 銷量累計的去重：`order_no` 主鍵，同一筆訂單只累計一次（B8 之前 Java order 的 MQ 會重送，之後退場） |
| `restock_queue`、`restock_purge_queue` | 補貨通知與清訂閱的佇列（D26）：`sku_id` 主鍵，`due_at`、`requested_at`、`claimed_until`（租約）、`attempts`、`last_error`、`failed_at` |

## 3. 庫存

- 扣減（`DeductStock(ctx, key, items)`）：先插入 `stock_operations`（`ON CONFLICT DO NOTHING`），再依 SKU ID 升序逐一 `UPDATE … WHERE stock_qty >= n AND deleted_at IS NULL`，每個寫一筆 `stock_movements`（delta 為負）。任何一個 SKU 不足整次回滾，回 `*InsufficientStockError`（第一個不足的 SKU）；SKU 不存在或已刪除回 `ErrSkuNotFound`。
- 重送：同一個 key 內容相同（品項與數量，不論順序）回成功且不改庫存；內容或種類不同回 `ErrIdempotencyKeyReused`。併發的同一個 key 在 UNIQUE 上排隊，後到的看到 0 列後走重送路徑。
- 歸還（`RestoreStock(ctx, key, deductKey, items)`）：依 `deductKey` 找原扣減（不存在回 `ErrDeductionNotFound`），先鎖原扣減列，再檢查每個 SKU 的累計歸還量不超過原扣減量（`ErrRestoreExceedsDeduction`）；`items` 為空表示歸還剩下的全部（沒有剩下的也回 `ErrRestoreExceedsDeduction`）。歸還不檢查 SKU 是否已刪除。
- 賣家調整（`AdjustStock(ctx, storeID, key, skuID, delta)`）：賣家改自己店家某個既有 SKU 的庫存，`storeID` 由呼叫端推導。用有號變動量而不是絕對值，因為 `stock_movements` 存的是有號 delta（`delta <> 0` 的只增不改帳本），變動量重送時內容固定可比對。先插入 `kind = 'ADJUST'` 的 `stock_operations`，再 `UPDATE … FROM products WHERE store_id = n AND 結果 BETWEEN 0 AND 2147483647`，寫一筆 movement。SKU 不存在、不屬於該店家或已刪除（含商品已刪除）回 `ErrSkuNotFound`；結果小於 0 回 `*InsufficientStockError`；結果超過 int32 上限回 `ErrInvalidStockInput`；失敗整筆回滾（含操作列）。重送規則同扣減，但鍵只在該店家內唯一：別家店或結帳用同一個 key 是各自獨立的操作，互相看不到。
- 輸入：key 1–200 字元、頭尾無空白；品項 1–100 個、數量為正、同一個 SKU 不重複；違反回 `ErrInvalidStockInput`。
- transaction：全部寫入在呼叫方的 transaction 內，沒有時自己開一個；呼叫方 transaction 內失敗時必須整個回滾。
- 扣減時點是「下單即扣、逾時未付歸還」（D2）；歸還的觸發在 B8。

- 賣家設定（`SetStock(ctx, storeID, key, skuID, quantity, expected)`）：把自己店家某個 SKU 的庫存設成絕對值，比對與寫入是同一句條件式 `UPDATE … WHERE stock_qty = expected`；現值不等於 `expected` 回 `*StockMismatchError`（帶現值），整次不生效、不留操作列。成功記一列 `SET` 操作與差額 movement（差額為 0 時只有操作列）。`quantity`、`expected` 在 0 到 int32 上限。重送：同鍵且（SKU、quantity、expected）相同回成功、不再比對現值（第一次已生效）。
- 冪等（`ADJUST`、`SET`）：鍵是賣家的 `request_id`，範圍是店家；`stock_operations.request_hash` 存請求內容（種類、SKU、delta 或 quantity／expected）的雜湊，同鍵內容或種類不同回 `ErrIdempotencyKeyReused`；併發同鍵靠插入操作列的 `ON CONFLICT DO NOTHING`，後到的讀回比對。庫存調整與設定不改商品的 etag。

## 4. 分類

- 種子在 migration `catalog_taxonomy_seed`：10 個頂層分組、52 個葉節點，來源與對照見 `backend/docs/plans/2026-11-24-b4-catalog/taxonomy-draft.md`。ID 由 Shopify ID 決定（頂層由 `top:<zh-TW 名稱>`），各環境相同。
- 沒有執行期寫入與管理畫面；要改分類就加 migration。新增子層或受管制品類前先做資格審核（ADR general-ecommerce-scope-cut）。
- `IsLeafCategory`：分類存在、有上層、沒有子節點。商品只能掛葉節點。

## 5. 商品寫入（B4-2b）

呼叫端（B4-3 的 handler）從登入身分推導 `storeID` 與 `media.Uploader`（店家所屬帳號），不信任 client 送的店家；這一層每個寫入與賣家讀取都以 `storeID` 限定，商品不是這家店的或已刪除回 `ErrProductNotFound`。輸入不合法回 `*InvalidInputError{Field, Reason}`（`errors.Is(err, ErrInvalidInput)`），使用者面文字由呼叫端依欄位與原因產生。

- `CreateProduct` 建立草稿；`UpdateProduct` 整體取代內容（狀態不變）；`SetProductStatus` 切換狀態；`DeleteProduct` 設 `deleted_at` 並把它還在的 SKU 一併標刪除（不可還原，列與圖片引用保留；刪除後扣減與賣家調整找不到這些 SKU，歸還原扣減仍可，D2）。全部在一個 transaction，並以 `LockOwnProduct` 鎖商品列。鎖順序：新圖的 `Confirm` 在鎖商品列之前，既有 SKU 列在商品列之後一次依 ID 升序鎖住（與 `DeductStock` 同序）；`MarkDeleted` 要比對寫入前後的圖，在商品列鎖住之後（media 只 INSERT 不鎖既有列，見 media.md）。讀取（`Product`、`PublicProduct`）以一句 SQL 讀出商品列與全部子表，是一份快照（會被別的模組呼叫，不開 `InReadSnapshot`，backend/AGENTS.md 規則 3）。
- 狀態（D22）：`DRAFT`、`ACTIVE`、`ARCHIVED` 任兩個都能互切，目標不是有效狀態回 `*InvalidInputError`，切成目前狀態是 no-op。切成 `ACTIVE` 時商品必須至少有 1 張主圖，ACTIVE 的商品每次 `UpdateProduct` 也一樣，否則 `ErrActiveRequiresGalleryImage`。`first_published_at` 只在第一次設成 `ACTIVE` 時填，之後不變、切回其他狀態也不清除。分類必須是葉節點。
- 選項與 SKU（D18）：0–2 個軸，軸名稱不重複，每軸 1–50 個值且不重複；SKU 以組合辨識，每個位置的值必須在該軸的值內、沒有軸的位置為空、組合不重複；沒有軸的商品剛好一個預設 SKU；SKU 最多 100 個。價格為正整數新台幣元，`compare_at_price` 必須大於售價。更新時組合還在就更新價格（庫存不動）、新組合新增（庫存給初始值）、不在輸入的組合以 `deleted_at` 刪除。
- 價格區間（D19）：每次 SKU 寫入的同 transaction 以沒刪除 SKU 的最小與最大價重算 `min_price`／`max_price`。
- 描述（D17 A）：`paragraph`（純文字）、`image`（新 token 或現有 `AssetID`，alt）、`list`（項目）、`table`（列的儲存格）。上限（D21）：區塊 100、描述圖片 20、整份描述的總字數 3000（段落、清單、表格文字合計）、清單 50 項、表格 50 列 × 10 欄、清單項與儲存格 200 字；上限寫在 `api/` 商品寫入訊息欄位的自訂 option，`uitext-gen` 生成 Go／TS／Dart 常數，DB CHECK 保留並由 drift 測試對齊。JSON 存在 `products.description`，圖片區塊只含 `asset_id`。
- 圖片：圖庫最多 9 張（待確認）與描述圖片都在 `product_images`（`GALLERY`／`DESCRIPTION`，各自從 0 起排序）。新 token 在同 transaction 內、鎖商品列之前 `media.ConfirmFields`（公開圖片），既有的圖以 `AssetID` 引用且必須屬於這個商品；沒有被引用的現有資產同 transaction `MarkDeleted`。Confirm 或 MarkDeleted 失敗整筆回滾。同一個 token 或資產不得出現兩次。
- hashtag（D14）：明確輸入，正規化與驗證是 `platform/hashtag`（與 social 同一份：去頭尾空白與 `#`、NFC、ASCII 小寫；字元集 a-z、0-9、漢字；1–40 字；最多 10 個）；寫入 `product_hashtags(name, display_name, position)`。
- `searchable_text`：標題、描述文字（段落、清單項、儲存格）、hashtag 名稱、分類名稱（en，缺時 zh-TW），以單一空格相接。

## 6. 讀取與列表

- `Product`（賣家，任何狀態、沒刪除）、`PublicProduct`（ACTIVE 或 ARCHIVED、沒刪除、店家在 `at` 可曝光；草稿回 `ErrProductNotFound`，下架的商品頁保留，呼叫端以 `Status` 決定 `availability`，D24 Q10）與 `AdminProduct`（任何店家、任何狀態、沒刪除）回完整內容：描述區塊、圖庫、選項軸、SKU、hashtag、`LowStockThreshold`、`Version`。
- 版本與 etag（D24 Q5）：`Product.Version` 起始為 1，只在賣家可編輯的內容（`UpdateProduct`）與狀態（`SetProductStatus` 實際改變狀態）改變時加一；庫存調整與設定、扣減與歸還、銷量、收藏數、評分、價格區間重算都不加。`UpdateProduct` 與 `SetProductStatus` 的 `version` 必填（`< 1` 回 `*InvalidInputError`），在鎖住商品列之後比對，不符回 `ErrVersionConflict`；`DeleteProduct` 的 `version` 選填（0 不比對）。對外 etag 是 `Etag(version)`，`ParseEtag` 還原。
- 建立的冪等（D24 Q4）：`CreateProduct(…, requestID, in)`；`(store_id, request_id)` 唯一，`request_hash` 是原始輸入（含新圖 token）的雜湊。同鍵同內容回第一次建立的商品目前的樣子（不再 Confirm 新圖），不同內容回 `ErrIdempotencyKeyReused`，已刪除的回 `ErrProductNotFound`。同鍵併發：`INSERT … ON CONFLICT DO NOTHING` 回 0 列的請求整筆回滾（新圖的 Confirm 也不留），在 transaction 外讀回先到的那筆、比對雜湊。帶新圖 token 的併發重送，後到者可能先在 `Confirm` 失敗（token 已被先到者用掉）而不是走重送路徑。
- 賣家清單與計數（Q6a、Q11）：`ListSellerProducts(storeID, SellerFilter{Status, StockStatus, Query}, after, size)` 與 `CountSellerProducts` 共用篩選。`Query` 以 `searchable_text ILIKE %query%` 比對（`%`、`_`、`\` 跳脫）；`StockStatus` 的 `LOW`、`OUT_OF_STOCK` 只比對 ACTIVE 的商品：`LOW` 是至少一個沒刪除的 SKU 庫存大於 0 且小於或等於商品的門檻（`low_stock_threshold`，沒設用 `DefaultLowStockThreshold` = 5），`OUT_OF_STOCK` 是至少一個 SKU 庫存為 0，兩者各自獨立。門檻範圍 0–9999（0 表示只有售完才提醒）。
- 建議安全庫存（Q6b）：`SuggestedSafetyStock(weeklySales)`：最近 8 週每週銷量（舊到新，缺的週算 0）；沒有銷量是 10，否則 `max(5, ceil(平均週銷量 × 2))`。銷量的來源是訂單，不在 catalog；這個函式只做規則，由呼叫端提供銷量（B4-3b）。
- `ListPublicProducts(at, PublicFilter, sort, after, size)`：只含買家看得到的商品（`catalog.buyer_visible_products(at)`：ACTIVE、沒刪除、店家在 `at` 可曝光、沒有生效中的 `PRODUCT` 下架措施；`db/objects.sql` 的唯一定義，單筆頁用含 ARCHIVED 的 `buyer_viewable_products(at)`）；`at` 是請求的 clock。購物車與結帳快照的 `Active`、`SkuPurchasable` 與 `DeductStock` 用同一個定義，不可見的商品扣庫存回 `ErrProductNotPurchasable`（`PRODUCT_NOT_PURCHASABLE`）；`ProductCards` 回 `Visible`，已下架的商品直接連結仍可瀏覽（`Visible` 真、`available` 假），被措施下架或店家不可曝光的卡片 `Visible` 為假、買家 API 只回 id。篩選：分類（頂層展開成它底下的葉節點）、價格區間（與 [min_price, max_price] 有交集）、hashtag（ANY／ALL）、店家、關鍵字（`searchable_text ILIKE`，GIN trigram 索引）。排序 `LATEST`（預設）、`PRICE_ASC`、`PRICE_DESC`、`SALES`、`RATING`、`RELEVANCE`（`word_similarity`，要有關鍵字）。`ListPublicProductsOfStores(at, storeIDs, …)` 是指定店家的最新上架，`storeIDs` 為空回空頁（供 buyer 的追蹤中動態）。`SuggestProductNames`、`SuggestHashtags`、`TrendingHashtags` 只算可見商品。
- `ListAdminProducts(AdminFilter{StoreID, Query}, after, size)`：全站沒刪除的商品，最新建立在前，回傳符合篩選的總數（每頁相同）。
- 分頁是 keyset（`ListCursor{Key, ID}`）：賣家與後台的 `Key` 是建立時間的微秒數，公開列表依排序決定（`LATEST` 是首次上架的微秒數，價格低到高取負值，評分乘 1000 取整，相關度乘 10^6 取整），同鍵依 ID 遞減；頁大小預設與上限 100。游標的編碼與跟請求參數的綁定在對外層（B4-3b）。列帶主圖庫第一張的 object key、沒刪除 SKU 的庫存總和（上限 int32）。
- 公開排序用 `CASE` 算出排序鍵，沒有對應的索引：量測前不加（D15）；只有 `searchable_text` 的 ILIKE 有 GIN 索引。
- 還沒有：有效限制措施（D7）的排除（B4-5）。

## 7. 評分、銷量與補貨通知（B4-3b）

- `SetRating(ctx, productID, average, count)`：review 在自己的 transaction 內把商品的評分平均（0 到 5）與評價則數設成絕對值；沒有版本欄，並行呼叫靠商品列的行鎖序列化，後提交的勝出；不改 etag。`count` 為 0 時 `average` 必為 0，不合法回 `*InvalidInputError`，商品不存在回 `ErrProductNotFound`（已刪除的商品仍更新）。
- `RecordSales(ctx, orderNo, items)`：把一筆訂單的銷量累計到各商品（同商品先加總），同一個 `orderNo` 只累計一次（`sales_orders` 去重），重送回 nil 不再累計。與呼叫方的 transaction 同進退；任何商品不存在整筆回滾並回 `ErrProductNotFound`。在呼叫方的 transaction 內回 error 時，呼叫方必須回滾那個 transaction（這裡不開 savepoint，否則登記了訂單卻沒累計，之後的重送被去重擋下）。
- 補貨通知（D26）：SKU 由不可買變可買（庫存大於 0、SKU 未刪、商品 ACTIVE 且未刪、店家可曝光），來源不限（`SetStock`、`AdjustStock`、`RestoreStock`、商品切成 ACTIVE）。偵測與排入在寫入的 transaction 內：庫存增加後該 SKU 庫存等於增加量（即原本是 0）且此刻可買，就寫一列 `restock_queue`，`due_at` 是請求時間加 5 分鐘；商品切成 ACTIVE 對它每個沒刪除的 SKU 都寫一列，不讀庫存。
- 合併與處理中的請求：沒被認領的列不動（保留較早的 `due_at`）；被認領處理中的列只把 `requested_at` 往後推至少 1 微秒。worker 完成時只在 `requested_at` 沒變才刪，變了就放掉租約、`due_at` 重設成現在加 5 分鐘，所以處理中的轉換不會遺失。
- worker（River 週期 job，每 30 秒，`PeriodicJobs()` 與 `AddWorkers` 匯出，B4-6 接線）：以 `FOR UPDATE SKIP LOCKED` 認領到期且沒被認領（或租約已過）的列並寫 5 分鐘租約，在 transaction 外重新確認可買、讀最新的名稱與主圖，呼叫 `RestockNotifier.NotifyRestocked`；不可買就不通知、刪列。通知是一次性訂閱，重處理安全。
- 失敗：同一列 `attempts` 加一、`due_at` 退避（1 分鐘起倍增，上限 1 小時）、記 `last_error`；累計 5 次設 `failed_at` 停手，列保留，新請求會重置。
- 清訂閱：商品或 SKU 刪除的 transaction 內寫 `restock_purge_queue`（立即到期），worker 把一輪認領的 SKU 合併成一次 `PurgeSKUSubscriptions`，語意同上。
- 不以店家恢復曝光觸發通知（Will 2026-11-27）。

## 8. 給別的模組與轉接層的批次讀取與收藏數（B4-4a）

- `ProductCards(at, ids, includeInactive)`：沒刪除的商品的卡片（`ProductSummary`）；`includeInactive` 為假只回 ACTIVE 且店家在 `at` 可曝光的，為真任何狀態（含草稿、下架、店家不可曝光）都回，直接連結仍可瀏覽。缺席的 ID 是不存在或已刪除。
- `ProductStoreIDs(ids)`：沒刪除的商品所屬店家，任何狀態、不看曝光。
- `StoresWithPublicProducts(at)`：至少有一件 ACTIVE 且沒刪除的商品、店家在 `at` 可曝光的店家 ID，升序（沒有時是空切片）。
- `CheckoutSnapshot(skuIDs)`、`SkuCartInfo(skuIDs)`（D20，不版本化）：同一句查詢回 `SkuSnapshot`：SKU 與商品此刻的標題、選項值（以 ` / ` 串接，預設 SKU 為空）、主圖 key、售價、原價（有才有值）、庫存、商品狀態。不過濾狀態與刪除（訂單與購物車仍指向已下架、已刪除的 SKU），`Active` 是商品 ACTIVE 且商品與 SKU 都沒刪除；不看店家曝光與庫存（庫存由呼叫端看 `StockQty`）。缺席的 SKU 是不存在。配送方式、重量與尺寸不在 catalog（D10），快照不帶。
- 批次輸入最多 `MaxBatchIDs`（200）個，超過或含零值回 `*InvalidInputError`；空輸入回空 map。
- `AddFavorite(productID)`／`RemoveFavorite(productID)`（D3）：buyer 在收藏列同一個 transaction 內加減收藏數，ctx 沒有 transaction 時是單句寫入；不改 etag。`AddFavorite` 對已刪除的商品回 `ErrProductNotFound`；`RemoveFavorite` 下限 0、已刪除的商品照減（計數跟著收藏列走）。重複與去重是呼叫方的事（收藏列的唯一鍵），這裡不去重。
- 轉接：`internal/legacy/catalogserver` 以舊的 product gRPC 契約服務 Java cart 的 `GetProductSimple`、`BatchGetSkuCartInfo` 與 order 的 `DeductStock`、`RestoreStock`、`BatchGetProductForCheckout`；`internal/legacy/catalogconsumer` 消費 `order.cancelled`（歸還）與 `order.sales.accumulated`（銷量）。兩者的歸還鍵是 `catalogserver.RestoreKey(stock_tx_id)`，指向原扣減 `stock_tx_id`。組裝在 B4-6。

## 9. 代行寫入與批次下架（B4-5d）

- `CreateProduct`、`UpdateProduct`、`SetProductStatus` 帶明確的代行者參數（店主傳 nil），成功的寫入在商品寫入的 transaction 內發事件（`ProductCreated`、`ProductUpdated`、`ProductPublished`、`ProductUnpublished`，已在目標狀態時發 `ProductPublishRepeated`、`ProductUnpublishRepeated`）；帶代行者的事件要有稽核 handler（`delegatedactor.AuditHandler`）才發得出去。重送的建立不寫入、不發事件；代行者建立時同一家店的 `request_id` 已建立過商品，回 `ErrCreateReplayed`（不發事件；同 session 同 `request_id` 的重送由 delegation 的稽核列在呼叫之前就回了第一次的結果）。`SetProductStatus` 回傳寫入後（no-op 時是目前）在同一個 transaction 內讀出的商品。
- `UnpublishProducts`（D29）原子批次下架：商品依 ID 升序鎖列，任一項版本與目前不符整批回 `*BatchVersionConflictError`（`errors.Is(err, ErrVersionConflict)` 成立，列出全部衝突的商品）、沒有寫入；已經下架的項目視為成功（不改版本，`VersionBefore` 與 `VersionAfter` 都是目前版本）；代行者的批次裡有草稿商品整批回 `*BatchNotPublishedError`（`errors.Is(err, ErrNotPublished)`，列出全部草稿）、沒有寫入；別家或不存在的商品整批回 `ErrProductNotFound`；成功發一個 `ProductsUnpublished`（項目依商品 ID 升序），並把同一份項目回給呼叫方（呼叫方據它組回應，不再讀商品）。
- 代行者不得把商品切成草稿（回 `*InvalidInputError`，`Field` 是 `Status`），也不得下架草稿商品（回 `ErrNotPublished`，與 social 的代行下架同規則）。

