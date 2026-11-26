# catalog 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

product-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

<!-- 空的那一列要換成真正的內容：TestEveryMovedModuleHasACarryOverAudit 在任何一格空白時失敗。 -->

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案：`product.id`、`hashtag.id` 用應用層 TSID，`product_sku.id` 用 BIGSERIAL | 跨進程不撞號；SKU id 不可枚舉 | 不成立：同一聚合兩種方案（CD10）；SKU id 外流到 cart、order 可被枚舉；TSID 沒有 node 位元 | 實體用應用層 UUIDv7，值的集合沒有 ID（ADR row-identity-by-data-kind） | D6：分類、商品、SKU 主鍵 UUIDv7（`Deps.NewID`）；hashtag、選項值、圖片引用是值的集合，用複合主鍵 |
| `status` VARCHAR(4) CodeMeta 數字碼（4001 草稿、4002 審核中〔預設〕、4003 未啟用、4004 啟用）＋`deleted_at`、`seller_deleted_at` | 賣家上下架；刪除後訂單與評價仍引用 | 狀態成立；預審不成立：沒有營運人員審，商品會永遠卡在審核中；CodeMeta 已凍結 | draft／active／archived 由賣家切換，不預審，違規事後治理（Shopify、蝦皮） | D16：`DRAFT`／`ACTIVE`／`ARCHIVED`（text＋CHECK，UPPER_SNAKE）；刪除是 `deleted_at`，列保留；`first_published_at` 保留，`ACTIVE` 必有它（`products_first_published_check`）；`seller_deleted_at` 不帶（D7：別人的狀態不複製進商品，可見性在讀取時 AND 店家可曝光與限制措施） |
| 庫存：`stock_transaction` 先查再扣、最後寫帳本；歸還用呼叫方帶的品項與數量（`StockService`） | 同一筆訂單重送只扣一次；取消時依原扣減歸還 | 需求成立；寫法不成立：併發雙扣（CD1）、歸還量信任呼叫方（CD2） | 帳本以冪等鍵 UNIQUE，先插帳本再條件扣減；歸還依原扣減（Shopify inventory adjustments） | D2：`stock_operations`（冪等鍵 UNIQUE）＋`stock_movements`（有號 delta）；扣減 `UPDATE … WHERE stock_qty >= n`；歸還只帶原扣減的鍵，累計不超過原扣減、先鎖原扣減列；重送內容不同回 `ErrIdempotencyKeyReused`；SKU 依 ID 升序鎖定；回歸測試 `TestConcurrentSameKeyDeductsOnce`、`TestRestoreFollowsTheOriginalDeduction`（mutation `b4-2.json`） |
| `stock_transaction`、`sales_count_event_dedup` 無清理，持續成長（CD5） | 庫存稽核；±1 計數去重 | 稽核成立；去重不成立：D3 改成同 transaction 更新與絕對值 | 帳本是稽核依據，量小時保留 | 去重表不建；`stock_movements` 只增不改、保留不清理（接受，B4 計畫 CD5） |
| 分類：`category` 自參照三層＋軟刪＋CodeMeta 狀態，admin 審核頁（CD11 打不存在的路由）；種子只有「蔬菜」「水果」 | 商品掛分類、依分類瀏覽 | 不成立：沒有營運人員做分類審核；種子是食品電商遺留 | 平台擁有固定分類法，賣家只選葉節點；分類變更是營運發佈的資料版本（Shopify、Google、蝦皮） | D8：`catalog.categories` 兩層、名稱 `LocalizedText`（JSONB，必有 zh-TW）、`shopify_taxonomy_id` 對照；版本化 migration 種子（10 個頂層、52 個葉節點，`taxonomy-draft.md`）；沒有執行期寫入、沒有審核頁；商品只能掛葉節點（`IsLeafCategory`）；種子 ID 由 Shopify ID 決定，各環境相同 |
| 商品上的分類三層 ID 與名稱快照 `cat_l1..l3_name` | 列表與訂單顯示分類名稱 | 不成立：分類是平台資料，讀時 join 即可；訂單快照是訂單自己的事 | 訂單行凍結名稱與單價；分類名稱不反正規化到商品 | D20：商品只存 `category_id`；快照由結帳用的商品快照 API 提供（B4-4） |
| hashtag：`hashtag` 實體表＋`product_hashtag` 關聯；hashtag 寫在描述文字裡由 client 抽（上限 10） | 標籤搜尋、熱門、自動完成 | 實體表不成立：違反 ADR row-identity-by-data-kind；從描述抽取不成立：兩套輸入模型與抽取規則 | 商品 tag 是值集合，明確欄位輸入；熱門由查詢聚合（Shopify tags） | D14：`product_hashtags(product_id, name, display_name, position)`；正規化規則與 social 共用一份（B4-2b 抽到 `platform`）；商品也用明確欄位 `hashtag_inputs`；各自聚合，不與貼文合併（Will 2026-11-24） |
| 價格：`price`＋`sale_price`（CHECK `0 < sale_price < price`）＋商品上 6 個 min／max 反正規化欄 | 顯示售價與原價、列表價格區間 | 成立；6 欄不成立：維護點多；限時特價是促銷的事 | `price`＋`compareAtPrice`；價格區間反正規化在商品上，同 transaction 重算 | D19：SKU `price`＋`compare_at_price`（`skus_compare_at_check`：大於售價）；商品 `min_price`／`max_price`（B4-2b 在 SKU 寫入時重算）；限時特價延後到促銷 |
| 規格：SKU 只有自由文字 `sku_name`＋`unit` | 一個商品多個規格 | 不成立：買家無法依規格篩選、賣家可建重複規格、client 畫不出選擇器 | 0–2 個選項軸，SKU 是值的組合，組合唯一（Shopify variants、蝦皮規格） | D18：`product_option_axes`／`product_option_values`；`product_skus` 的 `option1_value`、`option2_value`，`skus_combination_key` 對未刪除的 SKU 唯一；沒有選項的商品有一個預設 SKU（兩個選項值都是 NULL） |
| SKU 的 `shipping_method` JSONB（溫層、免運門檻、運費負擔者）、`restricted_category_codes`、商品的 `certifications`、`shipping_methods` | 運費與配送方式；範圍裁切前的食品與認證 | 不成立：運費規則歸 seller 的運費設定（B3，D10）；溫層與認證已裁切（ADR general-ecommerce-scope-cut） | 運費規則屬物流設定，商品只記實體屬性 | 不帶：catalog 不存配送方式、重量尺寸，不依賴運費設定（D10，Will 2026-11-24） |
| 圖片：`product.gallery` JSONB；描述 `description` JSONB blocks 內含圖片 | 商品圖庫與描述的圖文 | 圖片需求成立；JSONB 內嵌圖片不成立：引用只存在擁有者的表（ADR media-references-owner-side） | 有序資產引用清單；描述是結構化區塊（Shopify product media、Portable Text） | D17：`product_images(product_id, kind, position, asset_id, object_key, alt)`，`kind` 是 `GALLERY`／`DESCRIPTION`；描述是四種區塊（段落、圖片、清單、表格）的 JSON，圖片區塊以 `asset_id` 指向 `product_images`；儲存時 `Confirm`、移除時同 transaction `MarkDeleted`（B4-2b） |
| `searchable_text` ILIKE `%t%`（無索引的掃描） | 商品搜尋 | 成立；ILIKE 不成立：量大時全表掃描 | 百萬列以下用 `pg_trgm`；量測觸發才換搜尋引擎（GitLab、Discourse） | D15：`searchable_text` 欄由寫入端組（標題、描述文字、hashtag、選項軸名稱與值、分類名稱；B4-3a 加入選項）；B4-3a 的搜尋與賣家關鍵字查詢一起建 `pg_trgm` 擴充與 GIN trigram 索引（`ILIKE` 命中、`word_similarity` 排相關度）。`pg_trgm` 由平台擁有（`internal/platform/migrate` 的 `extensions`，`Apply` 先裝；Atlas 的 dev database 由 `make atlas-diff` 先裝），索引由 Atlas 從 schema.sql 產生進 migration `product_api_support`，模組不寫 `CREATE EXTENSION`；排序用 `CASE` 算鍵、沒有索引，量測觸發才換 |
| `version` 欄（V16、V17）與 `expectedVersion` 409 | 並發修改偵測 | 成立；欄位不成立：ADR optimistic-concurrency-etag 規定用 etag | etag（If-Match 語意） | D4、B4-3a：欄位叫 `content_version`（起始 1），對外是 etag（`Etag`／`ParseEtag`）；只在賣家可編輯的內容與狀態改變時加一，庫存、銷量、收藏數、評分、價格區間重算都不加（Shopify 商品與庫存是不同資源）；`UpdateProduct`、`SetProductStatus` 必帶版本，`DeleteProduct` 選填 |
| `idempotency_request` 表＋`IdempotencyCleanupJob`；`FavoriteEventDedupCleanupJob` | 寫入重送冪等 | 不成立：D4 規定寫入帶 `request_id`；去重表隨 D3 消失 | 請求冪等鍵（Stripe） | 不帶表與清理 job：B4-3a 的 `CreateProduct` 以 `(store_id, request_id)` 唯一索引加 `request_hash`（`platform/idempotency` 的雜湊）冪等、永久保存；賣家的 `AdjustStock`、`SetStock` 以 `stock_operations` 的 `(store_id, key)` 加 `request_hash` 冪等；同鍵併發 `ON CONFLICT DO NOTHING` 後讀回比對 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| `store_id` 由請求帶（CD9），gateway 在代操作時覆寫；失敗時以帳號 ID 當 `store_id` | 賣家只能動自己的店 | 不成立：帳號 ID 與店家 ID 撞值可讀到他店商品 | 店家身分來自憑證（Shopify Admin API） | D4：賣家 API 不收 `store_id`，由呼叫者推導；B4-3 實作，回歸測試跨店讀取回 not_found |
| 一套 REST 路由涵蓋公開、買家、賣家、後台 | 依角色授權 | 不成立：權限宣告混雜 | 依角色分 service（API 設計規範 Connect） | D4：Public、Buyer、Seller、Admin、Delegated 各一支 service（B4-3） |
| 商品與分類的 Redis 快取、`DELETE /admin/products/cache` | 降低讀取延遲 | 不成立：同進程直接查 DB，沒有量測 | 沒有量測前不加快取 | 不帶（D5：沒有 client 呼叫的端點隨 Java 刪） |

