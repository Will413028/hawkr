# 改成一般電商：生鮮專屬功能直接刪除，不改成「不分品類」

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-10-27（Will 決定） |
| 影響範圍 | pricebook-service、ai-service（採購的價格依據）、seller 模組（食品業者登錄、FDA 轉報）、content 模組（食譜）、Java shipping 的 Uber、website、admin、兩支 App |
| 執行計畫 | `2026-10-27-ecommerce-scope-cut`（計畫已刪除，見文末修訂） |
| 路徑基準 | `backend/` |

## 脈絡

作品集的示範品類改成一般電商（2026-10-18 已定）。當時的方向是「保留引擎、生鮮降成可換的品類設定」：食品業者登錄改成依品類的 `Permit`、食安轉報改成「主管機關轉報」、溫層改成「出貨條件」、pricebook 改成「價格參考來源 adapter」。這個方向沒有寫成 ADR，記在個人筆記；本 ADR 取代它。

約束：
- 外部：系統從未上線，沒有正式資料與使用者（根目錄 `AGENTS.md`「上線前不受相容性約束」）。
- 外部：食安法的轉報義務（1919、FDA、4 小時期限）只在平台賣食品時成立；改成一般電商後不成立。
- 繼承：遷移計畫的排程（A4 pricebook、A9 agent、階段 B 的 Java 改寫）。是我們自己的分段，不是約束；它只決定「刪舊服務的程式」比「搬過去再刪」便宜。
- 繼承：2026-10-18 的「不分品類」方向。是我們自己的決定，可推翻。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一般綜合電商的功能面（Shopee、momo、Etsy、Amazon marketplace）：目錄、購物車、結帳、金流、超商／宅配物流、售後、評價、優惠券、聊天、通知、賣家入駐與 KYC、檢舉與處分、電子發票。受管制品類的資格審核（Amazon「gated categories」，<https://sellercentral.amazon.com/help/hub/reference/external/G200333160>）只在引入受管制品類時才有 | 生鮮專屬功能全部不在其中 | 採用：沒有對應的直接刪 |
| 通用化（2026-10-18 方向） | 每個生鮮功能都要重新設計成抽象層（Permit、主管機關轉報、價格參考 adapter），抽象層沒有第二個實作、沒有使用者 | 否決：為沒有需求的品類預先抽象 |
| 保留不動、只換文案 | 作品集裡留著農產行情、食譜、冷鏈；搬遷仍要搬 pricebook | 否決：和一般電商定位矛盾 |

## 決策

刪除，不通用化。逐項：

- **D1 刪除**：pricebook 行情預測整個服務與所有消費端（賣家中心的行情雷達、價格地圖、賣家上架的作物品項選擇、助理的兩個季節工具）；食譜；website 的產地地圖瀏覽；首頁季節與水果裝飾；企業福利頁。
- **D2 保留採購助理（RFQ 詢價、報價、站外供應商 portal）**，但拿掉它對 pricebook 的依賴：品項只解析到平台分類樹，不再對到作物與行情；報價只看供應商報的價，不再和市場行情比較（拿掉行情依據與「價格冷熱」判斷）。理由：它是作品集主要的 LLM agent 成果；保留 pricebook 只為了它，等於為一個消費者留下 3 萬行的農產行情服務。
- **D3 刪除**食品業者登錄（`seller/internal/foodreg`）與 FDA 轉報（`case_referrals`）。通報、案件、處分、主管機關來函保存保留（一般電商也有的平台治理）。
- **D4 刪除** Uber 即時配送。client 現在刪；Java shipping 的 Uber 程式照 D5 的時機刪。
- **D5 散在欄位裡的生鮮屬性**（配送溫層、產銷履歷與有機標章、商品認證）：client 現在拿掉介面；後端 Java 不動，在階段 B 改寫 catalog、fulfillment、cart 時不帶這些欄位，屆時契約上一起消失。Java 仍要求的欄位（例如配送方式的溫層），client 在過渡期送固定的常溫值。

## 後果

