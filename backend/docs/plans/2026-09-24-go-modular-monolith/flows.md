# 跨服務一致性流程盤點（合併為單一 DB modular monolith 前）

範圍：`backend/` 22 個 deployable（12 Go + 10 Java）＋ gateway。排除測試與生成碼。
方法：程式碼為準、設計書／ADR 為輔。A1–A3 由子分析產出並已對過程式碼；D1–D6、C1–C3 由子分析產出，主 agent 抽驗 8 項結論皆相符（`V4` 非唯一索引、`sellers.rating` version bug、membership adapter 全 throw、`openUnresolved` 零呼叫端、`bindInvoiceDocument` 零業務呼叫端、`SellerMerchantIdResolver` 無實作、`logistics.cost.incurred` 直發無 outbox、seller_ranking 月份回退）；B1–B3、E1–E2 由主 agent 直接讀碼。

路徑縮寫：B=`backend`；ORD=`B/services/java/order-service/src/main/java/io/github/will413028/hawkr/order`；AS=`B/services/java/aftersales-service/src/main/java/io/github/will413028/hawkr/aftersales`；PAY=`B/services/java/payment-service/src/main/java/io/github/will413028/hawkr/payment`；SHIP=`B/services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping`；INV=`B/services/invoice-service/internal`；SEL=`B/services/seller-service/internal`；ACC=`B/services/account-service/internal`。

判定「單一 DB tx 可否移除」：**yes**＝機制只因拆服務存在，可整段換成同 tx；**partly**＝跨服務那一段可移除，但外部 provider（PayUni、AMEGO、Uber、Apple、R2、FCM）那一段仍需 durable job／pre-commit／lease；**no**＝機制本身是外部整合或業務需求。

## 0. 總表

| # | 流程 | deployables（依序） | 現行機制 | 單一 tx 可移除？ |
|---|---|---|---|---|
| A1 | checkout 建單 | gateway→order→(product,seller,buyer,shipping,coupon,invoice 讀)→cart→product→coupon | Redis checkout token；cart LOCKED；`stock_tx_records`＋`stock_transaction` PK 去重＋RestoreStock RPC＋StockCompensationWorker；`cart_cleanup_retry_tasks`；reserveCoupon＋CAS 回寫；`coupon_consumption_tasks`；coupon AutoReleaseJob | **yes**（外部呼叫只有 Uber quote） |
| A1b | 建立付款 | gateway→payment→order(snapshot)→PayUni | payment tx 內 RPC `SnapshotPaymentInfo` 再打 PayUni；gateway 再 best-effort 寫一次 | **partly**（snapshot 同 tx；PayUni 需 pre-commit） |
| A2 | 付款成功→PAID→建物流單 | PayUni→payuni-webhook→payment→order→shipping→order | `webhook_inbox` 兩段 tx＋outbox；`payment_webhooks`；payment `outbox_events`；`payment_event_dedup`；`order_outbox`；ShipmentCreationBackstopScheduler；PaymentDeadlineScheduler 反查 | **partly** |
| A3 | RFQ 接受→建單 | gateway→ai→(outbox worker)→order→ai | `procurement_order_provisioning_outbox`＋`acceptance_id` UNIQUE＋CAS（ADR rfq-acceptance-order-atomicity） | **yes** |
| B1 | 取消（待付款／售後內部／逾時／停權） | buyer/aftersales/scheduler→order→product(MQ)、coupon(RPC)、payment(RPC) | order tx 內 CAS＋`order_outbox`(order.cancelled)→product 歸還庫存（`(stockTxId,RESTORE)` 去重）；coupon release／payment close 為 tx 外 best-effort；coupon AutoReleaseJob 兜底 | **yes**（payment close 若打 PayUni 則 partly） |
| B2 | 賣家取消已付款單＋售後退款 | aftersales→order(RPC)→payment(RPC)→PayUni→payment(MQ)→aftersales/order(MQ)→invoice(MQ) | aftersales `@Transactional` 夾跨服務不可逆 RPC；`refund_tasks`＋RefundTaskWorker(lease)；`refund.completed` 直發＋RefundSyncWorker 無限期重發；order `financial_logs` UNIQUE 去重；`auto_refund_tasks`；payment `payment_refunds.idempotency_key` | **partly** |
| B3 | 賣家撥款（payout） | order→aftersales(RPC hold)→payment(RPC)→PayUni | `grant_payment_tasks`＋GrantPaymentWorker(lease)；payment 持 FOR UPDATE 跨 PayUni Grant＋grant_query 對帳 | **partly** |
| C1 | 發票開立／折讓／更正 | order→(MQ)→invoice→(gRPC 讀 order、payment)→AMEGO→notification | `order_outbox`→MQ→`invoice_inbox_events`（雙重去重、DEFERRED、INV.dl 兜底重掃）；`invoice_operations` lease；Revise RPC＋`revision_idempotency_key`＋60s hold | **partly** |
| C2 | 物流費／成本 true-up | order→shipping→Uber/PayUni→payuni-webhook→shipping→payment→order | `logistics.cost.incurred` 直發（無 outbox）；`logistics_cost_facts` event_id；N16 outbox 雙路；collection attempt pre-commit＋lease；月結 reconcile | **partly** |
| C3 | 會員訂閱計費 | buyer/seller→payment→PayUni→payuni-webhook→payment→(無 consumer) | `billing_commands` 冪等＋UNKNOWN＋V7/V8 lease；`membership_charge_attempts`；`payment.membership.*` outbox | **partly**（整條休眠） |
| D1 | 賣家入駐／KYC | gateway→seller→account；seller→PayUni；shipping→seller | `BindSeller` RPC＋`seller_deleted_outbox`(seller.onboarded) level-triggered 重授；`account_roles.source_id` 柵欄；`merchant_verification_outbox`；`payuni_merchant_profile_outbox`＋updates＋version CAS；payuni_sync 輪詢 | **partly** |
| D2 | 帳號刪除 cascade | seller/buyer→order(RPC 未結訂單)→account(RPC RemoveRole)→MQ fan-out（product,coupon,social,im,notification,crm,pricebook,media） | `seller_deleted_outbox` 兩段 checkpoint；`buyer_deleted_outbox`；`rolestate` OtherRoleState 協定；`account.deleted` 直發 best-effort；`apple_revoke_outbox` | **partly**（角色／帳號 yes；Apple／R2／FCM no） |
| D3 | ADM 代行 | gateway→account(intent)→seller/product/social/order(寫)→account(confirm)→notification | 兩段 intent→寫入→confirm；`admin_operation_outbox` lease；`DelegationContext`＋`audit_log_id` 契約 | **yes**（大部分） |
| D4 | 賣家停權 cascade | seller→(outbox)→product,coupon,order,notification,im | `product_suspension_snapshot`、`disabled_by_suspension`、order 取消未付款單 | **yes**（product/coupon；order partly） |
| D5 | 商品匯入 commit | product-import→product→media(R2) | `idempotency_request`（24h）、`product_import_commit_attempt`、`media_callback_inbox`、GC jobs | **partly** |
| D6 | 跨服務計數器 | order/buyer/review/seller→product/seller/ai/crm | MQ＋dedup 表（sales/favorite/seller_ranking_processed_events）、full-recompute、social counter.delta＋reconciler | **yes**（多數；hot row 例外） |
| E1 | 媒體引用 | 12 個 owner 服務→media | `asset_reference` 靠 Link（commit 前）→業務寫入→ReleaseReferences（best-effort）順序契約 | **partly**（引用 yes；R2 刪除 no） |
| E2 | gateway 協調的複本寫入 | gateway→order／seller | `SnapshotPaymentInfo` 重寫、`RecordStoreVisit` goroutine | **yes** |

