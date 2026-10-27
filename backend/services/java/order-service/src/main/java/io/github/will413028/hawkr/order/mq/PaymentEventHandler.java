package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.PaymentEventType;
import io.github.will413028.hawkr.order.entity.AutoRefundTaskEntity;

import io.github.will413028.hawkr.order.entity.CouponConsumptionTaskEntity;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderSalesAccumulatedPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderSalesAccumulatedPayload.ProductSalesItem;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import io.github.will413028.hawkr.order.repository.*;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * 支付事件交易性處理邏輯
 *
 * 從 PaymentEventConsumer 獨立出來，確保 Spring @Transactional proxy 正常運作。
 * Consumer 透過 Spring Bean 呼叫本 class，proxy 才能正確攔截交易邊界。
 */
@Service
public class PaymentEventHandler {

    private static final ObsLogger log = ObsLogger.forClass(PaymentEventHandler.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final OrderItemRepository orderItemRepository;
    private final PaymentEventDedupRepository dedupRepository;
    private final FinancialLogRepository financialLogRepository;
    private final AutoRefundTaskRepository autoRefundTaskRepository;
    private final CouponConsumptionTaskRepository couponConsumptionTaskRepository;
    private final OrderEventPublisher orderEventPublisher;
    private final io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient paymentGrpcClient;
    private final OrderOutboxMapper orderOutboxMapper;
    private final OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository;
    private final io.github.will413028.hawkr.order.service.OrderService orderService;
    private final ObsContext obsContext;

    public PaymentEventHandler(
            OrderRepository orderRepository,
            OrderItemRepository orderItemRepository,
            PaymentEventDedupRepository dedupRepository,
            FinancialLogRepository financialLogRepository,
            AutoRefundTaskRepository autoRefundTaskRepository,
            CouponConsumptionTaskRepository couponConsumptionTaskRepository,
            OrderEventPublisher orderEventPublisher,
            io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient paymentGrpcClient,
            OrderOutboxMapper orderOutboxMapper,
            OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository,
            io.github.will413028.hawkr.order.service.OrderService orderService,
            ObsContext obsContext
    ) {
        this.orderRepository = orderRepository;
        this.orderItemRepository = orderItemRepository;
        this.dedupRepository = dedupRepository;
        this.financialLogRepository = financialLogRepository;
        this.autoRefundTaskRepository = autoRefundTaskRepository;
        this.couponConsumptionTaskRepository = couponConsumptionTaskRepository;
        this.orderEventPublisher = orderEventPublisher;
        this.paymentGrpcClient = paymentGrpcClient;
        this.orderOutboxMapper = orderOutboxMapper;
        this.orderInvoiceSnapshotRepository = orderInvoiceSnapshotRepository;
        this.orderService = orderService;
        this.obsContext = obsContext;
    }

    /**
     * 支付成功處理（transaction_id 冪等，整段交易性）
     *
     * dedup INSERT + 訂單更新 + financial_log + 優惠券核銷任務
     * 任一步驟失敗全部 rollback，確保不會因 partial commit 導致 dedup 中毒。
     */
    @Transactional
    public void handlePaymentSuccess(JsonNode payload, String rawBody, String payloadHash) {
        String tradeNo = payload.path("trade_no").asText("");
        String orderId = payload.path("order_id").asText("");
        // fail-loud：amount 缺失 / 非數字 → 拋例外 → PaymentEventConsumer catch → MqErrorHandler → DLQ。
        // 「值為 0」保留（ZERO_AMOUNT 合法路徑，見下方 §2A.1 檢查），不可用 asLong(0) 把缺失吞成 0。
        long amount = parseRequiredAmount(payload, orderId);
        String payTime = payload.path("pay_time").asText("");
        String paymentType = payload.path("payment_type").asText("");

        if (tradeNo.isEmpty() || orderId.isEmpty()) {
            log.error("PaymentEventHandler.paymentSuccess.invalid", "trade_no 或 order_id 為空");
            return;
        }
        if (amount < 0) {
            log.error("PaymentEventHandler.paymentSuccess.invalidAmount",
                    "付款金額不合法: amount=" + amount + ", orderId=" + orderId);
            return;
        }
        // §2A.1：零元僅允許 ZERO_AMOUNT (2900) 付款方式；非 ZERO_AMOUNT 不可為 0 元
        if (amount == 0 && !PaymentMethod.ZERO_AMOUNT.value().equals(paymentType)) {
            log.error("PaymentEventHandler.paymentSuccess.invalidZeroAmount",
                    "非 ZERO_AMOUNT 付款方式金額不可為 0: paymentType=" + paymentType + ", orderId=" + orderId);
            return;
        }

        // 冪等檢查：INSERT ON CONFLICT DO NOTHING
        // event_type 對齊 publisher 實際送出的字串（PaymentEventType.value() = "payment.success"）
        int inserted = dedupRepository.insertIdempotent(tradeNo, orderId, PaymentEventType.PAYMENT_SUCCESS.value(), payloadHash);
        if (inserted == 0) {
            log.info("PaymentEventHandler.paymentSuccess.duplicate", "重複事件，忽略", "tradeNo", tradeNo);
            return;
        }

        // 鎖定訂單
        OrderEntity order = orderRepository.findByOrderNoForUpdate(orderId);
        if (order == null) {
            log.error("PaymentEventHandler.paymentSuccess.orderNotFound", "訂單不存在: " + orderId);
            return;
        }

        String currentStatus = order.getStatus();

        if (OrderStatus.PENDING_PAYMENT.value().equals(currentStatus)) {
            handlePendingPaymentOrder(order, orderId, tradeNo, amount, payTime, paymentType);
        } else if (OrderStatus.CANCELLED.value().equals(currentStatus)) {
            handleCancelledOrderPayment(orderId, tradeNo, amount, paymentType);
        } else if (OrderStatus.PAID.value().equals(currentStatus)) {
            log.info("PaymentEventHandler.paymentSuccess.alreadyPaid", "訂單已付款，忽略", "orderNo", orderId);
        } else {
            log.info("PaymentEventHandler.paymentSuccess.unexpected",
                    "非預期訂單狀態: orderNo=" + orderId + ", status=" + currentStatus);
        }
    }

    /**
     * 正常付款流程：待付款 → 已付款
     */
    private void handlePendingPaymentOrder(OrderEntity order, String orderId, String tradeNo, long amount, String payTime, String paymentType) {
        log.info("PaymentEventHandler.handlePending.start",
                "開始處理待付款訂單: orderNo=" + orderId
                        + ", checkoutId=" + order.getCheckoutId()
                        + ", couponInstanceId=" + order.getCouponInstanceId());

        // 原子更新狀態 + 付款資訊（避免 updateById 全欄位覆蓋的 race condition）
        LocalDateTime parsedPayTime = parsePayTime(payTime);
        String finalPaymentType = (paymentType != null && !paymentType.isEmpty()) ? paymentType : order.getPaymentMethod();
        int updated = orderRepository.updatePaymentConfirmed(
                orderId,
                OrderStatus.PENDING_PAYMENT.value(),
                OrderStatus.PAID.value(),
                tradeNo,
                parsedPayTime,
                finalPaymentType
        );
        if (updated == 0) {
            log.error("PaymentEventHandler.handlePending.updateFailed",
                    "付款確認更新失敗（狀態可能已變更）: orderNo=" + orderId);
            return;
        }
        // 同步 entity 以供後續邏輯使用
        order.setStatus(OrderStatus.PAID.value());
        order.setTransactionId(tradeNo);
        order.setPayTime(parsedPayTime);
        if (finalPaymentType != null) {
            order.setPaymentMethod(finalPaymentType);
        }
        log.info("PaymentEventHandler.handlePending.orderUpdated", "訂單狀態已更新為 PAID", "orderNo", orderId);

        // 冪等寫入 financial_logs
        financialLogRepository.insertIdempotent(
                TsidUtil.generate(),
                orderId,
                FinancialLogType.RECEIPT.value(),
                amount,
                tradeNo
        );
        log.info("PaymentEventHandler.handlePending.financialLogInserted", "財務日誌已寫入", "orderNo", orderId);

        // 購物車已在建單時清理（建單即清策略），此處不再建立清理任務

        // 落地優惠券核銷任務
        String couponInstanceId = order.getCouponInstanceId();
        if (couponInstanceId != null && !couponInstanceId.isEmpty()) {
            CouponConsumptionTaskEntity couponTask = new CouponConsumptionTaskEntity();
            couponTask.setTaskId(TsidUtil.generate());
            couponTask.setOrderNo(orderId);
            couponTask.setCouponInstanceId(couponInstanceId);
            couponTask.setBuyerId(order.getBuyerId());
            couponTask.setStatus(OrderConstants.TASK_STATUS_PENDING);
            couponTask.setRetryCount(0);
            couponTask.setMaxRetryCount(OrderConstants.DEFAULT_MAX_RETRY_COUNT);
            couponTask.setNextRetryTime(DateTimeUtils.nowUtc());
            couponConsumptionTaskRepository.insert(couponTask);
            log.info("PaymentEventHandler.handlePending.couponTaskCreated",
                    "優惠券核銷任務已建立: orderNo=" + orderId + ", couponInstanceId=" + couponInstanceId);
        } else {
            log.info("PaymentEventHandler.handlePending.noCouponTask", "無 couponInstanceId，跳過優惠券核銷任務", "orderNo", orderId);
        }

        log.info("PaymentEventHandler.paymentSuccess.paid", "訂單付款成功", "orderNo", orderId);

        // [B-207] #1181：賣家工作區 realtime（afterCommit）— 待付款 → 已付款
        orderEventPublisher.emitSellerOrderUpdated(
                order.getStoreId(), orderId, OrderEventPublisher.SRC_PAYUNI_WEBHOOK,
                OrderStatus.PENDING_PAYMENT.value(), OrderStatus.PAID.value(), null);

        // order.paid 改走 Outbox 可靠投遞（03 §3.12/§8.10）：事務內寫入，杜絕原 afterCommit best-effort 遺失。
        // seller/buyer meta 由 OrderOutboxPublisher 於派送時 best-effort 補（payload 形狀與原本一致，下游不受影響）。
        // notifyBuyerAccountId 同理由 OrderOutboxPublisher.enrich() 於派送前解析並覆寫，此處寫入的 core
        // payload 帶 null 佔位（buyer_id 8-service rekey Phase 1，鏡像上方 sellerName/buyerNickname 等欄位）。
        String shippingMethod = order.getShippingMethod() != null ? order.getShippingMethod() : "";
        // 58 §6.3.2.1：correlation_id ＋ invoice_snapshot_id 兩欄在**核心 payload** 落地
        // （不是 enrich 時才補）——enrich 是 best-effort 且跑在另一個執行緒／另一輪 tick，
        // 那裡的 ObsContext 已經不是付款這條鏈的了。
        OrderInvoiceSnapshotEntity invoiceSnapshot =
                orderInvoiceSnapshotRepository.findLatestByOrderNo(orderId);
        if (invoiceSnapshot == null) {
            // 不 fail：這只是 payload 上的指標，權威讀取走 GetOrderInvoiceSnapshotInternal。
            // 但要留下訊號——V12 之前建立的訂單本來就沒有快照，之後的訂單沒有就是 bug。
            log.warn("PaymentEventHandler.handlePending.invoiceSnapshotMissing",
                    "訂單無發票快照（V12 之前的訂單屬正常；之後的訂單代表建單路徑漏寫）",
                    "orderNo", orderId);
        }
        OrderPaidPayload corePaid = new OrderPaidPayload(
                orderId, shippingMethod, DateTimeUtils.nowUtc().toString(),
                order.getBuyerId(), null, order.getStoreId(), order.getTotalActualAmount(),
                null, null, null, null,
                obsContext.getCorrelationIdOrNull(),
                invoiceSnapshot == null ? null : String.valueOf(invoiceSnapshot.getId()));
        OrderOutboxEntity paidOutbox = new OrderOutboxEntity();
        paidOutbox.setAggregateId(orderId);
        paidOutbox.setEventType(OrderEventType.ORDER_PAID.value());
        paidOutbox.setExchange(MqConstants.EXCHANGE_ORDER);
        paidOutbox.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_PAID);
        paidOutbox.setStatus(OutboxEventStatus.PENDING.value());
        paidOutbox.setRetryCount(0);
        try {
            paidOutbox.setPayload(MAPPER.writeValueAsString(corePaid));
        } catch (com.fasterxml.jackson.core.JsonProcessingException e) {
            // 固定結構序列化理論上不會失敗；拋出讓事務 rollback，避免付款已記但事件遺失
            throw new IllegalStateException("order.paid payload 序列化失敗 orderNo=" + orderId, e);
        }
        orderOutboxMapper.insert(paidOutbox);

        // 銷量累計事件（觸發 product-service 累計 sales_count，見 35 設計書 §5.3）。
        //
        // 原本走 afterCommit + best-effort 直發。那個組合在本方法裡是**永久遺失**的形狀：
        // 上方 :116 的 dedup INSERT 與本事務同生共死 ⇒ commit 之後同一個 payment.success 重投一律被
        // dedup 擋掉；而 afterCommit 的 publish 走 OrderEventPublisher.publish（吞掉所有例外）
        // ⇒ MQ 短暫不可用時「訂單已 PAID、dedup 已寫、事件沒送出」，product.sales_count 永久少算，
        // 而 docs/開發規範/後端服務責任邊界規範.md 禁止寫 reconciliation job ⇒ 無任何救濟路徑。
        // ⇒ 改走**例外 A（同 tx outbox）**，與正上方 order.paid 同一個做法與同一個事務。
        Map<Long, Integer> productQtyMap = orderItemRepository.findByOrderNo(orderId).stream()
                .collect(Collectors.groupingBy(
                        item -> item.getProductId(),
                        Collectors.summingInt(item -> item.getQuantity())));
        List<ProductSalesItem> salesItems = productQtyMap.entrySet().stream()
                .map(e -> new ProductSalesItem(e.getKey(), e.getValue()))
                .collect(Collectors.toList());
        if (salesItems.isEmpty()) {
            log.warn("PaymentEventHandler.paymentSuccess.noSalesItems",
                    "找不到訂單明細，跳過 sales.accumulated 發布", "orderNo", orderId);
        } else {
            OrderSalesAccumulatedPayload salesPayload = new OrderSalesAccumulatedPayload(
                    orderId, DateTimeUtils.nowUtc().toString(), salesItems);
            OrderOutboxEntity salesOutbox = new OrderOutboxEntity();
            salesOutbox.setAggregateId(orderId);
            salesOutbox.setEventType(OrderEventType.ORDER_SALES_ACCUMULATED.value());
            salesOutbox.setExchange(MqConstants.EXCHANGE_ORDER);
            salesOutbox.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_SALES_ACCUMULATED);
            salesOutbox.setStatus(OutboxEventStatus.PENDING.value());
            salesOutbox.setRetryCount(0);
            try {
                salesOutbox.setPayload(MAPPER.writeValueAsString(salesPayload));
            } catch (com.fasterxml.jackson.core.JsonProcessingException e) {
                // 與 order.paid 同樣處置：拋出讓事務 rollback，避免付款已記但事件遺失
                throw new IllegalStateException(
                        "order.sales.accumulated payload 序列化失敗 orderNo=" + orderId, e);
            }
            orderOutboxMapper.insert(salesOutbox);
        }
    }

    /**
     * 已取消但收到付款 → 建立自動退款任務
     */
    private void handleCancelledOrderPayment(String orderId, String tradeNo, long amount, String paymentType) {
        // §2A.1.4 防禦：零元訂單沒有退款概念，不應為 ZERO_AMOUNT 建 auto_refund_task。
        // 零元訂單直接 PAID 不會經歷「CANCELLED → 收到 payment 成功」這個 race（沒 PayUni webhook），
        // 此分支僅為極端 race / admin 手動操作的兜底保護。
        if (PaymentMethod.ZERO_AMOUNT.value().equals(paymentType)) {
            log.warn("PaymentEventHandler.cancelledOrderPayment.zeroAmountIgnored",
                    "零元訂單已 CANCELLED 仍收到 payment.success，理論不該發生 — 跳過 auto_refund_task 建立, orderNo=" + orderId);
            return;
        }

        // 依付款方式決定初始狀態：ATM 等待買家操作 PayUni 頁面，非 ATM 由 Worker 自動退款
        boolean isAtm = PaymentMethod.ATM.value().equals(paymentType);
        String taskStatus = isAtm
                ? OrderConstants.TASK_STATUS_AWAITING_BUYER_REFUND_PAGE
                : OrderConstants.TASK_STATUS_PENDING;

        // ATM 路徑：預建 payment_refunds row（idempotency key = AUTO:{orderNo}:{tradeNo}）
        // 非 ATM 由 AutoRefundWorker 呼叫 createRefund 時才建（既有流程）
        if (isAtm) {
            String idempotencyKey = "AUTO:" + orderId + ":" + tradeNo;
            try {
                paymentGrpcClient.createRefund(tradeNo, amount, "訂單已取消但收到付款自動退款", idempotencyKey);
            } catch (Exception e) {
                log.error("PaymentEventHandler.cancelledOrderPayment.createRefundFailed",
                        "預建 payment_refunds 失敗 orderNo=" + orderId, e);
                // 不中斷主流程：auto_refund_task 仍建立，後續 async webhook 會用 tradeNo lookup
            }
        }

        AutoRefundTaskEntity task = new AutoRefundTaskEntity();
        task.setTaskId(TsidUtil.generate());
        task.setOrderNo(orderId);
        task.setTransactionId(tradeNo);
        task.setAmount(amount);
        task.setPaymentMethod(paymentType);
        task.setStatus(taskStatus);
        task.setRetryCount(0);
        task.setMaxRetryCount(OrderConstants.DEFAULT_MAX_RETRY_COUNT);
        task.setNextRetryTime(DateTimeUtils.nowUtc());
        autoRefundTaskRepository.insert(task);

        // ATM：設定 need_refund_page 標記
        if (isAtm) {
            orderRepository.updateNeedRefundPage(orderId, true);
            // TODO: 發布通知 MQ 事件，通知買家前往退款頁面
        }

        log.info("PaymentEventHandler.paymentSuccess.cancelled",
                "訂單已取消但收到付款，建立自動退款任務: paymentType=" + paymentType + ", taskStatus=" + taskStatus,
                "orderNo", orderId);
    }

    /**
     * 付款失敗處理：遞增 payment_attempts
     */
    public void handlePaymentFailed(String orderId) {
        if (orderId.isEmpty()) {
            log.error("PaymentEventHandler.paymentFailed.invalid", "order_id 為空");
            return;
        }
        int updated = orderRepository.incrementPaymentAttempts(orderId);
        if (updated > 0) {
            log.info("PaymentEventHandler.paymentFailed.counted",
                    "付款失敗，已遞增嘗試次數", "orderNo", orderId);
        }
    }

    /**
     * 待付款訂單系統批次取消（TX 內只做狀態變更，回傳補償資訊）。
     *
     * 由 {@code PaymentDeadlineScheduler}（付款逾時）與
     * {@code OrderExpiryCancellationService}（賣家停權批次取消，複用同一 TX 內防護，03 §5.1a.2）呼叫，
     * 皆透過 Spring Bean 呼叫本 class，確保 @Transactional proxy 正常運作。
     *
     * @param cancelReason {@code OrderCancelReason.value()}（03 §5.3.1；呼叫端負責語意正確，
     *                     本方法只驗合法性，寫入前經 {@code CodeMetaValidator}）
     */
    @Transactional
    public CancelInfo cancelExpiredOrderInTx(String orderNo, String cancelReason) {
        io.github.will413028.hawkr.common.utils.code.CodeMetaValidator.requireValidValue(
                io.github.will413028.hawkr.common.utils.code.OrderCancelReason.class, cancelReason, "cancelReason");

        OrderEntity order = orderRepository.findByOrderNoForUpdate(orderNo);
        if (order == null || !OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus())) {
            return null;
        }
        // 03 §5.3.1：cancel_reason 由呼叫端傳入（PAYMENT_TIMEOUT / SELLER_SUSPENDED 等）+ cancel_by_role='SYSTEM'
        int updated = orderRepository.cancelOrderByPaymentExpireCas(
                orderNo, OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(), cancelReason);
        if (updated == 0) {
            log.info("PaymentEventHandler.cancelExpired.skip", "訂單狀態已變更，跳過取消", "orderNo", orderNo);
            return null;
        }
        // §5.9 PENDING_PAYMENT 階段尚未建物流單，processing_deadline 必為 NULL；補呼叫天然冪等
        orderRepository.clearProcessingDeadline(orderNo);
        // #1368：庫存歸還 order.cancelled 與取消狀態同事務落 outbox（取代原 compensateAfterCancel TX 外直發）
        orderService.insertCancelledOutbox(orderNo);
        log.info("PaymentEventHandler.cancelExpired.success", "超時訂單已取消（TX 內）", "orderNo", orderNo);
        // [B-207] #1181：賣家工作區 realtime — 逾時取消
        orderEventPublisher.emitSellerOrderUpdated(
                order.getStoreId(), orderNo, OrderEventPublisher.SRC_SYSTEM,
                OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(), null);
        return new CancelInfo(orderNo, order.getCheckoutId(),
                order.getBuyerId(), order.getCouponInstanceId());
    }

    /**
     * 超時取消補償資訊
     */
    public record CancelInfo(String orderNo, String checkoutId, long buyerId, String couponInstanceId) {}

    /**
     * 取消待付款訂單
     */
    @Transactional
    public boolean cancelOrderIfPending(String orderNo, String reason) {
        OrderEntity order = orderRepository.findByOrderNoForUpdate(orderNo);
        if (order == null) {
            log.error("PaymentEventHandler.cancelOrder.notFound", "訂單不存在: " + orderNo);
            return false;
        }

        if (OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus())) {
            // 03 §5.3.1：payment 異常 path 視同 payment expire（reason='9101' PAYMENT_TIMEOUT, role='SYSTEM'）
            int updated = orderRepository.cancelOrderByPaymentExpireCas(
                    orderNo, OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(),
                    io.github.will413028.hawkr.common.utils.code.OrderCancelReason.PAYMENT_TIMEOUT.value());
            if (updated == 0) {
                log.info("PaymentEventHandler.cancelOrder.raceCondition", "訂單狀態已變更，跳過取消", "orderNo", orderNo);
                return false;
            }
            // §5.9 PENDING_PAYMENT 階段尚未建物流單，deadline 必為 NULL；補呼叫天然冪等
            orderRepository.clearProcessingDeadline(orderNo);
            log.info("PaymentEventHandler.cancelOrder.success", reason + "，取消訂單", "orderNo", orderNo);
            // [B-207] #1181：賣家工作區 realtime — 異常取消
            orderEventPublisher.emitSellerOrderUpdated(
                    order.getStoreId(), orderNo, OrderEventPublisher.SRC_SYSTEM,
                    OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(), null);
            return true;
        }

        log.info("PaymentEventHandler.cancelOrder.skip",
                "訂單非待付款狀態，忽略: orderNo=" + orderNo + ", status=" + order.getStatus());
        return false;
    }

    private LocalDateTime parsePayTime(String payTimeStr) {
        try {
            if (payTimeStr.isEmpty()) return DateTimeUtils.nowUtc();
            return LocalDateTime.parse(payTimeStr.replace("Z", "").replace("T", "T"));
        } catch (Exception e) {
            return DateTimeUtils.nowUtc();
        }
    }

    /**
     * fail-loud 取 payload.amount：缺失 / null / 非數字型別 / 非數字字串 → 拋
     * {@link IllegalStateException}（由 PaymentEventConsumer catch → MqErrorHandler → DLQ）。
     * 「值為 0」為合法輸入（ZERO_AMOUNT 付款路徑），由呼叫端的 §2A.1 檢查裁決。
     * wire 上 amount 為 JSON number（PaymentDomainEvent long 欄位）；數字字串兼容
     * PaymentDeadlineScheduler 反查合成 payload 等邊緣來源。
     */
    private static long parseRequiredAmount(JsonNode payload, String orderId) {
        JsonNode node = payload.get("amount");
        if (node == null || node.isNull()) {
            throw new IllegalStateException("payment.success payload 缺少 amount, orderId=" + orderId);
        }
        if (node.isNumber()) {
            return node.longValue();
        }
        if (node.isTextual()) {
            try {
                return Long.parseLong(node.asText().trim());
            } catch (NumberFormatException e) {
                throw new IllegalStateException(
                        "payment.success payload amount 非合法數字: '" + node.asText() + "', orderId=" + orderId, e);
            }
        }
        throw new IllegalStateException(
                "payment.success payload amount 型別非法: " + node.getNodeType() + ", orderId=" + orderId);
    }
}
