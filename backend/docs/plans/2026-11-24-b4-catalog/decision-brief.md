# B4 決策簡報 v2：catalog 從零設計（業界基線 vs 現況）

**拍板：** 2026-11-24 Will，D1–D20 全部照各題的「建議」（D13 併入 D8）。D18 的四端選規格 UI 改動一併同意。仍待辦：D8 初版分類清單交 Will 看；D14 商品與貼文熱門 hashtag 是否合併（產品問題，B4-2 開工前問）；D10、D11 的執行先與 B3 session 對齊。

快照 dev `496b048e2`。證據標記：【量】＝讀過 source 或跑過指令；【推】＝推論；【文件】＝公開文件的已知做法，沒在本 repo 實測。盤點見同目錄 `interface-inventory.md`、`client-inventory.md`、`defects.md`。

每題的格式：業界基線 → 現有機制為什麼存在 → 選項（各附「什麼情況下會失敗」）→ 建議 → 什麼事實會推翻建議 → 之後能否反悔、代價。

## 前提題：被改的程式是否排定要重寫、搬走或刪除？

| 程式 | 排定 | 分類 | 理由 |
|---|---|---|---|
| Java product-service（12 表、46 RPC） | B4 本身重寫 | — | 全部從零設計，Java 的形狀不是限制 |
| Java order-service | B8 | **REAL** | B8 的前置是 B2、B3、B4、B6（主計畫 B8 段「同一 transaction 鎖購物車、扣庫存」【量】）；B4 自己就是 B8 的前置，B8 不可能先做。合併 B4+B6+B8 是 2 萬行以上的一步【推，B7 簡報量過 order 約 20,021 行】 |
| Java cart-service | B6 | **REAL** | 依賴表 `cart → catalog`（`modules.go:24`【量】），catalog 不在單體前 cart 搬不進來 |
| ai-service | A9（進行中） | SELF-IMPOSED | 我們擁有它；但 A9 已在跑，等它刪除不花額外成本 |
| gateway 代行商品路由（7 條） | A10 | **SELF-IMPOSED** | 我們擁有；A7-2d 已定代行 Connect 形狀；沒有無法滿足的前置 → B4 直接做 |
| Java shipping-service、fulfillment | B3（另一 session 並行） | SELF-IMPOSED（協調） | 我們自己的排程；不推離基線，只決定動同檔的先後 |

丟棄的轉接（B6／B8 刪）：`legacy/catalogserver` 5 支 RPC＋`legacy/catalogconsumer` 2 個事件，參考 `legacy/couponserver` 估 400–700 行，占 B4 約 5–10%【推】。原則同 B7：模組 API 照「同一 transaction 內呼叫」設計，轉接只包一層，不決定模組形狀。

## 限制帳

| 限制 | 分類 | 推離基線？ | 來源 |
|---|---|---|---|
| B6／B8 前 cart、order 在 Java、跨進程扣庫存 | REAL | 是：需要暫時轉接與跨進程冪等 | 上表 |
| B3 並行改 shipping | SELF-IMPOSED（協調） | 否：只影響先後，邊界照基線（D10） | 使用者指定 |
| 系統沒上線 | 有利條件 | 否：不搬資料、契約可直接改 | 根 `AGENTS.md` |
| 範圍裁切（溫層、產銷履歷、有機標章、認證） | REAL（已決 ADR） | 否 | ADR `2026-10-27-general-ecommerce-scope-cut` |
| Java 的 TSID＋BIGSERIAL、±1 計數器、Redis、hashtag 實體表、`idempotency_request` 表、CodeMeta 狀態字串、SKU 上的運費規則 | SELF-IMPOSED | 不沿用 | 盤點 |
| 和現有程式一致 | SELF-IMPOSED | 不採用為理由 | 根 `AGENTS.md`「沿用也是決策」 |

## 決定題

### D1 Java cart／order 的過渡（擋 B4-1、B4-4）
- 基線：單體內結帳、扣庫存同一 transaction（Shopify 等單體或同庫做法【文件】）；B8 達成。
- 現有機制：微服務之間只能 gRPC＋MQ。
- 選項：
  - A：`legacy/catalogserver`（cart、order 用的 5 支 RPC）＋`legacy/catalogconsumer`（`order.cancelled`、`order.sales.accumulated`），轉呼叫模組 API，B6／B8 刪。失敗條件：轉接與模組各自判冪等→只以模組的冪等鍵判定。（建議）
  - B：B4、B6、B8 合併一步。失敗條件：一步跨數萬行、前置 B2、B3 未完成。
