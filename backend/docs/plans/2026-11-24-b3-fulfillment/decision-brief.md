# B3 決策簡報：fulfillment 從零設計（業界基線 vs 現況）

快照 dev `496b048e2`。證據：【量】讀過 source 或跑過指令（出處在三份盤點）；【推】推論；【文件】公開文件的已知做法，沒在本 repo 實測。盤點：`interface-inventory.md`（Java shipping，下稱 I）、`caller-inventory.md`（呼叫端，下稱 C）、`client-inventory.md`（client，下稱 K）。

## 前提題：被改的程式是否排定要重寫、搬走或刪除？

- Java shipping-service（144 檔、22,656 行【量】I §0）：除運費規則與出貨狀態外直接刪（Will 2026-11-23）。保留的兩塊也是**重寫**：運費現況是平台級距（`method × temp × zone × 材積`），不是賣家自訂規則（I §13-1【量】），ADR D2 的「賣家固定規則」在現有程式裡沒有對應；狀態現況 28 值裡 19 個是 PayUni 碼（I §5【量】）。Java 結構全部是 SELF-IMPOSED，不沿用。
- Java order、aftersales、notification、Go invoice-service 在 B3 之後仍活著到 B8、B9 等（C §1.2【量】），它們呼叫 shipping 的 gRPC 與 MQ 是 **REAL** 限制；B7 已用同樣方式處理（`internal/legacy/couponserver`）。
- 設計書 69（`SELLER_MANAGED`）已刪，主計畫與 ADR 仍引用它（C §6【量】）。可重用的只有「L0 手動推進」與「收貨 Gate」兩個概念（C §6.1，`git show` 讀回【量】）。

## 限制帳

| 限制 | 分類 | 來源 |
|---|---|---|
| B3 到 B8／B9 之間 Java order、aftersales 要讀出貨、收出貨事件 | REAL | C §1.1、§1.2 |
| `modules.go`：fulfillment 依賴 buyer、seller、media；catalog、ordering、aftersales、invoice、finance 依賴 fulfillment；cart **不**依賴 fulfillment | 與基線一致，cart 邊是待決 | `backend/internal/arch/modules.go:21-31`【量】 |
| 系統沒上線：不搬資料、不留相容 | 有利條件 | 根 `AGENTS.md` |
| 溫層欄位與 `shipping_ambient.go` 歸 B4／B6 | REAL（並行 session 分工） | Will 交辦 |
| Java 的 28 值狀態碼、級距表、10 個 provider capability、Redis 快取、9 支排程 | SELF-IMPOSED | I |
| 沒有任何 client 有「填物流業者＋追蹤號碼」畫面 | 事實：自寄是新畫面 | K【量】 |

## 業界基線

- **marketplace 出貨**：訂單付款後產生待出貨（fulfillment order）；賣家在處理期限內標記出貨並回填承運商與追蹤號碼；承運商用固定清單加「其他」（Etsy、eBay、Shopee 自行出貨都是清單）【文件】。
- **處理期限**：賣家自訂或平台預設 1–3 個工作日，逾期由平台自動取消並退款（Shopee 備貨天數、Etsy processing time）【文件】。
- **完成**：買家確認收貨，或出貨後 N 天自動完成（Shopee：送達後 7 天；沒有貨態時以出貨日起算的較長期限）【文件】。撥款跟完成走。
- **運費**：Shopify／Etsy 的 shipping profile——賣家層級的固定運費＋免運門檻，商品綁 profile【文件】。結帳以同一個報價函式計算並把結果快照進訂單。
- **一致性**：狀態轉換 CAS＋同交易寫 outbox；外部 webhook 先寫 inbox（UNIQUE 事件 ID）再處理。

## 決定題

依相依排序。每題：選項（業界光譜）→ 建議 → 擋住的步驟。

### D1 過渡期：Java 呼叫端怎麼看到出貨（擋步驟 4、7）
- A：Go fulfillment 擁有資料；`internal/legacy` 提供 Java **實際呼叫且自寄仍需要**的最小 gRPC（`GetShipmentByOrderNo`、`GetShipmentsByOrderNos` 等，逐支列在步驟 4）與 relay 重發 `shipment.*` 事件；PayUni 專屬 RPC 不提供，呼叫端隨刪。和 B7 couponserver 同做法。（建議）
- B：B3 等 B8 一起做——違反 D8 由下而上順序，B8 已 2 萬行。
- C：B3 同時改 Java order／aftersales——改即將被刪的程式。

