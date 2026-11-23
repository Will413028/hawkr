# B2 payment 盤點（Java payment-service → Go `internal/modules/payment`）

基準：repo `Will413028/hawkr` branch `dev` HEAD `df21e0d8a`，2026-11-23。全部由讀原始碼得出；標「推論」者未執行驗證。路徑除另註外相對 repo 根，`PS=backend/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment`。

決定編號：本檔的 D1–D6 指計畫 `2026-11-23-b2-payment.md` 的決定（D1 收款模式、D3 買家付款介面、D4 退款並行、D5 付款成功但訂單已不可付、D6 轉帳觸發）；「單體規則 5」指外部 provider 不在 transaction 內呼叫。

**開頭的重要事實（與計畫文字不一致）**：計畫 B2 行（`backend/docs/plans/2026-09-24-go-modular-monolith.md:345`）寫「物流財務與責任判定先留在 Java，B10 刪除」，但同檔 `:393`（Will 2026-11-23）改為「不改寫、直接刪」，而且**已經刪了**：commit `5883f499e`、`125db5844`、`28a120564`（皆 2026-11-23）刪掉 Java payment 的 logistics 套件、`LogisticsFinanceGrpcService`、`FulfillmentResponsibilityGrpcService`、相關 consumer、gateway `routes_logistics.go`、proto 的兩個 service。現存 116 個 .java 檔裡沒有任何 logistics／responsibility 程式碼（`rg -il 'logistics|responsib' $PS` 無結果）。只剩 Flyway V9–V16 建的物流表（沒有程式讀寫，見表 4）。計畫 `:345` 那一行已經過時。

計數指令：
- 檔案數：`find backend/services/java/payment-service/src -name '*.java' | wc -l` → 116（main 79、test 37，其中 `CryptoTest` 是 main() 手動工具，沒有 @Test）
- 測試方法：`rg -c '@(Test|ParameterizedTest)\b' backend/services/java/payment-service/src/test | awk -F: '{s+=$2}END{print s}'` → 193

---

## 1. 對外 endpoint

計數：`rg -n 'payments\.(GET|POST|PATCH)|admin\.GET|r\.(GET|POST)' backend/gateway/internal/router/routes_payment.go backend/gateway/internal/router/routes_webhook_simulator.go`（16 條）＋ payment-service 自己的 `@PostMapping`（`rg -n '@(Get|Post|Put|Delete|Request)Mapping' $PS` → 1 條）。openapi：`rg -n '^  /\S*(payment|grant|payuni|webhook)' backend/api/openapi/openapi.yaml`。Client：`rg -n -e '/payments' -e payment_transactions -e payment_refunds -e admin/grants -e payment/redirect -e payment/return -e apple_pay -e status_syncs website admin buyer-app seller-app`（排除 test、gen）。

payment-service 只有一個 REST controller；其餘全部是 gateway（Gin）→ gRPC `PaymentService`。

| # | Method 路徑 | gateway 註冊 / handler | 下游 RPC | openapi | client 呼叫者 | 提議去向 |
|---|---|---|---|---|---|---|
| 1 | POST /api/v1/app/payments | `gateway/internal/router/routes_payment.go:18` → `handler/payment.go:258` CreatePayment | CreatePayment | `openapi.yaml:13438` | website `lib/orders/repo.ts:1276`；buyer-app `features/order/data/payment_repository.dart:46`、`features/cart/presentation/widgets/checkout_payment_sheet.dart:623` | 保留→Go：Connect `PaymentService.CreatePayment`（buyer，步驟 5）：寫 payment 列與 job，job 經 provider port 建 Stripe PaymentIntent（帶 `transfer_group`，D1）；client_secret 或 checkout URL 依 D3 回給 client，取代 PayUni 表單；回傳時機見 Open items |
| 2 | GET /api/v1/app/payments/{trade_no} | `routes_payment.go:20` → `payment.go:340` | GetPayment | `:13702` | website `repo.ts:1316`（輪詢）；buyer-app `payment_repository.dart:71` | 保留→Go：Connect `PaymentService.GetPayment`（buyer 歸屬檢查，他人回 NOT_FOUND） |
| 3 | GET /api/v1/app/payments/orders/{order_id}/history | `routes_payment.go:22` → `payment.go:376`（賣家身分時改打 GetSellerPaymentHistory，`payment.go:390`） | GetPaymentHistory／GetSellerPaymentHistory | `:13586` | buyer-app `payment_repository.dart:90`；seller-app `features/order/data/order_repository.dart:112` | 保留→Go：Connect 兩支：buyer 付款嘗試歷程、seller 版（跨店 NOT_FOUND、遮蔽 buyer PII）（預設，可推翻：單一 PaymentIntent 可重試，歷程列同一筆付款的失敗嘗試；是否需要見 Open items） |
| 4 | POST /api/v1/app/payments/{trade_no}/status_syncs | `routes_payment.go:24` → `payment.go:436` | SyncPaymentStatus（PayUni Query，tx 內） | `:13866` | website `repo.ts:1330`、`lib/payments/usePaymentResolution.ts:48` | 保留→Go：Connect `SyncPaymentStatus`：讀本地狀態並排 job 重讀 PaymentIntent（外部呼叫在 job，不在 transaction 內），只補 webhook 延遲 |
| 5 | PATCH /api/v1/app/payments/{trade_no} | `routes_payment.go:26` → `payment.go:472` | ClosePayment | `:13702`（同路徑） | 無 client 呼叫（推論：rg 無命中） | 刪除：無 client 呼叫（推論：rg 無命中）；取消未完成付款由模組 API `CancelPaymentForOrder`（排 job 取消 PaymentIntent，計畫步驟 4）承接，不另開外部路由 |
| 6 | GET /api/v1/app/payments/refunds/{refund_no} | `routes_payment.go:28` → `payment.go:507` | GetRefund | `:13649` | 無 client 呼叫（推論） | 刪除外部路由（退款狀態由 aftersales／ordering 呈現）；模組 API 保留給 aftersales |
| 7 | GET /api/v1/app/payments/{trade_no}/refunds | `routes_payment.go:29` → `payment.go:542` | GetRefundsByTradeNo | `:13811` | 無 client 呼叫（推論） | 刪除外部路由 |
| 8 | POST /api/v1/app/payments/apple_pay/validate_merchant | `routes_payment.go:31` → `payment_apple_pay.go:55` | ValidateApplePayMerchant（mTLS 打 Apple） | `:13512` | website `lib/payment/apple-pay.ts:345` | 刪除：Apple Pay merchant validation 由 Stripe 處理，D3 兩案（Payment Element、Checkout）都不需要自建 |
| 9 | GET /api/v1/admin/payment_transactions | `routes_payment.go:36` → `handler/admin_payment.go:136` | ListPaymentTransactionsForAdmin | `:5843` | admin `src/app/admin/anomalies/page.tsx:756` | 保留→Go：Connect `admin.payment.v1alpha1` 加 `ListPayments`（宣告權限，步驟 5） |
| 10 | GET /api/v1/admin/payment_transactions/{trade_no} | `routes_payment.go:37` → `admin_payment.go:203` | GetPaymentTransactionForAdmin | `:5943` | admin（詳情，推論同頁） | 保留→Go：Connect `admin.payment.v1alpha1` 加 `GetPayment`（admin 版）（宣告權限，步驟 5） |
| 11 | GET /api/v1/admin/payment_refunds | `routes_payment.go:38` → `admin_payment.go:274` | ListRefundsForAdmin | `:5719` | admin `anomalies/page.tsx:913` | 保留→Go：Connect `admin.payment.v1alpha1` 加 `ListRefunds`（宣告權限，步驟 5） |
| 12 | GET /api/v1/admin/payment_refunds/{refund_no} | `routes_payment.go:39` → `admin_payment.go:334` | GetRefundForAdmin | `:5802` | admin（推論） | 保留→Go：Connect `admin.payment.v1alpha1` 加 `GetRefund`（admin 版）（宣告權限，步驟 5） |
| 13 | GET /api/v1/admin/grants | `routes_payment.go:40` → `admin_payment.go:386` | ListGrantsForAdmin | `:2464` | admin `anomalies/page.tsx:1051` | 保留→Go：Connect `admin.payment.v1alpha1` 加 `ListTransfers`：撥款頁改為轉帳列表（步驟 5；D1 下對應 PayUni grant 的是 Transfer） |
| 14 | POST /webhook/payuni/notify | `routes_payment.go:53` proxy → payuni-webhook-service | （MQ `payuni.events` → payment consumer） | 無 | PayUni（外部） | 刪除；改 `POST /api/webhooks/stripe`（單體 `internal/httpapi/payment/webhook.go` 已有收款帳戶用的，擴充事件類型），先寫 `inbox.events`、event id 去重 |
| 15 | GET /payment/return | `routes_payment.go:62` → `payment.go:624` | GetPaymentReturnPage | `:20637` | website `app/[locale]/(commerce)/payment/return/page.tsx`（頁面同名，另走 `/app/payments/{trade_no}` 輪詢） | 刪除（PayUni 導回頁；Stripe 用 return_url 回 website，結果靠 webhook） |
| 16 | POST /payment/return | `routes_payment.go:63` → `payment.go:659` | HandlePaymentPageReturn（解密＋更新狀態） | `:20637` | PayUni 瀏覽器 POST | 刪除（計畫 `:346` PaymentReturn 類） |
| 17 | GET /payment/redirect?id= | `routes_payment.go:69` → `payment.go:578` RedirectToPayUni | GetPaymentRedirectForm | `:20598` | App WebView（URL 由 CreatePayment 回傳） | 刪除（PaymentPageService／RedirectFormStore 類） |
| 18–20 | GET /dev/webhook-simulator、POST …/send、GET …/lookup | `routes_webhook_simulator.go:26-28`（只 local/dev） | proxy → payuni-webhook-service | 無 | 開發者瀏覽器 | 刪除（計畫 `:355`）；Stripe 用 `stripe trigger`／`stripe listen` |
| 21 | POST /payment/return（service 自己，:9924） | `$PS/adapter/http/PaymentReturnController.java:73` | 直接處理 | 無 | **沒有 gateway 路由轉到它**（`rg -n 'ServicePayment\b' backend/gateway --glob '!*_test.go'` 只有常數定義）；只有 PayUni ReturnURL 直接指向 service 時才會被打到 | 刪除（計畫已列）；推論目前是死碼 |

---

## 2. gRPC server 與 client

計數：`rg -n ',payment-service,' backend/docs/plans/2026-09-24-go-modular-monolith/rpc-servers.csv | rg -v 'removed: seller-managed'` → 33 列（29 PaymentService＋4 MembershipBillingService）；proto 實際 29 支（`rg -n '^\s*rpc ' backend/services/java/shared/java-proto/src/main/proto/payment/payment.proto`，與 `backend/api/proto/payment/payment.proto` 相同，`diff -q` 無差異）；呼叫者 `rg -n 'PaymentService' rpc-calls.csv`、`rg -n 'paymentStub|stub\.\w+\(' backend/services/java --glob '!payment-service/**'`、`rg -l 'PaymentServiceClient|paymentpb\.' backend --glob '*.go'`。

### 2a. server（`$PS/adapter/grpc/PaymentGrpcService.java`）

