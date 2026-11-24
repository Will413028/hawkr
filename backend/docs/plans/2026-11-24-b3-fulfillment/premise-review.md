# B3 前提重審：出貨該由誰擁有（業界基線 vs 現行計畫）

快照 dev `3d8ef0bc0`。證據：【量】讀過 source 或跑過指令；【推】推論；【文件】公開 API 文件的已知做法，沒在本 repo 實測、本次也沒上網逐頁核對。本檔挑戰 `decision-brief.md`（v1）沿用的兩個前提：「fulfillment 是擁有出貨狀態機的獨立模組」與「B3 要讓 Java order／aftersales 在過渡期看得到新出貨」。已經過一輪獨立批判（修正記錄在文末）。

## 前提題：被改的程式是否排定要重寫、搬走或刪除？

- v1 步驟 4 的過渡介面（`internal/legacy` 的 shipping gRPC 轉接、`shipment.*` relay、Java `CheckoutService` 改接 `QuoteShipping`）只服務 Java order、aftersales 與 Go invoice-service，三者都排在 B8、B9 或 invoice 改寫時刪除。這段程式在那時全部丟掉。規模沒量，只能說它要轉接 Java 端 19 處呼叫、3 個 client（`caller-inventory.md` §1.1【量】）並重發事件；先前寫的「佔 B3 的 20–30%」沒有依據，撤回。
- 分類：
  - 「Java order 在 B3 期間還活著」是 **REAL**：B8 的前置 B2、B4、B6 現在都沒完成（主計畫 `[ ]`【量】）。
  - 「B3 必須讓活著的 Java order 讀到**新的**出貨流程」是 **SELF-IMPOSED**：
    - Java 現行出貨整條建在 PayUni 物流上（建單呼叫 PayUni、狀態是 PayUni 碼、訂單由 PayUni 貨態事件推進；`interface-inventory.md` §5、§7【量】），而 PayUni 代理商不簽（Will 2026-10-29）、出貨主線已改自寄（ADR 2026-11-15）。
    - 系統沒上線（根 `AGENTS.md`）。
  - 兩條路都有會丟的工作：v1 的轉接，或 P3 的 Java 端刪改。P3 量的是後者。

## 業界基線：出貨歸誰

| 平台 | 賣家自寄的出貨記在哪 | 證據 |
|---|---|---|
| Etsy | `createReceiptShipment`：掛在 receipt（訂單）下，一張訂單可有多筆出貨，各帶承運商與追蹤號碼 | Etsy Open API v3【文件】 |
| eBay | `createShippingFulfillment`：`/order/{orderId}/shipping_fulfillment`，可部分出貨、多筆 | eBay Sell Fulfillment API【文件】 |
| Amazon 賣家自配送 | Orders API `confirmShipment`：訂單層確認出貨 | SP-API Orders【文件】 |
| Shopee | `logistics.ship_order` 以 `order_sn` 出貨；訂單狀態機（含 PROCESSED、SHIPPED、TO_CONFIRM_RECEIVE、COMPLETED）由平台訂單管 | Shopee Open Platform【文件】 |
| Shopify | 每張訂單一律至少一張 `FulfillmentOrder`，依出貨地點或第三方倉拆成多張；出貨（Fulfillment）掛在 FulfillmentOrder 下 | Shopify Admin API【文件】 |

- 共同點：出貨記錄是**訂單底下的 0..N 筆子實體**（承運商、追蹤號碼、出貨時間、含哪些品項），不是另一個服務的獨立狀態機；訂單的出貨狀態由這些記錄推導。Shopify 多一層 FulfillmentOrder 是為了「哪個地點出哪些品項」的路由，本專案沒有多地點或第三方倉。
- 運費設定在各家都**不在訂單裡**：Shopify 的 shipping profiles 與 Etsy 的 shipping profiles（含處理天數、運費、免運）是賣家或商店的設定，結帳時用它報價，把結果寫進訂單【文件】。

## 現行設計（v1）為什麼長這樣

- Java shipping-service 獨立出來，是因為要串多家物流商（PayUni、Tcat、Uber）與它們的 webhook。訂單狀態與出貨狀態各存一份，靠事件以 CAS（比對前置狀態才更新）同步（`caller-inventory.md` §1.2【量】）。這份雙重狀態是 Java 多數出貨缺陷的來源（`interface-inventory.md` D1、D2、D4、D10、D11【量】）。
- `modules.go` 的 `fulfillment` 模組與指向它的邊（catalog、invoice、ordering、aftersales、finance）是照 Java 服務切法定的（`backend/internal/arch/modules.go:21-31`【量】），沒重新評估。
- 物流商退場後，獨立出貨狀態機的必要條件（多家物流商的建單與貨態）已經不存在。

## 決定題

