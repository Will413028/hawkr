package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.FulfillmentMode;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceModel;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.StockOperationStatus;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.common.utils.validation.PayUniShippingConstraints;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderItemStatus;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.*;
import io.github.will413028.hawkr.order.exception.InsufficientStockException;
import io.github.will413028.hawkr.order.exception.InvalidStatusException;
import io.github.will413028.hawkr.order.exception.OrderNotFoundException;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.SellerCaseClassification;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.mq.dto.OrderCancelledPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderCompletedPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderReceiptConfirmedPayload;
import io.github.will413028.hawkr.order.repository.*;
import io.github.will413028.hawkr.product.grpc.StockItem;
import io.github.will413028.hawkr.product.grpc.StockOperationResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.github.will413028.hawkr.order.checkout.OrderPolicyResolver;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.UUID;

/**
 * 訂單核心業務邏輯
 */
@Service
public class OrderService {

    private static final ObsLogger log = ObsLogger.forClass(OrderService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final OrderItemRepository orderItemRepository;
    private final RefundRequestRepository refundRequestRepository;
    private final FinancialLogRepository financialLogRepository;
    private final StockTxRecordRepository stockTxRecordRepository;
    private final CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository;
    private final AutoRefundTaskRepository autoRefundTaskRepository;
    private final OrderNoGenerator orderNoGenerator;
    private final ProductGrpcClient productGrpcClient;
    private final CartGrpcClient cartGrpcClient;
    private final OrderConfig orderConfig;
    private final OrderEventPublisher eventPublisher;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;
    private final AfterSalesGrpcClient afterSalesGrpcClient;
    private final OrderCompletionTxHandler orderCompletionTxHandler;
    private final OrderCreationTxHandler orderCreationTxHandler;
    private final OrderOutboxMapper orderOutboxMapper;
    private final io.github.will413028.hawkr.order.checkout.OrderPolicyResolver orderPolicyResolver;
    /**
     * Uber Direct 資格閘門——<b>建單路徑的最後防線</b>。
     *
     * <p>結帳鏈（{@code CheckoutValidator}）是源頭防線，能讓買家在選項階段就看不到／立刻知道原因；
     * 本類別的兩支 {@code createOrder} 才是<b>所有建單路徑的共同出口</b>：
     * gRPC {@code CreateOrder}（購物車外建單）根本不經過 {@code CheckoutService}，
     * 只守結帳鏈等於把那條入口整個放掉。與上方 {@code recipientName} 縱深防禦同一個理由。
     */
    private final io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard uberOrderEligibilityGuard;
    private final Counter ordersCreatedCounter;
    private final CouponReleaseService couponReleaseService;

    public OrderService(
            OrderRepository orderRepository,
            OrderItemRepository orderItemRepository,
            RefundRequestRepository refundRequestRepository,
            FinancialLogRepository financialLogRepository,
            StockTxRecordRepository stockTxRecordRepository,
            CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository,
            AutoRefundTaskRepository autoRefundTaskRepository,
            OrderNoGenerator orderNoGenerator,
            ProductGrpcClient productGrpcClient,
            CartGrpcClient cartGrpcClient,
            OrderConfig orderConfig,
            OrderEventPublisher eventPublisher,
            SellerGrpcClient sellerGrpcClient,
            BuyerGrpcClient buyerGrpcClient,
            AfterSalesGrpcClient afterSalesGrpcClient,
            OrderCompletionTxHandler orderCompletionTxHandler,
            OrderCreationTxHandler orderCreationTxHandler,
            OrderOutboxMapper orderOutboxMapper,
            io.github.will413028.hawkr.order.checkout.OrderPolicyResolver orderPolicyResolver,
            io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard uberOrderEligibilityGuard,
            CouponReleaseService couponReleaseService,
            MeterRegistry meterRegistry
    ) {
        this.couponReleaseService = couponReleaseService;
        this.orderRepository = orderRepository;
        this.orderItemRepository = orderItemRepository;
        this.refundRequestRepository = refundRequestRepository;
        this.financialLogRepository = financialLogRepository;
        this.stockTxRecordRepository = stockTxRecordRepository;
        this.cartCleanupRetryTaskRepository = cartCleanupRetryTaskRepository;
        this.autoRefundTaskRepository = autoRefundTaskRepository;
        this.orderNoGenerator = orderNoGenerator;
        this.productGrpcClient = productGrpcClient;
        this.cartGrpcClient = cartGrpcClient;
        this.orderConfig = orderConfig;
        this.eventPublisher = eventPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.afterSalesGrpcClient = afterSalesGrpcClient;
        this.orderCompletionTxHandler = orderCompletionTxHandler;
        this.orderCreationTxHandler = orderCreationTxHandler;
        this.orderOutboxMapper = orderOutboxMapper;
        this.orderPolicyResolver = orderPolicyResolver;
        this.uberOrderEligibilityGuard = uberOrderEligibilityGuard;
        this.ordersCreatedCounter = Counter.builder("hawkr_orders_created")
                .description("訂單建立成功總數（business health，issue #1369）")
                .register(meterRegistry);
    }

    /**
     * 建立訂單：購物車鎖定 → 落地 INIT → 扣庫存 → 建單 → 失敗補償
     *
     * @param checkoutId 購物車結帳批次 ID（非購物車下單時為 null 或空字串）
     */
    public String createOrder(
            long buyerId,
            long storeId,
            List<CreateOrderItemParam> items,
            long totalOriginalAmount,
            long totalActualAmount,
            String checkoutId,
            String deliveryTemp,
            String shippingMethod,
            long shippingFee,
            String shippingFeePayer
    ) {
        uberOrderEligibilityGuard.requireEligible(
                io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Context.methodOnly(shippingMethod));

        items = new ArrayList<>(items);
        items.sort(Comparator.comparing(CreateOrderItemParam::skuId));

        boolean cartLocked = false;
        if (checkoutId != null && !checkoutId.isEmpty()) {
            List<String> skuIds = items.stream()
                    .map(i -> String.valueOf(i.skuId()))
                    .toList();
            int lockResult = cartGrpcClient.checkoutLock(buyerId, checkoutId, skuIds);
            if (lockResult != 0) {
                throwLockFailureWithOrderAlreadyCreatedCheck(buyerId, items);
            }
            cartLocked = true;
        }

        try {
            String stockTxId = UUID.randomUUID().toString();

            List<StockItem> stockItems = items.stream()
                    .map(i -> StockItem.newBuilder().setSkuId(String.valueOf(i.skuId())).setQuantity(String.valueOf(i.quantity())).build())
                    .toList();

            String payloadJson = serializePayload(items);
            StockTxRecordEntity initRecord = new StockTxRecordEntity();
            initRecord.setStockTxId(stockTxId);
            initRecord.setPhase(OrderConstants.STOCK_TX_PHASE_DEDUCT);
            initRecord.setStatus(OrderConstants.STOCK_TX_STATUS_INIT);
            initRecord.setPayloadSnapshot(payloadJson);
            initRecord.setRetryCount(0);
            stockTxRecordRepository.insert(initRecord);

            StockOperationResponse deductResponse;
            try {
                deductResponse = productGrpcClient.deductStock(stockTxId, stockItems);
            } catch (Exception e) {
                log.error("OrderService.createOrder.deductStock.error", "扣庫存 gRPC 失敗", e);
                stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_FAILED, e.getMessage(), 60);
                throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE, "庫存服務不可用，請稍後重試");
            }

            StockOperationStatus deductOpStatus;
            try {
                deductOpStatus = StockOperationStatus.fromValue(deductResponse.getStatus());
            } catch (IllegalArgumentException e) {
                stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_FAILED,
                        deductResponse.getStatus() + ": " + deductResponse.getMessage(), 60);
                throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "庫存扣減失敗（未知 status）: " + deductResponse.getStatus());
            }
            switch (deductOpStatus) {
                case SUCCESS, ALREADY_APPLIED -> {
                }
                case INSUFFICIENT_STOCK -> {
                    stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_FAILED, "庫存不足", 0);
                    List<InsufficientStockException.Item> insufficientItems = deductResponse.getInsufficientItemsList().stream()
                            .map(item -> new InsufficientStockException.Item(
                                    item.getSkuId(), item.getRequestedQty(), item.getAvailableQty()))
                            .toList();
                    throw new InsufficientStockException(deductResponse.getMessage(), insufficientItems);
                }
                default -> {
                    stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT, OrderConstants.STOCK_TX_STATUS_FAILED,
                            deductResponse.getStatus() + ": " + deductResponse.getMessage(), 60);
                    throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "庫存扣減失敗: " + deductResponse.getMessage());
                }
            }

            String orderNo;
            try {
                orderNo = orderCreationTxHandler.createOrderInTx(buyerId, storeId, items, totalOriginalAmount, totalActualAmount, stockTxId, checkoutId, deliveryTemp, shippingMethod, shippingFee, shippingFeePayer,
                        orderPolicyResolver.resolve(shippingMethod));
            } catch (Exception e) {
                log.error("OrderService.createOrder.dbTx.error", "建單 DB 交易失敗，執行庫存補償", e);
                compensateStock(stockTxId, stockItems);
                throw e;
            }

            log.info("OrderService.createOrder.success", "建立訂單成功", "orderNo", orderNo);
            ordersCreatedCounter.increment();

            if (checkoutId != null && !checkoutId.isEmpty()) {
                cleanupCartAfterOrderCreation(buyerId, checkoutId, orderNo);
            }

            return orderNo;
        } catch (Exception e) {
            if (cartLocked) {
                cartGrpcClient.checkoutUnlock(buyerId, checkoutId);
            }
            throw e;
        }
    }



    /** 賣方身分 lane：平台賣家自己就是訂單賣方。 */
    private static final String RFQ_LANE_PLATFORM = "PLATFORM";
    /** 賣方身分 lane：站外業者，賣方記為平台結算 store。 */
    private static final String RFQ_LANE_EXTERNAL = "EXTERNAL";

    /**
     * RFQ 建單結果。{@code replayed} 為真＝冪等命中，本次沒有建立新訂單。
     */
    public record RfqOrderResult(String orderNo, java.time.LocalDateTime paymentDeadline, boolean replayed) {
    }

    /**
     * RFQ 成交建單（64 號 §6.4）——<b>兩條 lane 都走這裡</b>。
     *
     * <p><b>與 {@link #createOrder} 的關係</b>：不是它的分支，是另一條 orchestration。
     * 本路徑<b>不扣庫存、不寫 stock_tx、不鎖購物車、不進補償 worker</b>。持久化共用
     * {@link OrderCreationTxHandler}，所以 {@code order_items} 與發票缺省快照照舊。
     *
     * <p>🔴 <b>為什麼 PLATFORM lane 也不扣庫存</b>：賣家回價<b>不綁任何 SKU</b>——
     * {@code procurement_seller_response_versions} 與
     * {@code procurement_buyer_quote_acceptances} 對 sku／product 都是零命中。賣家報的是
     * 「200 公斤、總價 30000」這種自帶單位基準的承諾，不是「買 N 件某商品」，沒有可扣的
     * 可售量。候選的 {@code sourceId} 是 {@code product_id} 講的是<b>候選怎麼被找到</b>，
     * 不是成交的標的。要扣就得自己發明一條從報價回推候選商品的對應，而候選是探索產物
     * 不是契約。⚠️ 連帶後果要明說：平台賣家的 RFQ 成交<b>不會減少他上架商品的庫存</b>。
     *
     * <p>⇒ lane 的差別只有兩件事：賣方是誰（{@code storeId}）、政策五欄走哪一組。
     *
     * <p><b>政策五欄刻意不走 {@link OrderPolicyResolver}</b>：resolver 的輸入是「配送方式」，
     * 而 RFQ 的政策來自「賣方是誰」。⚠️ 用錯值<b>不會被 CHECK 擋下</b>——三條 CHECK 只驗
     * 五欄彼此一致，不驗它對不對這張訂單；錯的代價是撥款與開票的法源整組錯掉而沒有任何
     * 東西會發現。
     */
    public RfqOrderResult createRfqOrder(
            long buyerId, long storeId, long acceptanceId,
            long totalAmount, long shippingFee,
            UUID skuId, String skuName, long unitPrice,
            String deliveryTemp, String lane
    ) {
        OrderEntity existing = orderRepository.findByAcceptanceId(acceptanceId);
        if (existing != null) {
            return new RfqOrderResult(existing.getOrderNo(), existing.getPaymentDeadline(), true);
        }

        if (!RFQ_LANE_PLATFORM.equals(lane) && !RFQ_LANE_EXTERNAL.equals(lane)) {
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }

        OrderPolicySnapshot policySnapshot = new OrderPolicySnapshot(
                CommerceModel.SELLER_DIRECT.value(),
                CollectionModel.SELLER_MERCHANT.value(),
                FulfillmentMode.SELLER_MANAGED.value(),
                InvoiceModel.SELLER_TO_BUYER.value(),
                OrderPolicyResolver.POLICY_VERSION);

        if (!sellerGrpcClient.getSellerMeta(storeId).getCanAcceptOrders()) {
            throw new OrderBusinessException(GrpcResponseCode.SELLER_NOT_ONBOARDED);
        }
        String merId = "";

        List<CreateOrderItemParam> items = List.of(new CreateOrderItemParam(
                skuId, skuName, 1, unitPrice, unitPrice, 0L, "", null, null));

        try {
            OrderEntity created = orderCreationTxHandler.createRfqOrderInTx(
                    buyerId, storeId, acceptanceId, items,
                    totalAmount, shippingFee,
                    deliveryTemp,
                    ShippingMethod.MERCHANT_DELIVERY.value(),
                    ShippingFeePayer.BUYER.value(),
                    merId,
                    policySnapshot);
            log.info("OrderService.createRfqOrder.success",
                    "RFQ 建單成功 acceptanceId=" + acceptanceId, "orderNo", created.getOrderNo());
            ordersCreatedCounter.increment();
            return new RfqOrderResult(created.getOrderNo(), created.getPaymentDeadline(), false);
        } catch (DuplicateKeyException e) {
            OrderEntity raced = orderRepository.findByAcceptanceId(acceptanceId);
            if (raced != null) {
                return new RfqOrderResult(raced.getOrderNo(), raced.getPaymentDeadline(), true);
            }
            throw e;
        }
    }

    private void compensateStock(String stockTxId, List<StockItem> stockItems) {
        try {
            StockOperationResponse response = productGrpcClient.restoreStock(stockTxId, stockItems);
            if (StockOperationStatus.SUCCESS.value().equals(response.getStatus())
                    || StockOperationStatus.ALREADY_APPLIED.value().equals(response.getStatus())
                    || StockOperationStatus.NOOP_NOT_FOUND.value().equals(response.getStatus())) {
                stockTxRecordRepository.insertRestoreIfAbsent(stockTxId, null,
                        OrderConstants.STOCK_TX_PHASE_RESTORE, OrderConstants.STOCK_TX_STATUS_SUCCESS, "{}");
            } else {
                log.error("OrderService.compensateStock.failed", "庫存補償回傳非成功狀態: " + response.getStatus());
                writeFailedRestoreRecord(stockTxId, "回傳狀態: " + response.getStatus());
            }
        } catch (Exception e) {
            log.error("OrderService.compensateStock.error", "庫存補償 gRPC 失敗", e);
            writeFailedRestoreRecord(stockTxId, e.getMessage());
        }
    }

    private void writeFailedRestoreRecord(String stockTxId, String error) {
        if (stockTxRecordRepository.findByStockTxIdAndPhase(
                stockTxId, OrderConstants.STOCK_TX_PHASE_RESTORE) != null) {
            return;
        }
        StockTxRecordEntity restoreRecord = new StockTxRecordEntity();
        restoreRecord.setStockTxId(stockTxId);
        restoreRecord.setPhase(OrderConstants.STOCK_TX_PHASE_RESTORE);
        restoreRecord.setStatus(OrderConstants.STOCK_TX_STATUS_FAILED);
        restoreRecord.setPayloadSnapshot("{}");
        restoreRecord.setLastError(error);
        restoreRecord.setRetryCount(1);
        restoreRecord.setNextRetryTime(DateTimeUtils.nowUtc().plusSeconds(60));
        stockTxRecordRepository.insert(restoreRecord);
    }

    @Transactional
    public void cancelOrder(long buyerId, String orderNo) {
        OrderEntity order = orderRepository.findByOrderNoForUpdate(orderNo);
        if (order == null) {
            throw new OrderNotFoundException(orderNo);
        }
        if (order.getBuyerId() != buyerId) {
            throw new OrderNotFoundException(orderNo);
        }
        if (!OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus())) {
            throw new InvalidStatusException("僅待付款訂單可取消");
        }

        int updated = orderRepository.cancelOrderByBuyerCas(orderNo,
                OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value());
        if (updated == 0) {
            throw new InvalidStatusException("訂單狀態已變更，無法取消");
        }
        insertCancelledOutbox(orderNo);
        enqueueCouponReleaseIfHeld(order);
        eventPublisher.emitSellerOrderUpdated(
                order.getStoreId(), orderNo, OrderEventPublisher.SRC_SYSTEM,
                OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(), null);
    }

    /**
     * 售後內部取消訂單的狀態變更 + 庫存歸還事件同事務落地（#1368）。
     *
     * <p>從 {@link io.github.will413028.hawkr.order.grpc.OrderGrpcService#cancelOrderInternal} 抽出——原 CAS 為
     * 單語句 auto-commit + TX 外 best-effort 發庫存歸還事件，事件遺失時庫存永久少計。改為 @Transactional
     * 使「狀態 CAS + order.cancelled outbox 寫入」原子；gRPC handler（非 proxy bean）跨 bean 呼叫本方法，
     * 避免 §11.1 self-invocation 事務失效。
     *
     * @return CAS 實際更新的列數（0 表示狀態已變更，呼叫端據此回 INVALID_STATUS）
     */
    @Transactional
    public int cancelOrderInternalInTx(String orderNo, String cancelReason, String cancelRemark, String cancelByRole) {
        int updated = orderRepository.cancelOrderInternalCas(
                orderNo,
                OrderStatus.CANCELLED.value(),
                cancelReason,
                cancelRemark != null ? cancelRemark : "",
                cancelByRole);
        if (updated == 0) {
            return 0;
        }
        insertCancelledOutbox(orderNo);
        enqueueCouponReleaseIfHeld(orderRepository.findByOrderNo(orderNo));
        return updated;
    }

    /**
     * 訂單持有優惠券時，在取消的 transaction 內寫入釋券任務（與狀態變更、order.cancelled outbox 同時提交或回滾）。
     * 取消提交後呼叫端的當場釋放只是加速，漏了也由任務補上。沒有記下券 id 的訂單由各自的補償路徑處理。
     */
    public void enqueueCouponReleaseIfHeld(OrderEntity order) {
        if (order == null) {
            return;
        }
        String couponInstanceId = order.getCouponInstanceId();
        if (couponInstanceId != null && !couponInstanceId.isEmpty()) {
            couponReleaseService.enqueueInTx(order.getOrderNo(), couponInstanceId);
        }
    }

    /**
     * 取消事務內落 order.cancelled 至發件匣（#1368，取代原 TX 外 best-effort 直發）。
     *
     * <p>由三個取消入口（買家取消 / 售後內部取消 / 逾時・停權取消）在各自取消事務內呼叫，與狀態變更
     * 原子落地，再由 {@link io.github.will413028.hawkr.order.scheduler.OrderOutboxPublisherScheduler} 保證至少一次投遞，
     * 完全鏡像 {@code order.paid} 的可靠投遞（03 §8.10）。
     *
     * <p>本方法只讀本地扣減快照組「核心 payload」（orderNo / stockTxId / items / 買賣家業務鍵 / cancelReason），
     * notification 用的 seller/buyer 反解析與 sellerName / avatar 補值延到派送時由
     * {@link io.github.will413028.hawkr.order.mq.OrderOutboxPublisher} enrich（鏡像 order.paid），避免在取消事務內做 gRPC。
     * core payload 寫 {@code storeId}（本地事實），{@code notifySellerAccountId} / {@code notifyBuyerAccountId}
     * 留 null 由派送時 enrich 反解析成登入 account_id
     * （2026-08-05 之前 storeId 曾暫存在已於 Phase C 移除的 sellerAccountId 欄位裡，那個雙義已拆除）。
     *
     * <p>無成功扣減紀錄 → 不落 outbox（無庫存可歸還，與原 early-return 行為一致）。序列化失敗拋例外
     * rollback，避免取消已記但事件遺失。
     */
    public void insertCancelledOutbox(String orderNo) {
        StockTxRecordEntity deductRecord = stockTxRecordRepository.findDeductByOrderNo(orderNo);
        if (deductRecord == null || !OrderConstants.STOCK_TX_STATUS_SUCCESS.equals(deductRecord.getStatus())) {
            return;
        }

        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            log.error("OrderService.insertCancelledOutbox.orderNotFound",
                    "查無訂單，跳過 order.cancelled 落 outbox", "orderNo", orderNo);
            return;
        }

        List<StockItem> stockItems = parsePayloadToStockItems(deductRecord.getPayloadSnapshot());
        List<OrderCancelledPayload.StockItemPayload> items = stockItems.stream()
                .map(si -> new OrderCancelledPayload.StockItemPayload(UUID.fromString(si.getSkuId()), si.getQuantity()))
                .toList();

        OrderCancelledPayload core = new OrderCancelledPayload(
                orderNo,
                deductRecord.getStockTxId(),
                items,
                DateTimeUtils.nowUtc().toString(),
                order.getBuyerId(),
                null,
                order.getStoreId(),
                null,
                null,
                null,
                order.getCancelReason());

        OrderOutboxEntity outbox = new OrderOutboxEntity();
        outbox.setAggregateId(orderNo);
        outbox.setEventType(OrderEventType.ORDER_CANCELLED.value());
        outbox.setExchange(MqConstants.EXCHANGE_ORDER);
        outbox.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
        outbox.setStatus(OutboxEventStatus.PENDING.value());
        outbox.setRetryCount(0);
        try {
            outbox.setPayload(MAPPER.writeValueAsString(core));
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("order.cancelled payload 序列化失敗 orderNo=" + orderNo, e);
        }
        orderOutboxMapper.insert(outbox);
    }

    /**
     * @deprecated 取消歸還改走 {@link #insertCancelledOutbox(String)} 事務內落 outbox，此方法僅供 StockCompensationWorker fallback
     */
    @Deprecated
    public void releaseStockAfterCancel(String orderNo) {
        StockTxRecordEntity deductRecord = stockTxRecordRepository.findDeductByOrderNo(orderNo);
        if (deductRecord == null || !OrderConstants.STOCK_TX_STATUS_SUCCESS.equals(deductRecord.getStatus())) {
            return;
        }

        List<StockItem> items = parsePayloadToStockItems(deductRecord.getPayloadSnapshot());
        compensateStock(deductRecord.getStockTxId(), items);
    }

    public OrderEntity getOrderByNo(String orderNo) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            throw new OrderNotFoundException(orderNo);
        }
        return order;
    }

    public List<OrderItemEntity> getOrderItems(String orderNo) {
        return orderItemRepository.findByOrderNo(orderNo);
    }

    public List<RefundRequestEntity> getRefundHistory(String orderNo) {
        return refundRequestRepository.findByOrderNo(orderNo);
    }

    /**
     * 更新訂單的 coupon_instance_id（reserve 成功後寫入 Instance ID）。
     * 03 §2.1 INV-3：走 updateCouponInstanceIdIfAbsent CAS（WHERE coupon_instance_id IS NULL），冪等。
     */
    @Transactional
    public void updateCouponInstanceId(String orderNo, String instanceId) {
        int updated = orderRepository.updateCouponInstanceIdIfAbsent(orderNo, instanceId);
        if (updated == 0) {
            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) {
                throw new OrderNotFoundException(orderNo);
            }
            log.info("OrderService.updateCouponInstanceId.skip",
                    "coupon_instance_id 已存在，跳過寫入；existing="
                            + order.getCouponInstanceId() + ", incoming=" + instanceId,
                    "orderNo", orderNo);
        }
    }

    public List<FinancialLogEntity> getFinancialLogs(String sourceRefNo) {
        return financialLogRepository.findBySourceRefNo(sourceRefNo);
    }

    public List<OrderEntity> getMyOrders(long buyerId, String statusFilter, int page, int size) {
        int offset = (page - 1) * size;
        List<String> statusList = parseStatusFilter(statusFilter);
        return orderRepository.findByUserIdAndStatus(buyerId, statusList, size, offset);
    }

    public long countMyOrders(long buyerId, String statusFilter) {
        List<String> statusList = parseStatusFilter(statusFilter);
        return orderRepository.countByUserIdAndStatus(buyerId, statusList);
    }

    /**
     * 買家訂單各 OrderStatus 原始筆數，規則見 03 設計書 §2.1「買家訂單 Tab 筆數 Rollup（SSOT）」。
     */
    public List<OrderStatusCountResult> getMyOrderStatusCounts(long buyerId) {
        return orderRepository.countMyOrdersGroupByStatus(buyerId);
    }

    /**
     * 組裝賣家訂單列表的查詢條件（56 號設計書 §6.1.10 的 Query 契約）。
     * <p>
     * 列表與筆數必須用同一個回傳實例，條件才不會分岔（見 {@link SellerOrderQuery}）。
     *
     * @throws IllegalArgumentException 日期非空但格式不合法（GrpcErrorHandler 映射為 INVALID_PARAM）
     */
    public SellerOrderQuery sellerOrderQuery(long storeId, long buyerId, String statusFilter,
                                             String keyword, String dateFrom, String dateTo) {
        return new SellerOrderQuery(storeId, buyerId, parseStatusFilter(statusFilter),
                toKeywordPattern(keyword), parseDateBound(dateFrom, "date_from", false),
                parseDateBound(dateTo, "date_to", true));
    }

    public List<OrderEntity> getSellerOrders(SellerOrderQuery query, int page, int size) {
        int offset = (page - 1) * size;
        return orderRepository.findSellerOrders(query, size, offset);
    }

    public long countSellerOrders(SellerOrderQuery query) {
        return orderRepository.countSellerOrders(query);
    }

    /**
     * keyword → ILIKE pattern；null／空白代表不篩選（回 null）。
     * <p>
     * LIKE 的萬用字元（{@code %}、{@code _}）與跳脫字元本身（{@code \}）先跳脫，否則使用者搜
     * 「100%」會變成「任意字串」而撈出整個店的訂單。PostgreSQL 的 LIKE 預設跳脫字元就是反斜線，
     * 故不另寫 ESCAPE 子句（寫 {@code ESCAPE '\'} 反而在 standard_conforming_strings 關閉時炸掉）。
     */
    private static String toKeywordPattern(String keyword) {
        if (keyword == null || keyword.isBlank()) {
            return null;
        }
        String escaped = keyword.trim()
                .replace("\\", "\\\\")
                .replace("%", "\\%")
                .replace("_", "\\_");
        return "%" + escaped + "%";
    }

    /**
     * ISO 8601 UTC 字串 → created_at 篩選邊界；null／空白代表不篩選（回 null）。
     * <p>
     * 兩個邊界都用閉區間（SQL 為 {@code >=} / {@code <=}），差別只在純日期形的補值：
     * {@code upperBound=true} 且輸入是 {@code "2026-08-13"} 這種純日期時，代表「整個 8/13 都要」，
     * 補到當日最後一微秒（PostgreSQL timestamptz 的精度上限；不用 {@link java.time.LocalTime#MAX}
     * 的 999999999ns，那會被 JDBC 四捨五入進位到隔天 00:00:00）。帶時間的形式逐字採用。
     * <p>
     * 格式不合法時明確拋錯而不是靜默忽略：靜默忽略會讓使用者設了日期卻拿到全部訂單，
     * 而且畫面上看不出篩選沒生效。
     */
    private static LocalDateTime parseDateBound(String iso, String fieldName, boolean upperBound) {
        if (iso == null || iso.isBlank()) {
            return null;
        }
        LocalDateTime parsed = DateTimeUtils.parseIso8601(iso);
        if (parsed == null) {
            throw new IllegalArgumentException(fieldName + " 格式不合法（需 ISO 8601 UTC）: " + iso);
        }
        boolean dateOnly = iso.trim().length() == 10 && iso.indexOf('T') < 0;
        return upperBound && dateOnly
                ? parsed.toLocalDate().atTime(23, 59, 59, 999_999_000)
                : parsed;
    }

    /**
     * 解析狀態篩選參數：支援逗號分隔多值（如 "15,25"），空字串表示不篩選（全部）
     */
    private List<String> parseStatusFilter(String statusFilter) {
        if (statusFilter == null || statusFilter.isBlank()) {
            return List.of();
        }
        return java.util.Arrays.stream(statusFilter.split(","))
                .map(String::trim)
                .filter(s -> !s.isEmpty())
                .toList();
    }

    /**
     * 賣家工作台 Dashboard 四桶 count（03 §2.1 SSOT）。
     * <p>
     * 回 {@code long[4]}：{pending, inProgress, closed, aftersaleCaseCount}。
     * 三桶（pending/inProgress/closed）以 order.status 分類並疊加售後覆蓋：
     * <ul>
     *   <li>有 active 售後的訂單從生命週期三桶排除、計入 aftersale queue；</li>
     *   <li>terminal-override（case status∈{33,50} 無申訴）售後的訂單若 order.status 未達終態則補入第三桶；</li>
     *   <li>三桶互斥、PENDING_PAYMENT(0) 不計入。</li>
     * </ul>
     * isTcatCalled（已出貨）窗口不在此處理：order.status 由 ShippingEventConsumer 在 210/921 推進到 30，最終一致。
     * <p>
     * ⚠️ <b>第三桶是「已結束」不是「已完成」</b>：它的語意是「賣家無需再動作」，成員含
     * CANCELLED(80) 與 FULLY_REFUNDED(70)，因此<b>不是成交數</b>。賣家面（web 狀態卡 / App tab 與
     * 儀表板）一律顯示「已結束」；「已完成」是 CodeMeta {@code OrderStatus} 50 單一狀態的 Label，
     * 只在代行面按狀態逐一計數時才成立。proto 欄名 {@code completed_count} 與 Gateway 對外 JSON key
     * {@code completedCount} 是既有 wire 契約（已被 App 消費）故保留，語意以本段為準。
     */
    public long[] getSellerOrderStatusCounts(long storeId) {
        SellerCaseClassification cls = afterSalesGrpcClient.getSellerCaseClassification(storeId);
        List<String> excludeOrderNos = new ArrayList<>(cls.activeOrderNos());
        excludeOrderNos.addAll(cls.completedOverrideOrderNos());

        long pending = orderRepository.countBySellerStatusExcludingOrders(
                storeId,
                List.of(OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value()),
                excludeOrderNos);

        long inProgress = orderRepository.countBySellerStatusExcludingOrders(
                storeId,
                List.of(OrderStatus.IN_TRANSIT.value(), OrderStatus.WAITING_PICKUP.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value(), OrderStatus.DELIVERED.value()),
                excludeOrderNos);

        long closedBase = orderRepository.countBySellerStatusExcludingOrders(
                storeId,
                List.of(OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value(),
                        OrderStatus.FULLY_REFUNDED.value(), OrderStatus.CANCELLED.value()),
                cls.activeOrderNos());

        long closedOverride = orderRepository.countBySellerOrdersInStatus(
                storeId,
                cls.completedOverrideOrderNos(),
                List.of(OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value(),
                        OrderStatus.IN_TRANSIT.value(), OrderStatus.WAITING_PICKUP.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value(), OrderStatus.DELIVERED.value()));

        long closed = closedBase + closedOverride;

        return new long[]{pending, inProgress, closed, cls.activeCaseCount()};
    }

    /**
     * 成立訂單（qualifying order）的狀態集合：已付款、未取消、未全額退款（已付款到完成，加部分退款；部分退款仍是成立的購買）。
     * 營收排名、各店訂單統計與單店分時間窗的訂單統計共用這一份，口徑只有一處。
     */
    public static final List<String> QUALIFYING_ORDER_STATUSES = List.of(
            OrderStatus.PAID.value(),
            OrderStatus.PENDING_SHIPMENT.value(),
            OrderStatus.IN_TRANSIT.value(),
            OrderStatus.WAITING_PICKUP.value(),
            OrderStatus.SHIPPING_ABNORMAL.value(),
            OrderStatus.DELIVERED.value(),
            OrderStatus.COMPLETED.value(),
            OrderStatus.PARTIALLY_REFUNDED.value());

    public List<SellerRevenueResult> getTopSellersByRevenue(int days, int limit) {
        if (days <= 0) days = 30;
        if (limit <= 0) limit = 10;
        if (limit > 50) limit = 50;
        return orderRepository.findTopSellersByRevenue(days, limit, QUALIFYING_ORDER_STATUSES);
    }

    /** ListStoreOrderStats 的一頁：rows 至多 pageSize 筆；nextAfterStoreId 0＝已是最後一頁。 */
    public record StoreOrderStatsPage(List<StoreOrderStatsResult> rows, long nextAfterStoreId) {}

    /**
     * 近 N 天各店的成交買家數、成交訂單數與賣家取消訂單數（keyset 分頁，store_id 升冪）。
     * 成交狀態與 {@link #getTopSellersByRevenue} 同一組；呼叫端（gRPC）已驗證 days 與 pageSize 的範圍。
     */
    public StoreOrderStatsPage listStoreOrderStats(int days, long afterStoreId, int pageSize) {
        List<StoreOrderStatsResult> fetched = orderRepository.findStoreOrderStats(
                days, afterStoreId, pageSize + 1, QUALIFYING_ORDER_STATUSES,
                OrderStatus.CANCELLED.value(), "SELLER");
        if (fetched.size() <= pageSize) {
            return new StoreOrderStatsPage(fetched, 0L);
        }
        List<StoreOrderStatsResult> rows = fetched.subList(0, pageSize);
        return new StoreOrderStatsPage(rows, rows.get(pageSize - 1).getStoreId());
    }

    /**
     * 單店在多個時間窗 [startMs, endMs)（Unix ms，以付款時間計）內的成立訂單統計，依輸入順序回傳，窗內沒有訂單時各值為 0。
     * 口徑與 {@link #listStoreOrderStats} 相同（{@link #QUALIFYING_ORDER_STATUSES}，不排除自買單）；呼叫端（gRPC）已驗證參數。
     */
    public List<StorePeriodOrderStatsResult> getStorePeriodOrderStats(long storeId, List<long[]> windows) {
        List<StorePeriodOrderStatsResult> out = new ArrayList<>(windows.size());
        for (long[] w : windows) {
            out.add(orderRepository.findStorePeriodOrderStats(storeId, w[0], w[1], QUALIFYING_ORDER_STATUSES));
        }
        return out;
    }

    public List<OrderEntity> searchOrders(String orderNo, long buyerId, long storeId, String status, int page, int size) {
        int offset = (page - 1) * size;
        return orderRepository.searchOrders(orderNo != null ? orderNo : "", buyerId, storeId, status, size, offset);
    }

    public long countSearchOrders(String orderNo, long buyerId, long storeId, String status) {
        return orderRepository.countSearchOrders(orderNo != null ? orderNo : "", buyerId, storeId, status);
    }


    /**
     * 取得物流建單所需訂單資訊（純內部呼叫）
     *
     * 授權由 HMAC internal-auth（server interceptor 驗簽 → INTERNAL_CALL）＋ netpol 保證，
     * 不再依賴 storeId==0 哨兵繞歸屬（#1366：魔數繞歸屬移除）。此 RPC 無 Gateway 入口、
     * 無帶真實 storeId 的活躍 caller，屬純內部讀。
     */
    public OrderEntity getOrderForShipping(String orderNo) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            throw new OrderNotFoundException(orderNo);
        }
        return order;
    }


    /**
     * 完成訂單（共用邏輯：買家確認收貨 / 自動完成共用）
     *
     * TX 內：DELIVERED(40) → COMPLETED(50) + 建立撥款任務
     *        + order.completed / order.receipt.confirmed 落 order_outbox（issue #2294 B8）
     * 冪等：僅 DELIVERED(40) 可轉 COMPLETED(50)
     *
     * @param orderNo     訂單編號
     * @param confirmType MANUAL（買家主動）或 AUTO（排程自動）
     * @return true=成功完成, false=冪等跳過
     */
    public boolean completeOrder(String orderNo, String confirmType) {
        return completeOrder(orderNo, confirmType, null);
    }

    /**
     * 完成訂單（含買家收貨照片快照）
     *
     * @param receiptPhotoKeysJson buyer 上傳照片 objectKey 轉為 JSON 字串（null 表示無）
     */
    public boolean completeOrder(String orderNo, String confirmType, String receiptPhotoKeysJson) {
        return orderCompletionTxHandler.completeOrderInTx(orderNo, confirmType, receiptPhotoKeysJson);
    }

    /**
     * 買家確認收貨
     * 驗證買家身分 + 訂單狀態為 DELIVERED(25) 後呼叫共用完成邏輯
     */
    public void confirmReceipt(long buyerId, String orderNo) {
        confirmReceipt(buyerId, orderNo, null);
    }

    /**
     * 買家確認收貨（含選填收貨照片）
     *
     * @param receiptPhotoKeysJson 收貨照片 objectKey 的 JSON 陣列字串（null 表示未附照片）
     */
    public void confirmReceipt(long buyerId, String orderNo, String receiptPhotoKeysJson) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            throw new OrderNotFoundException(orderNo);
        }
        if (order.getBuyerId() != buyerId) {
            throw new OrderNotFoundException(orderNo);
        }
        if (!OrderStatus.DELIVERED.value().equals(order.getStatus())) {
            if (OrderStatus.COMPLETED.value().equals(order.getStatus())) {
                log.info("OrderService.confirmReceipt.idempotent", "訂單已完成，跳過", "orderNo", orderNo);
                return;
            }
            throw new InvalidStatusException("僅已送達訂單可確認收貨，當前狀態: " + order.getStatus());
        }

        AfterSalesGrpcClient.ConfirmReceiptCaseResult caseResult =
                afterSalesGrpcClient.cancelActiveCaseOnConfirmReceipt(orderNo, buyerId);
        if (caseResult.outcome() == AfterSalesGrpcClient.ConfirmReceiptOutcome.BLOCKED) {
            log.info("OrderService.confirmReceipt.blockedByAftersale",
                    "訂單有進行中售後 case，擋住確認收貨, caseNo=" + caseResult.caseNo()
                            + ", blockingStatus=" + caseResult.blockingStatus(),
                    "orderNo", orderNo);
            throw new OrderBusinessException(GrpcResponseCode.AFTERSALE_CONFIRM_RECEIPT_BLOCKED);
        }

        completeOrder(orderNo, OrderConstants.CONFIRM_TYPE_MANUAL, receiptPhotoKeysJson);
    }


    /**
     * 建單即清購物車（best-effort + 重試任務補償）
     */
    private void cleanupCartAfterOrderCreation(long buyerId, String checkoutId, String orderNo) {
        cartGrpcClient.removeItemsByCheckoutId(buyerId, checkoutId);

        try {
            CartCleanupRetryTaskEntity task = new CartCleanupRetryTaskEntity();
            task.setTaskId(TsidUtil.generate());
            task.setOrderNo(orderNo);
            task.setCheckoutId(checkoutId);
            task.setBuyerId(buyerId);
            task.setStatus(OrderConstants.TASK_STATUS_PENDING);
            task.setRetryCount(0);
            task.setMaxRetryCount(OrderConstants.DEFAULT_MAX_RETRY_COUNT);
            task.setNextRetryTime(DateTimeUtils.nowUtc());
            cartCleanupRetryTaskRepository.insert(task);
        } catch (Exception e) {
            log.error("OrderService.cleanupCart.taskFailed", "建立購物車清理重試任務失敗", e);
        }
    }

    /**
     * 根據訂單號解鎖購物車（建單前失敗補償） 降級：失敗不阻擋主流程
     */
    public void unlockCartForOrder(String orderNo) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            return;
        }
        String checkoutId = order.getCheckoutId();
        if (checkoutId != null && !checkoutId.isEmpty()) {
            cartGrpcClient.checkoutUnlock(order.getBuyerId(), checkoutId);
        }
    }

    /**
     * 根據訂單號清理購物車（付款成功後呼叫） 降級：失敗不影響訂單
     */
    public void removeCartItemsForOrder(String orderNo) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            return;
        }
        String checkoutId = order.getCheckoutId();
        if (checkoutId != null && !checkoutId.isEmpty()) {
            cartGrpcClient.removeItemsByCheckoutId(order.getBuyerId(), checkoutId);
        }
    }


    /**
     * 03 §5.1.2：CartService.CheckoutLock 失敗時的分流。
     * <p>反查 buyer 是否已有 PENDING_PAYMENT 訂單包含當前任一 sku：
     * 有 → 拋 ORDER_ALREADY_CREATED(4092) 帶 orderNo，前端引導使用者前往付款；
     * 沒有 → 拋 CHECKOUT_LOCK_FAILED(6005)（真併發或殘留 LOCKED）。
     * <p>嚴禁將 lock 失敗一律壓成通用 CONFLICT(409)。
     */
    private void throwLockFailureWithOrderAlreadyCreatedCheck(long buyerId, List<CreateOrderItemParam> items) {
        List<UUID> skuIds = items.stream()
                .map(CreateOrderItemParam::skuId)
                .toList();
        OrderEntity existing = orderRepository.findLatestPendingPaymentByBuyerAndSkus(buyerId, skuIds);
        if (existing != null) {
            log.info("OrderService.checkoutLock.alreadyCreated",
                    "鎖定失敗但 buyer 已有 PENDING_PAYMENT 訂單，回 ORDER_ALREADY_CREATED",
                    "orderNo", existing.getOrderNo());
            throw new OrderBusinessException(GrpcResponseCode.ORDER_ALREADY_CREATED, existing.getOrderNo());
        }
        log.warn("OrderService.checkoutLock.failed",
                "鎖定失敗且查無對應 PENDING_PAYMENT 訂單，可能為真併發或殘留 LOCKED buyerId=" + buyerId);
        throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_LOCK_FAILED);
    }

    private String serializePayload(List<CreateOrderItemParam> items) {
        try {
            List<PayloadItem> payloadItems = items.stream()
                    .map(i -> new PayloadItem(i.skuId(), i.quantity()))
                    .toList();
            return MAPPER.writeValueAsString(payloadItems);
        } catch (JsonProcessingException e) {
            throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "序列化 payload 失敗");
        }
    }

    private List<StockItem> parsePayloadToStockItems(String json) {
        try {
            PayloadItem[] payloadItems = MAPPER.readValue(json, PayloadItem[].class);
            return java.util.Arrays.stream(payloadItems)
                    .map(p -> StockItem.newBuilder().setSkuId(String.valueOf(p.skuId())).setQuantity(String.valueOf(p.quantity())).build())
                    .toList();
        } catch (JsonProcessingException e) {
            throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "解析 payload 失敗");
        }
    }


    /**
     * 結帳模組專用建單入口
     * 支援 checkout_id 冪等檢查 + 收件人/配送區域/優惠券等新欄位
     */
    public String createOrderFromCheckout(CreateOrderParams params) {
        if (params.checkoutId() != null && !params.checkoutId().isEmpty()) {
            OrderEntity existing = orderRepository.findByCheckoutId(params.checkoutId());
            if (existing != null) {
                log.info("OrderService.createOrderFromCheckout.idempotent", "冪等命中，返回既有訂單",
                        "orderNo", existing.getOrderNo());
                return existing.getOrderNo();
            }
        }

        return createOrder(
                params.buyerId(), params.storeId(), params.items(),
                params.totalOriginalAmount(), params.totalActualAmount(),
                params.checkoutId(), params.deliveryTemp(), params.shippingMethod(),
                params.shippingFee(), params.shippingFeePayer(),
                params.recipientName(), params.recipientPhone(),
                params.shippingAddress(), params.shippingNote(),
                params.orderRemark(), params.cvsStoreId(), params.cvsStoreName(),
                params.shippingArea(), params.senderCity(), params.senderDistrict(),
                params.dimensionSum(), params.merId(), params.discountAmount(),
                params.invoiceSnapshot(), params.policySnapshot()
        );
    }

    /**
     * 建立訂單（擴展版，含收件人/配送區域/優惠券欄位）
     */
    public String createOrder(
            long buyerId, long storeId,
            List<CreateOrderItemParam> items,
            long totalOriginalAmount, long totalActualAmount,
            String checkoutId, String deliveryTemp, String shippingMethod,
            long shippingFee, String shippingFeePayer,
            String recipientName, String recipientPhone,
            String shippingAddress, String shippingNote,
            String orderRemark, String cvsStoreId, String cvsStoreName,
            String shippingArea, String senderCity, String senderDistrict,
            int dimensionSum, String merId, long discountAmount,
            io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver.Snapshot invoiceSnapshot,
            io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot policySnapshot
    ) {
        uberOrderEligibilityGuard.requireEligible(
                io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Context.methodOnly(shippingMethod));

        if (recipientName != null && shippingMethod != null && !shippingMethod.isEmpty()) {
            if (ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value().equals(shippingMethod)) {
                InputValidator.requireDisplayLengthBetween(recipientName,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                        PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名");
            } else {
                InputValidator.requireDisplayLengthBetween(recipientName,
                        1, PayUniShippingConstraints.HOME_RECIPIENT_NAME_MAX, "收件人姓名");
                if (shippingAddress != null && !shippingAddress.isEmpty()) {
                    InputValidator.requireDisplayLengthBetween(shippingAddress,
                            1, PayUniShippingConstraints.RECIPIENT_ADDRESS_MAX, "收件地址");
                }
            }
        }

        items = new ArrayList<>(items);
        items.sort(Comparator.comparing(CreateOrderItemParam::skuId));

        boolean cartLocked = false;
        if (checkoutId != null && !checkoutId.isEmpty()) {
            List<String> skuIds = items.stream()
                    .map(i -> String.valueOf(i.skuId()))
                    .toList();
            int lockResult = cartGrpcClient.checkoutLock(buyerId, checkoutId, skuIds);
            if (lockResult != 0) {
                throwLockFailureWithOrderAlreadyCreatedCheck(buyerId, items);
            }
            cartLocked = true;
        }

        try {
            String stockTxId = UUID.randomUUID().toString();
            List<StockItem> stockItems = items.stream()
                    .map(i -> StockItem.newBuilder()
                            .setSkuId(String.valueOf(i.skuId()))
                            .setQuantity(String.valueOf(i.quantity())).build())
                    .toList();

            String payloadJson = serializePayload(items);
            StockTxRecordEntity initRecord = new StockTxRecordEntity();
            initRecord.setStockTxId(stockTxId);
            initRecord.setPhase(OrderConstants.STOCK_TX_PHASE_DEDUCT);
            initRecord.setStatus(OrderConstants.STOCK_TX_STATUS_INIT);
            initRecord.setPayloadSnapshot(payloadJson);
            initRecord.setRetryCount(0);
            stockTxRecordRepository.insert(initRecord);

            StockOperationResponse deductResponse;
            try {
                deductResponse = productGrpcClient.deductStock(stockTxId, stockItems);
            } catch (Exception e) {
                log.error("OrderService.createOrder.deductStock.error", "扣庫存 gRPC 失敗", e);
                stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT,
                        OrderConstants.STOCK_TX_STATUS_FAILED, e.getMessage(), 60);
                throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE, "庫存服務不可用，請稍後重試");
            }

            StockOperationStatus deductOpStatus2;
            try {
                deductOpStatus2 = StockOperationStatus.fromValue(deductResponse.getStatus());
            } catch (IllegalArgumentException e) {
                stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT,
                        OrderConstants.STOCK_TX_STATUS_FAILED,
                        deductResponse.getStatus() + ": " + deductResponse.getMessage(), 60);
                throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "庫存扣減失敗（未知 status）: " + deductResponse.getStatus());
            }
            switch (deductOpStatus2) {
                case SUCCESS, ALREADY_APPLIED -> { }
                case INSUFFICIENT_STOCK -> {
                    stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT,
                            OrderConstants.STOCK_TX_STATUS_FAILED, "庫存不足", 0);
                    List<InsufficientStockException.Item> insufficientItems = deductResponse.getInsufficientItemsList().stream()
                            .map(item -> new InsufficientStockException.Item(
                                    item.getSkuId(), item.getRequestedQty(), item.getAvailableQty()))
                            .toList();
                    throw new InsufficientStockException(deductResponse.getMessage(), insufficientItems);
                }
                default -> {
                    stockTxRecordRepository.updateFailed(stockTxId, OrderConstants.STOCK_TX_PHASE_DEDUCT,
                            OrderConstants.STOCK_TX_STATUS_FAILED,
                            deductResponse.getStatus() + ": " + deductResponse.getMessage(), 60);
                    throw new OrderBusinessException(GrpcResponseCode.INTERNAL_ERROR, "庫存扣減失敗: " + deductResponse.getMessage());
                }
            }

            String orderNo;
            try {
                orderNo = orderCreationTxHandler.createOrderInTxExtended(buyerId, storeId, items,
                        totalOriginalAmount, totalActualAmount, stockTxId, checkoutId,
                        deliveryTemp, shippingMethod, shippingFee, shippingFeePayer,
                        recipientName, recipientPhone, shippingAddress, shippingNote,
                        orderRemark, cvsStoreId, cvsStoreName, shippingArea, senderCity, senderDistrict,
                        dimensionSum, merId, discountAmount, invoiceSnapshot, policySnapshot);
            } catch (Exception e) {
                log.error("OrderService.createOrder.dbTx.error", "建單 DB 交易失敗，執行庫存補償", e);
                compensateStock(stockTxId, stockItems);
                throw e;
            }

            log.info("OrderService.createOrder.success", "建立訂單成功", "orderNo", orderNo);
            ordersCreatedCounter.increment();

            if (checkoutId != null && !checkoutId.isEmpty()) {
                cleanupCartAfterOrderCreation(buyerId, checkoutId, orderNo);
            }

            return orderNo;
        } catch (Exception e) {
            if (cartLocked) {
                cartGrpcClient.checkoutUnlock(buyerId, checkoutId);
            }
            throw e;
        }
    }


    /**
     * 建單明細參數。{@code skuId} 為 {@code null} 代表 <b>non-catalog line</b>——
     * RFQ 成交的明細不指向任何 {@code product_sku}（64 號 §6.4.9、§6.4.15）。
     *
     * <p>🔴 <b>{@code skuId} 放寬成可為 {@code null} 之後，「忘記帶 sku_id」不再是編譯錯誤</b>。
     * 改由 {@link #catalog} 承接：catalog 的兩個呼叫端一律走它，工廠方法在執行時拒絕
     * {@code null}。canonical constructor 只保留給 RFQ 這條
     * non-catalog 路徑（Java 不允許 record 的 canonical constructor 比 record 本身更
     * 封閉，所以擋不住，但 catalog 呼叫端沒有理由繞過工廠方法）。
     */
    public record CreateOrderItemParam(
            UUID skuId,
            String skuName,
            int quantity,
            long unitPrice,
            long listPrice,
            long shareDiscount,
            String imageObjectKey,
            UUID productId,
            String productName
    ) {
        /**
         * catalog 明細：{@code skuId} 必填，這裡拒絕 null（UUID 是物件型別，編譯器不再擋，改在工廠方法擋）。
         * ⛔ catalog 的呼叫端不得改走 canonical constructor——那等於自己拆掉這道守門。
         */
        public static CreateOrderItemParam catalog(
                UUID skuId, String skuName, int quantity, long unitPrice, long listPrice,
                long shareDiscount, String imageObjectKey, UUID productId, String productName) {
            java.util.Objects.requireNonNull(skuId, "catalog 明細的 skuId 不可為空");
            return new CreateOrderItemParam(skuId, skuName, quantity, unitPrice, listPrice,
                    shareDiscount, imageObjectKey, productId, productName);
        }
    }

    /**
     * 結帳模組建單參數（含收件人/配送區域/優惠券等）
     */
    public record CreateOrderParams(
            long buyerId,
            long storeId,
            List<CreateOrderItemParam> items,
            long totalOriginalAmount,
            long totalActualAmount,
            String checkoutId,
            String deliveryTemp,
            String shippingMethod,
            long shippingFee,
            String shippingFeePayer,
            String recipientName,
            String recipientPhone,
            String shippingAddress,
            String shippingNote,
            String orderRemark,
            String cvsStoreId,
            String cvsStoreName,
            String shippingArea,
            String senderCity,
            String senderDistrict,
            int dimensionSum,
            String merId,
            long discountAmount,
            /**
             * 買家發票選擇快照（58 號 §5.4.3）。恆非 null——{@link io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver}
             * 對「未帶 invoice 區塊」回 {@code defaultSnapshot()}，不回 null。
             */
            io.github.will413028.hawkr.order.checkout.InvoiceIntakeResolver.Snapshot invoiceSnapshot,
            /**
             * 訂單政策快照五欄（65）。<b>恆非 null</b>——preview 決定、confirm 讀回，
             * 缺欄位在 {@code CheckoutService.extractPolicySnapshot} 就已 fail-closed。
             */
            io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot policySnapshot
    ) {}

    private record PayloadItem(UUID skuId, int quantity) {}
}