| # | RPC | 實作行 | csv 行 | 實際呼叫者（程式碼行） | 提議 disposition |
|---|---|---|---|---|---|
| 1 | CreatePayment | :304 | rpc-servers:425 | gateway `grpc/payment_client.go:54` | 保留→Go：模組 API `CreatePayment`＋Connect handler；Stripe adapter |
| 2 | GetPayment | :407 | :426 | gateway `:67` | 保留→Go：Connect handler（buyer） |
| 3 | GetPaymentHistory | :433 | :427 | gateway `:84` | 保留→Go：Connect handler（預設，可推翻；見表 1 #3） |
| 4 | GetSellerPaymentHistory | :480 | :428 | gateway `:97`（`handler/payment.go:390`） | 保留→Go：Connect handler（seller，預設，可推翻；見表 1 #3） |
| 5 | GetPublicPaymentStatus | :532 | :429 | gateway client `:173` 有包裝，**但沒有任何 handler 呼叫它**（`rg -n 'GetPublicPaymentStatus' backend/gateway --glob '!*_test.go' --glob '!*.pb.go'` 只有 client 定義） | 刪除（沒有呼叫者） |
| 6 | SyncPaymentStatus | :631 | :430 | gateway `:113` | 保留→Go：Connect handler：讀本地狀態並排 job 重讀 PaymentIntent（表 1 #4） |
| 7 | ClosePayment | :665 | :431 | gateway `:129`（PATCH 無 client） | 刪除：無 client 呼叫；取消未完成付款由 #8 承接 |
| 8 | ClosePaymentByOrderNo | :733 | :432 | order `order/grpc/client/PaymentGrpcClient.java:184` | 保留→Go：模組 API `CancelPaymentForOrder`：取消未完成 PaymentIntent（排 job）；B8 前由 `internal/legacy/paymentserver` 給 Java order（步驟 4），B8 改 in-process |
| 9 | GetActivePaymentStatusByOrderNo | :765 | :433 | order `PaymentGrpcClient.java:103` | 保留→Go：模組 API 讀付款列；B8 前由 legacy paymentserver 給 Java order（步驟 4） |
| 10 | CreateRefund | :883 | :434 | aftersales `aftersales/grpc/client/PaymentGrpcClient.java:100`（呼叫點 `ReturnReceiptService.java:521`、`RefundTaskWorker.java:211`、`SellerCancelService.java:150,298`、`AfterSaleResolutionService.java:469`）；order `PaymentGrpcClient.java:164`（`worker/AutoRefundWorker.java:110`） | 保留→Go：模組 API `RequestRefund`：tx 內鎖付款列、檢查累計上限、寫退款列（呼叫端冪等鍵）並排 job，並行退款只守累計上限（D4）；B2 期間由 legacy paymentserver 給 Java 用 |
| 11 | GetRefund | :944 | :435 | gateway `:147`；aftersales `:144` | 保留→Go：模組 API 讀退款列；legacy paymentserver 給 aftersales（步驟 4） |
| 12 | GetRefundsByTradeNo | :978 | :436 | gateway `:163` | 刪除外部；模組內查詢 |
| 13 | GetPaymentReturnPage | :575 | :437 | gateway `:183` | 刪除（PayUni） |
| 14 | GetPaymentRedirectForm | :606 | :438 | gateway `:192` | 刪除（PayUni） |
| 15 | GrantPayment | :691 | :439 | order `PaymentGrpcClient.java:122`（`worker/GrantPaymentWorker.java:152`） | 保留→Go：模組 API：寫 `transfers` 列並排 job，Transfer 帶 `source_transaction`、訂單完成後建立（D1、D6）；B2 期間由 legacy paymentserver 給 Java order 的 `GrantPaymentWorker`，B8 改 in-process |
| 16 | GetRefundPageUrl | :1095 | :440 | aftersales `:116`（`RefundPageService.java:90`） | 刪除（PayUni ATM 頁面版退款） |
| 17 | BatchGetProviderStatusByOrderIds | :1012 | :441 | order `PaymentGrpcClient.java:192` | 保留→Go：模組 API 批次讀付款列；legacy paymentserver 給 Java order（步驟 4），B8 改 in-process |
| 18 | BatchGetRefundProviderStatusByCaseIds | :1033 | :442 | aftersales `:151` | 保留→Go：模組 API 批次讀退款列；legacy paymentserver 給 aftersales（步驟 4），B10 改 in-process |
| 19 | BatchGetRefundProviderStatusByOrderIds | :1054 | :443 | order `PaymentGrpcClient.java:202` | 保留→Go：模組 API 批次讀退款列；legacy paymentserver 給 Java order（步驟 4），B8 改 in-process |
| 20 | HandlePaymentPageReturn | :1165 | :444 | gateway `:202` | 刪除（PayUni） |
| 21 | DecryptPayUniResult | :1258 | :445 | aftersales `:131`（`AfterSalesGrpcService.java:565`） | 刪除（PayUni） |
| 22 | ValidateApplePayMerchant | :1839 | :446 | gateway `:246` | 刪除：Apple Pay merchant validation 由 Stripe 處理（D3 兩案皆刪） |
| 23 | AckAtmRefundAccepted | :1311 | :447 | aftersales `:165` | 刪除（PayUni ATM） |
| 24 | ListPaymentTransactionsForAdmin | :1719 | :448 | gateway `:211` | 保留→Go：admin Connect handler `ListPayments` |
| 25 | GetPaymentTransactionForAdmin | :1739 | :449 | gateway `:218` | 保留→Go：admin Connect handler `GetPayment` |
| 26 | ListRefundsForAdmin | :1767 | :450 | gateway `:225` | 保留→Go：admin Connect handler `ListRefunds` |
| 27 | GetRefundForAdmin | :1787 | :451 | gateway `:232` | 保留→Go：admin Connect handler `GetRefund` |
| 28 | ListGrantsForAdmin | :1815 | :452 | gateway `:239` | 保留→Go：admin Connect handler `ListTransfers`（轉帳列表，同 #15） |
| 29 | GetPaymentFactsInternal | :827 | :453 | invoice-service（Go）`services/invoice-service/internal/client/payment/client.go:103` | 保留→Go：模組 API `PaymentFacts`；invoice 搬遷前由 legacy paymentserver 提供（步驟 4） |
| 30–33 | MembershipBillingService.Create/Suspend/Restart/EndMembershipSubscription | 不存在 | rpc-servers:371-374 | 無（proto 沒有這個 service；`rg -l MembershipBilling -t java -t go -g '*.proto' backend` 無結果） | 刪除：csv 過時，填 removed（membership 已由 V17 drop 表、程式已刪） |

另：rpc-calls.csv 的 gateway 列行號已漂移（例 `rpc-calls.csv:439` 寫 `payment_client.go:64`，實際 `:54`）。

### 2b. payment-service 當 client

| 呼叫 | 位置 | csv | 用途 | 提議 |
|---|---|---|---|---|
| OrderService.GetOrderForPayment | `$PS/adapter/grpc/client/OrderGrpcClient.java:66`；呼叫點 `PaymentApplicationService.java:141`（createPayment，**在 @Transactional 內**）、`:421`（resolveOrderStoreId） | rpc-calls:725 | 取金額、buyer、mer_id、商品名、訂單狀態 | 保留→Go：payment 模組經組裝點注入的訂單 port 讀（`internal/legacy/grpcclient` 打 Java order），在 tx 外取得，不在 transaction 內呼叫 gRPC；B8 換 in-process（計畫步驟 4） |
| OrderService.SnapshotPaymentInfo | `OrderGrpcClient.java:97`；`PaymentApplicationService.java:277,328`、`AtmReactivationService.java:117`、`DuplicatePaymentService.java:75` | rpc-calls:726 | 把付款方式／ATM 帳號寫回 orders（replicas.csv:16 雙寫） | 刪除：order 需要的付款方式由 `payment.paid` 事件帶（計畫步驟 4）；ATM 欄位消失 |

### 2c. `payuni_mer_id`

- 單體 `backend/internal/legacy/sellerserver/server.go:175-177`：回給 Java 的 `SellerMeta.payuni_mer_id` **一律空字串**。
- 消費者：order `order/checkout/CheckoutService.java:815`（SELLER_MERCHANT → `sellerMeta.getPayuniMerId()`）、`order/service/OrderService.java:362`（RFQ 建單，空字串時 fail-closed）、shipping `shipping/service/OnboardingService.java:170,263,1058`。
- 推論：目前結帳的 `mer_id` 是空字串，CreatePayment 打 PayUni 會失敗（或 RFQ 直接擋下）。也就是說，**付款流程端到端在 dev 上已經壞了**（沒有實測）。
- 提議：SCT（separate charges and transfers，D1）下收款進平台帳戶，結帳不再需要賣家的商店代號：Java checkout 的 `SELLER_MERCHANT` 與 RFQ 建單 `OrderService.java:362` 改看可接單（步驟 6）；`sellerserver` 不再填 `payuni_mer_id`，proto 欄位保留到 B3（shipping `OnboardingService.java:170,263,1058` 仍讀）；賣家的 Stripe account id 只在轉帳 job 由 payment 內部從 `payment.accounts` 查。

---

## 3. 事件（RabbitMQ）

計數：`rg -n 'payment' backend/docs/plans/2026-09-24-go-modular-monolith/events.csv`（扣掉已 removed 與非 payment-service 列，剩 22 列 payment 相關）；程式碼 `rg -n '@RabbitListener' $PS`（5 個 listener）、`rg -n 'ROUTING_KEY_' $PS/infrastructure/mq/PaymentEventPublisher.java`。

Outbox：`PaymentOutboxWriter.enqueueAndDispatch`（`$PS/infrastructure/mq/PaymentOutboxWriter.java:80`）在交易內寫 `outbox_events` 並在 afterCommit 即時投遞；`OutboxEventPublisherScheduler`（`scheduler/OutboxEventPublisherScheduler.java:46`，每 5 秒）補送。

| # | routing key | 方向 | events.csv | 程式碼 | 流程 | 提議 |
|---|---|---|---|---|---|---|
| 1 | payment.success | 產生 → order `order.payment.result` | :164 | `PaymentEventPublisher.java:68`；`PaymentNotifyHandler.handlePaid` `:206` | webhook／導回確認已付 → order 標 PAID、pay_time | 保留→Go：模組事件 `PaymentSucceeded`（同 transaction `events.Publish`）；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 2 | payment.failed | → order | :161 | `:69`；`handleFailed` `:260` | 付款失敗 | 保留→Go：模組事件 `PaymentFailed`；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 3 | payment.expired | → order | :160 | `:70`；`ExpiredTransactionCloseService.closeExpiredInTx` `:60` | 逾期未付 → order 過期 | 保留→Go：模組事件 `PaymentExpired`：River 週期 job 取消逾期的 PaymentIntent（外部呼叫在 job）後同 transaction 發；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 4 | payment.closed | → order | :158 | `:71`；`PaymentCloseService.closePayment` `:55` | 關閉／切換付款方式 | 保留→Go：模組事件 `PaymentClosed`：訂單取消或逾期而取消未完成的 PaymentIntent 後發（Stripe 下沒有切換付款方式，不再有 CLOSED_BY_SWITCH）；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 5 | payment.conflict | → order | :159 | `:72`；`handleConflict` `:315` | 本地已過期但 PayUni 成功 | 刪除：不保留 CONFLICT 狀態與 `payment.conflict`；PaymentIntent 在訂單已不可付後才成功，改由 payment（擁有者，計畫步驟 3）排自動全額退款 job（D5），完成發 `AutoRefundCompleted` |
| 6 | refund.succeeded | → aftersales `aftersales.payment.refund.result`、order `order.refund.result` | :167,:168 | `:73`；`RefundApplicationService.handleRefundSuccess` `:360` | 退款完成 → refund_tasks／case、order refund_total | 保留→Go：模組事件 `RefundSucceeded`：Stripe `charge.refund.updated`／`refund.updated` 的 inbox handler 重讀退款物件後同 transaction 發；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 7 | refund.failed | → aftersales、order | :165,:166 | `:74`；`handleRefundFailure` `:385` | 退款失敗 | 保留→Go：模組事件 `RefundFailed`：同上；B2 到 B8 之間由 `internal/legacy/paymentrelay` 發 RabbitMQ（路由鍵不變，步驟 4）；B8 改 ordering 後 relay 刪除 |
| 8 | payment.rescue.refund_initiated | 自產自消 → `payment.rescue.refund.queue` | :163 | 產生 `DuplicatePaymentService.java:375`、`PendingPaymentReconcileService.publishRescueEvents` `:273`；消費 `mq/RescueRefundConsumer.java:71` | 重複付款或過期後成功 → 自動退款 | 刪除（計畫前提）：重複付款在一張 PaymentIntent 下不會發生；遲到的成功由 webhook handler 排自動退款 job（D5） |
| 9 | payment.rescue.refund_completed | → notification `notification.payment` | :162 | `RescueRefundConsumer.java:244` | 推播通知買家 | 刪除（隨 rescue）：通知改訂閱模組事件 `AutoRefundCompleted`（D5，A8 notification）；A8 晚於 B2 時這段期間沒有通知，寫進 `CARRYOVER.md` 的差異 |
| 10 | payuni.payment.uop | 消費 ← payuni-webhook-service | :170 | `mq/PayUniWebhookConsumer.java:92` | PayUni 付款通知 → `PaymentNotifyHandler` | 刪除；Stripe webhook inbox |
| 11 | payuni.refund.notify | 消費 | :172 | `mq/PayUniRefundNotifyConsumer.java:65` | ATM 退款結果 | 刪除（計畫 `:346`） |
| 12 | payuni.refund.accepted | 消費 | :171 | `mq/PayUniRefundAcceptedConsumer.java:63` | ATM 退款受理 | 刪除 |
| 13 | payuni.unknown.fallback | 消費 | :175 | `mq/UnknownPaymentFallbackConsumer.java:52` | 無法辨識的 webhook，保守標失敗 | 刪除 |
| 14 | payuni.payment.recurring | 消費（csv） | :169 | **不存在**（`rg -n -i 'membership\|recurring' $PS` 無結果） | — | 刪除：csv 過時，已填 removed |
| 15–18 | payment.membership.{buyer,seller}.{success,failed} | 產生（csv） | :189-192 | **不存在** | — | 刪除：csv 過時，已填 removed |
| 19–21 | fulfillment.responsibility.*、logistics.cost.incurred | — | :142-144、:188 | 已刪 | — | 刪除：已填 removed |

另外 `PaymentOutboxExchangeContractTest.shippingRow_goesToShippingExchange` 說明 outbox 曾承載 `hawkr.shipping` exchange（物流事件）；物流刪掉之後已經沒有寫入者（推論）。

---

## 4. 資料表

計數：`rg -n 'CREATE TABLE|DROP TABLE' backend/services/java/payment-service/src/main/resources/db/migration/*.sql` → 建 20 張、V17 drop 4 張 → 現存 16 張；讀寫者：`rg -l '<table>' backend --glob '!**/target/**' --glob '!**/migration/**'`。replicas：`rg -n -i payment backend/docs/plans/2026-09-24-go-modular-monolith/replicas.csv`。

