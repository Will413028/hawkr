# catalog 模組從零設計：庫存、商品模型、分類與邊界

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-24（Will 決定，全部照建議） |
| 影響範圍 | 單體 `catalog`（新）、`seller`、`buyer`、`review`、`content`、`social`、`discovery`、`procurement`；`internal/legacy`；gateway 商品路由；website、admin、buyer-app、seller-app 的商品畫面 |
| 執行計畫 | `backend/docs/plans/2026-11-24-b4-catalog.md`；選項全文與業界來源在 `backend/docs/plans/2026-11-24-b4-catalog/decision-brief.md`（D1–D20） |

## 脈絡

Java product-service 改寫成單體的 catalog 模組。每個沿用的機制都先問「從零設計會怎麼做」。

- 外部：無正式使用者與資料；單一幣別新台幣（`backend/AGENTS.md:65`）。
- 繼承：Java cart、order 在 B6、B8 才改寫，B4 期間仍經 gRPC／MQ 呼叫商品；仍成立，因為 B8 的前置含 B4 本身、`cart → catalog`（`internal/arch/modules.go:24`），無法先做。
- 繼承：範圍裁切（溫層、產銷履歷、有機標章、商品認證，ADR `2026-10-27-general-ecommerce-scope-cut`）；仍成立，是產品決定。
- 繼承：資料識別依性質（ADR `2026-09-25-row-identity-by-data-kind` D1）、媒體引用在擁有者（`2026-10-03-media-references-owner-side`）、etag 樂觀鎖（`2026-09-25-optimistic-concurrency-etag`）、Connect 依角色分 service 與代行 service 由資源擁有者擁有（〈API 設計規範（Connect）〉）；仍成立，都是已接受的全系統規則。
- 不再成立而捨棄：Java 的資料模型、±1 MQ 計數、Redis 快取、預審流程（公司時期食品電商的營運流程）、`idempotency_request` 表、TSID＋BIGSERIAL 混用。

## 決定