### D2 運費規則模型與擁有者（擋 3、5；B6 依賴）
- A：fulfillment 擁有**賣家層級 shipping profile**：固定運費（TWD）、可選免運門檻、單一配送方式「賣家自寄」；商品不帶運費欄位。對外一個純報價 `QuoteShipping(seller_id, merchandise_subtotal) → fee, free_shipping_remaining`，ordering（結帳快照）與 cart（預覽）都呼叫它。（建議：作品集夠用、B6 契約最簡）
- B：Etsy 式多 profile、商品綁 profile——多一層選擇與 UI；延後到有需求。
- C：沿用 SKU JSONB 的 `feePayer`／`freeThreshold`——欄位缺金額、規則散在 catalog 與 ordering 三處（I §13-1），否決。
- 附帶：誰付運費（`feePayer`）拿掉——固定運費一律買家付，免運即賣家吸收。

### D3 cart→fulfillment 依賴邊（擋 2、3 的契約；B6）
- A：加邊，cart 直接呼叫 `QuoteShipping` 顯示預覽；結帳仍由 ordering 呼叫同一函式並快照。（建議）
- B：預覽也走 ordering 的 CheckoutValidate——cart 為了顯示運費依賴 ordering，方向反了。

### D4 出貨狀態機（擋 3）
- A：新的字串 enum（proto + Go 型別 + CHECK）：`AWAITING_SHIPMENT → SHIPPED → COMPLETED`；`AWAITING_SHIPMENT → CANCELLED`（逾期或訂單取消）；`SHIPPED → DELIVERY_ISSUE`（買家回報未收到）。不沿用 28 值數字碼。（建議）
- B：沿用 `ShippingStatus` 數字碼的子集——數字碼語意綁 PayUni，否決。

### D5 期限、自動完成與收貨 Gate（擋 3；與 B2 撥款相關）
- A：處理期限＝付款後 3 個工作日（`internal/platform/bizcalendar`，賣家不可自訂）；逾期 → `CANCELLED` 並發事件，由 ordering（過渡期 Java aftersales 現有的取消退款路徑）退款。自動完成＝出貨後 10 個日曆天，買家可提前確認收貨。保留收貨 Gate：撥款只由「買家確認或自動完成」觸發，賣家自稱送達不算。fulfillment 發 `shipment.completed`，撥款觸發的接線歸 B2／B10。（建議；天數可改）
- B：賣家自訂處理天數——多一個設定與 UI，延後。

### D6 配送異常轉售後（擋 3）
- A：自寄沒有貨態，不做自動偵測；買家在 `SHIPPED` 期間可回報「未收到」→ `DELIVERY_ISSUE` 並發事件，售後案件由 aftersales 建（B9 前：只發事件＋通知賣家，Java aftersales 不接）。（建議）
- B：逾期未完成自動轉異常——沒有訊號來源，只是換名的自動完成。

### D7 承運商欄位（擋 2、5）
- A：proto enum 固定清單（中華郵政、黑貓、新竹物流、嘉里大榮、宅配通）＋`OTHER` 加自由文字名稱；追蹤號碼必填（`OTHER` 也必填）。（建議）
- B：純自由文字——無法顯示一致名稱、無法日後接追蹤連結。

### D8 退貨物流（擋 4、7）
- A：退貨物流歸 aftersales（B9），同樣是買家自寄回填；B3 不做。Java aftersales 依賴的 4 支 PayUni 退貨 RPC（`CreateReturn*`、`RetryReturnPrintLabel`）與 `shipment.return.status_changed` 隨 Java shipping 刪除，**B3 到 B9 之間 Java 的退貨物流不可用**（系統未上線）。（建議）
- B：B3 做 direction=RETURN 的同一張出貨單——把 aftersales 的流程提前塞進 fulfillment。

### D9 物流 port 與模擬 adapter（ADR D3 已定；確認必要條件）
- 主線不經 port；port 在 B3 只被模擬 adapter 與測試使用（C Q7）。必要條件只有「證明上線時能插真實 adapter」，ADR 自己承認模擬只證明形狀。
- A：照 ADR 做最小 port（建單、取標籤、webhook inbox＋事件 ID UNIQUE），無 client UI、無 gateway 路由。（建議：ADR 是 Will 的決定）
- B：修訂 ADR D3，port 延後到決定上線時——少一個無生產使用者的機制。

### D10 配送方式碼收斂（擋 2、5、6）
- A：對外只剩「賣家自寄」，配送方式 enum 從契約拿掉（每筆訂單都自寄，沒有可選項）；`3001／3002／3003` 刪，`3005 MERCHANT_DELIVERY`（目前是 RFQ 站外出貨，I §13-9）由 procurement 自己的欄位表達；client 的 8 列守門測試改寫或刪除。（建議）
- B：保留單值 enum 為日後擴充——不留只為未來的欄位（不留技術債）。

### D11 賣家新訂單推播（擋 7；A8 notification 加邊）
- A：notification 改訂閱 `order.paid`（過渡期由 Java order 發）；不再依賴 `shipment.created`。（建議：綁原因「建單失敗不通知」在自寄不存在）
- B：fulfillment 建立出貨單時發同名事件——多一個轉手。