| # | 表 | migration | 主要欄位 | 讀寫者 | 提議 |
|---|---|---|---|---|---|
| 1 | payment_transactions | V1:9（V4 改 account_id→buyer_id） | id(TSID)、trade_no=orderNo+attempt_seq、order_id、attempt_seq、mer_id、buyer_id、payment_type(2001/2002/2003/2004/2900)、amount、status(0–6)、provider_trade_no/status/message、granted_at/grant_status/grant_message、atm_bank_code/virtual_account/expire_date、pay_time、closed_at、expire_at、last_reconciled_at、rescued_at、rescue_action、is_duplicate、duplicate_of_payment_id、duplicate_sequence、provider_create_at、provider_paid_at、reactivated_at、client_type、app_type、version；觸發器 updated_at | `PaymentTransactionMapper`（704 行）／`RepositoryImpl`；全部 service | 保留→Go：`payment.payments`（重設計）：一筆付款對一個 PaymentIntent（Stripe PaymentIntent id、`transfer_group`、amount、status 用 Go 型別＋CHECK、冪等鍵）；一筆訂單同時只有一筆進行中的付款（部分唯一索引）；刪除 ATM、grant、rescue、duplicate、reconcile、attempt_seq、client/app_type 欄；轉帳另有 `payment.transfers`（一筆付款最多一筆有效轉帳的唯一索引） |
| 2 | payment_refunds | V1:118；V2 觸發器 `trg_check_refund_total` | refund_no、payment_id、trade_no、order_id、amount、reason、status(0–3)、provider_*、estimated_refund_date、bank_date、idempotency_key（partial unique）、version | `RefundTransactionMapper`／`RefundApplicationService` | 保留→Go：`payment.refunds`（退款 intent 列：呼叫端冪等鍵 unique、Stripe refund id、狀態）；V2 trigger 的累計不超額改成 transaction 內 `SELECT … FOR UPDATE` 付款列再加總非失敗退款（D4；跨列加總單列 CHECK 守不住），以 integration 測試守 |
| 3 | payment_webhooks | V1:175 | provider、idempotency_key、encrypt_info、decrypted_payload、process_status、source_ip | Consumer／ReturnController／GrpcService 寫 | 刪除→平台 `inbox.events`（D29） |
| 4 | payment_logs | V1:219 | trade_no、log_level、log_type、message | `insertGrantLog` 等；DataCleanupScheduler 清 | 刪除（改 structured log；審計需求見 #2805） |
| 5 | outbox_events | V1:262；V11 加 exchange | exchange、event_type、routing_key、payload、status、retries、lease | Outbox writer／publisher／cleanup | 刪除→events.Publish＋River |
| 6 | payuni_audit_logs | V3:10 | trade_no、refund_no、direction、request/response 原文 | `PayUniAuditLogger` | 刪除；若要保留 provider 呼叫紀錄，改 D5 outbound client 寫遮罩過的紀錄（#2805） |
| 7–15 | logistics_receivables、logistics_collection_attempts、seller_logistics_accounts、logistics_adjustments、ledger_transactions、ledger_entries、fulfillment_exception_cases、fulfillment_case_resolutions、provider_settlement_lines | V9:58-438（V10、V12–V15 修改） | 物流應收、帳本、責任判定 | **沒有程式讀寫**（只有 `pkg/codemeta/registry.go`、文件、csv 提到） | 刪除（隨服務整個 DB；物流已刪） |
| 16 | logistics_cost_facts | V16:34 | provider、provider_delivery_ref、estimated_cost… | 無 | 刪除 |
| — | billing_provider_accounts、membership_provider_subscriptions、membership_charge_attempts、billing_commands | V5，V17 drop | — | — | 刪除：已刪（V17）；`db-tables.csv:5` 仍列 20 張（過時） |

Redis（非表）：`PaymentRedirectFormStore`（一次性 PayUni 表單，TTL）、`PaymentPageService` → 刪除。

replicas.csv：

| 行 | 內容 | 提議 |
|---|---|---|
| :8 | payment 持有 `payment_transactions.order_id,buyer_id,mer_id,amount,product_name`（CreatePayment 時從 order 同步） | 保留→Go：快照（replicas.csv 已填 snapshot）：付款當下的金額、買家與收款帳戶是事實，存進 `payment.payments`；mer_id 換為 Stripe account id；product_name 作 Stripe description（預設，可推翻） |
| :16 | order 持有 `orders.transaction_id,payment_method,atm_*`（payment 雙寫 SnapshotPaymentInfo） | 刪除改查詢（ATM 欄位刪；payment_method 由 payment 模組讀取） |
| :17 | order 持有 `orders.pay_time,status(PAID)`（payment.success 事件） | 保留→Go：order 自己的狀態，來源改模組事件 |
| :20 | order `refund_total_amount`、`financial_logs`（refund.succeeded） | 保留→Go：來源改事件；B5 收斂 |
| :21、:23、:24 | aftersales 持有 transaction_id、payment_method、mer_id、payuni_status… | 保留→Go：B5 決定；B2 只保證 refund 事件的 payload 照舊（relay 測試斷言） |
| :46–:48 | 物流 | 刪除：已填 removed（表無程式碼） |
| :49 | membership | 刪除：已填 removed（會員計費流程從未上線） |

---

## 5. 排程與外部 provider 呼叫（單體規則 5 檢查）

計數：`rg -n '@Scheduled' $PS`（4 個）；`rg -n 'paymentClient\.\w+' -o $PS | sort | uniq`（13 個呼叫點）；PayUni 路徑 `backend/services/java/shared/payuni-java-client/.../payment/PayUniPaymentEndpoint.java:11-58`。

本節的「規則 5」指單體規則 5（計畫不變量表也稱 D5，不要與計畫決定 D5「付款成功但訂單已不可付」混淆）：外部呼叫不能在 DB transaction 內進行，要先寫 intent，再由 River job 帶冪等鍵去呼叫。

| # | 項目 | 位置 | 外部呼叫 | 在 tx 內？ | 規則 5 | 提議 |
|---|---|---|---|---|---|---|
| 1 | createPayment（信用卡 UPP 表單） | `PaymentApplicationService.java:138` @Transactional → `:859` `createCreditCardPayment` | 只組表單（`/api/upp` 由瀏覽器 POST），不打 API | 是（另外在 tx 內打 order gRPC `:141`、`:277`） | ✗（gRPC 在 tx 內） | 保留→Go：改寫：tx 內寫 payment 列與 job，job 建 Stripe PaymentIntent（帶 `transfer_group`，冪等鍵 = payment id）；訂單資料由注入的 port 在 tx 外取得 |
| 2 | createPayment Apple/Google Pay | `:881`、`:912` | POST `/api/apple`、`/api/google` | 是 | ✗ | 刪除：Apple／Google Pay 隨 Stripe 付款元件（D3 兩案皆由 Stripe 處理錢包與 merchant validation），不另建流程 |
| 3 | createPayment ATM | `:946` | POST `/api/atm` | 是 | ✗ | 刪除：ATM（範圍「不做 ATM」，Stripe 沒有台灣 ATM 虛擬帳號） |
| 4 | syncPaymentStatus | `:454` @Transactional → `:469` queryTransaction | `/api/trade/query` | 是 | ✗ | 保留→Go：改寫：讀本地狀態並排 job 重讀 PaymentIntent（外部呼叫在 job） |
| 5 | grantPayment | `:527` @Transactional＋`findActiveByOrderIdForUpdate` → `:702` grant、`:792` grantQuery | `/api/trade/grant`、`/api/trade/grant_query` | 是，**持有列鎖**期間重試 | ✗ | 保留→Go：改寫為轉帳（D1、D6）：tx 內寫 `transfers` 列（唯一索引，一筆付款最多一筆有效轉帳）並排 job，job 呼叫 Stripe Transfer（帶 `source_transaction`，Idempotency-Key = transfer id）；grant_query 對帳去重刪除 |
| 6 | **createRefund** | `RefundApplicationService.java:60-149` | `/api/trade/query`（:308）→ `/api/trade/cancel`（:328）或 `/api/trade/close` CloseType=2（:334） | **是**：`@Transactional` 加上 `findCanonicalByTradeNoForUpdate`（:80）。整段 PayUni 往返（最多兩次 HTTP）期間都持有 payment 列的 FOR UPDATE 鎖；refund 列先 INSERT（:120），PayUni 失敗時拋例外，整個 tx rollback，**PayUni 那邊可能已經退了，本地卻沒有紀錄**（查詢成功後 cancel/close 逾時就會發生） | ✗ | 保留→Go：改寫（D4）：tx 內鎖 payment 列、加總非失敗退款檢查累計上限、INSERT refund（REQUESTED，呼叫端冪等鍵 unique）、Enqueue job；job 呼叫 Stripe `POST /v1/refunds`（Idempotency-Key = refund id），結果由 job 回寫或 `refund.updated` webhook 定案；訂單完成後的退款先撤回轉帳（D1，賣家餘額不足由平台吸收）。ATM 分支（:125-132）刪除 |
| 7 | createRescueRefund | `:172` @Transactional → `:236` | 同上 | 是 | ✗ | 刪除 |
| 8 | RescueRefundConsumer | `mq/RescueRefundConsumer.java:145` refund | `/api/trade/close` | 推論是 | ✗ | 刪除 |
| 9 | PendingPaymentReconcileScheduler | `scheduler/PendingPaymentReconcileScheduler.java:55`（5 分鐘）→ `PendingPaymentReconcileService.java:151` | `/api/trade/query` | 每筆一個 tx（claim lease） | 部分（有 claim） | 刪除（計畫 `:346`） |
| 10 | ExpiredTransactionScheduler | `scheduler/ExpiredTransactionScheduler.java:68`（60 秒） | 無 | 每筆短 tx，CAS＋outbox | ✓（無外部呼叫） | 保留→Go：改寫成 River 週期 job：逾期的付款取消 PaymentIntent（外部呼叫在 job）後同 transaction 發事件 |
| 11 | OutboxEventPublisherScheduler | `:46`（5 秒） | RabbitMQ | — | n/a | 刪除（River／events） |
| 12 | DataCleanupScheduler | `:75`（每日 02:30） | 無 | — | n/a | 刪除（inbox 清理由 platform 負責） |
| 13 | ApplePayMerchantValidator | `ApplePayMerchantValidator.java:96` | Apple `apple-pay-gateway.apple.com/paymentservices/*`（mTLS） | 否 | ✓ | 刪除：D3 兩案皆由 Stripe 處理 |
| 14 | buildRefundPageFormData | `PaymentApplicationService.java:447` → `offlineRefundPage` | `/api/offline_payment/refund`（瀏覽器表單） | — | — | 刪除（ATM） |

PayUni client 還有 `offlineRefund`、`cancelOfflineRefund`（`PaymentClient.java:271,333`），payment-service 沒有用到（死碼）。

---

## 6. 業務流程

- **付款建立**（`PaymentApplicationService.createPayment` :139）：先向 order 取付款資訊（GetOrderForPayment）。接著驗 buyer、驗訂單是否待付款、依付款方式檢核金額上下限（卡 1–199,999；ATM 15–49,999）。零元訂單直接 PAID（2900）。同一付款方式的未過期列直接回傳（冪等）。換付款方式時，舊列標 CLOSED_BY_SWITCH；切回 ATM 時嘗試復活舊列（AtmReactivation）。之後以 `trade_no = orderNo + attempt_seq` 建新列，SnapshotPaymentInfo 回寫 order，再依方式呼叫 PayUni（卡：UPP 表單存 Redis，回 `/payment/redirect?id=`；Apple/Google Pay：直接扣款；ATM：取虛擬帳號）。**PayUni 專屬**：attempt_seq／CLOSED_BY_SWITCH／ATM 復活／表單重導／ATM。**保留**：buyer 歸屬、待付款檢查、零元、金額範圍（Stripe 有自己的下限）、一筆訂單同時只有一筆進行中的付款。
- **付款結果**：有三個入口，都進 `PaymentNotifyHandler.handle`（:67）：PayUni webhook 經 payuni-webhook-service → MQ（`PayUniWebhookConsumer`）、瀏覽器導回（gRPC `HandlePaymentPageReturn`；service 自己的 `PaymentReturnController` 推論是死碼）、補收排程（PendingPaymentReconcile）。冪等鍵是 `tradeNo:providerTradeNo`，記在 `payment_webhooks` inbox；處理是狀態 CAS（INIT→PAID/FAILED），PAID 碰到過期列則標 CONFLICT。重複付款（同一訂單兩筆成功）由 `DuplicatePaymentService` 對較晚的那筆發 rescue 退款。最後 outbox 發 `payment.*` 給 order。**Go**：Stripe webhook → `inbox.events`（event id 去重）→ handler 重讀 PaymentIntent 後同 transaction 改狀態並 `events.Publish`；CONFLICT／duplicate／rescue／reconcile 全刪，換成「訂單已不可付卻收到成功 → 排自動全額退款 job」（D5，擁有者是 payment，計畫步驟 3；完成發 `AutoRefundCompleted` 給 A8 notification）。
- **退款**（`createRefund` :61）：先驗冪等鍵，再鎖 payment 列、拒絕零元、驗狀態、拒絕已有進行中的退款、驗累計額度（L2）＋DB 觸發器（L3）。之後 INSERT refund：信用卡類呼叫 PayUni 同步定案並發事件；ATM 則停在 REQUESTED，由 aftersales 拿 PayUni 頁面版 URL（GetRefundPageUrl）讓買家填帳戶，結果經 `payuni.refund.accepted/notify` 回來。呼叫者是 aftersales（退貨、取消、裁決、任務 worker）與 order（AutoRefundWorker）。**PayUni 專屬**：CloseStatus 分流、ATM 頁面版、Ack、Decrypt。**Go**：`RequestRefund`（表 5 #6、D4）：並行退款只守累計上限（鎖付款列後加總非失敗退款），呼叫端冪等鍵，「已有進行中的退款就拒絕」（`hasProcessing`、`REFUND_ALREADY_PROCESSING`）退場；訂單完成後的退款先撤回轉帳（D1）。
- **撥款／結算**（`grantPayment` :528）：order 的 GrantPaymentWorker 在訂單完成後呼叫。在鎖內呼叫 PayUni grant（代理商模式，平台先收款、完成後才撥給賣家子商店）；回應模稜兩可時記 PENDING_CONFIRM，之後用 grant_query 逐日對帳去重（GrantResponseRules）。**全部是 PayUni 代理商模式專屬**（GrantResponseRules、grant_query 對帳去重都刪）。**Go**（D1、D6）：付款帶 `transfer_group`；訂單完成時 Java order 的 `GrantPaymentWorker` 經 legacy gRPC `GrantPayment`（B8 改 in-process），payment 寫 `transfers` 列並排 job，job 建帶 `source_transaction` 的 Stripe Transfer（Idempotency-Key = transfer id）；完成後的退款先撤回轉帳，賣家餘額不足由平台吸收；B9 只剩手續費與對帳。
- **物流財務與責任判定**：已於 2026-11-23 刪除（commit `5883f499e` 等），不在 Java 裡了。剩下的只有 V9–V16 的表與 CodeMeta 值域。計畫 `:345` 的「先留在 Java」已過時。
- **後台查詢**：`PaymentAdminQueryService`（:62–:180）提供異常付款／退款／撥款的列表與詳情，用 MyBatis 動態排序。保留→Go：付款、退款列表與詳情走 admin Connect，撥款（grants）頁改為轉帳列表（步驟 5）。
- **付款事實給發票**：`GetPaymentFactsInternal` 給 invoice 讀 provider_paid_at 等欄位。保留。
- **PayUni 專屬、決定刪除**：AtmReactivationService、PendingPaymentReconcile*、PaymentReturnController、PaymentPageService、PaymentRedirectFormStore、UnknownPaymentFallbackConsumer、PayUniRefund{Accepted,Notify}Consumer、RescueRefundConsumer、GrantResponseRules、PayUniAuditLogger、ApplePayMerchantValidator（D3 兩案皆刪）、DuplicatePaymentService。DuplicatePaymentService 的核對（`rg -n 'DuplicatePaymentService' backend/services/java --type java`，排除自身檔與測試）：main 的呼叫者只有 `scheduler/PendingPaymentReconcileService.java:195`（`handleDuplicateFromReconcile`，補收排程）與 `adapter/webhook/handler/PaymentNotifyHandler.java:147→389`（`handleSwitchBackToPaid`）、`:420`（`handleDuplicateFromWebhook`）；後兩者是 PayUni 專屬分支（切換付款方式後舊管道完成、重複付款偵測），在 Stripe 一張 PaymentIntent 下不存在，隨 PaymentNotifyHandler 由 Stripe inbox handler 取代；其餘命中是註解與測試。結論：只服務 rescue／reconcile 與 PayUni 專屬分支，可與它們一起刪（已核對）。