## A. 下單／付款／RFQ（子分析，已對碼）

### A1 checkout confirm → 建單
- 機制：Redis 一次性 checkout token（preview 快照，30 分 TTL，`ORD/checkout/CheckoutService.java:73`）→ cart.checkoutLock（LOCKED＋`lock_expire_time`，cart UnlockExpiredItemsJob 兜底）→ tx 外 autocommit 寫 `stock_tx_records` INIT → product `DeductStock`（`stock_transaction` PK 去重）→ 建單 tx（orders/order_items/`order_invoice_snapshots`，stock_tx→SUCCESS）→ 失敗時 RestoreStock RPC＋StockCompensationWorker（60s）→ commit 後 `cart_cleanup_retry_tasks`＋CartCleanupRetryWorker → Uber quote CAS → reserveCoupon（orderNo 冪等）＋CAS 回寫 `coupon_instance_id` → 付款成功 tx 寫 `coupon_consumption_tasks`＋CouponConsumptionWorker；coupon AutoReleaseJob 對帳。
- 部分失敗：建單 commit 後、reserveCoupon 前崩潰 → 訂單 discount>0 但 coupon 未鎖、可重用（無兜底）；寫 cart cleanup task 前崩潰 → 已下單商品回到購物車。
- 單一 tx：**yes**。可刪 `stock_tx_records`、product `stock_transaction`、StockCompensationWorker、RestoreStock RPC、order.cancelled→product 歸還 consumer、`cart_cleanup_retry_tasks`＋worker、`coupon_consumption_tasks`＋worker、coupon AutoReleaseJob、CAS 回寫。cart LOCKED 可換 `SELECT … FOR UPDATE`。保留 Redis preview 快照、coupon RESERVED/USED 業務狀態、checkout_id 冪等（建議改 UNIQUE）。

### A1b 建立付款
- payment `PaymentApplicationService`（`PAY/application/service/PaymentApplicationService.java:277`）在 createPayment tx 內 RPC order `SnapshotPaymentInfo`，再打 PayUni；PayUni 失敗 payment rollback 但 order 快照不回滾。gateway `gateway/internal/handler/payment.go:224` 付款建立後**再 best-effort 寫一次**（錯誤忽略）。另 `AtmReactivationService:117`、`DuplicatePaymentService:75` 也寫。
- 單一 tx：**partly**。snapshot 同 tx（或改 join `payment_transactions`）；PayUni 不能進 tx → 先 commit INIT（trade_no 為 pre-commit key）再 tx 外呼叫。