- 推翻的事實：B8 前置全部完成。反悔代價：低。

### D2 庫存模型（擋 B4-2；修 CD1、CD2）
- 基線：
  - 數量放 SKU，**每次變動寫帳本**（Shopify inventory adjustments 帶 reason、Square inventory changes【文件】），帳本以呼叫方的冪等鍵唯一。
  - 扣減時點：單一倉、無預購時「下單即扣、逾時未付歸還」（蝦皮、momo 等【文件】）；多倉或出貨才定案時用「保留→出貨扣」（Shopify committed／available、Amazon reserved【文件】）。
  - 歸還依原扣減紀錄，不信任呼叫方數量。
- 現有機制：Java 先查帳本、扣、最後寫帳本（CD1 併發雙扣【量】）；歸還用呼叫方數量（CD2【量】）。
- 選項：
  - A：`catalog.stock_movements`（冪等鍵 UNIQUE、SKU、數量、原因、關聯原扣減），同 transaction「先插帳本→條件扣減 `WHERE stock >= n`」；歸還只帶冪等鍵（可部分，累計不超過原扣減）；下單即扣，逾時歸還由 B8 觸發。失敗條件：日後多倉→要加倉庫維度與保留狀態。（建議）
  - B：保留模型（on_hand／committed／available）。失敗條件：自寄流程沒有可靠的出貨扣減點，多一個狀態沒有使用者價值。
  - C：照 Java。失敗條件：CD1、CD2 留著。
- 推翻的事實：要多倉或預購。反悔代價：中（加欄位、改扣減點，帳本沿用）。

### D3 衍生計數：銷量、收藏數、評分（擋 B4-3；修 CD3、CD4、CD5）
- 基線：列表排序與顯示需要的計數**反正規化在商品上**，由事實擁有者在同一 transaction 更新，或以集合重算校正（評分摘要由 review 系統產生後寫到商品，Amazon、Shopify 評論 app 的做法【文件】）；不用無序 ±1 事件。
- 現有機制：微服務只能 MQ，所以 ±1 加去重表。
- 選項：
  - A：收藏數由 buyer 的 in-process 事件在同 transaction 更新（buyer 在 catalog 下層，`modules.go:22`【量】；`events.Publish` 在 transaction 內、同 transaction 訂閱者可做原子狀態變更，`backend/AGENTS.md` 單體規則 4【量，規則文字】）；評分由 review 呼叫 `SetRating(product, avg, count)` 傳重算後的絕對值；銷量在 B8 前由 `catalogconsumer` 以 `order_no` 冪等，B8 改成訂單完成的同 transaction。去重表全刪。失敗條件：熱門商品收藏寫入同一列排隊→量測觸發後改批次重算。（建議）
  - B：不反正規化，讀取時 join `review.pub_*`、`buyer.pub_*`。失敗條件：按評分、銷量排序每次跨 schema 聚合，無法建排序索引。
  - C：照 Java。失敗條件：CD3–CD5。
- 推翻的事實：量測顯示單列熱點。反悔代價：低。

### D4 對外 API 形狀（擋 B4-3；修 CD9）
- 基線：依呼叫者角色分 API（Shopify Storefront／Admin API、蝦皮 Open API 的賣家端【文件】）；店家身分來自憑證；寫入用 idempotency key（Stripe【文件】）。
- 本 repo SSOT：〈API 設計規範（Connect）〉已規定依角色分 service、`*DelegatedService` 由擁有資源的模組擁有、寫入帶 `request_id`（`docs/conventions/api-design-guidelines-connect.md:30`、`:176`【量】）。
- 現有機制：gateway REST 一套路由、`store_id` 由請求帶（CD9）；冪等用 `idempotency_request` 表；並發用 `version` 欄（V16、V17，代行寫入比對 `expectedVersion` 回 409【量】）。
- 選項：
  - A：公開瀏覽／搜尋、買家、賣家、後台、代行（D9）各一支 service；寫入用 `request_id` 冪等，**取代 `idempotency_request` 表**；並發用 etag（ADR `2026-09-25-optimistic-concurrency-etag`，規範 `:127`【量】），取代 `version`／`expectedVersion`。失敗條件：無（SSOT 已規定）。（建議）
  - B：一支 service 涵蓋多角色。失敗條件：權限宣告混雜、違反規範。
- 反悔代價：上線前低。proto 草稿在 B4-3 開工前交審。