---

## 7. Java 測試對照表

193 個測試方法，37 個檔（`CryptoTest` 是手動 main，不算測試，提議退場）。其中 integration（真 DB，`@SpringBootTest`／Testcontainers）：`rg -l 'Testcontainers|PostgreSQLContainer|@SpringBootTest' backend/services/java/payment-service/src/test` → 6 個檔（GrantSentinelSurvival、OutboxEnqueueTransaction、OutboxSentCleanup、RefundTotalGuard、ReconcileBoundary、ReconcileSchedulerDisabled）。路徑相對 `backend/services/java/payment-service/src/test/java/io/github/will413028/hawkr/payment/`；同類別後續列以〃表示同一提議。

彙總（193 個方法，依「提議去向」首詞：保留→Go 33、對照→平台已保護 8、刪除 6、退場 146）：保留→Go 的類別是 PaymentFactsInternal（6）、PaymentOwnershipBuyerIdContract（5）、SellerPaymentHistoryOwnership（7）、PaymentNotifyHandlerFailedPath（5）、GrantPaymentIdempotency（2，改為轉帳冪等）、RefundTotalGuardIntegration（4，累計上限）、ExpiredTransactionOutbox（4）；對照→平台已保護 2 類（OutboxWriterTransactionGuard、OutboxEnqueueTransactionIntegration）；刪除是 ConflictPath（6，CONFLICT 隨 D5 刪）；其餘退場。步驟 3 以標「保留→Go」的 33 列核對。