| # | 決定 | 否決的方案與理由 |
|---|---|---|
| D1 | B6／B8 前，Java cart、order 經 `legacy/catalogserver`（5 支 RPC）與 `legacy/catalogconsumer`（`order.cancelled`、`order.sales.accumulated`）呼叫模組 API；冪等只由模組判定；B6、B8 刪除 | B4＋B6＋B8 合併：一步跨兩萬行以上，且前置 B2、B3 未完成 |
| D2 | 庫存在 SKU；每次變動寫 `catalog.stock_movements`（呼叫方冪等鍵 UNIQUE），同 transaction 先插帳本再條件扣減；歸還只帶冪等鍵、累計不超過原扣減；下單即扣、逾時歸還由 ordering 觸發。基準：Shopify inventory adjustments、Square inventory changes、蝦皮下單扣庫存 | 保留模型（on_hand／committed／available，Shopify、Amazon）：單倉自寄沒有可靠的出貨扣減點，多一個狀態沒有使用者價值。照 Java 先查再扣：併發雙扣（CD1）、歸還信任呼叫方（CD2） |
| D3 | 銷量、收藏數、評分反正規化在商品上，由事實擁有者在同 transaction 更新：buyer 收藏經 in-process 事件，事件帶 buyer 重算後的絕對值（計數是快取，真值在擁有者，DDIA 第 11 章的衍生資料做法）；review 呼叫 `SetRating` 傳重算後的絕對值；銷量 B8 前由 `catalogconsumer` 以 `order_no` 冪等 | 讀取時跨 schema 聚合：無法建排序索引。±1 MQ＋去重表：亂序與下限截斷造成漂移（CD3–CD5） |
| D4 | Connect 依角色分公開、買家、賣家、後台、代行 service；店家由憑證推導；寫入以 `request_id` 冪等；並發用 etag | 單一 service：違反 API 規範。沿用 `store_id` 參數與 `version`：CD9 與兩套並發機制 |
| D5 | 四端都沒有呼叫的端點與 RPC（`client-inventory.md` §6.5）不改寫，隨 Java 刪 | 照搬：為沒有消費者的端點寫契約與測試 |
| D6 | 商品、SKU、分類用 UUIDv7；content banner 的商品動作同時改 UUID | 沿用 TSID＋BIGSERIAL：同聚合兩種方案（CD10） |
| D7 | 商品可見性＝賣家狀態 ∧ 無有效 seller 措施 ∧ 店家可曝光（`seller.pub_exposable_stores`）；措施不寫進商品；`seller_deleted_at` 與停權快照不帶 | 把他模組狀態複製進商品欄：雙寫與漂移 |
| D8 | 平台擁有固定分類法：取 Shopify Standard Product Taxonomy 子集（頂層 8–12、兩層），以版本化種子資料定義；內部 UUIDv7，另存 `shopify_taxonomy_id` 字串對照；名稱 `LocalizedText`（多語 ADR）；商品掛葉節點；不做即時 CRUD 與審核頁。基準：Shopify、Google product taxonomy | 照搬 CRUD＋審核：沒有營運人員，且現有審核頁送錯路徑（CD11）。自編（參考蝦皮類目）：沒有公開穩定 ID、要自己維護版本 |
| D9 | 代行商品路由在 B4 以 catalog 的 `CatalogDelegatedService` 實作，A10 只剩訂單 | 等 A10：B4 無法收尾。gateway 改打轉接：A10 就丟 |
| D10 | 商品只存啟用的配送方式代碼集合；運費規則、免運門檻、試算歸 fulfillment。基準：Shopify shipping profiles | catalog 存運費規則：兩處擁有 |
| D11 | 溫層從值域來源刪除，`shipping_ambient.go` 整檔由 B4 刪；動 shipping 部分前與 B3 對齊 | gateway 繼續過濾：補丁留在錯的層 |
| D12 | ai-service 的 4 支呼叫不寫轉接，B4 刪 Java 前等 A9 刪 ai-service | `catalogserver` 加 4 支：A9 完成就丟 |
| D14 | hashtag 是商品上的值集合（`catalog.product_hashtags`），以明確欄位 `hashtag_inputs` 輸入、不從描述抽；正規化與 social 共用一份；建議與熱門由 catalog、social 各自聚合，跨兩者的頁面在呈現層組合。基準：Shopify product tags、蝦皮商品 hashtag 導向商品搜尋 | 實體表：違反 row-identity ADR。合併成平台排行：要跨模組讀取模型，沒有量測觸發（單體規則 2）。從描述抽：兩套輸入模型與兩份抽取規則 |
| D15 | 搜尋用 `pg_trgm` GIN；量測觸發才換搜尋引擎。基準：GitLab 以 `pg_trgm` 搜尋、量大才加 Elasticsearch | `tsvector`：不切中文。現在上 Meilisearch：多一個服務，沒有量支撐 |
| D16 | 狀態 draft／active／archived（archived 可還原）；刪除另設 `deleted_at`、列保留；不預審，治理靠 D7。基準：Shopify product status、蝦皮不預審 | 保留預審：沒人審，商品卡在審核中 |
| D17 | 圖片 `catalog.product_images`（有序、alt、key），儲存時 `Confirm`、移除時同 transaction `MarkDeleted`；描述是結構化區塊（proto `oneof`：段落、圖片、清單、表格），伺服器驗證數量與長度，描述圖片同樣 `Confirm`／`MarkDeleted`。基準：Shopify rich text JSON、Portable Text、Contentful Rich Text | 純文字＋描述圖片（蝦皮）：刪掉 seller-app、buyer-app、website 已有的清單與表格能力。HTML：sanitize 與 App 端渲染成本 |
| D18 | 0–2 個選項軸，SKU 是選項值組合且唯一；無選項商品只有預設 SKU；四端改選規格 UI（Will 同意）。基準：Shopify options×variants、蝦皮兩層規格 | 自由文字 SKU 名：無法篩選、可重複、client 畫不出選擇器 |
| D19 | SKU 有售價與可選原價（CHECK 原價大於售價），新台幣元整數；商品只留 `min_price`、`max_price`，同 transaction 重算；限時特價屬促銷。基準：Shopify `price`／`compareAtPrice` | Java 6 個區間欄：反正規化維護點多 |
| D20 | catalog 提供結帳用商品快照（名稱、規格名、單價、主圖 key、店家、etag），訂單自存，以 etag 偵測下單時價格已變；商品上的分類名稱快照刪除。基準：Shopify order line item 快照 | 商品版本化：每次編輯留版本，只為訂單顯示 |

D13 併入 D8。D8、D14、D17 依追加簡報 `backend/docs/plans/2026-11-24-b4-catalog/followup-brief.md`（Will 2026-11-24）。

## 理由

取捨集中在三處：D2 選下單即扣，放棄多倉與預購的彈性，換來沒有保留狀態要對帳；D17 選結構化區塊，付出伺服器驗證與型別化的成本，換來多端一致且不用 sanitize（同日修訂：原選純文字，因三端已有區塊編輯與顯示而改）；D18 現在付四端 UI 的成本，因為選項模型上線後幾乎改不動。其餘多為把擁有者放對（D3、D7、D10、D20）或照已接受的全系統規則（D4、D6、D14、D17 圖片）。

## 預期結果

- catalog 的庫存在同一冪等鍵併發下只扣一次；歸還不超過原扣減（B4-2 integration 測試）。
- 計數不再有去重表，B8 後全部同 transaction。
- Java product-service、gateway 商品路由與 `shipping_ambient.go` 刪除（B4-6）。

## 撤銷條件

- 多倉或預購需求 → D2 改保留模型。
- 賣家需要行內樣式或連結 → D17 段落改成 span＋marks。
- 產品要跨貼文與商品的單一熱門排行 → D14 由 discovery 擁有讀取模型。
- 營運要不經部署改分類 → D8 加管理畫面。
- 搜尋 p95 超標 → D15 換搜尋引擎。

## Followup

- D8 初版葉節點清單交 Will（B4-2 開工前）。
- D10、D11 與 B3 session 對齊。
- B6、B8 完成時刪 `catalogserver`、`catalogconsumer`。

## 相關

- 計畫與簡報：`backend/docs/plans/2026-11-24-b4-catalog.md`、`backend/docs/plans/2026-11-24-b4-catalog/decision-brief.md`（commit `371ea99d6`）
- 主計畫：`backend/docs/plans/2026-09-24-go-modular-monolith.md` B4 段