### D5 沒有 client 呼叫的端點（擋 B4-1）
- 基線：契約只為有消費者的用途存在（先有用例再加方法，Google API 設計指南【文件】）。
- 清單：`client-inventory.md` §6.5【量】（認證 11 條、related、rating、`/public/products/{id}/skus`、產地與城市聚合、`DELETE /admin/products/cache`、`batch-change-category`、admin 分類 create／delete／restore／`{id}/products`、`/app/hashtags`；`GetStock`；order 的 CD8 死方法）。
- 選項：A：不改寫，隨 Java 刪（建議）；B：照搬成 Connect，失敗條件：為沒有消費者的端點寫契約與測試。
- A 的失敗條件：漏了間接呼叫→ B4-1 特性測試與四端 rg 會抓到。反悔代價：低。

### D6 ID 方案（擋 B4-2；修 CD10）
- 基線：一種 ID 方案、時間有序、對外不透明（UUIDv7，RFC 9562【文件】）；單體現行即 UUIDv7【量】。
- 本 repo SSOT：ADR `2026-09-25-row-identity-by-data-kind` D1「依資料性質決定識別」：被以 ID 指稱的實體才有 ID，值的集合沒有。
- 選項：A：商品、SKU、分類 UUIDv7（建議）；B：沿用 TSID＋BIGSERIAL，失敗條件：同聚合兩種方案、與單體其他模組不一致。
- A 的內容：商品、SKU、分類 UUIDv7；hashtag 不是實體（D14）；content banner 的商品動作同時換成 UUID。反悔代價：上線前低。

### D7 商品限制措施（擋 B4-5）
- 內容已決（主計畫 B4 段，Will 2026-10-18）。基線：平台下架是獨立於賣家狀態的治理層、可申訴（蝦皮「違規下架」與賣家「下架」分開【文件】），與已決內容一致。
- 建議：B4-5 子步驟；商品可見性＝賣家上架狀態（D16）∧ 無有效措施 ∧ 店家可曝光；措施不寫進商品狀態欄，讀 seller 的措施（擁有者是 seller）。Java 的 `seller_deleted_at`（V7）與停權快照表（V10 建、後已 DROP）都是「把別人的狀態複製進商品」的做法，不帶。反悔代價：低。

### D8 分類（擋 B4-2；修 CD11；取代 v1 的 D8、D13）
- 基線：**平台擁有固定分類法**，賣家只選葉節點（Shopify Standard Product Taxonomy、Google product taxonomy、蝦皮類目【文件】）；分類變更是營運發佈的資料版本，不是即時 CRUD；店家自己的分組是另一個概念。
- 現有機制：`category` 自參照＋軟刪、CodeMeta 狀態、種子只有「蔬菜」「水果」（V1:271【量】，食品電商時期遺留）；admin 有狀態變更頁（CD11 推定 404）。
- 選項：
  - A：分類樹放 catalog，以 migration 的版本化種子資料定義（通用電商的精簡分類，參考 Google product taxonomy 頂層）；admin 不做即時 CRUD，刪 category-audit 頁；商品只能掛葉節點。失敗條件：營運要經常改分類→再做管理畫面。（建議）
  - B：照搬 CRUD＋審核頁。失敗條件：沒有營運人員，為沒人用的流程寫契約與測試。
- 推翻的事實：營運需要不經部署改分類。反悔代價：低。
- 要你決定的產品內容：初版分類清單（我起草交你看）。

### D9 代行商品路由（擋 B4-5d、B4-6）
- 本 repo SSOT：`*DelegatedService` 由**擁有資源的模組**擁有，共用 interceptor 在 `internal/httpapi/delegation`，目標店家只來自 session（API 設計規範 `:30`【量】，A7-D25）。
- 修正 v1：v1 建議「在 delegation 模組實作」，與 SSOT 不符；應為 catalog 的 `CatalogDelegatedService`。
- 選項：
  - A：B4 實作 `CatalogDelegatedService`（7 條），刪 gateway 的代行商品 handler；A10 只剩訂單。失敗條件：A10 改了代行憑證形狀→社群代行一樣要改，不是 B4 特有。（建議）
  - B：B4-6 等 A10。失敗條件：Java product 多活一段，B4 無法收尾。
  - C：gateway 代行 handler 改打單體轉接。失敗條件：寫完 A10 就丟。
- 反悔代價：低（代行 service 是獨立的一支）。