### P1 出貨的擁有者（取代 v1 的 D4、D6，改寫 D1、D5、D11、D15）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- **A：訂單聚合擁有出貨**（建議）。
  - ordering（B8）擁有出貨記錄（0..N 筆，作品集先只開放整單一筆，資料表照多筆設計）。
  - ordering 也擁有由出貨記錄推導的訂單狀態（`AWAITING_SHIPMENT → SHIPPED → COMPLETED／CANCELLED`），以及逾期取消、自動完成、確認收貨、「未收到」回報。
  - 只有一份狀態。
  - 失敗條件：之後要接多地點、第三方倉，或物流商的貨態要回寫時，需要在 ordering 內加一層 FulfillmentOrder。這是單體內同模組的擴充【推】。
- B：獨立 fulfillment 模組擁有出貨狀態機（v1）。失敗條件：兩份狀態要靠事件同步，正是 Java 的缺陷來源。
- 推翻建議的事實：決定上線並接真實物流商，而且需要多地點或第三方倉。
- 可逆性：兩個方向互換，都是單體內搬程式，成本中。
- **連帶改 ADR**：ADR 2026-11-15 寫的作品集目標是「出貨單與訂單的狀態機、模組分界與事件」，D3 要做物流 port。選 A 時：
  - 狀態機在 B8 才交付；
  - 「模組分界」改成「訂單擁有出貨」；
  - 這份 ADR 要修訂（P5 一起處理）。

### P2 模組邊界與依賴邊（依 P1；v1 漏的依賴環）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- 問題：invoice 要讀出貨狀態，`invoice-service/internal/client/shipping/client.go`、`grpc/shipment_status.go`【量】。如果改成 invoice 依賴 ordering，而 ordering 已依賴 invoice（`modules.go:26`），就會形成依賴環。
- **A：出貨相關的下游都由訂單擁有者推或訂閱，不反向讀出貨**（建議；同 B2 D5「saga 的補償由擁有狀態的一方發起」）：
  - **invoice**：開票時點由 ordering 在狀態轉換時以 in-process 呼叫 invoice（ordering→invoice 已存在），invoice 不再讀出貨狀態。
  - **aftersales**：已依賴 ordering，直接讀訂單的出貨記錄。
  - **catalog→fulfillment 刪除**：P4 下商品不引用運費設定。這也推翻已拍板的 B4 D10（見 P6）。
  - **finance**：已退場（主計畫 B10），邊隨之刪。
  - **fulfillment 模組刪除。**
- B：保留 fulfillment 模組只放運費設定。代價：多一個只有一張表的模組，以及 cart、ordering 對它的新邊。

### P3 B3 到 B8 之間的 Java 訂單（取代 v1 的 D15；依 P1-A）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- 事實（已查程式【量】）：
  - Java 訂單要先到 DELIVERED 才能完成（`OrderRepository.java:667-672`），而 DELIVERED 只由 `ShippingEventConsumer` 推進。
  - 刪掉 Java shipping 後，Java 訂單停在已付款。下游因此不會觸發：
    - B2 的 `GrantPaymentWorker`（訂單完成才轉帳）。B2 的驗收直接呼叫 legacy RPC，不受影響（B2 計畫第 95 行）。
    - 單體 crm 收的 `order.completed`。
    - 賣家關店：碰到未結訂單會停（`closing/termination.go:21,41`），有已付款訂單的店就關不掉。
  - invoice-service 會失去 shipping gRPC 的呼叫對象。
  - coupon 沒有依賴出貨；notification 沒有逐檔看完。
- **A：接受這段空窗，並把 Java 端依賴出貨的程式刪乾淨**（建議）。
  - 要刪的有：`ShippingEventConsumer`、`AutoCompleteScheduler`、`ShippingFeeGrpcFetcher`、`ShipmentCreationBackstopScheduler`、order 與 aftersales 的 shipping client、invoice-service 的 shipping client 與 `shipment_status.go`。
  - 結帳運費固定 0，配送選項移除。
  - 不留回空的 stub。
  - 失敗條件：B3 到 B8 之間要示範「下單 → 完成 → 轉帳」，或要關一間有已付款訂單的店。
- B：在 Java 做最小補丁，讓賣家「標記已出貨」直接把訂單推到 SHIPPED 再到 DELIVERED，保住完成鏈。成本小於 v1 的轉接，但仍是在 B8 會刪掉的程式上加功能。
- C：v1 的完整轉接。到 B8 全部丟掉，成本最高。
- 推翻 A 的事實：B8 會晚很久（例如 B6 卡住），而這段期間需要完整的示範鏈。

