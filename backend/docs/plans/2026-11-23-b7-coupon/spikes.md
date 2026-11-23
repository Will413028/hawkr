# B7-0 spike 結論（快照 dev `a72634110`，唯讀；全部【量】＝讀 source，沒有執行服務）

路徑縮寫：`O=backend/services/java/order-service/src/main/java/io/github/will413028/hawkr/order`、`T=backend/infra/mq/topology/topology.yaml`。

## spike 1：Java order 取消路徑與 ReleaseCoupon

**結論：沒有任何一條呼叫 Release 的路徑帶重試；有一條會留下鎖的取消路徑完全不呼叫 Release。D3 要求的「Java 取消流程帶重試」成立，且要多補一條路徑。**

| 路徑 | 取消位置 | 呼叫 Release？ | 重試？ | file:line |
|---|---|---|---|---|
| 買家取消 `cancelOrder` | `orderService.cancelOrder` | 是，TX 外 `releaseCouponForOrder` | 否，例外只 log | `O/grpc/OrderGrpcService.java:493`、`:498`、`:1452-1463` |
| 售後內部取消 `cancelOrderInternal`（賣家取消、系統取消；aftersales 經 gRPC 呼叫） | `cancelOrderInternalInTx` | 是，TX 外 best-effort | 否 | `O/grpc/OrderGrpcService.java:2074`、`:2085` |
| 付款逾時排程 `PaymentDeadlineScheduler` → `cancelPendingOrder` | `cancelExpiredOrderInTx` | 是，`compensateAfterCancel` | 否；註解寫「失敗由 Coupon AutoReleaseJob 兜底」，D3 取消該 job | `O/service/OrderExpiryCancellationService.java:66-72`、`:162-176`；排程 `O/scheduler/PaymentDeadlineScheduler.java:55-68` |
| 賣家停權批次取消 `SellerSuspendedConsumer` | 走 `cancelPendingOrder`（同上） | 是（經上一列） | 否 | `O/mq/SellerSuspendedConsumer.java:86` |
| 結帳 Confirm 時 reserve 失敗的補償 | `orderService.cancelOrder` + `releaseStockAfterCancel` | **否** | 不適用 | `O/checkout/CheckoutService.java:1070-1087`（取消在 `:1081`） |
| Uber 報價鎖定失敗的取消 | 同上 | 否，但此時尚未 reserve，沒有鎖 | 不適用 | `O/checkout/CheckoutService.java:539`（reserve 在 `:1073` 之後） |
| `PaymentEventHandler.cancelOrderIfPending` | `cancelOrderByPaymentExpireCas` | 否 | 不適用；**沒有任何呼叫端**（死碼） | `O/mq/PaymentEventHandler.java:416-445` |

- 底層 client 吞掉所有例外且不重試：`O/checkout/client/CheckoutCouponGrpcClient.java:160-185`（deadline 10 秒、`catch (Exception e)` 只 `log.warn`）。呼叫端再包一層 try/catch 只 log（`OrderGrpcService.java:1459-1462`、`OrderExpiryCancellationService.java:171-176`）。client 目錄內沒有 retry／maxAttempts 設定【量：`rg -n -i "retry|maxAttempts" O/checkout/client/` 無命中】。
- 對照：核銷有重試（`O/worker/CouponConsumptionWorker.java:86-108`，任務表加指數退避），Release 沒有。
- **會留下 `locked` 的缺口（停止條件觸發）**：`CheckoutService.java:1073-1090`。`reserveCoupon` 在 coupon 端成功、但回應遺失或 `updateCouponInstanceId` 失敗時，進 catch 取消訂單，而 `coupon_instance_id` 沒寫進訂單，後續任何 Release 路徑都找不到它（它們都讀 `order.getCouponInstanceId()`，`OrderGrpcService.java:1456`）。D3 取消對帳後，這張券永久卡在 `locked`，只能 admin 作廢。
- 對 B7 的影響：
  1. B7-4 Java 改動批次（同 D4 的批次）：`CheckoutCouponGrpcClient.releaseCoupon` 失敗時有限次重試（或 Release 失敗時寫補償任務，比照 `CouponConsumptionWorker` 的任務表），並改掉 `OrderExpiryCancellationService.java:170` 的 AutoReleaseJob 註解。
  2. 結帳 reserve 失敗的補償：Release 要能以 `order_no` 為鍵（轉接層的 Release 本來就帶 `order_no`，`O/checkout/client/CheckoutCouponGrpcClient.java:165-168`），且 coupon 端的 Release 對「鎖了但訂單沒記錄 instance id」要能依 `order_no` 找到鎖；Java 的補償分支補一次 Release（帶 `order_no`）。需 Will 決定是否納入 B7-4（改變 Java 結帳行為），否則接受這個缺口並由 admin 作廢處理。

## spike 2：order.completed 能不能餵 `store_buyer_membership` 投影

**結論：可行，投影照 D3 做；不需要退回同步 port。**

- 事件：routing key `order.completed`，exchange `hawkr.order`（`backend/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/mq/MqConstants.java:12`、`:50`）。
- Payload `OrderCompletedPayload`（snake_case）：`event_id`、`order_no`、`buyer_id`、`notify_buyer_account_id`、`store_id`、`notify_seller_account_id`、`total_actual_amount`、`confirm_type`、`completed_at`；`buyer_id`、`store_id` 是業務鍵的原始值（非 account_id）（`O/mq/dto/OrderCompletedPayload.java:34-55`，檔頭說明 `:15-30`）。所以兩個鍵都有、型別是字串。
- 可靠性：狀態 CAS（`DELIVERED→COMPLETED`）與 `order_outbox` 在同一個 transaction（`O/service/OrderCompletionTxHandler.java:123`、`:117`、`:140-158`），由 outbox publisher 至少一次投遞（`O/mq/OrderOutboxPublisher.java:92`）；這是唯一進入 COMPLETED 的路徑（`rg -n "OrderStatus.COMPLETED.value\(\)"` 於 CAS 只有 `OrderCompletionTxHandler.java:123`）。消費端要以 `event_id` 或 `(store_id, buyer_id)` 冪等（投影是 upsert，天然冪等）。
- 與現行 MEMBER 判定對得上：Java `GetBuyerMerchantSummary` 的成立狀態是 `COMPLETED` 與 `PARTIALLY_REFUNDED`（`O/repository/OrderRepository.java:782-784`，不含 `FULLY_REFUNDED`）。`PARTIALLY_REFUNDED` 是從已完成之後的退款進入（若有不經 COMPLETED 的路徑，投影會少算那些訂單；系統未上線、沒有歷史資料，風險限於 B7 期間的新訂單，**【推】未逐一查 `AfterSalesRefundEventHandler` 的 allowedStatuses**）。
- 缺口：`topology.yaml` 目前**沒有任何 queue 綁 `order.completed`**（`rg -n "key: order.completed" T` 無命中；只有 `order.paid`、`order.receipt.confirmed` 等）。B7-4 要新增 coupon 的 queue 與 binding（事件由 Java 發、單體消費；這是 B7-4 的事件接線，註冊點照 D3 與不變量只在測試接線，B7-5 才在 `cmd/hawkr` 接上）。
- 取捨：投影只在事件抵達後才算成員；Java 到 B8 前仍是唯一寫入端，延遲是 outbox 輪詢間隔（【推】未量）。