### D10 catalog 與 fulfillment 的邊界（擋 B4-2；需和 B3 對齊）
- 基線：商品只記「能用哪些配送方式」與實體屬性（重量、尺寸）；運費規則、免運門檻、誰付運費、試算屬於物流設定（Shopify shipping profiles、蝦皮物流渠道設定【文件】）。
- 現有機制：Java SKU 存 `shipping_method` JSONB（含 `temperature`、`freeThreshold`、`feePayer`），product-service 自己算運費（`CalculateShippingOptions`，CD6）【量】。運費規則放在 SKU 是擁有者放錯。
- 選項：
  - A：catalog 存「啟用的配送方式代碼集合」（值集合）；運費規則與試算歸 fulfillment（B3）；catalog 驗證代碼經 fulfillment 的 API（依賴表已允許）。失敗條件：B3 的 API 未進單體→ B4-2 先定 port，B4-6 前接上。（建議）
  - B：catalog 自己存運費規則。失敗條件：運費規則兩處擁有。
- 推翻的事實：B3 計畫的設計不同。反悔代價：低（配送代碼集合與物流規則分開，搬動只是改擁有者）。

### D11 `shipping_ambient.go`（擋 B4-6；與 B3 協調）
- 基線：溫層已裁切，值域裡就不該有冷藏、冷凍——**從來源刪值**，不在 gateway 過濾。
- 呼叫點 7 個：product 4、shipping-service 3（`client-inventory.md` §1b【量】）。
- 選項：
  - A：B4 刪整檔；shipping-service 的非常溫值在來源刪（若 B3 已先刪 Java shipping，則隨 B3 消失）；動 shipping 部分前與 B3 session 確認沒有同檔未合併變更。失敗條件：B3 同時改同檔→合併衝突，靠先對齊避免。（建議）
  - B：只刪 product 的 4 個呼叫點，shipping 的 3 個給 B3。失敗條件：違反使用者指定的歸屬，檔案殘留到 B3。
- 反悔代價：低（刪的是過濾，值域來源已無溫層）。

### D12 ai-service 的 product 呼叫（擋 B4-1、B4-4、B4-6）
- ai-service 呼叫 ProductService 3 支（`GetProduct`、`GetProductList`、`GetStock`，`backend/services/ai-service/internal/client/product/adapter.go`）與 CategoryService 1 支（`GetCategoryTree`，`client/category/adapter.go`）【量】。
- 選項：A：B4-6 等 A9 刪 ai-service；A9 的單體模組若先完成，經 `legacy/productservice` port，B4-4 一起改接 catalog。失敗條件：A9 拖延→改 B。（建議）B：`catalogserver` 加這 4 支。失敗條件：寫完就丟。
- 反悔代價：低（若 A9 拖延，再補 B 的 4 支）。

### D14 hashtag（擋 B4-2；新題）
- 基線：tag 是商品上的值集合（Shopify product `tags`【文件】）；熱門、自動完成由查詢聚合。
- 本 repo SSOT：social 貼文 hashtag 已依 ADR `2026-09-25-row-identity-by-data-kind` D1 做成值集合（`social.post_hashtags`，沒有實體表，`backend/internal/modules/social/db/schema.sql:212-226`【量】）。
- 現有機制：Java 有 `hashtag` 實體表＋`product_hashtag` 關聯（V1:226、V1:252【量】）。
- 選項：
  - A：`catalog.product_hashtags(product_id, name, display_name, position)`，正規化規則與 social 共用一份（抽到 `platform`）；自動完成與熱門由查詢聚合（trigram 索引）。（建議）
  - B：照 Java 實體表。失敗條件：違反已接受 ADR；兩套正規化。
- 反悔代價：上線前低。
- 要你確認的產品問題：商品與貼文的熱門 hashtag、自動完成要不要合併成一個「平台 hashtag」查詢。

### D15 搜尋（擋 B4-3；新題）
- 基線：百萬列以下用資料庫內建（GitLab 以 `pg_trgm` 做 issue 與專案名稱搜尋、Discourse 用 Postgres 全文搜尋【文件】）；量測觸發才上 Elasticsearch／Meilisearch／Typesense（GitLab 的進階搜尋就是在量變大後才加 Elasticsearch【文件】）。中文沒有空白斷詞，Postgres 內建 `tsvector` 不切中文詞；`pg_trgm` 對一兩個字的查詢用不到索引【文件】。
- 現有機制：Java `searchable_text ILIKE '%t%'`（`ProductRepository.java:73`、`:114`【量】）；A7 social 做過 trigram spike（`2026-10-18-a7-social/step-1a-perf/`【量】）。
- 選項：
  - A：`searchable_text`＋`pg_trgm` GIN，排序用相似度＋銷量；一兩字查詢走掃描（小量可接受）。失敗條件：商品量大到短查詢變慢→換搜尋引擎（觸發：p95 超標）。（建議）
  - B：現在上 Meilisearch。失敗條件：多一個服務要部署、同步、監控，沒有量支撐。