## 與 product-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- ID：UUIDv7（Java 是 TSID 與 BIGSERIAL）。
- 狀態：`DRAFT`／`ACTIVE`／`ARCHIVED`＋`deleted_at`（Java 是 4001–4004，預設「審核中」）。
- 庫存：`stock_operations`＋`stock_movements` 帳本（Java 的 `stock_transaction`），冪等鍵 UNIQUE；歸還依原扣減。
- 分類：兩層、版本化種子、`LocalizedText` 名稱、Shopify 對照（Java 三層可編輯、種子只有兩筆）。
- 規格：0–2 個選項軸，SKU 是組合；價格 `price`＋`compare_at_price`（Java 自由文字規格、`price`＋`sale_price`）。
- 沒有：`certifications`、溫層、`shipping_methods`、`seller_deleted_at`、`version`（改叫 `content_version`）、分類名稱快照、`idempotency_request`（改成 `request_id`）。
- 賣家設定庫存（B4-3a）：`SetStock(quantity, expected)` 比對預期值的條件式 `UPDATE`，Java 沒有；低庫存門檻（每商品選填，預設 5）、關鍵字搜尋與賣家清單 `query`、後台游標列表與總數、公開單筆含 ARCHIVED 也是 B4-3a 新增，Java 的列表是 offset 與 ILIKE。
- 賣家調整庫存（B4-2b）：`AdjustStock` 以冪等鍵與有號 delta 記 `ADJUST` 操作，條件式更新、限定自己店家，既有 SKU 的庫存只由 stock 改（Java 沒有對應的專用方法）。
- 商品寫入（B4-2b）：整體取代；SKU 以選項值的組合辨識（Java 以 ID），組合在就更新價格、庫存不動；選項軸與值整組重寫。
- 描述：四種結構化區塊（段落、圖片、清單、表格），圖片區塊只存 `asset_id`（Java 的 `table_json`／`list_json` 是不解析的字串）；上限見 `internal/model/product.go`。
- hashtag：明確欄位輸入，正規化在 `platform/hashtag`，與 social 同一份（Java 從描述抽取）。
- 刪除商品不 `MarkDeleted` 圖片：列保留給訂單與評價引用，圖片的去留待訂單快照（D20、B4-4）決定。
- 計數與補貨（B4-3b）：`SetRating` 傳絕對值、由 review 在自己的 transaction 內呼叫，沒有版本欄；Java 的 `ReviewStatsService` 以事件累計。銷量以 `sales_orders`（`order_no` 去重）冪等，必要條件是 Java order 的 MQ 會重送，B8 之後退場。補貨通知（D26）是新增：Java 只有賣家以外的路徑會通知、賣家補庫存不通知（視為缺陷）；job 是 River、延後 5 分鐘、唯一鍵合併未結束的同 SKU job（River 不允許排除 running，執行中的 job 會擋下同 SKU 的新排入，窗口是毫秒）。