| # | 類別:行 | 方法 | 斷言的行為（DisplayName） | 提議 |
|---|---|---|---|---|
| 1 | `apter/grpc/CreateRefundPermissionTest.java:73` | `externalCall_isRejectedWithPermissionDenied` | 外部（非 internal）呼叫 createRefund → PERMISSION_DENIED 且不進退款流程 | 退場：模組 API 是 in-process，沒有「外部呼叫」；計畫步驟 4 沒有為 legacy paymentserver 另訂 internal-only guard，故不新增對應測試（是否要加見 Open items） |
| 2 | `apter/grpc/GetRefundPageUrlBalanceTest.java:109` | `atmFullRefund_prebuiltRow_getsPageUrl` | ATM 全額退款：預建 REQUESTED 本筆存在 → 排除本筆 → 成功取得退款頁 URL（不誤擋） | 退場：ATM 頁面版退款是 PayUni 專有（GetRefundPageUrl 刪除）；餘額排除本筆的語意併入退款額度測試 |
| 3 | `apter/grpc/GetRefundPageUrlBalanceTest.java:143` | `guardStillBlocksWhenOtherRefundsExhaustBalance` | 防線仍在：其他退款已用盡餘額時，超額請求仍回 REFUND_AMOUNT_EXCEEDED | 退場（同上列） |
| 4 | `apter/grpc/GrantPaymentAdapterTest.java:89` | `pendingConfirm_throwsGrpcError` | 內層回 PENDING_CONFIRM（grantOutcome=null）→ adapter throw gRPC error（非 SUCCESS code） | 退場：gRPC adapter 隨 Java 刪除；legacy `GrantPayment` 由步驟 4 的 integration 測試以舊 proto client 驗證；grant_outcome（PROVIDER_TOOK_OVER 等）是 PayUni 專屬，不搬 |
| 5 | `apter/grpc/GrantPaymentAdapterTest.java:102` | `errorCode_throwsGrpcError` | 內層回 PayUni 錯誤碼（grantOutcome=null）→ adapter throw gRPC error | 退場（同上列） |
| 6 | `apter/grpc/GrantPaymentAdapterTest.java:114` | `success_returnsGrpcSuccess` | 內層回 SUCCESS（grantOutcome=GRANTED）→ adapter 回 gRPC success 帶 grant_outcome | 退場（同上列） |
| 7 | `apter/grpc/GrantPaymentAdapterTest.java:128` | `providerTookOver_returnsNormalResponseWithOutcome` | 內層回 PROVIDER_TOOK_OVER → adapter 回正常回應（非錯誤）帶 grant_outcome、granted_at 空字串 | 退場（同上列） |
| 8 | `apter/grpc/PaymentFactsInternalTest.java:145` | `cardWithProviderPaidAt_isSet` | ① 卡片交易有 provider_paid_at → hasProviderPaidAt()==true 且值正確 | 保留→Go：invoice 讀付款事實改呼叫 payment 模組 API（PaymentFacts，過渡期經 legacy paymentserver）；①–⑤ 欄位映射、⑦ 唯讀搬成 unit/integration；⑥ 授權退場（in-process） |
| 9 | `apter/grpc/PaymentFactsInternalTest.java:160` | `atmWithNullProviderPaidAt_isUnset` | ② ATM 交易 provider_paid_at 為 NULL → hasProviderPaidAt()==false（不是空字串） | 保留→Go（同上列） |
| 10 | `apter/grpc/PaymentFactsInternalTest.java:177` | `noCanonicalTransaction_returnsBusinessErrorWithoutData` | ③ 查不到訂單的 canonical 交易 → 非 0 業務碼且不帶 data（不得回 code=0 ＋ 空 data） | 保留→Go（同上列） |
| 11 | `apter/grpc/PaymentFactsInternalTest.java:191` | `payTimeAndProviderPaidAt_areNotSwapped` | ④ pay_time 與 provider_paid_at 分屬不同欄位（防對調：兩者語意不同，接錯無任何訊號） | 保留→Go（同上列） |
| 12 | `apter/grpc/PaymentFactsInternalTest.java:205` | `remainingFieldsMapCorrectly` | ⑤ 其餘欄位對帳：order_no / trade_no / trade_amt / payment_type / is_duplicate | 保留→Go（同上列） |
| 13 | `apter/grpc/PaymentFactsInternalTest.java:221` | `nonInternalCall_isDenied` | ⑥ 非內部呼叫 → PERMISSION_DENIED，且不查任何資料 | 退場：in-process 模組 API 沒有外部呼叫（同 #1） |
| 14 | `apter/grpc/PaymentFactsInternalTest.java:242` | `readOnly_neverWrites` | ⑦ 只讀不寫：本 RPC 不得觸發任何寫入路徑 | 保留→Go：⑦ 只讀不寫（同 #8） |
| 15 | `apter/grpc/PaymentGrpcRedirectFormTest.java:37` | `validId_returnsAllFieldsWithIsPlatformMapped` | （無 DisplayName，見方法名） | 退場：PayUni UPP 表單重導（Redis opaque id）刪除；Stripe 用 Checkout/PaymentIntent client_secret |
| 16 | `apter/grpc/PaymentGrpcRedirectFormTest.java:65` | `unknownId_returnsNotFound` | （無 DisplayName，見方法名） | 退場（同上列） |
| 17 | `apter/grpc/PaymentOwnershipBuyerIdContractTest.java:139` | `buyerCallerWithResolvedBuyerId_getsTransaction` | 帶已解析 buyer_id 的買家呼叫 → 200，回傳交易 | 保留→Go：buyer 歸屬檢查改在 httpapi/use case（buyer_id 來自 session），Connect 特性測試覆蓋「他人交易回 NOT_FOUND」 |
| 18 | `apter/grpc/PaymentOwnershipBuyerIdContractTest.java:152` | `buyerCallerWithRawAccountId_isDeniedAsNotFound` | 帶 raw account_id 的買家呼叫 → NOT_FOUND（與『交易不存在』無法區分） | 保留→Go（同上列） |
| 19 | `apter/grpc/PaymentOwnershipBuyerIdContractTest.java:168` | `buyerScopedInternalCallWithoutBuyerId_failsLoud` | 內部呼叫傳播了買家 account-type 卻沒帶 buyer_id → UNAUTHENTICATED（fail loud，不塌成 NOT_FOUND） | 保留→Go（同上列） |
| 20 | `apter/grpc/PaymentOwnershipBuyerIdContractTest.java:181` | `buyerScopedInternalCallWithoutBuyerId_getPaymentHistory_failsLoud` | getPaymentHistory：內部呼叫傳播了買家 account-type 卻沒帶 buyer_id → UNAUTHENTICATED（fail loud，同一份 guard 邏輯複製在 verifyPaymentOwnershipByOrderId） | 保留→Go（同上列） |
| 21 | `apter/grpc/PaymentOwnershipBuyerIdContractTest.java:194` | `sellerAndBuyerCallerWithRawAccountId_isDeniedAsNotFound` | SELLER_AND_BUYER（code 3）也走買家歸屬檢查，raw account_id 一樣被拒 | 保留→Go（同上列） |
| 22 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:130` | `sellerSameStore_succeedsAndRedactsBuyerPii` | 賣家查自己店家的訂單 → 200，且遮蔽 buyerId / atmVirtualAccount / providerTradeNo | 保留→Go：賣家看付款歷程的跨店拒絕與 PII 遮蔽 → Connect handler 測試 + 特性測試 |
| 23 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:151` | `dualRoleSeller_sameStore_succeeds` | SELLER_AND_BUYER（code 3）賣家視角查自己店家 → 200（covers SELLER） | 保留→Go（同上列） |
| 24 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:163` | `sellerCrossStore_isDeniedAsNotFound` | 賣家查他店訂單 → NOT_FOUND（與訂單不存在無法區分），不回傳任何 row | 保留→Go（同上列） |
| 25 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:178` | `sellerOrderNotFound_isNotFound` | 賣家查不存在的訂單（resolveOrderStoreId=0）→ NOT_FOUND（與跨店同形） | 保留→Go（同上列） |
| 26 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:190` | `sellerMissingStoreId_failsLoud` | 賣家視角呼叫但未帶 store_id（Gateway 契約違反）→ UNAUTHORIZED（fail loud，不塌成 NOT_FOUND） | 保留→Go（同上列） |
| 27 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:202` | `pureBuyer_isDenied` | 純買家（code 1）呼叫賣家視角端點 → PERMISSION_DENIED | 保留→Go（同上列） |
| 28 | `apter/grpc/SellerPaymentHistoryOwnershipTest.java:211` | `sellerCannotAccessBuyerOnlyGetPayment` | #2501 收斂：純賣家（code 2）呼叫 buyer-only 的 getPayment → PERMISSION_DENIED（不再放行） | 保留→Go（同上列） |
| 29 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:110` | `firstUpdateSucceeds_publishesConflict` | a) updateToConflict 第一次成功 → 事務內 outbox 寫入 payment.conflict | 刪除：CONFLICT 分支隨 D5 刪除；新增 Go 測試「訂單已不可付卻收到成功 → 排自動全額退款 job（payment 擁有）」（步驟 3） |
| 30 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:126` | `conflictReloadConflict_idempotentNoPublish` | b) updated=0 → reload 為 CONFLICT → 冪等 success、不重發 | 刪除（同上列） |
| 31 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:143` | `conflictReloadPaid_retrySucceeds_publishes` | c) updated=0 → reload 仍 PAID → 新 version 重試成功 → 發 payment.conflict | 刪除（同上列） |
| 32 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:163` | `conflictReloadPaid_retryExhausted_throwsRetryable` | d) updated=0 → reload 仍 PAID → 重試仍 0 → 拋 WebhookRetryableException（不吞） | 刪除（同上列） |
| 33 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:177` | `conflictReloadOtherTerminal_stateConflictNoPublish` | e) updated=0 → reload 為其他終態（EXPIRED）→ stateConflict 不覆蓋、不發事件 | 刪除（同上列） |
| 34 | `apter/webhook/handler/PaymentNotifyHandlerConflictPathTest.java:194` | `conflictReloadMissing_throwsRetryable` | f) updated=0 → reload 找不到 row → 拋 WebhookRetryableException | 刪除（同上列） |
| 35 | `apter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:101` | `firstUpdateSucceeds_publishesFailed` | a) updateToFailed 第一次成功 → 事務內 outbox 寫入 payment.failed | 保留→Go：webhook 狀態轉移的 CAS／冪等語意搬到 Stripe event（`payment_intent.payment_failed`）的 inbox handler 測試：FAILED 不覆蓋 PAID、重複事件不重發 |
| 36 | `apter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:123` | `conflictReloadFailed_idempotentNoPublish` | b) updated=0 → reload 為 FAILED → 冪等 success、不發第二次事件 | 保留→Go（同上列） |
| 37 | `apter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:142` | `conflictReloadPaid_noOverwriteNoPublish` | c) updated=0 → reload 為 PAID → log 衝突、回 success、不覆蓋、不發 failed 事件 | 保留→Go（同上列） |
| 38 | `apter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:161` | `conflictReloadInit_retrySucceedsPublishes` | d) updated=0 → reload 仍 INIT → 用新 version 重試成功 → 發 payment.failed | 保留→Go（同上列） |
| 39 | `apter/webhook/handler/PaymentNotifyHandlerFailedPathTest.java:187` | `conflictReloadInit_retryExhausted_throws` | e) updated=0 → reload 仍 INIT → 重試又 0 → 拋 WebhookRetryableException | 保留→Go（同上列） |
| 40 | `plication/service/ApplePayMerchantValidatorTest.java:56` | `rejectsHttpScheme` | scheme=http 拒絕 8028 | 退場：Apple Pay merchant validation 由 Stripe 處理（D3 兩案皆刪），自建 mTLS validator 刪除 |
| 41 | `plication/service/ApplePayMerchantValidatorTest.java:62` | `rejectsFileScheme` | scheme=file 拒絕 8028 | 退場（同上列） |
| 42 | `plication/service/ApplePayMerchantValidatorTest.java:70` | `rejectsNonAppleHost` | host 非 *.apple.com 拒絕 8028 | 退場（同上列） |
| 43 | `plication/service/ApplePayMerchantValidatorTest.java:76` | `rejectsSpoofedAppleHost` | host 偽冒 apple.com.evil.com 拒絕 8028 | 退場（同上列） |
| 44 | `plication/service/ApplePayMerchantValidatorTest.java:82` | `acceptsUppercaseHost` | host 全大寫 APPLE-PAY-GATEWAY.APPLE.COM 通過 host 檢查（lowercased endsWith） | 退場（同上列） |
| 45 | `plication/service/ApplePayMerchantValidatorTest.java:92` | `rejectsIpv4LiteralHost` | IPv4 literal host 拒絕 8028（不會 endsWith .apple.com） | 退場（同上列） |
| 46 | `plication/service/ApplePayMerchantValidatorTest.java:98` | `rejectsIpv6LiteralHost` | IPv6 literal host 拒絕 8028（[]bracketed host 不會 endsWith .apple.com） | 退場（同上列） |
| 47 | `plication/service/ApplePayMerchantValidatorTest.java:106` | `rejectsNonPaymentServicesPath` | path 不在 /paymentservices/* 拒絕 8028 | 退場（同上列） |
| 48 | `plication/service/ApplePayMerchantValidatorTest.java:112` | `rejectsPathTraversal` | path traversal /paymentservices/../admin 經 normalize 後拒絕 8028 | 退場（同上列） |
| 49 | `plication/service/ApplePayMerchantValidatorTest.java:120` | `rejectsUserInfo` | URL 含 userInfo 拒絕 8028（defensive reject，Apple 不會給 user:pass@host） | 退場（同上列） |
| 50 | `plication/service/ApplePayMerchantValidatorTest.java:129` | `rejectsNonHttpsPort` | port != 443 (8080) 拒絕 8028 | 退場（同上列） |
| 51 | `plication/service/ApplePayMerchantValidatorTest.java:138` | `rejectsEmptyValidationUrl` | validation_url 空字串拒絕 8028 | 退場（同上列） |
| 52 | `plication/service/ApplePayMerchantValidatorTest.java:144` | `rejectsNullDomain` | domain_name = null 拒絕 8028 | 退場（同上列） |
| 53 | `plication/service/ApplePayMerchantValidatorTest.java:150` | `rejectsBlankDomain` | domain_name 空字串拒絕 8028 | 退場（同上列） |
| 54 | `plication/service/ApplePayMerchantValidatorTest.java:156` | `rejectsOverlongUrl` | 超長 validation_url（> 2048）拒絕 8028 | 退場（同上列） |
| 55 | `plication/service/ApplePayMerchantValidatorTest.java:163` | `rejectsOverlongDomain` | 超長 domain_name（> 253）拒絕 8028 | 退場（同上列） |
| 56 | `plication/service/ApplePayMerchantValidatorTest.java:172` | `rejectsDomainNotInAllowlist` | domain_name 不在 allowlist 拒絕 8028 | 退場（同上列） |
| 57 | `plication/service/ApplePayMerchantValidatorTest.java:179` | `rejectsWhenAllowlistEmpty` | allowlist 為空（未配置）拒絕 8028 | 退場（同上列） |
| 58 | `plication/service/ApplePayMerchantValidatorTest.java:188` | `rejectsMalformedUrl` | URL 語法錯誤拒絕 8028 | 退場（同上列） |
| 59 | `plication/service/ApplePayMerchantValidatorTest.java:196` | `certPathNotConfiguredFailsLoad` | SSRF 全通過但 cert/key path 未設定 → 8029 APPLE_PAY_CERT_LOAD_FAILED | 退場（同上列） |
| 60 | `plication/service/AtmRefundAcceptedNoActiveTest.java:81` | `active_marksAccepted` | active present → markAtmRefundAccepted（既有分支不受影響） | 退場：PayUni ATM 退款受理通知（PayUniRefundAcceptedConsumer 刪除） |
| 61 | `plication/service/AtmRefundAcceptedNoActiveTest.java:95` | `noActive_latestSucceeded_idempotentNoMark` | 無 active + 最新 SUCCEEDED → 終態冪等略過，不寫 mark、不 throw | 退場（同上列） |
| 62 | `plication/service/AtmRefundAcceptedNoActiveTest.java:108` | `noActive_latestFailed_idempotentNoMark` | 無 active + 最新 FAILED → 終態冪等略過，不寫 mark、不 throw | 退場（同上列） |
| 63 | `plication/service/AtmRefundAcceptedNoActiveTest.java:121` | `noActive_noRow_orphanNoMark` | 無 active + 查無 row → 孤兒受理，不寫 mark、不 throw | 退場（同上列） |
| 64 | `plication/service/AtmRefundAcceptedNoActiveTest.java:133` | `noActive_latestRequested_unexpectedNoMark` | 無 active + 最新 REQUESTED（非預期矛盾）→ 不寫 mark、不 throw | 退場（同上列） |
| 65 | `plication/service/AtmRefundWebhookFinalizeTest.java:78` | `activeSuccess_finalizesSucceededAndPublishes` | active + RefundStatus=1 → finalizeAtmRefundSucceeded + 發 succeeded | 退場：PayUni ATM；其中「找不到 active 不得靜默當成功、FAILED 後收到成功的校正」語意由 Go `refund.updated` inbox handler 測試承接（步驟 3） |
| 66 | `plication/service/AtmRefundWebhookFinalizeTest.java:95` | `activeFail_finalizesFailedAndPublishes` | active + RefundStatus=2 → finalizeAtmRefundFailed + 發 failed | 退場（同上列） |
| 67 | `plication/service/AtmRefundWebhookFinalizeTest.java:111` | `noActive_latestSucceeded_idempotentNoFinalizeNoPublish` | 無 active + 最新 SUCCEEDED → 冪等，不 finalize、不重發 | 退場（同上列） |
| 68 | `plication/service/AtmRefundWebhookFinalizeTest.java:125` | `noActive_latestFailed_conflict_correctsAndPublishes` | 無 active + 最新 FAILED + RefundStatus=1 → 衝突校正走 correctFailedToSucceeded（非 finalize）+ 補發 | 退場（同上列） |
| 69 | `plication/service/AtmRefundWebhookFinalizeTest.java:146` | `noActive_latestFailed_conflict_correctNoop_doesNotPublish` | 無 active + 最新 FAILED + RefundStatus=1 但校正 0 rows（已被別的路徑校正）→ 不重發 | 退場（同上列） |
| 70 | `plication/service/AtmRefundWebhookFinalizeTest.java:163` | `noActive_latestFailed_bothFailed_idempotent` | 無 active + 最新 FAILED + RefundStatus=2 → 冪等，不動作 | 退場（同上列） |
| 71 | `plication/service/AtmRefundWebhookFinalizeTest.java:177` | `noActive_noRow_orphanNoAction` | 無 active + 查無 row → 孤兒，不 finalize、不發事件 | 退場（同上列） |
| 72 | `plication/service/GrantDedupHardeningTest.java:106` | `firstAttempt_timeoutException_pendingConfirm_noRetry` | A1 首撥 timeout（PayUniApiException）→ 不穿透、不重試、標 PENDING_CONFIRM（updateGrantFailure） | 退場：PayUni grant／grant_query 去重；Stripe 以 idempotency key 取代 |
| 73 | `plication/service/GrantDedupHardeningTest.java:123` | `firstAttempt_hashInvalid_pendingConfirm` | A2 首撥 HASH_INVALID（不可信回應）→ 不重試、標 PENDING_CONFIRM | 退場（同上列） |
| 74 | `plication/service/GrantDedupHardeningTest.java:137` | `firstAttempt_rejectCode_retriesOnce_writesErrorCode` | A3 首撥 明確拒絕碼 → retry 一次（共 2 次）→ 仍失敗 → 寫 PayUni 錯誤碼 | 退場（同上列） |
| 75 | `plication/service/GrantDedupHardeningTest.java:151` | `firstAttempt_rejectThenRetrySuccess_writesSuccess` | A4 首撥 明確拒絕碼 → retry 成功 → 寫 SUCCESS（granted_at） | 退場（同上列） |
| 76 | `plication/service/GrantDedupHardeningTest.java:170` | `rejectWithCompletedRefund_providerTookOver_noRetry_writesOriginalCode` | A5 拒絕碼 + 存在已完成退款 → 正常回應 grant_outcome=PROVIDER_TOOK_OVER：不重試、grant_status 落 PayUni 原碼 | 退場（同上列） |
| 77 | `plication/service/GrantDedupHardeningTest.java:188` | `grant30023WithoutRefund_definitelyNotSent_retriesOnce` | A6 GRANT30023 但無已完成退款（請款批次未結時間性場景）→ 照既有明確未發動路徑（retry 一次、grantOutcome=null） | 退場（同上列） |
| 78 | `plication/service/GrantDedupHardeningTest.java:230` | `retry_reconcileHit_marksSuccess_noGrant` | B1 非首撥 + grant_query 命中已入帳 → 標 SUCCESS、絕不重打 Grant | 退場（同上列） |
| 79 | `plication/service/GrantDedupHardeningTest.java:249` | `retry_allDaysNoData_confirmedNotGranted_reGrants` | B2 非首撥 + 全區間逐日 QUERY03002（真的從未撥）查完 → 確認未撥 → 安全重發 Grant | 退場（同上列） |
| 80 | `plication/service/GrantDedupHardeningTest.java:270` | `retry_successMultiPageNoMatch_thenNoData_confirmedNotGranted_reGrants` | B2b 非首撥 + 某日 SUCCESS 多分頁（他人撥款）無命中 + 其餘日 QUERY03002 → 查完查無 → 安全重發 | 退場（同上列） |
| 81 | `plication/service/GrantDedupHardeningTest.java:294` | `retry_noDataThenHitNextDay_marksSuccess_noGrant` | B4 非首撥 + 隔日入帳：嘗試日 QUERY03002 空、隔日 SUCCESS 命中該 TradeNo → SUCCESS 短路（不誤判 INCONCLUSIVE） | 退場（同上列） |
| 82 | `plication/service/GrantDedupHardeningTest.java:316` | `retry_queryOutOfWindow_inconclusive_noGrant` | B5 非首撥 + grant_query 回 QUERY02004（超 30 天窗）→ 未查完 → INCONCLUSIVE：不盲目重發 | 退場（同上列） |
| 83 | `plication/service/GrantDedupHardeningTest.java:333` | `retry_beforePosting_inconclusive_noGrant_noQuery` | B3 非首撥 + 落在入帳前（today<=attemptDay）→ INCONCLUSIVE：不查、不重發、維持非成功 | 退場（同上列） |
| 84 | `plication/service/GrantDedupHardeningTest.java:350` | `retry_queryFails_inconclusive_noGrant` | B3b 非首撥 + grant_query 查詢失敗（未查完）→ INCONCLUSIVE：不盲目重發 | 退場（同上列） |
| 85 | `plication/service/GrantPaymentIdempotencyTest.java:83` | `alreadyGranted_shortCircuits_noPayUniCall` | grant_status==SUCCESS → 冪等短路，回既有結果、不打 PayUni | 保留→Go：改寫為轉帳冪等：已有有效轉帳不重建（`transfers` 唯一索引）、Transfer 帶 Idempotency-Key（D1、D6） |
| 86 | `plication/service/GrantPaymentIdempotencyTest.java:106` | `notYetGranted_callsPayUni` | grant_status==null（未撥款）→ 實際呼叫 PayUni Grant 並寫入 SUCCESS | 保留→Go（同上列） |
| 87 | `plication/service/GrantSentinelSurvivalIntegrationTest.java:177` | `ambiguousTimeout_pendingConfirmCommittedAndSurvives` | (a) 撥款 timeout（PayUniApiException 模稜兩可）→ grant_status=PENDING_CONFIRM 真的 commit 存活，granted_at 保持 null | 退場：PENDING_CONFIRM 哨兵是因為 grant 在 tx 內呼叫；單體規則 5（intent 列 + River job）結構上取代 |
| 88 | `plication/service/GrantSentinelSurvivalIntegrationTest.java:194` | `rejectCode_errorStatusCommittedAndSurvives` | (a') 撥款明確拒絕碼（retry 後仍失敗）→ grant_status=錯誤碼 commit 存活，granted_at 保持 null | 退場（同上列） |
| 89 | `plication/service/GrantSentinelSurvivalIntegrationTest.java:210` | `success_committedWithGrantedAt` | (正控) 撥款成功 → grant_status=SUCCESS commit 存活，granted_at 已寫入 | 退場（同上列） |
| 90 | `plication/service/PaymentRedirectFormStoreTest.java:60` | `store_thenConsume_returnsAllFields` | （無 DisplayName，見方法名） | 退場：Redis 一次性表單 store 刪除 |
| 91 | `plication/service/PaymentRedirectFormStoreTest.java:76` | `consume_isOneTime_secondReadIsEmpty` | （無 DisplayName，見方法名） | 退場（同上列） |
| 92 | `plication/service/PaymentRedirectFormStoreTest.java:83` | `consume_afterExpiry_isEmpty` | （無 DisplayName，見方法名） | 退場（同上列） |
| 93 | `plication/service/PaymentRedirectFormStoreTest.java:91` | `consume_unknownId_isEmpty` | （無 DisplayName，見方法名） | 退場（同上列） |
| 94 | `plication/service/PaymentRedirectFormStoreTest.java:96` | `consume_nullOrBlankId_isEmpty` | （無 DisplayName，見方法名） | 退場（同上列） |
| 95 | `plication/service/PaymentRedirectFormStoreTest.java:103` | `store_usesConfiguredKeyPrefixAndTtl` | （無 DisplayName，見方法名） | 退場（同上列） |
| 96 | `plication/service/RefundCloseStatusRoutingTest.java:118` | `closeStatus1_shouldCallCancelAuth` | CloseStatus=1（請款申請中）→ 呼叫 cancelAuth 取消授權 | 退場：PayUni CloseStatus（取消授權 vs 退款）分流；Stripe refund API 不分 |
| 97 | `plication/service/RefundCloseStatusRoutingTest.java:137` | `closeStatus2_shouldCallRefund` | CloseStatus=2（請款成功）→ 呼叫 refund 退款 | 退場（同上列） |
| 98 | `plication/service/RefundCloseStatusRoutingTest.java:156` | `closeStatus7_shouldCallRefund` | CloseStatus=7（請款處理中）→ 呼叫 refund 退款 | 退場（同上列） |
| 99 | `plication/service/RefundCloseStatusRoutingTest.java:175` | `closeStatus3_shouldThrowException` | CloseStatus=3（請款取消）→ 拋 IllegalStateException | 退場（同上列） |
| 100 | `plication/service/RefundCloseStatusRoutingTest.java:193` | `closeStatus9_shouldThrowException` | CloseStatus=9（未申請）→ 拋 IllegalStateException | 退場（同上列） |
| 101 | `plication/service/RefundCloseStatusRoutingTest.java:211` | `queryFailed_shouldThrowException` | 查詢 API 失敗 → 拋 IllegalStateException | 退場（同上列） |
| 102 | `plication/service/RefundCloseStatusRoutingTest.java:226` | `closeStatusNull_shouldThrowException` | CloseStatus 為 null → 拋 IllegalStateException | 退場（同上列） |
| 103 | `plication/service/RescueRefundAmountLimitTest.java:99` | `rescue_overLimit_rejectedAtApplicationLayerBeforeSave` | 超額（已退 400 + 本次 200 > 交易 500）→ 應用層拒絕，且退款 row 從未建立（不靠 L3） | 退場（RescueRefund 刪除）；額度上限語意由退款額度 integration 測試承接 |
| 104 | `plication/service/RescueRefundAmountLimitTest.java:119` | `rescue_exactlyAtRemainingBalance_allowed` | 剛好等於餘額（已退 400 + 本次 100 = 交易 500）→ 放行，正常建立並完成退款 | 退場（同上列） |
| 105 | `plication/service/RescueRefundAmountLimitTest.java:146` | `rescue_firstFullRefund_allowed` | 尚無任何退款（已退 0 + 本次 500 = 交易 500）→ 放行 | 退場（同上列） |
| 106 | `plication/service/RescueRefundFailurePathTest.java:97` | `rescue_creditCardThrows_providerStatusMustBeNull` | 信用卡 rescue：handleCreditCardRefund 拋例外時 provider_status 必須為 null | 退場：VARCHAR(4) provider_status 欄位問題隨 schema 重設計消失 |
| 107 | `plication/service/RescueRefundFailurePathTest.java:139` | `rescue_providerStatusLengthMustNotExceedColumnLimit` | 信用卡 rescue：寫入 provider_status 的字串不可超過 4 字元（VARCHAR(4) 防護） | 退場（同上列） |
| 108 | `plication/service/RescueRefundFailurePathTest.java:167` | `rescue_creditCardBusinessFailure_goesThroughHandleRefundFailure` | 信用卡 rescue：handleCreditCardRefund 回應失敗（PayUni 真有回應但業務失敗）走 handleRefundFailure | 退場（同上列） |
| 109 | `main/GrantResponseRulesTest.java:27` | `ambiguousCodes_classifyAsAmbiguous_withoutEvaluatingSupplier` | transport / 不可信回應碼 → AMBIGUOUS，且不評估退款判準 supplier（延遲查詢） | 退場：PayUni 撥款回應碼分類 |
| 110 | `main/GrantResponseRulesTest.java:39` | `nullOrBlank_classifyAsAmbiguous` | null / 空白碼 → AMBIGUOUS（無法確認，fail-safe 不重試；不評估 supplier） | 退場（同上列） |
| 111 | `main/GrantResponseRulesTest.java:47` | `payUniRejectCode_classifyAsDefinitelyNotSent` | 可信的 PayUni 業務拒絕碼（無已完成退款）→ DEFINITELY_NOT_SENT（可安全重試一次） | 退場（同上列） |
| 112 | `main/GrantResponseRulesTest.java:59` | `payUniRejectWithCompletedRefund_classifyAsProviderTookOver` | 可信拒絕 + 存在已完成退款 → PROVIDER_TOOK_OVER（碼不可知主判準，不重試） | 退場（同上列） |
| 113 | `main/GrantResponseRulesTest.java:70` | `grant30023WithoutCompletedRefund_stillDefinitelyNotSent` | GRANT30023 無已完成退款（請款批次未結時間性場景）→ DEFINITELY_NOT_SENT（不收斂、可重試） | 退場（同上列） |
| 114 | `main/GrantResponseRulesTest.java:77` | `ambiguousWithCompletedRefund_staysAmbiguous` | 模稜兩可（transport / 不可信）即使有已完成退款 → 仍 AMBIGUOUS（無可信 PayUni 決定，不得斷言接管） | 退場（同上列） |
| 115 | `main/GrantResponseRulesTest.java:88` | `reconciliationHit_exactTradeNoMatch` | TradeNo 精確相符 → 命中（已入帳，不重發） | 退場（同上列） |
| 116 | `main/GrantResponseRulesTest.java:94` | `reconciliationHit_mismatchOrNull` | TradeNo 不符 / null / 空白 → 未命中 | 退場（同上列） |
| 117 | `main/GrantResponseRulesTest.java:105` | `grantQueryDayComplete_success` | SUCCESS → 該日有效查完（正常回應，續比對 Result[]） | 退場（同上列） |
| 118 | `main/GrantResponseRulesTest.java:113` | `grantQueryDayComplete_noData_isEmptyValidDay` | QUERY03002（查無撥款資料）→ 該日有效查完（有效空頁，續掃下一日） | 退場（同上列） |
| 119 | `main/GrantResponseRulesTest.java:121` | `grantQueryDayComplete_outOfWindowOrUnexpected_notComplete` | QUERY02004（超 30 天窗）/ transport / 其他非預期 / null → 未查完（INCONCLUSIVE） | 退場（同上列） |
| 120 | `frastructure/mq/OutboxEventFactoryExchangeTest.java:35` | `validExchange_isPersistedVerbatim` | 合法 exchange → 原樣寫進 entity（不改寫、不正規化） | 退場：單體用 events.Publish / River，不再有 Rabbit outbox exchange |
| 121 | `frastructure/mq/OutboxEventFactoryExchangeTest.java:49` | `paymentExchange_isAlsoExplicit` | payment 事件走同一支 factory，exchange 一樣是 caller 顯式給的 | 退場（同上列） |
| 122 | `frastructure/mq/OutboxEventFactoryExchangeTest.java:65` | `blankExchange_failsFast` | exchange 為 null／空／任何 whitespace-only → fail-fast，entity 根本產不出來 | 退場（同上列） |
| 123 | `frastructure/mq/OutboxEventFactoryExchangeTest.java:82` | `singleNonBlankCharacter_isAccepted` | 對照組：exchange 只要有一個非空白字元就放行（fail-fast 不是恆擋） | 退場（同上列） |
| 124 | `frastructure/mq/OutboxProducerExchangeContractTest.java:97` | `a1_paymentOutboxWriter` | A1 PaymentOutboxWriter：exchange=payment.events，且 afterCommit 即時投遞仍在 | 退場：同上 |
| 125 | `frastructure/mq/OutboxProducerExchangeContractTest.java:122` | `a1_genericEnqueue_hasNoPaymentMetricAndNoDispatch` | A1 generic enqueue：不遞增 payment 業務 metric、不註冊 afterCommit（timing 交給 caller） | 退場（同上列） |
| 126 | `frastructure/mq/OutboxProducerExchangeContractTest.java:147` | `a1_wrapperDoesIncrementMetricOnCommit` | 對照組：payment wrapper 走 afterCommit 時確實會遞增 metric（證明計數器真的有接） | 退場（同上列） |
| 127 | `frastructure/mq/OutboxProducerExchangeContractTest.java:187` | `a3_duplicatePaymentService` | A3 DuplicatePaymentService：exchange=payment.events，且**不**註冊 afterCommit（純 INSERT） | 退場（同上列） |
| 128 | `frastructure/mq/OutboxProducerExchangeContractTest.java:211` | `a4_rescueRefundConsumer` | A4 RescueRefundConsumer：exchange=payment.events，且**不**註冊 afterCommit（純 INSERT） | 退場（同上列） |
| 129 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:71` | `paymentRow_goesToPaymentExchange` | payment 列 → 投遞到 payment.events | 退場：同上（含已刪的 shipping exchange 列） |
| 130 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:87` | `shippingRow_goesToShippingExchange` | shipping 列 → 投遞到 hawkr.shipping（同一支 publisher、同一張表） | 退場（同上列） |
| 131 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:111` | `exchangeWins_overRoutingKeyPrefix` | routing key 與 exchange 矛盾時，exchange 欄勝出（證明沒有 routing-key 推導） | 退場（同上列） |
| 132 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:137` | `invalidExchange_failsClosed` | exchange 為 null／空／空白 → Rabbit 零互動、走 guarded FAILED、不 fallback | 退場（同上列） |
| 133 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:153` | `invalidExchange_errorMessageIsDiagnosable` | fail-closed 的 last_error 要說得出「是 exchange 的問題」 | 退場（同上列） |
| 134 | `frastructure/mq/PaymentOutboxExchangeContractTest.java:174` | `claimLost_takesPrecedenceOverExchangeValidation` | claim 失敗時即使 exchange 無效，也不寫任何狀態（守衛在 claim 之後） | 退場（同上列） |
| 135 | `frastructure/mq/PaymentOutboxPublisherClaimTest.java:59` | `claimLost_doesNotPublishOrWriteStatus` | claim 回 0（另一個 pod 已認領）→ 不打 MQ、不寫任何狀態、回 false | 退場：platform outbox/River 取代 |
| 136 | `frastructure/mq/PaymentOutboxPublisherClaimTest.java:75` | `claimWon_publishesThenMarksSent` | claim 回 1 → 才投遞；claim 嚴格先於 publishConfirmed，成功後走具名 markSent | 退場（同上列） |
| 137 | `frastructure/mq/PaymentOutboxPublisherClaimTest.java:97` | `notAcked_marksFailedGuarded` | broker 未 ack → 走具名 markFailedGuarded（守衛 status<>SENT），不再 updateById 全欄覆寫 | 退場（同上列） |
| 138 | `frastructure/mq/PaymentOutboxPublisherClaimTest.java:119` | `claimUsesFutureLease` | claim 帶未來租約時間（lease），長度遠大於 publisher-confirm timeout | 退場（同上列） |
| 139 | `frastructure/mq/PaymentOutboxWriterTransactionGuardTest.java:56` | `t1_noTransactionAtAll` | T1｜無交易無 synchronization → throw IllegalStateException，mapper 零互動 | 對照→平台已保護：「事件只在 tx 內發」由 `events.Publish` 的既有守門提供 |
| 140 | `frastructure/mq/PaymentOutboxWriterTransactionGuardTest.java:82` | `t2_fakeSynchronizationIsNotEnough` | T2｜只有 synchronization、沒有實際交易 → 仍然 throw，mapper 零互動 | 對照→平台已保護（同上列） |
| 141 | `frastructure/mq/PaymentOutboxWriterTransactionGuardTest.java:108` | `paymentWrapperAlsoFailsFastOutsideTransaction` | payment wrapper 在交易外呼叫 → 同樣 fail-fast，且業務 metric 不得被計數 | 對照→平台已保護（同上列） |
| 142 | `frastructure/mq/PaymentOutboxWriterTransactionGuardTest.java:134` | `controlGroup_realTransactionFlagIsAccepted` | 對照組｜actualTransactionActive=true → 放行並 insert（證明閘門不是恆擋） | 對照→平台已保護（同上列） |
| 143 | `/PaymentRabbitListenerTopologyRegistrationTest.java:56` | `everyListenerQueueIsRegistered` | 每一條 @RabbitListener 的 queue 都登記在 topology.yaml | 退場：無 Rabbit listener |
| 144 | `/RescueRefundConsumerPayloadTest.java:44` | `rescueCompletedOutboxUsesCanonicalBuyerIdOnly` | （無 DisplayName，見方法名） | 退場：RescueRefund 刪除 |
| 145 | `rsistence/OutboxEnqueueTransactionIntegrationTest.java:126` | `t3_realTransactionIsAcceptedAndRowPersists` | T3｜真 TransactionTemplate 交易內 → enqueue 成功且 commit 後列真的在 DB 裡 | 對照→平台已保護：tx rollback 時事件與 job 不留，由 platform events／jobs 測試已保護 |
| 146 | `rsistence/OutboxEnqueueTransactionIntegrationTest.java:150` | `t4_rollbackRemovesTheOutboxRow` | T4｜交易 rollback → outbox 列必須不存在（真 DB 原子性） | 對照→平台已保護（同上列） |
| 147 | `rsistence/OutboxEnqueueTransactionIntegrationTest.java:173` | `controlGroup_rollbackVsCommitInSameEnvironment` | 對照組｜同一環境下 rollback 的消失、commit 的留下（證明查詢有鑑別力） | 對照→平台已保護（同上列） |
| 148 | `rsistence/OutboxEnqueueTransactionIntegrationTest.java:189` | `outsideTransaction_failsFastEvenWithRealDataSource` | 交易外呼叫（真 DataSource 在場）→ 仍然 fail-fast，DB 零新增列 | 對照→平台已保護（同上列） |
| 149 | `rsistence/OutboxExchangeSchemaContractTest.java:91` | `addsExchangeColumnWithContractShape` | V11 加的是 exchange VARCHAR(64) NOT NULL DEFAULT 'payment.events' | 退場：outbox_events 表不搬 |
| 150 | `rsistence/OutboxExchangeSchemaContractTest.java:103` | `defaultIsPaymentEvents` | DEFAULT 必須是 payment.events（舊 pod 寫入的語意正確值） | 退場（同上列） |
| 151 | `rsistence/OutboxExchangeSchemaContractTest.java:116` | `checkConstraintRejectsAllWhitespace` | CHECK 用 ~ '\\S' 而非 btrim（btrim 版本讓 tab/換行穿過去） | 退場（同上列） |
| 152 | `rsistence/OutboxExchangeSchemaContractTest.java:129` | `v1DidNotHaveExchange` | 對照組：V1 的建表沒有 exchange 欄（證明查詢有鑑別力） | 退場（同上列） |
| 153 | `rsistence/OutboxExchangeSchemaContractTest.java:139` | `entityHasExchangeField` | entity 有對應的 exchange 欄位（String） | 退場（同上列） |
| 154 | `rsistence/OutboxSentCleanupIntegrationTest.java:148` | `c1ToC6_onlyStaleSentRowsAreDeleted` | C1～C6｜只有超過保留期的 SENT 被刪，PENDING／FAILED／DEAD 與新的 SENT 全部留下 | 退場：清理由 River/inbox 平台負責 |
| 155 | `rsistence/OutboxSentCleanupIntegrationTest.java:180` | `c5plus_veryOldDeadIsStillKept` | C5+｜DEAD 即使超過 30 天（dead retention 設定值）也不得被 SENT 清理刪到 | 退場（同上列） |
| 156 | `rsistence/OutboxSentCleanupIntegrationTest.java:194` | `sentWithNullSentAt_isNotDeleted` | SENT 但 sent_at 為 NULL → 不刪（fail-safe：拿不準的列留著） | 退場（同上列） |
| 157 | `rsistence/OutboxSentCleanupIntegrationTest.java:213` | `c7_schedulerUsesConfiguredRetention` | C7｜scheduler 用既有 retention config（設 3 天時，5 天前的刪、2 天前的留） | 退場（同上列） |
| 158 | `rsistence/RefundTotalGuardIntegrationTest.java:153` | `sumRefundedAmount_includesInFlightAndSucceeded_excludesFailed` | L2: sumRefundedAmount 含在途(0/1)+成功(2)，不含 FAILED(3) | 保留→Go：integration：累計退款（在途加成功，不含失敗）不得超過付款金額，以鎖付款列後加總保證；新增並行兩筆退款競爭同一額度的測試（D4）；`hasProcessing`／REFUND_ALREADY_PROCESSING 隨 D4 退場 |
| 159 | `rsistence/RefundTotalGuardIntegrationTest.java:170` | `sumExcluding_atmFullRefund_excludesOwnRow` | L2 排除本筆: ATM 全額退款預建 REQUESTED row 後，排除本筆 → alreadyRefunded=0（不誤擋） | 退場：「排除本筆」是 ATM 預建 REQUESTED 列才需要；Go 以呼叫端冪等鍵取回既有退款列，不需要排除本筆 |
| 160 | `rsistence/RefundTotalGuardIntegrationTest.java:189` | `sumExcluding_stillCountsOtherRefunds` | L2 排除本筆: 排除本筆後仍計入其他在途+成功退款（不放寬 REF-02） | 退場：「排除本筆」是 ATM 預建 REQUESTED 列才需要；Go 以呼叫端冪等鍵取回既有退款列，不需要排除本筆 |
| 161 | `rsistence/RefundTotalGuardIntegrationTest.java:205` | `sumExcluding_nullExclude_includesAll` | L2 排除本筆: excludeRefundNo=null → 退化為含全部（等同 sumRefundedAmount） | 退場：「排除本筆」是 ATM 預建 REQUESTED 列才需要；Go 以呼叫端冪等鍵取回既有退款列，不需要排除本筆 |
| 162 | `rsistence/RefundTotalGuardIntegrationTest.java:218` | `hasProcessingRefund_trueForRequestedAndProcessing` | L2: hasProcessingRefund 對 REQUESTED(0) / PROCESSING(1) 為 true | 退場：`hasProcessing`／REFUND_ALREADY_PROCESSING 隨 D4 退場（並行退款只守累計上限） |
| 163 | `rsistence/RefundTotalGuardIntegrationTest.java:232` | `hasProcessingRefund_falseForSucceededOnly` | L2: hasProcessingRefund 對 SUCCEEDED(2) 為 false（否則部分退款被永久擋死） | 退場：`hasProcessing`／REFUND_ALREADY_PROCESSING 隨 D4 退場（並行退款只守累計上限） |
| 164 | `rsistence/RefundTotalGuardIntegrationTest.java:243` | `hasProcessingRefund_falseForFailed` | L2: hasProcessingRefund 對 FAILED(3) 為 false | 退場：`hasProcessing`／REFUND_ALREADY_PROCESSING 隨 D4 退場（並行退款只守累計上限） |
| 165 | `rsistence/RefundTotalGuardIntegrationTest.java:255` | `trigger_rejectsInsertExceedingTransactionAmount` | L3: 退款總額（在途+成功）超過交易金額的 INSERT 被 trigger 拒絕 | 保留→Go：改寫為退款 use case 的 integration 測試：鎖付款列後加總，超額被拒；不用 trigger（跨列加總單列 CHECK 守不住，計畫不變量表） |
| 166 | `rsistence/RefundTotalGuardIntegrationTest.java:270` | `trigger_allowsPartialUpToLimit_rejectsBeyond` | L3: 多筆部分退款累計未超額 → 允許；再超一元 → 拒絕 | 保留→Go：同上：多筆部分退款累計未超額允許，再超一元拒絕 |
| 167 | `rsistence/RefundTotalGuardIntegrationTest.java:284` | `trigger_failedRefundDoesNotConsumeQuota` | L3: FAILED 退款不佔額度，不阻擋後續在途退款 | 保留→Go：同上：FAILED 退款不佔額度 |
| 168 | `heduler/ExpiredTransactionOutboxTest.java:65` | `casWon_enqueuesToOutboxAfterCas` | CAS 成功 → payment.expired 走 outbox（不再事務外直發），且 CAS 嚴格先於 enqueue | 保留→Go：改寫為 River 週期 job：逾期的付款取消 PaymentIntent（外部呼叫在 job），CAS 與事件同 transaction；CAS 輸掉不發事件、事件寫入失敗要 rollback |
| 169 | `heduler/ExpiredTransactionOutboxTest.java:97` | `casLost_writesNoOutbox` | CAS 回 0（另一個 pod 已關單）→ 一筆 outbox 都不寫 | 保留→Go（同上列） |
| 170 | `heduler/ExpiredTransactionOutboxTest.java:120` | `enqueueFailure_propagatesOutOfTxMethod` | outbox enqueue 失敗必須傳播（讓該筆 tx rollback），不得被吞成 true | 保留→Go（同上列） |
| 171 | `heduler/ExpiredTransactionOutboxTest.java:144` | `notExpirable_skipsTx` | domain 判定不可標逾期 → 不進短 tx、不寫 outbox | 保留→Go（同上列） |
| 172 | `heduler/PendingPaymentReconcileSchedulerClaimTest.java:70` | `claimLost_doesNotEnterReconcileSingle` | claim 回 0（另一個 pod 已認領）→ 連 reconcileSingle 都不進，PayUni 查詢/audit/管道 B 全不發生 | 退場：PendingPaymentReconcile 刪除（Will 2026-11-23） |
| 173 | `heduler/PendingPaymentReconcileSchedulerClaimTest.java:85` | `claimWon_entersReconcileSingleAfterClaim` | claim 回 1 → 才進 reconcileSingle，且 claim 嚴格先於 reconcileSingle（InOrder） | 退場（同上列） |
| 174 | `heduler/PendingPaymentReconcileSchedulerClaimTest.java:109` | `claimUsesLeaseSemantics` | claim 是 lease 不是永久旗標：retryBefore 嚴格早於 stamp 的 now，退避間隔到期該列自動回候選 | 退場（同上列） |
| 175 | `heduler/PendingPaymentReconcileSchedulerClaimTest.java:149` | `leaseBaselineIsTakenPerItemNotPerTick` | R4-P1-2：stamp 取 claim 當下——後一筆的 stamp 嚴格晚於前一筆（共用 tick 起點會轉紅） | 退場（同上列） |
| 176 | `heduler/PendingPaymentReconcileServiceFailureTest.java:111` | `payUniFailure_returnsNoOpAndDoesNotStamp` | PayUni 查詢失敗時回 NO_OP，且不得自行 stamp last_reconciled_at（#2294 H6：退避 stamp 已上移為 Scheduler 的 claim 閘門） | 退場：同上 |
| 177 | `heduler/PendingPaymentReconcileServiceFailureTest.java:128` | `payUniThrows_returnsFailedAndDoesNotStamp` | paymentClient 直接拋例外時回 FAILED，且不得自行 stamp last_reconciled_at（#2294 H6） | 退場（同上列） |
| 178 | `heduler/PendingPaymentReconcileServiceFailureTest.java:143` | `initPathThrows_returnsFailedAndDoesNotStamp` | INIT 路徑 syncPaymentStatus 拋例外時回 FAILED，且不得自行 stamp last_reconciled_at（#2294 H6） | 退場（同上列） |
| 179 | `heduler/PendingPaymentReconcileServiceFailureTest.java:158` | `multipleFailures_areIsolatedAndDoNotStamp` | 連續多筆都失敗時彼此隔離：每一筆各自回 outcome，且沒有任何一筆自行 stamp（#2294 H6） | 退場（同上列） |
| 180 | `heduler/PendingPaymentReconcileServiceFailureTest.java:176` | `rescueOutboxWriteFailure_propagatesSoTxRollsBack` | rescue 事件 outbox 寫入失敗必須傳播使 tx rollback（markRescued 不得孤兒 commit） | 退場（同上列） |
| 181 | `heduler/PendingPaymentReconcileServiceFailureTest.java:207` | `payUniSuccessButNotPaid_skipsRescue` | PayUni 回 SUCCESS 但 TradeStatus 非 1（非衝突）時不應觸發 rescue | 退場（同上列） |
| 182 | `heduler/ReconcileBoundaryIntegrationTest.java:147` | `h7_expiredInsideGrace_shouldBePicked` | H7-1: EXPIRED + expire_at 在 grace 內 → 應被撈 | 退場：同上 |
| 183 | `heduler/ReconcileBoundaryIntegrationTest.java:162` | `h7_expiredOutsideGrace_shouldNotBePicked` | H7-2: EXPIRED + expire_at 超出 grace → 不應被撈 | 退場（同上列） |
| 184 | `heduler/ReconcileBoundaryIntegrationTest.java:178` | `h8_initInsideInitialDelay_shouldNotBePicked` | H8-1: INIT + created_at 在 initial-delay 內 → 不應被撈 | 退場（同上列） |
| 185 | `heduler/ReconcileBoundaryIntegrationTest.java:192` | `h8_initBeyondInitialDelay_shouldBePicked` | H8-2: INIT + created_at 超過 initial-delay → 應被撈 | 退場（同上列） |
| 186 | `heduler/ReconcileBoundaryIntegrationTest.java:208` | `h9_lastReconciledNull_shouldBePicked` | H9-1: last_reconciled_at IS NULL → 應被撈 | 退場（同上列） |
| 187 | `heduler/ReconcileBoundaryIntegrationTest.java:222` | `h9_lastReconciledInsideRetry_shouldNotBePicked` | H9-2: last_reconciled_at 在 retry interval 內 → 不應被撈 | 退場（同上列） |
| 188 | `heduler/ReconcileBoundaryIntegrationTest.java:236` | `h9_lastReconciledBeyondRetry_shouldBePicked` | H9-3: last_reconciled_at 超過 retry interval → 應被撈 | 退場（同上列） |
| 189 | `heduler/ReconcileBoundaryIntegrationTest.java:252` | `rescuedExpired_shouldNotBePicked` | 補充: EXPIRED + rescued_at 已標記 → 不應被撈 | 退場（同上列） |
| 190 | `heduler/ReconcileBoundaryIntegrationTest.java:266` | `paid_shouldNotBePicked` | 補充: PAID 不該被撈 | 退場（同上列） |
| 191 | `heduler/ReconcileBoundaryIntegrationTest.java:279` | `closed_shouldNotBePicked` | 補充: CLOSED 不該被撈 | 退場（同上列） |
| 192 | `heduler/ReconcileBoundaryIntegrationTest.java:292` | `reconcileProperties_defaults` | ReconcileProperties 預設值符合文件（grace=30, retry=5, initialDelay=3） | 退場（同上列） |
| 193 | `heduler/ReconcileSchedulerDisabledTest.java:43` | `disabledFlag_schedulerBeanShouldNotExist` | H10: enabled=false 時 PendingPaymentReconcileScheduler bean 不該被建立 | 退場：同上 |