- 推翻的事實：A7 spike 數據顯示 trigram 在預期資料量已不夠。反悔代價：低（搜尋在模組 API 後面，換實作不動契約）。

### D16 商品生命週期（擋 B4-2；新題）
- 基線：Shopify 商品狀態 draft／active／archived，賣家自己上下架；蝦皮、momo 的賣家上架不經人工預審，違規由事後檢舉與治理下架處理（D7）【文件】；刪除是封存（訂單仍引用）。
- 現有機制：status 4001 草稿、4002 審核中（**預設值**）、4003 未啟用、4004 啟用，加 `deleted_at`、`seller_deleted_at`、`first_published_at`（V1、V7、V15【量】）。預審是公司時期食品電商的營運流程。
- 選項：
  - A：`draft`／`active`／`archived`（proto enum＋Go 型別＋CHECK，取代 CodeMeta）；`archived` 是賣家下架、可還原成 `active`（Shopify 同義，對應 Java 4003）；刪除是另一個動作：設 `deleted_at`，從賣家清單消失、不可還原，但列保留（訂單、評價仍引用）。不預審，治理靠 D7。`first_published_at` 保留（「新上架」排序）。失敗條件：法規要求特定類目上架前審查→在該類目加預審狀態。（建議）
  - B：保留預審（4002）。失敗條件：沒有營運人員審，商品永遠卡在審核中。
- 推翻的事實：有需要預審的類目或法規。反悔代價：低（加一個狀態）。

### D17 圖片與描述（擋 B4-2；新題）
- 基線：商品圖片是有序的資產引用清單（Shopify product media，含排序與替代文字）；描述是結構化 rich text（Shopify `descriptionHtml`）或純文字＋圖片區塊（蝦皮）【文件】。
- 本 repo SSOT（圖片部分已定）：ADR `2026-10-03-media-references-owner-side`：引用只存在擁有者的表；移除時在擁有者的 transaction 內呼叫 `MarkDeleted`，R2 檔案由 media 的 job 刪（B1-D9 修訂，`:86-87`【量】）。ADR `2026-11-22-media-upload-channels`：public 圖走預簽 PUT→`CompleteUpload`，擁有者在自己的 transaction 內 `Confirm`【量】。ADR `2026-09-25-api-image-uri-template`：DB 只存 key，API 回 `Image`，新圖以 `*_staged_token` 寫入【量】。
- 現有機制：`product.gallery` JSONB、`description` JSONB blocks 內含圖片（V1【量】）。
- 圖片（照 ADR，不需要拍）：`catalog.product_images(product_id, position, media_key, alt)`；儲存時 `Confirm`、移除時同 transaction `MarkDeleted`。
- 描述格式（repo 沒有現成的 rich text 格式：content 只有 `notice_body TEXT`，`api/` 沒有 RichText【量，第 2 輪審查】）：
  - A：純文字（保留換行）＋獨立的「描述圖片」有序清單（同 `product_images` 加 `kind`）。蝦皮做法。失敗條件：賣家要圖文穿插或粗體、清單→改 B。（建議：四端都只要顯示文字與圖片，沒有編輯器成本）
  - B：結構化區塊（proto `oneof` 段落／圖片／清單），四端各寫 renderer。失敗條件：四端各一份 renderer 與賣家端編輯器，成本高。
  - C：HTML。失敗條件：要做 sanitize 與 XSS 防護，App 端要 WebView 或 HTML renderer。
- 推翻的事實：賣家端已有圖文穿插的編輯器需求。反悔代價：A→B 中（資料可轉換：文字變段落、圖片變區塊）。

### D18 規格與選項模型（擋 B4-2；新題）
- 基線：Shopify 最多 3 個選項軸（例如顏色、尺寸）× 值，組合出 variants；蝦皮最多 2 層規格；SKU 是選項值的組合，組合唯一【文件】。
- 現有機制：SKU 只有自由文字 `sku_name`＋`unit`（V1【量】），沒有選項軸；食品電商的「500g／1kg」用自由文字夠用。
- 選項：
  - A：商品定義 0–2 個選項軸（名稱＋有序值），SKU 是值的組合，`UNIQUE(product_id, 組合)`；沒有選項的商品只有一個預設 SKU。失敗條件：client 畫面要改成選規格的介面（UI 改動要你同意）。（建議，通用電商的衣服、3C 需要）
  - B：照 Java 自由文字。失敗條件：買家無法依規格篩選、賣家可建重複規格、client 無法畫規格選擇器。