- A4 取消；A9 縮小（採購少了價格依據）；seller 模組少了食品業者登錄與 FDA 轉報，相關 ADR（`2026-10-18-seller-governance-consolidated`、`2026-10-19-seller-kyc-payuni-consolidated`、`2026-10-20-admin-review-queue`）在執行時加修訂段；只服務已刪功能的 ADR（`2026-09-14-pricebook-freshness-describes-age-not-usability`、`2026-09-18-deal-temperature-outcome-definition`、Uber 的幾份）標 superseded。
- 採購的品項解析會變粗：同一個分類下的不同作物不再分開（例如「蘋果」只到分類，不分品種）；報價沒有市場行情可以對照。
- D5 的過渡期間，client 送的常溫值是假資料；它在階段 B 改寫 catalog（B4）時隨欄位一起刪除，不留到上線。
- 重新評估的條件：平台要賣受管制品類（食品、保健品、化妝品、醫療器材）時，再依基準做品類資格審核；採購需要價格參考時，另選一般商品的價格來源，不恢復農產行情。

## 取代與修訂的 ADR

每一份在對方檔案裡也有指回本 ADR 的標記（`rg -n "general-ecommerce-scope-cut" docs/adr` 可核對）。

| ADR | 關係 | 依據 |
|---|---|---|
| `2026-09-14-pricebook-freshness-describes-age-not-usability` | superseded | D1 |
| `2026-09-18-deal-temperature-outcome-definition` | superseded | D2 |
| `2026-09-13-procurement-category-fallback` | 部分取代（Pricebook 搜尋、行情映射與行情驗證） | D1、D2 |
| `2026-10-18-seller-governance-consolidated` | 修訂段（FDA 轉報刪除；pricebook 刪除） | D3 |
| `2026-10-19-seller-kyc-payuni-consolidated` | 修訂段（食品業者登錄刪除；pricebook 刪除） | D3 |
| `2026-10-20-admin-review-queue` | 修訂段（刪除兩種待辦） | D3 |
| `2026-10-02-seller-kyc-payuni-in-monolith`、`2026-09-24-go-modular-monolith`、`2026-09-18-seller-capability-authorization`、`2026-09-29-seller-suspension-and-business-identity-in-monolith`、`2026-10-03-vendor-portal-invite-scoped-access`、`2026-09-13-category-alias-offline-generation` | 修訂段（pricebook 刪除） | D1 |
| `2026-09-06-fulfillment-docs-split-into-six`、`2026-09-11-logistics-finance-domain-ownership`、`2026-09-11-logistics-amount-semantics-separation`、`2026-09-11-channel-fee-grossup-in-adapter`、`2026-09-12-inbound-payload-raw-bytes`、`2026-09-12-return-leg-classifier-canonical`、`2026-09-11-uber-logistics-fee-charge-trigger`、`2026-09-14-uber-create-idempotency-precommit-key` | 註記（設計書 66 的 Uber 決定刪除） | D4 |
| `2026-09-11-uber-commerce-model-seller-direct`、`2026-09-11-uber-logistics-fee-charge-trigger`、`2026-09-14-uber-create-idempotency-precommit-key`、`2026-09-06-uber-webhook-slice-in-payuni-webhook-service` | 待標 superseded：Java 的 Uber 程式還在，等主計畫 B3 刪除時一起標（`docs/plans/2026-09-24-go-modular-monolith.md` B3） | D4、D5 |

## 修訂：計畫的後續決定併入、計畫已刪除（2026-11-15，原決定日期與決定者見各項）

執行計畫 `2026-10-27-ecommerce-scope-cut` 與附件 `inventory.sh` 依「完成的計畫不留在 repo」的決定在 2026-11-15 刪除，git 歷史是紀錄。D1–D5 已在本檔；計畫的 D6–D9 在計畫裡才有，以下照計畫轉記（計畫未記的不補）。D1–D5 的結論不變；D9 改的是 D5 過渡期做法的承接位置（見該項）。延後項的位置與觸發條件已在 `2026-09-24-go-modular-monolith` 計畫的 A4、A9、B1、B3、B4、B6 與階段 C；ADR 取代與修訂的對照見上表。