### P4 運費設定的形狀與擁有者（改寫 v1 D2；影響 B6）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- 基線：Etsy、Shopify 的 shipping profile 屬於商店，可以有多份，商品引用其中一份；每份含處理天數、運費、免運門檻【文件】。
- **A：每個賣家一份設定，放在既有的 seller 模組**（建議）。
  - 內容：固定運費（TWD）、免運門檻（可空）、處理天數（預設 3 個工作日，可改 1–7）。
  - 報價是純函式 `QuoteShipping(seller, merchandise_subtotal)`。
  - cart、ordering 原本就依賴 seller（`modules.go:24,26`【量】），不必加邊，也不建新模組。
  - 理由：台灣國內單一區、沒有重量級距，多份設定沒有使用情境。
  - 失敗條件：賣家要讓大件、小件收不同運費。
- B：多份設定加商品引用（Etsy 基線）。catalog 要存設定的參照，多一組 UI。
- C：獨立的 shipping 模組（P2-B）。
- 處理天數由賣家設，是 Etsy 的做法；Shopee 是平台預設 2 天【文件】。處理天數決定 B8 的逾期期限。
- 可逆性：A 改 B 是加一張表和一個參照欄，成本低。

### P5 物流 port 與模擬 adapter（重提 v1 D9；連同 P1 修訂 ADR）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- 必要條件：「上線時能插真實物流商」。P1-A 下，貨態回寫要進訂單；port 的形狀取決於真實物流商，ADR 自己也寫了模擬只證明形狀。
- **A：修訂 ADR D3，port 與模擬 adapter 延後到決定上線並簽約時**（建議）。現在寫的 port 沒有生產使用者。inbox 去重的作品集展示已在 B2 做了（B2 計畫第 3、31 行，`internal/platform/inbox`【量】）。
- B：照 ADR D3，在 B8 的 ordering 內做最小 port。

### P6 B4 D10 要重開（已拍板，2026-11-24 Will）
- **狀態：已決 2026-11-24 Will（照建議 A）**；ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md` 文末修訂。
- 現況：B4 D10 定為「catalog 存啟用的配送方式代碼集合，經 fulfillment API 驗證」（`2026-11-24-b4-catalog.md:117`【量】）。只剩賣家自寄一種方式時，這個集合沒有內容，前提不成立。
- **建議**：選 P1-A、P4-A 時，B4 D10 改成「catalog 不存配送方式、不依賴運費設定」，通知 B4 session 修訂。

## 不受本次重審影響的 v1 決定

- 照舊的：
  - D7 承運商清單加 OTHER；
  - D10 契約拿掉配送方式；
  - D12 的刪除組（1、5、6）；
  - D13 文件與 ADR 標 superseded；
  - D14 與 B4 的刪除次序。
- D7 和 D12 的新畫面組（2、3、4）移到 B8。
- D8 退貨物流歸 B9 照舊。
- D3（cart 依賴邊）在 P4-A 下不需要。

## 限制帳

| 限制 | 分類 | 來源 |
|---|---|---|
| Java order、aftersales 活到 B8、B9 | REAL：B8 的前置 B2、B4、B6 未完成 | 主計畫【量】 |
| B3 期間要讓 Java 看到新出貨 | SELF-IMPOSED：系統未上線，而且 Java 出貨流程本身綁 PayUni | 根 `AGENTS.md`、ADR 2026-11-15 |
| `modules.go` 的 fulfillment 邊 | SELF-IMPOSED：我們自己的程式，照 Java 切法 | `modules.go:21-31` |
| 溫層與 `shipping_ambient.go` 歸 B4、B6 | SELF-IMPOSED：並行 session 的分工，照 Will 指定遵守 | Will 交辦；與 B4 簡報一致 |
| B6 需要運費報價 | REAL：B6 的範圍 | 主計畫 B6 |
| 台灣物流商的多賣家模式要簽約 | REAL | ADR 2026-11-15 |
| B4 D10 已拍板 | 我們自己的決定，可重開（P6） | B4 計畫 |

## 終態與觸發

- 終態：
  - ordering 擁有訂單底下的出貨記錄與訂單狀態機；
  - seller 擁有賣家運費設定與報價；
  - 沒有 fulfillment 模組、Java shipping、PayUni 物流、Uber。
- 觸發重看：
  - 決定上線並簽物流商：重開 P1、P5；
  - 賣家要多種運費：P4 改選 B。

## 批判修正記錄（第 1 輪，2026-11-24）

- 2 個 blocking：
  - 依賴環 → 新增 P2；
  - B4 D10 衝突 → 新增 P6。
- 6 個 should：
  - 補上空窗的影響與選項 P3-B；
  - 基線改成「0..N 筆出貨」；
  - 不留 stub，改成刪除清單；
  - P1、P5 寫明要修訂 ADR，B2 的 inbox 已查證；
  - 撤回沒有依據的「20–30%」。
- 2 個 nit：
  - 運費設定改放 seller（P4-A）；
  - 溫層的分類改成 SELF-IMPOSED。
