# Payment Service 詳細設計書

---

> ⚠️ **錯誤處理慣例**：本設計書內所有 sequence diagram 之 gRPC client 呼叫，error path 統一依 [`Java gRPC 錯誤處理規範.md §7.2.1`](../開發規範/Java%20gRPC%20%E9%8C%AF%E8%AA%A4%E8%99%95%E7%90%86%E8%A6%8F%E7%AF%84.md#721-%E6%AC%8A%E5%A8%81%E6%99%82%E5%BA%8F%E5%9C%96%E6%89%80%E6%9C%89%E8%A8%AD%E8%A8%88%E6%9B%B8%E5%BC%95%E7%94%A8%E6%AD%A4%E5%9C%96%E4%B8%8D%E5%86%8D%E5%80%8B%E5%88%A5%E7%B9%AA%E8%A3%BD)。個別 diagram 僅繪製 happy path，error 分支不重複繪製。Client 禁止 `if (resp.code != 0) throw`、禁止 `throw new RuntimeException`，一律 `GrpcClientErrorTranslator.translate(e, XxxBusinessException::new)`。

## 1. 服務概述與責任邊界

### 1.1 服務職責

- 唯一職責：作為系統與第三方金流（PayUni 代理商模式）之間的唯一閘道
- 核心功能：
    - 交易參數的封裝、加密（AES-256-GCM）與簽章（SHA256），加解密委託 `payuni-java-client` SDK
    - 支援多種支付方式：信用卡（UPP）、Apple Pay（OnWeb/InApp 依 ClientType 自動切換）、Google Pay（OnWeb/InApp 依 ClientType 自動切換）、ATM
    - **Apple Pay on Web Merchant Validation**：作為前端 `ApplePaySession.onvalidatemerchant`（Safari）/ W3C `PaymentRequest.onmerchantvalidation`（Chrome）的後端代理；使用 Apple Pay Merchant Identity Certificate（PEM + Key，由 PayUni 協助申請）對 Apple `validationURL` 發起 mTLS POST，原樣透傳 `merchantSession` JSON 給前端調用 `completeMerchantValidation()`。詳見 §5.11
    - **代理商撥款控制**：所有交易建立時帶入 `GrantExport=1`，由代理商控制撥款時機；買家確認收貨後透過 Grant API 發動撥款指示
    - 透過 MQ 消費 `payuni-webhook-service` 發布的付款結果事件，處理交易狀態更新
    - 管理交易狀態流轉（INIT → PAID / FAILED / EXPIRED / CONFLICT / CLOSED）
    - 獨立的退款生命週期管理（REQUESTED → PROCESSING → SUCCEEDED / FAILED）
    - 主動查詢 PayUni 交易狀態（SyncPaymentStatus）
    - 付款結果事件（`payment.success` / `payment.failed` / `payment.conflict`）由**六個發布點**經**兩收斂點**（A=`PaymentNotifyHandler`、B=`PaymentApplicationService.publishEvent`）統一走 `PaymentOutboxWriter.enqueueAndDispatch`：事務內 INSERT `outbox_events`（與狀態落地、Inbox 標記原子）+ afterCommit 即時 publisher-confirm 投遞 + 排程器每 5s 兜底 + 消費端 `trade_no` 冪等（原子化 + 即時履約 + at-least-once 可靠投遞，見 §5.10）；rescue 事件（`payment.rescue.refund_initiated`）亦走 outbox 與觸發它的 rescue marker 同 tx 原子化——reconcile 單筆 rescue 經 `PaymentOutboxWriter.enqueueAndDispatch`、重複付款偵測經 `DuplicatePaymentService` 同 tx INSERT（詳 §2A.6.2 / §5.7）；ATM「需人工」**不發 MQ 事件**，改以 `payment_transactions.rescue_action='8902'` DB 狀態 + sql_exporter `PaymentRescueManualPending` 告警；**`payment.expired` 同樣走 outbox**（`ExpiredTransactionScheduler` 的 per-item catch 會吞例外，事務邊界對它沒有保護作用，見 §5.7）；其餘事件（`payment.closed` / `refund.*`）由 `PaymentEventPublisher` 在 gRPC 觸發的事務內直接發布，失敗即 rollback

### 1.2 上游 / 下游依賴

**上游依賴（Payment Service 呼叫）：**

- **Order Service（gRPC）**：`GetOrderForPayment(order_no)` — 建立交易前查詢 `mer_id`（PayUni 商家編號）+ `amount`（付款金額）+ `buyer_id`（身分驗證用）。確保金額和商家編號全程後端控制，前端無法竄改。

**下游依賴（消費 Payment Service 事件）：**

- **Order Service（MQ）**：消費 `payment.events` Exchange 的事件（`payment.success`、`payment.failed`、`payment.expired`、`payment.closed`、`payment.conflict`、`refund.succeeded`、`refund.failed`），更新訂單狀態、釋放庫存等。

### 1.3 非職責範圍

- 不處理庫存（由訂單服務負責）
    - 不處理物流（由物流服務負責）
- 不直接面對使用者介面
- **不直接接收 PayUni HTTP Webhook**（由 `payuni-webhook-service` 統一接收、驗簽、解密、去重後透過 MQ 分發）
- 🔴 **不決定第三方手續費的經濟負擔方**：本服務只負責「把正確的金額用正確的通道送出去」。手續費**由誰吸收**是業務決策（Uber 物流服務費的 `agent_fee` 手續費＝賣家負擔，65 稽核附錄 `ADR-005`），本書不得自行推定、也不得把費率寫成常數
- **不處理商家狀態異動（MerStatus）Webhook**（由 `payuni-webhook-service` 發布至 `seller.review.result`，由單體 seller 模組消費，AGENT_NOTIFY 單通道）

### 1.4 系統架構圖

```mermaid
graph TD
User((User/Browser))
GW[Gateway :9900]
PaySvc["Payment Service :9924/9925"]
WebhookSvc["PayUni Webhook Service :9946"]
PayUni["PayUni（代理商模式）"]
DB[(PostgreSQL)]
MQ((RabbitMQ))
OrderSvc[Order Service]

User -- 1. 選付款方式 --> GW
GW -- 2. gRPC CreatePayment(orderNo) --> PaySvc
PaySvc -- 2.5 gRPC GetOrderForPayment --> OrderSvc
OrderSvc -- 2.5 mer_id + amount --> PaySvc
PaySvc -- 3. AES-256-GCM 加密 --> PaySvc
PaySvc -- 4. UppFormData --> GW
GW -- 5. Return Form --> User
User -- 6. Form POST --> PayUni
PayUni -- 7. Webhook --> WebhookSvc
WebhookSvc -- 8. verify/decrypt/dedup --> WebhookSvc
WebhookSvc -- 9. Publish payuni.events --> MQ
MQ -- 10. payuni.payment.uop --> PaySvc
PaySvc -- 11. 狀態 + outbox_events 同事務原子寫入 --> DB
PaySvc -- 12. afterCommit 即時 / 排程器兜底 publisher-confirm 投遞 payment.events --> MQ
MQ -- 13. Consume --> OrderSvc
```

### 1.5 名詞定義

| 名詞 | 說明 |
|------|------|
| TradeNo | 系統內部交易編號，即 `payment_transactions.trade_no`，格式 **`{orderNo}{序號}`**（序號自 `001` 起遞增，`attempt_seq ≥ 1000` 時為 4 位），同時作為 PayUni 的 MerTradeNo。完整規則見 §2A.3.6 |
| MerTradeNo | PayUni 端的商店交易編號，與系統 TradeNo 值相同（同一訂單的每次付款嘗試各自一個） |
| ProviderTradeNo | PayUni 回傳的 UNi 序號（`payment_transactions.provider_trade_no`），用於退款 / 查詢 / 撥款 API |
| GrantExport | PayUni 代理商撥款控制旗標，設為 `1` 表示由代理商控制撥款時機，PayUni 不會自動撥款給商店 |
| UPP | PayUni Unified Payment Page — 信用卡統一付款頁面，採用 Form POST 重導模式 |
| Inbox | `payment_webhooks` 表，用於記錄從 MQ 消費的每一筆事件，確保冪等性與稽核追蹤 |
| Apple Merchant Identity Certificate | Apple Developer 後台為 Merchant ID 簽發的 mTLS 客戶端憑證（PEM + Private Key），由 PayUni 協助申請流程（見 [PayUni 金流官方 API 總覽 §4.2](../第三方整合/PayUni%20金流官方%20API%20總覽.md)）。後端載入後對 Apple `apple-pay-gateway*.apple.com` 取得 `merchantSession` |
| validationURL | Apple Pay sheet 啟動瞬間在 `onvalidatemerchant`（Safari `ApplePaySession`）或 `onmerchantvalidation`（Chrome W3C `PaymentRequest`）callback 中提供的一次性 URL。必須由後端使用 Apple Merchant Identity Certificate POST 才能換得 `merchantSession`，前端不得直接呼叫 |
| merchantSession | Apple 回傳的 opaque JSON，前端取得後傳入 `ApplePaySession.completeMerchantValidation()` / `PaymentRequest event.complete()` 啟動付款 sheet。後端原樣透傳，不解析、不快取 |

### 1.6 角色與權限邊界

| 呼叫來源 | 呼叫方式 | 可用 RPC | 權限機制 |
|----------|----------|----------|----------|
| Gateway（前端 APP） | gRPC（經 JWT 驗證） | CreatePayment, GetPayment, GetPublicPaymentStatus, SyncPaymentStatus, ClosePayment, GetRefund, GetRefundsByTradeNo | JWT `accountId` + `accountType` 驗證 |
| Order Service | gRPC（內部服務） | ClosePaymentByOrderNo, GrantPayment, GetActivePaymentStatusByOrderNo | 內部授權（見下方） |
| AfterSales Service | gRPC（內部服務） | GetRefundPageUrl, CreateRefund | 內部授權（見下方） |
| Gateway（頂層路由） | HTTP（無 JWT） | HandlePaymentPageReturn, DecryptPayUniResult, GetPaymentReturnPage | 頂層路由，無需 JWT（WebView / PayUni 導回） |

> 🔴 **內部授權以 HMAC 簽章為準，`x-internal-service` 字面值不是信任依據**：呼叫端由 `GrpcAuthClientInterceptor` 自動加簽（四個 header：`x-internal-service` / `-ts` / `-nonce` / `-sig`，canonical 綁 accountId 與時間窗）；server 端 `GrpcAuthServerInterceptor` **只對帶 `x-internal-service` 的呼叫驗簽**，通過才令 `isInternalCall()=true`，此時 `GrpcAuthContext.requireXxx()` 跳過使用者面授權。行為依 `INTERNAL_AUTH_MODE`：`enforcing` 下缺簽或驗簽失敗一律 `UNAUTHENTICATED`；`permissive` 放行並記 `internal.auth.unsigned` / `internal.auth.invalid` WARN 供觀察。完整規格見 [52-微服務間內部授權](./52-微服務間內部授權（Internal%20Service%20Authorization）詳細設計書.md)。
> **禁止**以 header 字面值作為授權依據，也禁止 Gateway 帶或簽這組 header（使用者面一律走 `requireAccountType`）。

> **CreateRefund 收斂為 internal-only**：`CreateRefund` 僅接受內部服務 internal call（`isInternalCall()==true`），列於上表 AfterSales Service 內部呼叫列（實際發起方為其 `RefundTaskWorker`），**不再對 Gateway（前端 APP）開放**。退款一律由買家提出售後申請 → AfterSales 審核核准後經 internal call 觸發（見 §5.5.3），買家不再有自助發動退款的入口，避免繞過售後審核造成資損。查詢類 `GetRefund` / `GetRefundsByTradeNo`（買家查自己的退款）仍保留於 Gateway 列。

---

## 2. 狀態模型與業務規則

本服務的交易狀態與退款狀態為**兩個獨立的狀態機**。退款擁有獨立生命週期，不影響交易主表狀態。

所有狀態更新都必須遵守「白名單狀態轉移」+ 樂觀鎖（version）。

### 2.1 交易狀態機

**Payment Transaction Status（交易狀態）：**

| **Code** | **Status** | **說明** |
| --- | --- | --- |
| 0 | INIT | 建立交易完成，尚未付款 |
| 1 | PAID | 已付款完成 |
| 2 | FAILED | 付款失敗（金流回傳失敗） |
| 3 | EXPIRED | 逾時未付款（終態） |
| 4 | CONFLICT | 同步查詢與 Webhook 結果不一致（需人工介入） |
| 5 | CLOSED | 交易已關閉（終態） |

> 終態：**PAID / FAILED / EXPIRED / CONFLICT / CLOSED** 不可回退。

**交易狀態轉移矩陣（白名單）：**

| **From ＼ To** | **INIT (0)** | **PAID (1)** | **FAILED (2)** | **EXPIRED (3)** | **CONFLICT (4)** | **CLOSED (5)** |
| --- | --- | --- | --- | --- | --- | --- |
| INIT (0) | - | ◯ | ◯ | ◯ | - | ◯ |
| PAID (1) | - | - | - | - | ◯ | - |
| FAILED (2) | - | - | - | - | ◯ | - |
| EXPIRED (3) | - | - | - | - | - | - |
| CONFLICT (4) | - | - | - | - | - | - |
| CLOSED (5) | - | - | - | - | - | - |

**補充規則：**
- INIT → PAID：由 **MQ 消費 PayUni Webhook 事件** 或 **SyncPaymentStatus（主動查詢補單）** 觸發
- INIT → FAILED：由 **MQ Webhook 事件**（付款失敗）或 **SyncPaymentStatus** 觸發
- INIT → EXPIRED：由 **ExpiredTransactionScheduler** 觸發（expire_at 已到期）
- PAID/FAILED → CONFLICT：Apple Pay / Google Pay 同步終態與後續 Webhook 結果矛盾時觸發（由 `PaymentNotifyHandler` 衝突檢測邏輯處理）
- INIT → CLOSED：由 **ClosePayment gRPC** 或 **ClosePaymentByOrderNo gRPC**（Order Service 取消訂單時）觸發（僅待付款的交易可主動關閉）

### 2.2 退款狀態機

退款狀態獨立於交易狀態，值域為 `RefundStatus` 的**六個**值（定義見 [業務狀態碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務狀態碼.md)）：

| **Code** | **Status** | **說明** |
| --- | --- | --- |
| 0 | REQUESTED | 退款已申請 |
| 1 | PROCESSING | 退款處理中（PayUni 已受理） |
| 2 | SUCCEEDED | 退款成功（終態） |
| 3 | FAILED | 退款失敗（終態） |
| 4 | APPROVED | 退款已核准、尚未送出至金流 |
| 5 | MANUAL_REVIEW | 需人工介入判定，未進入自動處理 |

**退款狀態轉移矩陣（白名單）：**

| **From ＼ To** | **REQUESTED (0)** | **PROCESSING (1)** | **SUCCEEDED (2)** | **FAILED (3)** | **APPROVED (4)** | **MANUAL_REVIEW (5)** |
| --- | --- | --- | --- | --- | --- | --- |
| REQUESTED (0) | - | ◯ | ◯ | ◯ | ◯ | ◯ |
| PROCESSING (1) | - | - | ◯ | ◯ | - | ◯ |
| SUCCEEDED (2) | - | - | - | - | - | - |
| FAILED (3) | - | - | - | - | - | - |
| APPROVED (4) | - | ◯ | - | ◯ | - | ◯ |
| MANUAL_REVIEW (5) | - | ◯ | ◯ | ◯ | ◯ | - |

**補充規則：**
- REQUESTED → PROCESSING：PayUni 退款 API 回應受理成功
- REQUESTED → FAILED：PayUni 退款 API 即時回應失敗
- PROCESSING → SUCCEEDED / FAILED：由 PayUni 退款結果回調或主動查詢觸發
- APPROVED：已核准但尚未送出至金流；MANUAL_REVIEW：自動路徑無法判定，等待人工處置
- **終態只有 SUCCEEDED(2) 與 FAILED(3)**；`APPROVED(4)` 與 `MANUAL_REVIEW(5)` 皆為非終態

🔴 **撥款資格的連帶（跨服務不變式）**：凡退款**未終結**一律不得撥款——`status ∈ (REQUESTED, PROCESSING, APPROVED, MANUAL_REVIEW, FAILED)` 時撥款閘門必須擋下。新增任何退款狀態值時**必須同批檢查該閘門的集合**，漏一個值就等於錢在爭議未決時被撥出去（撥款不可逆，平台端無賣家扣回機制）。

🔴 **DB CHECK 必須與值域同步**：`chk_refund_status` 少列任一值即代表該狀態無法落地，呼叫端只會看到 DB 約束違反（見 §3.3）。

### 2.3 狀態更新落地規範

所有狀態更新搭配**樂觀鎖（version）** 與**條件式更新**：

```sql
-- 範例：付款成功
UPDATE payment_transactions
SET status = 1, provider_trade_no = ?, pay_time = ?, version = version + 1
WHERE trade_no = ? AND status = 0 AND version = ?;
```

**若影響筆數 = 0：**
- 代表已被其他流程更新（版本衝突）或已進終態
- 記錄 WARNING 至 `payment_logs`（含 trade_no / from_status / to_status / trigger_source）
- 不得強制覆寫狀態

### 2.4 重複扣款防護規則

防止同一訂單被重複扣款，是支付服務最關鍵的安全設計。本服務採用三層縱深防禦，任一層失效時由下一層攔截。

**攻擊情境（未防護前）：**

```
使用者按下「付款」→ 網路超時 → 再按一次
→ 兩筆獨立的 PayUni 交易被建立（各有不同 trade_no）
→ 重複扣款
```

**第一層：應用層冪等性檢查（主要防線）**

- 位置：`PaymentApplicationService.createPayment()`
- 機制：建立交易前先查詢 `findActiveByOrderId(orderId)`
- 邏輯：
  - 找到 **PAID** 交易 → 拋出異常「訂單已付款，不可重複建立支付」
  - 找到 **INIT** 交易 → 回傳既有交易資訊（冪等性回應）
  - 無活躍交易 → 正常建立新交易
- 效果：同一 order_id 的重複請求不會產生新的 PayUni 交易

**第二層：資料庫約束（最後防線）**

- 位置：`V1__init_payment_tables.sql`
- 機制：Partial Unique Index
  ```sql
  CREATE UNIQUE INDEX idx_payment_transactions_order_active
      ON payment_transactions(order_id)
      WHERE status IN (0, 1);  -- INIT / PAID
  ```
- 效果：即使應用層冪等性檢查因競態條件（Race Condition）被繞過，DB 約束仍會拒絕插入，確保同一訂單最多只有一筆活躍交易
- 設計考量：使用 Partial Index 而非全表 UNIQUE，允許已終結的交易（FAILED/EXPIRED/CONFLICT/CLOSED）重新發起支付

**第三層：Webhook 冪等性保護（輔助防線）**

- 位置：`PayUniWebhookConsumer` 冪等檢查 + `markAsProcessed()`
- 機制：
  1. Consumer 接收 MQ 事件後先查詢 `existsProcessed(webhookType, idempotencyKey)` — 已處理直接 ACK 跳過
  2. Inbox 記錄初始 `is_processed = FALSE`、`process_status = '8201'`（`WebhookProcessStatus.PENDING` 的 CodeMeta value）
  3. Handler 執行成功後，在同一 Transaction 內呼叫 `markAsProcessed(webhookId, processStatus)` — 設為 `is_processed = TRUE`
  4. 唯一索引 `UNIQUE (webhook_type, idempotency_key) WHERE is_processed = TRUE` 阻擋重複處理
- 效果：PayUni 重送相同 Webhook 時（經由 webhook-service 再次投遞），第二次被冪等性檢查或唯一索引攔截，不會重複更新狀態或重複發送 MQ 事件

**三層防禦總覽：**

| 層級 | 防禦位置 | 防禦對象 | 機制 |
|------|----------|----------|------|
| 第一層 | Application | 前端重複提交 | `findActiveByOrderId` 冪等查詢 |
| 第二層 | Database | 競態條件 (Race Condition) | Partial Unique Index |
| 第三層 | MQ Consumer | 重複投遞 / PayUni 重送 | `is_processed` + 唯一索引 |

**gRPC 錯誤回傳（強制）**：

重複付款攔截後，gRPC 層必須回傳明確的業務錯誤碼，禁止回傳泛用 5000：

| 攔截層 | 異常類型 | gRPC 回傳碼 | 回傳訊息 |
|--------|---------|------------|---------|
| 應用層（PAID 狀態） | `IllegalStateException` | `409 CONFLICT` | `重複付款` |
| DB 層（唯一約束） | `DataIntegrityViolationException` | `409 CONFLICT` | `重複付款` |
| 應用層（訂單狀態不允許） | `IllegalStateException` | `INVALID_PARAM` | 原始訊息 |

---

## 2A. 金額/狀態防護（建立付款的入口檢查）

> 本章節集中所有「createPayment 入口」的防護規則。實作位置：`PaymentApplicationService.createPayment()`。
> 子章節執行順序：**§2A.1 → §2A.2 → §2A.3 → §2A.4 → §2A.5 → §2A.6**（任一層擋下即拒絕，不繼續往下）。
>
> §2A.7「訂單付款方式快照同步」是顯示層規範，由 §2A.3/§2A.5/§2A.6 各自觸發，不在執行順序內。

### 2A.1 零元訂單政策

#### 2A.1.1 政策

訂單套用 100% 折抵券、滿額禮或全額折抵活動後，`totalActualAmount` 可能為 0；而 PayUni **不收 0 元交易**（最低金額見 §2A.2）。

零元訂單**不呼叫 PayUni**：由 payment-service 直接寫一筆 `payment_type='2900'（ZERO_AMOUNT）` 紀錄並標記 `status='1'（PAID）`，走原本 PAID 後續流程（出貨／核銷券／扣庫存）。判斷與寫入都在 payment-service，前端流程不變（仍是「建立訂單 → 打 createPayment」兩步）。

#### 2A.1.2 觸發條件

`createPayment` 入口取得 order 後：

```
order.totalActualAmount == 0
```

**邊界**：`< 0` 由 §2A.2 攔截（負值視為非法金額）。

#### 2A.1.3 業務規則

| 步驟 | 動作 |
|------|------|
| 1 | `createPayment` 取得訂單後檢查 `totalActualAmount == 0` |
| 2 | **跳過所有後續金額/狀態防護**（§2A.2~§2A.6 不適用） |
| 3 | 寫一筆 `payment_transactions` 紀錄：`payment_type = '2900'（ZERO_AMOUNT）`、`provider = INTERNAL`、`status = '1'（PAID）`、`amount = 0` |
| 4 | **不呼叫任何 PayUni API**，`provider_trade_no = NULL` |
| 5 | 發布 `payment.paid` event，下游消費者（出貨、核銷券、扣庫存）行為一致，不需感知金額為 0 |
| 6 | 回傳 createPayment response，`status = PAID`、`amount = 0`、無 redirectUrl、無 ATM 虛擬帳號 |

🔴 **下游必須接受 `amount == 0`（跨服務契約）**：order-service `PaymentEventHandler.handlePaymentSuccess` 的金額守衛必須是「`amount < 0` 一律拒、`amount == 0` 僅允許 `paymentType == ZERO_AMOUNT('2900')`」。寫成 `amount <= 0` 就會把零元 `payment.success` 整個拒掉，而 `orders.status` 會**永遠停在 `PENDING_PAYMENT`**，且事件已被 ACK，不會有任何重試。

#### 2A.1.4 退款規則

🔴 **零元訂單沒有退款概念**：沒收錢 → 不需退錢 → **`payment_refunds` 不得為 ZERO_AMOUNT 建任何 row**（`chk_refund_amount_positive` 也會擋下 `amount = 0` 的 INSERT）。

四層防護：

| 層 | 行為 |
|----|------|
| **aftersales-service `AfterSaleResolutionService.resolve`**（業務入口） | 賣家對零元訂單發起售後決議時，若 `resolutionType ∈ {RETURN_REFUND, PARTIAL_REFUND, FULL_REFUND_NO_RETURN}` → 拋 `AFTERSALE_RESOLUTION_NOT_ALLOWED_FOR_ZERO_AMOUNT`（9022）。零元訂單僅允許 `COUPON_COMPENSATION`（補一張新券）、`REJECT`（拒絕）。**為何禁止 RETURN_REFUND**：售後 `refund.completed` event 在 order-service 會把訂單狀態誤標 `PARTIALLY_REFUNDED` / `FULLY_REFUNDED` 並寫 refund financial_log，但**不歸還優惠券**（既有設計）；對沒收錢的零元訂單而言，「退貨退款」變成「買家退商品卻沒拿回券」，業務上不公平。正確路徑是賣家發 `COUPON_COMPENSATION` 補一張新券。 |
| **aftersales-service `RefundTaskWorker.execute`**（兜底短路） | 撈起 task 後判斷：若 `task.paymentMethod == ZERO_AMOUNT` → **跳過 `paymentGrpcClient.createRefund` 呼叫**，直接走「成功路徑」（task 標 DONE / `closeParentEntity` / `publishRefundCompletedEvent` / audit log）。等同「零元退款 step 自動完成」，case state machine 不變。此處覆蓋所有 `RETURN_REFUND` 退貨完成、`PlatformDecisionService` 仲裁 FULL/PARTIAL_REFUND 等可能誤觸發的場景。 |
| **payment-service `RefundApplicationService.refund`**（契約防禦） | 入口判斷：若 `transaction.paymentType == ZERO_AMOUNT` → 拋 `PAYMENT_ZERO_AMOUNT_NOT_REFUNDABLE`（8027）。即使上游所有層都漏判，到此一律擋下。 |
| **order-service `PaymentEventHandler.handleCancelledOrderPayment`** | 雖然零元訂單直接 PAID 不會走「已 CANCELLED + 收到 payment 成功」分支，仍對 ZERO_AMOUNT 加防禦 if + warn log，避免任何 race 或 admin 手動操作意外觸發退款。 |

🔴 **攔截點必須在 `RefundTaskWorker`，不得下放到各 `createRefundTask` 呼叫處**：所有 aftersales 退款路徑（`AfterSaleResolutionService` / `PlatformDecisionService` / `ReturnReceiptService` / `SellerCancelService`）最後都收斂到 `RefundTaskWorker` 撈起 task → `paymentGrpcClient.createRefund`。在 Worker 一處攔截即涵蓋全部路徑；分散到各呼叫處則新增一條路徑就會漏一次，而漏掉的症狀是零元訂單去打 PayUni 退款。task entity 已有 `paymentMethod` 欄位，判斷不需新增 schema。

**照常建 task 再於 Worker 短路**（而非上游不建 task）：case state machine 對零元退款透明，等同「refund step 即時完成」，各 resolution type 不需要各自新增「跳過 refund 直接終止 case」分支。

**對買家/賣家 UI 行為**：零元訂單發起售後/取消 → case 流程完整跑完（含 RESOLVED / REFUND_COMPLETED）→ 但無 PayUni 互動、無 `payment_refunds` row。下游 `refund.completed` event 仍正常發出（券恢復、庫存恢復、訂單狀態更新一致）。

#### 2A.1.5 Reconcile 排除規則

[`PendingPaymentReconcileService`](services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileService.java) 在執行 PayUni query API 對帳時，**必須排除** `payment_type = '2900'`（ZERO_AMOUNT） 的紀錄：

- 對應的 SQL `findReconcileCandidates` 加條件 `payment_type != '2900'`（ZERO_AMOUNT）
- 理由：這類紀錄沒有 `provider_trade_no`，呼叫 PayUni query 會 400 錯誤

#### 2A.1.6 流程圖

```mermaid
flowchart TD
    A[createPayment 入口] --> B[查詢訂單]
    B --> C{totalActualAmount == 0?}
    C -->|否| D[繼續 §2A.2 最低金額檢查]
    C -->|是| E[寫 payment_transactions<br/>method=ZERO_AMOUNT<br/>status=PAID]
    E --> F[發 payment.paid event]
    F --> G[回傳 status=PAID, amount=0]
    G --> H[下游：出貨/核銷券/扣庫存]
```

#### 2A.1.7 時序圖

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端
    participant GW as Gateway
    participant PAY as PaymentService
    participant DB as payment_transactions
    participant MQ as RabbitMQ
    participant DOWN as 下游消費者

    FE->>GW: POST /app/payments<br/>{orderNo, paymentType=2001}
    GW->>PAY: gRPC CreatePayment
    PAY->>PAY: 取訂單 → totalActualAmount=0
    PAY->>DB: @Transactional 內原子（收斂點 B）：<br/>INSERT method=ZERO_AMOUNT, status=PAID, amount=0<br/>+ PaymentOutboxWriter.enqueueAndDispatch → INSERT outbox_events(payment.success, PENDING)<br/>+ 註冊 afterCommit
    PAY->>MQ: afterCommit 即時 publishConfirmed（失敗交排程器每 5s 兜底）payment.success
    PAY-->>GW: status=PAID, amount=0
    GW-->>FE: 200 {status: PAID}
    MQ->>DOWN: payment.success event（trade_no dedup）
    DOWN->>DOWN: 出貨 / 核銷券 / 扣庫存
```

#### 2A.1.8 與其他章節耦合說明

- 與**優惠券核銷**（coupon-service）：`payment.paid` event 觸發核銷，coupon-service 不需感知金額為 0
- 與**庫存扣減**：訂單建立時已扣庫存（PENDING_PAYMENT 階段），與付款金額無關
- 與**對帳**：reconcile 排除（§2A.1.5），財務報表查詢時可依 `payment_type` 過濾

---

### 2A.2 付款方式金額範圍政策

#### 2A.2.1 金額上下限與雙層攔截

PayUni 對各付款方式有**硬性的上下限**：

| 付款方式 | PayUni 限制 | 來源 |
|---------|------------|------|
| 信用卡（CREDIT_CARD） | 1 ~ 199,999 元 | PayUni 業務確認；超範圍 PayUni 直接拒絕 |
| Apple Pay（APPLE_PAY） | 1 ~ 199,999 元 | PayUni 官方文件 §4 未明列；底層 `PaymentType=1` 走信用卡通道（見 PayUni 金流官方 API 總覽 §4.5 `PaymentType: 1=信用卡`），保守套用信用卡限制 |
| Google Pay（GOOGLE_PAY） | 1 ~ 199,999 元 | 同上 |
| ATM 轉帳（ATM） | 15 ~ 49,999 元 | PayUni 業務確認；低於下限時 PayUni 回 `ATM02007`「訂單金額，不為允許範圍區間」 |

不在我方擋下，PayUni 的技術錯誤會直接暴露給使用者。

🔴 **禁止用單一全局最低金額同時擋兩層**：那會讓 L1 在還不知道 paymentMethod 的情況下擋掉 1~14 元訂單（使用者選信用卡本來付得了），同時完全沒有上限檢核。

**雙層攔截，按是否已知 paymentMethod 分工**：

| 層 | 位置 | 已知 paymentMethod？ | 行為 |
|----|------|------------------|------|
| **L1** | order-service `CheckoutService.confirm()` | ❌ 否（訂單建立時尚未選付款方式） | 用「最寬鬆上限」`ORDER_MAX_AMOUNT=199_999` 早期擋下不可能成立的訂單 |
| **L2** | payment-service `PaymentApplicationService.createPayment()` | ✅ 是 | 按 paymentMethod 精準上下限檢核 |

> **零元例外**：金額 = 0 由 §2A.1 處理，**不**進入本上下限規範。L2 在 §2A.1 零元分支之後檢核。

#### 2A.2.2 觸發條件

| 層 | 條件 |
|----|------|
| L1 | `totalActualAmount > PaymentMethodLimitRules.ORDER_MAX_AMOUNT` (199,999) |
| L2 | `amount > 0` 且 `(amount < limits.min OR amount > limits.max)`，其中 `limits = PaymentMethodLimitRules.limits(paymentMethod)` |

#### 2A.2.3 業務規則

##### L1（order-service `CheckoutService.confirm`）

```
if (totalActualAmount > ORDER_MAX_AMOUNT) {
    throw OrderBusinessException(ORDER_AMOUNT_EXCEEDS_LIMIT, totalActualAmount, ORDER_MAX_AMOUNT);
}
// 不檢查下限：下限因 paymentMethod 而異（信用卡 1 元、ATM 15 元），訂單建立時無法判定，留給 L2。
```

**錯誤訊息**：`訂單金額過高（X 元），超出系統支援上限（199,999 元），請分批下單或聯繫客服`
**ObsLogger event**：`CheckoutService.confirm.amountExceedsLimit`

##### L2（payment-service `PaymentApplicationService.createPayment`）

順序：(1) 零元分支 → (2) 範圍檢核 → (3) 切換規則。

```
if (amount == 0) { handleZeroAmount(...); return; }   // §2A.1
PaymentMethodLimitRules.validateOrThrow(paymentMethod, amount);
// 不通過 → 拋 PaymentBusinessException(PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE, label, amount, min, max)
// 通過 → 進入 §2A.3 切換規則
```

**錯誤訊息**（例）：`所選付款方式不支援此訂單金額：付款方式=ATM 轉帳、訂單金額=50000 元、允許範圍=15~49999 元`
**ObsLogger event**：`PaymentApplicationService.createPayment.amountOutOfRange`

##### 觀測（如何辨識是哪層擋下）

- **OTel span name**：order-service = `OrderService.CheckoutConfirm`，payment-service = `PaymentService.CreatePayment`
- **錯誤碼分流**：L1 用 `ORDER_AMOUNT_EXCEEDS_LIMIT`（8026），L2 用 `PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE`（8021）
- **traceId**：客服收到使用者回報的 traceId → 查 trace → 自動定位

#### 2A.2.4 規則來源（PaymentMethodLimitRules helper）

依 [Java 微服務開發規範 §6](../開發規範/Java%20微服務開發規範.md) 與 [代碼管理（CodeMeta）](../代碼管理（CodeMeta）/README.md)「禁止 CodeMeta enum 內含自定義方法區塊」，**規則資料不放 PaymentMethod enum，改放獨立 helper class**：

**位置**：`services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/helper/PaymentMethodLimitRules.java`

**理由**：order-service 與 payment-service 兩處共用 → 依 [Java 微服務開發規範 §15.2](../開發規範/Java%20微服務開發規範.md)「跨服務重複必須抽取」放 common-core。

**規則資料**：

| PaymentMethod | min | max | 出處 |
|---------------|-----|-----|------|
| CREDIT_CARD | 1 | 199,999 | PayUni 業務 |
| APPLE_PAY | 1 | 199,999 | PayUni 文件未列；底層信用卡通道，保守同信用卡 |
| GOOGLE_PAY | 1 | 199,999 | 同上 |
| ATM | 15 | 49,999 | PayUni 業務確認（超出時回 `ATM02007`） |
| ZERO_AMOUNT | — | — | 不適用（呼叫端 amount==0 路徑跳過） |

**公開 API**：

```java
public final class PaymentMethodLimitRules {
    public static final long ORDER_MAX_AMOUNT = 199_999L;  // L1 用，= 所有 method max 中最大值
    public record AmountRange(long min, long max) {}
    public static AmountRange limits(PaymentMethod method);  // ZERO_AMOUNT 拋 IllegalArgumentException
    public static boolean isWithinRange(PaymentMethod method, long amount);
}
```

🔴 **上下限值寫在 helper 常數，不做成可配置參數**：它們是 PayUni 的外部 contract、不是業務參數；做成 env var 等於讓任何人改一個環境變數就能繞過 PayUni 的硬限制，而症狀會出現在 PayUni 端。PayUni 公告變更時改 helper 一處並走正常 PR review。

#### 2A.2.5 流程圖

```mermaid
flowchart TD
    subgraph L1 [L1: order-service 訂單建立]
        A1[CheckoutService.confirm] --> A2{totalActualAmount > 199_999?}
        A2 -->|是| A3[拒絕建單<br/>ORDER_AMOUNT_EXCEEDS_LIMIT 8026]
        A2 -->|否| A4[建立訂單成功<br/>0 ≤ amount ≤ 199_999]
    end
    subgraph L2 [L2: payment-service 付款建立]
        B1[createPayment 入口] --> B2{amount == 0?}
        B2 -->|是| B3[§2A.1 零元路徑<br/>標 PAID 不打 PayUni]
        B2 -->|否| B4[PaymentMethodLimitRules<br/>.validateOrThrow]
        B4 --> B5{在範圍內?}
        B5 -->|否| B6[拒絕付款<br/>PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE 8021]
        B5 -->|是| B7[繼續 §2A.3 切換規則]
    end
    A4 -.後續打 createPayment.-> B1
```

#### 2A.2.6 PayUni 真實錯誤對照

| PayUni Status | PayUni Message | 我方錯誤碼 | 我方訊息 |
|---|---|---|---|
| `ATM02007` | 訂單金額，不為允許範圍區間 | `PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE` (8021) | 所選付款方式不支援此訂單金額：付款方式=ATM 轉帳、訂單金額=14 元、允許範圍=15~49999 元 |
| （信用卡上限超出）| PayUni 拒絕請求 | `PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE` (8021) | 所選付款方式不支援此訂單金額：付款方式=信用卡、訂單金額=250000 元、允許範圍=1~199999 元 |
| （訂單金額過高，L1 提前擋下）| — | `ORDER_AMOUNT_EXCEEDS_LIMIT` (8026) | 訂單金額過高（250000 元），超出系統支援上限（199999 元），請分批下單或聯繫客服 |

> 錯誤碼 8021 的語意是 `PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE`（值域 SSOT 見 [錯誤碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/錯誤碼.md)）。

#### 2A.2.7 前端付款方式 gating（availablePaymentMethods，後端驅動）

§2A.2 的上下限是「**寫入/付款時的後端權威防線（L1 8026 / L2 8021）**」。為避免前端讓使用者選到必然被拒的方式，同一套限制值（`PaymentMethodLimitRules`）另外對前端**提早回饋可用性**：

- 於 `PaymentMethodLimitRules` 新增純函式 `resolveAvailability(amount)`，依各方式上下限算出 `List<{paymentType, eligible, ineligibleReason}>`（理由碼 `PaymentMethodIneligibleReason`：`AMOUNT_BELOW_MIN` / `AMOUNT_ABOVE_MAX`）。**限制值 SSOT 不變**，仍只在此 helper。
- 消費者為 **order-service**（非 payment-service）：結帳預覽 `CheckoutPreviewData.available_payment_methods` 與訂單詳情 `OrderDetailView.available_payment_methods`（再次付款）。規格見 [20-結帳模組設計書 §3.4.1](../詳細設計書/20-結帳模組（Checkout%20Module）詳細設計書.md) 與 [03-訂單微服務設計書 §4.2.1](../詳細設計書/03-訂單微服務（Order%20Service）詳細設計書.md)。
- **分層**：前端 gating 純 UX；本服務 L2（8021）+ order L1（8026）仍為最終權威，前端不得因有 availability 而移除 8021/8026 處理。

---

### 2A.3 paymentType 切換規則

> 本章節定義付款方式切換與重試的核心規則。切換/重試**立即生效**（PayUni 端 10 分鐘 MerTradeNo 限制由獨立 MerTradeNo 序列化機制規避，詳 §2A.3.1）。

#### 2A.3.1 PayUni 硬性約束（必讀）

設計本章節前必須先理解 PayUni 的三條無法改變的約束：

| # | 約束 | 來源 / 證據 |
|---|------|-----------|
| C1 | **同一 MerTradeNo 在 10 分鐘內只能建立一次** PayUni 交易 | PayUni 後端限制（業務確認）|
| C2 | **PayUni 端已建立的交易無法主動關閉**（特別是 ATM 虛擬帳號）| 沒有對應 close API |
| C3 | **PayUni 不阻擋重複付款** — 同一筆訂單可同時存在多個有效付款管道（例：ATM 虛擬帳號 + 信用卡），且 PayUni 兩個都會收 | PayUni 行為 |

這三條約束 → 推導出本章節（切換規則）+ §2A.6（重複付款處理）兩者的必要性。

> **MerTradeNo 序列化（C1 對策）**：MerTradeNo = `{orderNo}{3位序號}`（如 `O382716459001`），每次付款嘗試使用獨立 MerTradeNo → PayUni 視為獨立交易 → C1 約束不生效，切換/重試立即執行，無冷卻。**禁止「簡化」回 MerTradeNo = orderNo**，否則同訂單 10 分鐘內的切換/重試會被 PayUni 擋下。C2、C3 約束不受 MerTradeNo 序列化影響，§2A.5 ATM 復活機制與 §2A.6 重複付款處理仍為必要防護。

#### 2A.3.3 業務規則

> **核心**：每次付款嘗試使用獨立 MerTradeNo（`{orderNo}{3位序號}`），不受 PayUni 10 分鐘限制，切換/重試**立即生效**。

##### 同類型重試規則

| paymentType | 既有 INIT row 狀態 | 行為 |
|-------------|-------------------|------|
| **信用卡** | INIT 且 `expire_at > nowUtc`（< 10 分鐘） | 回 cached redirect URL（冪等，不打 PayUni） |
| **信用卡** | INIT 且 `expire_at ≤ nowUtc`（≥ 10 分鐘） | 標 EXPIRED → 建新交易（新 MerTradeNo） |
| **信用卡** | FAILED（終態） | 建新交易（新 MerTradeNo） |
| **Apple Pay / Google Pay** | FAILED（終態，同步立即確定） | 建新交易（新 MerTradeNo） |
| **Apple Pay / Google Pay** | INIT 不應存在（同步呼叫） | 若存在且 `expire_at ≤ nowUtc` → 標 EXPIRED → 新建；否則回 cached |
| **ATM** | INIT 且 `expire_at > nowUtc`（未過期） | 回 cached 虛擬帳號（冪等，不打 PayUni） |
| **ATM** | INIT 且 `expire_at ≤ nowUtc`（已過期） | 標 EXPIRED → 建新交易（新 MerTradeNo） |
| **任何** | PAID | 拒絕（`PAYMENT_ALREADY_EXISTS`） |

##### 不同類型切換規則

| 切換方向 | 行為 | 說明 |
|---------|------|------|
| **ATM → 信用卡** | 立即：(1) UPDATE ATM row `status=CLOSED_BY_SWITCH` (2) 依信用卡同類型規則處理 | 遵守信用卡重試規則：若已有未過期 CC INIT 回 cached |
| **信用卡 → ATM** | 立即：(1) UPDATE CC row `status=CLOSED_BY_SWITCH` (2) 嘗試 §2A.5.4 ATM 復活 → 復活失敗則新建 ATM | CC 頁面 10 分鐘自然失效；若使用者在此期間完成 CC 付款 → §2A.6 兜底 |
| **信用卡 → 信用卡** | 遵守信用卡同類型規則 | 未過期回原頁面；過期建新 |
| **ATM → ATM** | 遵守 ATM 同類型規則 | 未過期回原帳號；過期建新 |
| **任何 → Apple/Google Pay** | 立即：(1) CLOSE 舊 row (2) 建新交易 → 同步呼叫 PayUni | 結果立即確定（PAID 或 FAILED） |
| **Apple/Google Pay → 任何** | FAILED 為終態 → 無活躍 row → 直接建新 | 不需 CLOSE（已是終態） |

> **冪等鍵**：findActive 查詢使用 **(orderId, paymentType)** 兩欄複合鍵。
>
> **過期檢查嵌入位置（critical）**：§2A.5 的 L2 即時過期兜底**內嵌在同類型重試「找到 row 之後」**，而非獨立步驟。
>
> **信用卡 `expire_at` 設定**：建立信用卡交易時設 `expire_at = nowUtc() + 600s`，同時傳 `TradeLExpireSec=600` 給 PayUni，確保我方與 PayUni 頁面過期時間**完全同步**。
>
> **重複付款防護**：切換後舊 PayUni 交易仍存活（C2 約束），若使用者在舊管道完成付款 → §2A.6 重複付款處理自動偵測並退款。

#### 2A.3.4 新狀態：CLOSED_BY_SWITCH

| 項目 | 值 |
|------|---|
| Enum 名稱 | `CLOSED_BY_SWITCH` |
| Code 值 | `6`（既有 PaymentTransactionStatus 用單字元編碼 0~5，下一個為 6） |
| 中文 label | 「因切換付款方式而關閉」 |
| 與既有 `CLOSED` 的差別 | `CLOSED` = 一般關閉（含過期、人工關閉等）；`CLOSED_BY_SWITCH` = 使用者主動切換付款方式造成的關閉，**保留審計軌跡** |
| 是否視為終態 | 是（不可再轉回 INIT/PAID） |
| 是否被 partial unique index 排除 | 是（不在 `WHERE status IN (INIT, PAID)` 條件內，新 row 可正常 INSERT） |

#### 2A.3.5 切換後 PayUni 端的副作用（核心設計考量）

切換 paymentType 後，**PayUni 端舊交易仍存活**（C2 約束）。這帶來的後果：

```mermaid
flowchart TD
    A[使用者選 ATM<br/>取得虛擬帳號 A] --> B[10 分鐘後<br/>切換信用卡]
    B --> C[新建信用卡交易<br/>送 PayUni]
    C --> D[使用者完成信用卡付款<br/>PAID]
    D --> E{使用者後續行為?}
    E -->|誠實使用者| F[沒事，<br/>虛擬帳號 A 自然過期]
    E -->|惡意 / 失誤使用者| G[拿虛擬帳號 A<br/>到 ATM 轉帳]
    G --> H[第二筆 PAID<br/>重複付款]
    H --> I[§2A.6 重複付款處理<br/>自動偵測 + 退款]
```

**這是為何 §2A.6 重複付款處理是必要功能**，不是 nice-to-have。即便我方設計了切換規則，只要 PayUni 端舊交易還活著，重複付款就有機會發生 — 必須有 §2A.6 兜底。

> 🔴 **不主動關閉 PayUni 端的舊交易**：ATM 沒有關閉 API（C2 約束），而「切換時嘗試 cancel 舊授權」會新增一條失敗路徑（cancel 失敗要怎麼辦），卻不免除 §2A.6 兜底。⇒ 舊交易一律讓它自然到期，重複付款由 §2A.6 處理。

#### 2A.3.6 資料模型

> **核心**：`trade_no` ＝ `{order_id}{3位序號}`。每次付款嘗試產生獨立 `trade_no`，送 PayUni 作為 `MerTradeNo`。

##### 三個欄位的角色

| 欄位 | 來源 | 格式 | 用途 |
|------|------|------|------|
| `id` (PK, BIGINT) | TSID | `177500001` | DB 主鍵，內部用 |
| `trade_no` (VARCHAR 32) | **`{order_id}{attempt_seq:03d}`** | `O038032821001` | 我方對外識別碼 + **送 PayUni 當 MerTradeNo** + gRPC path param |
| `order_id` (VARCHAR 64) | Order Service | `O038032821` | 原始訂單編號，**所有 row 共用** |
| `attempt_seq` (SMALLINT) | 遞增序號 | `1`, `2`, `3`... | 同一訂單的第 N 次付款嘗試（從 1 開始） |
| `provider_trade_no` (VARCHAR 64) | PayUni 回傳 | `1775567114497535288` | PayUni 內部 serial，退款/查詢用 |

**`trade_no` 生成邏輯**：
```java
int nextSeq = transactionRepository.findMaxAttemptSeqByOrderId(orderId)
    .map(seq -> seq + 1)
    .orElse(1);
String tradeNo = orderId + String.format("%03d", nextSeq);
// O038032821 + 001 = O038032821001
```

**MerTradeNo 長度驗證**：`orderNo`（10 字元）+ 序號（3 字元）= 13 字元 ≤ PayUni 限制 25 字元 ✓

##### 同一 `order_id` 多筆 row 範例

| row | id | trade_no | order_id | attempt_seq | payment_type | status | provider_trade_no | MerTradeNo 送 PayUni |
|-----|-----|----------|----------|------------|--------------|--------|-------------------|---------------------|
| 1 | 177500001 | `O038032821001` | O038032821 | 1 | ATM | CLOSED_BY_SWITCH | `1775..001` | `O038032821001` |
| 2 | 177500002 | `O038032821002` | O038032821 | 2 | CREDIT_CARD | PAID | `1775..535` | `O038032821002` |

**每筆 row 的 `trade_no` 不同**（序號遞增），PayUni 視為獨立交易 → C1 約束不生效 → 無需等待 10 分鐘。

**對賣家通知信的影響**：PayUni 通知信中 MerTradeNo 為 `O038032821002`。賣家以訂單編號 `O038032821` 搜尋仍可匹配，因為 **MerTradeNo 的前 10 碼恆等於 orderNo**（見下方 `extractOrderId` 的前提）——prefix 搜尋成立靠的是這個不變式，不是巧合。

##### attempt_seq 併發安全

兩筆併發 CreatePayment 可能同時讀到 `MAX(attempt_seq)=1`，各自嘗試建 `trade_no=O123002`。

**防護機制（三層）**：

| 層級 | 機制 | 效果 |
|------|------|------|
| L1：Partial unique index | `UNIQUE(order_id) WHERE status IN (INIT, PAID)` | 同一訂單同時只能有一筆活躍 row → 第二筆 INSERT 觸發 `DuplicateKeyException` |
| L2：trade_no UNIQUE | `UNIQUE(trade_no)` | 即使 L1 未攔截（不同狀態），同 trade_no 也會被擋 |
| L3：應用層重試 | catch `DuplicateKeyException` → 重新查詢 → 回 cached（冪等） | 使用者無感知，不報錯 |

**L1 是主力**：正常流程中，CreatePayment 只在「無活躍 row」或「活躍 row 已過期標 EXPIRED」時才 INSERT。L1 保證兩筆併發 INSERT 只有一筆成功。

🔴 **`findMaxAttemptSeqByOrderId` 不得加 `SELECT ... FOR UPDATE`**：它是跨越同一 `order_id` **全部歷史 row** 的查詢，加行鎖等於鎖住該訂單的所有付款紀錄。序號碰撞由上表 L1/L2 的唯一約束擋下、L3 轉成冪等回應即可。

##### extractOrderId 規格

從 `MerTradeNo` 提取 `order_id` 的工具方法。**不能硬編碼「去掉末 3 碼」**，因為 attempt_seq ≥ 1000 時序號為 4 位。

```java
/**
 * 從 MerTradeNo（trade_no）提取 order_id。
 *
 * 策略：order_id 格式固定為 O{9位數字}（10字元），
 * trade_no = order_id + attempt_seq（3~N 位數字）。
 * 直接取前 10 字元即可。
 *
 * MerTradeNo 長度必須 > 10（orderNo 10 字元 + 序號 3 位以上），否則拋例外。
 */
public static String extractOrderId(String merTradeNo) {
    if (merTradeNo == null || merTradeNo.length() < 10) {
        throw new IllegalArgumentException("Invalid MerTradeNo: " + merTradeNo);
    }
    return merTradeNo.substring(0, 10);  // O{9位數字} = 固定 10 字元
}
```

> **前提**：此方法依賴 `orderNo` 格式固定為 `O{9位數字}`（10 字元），由 `OrderNoGenerator` 保證。若 orderNo 格式變更，此方法必須同步調整。

##### Schema 契約（本節依賴的不變式）

| 項目 | 契約 |
|--------|------|
| `trade_no` 內容 | `order_id + attempt_seq`（序號至少 3 位） |
| `trade_no` UNIQUE | **必須維持**。每次嘗試的 trade_no 不同，唯一性成立；且 §3.3 的退款總額 trigger 依賴它取 `FOR UPDATE` 鎖 |
| `attempt_seq` | `SMALLINT NOT NULL DEFAULT 1` |
| `provider_trade_no` | partial unique（`WHERE provider_trade_no IS NOT NULL`，允許多筆 NULL） |
| `idx_payment_transactions_order_active` | `UNIQUE(order_id) WHERE status IN ('0','1') AND is_duplicate = FALSE`，基於 `order_id` 而非 `trade_no`，是整個追蹤機制的安全閥 |
| `expire_at` 用途 | **所有付款方式都設定**（信用卡 `nowUtc() + 600s`，對應 PayUni `TradeLExpireSec=600`；ATM 取 PayUni `ExpireDate`） |
| `chk_status` | 值域含 `6`（CLOSED_BY_SWITCH） |

##### gRPC API 語意

**`/app/payments/{trade_no}` 語意**：

- `trade_no` 現在唯一識別一筆 payment attempt（UNIQUE constraint 保證）
- 直接 `WHERE trade_no = ?` 即可，**不需要「查最新活躍」的曖昧邏輯**
- **新增 API**：`GET /app/payments/orders/{order_id}/history` 依 `order_id` 列出該訂單所有付款嘗試（參數為 `order_id`，非 `trade_no`，避免語義混淆）

##### Repository 介面

| 方法 | 變更 |
|------|------|
| `findByTradeNo(String tradeNo)` | 精準查找（trade_no UNIQUE），回 `Optional<PaymentTransaction>` |
| `findActiveByOrderId(String orderId)` | partial unique index 保證最多一筆 |
| `findActiveByOrderIdAndPaymentType(String orderId, PaymentMethod method)` | 同類型重試偵測用 |
| `findHistoryByOrderId(String orderId)` | 回 `List<PaymentTransaction>`，依 created_at DESC |
| `findByProviderTradeNo(String providerTradeNo)` | webhook handler 段 1 精準追蹤 |
| `findMaxAttemptSeqByOrderId(String orderId)` | 取目前最大序號，用於生成下一個 trade_no |

##### Webhook 追蹤機制（核心）

**Webhook 帶的關鍵欄位**：

| Webhook 欄位 | 值 | 對應我方欄位 |
|--------------|---|------------|
| `MerTradeNo` | `O038032821001`（= trade_no） | `payment_transactions.trade_no`（**直接匹配**，UNIQUE） |
| `TradeNo` | PayUni 內部 serial | `payment_transactions.provider_trade_no` |

**追蹤策略：三段查找 + 兜底**

```
段 1：provider_trade_no 精準查找（最常見路徑）
  findByProviderTradeNo(webhook.TradeNo)
  → 找到 → 檢查 row 狀態：
     a. row 是 INIT/PAID（非終態）→ 處理該 row
     b. row 是 CLOSED_BY_SWITCH/EXPIRED/CLOSED 終態 + webhook 帶 PAID
        → §2A.6 入口（切換後使用者用舊管道完成付款）
  → 找不到 → 進入段 1.5

段 1.5（新增）：trade_no 精準查找
  findByTradeNo(webhook.MerTradeNo)
  → 找到 → 用 trade_no（UNIQUE）精準鎖定該筆 row
  → 處理該 row，補登 provider_trade_no（信用卡 UPP 首次 webhook）
  → 找不到 → 進入段 2

段 2：order_id 活躍查找（兜底）
  extractOrderId(webhook.MerTradeNo)  // 取前 10 字元（O + 9位數字）
  findActiveByOrderId(orderId)
  → 找到 → 異常路徑：MerTradeNo 不匹配活躍 row 的 trade_no → 警報
  → 找不到 → 進入段 3

段 3：重複付款偵測（§2A.6 入口）
  findHistoryByOrderId(orderId)
  → 找到歷史 PAID row → §2A.6 重複付款處理
  → 找不到任何 row → 拋警報
```

多數信用卡 webhook 命中**段 1.5**：`trade_no` UNIQUE 保證唯一，不必依賴 partial unique index 的間接保證。

🔴 **`PaymentTransactionStatus.isTerminal()` 必須包含 `CLOSED_BY_SWITCH`**：切換場景下 row 會停在 CLOSED_BY_SWITCH，webhook 帶 PAID 進來時要靠這個判定轉入段 1.b（§2A.6 流程）。漏了它，使用者用舊管道完成的付款會走進「非終態 row 直接處理」的分支而不被視為重複付款。
>
> **ATM reactivate 場景的時間欄位**：ATM webhook payload 不含 AuthDay/AuthTime/PaymentDay，reactivate 後 `provider_paid_at` 維持 NULL，由 reconcile 後續 query API 補登。

##### ATM 取號時必須寫入 provider_trade_no

`PaymentTransactionMapper.updateAtmInfo` SQL 必須包含 `provider_trade_no = #{providerTradeNo}`（PayUni `/api/atm` 同步回應的 TradeNo），否則段 1 精準查找失效。

#### 2A.3.7 流程圖（MerTradeNo 序列化）

```mermaid
flowchart TD
    A[createPayment 入口] --> B[§2A.1 零元檢查]
    B --> C[§2A.2 最低金額檢查]
    C --> D[findActiveByOrderIdAndPaymentType]
    D --> E{找到同 type 的<br/>INIT/PAID 紀錄?}
    E -->|是| EX{expire_at ≤ nowUtc?<br/>已過期?}
    EX -->|是 已過期| EY[標 row.status=EXPIRED<br/>視為無既有 row]
    EX -->|否 仍有效| F[回 cached 冪等回應<br/>不打 PayUni]
    E -->|是 PAID| PAID_REJ[拒絕<br/>PAYMENT_ALREADY_EXISTS]
    EY --> G[findActiveByOrderId]
    E -->|否| G
    G --> H{找到其他 type<br/>的 INIT/PAID?}
    H -->|否| I[新建付款]
    H -->|是 PAID| PAID_REJ
    H -->|是 INIT 不同 type| ATM_CHK{請求=ATM?}
    ATM_CHK -->|是| ATM_REACT[§2A.5.4 ATM 復活嘗試]
    ATM_REACT -->|復活成功| ATM_OK[回原虛擬帳號]
    ATM_REACT -->|復活失敗| SWITCH
    ATM_CHK -->|否| SWITCH[UPDATE 舊 row<br/>status=CLOSED_BY_SWITCH]
    SWITCH --> I
    I --> SEQ[計算 attempt_seq<br/>= MAX + 1]
    SEQ --> TN[trade_no = order_id<br/>+ format 03d seq]
    TN --> N[呼叫 PayUni API<br/>MerTradeNo=trade_no]
    N --> CC_EXP{信用卡?}
    CC_EXP -->|是| CC_SET[設 expire_at<br/>= nowUtc + 600s<br/>傳 TradeLExpireSec=600]
    CC_EXP -->|否 ATM| ATM_SET[expire_at 由<br/>PayUni ExpireDate 設定]
    CC_EXP -->|否 Apple/Google| SYNC[同步結果<br/>直接 PAID 或 FAILED]
    CC_SET --> O[INSERT 新 row + 回應]
    ATM_SET --> O
    SYNC --> O
```

#### 2A.3.8 時序圖：ATM 切換信用卡（立即，無冷卻）

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端
    participant GW as Gateway
    participant PAY as PaymentService
    participant DB as payment_transactions
    participant PU as PayUni

    FE->>GW: 第一次：POST /app/payments<br/>{orderNo:O123, paymentType:ATM}
    GW->>PAY: CreatePayment
    PAY->>DB: findActiveByOrderIdAndPaymentType(O123, ATM) → null
    PAY->>DB: findActiveByOrderId(O123) → null
    PAY->>DB: findMaxAttemptSeqByOrderId(O123) → null
    Note over PAY: attempt_seq=1, trade_no=O123001
    PAY->>PU: /api/atm (MerTradeNo=O123001)
    PU-->>PAY: SUCCESS, TradeNo=1775..001<br/>虛擬帳號 8220...
    PAY->>DB: INSERT id=TSID, trade_no=O123001,<br/>attempt_seq=1, order_id=O123,<br/>provider_trade_no=1775..001,<br/>method=ATM, status=INIT,<br/>expire_at=(PayUni ExpireDate)
    PAY-->>FE: ATM 虛擬帳號

    Note over FE: 使用者立刻改變主意<br/>（不需等待 10 分鐘！）

    FE->>GW: 第二次：POST /app/payments<br/>{orderNo:O123, paymentType:CREDIT_CARD}
    GW->>PAY: CreatePayment
    PAY->>DB: findActiveByOrderIdAndPaymentType(O123, CC) → null
    PAY->>DB: findActiveByOrderId(O123) → row1 (ATM, INIT)
    Note over PAY: 不同 type 切換 → 立即執行（無冷卻）
    PAY->>DB: UPDATE row1 status='6'（CLOSED_BY_SWITCH）
    PAY->>DB: findMaxAttemptSeqByOrderId(O123) → 1
    Note over PAY: attempt_seq=2, trade_no=O123002
    PAY->>PU: /api/upp (MerTradeNo=O123002,<br/>TradeLExpireSec=600)
    Note over PU: 不同 MerTradeNo → 立即接受
    PU-->>PAY: SUCCESS, encryptInfo
    PAY->>DB: INSERT id=TSID(新), trade_no=O123002,<br/>attempt_seq=2, order_id=O123,<br/>method=CC, status=INIT,<br/>expire_at=nowUtc+600s
    PAY-->>FE: redirectUrl

    Note over FE,PU: 使用者完成信用卡付款後<br/>PayUni webhook 帶 MerTradeNo=O123002<br/>段 1.5 用 trade_no 精準查找
```

#### 2A.3.9 時序圖：信用卡付款失敗後立即重試

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端
    participant GW as Gateway
    participant PAY as PaymentService
    participant DB as payment_transactions
    participant PU as PayUni

    FE->>GW: POST /app/payments<br/>{orderNo:O123, paymentType:CC}
    GW->>PAY: CreatePayment
    PAY->>DB: findActiveByOrderIdAndPaymentType(O123, CC) → null
    PAY->>DB: findActiveByOrderId(O123) → null
    Note over PAY: attempt_seq=1, trade_no=O123001
    PAY->>PU: /api/upp (MerTradeNo=O123001,<br/>TradeLExpireSec=600)
    PAY->>DB: INSERT trade_no=O123001, CC, INIT,<br/>expire_at=nowUtc+600s
    PAY-->>FE: redirectUrl (A)

    Note over FE,PU: 使用者在 PayUni 頁面<br/>輸入卡號但付款失敗

    PU-->>PAY: Webhook: MerTradeNo=O123001,<br/>TradeStatus=2 (FAILED)
    PAY->>DB: UPDATE trade_no=O123001<br/>status='2'（FAILED）

    Note over FE: 使用者立刻重試<br/>（不需等待 10 分鐘！）

    FE->>GW: POST /app/payments<br/>{orderNo:O123, paymentType:CC}
    GW->>PAY: CreatePayment
    PAY->>DB: findActiveByOrderIdAndPaymentType(O123, CC) → null<br/>(O123001 是 FAILED，非 INIT/PAID)
    PAY->>DB: findActiveByOrderId(O123) → null
    Note over PAY: attempt_seq=2, trade_no=O123002
    PAY->>PU: /api/upp (MerTradeNo=O123002,<br/>TradeLExpireSec=600)
    Note over PU: 不同 MerTradeNo → 立即接受
    PAY->>DB: INSERT trade_no=O123002, CC, INIT,<br/>expire_at=nowUtc+600s
    PAY-->>FE: redirectUrl (B，新頁面)
```

#### 2A.3.9b 時序圖：信用卡未過期時冪等返回

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端
    participant GW as Gateway
    participant PAY as PaymentService
    participant DB as payment_transactions

    FE->>GW: POST /app/payments<br/>{orderNo:O123, paymentType:CC}
    GW->>PAY: CreatePayment
    PAY->>DB: ... 建立 O123001 CC INIT 成功 ...
    PAY-->>FE: redirectUrl (A)

    Note over FE: 5 分鐘後重複點擊

    FE->>GW: POST /app/payments<br/>{orderNo:O123, paymentType:CC}
    GW->>PAY: CreatePayment
    PAY->>DB: findActiveByOrderIdAndPaymentType(O123, CC)<br/>→ O123001 (INIT, expire_at > now)
    Note over PAY: 未過期 → 回 cached
    PAY-->>FE: redirectUrl (A，同一頁面)
    Note over FE: 不打 PayUni，無新交易
```

---

### 2A.4 信用卡頁面過期機制

> §2A.3.1 的 MerTradeNo 序列化已規避 PayUni C1 限制，業務層面無冷卻機制。本節定義信用卡 UPP 頁面的過期判定，確保我方 `expire_at` 與 PayUni 端時間視窗一致。

#### 2A.4.1 信用卡頁面過期判定

信用卡 UPP 頁面有 PayUni 端的過期時間（由 `TradeLExpireSec` 參數控制）。我方在建立時同步記錄，確保兩端一致。

##### 過期參數

| 項目 | 值 |
|------|---|
| **PayUni 參數** | `TradeLExpireSec=600`（顯式傳入，不依賴預設值） |
| **我方欄位** | `expire_at = nowUtc() + 600s` |
| **同步保證** | 過期時長由我方定義並送出，PayUni 端與我方**完全一致** |
| **常數定義** | `PaymentExpiryConstants.CREDIT_CARD_EXPIRE_SECONDS = 600` |

##### Apple Pay / Google Pay

| 項目 | 說明 |
|------|------|
| 過期概念 | **不適用** — 同步呼叫，結果立即確定（PAID 或 FAILED） |
| `expire_at` | 設為 `nowUtc() + 600s`（防禦性，供排程清理 INIT 殘留用） |
| 失敗後重試 | FAILED 為終態 → 無活躍 row → 立即建新交易 |

##### ATM

| 項目 | 說明 |
|------|------|
| 過期來源 | PayUni `/api/atm` response 的 `ExpireDate`（**唯一真實來源**） |
| `expire_at` | `DateTimeUtils.parsePayUniDateTimeToUtc(response.getExpireDate())` |
| 詳見 | §2A.5（ATM 取號保護） |

##### 過期判定統一邏輯

所有 paymentType 的過期判定使用同一欄位 `expire_at`：

```java
boolean isExpired = expire_at != null && DateTimeUtils.nowUtc().isAfter(expire_at);
```

🔴 **邊界語意：`isAfter` 不含等於** ⇒ `expire_at == now` 判定為**未過期**（回既有交易，不重新建立）。

| paymentType | expire_at 來源 | 典型值 |
|-------------|---------------|--------|
| CREDIT_CARD | `nowUtc() + 600s` | 建立後 10 分鐘 |
| APPLE_PAY / GOOGLE_PAY | `nowUtc() + 600s`（防禦性） | 同上 |
| ATM | PayUni ExpireDate | 建立後 1~180 天 |

---

### 2A.5 ATM 取號保護

#### 2A.5.1 取號原則

🔴 **同一訂單在任一時刻只能有一個有效的 ATM 虛擬帳號。** 使用者拿到虛擬帳號 A 之後，在它過期前再次 createPayment（同 type 重打，或切換流程後再切回 ATM），**不得**讓 PayUni 發新的虛擬帳號 B——兩個有效帳號會讓使用者搞混付哪個，且先付 A 後付 B 就直接構成 §2A.6 的重複付款。

有效與否**一律以 PayUni 回傳的 `ExpireDate` 為單一真實來源**判斷（見 §2A.5.5），未過期前不重新取號。

#### 2A.5.2 業務規則

| 情境 | 處理 |
|------|------|
| **A. 既有 ATM row 在 `expire_at` 之前**（虛擬帳號仍有效） | createPayment 同 type 重打 → 既有冪等回 cached（含原始虛擬帳號）；不打 PayUni |
| **B. 既有 ATM row 已過 `expire_at`**（虛擬帳號已過期） | (a) 該 row 應已被 [ExpiredTransactionScheduler](services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/ExpiredTransactionScheduler.java) 標為 EXPIRED；(b) createPayment 即時再驗一次 `nowUtc > expire_at` 兜底；(c) 標 EXPIRED 後進入新建流程（新 MerTradeNo，不受 C1 限制） |
| **C. 切換到非 ATM 後 PayUni 端虛擬帳號還活著** | 我方無法主動關閉（C2 約束）。等過期或使用者實際付款 → 若使用者付款 → 觸發 §2A.6 重複付款處理 |
| **D. 切換回 ATM**（先 ATM → 切信用卡 → 切回 ATM） | **優先嘗試復活舊 row**（見 §2A.5.4 ATM 復活機制）：若舊 ATM CLOSED_BY_SWITCH row 的 `expire_at > nowUtc`，則復活該 row 並回原虛擬帳號，**不打 PayUni**。若舊 row 已過期或不存在 → 走新建流程（新 MerTradeNo） |

#### 2A.5.3 過期偵測：兩層並存

| 層級 | 機制 | 角色 |
|------|------|------|
| **L1 排程** | `ExpiredTransactionScheduler` 定期掃 `WHERE status='0' AND expire_at < nowUtc()` 標為 EXPIRED | 主路徑，定期清理 |
| **L2 即時** | `createPayment` 入口取出既有 row 後，**檢查 `nowUtc() > expire_at`** | 兜底，避開「排程還沒跑到」的瞬間 |

兩層並存的理由：
- 只靠 L1：排程跑之前的瞬間，使用者重打 createPayment 會被冪等回舊 cached（含已過期虛擬帳號），UX 錯誤
- 只靠 L2：未來如果有 reconcile 或其他路徑需要「該 row 已 EXPIRED」的訊號，沒有 L1 標記就拿不到

#### 2A.5.4 ATM 復活機制（避免 PayUni 端兩個有效虛擬帳號）

> **載體**：`AtmReactivationService.tryReactivate(@Transactional)`，由 `PaymentApplicationService.createPayment` 的切換分支在 `paymentType == ATM` 時跨 bean 呼叫；無可復活 row 才 fall back 到新建路徑（新 MerTradeNo）。依賴三支 mapper SQL：`findActiveInitByOrderIdForUpdate` / `findReactivableAtmRowForUpdate` / `markReactivated`。
>
> 🔴 **隔離級別是 READ_COMMITTED，不要宣告 REPEATABLE_READ**：本方法以 propagation REQUIRED 併入 outer `createPayment` 的既有 transaction，而 Spring 在加入既有交易時**會忽略 inner method 的 isolation override**——寫了也不會生效，只會讓讀的人以為有更強的保證。
>
> **正確性由兩次 `SELECT ... FOR UPDATE` 行鎖保證**：`findActiveInit` 與 `findReactivableAtm` 各自鎖住具體 row，鎖定後其他事務改不動它；在這個 lock-then-modify 模式下，READ_COMMITTED + FOR UPDATE 與 REPEATABLE_READ + FOR UPDATE 在本路徑等價。

##### 動機

依 §2A.5.1 的核心原則「不讓使用者拿到兩個有效的 ATM 虛擬帳號」。但 §2A.3 切換規則允許「先 ATM → 切信用卡 → 切回 ATM」的流程，**舊的 ATM 虛擬帳號仍活在 PayUni 端到 expire_at**（C2 約束）。

如果切回 ATM 時直接走「新建」流程，PayUni 會發新的虛擬帳號 B ⇒ 使用者同時擁有 A 和 B 兩個有效帳號 ⇒ 違反 §2A.5.1 且直接提高重複付款機率。

因此切回 ATM 時**必須優先嘗試復活舊的 ATM CLOSED_BY_SWITCH row**（重用既有虛擬帳號 A），不打 PayUni、不發新帳號。

##### 復活條件

| 條件 | 說明 |
|------|------|
| C1 | `paymentType = ATM` |
| C2 | 同 `order_id` 存在至少一筆 `payment_type='2004' AND status='6' (CLOSED_BY_SWITCH)` 的 row |
| C3 | 該 row 的 `expire_at > nowUtc` |
| C4 | 該 row 的 `provider_trade_no IS NOT NULL`（已成功取過 PayUni 虛擬帳號）|

四個條件**全部成立**才能復活。任一不成立 → 走新建流程（新 MerTradeNo）。

> 若同 order_id 有多筆符合條件的 row（極罕見：使用者反覆切換多次），取 `expire_at DESC LIMIT 1`，即剩餘時間最長的那筆。

##### 復活動作（單一 transaction，READ_COMMITTED + 行鎖）

```sql
-- 邏輯交易範圍示意；實際邊界由 AtmReactivationService 的 @Transactional 決定
BEGIN;

-- 1. 鎖定既有 INIT row（例如使用者目前正在用的信用卡 INIT）
SELECT ... FROM payment_transactions
  WHERE order_id = ? AND status = '0'  -- INIT
  FOR UPDATE;
-- → currentInitRow

-- 2. 鎖定可復活的 ATM CLOSED_BY_SWITCH row（最新 expire_at）
SELECT ... FROM payment_transactions
  WHERE order_id = ?
    AND payment_type = '2004'
    AND status = '6'  -- CLOSED_BY_SWITCH
    AND expire_at > now() at time zone 'utc'
    AND provider_trade_no IS NOT NULL
  ORDER BY expire_at DESC
  LIMIT 1
  FOR UPDATE;
-- → reactivableRow (若無則整個流程退回到「新建」)

-- 3. 翻轉狀態（順序很重要：先讓 INIT 變 CLOSED_BY_SWITCH，騰出 partial unique index 槽位）
UPDATE payment_transactions
   SET status = '6'  -- CLOSED_BY_SWITCH
     , closed_at = now()
 WHERE id = currentInitRow.id;

UPDATE payment_transactions
   SET status = '0'  -- INIT (復活)
     , reactivated_at = now() at time zone 'utc'
     , updated_at = now()
 WHERE id = reactivableRow.id;

COMMIT;
```

**SQL 順序解釋**：partial unique index `WHERE status IN ('0','1')` 同時刻最多一筆 row。先把 currentInitRow 移出（變 CLOSED_BY_SWITCH），再把 reactivableRow 移入（變 INIT），任何時刻 INIT 集合都最多一筆，**index 不會被違反**。

##### 復活不打 PayUni

| 機制 | 是否觸發 | 理由 |
|------|---------|------|
| PayUni `/api/atm` 呼叫 | ❌ 不呼叫 | 復活的目的就是重用既有虛擬帳號 |
| `payment.paid` event | ❌ 不發 | 復活後狀態仍是 INIT，未付款。對下游而言這是冪等查詢 |

##### 復活成功後的回應

回給使用者的 createPayment response 與「正常 ATM 建立」一樣：

```json
{
  "tradeNo": "O038032821001",
  "paymentType": "2004",
  "amount": "900",
  "atmBankCode": "822",
  "atmVirtualAccount": "8220100000184846",  // 復活前的舊帳號
  "expireAt": "1775653324547"               // 復活前的舊到期時間
}
```

使用者拿到的虛擬帳號**完全與第一次取號時相同**，不會察覺有「復活」這個內部動作。

##### 復活的審計欄位

`payment_transactions.reactivated_at TIMESTAMPTZ NULL`（欄位語意見 §3.1）：
- 第一次建立時：NULL
- 第一次復活時：寫入 nowUtc
- 第二次以後復活時：覆寫為最新 nowUtc

> **不記錄復活次數**：`reactivated_at` 覆寫為最近一次復活時間即可，不新增計數欄位。

##### 邊界情境

| 情境 | 處理 |
|------|------|
| 復活成功的瞬間 ExpireScheduler 標 row 為 EXPIRED | 不會發生：ExpireScheduler 條件 `WHERE status='0' AND expire_at < nowUtc`，復活前 row 是 status='6'（不在排程範圍）；復活後 expire_at 仍未到（C3 條件保證）|
| 復活流程入口前提 | 復活流程**僅在 §2A.3 切換偵測為「不同 type 切換」時觸發**，意即 currentInitRow 必然存在（findActiveByOrderId 已找到一筆 INIT/PAID）。若已是 PAID 而非 INIT，整個 createPayment 在 §2.4 既有「重複扣款防護」就被攔下（找到 PAID 直接 409），根本不會進入復活流程 |
| 復活的 row 與當前 createPayment 請求的金額不同（不應發生但保險起見） | 拋警報「ATM 復活金額不符」，**拒絕復活**，進入新建流程（保守策略）|
| 多筆可復活 row（使用者反覆切換多次） | `ORDER BY expire_at DESC LIMIT 1`，選剩餘時間最長的那筆 |

#### 2A.5.5 ExpireDate 統一以 PayUni 為真實來源

##### expire_at 的唯一來源

🔴 **`payment_transactions.expire_at` 的唯一來源是 PayUni `/api/atm` response 的 `ExpireDate`**：

```
PayUni /api/atm response → "ExpireDate"（格式 yyyy-MM-dd HH:mm:ss，時區 UTC+8）
        ↓
DateTimeUtils.parsePayUniDateTimeToUtc(s)   // 見 §10
        ↓
LocalDateTime expireAt（值語意 UTC）
        ↓
寫入 payment_transactions.expire_at
```

**禁止事項**（每一條對應一種會讓我方與 PayUni 不一致的算法）：

| 禁止 | 若違反會發生什麼 |
|---|---|
| 自行以 `nowUtc() + N 天` 推算 `expire_at` | 與 PayUni 實際設定的「該日 23:59:59 UTC+8」相差最多 24 小時＋時區誤差，使用者看到的期限與真正能繳費的期限不同 |
| 用 `LocalDate.now()` 之類**無時區**的當下日期算送給 PayUni 的天數 | UTC 容器在 UTC 16:00~24:00（＝台北隔日）會算早一天，PayUni 拿到錯的 ExpireDate |
| 以 `yyyyMMdd` 解析 `ExpireDate` | 實際格式是 `yyyy-MM-dd HH:mm:ss`，解析必拋例外，而 catch 之後多半會退回自算 ⇒ 回到第一列 |
| 拿不到 `ExpireDate` 時 fallback 自算 | 靜默產生與 PayUni 不一致的期限。**正確行為是拋錯** |

##### 有效天數（`expireDays`）政策

🔴 **ATM 有效天數是後端業務政策，由後端 config 決定，前端不傳。**

```properties
# common-core-defaults.properties
hawkr.payment.atm.expire-days=${PAYMENT_ATM_EXPIRE_DAYS:2}
```

| 項目 | 值 |
|------|---|
| **預設值** | `2` |
| **生效效果** | PayUni ExpireDate ＝ 建立日 +1 天的 23:59:59 UTC+8（達成「隔日截止」） |
| **為何不是 1** | `expireDays=1` 會讓期限落在**當日** 23:59:59；晚上 23:00 下單只剩 1 小時可繳費 |
| **覆寫** | 環境變數 `PAYMENT_ATM_EXPIRE_DAYS` |

**契約**：`CreatePayment` 請求**不帶** `expireDays`；服務端不從請求取有效天數。


#### 2A.5.6 流程圖

```mermaid
flowchart TD
    A[createPayment paymentType=ATM<br/>通過 §2A.1/§2A.2/§2A.3 切換規則] --> B[findActiveByOrderIdAndPaymentType<br/>O123, ATM]
    B --> C{找到既有 row?}
    C -->|否| D[新建 ATM 流程<br/>§2A.5.7 時序圖]
    C -->|是 INIT| E{nowUtc > expire_at?}
    E -->|否 仍有效| F[回 cached<br/>含舊虛擬帳號]
    E -->|是 已過期| G[標 row status=EXPIRED<br/>L2 即時兜底]
    G --> D
    C -->|是 PAID| J[409 訂單已付款]
```

#### 2A.5.7 時序圖：建立 ATM + ExpireDate 解析

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端
    participant GW as Gateway
    participant PAY as PaymentService
    participant DB as payment_transactions
    participant PU as PayUni

    FE->>GW: POST /app/payments<br/>{orderNo:O123, paymentType:ATM}
    GW->>PAY: CreatePayment
    PAY->>PAY: 通過 §2A.1~§2A.4
    PAY->>PAY: 讀 hawkr.payment.atm.expire-days = 2
    PAY->>PAY: DateTimeUtils.todayInTaipei().plusDays(2)<br/>= 2026-04-09 (台北日期)
    PAY->>PU: /api/atm<br/>ExpireDate=2026-04-09<br/>MerTradeNo=O123001
    PU-->>PAY: SUCCESS<br/>ExpireDate=2026-04-09 23:59:59<br/>(yyyy-MM-dd HH:mm:ss UTC+8)<br/>TradeNo=1775..001<br/>PayNo=8220...
    PAY->>PAY: parsePayUniDateTimeToUtc<br/>= 2026-04-09 15:59:59 UTC
    PAY->>DB: INSERT trade_no=O123001, attempt_seq=1,<br/>expire_at=2026-04-09 15:59:59+00,<br/>provider_trade_no=1775..001,<br/>atm_virtual_account=8220...
    PAY-->>FE: 虛擬帳號 + expireAt
```

#### 2A.5.8 與 §2A.6 重複付款處理的銜接

當使用者切換 paymentType（例 ATM → 信用卡）後，PayUni 端的舊 ATM 虛擬帳號**仍存活到 ExpireDate**（C2 約束，PayUni 沒有關閉 API）。若使用者後續真的去 ATM 轉帳付款，PayUni 會送 webhook 進來，這時 §2A.3.6「Webhook 追蹤機制」段 3 會偵測到「歷史 row 但無 INIT」的情境，自動進入 §2A.6 重複付款處理流程。

本章節（§2A.5）只負責「不讓我方主動去多取虛擬帳號」，**不負責**處理 PayUni 端舊虛擬帳號被使用後的重複付款問題 — 那是 §2A.6 的職責。

---

### 2A.6 重複付款處理

> 本章節是整個 §2A 設計的「兜底機制」。前面 §2A.3~§2A.5 已盡力預防重複付款，但因 PayUni C2/C3 約束（無法主動關閉、不阻擋重複付款），仍可能發生「同一筆訂單收到兩筆 PAID」的情境。本章節定義偵測與自動退款的處理規則。

#### 2A.6.1 重複付款的成因與處理原則

重複付款**無法在上游根絕**，因為 §2A.3.1 的兩條 PayUni 硬性約束：
- **C2**：PayUni 端已建立的交易**無法主動關閉**（特別是 ATM 虛擬帳號）
- **C3**：PayUni 端**不阻擋重複付款**

⇒ 本節是兜底機制，必須存在。三種會實際發生的情境：

**情境 1：誠實使用者失誤**

```
1. 使用者選 ATM → PayUni 發虛擬帳號 A → 寫 row1 (ATM, INIT)
2. 過 10 分鐘後使用者切信用卡 → row1: CLOSED_BY_SWITCH，新建 row2 (CREDIT, INIT)
3. 使用者完成信用卡付款 → row2: PAID
4. 隔天使用者忘了已經付過了，又拿虛擬帳號 A 去 ATM 轉帳
5. PayUni 收到，發 webhook 進來
   → 段 3 偵測到「order_id 已 PAID 但 webhook.TradeNo 對不上既有 row」
   → 進入 §2A.6 重複付款處理
```

**情境 2：刻意重複付款**

```
1. 使用者選 ATM → 拿虛擬帳號 A
2. 切信用卡 → 完成信用卡付款 (PAID)
3. 故意又拿虛擬帳號 A 去 ATM 轉帳 → 第二筆 PAID
4. 期待我方退款流程處理錯誤、退錯筆、或重複退款
```

**情境 3：reconcile 偵測**

```
1. row1 (ATM) 被標 EXPIRED（虛擬帳號過期，狀態 = 3）
2. PendingPaymentReconcileScheduler 跑 reconcile，呼叫 PayUni query
3. PayUni 回 TradeStatus=1（已付款）→ rescue 路徑觸發
4. 但同時 row2 (CREDIT) 已經 PAID
5. → 進入 §2A.6 重複付款處理
```

##### 處理原則

| 項目 | 規則 |
|------|------|
| **偵測管道** | (a) Webhook 即時偵測（§2A.3.6 段 3）<br>(b) Reconcile 補救偵測（既有 [PendingPaymentReconcileService](services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileService.java) 擴充）|
| **退款優先序** | 信用卡 > ATM；兩筆同方式退較晚那筆（見 §2A.6.3） |
| **退款方式** | 信用卡走 `/api/trade/close`（即時）；ATM 走 `offlineRefundPage`（需使用者填銀行資訊） |
| **觸發機制** | 統一發 `payment.rescue.refund_initiated` event（既有），由 RescueRefundConsumer 處理 |
| **標記欄位** | `payment_transactions` 的 `is_duplicate`、`duplicate_of_payment_id`、`duplicate_sequence`（見 §3.1） |
| **保留審計** | 重複付款的「副」row 不會被刪除，只標 `is_duplicate=TRUE`，賣家後台/客服可查 |

#### 2A.6.2 偵測管道

##### 管道 A：Webhook 即時偵測（最常見路徑）

**入口**：§2A.3.6「Webhook 追蹤機制」段 3

**觸發條件**（`webhook.MerTradeNo` 含序號，需先 `extractOrderId`）：
- `findByProviderTradeNo(webhook.TradeNo)` → 找不到（段 1 失敗）
- `findByTradeNo(webhook.MerTradeNo)` → 找不到（段 1.5 失敗）
- `orderId = extractOrderId(webhook.MerTradeNo)` — 從 `O123001` 提取 `O123`
- `findActiveByOrderId(orderId)` → 找不到 INIT row（段 2 失敗）
- `findHistoryByOrderId(orderId)` → 找到至少一筆 PAID row

**處理**（**整個流程包在單一 DB transaction 內**，確保 race-free）：

```
@Transactional(isolation = REPEATABLE_READ)
processDuplicatePaymentWebhook(webhook):
  0. orderId = extractOrderId(webhook.MerTradeNo)  // O123001 → O123
     nextSeq = findMaxAttemptSeqByOrderId(orderId) + 1
     newTradeNo = orderId + format("%03d", nextSeq)

  1. SELECT ... FROM payment_transactions
       WHERE order_id = ? AND status = '1'（PAID） AND is_duplicate = FALSE
       ORDER BY created_at ASC
       FOR UPDATE                          # 行鎖既有「主」row，避免併發 race
     → existingPaid (鎖定)

  2. INSERT 新 row：
     - id = TSID（新）
     - trade_no = newTradeNo               # 含序號，如 O123003
     - attempt_seq = nextSeq
     - order_id = orderId
     - payment_type = webhook.PaymentType
     - provider_trade_no = webhook.TradeNo  # UNIQUE 約束防 webhook 重投
     - status = PAID
     - is_duplicate = TRUE                  # 暫定為「副」，下一步可能翻轉
     - duplicate_of_payment_id = existingPaid.id
     - duplicate_sequence = (SELECT COALESCE(MAX(duplicate_sequence), 0) + 1
                              FROM payment_transactions
                              WHERE order_id = ?)
     → newRow

  3. target = chooseRefundTarget(existingPaid, newRow)   # §2A.6.3 演算法

  4. 主/副翻轉（必要時）：
     if target == existingPaid:
       # 退舊那筆 → 翻轉
       UPDATE existingPaid SET is_duplicate=TRUE, duplicate_of_payment_id=newRow.id
       UPDATE newRow SET is_duplicate=FALSE, duplicate_of_payment_id=NULL, duplicate_sequence=NULL  # 清 stale 欄位（is_duplicate=FALSE 是業務權威來源）
       refundTarget = existingPaid
     else:
       # 退新那筆 → 不翻轉，維持初始
       refundTarget = newRow

  5. 在同一 transaction 內寫 outbox event：
     INSERT INTO outbox_events (event_type, payload, status)
     VALUES ('payment.rescue.refund_initiated', {refundTarget...}, '8221')
     -- '8221' = OutboxEventStatus.PENDING 的 CodeMeta value（欄位存 value 不存 enum 名）

  6. COMMIT
  # outbox publisher 後續非同步派送到 RabbitMQ，由 RescueRefundConsumer 處理
```

🔴 **事件必須走 outbox，不得直接 publish 到 RabbitMQ**：步驟 5 與步驟 1~4 要原子——DB 寫入成功則事件必發、DB rollback 則事件必不發。直接 publish 會產生兩個方向的破口：「DB 寫成功 ＋ MQ 斷線 ⇒ 退款事件漏發」與「MQ 發送成功 ＋ DB rollback ⇒ 重複退款」。

🔴 **本路徑必須是 REPEATABLE_READ**（與 §2A.5.4 不同，該處併入既有交易故 override 無效；此處是獨立 bean 的獨立交易，宣告會生效）：預設的 READ_COMMITTED 在「先讀 existingPaid → 後讀 `max(duplicate_sequence)`」之間可能讀到**不同 snapshot**，序號因此重複。配合 `FOR UPDATE` 行鎖，併發 webhook 才會被序列化處理。

##### 管道 B：Reconcile 補救偵測

**入口鏈路**：`PendingPaymentReconcileScheduler` → [PendingPaymentReconcileService.reconcileSingle()](services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/scheduler/PendingPaymentReconcileService.java) → `reconcileExpired()`（**private**，不可由外部直接呼叫，見下方 Tx 載體規定）

reconcile 對 EXPIRED row 呼叫 PayUni query，若 TradeStatus=1（PayUni 已收款）則觸發 rescue 退款。

**分流規則**：reconcile 確認 EXPIRED + PayUni SUCCESS 衝突後、進入單筆 rescue 退款前，先檢查同 `order_id` 是否已有其他 PAID row：

```
reconcileExpired(candidate):
  ... PayUni query 確認 SUCCESS, 寫 audit log ...

  # §2A.6.2 管道 B 分流
  otherPaid = findOtherPaidRowsForSameOrder(candidate.orderId, excludeId=candidate.id)
  if otherPaid 非空:
    duplicatePaymentService.handleDuplicateFromReconcile(candidate, otherPaid[0])
    transactionRepository.markRescued(tradeNo, now, REFUNDED)
    return RESCUE_DUPLICATE_DETECTED
  else:
    # 既有單筆 rescue 路徑（refundApplicationService.createRescueRefund）
    ...
```

**ReconcileOutcome**：本分流回 `RESCUE_DUPLICATE_DETECTED`，scheduler 的 tick.done log 以 `duplicate_detected` 計數。

**本分流依賴的兩項實作契約**：

- `PaymentTransactionMapper.findOtherPaidRowsForSameOrder` 的述詞必須是
  `WHERE order_id = ? AND id != ? AND status = '1'（PAID） AND is_duplicate = FALSE` ——
  少了 `is_duplicate = FALSE` 會把已判定為副 row 的那筆再撈出來，形成互指。
- `DuplicatePaymentService` 的偵測 ＋ `chooseRefundTarget` ＋ 主副翻轉 ＋ 寫 outbox event
  **整個方法必須是 `@Transactional(isolation = REPEATABLE_READ)`**，且 `lockExistingPaidByOrderId`
  以 `SELECT ... FOR UPDATE` 鎖住既有 PAID row。

**狀態轉換**：管道 B 走完後，candidate 的 `is_duplicate` / `duplicate_of_payment_id` 由 `DuplicatePaymentService.handleDuplicateFromReconcile` 內部處理（含 chooseRefundTarget 演算法 + 翻轉 + snapshot 同步），candidate 的 status 仍保持 EXPIRED（不轉 PAID），但會被標 `rescued_at` 避免下次 reconcile 重複處理。實際退款行為由翻轉後的 refundTarget 觸發 `payment.rescue.refund_initiated` event 完成。

**Tx 載體規定（見 §2A.6.5）**：

- `PendingPaymentReconcileService.reconcileExpired` 由同 class `reconcileSingle` 直接 `reconcileExpired(candidate)` self-invocation 呼叫 → **禁止標 `@Transactional`**（Spring AOP CGLIB proxy 不攔截 self-invocation 會讓 annotation 失效，造成 rescue marker 與事件發布在 mid-flight throw 時無 rollback 保護）
- visibility：`reconcileExpired` 改為 `private`，表達「不可由外部 caller 進入」的意圖；外部唯一入口為 `reconcileSingle`
- Tx 邊界：
  - **tx 外**：`paymentClient.queryTransaction`（PayUni HTTP RPC，外部 RPC 不應佔 DB 連線）、`writeRescueAuditLog`（內含 try-catch 設計上要求即使後續退款失敗仍保留 audit）、`duplicatePaymentService.handleDuplicateFromReconcile` 與 `refundApplicationService.createRescueRefund`（下游 service 各自 `@Transactional` 走 proxy 完整 commit，不需外層 join）
  - **tx 內**（以 `txTemplate.executeWithoutResult(...)` 包覆，避免與本章節「管道 A / 管道 B」術語混淆，以下使用 reconcileExpired 內部分支命名）：
    - **單筆 rescue 分支**（otherPaid 為空、`createRescueRefund` 已完成）：`markRescued` + `publishRescueEvents` 為單一短 tx。`publishRescueEvents` 經 `PaymentOutboxWriter.enqueueAndDispatch`（§5.10.1 transactional outbox，發布 `payment.rescue.refund_initiated`；ATM「需人工」不再發 MQ 事件，改以同 tx `markRescued` 設 `rescue_action='8902'` DB 狀態標記、由 sql_exporter 告警）於同 tx 內 INSERT `outbox_events`，與 `markRescued` 原子落地（rollback 皆不落地、commit 一併落地），commit 後由 afterCommit 即時 publisher-confirm 投遞、失敗交 `OutboxEventPublisherScheduler` 每 5s 兜底。**杜絕原子化破口**：若 rescue 事件仍以「tx 內 best-effort 直發 MQ + catch 吞例外」，則 `markRescued` 已 commit（設 `rescued_at` → candidate SQL `AND (status='0' OR rescued_at IS NULL)` 永久排除該 row）但事件遺失 → `RescueRefundConsumer` 永不觸發 → payment status 永久卡 EXPIRED（退款金流可能已完成、無資損，但狀態永久不一致無補償）。故 `publishRescueEvents` **禁止 catch 吞例外**：outbox 序列化失敗由 `enqueueAndDispatch` 自行 throw 使 txTemplate rollback（`markRescued` 一併回滾 → `rescued_at` 未設 → 下次 reconcile 重掃重試）
    - **重複付款分支**（otherPaid 非空、`handleDuplicateFromReconcile` 已完成）：`markRescued`（單一 UPDATE）為單一短 tx，與單筆 rescue 分支行為一致（`handleDuplicateFromReconcile` 內部已於自身 `@Transactional` 走 outbox 發 `payment.rescue.refund_initiated`）
- 違規檢核：本服務及未來新增 scheduler service 不得出現：(1)「`@Transactional protected/public` 方法 + 同 class `this.fooTx(...)` self-invocation」模式；(2) 於短 tx 內「事件發布 catch 吞例外」——會造成狀態 UPDATE 落地但事件遺失的原子化破洞（rescue 事件一律走 outbox，序列化失敗必 throw 使 tx rollback）

##### 兩個管道的差別

| 屬性 | 管道 A (Webhook) | 管道 B (Reconcile) |
|------|------------------|--------------------|
| 觸發時機 | 即時（webhook 進來時） | 排程（5 分鐘一輪） |
| 副付款 row 來源 | webhook 進來時新建 | candidate 本身就是副 row（已存在） |
| 主付款 row 識別 | 透過 `findHistoryByOrderId` 找最新 PAID | 透過 `findOtherPaidRowsForSameOrder` |
| 速度 | 秒級 | 分鐘級 |

兩者最終都收斂到同一個 `payment.rescue.refund_initiated` event，由同一個 consumer 處理 — 退款邏輯統一。

#### 2A.6.3 退款對象選擇（核心演算法）

當偵測到同 order_id 有兩筆 PAID 時，必須決定哪一筆**保留為訂單最終付款**（主），哪一筆**退掉**（副）。

##### 付款方式分類

退款優先序的核心是「**容易退的優先退**」。依退款執行特性分兩類：

| 類別 | paymentType | 退款 API | 是否需使用者介入 |
|------|------------|---------|---------------|
| **電子即時退款類** | `CREDIT_CARD` / `APPLE_PAY` / `GOOGLE_PAY` | `/api/trade/close` | **不需要**，即時原路退回 |
| **離線退款類** | `ATM` | `/api/trade/refund_pos_page` (offlineRefundPage) | **需要**，使用者填寫銀行帳戶資訊後 PayUni 處理（2~3 工作天到帳） |

> **設計原則**：信用卡 / Apple Pay / Google Pay 三者**視為同一層級**，無內部優先序差別（PayUni 後端用同一支退款 API，從使用者視角也都是信用卡帳單原路退）。

##### 規則表

| # | 兩筆 row 的類別 | 主（保留） | 副（退） | 理由 |
|---|---------------|----------|--------|------|
| R1 | 一筆電子即時類、一筆離線類 | **離線類**（ATM） | **電子即時類** | 退「容易退的」對使用者最友善。電子即時類直接打 PayUni 即完成；離線類需使用者填表 |
| R2 | 兩筆都是電子即時類（含 CREDIT_CARD/APPLE_PAY/GOOGLE_PAY 任意組合） | 較早那筆 | **較晚那筆** | 較早那筆視為「真正的訂單付款」；較晚那筆視為「重複動作」。退較晚最符合直覺 |
| R3 | 兩筆都是離線類（兩筆 ATM） | 較早那筆 | **較晚那筆** | 同 R2。但兩筆都需使用者填表 → 落入待人工路徑（`rescue_action='8902'`，sql_exporter `PaymentRescueManualPending` 告警） |

> **「較早 / 較晚」定義**：依 `payment_transactions.created_at` 比較，較大值為較晚。
>
> **跨類別比較不依時間**：R1 不論誰先誰後，永遠退電子即時類那筆。理由是退款執行成本（使用者體驗）遠大於「先後直覺」。

##### 演算法（虛擬碼）

```
ELECTRONIC_INSTANT = {CREDIT_CARD, APPLE_PAY, GOOGLE_PAY}
OFFLINE = {ATM}

function paymentCategory(type):
    if type in ELECTRONIC_INSTANT: return ELECTRONIC_INSTANT
    if type in OFFLINE: return OFFLINE
    raise UnsupportedPaymentTypeForDuplicate

function chooseRefundTarget(rowA, rowB):
    catA = paymentCategory(rowA.payment_type)
    catB = paymentCategory(rowB.payment_type)

    # R1: 跨類別 → 退電子即時類那筆
    if catA != catB:
        return rowA if catA == ELECTRONIC_INSTANT else rowB

    # R2/R3: 同類別 → 退較晚那筆
    return rowB if rowA.created_at < rowB.created_at else rowA
```

> **更多筆**：理論上 PayUni C3 約束允許 N 次重複付款。本設計以「成對偵測」為主：每收到第 N+1 筆 webhook 就跟既有「主」比較一次，逐步收斂。`duplicate_sequence` 遞增記錄。
>
> **未支援類別**：若 PayUni 未來新增其他付款方式（如 LINE Pay、JKoPay），必須在 `paymentCategory()` 補上分類後才能進入重複付款處理。否則直接拋 `UnsupportedPaymentTypeForDuplicate` 進入人工處理（保守策略，避免誤退款）。

#### 2A.6.4 標記設計

##### Schema 變更

`payment_transactions` 新增 3 個欄位：

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `is_duplicate` | `BOOLEAN NOT NULL` | `FALSE` | 是否為重複付款被退的「副」row |
| `duplicate_of_payment_id` | `BIGINT NULL` | NULL | 指向「主」row 的 `id`（PK），FK 自關聯 |
| `duplicate_sequence` | `SMALLINT NULL` | NULL | 第幾次重複（1, 2, ...），給人類閱讀用 |

**索引**：

```sql
CREATE INDEX idx_payment_transactions_duplicate
    ON payment_transactions(duplicate_of_payment_id)
    WHERE is_duplicate = TRUE;
```

🔴 **重複付款的標記欄位放 `payment_transactions` 自關聯，不放 `orders`**：一筆 order 可以有多筆 payment（這正是重複付款的本質），放在 orders 表無法表達「哪一筆是副的」。

##### 顯示用「DUP 後綴」

賣家後台/客服查詢介面顯示時，動態組合：

```java
String displayTradeNo = tx.isDuplicate()
    ? tx.tradeNo() + "-DUP" + tx.duplicateSequence()
    : tx.tradeNo();
// 例：O038032821-DUP1
```

**不存實體欄位**，理由：
- 字串後綴不可索引、不可查詢、容易被誤改
- DB 欄位 `is_duplicate` + `duplicate_sequence` 已可 reconstruct 顯示字串
- PayUni 通知信顯示 `MerTradeNo=O038032821001`（含序號），賣家搜尋 `O038032821` 可 prefix 匹配

#### 2A.6.5 退款執行流程

依 §2A.6.3 選定退款對象後，發 `payment.rescue.refund_initiated` event。`RescueRefundConsumer` 接收後依 paymentType 走不同退款 API：

> **退款完成的表達方式**：rescue 退款**不另建退款 row**，以 `payment_transactions.status` 由 PAID 翻轉為 CLOSED 表達完成（`payment_refunds` 只承載走 §5.5 正規退款流程的退款）。

| paymentType | 退款 API | 即時性 | 後續 |
|-------------|---------|------|------|
| CREDIT_CARD | `PaymentClient.refund()` → `/api/trade/close` | 即時（同步 response 即知成功） | `markRescueRefunded` 將副 row PAID→CLOSED（status=5），同 tx INSERT outbox `payment.rescue.refund_completed` |
| APPLE_PAY / GOOGLE_PAY | 同上（PayUni 視為信用卡子類型） | 即時 | 同上 |
| ATM | `PaymentClient.offlineRefundPage()` | **非即時**，需使用者填銀行資訊 | 標 `rescue_action='8902'`（待人工），由 sql_exporter `PaymentRescueManualPending` 告警通知運維人工介入。**不發 MQ 事件**——沒有 consumer 的事件是孤兒，訊號會靜默消失 |

##### 信用卡退款流程

```
1. RescueRefundConsumer 接到 payment.rescue.refund_initiated
2. 依 provider_trade_no 鎖定退款 row（refundTarget），確認 payment_type=CREDIT_CARD
3. 呼叫 PayUni /api/trade/close (CloseType=2 退款)
4. 收到 SUCCESS（或 alreadyRefunded 冪等）→ markRescueRefunded(target.id) 將 row PAID→CLOSED
5. markRescueRefunded updated > 0 → INSERT outbox_events
   (event_type=payment.rescue.refund_completed, status=PENDING)
   → OutboxEventPublisherScheduler 5s 內派送到 RabbitMQ → notification-service 推播給使用者
6. markRescueRefunded updated == 0（PayUni 已退但本地未記錄）→ log error，等待人工介入，不發 completed event
```

**Tx 載體規定**：

- `RescueRefundConsumer.executeCreditRefund` **禁止標 `@Transactional`**，由 `onRescueRefundInitiated` 同 class 直接 `this.executeCreditRefund(...)` call 屬 self-invocation，Spring AOP CGLIB proxy 不攔截 → annotation 失效
- 與上述步驟 1–6 對應：
  - **步驟 3（PayUni `/api/trade/close`）在 tx 外** — 由 `executeCreditRefund` 為 entry method 直接呼叫，外部 HTTP RPC 不應佔 DB 連線
  - **步驟 3.5（rescue.refund_completed payload 序列化）在 tx 外** — `payloadJson = objectMapper.writeValueAsString(payload)` 屬純 CPU 工作，與 PayUni HTTP RPC 同樣「不應佔 DB 連線」。**必須**在進入 tx 前完成序列化；序列化失敗（`JsonProcessingException`）直接 throw `RuntimeException` 由 `onRescueRefundInitiated` outer catch → `MqErrorHandler.handleFailure` → DLQ retry。**禁止**在 `persistRefundResultBody` tx body 內 serialize + catch `return` — `executeWithoutResult` 僅在 throw 才 rollback，`return` 屬正常結束會讓 tx commit，造成 `markRescueRefunded` UPDATE 落地但 outbox INSERT 漏寫 → 等效於原子化破口——序列化失敗這條 path 正是最容易漏的一條
  - **步驟 4–5（`markRescueRefunded` + `INSERT outbox_events`）在 tx 內** — 拿到 PayUni 成功 response **且** 序列化成功後，以 `txTemplate.executeWithoutResult(status -> persistRefundResultBody(..., payloadJson))` 包覆為**單一短 tx**，保證「row PAID→CLOSED」與「完成事件 outbox 落地」原子化。tx body 內只做 DB 寫入，不做序列化、不做外部 RPC、不做 catch+return
  - **步驟 6（updated == 0 路徑）為 tx 完成後的後處理** — 不發 outbox 完成事件，等待人工介入
- 實作必須：constructor inject `PlatformTransactionManager` → 建 `TransactionTemplate` field → `executeCreditRefund` 為 entry（依序：呼叫 PayUni → 成功時序列化 payload → 進 txTemplate 包覆 `persistRefundResultBody`）→ `persistRefundResultBody(target, rescueReason, closedAt, payloadJson)` 為 private body method 僅由 caller 透過 `txTemplate` 包覆 call，內含 markRescueRefunded UPDATE + outbox INSERT 兩個 DB write
- 違規檢核：本服務及未來新增 MQ Consumer / Service / Controller 不得出現以下模式：(1)「`@Transactional protected/public` 方法 + 同 class `this.fooTx(...)` self-invocation」、(2)「tx body 內 serialize + catch + return」(silently swallow → tx commit → 原子化破洞)

##### ATM 退款流程

```
1. RescueRefundConsumer 接到 payment.rescue.refund_initiated
2. 取出副 row，確認 payment_type=ATM
3. 由於 ATM 退款需使用者填銀行資訊，標 rescue_action='8902'（待人工；不發 MQ 事件），由 sql_exporter PaymentRescueManualPending 告警通知運維
4. 系統寄通知信給使用者：「您的訂單 O123 因重複付款，請點選連結填寫退款銀行資訊」
   - 連結帶 returnUrl，使用者點擊後 → /api/trade/refund_pos_page → 填表 → PayUni 處理
5. PayUni 處理完成後送 webhook（既有 RefundResultConsumer 處理）→ 更新副 row status=REFUNDED
```

#### 2A.6.6 與既有 payment.rescue.* event 的整合

既有 [PaymentEventType](services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/PaymentEventType.java) 已有 rescue events，原本是給「reconcile 在 grace 視窗內偵測到 EXPIRED 但 PayUni 已成功」的單筆 rescue 用。

本章節使用 `payment.rescue.refund_initiated`（觸發）與 `payment.rescue.refund_completed`（完成）兩個事件。🔴 **ATM「需人工」不得以 MQ 事件承載**——它沒有 consumer，發出去等於訊號消失；一律走 `rescue_action='8902'` DB 狀態 ＋ 告警：

| Event | 用途 | §2A.6 角色 |
|-------|---------|---------------|
| `payment.rescue.refund_initiated` | rescue 退款已發起（觸發 RescueRefundConsumer） | 重複付款偵測或 EXPIRED rescue 後對副 row 發起退款 |
| `payment.rescue.refund_completed` | rescue 退款完成（信用卡） | RescueRefundConsumer 信用卡路徑退款成功 + markRescueRefunded > 0 後，由 outbox publish；notification-service 訂閱發送通知給使用者 |

**`payment.rescue.refund_completed` payload 結構**（snake_case，與全專案 PaymentDomainEvent 慣例一致）：

| 欄位 | 來源 | 說明 |
|------|------|------|
| `trade_no` | target.tradeNo | 退款 row 的 trade_no |
| `order_id` | target.orderId | 訂單編號（notification reference_id 依據） |
| `account_id` | target.accountId | 通知對象（買家 accountId） |
| `mer_id` | target.merId | PayUni merId |
| `provider_trade_no` | target.providerTradeNo | PayUni 端 TradeNo |
| `amount` | target.amount | 退款金額（元） |
| `payment_type` | target.paymentType.value | 1001(CREDIT_CARD) / 1003(APPLE_PAY) / 1004(GOOGLE_PAY) |
| `rescue_reason` | event payload 帶過來 | `RESCUE_DUPLICATE_PAYMENT` / `RESCUE_RECONCILE_AFTER_EXPIRED` |
| `refunded_at` | DateTimeUtils.nowUtc | 退款完成時間（UTC ISO 8601） |
| `event_type` | constant | `payment.rescue.refund_completed` |
| `aggregate_type` | constant | `payment` |
| `aggregate_id` | target.tradeNo | 同 trade_no |
| `occurred_at` | DateTimeUtils.nowUtc | 事件發生時間 |

> **不發 completed event 的情境**（與 触發點清單一致）：
> 1. ATM 路徑（標 `rescue_action='8902'` 待人工、由 sql_exporter 告警觸發人工頁面流程與後續通知）
> 2. PayUni close 失敗（retry / DLQ 中，使用者不需中間態）
> 3. PayUni close 成功但 markRescueRefunded == 0（PayUni 已退但本地未記錄，需人工介入避免幻覺通知）

**Event payload 擴充**：新增一個欄位 `rescue_reason` 區分原因。`rescue_reason` 不是 PaymentEventType 的子分類（兩者語意層級不同），而是 event payload 內的獨立欄位，由新增的 **`RescueReason` enum** 承載：

```java
// 新增 enum：services/java/shared/common-core/.../utils/code/RescueReason.java
public enum RescueReason implements CodeMeta {
    RESCUE_RECONCILE_AFTER_EXPIRED(
        "RESCUE_RECONCILE_AFTER_EXPIRED", "9001",
        "EXPIRED reconcile rescue",
        "reconcile 在 grace 視窗內偵測到 EXPIRED 但 PayUni 已成功"),
    RESCUE_DUPLICATE_PAYMENT(
        "RESCUE_DUPLICATE_PAYMENT", "9002",
        "重複付款偵測",
        "§2A.6 重複付款處理觸發的退款");
    // ... 其餘 CodeMeta 介面實作
}
```

| `rescue_reason` 值 | 語意 | 來源 |
|-------------------|------|------|
| `RESCUE_RECONCILE_AFTER_EXPIRED` | 既有 EXPIRED rescue 路徑 | reconcile scheduler 在 EXPIRED grace 視窗內偵測到 PayUni 已成功 |
| `RESCUE_DUPLICATE_PAYMENT` | **新增**：重複付款偵測 | §2A.6 webhook 段 3 / reconcile 補救偵測 |

> **既有 event payload 遷移**：既有 `payment.rescue.refund_initiated` event 的 payload 已包含 `RESCUE_REASON` 字串欄位。該欄位的值一律取自 `RescueReason.code()`，**不得硬編字串**；合法值為 `RESCUE_RECONCILE_AFTER_EXPIRED` 與 `RESCUE_DUPLICATE_PAYMENT`。

下游 consumer 不需要區分這兩種 reason（退款邏輯一樣），但日誌與告警可以據此分類統計。

#### 2A.6.7 流程圖：完整偵測 + 退款

```mermaid
flowchart TD
    A[Webhook 進來<br/>PaymentNotifyHandler] --> S1[§2A.3.6 段 1<br/>findByProviderTradeNo]
    S1 --> S1Q{找到?}
    S1Q -->|否| S2[段 2<br/>findActiveByOrderId]
    S1Q -->|是| T1{terminal?}

    T1 -->|否 INIT| HP[handlePaid<br/>updateToPaid]
    T1 -->|是<br/>CLOSED_BY_SWITCH/EXPIRED/CLOSED<br/>+ webhook PAID| B1B[§2A.3.6 段 1.b<br/>switchBackToPaid<br/>X1 case]
    T1 -->|是 PAID + webhook PAID<br/>provider_trade_no 不同| B1C[§2A.3.6 段 1.c<br/>真重複付款偵測]

    S2 --> S2Q{找到 INIT/PAID?}
    S2Q -->|是 INIT| HP
    S2Q -->|是 PAID + webhook 不同 TradeNo| B1C
    S2Q -->|否| S3[段 3<br/>findHistoryByOrderId]
    S3 --> S3Q{找到歷史 PAID?}
    S3Q -->|否| NF[404 交易不存在]
    S3Q -->|是| HD[handleDuplicateFromWebhook]

    B1B --> RA[reactivateRowAsDuplicate<br/>row1: 6→1 + 標 duplicate]
    RA --> FLIP1[markAsDuplicate(main)<br/>+ unmarkDuplicate(reactivated)]
    FLIP1 --> CRT1[chooseRefundTarget<br/>§2A.6.3 R1 跨類別退電子即時]
    CRT1 --> SNAP1[snapshot orders.payment_method<br/>§2A.7 觸發點 #5]
    SNAP1 --> OB[outbox_events INSERT<br/>payment.rescue.refund_initiated]

    B1C --> HD
    HD --> INS[INSERT 副 row<br/>provider_trade_no=新 webhook TradeNo<br/>UNIQUE 不撞]
    INS --> CRT2[chooseRefundTarget]
    CRT2 --> FLIP2[必要時翻轉]
    FLIP2 --> SNAP2[snapshot orders.payment_method]
    SNAP2 --> OB

    AA[Reconcile 排程<br/>§2A.6.2 管道 B] --> AAQ[reconcileExpired<br/>PayUni query SUCCESS]
    AAQ --> BB[findOtherPaidRowsForSameOrder]
    BB --> CC{有其他 PAID?}
    CC -->|否| EE[既有單筆 rescue 邏輯<br/>createRescueRefund]
    CC -->|是| DD[handleDuplicateFromReconcile]
    DD --> CRT3[chooseRefundTarget]
    CRT3 --> FLIP3[必要時翻轉]
    FLIP3 --> SNAP3[snapshot orders.payment_method]
    SNAP3 --> OB

    OB --> SCH[OutboxEventPublisherScheduler<br/>5s tick + 指數退避]
    SCH --> MQ[RabbitMQ EXCHANGE_PAYMENT]
    MQ --> RES[RescueRefundConsumer]
    RES --> RT{refundTarget type?}
    RT -->|CREDIT_CARD<br/>APPLE_PAY<br/>GOOGLE_PAY| CALL[呼叫 PayUni /api/trade/close]
    RT -->|ATM| MAN[標 rescue_action=8902 待人工<br/>sql_exporter 告警 + 使用者填表]
    CALL --> SUC{SUCCESS?<br/>或已退款冪等}
    SUC -->|是| MARK[markRescueRefunded<br/>row PAID→CLOSED]
    SUC -->|否| FAIL[retry max 3 → DLQ]
    MARK --> NOTIF[INSERT outbox_events<br/>payment.rescue.refund_completed<br/>OutboxEventPublisherScheduler 5s 派送<br/>notification-service 推播]
```

#### 2A.6.8 時序圖：誠實使用者觸發（webhook 偵測，退舊信用卡）

```mermaid
sequenceDiagram
    autonumber
    participant USR as 使用者
    participant PU as PayUni
    participant PAY as PaymentService
    participant DB as payment_transactions
    participant OB as outbox_events
    participant MQ as RabbitMQ
    participant RES as RescueRefundConsumer

    Note over USR,DB: 前置：row1=ATM CLOSED_BY_SWITCH<br/>row2=CREDIT_CARD PAID（使用者已付）

    USR->>PU: 隔天用舊 ATM 虛擬帳號轉帳 NT$900
    PU->>PAY: webhook<br/>MerTradeNo=O123001, TradeNo=1775..099,<br/>PaymentType=2 (ATM)

    rect rgb(245, 245, 220)
    Note over PAY,OB: 單一 DB transaction (REPEATABLE_READ)
    PAY->>DB: findByProviderTradeNo(1775..099) → null
    PAY->>DB: findActiveByOrderId(O123) → null
    PAY->>DB: SELECT ... WHERE order_id=O123 AND<br/>status='1'（PAID） AND is_duplicate=FALSE<br/>FOR UPDATE
    DB-->>PAY: existingPaid = row2 (CREDIT, 鎖定)
    PAY->>DB: INSERT row3 (ATM, PAID,<br/>is_duplicate=TRUE,<br/>duplicate_of=row2.id, sequence=1,<br/>provider_trade_no=1775..099)
    PAY->>PAY: chooseRefundTarget(row2, row3)
    Note over PAY: R1: 跨類別<br/>row2=ELECTRONIC_INSTANT<br/>row3=OFFLINE<br/>→ 退電子即時類 = row2
    PAY->>DB: 翻轉 — UPDATE row3 SET is_duplicate=FALSE,<br/>duplicate_of=NULL
    PAY->>DB: UPDATE row2 SET is_duplicate=TRUE,<br/>duplicate_of=row3.id
    PAY->>OB: INSERT outbox_events<br/>(payment.rescue.refund_initiated,<br/>target=row2, reason=RESCUE_DUPLICATE_PAYMENT)
    Note over PAY: COMMIT
    end

    OB->>MQ: OutboxPublisher 非同步派送
    MQ->>RES: 派送 event
    RES->>PU: /api/trade/close<br/>(TradeNo=row2.provider_trade_no,<br/>CloseType=2)
    PU-->>RES: SUCCESS
    RES->>DB: markRescueRefunded(row2.id)<br/>row2 PAID→CLOSED
    RES->>OB: INSERT outbox_events<br/>(payment.rescue.refund_completed,<br/>target=row2, rescue_reason)
    OB->>MQ: OutboxEventPublisherScheduler 派送
    MQ->>USR: notification-service 推播<br/>「重複付款已自動退款 NT$900<br/>信用卡原路退回」
```

> **關鍵設計：「主/副」翻轉的事務隔離**：
> - 整個 webhook 處理（步驟 4~13）包在**單一 DB transaction** 內，REPEATABLE_READ 隔離
> - `SELECT ... FOR UPDATE` 對既有 PAID row 加行鎖，併發 webhook 會序列化處理
> - 翻轉後寫 outbox event 與 row 變更**同一 transaction COMMIT**，無「DB 成功 / MQ 失敗」或「MQ 發送 / DB rollback」的不一致風險
> - `provider_trade_no` UNIQUE 約束（§2A.3.6 schema）防 webhook 重投造成重複 INSERT

#### 2A.6.9 時序圖：reconcile 補救偵測

```mermaid
sequenceDiagram
    autonumber
    participant SCH as ReconcileScheduler
    participant SVC as ReconcileService
    participant DB as payment_transactions
    participant OB as outbox_events
    participant PU as PayUni
    participant MQ as RabbitMQ
    participant RES as RescueRefundConsumer
    participant USR as 使用者

    Note over DB: 前置：row1 (ATM, EXPIRED, expire 後 grace 內)<br/>row2 (CREDIT, PAID)

    SCH->>SVC: 每 5 分鐘 tick
    SVC->>DB: findReconcileCandidates → row1
    SVC->>PU: query MerTradeNo=O123
    PU-->>SVC: TradeStatus=1 (PayUni 端 row1 也已收款)

    rect rgb(245, 245, 220)
    Note over SVC,OB: 單一 DB transaction (REPEATABLE_READ)
    SVC->>DB: SELECT ... WHERE order_id=O123 AND<br/>id != row1.id AND status='1'（PAID） AND<br/>is_duplicate=FALSE FOR UPDATE
    DB-->>SVC: [row2 (CREDIT, PAID, 鎖定)]
    Note over SVC: 重複付款情境 — row1 與 row2 都 PAID
    SVC->>DB: SELECT ... WHERE id=row1.id FOR UPDATE
    SVC->>DB: UPDATE row1 SET status=PAID,<br/>is_duplicate=TRUE,<br/>duplicate_of=row2.id, sequence=1
    SVC->>SVC: chooseRefundTarget(row1, row2)
    Note over SVC: R1: 跨類別<br/>row1=OFFLINE, row2=ELECTRONIC_INSTANT<br/>→ 退電子即時類 = row2
    SVC->>DB: 翻轉 — UPDATE row1 SET<br/>is_duplicate=FALSE, duplicate_of=NULL
    SVC->>DB: UPDATE row2 SET is_duplicate=TRUE,<br/>duplicate_of=row1.id
    SVC->>OB: INSERT outbox_events<br/>(payment.rescue.refund_initiated,<br/>target=row2, reason=RESCUE_DUPLICATE_PAYMENT)
    Note over SVC: COMMIT
    end

    OB->>MQ: OutboxPublisher 非同步派送
    MQ->>RES: 派送 event
    RES->>PU: /api/trade/close (TradeNo=row2.provider_trade_no)
    PU-->>RES: SUCCESS
    RES->>DB: markRescueRefunded(row2.id)<br/>row2 PAID→CLOSED
    RES->>OB: INSERT outbox_events<br/>(payment.rescue.refund_completed)
    OB->>MQ: OutboxEventPublisherScheduler 派送
    MQ->>USR: notification-service 推播
```

#### 2A.6.10 邊界與異常處理（無風險清單）

##### 併發安全

| 情境 | 防護機制 |
|------|---------|
| 兩個 webhook 同時進來（同一 order_id，不同 provider_trade_no） | 整個處理在 `@Transactional(REPEATABLE_READ)` 內，`SELECT ... FOR UPDATE` 對既有 PAID row 加行鎖。後到的 webhook 等鎖釋放後重新讀 snapshot，看到的「主」可能已變化，邏輯仍正確 |
| Webhook 重投（同一 webhook 進來兩次） | `provider_trade_no` UNIQUE constraint 攔下第二次 INSERT，視為冪等跳過，不會誤判為重複付款 |
| Reconcile 與 Webhook 同時偵測到同一筆 | 兩者都用 `FOR UPDATE` 鎖既有 PAID row，後到的會等鎖；先到完成後 `is_duplicate` 已標記，後到再讀就會看到最新狀態，避免重複處理 |

##### 事務原子性

| 情境 | 防護機制 |
|------|---------|
| DB 寫入成功但 MQ 派送失敗 | 用 outbox pattern：`payment.rescue.refund_initiated` event 寫進 `outbox_events` 表，與 row 變更同 transaction COMMIT。OutboxPublisher 非同步派送，失敗會重試 |
| MQ 派送成功但 DB rollback | 同上：outbox event 寫入 rollback 後不存在，OutboxPublisher 永遠不會派送 |
| 退款 API 呼叫成功但本地 DB 更新失敗 | `RescueRefundConsumer.executeCreditRefund` 分兩段：(a) **tx 外**呼叫 PayUni `/api/trade/close`；(b) 拿到成功 response 後以 `TransactionTemplate` 包覆 `markRescueRefunded`（row PAID→CLOSED）+ `INSERT outbox_events`（payment.rescue.refund_completed）為單一 tx。(b) 任一步失敗 → tx rollback，row 維持 PAID + outbox event 不存在，但 PayUni 端已退款 → 由 markRescueRefunded `WHERE id=? AND status='1'` 行鎖天然冪等保證 MQ 重投時 updated 仍可能 > 0（若 row 真未被推進）；若已被別處推進則 updated=0 走 log error 等人工介入，不會幻覺重複通知 |

##### 業務邊界

| 情境 | 處理 |
|------|------|
| 退款 API 失敗（PayUni 5xx 或業務錯誤） | 發 `refund.failed` event，副 row 維持 `status=PAID + is_duplicate=TRUE`，加 `refund_attempt_count` 計數，由 RescueRefundConsumer 重試（max 3 次），耗盡後人工介入 |
| 退款處理中 webhook 又進來第三筆 PAID | 同邏輯處理：rescan 「主」（is_duplicate=FALSE 那筆），新進 row 視為新「副」，`duplicate_sequence = MAX + 1`。每次 webhook 各自有獨立 transaction |
| 兩筆都是 ATM（R3 情境） | 退較晚那筆，但 ATM 退款需使用者介入 → 標 `rescue_action='8902'` 待人工，由 sql_exporter `PaymentRescueManualPending` 告警；若使用者一直不填，backlog 持續由該告警提醒人工客服 |
| 主 row 已被退款（status=REFUNDED）後，又收到第三筆 PAID webhook | `findOtherPaidRowsForSameOrder` 條件 `status='1'（PAID） AND is_duplicate=FALSE` → 找不到任何 row → 視為「**全新付款 → 沒有主可比較**」→ 拋警報「異常：訂單已全額退款但又收到付款」，由人工處理 |
| 副 row 退款成功後，主 row 也被退款（例如使用者主動取消訂單） | 兩筆都進入退款流程，refund_transactions 各一筆。Order 服務偵測「全額退款」後進入 CANCELLED 狀態 |
| webhook 帶的金額與 existingPaid 不同 | **拋警報並停止自動退款**「重複付款金額不一致：existingPaid=X, new=Y」，由人工檢查（金額不同代表 PayUni 異常或商業流程出錯，不可自動處理） |
| 未支援的 paymentType 進入重複付款流程 | `paymentCategory()` 拋 `UnsupportedPaymentTypeForDuplicate` → 不寫副 row、不發 event、寫警報 log → 人工處理（保守策略） |

##### 退款不可變性

> **核心安全原則**：一旦 outbox event 寫入並 COMMIT，退款流程**不可被取消**。設計上沒有 API 能取消已發起的 rescue 退款。
>
> 理由：取消流程會引入「退款請求 vs 取消請求」競態，極易誤退或漏退。寧可走完退款後再人工補回（信用卡授權重做、ATM 重新轉帳），不允許半途取消。

#### 2A.6.11 與其他章節的關係總覽

| 章節 | 關係 |
|------|------|
| §2.4 既有「重複扣款防護三層」 | **互補**：§2.4 防止「我方建立多筆 PayUni 交易」，§2A.6 處理「PayUni 端真的收到兩筆付款」 |
| §2A.3 切換規則 | **依賴**：切換產生 CLOSED_BY_SWITCH row 是重複付款的高頻來源 |
| §2A.4 信用卡頁面過期 | **互補**：過期機制管理單次付款嘗試的生命週期，§2A.6 處理跨嘗試的重複付款 |
| §2A.5 ATM 取號保護 | **互補**：§2A.5 防多取虛擬帳號，§2A.6 處理舊虛擬帳號被使用 |
| §10 PayUni 時區一致性 | **依賴**：偵測時需要對齊 PayUni 時間軸 |

#### 2A.6.12 Webhook handler atomicity

`PaymentNotifyHandler` 的終態寫入是 optimistic lock（`WHERE version = ?`），版本衝突時 `updated == 0`。

🔴 **`updated == 0` 絕不可回 success。** 回 success 會造成狀態不收斂的死局：handler 回 success ⇒ 外層 transaction commit ⇒ `markAsProcessed` 標 `is_processed=true, status=SUCCESS` ⇒ 下次重投同 webhook 被 `existsProcessed()` dedup skip ⇒ `payment_transactions` row **永遠停在 INIT**，只能手動 `DELETE FROM payment_webhooks` 才救得回來。

##### reload + retry 骨架

```
handlePaid(transaction, providerTradeNo, ...):
  updated = updateToPaid(tradeNo, ..., transaction.version)
  if updated == 0:
    reloaded = findByTradeNo(tradeNo).orElseThrow(WebhookRetryableException)

    if reloaded.status == PAID:
      return success("付款成功（冪等）")  # idempotent fast path

    updated = updateToPaid(tradeNo, ..., reloaded.version)  # 用新 version 重試一次
    if updated == 0:
      throw WebhookRetryableException  # 真實併發競爭，請重試

    transaction = reloaded

  publish PaymentSucceeded event
  return success("付款成功", mqEventData)
```

##### Retry / DLQ 鏈路

| 階段 | 行為 |
|---|---|
| handler throw `WebhookRetryableException` | RuntimeException 冒泡 |
| `transactionTemplate.execute` | 偵測 RuntimeException → rollback 整個 transaction，含 `markAsProcessed` |
| `payment_webhooks` row | 仍是 `is_processed=false, status=PENDING` |
| `existsProcessed` 下次查詢 | 仍 false → 重投不會被 dedup skip |
| Consumer try/catch | 接到例外 → `MqErrorHandler.handleFailure` |
| `MqErrorHandler` | ACK 原訊息 + 重新發布到原 queue + `x-retry-count++` |
| 累計 3 次後 | `basicReject` → DLQ + `ObsLogger.error` 告警 |

🔴 **重試訊號一律走例外（`WebhookRetryableException`），不得改成 result type**：例外冒泡才會觸發 `transactionTemplate` rollback，讓 `markAsProcessed` 一併回滾；回一個 result type 則交易照樣 commit，dedup 標記留下 ⇒ 回到上面那個死局。

##### 三分支對稱（handlePaid / handleFailed / handleConflict）

同一 optimistic-lock version 衝突死局適用於三個終態寫入分支，全部套用相同 reload + retry 骨架，禁止任一分支在 `updated==0` 時靜默回 success：

| 分支 | 條件更新方法（WHERE） | reload 後冪等判定 | retry 適用狀態 |
|---|---|---|---|
| `handlePaid` | `updateToPaid`（`status='0'`）| reloaded=PAID → 冪等 | INIT |
| `handleFailed` | `updateToFailed`（`status='0'`）| reloaded=FAILED → 冪等；已終態非 FAILED → stateConflict 不覆蓋 | INIT |
| `handleConflict` | `updateToConflict`（`status IN('1','2')`）| reloaded=CONFLICT → 冪等；已終態非 PAID/FAILED → stateConflict 不覆蓋 | PAID / FAILED |

**`handleConflict` 的處置與另外兩支完全對稱**：reload by id → 已 CONFLICT 則冪等略過 → 仍 PAID/FAILED 則以新 version 重試一次（成功才發 `payment.conflict`）→ 重試仍失敗拋 `WebhookRetryableException` → reload 後為其他終態則 `stateConflict` 不覆蓋。三分支共用同一 retry/DLQ 鏈路（上表），**不新增例外型別、不新增 event**。

🔴 **三分支的對稱性是本節的不變式**：任一分支在 `updated == 0` 時回 success，該分支的事件就會遺失且狀態永不收斂，而症狀只在版本衝突發生時才出現（平時完全看不出來）。

---

### 2A.7 訂單付款方式快照同步（顯示層規範）

> 本章節集中規範 `orders.payment_method` 欄位的更新時機與顯示邏輯。因 §2A.3 切換 + §2A.5.4 復活 + §2A.6 重複付款翻轉三個機制都會改變「主付款方式」，需要明確的快照同步策略，避免訂單列表顯示與真實付款方式不一致。

#### 2A.7.1 問題

訂單列表（`/app/orders`）與訂單明細（`/app/orders/{order_no}`）都需顯示「付款方式」欄位。同一筆訂單在不同時刻的「主付款」可能改變：

| 時刻 | 訂單狀態 | row1 | row2 | row3 | 主付款 |
|------|---------|------|------|------|-------|
| T0 | PENDING_PAYMENT | ATM INIT | — | — | ATM |
| T0+11min | PENDING_PAYMENT | ATM CLOSED_BY_SWITCH | CREDIT INIT | — | CREDIT |
| T0+12min | PENDING_PAYMENT | ATM CLOSED_BY_SWITCH | CREDIT CLOSED_BY_SWITCH | ATM 復活成 INIT | ATM（同 row1，復活）|
| T0+30min | PAID | ATM CLOSED_BY_SWITCH | CREDIT CLOSED_BY_SWITCH | ATM PAID | ATM |
| 假想：使用者又付一次信用卡 | PAID | ATM CLOSED_BY_SWITCH | CREDIT CLOSED_BY_SWITCH | ATM PAID（is_duplicate 翻轉=TRUE）| 信用卡 PAID（新主）|

如果 `orders.payment_method` 不及時同步，前端會顯示錯誤的付款方式。

#### 2A.7.2 顯示邏輯規範

| 訂單狀態 | 顯示來源 | 對應 SQL 邏輯 |
|---------|---------|--------------|
| **未付款** (PENDING_PAYMENT) | 最新一筆 INIT row 的 payment_type | `WHERE order_id=? AND status='0' LIMIT 1`（partial unique index 保證唯一）|
| **已付款** (PAID) | 主付款的 payment_type | `WHERE order_id=? AND status='1' AND is_duplicate=FALSE LIMIT 1` |
| **重複付款處理中** | 同上（主付款那筆） | 同上 |
| **退款後** | 同上（若主被退、副變主，顯示新主） | 同上 |
| **完全退款** (REFUNDED) | 退款前的主付款 payment_type | 從歷程中找最後一筆 PAID（即使現在已 REFUNDED）|

#### 2A.7.3 快照存於 `orders.payment_method`（既有欄位）

顯示用的付款方式一律讀 `orders.payment_method` 快照，**不在讀路徑動態 join `payment_transactions`**（訂單列表每筆都要 join，且 §2A.6 之後同一 order 可能多筆 payment，join 出來的「哪一筆」本身就有歧義）。

**快照觸發時機**：

| # | 觸發點 | 觸發者 | 動作 |
|---|--------|--------|------|
| 1 | 第一次 createPayment 成功 | payment-service `createPayment()` | 呼叫 `OrderGrpcClient.snapshotPaymentInfo()`，寫入 `payment_method = 新 row.payment_type` |
| 2 | 切換 paymentType 成功（§2A.3.7 切換流程結束） | payment-service `handleSwitch()` | 同上，寫入新建 row 的 payment_type |
| 3 | ATM 復活成功（§2A.5.4） | payment-service `AtmReactivationService.tryReactivate()` | 寫入 ATM（即復活的 row.payment_type） |
| 4 | webhook 標 PAID 且本筆是「主」 | webhook handler | 既有邏輯不變（snapshot 主 row 的 payment_type）|
| 5 | §2A.6 重複付款翻轉（主/副調整） | DuplicatePaymentService | 翻轉後立刻呼叫 snapshotPaymentInfo，寫入新主的 payment_type |
| 6 | 退款完成，副被退、無變化 | RescueRefundConsumer | 不需要 snapshot（主未變） |
| 7 | 退款完成，主被退（極罕見） | RescueRefundConsumer | 重新計算主（剩餘 PAID 中找 is_duplicate=FALSE），寫入新主 |

#### 2A.7.4 訂單明細頁顯示「付款歷程」

訂單明細頁可額外呼叫 `GET /app/payments/{trade_no}/history`（§2A.3.6 已規劃），列出該 order 所有 payment_transactions row（含 CLOSED_BY_SWITCH），讓使用者/客服看到完整歷程：

```
2026-04-08 10:00  ATM 取號（虛擬帳號 8220...）          [已切換]
2026-04-08 10:11  信用卡建立                              [已切換]
2026-04-08 10:25  ATM 復活（重用帳號 8220...）           [待付款]
2026-04-08 11:30  ATM 付款成功 NT$900                     [已付款] ✓
```

歷程顯示規則：

| status | 標籤 | 額外資訊 |
|--------|------|---------|
| INIT | 待付款 | 顯示虛擬帳號 / redirectUrl 是否仍有效 |
| PAID + is_duplicate=FALSE | 已付款 ✓ | 顯示金額、付款時間 |
| PAID + is_duplicate=TRUE | 重複付款（待退款）| 顯示「副付款，正在自動退款」|
| REFUNDED | 已退款 | 顯示退款時間、退款金額 |
| CLOSED_BY_SWITCH | 已切換 | 顯示切換到哪個 paymentType |
| EXPIRED | 已逾期 | 顯示逾期時間 |

復活的 row 在歷程中可顯示 `reactivated_at` 標記為「復活」事件，與第一次建立區分。

#### 2A.7.5 一致性保證

##### 同 transaction 內的快照更新

| 觸發點 | 是否與本地 DB 變更同 transaction |
|--------|-----------------------------|
| 1, 2, 3（payment-service 主動）| **跨服務 gRPC，無法同 transaction** |
| 4, 5, 7（webhook / consumer）| 同上 |

跨服務一致性策略：
- payment-service 內部操作完成後，**呼叫 order-service gRPC**（best-effort）
- 失敗時記 ObsLogger ERROR 並重試（最多 3 次），仍失敗則告警人工
- order 表 snapshot 不一致是「顯示問題」，**不影響業務正確性**（業務正確性由 payment_transactions 表保證）

##### Race condition 防護

兩個 webhook 同時更新 `orders.payment_method` 的場景**極罕見**（因為 partial unique index 已限制同時最多一筆 PAID + is_duplicate=FALSE）。即使發生，後到的 snapshot 會覆寫先到的，最終一致性即可：

- 不引入額外樂觀鎖欄位
- payment-service 每次都用「當下查到的主 row.payment_type」呼叫 snapshotPaymentInfo
- order-service 直接 UPDATE，不做版本比對
- 即使順序錯亂，下一次任何狀態變化（webhook / reconcile）會再次觸發 snapshot 收斂到正確值

#### 2A.7.6 與其他章節的關係

| 章節 | 關係 |
|------|------|
| §2A.3 切換規則 | 觸發點 #2：切換成功後 snapshot |
| §2A.5.4 ATM 復活 | 觸發點 #3：復活成功後 snapshot |
| §2A.6 重複付款處理 | 觸發點 #5：翻轉主/副後 snapshot |
| 既有 [SnapshotPaymentInfo](gateway/internal/handler/payment.go#L196) | 觸發點 #1, #4：既有付款成功流程不變 |

---

### 2A.8 Apple Pay / Google Pay Mode 切換規則

> 來源：PayUni 金流官方 API 總覽 §4.6（AppleObject）、§5.6（GoogleObject）。完整編碼規則見 [11 號設計書 §7.4](./11-PayUni%20Java%20Client（payuni-java-client）詳細設計書.md#74-apple-pay--google-pay-mode-與-token-編碼規則)。

#### 2A.8.1 Mode 的語意與切換規則

PayUni 的 Apple Pay / Google Pay API 各有 `AppleMode` / `GoogleMode` 參數（`"1"`=OnWeb / `"2"`=InApp）。

🔴 **mode 僅是 PayUni 端的「來源分類標記」，不影響 token 編碼**：兩個 mode 的 token **一律 Base64Encode**（Apple 與 Google 皆同；詳見 11 號設計書 §7.4.1.1）。把 mode 當成編碼開關（例如 mode=1 不做 base64）會讓 web 流程的 token 上送失敗。

🔴 **mode 不得硬寫**：一律依 `transaction.clientType()` 動態決定（對應表見 §2A.8.2）。硬寫 `AppleMode=2` / `GoogleMode=2` 等於只支援 mobile native app，web 流程無法正確上送。

token 編碼由 `payuni-java-client` 內部統一處理，`payment-service` **不自行 base64**（協定細節封裝在 client，見 11 號設計書 §7.4.2）。

> **caller 契約補充**：`applePayToken` 字串內容為 **`JSON.stringify(token.paymentData)` 的結果**（即 Apple Pay token 內層的 paymentData 物件，含 `data` / `signature` / `header` / `version`，**不**含 `paymentMethod` / `transactionIdentifier`）。Apple Pay JS 與 iOS PKPaymentToken 兩端都需在 caller 端做 paymentData 抽取後再傳給 backend；Flutter `pay` 套件的 `result['token']` 已替我們抽出 paymentData JSON。詳見 [PayUni 金流官方 API 總覽 §4.6](../第三方整合/PayUni%20金流官方%20API%20總覽.md#46-appleobject-參數說明)。

#### 2A.8.2 ClientType → Mode 對應

| ClientType | AppleMode | GoogleMode | 觸發場景 |
|-----------|-----------|------------|---------|
| `WEB_BROWSER` | `"1"` (OnWeb) | `"1"` (OnWeb) | PC 瀏覽器、行動瀏覽器 |
| `MOBILE_APP` | `"2"` (InApp) | `"2"` (InApp) | iOS / Android 原生 App |

**fallback**：`transaction.clientType()` 為空 / null / 不可識別時視為 `MOBILE_APP`（與 §1.x 既有規則一致），對應 `mode="2"`。

#### 2A.8.3 觸發條件

| 流程 | 觸發點 |
|------|--------|
| Apple Pay 建單 | `handleApplePayPayment(transaction, applePayToken, userEmail)` → 從 `transaction.clientType()` 推導 mode → 傳入 `paymentClient.createApplePayPayment(..., mode)` |
| Google Pay 建單 | `handleGooglePayPayment(transaction, googlePayToken, userEmail)` → 同上 |

#### 2A.8.4 業務規則

| 規則 | 說明 |
|------|------|
| R1 | mode 推導使用 helper 方法 `resolveProviderMode(ClientType)` 集中邏輯，避免兩處付款方法各自寫 if/else |
| R2 | `applePayToken` 永遠傳「原始 paymentData JSON 字串」、`googlePayToken` 永遠傳「原始 Google Pay token JSON 字串」給 `payuni-java-client`，**不**在 payment-service 做 base64（base64 由 client 統一處理） |
| R3 | mode 不入庫（PayUni 協定內部值），與訂單 / 交易 / event 無關 |
| R4 | mode 對 webhook 流程無影響（webhook 是 PayUni 主動通知，不依賴本次 mode 值） |
| R5 | **Web Google Pay (mode=1)：前端在 `loadPaymentData()` 開 sheet 前，必須拿到動態 `gatewayMerchantId`（PayUni 商店代號）塞入 tokenizationSpec**。來源：preview API response 的 `payment_mer_id`（結帳新流程）或 orderDetail response 的 `payment_mer_id`（重試新流程）。詳見 [20 設計書 §4.13 和單與 paymentMerId 決策](./20-結帳模組（Checkout%20Module）詳細設計書.md#413-和單與-paymentmerid-決策web-mobile-google-pay-動態-gatewaymerchantid-支援)。Apple Pay 不需要此值（merchant validation 由後端 cert 完成、merchantIdentifier 寫死 `merchant.io.github.will413028.hawkr`） |
| R6 | **後端 `createPayment` 仍從 DB `orders.mer_id` 取 merId 呼叫 PayUni，與前端送來的值無關**（既有契約 `CreatePaymentRequest.java`）。前端的 `gatewayMerchantId` 只影響 Google Pay token 加密 metadata，不影響後端決策的收款 merchant — 形成 defense-in-depth |

#### 2A.8.5 流程圖

```mermaid
flowchart TD
  A[PaymentApplicationService.createPayment] --> B{paymentType?}
  B -->|APPLE_PAY| C1[handleApplePayPayment transaction, token, email]
  B -->|GOOGLE_PAY| C2[handleGooglePayPayment transaction, token, email]
  C1 --> D[resolveProviderMode transaction.clientType]
  C2 --> D
  D --> E{ClientType?}
  E -->|WEB_BROWSER| F1[mode = 1 OnWeb]
  E -->|MOBILE_APP / fallback| F2[mode = 2 InApp]
  F1 --> G[paymentClient.createApplePayPayment ..., mode<br/>or createGooglePayPayment ..., mode]
  F2 --> G
  G --> H[PayUni API]
```

#### 2A.8.6 與其他章節耦合說明

| 章節 | 關係 |
|------|------|
| §1.1 服務職責 | 補充「OnWeb/InApp 自動切換」 |
| §2A.2 金額範圍 | 不影響（mode 不改變金額限制） |
| §2A.3 paymentType 切換 | 不影響（mode 是 PayUni 協定內部值，與業務切換無關） |
| §2A.7 訂單付款方式快照 | 不影響（snapshot 不含 mode） |
| §11 PayUni 互動 Audit Logging | client 端日誌會記錄完整 params 含 AppleMode/GoogleMode，無需額外改動 |

---

## 3. 資料庫設計

使用 PostgreSQL。Migration 由 Flyway 管理，僅保留一份 `V1__init_payment_tables.sql`（包含所有表、索引、約束、清理索引與重複扣款防護索引）。

### 3.1 payment_transactions

此表為核心，紀錄所有交易狀態。主鍵使用 TSID（`IdType.INPUT`）。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
| --- | --- | --- | --- |
| id | BIGINT | PK | TSID（應用層生成，`TsidUtil.generate()`） |
| trade_no | VARCHAR(32) | UK | **`{orderNo}{3位序號}`**（如 `O400280260001`），即 PayUni MerTradeNo。每次付款嘗試產生獨立 trade_no（詳見 §2A.3.6） |
| order_id | VARCHAR(64) | YES | 來自 Order Service 的原始訂單號（如 `O400280260`），同一訂單的所有 row 共用 |
| attempt_seq | SMALLINT | YES | 同一訂單的第 N 次付款嘗試（從 1 開始遞增），DEFAULT 1 |
| mer_id | VARCHAR(32) | NO | PayUni 子商家 MerID（代理商模式，選填） |
| account_id | VARCHAR(64) | NO | 買家帳號 ID |
| payment_type | SMALLINT | YES | 支付方式（1:CREDIT_CARD, 2:APPLE_PAY, 3:GOOGLE_PAY, 4:ATM） |
| amount | BIGINT | YES | 交易金額（新台幣元），CHECK (amount > 0) |
| product_name | VARCHAR(200) | NO | 商品名稱（顯示於金流頁面） |
| status | SMALLINT | YES | 交易狀態：0:INIT, 1:PAID, 2:FAILED, 3:EXPIRED, 4:CONFLICT, 5:CLOSED, 6:CLOSED_BY_SWITCH |
| provider_trade_no | VARCHAR(64) | NO | PayUni 回傳的交易序號（用於退款/查詢） |
| provider_status | VARCHAR(20) | NO | PayUni TradeStatus 原始值（遵循[收斂規範](../開發規範/PayUni%20錯誤回應收斂規範.md) §4.1 SoT）|
| provider_message | TEXT | NO | PayUni 回傳訊息（型別為 TEXT，避免長訊息截斷）|
| provider_create_at | TIMESTAMPTZ | NO | PayUni 認定的交易建立時間（§10.5 由 webhook／查詢回應解析寫入）|
| provider_paid_at | TIMESTAMPTZ | NO | PayUni 認定的使用者真實付款時間（§10.5）|
| granted_at | TIMESTAMPTZ | NO | PayUni 撥款成功時間（里程碑，規範 §3.4 允許）|
| grant_status | VARCHAR(20) | NO | PayUni `/api/merchant/grant` 回應狀態碼；撥款為獨立 PayUni 操作，不與 `provider_status` 共用（見規範 §2.3 撥款欄位封閉規則）|
| grant_message | TEXT | NO | PayUni 撥款回應訊息 |
| is_duplicate | BOOLEAN | YES | 是否為重複付款 row（DEFAULT FALSE）。標 TRUE 的副 row 不佔 `idx_payment_transactions_order_active` 的 slot（§2A.6.4）|
| duplicate_of_payment_id | BIGINT | NO | 指向同訂單「最早成功 row」的 id（§2A.6.4）|
| duplicate_sequence | SMALLINT | NO | 同訂單第 N 次付款（1＝首筆，2 以上為重複）（§2A.6.4）|
| reactivated_at | TIMESTAMPTZ | NO | ATM row 最後一次復活時間，僅供審計；覆寫為最新值，不記次數（§2A.5.4）|
| atm_bank_code | VARCHAR(10) | NO | ATM 銀行代碼 |
| atm_virtual_account | VARCHAR(20) | NO | ATM 虛擬帳號 |
| atm_expire_date | DATE | NO | ATM 繳費期限 |
| pay_time | TIMESTAMPTZ | NO | 實際付款時間（來自 Webhook） |
| closed_at | TIMESTAMPTZ | NO | 關閉時間 |
| expire_at | TIMESTAMPTZ | NO | 交易到期時間 |
| created_at | TIMESTAMPTZ | YES | 建立時間（DEFAULT NOW()） |
| updated_at | TIMESTAMPTZ | YES | 更新時間（DEFAULT NOW()） |
| last_reconciled_at | TIMESTAMPTZ | NO | L2 補救路徑：最近一次 reconcile 處理時間，用於控制重撈退避（§5.7.1） |
| rescued_at | TIMESTAMPTZ | NO | L2 補救路徑：rescue 退款觸發時間（EXPIRED→PayUni SUCCESS 衝突時設定） |
| rescue_action | VARCHAR(30) | NO | L2 補救路徑：rescue 動作結果，REFUNDED 或 REFUND_PENDING_MANUAL |
| client_type | VARCHAR(16) | YES | CodeMeta `ClientType` value（`MOBILE_APP` / `WEB_BROWSER`），DEFAULT `'MOBILE_APP'`。決定 §5.9 PayUni 回調完成後的最終出口（deep link 或 web URL） |
| app_type | VARCHAR(8) | YES | CodeMeta `AppType` value（`BUYER` / `SELLER`），DEFAULT `'BUYER'`。與 `client_type` 一同決定 redirect 目標 |
| version | INTEGER | YES | 樂觀鎖版本（DEFAULT 0） |

**索引：**
- `idx_payment_transactions_order_id (order_id)` — 訂單查詢
- `idx_payment_transactions_account_id (account_id)` — 帳號查詢
- `idx_payment_transactions_status (status)` — 狀態過濾
- `idx_payment_transactions_created_at (created_at)` — 時間範圍查詢
- `idx_payment_transactions_provider_trade_no (provider_trade_no)` — PayUni 交易號查詢
- `uq_payment_transactions_provider_trade_no UNIQUE (provider_trade_no) WHERE provider_trade_no IS NOT NULL` — PayUni 交易序號唯一性（§2A.3.6）
- `idx_payment_transactions_expire_at (expire_at) WHERE status = 0` — 逾期交易掃描
- `idx_payment_transactions_order_active UNIQUE (order_id) WHERE status IN ('0', '1') AND is_duplicate = FALSE` — **重複扣款防護**：同一訂單同時只能有一筆活躍交易（INIT 或 PAID）。🔴 述詞必須帶 `is_duplicate = FALSE`，否則 §2A.5.4 reactivate 路徑標記出的副 row 會佔住 slot，使復活寫入撞唯一鍵
- `idx_payment_transactions_duplicate (duplicate_of_payment_id) WHERE is_duplicate = TRUE` — 重複付款 row 反查（§2A.6.4）
- `idx_payment_transactions_reconcile (created_at, last_reconciled_at) WHERE status IN ('0', '3')` — L2 補救路徑掃描專用部分索引（§5.7.1）

**約束：**
- `chk_payment_type CHECK (payment_type IN (1, 2, 3, 4))`
- `chk_status CHECK (status IN (0, 1, 2, 3, 4, 5, 6))`
- `chk_amount_valid CHECK ((payment_type = '2900' AND amount = 0) OR (payment_type <> '2900' AND amount > 0))` — §2A.1：ZERO_AMOUNT 必為 0、其他付款方式必 > 0，雙向不變式由 DB 端硬保護
- `trade_no` 的 `UNIQUE` 成立且必須維持：§2A.3.6 MerTradeNo 序列化後每次嘗試的 `trade_no` 皆不同，而 §3.3 退款總額 trigger 依賴它取 `FOR UPDATE` 鎖

### 3.2 payment_logs

用於除錯、狀態衝突追蹤與爭議排查的**業務日誌**。主鍵使用 BIGSERIAL（內部日誌表）。

> **範圍界定**：本表是**純業務日誌**。PayUni 出向／入向互動的稽核紀錄一律寫專表 `payuni_audit_logs`（見 §11），**不得**寫入本表。
> - `log_type` 值域僅限業務型別（`GRANT_*`、`RESCUE`、`WEB_SELLER_FALLBACK` 等狀態機／救援／fallback 紀錄）。禁止新增 `PAYUNI_REQUEST` / `PAYUNI_RESPONSE` / `PAYUNI_ERROR` / `PAYUNI_INBOUND_WEBHOOK` / `PAYUNI_INBOUND_RETURN` 這五個 PayUni 互動型別。
> - `log_level` 值域為 `INFO` / `WARNING` / `ERROR`（DB CHECK 約束；本書其他處若出現 `WARN` 簡寫，以此三值為準）。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
| --- | --- | --- | --- |
| id | BIGSERIAL | PK | 自增主鍵 |
| trade_no | VARCHAR(32) | NO | 關聯交易號 |
| refund_no | VARCHAR(32) | NO | 關聯退款號 |
| log_type | VARCHAR(50) | YES | 日誌類型 |
| log_level | VARCHAR(10) | YES | INFO / WARNING / ERROR |
| message | TEXT | YES | 日誌內容 |
| from_status | SMALLINT | NO | 狀態變更前 |
| to_status | SMALLINT | NO | 狀態變更後 |
| trigger_source | VARCHAR(50) | NO | 觸發來源：SYNC_API / WEBHOOK / TTL_CONSUMER / MANUAL |
| conflict_detail | JSONB | NO | 衝突詳情（同步與 Webhook 結果不一致時記錄） |
| operator_id | VARCHAR(64) | NO | 人工介入操作者 |
| operator_reason | VARCHAR(500) | NO | 人工介入原因 |
| created_at | TIMESTAMPTZ | YES | 建立時間（DEFAULT NOW()） |

**索引：**
- `idx_payment_logs_trade_no (trade_no)` — 交易號查詢
- `idx_payment_logs_refund_no (refund_no)` — 退款號查詢
- `idx_payment_logs_log_type (log_type)` — 日誌類型過濾
- `idx_payment_logs_created_at (created_at)` — 時間範圍查詢
- `idx_payment_logs_cleanup (created_at, log_level)` — 清理排程用

### 3.3 payment_refunds

退款擁有獨立的生命週期，與交易主表分離管理。支援同一筆交易的多次/部分退款。主鍵使用 TSID。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
| --- | --- | --- | --- |
| id | BIGINT | PK | TSID（應用層生成） |
| refund_no | VARCHAR(32) | UK | 內部退款編號（格式：`RFD{yyyyMMddHHmmss}{random4}`） |
| payment_id | BIGINT | YES | FK → `payment_transactions(id)`（指向 PK，不依賴 `trade_no` 唯一性） |
| trade_no | VARCHAR(32) | YES | 關聯交易號（保留供查詢／顯示，本身無 FK） |
| idempotency_key | VARCHAR(64) | NO | 冪等鍵，由呼叫端傳入（AfterSales `case_no` 或 `cancel_id`），防止同一售後單重複退款 |
| order_id | VARCHAR(64) | YES | 訂單號（便於對帳/事件 payload） |
| amount | BIGINT | YES | 退款金額（新台幣元），CHECK (amount > 0) |
| status | VARCHAR(4) | YES | 退款狀態，值域＝ `RefundStatus` 六值：`0:REQUESTED`／`1:PROCESSING`／`2:SUCCEEDED`／`3:FAILED`／`4:APPROVED`／`5:MANUAL_REVIEW`。🔴 現行 `V1` 的 `chk_refund_status` 只收 `0~3`，擴到六值需 expand migration（見下方 CHECK 說明） |
| reason | VARCHAR(500) | NO | 退款原因 |
| provider_refund_no | VARCHAR(64) | NO | PayUni 退款序號 |
| provider_status | VARCHAR(20) | NO | PayUni 退款狀態原始值（遵循收斂規範 §4.1 SoT；單一欄位雙階段寫入規則見規範 §4.4）|
| provider_message | TEXT | NO | PayUni 回傳訊息（型別為 TEXT，避免長訊息截斷）|
| estimated_refund_date | DATE | NO | ATM 退款同步受理時 PayUni 回報的預計扣款日（里程碑，僅 ATM 有值）|
| provider_estimated_refund_at | TIMESTAMPTZ | NO | `EstimatedRefundDate` 解析後的時間戳（§10.5；gRPC 以 epoch millis 回傳，Gateway 轉 ISO 8601）|
| bank_date | DATE | NO | ATM 退款實際到帳日（來自 webhook RefundStatus=1 時）|
| completed_at | TIMESTAMPTZ | NO | 退款完成時間 |
| created_at | TIMESTAMP | YES | 建立時間（DEFAULT NOW()） |
| updated_at | TIMESTAMP | YES | 更新時間（DEFAULT NOW()） |
| version | INTEGER | YES | 樂觀鎖版本（DEFAULT 0） |

**索引：**
- `idx_payment_refunds_payment_id (payment_id)` — 依交易 PK 查退款
- `idx_payment_refunds_trade_no (trade_no)` — 原交易號查詢
- `idx_payment_refunds_order_id (order_id)` — 訂單號查詢
- `idx_payment_refunds_status (status)` — 狀態過濾
- `idx_payment_refunds_created_at (created_at)` — 時間範圍查詢
- `idx_payment_refunds_idempotency_key UNIQUE (idempotency_key) WHERE idempotency_key IS NOT NULL` — 冪等鍵唯一性

**約束：**
- `chk_refund_status CHECK (status IN (0, 1, 2, 3, 4, 5))` — 值域必須涵蓋 `RefundStatus` 的全部六個 CodeMeta 值（含 `APPROVED(4)`／`MANUAL_REVIEW(5)`）。🔴 CHECK 少一個值即等於該狀態無法落地，而呼叫端只會看到 DB 約束違反
- `chk_refund_amount_positive CHECK (amount > 0)`
- `FOREIGN KEY (payment_id) REFERENCES payment_transactions(id)` ON DELETE RESTRICT

**冪等查詢：**
- `findByProviderRefundNo(providerRefundNo)` — 依 PayUni 退款編號查詢，用於退款回調冪等檢查（若已存在且為終態則跳過處理）

### 3.4 payment_webhooks

紀錄從 MQ 消費的每一筆 PayUni 付款事件（含處理狀態），用於除錯、冪等控制與事後稽核。主鍵使用 BIGSERIAL。

透過 `webhook_type` + `idempotency_key` 組合實現冪等性保護。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
| --- | --- | --- | --- |
| id | BIGSERIAL | PK | 自增主鍵 |
| webhook_type | VARCHAR(50) | YES | Webhook 類型（DEFAULT `'8241'`＝`WebhookProvider.PAYUNI`） |
| trade_no | VARCHAR(32) | NO | 關聯交易號 |
| provider_trade_no | VARCHAR(64) | NO | PayUni 交易序號 |
| idempotency_key | VARCHAR(128) | NO | 冪等鍵（`{MerTradeNo}:{TradeNo}`） |
| raw_request | TEXT | YES | 原始 MQ 事件內容 |
| decrypted_payload | JSONB | NO | 解密後資料（由 webhook-service 解密） |
| is_valid | BOOLEAN | YES | 驗證是否通過（DEFAULT FALSE） |
| is_processed | BOOLEAN | YES | 是否已處理（DEFAULT FALSE） |
| process_status | VARCHAR(20) | YES | 處理狀態（DEFAULT `'8201'`＝`WebhookProcessStatus.PENDING`），見下方狀態表 |
| error_message | TEXT | NO | 錯誤或結果訊息 |
| received_at | TIMESTAMPTZ | YES | 接收時間（DEFAULT NOW()） |
| processed_at | TIMESTAMPTZ | NO | 處理完成時間 |
| source_ip | VARCHAR(45) | NO | 來源標記（MQ 時為 `MQ:{queue_name}`） |
| user_agent | VARCHAR(500) | NO | User-Agent |

**Webhook 類型（webhook_type）：**

| 值 | 說明 | idempotency_key 組合 |
|---|---|---|
| `PAYUNI_PAYMENT` | 付款結果通知 | `{MerTradeNo}:{TradeNo}` |

> **注意**：商家狀態異動（`PAYUNI_MER_STATUS`）已移至 `payuni-webhook-service` 直接發布至 `seller.review.result`（AGENT_NOTIFY 單通道），Payment Service 不再處理。

**處理狀態（process_status）：**

| 值 | 說明 |
|---|---|
| `PENDING` | 初始狀態（Inbox 已儲存，尚未處理） |
| `SUCCESS` | 處理成功（已更新交易狀態 + 發布 MQ 事件） |
| `FAILED` | 處理失敗（交易不存在、缺少欄位等） |
| `SKIPPED` | 跳過處理（交易已終態、狀態仍處理中等） |

**索引：**
- `idx_payment_webhooks_webhook_type (webhook_type)` — 類型過濾
- `idx_payment_webhooks_trade_no (trade_no)` — 交易號查詢
- `idx_payment_webhooks_received_at (received_at)` — 時間範圍查詢
- `idx_payment_webhooks_is_processed (is_processed) WHERE NOT is_processed` — 未處理記錄
- `UNIQUE (webhook_type, idempotency_key) WHERE is_processed = TRUE` — **冪等性保護**
- `idx_payment_webhooks_cleanup (processed_at) WHERE is_processed = TRUE` — 清理排程

> **注意**：本服務的 `outbox_events` 表（§3.5）為付款結果事件與 rescue refund 事件的可靠投遞載體。付款結果事件（`payment.success` / `payment.failed` / `payment.conflict`）由**六個發布點**經**兩收斂點**（A=`PaymentNotifyHandler`、B=`PaymentApplicationService.publishEvent`）統一走 `PaymentOutboxWriter.enqueueAndDispatch`：在「交易狀態落地（notify 另 + `payment_webhooks` 標記已處理）」同一事務內寫入 `outbox_events`（status=PENDING），由 **afterCommit 即時 dispatch**（正常路徑）+ `OutboxEventPublisherScheduler` 每 5s **兜底** 驅動 `PaymentOutboxPublisher` 以 publisher-confirm 確認投遞（見 §5.10）。**`payment.expired` 亦寫入本表**（`ExpiredTransactionScheduler` 走《[多副本排程搶佔規範](../開發規範/多副本排程搶佔規範.md)》例外 A，見 §5.7）。`payment.closed` / `refund.*` 仍由 `PaymentEventPublisher` 於 gRPC 觸發的事務內直接發布，不經 outbox。

### 3.5 outbox_events

事件可靠投遞載體。與 `payuni-webhook-service` 的同名表共用同一組 `OutboxEventStatus` CodeMeta 值。主鍵使用 BIGSERIAL。

| **欄位名稱** | **類型** | **必填** | **說明 / 備註** |
| --- | --- | --- | --- |
| id | BIGSERIAL | PK | 自增主鍵 |
| event_type | VARCHAR(64) | YES | 事件型別 |
| exchange | VARCHAR(64) | YES | 🔴 **投遞目標 exchange，每一列自己帶**（DEFAULT `'payment.events'`）。publisher **讀這一欄決定投遞目標，不得從 `routing_key` 推導**（隱藏對照表會讓新增 routing key 的人不知道要同步改它）。既有 payment 事件（`payment.success` / `payment.failed` / `payment.conflict` / `payment.expired` / `payment.rescue.*`）＝ `payment.events`；fulfillment responsibility 事件 ＝ `hawkr.shipping`。契約 SSOT ＝ 36 §3.3.1「Outbox persisted exchange」 |
| routing_key | VARCHAR(128) | YES | MQ routing key |
| payload | JSONB | YES | 事件內容 |
| status | VARCHAR(20) | YES | `OutboxEventStatus` value：`8221`=PENDING／`8222`=SENT／`8223`=FAILED／`8224`=DEAD（DEFAULT `8221`） |
| retry_count | SMALLINT | YES | 重送次數（DEFAULT 0） |
| next_retry_at | TIMESTAMPTZ | NO | 下次重送時間 |
| sent_at | TIMESTAMPTZ | NO | 投遞成功時間；🔴 **保留期清理的起點**（§5.8） |
| last_error | TEXT | NO | 最近一次投遞錯誤 |
| created_at | TIMESTAMPTZ | YES | 建立時間（DEFAULT NOW()） |

**索引：**
- `idx_outbox_events_pending (status, next_retry_at) WHERE status IN ('8221', '8223')` — 待重送掃描（PENDING / FAILED）

**約束：**
- `chk_payment_outbox_status CHECK (status IN ('8221', '8222', '8223', '8224'))`
- `chk_payment_outbox_exchange CHECK (exchange ~ '\S')` — 擋空字串與**任何** whitespace-only 值。🔴 **刻意不用 `btrim(exchange) <> ''`**：`btrim` 預設只剝空格，tab／換行組成的值會穿過去；`~ '\S'` 要求至少一個非空白字元，涵蓋 space／tab／CR／LF／FF／VT 全部

**exchange 欄的兩層防護不可互相取代：**

| 層 | 擋什麼 | 拿掉它會怎樣 |
|---|---|---|
| DB `DEFAULT 'payment.events'` | 「INSERT 時未提及該欄」——滾動更新視窗內舊版 pod 的寫入 | 舊 pod 的 outbox 寫入被 DB 拒絕，而 outbox 寫入與業務狀態同一交易 ⇒ 整筆業務交易 rollback ⇒ 對外 500，直到該 pod 被汰換 |
| publisher fail-closed | 「明確寫入 null／空白」——corrupt／手寫 SQL 造出的壞列 | 那些列被靜默發到錯的 exchange（DB DEFAULT 攔不到明確寫入的 null） |

🔴 **DEFAULT 不是 publisher 的 fallback**：36 §3.3.1 明令 publisher 對 null／空白 exchange **不投遞並標 FAILED**，不得 fallback 到預設值。factory 側的 exchange 是必填參數，漏傳即編譯錯。

### 3.6 索引設計總覽

以下為所有表索引的統一參考表：

| 表名 | 索引名稱 | 欄位 | 類型 | 用途 |
|------|----------|------|------|------|
| payment_transactions | `idx_payment_transactions_order_id` | `(order_id)` | B-tree | 訂單查詢 |
| payment_transactions | `idx_payment_transactions_account_id` | `(account_id)` | B-tree | 帳號查詢 |
| payment_transactions | `idx_payment_transactions_status` | `(status)` | B-tree | 狀態過濾 |
| payment_transactions | `idx_payment_transactions_created_at` | `(created_at)` | B-tree | 時間範圍查詢 |
| payment_transactions | `idx_payment_transactions_provider_trade_no` | `(provider_trade_no)` | B-tree | PayUni 交易號查詢 |
| payment_transactions | `uq_payment_transactions_provider_trade_no` | `(provider_trade_no) WHERE provider_trade_no IS NOT NULL` | Partial Unique | PayUni 交易序號唯一性 |
| payment_transactions | `idx_payment_transactions_expire_at` | `(expire_at) WHERE status = 0` | Partial | 逾期交易掃描 |
| payment_transactions | `idx_payment_transactions_order_active` | `(order_id) WHERE status IN ('0','1') AND is_duplicate = FALSE` | Partial Unique | 重複扣款防護 |
| payment_transactions | `idx_payment_transactions_duplicate` | `(duplicate_of_payment_id) WHERE is_duplicate = TRUE` | Partial | 重複付款 row 反查 |
| payment_logs | `idx_payment_logs_trade_no` | `(trade_no)` | B-tree | 交易號查詢 |
| payment_logs | `idx_payment_logs_refund_no` | `(refund_no)` | B-tree | 退款號查詢 |
| payment_logs | `idx_payment_logs_log_type` | `(log_type)` | B-tree | 日誌類型過濾 |
| payment_logs | `idx_payment_logs_created_at` | `(created_at)` | B-tree | 時間範圍查詢 |
| payment_logs | `idx_payment_logs_cleanup` | `(created_at, log_level)` | B-tree | 清理排程 |
| payment_refunds | `idx_payment_refunds_payment_id` | `(payment_id)` | B-tree | 依交易 PK 查退款 |
| payment_refunds | `idx_payment_refunds_trade_no` | `(trade_no)` | B-tree | 原交易號查詢 |
| payment_refunds | `idx_payment_refunds_order_id` | `(order_id)` | B-tree | 訂單號查詢 |
| payment_refunds | `idx_payment_refunds_status` | `(status)` | B-tree | 狀態過濾 |
| payment_refunds | `idx_payment_refunds_created_at` | `(created_at)` | B-tree | 時間範圍查詢 |
| payment_refunds | `idx_payment_refunds_idempotency_key` | `(idempotency_key) WHERE idempotency_key IS NOT NULL` | Partial Unique | 冪等鍵 |
| payment_webhooks | `idx_payment_webhooks_webhook_type` | `(webhook_type)` | B-tree | 類型過濾 |
| payment_webhooks | `idx_payment_webhooks_trade_no` | `(trade_no)` | B-tree | 交易號查詢 |
| payment_webhooks | `idx_payment_webhooks_received_at` | `(received_at)` | B-tree | 時間範圍查詢 |
| payment_webhooks | `idx_payment_webhooks_is_processed` | `(is_processed) WHERE NOT is_processed` | Partial | 未處理記錄 |
| payment_webhooks | （唯一索引） | `(webhook_type, idempotency_key) WHERE is_processed = TRUE` | Partial Unique | 冪等性保護 |
| payment_webhooks | `idx_payment_webhooks_cleanup` | `(processed_at) WHERE is_processed = TRUE` | Partial | 清理排程 |
| outbox_events | `idx_outbox_events_pending` | `(status, next_retry_at) WHERE status IN ('8221','8223')` | Partial | 待重送掃描 |

### 3.7 Migration 管理

所有表、索引、約束、觸發器的初始定義位於 `V1__init_payment_tables.sql`（5 張表：`payment_transactions`、`payment_logs`、`payment_refunds`、`payment_webhooks`、`outbox_events`，含全部業務索引、partial index、CHECK 約束與 `updated_at` 觸發器）。

🔴 **`V1__init_payment_tables.sql` 已凍結**（改動任一字元即破壞 Flyway checksum，服務啟動失敗）。所有 schema 變更一律新增 `V{N+1}__{verb}_{target}.sql`，禁止跳號、禁止 SSH 手跑 DDL 取代 migration。

**查目前最大版號**（⚠️ 不要用 `ls`：它是字典序，`V10` 會排在 `V1` 之後而看起來比 `V9` 小）：

```bash
ls services/java/payment-service/src/main/resources/db/migration/ \
  | sed -n 's/^V\([0-9]*\)__.*/\1/p' | sort -n | tail -1
```

流程見 [DB Migration 相容性規範（expand-contract）](../開發規範/DB%20Migration%20相容性規範（expand-contract）.md) 與 [38-Flyway 自動同步與 DB 自動初始化](./38-Flyway%20自動同步與%20DB%20自動初始化（Flyway%20Auto%20Sync%20%26%20DB%20Auto%20Init）詳細設計書.md)。

⚠️ `webhook_inbox.provider_event_at`（§10.5 寫入 PayUni 端事件時間）屬 **payuni-webhook-service** 的 schema，不在本服務 migration 內，見 [10-PayUni Webhook 服務](./10-PayUni%20Webhook%20服務%EF%BC%88payuni-webhook-service%EF%BC%89詳細設計書.md)。

---

## 4. 對外 / 對內介面定義

### 4.1 Gateway 對外 REST 映射

**需 JWT 認證（`/api/v1/app/payments`）：**

| HTTP Method | 路徑 | Handler | 對應 gRPC RPC | 說明 |
|-------------|------|---------|---------------|------|
| POST | `/api/v1/app/payments` | `CreatePayment` | `CreatePayment` | 建立支付交易 |
| GET | `/api/v1/app/payments/:trade_no` | `GetPayment` | `GetPayment` | 查詢支付詳情 |
| POST | `/api/v1/app/payments/:trade_no/status_syncs` | `SyncPaymentStatus` | `SyncPaymentStatus` | 同步支付狀態（呼叫 PayUni Query API） |
| PATCH | `/api/v1/app/payments/:trade_no` | `ClosePayment` | `ClosePayment` | 關閉交易 |
| GET | `/api/v1/app/payments/refunds/:refund_no` | `GetRefund` | `GetRefund` | 查詢退款詳情 |
| GET | `/api/v1/app/payments/:trade_no/refunds` | `GetRefundsByTradeNo` | `GetRefundsByTradeNo` | 依交易號查詢退款列表 |
| POST | `/api/v1/app/payments/apple_pay/validate_merchant` | `ValidateApplePayMerchant` | `ValidateApplePayMerchant` | Apple Pay on Web 取得 `merchantSession`（詳見 §5.11） |

> **退款不對買家開放**：原 buyer 路由 `POST /api/v1/app/payments/refunds → CreateRefund` **已移除**（Gateway 不再暴露該入口）。退款一律由買家提出售後申請 → AfterSales 審核核准後經 internal call 觸發 `CreateRefund`（見 §5.5.3），前端退款入口走售後申請流程，不再直呼付款服務退款。查詢類 `GET .../refunds/:refund_no`、`GET .../:trade_no/refunds` 保留供買家查自己的退款。

**頂層路由（無需 JWT）：**

| HTTP Method | 路徑 | Handler | 說明 |
|-------------|------|---------|------|
| GET | `/payment/redirect?id={redirectId}` | `RedirectToPayUni` | 信用卡 UPP 重導，APP WebView 開啟；`id` 為後端產生之不可猜 opaque token（短效 10 分鐘、一次性 consume），Gateway 依 id 呼叫 gRPC `GetPaymentRedirectForm` 取回 canonical 表單資料後 `html/template` render，query 不再攜帶任何表單資料（安全重構：query 不得攜帶表單資料） |
| POST | `/payment/return` | `PaymentReturnPost` | PayUni UPP 付款完成 Form POST 導回 → gRPC `HandlePaymentPageReturn` → 302 redirect Deep Link |
| GET | `/payment/return?tradeNo={tradeNo}` | `PaymentReturn` | 手動查詢付款狀態（備用）→ gRPC `GetPaymentReturnPage` → HTML |

**Gateway Handler DTO 透傳 provider_* 欄位**（遵循[收斂規範 §7](../開發規範/PayUni%20錯誤回應收斂規範.md)）：

| DTO | 欄位 | 適用層 | 說明 |
|---|---|---|---|
| `PaymentDetailResp` | `providerStatus` / `providerMessage` | App + Admin | 付款 PayUni 原始狀態與訊息 |
| `PaymentDetailResp` | `grantedAt` / `grantStatus` / `grantMessage` | **僅 Admin** | 撥款欄位封閉規則；App DTO 必須不帶（見規範 §6.2）|
| `RefundDetailResp` | `providerStatus` / `providerMessage` | App + Admin | 退款 PayUni 原始狀態 |
| `RefundDetailResp` | `estimatedRefundDate` / `bankDate` | App + Admin | ATM 退款日期（僅 ATM 有值）|

**實作策略（Gateway DTO 分層）**：

Proto `PaymentDetail` 保持**單一 message** 含所有欄位（grant_* / provider_* 都在裡面），Gateway handler 在 **DTO 映射層**分層控制欄位曝光：

- `PaymentDetailResp`（App 用）：從 proto 映射時**刻意省略** `granted_at` / `grant_status` / `grant_message` 三個欄位
- `AdminPaymentDetailResp`（Admin 用）：完整映射所有欄位，包含 grant_*

禁止前端 parser 誤解欄位：若 App DTO 欄位缺少代表不支援，不應使用 JSON omitempty 遮掩。

envelope 結構不變，provider 欄位放 `data.*`；錯誤時 `envelope.message` 透傳微服務 `resp.Message`（遵循規範 §7.1–§7.3）。Swagger 註解依 [Gateway Swagger 註解規範](../開發規範/Gateway%20Swagger%20註解規範.md) 同步，App / Admin DTO 各自有獨立 struct。

**Webhook 代理路由（無需 JWT）：**

| HTTP Method | 路徑 | 說明 |
|-------------|------|------|
| POST | `/webhook/payuni/notify` | 代理至 `payuni-webhook-service`（路徑改寫為 `/webhooks/payuni/notify`） |

### 4.2 gRPC Proto（PaymentService）

```protobuf
syntax = "proto3";
package io.github.will413028.hawkr.payment.grpc;

option go_package = "github.com/Will413028/hawkr/go-pkg/proto/payment";
option java_package = "io.github.will413028.hawkr.payment.grpc";
option java_multiple_files = true;

service PaymentService {
  rpc CreatePayment (CreatePaymentRequest) returns (CreatePaymentResponse);
  rpc GetPayment (GetPaymentRequest) returns (GetPaymentResponse);
  rpc GetPublicPaymentStatus (GetPublicPaymentStatusRequest) returns (GetPublicPaymentStatusResponse);
  rpc SyncPaymentStatus (SyncPaymentStatusRequest) returns (SyncPaymentStatusResponse);
  rpc ClosePayment (ClosePaymentRequest) returns (ClosePaymentResponse);
  // 依訂單編號關閉付款交易（Order Service 取消訂單時呼叫，內部 RPC）
  rpc ClosePaymentByOrderNo (ClosePaymentByOrderNoRequest) returns (ClosePaymentByOrderNoResponse);
  // 依訂單編號反查活躍/最終付款狀態（internal-only，Order Service 逾期取消前守門用，見 §5.12）
  // 免權限（isInternalCall()）；以 order_no 為鍵回傳該訂單 canonical 交易的付款狀態
  rpc GetActivePaymentStatusByOrderNo (GetActivePaymentStatusByOrderNoRequest) returns (GetActivePaymentStatusByOrderNoResponse);
  rpc CreateRefund (CreateRefundRequest) returns (CreateRefundResponse);
  rpc GetRefund (GetRefundRequest) returns (GetRefundResponse);
  rpc GetRefundsByTradeNo (GetRefundsByTradeNoRequest) returns (GetRefundsByTradeNoResponse);

  // ATM 退款頁面版 URL 組裝（AfterSales Service 呼叫，新增）
  rpc GetRefundPageUrl (GetRefundPageUrlRequest) returns (GetRefundPageUrlResponse);

  // 發動撥款指示（代理商模式：買家確認收貨後，由 Order Service 呼叫）
  rpc GrantPayment (GrantPaymentRequest) returns (GrantPaymentResponse);

  // 批次查詢付款 provider 狀態（Order 聚合用，遵循收斂規範 §6.3）
  rpc BatchGetProviderStatusByOrderIds (BatchGetProviderStatusByOrderIdsRequest) returns (BatchGetProviderStatusByOrderIdsResponse);

  // 批次查詢退款 provider 狀態 by case_ids（AfterSales 聚合用）
  rpc BatchGetRefundProviderStatusByCaseIds (BatchGetRefundProviderStatusByCaseIdsRequest) returns (BatchGetRefundProviderStatusResponse);

  // 批次查詢退款 provider 狀態 by order_ids（Order 聚合用，取每個 order 最新一筆 payment_refunds）
  rpc BatchGetRefundProviderStatusByOrderIds (BatchGetRefundProviderStatusByOrderIdsRequest) returns (BatchGetRefundProviderStatusResponse);

  // 付款結果頁面查詢（GET /payment/return?tradeNo=... 備用端點）
  rpc GetPaymentReturnPage (GetPaymentReturnPageRequest) returns (GetPaymentReturnPageResponse);

  // 信用卡 UPP 重導表單查詢（GET /payment/redirect?id=... internal-only，取回 canonical 表單並一次性 consume，見 §5.9）
  rpc GetPaymentRedirectForm (GetPaymentRedirectFormRequest) returns (GetPaymentRedirectFormResponse);

  // PayUni UPP 付款完成 Form POST 導回處理（POST /payment/return）
  // Gateway 提取 EncryptInfo + HashInfo → 此 RPC 驗簽解密 → 從 payment_transactions 讀取 client_type / app_type → 組 redirect_url
  // Gateway 不再傳 app_type；service 端從 DB lookup 取得（SoT 在 payment_transactions），詳見 §5.9
  rpc HandlePaymentPageReturn (HandlePaymentPageReturnRequest) returns (HandlePaymentPageReturnResponse);

  // 解密 PayUni 加密結果（通用工具 RPC，供 Gateway 在需要時呼叫）
  rpc DecryptPayUniResult (DecryptPayUniResultRequest) returns (DecryptPayUniResultResponse);

  // Apple Pay on Web Merchant Validation：對 Apple validationURL 取 merchantSession
  // 使用 Apple Pay Merchant Identity Certificate (PEM + Key) 進行 mTLS POST，原樣透傳 Apple 回應
  // 詳見 §5.11
  rpc ValidateApplePayMerchant (ValidateApplePayMerchantRequest) returns (ValidateApplePayMerchantResponse);
}

// === 型別說明（本檔不宣告 enum）===
// 🔴 PaymentType / TransactionStatus / RefundStatus 在 wire 上一律是 string，
//    內容為 CodeMeta value（api/proto/payment/payment.proto 逐字如此）。
//    值域 SSOT ＝ docs/代碼管理（CodeMeta）/業務狀態碼.md、業務類型碼.md，本檔不複製碼表。

// === 建立交易 ===
message CreatePaymentRequest {
  string order_no = 1;          // 訂單編號（必填，Payment Service 從 Order Service 查詢 mer_id + amount）
  string payment_type = 2;      // 支付方式 value（CodeMeta PaymentType）
  string expire_days = 3;      // 🔴 已停用：ATM 有效天數由後端 config `hawkr.payment.atm.expire-days` 決定，服務端不讀本欄位（見 §2A.5.5）。保留欄號以免重用
  string apple_pay_token = 4;
  string google_pay_token = 5;
  string bank_type = 6;        // ATM 銀行代碼
  string user_email = 7;
  int64 account_id = 8;        // Gateway JWT 注入（E.5.5 例外 #1）
  string client_type = 9;      // CodeMeta ClientType value（MOBILE_APP / WEB_BROWSER）；空字串視為 MOBILE_APP（向下相容）。決定 §5.9 PayUni 回調完成後最終出口
  string app_type = 10;        // CodeMeta AppType value（BUYER / SELLER）；空字串視為 BUYER（向下相容）。Gateway 由 JWT AccountType 推導後注入，禁止寫死字串字面量
  // mer_id、amount、product_name 由 Payment Service 從 Order Service 查詢，不由前端傳入
}
message CreatePaymentResponse {
  int32 code = 1;              // 0=成功
  string message = 2;
  PaymentResult data = 3;
}
message PaymentResult {
  string trade_no = 1;
  string order_id = 2;
  string payment_type = 3;      // 支付方式 value
  string amount = 4;           // 金額（新台幣元，數字字串格式）
  string status = 5;            // 交易狀態 value
  UppFormData upp_form_data = 6;
  string atm_bank_code = 7;
  string atm_virtual_account = 8;
  string atm_expire_date = 9;

  // 保留欄位編號 10, 11（已移除 CVS 相關欄位）
  reserved 10, 11;

  string expire_at = 12;       // Unix timestamp 毫秒（數字字串）
}
message UppFormData {
  string action_url = 1;
  string mer_id = 2;
  string version = 3;          // "2.0"
  string encrypt_info = 4;     // AES-256-GCM 加密
  string hash_info = 5;        // SHA256 簽章
  string is_platform = 6;      // "1"（代理商模式）
}

// === 公開查詢（免驗證，供付款結果頁面使用） ===
message GetPublicPaymentStatusRequest { string trade_no = 1; }
message GetPublicPaymentStatusResponse { int32 code = 1; string message = 2; PublicPaymentStatus data = 3; }
message PublicPaymentStatus {
  string trade_no = 1;
  string order_id = 2;
  string amount = 3;             // 金額（新台幣元，數字字串）
  string status = 4;            // 交易狀態 value
}

// === 查詢 / 同步 / 關閉 ===
message GetPaymentRequest { string trade_no = 1; }
message GetPaymentResponse { int32 code = 1; string message = 2; PaymentDetail data = 3; }
message SyncPaymentStatusRequest { string trade_no = 1; }
message SyncPaymentStatusResponse { int32 code = 1; string message = 2; PaymentDetail data = 3; }
message ClosePaymentRequest { string trade_no = 1; }
message ClosePaymentResponse { int32 code = 1; string message = 2; }
message ClosePaymentByOrderNoRequest { string order_no = 1; }
message ClosePaymentByOrderNoResponse { int32 code = 1; string message = 2; }

// 依訂單編號反查付款狀態（internal-only，§5.12）
message GetActivePaymentStatusByOrderNoRequest { string order_no = 1; }
message GetActivePaymentStatusByOrderNoResponse {
  int32 code = 1;
  string message = 2;
  // 該訂單 canonical 交易的付款狀態；查無交易時 data 為空、code 仍 0（訂單未建立付款交易屬正常情況）
  PaymentDetail data = 3;
}

// === 退款 ===
message CreateRefundRequest { string trade_no = 1; string amount = 2; string reason = 3; string refund_no = 8; /* 冪等鍵（新增） */ }
message CreateRefundResponse { int32 code = 1; string message = 2; RefundDetail data = 3; }
message GetRefundRequest { string refund_no = 1; }
message GetRefundResponse { int32 code = 1; string message = 2; RefundDetail data = 3; }
message GetRefundsByTradeNoRequest { string trade_no = 1; }
message GetRefundsByTradeNoResponse { int32 code = 1; string message = 2; repeated RefundDetail refunds = 3; }

// === 撥款指示 ===
message GrantPaymentRequest {
  string order_no = 1;          // 訂單編號（Payment Service 用 order_no 查詢對應的已付款交易）
}
message GrantPaymentResponse {
  int32 code = 1;               // 0=成功
  string message = 2;
  GrantPaymentResult data = 3;
}
message GrantPaymentResult {
  string trade_no = 1;          // 系統交易編號
  string order_id = 2;          // 訂單編號
  string provider_trade_no = 3; // PayUni UNi序號
  string amount = 4;            // 撥款金額（新台幣元，數字字串）
  string granted_at = 5;        // 撥款指示時間（ISO 8601）
}

// === ATM 退款頁面版 ===
message GetRefundPageUrlRequest {
  string trade_no = 1;                    // Payment trade_no
  string amount = 2;                      // 退款金額
  string return_url = 3;                  // PayUni 完成後導回 URL（Gateway 中繼 → URL Scheme 導回 APP）
}

message GetRefundPageUrlResponse {
  int32 code = 1;
  string message = 2;
  RefundPageData data = 3;
}

message RefundPageData {
  string redirect_url = 1;                // 組裝好的 redirect URL（含加密參數，前端用 WebView 開啟）
}

// === 信用卡 UPP 重導表單查詢（internal-only，§5.9）===
message GetPaymentRedirectFormRequest {
  string redirect_id = 1;    // opaque token（UUID，Redis key `payment:redirect:{id}` 一次性 consume）
}
message GetPaymentRedirectFormResponse {
  string action_url = 1;     // PayUni action URL（Gateway 端另需 host allowlist 驗證）
  string mer_id = 2;
  string version = 3;
  string encrypt_info = 4;
  string hash_info = 5;
  bool is_platform = 6;      // 非數字語意，保持 bool
}

// === 共用 ===
message PaymentDetail {
  string trade_no = 1;
  string order_id = 2;
  string mer_id = 3;
  string account_id = 4;
  string payment_type = 5;      // 支付方式 value
  string amount = 6;            // 金額（新台幣元，數字字串格式）
  string product_name = 7;
  string status = 8;            // 交易狀態 value
  string provider_trade_no = 9;
  string provider_status = 10;  // PayUni TradeStatus 原始值（收斂規範 §4.1 SoT）
  string provider_message = 11; // PayUni 回傳訊息
  string atm_bank_code = 12;
  string atm_virtual_account = 13;
  string atm_expire_date = 14;
  string pay_time = 15;        // Unix timestamp 毫秒（數字字串）
  string closed_at = 16;       // Unix timestamp 毫秒（數字字串）
  string expire_at = 17;       // Unix timestamp 毫秒（數字字串）
  string created_at = 18;      // Unix timestamp 毫秒（數字字串）
  string updated_at = 19;      // Unix timestamp 毫秒（數字字串）
  string granted_at = 20;      // 撥款成功時間（里程碑，Admin only 使用）
  string grant_status = 21;    // 撥款 PayUni 狀態（Admin only，不透傳給買/賣家）
  string grant_message = 22;   // 撥款 PayUni 訊息（Admin only）
}
message RefundDetail {
  string refund_no = 1;
  string trade_no = 2;
  string order_id = 3;
  string amount = 4;            // 退款金額（新台幣元，數字字串格式）
  string status = 5;            // 退款狀態 value（CodeMeta RefundStatus）
  string reason = 6;
  string provider_refund_no = 7;
  string provider_status = 8;   // PayUni 退款狀態原始值（單一欄位雙階段寫入：規範 §4.4）
  string provider_message = 9;  // PayUni 回傳訊息
  string completed_at = 10;    // Unix timestamp 毫秒（數字字串）
  string created_at = 11;      // Unix timestamp 毫秒（數字字串）
  string estimated_refund_date = 12; // ATM 預計扣款日（YYYY-MM-DD，僅 ATM sync success 有值）
  string bank_date = 13;        // ATM 實際到帳日（YYYY-MM-DD，僅 ATM async RefundStatus=1 有值）
}

// === 批次聚合（Order / AfterSales 異常列表聚合用，遵循收斂規範 §6.3）===

message ProviderStatusEntry {
  string provider_status = 1;
  string provider_message = 2;
}

message BatchGetProviderStatusByOrderIdsRequest {
  repeated string order_ids = 1;   // 建議單次 ≤ 100
}
message BatchGetProviderStatusByOrderIdsResponse {
  int32 code = 1;
  string message = 2;
  map<string, ProviderStatusEntry> entries = 3;  // key = order_id；找不到則省略
}

message BatchGetRefundProviderStatusByCaseIdsRequest {
  repeated string case_ids = 1;
}
message BatchGetRefundProviderStatusByOrderIdsRequest {
  repeated string order_ids = 1;
}
message BatchGetRefundProviderStatusResponse {
  int32 code = 1;
  string message = 2;
  map<string, ProviderStatusEntry> entries = 3;  // key = case_id 或 order_id
}

// === Apple Pay Web Merchant Validation（§5.11）===

message ValidateApplePayMerchantRequest {
  string validation_url = 1;  // Apple 提供的一次性 validationURL（host 限 *.apple.com 且 path 開頭 /paymentservices/）
  string domain_name = 2;     // 前端發起付款的網域（如 web-dev.example.com），必須在 allowlist 內
  string display_name = 3;    // Apple Pay sheet 顯示的商家名稱（一般為商品名稱或品牌名）
}
message ValidateApplePayMerchantResponse {
  int32 code = 1;
  string message = 2;
  string merchant_session_json = 3;  // Apple 回應原文 JSON，前端原樣傳入 completeMerchantValidation()
}
```

### 4.3 依賴的外部 gRPC

| 服務 | RPC | 用途 | 呼叫時機 |
|------|-----|------|----------|
| Order Service | `GetOrderForPayment(order_no)` | 查詢 `mer_id` + `amount` + `buyer_id` + `status` + `product_name` | `CreatePayment` 前置步驟 |

### 4.4 MQ 發布事件（Payment → 下游）

**Exchange：** `payment.events`

| Routing Key | 業務時機 | 下游處理 |
|---|---|---|
| `payment.success` | 付款成功 | Order 推進訂單為 PAID 並啟動下游 |
| `payment.failed` | 付款失敗 | Order 記錄失敗 |
| `payment.expired` | 交易逾期 | Order 標記訂單過期 |
| `payment.closed` | 交易關閉 | Order 標記訂單關閉 |
| `payment.conflict` | 狀態衝突 | Order 異常告警 |
| `refund.succeeded` | 退款成功（三個發布點：信用卡 sync 成功、ATM async webhook `RefundStatus=1`、FAILED→SUCCEEDED 衝突校正；ATM sync 受理不在此列，見 §4.4.2） | AfterSales `RefundResultConsumer` 依 payload 日期欄位 + task state 推進 `refund_task`（詳見 09 §4.5 處理規則）；Order log only |
| `refund.failed` | 退款失敗（兩個發布點：信用卡 sync 失敗、ATM async webhook `RefundStatus=2`） | AfterSales 依 §4.4.2 處置（ATM 退回 AWAITING_BUYER_REFUND_PAGE；信用卡 / Apple Pay / Google Pay 僅補寫 snapshot）；Order log only |

> **`aftersales.payment.refund.result` 是 queue 名，不是 routing key**。退款結果**沒有** `payment.refund.result` 這個 routing key（`MqConstants` 內不存在該常數）；exchange `payment.events` 上實際發出的只有 `refund.succeeded` / `refund.failed` 兩個 key（`MqConstants.ROUTING_KEY_REFUND_SUCCEEDED` / `ROUTING_KEY_REFUND_FAILED`，由 `PaymentEventPublisher.getRoutingKey` 對應）。`aftersales.payment.refund.result`（AfterSales `RefundResultConsumer`）與 `order.refund.result`（Order log-only consumer）是各自 binding 這兩個 key 的 **queue 名稱**，定義在 `infra/mq/topology/topology.yaml`。拿 queue 名當 routing key 去建 binding ⇒ 綁不到任何訊息，**broker 不會報錯，訂閱靜默失效**。

> 完整 producer / consumer / payload class / DLQ / idempotency 設計見
> [36-MQ 與事件總覽 §3.2 支付領域](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md#32-支付領域payment-domain)。
>
> rescue refund 路徑（Outbox Pattern）見 §2A.6。

> **投遞語意**：`payment.success` / `payment.failed` / `payment.conflict` 由**六發布點**經**兩收斂點**（A=`PaymentNotifyHandler`、B=`PaymentApplicationService.publishEvent`）統一走 `PaymentOutboxWriter.enqueueAndDispatch`：於交易狀態落地事務內經 `outbox_events` 寫入、afterCommit 即時 + 排程器每 5s 兜底、由 `PaymentOutboxPublisher` 以 publisher-confirm 確認投遞（**原子化 + 即時履約 + at-least-once 確認**，見 §5.10）。**`payment.expired`（`ExpiredTransactionScheduler`）同樣走這條 outbox**（該處的 CAS 是永久狀態轉換，且 publish 例外被同方法內的 catch 吞掉使 `@Transactional` 形同虛設，見 §5.7）。`payment.closed`（`PaymentCloseService`）/ `refund.succeeded` / `refund.failed` 仍由 `PaymentEventPublisher`（本體不改）於 gRPC 觸發的事務內直接發布（失敗即 rollback），不經 outbox。下游冪等（`payment_event_dedup`，`trade_no` 單欄）不受投遞方式改變影響。

#### 4.4.1 `refund.succeeded` / `refund.failed` payload 規範（遵循收斂規範 §5.2）

🔴 **下表的欄名就是 wire 上的實際鍵（camelCase）**，由 `PaymentDomainEvent.RefundSucceeded#payload` / `RefundFailed#payload` 送出。

| wire 欄位 | 類型 | 哪個事件有 | 來源 |
|---|---|---|---|
| `refundNo` | string | 兩者 | 本地退款號 |
| `tradeNo` | string | 兩者 | 本地交易號 |
| `orderId` | string | 兩者 | 訂單號 |
| `amount` | number | 兩者 | 退款金額（元）；wire 上是 JSON 數值（Java `long`），不是字串 |
| `providerRefundNo` | string | 僅 `refund.succeeded` | PayUni 退款單號 |
| `payuniStatus` | string | 兩者 | PayUni 原始狀態碼。寫入映射遵循規範 §4.4：sync 寫 response.Status；async webhook 寫 **inner RefundStatus**（1/2）|
| `payuniMessage` | string | 兩者 | PayUni 原始訊息 |
| `estimatedRefundDate` | string | 僅 `refund.succeeded`；**恆為空字串** | 設計上為 PayUni response `EstimatedRefundDate`（ISO 8601 YYYY-MM-DD），但 ATM sync 受理不經 MQ（見 §4.4.2），三個 `refund.succeeded` 發布點傳進來的都是 null |
| `bankDate` | string | 僅 `refund.succeeded`（ATM async RefundStatus=1）| PayUni webhook `BankDate`（ISO 8601 YYYY-MM-DD）|
| `completedAt` | string | 僅 `refund.succeeded` | 退款完成時間（UTC ISO 8601）|
| `occurredAt` | string | 兩者 | UTC ISO 8601 event 發出時間 |

**沒有 `status` 欄位**：退款成敗一律由 routing key（`refund.succeeded` / `refund.failed`）判定，兩個 payload 都不帶平台退款狀態。

**「僅某事件有」的欄位在該事件內恆存在**：publisher 對 null 一律送空字串（不是省略 key），所以 consumer 判斷有無要看「是否為空字串」而非「key 是否存在」。

**不加 `phase` 欄位**：consumer 靠 `estimated_refund_date` / `bank_date` 是否為空判斷階段即可，PayUni 本身也沒這概念。

> 🔴 **`refund.failed` 的欄位子集**：只有 `refundNo` / `tradeNo` / `orderId` / `amount` / `payuniStatus` / `payuniMessage` / `occurredAt`。
>
> 🔴 **改鍵名前必須同批改 consumer**：aftersales `RefundResultConsumer.onRefundResult` 讀 `refundNo` / `tradeNo` / `payuniStatus` / `payuniMessage` / `estimatedRefundDate` / `bankDate`（成敗取自 routing key；`estimatedRefundDate` 恆為空 ⇒ 它的「ATM 同步受理」分支走不到）。`orderId` / `amount` / `providerRefundNo` / `completedAt` 目前無人讀取。
>
> ⚠️ **[36 §7.4.1](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md) 的 snake_case 是本事件鏈的目標命名，遷移計畫列於 36 §7.6.4。在 publisher 遷移之前，consumer 一律照上表的 camelCase 讀**——照目標命名改 consumer 會讓它讀到 `undefined`，而 MQ 消費失敗是靜默的。
>
> ⚠️ order-service 的 `RefundResultConsumer` 是 log-only，它讀的 `refund_no` 是 snake 鍵、publisher 從未送出 ⇒ 該筆 log 的欄位恆為空字串。**不要把它當成「wire 上已是 snake_case」的證據。**

#### 4.4.2 發布時機矩陣

| 支付方式 | 階段 | 結果 | 是否發 event | Consumer 預期動作 |
|---|---|---|---|---|
| 信用卡 | sync | 成功 | ✅ | 推 task state → COMPLETED |
| 信用卡 | sync | 失敗（第一次或 retry 後）| ✅ | 推 task state → FAILED（升級異常）|
| ATM | sync | 成功 | ❌ 不發 | 由 AfterSales `handleRefundPageReturn` 就地推進 task state → AWAITING_WEBHOOK 並寫入 estimated_refund_date，另以 gRPC `AckAtmRefundAccepted` 通知 Payment 推進 PROCESSING |
| ATM | sync | 失敗 | ❌ 不發 | 同樣由 AfterSales `handleRefundPageReturn` 就地處置：**更新 `payuni_status` / `payuni_message` snapshot，task state 退回 AWAITING_BUYER_REFUND_PAGE**（讓買家下次查詢仍看到錯誤原因與重填按鈕）；REF02037 例外走對帳不重填 |
| ATM | async webhook | RefundStatus=1 | ✅ | 推 task state → COMPLETED，寫入 bank_date |
| ATM | async webhook | RefundStatus=2 | ✅ | 發 `refund.failed`，AfterSales 依 [09 §5.4.3.1](./09-售後微服務（AfterSales%20Service）詳細設計書.md#5431-atm-退款失敗處置的健壯化issue-1288) 對帳處置（不再一律 FAILED；REF02037 不重填、對帳無法判定→MANUAL_INTERVENTION）|

> **為什麼 ATM sync 兩種結果都不走 MQ**：PayUni 頁面版是把使用者導回 **AfterSales** 的 `handleRefundPageReturn`，而 AfterSales 本身就是 `refund_tasks` 的 owner —— snapshot 與 task state 在同一個呼叫裡就地寫完（持久 UX），URL scheme 同時回前端（即時 UX），不需要繞 Payment → MQ → AfterSales。Payment 側只有「受理成功」需要同步 PROCESSING，走 gRPC `AckAtmRefundAccepted`。

> **注意**：商家狀態異動事件（`seller.events / seller.review.result`）已移至 `payuni-webhook-service` 直接發布（AGENT_NOTIFY 單通道），Payment Service 不再負責 seller.events 的生產。

#### 4.4.3 退款通知的 template 歸屬

🔴 **`refund.succeeded`（買家）／`refund.succeeded.seller`／`payment.rescue.refund_completed.seller` 三個 notification template 不存在，且不得新增。** 它們的 `target_user_field`（`buyerAccountId` / `sellerAccountId`）不在對應的 payment payload 內（`RefundSucceeded.payload()` 只發 `orderId`／`refundNo`／`amount`…；rescue payload 只有 `buyerAccountId`），`extractUserID` 找不到收件人 ⇒ 建了也是零投遞。

三條退款來源的通知歸屬：

| 退款來源 | 是否經 aftersales case | 通知 template |
|---|---|---|
| 售後 case 退款（RefundTaskWorker / AfterSaleResolutionService / SellerCancelService） | 是 | `aftersales.refund.completed.buyer` / `.seller`（aftersales 發，target 由 case 反查） |
| 直接退款（`POST /api/v1/app/payments/refunds`） | 否 | 無（無前端觸發；買家退款一律走售後申請） |
| Rescue 重複付款自動退款（order AutoRefundWorker） | 否 | `payment.rescue.refund_completed.buyer`（target `buyerAccountId` 已在 payload 內） |

**保留 event 本身**：`refund.succeeded` / `refund.failed` 兩個 event 必須繼續發布——order 的 log-only consumer 與 aftersales `RefundResultConsumer`（推進 refund_task）依賴它們，那是業務 consumer 不是通知。

⚠️ **不要為 payment 端的退款通知建賣家識別 enrich**：payment 不持有 `store_id`，重複付款退的是副交易、主交易與賣家撥款不受影響。

### 4.5 MQ 消費事件（下游 → Payment）

Payment Service **不再直接接收 PayUni HTTP Webhook**。改為透過 RabbitMQ 消費 `payuni-webhook-service` 發布的事件。

| Routing Key | Exchange | 業務語意 | payment-service 處理 |
|---|---|---|---|
| `payuni.payment.uop` | `payuni.events` | PayUni UOP 付款結果（信用卡 / ATM sync / 退款 sync）| 解析 → 推進 `payment_transactions` 狀態 → publish 對應 `payment.*` event |

> 完整 queue / DLQ / payload envelope / idempotency 設計見
> [36-MQ 與事件總覽 §3.5 PayUni Webhook 領域](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md#35-payuni-webhook-領域payuni-webhook-domain)。

### 4.6 錯誤碼定義

**重複付款相關錯誤碼：**

| 攔截層 | 異常類型 | gRPC 回傳碼 | 回傳訊息 |
|--------|---------|------------|---------|
| 應用層（PAID 狀態） | `IllegalStateException` | `409 CONFLICT` | `重複付款` |
| DB 層（唯一約束） | `DataIntegrityViolationException` | `409 CONFLICT` | `重複付款` |
| 應用層（訂單狀態不允許） | `IllegalStateException` | `INVALID_PARAM` | 原始訊息 |

**撥款指示錯誤碼：**

| 情境 | 回傳訊息 | 備註 |
|------|---------|------|
| 交易不存在或非 PAID 狀態 | `交易不存在或尚未付款` | — |
| provider_trade_no 為空 | `缺少 PayUni 交易編號，無法撥款` | — |
| PayUni Grant API **明確未發動**（明確拒絕碼 / 參數錯，retry 一次後仍失敗）| PayUni 錯誤訊息 | 寫入 `grant_status`=PayUni 錯誤碼 / `grant_message`；進 Admin 異常清單（見 §5.6 去重硬化 A） |
| PayUni Grant API **模稜兩可**（timeout / 連線中斷 / `PARSE_ERROR` / `HASH_INVALID` / 未知碼）| PayUni 原因訊息 | **不自動重試**，寫入 `grant_status`=`PENDING_CONFIRM` / `grant_message`；進 Admin，下輪重試由 grant_query 對帳把關（見 §5.6 去重硬化 A/B） |

**退款相關錯誤碼：**

| 情境 | 回傳訊息 |
|------|---------|
| 交易請款已取消 | `交易請款已取消，無法退款` |
| 交易尚未請款 | `交易尚未請款，無法退款` |
| ATM 退款走錯路徑 | `ATM 退款請使用 GetRefundPageUrl 走頁面版` |
| 未知的請款狀態 | `未知的請款狀態: {closeStatus}` |

**Apple Pay Merchant Validation 錯誤碼（詳見 §5.11.7）：**

| 情境 | 對應 CodeMeta | 統一回傳訊息（CodeMeta 預設） |
|---|---|---|
| (a) validationURL scheme ≠ https / host 不以 `.apple.com` 結尾 / path 不開頭 `/paymentservices/` / 含 userInfo / port ≠ 443 / 長度超限；(b) `domain_name` 不在 `APPLE_PAY_DOMAIN_ALLOWLIST` 內 | `APPLE_PAY_VALIDATION_URL_REJECTED` | `Apple Pay 驗證網址不符合白名單` |
| PEM cert / Private Key 載入失敗（檔不存在、PKCS#8 解析錯誤、不支援的 PKCS#1 格式、cert 與 key 不配對） | `APPLE_PAY_CERT_LOAD_FAILED` | `Apple Pay 憑證載入失敗` |
| Apple 回非 2xx 或網路 / TLS 錯誤 | `APPLE_PAY_MERCHANT_VALIDATION_FAILED` | `Apple Pay merchant 驗證失敗` |

> 8028 兩種情境（SSRF 白名單 + domain allowlist）共用同一訊息，刻意不細分以縮小 attack surface（不洩漏「網址錯」還是「網域錯」的訊息差異給呼叫端）。後端 log 帶具體拒絕原因（event `validate.rejected` 含 reason 欄位）供運維查 trace。

文案分流由前端依 code 處理（後端僅提供 code + fallback 訊息）。詳見 §5.11.7。

### 4.7 冪等鍵與重試語意

| 場景 | 冪等鍵 | 冪等機制 | 說明 |
|------|--------|----------|------|
| CreatePayment | `order_id`（活躍交易） | `findActiveByOrderId` + Partial Unique Index | PAID 拒絕、INIT 回傳既有交易 |
| Webhook/Return 處理 | `{MerTradeNo}:{TradeNo}` | `payment_webhooks` 唯一索引 `WHERE is_processed = TRUE` | 先到先處理，後到跳過 |
| CreateRefund | `refund_no`（呼叫端傳入，落 DB 的 `payment_refunds.idempotency_key`） | `idx_payment_refunds_idempotency_key` 唯一索引 | SUCCEEDED/PROCESSING 回傳既有結果，FAILED 允許重試 |
| ClosePaymentByOrderNo | `order_no` | 無活躍交易 / 已關閉 / 已非 INIT 皆視為成功 | 無論呼叫幾次，結果一致 |
| GrantPayment | `order_no`（交易維度） | `findActiveByOrderIdForUpdate`（`FOR UPDATE` 鎖交易 row）+ `grant_status == SUCCESS` 短路 + **grant_query 對帳去重**（重試發動前查 PayUni 撥款入帳明細） | 已撥款直接回傳既有 `GrantPaymentResult`，不重打 PayUni；擋時序型重撥（partial-success 重試 / in-method 重試——模稜兩可標 `PENDING_CONFIRM` 不自動重試），併發型重撥由上游 `grant_payment_tasks` 去重，見 §5.6 |

**PayUni 呼叫重試語意（遵循 [收斂規範 §8](../開發規範/PayUni%20錯誤回應收斂規範.md)）：**

| 呼叫類型 | 策略 |
|---------|------|
| 主流程同步 PayUni 呼叫（建單 / 退款 / 撥款 / 查詢）| 呼叫當下同步 retry **最多一次**；失敗立即寫入對應表的 `provider_*` / `grant_*` 並拋業務例外 |
| ATM async PayUni webhook | 依既有 MQ 消費重試機制，不新增 retry |
| L2 補救（既有保留）| `PendingPaymentReconcileScheduler`（§5.7.1）/ `RescueRefundConsumer`（§5.5 退款救援），屬補救設計不推翻 |
| Task worker 層 retry | AfterSales `refund_tasks` Worker 對**信用卡**退款失敗自動 retry 一次（ATM 不做）；詳見 09-售後設計書 |

**禁止事項**：
- 禁止在 `payment_transactions` / `payment_refunds` 等 state 表新增 `retry_count` / `next_retry_at` / `*_error_type` 欄位
- 禁止為主流程新建排程 scheduler 做 PayUni 呼叫重試
- 禁止提供「手動重試 PayUni 呼叫」API；人工介入走 Admin 異常清單 + PayUni 後台 + webhook 閉環

---

## 5. 業務流程詳細設計

> **PayUni Client 統一性說明**：本章節所有提到的 `PaymentClient` 均指 `services/java/shared/payuni-java-client` 提供的 `io.github.will413028.hawkr.payuni.payment.PaymentClient`，回應型別 `PayUniApiResponse` 為 `io.github.will413028.hawkr.payuni.http.PayUniApiResponse`。payment-service **不再保留任何自製 PayUni client / cryptoService / DTO**，所有 PayUni 通訊統一走 shared SDK（共用模組優先：新增 code 前必先查 `services/java/shared/*` 與 `pkg/*`，禁止重複實作，見 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md)）。

### 5.1 建立交易（CreatePayment）

所有支付方式在建立交易前，**必須先從 Order Service 取得訂單付款資訊，再執行冪等性檢查**（防止重複扣款）：

**前置步驟 0：查詢訂單付款資訊（所有支付方式共用）**

1. 呼叫 **Order Service** `GetOrderForPayment(order_no)` → 取得 `mer_id` + `amount` + `buyer_id` + `status` + `product_name`
2. **驗證訂單狀態**：`status` 必須為 `PENDING_PAYMENT`，否則拒絕（訂單已取消/已付款等）
3. **驗證身分**：訂單的 `buyer_id` 必須與請求中的 `buyer_id` 一致（兩邊都是 Gateway `buyerid` resolver 解析後的現任 buyer surrogate，非登入 `account_id`；防止他人替付/竄改）

> **安全設計**：`mer_id`（PayUni 商家編號）和 `amount`（付款金額）全程由後端控制。前端僅傳 `order_no` 和 `payment_type`，Payment Service 從 Order Service 取得真實資料，避免前端竄改商家編號或金額。

**前置步驟 1：冪等性檢查（所有支付方式共用）**

1. 驗證參數：order_no 不為空、從 Order Service 取得的 amount > 0
2. **查詢活躍交易**：`findActiveByOrderId(orderNo)` — 查詢 `status IN (0:INIT, 1:PAID)` 的既有交易
3. **若找到 PAID 交易** → 拋出異常「訂單已付款，不可重複建立支付」（阻斷重複扣款）
4. **若找到 INIT 交易** → 先依 §2A.4.1 判定是否已過 `expire_at`（已過則標 EXPIRED 並走新建流程）；仍有效才回傳既有交易資訊（冪等性回應，不呼叫 PayUni）：
   - 信用卡：以同一 `trade_no` 重新產生 UPP Form（PayUni 視為同一筆交易）
   - ATM：回傳已存儲的虛擬帳號（atm_bank_code / atm_virtual_account / atm_expire_date）
   - Apple Pay / Google Pay：回傳既有交易狀態
5. **若無活躍交易** → 進入下方正常建立流程

根據支付方式不同，分為三種流程：

> **代理商撥款控制**：所有支付方式在建立交易時，EncryptInfo 內層一律帶入 `GrantExport=1`，告知 PayUni 此筆交易由代理商控制撥款時機，PayUni 不會自動撥款給商店。待買家確認收貨後，再由 Payment Service 呼叫 Grant API 發動撥款指示（見 §5.6）。

**A) 信用卡（UPP 重導模式）**

1. 生成 TSID ＋ `trade_no`（格式 **`{orderNo}{序號}`**，序號取 `MAX(attempt_seq)+1` 並補零至 3 位；規則見 §2A.3.6）
2. DB：INSERT payment_transactions，status = INIT，`expire_at = nowUtc() + 600s`（同時送 PayUni `TradeLExpireSec=600`，見 §2A.4）
3. 呼叫 `PaymentClient.createCreditCardPayment()`：
   - 組裝參數（含 `GrantExport=1`）→ AES-256-GCM 加密 → SHA256 簽章（委託 `payuni-java-client` SDK）
   - 回傳 `UppFormData`（action_url + encrypt_info + hash_info + is_platform="1"）
4. 回傳 gRPC：UppFormData 供前端 Form POST 導向 PayUni 支付頁

**B) Apple Pay / Google Pay（幕後即時付）**

1. 同上步驟 1-2
2. 呼叫 `PaymentClient.createApplePayPayment()` 或 `createGooglePayPayment()`（含 `GrantExport=1`）
3. PayUni 同步回傳結果 → 立即更新交易狀態（PAID / FAILED）
4. 若成功 → 直接發布 MQ 事件（PAYMENT_SUCCESS）到 `payment.events`
5. 回傳 gRPC：包含最終狀態

**C) ATM（轉帳繳費模式）**

1. 同上步驟 1-2
2. 呼叫 `PaymentClient.createAtmPayment()`（含 `GrantExport=1`）
3. PayUni 回傳虛擬帳號 → 更新 `atm_bank_code` / `atm_virtual_account` / `atm_expire_date`，並寫入 `provider_trade_no`（同步回應的 `TradeNo`；漏寫會使 webhook 段 1 精準查找失效，見 §2A.3.6）
4. 設定 `expire_at`：**一律以 PayUni response 的 `ExpireDate` 經 `parsePayUniDateTimeToUtc` 轉 UTC**，🔴 **禁止自行推算**（見 §2A.5.5）
5. 回傳 gRPC：包含繳費資訊

### 5.2 接收付款通知（MQ Consumer + Handler 架構）

由 `PayUniWebhookConsumer`（MQ 消費者）→ `WebhookNotifyHandler`（業務處理器）處理。

採用 **Strategy Pattern**：Consumer 負責統一的前置流程（MQ 接收、冪等、Inbox），Handler 負責付款結果的業務邏輯。

**5.2.1 架構概覽**

```
payuni-webhook-service (HTTP 入口，驗簽/解密/去重)
  └─ MQ: payuni.events / payuni.payment.uop
       └─ PayUniWebhookConsumer (MQ 消費者，MANUAL ACK)
            ├─ 冪等檢查（payment_webhooks）
            ├─ 儲存 Inbox 記錄
            ├─ TransactionTemplate（Handler + markProcessed）
            └─ WebhookNotifyHandler (策略介面)
                 └─ PaymentNotifyHandler (付款結果)
```

**Handler 介面契約：**

```java
public interface WebhookNotifyHandler {
    WebhookType webhookType();
    String extractIdempotencyKey(Map<String, String> decryptedData);
    WebhookHandleResult handle(Map<String, String> decryptedData);
}
```

**WebhookHandleResult：**

```java
public record WebhookHandleResult(
    String processStatus,       // SUCCESS / FAILED / SKIPPED
    String message
) {}
```

> **付款結果事件走 outbox（收斂點 A）**：`PaymentNotifyHandler.handle` 是六發布點中三 notify 入口的收斂點 A（收斂點 B=`PaymentApplicationService.publishEvent`，見 §5.10.1）。`handle` 內判定終態後，**在同一 DB 事務內**經 `PaymentOutboxWriter.enqueueAndDispatch(event)` 將 `payment.success` / `payment.failed` / `payment.conflict` 寫入 `outbox_events`（PENDING）並註冊 afterCommit dispatch，與交易狀態落地、`markAsProcessed` 三者原子。因此 `WebhookHandleResult` **不再攜帶 `MqEventData`**、呼叫端（三 notify 入口）**不再於事務外直發**（原「回傳 mqEvent → Consumer TX 外 publish」模式已移除）。envelope 組裝集中於 `PaymentOutboxWriter`（**統一補齊 notify 路徑原缺的 `event_id`**；序列化失敗即 throw 讓事務 rollback，禁止 catch + return 造成 UPDATE 落地但 outbox 漏寫的原子化破口，對齊 §2A.6.5）。

**5.2.2 Consumer 統一流程**

1. **接收 MQ 事件**：從 `payment.payuni.webhook` Queue 接收 Event Envelope
2. **提取 payload**：使用 `PayUniWebhookEnvelope.parse(body)` 統一解析 → `env.payload()` 取得 `Map<String, String>` 解密後資料
3. **判斷 Handler**：根據 `WebhookType.PAYUNI_PAYMENT` 查找對應 Handler
4. **提取冪等鍵**：`handler.extractIdempotencyKey(decryptedData)` → `{MerTradeNo}:{TradeNo}`
5. **冪等檢查**：查詢 `payment_webhooks (webhook_type, idempotency_key) WHERE is_processed = TRUE`
   - 已處理 → ACK + 跳過
6. **儲存 Pending Inbox**：寫入 `payment_webhooks`（`process_status = '8201'` ＝ `WebhookProcessStatus.PENDING`）— **在 Transaction 外**
7. **TransactionTemplate**（單一 DB Transaction，三寫入原子化）：
   - `handler.handle(decryptedData)` → 交易狀態落地（`updateToPaid` / `updateToFailed` / `updateToConflict`）**＋** `PaymentOutboxWriter.enqueueAndDispatch(event)` 寫 `outbox_events`（status=PENDING）並註冊 afterCommit dispatch
   - `webhookMapper.markAsProcessed(webhookId, processStatus, message)`
8. **afterCommit 即時投遞**：事務 commit 後同執行緒立即 `PaymentOutboxPublisher.publishConfirmed`（吞例外不 rethrow；正常路徑即時履約，失敗交排程器兜底，見 §5.10.1）
9. **MANUAL ACK**：`channel.basicAck(deliveryTag, false)`
10. **異常處理**：`channel.basicNack(deliveryTag, false, true)` — 拒絕並重新排隊

> 付款結果事件的 MQ 投遞由步驟 8 的 **afterCommit 即時 dispatch**（正常路徑）+ `OutboxEventPublisherScheduler` 每 5s **兜底**（afterCommit 失敗時）雙路徑完成，皆走 `PaymentOutboxPublisher` publisher-confirm（見 §5.10）；下游 `trade_no` dedup 吸收 at-least-once 重送。

> **設計要點**：步驟 6 在 Transaction 外儲存 Inbox，確保即使 Handler 失敗也能保留記錄供稽核。步驟 7 在同一 Transaction 內完成「交易狀態落地 ＋ outbox 事件寫入 ＋ 標記已處理」**三者原子**——rollback 時三者皆不落地、commit 時三者一併落地，**杜絕「狀態已落地但事件未發」的雙寫非原子破口**。步驟 8 的 afterCommit 只在 commit **成功後**才即時投遞（消除 over-fulfillment），且 callback 吞例外不 rethrow（confirm 失敗不得讓已成功付款回錯）。原「事務外直發 MQ」已移除（事務 commit 後、直發前程序崩潰會遺失事件，正是本次修復對象）。

> **三 notify 入口共用同一 handle（收斂點 A）**：付款結果的 notify 路徑除 MQ webhook 入口（`PayUniWebhookConsumer`）外，尚有 PayUni 導回頁入口——HTTP `POST /payment/return`（`PaymentReturnController`）與 gRPC `HandlePaymentPageReturn`（`PaymentGrpcService`）。三者皆在各自事務內呼叫**同一** `PaymentNotifyHandler.handle`，故 outbox 化只需改 `handle` 一處，三入口一併受益、一併移除事務外直發。另有三個 in-tx 同步付款發布點（零元 / poll·reconcile / Apple·Google Pay 同步）經收斂點 B（`PaymentApplicationService.publishEvent`）統一走同一 writer，合計**六發布點**（見 §5.10.1）。

**5.2.3 PaymentNotifyHandler（付款結果）**

| 步驟 | 動作 | 說明 |
|------|------|------|
| 1 | 查詢交易 `findByTradeNo(MerTradeNo)` | 交易不存在 → `FAILED` |
| 2 | 檢查終態 `status.isTerminal()` | 已是終態 → 進入衝突檢測（2a/2b） |
| 2a | 終態 + 同步付款 + 解析 Webhook 狀態，`isConflict()` 判定矛盾 | 矛盾 → `updateToConflict()` **＋ outbox enqueueAndDispatch** `PaymentConflict`（`payment.conflict`），同事務 |
| 2b | 終態 + 狀態一致（無矛盾） | → `SKIPPED` |
| 3 | 解析 PayUni 狀態 `parseProviderStatus(TradeStatus)` | 無法解析 → `FAILED` |
| 4a | TradeStatus=1 → `updateToPaid()` **＋ outbox enqueueAndDispatch** `PaymentSucceeded`（`payment.success`） | 條件式更新 + 樂觀鎖；狀態與 outbox 同事務 |
| 4b | TradeStatus=2 → `updateToFailed(providerTradeNo)` **＋ outbox enqueueAndDispatch** `PaymentFailed`（`payment.failed`） | 條件式更新 + 樂觀鎖；**寫回 `provider_trade_no`** 供後續對帳；狀態與 outbox 同事務 |
| 4c | TradeStatus=0 → `SKIPPED`（處理中） | 不更新狀態、不 enqueue |

> **outbox enqueueAndDispatch**：步驟 2a / 4a / 4b 的事件不再由 `handle` 回傳交呼叫端事務外直發，而是在 `handle` 的**同一事務內**經 `PaymentOutboxWriter.enqueueAndDispatch(event)` 寫入 `outbox_events`（PENDING）並註冊 afterCommit dispatch，與 `updateToXxx` 的狀態更新原子成立；投遞由 afterCommit 即時（正常路徑）+ §5.10 排程器兜底 + publisher-confirm 完成。

> **衝突檢測（僅影響 Apple Pay / Google Pay）**
>
> `PaymentNotifyHandler` 在步驟 2 的終態檢查中，先解析 Webhook 的 `TradeStatus`，透過 `PaymentDomainService.isConflict()` 比對 DB 現有終態與 Webhook 狀態：
>
> - **矛盾**（例如 DB=PAID 但 Webhook=FAILED）：走 `updateToConflict()` + `PaymentConflict` 事件分支，發布至 `payment.conflict` routing key，觸發人工介入
> - **一致**（例如都是 PAID）：維持 SKIPPED 行為
>
> **影響範圍**：僅限 Apple Pay / Google Pay。這兩種付款方式在 `CreatePayment` 時即取得 PayUni 同步回傳的終態（PAID / FAILED），交易在 Webhook 到達前已不是 INIT。信用卡 UPP / ATM 等非同步付款方式的交易在 `CreatePayment` 後保持 INIT，Webhook 是唯一寫入終態的管道，不存在衝突情境。

### 5.3 同步交易狀態（SyncPayment）

供 Gateway 或其他服務主動查詢 PayUni 最新狀態：

1. 查詢 DB 交易
2. 呼叫 `PaymentClient.queryTransaction(merchantId, tradeNo)`
3. 依 PayUni TradeStatus 更新本地狀態
4. 若與既有 Webhook 結果衝突 → 標記 CONFLICT

### 5.4 關閉交易（ClosePayment）

**ClosePayment：**

1. 查詢交易，確認 `canClose()`（僅 INIT 可關閉）
2. 條件式更新：`updateToClosed(tradeNo, closedAt, version)`
3. 直接發布 MQ 事件：PaymentClosed 到 `payment.events / payment.closed`

**ClosePaymentByOrderNo（依訂單編號關閉交易）：**

**呼叫方**：Order Service（訂單取消 / 超時取消時，best-effort 呼叫）

**流程：**
1. 依 `orderNo` 查詢活躍交易（`findActiveByOrderId`，status IN (INIT)）
2. 若無活躍交易 → 回傳成功（訂單可能未建立付款交易，屬正常情況）
3. 若有 → 複用 `closePayment(tradeNo)` 關閉交易（INIT → CLOSED）
4. 若交易已非 INIT（已付款/已過期/已關閉）→ 回傳成功（冪等）

**冪等保證**：無論呼叫幾次，結果一致。無活躍交易、已關閉、已非 INIT 皆視為成功。

**內部 RPC**：僅供微服務間呼叫，不暴露至 Gateway。

**Tx 載體規定（見 §2A.6.5）**：

- `closePayment` **必須**抽到獨立 bean `PaymentCloseService`，**實作位置**：`services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment/application/service/PaymentCloseService.java`；由 `PaymentApplicationService.closePaymentByOrderNo` 跨 bean 注入呼叫 — 這樣 `closePaymentByOrderNo` 內的 `paymentCloseService.closePayment(...)` 才會經 Spring AOP CGLIB proxy → `closePayment` 上的 `@Transactional` 才會真實生效（獨立 tx 邊界）
- `PaymentGrpcService.closePayment` handler 改直接注入 `PaymentCloseService` 呼叫（不再經 `PaymentApplicationService`），讓 RPC entry 與 `closePaymentByOrderNo` 的 caller 都走同一個獨立 bean 的 proxy 路徑；`PaymentApplicationService.closePayment` 同時刪除（單一委派 wrapper 無存在價值，避免兩個 close API entry 造成 caller 困惑）
- 禁止：在 `PaymentApplicationService` 同 class 內以 `this.closePayment(...)` 形式呼叫已標 `@Transactional` 的 method — CGLIB proxy 不攔截 self-invocation，annotation 失效後 inner method 等同無 tx，只是依附於 outer caller 的 tx，未來改 propagation（如 `REQUIRES_NEW`）會靜默失效 → 形式 vs 實際差異即「語義謊言」，未來陷阱
- `PaymentApplicationService.closePaymentByOrderNo` 本身**不**標 `@Transactional`（其工作只是 `findActiveByOrderId` 查詢 + 委派，無多步原子寫入需求）；真正需要 tx 的 `closePayment` 內含「updateToClosed UPDATE + publishEvent outbox INSERT」由獨立 bean 的 proxy 保護
- 違規檢核：本服務內所有 (1)「`@Transactional protected/public` 方法 + 同 class `this.fooTx(...)` self-invocation」、(2)「tx body 內 serialize + catch + return（silently swallow → tx commit → 原子化破洞）」模式一律屬違規，與 §2A.6.5 RescueRefundConsumer / §10.1 PendingPaymentReconcileService 規範對齊

### 5.5 退款流程（Refund）

退款擁有**獨立生命週期**，不改變交易主表狀態。

#### 5.5.1 PayUni 退款 API 分流策略

依付款方式自動分流至對應的 PayUni 退款 API：

| 付款方式 | payment_type | 退款 API | Endpoint | 回應模式 | 特殊需求 |
|---------|-------------|---------|----------|---------|---------|
| 信用卡 | 1 (CREDIT_CARD) | 信用卡請退款 | `/api/trade/close` | **即時** | CloseType=2, TradeAmt |
| Apple Pay | 2 (APPLE_PAY) | 信用卡請退款 | `/api/trade/close` | **即時** | 同信用卡（走銀行信用卡通道） |
| Google Pay | 3 (GOOGLE_PAY) | 信用卡請退款 | `/api/trade/close` | **即時** | 同信用卡（走銀行信用卡通道） |
| ATM 轉帳 | 4 (ATM) | 非信用卡退款轉匯（**請求頁面版**） | `/api/offline_payment/refund` | **非即時**（隔日匯款） | **PayUni 頁面收集銀行資訊，平台不接觸** |

**信用卡退款（含 Apple Pay / Google Pay）**：

- **退款 API**：`POST /api/trade/close` CloseType=2，Version=`1.0`
- **取消授權 API**：`POST /api/trade/cancel`，Version=`1.0`（獨立 API，非 `/api/trade/close`）
- **即時回應**：`SUCCESS` = 退款成功 / 取消授權成功
- **PaymentClient**：`PaymentClient.refund()`（退款）或 `PaymentClient.cancelAuthorization()`（取消授權）
- **180 天期限**：請款完成後 180 天內可退款

**退款前必須先查詢請款狀態（CloseStatus 分流邏輯）**：

```
1. 呼叫 PaymentClient.queryTransactionByProviderTradeNo(merId, providerTradeNo) 查詢交易狀態
2. 從查詢結果取得 CloseStatus（請款狀態，信用卡專用欄位）
3. 依 CloseStatus 分流：
   - "1"（請款申請中）→ 呼叫 cancelAuthorization()（/api/trade/cancel，取消授權）
   - "2"（請款成功）  → 呼叫 refund()（/api/trade/close CloseType=2，退款）
   - "7"（請款處理中）→ 呼叫 refund()（/api/trade/close CloseType=2，退款）
   - "3"（請款取消）  → 拋 IllegalStateException（交易請款已取消，無法退款）
   - "9"（未申請）    → 拋 IllegalStateException（交易尚未請款，無法退款）
   - 其他            → 拋 IllegalStateException（未知的請款狀態）
4. 呼叫端（AfterSales RefundTaskWorker，經 internal call）無需區分 CloseStatus 分流，統一呼叫 CreateRefund 即可（CreateRefund 已收斂為 internal-only，唯一發起方見 §5.5.3）
```

> **設計決策**：
> - PAYUNi 預設自動請款模式，正常情況下信用卡交易的 CloseStatus 應為 `"2"`（請款成功），直接走退款路徑
> - 取消授權比退款更快且無手續費。當交易尚在請款排程中（CloseStatus=`"1"`），取消授權是更優的選擇
> - 此分流邏輯封裝在 Payment Service 內部，內部呼叫端（AfterSales RefundTaskWorker）統一呼叫 CreateRefund 即可

**信用卡退款參數**（`/api/trade/close` CloseType=2）：

| 參數 | 類型 | 說明 | 來源 |
|------|------|------|------|
| MerID | String | 商店代號 | `payment_transactions.mer_id` |
| TradeNo | String | UNi 序號 | `payment_transactions.provider_trade_no`（**非**系統 trade_no） |
| Timestamp | Int | 時間戳記 | `System.currentTimeMillis() / 1000` |
| CloseType | Int | 關帳類型 | 固定 `2`（退款） |
| TradeAmt | Int | 退款金額 | 退款金額（新台幣元） |

**信用卡取消授權參數**（`/api/trade/cancel`，獨立 API）：

| 參數 | 類型 | 說明 | 來源 |
|------|------|------|------|
| MerID | String | 商店代號 | `payment_transactions.mer_id` |
| TradeNo | String | UNi 序號 | `payment_transactions.provider_trade_no` |
| Timestamp | Int | 時間戳記 | `System.currentTimeMillis() / 1000` |

> **注意**：取消授權 API 不需要金額參數（整筆授權取消）。參見 PayUni 逆向金流 API 總覽 §3。

**ATM 退款轉匯（請求頁面版）**：

- **API**：`POST /api/offline_payment/refund`（**請求頁面版**，非 API 版）
- **非即時**：PayUni 隔日扣款 + 匯款，2~3 工作天到帳
- **退款結果通知**：PayUni Notify（RefundStatus: 1=轉匯成功, 2=轉匯失敗）
- **PaymentClient**：`PaymentClient.offlineRefundPage()`（新增，組裝頁面版參數）
- **180 天期限**：付款完成後 180 天內可退款
- **當日可取消**：發動退款當日可執行取消退款，隔日則不可

> **設計決策**：ATM 退款改用**請求頁面版**而非 API 版。銀行帳號、身分證字號等高機敏資料由 PayUni 頁面直接收集，**平台完全不接觸、不儲存、不傳輸**。

ATM 退款頁面版參數（由 Payment Service 組裝）：

| 參數 | 類型 | 說明 | 來源 |
|------|------|------|------|
| MerID | String | 商店代號 | `payment_transactions.mer_id` |
| TradeNo | String | UNi 序號 | `payment_transactions.provider_trade_no` |
| Timestamp | Int | 時間戳記 | `System.currentTimeMillis() / 1000` |
| PaymentType | Int | 支付方式 | 固定 `2`（ATM 轉帳） |
| RefundAmt | Int | 退款金額 | 退款金額（新台幣元） |
| ReturnURL | String | 完成後返回網址 | AfterSales 傳入（Gateway 中繼 → URL Scheme 導回 APP） |

> **不需傳入** `BankCode`、`BankSubCode`、`BankAccountNo`、`UID`，這些由 PayUni 頁面向買家收集。

> **重要**：`TradeNo` 是 PayUni 的 UNi 序號（`payment_transactions.provider_trade_no`），**不是**系統 `trade_no`。

**新增 RPC：GetRefundPageUrl（ATM 退款頁面版 URL 組裝）**：

AfterSales Service 呼叫此 RPC 取得 PayUni 退款頁面 redirect URL，回傳給前端用 WebView 開啟。流程參考信用卡 UPP 的 redirect 模式 + 超商門市地圖的 URL Scheme 導回模式。

> **GetRefundPageUrl 用途**：AfterSales Service 呼叫此 RPC 取得 PayUni ATM 退款頁面版（`/api/offline_payment/refund`）的 redirect URL。Payment Service 內部使用 `PaymentClient.offlineRefundPage()` 組裝加密參數。買家在 WebView 中填寫銀行帳號/身分證，平台完全不接觸。

**4.6.2 退款分流實作**

```java
if (transaction.paymentType() == PaymentMethod.ATM.dbValue()) {
    // ATM：不由 CreateRefund 執行，改由 GetRefundPageUrl 走 PayUni 頁面版
    throw new IllegalArgumentException("ATM 退款請使用 GetRefundPageUrl 走頁面版");
} else {
    // 信用卡 / Apple Pay / Google Pay：先查詢請款狀態再決定路徑
    PayUniApiResponse queryResult = paymentClient.queryTransactionByProviderTradeNo(
            transaction.merId(), transaction.providerTradeNo());
    String closeStatus = queryResult.getCloseStatus();

    switch (closeStatus) {
        case "1" -> {
            // 請款申請中 → 取消授權（/api/trade/cancel，更快、無手續費）
            response = paymentClient.cancelAuthorization(
                    transaction.merId(), transaction.providerTradeNo());
        }
        case "2", "7" -> {
            // 請款成功 / 請款處理中 → 退款（/api/trade/close CloseType=2）
            response = paymentClient.refund(
                    transaction.merId(), transaction.providerTradeNo(),
                    refund.refundNo(), request.amount());
        }
        case "3" -> throw new IllegalStateException("交易請款已取消，無法退款");
        case "9" -> throw new IllegalStateException("交易尚未請款，無法退款");
        default -> throw new IllegalStateException("未知的請款狀態: " + closeStatus);
    }
}
```

ATM 退款的狀態差異：

| 付款方式 | PayUni 回應 | 初始退款狀態 | 最終態推進方式 |
|---------|-----------|------------|-------------|
| 信用卡系列 | SUCCESS = 退款成功 | SUCCEEDED（即時） | — |
| ATM | SUCCESS = 申請受理 | PROCESSING（非即時） | PayUni Notify → webhook-service → MQ → Payment Service |

#### 5.5.3 建立退款（CreateRefund）

1. gRPC 接收：trade_no, amount, reason, refund_no（冪等鍵），ATM 額外帶 bank_code/bank_sub_code/bank_account_no/uid
2. **冪等檢查（refund_no）**：
   - 若 `refund_no` 非空，查詢 `payment_refunds WHERE refund_no = ?`
   - 已存在且 status = SUCCEEDED → 直接回傳既有結果（不重複退款）
   - 已存在且 status = PROCESSING → 回傳「處理中」
   - 已存在且 status = FAILED → 允許重試（建立新退款）
   - 不存在 → 進入正常流程
3. 前置檢查（已於 §5.5.5 L1 取得 `payment_transactions` 行鎖後執行；狀態集合的完整規則見 §5.5.5 L2）：
   - 交易必須為 PAID 或 CLOSED 且 `isRefundable()` = true
   - 無未終結的退款申請（`hasProcessingRefund`，`status IN ('0','1')` = REQUESTED + PROCESSING，**不含 SUCCEEDED**；含 SUCCEEDED 會讓多次 / 部分退款被永久擋死）— 防止重複退款申請
   - 退款金額 + 已退款累計（`sumRefundedAmountByTradeNo`，`status IN ('0','1','2')` = 在途 + 成功）≤ 交易金額
4. 建立退款單：INSERT `payment_refunds`，status=REQUESTED，refund_no=冪等鍵
5. 依付款方式分流呼叫 PayUni API（§5.5.1），遵循[收斂規範 §8](../開發規範/PayUni%20錯誤回應收斂規範.md)「主流程同步呼叫 retry 最多一次」；信用卡額外由 AfterSales Worker 層做任務 retry（見 09-售後設計書）
6. 處理回應（**必須寫入 `provider_status` / `provider_message`**，遵循規範 §4.1 SoT；發 MQ event 含 `payuni_status` / `payuni_message` 等擴充欄位，遵循規範 §5.2 + §4.4 單一欄位雙階段寫入）：
   - 信用卡成功 → `updateStatusToSucceeded()` + MQ: `refund.succeeded`
   - 信用卡 retry 後仍失敗 → `updateStatusToFailed()` + MQ: `refund.failed`（升級異常）
   - ATM sync 成功（頁面版受理）→ AfterSales `handleRefundPageReturn` 以 gRPC `AckAtmRefundAccepted` 呼進來，`markAtmRefundAccepted()` 推進 PROCESSING + 寫入 `estimated_refund_date`；**不發 MQ**（見 §4.4.2）
   - ATM sync 失敗（PayUni 頁面版錯誤回來）→ 整段由 AfterSales `handleRefundPageReturn` 就地處置（更新 `refund_tasks` 的 `payuni_*` snapshot、REF02037 走對帳、其餘退回 AWAITING_BUYER_REFUND_PAGE），**不進 Payment 的退款狀態機、不發 MQ**（見 §4.4.2）；同時透過 URL scheme 回前端即時顯示
   - ATM async webhook → 依 `RefundStatus` 更新，寫入 `bank_date`（若成功）+ MQ: `refund.succeeded` / `refund.failed`

> **冪等保障**：`refund_no` 為呼叫端傳入的冪等鍵（如售後單 case_no），Payment Service 以此防止 Worker crash 後重試導致重複退款。`updateStatusToSucceeded` / `updateStatusToFailed` 均使用條件更新（`WHERE status IN (0,1) AND version = ?`），確保狀態轉移為原子操作。

**權限調整（僅接受 internal call）**：`PaymentGrpcService.createRefund()` 改為**僅接受內部服務呼叫**（`GrpcAuthContext.isInternalCall() == true`）；外部 BUYER（或任何帶 JWT 的前端）直呼一律回 `PERMISSION_DENIED`（Gateway 塌縮為 HTTP 403）。

**正確路徑**：買家申請退貨 → AfterSales 審核核准 → `RefundTaskWorker` 以 internal call（`x-internal-service` metadata）呼叫 `createRefund`。買家不再有直呼退款的入口（§4.1 已移除 buyer 路由、§1.6 權限表 `CreateRefund` 收斂為 internal-only，三處一致）。

🔴 **`status == PAID` 不是充分的業務閘門**：退款的前提是「存在一筆已核准的售後案件」，而該前提由 AfterSales 持有 ⇒ payment 端唯一能守住的方式就是**只接受 internal call**。把 buyer-facing 入口留著、只靠 `status == PAID` 判斷，買家即可繞過售後審核自助退款造成資損。

**Proto 契約**：

```protobuf
message CreateRefundRequest {
  string trade_no = 1;
  string amount = 2;
  string reason = 3;
  // field 4~7 已移除（ATM 改用 PayUni 頁面版，平台不接觸銀行機敏資訊）
  string refund_no = 8;       // 冪等鍵（呼叫端傳入，如 case_no / cancel_id）
}
```

#### 5.5.4 退款完成閉環

退款申請成功不代表退款已完成。最終態由 PayUni 回調或主動查詢推進：

- PROCESSING → SUCCEEDED：退款完成 → MQ: RefundSucceeded（`payment.events` / `refund.succeeded`）
- PROCESSING → FAILED：退款失敗 → MQ: RefundFailed（`payment.events` / `refund.failed`）

**ATM 退款 Webhook 處理**：PayUni Notify → webhook-service → MQ(`payuni.refund.*`) → Payment Service 消費 → 更新 `payment_refunds` 狀態 → 發布 `payment.events` exchange 上的 `refund.succeeded` / `refund.failed` MQ 事件供下游（AfterSales / Order Service）消費。

#### 5.5.4.1 ATM 退款最終結果 Webhook 的 finalize 語意（權威結果 lossless）

ATM 退款的最終結果由 PayUni 非同步 Webhook（inner `RefundStatus` = 1 成功 / 2 失敗）帶回，是該退款是否真正到帳的**唯一權威訊號**。`RefundApplicationService.handleAtmRefundWebhook` 收到此結果時，必須保證「只要 PayUni 已成功退款，我方最終狀態必定收斂為 SUCCEEDED 且發出 `refund.succeeded`」，任何競態 / 重送都不得使此結果遺失。

🔴 **兩個會讓權威結果靜默遺失的寫法，禁止使用**：
- 以 `findActiveByTradeNo(tradeNo)`（只撈 REQUESTED/PROCESSING）定位待 finalize 的退款，回空就 return ⇒ consumer 仍 `markAsProcessed(SUCCESS)`，PayUni 已退款的成功訊號被丟棄、下游收不到 `refund.succeeded`。
- finalize 沿用帶 `AND version = #{version}` 樂觀鎖的共用 `updateToSucceeded` ⇒ webhook 重送／併發下 `updated = 0` 會被誤當「已處理」略過，同樣 lost-update。

##### finalize 的兩個獨立守則

**(1) 找不到 active 退款 ≠ 成功**：`RefundStatus=1` 但 `findActiveByTradeNo` 回空時，不得靜默當成功，改以該 trade_no 查「最新一筆」退款（不限狀態）判定：

| 最新退款狀態 | 語意 | 動作 | 可觀測 |
|---|---|---|---|
| SUCCEEDED（終態成功） | 已成功落地（重送 / 已被 sync 收斂） | 冪等 OK，**不**重發事件 | 記 INFO |
| FAILED（我方標失敗）但 PayUni 回成功 | **狀態衝突**：PayUni 實際已退款、我方誤標失敗 | 走 `correctFailedToSucceeded`（`WHERE status='3'`）校正為 SUCCEEDED（以 PayUni 為準）+ 補發 `refund.succeeded`。**不可**用 `finalizeAtmRefundSucceeded`（其 `WHERE status IN('0','1')` 不含 '3'，對 FAILED row 命中 0 rows → 靜默丟失成功結果） | 記 ERROR |
| 查無任何退款 row | 孤兒結果（無對應本地退款） | 告警等人工核對，不臆造 row | 記 ERROR |

**(2) finalize 用狀態機冪等，不用 version 樂觀鎖**：webhook 帶回的是外部權威終態，其套用不能因本地 version 競態失敗。ATM webhook finalize 路徑改用「狀態機冪等」條件更新（`WHERE refund_no AND status IN ('0','1')`，**不含** `version` 條件），沿用既有 `updateToProcessingWithEstimate` 的既定 pattern（回調可能重試，冪等由狀態機約束）。只要退款尚未終態，webhook 最終結果**必定**套用；已終態則條件不命中天然冪等。

**與信用卡同步路徑分離（不動共用方法）**：共用的 `updateToSucceeded` / `updateToFailed`（帶 version 守衛）供信用卡**同步**退款路徑使用——該路徑由本服務主動呼叫、需要 version 守衛偵測與其他同步寫入的競態，**保留不動**。ATM webhook finalize 另走專屬冪等方法（`finalizeAtmRefundSucceeded` / `finalizeAtmRefundFailed`），兩路徑不共用同一守衛，避免修改波及信用卡。

```
handleAtmRefundWebhook(tradeNo, RefundStatus, message, bankDate):
  active = findActiveByTradeNo(tradeNo)          # 僅 REQUESTED/PROCESSING
  if active present:
    if RefundStatus == 1:
      finalizeAtmRefundSucceeded(refund_no)      # WHERE status IN (0,1)，無 version
      publish refund.succeeded
    else:
      finalizeAtmRefundFailed(refund_no)         # WHERE status IN (0,1)，無 version
      publish refund.failed              # 觸發 aftersales §5.4.3.1 對帳處置
  else:                                          # ★ 找不到 active，不得靜默當成功
    latest = findLatestByTradeNo(tradeNo)        # 最新一筆，不限狀態
    switch latest.status:
      SUCCEEDED: log INFO  (冪等，成功已落地，不重發)
      FAILED & RefundStatus==1:                  # 衝突校正
        log ERROR (狀態衝突)
        correctFailedToSucceeded(refund_no)      # WHERE status='3'（非 finalizeAtmRefundSucceeded，其 WHERE IN('0','1') 不含 '3' 會 no-op）
        if updated > 0: publish refund.succeeded
      FAILED & RefundStatus==2: log INFO  (冪等，我方與 PayUni 皆失敗，不動作)
      none:    log ERROR (孤兒結果，人工核對)
```

**涉及檔案**：`application/service/RefundApplicationService.java`（`handleAtmRefundWebhook`）、`infrastructure/persistence/mapper/RefundTransactionMapper.java`（新增 `finalizeAtmRefundSucceeded` / `finalizeAtmRefundFailed`，WHERE status IN('0','1') 無 version；新增 `correctFailedToSucceeded`，WHERE status='3' 專供衝突校正；新增 / 沿用 `findLatestByTradeNo` 取最新一筆，SQL `ORDER BY created_at DESC LIMIT 1`）、`domain/repository/RefundTransactionRepository.java` 與 `...repository/RefundTransactionRepositoryImpl.java` 對應介面。

> **一致性總則**：PayUni 已成功退款為單一真實來源；ATM 退款最終結果 webhook 為權威，finalize 冪等且 lossless（狀態機冪等，非 version 樂觀鎖），任何路徑不得把已成功退款標為失敗或丟棄。本服務不引入補償 / 對帳 scheduler job（遵 [後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)）：對帳於此單一 webhook 寫入路徑內即時完成；aftersales 端對應處置（失敗 revert 前對帳、REF02037 不重填）見 [09 §5.4.3.1](./09-售後微服務（AfterSales%20Service）詳細設計書.md#5431-atm-退款失敗處置的健壯化issue-1288)。

#### 5.5.4.2 ATM 退款結果 Webhook 真正遺失的處置

§5.5.4.1 保證「webhook 到達時」權威結果 lossless；本節處理更上游的邊界：**PayUni 退款轉匯結果 Webhook（[逆向金流 §6.7](../第三方整合/PayUni%20逆向金流%20API%20總覽.md#67-退款轉匯結果-notifypayuni--商店)，inner `RefundStatus`）從未到達 / 長期不到**（PayUni 端未推、或我方接收端在 PayUni 重送窗口內全程不可用）。此時 `refund_task` 永久停在 `AWAITING_WEBHOOK(15)`、`payment_refunds` 停在 `PROCESSING(1)`，無自動收斂。

**為何不引入自動對帳 reconciler**（延伸 §5.5.4.1「不引入補償 / 對帳 scheduler job」立場至本場景）：

1. **技術不可行**：PayUni 對 ATM／線下退款（PaymentType=2）**不提供任何退款狀態查詢 API**。交易查詢 `/api/trade/query`（[逆向金流 §2.4](../第三方整合/PayUni%20逆向金流%20API%20總覽.md)）的 `RefundStatus` 僅列於「信用卡專用欄位（PaymentType=1）」；線下交易專用欄位只有 `OffChannel/OffPayChannel/OffPayNo/OffExpireTime`，無任何退款欄位（官方 spec + sandbox 實測雙證）。無權威來源可反查 → 無法自動判定終態。
2. **安全性**：§5.5.4.1 明訂[逆向金流 §6.7](../第三方整合/PayUni%20逆向金流%20API%20總覽.md#67-退款轉匯結果-notifypayuni--商店)是「退款是否真正到帳的唯一權威訊號」。無權威確認下自動把狀態設為 SUCCEEDED / FAILED，等同把「未匯出 / 未知」誤標為終態，違反權威結果 lossless 原則。
3. 與**付款** Webhook 遺失不同：付款可用 `SyncPaymentStatus`（查 `TradeStatus`，§8.4）主動補單；ATM **退款**無對等查詢 API，此補救不適用。

**處置（依 [後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)：偵測歸 observability、回復歸維運，不寫 application code）：**

| 層面 | 機制 | 位置 |
|---|---|---|
| 偵測 | observability 告警：`refund_task` 停 `AWAITING_WEBHOOK(15)`（ATM）超過門檻（預設 5 天，須 > prod 逆向金流 §6.7 正常 2–3 工作天）→ 進業務告警頻道 | monitoring stack（obs 層 SQL metric + Prometheus alert rule），**非** application `@Scheduled` |
| 回復 | 維運 runbook：查 PayUni 代理商後台 UNi 帳戶核實退款轉匯是否完成 → **優先請 PayUni 重送 逆向金流 §6.7**（走既有冪等 Webhook 路徑，自動收斂並補發 `refund.succeeded`）；無法重送才人工校正 | [基礎建設：ATM 退款結果 Webhook 遺失維運處置](../基礎建設/ATM%20退款結果%20Webhook%20遺失維運處置.md) |

> **一致性補述**：本場景不改變 §5.5.4.1 的單一寫入路徑；回復首選是讓真正的 逆向金流 §6.7 重新流經既有冪等 finalize 路徑，而非新增平行寫入或對帳基礎設施。偵測留在 observability 層（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)：即使「偵測 + log warn」也不得放進 application）。

#### 5.5.4.3 ATM 退款受理 Notify 找不到 active 退款的處置

ATM 退款受理 Notify（[逆向金流 §6.6](../第三方整合/PayUni%20逆向金流%20API%20總覽.md#66-返回參數encryptinfo) async 副本，帶 `EstimatedRefundDate`）由 `PayUniRefundAcceptedConsumer` 消費 `payuni.refund.accepted` 後呼叫 `RefundApplicationService.handleAtmRefundAccepted`，用途是把退款推進 `REQUESTED → PROCESSING` 並寫入 `estimated_refund_date`——屬**非終態的中間里程碑**，與 §5.5.4.1 的「終態權威結果」性質不同。

🔴 **同一個 root cause class 與 §5.5.4.1**：以 `findActiveByTradeNo(tradeNo)`（只撈 `status IN('0','1')`）定位退款、回空就 `log.warn` return，而 consumer 仍 `markAsProcessed(SUCCESS)` + ack ⇒ **lookup 回空被靜默視為成功**。凡是 webhook／consumer 的 lookup 回空，都必須明確判定它代表什麼，不得預設為成功。

**與 §5.5.4.1 的關鍵差異（決定處置不同）**：

- 受理 Notify（逆向金流 §6.6）與結果 Notify（逆向金流 §6.7）是**兩條獨立 queue、無跨訊息順序保證**。結果 Notify 可能先被處理使退款進入終態（`status=2/3`），之後受理 Notify 抵達，`findActiveByTradeNo` 自然回空——這是**正常且預期**的情境。
- 受理只寫「中間里程碑」，退款已終態時該推進**本就無意義**，故「已終態時 no-op」是**行為正確**的，並非結果遺失。因此本節不需 §5.5.4.1 的「衝突校正 + 補發事件」邏輯，也**不 throw / requeue**（受理里程碑不可重試，requeue 只會 infinite-loop）。
- 「row 未落地」race **不成立**：ATM 退款 row 於 `createRefund` 的 `@Transactional` 內同步預建（`status=0` REQUESTED）、commit 後才發出退款頁 URL 供買家填單，PayUni 才可能發受理 Notify——strict happens-before，受理 Notify 抵達時 row 必然已存在。

**處置**：回空時不再靜默 `warn`，改以 `findLatestByTradeNo` 判定（沿用 §5.5.4.1 既有方法），修正 log 語意並補齊與結果 Notify 對稱的孤兒可觀測性：

| 最新退款狀態 | 語意 | 動作 | 可觀測 |
|---|---|---|---|
| SUCCEEDED / FAILED（終態） | 結果 Notify 已先收斂，受理里程碑無意義 | 冪等略過（no-op，正確） | 記 INFO |
| 查無任何退款 row | 孤兒受理 Notify（無對應本地退款，近乎不可能） | 告警等人工核對，不臆造 row | 記 ERROR |
| REQUESTED / PROCESSING 卻找不到 active | 理論上不會發生（同一 row 同時滿足兩查詢矛盾） | 告警人工核對 | 記 ERROR |

```
handleAtmRefundAccepted(tradeNo, estimatedRefundDate, message):
  active = findActiveByTradeNo(tradeNo)          # 僅 REQUESTED/PROCESSING
  if active present:
    markAtmRefundAccepted(refund_no)             # WHERE status='0' 狀態機冪等，無 version（既有分支不動）
    return
  else:                                          # ★ 回空不再靜默 warn
    latest = findLatestByTradeNo(tradeNo)        # 最新一筆，不限狀態
    switch latest.status:
      SUCCEEDED / FAILED: log INFO (結果已先收斂，受理里程碑冪等略過)
      none:               log ERROR (孤兒受理 Notify，人工核對)
      REQUESTED/PROCESSING: log ERROR (非預期，人工核對)
    return                                        # 皆不 throw（受理不可重試）
```

**與 §5.5.4.1 一致性**：兩者同屬「lookup 回空不得靜默」原則；差別在結果 Notify 是**終態權威**（回空需 lossless 校正 + 補發），受理 Notify 是**中間里程碑**（回空的終態情境是正確 no-op，僅需修正 log 語意 + 孤兒告警）。row-found 分支既有冪等（`markAtmRefundAccepted` WHERE `status='0'`）不動。

**涉及檔案**：`application/service/RefundApplicationService.java`（`handleAtmRefundAccepted` 回空分支）。沿用既有 `findLatestByTradeNo`，不新增 mapper / repository 方法、不新增 CodeMeta / Flyway / MQ event。

> **一致性補述**：本節不改變 §5.5.4.1 的單一寫入路徑，亦不新增任何平行寫入——回空分支只做 `findLatestByTradeNo` 讀取與 log 語意修正，既有冪等 WHERE 條件（`markAtmRefundAccepted` WHERE `status='0'`）不變。不引入補償 / 對帳 scheduler job（遵 [後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)）：終態收斂仍由 §6.7 結果 Notify 的既有冪等 finalize 路徑（§5.5.4.1）唯一負責，受理 Notify 回空的偵測留在 observability 層（log INFO/ERROR），不放進 application 補償邏輯。

#### 5.5.5 退款併發控制與超額退款三層防護

防止同一交易被併發或多次退款累計超過交易金額（超額退款資損），是退款流程最關鍵的金額安全設計。比照 §2.4 重複扣款防護，採三層縱深防禦，任一層失效時由下一層攔截。退款狀態值一律引用 §2.2 退款狀態機（`0` REQUESTED / `1` PROCESSING / `2` SUCCEEDED / `3` FAILED），本節不重列狀態表。

**攻擊情境（未防護前）：**

```
同一筆 PAID 交易（金額 100）
併發兩個 createRefund 請求（各 amount=100）
→ 兩請求同時通過「已退款累計 ≤ 交易金額」檢查（各自讀到累計 0，因在途退款未計入）
→ 建立兩筆退款各 100，實退 200 → 超額退款資損
```

**第一層：應用層行鎖（序列化同交易的併發退款建立）**

- 位置：`RefundApplicationService.createRefund()`
- 機制：讀原始交易改用帶行鎖的查詢（`SELECT ... FOR UPDATE` 鎖住 `payment_transactions` 該 row，如 `findByTradeNoForUpdate`）
- **鎖範圍**：`createRefund` 全程為單一 `@Transactional`，L1 `SELECT ... FOR UPDATE` 行鎖**持有至 PayUni 退款呼叫結束**——涵蓋 L2 餘額 / 未終結檢查 → INSERT REQUESTED 退款 → 呼叫 PayUni 退款 API → 寫入最終狀態，全程持鎖，**PayUni 回應後才 commit 釋鎖**（與 §5.6 grantPayment 一致）。論證：維持信用卡退款「同步呼叫即得終態」的單事務原子性、避免拆雙階段引入 partial-failure（INSERT REQUESTED 成功但 PayUni 未成功呼叫的窗口）、退款為低頻操作故持鎖跨 PayUni 的鎖窗可接受。
- 效果：同一 `trade_no` 的併發 `createRefund` 被序列化，後到者需等前一筆整個 `createRefund` 事務（含 PayUni 呼叫）結束釋鎖後才讀取，避免兩請求同時讀到過期的「已退款累計」

**第二層：在途餘額計算 + 未終結申請檢查（兩個獨立查詢，狀態集合刻意不同）**

- 位置：`RefundApplicationService` 前置檢查（`sumRefundedAmountByTradeNo` + `hasProcessingRefund`，兩者職責分離）
- **可退餘額（`sumRefundedAmountByTradeNo`）用 `status IN ('0','1','2')`**（REQUESTED + PROCESSING + SUCCEEDED，即在途 + 成功）：可退餘額 = 交易金額 − 此 SUM；退款金額 > 可退餘額即拒。🔴 **述詞不得只算 SUCCEEDED（`'2'`）**：在途退款不計入已退累計時，併發的兩筆退款會各自誤判仍有餘額。
- **未終結申請檢查（`hasProcessingRefund`）用 `status IN ('0','1')`**（REQUESTED + PROCESSING，**不含 SUCCEEDED**）：只擋「同交易已有一筆尚未終結的退款申請」。🔴 **述詞必須含 REQUESTED**：只擋 PROCESSING 會放過剛建立、尚未送出的那一筆。
- **為何 `hasProcessingRefund` 絕不可含 SUCCEEDED**：本服務支援同一交易的多次 / 部分退款（`sumRefundedAmountByTradeNo` 為累計設計即為此）。若把 SUCCEEDED 也算「處理中」，交易一旦有一筆退款成功，`hasProcessingRefund` 將**恆為 true**、後續所有部分退款被永久擋死，打破多次退款能力。已成功退款的額度佔用由「可退餘額」那條負責（含 SUCCEEDED），與「是否有未終結申請」職責嚴格分離。
- 效果：在途 / 成功退款即時佔用可退餘額（餘額不足即拒）；同時同交易不得有兩筆未終結（REQUESTED/PROCESSING）退款申請並存。

**第三層：資料庫兜底約束（最後防線）**

- 位置：`payment_refunds`，走 Flyway `V{N+1}`（V1 凍結）
- 不變式：同一 `trade_no` 的退款總額（在途 + 成功）≤ 對應交易金額
- 機制：因 PostgreSQL `CHECK` 無法跨 row 聚合，改以 `BEFORE INSERT/UPDATE` trigger（或等價機制）實現。**trigger 內必須先取序列化鎖再聚合**——否則 READ COMMITTED 下裸 `SUM()` 再比對會重演 L1 的 TOCTOU（兩併發交易各讀到偏低的 SUM 都通過），L3 形同虛設。取鎖手段擇一：
  - `PERFORM pg_advisory_xact_lock(hashtext(NEW.trade_no))`（交易級 advisory lock，同 `trade_no` 序列化，事務結束自動釋放）；或
  - 先 `SELECT 1 FROM payment_transactions WHERE trade_no = NEW.trade_no FOR UPDATE`（鎖對應交易 row）

  取鎖後才計算 `(SELECT COALESCE(SUM(amount),0) FROM payment_refunds WHERE trade_no = NEW.trade_no AND status IN ('0','1','2')) + NEW.amount`（`BEFORE INSERT` 時 NEW row 尚未寫入表，須顯式加上本次 `NEW.amount`，避免漏算本筆），與交易金額比對，超過即 `RAISE EXCEPTION` 拒絕寫入（比照 §2.4 第二層 Partial Unique Index 的 DB 兜底思路）
- 效果：即使 L1 行鎖 / L2 餘額計算因競態或未來新 caller 被繞過，L3 以自身的序列化鎖獨立成立，DB 仍拒絕使退款總額超過交易金額的寫入

**三層防禦總覽：**

| 層級 | 防禦位置 | 防禦對象 | 機制 |
|------|----------|----------|------|
| 第一層 | Application | 同交易併發 `createRefund` | `SELECT ... FOR UPDATE` 鎖交易 row |
| 第二層 | Application | 在途退款未計入已用額度 | 可退餘額含 `status IN ('0','1','2')` |
| 第三層 | Database | 競態繞過 / 未來新 caller | trigger：退款總額 ≤ 交易金額 |

**時序圖（兩筆併發全額退款，一筆先取得行鎖建立、另一筆等鎖後見餘額不足被拒）：**

```mermaid
sequenceDiagram
    autonumber
    participant A as Caller A<br/>(createRefund amount=100)
    participant B as Caller B<br/>(createRefund amount=100)
    participant DB as payment_transactions /<br/>payment_refunds
    A->>DB: SELECT ... FOR UPDATE (trade_no=T) — 取得行鎖
    B->>DB: SELECT ... FOR UPDATE (trade_no=T) — 等鎖（阻塞）
    A->>DB: 可退餘額 = 100 − SUM(在途+成功=0) = 100，足夠
    A->>DB: INSERT payment_refunds status=REQUESTED amount=100
    A-->>DB: COMMIT（釋放行鎖）
    B->>DB: 取得行鎖，重讀 SUM(在途+成功=100)
    B->>DB: 可退餘額 = 100 − 100 = 0，不足
    Note over B: 拒絕（退款金額超過可退餘額）
    Note over DB: 若 L1/L2 被繞過，L3 trigger 於 INSERT 時再擋一次
```

**涉及檔案 / 實作位置**：
- `application/service/RefundApplicationService.java`（`createRefund` 前置檢查與 L1 行鎖 / 釋鎖流程）
- `domain/repository/PaymentTransactionRepository.java` / `...repository/PaymentTransactionRepositoryImpl.java` 及 `infrastructure/persistence/mapper/PaymentTransactionMapper.java`：新增交易查詢的 `FOR UPDATE` 版本（如 `findByTradeNoForUpdate`，L1 鎖 `payment_transactions` row）
- `domain/repository/RefundTransactionRepository.java` / `...repository/RefundTransactionRepositoryImpl.java` 及 `infrastructure/persistence/mapper/RefundTransactionMapper.java`：
  - `sumRefundedAmountByTradeNo` SQL 改 `WHERE ... AND status IN ('0','1','2')`（在途 + 成功）
  - `hasProcessingRefund` SQL 改 `WHERE ... AND status IN ('0','1')`（REQUESTED + PROCESSING，不含 SUCCEEDED）
- 新增 Flyway `V{N+1}__add_refund_total_amount_guard.sql`（payment-service）：L3 `BEFORE INSERT/UPDATE` trigger（含 advisory / `FOR UPDATE` 序列化鎖後聚合比對）

### 5.6 撥款指示（GrantPayment）

> 🔴 **`grant` 與 `agent_fee` 是兩條不同的 PayUni 通道，不得混用**：
> - **`grant`（撥款指示）**＝ 平台把買家貨款撥給賣家，即本節的主題。
> - **`agent_fee`（代理商收付）**＝ 平台向賣家收取物流服務費，是完全不同的通道與費用。
>
> 兩條硬規則：① **費率不得寫成商業常數**（含把「1%」寫進 code 或設定檔）——精確費率、扣費時點、gross／net 方向、是否另行扣收，一律以**該筆交易的實際查詢值**記帳；② 該手續費**必須與物流商實際成本分帳**，不得併成單一數字（否則對帳時無法還原任一方）。手續費由誰吸收是業務決策，本服務不自行推定（見 §1.3）。

代理商模式下，所有交易建立時帶入 `GrantExport=1`，PayUni 不會自動將款項撥給商店。由代理商（本系統）在業務流程完成後（買家確認收貨），主動呼叫 PayUni Grant API 發動撥款指示。

**觸發來源**：Order Service 在買家確認收貨、訂單狀態更新為 COMPLETED 後，呼叫 `GrantPayment(order_no)` RPC。

**PayUni Grant API 規格**：

| 項目 | 值 |
|------|------|
| URL | `{base-url}/api/trade/grant` |
| 外層參數 | `AgentID`（代理商前綴字）+ `Version=1.0` + `EncryptInfo` + `HashInfo` |
| EncryptInfo 內層 | `MerID`（商店代號）+ `TradeNo`（UNi序號）+ `Amt`（金額） |
| 回應 | `Status=SUCCESS` 或錯誤代碼 |

> **關鍵差異**：Grant API 外層使用 `AgentID`（代理商前綴字，如 `HAWK`），與其他 API 使用 `MerID` + `IsPlatForm=1` 不同。

**鎖範圍（@Transactional 邊界，明文論證）**：`grantPayment` 標 `@Transactional`；第 2 步 `findActiveByOrderIdForUpdate` 對 `payment_transactions` 該 row 取得行鎖，**行鎖持有至事務結束**（涵蓋 PayUni Grant 呼叫與最終 `grant_*` 寫入）。

- **與 CreateRefund L1 一致（皆持鎖跨 PayUni）**：CreateRefund 亦全程單一 `@Transactional`、L1 行鎖持有至 PayUni 退款呼叫結束（見 §5.5.5 L1）；撥款採同一策略。撥款尤其**必須**持鎖跨呼叫——`grant_status` 在 PayUni 回應前不變，若中途釋鎖，併發 / 未來新 caller 的第二次撥款會同樣通過 `grant_status != SUCCESS` 檢查 → 雙重撥款；持鎖跨呼叫才能讓第二次撥款在取鎖後看到 `grant_status==SUCCESS` 而短路。
- **trade-off（務實取向，可接受）**：撥款為**低頻、單訂單一次性**操作（買家確認收貨才觸發一次），持鎖窗口短；此窗內同 row 的 CreateRefund L1 會被阻塞等待，屬可接受（撥款進行中序列化後續退款，語意上亦合理）。且併發型重撥已由上游 `grant_payment_tasks` 去重（見下），此行鎖主要作時序型重撥與未來新 caller 的縱深防禦。
- **鎖窗**：`FOR UPDATE` 行鎖跨 PayUni 呼叫持有至事務結束。🔴 **不得改成「鎖內寫中間標記後釋鎖、PayUni 呼叫於鎖外」**——那會新增一個撥款中間狀態，而 `grant_status` 的值域是「`SUCCESS` ∪ PayUni 錯誤碼 ∪ `PENDING_CONFIRM`」的異質欄位，多一個我方自造的中間值會讓 B 對帳的判定集合再分岔。

**業務流程**：

1. 接收 gRPC 請求：`order_no`
2. 查詢並鎖定交易：`findActiveByOrderIdForUpdate(order_no)`（`SELECT ... FOR UPDATE`）→ 取得已付款交易並鎖住該 row，序列化同交易的併發 / 重試撥款
3. **撥款前冪等短路**：鎖後檢查 `grant_status == SUCCESS` → 已撥款，直接回傳既有 `GrantPaymentResult`，**不再呼叫 PayUni**（避免重撥）
4. 前置驗證（非 SUCCESS 才續行）：
   - 交易必須存在且為 **PAID** 狀態
   - `provider_trade_no` 不為空（需有 PayUni UNi序號）
5. **重試前 grant_query 對帳去重（去重硬化 B）**：若本次非首撥（`grant_status` 非 null 且 ∈ {`PENDING_CONFIRM`、先前 PayUni 錯誤碼}）→ 先呼叫 PayUni 撥款查詢 `grant_query`，**逐日涵蓋 [首次發動嘗試日, 今日] 區間（≤30 天）+ 遍歷全分頁至 `PageInfo.Total`**（見「去重硬化 B」），以全域唯一 **`Result.TradeNo`==`provider_trade_no`（UNi 序號）精確比對即命中**（`Result.Type` / `Result.FundsAmt` 因[代理商 API §6.4](../第三方整合/PayUni%20代理商專用%20API%20總覽.md#64-返回參數)無代碼表、語意未證實，暫不作必要條件，僅 parse 入 log 供稽核，待 sandbox 實證後再收緊）：
   - **命中已入帳** → `UPDATE grant_status='SUCCESS', granted_at=NOW()` 補記終態並回傳既有 `GrantPaymentResult`，**不重發**
   - **全區間+全分頁查完仍查無** → 才續行發動；若尚未查完 / 落在入帳前（結算延遲）→ **維持 `PENDING_CONFIRM`、不盲目重發**，延到下一次對帳
   - 首撥（`grant_status` 為 null）不查 `grant_query`，直接發動
6. 呼叫 PayUni Grant API：
   - 外層：`AgentID=HAWK`（代理商前綴字，從設定檔讀取）、`Version=1.0`
   - EncryptInfo 內層：`MerID`=交易的 `mer_id`、`TradeNo`=交易的 `provider_trade_no`、`Amt`=交易的 `amount`
7. 處理回應（`invokeGrantWithRetry` **try/catch 捕捉 `PayUniApiException`**、連同非成功回應交 `GrantResponseRules` 分類，遵循[收斂規範 §8](../開發規範/PayUni%20錯誤回應收斂規範.md) 同步 retry **最多一次**；**失敗終態隨交易 commit 存活、不 throw rollback**，失敗訊號於 commit 後產生——見「去重硬化 A」）：
   - `Status=SUCCESS` → `UPDATE payment_transactions SET granted_at=NOW(), grant_status='SUCCESS', grant_message='撥款指示成功'` + 記錄 `payment_logs`
   - **明確未發動**（PayUni 明確拒絕碼 / 參數錯）→ 同步 retry **最多一次**；仍失敗 → `UPDATE grant_status=PayUni 錯誤碼, grant_message=PayUni 訊息`（隨交易 commit 存活）+ 記錄 `payment_logs`，commit 後回傳錯誤（進 Admin 異常清單）
   - **已由 PayUni 接管**（`GrantResponseClass=PROVIDER_TOOK_OVER`：判準 = **確定拒絕 + 存在已完成退款**。`hasSucceededRefund` 為**確定拒絕時才執行的延遲查詢**（查 `payment_refunds` 有無完成記錄），**不在每次 grantPayment 前置查**——正常撥款路徑零額外查詢成本；GRANT 拒絕碼僅為線索、判準碼不可知，見 §5.6.1）→ **不 retry**，`UPDATE grant_status=PayUni 回應碼原樣落地`、`granted_at` 保持 NULL（隨交易 commit 存活）+ 記錄 `payment_logs`，commit 後回**正常 gRPC 回應**（response `grant_outcome=PROVIDER_TOOK_OVER`，**非 error**；Order 端 Worker 據此收斂 task 為 `PROVIDER_AUTO`，[03 §3.11.2](./03-訂單微服務（Order%20Service）詳細設計書.md#3112-撥款已由-payuni-接管的收斂worker-韌性)）
   - **模稜兩可**（`PayUniApiException`：timeout / 連線中斷 / `HTTP_ERROR` / `EMPTY_RESPONSE`；非成功回應：`PARSE_ERROR` / `HASH_INVALID` / 未知碼）→ **不自動重試**，`UPDATE grant_status='PENDING_CONFIRM', grant_message=原因`（隨交易 commit 存活）+ 記錄 `payment_logs`，commit 後回傳錯誤（進 Admin；下輪重試由第 5 步 `grant_query` 對帳把關）

**冪等定位與去重層級（誠實標註）**：

- **併發型重撥已由上游 Order Service 擋住**：`grant_payment_tasks` 以 `UNIQUE(order_no)` + `claimTask` CAS 保證同一訂單的撥款任務只被領取執行一次，故併發重複觸發 `GrantPayment` 在上游即被去重。
- **本守衛的定位**：payment-service 側的 `@Transactional` + `FOR UPDATE` + `grant_status == SUCCESS` 短路，防的是**時序型重撥**（partial-success 後的重試、in-method 立即重試）與**未來新 caller** 直呼 `GrantPayment` 的縱深防禦，並非取代上游去重。
- **`grant_status == SUCCESS` 短路擋不掉的那個窗口**：「PayUni 實際已撥款、但我方在處理回應時記成失敗」（例如撥款成功而回應 timeout）。撥款**無 webhook**、交易查詢 `trade/query` 也沒有撥款狀態欄位 ⇒ 唯一的收斂手段是 `grant_query`（[代理商專用 API 總覽 §6](../第三方整合/PayUni%20代理商專用%20API%20總覽.md#6-撥款查詢-grant_query)）：按日（`QueryDate`）對帳當日撥款入帳明細（`Result` 含 `TradeNo` / `ExecuteAmt` / `FundsAmt` / `Type`）。下方去重硬化 A ＋ B 即是為此。

**去重硬化（A：收緊重試 + B：grant_query 對帳）**：

以下兩層在既有 `grant_status == SUCCESS` 短路之上，收斂 partial-success 重撥窗口。

| 層級 | 消除的窗口 | 機制 | 落地位置 |
|------|-----------|------|---------|
| **A. 收緊撥款重試** | 「入帳前」即時雙重撥款 | 撥款回應分「明確未發動 / 模稜兩可」，模稜兩可不自動重試、標 `PENDING_CONFIRM` | `GrantResponseRules`（純函式）+ `invokeGrantWithRetry` |
| **B. grant_query 對帳去重** | 「入帳後」重試雙重撥款 | 重試發動前先 `grant_query` 查是否已入帳，已撥則標 `SUCCESS` 不重發 | `PaymentApplicationService.grantPayment` 重試前（單一寫入路徑內同步） |

**A. 收緊撥款重試（消除「入帳前」即時雙重撥款）**

🔴 **兩條會讓 `grant_status` 停在 null 的寫法，兩條都禁止**（`grant_status` 停在 null ⇒ 重試時「非首撥」條件永不成立 ⇒ **B 對帳永不觸發、整個去重硬化形同虛設**，且首次其實已發動成功時仍會無條件重打同一個 `TradeNo` ⇒ 雙重撥款）：

| 禁止的寫法 | 為什麼 |
|---|---|
| `invokeGrantWithRetry` 對 `paymentClient.grant()` **不加 try/catch** | timeout／連線中斷／`HTTP_ERROR`／`EMPTY_RESPONSE` 由 `PaymentClient` 丟 `PayUniApiException`（RuntimeException），直接穿透 `grantPayment` ⇒ **連寫入都沒執行**。而這正是「PayUni 可能已收到指示」的最危險路徑 |
| 失敗分支「先寫 `grant_*` 再 throw」 | `grantPayment` 標 `@Transactional`，Spring 對 RuntimeException 回滾 ⇒ **剛寫的 `grant_*` 被撤銷**，等於沒寫 |

- **分類**：撥款回應分兩類——
  - **明確未發動**（PayUni 明確拒絕碼 / 參數錯）→ PayUni 未受理，**可安全重試**。
  - **模稜兩可**（`PayUniApiException`：timeout / 連線中斷 / `HTTP_ERROR` / `EMPTY_RESPONSE`；非成功回應：`PARSE_ERROR` / `HASH_INVALID` / 未知碼 = PayUni **可能已收到指示**）→ **不自動重試**，`grant_status` 標新哨兵值 `PENDING_CONFIRM`。
- 🔴 **關鍵前提 — 哨兵值／終態必須存活（不得被撥款失敗的 rollback 撤銷）**：
  - `invokeGrantWithRetry` 必須 **try/catch 捕捉 `PayUniApiException`**（涵蓋 timeout / IO / `HTTP_ERROR` / `EMPTY_RESPONSE`），連同非成功回應（`PARSE_ERROR` / `HASH_INVALID` / 未知碼）**一律轉交 `GrantResponseRules` 判類，杜絕例外穿透**（否則寫入根本不執行）。
  - 失敗分支的終態 `grant_status`（PayUni 錯誤碼 或 `PENDING_CONFIRM`）**寫入必須隨交易 commit 存活**。鎖範圍要求 `FOR UPDATE` 行鎖**跨 PayUni 呼叫持有至事務結束**，而 `grant_status` 就在這把被鎖的 row 上 ⇒ 採 write-then-return：**在持鎖的同一 `@Transactional` 內寫入終態後讓交易 commit，不以 throw 觸發 rollback**（同 `RefundApplicationService.handleRefundFailure` 的安全模式）。對 Order Service 的失敗訊號改在**交易 commit 之後**產生（由非交易的外層薄包裝方法依已落地的 `grant_status != SUCCESS` 拋 `PaymentBusinessException`，或由 gRPC adapter 依回傳狀態對應錯誤），保留 `grant_payment_tasks` 既有的跨呼叫重試。
  - **明文拒絕 REQUIRES_NEW 獨立交易寫哨兵值**：外層 `@Transactional` 已對同一 `payment_transactions` row 持 `FOR UPDATE` 行鎖；REQUIRES_NEW 於**獨立連線** UPDATE 同一 row 會 block 在該行鎖上、而外層又懸置等待內層返回 → **自我死鎖（lock wait timeout）**。故終態寫入落在持鎖的同一交易內、以「不拋例外正常 commit」存活，而非另開交易。
  - 🔴 **adapter 契約必須同步，否則失敗被誤回 SUCCESS**：write-then-return 讓內層 `grantPayment` 失敗時**正常 return**（不 throw）。若 adapter `PaymentGrpcService.grantPayment` 把 service 的**任何非例外回傳**都組成 gRPC SUCCESS，且 `GrantPaymentResult` 無欄位承載失敗 ⇒ 失敗會被回成 SUCCESS ⇒ Order Service 的 `grant_payment_tasks` 誤判成功不再重試 ⇒ **連 B（grant_query 對帳）都進不了重試路徑**。必配套：
    - 內層回傳型別（`GrantPaymentResult` 或等價 DTO）**新增 `grantStatus` / `grantMessage` 欄位**，讓 adapter 能判斷成敗。
    - `PaymentGrpcService.grantPayment`（adapter，在 `@Transactional` **外**）依內層回傳的 `grantStatus`：`SUCCESS` → 回 gRPC success response；`PENDING_CONFIRM` / PayUni 錯誤碼 → **throw gRPC error**（即由此處產生失敗訊號，維持既有「Order task 收 gRPC error → 重試」語意）。
    - **gRPC 契約變更（本次唯一 proto 變更）**：失敗（`PENDING_CONFIRM` / PayUni 錯誤碼）對 Order Service 仍是 gRPC error；`PROVIDER_TOOK_OVER` 則走**正常回應**——`GrantPaymentResponse`（`api/proto/payment/payment.proto`）**新增 `grant_outcome` string 欄位**（值域 `GRANTED` / `PROVIDER_TOOK_OVER`；內部服務間 gRPC，依工程契約 §4.1 例外 3），Order 端 `GrantPaymentWorker` 需讀取該欄位收斂 task（03 §3.11.2 / §4.3）。Proto 同步流程照工程契約 §3（改 `api/proto` → `gen-go-proto.sh` → rsync → `mvn clean install`）。throw 的**位置**從 `@Transactional` 內移到 adapter 層——內層 commit 讓哨兵存活，外層 adapter 依落地的 `grant_status` 轉 gRPC error / 正常回應。這是 write-then-return 的**必要配套**，否則 Order 端收到假 SUCCESS。
  - 🔴 **終態哨兵值若不 commit 存活，B 對帳永遠讀不到 `PENDING_CONFIRM` 或錯誤碼，整套硬化形同虛設。**
- **`PENDING_CONFIRM` 語意**：為 `grant_status` 的**裸字串哨兵值**（非 CodeMeta enum——`grant_status` 欄位是「`SUCCESS` ∪ PayUni 錯誤碼」的異質欄位，不套 CodeMeta），常數集中於 `PayUniConstants`；因非 SUCCESS，該筆自動進 Admin 異常清單（[收斂規範 §8.4](../開發規範/PayUni%20錯誤回應收斂規範.md#84-人工介入閉環) 人工介入閉環）。
- **守收斂規範（§8.1 / §8.2 / §8.3 明文標註）**：
  - 分類邏輯放 payment-service **純函式 helper**（單一服務 → `services/java/payment-service/.../domain/GrantResponseRules.java`，對齊既有 `domain/PaymentStatusRules.java` / `domain/RefundStatusRules.java` 慣例；依 [Java 微服務開發規範 §6](../開發規範/Java%20微服務開發規範.md) 與 [代碼管理（CodeMeta）](../代碼管理（CodeMeta）/README.md)——**不**放進 CodeMeta enum、不含 I/O）。
  - **只持久化單一 `grant_status` 值**（`SUCCESS` / PayUni 錯誤碼 / `PENDING_CONFIRM`）；state 表 `payment_transactions` **禁**新增 `error_type` 欄位、**禁** `retry_count` / `next_retry_at`（[收斂規範 §8.2 / §8.3](../開發規範/PayUni%20錯誤回應收斂規範.md#82-state-表-vs-task-表的欄位規範)——分類靠 PayUni 錯誤碼本身，retry 屬 Order 側 task 表 `grant_payment_tasks`）。
  - 同步 retry 仍**最多一次**（[§8.1](../開發規範/PayUni%20錯誤回應收斂規範.md#81-重試原則分三類場景)；模稜兩可為 0 次，仍在「最多一次」語意內）。

**B. 重試發動前 grant_query 對帳去重（消除「入帳後」重試雙重撥款）**

- 對 `grant_status ∈ {PENDING_CONFIRM、PayUni 錯誤碼}` 的撥款，**重試發動前先呼叫 PayUni `grant_query`**（新串接，`callApi` + 外層 `MerID` + `IsPlatForm=1`，[總覽 §6](../第三方整合/PayUni%20代理商專用%20API%20總覽.md#6-撥款查詢-grant_query)），在撥款入帳明細 `Result[]` 中以全域唯一 **`Result.TradeNo`==`provider_trade_no`（UNi 序號）精確比對**尋找命中：
  - **命中判準（僅 TradeNo）**：`TradeNo` 為 PayUni 全域唯一序號，命中即代表該撥款已進入撥款帳務，直接判為已撥。`Result.Type`（入帳/出帳）與 `Result.FundsAmt`（疑為扣費後淨額）因[文件 §6.4](../第三方整合/PayUni%20代理商專用%20API%20總覽.md#64-返回參數)無代碼表、語意未證實，**暫不作必要條件**（用未證實猜測值當必要條件會 false-negative → 誤判未撥 → 重複撥款，正是要消除的最大風險）；兩者僅 parse 入 log 供稽核。
  - **命中已入帳** → 標 `grant_status='SUCCESS'` + `granted_at`，**不重發**。
  - **查無入帳（僅在下述完整涵蓋後才成立）** → 才安全重發。
- **查詢範圍必須完整**（涵蓋不足會誤判「未撥」而錯誤重發）：
  - **日期區間非單日**：撥款「隔日入帳」不保證恰為 1 個日曆天（假日 / 週末結算延遲），故 `QueryDate` 需**逐日涵蓋 [首次模稜兩可發動嘗試日, 今日] 區間**（受 grant_query **30 天內**上限約束；區間起點取交易的 `updated_at`——每次撥款嘗試寫入終態時都會更新，故為有效基準；**不可用 `granted_at`**，其僅 SUCCESS 分支寫入、失敗路徑為 null）。只查「發動日隔日」單日，遇結算延遲會查無 → 反而重發（正是要消除的風險）。
  - **遍歷分頁**：grant_query 每頁固定 **500 筆**並回 `PageInfo.Current` / `PageInfo.Total`（[總覽 §6.4](../第三方整合/PayUni%20代理商專用%20API%20總覽.md#64-返回參數)），單日資料可能多頁；必須**逐頁查至 `PageInfo.Total`** 才算查完當日，否則可能漏掉目標 `TradeNo`。
  - **「查無入帳」判定**：唯有**應涵蓋日期區間逐日 + 各日全分頁**都比對過仍無命中，才成立「未撥」→ 可安全重發；否則（尚未覆蓋完 / 落在入帳前）→ **維持 `PENDING_CONFIRM`、延到下一次對帳**（不盲目重發）。
  - **逐日對帳的「有效空日」判定**：grant_query 對「窗口內某日無撥款」回外層 `Status=`**`QUERY03002`**（查無符合撥款資料，**非 `SUCCESS`**）——這是**有效空結果**、常態（撥款「隔日入帳」，首次發動嘗試日當天對本交易本就恆空）。故 `QUERY03002` 須視為**該日有效空頁（`count=0`）並續掃下一日**，**不**得當成查詢失敗。反之唯有 **`QUERY02004`**（查詢日期須為 30 天內／超窗）／ transport 錯誤（`PayUniApiException`）／其他非預期 `Status` 才算「該日未查完」→ 維持 `INCONCLUSIVE`（不重發，延到下一次對帳）。判準集中於純函式 `GrantResponseRules.isGrantQueryDayComplete(isSuccess, status)`（`SUCCESS` 或 `QUERY03002` 皆為「該日已有效查完」），`QUERY03002` / `QUERY02004` 以具名常數 `GrantResponseRules.GRANT_QUERY_NO_DATA` / `GRANT_QUERY_OUT_OF_WINDOW` 定義，不裸字串硬編。**若把 `QUERY03002` 誤當失敗**：逐日對帳第一天（嘗試日空日）即誤回 `INCONCLUSIVE`、抵達入帳日前就返回 → 對帳 B「自動判定已撥／未撥」退化成永遠 `PENDING_CONFIRM` 需人工（安全不重撥、但功能失效）。
- **守責任邊界（明文標註）**：`grant_query` 對帳邏輯放在**撥款重試的單一寫入路徑內同步執行**（`grantPayment` 重試發動前，含 Order 側 `grant_payment_tasks` worker 重跑 / Admin `retry_grant` 觸發的重跑），**明文不引入獨立 reconciliation job / 對帳排程**（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)：目的為業務正確性而非資料完整性監測，採同事務單一寫入路徑）。

**涉及檔案 / 實作位置**：
- `application/service/PaymentApplicationService.java`（`grantPayment` / `invokeGrantWithRetry`：加 `@Transactional`、鎖範圍如上、撥款前 `grant_status == SUCCESS` 冪等短路；**收緊**——`invokeGrantWithRetry` 對 `paymentClient.grant()` 補 try/catch **捕捉 `PayUniApiException`**、連同非成功回應交 `GrantResponseRules` 判類，模稜兩可不重試改標 `PENDING_CONFIRM`；**失敗終態改為隨交易 commit 存活**（不再「寫入後 throw」導致 rollback，比照 `RefundApplicationService.handleRefundFailure`），失敗訊號改於 `@Transactional` commit 後由**非交易外層薄包裝 method**（或 gRPC adapter）依 `grant_status != SUCCESS` 產生，避免 self-invocation 使外層 method 的交易語意失效；重試發動前執行 grant_query 對帳去重（B），全程在同一持鎖寫入路徑內同步）
- `domain/repository/PaymentTransactionRepository.java` / `...repository/PaymentTransactionRepositoryImpl.java` 及 `infrastructure/persistence/mapper/PaymentTransactionMapper.java`：新增 `findActiveByOrderIdForUpdate`（`SELECT ... WHERE order_id=? AND status=1 FOR UPDATE`）
- `domain/GrantResponseRules.java`（**新增**純函式 helper，依 [代碼管理（CodeMeta）](../代碼管理（CodeMeta）/README.md)「CodeMeta enum 不含業務輔助邏輯」：撥款回應分類「明確未發動（可重試）/ 模稜兩可（含捕捉的 `PayUniApiException`，不重試、標 `PENDING_CONFIRM`）/ **已由 PayUni 接管（`GrantResponseClass` 新增第三值 `PROVIDER_TOOK_OVER`：主判準=存在已完成退款，GRANT 拒絕碼僅為線索、碼不可知，不重試、不進對帳迴圈；與 `ReconcileOutcome.HIT_ALREADY_GRANTED` 為不同機制、不合流，§5.6.1）**」；對齊既有 `domain/PaymentStatusRules.java` / `domain/RefundStatusRules.java`；不含 I/O、不進 CodeMeta enum。另含 grant_query 逐日對帳的「該日是否有效查完」判斷 `isGrantQueryDayComplete(isSuccess, status)`（`SUCCESS` 或 `QUERY03002` 皆為有效查完；`QUERY02004`/transport/其他 → 未查完），及具名常數 `GRANT_QUERY_NO_DATA="QUERY03002"` / `GRANT_QUERY_OUT_OF_WINDOW="QUERY02004"`，不裸字串硬編）
- `constant/PayUniConstants.java`：新增 `PENDING_CONFIRM` 哨兵字串常數（`grant_status` 專用）
- `services/java/shared/payuni-java-client/.../PayUniPaymentEndpoint.java`：新增 `PATH_GRANT_QUERY = "/api/trade/grant_query"`（版本 `1.0`）
- `services/java/shared/payuni-java-client/.../PaymentClient.java`：新增 `grantQuery(merId, queryDate)`（`callApi` + `IsPlatForm=1`，回傳撥款入帳明細 `Result[]`）
- `application/dto/GrantPaymentResult.java`（record 新增 `grantStatus` / `grantMessage` / `grantOutcome` 欄位，承載失敗 / `PENDING_CONFIRM` / `PROVIDER_TOOK_OVER`，供 adapter 判斷回 success / 正常 outcome 回應 / throw error）
- `adapter/grpc/PaymentGrpcService.java`（`grantPayment` 在 `@Transactional` 外，依內層回傳決定 → `SUCCESS` 回 gRPC success（`grant_outcome=GRANTED`）；`PROVIDER_TOOK_OVER` 回**正常 gRPC 回應**（`grant_outcome=PROVIDER_TOOK_OVER`，非 error）；`PENDING_CONFIRM` / 其他 PayUni 錯誤碼 → throw gRPC error。proto 變更僅 `GrantPaymentResponse` 加 `grant_outcome` 欄位，Order 端 Worker 配合讀取）
- `api/proto/payment/payment.proto`：`GrantPaymentResponse` **新增 `grant_outcome` string 欄位**（`GRANTED` / `PROVIDER_TOOK_OVER`；工程契約 §3 同步流程 / §4.1 例外 3）

**撥款欄位封閉規則（見收斂規範 §2.3 / §6.2）**：`granted_at` / `grant_status` / `grant_message` **只能暴露在 Admin API**，買家 / 賣家的訂單或付款 App API 嚴禁出現這些欄位。

**人工介入閉環**：撥款失敗時走 Admin 異常清單（`/api/v1/admin/grants`）→ 運營登入 PayUni 後台調整 → 重發撥款。本服務不提供手動重試撥款 API（規範 §8.4）。

#### 5.6.1 退款交易的 PayUni 主動撥款（領域事實，撥款語意主場）

> 本節為「退款 × 撥款」PayUni 領域事實的 SSOT；Order 端任務建立（一律 PENDING）與收斂規則見 [03 §3.11.1 / §3.11.2](./03-訂單微服務（Order%20Service）詳細設計書.md#3111-建立撥款任務一律-pending不預分流)。

**PayUni 官方規則**：

- **退款（含部分退款）觸發 PayUni 主動撥款**：交易一旦發生退款（不論全額或部分、不論我方是否已發撥款指示），PayUni 即對該交易接管撥款——剩餘應撥金額由 PayUni 主動撥給商家，**不需**（也**不受理**）我方補發撥款指示。
- **成對分錄**：PayUni 主動撥款以成對分錄呈現——`+原交易金額` 與 `−退款金額` 兩筆，手續費按退款比例回沖。`grant_query` 對帳時會看到同一 `TradeNo` 的成對 `Result` 記錄，不可只比對單筆金額。
- **入帳日 = 退款完成日 + 7（台北日曆日）**：D+7 規則以台北時區日曆日計；多筆退款時以**最後一筆退款完成日**起算。
- **補發指示會被拒絕（實測回 `GRANT30023`）**：對已進退款接管的交易呼叫 Grant API，PayUni 拒絕補發。此為**預期收斂訊號**而非錯誤（見下方回應分類）。⚠️ `GRANT30023` **非退款專屬碼**——另有「付款當日請款批次未結」的時間性場景回同碼（混淆因子）；相鄰場景碼 `GRANT30018`（已撥款）、`GRANT30019`（已有撥款日期）同屬「不應/不需再發指示」家族。
- **排程中不可事前查詢**：PayUni 主動撥款在入帳前處於 PayUni 端內部排程，`grant_query` **查不到未入帳的排程記錄**（僅入帳後可對帳）；`trade/query` 亦無撥款狀態欄位。故 D+7 窗口內「查無」不代表「未撥」，不可據此重發指示。
- 入帳日細則：信用卡退款——撥款入帳日同退款的撥款入帳日；非信用卡（ATM 等）——退款轉匯，撥款入帳日為退款發動隔日。

**「已由 PayUni 接管」回應分類（`GrantResponseRules`，碼不可知）**：既有「明確未發動 / 模稜兩可」二分之外，獨立第三類「**已由 PayUni 接管**」——

- **主判準 = 確定拒絕 + 該交易存在已完成退款**（`hasSucceededRefund`：查 `payment_refunds` 有無完成記錄，**確定拒絕時才延遲查詢**，不在每次 grantPayment 前置查——正常撥款零額外成本）。GRANT 拒絕碼（`GRANT30023` 實測於退款接管、`GRANT30018` 已撥款、`GRANT30019` 已有撥款日期）**僅作觸發線索列舉，不得把 `GRANT30023` 硬編為唯一判準**——30023 另有「付款當日請款批次未結」時間性混淆因子，無退款而回 30023 時屬可重試場景、不收斂
- **實作註記**：`GrantResponseClass` 於現有 `DEFINITELY_NOT_SENT` / `AMBIGUOUS` 二值之外**新增第三值 `PROVIDER_TOOK_OVER`**，映射為不重試、以**正常 gRPC 回應**（`grant_outcome=PROVIDER_TOOK_OVER`）通知 Order 端收斂 task。與 `ReconcileOutcome.HIT_ALREADY_GRANTED`（`PaymentApplicationService` 內 grant_query **對帳結果**的分類）為**不同機制、不合流**——前者判「回應當下該不該重試」，後者判「對帳後是否已入帳」
- `grant_status` 落 PayUni 回應碼原樣、`granted_at` 保持 NULL（隨交易 commit 存活，同 A 段安全模式）；adapter 回**正常 gRPC 回應**，response `grant_outcome=PROVIDER_TOOK_OVER`（**非 error**）
- **不同步 retry、不進 `PENDING_CONFIRM` 對帳迴圈**：判準成立即代表 PayUni 已接管，重試必然同碼
- Order Service `GrantPaymentWorker` 收到 `grant_outcome=PROVIDER_TOOK_OVER` → task 收斂 `PROVIDER_AUTO(50)`（不重試、不進人工介入；**不做本地 `refund_total_amount` 啟發式**——凡 gRPC error 照既有失敗路徑），見 [03 §3.11.2](./03-訂單微服務（Order%20Service）詳細設計書.md#3112-撥款已由-payuni-接管的收斂worker-韌性)
- 帳務定位：`PROVIDER_TOOK_OVER` 交易的 `granted_at` **刻意保持 NULL**（PROVIDER_AUTO 為終態，Worker 不再撈取、`grantPayment` 不再被呼叫，系統內**無**補記觸發路徑）；帳務權威 = PayUni 端撥款明細 + `payment_refunds` / `financial_logs`。**不為此建立任何對帳 / 巡檢機制**（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)：資料完整性監測不進 application code）；維運需要核對時以 `grant_query` 腳本人工執行

### 5.7 逾期處理排程

不再使用 RabbitMQ TTL 延遲訊息。改為**排程輪詢**：

1. `ExpiredTransactionScheduler` 每 60 秒執行
2. 查詢 `status = INIT AND expire_at <= NOW()` 的交易（batch 100 筆）
3. 對每筆交易：
   - 呼叫 `PaymentDomainService.canMarkAsExpired()` 確認可轉移
   - 條件式更新：`updateToExpired(tradeNo, version)`
   - 成功 → **在同一事務內** `PaymentOutboxWriter.enqueueAndDispatch(PaymentExpired → payment.events / payment.expired)`（《多副本排程搶佔規範》例外 A）
4. OrderSvc 消費 MQ 事件 → 釋放庫存

> **⚠️ 搶佔語意**
> `updateToExpired` 的 CAS（`WHERE trade_no=? AND status='0' AND expire_at < #{now} AND version=#{version}`）**確實擋住了跨 pod 重複**——`status='3'` 是終態，第二個 pod 拿到 `affected=0`。**但它是永久狀態轉換，不是租約**：CAS 成功後 row 永久離開候選集合（候選述詞 `status='0'`），若後續 publish 失敗，`payment.expired` **永久遺失、無告警、無重試**，訂單的庫存與券再也不會被釋放。
>
> 🚨 **`@Transactional` 在這裡完全沒有保護作用，這一點最反直覺、必須寫明**：`processExpiredTransactions` 有 `@Transactional`，但 publish 的例外被**同一個方法內**的 per-item `catch` 吞掉 ⇒ 例外沒有穿過 Spring proxy ⇒ **事務不會被標 rollback-only ⇒ `updateToExpired` 照樣 commit**。看到 `@Transactional` 就判定安全，是這一筆最容易犯的錯。
>
> **走《[多副本排程搶佔規範](../開發規範/多副本排程搶佔規範.md)》例外 A（同 tx outbox）。** payment-service **已有 `outbox_events` ＋ `PaymentOutboxWriter`**（[§5.10.1](#5101-付款結果事件原子化--即時投遞)）⇒ 把 `PaymentExpired` 改成在 `updateToExpired` 的**同一個事務內** `enqueueAndDispatch`，由 outbox 保證至少一次送達。基建成本幾乎為零，**不需要引入 lease、不需要 migration**。
> 🔴 **不得改走例外 B（把 publish 移到 CAS 之前）**：`payment.expired` 的下游是 order-service 的釋放庫存／券流程，重複投遞的代價不可比照通知類事件。
>
> 🚩 **事務邊界：逐筆跨 bean 短 tx，不得整批一個 tx。**
> 只把 `eventPublisher.publish(...)` 換成 `outboxWriter.enqueueAndDispatch(...)` **並不足夠** —— 上面那段自己寫明的問題（per-item `catch` 在同一方法內吞掉例外 ⇒ 不標 rollback-only）對 **outbox INSERT 失敗**同樣成立：序列化失敗被吞 ⇒ `updateToExpired` 照樣 commit ⇒ 又回到「狀態已轉、事件沒落 outbox」。
> ⇒ 新增 `ExpiredTransactionCloseService.closeExpiredInTx(transaction)`（`@Transactional`，承載 CAS ＋ `enqueueAndDispatch`），`ExpiredTransactionScheduler` **移除自身的 `@Transactional`**、逐筆跨 bean 呼叫（同 class 內 `this.` 呼叫不經 proxy ⇒ 事務失效，見 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md)）。
> **rollback 後仍滿足候選述詞**：`status` 仍為 `'0'`(INIT)、`expire_at` 仍已過 ⇒ `findExpiredTransactions` 下一 tick（60 秒）必然重撈。
> **殘餘風險（誠實標註）**：commit 成功、afterCommit 即時投遞失敗 ⇒ 事件留在 `outbox_events` PENDING，由 `OutboxEventPublisherScheduler` 每 5s 兜底重投 —— 事件不會遺失，只會延遲。
> **附帶效果**：逐筆短 tx 讓每一筆彼此隔離；整批一個 tx 則是長交易，且一筆失敗會讓整批 rollback。

#### 5.7.1 待付款交易補收排程（PendingPaymentReconcileScheduler）

**目的**：作為 PayUni Webhook 的 L2 安全網。實測 PayUni Webhook 失敗（5xx / timeout）後**不會自動重送**，僅提供「後台手動補發」與「交易查詢 API」兩種補救手段。為避免使用者實際付款成功但因 Webhook 漏收導致訂單卡 INIT 直到 expire，本排程主動掃描 INIT 交易並呼叫 PayUni 交易查詢 API（`/api/trade/query`）拉取最新狀態。

**時序圖（INIT 主路徑 + EXPIRED rescue 退款路徑）**：

```mermaid
sequenceDiagram
    autonumber
    participant Sched as PendingPaymentReconcileScheduler
    participant Repo as PaymentTransactionRepository
    participant App as PaymentApplicationService
    participant Refund as RefundApplicationService
    participant Client as PaymentClient<br/>(payuni-java-client)
    participant PayUni as PayUni 交易查詢 API
    participant Outbox as payment_outbox
    participant MQ as RabbitMQ
    participant Order as order-service

    Note over Sched: 每 5 分鐘 fixedDelay 觸發
    Sched->>Repo: findReconcileCandidates(batch=50)<br/>WHERE status IN (INIT, EXPIRED)<br/>AND created_at < now()-3min<br/>AND expire_at > now()-30min<br/>AND last_reconciled_at < now()-5min
    Repo-->>Sched: List<PaymentTransaction>

    loop 每筆交易
        Sched->>Repo: claimForReconcile(tradeNo, now, leaseUntil)<br/>CAS: SET last_reconciled_at = now<br/>WHERE 候選述詞 AND last_reconciled_at 已到期
        alt affected == 0（另一個 pod 已搶到）
            Sched->>Sched: skip 本筆，continue
        end
        Sched->>App: syncPaymentStatus(tradeNo)
        App->>Repo: findByTradeNo
        Repo-->>App: transaction
        App->>Client: queryTransaction(merId, tradeNo)
        Client->>PayUni: POST /api/trade/query
        PayUni-->>Client: {TradeStatus, ...}
        Client-->>App: PayUniApiResponse

        alt INIT + PayUni TradeStatus=1（主路徑）
            App->>Repo: updateToPaid(tradeNo, version)
            App->>Outbox: 寫入 PaymentPaid event
            App-->>Sched: PaymentDetailResponse(PAID)
            Outbox->>MQ: OutboxPublisher 發布
            MQ->>Order: payment.paid 事件
        else EXPIRED + PayUni TradeStatus=1（rescue 路徑）
            App-->>Sched: 偵測衝突（status==EXPIRED 但 PayUni SUCCESS）
            Sched->>App: handleReconcileConflict(tradeNo, response)
            App->>App: 寫 payment_logs (WARN, EXPIRED→rescue)
            App->>Refund: createRescueRefund(tradeNo, fullAmount,<br/>reason=RESCUE_RECONCILE_AFTER_EXPIRED,<br/>idempotencyKey=rescue:{tradeNo})
            Note right of Refund: 跳過 canRequestRefund<br/>保留 idempotency / 金額驗證
            alt 信用卡 / Apple Pay / Google Pay
                Refund->>Client: refund(...) → /api/trade/close
                Client-->>Refund: SUCCESS
                Refund->>Repo: 退款記錄轉 SUCCEEDED
                Refund->>Outbox: 寫入 RefundSucceeded
                App->>Repo: 標記 rescued_at + rescue_action='8901'（REFUNDED）
            else ATM 轉帳（無法全自動）
                Refund->>Repo: 退款記錄停在 REQUESTED
                App->>Repo: 標記 rescued_at + rescue_action='8902'（REFUND_PENDING_MANUAL）
                Note over App: 不發 MQ 事件<br/>rescue_action=8902 由 sql_exporter 告警
            end
            App->>Outbox: 寫入 PaymentRescueRefundInitiated
        else PayUni 回 9/2/3/4 等非成功 / 查詢失敗
            App-->>Sched: 維持原狀，下次再試
        end

        Note over Sched,Repo: last_reconciled_at 已於 claim 當下寫入<br/>不再於 finally 補 stamp
        Sched->>Sched: sleep 100ms
    end
    Sched->>Sched: 紀錄 metrics (scanned/updated/conflict/rescue.*)
```

**設計總則**：
- **匯流到主路徑**：reconcile 不另寫狀態變更邏輯，完全復用 `PaymentApplicationService.syncPaymentStatus(tradeNo)`。webhook 路徑與 reconcile 路徑共用同一個 application service method，確保狀態機、Outbox、下游事件發布行為完全一致。
- **不取代 webhook**：webhook 仍是主路徑（秒級即時、低成本），reconcile 是補救路徑（分鐘級延遲、高成本），兩者互補。
- **不重複動作**：`syncPaymentStatus` 內已有「終態直接返回」早退，冪等由狀態機本身保證。

**掃描條件**（候選交易）：

| 條件 | 說明 |
|------|------|
| `status IN ('0', '3')`（INIT 或 EXPIRED） | INIT 為主要對象；EXPIRED 在 grace 視窗內仍納入掃描，用於觸發 rescue 自動退款（見「過期邊界 rescue 流程」） |
| `created_at < NOW() - INTERVAL '3 minutes'` | Initial delay：給 webhook 主路徑 3 分鐘優先窗口，避免每筆新交易立刻被搶查 |
| `expire_at > NOW() - INTERVAL '30 minutes'` | Grace 視窗：交易過期後 30 分鐘內仍可 reconcile，覆蓋三類情境：① webhook 故障期間 ② 我方/PayUni 時鐘飄移 ③ PayUni 內部 `DataSource=B`（處理中未完整）延遲落地 |
| `last_reconciled_at IS NULL OR last_reconciled_at < NOW() - INTERVAL '5 minutes'` | 退避：同筆交易最少間隔 5 分鐘才再 reconcile，避免熱循環與 PayUni rate limit 壓力 |

**排程參數**：

| 參數 | 預設值 | 說明 |
|------|--------|------|
| `fixedDelay` | 5 分鐘（300_000 ms） | 平衡即時性與 API 成本 |
| `BATCH_SIZE` | 50 | 單次 tick 最多處理筆數 |
| 單筆呼叫間 sleep | 100 ms | 對 PayUni 友善 |
| Initial delay | 3 分鐘 | 給 webhook 優先窗口 |
| Retry interval | 5 分鐘 | 同筆交易最小重撈間隔 |
| Grace（過期後容忍） | 30 分鐘 | 涵蓋 webhook 故障 / 時鐘飄移 / `DataSource=B` 延遲 |

**處理流程**：

```
1. 撈出符合掃描條件的 INIT 交易（最多 BATCH_SIZE 筆）
2. 對每筆交易：
   a. claim（lease CAS）：SET last_reconciled_at = NOW()
      WHERE trade_no = ? AND <候選述詞> AND (last_reconciled_at IS NULL OR last_reconciled_at < NOW() - retryInterval)
      affected == 0 → skip 本筆（另一個 pod 已搶到），continue
   b. 呼叫 syncPaymentStatus(tradeNo)
      - 復用既有：呼叫 PayUni queryTransaction、解析 TradeStatus、條件式更新狀態、發 Outbox
   c. sleep 100 ms
3. 紀錄本次 tick 的 metrics（scanned/claimed/skipped/updated/failed）
```

**多副本搶佔**

`payment-service` 在 prod **確實是 `replicas: 2`**（overlay 覆寫掉預設的 `1`；副本數是本規範的判定輸入且會變，查證方式見[規範 §〇](../開發規範/多副本排程搶佔規範.md)）⇒ 兩個 pod 每 5 分鐘各撈同一批候選 ⇒ **同一筆交易會對 PayUni 查詢 API 打兩次**；更嚴重的是 rescue 分支會走到 `createRescueRefund`（真實退款）。

| 項目 | 內容 |
|---|---|
| 租約欄 | **既有的 `payment_transactions.last_reconciled_at`**（候選述詞本來就是租約形狀），**零 migration** |
| 租約長度 | `retryIntervalMinutes` 預設 **5 分鐘**（stamp **在 claim 當下**取，見下方 🚩） |
| 候選窗口寬度 | **≥ 30 分鐘**（`expire_at > NOW() - grace(30min)`；row 在 `expire_at + 30min` 掉出候選）⇒ 尾段仍有約 6 次重試機會 ✅ |
| 閘門位置 | `reconcileSingle` **之前**。🔴 **不得放在 `finally`**——那是「事後記錄」不是閘門，跨 pod 的兩筆都已經做完副作用才寫 stamp |

- **`updateLastReconciledAt` → `claimForReconcile`**：加上 CAS 述詞，並把呼叫點從 `finally` 移到副作用之前。**`finally` 內的 stamp 一併移除**，否則會覆蓋 lease、把租約重置成更短。
- ⚠️ **這是冷卻租約不是「已對帳」旗標**：pod 在 claim 與 PayUni 查詢之間崩潰時，租約到期 row 自動回到候選集合。
- **行為退化（已知，可接受）**：退避語意從「**上次結束**起算 5 分鐘」變成「**上次開始**起算 5 分鐘」。單筆工作耗時遠小於 5 分鐘（含 100ms sleep 與一次 HTTP 往返），實務差異可忽略。
- 🚩 **stamp（＝租約基準）必須在 claim 當下逐筆取 `DateTimeUtils.nowUtc()`，不是 tick 起點的 `now`**（[規範 §三第 6 步](../開發規範/多副本排程搶佔規範.md)）。用 tick 起點時第 k 筆的**有效**租約 = 5 分 − (前 k−1 筆的耗時)；`batchSize` 預設 **50**、單筆最壞是一次 PayUni 同步查詢（`PayUniHttpClient` connect/write/read 各 30 秒 ⇒ **90 秒**）＋ 可能一次退款 ⇒ 批次上界 ≈ **75 分鐘 ≫ 5 分鐘租約**，**跑到第 4 筆之後租約就已過期**、另一個 pod 合法搶走同一筆 —— 而 `reconcileSingle` 內的 PayUni 查詢／rescue audit INSERT／管道 B 重複付款處理**三者皆無冪等鍵**（正是本節要防的東西）。`retryBefore`（CAS 述詞）維持 tick 起點：那個方向讓候選集合更小，是保守的。
- ⚠️ **rescue 分支的雙重保護不變**：`createRescueRefund` 的 `idempotencyKey = rescue:{tradeNo}` 與 `hasProcessingRefund` 檢查**照舊保留** —— claim 是第一道防線，冪等鍵是最終防線，兩者不互相取代。
- **`markRescued` 維持永久旗標，且那是正確的**：它與 outbox `INSERT` 包在**同一個 `txTemplate` 短 tx** 內 ⇒ 屬《[多副本排程搶佔規範](../開發規範/多副本排程搶佔規範.md)》**例外 A**（重複退款代價 > 遺失代價，而「不遺失」交給同 tx outbox）。**這是本 repo 例外 A 的標準正例，不要把它改成 lease。**

**過期邊界 rescue 流程（自動退款）**：

當 reconcile 在 grace 視窗內查到 PayUni 回 `TradeStatus=1`（已付款）但本地交易為 EXPIRED 時，代表「使用者實際付款成功，但訂單已被取消」。此時必須**自動發起退款**將款項退回給買家，避免使用者損失。

**為什麼不能直接走 EXPIRED → PAID → createRefund**：

| 路徑 | 問題 |
|------|------|
| 反向改 EXPIRED → PAID | 會重發 `payment.paid` 事件給 order-service，但訂單已被取消、庫存已釋放，下游處理會混亂 |
| 直接呼叫 `RefundApplicationService.createRefund` | `RefundDomainService.canRequestRefund` 只允許 PAID 狀態退款（`RefundDomainService.canRequestRefund`），會被擋下 |

**因此設計專用 rescue 路徑**（不污染既有付款/退款主路徑）：

1. 新增 `RefundApplicationService.createRescueRefund(tradeNo, amount, reason, idempotencyKey)` —— 與既有 `createRefund` 共用 90% 流程，差別僅在：
   - **跳過 `canRequestRefund` 狀態檢查**（因為呼叫時交易已是 EXPIRED）
   - **保留 `hasProcessingRefund` 檢查**（避免雙重退款）
   - **保留金額驗證、idempotencyKey 冪等檢查、PayUni client 呼叫、handleRefundSuccess/Failure 完整路徑**
   - 退款 reason 標記為 `RESCUE_RECONCILE_AFTER_EXPIRED`
   - idempotencyKey 採固定樣式 `rescue:{tradeNo}`，確保同筆交易最多觸發一次 rescue

2. **新增 PaymentApplicationService.handleReconcileConflict(tradeNo, payUniResponse)**：
   - 由 `PendingPaymentReconcileScheduler` 在偵測到衝突時呼叫
   - 寫入 `payment_logs` audit 紀錄（log_level=WARN，from_status=EXPIRED，event_data 含 PayUni 回應）
   - 呼叫 `refundApplicationService.createRescueRefund(...)` 全額退款
   - 發布 `PaymentRescueRefundInitiated` 事件至 Outbox（供 AfterSales / Notification 通知買家「訂單已取消但款項已退還」）
   - try/catch 包覆，**不論結果都要更新 `last_reconciled_at`**，避免每 tick 重複嘗試

3. **支付方式差異**：

| 支付方式 | rescue 行為 | 是否可全自動完成 |
|---------|------------|------------------|
| 信用卡 / Apple Pay / Google Pay | `createRescueRefund` 內部呼叫 PayUni `/api/trade/close`，**同步完成退款** | ✅ 是 |
| ATM 轉帳 | `createRescueRefund` 建立 REQUESTED 狀態的退款記錄，等待頁面版完成。**需人工介入**透過 AfterSales 取得頁面版 URL 並完成轉匯流程 | ❌ 否（記錄已建立、發布 alert，等人工） |

**冪等保證**：
- `createRescueRefund` 接受外部傳入的 idempotencyKey，使用固定樣式 `rescue:{tradeNo}`
- 同一筆交易在多次 reconcile tick 中反覆觸發 rescue 時，第二次起會命中 idempotency 早退（`refundRepository.findByIdempotencyKey`），不會重複退款
- 即使 scheduler 在退款進行中崩潰重啟，下次 tick 重撈到該筆 EXPIRED 交易並再次觸發 rescue，也會被 idempotency 攔下

**Audit / Alert**：
- `payment_logs` 寫入完整 PayUni 回應與 rescue 結果，留存追溯
- 發布 `PaymentRescueRefundInitiated` 事件，AfterSales / Notification 服務消費後通知買家
- ATM 衝突標 `rescue_action='8902'`（REFUND_PENDING_MANUAL）DB 狀態，由 sql_exporter 計數 + `PaymentRescueManualPending` 業務告警通知人工介入（**不發 MQ 事件**：沒有 consumer 的事件是孤兒，訊號會靜默消失）

**冪等保證**：
- syncPaymentStatus 早退保護（terminal status 不重複處理）
- 條件式更新使用 version 樂觀鎖
- 下游 order-service 對 `payment.paid` 事件已用 `tradeNo` 做 idempotent

**DB 變更**（🔴 `V1` 已凍結，一律以 `V{N+1}__{verb}_{target}.sql` append-only 落地，見 [DB Migration 相容性規範（expand-contract）](../開發規範/DB%20Migration%20相容性規範（expand-contract）.md)）：

```sql
-- payment_transactions 新增欄位
ALTER TABLE payment_transactions ADD COLUMN last_reconciled_at TIMESTAMPTZ NULL;
ALTER TABLE payment_transactions ADD COLUMN rescued_at         TIMESTAMPTZ NULL;
ALTER TABLE payment_transactions ADD COLUMN rescue_action      VARCHAR(30) NULL;

-- 部分索引：reconcile 掃描專用（涵蓋 INIT 與 grace 視窗內的 EXPIRED）
CREATE INDEX idx_payment_transactions_reconcile
    ON payment_transactions (created_at, last_reconciled_at)
    WHERE status IN ('0', '3');

COMMENT ON COLUMN payment_transactions.last_reconciled_at IS '最近一次 PendingPaymentReconcileScheduler 處理時間，用於控制重撈退避';
COMMENT ON COLUMN payment_transactions.rescued_at IS 'rescue 退款觸發時間，僅 EXPIRED→PayUni SUCCESS 衝突時設定';
COMMENT ON COLUMN payment_transactions.rescue_action IS 'rescue 動作結果：REFUNDED（信用卡同步退款完成）/ REFUND_PENDING_MANUAL（ATM 待人工頁面版完成）';
```

**配置**：

```yaml
hawkr:
  payment:
    reconcile:
      pending:
        enabled: true
        fixed-delay-ms: 300000
        batch-size: 50
        initial-delay-minutes: 3
        retry-interval-minutes: 5
        grace-minutes: 30
```

**觀測指標**（透過 Micrometer）：

| Metric | 類型 | 說明 |
|--------|------|------|
| `payment.reconcile.pending.scanned` | counter | 每次 tick 掃出筆數 |
| `payment.reconcile.pending.updated` | counter | 成功改為終態筆數 |
| `payment.reconcile.pending.failed` | counter | PayUni 查詢失敗筆數 |
| `payment.reconcile.pending.conflict` | counter | grace 視窗內查到 SUCCESS 但本地已 EXPIRED 的衝突筆數 |
| `payment.reconcile.rescue.success` | counter | rescue 自動退款成功（信用卡）筆數 |
| `payment.reconcile.rescue.manual_required` | counter | rescue 需人工介入（ATM）筆數 |
| `payment.reconcile.rescue.failed` | counter | rescue 退款失敗筆數 |

**告警**：連續 3 次 tick 失敗率 > 50% → ERROR 級 log + alert。

**明確排除**：
- ❌ 不對 PayUni 回應 `not found` 的 ghost 交易做自動處理
- ❌ **不涵蓋退款 reconcile**：ATM 退款無對等的查詢 API（理由見 §5.5.4.2），無權威來源可反查
- ❌ 不在 rescue 路徑內反向轉換交易狀態（EXPIRED 維持 EXPIRED，rescue 退款獨立記錄）

---

### 5.8 資料清理排程

**DataCleanupScheduler** 每日 02:30 執行，清理過期日誌與 Webhook 記錄。

**清理對象：**

| 對象表 | 條件 | 預設保留天數 | 組態 Key |
|--------|------|------------|----------|
| `payment_logs`（INFO/WARNING） | `level != 'ERROR'` | 180 天 | `payment.cleanup.log-retention-days` |
| `payment_logs`（ERROR） | `level = 'ERROR'` | 365 天 | `payment.cleanup.error-log-retention-days` |
| `payment_webhooks` | `is_processed = TRUE` | 90 天 | `payment.cleanup.webhook-retention-days` |
| `payuni_audit_logs`（金流爭議稽核專表，見 §11） | `created_at` 早於保留期 | **2555 天（7 年）** | `payment.cleanup.payuni-audit-retention-days` |
| `outbox_events`（**只有 SENT**） | `status = '8222'` **且** `sent_at` 早於保留期 | 7 天 | `payment.cleanup.outbox-sent-retention-days` |

> `payuni_audit_logs` 為 append-only 稽核專表，保留期 7 年（金流爭議追溯下限）。清理由本 `DataCleanupScheduler` 擴充一個清理項達成，不新增獨立 scheduler；保留天數可配置，預設 2555 天。

> **`outbox_events` 的 SENT 清理**：由本 `DataCleanupScheduler` 承擔（同一支 scheduler、同一個 cron、同一種 `@Value` 綁定、mapper 具名 `@Delete`），**不新增第二套 cleanup**。
> - **retention 起點是 `sent_at` 不是 `created_at`**：與 `payment_webhooks` 用 `processed_at` 同一條慣例。
>   `sent_at` 為 NULL 的列不會被刪（NULL 比較不成立）——方向 fail-safe，拿不準的列留著。
> - **述詞正面列舉** `status = '8222'`，不用 `status <> …`：否定述詞會讓日後新增的狀態值被一起刪掉，而且完全無聲。
> - 🔴 **`payment.cleanup.outbox-dead-retention-days`（30 天）刻意維持未接線**。DEAD（8224）是 permanent
>   delivery failure 的證據，也是 `payment_outbox_dead_rows` 告警的來源。讓 timer 自動刪它的語意是錯的：
>   **事故沒解決 → 保留期到 → 列消失 → 告警自己恢復綠**。未來若真要 DEAD retention，前提是先有明確的
>   operational resolution／acknowledgement／replay 語意（例如「已人工補發並註記」才可清），而不是單純時間到期。

**日誌策略：** 無事不印；有刪除才記錄總筆數；異常必記 ERROR

### 5.9 付款頁面導回流程

**Gateway Redirect 端點（信用卡 UPP 流程輔助）：**

信用卡支付採用 UPP 重導模式，Gateway 提供輔助端點供 APP WebView 開啟：

- **URL**: `GET /payment/redirect?id={redirectId}`
- **位置**: 頂層路由（非 `/api/v1/` 下），**無需 JWT**（WebView 開啟，無法攜帶 JWT）
- **用途**: 將 `CreatePayment` 產生的 canonical UPP 表單資料，經後端存 Redis + 取回後轉換為自動提交的 HTML 表單頁，POST 到 PayUni 支付頁
- **安全設計**：GET 端零 client 可控 render 資料——`redirect_id` 為後端產生之不可猜 opaque token（UUID），query 不再攜帶任何表單欄位（舊 `?data={base64}` round-trip 方式已物理移除，因其資料未經 escape 直接拼入 HTML，構成反射型 XSS + open form-action）
- **流程**:
  1. `CreatePayment` 成功後，Payment Service 將 canonical 表單資料（`action_url`/`mer_id`/`version`/`encrypt_info`/`hash_info`/`is_platform`）寫入 Redis（key `payment:redirect:{id}`，TTL 10 分鐘，對齊 MerTradeNo 窗），組裝 `redirectUrl`（格式：`{GATEWAY_URL}/payment/redirect?id={redirectId}`）
  2. 前端（APP WebView）開啟 `redirectUrl`
  3. Gateway 呼叫 gRPC `GetPaymentRedirectForm(redirect_id)` 一次性 consume 該 Redis record（取出即刪，防重放）→ 取得 canonical 表單資料 → 以 `html/template`（context-aware escape）渲染自動提交表單（含 `MerID`、`EncryptInfo`、`HashInfo`、`IsPlatForm` 等隱藏欄位）
  4. `action_url` 需通過 host allowlist 驗證（僅允許 PayUni 官方網域），驗證失敗回受控錯誤頁（非 500 leak）
  5. 瀏覽器自動 POST 到 PayUni `action_url`，進入金流付款頁
- **回應**: `text/html`（自動提交的 HTML 頁面）；`redirect_id` 不存在/已過期/已消費 → gRPC 回業務錯誤 → Gateway 渲染受控錯誤頁
- **注意**: 此端點需一次 gRPC 呼叫（`GetPaymentRedirectForm`）取回 canonical 表單，非純 Gateway 行為

**付款結果回調（ReturnURL → 302 redirect 最終出口）：**

PayUni 信用卡 UPP 付款完成後，以 **Form POST** 方式將使用者瀏覽器導回此 URL。
Gateway 提取 EncryptInfo + HashInfo → 呼叫 Payment Service **gRPC** 解密 → 從 `payment_transactions` 讀取 `client_type` / `app_type` → 依此分支組 `redirect_url` → **302 redirect** 導回對應端（行動 App 走 deep link / 網頁走 web URL）。

> **統一模式**：與超商門市地圖、ATM 退款頁面回調採相同架構（Gateway 薄代理 + 微服務 gRPC 處理 + 302 redirect）。

- **POST /payment/return**（PayUni UPP 導回）
  - **位置**: 頂層路由（非 `/api/v1/` 下），**無需 JWT**
  - **Gateway 行為**: 提取 Form POST 的 `EncryptInfo` + `HashInfo` → 呼叫 Payment gRPC `HandlePaymentPageReturn`（不傳 client_type / app_type，由 service 從 DB lookup）
  - **Payment Service 行為**: 驗簽 → 解密 EncryptInfo → 取得 MerTradeNo（= trade_no）→ lookup `payment_transactions` 取出 `client_type` / `app_type` → 記錄 payment_webhooks（稽核）→ 更新交易狀態（冪等）→ 發布 MQ 事件 → 依 `client_type` × `app_type` 組裝 `redirect_url`
  - **回應**: **302 redirect** → 依矩陣決定的最終出口 URL（見下方「最終出口分支矩陣」）

- **GET /payment/return?tradeNo={tradeNo}**（手動查詢，備用）
  - **Gateway 行為**: 透過 gRPC `GetPaymentReturnPage(tradeNo)` 查詢狀態，回傳 HTML
  - **回應**: `text/html`

**ReturnURL 組裝**：建立交易時，Payment Service 動態組裝 ReturnURL：
```
{PAYUNI_RETURN_URL}?tradeNo={tradeNo}
```
例如：`https://api.example.com/payment/return?tradeNo=TRD20260306123456ABCD`

> **注意**：PayUni UPP POST 導回時**不保留** URL query params；`tradeNo` 從加密的 EncryptInfo 中解密取得，`client_type` / `app_type` 從 `payment_transactions` lookup 取得（**SoT 在 DB**，不依賴 query round-trip）。

**Return 與 Webhook 的關係**：
- 兩者攜帶相同的交易結果資料，透過冪等鍵（`tradeNo:providerTradeNo`）確保不重複處理
- **誰先到誰處理**：Return POST 或 Webhook MQ 消費，先到的更新 DB 並發布 MQ 事件，後到的冪等跳過
- Return 處理成功後，即使 Webhook 未到，交易狀態也已更新，前台可立即看到付款結果

**狀態映射**：

| PayUni TradeStatus | 內部狀態 | 最終出口 status 參數 | 圖示 |
|---|---|---|---|
| 1 | PAID | `SUCCESS` | ✅ |
| 2 | FAILED | `FAILED` | ❌ |
| 0 | INIT | `PENDING` | ⏳ |

**最終出口分支矩陣**：

`redirect_url` 依 `client_type` × `app_type` 二維選擇：

| client_type | app_type | redirect_url 模板 |
|---|---|---|
| `MOBILE_APP` | `BUYER` | `hawkrbuyer://payment/result?tradeNo={tradeNo}&status={STATUS}` |
| `MOBILE_APP` | `SELLER` | `hawkrseller://payment/result?tradeNo={tradeNo}&status={STATUS}` |
| `WEB_BROWSER` | `BUYER` | `{WEB_FRONTEND_URL}/orders/{orderNo}?status={STATUS}&trade_no={tradeNo}&code={code}` |
| `WEB_BROWSER` | `SELLER` | （Phase 1 不支援；service 偵測到此組合 fallback 為 BUYER 路徑並寫入 `payment_logs` WARN 留痕）|

- `{orderNo}` 為原始訂單號（`payment_transactions.order_id`），不是 `trade_no`。
- `{code}` 為 PayUni 原始狀態碼或內部映射碼（用於前端細部錯誤文案；對外不可信，只當 hint）。
- `WEB_BROWSER` 路徑下不再渲染 Java HTML 中繼頁，改由 service 直接 302 → web 前端，由前端負責顯示結果（toast / modal）。
- `MOBILE_APP` 路徑維持既有 deep link 模式（WebView 攔截 URL Scheme）。

**Web 結果頁安全約定（強制）**：

`WEB_BROWSER` 路徑的 `status` / `code` query 參數**只是 UI hint**，使用者可在網址列任意竄改。前端**必須**在落地頁載入後立即以 `trade_no` 呼叫 `GET /api/v1/app/payments/{trade_no}` 取真實狀態，再決定要顯示成功 / 失敗 / 處理中文案。任何依賴 query `status` 直接渲染「付款成功」的實作均屬安全漏洞。

> **MOBILE_APP 路徑同樣建議二次驗證**，但因 deep link 多由 App 控制流，攻擊面較小；本文件不強制，由 App 端依自身需要決定。

**HTML 頁面行為**（僅 `MOBILE_APP` 路徑使用，做為 302 失敗時的 fallback）：
1. 顯示付款結果（圖示 + 標題 + 訂單編號 + 金額）
2. 「返回 APP」按鈕，href 指向 deep link
3. 10 秒倒數後自動跳轉 deep link（`window.location.href`）

**前端行為**（依 `status` 參數）：

| status | MOBILE_APP（App） | WEB_BROWSER（Web） |
|--------|---------|---------|
| `SUCCESS` | Toast「付款成功」→ 導回首頁 | 落地頁先呼 `GetPayment` 確認 → 顯示付款成功 modal |
| `FAILED` | Toast「付款失敗，請到我的訂單重新付款」→ 導向訂單頁 | 落地頁先呼 `GetPayment` 確認 → 顯示失敗 modal + 重試按鈕 |
| `PENDING` | Toast「付款處理中」→ 導向訂單頁 | 落地頁先呼 `GetPayment` 確認 → 顯示處理中 modal + 自動 polling |

**環境變數**：

| 名稱 | 範例值（test） | 說明 |
|---|---|---|
| `PAYUNI_RETURN_URL` | `https://test-api.example.com/payment/return` | PayUni Form POST 導回的 ReturnURL（既有）|
| `WEB_FRONTEND_URL` | `https://web-dev.example.com` | 網頁前端 base URL（**新增**）；當 `client_type=WEB_BROWSER` 時，service 用此值組 redirect_url |

> 新增 `WEB_FRONTEND_URL` 必須依「環境變數同步流程」5 處同步：`.env*`、`docker-compose-local.yml`、`k8s/values.yaml`、SSH 遠端、實機驗證。

### 5.10 事件發布機制

本服務事件依可靠性需求分兩類發布：

| 事件類 | 發布機制 | 對象 |
|---|---|---|
| **付款結果事件** | **Transactional Outbox + afterCommit 即時投遞 + 排程器兜底**（`PaymentOutboxWriter.enqueueAndDispatch`） | `payment.success` / `payment.failed` / `payment.conflict`——由**六個發布點**經**兩收斂點**統一產生（收斂點 A=§5.2 `PaymentNotifyHandler`；收斂點 B=`PaymentApplicationService.publishEvent`，見 §5.10.1）、rescue refund（`payment.rescue.refund_initiated` / `refund_completed`，見 §2A.6） |
| **排程觸發的永久狀態轉換事件** | **同上那條 Transactional Outbox**（規範例外 A） | `payment.expired`（`ExpiredTransactionScheduler`）——與 `updateToExpired` **同事務** `enqueueAndDispatch`。🔴 不得改成事務內直接發布：該 CAS 是永久狀態轉換，而 publish 的例外被同方法內的 catch 吞掉，`@Transactional` 對它形同虛設（見 §5.7） |
| **其餘業務事件** | `PaymentEventPublisher` 事務內直接發布（失敗 rollback） | `payment.closed`（`PaymentCloseService`）、`refund.succeeded` / `refund.failed`（`RefundApplicationService`）、`PendingPaymentReconcileService` 補救事件——**本體 `PaymentEventPublisher.publish` 不改，零波及** |

> Event Type → Routing Key 對應、payload class、Exchange 配置見 [36-MQ 與事件總覽 §3.2 支付領域](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md#32-支付領域payment-domain)。

#### 5.10.1 付款結果事件原子化 + 即時投遞

**問題**：修復前，付款結果事件（`payment.success` / `payment.failed` / `payment.conflict`）散落於**六個發布點**，全部是「狀態已在事務內落地、但事件於事務外/後直接 `PaymentEventPublisher.publish` 發送」的**雙寫非原子**：

| # | 發布點 | 事務載體 | 舊發送方式 |
|---|---|---|---|
| 1a | `PayUniWebhookConsumer`（webhook 消費） | `transactionTemplate.execute` | `handle` 回傳 mqEvent → 消費端事務外直發 |
| 1b | `PaymentReturnController`（return URL 回導） | `transactionTemplate.execute` | 同上 → `publishMqEvent`（吞例外）|
| 1c | `PaymentGrpcService.HandlePaymentPageReturn`（頁面導回） | `transactionTemplate.execute` | 同上 → `publishReturnMqEvent`（吞例外）|
| 2 | `handleZeroAmount`（零元付款） | `@Transactional createPayment` | `publishEvent` → 事務內直發 |
| 3 | `updateTransactionStatus`（poll / reconcile 同步 PAID/FAILED） | `@Transactional syncPaymentStatus` | 同上 |
| 4 | `processLocalDateTimePaymentResponse`（Apple / Google Pay 同步 PAID/FAILED） | `@Transactional createPayment` | 同上 |

任一發布點只要「事務已 commit（交易 = PAID）但發送前程序崩潰 / MQ 中斷 / §10.8 環境共用 RabbitMQ 被搶訊息」，或 in-tx 直發者「事務 rollback 但事件已流出（over-fulfillment）」，都會造成訂單狀態與 MQ 事件不一致 → 訂單停在 `PENDING_PAYMENT`（under-fulfillment）或訂單被假成功推進（over-fulfillment）。

**修復（transactional outbox with immediate dispatch）**：六個發布點透過**兩個天然收斂點**統一改走 `PaymentOutboxWriter.enqueueAndDispatch(event)`：

| 收斂點 | 涵蓋發布點 | 接入方式 |
|---|---|---|
| **A**：`PaymentNotifyHandler.handle`（`handlePaid` / `handleFailed` / `handleConflict`）| #1a / #1b / #1c 三 notify 入口 | 三入口共用 `handle` 產生事件 → 改一處三入口受益；三入口移除事務外直發與 `eventPublisher` 欄位 |
| **B**：`PaymentApplicationService.publishEvent`（私有 helper）| #2 / #3 / #4 三 in-tx producer | 三方法都經此 helper → 只改 helper body（`eventPublisher.publish` → `outboxWriter.enqueueAndDispatch`），一處改三點受益 |

**兩收斂點涵蓋的付款結果事件（success / failed / conflict）全部統一走 writer，不區分事件類型、無特例**。機制：

1. **事務內** `INSERT outbox_events(status=PENDING)` — 與 `updateToXxx` 狀態落地、`markAsProcessed`（收斂點 A）或狀態更新（收斂點 B）**原子**：rollback 三者皆不落地、commit 一併落地（根治 under+over 兩面 dual-write）。
2. **afterCommit 即時投遞**（`TransactionSynchronizationManager.registerSynchronization` 的 `afterCommit` callback）：事務成功 commit 後**立即**在原執行緒呼叫 `PaymentOutboxPublisher.publish`（內含 `publishConfirmed`，見 §5.10.2）→ 正常路徑**即時履約**、下游立刻收到，**不必等 5s 輪詢**。
3. **投遞結果標記**：broker ack → `markSENT`（排程器不再撿）；nack / timeout / unroutable → `markFailed`（指數退避）；afterCommit 若因例外未能標記 → 留 `PENDING`。
4. **排程器兜底**：`OutboxEventPublisherScheduler` 每 5s 掃 `PENDING` / 到期 `FAILED` 重投（涵蓋 afterCommit 送失敗、或 afterCommit 送成功但 `markSENT` 前崩潰的視窗）。
5. **消費端冪等**：下游 order-service 以 `trade_no`（`transaction_id`）單欄 `ON CONFLICT DO NOTHING` dedup → afterCommit 與排程器兜底可能重送同一 `trade_no`（at-least-once），消費端吸收（見 §6.1）。

**原子化 + 即時投遞前後對照**：

```mermaid
flowchart TB
  subgraph BEFORE["修復前：雙寫非原子（六發布點皆有遺失/假成功窗口）"]
    direction TB
    B1["TX 內：updateToXxx（status 落地）<br/>（notify 另 + markAsProcessed）"] --> B2["TX commit ✅"]
    B2 --> B3{{"⚠️ 不一致窗口<br/>commit 後、直發前"}}
    B3 -->|正常| B4["TX 外/後：PaymentEventPublisher.publish<br/>payment.success/failed/conflict"]
    B3 -->|程序崩潰 / MQ 中斷 / 訊息被搶| B5["事件永久遺失（under-fulfillment）<br/>訂單停在 PENDING_PAYMENT"]
    B6["in-tx 直發者：TX rollback<br/>但事件已流出"] --> B7["假成功（over-fulfillment）"]
  end

  subgraph AFTER["修復後：outbox 原子 + afterCommit 即時 + 排程器兜底"]
    direction TB
    A1["TX 內（原子）：<br/>updateToXxx（status 落地）<br/>+ PaymentOutboxWriter.enqueueAndDispatch<br/>　→ INSERT outbox_events(PENDING)<br/>+ registerSynchronization(afterCommit)<br/>（notify 另 + markAsProcessed）"] --> A2["TX commit ✅<br/>（狀態與事件同生共死）"]
    A2 --> A3["afterCommit 即時：同執行緒呼叫<br/>PaymentOutboxPublisher.publishConfirmed<br/>（正常路徑即時履約，不等輪詢）"]
    A3 -->|ack| A4["markSENT（排程器不再撿）"]
    A3 -->|nack / timeout / unroutable| A5["markFailed + 指數退避"]
    A3 -.->|afterCommit 例外| A6["吞例外不 rethrow<br/>留 PENDING"]
    A5 --> A7["OutboxEventPublisherScheduler 每 5s<br/>掃 PENDING / 到期 FAILED 兜底重投"]
    A6 --> A7
    A7 -->|重投 ack| A4
  end
```

> **afterCommit callback 必吞例外、不得 rethrow**：afterCommit 在事務 commit **之後**於原請求執行緒同步執行，若拋例外會上拋至 commit 呼叫方（`createPayment` / `syncPaymentStatus` 的 gRPC caller），造成**付款已成功卻對前端回錯**。故 dispatch 整段 `try/catch` 吞掉——confirm 失敗只 `markFailed` 交排程器兜底，付款 API 仍正常返回（DB / outbox 已安全落地，confirm 失敗不回滾已 commit 的事務）。此為 hot path，[《開發工作流程》](../開發規範/開發工作流程.md) §6 的結帳 smoke 必含。

> **與 §2A.6.5 對齊**：`PaymentOutboxWriter.enqueueAndDispatch` 內的 payload 序列化若失敗（`JsonProcessingException`）必須 **throw** 讓事務 rollback，**禁止** catch + return（會讓事務正常 commit 造成 UPDATE 落地但 outbox 漏寫 = 原子化破口重演）。payload JSON 於 INSERT 時凍結一次 → afterCommit 送與排程器兜底送**同一份 bytes**（含同 `event_id` / `occurred_at`），at-least-once 重送完全一致。envelope 組裝（含 `event_id` UUID v4 注入，對應 36 號 §F3）集中於 `PaymentOutboxWriter`——**統一補齊收斂點 A 原缺的 `event_id`**（notify 路徑舊 payload 無 `event_id`），取代原 `PaymentEventPublisher` / rescue / duplicate 三份重複邏輯；下游只讀 `trade_no` / `order_id` 等既有欄位且 dedup on `trade_no`，補 `event_id` 非破壞。

> **邊界（不波及非 success 事件）**：`PaymentEventPublisher.publish` **本體不改**，仍被 4 個非付款結果 caller 使用——`ExpiredTransactionScheduler`（expired）、`PaymentCloseService`（closed）、`RefundApplicationService`（refund）、`PendingPaymentReconcileService`（rescue），**零波及**。改動只落在收斂點 A（`PaymentNotifyHandler`）與收斂點 B（私有 `publishEvent`，封閉於 createPayment / syncPaymentStatus 流），對外零外溢。

> **依賴 L2（同波交付）**：afterCommit dispatch 呼叫的 `PaymentOutboxPublisher.publish` 內含 `publishConfirmed`（§5.10.2）→ L1' 落地需 L2（`ObsRabbitPublisher.publishConfirmed` + `publisher-confirm-type=correlated` 設定）同波交付，缺 L2 則 afterCommit 無法確認投遞。

#### 5.10.2 Outbox 投遞確認（L2，publisher-confirm）

`outbox_events` 的投遞由**兩條路徑**共用同一 `PaymentOutboxPublisher.publish`：(a) L1' 的 **afterCommit 即時投遞**（正常路徑，事務 commit 後同執行緒立即送）、(b) `OutboxEventPublisherScheduler`（每 5s 掃 PENDING / 到期 FAILED）**兜底**。兩者皆改用 **publisher-confirm** 確認 broker 已收下才標記 SENT：

- `ObsRabbitPublisher` 新增 `boolean publishConfirmed(exchange, routingKey, byte[], event, timeoutMs)`：以 `CorrelationData` + `convertAndSend` 注入同組 obs headers 後，`future.get(timeout)` 等待 broker confirm；`!isAck` / timeout / `getReturned != null`（unroutable）→ 回 `false`。既有 `publish(...)` 方法不變，全數既有 caller 無感。
- `PaymentOutboxPublisher`：`publishConfirmed` 回 `true` 才 `markSENT`，否則 `markFailed` 走指數退避重投（5s→15s→45s→135s→405s，超 maxRetries 標 DEAD 告警）。
- 全域設定 `spring.rabbitmq.publisher-confirm-type=correlated` + `publisher-returns=true` + `template.mandatory=true`（置於 `observability-defaults.properties`，correlated 為非破壞性，既有直發 caller 不 block、語意不變）。

#### 5.10.2b Outbox 多副本搶佔

**問題**：`outbox_events` 有**兩條進入點** —— (a) `PaymentOutboxWriter` 的 afterCommit 即時投遞、(b) `OutboxEventPublisherScheduler` 每 5 秒兜底掃描。payment 在 prod 是 `replicas: 2` ⇒ 兩個 pod 的排程各撈同一批 `PENDING` row ⇒ **同一則付款結果事件會被 publish 兩次**（下游雖有 `trade_no` dedup，但那是最終防線不是設計意圖，且會放大 MQ 流量與 dedup 索引寫入）。

**搶佔閘門的位置：放在 `PaymentOutboxPublisher.publish()` 的第一行，不放在 Scheduler。**
理由：兩條進入點都會走到 `publish()`，閘門放 Scheduler 只擋得住其中一條。

| 項目 | 內容 |
|---|---|
| 租約欄 | **既有的 `outbox_events.next_retry_at`**，零 migration |
| claim SQL | `UPDATE outbox_events SET next_retry_at = #{now} + 60s WHERE id=#{id} AND status IN ('8221','8223') AND (next_retry_at IS NULL OR next_retry_at <= #{now})`（**與既有候選述詞 `OutboxEventMapper.findPendingEvents` 逐字一致**）|
| 租約長度 | **60 秒**（遠大於單筆最壞耗時 = 一次 publisher-confirm 往返，`confirm-timeout-ms` 秒級）|
| 候選窗口寬度 | **∞**（候選述詞無時間上界）⇒ 任何租約合格 |
| 終態標記 | `markSent` 加守衛 `AND status <> '8222'`；`markFailed` / `markDead` 一律改具名 `@Update` ＋ 明確守衛述詞，**不再用 `updateById` 全欄位覆蓋**（避免與 claim 的寫入互相覆蓋）|

> 🚨 **`outbox_events.status` 存的是 `OutboxEventStatus` 的 CodeMeta value 不是 enum 名**：`'8221'`＝PENDING、`'8222'`＝SENT、`'8223'`＝FAILED、`'8224'`＝DEAD（SSOT＝[代碼管理（CodeMeta）／內部技術碼](../代碼管理（CodeMeta）/內部技術碼.md)）。claim 述詞若寫成 `status IN ('PENDING',…)` 會**恆 `affected=0` ⇒ 付款結果事件全部不再投遞**（沒有錯誤、只是每筆都被當成「別的 pod 搶走了」而 skip）。另：`OutboxEventStatus` **沒有 `RETRYING`**，重試中的狀態就是 `FAILED('8223')`。

- ⚠️ **這是冷卻租約不是「已派送」旗標**：pod 在 claim 與 `publishConfirmed` 之間崩潰時，租約到期 row 自動回到候選集合、下一次掃描重送 ⇒ **at-least-once 契約不變**。
- **行為退化（已知，可接受）**：崩潰情境下事件會**延後一個租約（60 秒）**才重送，現況是「下一 tick（5 秒）就重來」。換來的是不重複。
- ⚠️ **afterCommit 即時投遞路徑也會經過 claim**：正常路徑下該 row 剛 `INSERT`、`next_retry_at IS NULL` ⇒ claim 必成功，**即時履約不受影響**。
- ⚠️ **候選述詞禁止用 `status <> '8222'`**：那會讓 `DEAD`（`'8224'`）row 永久滯留、批次上限被佔滿 ⇒ 整條 outbox starvation（**單 pod 也會壞**）。一律**正面枚舉**要處理的狀態（本服務現況 `findPendingEvents` 已是 `IN ('8221','8223')`，修改時不要順手「簡化」掉它；[規範 §六](../開發規範/多副本排程搶佔規範.md)）。

**事務邊界對照（更新）**

| 動作 | 在事務內？ | 說明 |
|---|---|---|
| `updateToXxx` 狀態落地 ＋ `INSERT outbox_events(PENDING)` | ✅ 同一事務 | 原子（§5.10.1） |
| **claim（lease CAS on `next_retry_at`）** | ❌ **事務外，自動提交** | 它必須跨 pod 立即可見；包進事務反而讓其他 pod 看不到 |
| `publishConfirmed`（MQ I/O） | ❌ 事務外 | 網路 I/O 不進 DB 交易（§1.6） |
| `markSent` / `markFailed` / `markDead` | ❌ 事務外，各自具名 UPDATE ＋ 守衛述詞 | — |

> **為什麼不用 `FOR UPDATE SKIP LOCKED`**：要讓它有搶佔效果就得把 `publishConfirmed` 的網路 I/O 包進 DB 交易並持有 row lock 直到 broker confirm 回來——直接違反上表第 3 列。**一律用 CAS。**

#### 5.10.2c 一致性、交易與補償邊界

本服務有三處多副本搶佔相關的設計 —— outbox dispatcher lease（本節）、`PendingPaymentReconcileScheduler` 的 lease on `last_reconciled_at`（[§5.7.1](#571-待付款交易補收排程pendingpaymentreconcilescheduler)）、`payment.expired` 走 outbox（[§5.7](#57-逾期處理排程)）。三者共同的邊界結論：

- **不引入補償任務基礎設施**：三處都是「把既有寫入路徑寫對」（claim 述詞、候選述詞、事件寫入位置），**沒有新增任何 reconciliation job / drift 偵測 / backfill runner / reflow 表**，符合 [後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)。B4 使用的 `outbox_events` 是**既有**表，屬「保 MQ 必發」的 Outbox，正是該規範明列的例外。
- **單一寫入路徑為唯一真實來源**：`payment_transactions` 的狀態推進仍只有原本那一條路徑；lease 只是把「誰現在有權跑」表達成時間，不新增第二條寫入路徑。B4 也沒有新增第二個 `payment.expired` 的產生點——只是把同一個事件的投遞載體換成 outbox。
- **零 migration、零新欄位**：H2／H6 用既有時間欄，B4 用既有 `outbox_events` 表。
- **行為退化（已知，可接受）**：pod 在 claim 與副作用之間崩潰時，該筆**延後一個租約**才重試（現況「下一 tick 就重來」）。**at-least-once 契約不變。**
- **租約不是鎖**：租約到期後原持有者若仍活著，可能與新持有者同時處理同一筆 → 回到「重複一次」。下游冪等（`payment_event_dedup` 的 `trade_no` 單欄、`refund_no`）仍是最終防線。

#### 5.10.3 元件職責

**六發布點 + 兩收斂點 + 統一 writer**：

| 元件 | 角色 | 職責 |
|---|---|---|
| `PaymentOutboxWriter` | **統一 writer** | `enqueueAndDispatch(event)`：組 envelope（補 `event_id`）→ `new OutboxEventEntity(PENDING)` → `outboxMapper.insert`（序列化失敗 throw）→ `registerSynchronization(afterCommit)` 內 `try{ PaymentOutboxPublisher.publish } catch(全部){ log 不 rethrow }` |
| `PaymentNotifyHandler` | **收斂點 A** | `handlePaid` / `handleFailed` / `handleConflict` 在**事務內**呼叫 `PaymentOutboxWriter.enqueueAndDispatch`，回 `WebhookHandleResult.success(message)`。🔴 `WebhookHandleResult` **不得攜帶 MQ event**——帶了就會回到「事務外直發」 |
| `PaymentApplicationService` | **收斂點 B** | 私有 `publishEvent(event)` 一律走 `outboxWriter.enqueueAndDispatch`；`handleZeroAmount` / `updateTransactionStatus` / `processLocalDateTimePaymentResponse` 三個發布點都經由它 |
| `WebhookHandleResult` | 收斂點 A 配套 | 只承載處理結果訊息；**無** `mqEvent` 欄位、無 `MqEventData` |
| `PayUniWebhookConsumer` / `PaymentReturnController` / `PaymentGrpcService` | 三 notify 入口 | 🔴 **不得在事務外直發付款結果事件**（不持有 `eventPublisher`、不實作 `publishMqEvent` 之類的旁路） |
| `PaymentEventPublisher` | 非付款結果事件 | 承載 `payment.closed` / `refund.*` / rescue 的事務內直發。⚠️ **`payment.expired` 走 outbox，不是本 publisher 的 caller**（見 §5.7） |
| `PaymentOutboxPublisher` | 投遞 | `publish` 走 `ObsRabbitPublisher.publishConfirmed`，**broker ack 後才 markSENT**（afterCommit 與排程器共用同一支） |
| `ObsRabbitPublisher`（observability-common） | L2 | 提供 `publishConfirmed(...)` |
| `observability-defaults.properties` | L2 | `publisher-confirm-type=correlated` 等 publisher-confirm 組態 |

> rescue refund 走 Outbox Pattern 的完整設計（`payment.rescue.refund_initiated` / `refund_completed`）見 §2A.6；本節的 publisher-confirm 升級同時涵蓋 rescue / duplicate 既有 outbox 投遞（payload 不變、下游無感），二者維持排程器 5s 投遞、不強制加 afterCommit（非同步關鍵路徑）。

### 5.11 Apple Pay Web Merchant Validation 流程

> 適用情境：Web 端（Safari `ApplePaySession` + Chrome W3C `PaymentRequest`）發起 Apple Pay 付款前的前置流程。行動 App 走原生 Apple Pay 不在此範圍。

#### 5.11.1 責任分工

Apple Pay on Web 規範要求商家**後端**持有 Apple Pay Merchant Identity Certificate，並在前端 `onvalidatemerchant` / `onmerchantvalidation` callback 中由後端代為向 Apple 取得 `merchantSession`。🔴 **前端不得直接呼叫 Apple `validationURL`**（CORS ＋ mTLS 皆不允許）；缺少這道後端代理，前端的 `requestApplePayToken()` 會在第一步就 throw。

因此 payment-service 提供 `ValidateApplePayMerchant` RPC，由 Gateway 暴露 `POST /api/v1/app/payments/apple_pay/validate_merchant`：後端以 Merchant Identity Certificate 做 mTLS POST 取得 `merchantSession`，**原樣透傳**給前端（不解析、不快取）。

付款本身仍走 PayUni `/api/apple` 幕後 API（`PaymentApplicationService.handleApplePayPayment`），OnWeb/InApp mode 切換見 §2A.8。

#### 5.11.2 完整時序圖

```mermaid
sequenceDiagram
participant FE as Web 前端
participant Apple as Apple Pay sheet / Apple gateway
participant GW as Gateway
participant PS as PaymentService
participant Cert as PEM + Key (本地檔 / k8s secret)

Note over FE: 使用者選 Apple Pay 並提交結帳
FE->>FE: ApplePaySession.begin() / PaymentRequest.show()
Apple-->>FE: onvalidatemerchant / onmerchantvalidation<br/>event.validationURL
FE->>GW: POST /app/payments/apple_pay/validate_merchant<br/>{validationURL, domainName, displayName}
GW->>PS: gRPC ValidateApplePayMerchant
PS->>PS: 1) SSRF 防護：scheme=https + host *.apple.com + path /paymentservices/*
PS->>PS: 2) domainName 必須在 APPLE_PAY_DOMAIN_ALLOWLIST 內
PS->>Cert: 3) 載入 PEM + Key（首次載入做 LRU 快取）
PS->>Apple: 4) HTTPS POST validationURL (mTLS)<br/>{merchantIdentifier, displayName, initiative:"web", initiativeContext:domainName}
Apple-->>PS: 200 OK<br/>merchantSession JSON
PS-->>GW: merchant_session_json
GW-->>FE: {data: {merchantSessionJson}}
FE->>FE: session.completeMerchantValidation(JSON.parse(merchantSessionJson))
Apple-->>FE: 顯示 Apple Pay sheet → 使用者授權 → ApplePay paymentToken
FE->>GW: POST /app/payments<br/>{orderNo, paymentType=APPLE_PAY, applePayToken}
GW->>PS: gRPC CreatePayment
PS->>PS: handleApplePayPayment（§2A.8 mode 切換）
Note over PS,GW: 後續走 §7.4 同步付款流程
```

#### 5.11.3 業務流程圖（決策樹）

```mermaid
flowchart TD
  A[gRPC ValidateApplePayMerchant] --> B{validation_url 格式檢查}
  B -->|scheme≠https / host 不在 *.apple.com / path 不在 /paymentservices/*| R1[APPLE_PAY_VALIDATION_URL_REJECTED]
  B -->|通過| C{domain_name 在 allowlist?}
  C -->|否| R2[APPLE_PAY_VALIDATION_URL_REJECTED]
  C -->|是| D{PEM + Key 載入}
  D -->|檔不存在 / 格式錯誤 / 解析失敗| R3[APPLE_PAY_CERT_LOAD_FAILED]
  D -->|成功| E[mTLS HttpClient POST Apple validationURL]
  E -->|非 2xx / IOException / SSLException| R4[APPLE_PAY_MERCHANT_VALIDATION_FAILED]
  E -->|2xx + body| F[原樣回傳 merchant_session_json]
```

#### 5.11.4 SSRF 防護規格

`validation_url` 必須**同時**通過以下三項檢查，否則回 `APPLE_PAY_VALIDATION_URL_REJECTED`：

| 檢查項 | 規則 | 範例（允許 / 拒絕） |
|---|---|---|
| Scheme | 必須 `https` | ✅ `https://...` / ❌ `http://...`、`file://` |
| Host | 必須以 `.apple.com` 結尾（含 `apple-pay-gateway.apple.com` / `apple-pay-gateway-cert.apple.com` / `apple-pay-gateway-nc-pod1.apple.com` 等所有 Apple subdomain） | ✅ `apple-pay-gateway.apple.com` / ❌ `evil.com`、`apple.com.evil.com` |
| Path | 必須開頭 `/paymentservices/` | ✅ `/paymentservices/startSession` / ❌ `/admin/`、`/../etc/passwd` |

**`domain_name` 額外檢查**：必須在 `APPLE_PAY_DOMAIN_ALLOWLIST` 環境變數（逗號分隔）內，否則同樣回 `APPLE_PAY_VALIDATION_URL_REJECTED`。

> **設計理由**：Apple Pay sheet 啟動瞬間，`validationURL` 是 Apple 提供的、前端無法控制的值，理論上一定符合上述規格。但 endpoint 暴露在公網，惡意呼叫者可能送任意 URL 試圖把後端當 SSRF 跳板。三層白名單拒絕一切非預期 URL。

#### 5.11.5 mTLS 與 PEM 載入機制

**Apple POST body**（依 Apple 官方規格）：
```json
{
  "merchantIdentifier": "merchant.io.github.will413028.hawkr.sandbox",
  "displayName": "Hawkr",
  "initiative": "web",
  "initiativeContext": "web-dev.example.com"
}
```

**HttpClient 構建**：
- Java 17 `HttpClient` + `SSLContext`
- 從 `APPLE_PAY_MERCHANT_CERT_PATH` 讀 X.509 cert、`APPLE_PAY_MERCHANT_KEY_PATH` 讀 **PKCS#8** private key → 組裝 in-memory PKCS12 `KeyStore` → `KeyManagerFactory` → `SSLContext`
- 載入失敗（檔不存在 / PEM 格式錯誤 / cert 與 key 不配對）回 `APPLE_PAY_CERT_LOAD_FAILED`
- **首次載入後快取 SSLContext（process 內 singleton）**，避免每次呼叫重新解析 PEM。憑證過期由 Apple 端 5xx 觸發，container 重啟即重新載入

**Private Key 格式限制（PKCS#8-only）**：

僅支援 PEM 格式的 PKCS#8 private key（檔頭 `-----BEGIN PRIVATE KEY-----`），透過 JDK 原生 `KeyFactory.getInstance("RSA")` + `PKCS8EncodedKeySpec` 解析，**不引入** BouncyCastle 等外部 dep。

若提供的是 PKCS#1（檔頭 `-----BEGIN RSA PRIVATE KEY-----`，openssl 預設輸出格式之一），啟動前由 `readPrivateKey()` 先做格式偵測、直接拋 `IllegalStateException` 含明確指引：

```
openssl pkcs8 -topk8 -nocrypt -in <pkcs1.key> -out <pkcs8.key>
```

由 `loadSslContext()` outer catch 包成 `APPLE_PAY_CERT_LOAD_FAILED` (8029) 回前端，**並在 log 帶具體錯誤類型**（`IllegalStateException` / `InvalidKeySpecException` / `KeyStoreException` / ...）供 ops 排查。

PayUni 給的 dev key 是 PKCS#8（`BEGIN PRIVATE KEY`，無需轉換）；prod cert 申請流程同樣建議要求 PKCS#8。

**Timeout**：connect 5s、request 10s。Apple 端罕見超過 3s。

**重試**：**不重試**。Apple 失敗多數為 PEM 過期 / domain 未 verify 等不可重試錯誤；前端會在使用者再次點按 Apple Pay 時重新發起。

**Response 處理**：HTTP 200 + body 為 JSON → 原樣放入 `merchant_session_json` 回傳。**不解析、不轉換、不截斷**（merchant session 內含 Apple 簽章，任何修改都會讓前端 `completeMerchantValidation()` 拒絕）。

#### 5.11.6 環境變數

| 變數 | dev / sandbox 範例 | prod 範例 | 說明 |
|---|---|---|---|
| `APPLE_PAY_MERCHANT_ID` | `merchant.io.github.will413028.hawkr.sandbox` | `merchant.io.github.will413028.hawkr` | Apple Developer 後台註冊的 Merchant ID；POST body 的 `merchantIdentifier` 欄位 |
| `APPLE_PAY_DISPLAY_NAME` | `Hawkr` | `Hawkr` | Apple Pay sheet 顯示的商家名稱；POST body 的 `displayName` 欄位 |
| `APPLE_PAY_MERCHANT_CERT_PATH` | `/secrets/apple-pay/merchant_id.pem` | `/secrets/apple-pay/merchant_id.pem` | Apple 簽發的 X.509 Merchant Identity Certificate（PEM `BEGIN CERTIFICATE`）。container 內 path 兩環境相同（靠 k8s Secret 內容差異） |
| `APPLE_PAY_MERCHANT_KEY_PATH` | `/secrets/apple-pay/merchant_id.key` | `/secrets/apple-pay/merchant_id.key` | 對應 Merchant Identity 的 **PKCS#8** Private Key（PEM `BEGIN PRIVATE KEY`） |
| `APPLE_PAY_DOMAIN_ALLOWLIST` | `web-dev.example.com` | `example.com` | 允許發起付款的網域（逗號分隔）；驗證 `domain_name` 入參 |

> **憑證輪替與環境切換**（含 domain verify、k8s Secret 建立、helm 套用、驗證）的完整步驟見 [`docs/速查參考/Apple Pay Merchant Cert 輪替 SOP.md`](../速查參考/Apple%20Pay%20Merchant%20Cert%20輪替%20SOP.md)

#### 5.11.7 錯誤碼與前端文案分工

後端僅回 code（見 §4.6 末段表），訊息文案由前端決定：

| Code | 建議前端文案 |
|---|---|
| `APPLE_PAY_VALIDATION_URL_REJECTED` | 「Apple Pay 服務暫時無法使用，請改其他付款方式」（內部錯誤、不暴露白名單細節） |
| `APPLE_PAY_CERT_LOAD_FAILED` | 「系統忙碌中，請稍後再試或改其他付款方式」（憑證問題，使用者無法自助處理） |
| `APPLE_PAY_MERCHANT_VALIDATION_FAILED` | 「Apple Pay 驗證失敗，請重試或改其他付款方式」（多為 Apple 端瞬時錯誤） |

#### 5.11.8 Web 端依賴（Domain Association File）

Apple Developer 後台註冊 Merchant Domain 後，要求網域於 `/.well-known/apple-developer-merchantid-domain-association` 提供 Apple 簽發的對應 token（純文字 base64，約 5KB）。

| 項目 | 位置 | 負責 |
|---|---|---|
| 動態 serve | hawkr-website `app/api/apple-pay/domain-association/route.ts` 讀環境變數 `APPLE_PAY_DOMAIN_ASSOCIATION`（不在本 repo） | web 團隊 |
| Rewrite | hawkr-website `next.config.ts` 內 `.well-known/apple-developer-merchantid-domain-association` → `/api/apple-pay/domain-association`（不在本 repo） | web 團隊 |
| Token 更新 | Apple Developer 後台「Re-verify」→ 下載新 token → 更新 web 環境變數 → 重啟容器 → Apple 後台點 Verify | 後端 + Apple Developer 帳號持有者 |

> hawkr-website 為獨立 repo，不在 backend-payuni 範圍。**部署於 Cloudflare Workers（OpenNext），dev/prod 皆非 VM 容器**；`APPLE_PAY_DOMAIN_ASSOCIATION` 改值後須由該 repo `deploy.yml` 的 `deploy-cloudflare` job 重新部署才生效，`.well-known` URL 才會回 200。後端設計依賴 domain 已 verify 才能成功取得 `merchantSession`。

#### 5.11.10 範圍邊界

| 項目 | 歸屬 |
|---|---|
| Google Pay merchant validation | **不適用**：PayUni `/api/google` 無此前置流程 |
| 行動 App 原生 Apple Pay 流程 | 走系統 PassKit，**不需後端代理**，沿用 §2A.8 的 InApp mode |
| Domain Association file host | hawkr-website 負責（§5.11.8） |

### 5.12 GetActivePaymentStatusByOrderNo（逾期取消前反查，internal-only）

> L3 縱深防護。供 Order Service `PaymentDeadlineScheduler` 在逾期**取消訂單前**同步反查該訂單付款狀態，避免「付款已成功但事件尚未送達」時誤取消訂單造成資損。

#### 5.12.1 用途與呼叫端

- **呼叫端**：Order Service（`PaymentDeadlineScheduler` → `PaymentGrpcClient.getActivePaymentStatusByOrderNo`）。
- **權限**：**internal-only**——僅接受 internal call（`GrpcAuthContext.isInternalCall()==true`，§5.2 內部服務免權限）。排程器無使用者情境，既有 `GetPayment` / `SyncPaymentStatus` 以 `trade_no` 為鍵且需 BUYER/SELLER 授權會被擋，故新增 by-orderNo 的 internal RPC。
- **不暴露 Gateway**：不加任何 `/app` / `/admin` 路由（無孤兒 RPC 疑慮，屬服務間內部查詢）。

#### 5.12.2 API 定義

| 項 | 內容 |
|---|---|
| RPC | `GetActivePaymentStatusByOrderNo(GetActivePaymentStatusByOrderNoRequest) returns (GetActivePaymentStatusByOrderNoResponse)` |
| Request | `string order_no = 1`（訂單編號） |
| Response | `int32 code = 1` / `string message = 2` / `PaymentDetail data = 3`（復用既有 `PaymentDetail`，見 §4.2） |
| 語意 | 以 `order_no` 查該訂單 canonical 交易；查無交易時 `code=0` 且 `data` 空（訂單未建立付款交易屬正常）。有 **PAID** 交易時回該筆，其 `status = '1'`（CodeMeta `TransactionStatus.PAID`） |

> 🔴 **本 RPC 以 `order_no` 為鍵，不得改成 `trade_no`**：切換付款方式會產生新 row、§2A.6 下同一 order 可能有多筆 row，呼叫端手上的 `trade_no` 未必就是那筆 PAID 的。由 payment（SoT）以 `order_no` 聚合判定「是否存在 PAID 交易」是唯一穩健的問法。實作走 `transactionRepository.findHistoryByOrderId`，命中 PAID row 即回。

#### 5.12.3 業務流程

```mermaid
flowchart TD
  A["gRPC GetActivePaymentStatusByOrderNo(order_no)"] --> B{"isInternalCall()?"}
  B -->|否| R1["拒絕（非 internal 呼叫）"]
  B -->|是| C["findHistoryByOrderId(order_no)"]
  C --> D{"存在 status=PAID 的 row?"}
  D -->|是| E["回 PaymentDetail(status=PAID)"]
  D -->|否，有其他 row| F["回該 canonical row 之狀態"]
  D -->|否，無任何交易| G["回 code=0 + data 空"]
```

## 6. 訊息佇列設計

> 本服務涉及的 routing key 業務語意對應見 §4.4 / §4.5。
> rescue refund Outbox Pattern 內部技術設計見 §2A.6。
>
> 完整 producer / consumer / exchange / queue / DLQ / payload class / Ack 設計見：
> - [36-MQ 與事件總覽 §3.2 支付領域](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md#32-支付領域payment-domain)（payment.events 發布 + rescue refund queue）
> - [36-MQ 與事件總覽 §3.5 PayUni Webhook 領域](./36-MQ%20與事件總覽（MQ%20%26%20Event%20Catalog）.md#35-payuni-webhook-領域payuni-webhook-domain)（payuni.payment.uop 消費）

### 6.1 業務冪等保證

**Webhook Consumer 冪等（incoming）**：

1. Consumer 接收 MQ 事件後先查詢 `existsProcessed(webhookType, idempotencyKey)` — 已處理直接 ACK 跳過
2. Inbox 記錄初始 `is_processed = FALSE`、`process_status = '8201'`（`WebhookProcessStatus.PENDING` 的 CodeMeta value）
3. Handler 執行成功後，在同一 Transaction 內呼叫 `markAsProcessed(webhookId, processStatus)` — 設為 `is_processed = TRUE`
4. 唯一索引 `UNIQUE (webhook_type, idempotency_key) WHERE is_processed = TRUE` 阻擋重複處理

**下游 Consumer 冪等（outgoing）**：

付款結果事件（`payment.success` / `payment.failed` / `payment.conflict`）、rescue refund **與 `payment.expired`** 走 **Transactional Outbox + afterCommit 即時投遞 + 排程器兜底 + publisher-confirm**（§5.10）；其餘事件（`payment.closed` / `refund.*`）於事務內直接發布。可靠性由以下機制保證：
- 付款結果事件**與 `payment.expired`**：與狀態更新同事務寫入 `outbox_events`（afterCommit 立即 publisher-confirm 投遞、失敗交排程器每 5s 兜底），確認 broker ack 才標 SENT（at-least-once 必達）；afterCommit 與兜底可能重送同一 `trade_no`
- gRPC 觸發場景（closed / refund）：MQ 發布在 Transaction 內，失敗時 Transaction 回滾
- Consumer 端冪等：下游以 `trade_no`（`transaction_id` 單欄 `ON CONFLICT DO NOTHING`）/ `refund_no` 做冪等消費，容忍 at-least-once 重送

## 7. 時序圖總覽

### 7.1 信用卡付款流程

> 適用付款方式：信用卡 UPP、ATM 轉帳。`CreatePayment` 後交易保持 INIT，由 `payuni-webhook-service` 解密並透過 MQ 發送付款結果，Payment Service 消費後推進至終態。

```mermaid
sequenceDiagram
participant Frontend
participant Gateway
participant PaySvc
participant OrderSvc
participant DB
participant Inbox as payment_webhooks
participant Outbox as outbox_events
participant WebhookSvc as payuni-webhook-service
participant PayUni
participant MQ

Note over Frontend: 結帳完成後，用戶選付款方式
Frontend->>Gateway: POST /payments（orderNo, paymentType）
Gateway->>PaySvc: gRPC CreatePayment(orderNo, paymentType)

Note over PaySvc,OrderSvc: Step 0：查詢訂單付款資訊
PaySvc->>OrderSvc: gRPC GetOrderForPayment(orderNo)
OrderSvc-->>PaySvc: {merId, amount, buyerAccountId, status, productName}
PaySvc->>PaySvc: 驗證訂單狀態=PENDING_PAYMENT + 驗證買家身分

Note over PaySvc,DB: Step 1：冪等性檢查（防重複扣款）
PaySvc->>DB: SELECT * FROM payment_transactions WHERE order_id=? AND status IN (0,1)

alt 已有 PAID 交易
    PaySvc-->>Gateway: Error「訂單已付款，不可重複建立支付」
else 已有 INIT 交易（冪等回應）
    PaySvc->>PayUni: 以同一 trade_no 重新產生 UPP Form
    PaySvc-->>Gateway: UppFormData（既有交易的 trade_no）
else 無活躍交易（正常建立）
    PaySvc->>DB: INSERT payment_transactions (INIT, expire_at, mer_id, amount)
    PaySvc->>PayUni: AES-256-GCM 加密 + SHA256 簽章（via SDK）
    PaySvc-->>Gateway: UppFormData (action_url, encrypt_info, hash_info, is_platform="1")
end

Gateway-->>Frontend: Return UPP form / redirect URL
Frontend->>PayUni: Form POST (重導至支付頁)

Note right of PayUni: 使用者完成付款

PayUni->>WebhookSvc: POST /webhooks/payuni/notify (EncryptInfo + HashInfo)
WebhookSvc->>WebhookSvc: verify + decrypt + dedup (Inbox/Outbox)
WebhookSvc->>MQ: Publish payuni.events / payuni.payment.uop

MQ->>PaySvc: Consume (MANUAL ACK)

Note over PaySvc: Consumer 統一流程
PaySvc->>PaySvc: 1. 提取 payload → decryptedData
PaySvc->>PaySvc: 2. extractIdempotencyKey → tradeNo:providerTradeNo
PaySvc->>Inbox: 3. 冪等檢查 existsProcessed(PAYUNI_PAYMENT, key)?
PaySvc->>Inbox: 4. INSERT (PENDING) — Transaction 外

Note over PaySvc,Outbox: 5. TransactionTemplate（收斂點 A=PaymentNotifyHandler.handle，三者原子）
PaySvc->>PaySvc: handler.handle(decryptedData) — PaymentNotifyHandler
PaySvc->>DB: UPDATE payment_transactions SET status=PAID WHERE status=INIT AND version=?
PaySvc->>Outbox: PaymentOutboxWriter.enqueueAndDispatch → INSERT outbox_events (payment.success, PENDING)<br/>+ registerSynchronization(afterCommit)
PaySvc->>Inbox: markAsProcessed(webhookId, SUCCESS)
Note over PaySvc,Outbox: TX commit：狀態 + 事件 + Inbox 同生共死（無事務外直發）

Note over PaySvc,MQ: 6. afterCommit 即時投遞（正常路徑即時履約，同執行緒，不等輪詢）
PaySvc->>MQ: PaymentOutboxPublisher.publishConfirmed → payment.events / payment.success
alt broker ack
    MQ-->>PaySvc: ack
    PaySvc->>Outbox: markSENT（排程器不再撿）
else nack / timeout / unroutable / afterCommit 例外（吞不 rethrow）
    PaySvc->>Outbox: markFailed（退避）/ 留 PENDING
    Note over PaySvc,MQ: 7. 排程器兜底：OutboxEventPublisherScheduler 每 5s 掃 PENDING/FAILED 重投
    PaySvc->>MQ: publishConfirmed（重投）
    MQ-->>PaySvc: ack → markSENT
end

PaySvc->>PaySvc: channel.basicAck(deliveryTag)
MQ->>OrderSvc: Consume payment.success（trade_no dedup，容忍重送）
OrderSvc->>OrderSvc: 更新訂單狀態（冪等）
```

### 7.2 逾期處理流程

```mermaid
sequenceDiagram
participant Scheduler as ExpiredTransactionScheduler
participant PaySvc
participant DB
participant MQ
participant OrderSvc

Note over Scheduler: 每 60 秒執行

Scheduler->>DB: SELECT status=INIT AND expire_at <= NOW() LIMIT 100
DB-->>Scheduler: expired transactions list

loop 逐筆處理
    Scheduler->>PaySvc: canMarkAsExpired(transaction)?
    alt 可標記逾期
        Scheduler->>DB: TX 內：UPDATE status=EXPIRED, version=version+1 WHERE trade_no=? AND version=?
        Scheduler->>DB: TX 內：INSERT outbox_events(payment.expired, status=PENDING)<br/>（與狀態轉換同事務，規範例外 A）
        Scheduler->>DB: COMMIT
        Note over Scheduler,MQ: 投遞由 afterCommit 即時 + OutboxEventPublisherScheduler 每 5s 兜底<br/>PaymentOutboxPublisher 以 publisher-confirm 確認後標 SENT（§5.10）
        Scheduler-->>MQ: （經 outbox）payment.events / payment.expired
    else 狀態已變更
        Scheduler->>Scheduler: 跳過（已被 Webhook 更新）
    end
end

MQ->>OrderSvc: Consume payment.expired
OrderSvc->>OrderSvc: 釋放庫存（冪等）
```

### 7.3 退款流程

```mermaid
sequenceDiagram
participant Worker as AfterSales<br/>RefundTaskWorker
participant PaySvc as Payment Service
participant DB
participant Refund as payment_refunds
participant PayUni
participant MQ
participant OrderSvc as Order Service

Note over Worker: 買家申請退貨 → 售後審核核准 → Worker claim 退款任務
Worker->>PaySvc: gRPC CreateRefund(trade_no, amount, refund_no)<br/>internal call（x-internal-service）
PaySvc->>DB: SELECT payment_transactions WHERE trade_no=? FOR UPDATE（L1 行鎖）
PaySvc->>Refund: SUM(amount) WHERE status IN ('0','1','2')（L2 可退餘額）
PaySvc->>PaySvc: 驗證 isRefundable() + hasProcessingRefund(status IN '0','1') + 餘額檢查

alt 驗證失敗
    PaySvc-->>Worker: Error（狀態不允許 / 金額超額 / 已有未終結退款）
else 驗證通過
    PaySvc->>Refund: INSERT payment_refunds ('0'（REQUESTED）, refund_no)（L3 trigger 兜底）
    Note over PaySvc,DB: 單一 @Transactional，L1 行鎖持有至 PayUni 呼叫結束（PayUni 呼叫在鎖內）
    PaySvc->>PayUni: Refund API (mer_id, provider_trade_no, refund_no, amount)（仍持 L1 行鎖）

    alt 信用卡系列（即時退款）
        PayUni-->>PaySvc: Success (provider_refund_no)
        PaySvc->>Refund: UPDATE status='2'（SUCCEEDED）, provider_refund_no=?, completed_at=NOW()
        Note over PaySvc,DB: PayUni 回應成功、寫入終態後 commit → 釋放 L1 行鎖
        PaySvc->>MQ: Publish to payment.events / refund.succeeded
        PaySvc-->>Worker: Success (refund_no, SUCCEEDED)
    else ATM（非即時，受理成功）
        PayUni-->>PaySvc: Success (provider_refund_no)
        PaySvc->>Refund: UPDATE status='1'（PROCESSING）, provider_refund_no=?
        Note over PaySvc,DB: 受理成功、寫入 PROCESSING 後 commit → 釋放 L1 行鎖<br/>（後續 ATM webhook finalize 為另一事務，不在本鎖內）
        PaySvc-->>Worker: Success (refund_no, PROCESSING)

        Note right of PayUni: ATM 退款完成（隔日匯款）

        alt 退款回調（經由 webhook-service → MQ）
            PayUni->>WebhookSvc: Webhook（退款結果）
            WebhookSvc->>MQ: Publish payuni.events / payuni.refund.*
            MQ->>PaySvc: Consume（退款結果事件）
        else 主動查詢（排程或手動觸發）
            PaySvc->>PayUni: QueryRefund API
            PayUni-->>PaySvc: 退款結果
        end

        PaySvc->>Refund: UPDATE status='2'（SUCCEEDED）, completed_at=NOW()
        PaySvc->>MQ: Publish to payment.events / refund.succeeded
    else PayUni 即時拒絕
        PayUni-->>PaySvc: Fail (error message)
        PaySvc->>Refund: UPDATE status='3'（FAILED）, provider_message=?
        Note over PaySvc,DB: PayUni 回應後、寫入 FAILED 終態後 commit → 釋放 L1 行鎖
        PaySvc->>MQ: Publish to payment.events / refund.failed
        PaySvc-->>Worker: Fail (error message)
    end
end

MQ->>OrderSvc: Consume refund.succeeded
OrderSvc->>OrderSvc: 更新訂單退款狀態（冪等）
```

### 7.4 同步付款流程（Apple/Google Pay）

> 適用付款方式：Apple Pay、Google Pay。`CreatePayment` 時 PayUni 同步回傳付款結果，交易直接進入 PAID / FAILED。後續 Webhook 為補充確認，若與同步結果矛盾則觸發衝突檢測。
>
> **Web 端 Apple Pay 額外前置流程**（merchant validation）見 §5.11；本節只涵蓋 `CreatePayment` 後的同步付款交易，不重複描述 merchant validation 環節。行動 App 走原生 PassKit 無此前置。

```mermaid
sequenceDiagram
participant Gateway
participant PaySvc
participant OrderSvc
participant DB
participant PayUni
participant MQ

Gateway->>PaySvc: gRPC CreatePayment(orderNo, APPLE_PAY, apple_pay_token)
PaySvc->>OrderSvc: GetOrderForPayment(orderNo)
OrderSvc-->>PaySvc: {merId, amount, buyerAccountId}
PaySvc->>DB: INSERT payment_transactions (INIT, mer_id, amount)
PaySvc->>PayUni: ApplePay API (加密 + 簽章 + token via SDK)
PayUni-->>PaySvc: 同步結果 (TradeStatus=1/2)

Note over PaySvc,DB: @Transactional createPayment（收斂點 B=PaymentApplicationService.publishEvent）
alt 付款成功 (TradeStatus=1)
    PaySvc->>DB: UPDATE status=PAID, provider_trade_no=?, pay_time=?<br/>+ PaymentOutboxWriter.enqueueAndDispatch → INSERT outbox_events(payment.success, PENDING)<br/>+ 註冊 afterCommit（同事務原子）
    Note over PaySvc,MQ: TX commit 後 afterCommit 即時投遞（吞例外，失敗交排程器兜底）
    PaySvc->>MQ: PaymentOutboxPublisher.publishConfirmed → payment.events / payment.success
    PaySvc-->>Gateway: PaymentResult (status=PAID)
else 付款失敗 (TradeStatus=2)
    PaySvc->>DB: UPDATE status=FAILED<br/>+ PaymentOutboxWriter.enqueueAndDispatch → INSERT outbox_events(payment.failed, PENDING)<br/>+ 註冊 afterCommit（同事務原子）
    PaySvc->>MQ: afterCommit publishConfirmed → payment.events / payment.failed
    PaySvc-->>Gateway: PaymentResult (status=FAILED)
end

MQ->>OrderSvc: Consume（trade_no dedup）
```

> **衝突情境**：Apple Pay / Google Pay 在 `CreatePayment` 同步取得終態後，PayUni 仍可能非同步發送 Webhook（經由 webhook-service → MQ → PayUniWebhookConsumer）。`PaymentNotifyHandler` 步驟 2 會解析 Webhook 的 `TradeStatus`，若與 DB 現有終態矛盾（例如同步 PAID 但 Webhook FAILED），走 `updateToConflict()` + `PaymentConflict` 事件分支，發布至 `payment.conflict` routing key。若兩者一致則維持 SKIPPED。信用卡 UPP / ATM 不受影響（Webhook 是唯一終態來源）。

### 7.5 撥款指示流程

> 適用場景：買家確認收貨 → Order Service 更新訂單狀態為 COMPLETED → 呼叫 Payment Service GrantPayment RPC → Payment Service 呼叫 PayUni Grant API。

```mermaid
sequenceDiagram
participant OrderSvc as Order Service
participant PaySvc as Payment Service
participant DB as PostgreSQL
participant PayUni as PayUni

Note over OrderSvc: 買家確認收貨，訂單狀態 DELIVERED(40) → COMPLETED(50)

OrderSvc->>PaySvc: gRPC GrantPayment(order_no)

Note over PaySvc,DB: @Transactional 開始（行鎖持有至事務結束，涵蓋 PayUni 呼叫）
PaySvc->>DB: findActiveByOrderIdForUpdate(order_no)<br/>SELECT ... WHERE order_id=? AND status=1(PAID) FOR UPDATE
DB-->>PaySvc: transaction (trade_no, mer_id, provider_trade_no, amount, grant_status)

alt grant_status == SUCCESS（撥款前冪等短路）
    PaySvc-->>OrderSvc: Success（回傳既有 GrantPaymentResult，不呼叫 PayUni）
else 交易不存在或非 PAID 狀態
    PaySvc-->>OrderSvc: Error「交易不存在或尚未付款」
else provider_trade_no 為空
    PaySvc-->>OrderSvc: Error「缺少 PayUni 交易編號，無法撥款」
else 驗證通過（grant_status != SUCCESS）
    alt 首撥（grant_status == null）— 直接發動
        Note over PaySvc,PayUni: Grant API（代理商級別，仍持行鎖）；invokeGrantWithRetry try/catch 捕捉 PayUniApiException（不穿透）
        PaySvc->>PayUni: POST /api/trade/grant<br/>外層 AgentID=HAWK, Version=1.0；EncryptInfo{MerID, TradeNo(UNi序號), Amt}
        PayUni-->>PaySvc: {Status, Message} 或 PayUniApiException(timeout/HTTP_ERROR/EMPTY_RESPONSE)
        alt Status=SUCCESS
            PaySvc->>DB: grant_status='SUCCESS', granted_at=NOW()（commit）+ payment_logs(GRANT_SUCCESS)
            PaySvc-->>OrderSvc: gRPC Success
        else 明確未發動（拒絕碼/參數錯）— A：同步 retry 最多一次
            PaySvc->>DB: grant_status=PayUni錯誤碼（隨交易 commit 存活，不 throw rollback）
            PaySvc-->>OrderSvc: adapter 依 grant_status!=SUCCESS throw gRPC error（進 Admin）
        else 已由 PayUni 接管（GrantResponseClass=PROVIDER_TOOK_OVER：確定拒絕 + hasSucceededRefund 延遲查詢命中，§5.6.1）— 不重試
            PaySvc->>DB: grant_status=PayUni回應碼原樣、granted_at 保持 NULL（隨交易 commit 存活，不 throw rollback）
            PaySvc-->>OrderSvc: 正常 gRPC 回應（grant_outcome=PROVIDER_TOOK_OVER，非 error）<br/>Order 端 Worker 據此收斂 task=PROVIDER_AUTO（03 §3.11.2，不重試、不進人工介入）
        else 模稜兩可（PayUniApiException / PARSE_ERROR / HASH_INVALID / 未知碼）— A：不自動重試
            PaySvc->>DB: grant_status='PENDING_CONFIRM'（隨交易 commit 存活，不 throw rollback）
            PaySvc-->>OrderSvc: adapter throw gRPC error（進 Admin；下輪由 grant_query 對帳把關）
        end
    else 非首撥（grant_status ∈ {PENDING_CONFIRM, PayUni錯誤碼}）— 去重硬化 B：重試前 grant_query 對帳
        Note over PaySvc,PayUni: grant_query 對帳（callApi + MerID + IsPlatForm=1）
        loop QueryDate 逐日涵蓋 [首次發動嘗試日, 今日]（≤30 天，起點取 updated_at）
            loop Page 1..PageInfo.Total（每頁 500 筆，遍歷全分頁）
                PaySvc->>PayUni: POST /api/trade/grant_query<br/>QueryDate, Page
                PayUni-->>PaySvc: Status=SUCCESS→Result[]（TradeNo…）+ PageInfo<br/>Status=QUERY03002→該日有效空頁(count=0)，續下一日<br/>Status=QUERY02004/transport→該日未查完→INCONCLUSIVE
            end
        end
        alt 命中已入帳（僅以 TradeNo==provider_trade_no 精確比對）
            PaySvc->>DB: UPDATE grant_status='SUCCESS', granted_at=NOW()（補記終態，交易 commit）
            PaySvc-->>OrderSvc: gRPC Success（已撥款，不重發，不打 Grant API）
        else 未查完（QUERY02004超窗/transport/其他非預期）/ 落在入帳前（結算延遲）
            PaySvc-->>OrderSvc: adapter throw gRPC error（維持 PENDING_CONFIRM，不重發，不打 Grant API）
        else 全區間逐日（含 QUERY03002 空日）+全分頁查完仍查無 → 確認未撥，安全重發
            Note over PaySvc,PayUni: 發動 Grant API（分類同上「首撥」：SUCCESS / 明確未發動→錯誤碼 / 已由 PayUni 接管→正常回應 grant_outcome=PROVIDER_TOOK_OVER 不重試 / 模稜兩可→PENDING_CONFIRM）
            PaySvc->>PayUni: POST /api/trade/grant<br/>EncryptInfo{MerID, TradeNo, Amt}
            PayUni-->>PaySvc: {Status, Message} 或 PayUniApiException
            alt Status=SUCCESS
                PaySvc->>DB: grant_status='SUCCESS', granted_at=NOW()（commit）
                PaySvc-->>OrderSvc: gRPC Success
            else 失敗（明確未發動→錯誤碼 / 模稜兩可→PENDING_CONFIRM，均隨 commit 存活、不 rollback）
                PaySvc->>DB: grant_status=錯誤碼 或 PENDING_CONFIRM
                PaySvc-->>OrderSvc: adapter throw gRPC error（進 Admin）
            end
        end
    end
end
Note over PaySvc,DB: @Transactional commit（終態哨兵值落地）→ 釋放行鎖；失敗訊號於 commit 後產生
```

### 7.6 信用卡導回流程

> 適用場景：信用卡 UPP 付款完成後，PayUni 以 **Form POST** 方式將使用者瀏覽器導回 ReturnURL。
> Gateway 提取 EncryptInfo + HashInfo → 呼叫 Payment Service **gRPC** → 解密 + DB lookup（client_type / app_type）+ 更新 DB + 依矩陣組 redirect_url → **302 redirect** 導回對應端。
>
> **統一模式**：信用卡導回、超商門市地圖、ATM 退款頁面回調三者採相同架構——**Gateway 薄代理 ＋ 微服務 gRPC ＋ 302 redirect**，Gateway 不持有業務邏輯。
>
> **核心設計**：Return POST 與 Webhook 通知攜帶相同的交易結果，透過冪等鍵確保不重複處理。**誰先到誰處理**，不再強制仰賴 Webhook 更新交易狀態。

```mermaid
sequenceDiagram
participant Client as 買家端（App / Browser）
participant Stage as WebView 或 Browser
participant PayUni
participant Gateway as Gateway :9900
participant PaySvc as Payment Service
participant DB as PostgreSQL
participant MQ as RabbitMQ
participant WebhookSvc as payuni-webhook-service

Note over Client,Stage: 使用者在 WebView（App）或 Browser（Web）完成信用卡付款

par PayUni 同時觸發兩條路徑
    Note over PayUni,WebhookSvc: 路徑 A：Server-to-Server Webhook（兜底）
    PayUni->>WebhookSvc: POST NotifyURL（背景通知）
    WebhookSvc->>MQ: Publish payuni.payment.uop
    MQ->>PaySvc: Consume → 冪等處理（若 Return 已處理則跳過）
and
    Note over PayUni,Stage: 路徑 B：Browser Form POST（優先路徑）
    PayUni->>Stage: POST /payment/return（EncryptInfo + HashInfo）
end

Stage->>Gateway: POST /payment/return（Form POST）
Gateway->>PaySvc: gRPC HandlePaymentPageReturn(EncryptInfo, HashInfo)

Note over PaySvc: 1. 驗簽（HashInfo）
Note over PaySvc: 2. 解密 EncryptInfo → 取得 MerTradeNo、TradeStatus
Note over PaySvc: 3. lookup payment_transactions → 取 client_type / app_type

PaySvc->>DB: 儲存 payment_webhooks（Inbox 稽核記錄）
PaySvc->>DB: 冪等檢查（idempotency_key）

alt 首次處理
    Note over PaySvc,DB: TX 內原子（收斂點 A=PaymentNotifyHandler.handle）
    PaySvc->>DB: 更新 payment_transactions 狀態（樂觀鎖）<br/>+ PaymentOutboxWriter.enqueueAndDispatch → INSERT outbox_events(PENDING)<br/>+ 註冊 afterCommit + markAsProcessed
    Note over PaySvc,MQ: afterCommit 即時投遞（吞例外，失敗交排程器每 5s 兜底）
    PaySvc->>MQ: PaymentOutboxPublisher.publishConfirmed → payment.success / payment.failed
    MQ->>MQ: order-service 消費（trade_no dedup）→ 更新訂單狀態
else 已被 Webhook 處理過（冪等跳過）
    Note over PaySvc: 直接用已知結果組 redirect_url
end

alt client_type = MOBILE_APP
    Note over PaySvc: redirect_url = hawkr{buyer|seller}://payment/result?tradeNo=...&status=...
else client_type = WEB_BROWSER
    Note over PaySvc: redirect_url = {WEB_FRONTEND_URL}/orders/{orderNo}?status=...&trade_no=...&code=...
end

PaySvc-->>Gateway: redirect_url
Gateway-->>Stage: 302 redirect → redirect_url

alt MOBILE_APP
    Stage->>Client: WebView 攔截 URL Scheme → 導回 App
    Note over Client: 依 status 顯示 Toast + 導向對應頁面
else WEB_BROWSER
    Stage->>Client: Browser 載入 web 落地頁
    Note over Client: 落地頁載入後**強制**呼 GetPayment(trade_no)<br/>取真實狀態 → 顯示對應 toast / modal<br/>（query status 不可信，僅作 UI hint）
end
```

> **重要說明**：
> - Return POST 與 Webhook 是兩條**獨立路徑**，到達順序不保證
> - 兩者使用**相同的冪等鍵**（`tradeNo:providerTradeNo`）與**相同的 PaymentNotifyHandler**，確保交易狀態只更新一次
> - Payment Service 在 `payment_webhooks` 表記錄來源（`GRPC_RETURN` 或 `MQ:{queue}`），方便稽核追蹤
> - 此設計消除了對 Webhook 的強依賴：即使 Webhook 延遲或遺失，Return POST 已完成交易狀態更新
> - **client_type / app_type SoT 在 DB**：`HandlePaymentPageReturn` 不從 query 或 RPC 入參取，避免 PayUni UPP 不保留 query 的限制 + 防止偽造
> - **WEB_BROWSER 安全約定**：query status 僅作 UI hint，前端必須以 trade_no 二次呼 `GetPayment` 確認真實狀態（見 §5.9）

---

## 8. 一致性、交易與補償設計

### 8.1 本地交易邊界

Payment Service 內部使用 `TransactionTemplate` 確保以下操作在同一 DB Transaction 內完成：

- **付款結果 notify 三入口場景**（MQ Consumer / HTTP 導回頁 / gRPC 導回頁，收斂點 A）：`handler.handle(decryptedData)`（交易狀態落地 **＋** `PaymentOutboxWriter.enqueueAndDispatch` 寫 `outbox_events` + 註冊 afterCommit dispatch）+ `webhookMapper.markAsProcessed(...)` — 狀態、事件、Inbox 標記**三者原子**
- **同步付款 in-tx 場景**（`CreatePayment` 零元 / Apple·Google Pay 同步、`syncPaymentStatus` poll·reconcile，收斂點 B）：狀態更新與 `PaymentOutboxWriter.enqueueAndDispatch`（經 `PaymentApplicationService.publishEvent`）在同一 `@Transactional` 內原子，afterCommit 即時投遞
- **其餘 gRPC / 排程觸發場景**（`ClosePayment` / expired / refund）：狀態更新在 Transaction 內，`PaymentEventPublisher`（本體不改）直接發布失敗時 Transaction 回滾

> **設計要點**：付款結果事件（六發布點）**不再於 Transaction 外/前直發**，而是與狀態更新同事務寫入 `outbox_events`，由 **afterCommit 即時 dispatch**（正常路徑即時履約）+ 排程器兜底 + publisher-confirm 保證投遞（§5.10）——同時杜絕「commit 後、直發前程序崩潰 → 事件遺失（under-fulfillment）」與「事務 rollback 但事件已流出（over-fulfillment）」兩面破口。afterCommit callback 吞例外不 rethrow（confirm 失敗不得讓已成功付款回錯）。Inbox 儲存（Consumer 步驟 6）仍在 Transaction 外，確保即使 Handler 失敗也能保留記錄供稽核。其餘事件（closed / expired / refund）維持事務內直發（失敗 rollback）。

### 8.2 跨服務一致性策略

本服務透過以下機制保證跨服務最終一致性：

- **付款結果事件（success / failed / conflict）**：Outbox Pattern + publisher-confirm——與狀態更新同事務寫入 `outbox_events`，排程器確認 broker ack 才標 SENT（§5.10），保證「狀態已改 ⇒ 事件必達至少一次」
- **其餘事件（expired / closed / refund）**：於 gRPC / 排程觸發的 Transaction 內直接發布，失敗時 Transaction 回滾
- **Consumer 端冪等**：下游以 trade_no / refund_no（order-service `payment_event_dedup`）做冪等消費，容忍 at-least-once 重送
- **補單機制**：`SyncPaymentStatus` RPC 可手動觸發 PayUni 狀態同步，補償 Webhook 遺失的情況
- **逾期取消守門（縱深防護）**：Order Service 逾期取消訂單前，經 `GetActivePaymentStatusByOrderNo`（§5.12）反查付款 SoT，PAID 則不取消並收斂——涵蓋事件 in-flight / outbox 延遲 / DEAD 的殘餘窗口（見 [03 §8.5](./03-訂單微服務（Order%20Service）詳細設計書.md#85-paymentdeadlinescheduler付款超時取消)）

### 8.3 Webhook Inbox 模式

Payment Service 使用 `payment_webhooks` 表作為 Inbox，記錄所有從 MQ 消費的事件：

1. Consumer 接收 MQ 事件
2. 提取冪等鍵 → 查詢是否已處理（`existsProcessed`）
3. 儲存 Pending Inbox（Transaction 外）
4. TransactionTemplate 內執行 Handler（狀態落地 + `PaymentOutboxWriter.enqueue` 寫 `outbox_events`）+ markAsProcessed（三者原子）
5. MANUAL ACK
6. 付款結果事件由 `OutboxEventPublisherScheduler` + `PaymentOutboxPublisher` 以 publisher-confirm 非同步投遞（§5.10，不在 Consumer 主流程內）

此模式確保：
- **冪等性**：唯一索引 `UNIQUE (webhook_type, idempotency_key) WHERE is_processed = TRUE` 阻擋重複處理
- **稽核追蹤**：所有事件（含已跳過的）皆有記錄
- **故障恢復**：PENDING 狀態的 Inbox 記錄可用於排查未成功處理的事件

### 8.4 補償與人工修復策略

**CONFLICT 狀態處理**：
- 當 Apple Pay / Google Pay 同步結果與後續 Webhook 矛盾時，交易標記為 CONFLICT
- CONFLICT 為終態，不可自動恢復，需人工介入
- `payment_logs` 的 `conflict_detail`（JSONB）記錄衝突雙方的狀態與來源
- `PaymentConflict` 事件發布至 `payment.conflict` routing key，可觸發告警

**DLQ 人工檢視**：
- `payment.payuni.webhook.dlq` 存放處理失敗的 MQ 訊息
- DLQ 不配置 Consumer，僅供人工檢視與手動重投
- 重試超過 1 次（`isRedelivered()`）後自動路由至 DLQ

**手動補單**：
- 透過 `SyncPaymentStatus` RPC 主動查詢 PayUni 最新交易狀態
- 若 Webhook 遺失或 DLQ 中的訊息無法自動修復，可透過此 RPC 手動補單

---

## 9. 環境配置

### 9.1 Port / Node ID

**服務埠號：**

| 項目 | Port | 說明 |
|---|---|---|
| HTTP（Actuator） | 9924 | 健康檢查與監控 |
| gRPC | 9925 | 供 Gateway / 其他服務呼叫 |

**TSID 配置：**

```yaml
hawkr:
  tsid:
    enabled: true
    node-id-base: 0              # Payment Service: 0-99
    pod-index: ${POD_INDEX:0}
```

### 9.2 環境變數與配置

Payment Service 使用 Spring Boot application.yml，敏感配置透過環境變數注入。

**PayUni 代理商模式配置（非商家模式）：**

```yaml
payuni:
  agent:
    id: ${PAYUNI_AGENT_ID:HAWK}    # 代理商前綴字（4碼大寫，撥款 API 使用）
    key: ${PAYUNI_AGENT_KEY}       # 代理商 AES Key（32 bytes）
    iv: ${PAYUNI_AGENT_IV}         # 代理商 AES IV（16 bytes）
  api:
    base-url: ${PAYUNI_API_URL:https://sandbox-api.payuni.com.tw}
  callback:
    notify-url: ${PAYUNI_NOTIFY_URL}   # Webhook 通知 URL（指向 webhook-service）
    return-url: ${PAYUNI_RETURN_URL}   # 支付完成導回 URL
```

> **重要**：使用**代理商模式**（IsPlatForm="1"），加密 Key/IV 為代理商級別，非子商家級別。子商家 MerID 由 Order Service 傳入。
> **撥款控制**：所有交易建立時帶入 `GrantExport=1`，由代理商控制撥款時機。撥款 API 外層使用 `AgentID`（`payuni.agent.id`）而非 `MerID`。
> **注意**：`notify-url` 現在應指向 `payuni-webhook-service`（Port 9946），不再指向 Payment Service。

**資料庫：**

```yaml
spring:
  datasource:
    url: jdbc:postgresql://${DB_HOST:127.0.0.1}:${DB_PORT:5432}/${DB_NAME:hawkr_payment}
    username: ${DB_USER:dbadmin}
    password: ${DB_PASSWORD}
```

**RabbitMQ：**

```yaml
spring:
  rabbitmq:
    host: ${RABBITMQ_HOST:127.0.0.1}
    port: ${RABBITMQ_PORT:5672}
    username: ${RABBITMQ_USERNAME:guest}
    password: ${RABBITMQ_PASSWORD:guest}
```

**清理排程配置：**

```yaml
payment:
  cleanup:
    enabled: ${CLEANUP_ENABLED:true}
    cron: "0 30 2 * * ?"
    log-retention-days: 90
    error-log-retention-days: 365
    webhook-retention-days: 90
    outbox-sent-retention-days: 7    # 已接線（只清 SENT，以 sent_at 為起點）
    outbox-dead-retention-days: 30   # 🔴 刻意未接線，見 §5.8 說明
```

**ATM 有效天數**（見 §2A.5.5）：由 common-core 預設承載，**不在本服務的 `application.yml` 另立一份**——兩份會漂移，而實際生效的是哪一份要從綁定順序推導才知道。

```properties
hawkr.payment.atm.expire-days=${PAYMENT_ATM_EXPIRE_DAYS:2}
```

### 9.3 安全規範

**通訊加密與驗簽：**
- **加密**：PayUni 通訊使用 AES-256-GCM 加密 + SHA256 簽章，委託 `payuni-java-client` SDK 執行
- **Webhook 驗簽**：由 `payuni-webhook-service` 統一驗簽，Payment Service 信任 MQ 來源
- **金額與商家編號信任**：CreatePayment 的 `amount` 和 `mer_id` 由 Payment Service 主動從 Order Service 查詢（`GetOrderForPayment`），不信任前端傳入值。前端僅傳 `order_no`
- **密鑰管理**：AES Key/IV 嚴禁硬編碼，必須從環境變數注入
- **日誌安全**：禁止在日誌中印出加密 Key/IV 或完整加密字串

**ATM 退款銀行資訊（平台不落地）：**

ATM 退款一律走 PayUni 頁面版（UPP）：買家在 PayUni 頁面自行填寫銀行帳號與身分證字號，由 PayUni 直接收單，退款結果經 Webhook 回傳。**平台全程不接收、不傳輸、不儲存買家銀行資訊**，因此 ATM 退款路徑上沒有需要加密的欄位，也沒有對應的加解密元件。

**頁面版呼叫鏈**：

| 層 | 位置 | 職責 |
|------|------|------|
| AfterSales RPC | `AfterSalesGrpcService.getRefundPageUrl()` | 售後案件發起，向 Payment 取得 redirect URL |
| Payment RPC | `PaymentGrpcService.getRefundPageUrl()` | 校驗可退餘額（排除本筆，§5.5.5），再組裝頁面參數 |
| SDK 組裝 | `PaymentApplicationService.buildRefundPageFormData()` → `PaymentClient.offlineRefundPage()` | 產生 `UppFormData`（加密參數 + 簽章），轉為 redirect URL |
| 買家填寫 | PayUni 頁面 | 銀行欄位只存在於買家與 PayUni 之間 |

平台端流經的資料僅有 `TradeNo` / `RefundAmt` / `ReturnURL`，不含任何銀行欄位。

> 🔴 **平台端不持有任何銀行帳號於退款路徑上**：ATM 退款只有「頁面版」一條路徑（使用者在 PayUni 頁面自行填寫銀行資訊），平台端流經的資料僅 `TradeNo` / `RefundAmt` / `ReturnURL`。`PaymentClient.offlineRefund()`（API 版，需帶銀行欄位）**無任何呼叫端，不得接上**。
>
> `PaymentClient.offlineRefund()`（需後端提供銀行資訊的 API 版）同樣無任何呼叫端，ATM 退款只有頁面版一條路徑。

**平台實際持有的銀行帳號**不在退款路徑，而在賣家實名認證的 `certification_applications.bank_account`（賣家收款帳戶，18 號設計書）。

### 9.4 排程任務清單

| 排程 | 類別 | 頻率 | 用途 |
|------|------|------|------|
| 逾期交易檢查 | `ExpiredTransactionScheduler` | 每 60 秒（`fixedDelay=60000`）| 標記 `expire_at` 已到期的 INIT 交易為 EXPIRED。狀態 CAS 與 `payment.expired` 事件由 `ExpiredTransactionCloseService.closeExpiredInTx` 在**同一事務內寫入 `outbox_events`**（非直接發布 MQ；投遞語意見 §4.4） |
| Outbox 事件派送 | `OutboxEventPublisherScheduler` | 每 5 秒（`fixedDelay=5000`）| 掃 `outbox_events` 的 PENDING / 已到重試時間的 FAILED，交 `PaymentOutboxPublisher` 投遞；超過 `payment.outbox.max-retries`（預設 5）標 DEAD |
| L2 付款對帳補救 | `PendingPaymentReconcileScheduler` | 每 5 分鐘（`hawkr.payment.reconcile.pending.fixed-delay-ms`，預設 300000；`...pending.enabled=false` 可停用）| 撈 reconcile 候選交易逐筆向 PayUni 查詢並收斂狀態，outcome 記入 `hawkr_payment_reconcile_outcome` metric（§5.7.1） |
| 資料清理 | `DataCleanupScheduler` | 每日 02:30（`payment.cleanup.cron`，預設 `0 30 2 * * ?`）| 清理過期日誌 / Webhook 記錄 |

---

## 10. PayUni 時區與時間欄位處理

> **本章節是全域時區一致性附錄**：PayUni 所有時間欄位的真實格式、統一的解析／格式化工具方法，以及跨邊界的寫入契約。

### 10.1 背景

依 [Java 微服務開發規範 §2（型別對應：時間）](../開發規範/Java%20微服務開發規範.md)，全系統採用 UTC：
- DB 用 `TIMESTAMPTZ`
- Java 用 `LocalDateTime`（值語意 UTC）
- 取時間用 `DateTimeUtils.nowUtc()`，禁止裸 `LocalDateTime.now()`
- API 輸出 ISO 8601 帶 Z

但 **PayUni 是台灣金流系統，所有時間欄位都是 UTC+8**。跨邊界（與 PayUni 互動）必須明確標記時區並轉換，否則跨日邊界會差一天。

本章節集中處理所有 PayUni 時區邊界。

### 10.2 PayUni 時間欄位完整對照表

> 格式以 [PayUni 金流官方 API 總覽](../第三方整合/PayUni%20金流官方%20API%20總覽.md) 為準；下表的「備註」欄記載官方文件未明載、但 API 實際行為必然如此的細節。

| API / 來源 | 欄位 | 方向 | 真實格式 | 時區 | 備註 |
|-----------|------|------|---------|------|------|
| `/api/atm` request | `ExpireDate` | 我方→PayUni | `yyyy-MM-dd` | UTC+8 | 我方送日期，PayUni 設為「該日 23:59:59 UTC+8」 |
| `/api/atm` response | `ExpireDate` | PayUni→我方 | `yyyy-MM-dd HH:mm:ss` | UTC+8 | 永遠是「該日 23:59:59」 |
| `/api/upp` (信用卡) webhook | `AuthDay` | PayUni→我方 | `yyyyMMdd`（無分隔） | UTC+8 | — |
| `/api/upp` webhook | `AuthTime` | PayUni→我方 | `HHmmss`（無分隔） | UTC+8 | 與 `AuthDay` 拼接才是完整時間 |
| `/api/atm` webhook | **無付款時間欄位** | — | — | — | ATM webhook 不帶付款時間，必須改查 query API |
| `/api/trade/query` response | `CreateDay` | PayUni→我方 | `yyyy-MM-dd HH:mm:ss` | UTC+8 | — |
| `/api/trade/query` response | `PaymentDay` | PayUni→我方 | `yyyy-MM-dd HH:mm:ss`，**未付款時為 `0000-00-00 00:00:00` 占位符** | UTC+8 | 呼叫端必須處理占位符（parser 回 `null`） |
| `/api/trade/query` response | `OffExpireTime` | PayUni→我方 | `yyyy-MM-dd HH:mm:ss` | UTC+8 | ATM 虛擬帳號到期時間 |
| `/api/trade/close` (退款) response | **無時間欄位** | — | — | — | 退款回應不帶時間，到帳日另由 webhook 帶回 |
| `/api/home_delivery/get_obt_number_pdf` request | `ShipDate`, `DeliveryDate` | 我方→PayUni | `yyyyMMdd`（無分隔） | UTC+8 | 物流取號日期，物流驗證確認 |
| 愛金卡/LinePay/JKoPay webhook | `PayTime` | PayUni→我方 | `yyyy-MM-dd HH:mm:ss` | UTC+8 | PayUni 文件 |
| 後支付 AFTEE webhook | `CreateDT` | PayUni→我方 | 文件未明確 | UTC+8 | 文件未明載格式；目前無消費路徑 |

**關鍵觀察**：PayUni **同一時間概念用三種不同格式**（`yyyy-MM-dd HH:mm:ss` / `yyyyMMdd` / `yyyyMMdd + HHmmss`）。我方 parser 必須**按 source 分流**處理，不可假設統一格式。

### 10.3 DateTimeUtils 新增工具方法

> 位置：[services/java/shared/common-core/.../utils/DateTimeUtils.java](services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/DateTimeUtils.java)
>
> 既有方法（不變）：`nowUtc()`、`formatIso8601()`、`toEpochMillis()`

#### 新增方法

```java
public class DateTimeUtils {

    /** PayUni / 物流 系統的時區（台灣）。 */
    public static final ZoneId PAYUNI_ZONE = ZoneId.of("Asia/Taipei");

    /** 占位符：PayUni query API 對未付款 trade 回傳的 PaymentDay 值。 */
    private static final String PAYUNI_NULL_DATETIME_PLACEHOLDER = "0000-00-00 00:00:00";

    private static final DateTimeFormatter PAYUNI_DATE_FMT =
            DateTimeFormatter.ofPattern("yyyy-MM-dd");
    private static final DateTimeFormatter PAYUNI_DATETIME_FMT =
            DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");
    private static final DateTimeFormatter PAYUNI_AUTH_DAY_FMT =
            DateTimeFormatter.ofPattern("yyyyMMdd");
    private static final DateTimeFormatter PAYUNI_AUTH_TIME_FMT =
            DateTimeFormatter.ofPattern("HHmmss");

    // ===== 我方 → PayUni（送出日期）=====

    /** 取得目前在台北時區的日期（送 PayUni 用，避免 UTC 容器跨日邊界錯一天）。 */
    public static LocalDate todayInTaipei() {
        return LocalDate.now(PAYUNI_ZONE);
    }

    /** 取得 N 天後的台北日期。 */
    public static LocalDate taipeiPlusDays(int days) {
        return todayInTaipei().plusDays(days);
    }

    /** 格式化為 PayUni `yyyy-MM-dd` 字串。 */
    public static String formatPayUniDate(LocalDate date) {
        return date.format(PAYUNI_DATE_FMT);
    }

    /** 格式化為 PayUni 物流 `yyyyMMdd` 字串。 */
    public static String formatShippingDate(LocalDate date) {
        return date.format(PAYUNI_AUTH_DAY_FMT);
    }

    // ===== PayUni → 我方（解析回傳）=====

    /**
     * 解析 PayUni `yyyy-MM-dd HH:mm:ss` UTC+8 字串為 UTC LocalDateTime。
     *
     * @return UTC 的 LocalDateTime；若輸入為 null/blank/PayUni 占位符 → 回 null
     */
    public static LocalDateTime parsePayUniDateTimeToUtc(String s) {
        if (s == null || s.isBlank() || PAYUNI_NULL_DATETIME_PLACEHOLDER.equals(s.trim())) {
            return null;
        }
        LocalDateTime taipeiLocal = LocalDateTime.parse(s.trim(), PAYUNI_DATETIME_FMT);
        return taipeiLocal.atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC).toLocalDateTime();
    }

    /**
     * 解析 PayUni 信用卡 webhook 的 AuthDay (`yyyyMMdd`) + AuthTime (`HHmmss`) UTC+8
     * 為 UTC LocalDateTime。
     *
     * @return UTC 的 LocalDateTime；若任一輸入為 null/blank → 回 null
     */
    public static LocalDateTime parsePayUniAuthDayTimeToUtc(String authDay, String authTime) {
        if (authDay == null || authDay.isBlank() || authTime == null || authTime.isBlank()) {
            return null;
        }
        LocalDate date = LocalDate.parse(authDay.trim(), PAYUNI_AUTH_DAY_FMT);
        LocalTime time = LocalTime.parse(authTime.trim(), PAYUNI_AUTH_TIME_FMT);
        LocalDateTime taipeiLocal = LocalDateTime.of(date, time);
        return taipeiLocal.atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC).toLocalDateTime();
    }

    /**
     * 將 PayUni `yyyy-MM-dd` 日期視為「該日 23:59:59 UTC+8」轉為 UTC LocalDateTime。
     * 給「ATM ExpireDate 補位」用（例如 PayUni 異常時的告警，不應在主路徑使用）。
     */
    public static LocalDateTime payUniDateEndOfDayToUtc(String yyyyMmDd) {
        if (yyyyMmDd == null || yyyyMmDd.isBlank()) return null;
        LocalDate date = LocalDate.parse(yyyyMmDd.trim(), PAYUNI_DATE_FMT);
        return date.atTime(23, 59, 59).atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC).toLocalDateTime();
    }
}
```

#### 使用守則

1. **送 PayUni 任何「日期」參數**：必用 `todayInTaipei()` / `taipeiPlusDays(n)` + `formatPayUniDate()`，**禁止**裸 `LocalDate.now()`
2. **解析 PayUni 回的時間欄位**：必用對應 parser，**禁止**自行 `LocalDate.parse()`
3. **0000-00-00 00:00:00 占位符**：自動回 `null`，呼叫端必須處理 null
4. **時區轉換**：所有 parser 內部完成 UTC+8 → UTC 轉換，呼叫端拿到的就是 UTC LocalDateTime（值語意 UTC，可直接寫 DB）
5. ⚠️ **PayUni 物流 API 是例外**：它的日期參數確實吃 `yyyyMMdd`（無分隔），shipping 的物流 strategy 維持該格式，**不要**比照金流欄位改掉（見 06 設計書）

### 10.5 PayUni 真實時間軸寫入 DB

🔴 **webhook handler 必須抽出 PayUni 的時間欄位並落 DB。** 只看 status code 的話，對帳時無法回溯「PayUni 認為的付款時間」，而我方的 `received_at` / `pay_time` 是**我方時間**（可能延遲、可能是重投），兩者不可互相冒充。

#### 時間欄位

##### `payment_transactions`

| 欄位 | 型別 | NULL? | 來源 | 說明 |
|------|------|-------|------|------|
| `provider_create_at` | `TIMESTAMPTZ` | 可 NULL | PayUni query `CreateDay` 或 webhook `AuthDay+AuthTime` | PayUni 認為交易建立的時間 |
| `provider_paid_at` | `TIMESTAMPTZ` | 可 NULL | webhook 信用卡：`AuthDay+AuthTime`<br>webhook ATM：query API 補回 `PaymentDay` | PayUni 認為使用者真實付款的時間 |

> 既有 `pay_time` (TIMESTAMPTZ) 欄位是「我方寫入時間」，與 `provider_paid_at` 不同。**保留兩個欄位**以區分「我方收到 webhook 的時間」與「PayUni 認為的真實付款時間」。

##### `payment_refunds`

| 欄位 | 型別 | NULL? | 說明 |
|------|------|-------|------|
| `provider_estimated_refund_at` | `TIMESTAMPTZ` | 可 NULL | PayUni `EstimatedRefundDate`（ATM 退款預估到帳日），UTC+8 視為當日 00:00 轉 UTC 存 |

##### `webhook_inbox`（payuni-webhook-service）

| 欄位 | 型別 | NULL? | 說明 |
|------|------|-------|------|
| `provider_event_at` | `TIMESTAMPTZ` | 可 NULL | PayUni 端事件發生時間（從 decrypted_payload 抽出） |

> `received_at` 既有欄位保留為「我方接收時間」，與 `provider_event_at`（PayUni 端時間）區分。
>
> **NULL 比例預期偏高**：依 §10.2，並非所有 webhook payload 都含時間欄位 — 例如 ATM webhook 完全無時間欄位，物流 webhook 也僅 status 變更通知。對於這些 source，`provider_event_at` 會是 NULL，由 `received_at` 作為唯一時間參考。
>
> **必須是獨立欄位，不得改成從 JSONB `decrypted_payload` 即席查詢**：
> 1. **效能**：對 `provider_event_at` 建索引比 `jsonb_path_query` 快數量級
> 2. **對帳精度**：信用卡 webhook 帶 `AuthDay+AuthTime`、愛金卡/LinePay 帶 `PayTime`，這些 source 的 `provider_event_at` 是後續對帳的關鍵時間軸
> 3. **演進空間**：若 PayUni 未來新增更多帶時間的 webhook 類型，欄位無需 schema 變更即可支援
> 4. **NULL 多不影響**：`TIMESTAMPTZ` NULL 不佔額外空間（PostgreSQL bitmap），且查詢時 `WHERE provider_event_at IS NOT NULL` 會走索引

#### 寫入邏輯

##### Webhook handler 解析時序

```mermaid
flowchart TD
    A[webhook 進來] --> B[寫 webhook_inbox<br/>received_at=nowUtc]
    B --> C{decrypted_payload<br/>含哪些時間欄位?}
    C -->|信用卡 AuthDay+AuthTime| D[parsePayUniAuthDayTimeToUtc<br/>→ provider_paid_at]
    C -->|ATM 無時間欄位| E[provider_paid_at=NULL<br/>稍後 reconcile 補]
    C -->|愛金卡/LinePay PayTime| F[parsePayUniDateTimeToUtc<br/>→ provider_paid_at]
    D --> G[寫入 payment_transactions]
    E --> G
    F --> G
    G --> H[更新 webhook_inbox.provider_event_at]
```

##### Reconcile 補登 ATM PaymentDay

ATM webhook 不帶付款時間（§10.2 已確認）。`PendingPaymentReconcileService` 在執行 query API 後，從 response 抽出 `PaymentDay` 補登：

```java
if (payment.paymentType() == ATM && payment.providerPaidAt() == null) {
    String paymentDay = response.getPaymentDay();  // "yyyy-MM-dd HH:mm:ss"
    LocalDateTime paidAt = DateTimeUtils.parsePayUniDateTimeToUtc(paymentDay);
    if (paidAt != null) {
        repository.updateProviderPaidAt(payment.tradeNo(), paidAt);
    }
}
```

#### EstimatedRefundDate 的跨層契約

`EstimatedRefundDate` 的處理契約：

1. payment-service 在收到 webhook 時用 `parsePayUniDateTimeToUtc` 解析後存入 `payment_refunds.provider_estimated_refund_at`
2. gRPC response 回傳 `epoch_millis` 字串（與其他時間欄位一致）
3. Gateway 將其轉為 ISO 8601 帶 `+08:00` 顯示給前端

### 10.6 與其他章節的關係

| 章節 | 關係 |
|------|------|
| §2A.5 ATM 取號保護 | **依賴**：`expire_at` 必須取自 PayUni `ExpireDate` 並經 §10.3 的 parser 轉 UTC |
| §2A.6 重複付款處理 | **依賴**：`provider_paid_at` 欄位讓對帳時能回溯真實時間軸 |
| §3.1 payment_transactions schema | 新增 `provider_create_at` / `provider_paid_at` 欄位 |
| 06 物流設計書 | D4/D5/D6 物流相關修正落地於 06 設計書（本章節僅列清單） |
| 11 PayUni Java Client 設計書 | D1 PaymentClient 修正落地於 11 設計書 |
| [Java 微服務開發規範 §2](../開發規範/Java%20微服務開發規範.md) 時間欄位規範 | 本章節是該規範的「PayUni 邊界擴充」 |

---

## 11. PayUni 互動 Audit Logging（payuni_audit_logs 專表）

### 11.1 稽核需求與範圍

金流爭議發生時必須能完整回溯「我方送了什麼參數給 PayUni、PayUni 回了什麼、耗時多久、驗簽／解密是否通過」。保留期對齊爭議追溯下限（信用卡爭議 ＋ 對帳緩衝 ＋ 跨年月結，實務下限 **7 年**）。

稽核資料寫**專用表 `payuni_audit_logs`**（payment、shipping 兩服務同構各一張），原文**不截斷**。

🔴 **不得把 PayUni 互動稽核寫回 `payment_logs`**，三個理由都是硬的：

| 理由 | 具體後果 |
|---|---|
| 保留期衝突 | `payment_logs` 是業務日誌（狀態機／救援／衝突），保留期 180/365 天；稽核要 7 年。同一張表兩種保留期無法並存 |
| 截斷即失去稽核原文 | `payment_logs.message` 截斷至 8000 字元防爆 row，而 PayUni 原始回應（加密 `EncryptInfo` 全文、v2.0 `Result[]` 陣列、HTML 版回應）常超過 |
| 欄位語意扁平 | 出向 request／response／error 全塞進單一文字欄，無法依 `api_path`／`outer_status`／`http_status` 結構化查詢，`trade_no` 也抽不完整 |

`payment_logs` 因此回歸純業務日誌（§3.2）。

### 11.2 payuni_audit_logs 表結構

本表由 `V3__create_payuni_audit_logs.sql` 建立（append-only 新增，V1 凍結不動）。shipping-service 為同構表（[06 §10](./06-物流微服務（Shipping%20Service）詳細設計書.md#10-payuni-互動-audit-loggingpayuni_audit_logs)）。

| 欄位 | 型別 | 說明 |
|---|---|---|
| id | BIGSERIAL | PK，內部稽核表（§7 BIGSERIAL） |
| direction | VARCHAR(10) NOT NULL | `OUTBOUND` / `INBOUND`，CodeMeta `PayUniAuditDirection`；寫入前 `CodeMetaValidator.validate(PayUniAuditDirection.class, ...)` |
| trade_no | VARCHAR(32) NULL | 出向自 request params 的 `MerTradeNo`；grant_query 等按日期查詢 API 天生無單號 → null。建 index |
| refund_no | VARCHAR(32) NULL | 退款相關呼叫填。建 partial index（`WHERE refund_no IS NOT NULL`） |
| mer_id | VARCHAR(20) NULL | PayUni 商家編號（`HAWK<account_id>`） |
| api_path | VARCHAR(100) NOT NULL | 如 `/api/trade/grant` |
| request_params | JSONB NULL | 出向 = 業務參數明文（憑證類欄位如 `CustomerToken` 必遮罩）；入向 = 解密後 payload |
| response_raw | TEXT NULL | 原始回應 body 全文（含加密 `EncryptInfo` 原文、HTML 標籤全文）；二進位檔不落內容。**無截斷** |
| response_decrypted | JSONB NULL | 解密後內層。key 為 PayUni 原生 PascalCase（`MerID`/`TradeNo`…），見 §11.2.1 opaque passthrough 例外 |
| outer_status | VARCHAR(20) NULL | 外層 `Status`；parse/decrypt/hash 驗證失敗記為特殊值 `PARSE_ERROR` / `HASH_INVALID`（非 error 路徑） |
| outer_message | TEXT NULL | 外層 `Message`；二進位下載在此記 metadata（型別/bytes） |
| error_code | VARCHAR(50) NULL | 傳輸層錯誤（IOException / 非 200 / EMPTY response） |
| error_message | TEXT NULL | 傳輸層錯誤訊息 |
| http_status | INTEGER NULL | HTTP 狀態碼（IO 錯誤時 0） |
| duration_ms | BIGINT NULL | 呼叫耗時 |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | MetaObjectHandler 自動填充（§21）。建 index |

**索引**：`idx_payuni_audit_trade_no (trade_no)`、`idx_payuni_audit_refund_no (refund_no) WHERE refund_no IS NOT NULL`（partial，該欄多數為 NULL）、`idx_payuni_audit_created_at (created_at)`（清理排程用）。api_path 低基數、無查詢回報需求，不建索引。

使用的 Enum：`PayUniAuditDirection` → 詳見 [代碼管理（CodeMeta）/內部技術碼.md#PayUniAuditDirection](../代碼管理（CodeMeta）/內部技術碼.md#payuniauditdirection)。

#### 11.2.1 response_decrypted 的 PascalCase opaque passthrough 例外

`response_decrypted` 的 JSONB key **刻意保留 PayUni 原生 PascalCase**（`MerID` / `TradeNo` / `TradeStatus`…），**不套用 [Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md)「JSONB 內容 key 一律 camelCase」的慣例**。理由：本欄是「忠實稽核 PayUni 回應原文」，任何 key 轉換都會讓稽核值與 PayUni 實際回傳不一致，喪失爭議舉證力。TypeHandler 以裸 `new ObjectMapper()`（不套命名策略）原樣寫入，明確標註為 **opaque passthrough 例外**——此為該 camelCase 規則的白名單特例，僅適用本欄。

#### 11.2.2 憑證遮罩

`request_params` 與 `response_decrypted` 中的憑證類欄位（`CustomerToken`、`Hash`、卡號等機敏值）寫入前必遮罩（保留前後各 2~4 碼、中段以 `*` 取代）。信用卡機敏資訊由 UPP 瀏覽器 Form POST 直送 PayUni、不經本 client，**從不落庫**（PCI 正確設計，見 [11 §5](./11-PayUni%20Java%20Client（payuni-java-client）詳細設計書.md)）。

### 11.3 payuni-java-client 稽核 hook（onExchange）

出向稽核由 payuni-java-client 的 `PayUniRequestLogger` 介面承載，本次**替換**為單一聚合 hook（完整定義見 [11 §5.5](./11-PayUni%20Java%20Client（payuni-java-client）詳細設計書.md#55-payunirequestlogger-稽核-hookonexchange)）：

- 介面只有單一方法 `void onExchange(PayUniExchangeContext ctx)`（default no-op）。🔴 **不得回到 `onRequest` / `onResponse` / `onError` 三方法形式**：那個形式的 hook 在**解密前**觸發，拿不到解密後的內層內容，稽核會缺一半。唯一實作者是 payment 的 `PayUniAuditLogger`。
- 觸發語意：`callApi` / `callAgentApi` 於 `parseResponse` **之後**觸發；`callApiHtml` 於 body 讀取後觸發（無 parseResponse）；所有錯誤路徑（timeout / 非 200 / body null / parse/decrypt/hash 失敗）亦觸發。實作採「可變 context 累加器 + finally 單次觸發」保證**恰好一次**。
- parse/decrypt/hash 失敗：沿用 `parseResponse` 既有的特殊 `outer_status` 回傳語意（`PARSE_ERROR` / `HASH_INVALID`，吞例外不走 error 路徑），於 finally 統一觸發，raw body + 特殊 outer_status 一併帶給 hook；並把**解密後內層一併帶給 hook**（舊設計 hook 於解密前觸發、拿不到解密內容——此為本次稽核完整性改善點）。
- 共用模組**不依賴 payment-service DB layer** 原則不變：payuni-java-client 提供純函式 helper（context → 中性欄位 record，含遮罩規則），不內建 JDBC writer；各服務 logger 為 persist-only thin 實作。

### 11.4 PayUniAuditLogger 實作（payment-service）

`PayUniAuditLogger` 為 `payuni_audit_logs` 的**唯一寫入入口**，同時承載出向與入向：

```java
@Component
public class PayUniAuditLogger implements PayUniRequestLogger {
    private final PayUniAuditLogMapper mapper;   // 寫 payuni_audit_logs（非 payment_logs）

    // 出向：hook 回呼 → direction=OUTBOUND
    @Override public void onExchange(PayUniExchangeContext ctx) { /* map → insert */ }

    // 入向（payment-service 內部方法，觸發點在自家 controller / consumer）
    /** 異步 webhook 解密後寫稽核（在 idempotency check 之前） */
    public void recordInboundWebhook(String tradeNo, String providerTradeNo,
                                     Map<String, String> decryptedPayload);   // direction=INBOUND
    /** 同步 return Form POST 解密後寫稽核 */
    public void recordInboundReturn(String tradeNo, String providerTradeNo,
                                    Map<String, String> decryptedPayload);     // direction=INBOUND
}
```

- 出向：`onExchange` 收 payuni-java-client 傳來的 `PayUniExchangeContext`，經純函式 helper 映射為中性欄位 record 後 INSERT（`direction=OUTBOUND`，`trade_no` 自 `request_params.MerTradeNo`）。
- 入向：`recordInboundWebhook` / `recordInboundReturn` 改寫 `payuni_audit_logs`（`direction=INBOUND`），`request_params` 存解密後 payload、`trade_no`/`mer_id` 照填、`api_path` 記入向路徑（如 `/webhook/payuni/notify`、`/payment/return`）。
- 寫入時機不變：webhook 於 `PayUniWebhookEnvelope.parse` 之後、idempotency check **之前**（即使被 dedup hit 仍留稽核，可觀察 PayUni 重投行為）；return 於 `cryptoService.decrypt` 成功後、`saveReturnInbox` 之前。
- **停用**：`payment_logs` 的 `PAYUNI_REQUEST` / `PAYUNI_RESPONSE` / `PAYUNI_ERROR` / `PAYUNI_INBOUND_WEBHOOK` / `PAYUNI_INBOUND_RETURN` 五型不再寫入；`PayUniAuditLogger` 不再依賴 `PaymentLogMapper`。

### 11.5 責任邊界（三表互補，非重複）

| 維度 | `payuni_audit_logs`（本表） | `payment_webhooks`（payment-service） | `webhook_inbox`（payuni-webhook-service） |
|---|---|---|---|
| 領域 | PayUni ↔ 我方對話完整稽核（出向+入向，單表還原時序） | payment 端業務 inbox 狀態機 | webhook-service 入口收單/去重/入隊 |
| Idempotency | ❌ append-only（PayUni retry 也記，含解密/驗簽失敗） | ✅ idempotency_key UNIQUE 防業務重複處理 | ✅ event_key UNIQUE 收單去重 |
| 保留期 | **7 年** | 90 天 | 依 webhook-service 策略 |
| 內容完整性 | 原文全文、不截斷、含加密原文/HTML | 業務欄位 + process_status | 收單快照 |
| 查詢場景 | 「我方對 PayUni 全部對話歷史」（爭議舉證） | 「這筆訂單收到幾次通知、業務處理結果」 | 「入口收了什麼、去重結果」 |

- **與 `payment_webhooks` 的重複是 by design**：查詢場景不同，稽核表 self-contained（不 join）；且業務狀態機只動 `payment_webhooks`，不需同步稽核表。
- **與 `webhook_inbox` 非重複**：兩者是不同服務、不同稽核視角——`webhook_inbox` 是**入口收單視角**（webhook-service 收到什麼），`payuni_audit_logs`（INBOUND）是**下游消費視角**（payment-service 解密後看到什麼）。互補而非替代。

### 11.6 一致性、交易與補償設計

- **單一寫入路徑**：`payuni_audit_logs` 的**唯一寫入入口**為 `PayUniAuditLogger`（單一 class），無第二條寫路徑。
- **append-only**：只 INSERT、不 UPDATE、不 DELETE（清理由保留期排程統一執行）。
- **寫入失敗不阻斷主流程**：INSERT 以 try/catch 包覆，失敗僅 `ObsLogger.warn`，**全吞不拋**，PayUni 主流程（付款/退款/撥款）不受稽核落庫影響。
- **不引入補償任務基礎設施**：稽核落庫失敗即接受遺失該筆稽核（副本性質、非業務正確性來源），**不建 reconciliation / backfill / outbox / retry 任務**。此為刻意設計：稽核表是旁路副本，其完整性不足以正當化補償基礎設施的維運成本（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)）。

### 11.7 時序圖（出向 + 入向）

```mermaid
sequenceDiagram
    autonumber
    participant USR as 使用者
    participant PU as PayUni
    participant GW as Gateway
    participant WH as payuni-webhook-service
    participant MQ as RabbitMQ
    participant PAY as payment-service
    participant CLI as payuni-java-client
    participant AUD as payuni_audit_logs
    participant INBOX as payment_webhooks

    rect rgb(245,245,220)
    Note over PAY,AUD: 出向：我方 → PayUni（單一 onExchange hook）
    PAY->>CLI: callApi /api/atm
    CLI->>PU: POST（加密 Form）
    PU-->>CLI: response
    CLI->>CLI: parseResponse（含解密/驗簽；失敗→特殊 outer_status）
    CLI->>PAY: onExchange(ctx)（finally 恰好一次）
    PAY->>AUD: INSERT direction=OUTBOUND<br/>(request_params/response_raw/response_decrypted/outer_status...)
    end

    rect rgb(220,245,245)
    Note over PU,AUD: 入向 A：異步 webhook
    PU->>GW: POST /webhook/payuni/notify
    GW->>WH: proxy（入 webhook_inbox）
    WH->>MQ: publish payuni.events
    MQ->>PAY: PayUniWebhookConsumer
    PAY->>PAY: PayUniWebhookEnvelope.parse + 抽 trade_no
    PAY->>AUD: recordInboundWebhook<br/>direction=INBOUND（dedup 之前 — retry 也記）
    PAY->>INBOX: idempotency check + 寫業務狀態
    end

    rect rgb(245,220,245)
    Note over USR,AUD: 入向 B：同步 return
    USR->>PU: 信用卡介面填完
    PU->>GW: POST /payment/return Form POST
    GW->>PAY: PaymentReturnController
    PAY->>PAY: cryptoService.decrypt(EncryptInfo)
    PAY->>AUD: recordInboundReturn<br/>direction=INBOUND
    PAY->>INBOX: idempotency check + 寫業務狀態
    end
```

### 11.8 元件職責

| 元件 | 職責 |
|------|---------|
| `PayUniRequestLogger`（payuni-java-client） | 稽核 hook 介面，單一方法 `onExchange(PayUniExchangeContext)`，default no-op |
| `PayUniExchangeContext`（payuni-java-client） | 聚合 context record（見 11 號設計書 §5.5） |
| `PayUniHttpClient`（payuni-java-client） | 三個入口共用「可變 context 累加器 ＋ finally 單次 `onExchange`」。🔴 parse／decrypt／hash 失敗必須回**特殊 `outer_status`**，不得走 error 路徑——否則失敗案例就沒有稽核留痕 |
| `PayUniAuditLogger`（payment-service） | 實作 `onExchange` 寫 `payuni_audit_logs`；入向 webhook／return 同樣寫本表（`direction=INBOUND`）。**不得依賴 `PaymentLogMapper`** |
| `PayUniAuditLogMapper` ＋ Entity ＋ JSONB TypeHandler | 新表的持久化層；`response_decrypted` 走 PascalCase opaque passthrough TypeHandler（見 §11.2.1） |
| `DataCleanupScheduler` | `payuni_audit_logs` 的 7 年清理項（`payment.cleanup.payuni-audit-retention-days`，預設 2555） |

### 11.9 保留期策略

| 表 | INFO/WARN | ERROR | 配置 key |
|---|---|---|---|
| `payuni_audit_logs` | **7 年（2555 天）** | 同左 | `payment.cleanup.payuni-audit-retention-days` |
| `payment_logs` | 180 天 | 365 天 | `payment.cleanup.log-retention-days` |
| `payment_webhooks` | 90 天 | 90 天 | `payment.cleanup.webhook-retention-days` |

`payuni_audit_logs` 7 年保留由既有 `DataCleanupScheduler`（每日 02:30）擴充一個清理項達成，**不新增獨立 scheduler**。`payment_logs` 移除 PAYUNI_* 稽核後回歸業務日誌，維持 180/365 天不變。

### 11.10 可驗證的不變式

**出向留痕**：每一次對 PayUni 的呼叫都必須有一筆 `direction='OUTBOUND'` 的 row，且 `response_raw` 為**未截斷**原文、`response_decrypted` 保有 PascalCase 原鍵：

```sql
SELECT direction, api_path, outer_status, http_status, duration_ms, created_at
FROM payuni_audit_logs
WHERE trade_no = 'O123456' AND direction = 'OUTBOUND'
ORDER BY id DESC LIMIT 4;
-- 可看到 /api/atm 出向 request_params（NotifyURL/ExpireDate/TradeAmt）+ response_raw 全文（不截斷）+ response_decrypted（PascalCase）
```

**完整對話還原**：同一筆交易的出向與入向必須能按時序串成完整對話（典型順序：`OUTBOUND /api/atm` → `INBOUND /payment/return` → `INBOUND /webhook/payuni/notify` → `OUTBOUND /api/trade/query`）：

```sql
SELECT direction, api_path, outer_status, created_at
FROM payuni_audit_logs
WHERE trade_no = 'O123456'
ORDER BY created_at;
```

🔴 **失敗路徑不得吞稽核**：parse／decrypt／hash 失敗時仍必須有一筆 row，`outer_status` 為對應的失敗值（如 `HASH_INVALID`）且 `response_raw` 保有原文。失敗案例正是爭議時最需要回溯的那一種。

---

## 12. PAYMENT_UOP eventKey 格式

> 本章節對應 payuni-webhook-service 的 `EventKeyBuilder`，因直接決定 §2A.6 能否被觸發，規格記錄於本設計書。

### 12.1 格式

```
{merId}:{merTradeNo}:{tradeNo}:{tradeStatus}
```

第三段是 `payload.TradeNo`（PayUni 內部 serial），它是 PayUni 對每一筆獨立交易的唯一識別。

🔴 **第三段不得改用 `MerTradeNo` 以外的推導值，也不得省略**：同一 order 的信用卡與 ATM 兩筆獨立 PayUni PAID 通知，其 `MerTradeNo`／`TradeStatus`／`TradeAmt` 三者皆相同 ⇒ 少了 `TradeNo` 這一段，兩筆的 eventKey 會完全相同，後到的那筆被 `webhook_inbox` UNIQUE 約束擋下，**§2A.6 重複付款偵測永遠不會被 ATM 那一筆觸發**。

### 12.2 去重行為

| 場景 | 期望 |
|---|---|
| 信用卡通知 retry（PayUni 重投）| eventKey 相同 → 攔下 |
| ATM 通知 retry | eventKey 相同 → 攔下 |
| 同 order 信用卡 + ATM 都成功（§2A.6）| `TradeNo` 不同 → 兩筆都進 |

### 12.3 為什麼是 `TradeNo` 而不是 `PaymentType`

`PaymentType` 雖能區分信用卡（1）與 ATM（2），但 MerTradeNo 序列化後**同一 order 同一 type 也可能有多筆獨立 PayUni 交易** ⇒ 以 type 推導會再度誤攔。`TradeNo` 是 PayUni 的權威 unique key，是唯一可靠的 dedup 依據。

---

## 附錄 A：本服務使用的 Enum

完整定義統一維護於 [代碼管理（CodeMeta）](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/README.md)，以下僅列出本服務涉及的 Enum：

🔴 **本表只列 Enum 名與定義來源，不複製值**：值的 SSOT 在 CodeMeta `.md`，複製一份到這裡必然漂移（而漂移的那一份不會告訴讀者它漂了）。

| Enum | 定義來源 |
|------|---------|
| PaymentTransactionStatus | [業務狀態碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務狀態碼.md) |
| RefundStatus | [業務狀態碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務狀態碼.md) |
| PaymentMethod | [業務類型碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/業務類型碼.md) |
| PaymentEventType | [內部技術碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/內部技術碼.md) |
| GrpcResponseCode | [錯誤碼](../代碼管理%EF%BC%88CodeMeta%EF%BC%89/錯誤碼.md)（本服務使用的碼見 §4.6） |