### A2 PayUni 付款成功 → PAID → 建物流單
- 鏈：PayUni → payuni-webhook（`webhook_inbox` UNIQUE、兩段 tx、`outbox_events`，ADR webhook-two-phase-transaction）→ MQ → payment（`payment_webhooks` 第二層 inbox、version CAS、`outbox_events` afterCommit＋5s 兜底）→ MQ → order（`payment_event_dedup`、FOR UPDATE＋CAS、`order_outbox` 寫 order.paid／order.sales.accumulated）→ shipping `createShipmentFromOrder`（tx 內呼叫 PayUni／Uber 物流，commit 前發 shipment.created 且吞例外）→ order 回寫 shipment_no；ShipmentCreationBackstopScheduler（5min）→ 放棄時 `order.shipment.creation_failed` 交 aftersales 取消退款。逾期未付走 PaymentDeadlineScheduler＋RPC 反查付款（後端責任邊界規範 §七「同步前置 gate」）。
- 缺口：shipment.created 遺失 → backstop 撞冪等提前 return 不重發 → 最後把已建物流單的訂單取消退款；CREATE_FAILED 空殼讓 backstop no-op；Uber 在 tx 內、insert 前呼叫（C2）。
- 單一 tx：**partly**。可刪 `payment_webhooks`、webhook outbox→payment MQ、`payment_event_dedup`、payment.success→order 投遞、coupon task、dispatch 時 gRPC enrich、orders 物流投影欄（改 join shipments）。保留 `webhook_inbox`（外部 at-least-once 入口）、建物流單 durable job（PayUni／Uber 不能進 tx）、`auto_refund_tasks`、`grant_payment_tasks`、payment 對帳排程、notification／invoice／CRM 非同步 fan-out。

### A3 RFQ 接受報價 → 建 RFQ 訂單
- gateway → ai（同 tx：FOR UPDATE＋CAS＋acceptance＋`procurement_order_provisioning_outbox`）→ `pkg/outbox` Worker → order `CreateRfqOrder`（`orders.acceptance_id` UNIQUE 冪等）→ ai CAS `AttachOrderToAcceptance`；中間態 CREATING_ORDER／ORDER_FAILED／RETRY_EXHAUSTED 對買家可見（ADR 明定）。
- 缺口：ai 無 order.paid／order.cancelled consumer，`MarkAcceptancePaymentFinalized`／`ReleaseAcceptance` 無呼叫端（影響 UNVERIFIED）。
- 單一 tx：**yes**（ADR 自陳「若引入跨服務交易可重看」）。可刪 outbox 表、worker、中間狀態與可觀察窗口。

## B. 取消／退款／撥款（主 agent 讀碼）

### B1 取消訂單（未出貨）
- 入口與順序：
  - 買家取消待付款：order gRPC `cancelOrder`（`ORD/grpc/OrderGrpcService.java:481`）→ `OrderService.cancelOrder`（`ORD/service/OrderService.java:453`，`@Transactional`：FOR UPDATE＋CAS＋`insertCancelledOutbox` :522）→ tx 外 `closePaymentForOrder`（payment RPC，best-effort）→ `releaseCouponForOrder`（:1476，coupon RPC，catch-all 吞錯）。
  - 售後內部取消：`cancelOrderInternal`（:2027）→ `cancelOrderInternalInTx`（`OrderService.java:491`，CAS＋outbox）→ tx 外 releaseCoupon best-effort。
  - 逾時未付：PaymentDeadlineScheduler → `OrderExpiryCancellationService.cancelPendingOrder`（先反查付款 SoT）→ 同上＋coupon release（:172）。
  - 停權：order `SellerSuspendedConsumer` 逐筆 CAS 取消 PENDING_PAYMENT（見 D4）。
- 庫存歸還：`order.cancelled` 只在「有 SUCCESS 扣庫存紀錄」時落 outbox（`OrderService.java:522-528`）→ product `OrderCancelledConsumer`（`(stockTxId, RESTORE)` 去重）。另 invoice／notification 也消費（invoice 因此拿到不完整的取消集合，見 C1）。
- 部分失敗：coupon release 失敗只 log，靠 coupon AutoReleaseJob 對帳釋放；payment close 失敗推測靠 payment ExpiredTransactionScheduler 到期（UNVERIFIED）。庫存歸還走 outbox，無遺失。
- 單一 tx：**yes**。取消 CAS、庫存歸還、coupon 釋放可同 tx（刪 order.cancelled→product consumer、`stock_tx_records` RESTORE 相位、AutoReleaseJob 的取消對帳部分）。payment close 若只是本地狀態可同 tx；若需通知 PayUni 則維持 tx 外 job（UNVERIFIED：`paymentCloseService.closePayment` 是否打 PayUni）。

### B2 賣家取消已付款單＋售後退款（refund_tasks）
- 賣家取消：`AS/service/SellerCancelService.java:63 cancelShipment`（`@Transactional`）：
  1. RPC order `getOrderForAfterSales`（:69）讀快照 → insert `seller_cancel_requests`（抄 transaction_id／mer_id／payment_method／refund_amount）。
  2. RPC order `cancelOrderInternal`（:122）——**order 自己 commit，對 aftersales 而言已不可逆**（程式註解 INV-5「訂單已跨服務取消（不可逆）」）。
  3. ATM：RPC payment `createRefund`（key `CANCEL:{cancelId}:{attempt}`，預建 `payment_refunds`）；失敗時落 FAILED refund_task 不 rollback。
  4. insert `refund_tasks`、cancel→REFUND_PROCESSING、publish awaiting_buyer_page（直發）。
  - 系統取消 `cancelShipmentBySystem`（:217）由 `order.shipment.creation_failed`／`order.fulfillment.failed`／`shipment.processing.timeout` 事件觸發，以 `seller_cancel_requests` 查重冪等。
