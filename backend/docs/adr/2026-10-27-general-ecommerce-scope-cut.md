# 改成一般電商：生鮮專屬功能直接刪除，不改成「不分品類」

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-10-27（Will 決定） |
| 影響範圍 | pricebook-service、ai-service（採購的價格依據）、seller 模組（食品業者登錄、FDA 轉報）、content 模組（食譜）、Java shipping 的 Uber、website、admin、兩支 App |
| 執行計畫 | `docs/plans/2026-10-27-ecommerce-scope-cut.md` |

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

每一份在對方檔案裡也有指回本 ADR 的標記（`rg -n "general-ecommerce-scope-cut" backend/docs/adr` 可核對）。

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