---

## 8. CodeMeta 值域與錯誤碼

計數：`rg -o -N 'common\.utils\.code\.\w+' $PS | sed 's/.*code\.//' | sort | uniq -c`（14 個值域型別，另外 GrpcResponseCode、CodeMetaValidator、helper 不算值域）；`rg -o -N --no-filename 'GrpcResponseCode\.\w+' $PS | sort -u | wc -l` → 31 個錯誤碼。定義在 `backend/docs/代碼管理（CodeMeta）/`，Java enum 在 `backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/`。

| 值域 | 定義 | 用途 | 提議 |
|---|---|---|---|
| PaymentMethod（2001 卡、2002 Apple Pay、2003 Google Pay、2004 ATM、2900 零元） | 業務類型碼.md:62 | payment_type、金額範圍 | 保留→Go：proto enum `PaymentMethod`（對外）＋模組 Go 型別＋CHECK；刪除 ATM（範圍「不做 ATM」）；Apple／Google Pay 不另列（Stripe 統一為 card，錢包是其屬性；預設，可推翻）；零元保留 |
| PaymentTransactionStatus（0 INIT…6） | 業務狀態碼.md:73 | 交易狀態 | 保留→Go：Go 型別＋CHECK，重新定義（pending／succeeded／failed／canceled／expired）；刪除 CONFLICT、CLOSED_BY_SWITCH |
| RefundStatus（0–3） | 業務狀態碼.md:226、內部技術碼.md | 退款狀態 | 保留→Go：Go 型別＋CHECK＋proto enum（aftersales 顯示） |
| PaymentEventType | 內部技術碼.md:80 | outbox event_type | 刪除（Go 事件型別取代） |
| PaymentRescueAction（8901/8902） | 內部技術碼.md:358 | rescue_action | 刪除 |
| RescueReason | 內部技術碼.md:375 | rescue | 刪除 |
| WebhookProcessStatus | 內部技術碼.md:104 | payment_webhooks | 刪除（inbox 平台） |
| WebhookProvider | 內部技術碼.md:185 | 同上 | 刪除 |
| PayUniAuditDirection | 內部技術碼.md:661 | payuni_audit_logs | 刪除 |
| OutboxEventStatus | 內部技術碼.md:147 | outbox | 刪除 |
| ClientType（MOBILE_APP/WEB_BROWSER） | 業務類型碼.md:26 | redirect_url 組法 | 刪除（Stripe 由 client 給 return_url） |
| AppType（BUYER/SELLER） | 業務類型碼.md:44 | 同上 | 刪除 |
| AccountType | （common） | 呼叫者角色判斷 | 刪除（Connect session） |
| OrderStatus | 業務狀態碼.md | 判斷可付款 | 刪除：不屬 payment；ordering 擁有 |