- 售後案件退款：`AfterSaleResolutionService`（:519 insert refund_task）／`PlatformDecisionService`（:222）；優惠券補償在 aftersales tx 內 RPC coupon `grantSellerCompensationCoupon(caseNo)`（:347）；退貨單在 tx 內 RPC shipping 建退貨物流單（:160-188，PayUni 物流）。
- 執行：`AS/scheduler/RefundTaskWorker.java`（lease on `next_retry_time` 120s）→ RPC payment `createRefund(refundNo 冪等)` → `markTaskDoneAndAdvance`（:236：DB update 後**直發** `refund.completed`／`cancel.completed`，無 outbox）。`RefundSyncWorker`（每小時、lease 3h on `resync_at`）把所有 DONE task **無限期重發**（檔頭自陳無終止條件），order 端以 `financial_logs UNIQUE(type, external_txn_id)` 吸收重複。
- payment：`PAY/application/service/RefundApplicationService.java:60 createRefund`（`@Transactional`）：FOR UPDATE canonical 交易 → 驗金額 → insert `payment_refunds`（idempotency_key）→ **在同一 DB tx 內呼叫 PayUni 退款**（無 pre-commit）→ `refund.succeeded/failed` 直發（非 outbox，36 §3.2）。ATM 改走 PayUni 頁面版，結果經 `payuni.refund.notify/accepted` webhook → `handleAtmRefundWebhook`（:418）；aftersales `AtmRefundReconciler` 在 revert 前同步反查 payment。
- order：`AfterSalesEventConsumer.onRefundCompleted`→`AfterSalesRefundEventHandler.handleRefundCompleted`（`ORD/mq/AfterSalesRefundEventHandler.java:56`）：`financial_logs.insertIdempotent`、累加 `orders.refund_total_amount`、狀態 FULLY/PARTIALLY_REFUNDED、同 tx 寫 `order.refund.finalized` outbox→invoice。`onCancelCompleted`→`SellerCancelRefundTxHandler`。另 `auto_refund_tasks`（`ORD/mq/PaymentEventHandler.java:330`：已取消訂單又收到付款）→ AutoRefundWorker → payment createRefund。售後倒數：`aftersales.case.*` 事件 → order `AfterSalesCountdownConsumer` 暫停／恢復自動完成倒數，`CountdownRecoveryWorker` 兜底。
- 部分失敗（現況）：
  - (a) 步驟 2 成功後 aftersales 本地 tx 失敗 → 訂單已 CANCELLED，但 `seller_cancel_requests`／`refund_tasks` 被 rollback → **已付款卻無退款任務**（推論自交易邊界，未見兜底；UNVERIFIED 是否有 admin 異常清單可撈）。
  - (b) coupon 補償／退貨物流單 RPC 成功、aftersales tx 失敗 → 券已發或退貨單已建但 case 仍 PENDING（coupon 端是否以 caseNo 去重：UNVERIFIED）。
  - (c) payment createRefund：PayUni 成功但 DB commit 失敗 → 退款已執行、`payment_refunds` 無紀錄；重試以同 refundNo 查不到 → 可能重打 PayUni（PayUni 是否擋：UNVERIFIED）。
  - (d) `refund.completed` 直發失敗 → 依賴 RefundSyncWorker 重發，延遲 ≈ DONE 總數/50 × 3h，隨單量線性成長。
  - (e) 信用卡退款僅重試 1 次（`CC_RETRY_LIMIT`）即終態 FAILED，進 admin 異常清單。
- 單一 tx：**partly**。
  - 可移除：cancel request＋order 取消＋refund task 同 tx（消除 (a)）；coupon 補償同 tx（消除 (b)）；`refund.completed`／`cancel.completed`／`refund.succeeded/failed` 的 MQ 與 RefundSyncWorker、`financial_logs` 當去重用途、order/aftersales 兩個 RefundResultConsumer、AtmRefundReconciler 的 RPC 反查（改同庫查）、倒數暫停／恢復事件與 CountdownRecoveryWorker；`refund_tasks`＋`auto_refund_tasks`＋`payment_refunds` 可收斂成一張退款命令表。
  - 必須保留：退款命令表＋lease worker（PayUni 退款在外部）；payment 端應改成「pre-commit 退款列 → tx 外呼叫 PayUni → 收尾 tx」（(c) 是既有問題，合庫不會自動修好）；PayUni refund webhook inbox；退貨物流單的 provider 呼叫。

### B3 賣家撥款（payout / grant）
- 建立：`grant_payment_tasks` 在訂單完成 tx（`ORD/service/OrderCompletionTxHandler.java:106`）或 N16 責任判定 tx（`ORD/service/ResponsibilityFinalizedTxHandler.java:112`）內建立。
- 執行：`ORD/worker/GrantPaymentWorker.java`（lease on `next_retry_time`）→ RPC aftersales `getPayoutHold`（:145，失敗一律視為 hold，fail-closed）→ RPC payment `grantPayment`（:152）→ `PaymentApplicationService.grantPayment`（`@Transactional`，FOR UPDATE 持鎖跨 PayUni `/api/merchant/grant`；`payment_transactions.grant_status` 為 SoT；非首撥先 grant_query 對帳）→ order `markGrantTaskDone` → 直發 `payout.completed`（無 DLQ，notification＋SSE）。帳本（`ledger_*`）只屬物流財務線，撥款不入帳本。
- 部分失敗：PayUni 成功但 order 標記失敗 → 下輪重打，payment 以 grant_status SUCCESS 短路（冪等）；payment tx 在 PayUni 成功後 rollback → grant_status 未落地，下輪 grant_query 對帳補記（設計內）。
- 單一 tx：**partly**。可移除 `getPayoutHold` RPC（改同 tx 讀 aftersale_cases）、grant task 與 grant_status 的雙重狀態（可合一）；保留 grant 命令表＋lease＋grant_query 對帳（PayUni 外部）、payout.completed 通知。

## C. 發票／物流財務／會員計費（子分析，主 agent 抽驗）