### D12 UI 改動同意（擋 5、6；分組，K §9）
1. 結帳與購物車：移除超商取貨、門市選擇與地址簿門市分頁、配送方式分組；運費改讀 `QuoteShipping`。
2. 買家物流追蹤：改顯示承運商＋追蹤號碼＋出貨時間，不再顯示貨態時間軸；新增「確認收貨」「回報未收到」。
3. 賣家出貨（新畫面）：單一「填承運商＋追蹤號碼」對話框，取代黑貓預約、叫車、7-11 交貨便與標籤列印。
4. 賣家物流開通：刪 website 物流開通頁、seller-app 物流設定、上手清單的「物流設定」步驟；改成賣家設定頁的「運費設定」（固定運費＋免運門檻）。
5. 異常：刪 seller-app 異常看板與首頁橫幅、website「配送異常」快篩與 badge、admin 異常頁 `shipping` tab、通知偏好的「配送異常」開關。
6. 商品設定：移除依尺寸即時報價（`POST /app/shipping_quotes`）與商品層運費欄位。
7. 欄位內容：admin 商品詳情、訂單列表配送方式欄、訂單卡配送資訊改成承運商／追蹤號碼。
- 建議：1–7 全同意（皆由 ADR 2026-11-15 直接導出）；新畫面 3、4 先做線框給你看再實作。

### D13 文件與既有 ADR（擋 8）
- A：主計畫與 ADR 對設計書 69 的引用改指本計畫；設計書 65、06、20–23 改寫成自寄，03、09、29、36、58 刪物流商段落；免運補貼 ADR（`2026-09-11-free-shipping-subsidy-attribution`）、`logistics-finance-domain-ownership`、`shipping-fee-cache-no-warmup-no-scheduler`、`checkout-free-shipping-and-degradation` 與 4 份 Uber ADR 標 superseded。（建議）

### D14 與 B4 的刪除次序（擋 7）
- gateway `handler/shipping.go`、`shipping_options.go` 引用 B4 的 `shipping_ambient.go`（C Q8）。
- A：B3 刪 handler 時連同對它的引用一起刪，`shipping_ambient.go` 本身留給 B4；兩邊誰先合進 dev，後者 rebase 解衝突。（建議）

## 既有缺陷處置（I §12，24 條）

全部隨 Java 刪除消失，建議處置「接受（隨刪除）」；新設計以下列方式避免重犯，不另寫回歸測試：D1（新表 `order_id` UNIQUE、不混退貨）、D2／D4／D10（outbox 同交易、事件 ID 固定）、D9（inbox UNIQUE 先寫）、D6（用 `bizcalendar`）、D13（ID 用模組既有方案）、D21（文字走 `internal/text`／uitext）。D3、D5、D7、D11、D12、D15–D20、D22–D24 的機制不存在於新設計。拍板時一併確認。

## 拍板順序

D1、D2、D4、D10 先（決定契約形狀），接著 D15；D3、D5、D6、D7、D8 依 D2／D4；D11、D13、D14 可同時；D12 UI 同意可同時給，但 3、4 的線框在步驟 2 契約後交。D9 獨立。

### D15 過渡期 Java 結帳怎麼拿運費（擋 4；審查第 1 輪 F2 升格）
- 現況：Java order 的 `CheckoutService`、`CheckoutValidator` 先把平台級距運費拉進本地快取，結帳試算並回 `CheckoutValidate.available_shipping_options`（`caller-inventory.md` §1.3）。Java order 會在 B8 刪，D1 否決了「改即將被刪的 Java」，但結帳仍得算運費。
- A：最小改動——運費來源換成 legacy 轉接的 `QuoteShipping`，刪本地快取與 `ShippingFeeGrpcFetcher`；`available_shipping_options` 只回一筆「賣家自寄」。（建議：B3 到 B8 之間結帳金額正確）
- B：過渡期結帳固定 0 運費，到 B8 才接——Java 改動最少，但這段期間結帳金額是錯的。

## 與 B4 計畫的交界（B4 session 並行，`2026-11-24-b4-catalog/decision-brief.md`，2026-11-24 讀）

- **B4 D10**（建議：catalog 存「啟用的配送方式代碼集合」，經 fulfillment 驗證）與本簡報 **D10**（建議：契約拿掉配送方式，只有賣家自寄）衝突。若本 D10 選 A，catalog 不需要配送方式欄位，B4 D10 的 A 變成「catalog 不存配送方式、運費全歸 fulfillment」；兩題要一起拍。
- **B4 D11**（建議：B4 刪整個 `shipping_ambient.go`）與本 **D14** 相容：B3 刪 handler 時帶走引用；兩邊誰先合進 dev，後者 rebase。