錯誤碼（GrpcResponseCode.java 行號）：

| 碼 | 名稱 | 提議 |
|---|---|---|
| 8001 | PAYMENT_ALREADY_EXISTS (:137) | 保留→Go：ErrorReason `PAYMENT_ALREADY_PAID` |
| 8002 | INVALID_PAYMENT_TYPE (:138) | 保留→Go：INVALID_ARGUMENT（proto 驗證） |
| 8003 | PAYMENT_STATUS_INVALID (:139) | 保留→Go：ErrorReason `PAYMENT_NOT_REFUNDABLE`（FailedPrecondition） |
| 8004 | REFUND_ALREADY_PROCESSING (:140) | 刪除：D4 並行退款，只守累計上限（`ErrRefundExceedsPaid`），不再有「已有進行中的退款」 |
| 8006 | QUERY_TRANSACTION_FAILED (:142) | 刪除（PayUni query） |
| 8007 | GRANT_FAILED (:143) | 刪除：PayUni 撥款失敗碼；轉帳失敗是 job／轉帳列的狀態，不是對外錯誤碼 |
| 8008 | ATM_CREATION_FAILED (:144) | 刪除 |
| 8010、8017、8018、8019 | CLOSE_STATUS_* (:146,:153-:155) | 刪除（PayUni CloseStatus） |
| 8012 | ORDER_QUERY_FAILED (:148) | 刪除（internal） |
| 8013 | NO_PERMISSION_FOR_PAYMENT (:149) | 保留→Go：NOT_FOUND（不洩漏存在與否，與 ownership 測試一致） |
| 8014 | ORDER_STATUS_NOT_PAYABLE (:150) | 保留→Go：ErrorReason `ORDER_NOT_PAYABLE` |
| 8015 | TRANSACTION_NOT_FOUND_FOR_ORDER (:151) | 保留→Go：NOT_FOUND |
| 8016 | TRANSACTION_NOT_PAID (:152) | 刪除：PayUni 撥款前檢查碼 |
| 8020 | PAYMENT_TYPE_REQUIRED (:156) | 保留→Go：INVALID_ARGUMENT |
| 8021 | PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE (:157) | 保留→Go：ErrorReason `PAYMENT_AMOUNT_OUT_OF_RANGE`（文案走 client 呈現層） |
| 8023、8024、8025 | PAYMENT_DUPLICATE_DETECTED、AMOUNT_MISMATCH、UNSUPPORTED_PAYMENT_TYPE_FOR_DUPLICATE (:158-:160) | 刪除（duplicate 流程） |
| 8027 | PAYMENT_ZERO_AMOUNT_NOT_REFUNDABLE (:162) | 保留→Go：模組內 sentinel（只有內部呼叫者） |
| 8028、8029、8030 | APPLE_PAY_* (:163-:165) | 刪除：Apple Pay merchant validation（D3 兩案皆刪） |
| 9006 | REFUND_AMOUNT_EXCEEDED (:328) | 保留→Go：模組錯誤 `ErrRefundExceedsPaid`（內部呼叫者） |
| 0、400、404、409、500 | SUCCESS、INVALID_PARAM、NOT_FOUND、CONFLICT、INTERNAL_ERROR | 對照→平台已保護：Connect 標準 code |