### C1 發票開立、退款折讓、更正
- 順序：CheckoutConfirm 同步 gRPC invoice `validateOrderInvoiceInput`／`resolveOrderInvoiceMode`（`ORD/checkout/CheckoutService.java:1525/:1607`）→ 建單同 tx 寫 `order_invoice_snapshots`（`ORD/service/OrderCreationTxHandler.java:101`）→ 付款成功同 tx 寫 `order_outbox`(order.paid) → MQ → invoice C1 consumer（`INV/mq/order_paid_consumer.go:65`）：inbox 獨立 tx 先 commit（雙重去重 `uq_invinbox_event_id`＋`(event_type,business_dedup_key)`）→ 同步 gRPC 讀 order `GetOrderInvoiceSnapshotInternal`、payment `GetPaymentFactsInternal` → `Issuance.Prepare` 同 tx 寫 documents／lines／operations（`INV/usecase/issuance.go:214`）→ INV.op lease worker 送 AMEGO（`INV/usecase/submit.go:86`）→ 成功同 tx 寫 `invoice_outbox_events` → notification。
  - 退款：`order.refund.finalized`（`ORD/mq/OrderRefundFinalizedOutboxWriter.java:120`）→ C2（`INV/mq/refund_finalized_consumer.go:107`）；找不到 document 落 DEFERRED，等 order.paid 到時 `reclaimDeferred` 撿回 → `invoice_adjustments` ALLOWANCE／VOID → AMEGO。鍵 `REFUND:{refund_ref}`，禁止用 event_id（RefundSyncWorker 每次換 UUID）。
  - 取消：order.cancelled → C3，inbox 與 `invoice_cancelled_orders` 同 tx。
  - 更正：`INV/usecase/correction.go:174` 三段：60s Hold → tx 外 RPC order `ReviseOrderInvoiceSnapshotInternal`（`revision_idempotency_key`，order V13）→ 落地 tx。
- 部分失敗：outbox DEAD 後無自動救援；C1 gRPC 失敗時 inbox 已 commit、MQ 重投被去重吞掉，只能靠 INV.dl 每 5 分鐘兜底重掃（上線前 dev 卡 4 筆無告警）；退款先於付款到達且付款永不來 → 無出口；AMEGO 逾時 → WAITING_CONFIRMATION recheck。平台→賣家物流費發票未接線（`BuildLogisticsFeeInvoice` 無呼叫端，`logistics_receivables.invoice_document_id` 恆 NULL）。
- 單一 tx：**partly**。可移除 order_outbox→MQ→inbox 三條、`business_dedup_key`、DEFERRED/reclaim（亂序只因 MQ）、INV.dl 兜底、四支唯讀 gRPC、Revise RPC＋三段 tx、`invoice_cancelled_orders`、`invoice_business_day_mirror`。建議付款 tx 內寫一列「開票 intake job」（付款 commit 不綁開票決策）。保留 `invoice_operations`（operation_key、lease、`provider_request_no`、unknown/recheck）、provider audit、`invoice_idempotency_keys`（對外 API）、通知 outbox（可改同庫 job）。
- 約束：ADR `2026-09-18-invoice-service-architecture.md` ADR-01 以「AppKey 憑證邊界、5 年法定保存與獨立 migration 節奏、AMEGO 故障隔離、單副本 lease 排程、發票狀態唯一 SSOT」選擇獨立服務——合併後這些要改由 module 邊界、獨立 schema 與獨立 worker pool 維持。

### C2 物流費與成本 true-up（Uber、PayUni）
- 順序：order.paid／backstop → shipping `createShipmentFromOrder`（`SHIP/service/ShippingService.java:269`，`@Transactional`，**tx 內呼叫 Uber Create**、之後才 insert shipments :343）→ Uber webhook 經 payuni-webhook（inbox T1＋outbox T2）→ shipping 狀態 CAS → leg terminal 時**直發** `logistics.cost.incurred`（`SHIP/mq/ShippingEventPublisher.java:164`，無 outbox）→ payment `LogisticsCostIncurredConsumer:66` 寫 `logistics_cost_facts`（V16）→ converge；ADM 經 gateway `finalizeResponsibility`（`PAY/.../ResponsibilityCaseService:425`，同 tx CAS＋N16 outbox）→ N16 雙路：payment 自己消費（converge→createReceivable）＋order（35→54、建 grant task）→ `LogisticsCollectionScheduler` → `submitOne`（`LogisticsCollectionService:303`，Propagation.NEVER：pre-commit attempt 先 commit 再打 PayUni agent_fee）→ query → SETTLED／UNCONFIRMED；月結 `reconcileOne` 寫 adjustments＋ledger。
- 部分失敗：Uber 成功但 insert rollback → 孤兒 Uber 單；ADR `2026-09-14-uber-create-idempotency-precommit-key.md` 第一層（呼叫前落地 claim 列）**未實作**（`uber_deliveries` 零 writer），order.paid consumer 與 backstop 併發 read-then-act 可能派兩趟車；cost.incurred 發送失敗即遺失且月結匯入無呼叫端 → 無自動補回；`openUnresolved` 零呼叫端 → 正常送達的 leg 永停 AWAITING_RESPONSIBILITY、不建應收（與 70 號設計不符）；`SellerMerchantIdResolver` 無實作 bean、`auto-submit-enabled` 預設 false → 收款整條 no-op（賣家 MerID 在 seller-service，屬拆分造成的缺讀）；order 端 N16 失敗進 DLQ、訂單停在 35。
- 單一 tx：**partly**。可移除 cost.incurred MQ 與 event_id 去重（terminal CAS 同 tx 寫 cost_facts＋converge，順便補遺失缺口）、N16→order（35→54 併入 finalize tx）、order 建物流單 backstop（改付款 tx 內一列 job）、`orders.shipment_*` 投影、`getOrderForShipping`、MerID 查詢改 join、webhook outbox→MQ。保留 Uber pre-commit＋991 CREATE_UNKNOWN＋回查 lease、collection attempt pre-commit＋lease＋UNCONFIRMED、receivable `idempotency_key`／`provider_delivery_ref` UNIQUE、`webhook_inbox`、ledger、月結 true-up（業務需求）。
- 約束：ADR `2026-09-11-logistics-finance-domain-ownership.md`——shipping 只發成本事實、不持有應收；Logistics Finance 9 張表不得與 business payment 表 JOIN、`io.github.will413028.hawkr.payment.logistics.**` 不得被結帳／退款 hot path import（ADR 明寫「併入同一服務後只剩 package 邊界與 gate」）。