- **D6 步驟 5 不量改動前的基線**（Will 2026-10-28）：採購品項解析改用分類樹召回後，不量改動前的解析品質；離線評測用本機種子分類樹（33 節點），golden set 依新行為重寫。理由：完整分類樹只在舊公司資料庫，本機與 repo 沒有（`category_tree_snapshot.json` 沒有父子關係），pricebook 也沒有本機資料，量不到基線；改成一般電商後生鮮大分類樹沒有保留價值。計畫的停止條件：離線評測上解析不出任何分類的查詢超過全部的三分之一，表示分類樹不夠當品項來源，回報 Will 再決定。完成時離線評測 precision@1 為 0.939（31/33）。未選方案：計畫未記。
- **D7 下游快照與核准表的行情欄位一起刪**（Will 2026-10-28，2026-10-29 修訂）：確認快照與發布核准表的行情欄位與發布阻擋一起刪，是 D2「報價只看供應商報的價」的直接結果。`quality_knowledge_state` 原決定保留；設計審查發現所有寫入點都寫死 `BUYER_QUALITY_UNKNOWN`、拿掉 pricebook 後沒有程式會產生第二個值，Will 在 2026-10-29 改成一起刪（migration `20261028164119_drop_quality_knowledge_state`）。
- **D8 買家端結帳原樣回傳 `deliveryTemp`**（Will 2026-10-28）：步驟 7 的買家端結帳不送固定常溫值，原樣回傳伺服器配送選項的 `deliveryTemp`。理由：Java 以它查運費、材積上限、免運與運費負擔，且不檢查它是否屬於商品自己的配送方式，寫死常溫會讓冷藏、冷凍商品算錯。Java 組好的配送選項名稱含溫層字樣（例如「黑貓宅急便（常溫）」），client 不改寫，隨主計畫 B4 處理；購物車與結帳契約裡的溫層欄位隨 B6 處理。未選：D5 原寫的固定常溫值（會算錯運費）。
- **D9 賣家端的溫層舊要求由 gateway 雙向承接**（Will 2026-10-29）：賣家端請求的 `temperature`、超商 `goodsType` 由 gateway 一律補常溫（`backend/gateway/internal/handler/shipping_ambient.go`）；賣家端回應（運費試算選項、物流開通與配送方式清單）只回常溫列且不帶溫層欄位；匯入預覽只拿掉溫層欄位、不過濾列。CSV 匯入的溫層欄位由 Java 解析（`CsvParseService.java:478-499`），賣家仍可能存進冷藏或冷凍，隨 B4 決定。這取代 D5 寫的「client 在過渡期送固定的常溫值」：改由 gateway 承接，client 不再有常溫常數。理由：gateway 是我們擁有的轉接層，client 契約一開始就是最終形狀，B4 只刪 gateway 一處，App 不必發版。買家結帳仍照 D8。未選：每個 client 一個常溫常數（計畫原步驟 7 的做法，website 與 seller-app 的常數檔已刪）。

計畫的規模、步驟、各步驗收與審查紀錄不再保存。

## 修訂：商品匯入（批次匯入精靈）刪除，不改寫（2026-11-15，Will 決定）

範圍再縮一項：商品批次匯入（CSV 匯入精靈，Java product-import-service，主計畫 B5 productimport）整個刪除，不改寫成 Go。被刪的規模：7 張表、18 個 gateway 端點、跨 product、media、shipping 的 RPC 與一組 MQ 拓撲。決定時沒有另記理由。上線前沒有相容性約束，直接刪除，不留相容層。

- 刪除：`services/java/product-import-service`；gateway 的 `/api/v1/app/product_import_jobs` 全部 18 個端點與 OpenAPI operation；product-service 與 media-service 只服務匯入的 RPC（`ValidateLeafCategoryInternal`、`CreateProductFromImportInternal`、`BatchCheckSellerProductNames`、`ValidateProductDraft`、`GetProductsBriefInternal`、`ImportDownloadAsset`、`FinalizeImportAsset`、`BindStagedAsset`）、media 的 `import_asset` 表與下載 worker、`product.import_job_id`／`import_item_id` 與 `media_asset.import_store_id` 欄位；匯入專用的 MQ exchange 與 queue；CodeMeta 的匯入值域（`ImportJobStatus` 等）、錯誤碼 1100–1199、`AssetReferenceType` 的 7116、7117 與 `ProductCertificationSource` 的 `PRODUCT_IMPORT`。
- D9 中與匯入有關的部分作廢：匯入預覽、CSV 匯入的溫層欄位都不再存在；B4 不必再決定匯入端的溫層處理。
- 主計畫：B5 標為已移除；模組表的 `productimport` 列取消（見 `2026-09-24-go-modular-monolith` 的對應修訂）。

物流商已依 `2026-11-15-fulfillment-seller-managed-shipping.md` 退場（Java shipping、`payuni-webhook-service` 與 Uber 程式於 B3 刪除）；本檔提到的物流商只作歷史脈絡。