---

## 9. 現有缺陷與公司時期未完成項

缺陷（讀碼發現，未實測）：

1. **createRefund 在 tx 內持鎖呼叫 PayUni，且失敗時本地紀錄會不見**（`RefundApplicationService.java:60-149`）：見表 5 #6。查詢成功、close 逾時這種情況下，PayUni 可能已經退款，但拋出的例外讓 refund 列 rollback；後續重試走到新的 refund 列，可能**重複退款**（`REFUND_ALREADY_PROCESSING` 擋不到，因為前一列已經 rollback 掉了）。
2. createPayment、syncPaymentStatus、grantPayment 同樣在 `@Transactional` 內呼叫 PayUni 與 order gRPC（`PaymentApplicationService.java:138,454,527`）。grantPayment 在持有列鎖期間重試。
3. 明文 PII 與付款資料寫進 log：`PaymentReturnController.java:101` 把整個 `decryptedData` 寫進 info log；`PaymentGrpcService.java:1188-1196` 同樣；`payment_webhooks.decrypted_payload`、`payuni_audit_logs` 存原文。這與 #2805（KYC 明文 PII 進 log 與 `raw_request`）是同一類問題。
4. `PaymentReturnController`（POST :9924 `/payment/return`）沒有任何 gateway 路由轉到它，推論是死碼；`.env.template:190` 的 `PAYUNI_RETURN_URL=hhttp://…` 拼錯，而且指向 gateway 而不是 service。
5. `GetPublicPaymentStatus` RPC 沒有呼叫者（gateway 只有 client 包裝，`payment_client.go:169`）；PATCH close、GET refund、GET trade refunds 三條 REST 沒有 client 呼叫者（推論，依 rg）。
6. `payuni_mer_id` 由單體回空字串（`sellerserver/server.go:175`），推論 dev 上付款建立已經壞了。
7. 盤點 CSV 過時（已於步驟 0 補 disposition）：rpc-servers.csv:371-374（MembershipBilling）、events.csv:189-192、replicas.csv:46-49（物流、membership）；db-tables.csv:5（20 張表）沒有 disposition 欄，仍是過時計數；rpc-calls.csv 的 gateway 行號已漂移。
8. 計畫 `:345` 說物流「先留在 Java、B10 刪除」，但程式已刪（`:393` 與 commit `5883f499e`）；Flyway V9–V16 的 10 張物流表沒有程式讀寫。
9. `backend-characterization.yml` 已在 commit `cb9ad1cad` 刪除；D2（2026-11-23）決定 B2 不做對舊 Java 的 REST 特性測試，改以 §7 對照表加 Connect 端新測試，差異寫進 `CARRYOVER.md`。
10. 撥款模型矛盾：已由 D1（2026-11-23）解決為 separate charges and transfers；GrantPayment 實作為「訂單完成後建立帶 `source_transaction` 的 Transfer」（D6），B9 只剩手續費與對帳。
11. `RescueRefundFailurePathTest` 記錄 `provider_status VARCHAR(4)` 曾被塞入長字串：schema 欄寬與用途不符的歷史問題（隨重設計消失）。
12. PayUni client 的 `offlineRefund`、`cancelOfflineRefund` 是死碼（`PaymentClient.java:271,333`）。

公司時期未完成項：

- **#2805**：計畫 `:250`；維護者的個人清單「PAYUNi 整合：KYC 明文 PII 進 log 與 `raw_request`」。A3 11c 移交 B2（`backend/docs/plans/2026-09-29-a3-buyer-seller.md:475`）。在 payment 的對應：上面缺陷 3。KYC 部分已隨 PayUni 綁定刪除；在 B2 的意義是新的 Stripe outbound client 不得把 PII／秘密寫進 log 或保存原文。
- **D5 outbound client 的遮罩**：計畫 `:251`；維護者的個人清單「D5 的 outbound client 在寫 log 前遮掉憑證與 PII，測試驗證遮罩確實生效；把請求記成可重跑的 curl 有助除錯外部 provider（參考 vocus 的 requestlogger 做法，不搬程式碼）。與 #2805 同一件事」。現況：Go `internal/modules/payment/internal/account/stripe.go` 沒有 log 呼叫（`rg -n 'log|slog' stripe.go` 無結果），模組不變量寫「payload 與明文 fingerprint 不寫 log」。提議：B2 建一個共用的 provider HTTP client 包裝（遮 `Authorization`、卡號、email、電話），加測試斷言遮罩生效；同時把 #2805 與遮罩這兩項一起結案。

---

## 10. 特性測試範圍

現況（讀碼）：
- 共用套件在 `backend/internal/httpapi/characterize/`（`characterize.go`、`rest.go`、`identity.go`、`fakemedia.go`、`stats.go`）。
- `make new-characterization name=payment legacy=payment-service`（`backend/Makefile:328`）會從 `backend/cmd/new-module/templates/characterization/` 產生 `driver_test.go`、`scenarios_test.go`、`connect_test.go`、`rest_test.go`、`gateway_test.go`、`monolith_integration_test.go`、`legacy.sh`。
- 執行：`backend/scripts/characterize.sh payment`（本機起單體＋gateway＋`legacy.sh` 起的舊服務；只用 infra 的 Postgres／Redis）。
- 唯一現存的例子是 `backend/internal/httpapi/social/characterization/`，但它的 legacy 側（REST driver、gateway_test、legacy.sh）與 CI workflow 已在 `cb9ad1cad` 刪除，只剩 Connect 側。要看 REST driver 的寫法，得看 `cb9ad1cad^` 的版本或模板 `rest_test.go.tmpl`。

payment 的特殊困難：舊服務的業務流程依賴 PayUni（sandbox 需要憑證、網路）。characterize.sh 是 `env -i`、不連外部的。所以 REST 驅動舊 gateway 時，只有**不碰 PayUni 的 endpoint**能跑；建立付款、退款、同步都會打 PayUni。也因為換了 provider，新舊行為本來就不同（Stripe 回 client_secret，PayUni 回表單 URL）。

REST driver 必須涵蓋（兩邊行為應一致的部分）：

| 情境 | REST（舊 gateway） | Connect（單體） | 可行性 |
|---|---|---|---|
| 取得自己的付款（GET /app/payments/{trade_no}） | ✓ | GetPayment | 需要 fixture：直接寫 payment DB（legacy.sh 套 Flyway 後 INSERT），不經 PayUni |
| 他人付款回 NOT_FOUND（ownership） | ✓ | ✓ | 同上 |
| 付款歷程：buyer、seller 本店、seller 他店 NOT_FOUND、seller 遮蔽 buyer PII | ✓（`/orders/{id}/history`） | ✓ | 同上；需要 order 回 store_id（GetOrderForPayment → Java order 也要起，或改寫情境） |
| 後台：付款列表／詳情、退款列表／詳情 | ✓ | admin Connect | fixture 可行 |
| 零元訂單建立付款 → PAID | ✓（不打 PayUni，但要打 order gRPC） | ✓ | 要起 Java order，或標為與舊實作的差異 |
| 建立付款（卡）、退款、狀態同步、webhook | ✗（PayUni） | 只有 Connect＋fake provider | 寫進 `CARRYOVER.md`「與舊服務的差異」，不寫成共用情境 |
| PayUni 專屬路由（redirect、return、apple_pay、simulator、webhook） | — | — | 刪除，不寫情境 |

需要補的：恢復（或重寫）`.github/workflows/backend-characterization.yml`（缺陷 9）；寫一個能套 payment Flyway 並播種的 `legacy.sh`；決定是否也要起 Java order-service（GetOrderForPayment），不起的話，依賴 order 的情境要列為差異。

---

## 無法確定的事項

- 付款流程在 dev 上是否真的壞了（推論自 `payuni_mer_id` 空字串，沒有跑）。
- 表 1 的「無 client 呼叫者」是依 rg 得出的負面結論，沒有排除動態組字串的路徑。
- DuplicatePaymentService：已核對（見 §6 最後一項），呼叫者是 reconcile 與 PaymentNotifyHandler 的 PayUni 專屬分支。
- 撥款模型、ATM、Apple Pay 的去向與 REFUND_ALREADY_PROCESSING 是否保留：已由 D1、D4、範圍「不做 ATM」與 D3 解決；標「預設，可推翻」的列是計畫沒有回答的取捨，列在步驟 0 的回報。