### C3 會員訂閱計費與 entitlement
- 設計順序：buyer/seller → payment `CreateMembershipSubscription`（request_id 冪等）→ `billing_commands` PROCESSING → tx 外呼叫 provider → SUCCEEDED/UNKNOWN；每期 PayUni webhook → `ChargeResult.apply`（subscription FOR UPDATE、authoritative winner、`membership_charge_attempts`、outbox `payment.membership.{buyer|seller}.{success|failed}`）；V7 partial unique 每 subscription 一個 active command、V8 `lease_expires_at`。
- 實際：**整條休眠**。`PayUniSubscriptionProviderAdapter` 全部 throw `PROVIDER_CAPABILITY_UNSUPPORTED`；payment 以外無呼叫端；`payment.membership.*` 無 consumer；`pkg/membershipkernel` 未使用；`buyer_memberships.source`／`seller_membership_subscriptions.source` CHECK 只允許 admin／system_default（會員狀態只由 ADM AssignMembership 本地寫）。ai-service 每輪同步 gRPC `GetBuyerEntitlement`（fail-closed，ADR buyer-membership-entitlement-model D4），配額 reserve／confirm／release。seller AssignMembership 同 tx 寫 lifecycle outbox → `seller.membership_changed` → ai `seller_ranking_facts`。
- 單一 tx：**partly**。可移除 `payment.membership.*` outbox→MQ→consumer（扣款 winner 與 entitlement 異動同 tx，最大收益）、`membership_id/subject_id` 改 FK、`seller.membership_changed`＋`seller_ranking_facts` tier 欄、`GetBuyerEntitlement` RPC。保留 `billing_commands`（冪等、UNKNOWN、lease——因 PayUni 在外部）、`membership_charge_attempts`＋`provider_event_key`、`webhook_inbox`、usage reservations（外部 LLM）、quote 快照。UNKNOWN 無排程對帳（只在呼叫端重試時對帳）是既有缺口。

## D. 身分／治理／匯入／計數器（子分析，主 agent 抽驗）

### D1 賣家入駐／KYC
- 開店：seller `CreateSeller`（`SEL/usecase/seller.go:213`）同 tx 寫 `sellers`＋`seller_deleted_outbox`(seller.onboarded)（:264）→ tx 後同步 RPC account `BindSeller`（:289；`ACC/usecase/bind_seller.go:44`：`account_roles` ON CONFLICT DO NOTHING、`RepointAccountRoleSource(source_id=store_id)`、重算 `account_type`、`roles_epoch`）→ outbox worker level-triggered 重授並發 SSE。
- KYC：`Submit`（`SEL/usecase/certification.go:339`）tx 前 RPC account `GetAccountInfo`、social `HasPublishedPost`、media `LinkImages`（:485）→ 本地 tx → `ReleaseReferences` best-effort。`Approve`（:748）：`GetByAccountIDForUpdate` 在 tx 外（鎖無效）→ PayUni `BindSeller`（:600）＋`payuni_registrations` upsert 皆 tx 外 → tx 寫 forwarded／review／`merchant_verification_outbox`（只給 notification）；AGENT04017 由 `recoverFromExistingMerchant`（:684）自癒；`payuni_sync` 每 15 分輪詢寫 `payuni_status_history`；改地址走 `payuni_merchant_profile_outbox`＋`payuni_merchant_profile_updates` UNIQUE claim＋`merchant_profile_version` CAS。shipping 開通時 RPC 讀 `payuni_mer_id` 存 `seller_shipment_settings.mer_id`。
- 部分失敗：BindSeller RPC 失敗靠 outbox，5 次後 failed、無 redrive → 有店無角色；PayUni 成功本地 tx 失敗 → 重試走 relink／自癒；並發核准兩次 PayUni，靠 application `version` CAS 擋第二個 tx。
- 單一 tx：**partly**。可移除 BindSeller RPC＋onboarded 角色子任務、`source_id` 延遲撤銷柵欄語意（留 FK）、三支前置 RPC、shipping `mer_id` 複本。保留 PayUni bind／update／輪詢、自癒、profile outbox＋updates＋CAS、status history。Approve 的「外部呼叫→本地狀態」需另補 intent 列（合庫修不好）。