- 推翻的事實：商品範圍只限單一規格商品。反悔代價：**上線後高**（資料模型與四端 UI），上線前中。

### D19 價格模型（擋 B4-2；新題）
- 基線：價格在 SKU，整數最小貨幣單位；原價＋售價（Shopify `compareAtPrice`＋`price`）；限時特價是促銷系統的事（Shopify discounts、蝦皮限時特賣是活動），不放在商品欄位【文件】；列表的價格區間反正規化在商品上，同 transaction 由 SKU 變更維護。
- 現有機制：`price`、`sale_price`（CHECK `0 < sale_price < price`，無時段、無幣別），商品上 6 個 min／max 反正規化欄（V1【量】）。
- 選項：
  - A：SKU 有 `price`（售價）與可選 `compare_at_price`（原價，CHECK 大於售價）；新台幣元整數，不存幣別（單一幣別，`backend/AGENTS.md:65` 規定金額是新台幣元整數【量】）；商品只留 `min_price`、`max_price`，在 SKU 寫入的同 transaction 重算；限時特價延後到促銷（觸發：要做限時特賣）。失敗條件：要多幣別→加幣別欄。（建議）
  - B：照 Java 6 欄。失敗條件：多 4 欄反正規化，維護點多。
- 反悔代價：低。

### D20 訂單快照（擋 B4-4 的轉接介面；新題）
- 基線：訂單行在下單時凍結商品名稱、規格、單價、圖片 key（Shopify order line item 帶 title、variant_title、price 的快照；商品之後改了不影響訂單）【文件】；不做商品版本化。
- 現有機制：order 經 gRPC 讀商品後自己存；商品上有分類名稱快照 `cat_l1..l3_name`（V1【量】）。
- 選項：
  - A：catalog 提供「結帳用的商品快照」API（名稱、規格名、單價、主圖 key、店家），訂單自己存快照，`catalogserver` 的結帳批次讀就是它的轉接；商品上的分類名稱快照刪除（分類是平台資料，讀時 join）。（建議）
  - B：商品版本化，訂單存版本 ID。失敗條件：每次編輯產生版本列，為了訂單顯示保留全部歷史。
- 反悔代價：低。

## 既有缺陷的建議處置

| 缺陷 | 建議處置 |
|---|---|
| CD1 扣庫存雙扣 | 修（D2），回歸測試：同一冪等鍵併發兩次只扣一次 |
| CD2 歸還量信任呼叫方 | 修（D2），回歸測試：超過原扣減的歸還被拒 |
| CD3 收藏數漂移 | 修（D3） |
| CD4 評分亂序 | 修（D3），傳絕對值 |
| CD5 去重與帳本無清理 | 去重表隨 D3 消失；帳本保留（量小，是庫存稽核依據），接受 |
| CD6 運費試算郵遞區號 | 歸 B3（D10） |
| CD7 認證事件無消費者 | 隨裁切刪除 |
| CD8 order 死方法 | `catalogserver` 不實作，B8 刪 |
| CD9 `GetProduct` 擁有者判斷 | 修（D4） |
| CD10 ID 混用 | 修（D6） |
| CD11 admin 分類審核 404 | 隨 D8 刪頁 |

## 終態與觸發

- B4 終態：catalog 在單體，Java 只剩 cart、order 經轉接呼叫。
- B6、B8 終態：轉接刪除，結帳、扣庫存、鎖券同一 transaction（觸發：B6、B8 完成）。
- 延後：多倉／保留模型（觸發：多倉或預購）；搜尋引擎（觸發：搜尋 p95 超標）；分類管理畫面（觸發：營運需要不經部署改分類）；限時特價（觸發：要做限時特賣，歸促銷）；店家自訂商品分組（觸發：店面要分組畫面）。補貨通知已存在（product 發 `product.restock` 給 buyer，`interface-inventory.md` §3【量】），B4-3 改成 in-process 事件，不延後。

## 拍板順序

D1、D2、D4、D5、D6、D7、D9、D12、D14、D15、D16、D17、D19、D20 可一次拍；D18 會帶動四端選規格的 UI，要你同意 UI 改動；D8 要附初版分類清單；D10、D11 先和 B3 session 對齊。