### D2 帳號刪除 cascade
- 賣家：`SEL/usecase/delete_seller_account.go:94` tx 內鎖 seller、RPC order `GetActiveMerchantOrderCount`（對方庫無鎖）、寫 audit、closed、`seller_deleted_outbox` → tx 後 buyer `HasActiveBuyer`、account `RemoveRole`（帶 source_id）→ `ACC/usecase/remove_role.go:81` CTE 柵欄撤銷、無 active 角色且對側 ABSENT 才 cascade 軟刪帳號＋刪 `auth_providers` → `account.deleted` **直發 best-effort、無 outbox**（:290）。seller outbox 兩段 checkpoint（`event_published_at`→`role_removed_at`）。`pkg/rolestate` 三態 OtherRoleState 協定防誤刪。
- 買家：`B/services/buyer-service/internal/usecase/delete_buyer_account.go:98`：未結訂單 → `pending_deletion`（由 `order.completed` 觸發 `CompletePendingDeletion`）；否則 closed＋清資料＋`buyer_deleted_outbox` → tx 後同步 `RemoveBuyerRole`（無 checkpoint、無 source_id）。
- 下游：`seller.deleted`→product 下架＋`seller_deleted_at`、social、notification、im、pricebook；`buyer.deleted`→coupon 作廢、social、im、notification、crm PII redaction；`account.deleted`→media ref 釋放、social、im、notification。Apple revoke 只在 `UnbindProvider` 入列 `apple_revoke_outbox`，cascade 不觸發。
- 缺口：`account.deleted` 無 outbox 且重試走 `alreadySoftDeleted` 不重發 → media ref／likes 永久殘留；買家角色撤銷失敗只能重按；未結訂單檢查與刪除非原子（TOCTOU）；outbox failed 後只能人工。
- 單一 tx：角色／帳號部分 **yes**（`role_removed_at` 子階段、RemoveRole RPC、OtherRoleState UNKNOWN 協定、`HasActive*` RPC、`account.deleted` 事件本身）；下游 DB 清理 **yes/partly**（可同 tx 或同庫 job，fan-out 大者維持非同步但不需 MQ）；Apple／Redis／revalidate／R2／FCM **no**。

### D3 ADM 代行
- gateway 每請求 `GetStoreStateForGuard`（fail-closed、不快取）→ `Orchestrate`（`gateway/internal/handler/delegation/orchestrator.go:143`）：ReadBefore → account `RecordAuditIntent`（PENDING，`UNIQUE(session_id, idempotency_key)`）→ domain 寫入（seller／product／social／order，帶 `DelegationContext`）→ `ConfirmAuditResult`（同 tx 翻結果＋`admin_operation_outbox`）→ dispatcher lease CAS → notification。domain 端只驗形狀／capability／store 與 `audit_log_id`，不回查 session。ADR `2026-09-18-adm-delegation-service-ownership.md` §2 明言兩階段是因「跨服務時兩者不在同一個交易裡」。
- 缺口：confirm 失敗永停 PENDING；replay 把 PENDING 補成 SUCCEEDED（可能誤標，UNVERIFIED）；店家狀態檢查與寫入 TOCTOU；排程代行貼文在 session 過期後可能照發（UNVERIFIED）。
- 單一 tx：**yes**（大部分）。intent＋寫入＋confirm＋outbox 同 tx，可刪 PENDING、replay-heal、confirm 重試、Batch intent/confirm RPC、`audit_log_id` 傳遞契約；FAILED 列於 rollback 後另開 tx 寫；店家狀態 `SELECT … FOR SHARE sellers`。session／authorization 表為業務本體，保留。

### D4 賣家停權 cascade
- `SEL/usecase/seller_suspension.go:77` 同 tx 寫 `seller_suspensions`、Recompute `sellers.status`、outbox `seller.suspended` → product `offShelfForSuspension`（`product_suspension_snapshot` PK `(suspension_id, product_id)`）、coupon（ACTIVE→INACTIVE＋`disabled_by_suspension`）、order（CAS 取消 PENDING_PAYMENT，不可逆）、im；Lift → `seller.unsuspended` → product／coupon 還原。治理 full_suspension 只 Recompute＋Redis、不發事件，靠 product 轉 ACTIVE 時同步 `GetSellerMeta` fail-closed 擋。
- 缺口：outbox 無每賣家順序保證；restore 不檢查仍在停權或 `seller_deleted_at`、coupon 旗標未綁 suspension_id → 遲到的 unsuspended 可能在新停權／刪除後復活商品與券。
- 單一 tx：product／coupon **yes**（公開可見性改 join `sellers.status`，可刪 `product_suspension_snapshot`、`disabled_by_suspension`、`product.seller_deleted_at` 與兩組 consumer）；order 取消 **partly**（付款 SoT gate，保留同庫 job）。

### D5 商品匯入 commit
- product-import `CommitJobService.commitJob:43` → MQ commit → `CommitItemService.commitOne:99` → product `ValidateProductDraft`＋`CreateProductFromImportInternal` → product `@Transactional` 內逐圖同步 RPC media `FinalizeImportAsset`（R2 tmp→public 並刪 tmp）＋`LinkImages` → 回 PI `applySuccessBody:232` → `job.commit_finished`。冪等：`import:{itemId}:rev:{rev}` → product `idempotency_request`（24h 清）；`product_import_commit_attempt` UNIQUE、無 fencing；`product(import_job_id, import_item_id)` 只有非唯一索引（`B/services/java/product-service/src/main/resources/db/migration/V4__add_product_import_origin_columns.sql:5-7`，已抽驗）；下載回呼 `media_callback_inbox`。
- 缺口：Link 後 product rollback → 幽靈 `asset_reference`＋R2 外洩；Finalize 刪 tmp 後 DB 失敗 → import_asset 卡 UPLOADED；revision 變更或 >24h 重試 → 重複商品；DLQ 後 item 永停 COMMITTING；commitJob 在 commit 前 publish。
- 單一 tx：**partly**。可移除 `idempotency_request`（改 `UNIQUE(import_item_id)`＋item CAS）、callback inbox、commit_attempt 大部分、`cleanStaleReferences`、gc.expire_job MQ；保留 R2 copy/delete、孤兒 GC、外部 URL 下載 timeout。

### D6 跨服務反正規化計數器
- `product.sales_count`：order.paid 同 tx outbox → `sales_count_event_dedup`（PK order_no）；取消／退款不扣回（UNVERIFIED 是否刻意）。`favorite_count`：buyer `SafeGo` fire-and-forget 直發（`B/services/buyer-service/internal/usecase/favorite.go:372-418`）→ 遺失即永久漂移。`product.rating/review_count`、`sellers.rating`：review commit 後直發絕對值、無版本；**bug（已抽驗）**：`UpdateSellerRatingByStoreID`（`B/services/seller-service/db/queries/sellers.sql:190`）不 bump version，`UpdateSeller`（:99）會寫回舊 rating。followers：每次 gRPC COUNT buyer `seller_subscriptions`。social like/comment：`counter.delta` 非冪等＋10 分全量重算（ADR mq-social-counter-async-delta）。ai `seller_ranking_facts`：**bug（已抽驗）** 上月事件晚到走 `ELSE 1` 把 `count_period` 回退並清本月（`B/services/ai-service/db/queries/seller_ranking_facts.sql:8-16`）。crm ledger：PK event_id＋advisory lock。restock：`restock_event_dedup` 只是 fast path。
- 單一 tx：favorite／rating／review_count／seller rating／crm／followers／ai ranking **yes**（同 tx delta 或直接 COUNT，刪 consumer 與 dedup 表）；social 計數、sales_count（hot row）、restock fan-out 維持非同步（同庫 job，去掉 MQ）。

## E. 其他跨服務一致性點（主 agent 讀碼）

### E1 媒體引用 `asset_reference`
- 12 個 owner 服務（product、seller、buyer、social、content、im、invoice、review、order、shipping、ai、product-import）以 media gRPC `FinalizeImages`→`LinkImages`（commit 前、失敗須致命）→ 業務寫入 → `ReleaseReferences`（best-effort、只送一次）維持「有引用集合 ⊇ 業務列指向集合」不變式（`docs/詳細設計書/08-多媒體微服務（Media Service）詳細設計書.md` §4.6.2）。失敗一律倒向洩漏；殘留引用與資產互相保護，GC 永遠清不到，只能維運 SQL（`scripts/audit_media_reference_precheck.py`）。
- 單一 tx：**partly**。Link／Release 與業務寫入同 tx（`asset_reference` 可加 FK 甚至改由 owner 表反查），洩漏類缺口消失；R2 物件刪除與孤兒 GC 仍非同步。

### E2 gateway 協調的複本寫入
- `gateway/internal/handler/payment.go:224` 付款建立後 best-effort 重寫 order 付款快照（與 A1b payment 端重複）；`gateway/internal/handler/seller.go:289` goroutine `RecordStoreVisit`。gateway 另持 Redis `buyer:current:`／`store:current:`（60s TTL，`gateway/internal/storeid/resolver.go:23`）。
- 單一 tx：**yes**（刪 gateway 端重寫；resolver 快取可保留為效能快取或改同庫查）。

## F. 跨流程結論

- **只因拆服務而存在（合庫後可刪）**：`stock_tx_records`／`stock_transaction`／StockCompensationWorker、`cart_cleanup_retry_tasks`、`coupon_consumption_tasks`、coupon AutoReleaseJob、`payment_event_dedup`、`payment_webhooks`（第二層 inbox）、payment→order／order→invoice／shipping→payment／aftersales→order 的 MQ 跳與對應 inbox／dedup、`procurement_order_provisioning_outbox`、RefundSyncWorker、`financial_logs` 去重用途、`invoice_cancelled_orders`、`invoice_business_day_mirror`、Revise RPC 三段 tx、`seller_deleted_outbox` 的 role 子階段、`OtherRoleState` 協定、代行 intent/confirm 兩段、`product_suspension_snapshot`／`disabled_by_suspension`、`idempotency_request`、`media_callback_inbox`、計數器 dedup 表（`sales_count_event_dedup`、`favorite_count_event_dedup`、`seller_ranking_processed_events`）、gateway 快照重寫。
- **與拆分無關、必須保留**：`webhook_inbox`、所有 pre-commit／lease／UNKNOWN 回查（Uber create、collection attempts、`invoice_operations`、`billing_commands`、grant、refund 命令）、`invoice_idempotency_keys`、`apple_revoke_outbox`、R2 GC、LLM usage reservations、通知類 outbox（可改同庫 job，不需 broker）。
- **合庫不會自動修好的既有缺口**（外部呼叫在 DB tx 內或 tx 外無 intent 列）：payment `createRefund` 在 tx 內打 PayUni（B2-c）、shipping `createShipmentFromOrder` 在 tx 內打 Uber（C2）、seller `Approve` 在 tx 外打 PayUni（D1）、aftersales 在 tx 內 RPC 取消訂單後自身 rollback（B2-a，合庫後消失）。
- **設計書寫了但程式沒接**：recurring-service（67 號，無 deployable）、會員計費 provider adapter、物流費發票、`openUnresolved`、`SellerMerchantIdResolver`、Uber pre-commit 第一層、ai acceptance 的付款／取消回收。
