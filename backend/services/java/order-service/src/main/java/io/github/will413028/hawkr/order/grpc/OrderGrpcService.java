package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.grpc.DelegationCapability;
import io.github.will413028.hawkr.common.grpc.DelegationContext;
import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.grpc.GrpcServiceHelper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.ProviderSource;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.entity.*;
import io.github.will413028.hawkr.order.exception.CheckoutExpiredException;
import io.github.will413028.hawkr.order.exception.CheckoutPriceChangedException;
import io.github.will413028.hawkr.common.utils.code.CartItemUnavailableReason;
import io.github.will413028.hawkr.order.exception.InsufficientStockException;
import io.github.will413028.hawkr.order.exception.UnavailableItemsException;
import io.github.will413028.hawkr.order.exception.InvalidStatusException;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.exception.OrderNotFoundException;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.payment.grpc.GetPaymentResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.order.grpc.converter.OrderProtoConverter;
import io.github.will413028.hawkr.order.grpc.converter.PaymentMethodAvailabilityConverter;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.order.util.MaskUtils;
import io.github.will413028.hawkr.order.grpc.*;
import io.github.will413028.hawkr.shipping.grpc.GetShipmentResponse;
import io.github.will413028.hawkr.shipping.grpc.ShipmentDetailView;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import org.slf4j.MDC;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import org.springframework.dao.DuplicateKeyException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import io.github.will413028.hawkr.order.grpc.OrderServiceGrpc;

/**
 * Order Service gRPC 實作
 *
 * 統一買家/賣家/管理端 API
 */
@GrpcService
public class OrderGrpcService extends OrderServiceGrpc.OrderServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(OrderGrpcService.class);

    private final OrderService orderService;
    private final CheckoutService checkoutService;
    private final CheckoutCouponGrpcClient checkoutCouponGrpcClient;
    private final OrderRepository orderRepository;
    private final OrderItemRepository orderItemRepository;
    private final ShippingGrpcClient shippingGrpcClient;
    private final PaymentGrpcClient paymentGrpcClient;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;
    private final io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient mediaGrpcClient;
    private final MediaUrlProperties mediaUrlProperties;
    private final io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository grantPaymentTaskRepository;
    private final io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository;
    private final OrderEventPublisher eventPublisher;

    /**
     * 代理商 PayUni MerID（和單代收，20 號 §4.13）。空字串 ＝ 本環境未設定 ⇒ 沒有任何訂單是和單。
     * {@code orders.mer_id} 等於它就是和單——那是 order-service 在讀取端唯一拿得到的訊號，
     * 快照表刻意不存這個布林（它是「付款收款方是誰」的衍生值，不是買家的發票選擇）。
     */
    private final String agentMerId;

    public OrderGrpcService(OrderService orderService, CheckoutService checkoutService,
                            CheckoutCouponGrpcClient checkoutCouponGrpcClient,
                            OrderRepository orderRepository, OrderItemRepository orderItemRepository,
                            ShippingGrpcClient shippingGrpcClient, PaymentGrpcClient paymentGrpcClient,
                            SellerGrpcClient sellerGrpcClient, BuyerGrpcClient buyerGrpcClient,
                            io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient mediaGrpcClient,
                            MediaUrlProperties mediaUrlProperties,
                            io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository grantPaymentTaskRepository,
                            io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository orderInvoiceSnapshotRepository,
                            OrderEventPublisher eventPublisher,
                            @org.springframework.beans.factory.annotation.Value("${payuni.agent.mer-id:}") String agentMerId) {
        this.orderService = orderService;
        this.checkoutService = checkoutService;
        this.checkoutCouponGrpcClient = checkoutCouponGrpcClient;
        this.orderRepository = orderRepository;
        this.orderItemRepository = orderItemRepository;
        this.shippingGrpcClient = shippingGrpcClient;
        this.paymentGrpcClient = paymentGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.mediaGrpcClient = mediaGrpcClient;
        this.mediaUrlProperties = mediaUrlProperties;
        this.grantPaymentTaskRepository = grantPaymentTaskRepository;
        this.orderInvoiceSnapshotRepository = orderInvoiceSnapshotRepository;
        this.eventPublisher = eventPublisher;
        this.agentMerId = agentMerId;
    }

    /**
     * 聚合物流資訊（graceful fallback：失敗時回傳 null，使用 Order 快照）
     */
    /**
     * 批次聚合 shipment（賣家列表頁專用，graceful fallback：失敗時回空 Map）
     */
    private Map<String, ShipmentDetailView> fetchShipmentsBatchQuietly(List<String> orderNos) {
        if (orderNos == null || orderNos.isEmpty()) {
            return Collections.emptyMap();
        }
        try {
            io.github.will413028.hawkr.shipping.grpc.GetShipmentsByOrderNosResponse resp =
                    shippingGrpcClient.getShipmentsByOrderNos(orderNos);
            // shipping-service 失敗時會拋 StatusRuntimeException，由下方 catch 降級為空 Map
            Map<String, ShipmentDetailView> map = new java.util.HashMap<>();
            for (ShipmentDetailView v : resp.getDataList()) {
                map.put(v.getOrderNo(), v);
            }
            return map;
        } catch (Exception e) {
            log.warn("OrderGrpcService.fetchShipmentsBatch.fallback",
                    "批次物流聚合失敗，使用 Order 快照", "count", String.valueOf(orderNos.size()));
            return Collections.emptyMap();
        }
    }

    private ShipmentDetailView fetchShipmentQuietly(String orderNo) {
        try {
            GetShipmentResponse resp = shippingGrpcClient.getShipmentByOrderNo(orderNo);
            if (resp.getCode() == 0 && resp.hasData()) {
                return resp.getData();
            }
        } catch (Exception e) {
            log.warn("OrderGrpcService.fetchShipment.fallback",
                    "物流資訊聚合失敗，使用 Order 快照", "order_no", orderNo);
        }
        return null;
    }


    /**
     * 聚合買家資訊（graceful fallback：失敗時回傳空 Map）。
     * order.buyer_id 是 PK（下單當下的買家身分），不是 account_id，故走
     * batchGetBuyerDisplayByBuyerIds（PK 查詢），見 2026-07-26 事故。
     */
    private Map<Long, BuyerDisplay> fetchBuyerDisplayQuietly(List<Long> buyerIds) {
        if (buyerIds == null || buyerIds.isEmpty()) {
            return Collections.emptyMap();
        }
        try {
            return buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(buyerIds);
        } catch (Exception e) {
            log.warn("OrderGrpcService.fetchBuyerDisplay.fallback",
                    "買家資訊聚合失敗", "count", String.valueOf(buyerIds.size()));
            return Collections.emptyMap();
        }
    }

    /**
     * 聚合賣家資訊（graceful fallback：失敗時回傳空 Map）
     */
    private Map<Long, SellerMeta> fetchSellerMetaQuietly(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Collections.emptyMap();
        }
        try {
            return sellerGrpcClient.batchGetSellerMeta(storeIds);
        } catch (Exception e) {
            log.warn("OrderGrpcService.fetchSellerMeta.fallback",
                    "賣家資訊聚合失敗", "count", String.valueOf(storeIds.size()));
            return Collections.emptyMap();
        }
    }

    // ==================== Provider 狀態聚合（收斂規範 §6.2 / §6.3）====================

    /** Pair of provider status + message（避免多處重建） */
    private record ProviderPair(String status, String message) {
        static final ProviderPair EMPTY = new ProviderPair("", "");
    }

    // §29 Provider 取源分組（收斂規範 §6.2）：依 order.status 決定向哪個 SoT 服務查。
    // 使用 OrderStatus enum value 避免 magic string；switch-case 無法用 method call 故抽至 Set。
    private static final java.util.Set<String> STATUS_PAYMENT_SOURCE = java.util.Set.of(
            io.github.will413028.hawkr.common.utils.code.OrderStatus.PENDING_PAYMENT.value()
    );
    private static final java.util.Set<String> STATUS_REFUND_SOURCE = java.util.Set.of(
            io.github.will413028.hawkr.common.utils.code.OrderStatus.PARTIALLY_REFUNDED.value(),
            io.github.will413028.hawkr.common.utils.code.OrderStatus.FULLY_REFUNDED.value()
    );
    private static final java.util.Set<String> STATUS_SHIPPING_SOURCE = java.util.Set.of(
            io.github.will413028.hawkr.common.utils.code.OrderStatus.PENDING_SHIPMENT.value(),
            io.github.will413028.hawkr.common.utils.code.OrderStatus.IN_TRANSIT.value(),
            io.github.will413028.hawkr.common.utils.code.OrderStatus.WAITING_PICKUP.value(),
            io.github.will413028.hawkr.common.utils.code.OrderStatus.SHIPPING_ABNORMAL.value(),
            io.github.will413028.hawkr.common.utils.code.OrderStatus.DELIVERED.value()
    );

    /**
     * 批次聚合每個 order 的 provider_status / provider_message：
     * 依 order.status 分組決定 SoT 服務（PAYMENT / REFUND / SHIPPING / RETURN_SHIPPING），
     * 對每組各發一次 batch RPC。部分失敗容忍（收斂規範 §6.3）。
     */
    private Map<String, ProviderPair> resolveProviderStatusMap(List<OrderEntity> orders) {
        Map<String, ProviderPair> out = new java.util.HashMap<>();
        if (orders == null || orders.isEmpty()) return out;

        List<String> paymentOrderNos = new java.util.ArrayList<>();
        List<String> refundOrderNos = new java.util.ArrayList<>();
        List<String> shippingOrderNos = new java.util.ArrayList<>();
        List<String> returnShippingOrderNos = new java.util.ArrayList<>();

        for (OrderEntity o : orders) {
            String s = o.getStatus();
            if (s == null) continue;
            if (STATUS_PAYMENT_SOURCE.contains(s)) {
                paymentOrderNos.add(o.getOrderNo());
            } else if (STATUS_REFUND_SOURCE.contains(s)) {
                refundOrderNos.add(o.getOrderNo());
            } else if (STATUS_SHIPPING_SOURCE.contains(s)) {
                shippingOrderNos.add(o.getOrderNo());
            }
            // 其他狀態不聚合
        }

        mergePaymentProvider(paymentOrderNos, out);
        mergeRefundProvider(refundOrderNos, out);
        mergeShippingProvider(shippingOrderNos, false, out);
        mergeShippingProvider(returnShippingOrderNos, true, out);
        return out;
    }

    private void mergePaymentProvider(List<String> orderNos, Map<String, ProviderPair> out) {
        if (orderNos.isEmpty()) return;
        try {
            var resp = paymentGrpcClient.batchGetProviderStatusByOrderIds(orderNos);
            resp.getEntriesMap().forEach((k, v) ->
                    out.put(k, new ProviderPair(v.getProviderStatus(), v.getProviderMessage())));
        } catch (Exception e) {
            log.warn("OrderGrpcService.providerStatus.payment.fallback",
                    "Payment provider 聚合失敗: " + e.getMessage());
        }
    }

    private void mergeRefundProvider(List<String> orderNos, Map<String, ProviderPair> out) {
        if (orderNos.isEmpty()) return;
        try {
            var resp = paymentGrpcClient.batchGetRefundProviderStatusByOrderIds(orderNos);
            resp.getEntriesMap().forEach((k, v) ->
                    out.put(k, new ProviderPair(v.getProviderStatus(), v.getProviderMessage())));
        } catch (Exception e) {
            log.warn("OrderGrpcService.providerStatus.refund.fallback",
                    "Refund provider 聚合失敗: " + e.getMessage());
        }
    }

    private void mergeShippingProvider(List<String> orderNos, boolean isReturn, Map<String, ProviderPair> out) {
        if (orderNos.isEmpty()) return;
        try {
            var resp = shippingGrpcClient.batchGetProviderStatusByOrderIds(orderNos, isReturn);
            resp.getEntriesMap().forEach((k, v) ->
                    out.put(k, new ProviderPair(v.getProviderStatus(), v.getProviderMessage())));
        } catch (Exception e) {
            log.warn("OrderGrpcService.providerStatus.shipping.fallback",
                    "Shipping provider 聚合失敗 isReturn=" + isReturn + " err=" + e.getMessage());
        }
    }

    /**
     * 取單筆 order 的 provider 狀態（給 detail endpoint 用，不走 batch）。
     */
    private ProviderPair resolveProviderStatusFor(OrderEntity order) {
        Map<String, ProviderPair> map = resolveProviderStatusMap(List.of(order));
        return map.getOrDefault(order.getOrderNo(), ProviderPair.EMPTY);
    }

    /** 將 ProviderPair 寫入既有的 OrderDetailView（避免破壞 converter signature）。 */
    private static OrderDetailView enrichProvider(OrderDetailView base, ProviderPair pair) {
        if (pair == null || (pair.status().isEmpty() && pair.message().isEmpty())) {
            return base;
        }
        return base.toBuilder()
                .setProviderStatus(pair.status())
                .setProviderMessage(pair.message())
                .build();
    }

    /**
     * 再次付款 surface：依訂單金額填 availablePaymentMethods（§3.4.1 / 03 §4.2.1）。
     * <p>
     * **僅買家 getOrder 路徑呼叫**，內部以 status==PENDING_PAYMENT 守門：
     * 非 pending（已付/已取消）不能再 CreatePayment，回付款方式為噪音，故回原 base。
     * 賣家/管理員/物流 caller 不呼叫本方法，OrderDetailView.available_payment_methods 保持空，
     * 避免污染共用 {@link OrderProtoConverter#toDetailView}。
     */
    private static OrderDetailView enrichAvailablePaymentMethods(OrderDetailView base, OrderEntity order) {
        if (!io.github.will413028.hawkr.common.utils.code.OrderStatus.PENDING_PAYMENT.value().equals(order.getStatus())) {
            return base;
        }
        long amount = order.getTotalActualAmount() == null ? 0L : order.getTotalActualAmount();
        return base.toBuilder()
                .addAllAvailablePaymentMethods(PaymentMethodAvailabilityConverter.resolve(amount))
                .build();
    }

    /** 將 ProviderPair 寫入既有的 OrderSummaryView。 */
    private static OrderSummaryView enrichProvider(OrderSummaryView base, ProviderPair pair) {
        if (pair == null || (pair.status().isEmpty() && pair.message().isEmpty())) {
            return base;
        }
        return base.toBuilder()
                .setProviderStatus(pair.status())
                .setProviderMessage(pair.message())
                .build();
    }

    /**
     * 取得 CDN base URL（供 Converter 組裝圖片 URL）
     */
    private String getCdnBaseUrl() {
        return mediaUrlProperties.getBaseUrl();
    }

    // ==================== 買家端 ====================
    @Override
    public void createOrder(CreateOrderRequest request, StreamObserver<CreateOrderResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可建立訂單
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            List<OrderService.CreateOrderItemParam> items = request.getItemsList().stream()
                    .map(i -> {
                        long unitPrice = ProtoConversionUtils.parseLongRequired(i.getUnitPrice(), "unitPrice");
                        // listPrice 過渡期：proto 端若為空字串，視為「無特價」，listPrice = unitPrice
                        long listPrice = i.getListPrice().isEmpty()
                                ? unitPrice
                                : ProtoConversionUtils.parseLongRequired(i.getListPrice(), "listPrice");
                        // catalog 建單：走 CreateOrderItemParam.catalog（primitive long）
                        // ⇒ sku_id 缺值仍是編譯期＋parse 期雙重擋下，不因 RFQ 放寬而鬆掉。
                        return OrderService.CreateOrderItemParam.catalog(
                            ProtoConversionUtils.parseLongRequired(i.getSkuId(), "skuId"),
                            i.getSkuName(),
                            ProtoConversionUtils.parseIntRequired(i.getQuantity(), "quantity"),
                            unitPrice,
                            listPrice,
                            0L,  // CreateOrder 走購物車外建單路徑：share_discount=0（無優惠券分攤）
                            "",
                            ProtoConversionUtils.parseLongOrNull(i.getProductId()),
                            i.getProductName()
                        );
                    })
                    .toList();

            String orderNo = orderService.createOrder(
                    buyerId,
                    ProtoConversionUtils.parseLongRequired(request.getStoreId(), "storeId"),
                    items,
                    ProtoConversionUtils.parseLongRequired(request.getTotalOriginalAmount(), "totalOriginalAmount"),
                    ProtoConversionUtils.parseLongRequired(request.getTotalActualAmount(), "totalActualAmount"),
                    request.getCheckoutId(),
                    request.getDeliveryTemp().isEmpty() ? null : request.getDeliveryTemp(),
                    request.getShippingMethod().isEmpty() ? null : request.getShippingMethod(),
                    ProtoConversionUtils.parseLongOrDefault(request.getShippingFee(), 0L),
                    request.getShippingFeePayer().isEmpty() ? null : request.getShippingFeePayer()
            );

            responseObserver.onNext(CreateOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("建立成功")
                    .setData(CreateOrderData.newBuilder().setOrderNo(orderNo).build())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CreateOrderResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (InsufficientStockException e) {
            responseObserver.onNext(CreateOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.INSUFFICIENT_STOCK.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateOrderResponse.newBuilder(), e, "OrderGrpcService", "createOrder", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * RFQ 成交建單（64 號 §6.4）——兩條 lane 都走這裡，差別只在賣方身分與政策五欄。
     *
     * <p><b>internal-only</b>：呼叫端是 ai-service 的 outbox worker，沒有使用者情境。
     * 簽章由 server interceptor 驗（52 號 HMAC 四 header），這裡只讀判定結果。
     * ⛔ 不得用 {@code requireAccountType(BUYER)}——那條路徑需要一個買家 JWT，
     * 而 worker 手上沒有、也不該有。
     *
     * <p>冪等：同一個 {@code acceptanceId} 重送回既有 {@code orderNo} ＋
     * {@code replayed: true}，不建第二張。
     */
    @Override
    public void createRfqOrder(CreateRfqOrderRequest request,
                               StreamObserver<CreateRfqOrderResponse> responseObserver) {
        try {
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("CreateRfqOrder 僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            OrderService.RfqOrderResult result = orderService.createRfqOrder(
                    request.getBuyerId(),
                    ProtoConversionUtils.parseLongRequired(request.getStoreId(), "storeId"),
                    ProtoConversionUtils.parseLongRequired(request.getAcceptanceId(), "acceptanceId"),
                    ProtoConversionUtils.parseLongRequired(request.getTotalAmount(), "totalAmount"),
                    ProtoConversionUtils.parseLongOrDefault(request.getShippingFee(), 0L),
                    // RFQ 明細是 non-catalog line（64 號 §6.4.9）：wire 上 sku_id 缺值就是
                    // proto3 string 的 ""，映成 null。
                    // ⛔ 不用 parseLongOrNull——它把格式不合法也吞成 null，那會讓「沒有 SKU」
                    //    與「解析失敗」共用同一個值，而後者是 bug（§6.4.15 對哨兵的同一條立場）。
                    //    慣例取自同一支呼叫裡的 deliveryTemp。
                    request.getSkuId().isEmpty()
                            ? null
                            : ProtoConversionUtils.parseLongRequired(request.getSkuId(), "skuId"),
                    request.getSkuName(),
                    ProtoConversionUtils.parseLongRequired(request.getUnitPrice(), "unitPrice"),
                    request.getDeliveryTemp().isEmpty() ? null : request.getDeliveryTemp(),
                    request.getLane());

            CreateRfqOrderData.Builder data = CreateRfqOrderData.newBuilder()
                    .setOrderNo(result.orderNo())
                    .setReplayed(result.replayed());
            if (result.paymentDeadline() != null) {
                data.setPaymentDeadline(DateTimeUtils.formatIso8601(result.paymentDeadline()));
            }
            responseObserver.onNext(CreateRfqOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("建立成功")
                    .setData(data.build())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CreateRfqOrderResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateRfqOrderResponse.newBuilder(), e,
                    "OrderGrpcService", "createRfqOrder", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void cancelOrder(CancelOrderRequest request, StreamObserver<CancelOrderResponse> responseObserver) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml），不只有明確帶 bzCol 的那幾行。
        MDC.put("order_no", request.getOrderNo());
        try {
            // 授權：僅 BUYER 可取消訂單
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            // #1368：取消狀態變更 + 庫存歸還 order.cancelled outbox 同事務落地（cancelOrder 內），
            // 由 outbox scheduler 保證至少一次投遞，取代原 TX 外 best-effort 直發
            orderService.cancelOrder(buyerId, request.getOrderNo());
            // 關閉付款交易（best-effort）
            closePaymentForOrder(request.getOrderNo());
            // 購物車已在建單時清理（建單即清策略），取消訂單無需解鎖
            // 釋放優惠券（軟依賴）
            releaseCouponForOrder(request.getOrderNo());

            responseObserver.onNext(CancelOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("取消成功")
                    .setSuccess(true)
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CancelOrderResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(CancelOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (InvalidStatusException e) {
            responseObserver.onNext(CancelOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_STATUS.code())
                    .setMessage(e.getMessage())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CancelOrderResponse.newBuilder(), e, "OrderGrpcService", "cancelOrder", GrpcResponseCode.INTERNAL_ERROR);
        } finally {
            MDC.remove("order_no");
        }
    }

    @Override
    public void getMyOrders(GetMyOrdersRequest request, StreamObserver<GetMyOrdersResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可查詢自己的訂單
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            int page = ProtoConversionUtils.parseIntRequired(request.getPage().getPage(), "page");
            int size = ProtoConversionUtils.parseIntRequired(request.getPage().getSize(), "size");

            List<OrderEntity> orders = orderService.getMyOrders(buyerId, request.getStatusFilter(), page, size);
            long total = orderService.countMyOrders(buyerId, request.getStatusFilter());

            // 聚合賣家資訊
            List<Long> sellerIds = orders.stream().map(OrderEntity::getStoreId).distinct().toList();
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(sellerIds);
            String cdnBaseUrl = getCdnBaseUrl();

            Map<String, ProviderPair> providerMap = resolveProviderStatusMap(orders);
            List<OrderSummaryView> summaries = orders.stream()
                    .map(o -> {
                        OrderSummaryView base = OrderProtoConverter.toSummaryView(
                                o, orderService.getOrderItems(o.getOrderNo()),
                                sellerMetaMap.get(o.getStoreId()), null, cdnBaseUrl, false);
                        return enrichProvider(base, providerMap.get(o.getOrderNo()));
                    })
                    .toList();

            responseObserver.onNext(GetMyOrdersResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .addAllOrders(summaries)
                    .setMeta(OrderProtoConverter.toPageMeta(page, size, total))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetMyOrdersResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetMyOrdersResponse.newBuilder(), e, "OrderGrpcService", "getMyOrders", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getMyOrderStatusCounts(GetMyOrderStatusCountsRequest request, StreamObserver<GetMyOrderStatusCountsResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可查詢自己的訂單筆數
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            List<OrderStatusCountResult> counts = orderService.getMyOrderStatusCounts(buyerId);

            GetMyOrderStatusCountsResponse.Builder builder = GetMyOrderStatusCountsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            long total = 0;
            for (OrderStatusCountResult c : counts) {
                builder.addCounts(OrderStatusCount.newBuilder()
                        .setStatus(c.getStatus())
                        .setCount(Long.toString(c.getCount()))
                        .build());
                total += c.getCount();
            }
            builder.setTotal(Long.toString(total));

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetMyOrderStatusCountsResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetMyOrderStatusCountsResponse.newBuilder(), e, "OrderGrpcService", "getMyOrderStatusCounts", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getOrder(GetOrderRequest request, StreamObserver<GetOrderResponse> responseObserver) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            // 授權：僅 BUYER 可查詢訂單詳情
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            OrderEntity order = orderService.getOrderByNo(request.getOrderNo());
            // Ownership 檢查：訂單必須屬於該買家
            if (order.getBuyerId() != buyerId) {
                // 越權防枚舉（API 設計規範 §14.1）：回應與「訂單不存在」完全一致，不洩漏訂單存在
                throw new OrderNotFoundException(request.getOrderNo());
            }

            List<OrderItemEntity> items = orderService.getOrderItems(request.getOrderNo());
            List<RefundRequestEntity> refunds = orderService.getRefundHistory(request.getOrderNo());
            ShipmentDetailView shipment = fetchShipmentQuietly(order.getOrderNo());

            // 聚合賣家資訊
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(List.of(order.getStoreId()));
            SellerMeta sellerMeta = sellerMetaMap.get(order.getStoreId());
            String cdnBaseUrl = getCdnBaseUrl();

            OrderDetailView baseDetail = OrderProtoConverter.toDetailView(
                    order, items, refunds, shipment, sellerMeta, null, cdnBaseUrl, false);
            // 買家路徑：先 enrichProvider，再 enrichAvailablePaymentMethods（僅 pending 填，再次付款 surface §4.2.1）
            OrderDetailView detail = enrichAvailablePaymentMethods(
                    enrichProvider(baseDetail, resolveProviderStatusFor(order)), order);

            responseObserver.onNext(GetOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detail)
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetOrderResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(GetOrderResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderResponse.newBuilder(), e, "OrderGrpcService", "getOrder", GrpcResponseCode.INTERNAL_ERROR);
        } finally {
            MDC.remove("order_no");
        }
    }

    @Override
    public void confirmReceipt(ConfirmReceiptRequest request, StreamObserver<ConfirmReceiptResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可確認收貨
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            // 階段 0：若有 stagedTokens 則先 finalize，失敗整個收貨失敗（嚴格模式，安全為主）
            java.util.List<String> stagedTokens = request.getStagedTokensList();
            java.util.List<String> assetIds = new java.util.ArrayList<>();
            String receiptPhotoKeysJson = null;
            if (!stagedTokens.isEmpty()) {
                java.util.List<io.github.will413028.hawkr.media.grpc.FinalizedAsset> assets;
                try {
                    assets = mediaGrpcClient.finalizeImages(stagedTokens,
                            io.github.will413028.hawkr.common.utils.code.AssetReferenceType.ORDER_RECEIPT.value());
                } catch (Exception fe) {
                    log.error("OrderGrpcService.confirmReceipt.finalize.error",
                            "收貨照片轉正失敗 orderNo=" + request.getOrderNo()
                                    + " err=" + fe.getClass().getSimpleName() + ":" + fe.getMessage(), fe);
                    responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                            .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                            .setMessage("收貨照片上傳失敗，請稍後再試")
                            .setSuccess(false)
                            .build());
                    responseObserver.onCompleted();
                    return;
                }
                // 存 objectKey（49 §5.3/§6.2）；顯示 URL 由 Gateway 組
                java.util.List<String> keys = new java.util.ArrayList<>();
                for (io.github.will413028.hawkr.media.grpc.FinalizedAsset a : assets) {
                    assetIds.add(a.getAssetId());
                    keys.add(a.getObjectKey());
                }
                try {
                    receiptPhotoKeysJson = new com.fasterxml.jackson.databind.ObjectMapper()
                            .writeValueAsString(keys);
                } catch (Exception je) {
                    receiptPhotoKeysJson = null;
                }
            }

            // 階段 1：linkImages 必須在把 objectKey 寫進 orders 之前成功（#2222）。
            // 舊版排在確認收貨之後且吞掉錯誤：orders.receipt_photo_keys 指著這些資產、
            // 卻沒有 asset_reference，7 天後被 media GC 連同 R2 物件實體刪除，爭議時無佐證可查。
            // 失敗即回錯（與階段 0 的嚴格模式一致）：訂單未確認收貨，剛轉正的照片無人引用，
            // 由 media GC 依孤兒規則回收，買家重試即可。
            if (!assetIds.isEmpty()) {
                try {
                    mediaGrpcClient.linkImages(
                            io.github.will413028.hawkr.common.utils.code.AssetReferenceType.ORDER_RECEIPT.value(),
                            request.getOrderNo(), assetIds);
                } catch (Exception le) {
                    log.error("OrderGrpcService.confirmReceipt.link.error",
                            "收貨照片引用建立失敗 orderNo=" + request.getOrderNo()
                                    + " err=" + le.getClass().getSimpleName() + ":" + le.getMessage(), le);
                    responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                            .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                            .setMessage("收貨照片上傳失敗，請稍後再試")
                            .setSuccess(false)
                            .build());
                    responseObserver.onCompleted();
                    return;
                }
            }

            // 階段 2：確認收貨（內含買家身分驗證 + 狀態檢查 + 完成 + MQ 事件發布）
            orderService.confirmReceipt(buyerId, request.getOrderNo(), receiptPhotoKeysJson);

            responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("確認收貨成功")
                    .setSuccess(true)
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (InvalidStatusException e) {
            responseObserver.onNext(ConfirmReceiptResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_STATUS.code())
                    .setMessage(e.getMessage())
                    .setSuccess(false)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ConfirmReceiptResponse.newBuilder(), e, "OrderGrpcService", "confirmReceipt", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 賣家端 ====================
    @Override
    public void getSellerOrders(GetSellerOrdersRequest request, StreamObserver<GetSellerOrdersResponse> responseObserver) {
        try {
            // 授權：賣家本人或代行（orders.read）。訂單面只有 orders.read，寫入全在禁區。
            DelegationContext delegation = delegationOrNull(request.hasDelegation(), request.getDelegation());
            GrpcAuthContext.requireSellerOrDelegated(delegation, request.getStoreId(),
                    DelegationCapability.ORDERS_READ);
            long storeId = request.getStoreId();

            int page = ProtoConversionUtils.parseIntRequired(request.getPage().getPage(), "page");
            int size = ProtoConversionUtils.parseIntRequired(request.getPage().getSize(), "size");

            // 解析可選的買家篩選條件
            long buyerId = ProtoConversionUtils.parseLongOrDefault(request.getBuyerId(), 0L);

            // 列表與筆數共用同一個 query 實例，條件才不會分岔（total 對不上 records 不會報錯）
            SellerOrderQuery query = orderService.sellerOrderQuery(storeId, buyerId,
                    request.getStatusFilter(), request.getKeyword(),
                    request.getDateFrom(), request.getDateTo());
            List<OrderEntity> orders = orderService.getSellerOrders(query, page, size);
            long total = orderService.countSellerOrders(query);

            // 聚合賣家資訊（賣家端：賣家查自己的訂單，只有一個 storeId）
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(List.of(storeId));
            // 聚合買家資訊（賣家視角需要顯示買家暱稱+頭像）
            List<Long> buyerIds = orders.stream().map(OrderEntity::getBuyerId).distinct().toList();
            Map<Long, BuyerDisplay> buyerDisplayMap = fetchBuyerDisplayQuietly(buyerIds);
            String cdnBaseUrl = getCdnBaseUrl();

            // 賣家列表頁：批次取得即時 shipment（含最新出貨照片，避免賣家剛上傳的快照延遲）
            Map<String, ShipmentDetailView> shipmentMap = fetchShipmentsBatchQuietly(
                    orders.stream().map(OrderEntity::getOrderNo).toList());

            // 收斂規範 §6.3：批次解析 providerStatus / providerMessage
            Map<String, ProviderPair> providerMap = resolveProviderStatusMap(orders);

            List<OrderSummaryView> summaries = orders.stream()
                    .map(o -> {
                        OrderSummaryView base = OrderProtoConverter.toSummaryView(o,
                                orderService.getOrderItems(o.getOrderNo()),
                                sellerMetaMap.get(o.getStoreId()),
                                buyerDisplayMap.get(o.getBuyerId()), cdnBaseUrl, true,
                                shipmentMap.get(o.getOrderNo()));
                        return enrichProvider(base, providerMap.get(o.getOrderNo()));
                    })
                    // 代行讀取：遮罩 ＋ 履約／金流欄位淨空（列表永遠不揭露，本 RPC 也不吐聯絡欄位）
                    .map(s -> delegation != null ? applyDelegatedProjection(s) : s)
                    .toList();

            responseObserver.onNext(GetSellerOrdersResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .addAllOrders(summaries)
                    .setMeta(OrderProtoConverter.toPageMeta(page, size, total))
                    .build());
            responseObserver.onCompleted();
        } catch (DelegationDeniedException e) {
            log.warn("OrderGrpcService.getSellerOrders.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(GetSellerOrdersResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetSellerOrdersResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerOrdersResponse.newBuilder(), e, "OrderGrpcService", "getSellerOrders", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getSellerOrderDetail(GetSellerOrderDetailRequest request, StreamObserver<GetSellerOrderDetailResponse> responseObserver) {
        try {
            // 授權：賣家本人或代行（orders.read）
            DelegationContext delegation = delegationOrNull(request.hasDelegation(), request.getDelegation());
            GrpcAuthContext.requireSellerOrDelegated(delegation, request.getStoreId(),
                    DelegationCapability.ORDERS_READ);

            // PII 揭露的補充斷言（全案唯一一處呼叫端補的判定）：orders.read 落在 S1b 的「不驗」側，
            // 而揭露完整收件資訊是必落稽核的，Validate 又看不到 reveal_recipient_contact ⇒ 只能在這裡補。
            boolean reveal = request.getRevealRecipientContact();
            if (reveal) {
                if (delegation == null) {
                    responseObserver.onNext(GetSellerOrderDetailResponse.newBuilder()
                            .setCode(GrpcResponseCode.INVALID_PARAM.code())
                            .setMessage("reveal_recipient_contact 僅適用於代行讀取")
                            .build());
                    responseObserver.onCompleted();
                    return;
                }
                if (delegation.getAuditLogId().isEmpty()) {
                    // 9289 而不是 9260（#2744）：audit_log_id 由 Gateway 的 orchestrator 在
                    // 呼叫本 RPC 之前寫入稽核意圖時填上，操作者碰不到它。缺了它代表 Gateway
                    // 沒走完自己的兩階段流程 —— 與 DelegationContextValidator 的 S1b 同一個
                    // 主體，重試與重填表單都無效。
                    throw new DelegationDeniedException(
                            GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH,
                            "PII 揭露必須帶稽核列 ID");
                }
            }
            long storeId = request.getStoreId();

            OrderEntity order = orderService.getOrderByNo(request.getOrderNo());
            // Ownership 檢查：訂單必須屬於該賣家
            if (order.getStoreId() != storeId) {
                // 越權防枚舉（API 設計規範 §14.1）：回應與「訂單不存在」完全一致，不洩漏訂單存在
                throw new OrderNotFoundException(request.getOrderNo());
            }

            List<OrderItemEntity> items = orderService.getOrderItems(request.getOrderNo());
            List<RefundRequestEntity> refunds = orderService.getRefundHistory(request.getOrderNo());
            ShipmentDetailView shipment = fetchShipmentQuietly(order.getOrderNo());

            // 聚合賣家資訊
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(List.of(order.getStoreId()));
            SellerMeta sellerMeta = sellerMetaMap.get(order.getStoreId());
            // 聚合買家資訊（賣家視角需要顯示買家暱稱+頭像）
            Map<Long, BuyerDisplay> buyerDisplayMap = fetchBuyerDisplayQuietly(List.of(order.getBuyerId()));
            BuyerDisplay buyerDisplay = buyerDisplayMap.get(order.getBuyerId());
            String cdnBaseUrl = getCdnBaseUrl();

            OrderDetailView baseDetail = OrderProtoConverter.toDetailView(
                    order, items, refunds, shipment, sellerMeta, buyerDisplay, cdnBaseUrl, true);
            // 收斂規範 §6.3：詳情端單筆聚合 provider 狀態
            OrderDetailView detail = enrichProvider(baseDetail, resolveProviderStatusFor(order));
            if (delegation != null) {
                detail = applyDelegatedProjection(detail, reveal);
            }

            responseObserver.onNext(GetSellerOrderDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detail)
                    .build());
            responseObserver.onCompleted();
        } catch (DelegationDeniedException e) {
            log.warn("OrderGrpcService.getSellerOrderDetail.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(GetSellerOrderDetailResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetSellerOrderDetailResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(GetSellerOrderDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerOrderDetailResponse.newBuilder(), e, "OrderGrpcService", "getSellerOrderDetail", GrpcResponseCode.INTERNAL_ERROR);
        }
    }


    // ==================== 代行讀取投影（56 §8.1.5 / §8.1.0） ====================

    /** proto3 message 欄位天生有 presence：未帶 = 賣家本人操作，帶了 = 代行操作。 */
    private static DelegationContext delegationOrNull(boolean present, DelegationContext delegation) {
        return present ? delegation : null;
    }

    /**
     * 列表項的代行投影：地址遮罩 ＋ 履約／金流欄位淨空。
     *
     * 履約與金流欄位的排除靠的是這裡，不是路由禁字斷言——那條斷言只掃<b>路徑</b>，
     * 漏掉這些欄位不會 panic、不會轉紅，只會靜默把託運單與金流資訊交給代行者。
     */
    private static OrderSummaryView applyDelegatedProjection(OrderSummaryView summary) {
        return summary.toBuilder()
                .setShippingAddress(nullToEmpty(MaskUtils.maskAddressToDistrict(summary.getShippingAddress())))
                // 履約物件
                .clearLabelKey()
                .clearLabelExpireAt()
                .clearPartnerId()
                .clearOdno()
                .clearValidationNo()
                .clearShipmentPhotoKeys()
                .clearPickupStoreName()
                // 金流與退款
                .clearProviderStatus()
                .clearProviderMessage()
                .clearNeedRefundPage()
                .build();
    }

    /**
     * 詳情的代行投影。
     *
     * @param reveal true 僅由 {@code recipient-contact} 端點傳入，且該路徑已斷言過 audit_log_id；
     *               false（含未設定）一律遮罩——proto3 bool 零值就是 false，所以任何忘記填、
     *               任何新的呼叫點，預設都落在安全的那一邊
     */
    private static OrderDetailView applyDelegatedProjection(OrderDetailView detail, boolean reveal) {
        OrderDetailView.Builder builder = detail.toBuilder()
                // 履約物件
                .clearShipmentNo()
                .clearLabelKey()
                .clearLabelExpireAt()
                .clearPartnerId()
                .clearOdno()
                .clearValidationNo()
                .clearShipmentPhotoKeys()
                .clearReceiptPhotoKeys()
                .clearPickupStoreName()
                .clearCvsStoreId()
                // 金流
                .clearTransactionId()
                .clearPaymentMerId()
                .clearAtmBankCode()
                .clearAtmVirtualAccount()
                .clearAtmExpireDate()
                .clearProviderStatus()
                .clearProviderMessage()
                .clearAvailablePaymentMethods()
                // 退款與撥款
                .clearRefundHistory()
                .clearNeedRefundPage();
        if (!reveal) {
            builder.setRecipientPhone(nullToEmpty(MaskUtils.maskPhone(detail.getRecipientPhone())))
                    .setShippingAddress(nullToEmpty(MaskUtils.maskAddressToDistrict(detail.getShippingAddress())));
        }
        return builder.build();
    }

    /** 空字串→null（proto 的 string 欄位沒有 null，空字串就是「未提供」）。 */
    private static String emptyToNull(String value) {
        return (value == null || value.isBlank()) ? null : value;
    }

    /**
     * 空字串→null，否則解析成 Long。
     *
     * <p>解析失敗一律拋 {@link NumberFormatException} 讓外層轉 500，<b>不要吞成 null</b>：
     * 吞掉的後果是「買家指定了某張常用發票資料，而系統靜默當成沒指定」。
     */
    private static Long emptyToNullLong(String value) {
        return (value == null || value.isBlank()) ? null : Long.valueOf(value.trim());
    }

    private static String nullToEmpty(String value) {
        return value != null ? value : "";
    }

    // ==================== 服務間呼叫 ====================

    @Override
    public void getOrderForShipping(GetOrderForShippingRequest request, StreamObserver<GetOrderForShippingResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();

            log.info("OrderGrpcService.getOrderForShipping.start", "取得物流建單資訊",
                    "order_no", orderNo);

            // 純內部讀：授權由 HMAC internal-auth + netpol 保證，request.store_id 已不再用於歸屬校驗（#1366）
            OrderEntity order = orderService.getOrderForShipping(orderNo);

            // 取第一筆明細的 skuName 作為 product_name
            OrderItemEntity firstItem = orderItemRepository.findFirstByOrderNo(orderNo);
            String productName = (firstItem != null) ? firstItem.getSkuName() : "";

            OrderShippingInfo info = OrderShippingInfo.newBuilder()
                    .setOrderNo(order.getOrderNo())
                    .setStatus(order.getStatus())
                    .setStoreId(String.valueOf(order.getStoreId()))
                    .setBuyerId(String.valueOf(order.getBuyerId()))
                    .setTotalActualAmount(String.valueOf(order.getTotalActualAmount()))
                    .setRecipientName(order.getRecipientName() != null ? order.getRecipientName() : "")
                    .setRecipientPhone(order.getRecipientPhone() != null ? order.getRecipientPhone() : "")
                    .setShippingAddress(order.getShippingAddress() != null ? order.getShippingAddress() : "")
                    .setCvsStoreId(order.getCvsStoreId() != null ? order.getCvsStoreId() : "")
                    .setCvsStoreName(order.getCvsStoreName() != null ? order.getCvsStoreName() : "")
                    .setShippingMethod(order.getShippingMethod() != null ? order.getShippingMethod() : "")
                    .setProductName(productName)
                    .setMerId(order.getMerId() != null ? order.getMerId() : "")
                    .setDeliveryTemp(order.getDeliveryTemp() != null ? order.getDeliveryTemp() : "")
                    .setDimensionSum(order.getDimensionSum() != null ? String.valueOf(order.getDimensionSum()) : "0")
                    .build();

            responseObserver.onNext(GetOrderForShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(info)
                    .build());
            responseObserver.onCompleted();

            log.info("OrderGrpcService.getOrderForShipping.done", "取得物流建單資訊完成",
                    "order_no", orderNo);
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(GetOrderForShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderForShippingResponse.newBuilder(), e, "OrderGrpcService", "getOrderForShipping", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getOrderByShipTradeNo(GetOrderByShipTradeNoRequest request, StreamObserver<GetOrderByShipTradeNoResponse> responseObserver) {
        try {
            OrderEntity order = orderService.getOrderByShipmentNo(request.getShipTradeNo());
            List<OrderItemEntity> items = orderService.getOrderItems(order.getOrderNo());
            List<RefundRequestEntity> refunds = orderService.getRefundHistory(order.getOrderNo());

            // 內部呼叫（Shipping→Order），不回查 Shipping 避免循環，不需賣家/買家資訊
            responseObserver.onNext(GetOrderByShipTradeNoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(OrderProtoConverter.toDetailView(order, items, refunds, null, null, null, getCdnBaseUrl(), false))
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(GetOrderByShipTradeNoResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderByShipTradeNoResponse.newBuilder(), e, "OrderGrpcService", "getOrderByShipTradeNo", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getOrderForPayment(GetOrderForPaymentRequest request, StreamObserver<GetOrderForPaymentResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            log.info("OrderGrpcService.getOrderForPayment.start", "取得訂單付款資訊",
                    "order_no", orderNo);

            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) {
                responseObserver.onNext(GetOrderForPaymentResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 取第一筆明細的 skuName 作為 product_name
            OrderItemEntity firstItem = orderItemRepository.findFirstByOrderNo(orderNo);
            String productName = (firstItem != null) ? firstItem.getSkuName() : "";

            // totalActualAmount 已包含運費，直接使用
            long amount = order.getTotalActualAmount();

            OrderPaymentInfo info = OrderPaymentInfo.newBuilder()
                    .setOrderNo(order.getOrderNo())
                    .setMerId(order.getMerId() != null ? order.getMerId() : "")
                    .setAmount(String.valueOf(amount))
                    .setBuyerId(String.valueOf(order.getBuyerId()))
                    .setStatus(order.getStatus())
                    .setProductName(productName)
                    // #2501 供 payment-service seller 付款歷程 store 歸屬驗證
                    .setStoreId(String.valueOf(order.getStoreId()))
                    .build();

            responseObserver.onNext(GetOrderForPaymentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(info)
                    .build());
            responseObserver.onCompleted();

            log.info("OrderGrpcService.getOrderForPayment.done", "取得訂單付款資訊完成",
                    "order_no", orderNo);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderForPaymentResponse.newBuilder(), e, "OrderGrpcService", "getOrderForPayment", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 服務間呼叫（review-service）====================
    @Override
    public void getOrderForReview(GetOrderForReviewRequest request, StreamObserver<GetOrderForReviewResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();

            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) {
                responseObserver.onNext(GetOrderForReviewResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<OrderItemEntity> items = orderItemRepository.findByOrderNo(orderNo);

            responseObserver.onNext(GetOrderForReviewResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(OrderProtoConverter.toOrderForReviewData(order))
                    .addAllItems(OrderProtoConverter.toOrderItemsForReview(items, getCdnBaseUrl()))
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderForReviewResponse.newBuilder(), e, "OrderGrpcService", "getOrderForReview", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 結帳模組 ====================

    @Override
    public void checkoutValidate(CheckoutValidateRequest request, StreamObserver<CheckoutValidateResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            CheckoutValidateData data = checkoutService.validate(buyerId, request.getItemsList());

            responseObserver.onNext(CheckoutValidateResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (OrderBusinessException e) {
            responseObserver.onNext(CheckoutValidateResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CheckoutValidateResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutValidateResponse.newBuilder(), e, "OrderGrpcService", "checkoutValidate", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * Uber 即時報價中繼（66 §6.1.3；Stage 8B W2-2）。
     *
     * <p>🔴 沒有 {@code OrderBusinessException} 分支<b>是刻意的</b>：
     * {@code CheckoutService.uberQuote} 的契約是「業務失敗回 {@code available=false} ＋ 理由碼」，
     * 不拋業務例外。真的冒出 {@code OrderBusinessException} 表示那條契約破了，
     * 應該走 {@code GrpcErrorHandler} 的 500 讓它被看見，而不是靜靜轉成一個業務碼。
     */
    @Override
    public void getUberQuoteForCheckout(OrderUberQuoteRequest request,
                                        StreamObserver<OrderUberQuoteResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);

            OrderUberQuoteData data = checkoutService.uberQuote(
                    request.getBuyerId(), request.getAddressId(), request.getItemsList());

            responseObserver.onNext(OrderUberQuoteResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            // 🔴 只收 requireAccountType 自己丟的那兩個 status。
            //    下游 gRPC（product / buyer / shipping）的 UNAVAILABLE、DEADLINE_EXCEEDED 也是
            //    StatusRuntimeException——把它們一起收進來會變成「權限不足」，
            //    而真正的成因是某個服務掛了。那種歸因錯誤要花一整輪才追得回來。
            Status.Code code = e.getStatus().getCode();
            if (code != Status.Code.UNAUTHENTICATED && code != Status.Code.PERMISSION_DENIED) {
                GrpcErrorHandler.handleException(responseObserver, OrderUberQuoteResponse.newBuilder(), e, "OrderGrpcService", "getUberQuoteForCheckout", GrpcResponseCode.INTERNAL_ERROR);
                return;
            }
            responseObserver.onNext(OrderUberQuoteResponse.newBuilder()
                    .setCode(code == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, OrderUberQuoteResponse.newBuilder(), e, "OrderGrpcService", "getUberQuoteForCheckout", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void checkoutShippingFee(CheckoutShippingFeeRequest request, StreamObserver<CheckoutShippingFeeResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);

            CheckoutShippingFeeData data = checkoutService.shippingFee(
                    request.getItemsList(),
                    request.getShippingMethod(),
                    request.getDeliveryTemp(),
                    request.getReceiverZipCode(),
                    request.getCvsStoreId()
            );

            responseObserver.onNext(CheckoutShippingFeeResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (UnavailableItemsException e) {
            // 20 §4.2：ShippingFee 不做 row 級攔截，但若 resolved 空（所有 SKU LOOKUP_FAILED / 全 inactive）
            // 用 envelope code=4001 + 通用 message 回；前端引導買家返回購物車（已反灰）
            responseObserver.onNext(CheckoutShippingFeeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INSUFFICIENT_STOCK.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CheckoutShippingFeeResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutShippingFeeResponse.newBuilder(), e, "OrderGrpcService", "checkoutShippingFee", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void checkoutPreview(CheckoutPreviewRequest request, StreamObserver<CheckoutPreviewResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            CheckoutPreviewData data = checkoutService.preview(
                    request.getItemsList(),
                    request.getShippingMethod(),
                    request.getDeliveryTemp(),
                    request.getReceiverZipCode(),
                    request.getCvsStoreId(),
                    request.getCouponTemplateId(),
                    buyerId,
                    request.getConsolidated(),
                    // 3003 專用（其餘配送方式為空字串）：結構化收件地址的來源鍵 ＋ 買家手上那筆報價。
                    // 少了它們，preview 對 3003 會依 deny-by-default 回 9503，
                    // 而買家在購物車是選得到 Uber 的 ⇒ 一條可達的死路。
                    request.getAddressId(),
                    request.getUberQuoteId()
            );

            responseObserver.onNext(CheckoutPreviewResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (UnavailableItemsException e) {
            // 20 §3.4 + §4.14：row 級不可購買 → envelope code=4001 + data.unavailable_items[]
            CheckoutPreviewData.Builder dataBuilder = CheckoutPreviewData.newBuilder();
            for (var u : e.getItems()) {
                dataBuilder.addUnavailableItems(UnavailableItem.newBuilder()
                        .setSkuId(u.skuId())
                        .setSkuName(u.skuName())
                        .setReason(u.reason())
                        .setRequestedQty(u.requestedQty())
                        .setAvailableQty(u.availableQty())
                        .build());
            }
            responseObserver.onNext(CheckoutPreviewResponse.newBuilder()
                    .setCode(GrpcResponseCode.INSUFFICIENT_STOCK.code())
                    .setMessage(e.getMessage())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CheckoutPreviewResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutPreviewResponse.newBuilder(), e, "OrderGrpcService", "checkoutPreview", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void checkoutConfirm(CheckoutConfirmRequest request, StreamObserver<CheckoutConfirmResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);

            CheckoutConfirmData data = checkoutService.confirm(request);

            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("建立成功")
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (CheckoutExpiredException e) {
            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(GrpcResponseCode.CHECKOUT_EXPIRED.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (CheckoutPriceChangedException e) {
            // 金額變動：回傳變動明細供前端提示
            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(GrpcResponseCode.CHECKOUT_PRICE_CHANGED.code())
                    .setMessage(e.getMessage())
                    .setPriceChanged(CheckoutPriceChangedDetail.newBuilder()
                            .setOldTotalAmount(String.valueOf(e.getOldTotalAmount()))
                            .setNewTotalAmount(String.valueOf(e.getNewTotalAmount()))
                            .setChangeReasons(String.join(",", e.getChangeReasons()))
                            .setNewSubtotal(String.valueOf(e.getNewSubtotal()))
                            .setNewShippingFee(String.valueOf(e.getNewShippingFee()))
                            .setNewDiscount(String.valueOf(e.getNewDiscount()))
                            .build())
                    .build());
            responseObserver.onCompleted();
        } catch (UnavailableItemsException e) {
            // 20 §3.5 + §4.14：row 級不可購買 → envelope code=4001 + data.unavailable_items[]
            // 由 CheckoutService.confirm 在 resolveItems 階段 throw
            CheckoutConfirmData.Builder dataBuilder = CheckoutConfirmData.newBuilder();
            for (var u : e.getItems()) {
                dataBuilder.addUnavailableItems(UnavailableItem.newBuilder()
                        .setSkuId(u.skuId())
                        .setSkuName(u.skuName())
                        .setReason(u.reason())
                        .setRequestedQty(u.requestedQty())
                        .setAvailableQty(u.availableQty())
                        .build());
            }
            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(GrpcResponseCode.INSUFFICIENT_STOCK.code())
                    .setMessage(e.getMessage())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (InsufficientStockException e) {
            // 後備：建單階段（OrderService.createOrderFromCheckout）DeductStock 失敗時可能仍 throw 舊例外。
            // 20 §3.5 + §4.14：envelope code=4001 + data.unavailable_items[reason=INSUFFICIENT_STOCK]
            CheckoutConfirmData.Builder dataBuilder = CheckoutConfirmData.newBuilder();
            for (InsufficientStockException.Item item : e.getItems()) {
                dataBuilder.addUnavailableItems(UnavailableItem.newBuilder()
                        .setSkuId(item.skuId())
                        .setReason(CartItemUnavailableReason.INSUFFICIENT_STOCK.value())
                        .setRequestedQty(item.requestedQty())
                        .setAvailableQty(item.availableQty())
                        .build());
            }
            responseObserver.onNext(CheckoutConfirmResponse.newBuilder()
                    .setCode(GrpcResponseCode.INSUFFICIENT_STOCK.code())
                    .setMessage(e.getMessage())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CheckoutConfirmResponse.newBuilder(), e, "OrderGrpcService", "checkoutConfirm", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * 釋放優惠券（軟依賴：失敗僅記錄警告）
     */
    /**
     * 關閉付款交易（best-effort，失敗僅記錄日誌）
     */
    private void closePaymentForOrder(String orderNo) {
        try {
            paymentGrpcClient.closePaymentByOrderNo(orderNo);
        } catch (Exception e) {
            log.error("OrderGrpcService.closePayment.error", "關閉付款交易失敗（best-effort）: " + e.getMessage(), e);
        }
    }

    private void releaseCouponForOrder(String orderNo) {
        try {
            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) return;
            String couponInstanceId = order.getCouponInstanceId();
            if (couponInstanceId != null && !couponInstanceId.isEmpty()) {
                checkoutCouponGrpcClient.releaseCoupon(couponInstanceId, orderNo);
            }
        } catch (Exception e) {
            log.error("OrderGrpcService.releaseCoupon.error", "釋放優惠券失敗", e);
        }
    }

    // ==================== 管理端 ====================
    @Override
    public void searchOrders(SearchOrdersRequest request, StreamObserver<SearchOrdersResponse> responseObserver) {
        try {
            // 授權：僅 ADMIN 可搜尋所有訂單
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            int page = ProtoConversionUtils.parseIntRequired(request.getPage().getPage(), "page");
            int size = ProtoConversionUtils.parseIntRequired(request.getPage().getSize(), "size");

            // buyer_id / store_id 為管理端查詢參數（SearchOrdersRequest 欄位名），空字串表示不篩選
            long buyerIdFilter = ProtoConversionUtils.parseLongOrDefault(request.getBuyerId(), 0L);
            long storeIdFilter = ProtoConversionUtils.parseLongOrDefault(request.getStoreId(), 0L);

            List<OrderEntity> orders = orderService.searchOrders(
                    request.getOrderNo(), buyerIdFilter, storeIdFilter, request.getStatus(), page, size);
            long total = orderService.countSearchOrders(
                    request.getOrderNo(), buyerIdFilter, storeIdFilter, request.getStatus());

            // 聚合賣家資訊
            List<Long> sellerIds = orders.stream().map(OrderEntity::getStoreId).distinct().toList();
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(sellerIds);
            // 聚合買家資訊（管理端需要顯示買家暱稱+頭像）
            List<Long> buyerIds = orders.stream().map(OrderEntity::getBuyerId).distinct().toList();
            Map<Long, BuyerDisplay> buyerDisplayMap = fetchBuyerDisplayQuietly(buyerIds);
            String cdnBaseUrl = getCdnBaseUrl();

            List<OrderSummaryView> summaries = orders.stream()
                    .map(o -> OrderProtoConverter.toSummaryView(o, orderService.getOrderItems(o.getOrderNo()),
                            sellerMetaMap.get(o.getStoreId()),
                            buyerDisplayMap.get(o.getBuyerId()), cdnBaseUrl, true))
                    .toList();

            responseObserver.onNext(SearchOrdersResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .addAllOrders(summaries)
                    .setMeta(OrderProtoConverter.toPageMeta(page, size, total))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(SearchOrdersResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, SearchOrdersResponse.newBuilder(), e, "OrderGrpcService", "searchOrders", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getOrderAggregates(GetOrderAggregatesRequest request, StreamObserver<GetOrderAggregatesResponse> responseObserver) {
        try {
            // Auth：僅 ADMIN，或帶 x-internal-service（GrpcAuthContext.INTERNAL_CALL）自動 bypass —
            // 與 searchOrders 相同模式，crm-service 走 internalServiceInterceptor 呼叫。
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            int page = ProtoConversionUtils.parseIntRequired(request.getPage().getPage(), "page");
            int size = ProtoConversionUtils.parseIntRequired(request.getPage().getSize(), "size");
            long storeId = request.getStoreId();

            List<OrderAggregateResult> rows = orderService.getOrderAggregates(storeId, page, size);
            long total = orderService.countOrderAggregates(storeId);

            GetOrderAggregatesResponse.Builder builder = GetOrderAggregatesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setMeta(OrderProtoConverter.toPageMeta(page, size, total));

            for (OrderAggregateResult r : rows) {
                builder.addData(OrderAggregateItem.newBuilder()
                        .setStoreId(String.valueOf(r.getStoreId()))
                        .setBuyerId(String.valueOf(r.getBuyerId()))
                        .setOrderCount(String.valueOf(r.getOrderCount()))
                        .setTotalNetAmount(r.getTotalNetAmount() != null ? String.valueOf(r.getTotalNetAmount().longValue()) : "0")
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetOrderAggregatesResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderAggregatesResponse.newBuilder(), e, "OrderGrpcService", "getOrderAggregates", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getAdminOrderDetail(GetAdminOrderDetailRequest request, StreamObserver<GetAdminOrderDetailResponse> responseObserver) {
        try {
            // 授權：僅 ADMIN 可查詢管理端訂單詳情
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            OrderEntity order = orderService.getOrderByNo(request.getOrderNo());
            List<OrderItemEntity> items = orderService.getOrderItems(request.getOrderNo());
            List<RefundRequestEntity> refunds = orderService.getRefundHistory(request.getOrderNo());
            List<FinancialLogEntity> financialLogs = orderService.getFinancialLogs(request.getOrderNo());
            ShipmentDetailView shipment = fetchShipmentQuietly(order.getOrderNo());

            // 聚合賣家資訊
            Map<Long, SellerMeta> sellerMetaMap = fetchSellerMetaQuietly(List.of(order.getStoreId()));
            SellerMeta sellerMeta = sellerMetaMap.get(order.getStoreId());
            // 聚合買家資訊（管理端需要顯示買家暱稱+頭像）
            Map<Long, BuyerDisplay> buyerDisplayMap = fetchBuyerDisplayQuietly(List.of(order.getBuyerId()));
            BuyerDisplay buyerDisplay = buyerDisplayMap.get(order.getBuyerId());
            String cdnBaseUrl = getCdnBaseUrl();

            // §29 Admin 對帳明細：依 order.status 聚合 provider_*
            ProviderPair providerPair = resolveProviderStatusFor(order);
            OrderDetailView baseDetailView = OrderProtoConverter.toDetailView(order, items, refunds, shipment, sellerMeta, buyerDisplay, cdnBaseUrl, true);
            OrderDetailView enrichedDetailView = enrichProvider(baseDetailView, providerPair);

            AdminOrderDetailData.Builder dataBuilder = AdminOrderDetailData.newBuilder()
                    .setOrderInfo(enrichedDetailView)
                    .addAllLogs(OrderProtoConverter.toFinancialLogViews(financialLogs));

            // §29 providerContext 附加額外欄位（trade_no / refund_no / ship_trade_no 等對帳細節）
            String source = resolveProviderSource(order.getStatus());
            if (!source.isEmpty() && (!providerPair.status().isEmpty() || !providerPair.message().isEmpty())) {
                AdminProviderContext.Builder pcBuilder = AdminProviderContext.newBuilder()
                        .setProviderSource(source)
                        .setProviderStatus(providerPair.status() == null ? "" : providerPair.status())
                        .setProviderMessage(providerPair.message() == null ? "" : providerPair.message());
                // 補對帳細節
                if (ProviderSource.SHIPPING.value().equals(source)) {
                    if (shipment != null) {
                        pcBuilder.setShipTradeNo(shipment.getShipTradeNo() == null ? "" : shipment.getShipTradeNo());
                    }
                }
                if (ProviderSource.PAYMENT.value().equals(source)) {
                    pcBuilder.setProviderTradeNo(order.getTransactionId() == null ? "" : order.getTransactionId());
                }
                dataBuilder.setProviderContext(pcBuilder.build());
            }

            responseObserver.onNext(GetAdminOrderDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetAdminOrderDetailResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (OrderNotFoundException e) {
            responseObserver.onNext(GetAdminOrderDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetAdminOrderDetailResponse.newBuilder(), e, "OrderGrpcService", "getAdminOrderDetail", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * §29 依 order.status 決定 provider 取源（與 resolveProviderStatusMap 內的分組一致）。
     * 回傳空字串代表「此訂單目前沒有第三方環節可對帳」——那不是 ProviderSource 的成員，
     * 所以這裡回字串而非 enum。
     */
    private static String resolveProviderSource(String status) {
        if (status == null) return "";
        if (STATUS_PAYMENT_SOURCE.contains(status)) return ProviderSource.PAYMENT.value();
        if (STATUS_REFUND_SOURCE.contains(status)) return ProviderSource.PAYMENT_REFUND.value();
        if (STATUS_SHIPPING_SOURCE.contains(status)) return ProviderSource.SHIPPING.value();
        return "";
    }

    // ==================== §29 A1 Admin 異常訂單列表 ====================

    @Override
    public void listAbnormalOrdersForAdmin(ListAbnormalOrdersForAdminRequest request,
                                            StreamObserver<ListAbnormalOrdersForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            int pageNum = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = Math.min(ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20), 100);

            var wrapper = new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<OrderEntity>();

            // 預設異常 status filter。目前 OrderStatus enum 唯一的「異常」終態是 SHIPPING_ABNORMAL。
            // 若 caller 未帶 status 則預設只列此狀態；caller 可顯式傳 status=... 覆寫。
            if (org.springframework.util.StringUtils.hasText(request.getStatus())) {
                wrapper.in(OrderEntity::getStatus, java.util.Arrays.asList(request.getStatus().split(",")));
            } else {
                wrapper.eq(OrderEntity::getStatus,
                        io.github.will413028.hawkr.common.utils.code.OrderStatus.SHIPPING_ABNORMAL.value());
            }

            if (org.springframework.util.StringUtils.hasText(request.getBuyerId())) {
                try {
                    wrapper.eq(OrderEntity::getBuyerId, Long.parseLong(request.getBuyerId()));
                } catch (NumberFormatException ignore) {}
            }
            if (org.springframework.util.StringUtils.hasText(request.getStoreId())) {
                try {
                    wrapper.eq(OrderEntity::getStoreId, Long.parseLong(request.getStoreId()));
                } catch (NumberFormatException ignore) {}
            }
            if (org.springframework.util.StringUtils.hasText(request.getPaymentMethod())) {
                wrapper.eq(OrderEntity::getPaymentMethod, request.getPaymentMethod());
            }
            if (org.springframework.util.StringUtils.hasText(request.getShippingMethod())) {
                wrapper.eq(OrderEntity::getShippingMethod, request.getShippingMethod());
            }

            String sort = request.getSort();
            if (!org.springframework.util.StringUtils.hasText(sort)) sort = "updated_at_desc";
            switch (sort) {
                case "updated_at_asc" -> wrapper.orderByAsc(OrderEntity::getUpdatedAt);
                case "created_at_desc" -> wrapper.orderByDesc(OrderEntity::getCreatedAt);
                case "created_at_asc" -> wrapper.orderByAsc(OrderEntity::getCreatedAt);
                default -> wrapper.orderByDesc(OrderEntity::getUpdatedAt);
            }

            com.baomidou.mybatisplus.extension.plugins.pagination.Page<OrderEntity> page =
                    new com.baomidou.mybatisplus.extension.plugins.pagination.Page<>(pageNum, pageSize);
            var result = orderRepository.selectPage(page, wrapper);

            // 批次聚合 provider_*
            Map<String, ProviderPair> providerMap = resolveProviderStatusMap(result.getRecords());

            ListAbnormalOrdersForAdminData.Builder dataBuilder = ListAbnormalOrdersForAdminData.newBuilder()
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()));

            for (OrderEntity o : result.getRecords()) {
                ProviderPair pair = providerMap.getOrDefault(o.getOrderNo(), ProviderPair.EMPTY);
                dataBuilder.addRecords(AdminAbnormalOrderSummary.newBuilder()
                        .setOrderNo(o.getOrderNo() == null ? "" : o.getOrderNo())
                        .setBuyerId(o.getBuyerId() == null ? "" : String.valueOf(o.getBuyerId()))
                        .setStoreId(o.getStoreId() == null ? "" : String.valueOf(o.getStoreId()))
                        .setStatus(o.getStatus() == null ? "" : o.getStatus())
                        .setTotalActualAmount(o.getTotalActualAmount() == null ? "" : String.valueOf(o.getTotalActualAmount()))
                        .setPaymentMethod(o.getPaymentMethod() == null ? "" : o.getPaymentMethod())
                        .setShippingMethod(o.getShippingMethod() == null ? "" : o.getShippingMethod())
                        .setProviderStatus(pair.status() == null ? "" : pair.status())
                        .setProviderMessage(pair.message() == null ? "" : pair.message())
                        .setProviderSource(resolveProviderSource(o.getStatus()))
                        .setCreatedAt(ProtoConversionUtils.formatIso8601(o.getCreatedAt()))
                        .setUpdatedAt(ProtoConversionUtils.formatIso8601(o.getUpdatedAt()))
                        .build());
            }

            responseObserver.onNext(ListAbnormalOrdersForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListAbnormalOrdersForAdminResponse.newBuilder(),
                    e, "OrderGrpcService", "listAbnormalOrdersForAdmin",
                    GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 服務間：買家消費彙總 ====================

    @Override
    public void getBuyerMerchantSummary(GetBuyerMerchantSummaryRequest request,
                                        StreamObserver<GetBuyerMerchantSummaryResponse> responseObserver) {
        try {
            var results = orderRepository.findBuyerMerchantSummary(request.getBuyerId());
            var builder = GetBuyerMerchantSummaryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (var r : results) {
                long lastOrderMs = r.getLastOrderTime() != null
                        ? DateTimeUtils.toEpochMillis(r.getLastOrderTime())
                        : 0L;
                builder.addData(BuyerMerchantSummaryItem.newBuilder()
                        .setStoreId(String.valueOf(r.getStoreId()))
                        .setTotalSpending(r.getTotalSpending() != null
                                ? r.getTotalSpending().toBigInteger().toString() : "0")
                        .setLastOrderTime(String.valueOf(lastOrderMs))
                        .setTotalRefund(r.getTotalRefund() != null
                                ? r.getTotalRefund().toBigInteger().toString() : "0")
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetBuyerMerchantSummaryResponse.newBuilder(), e, "OrderGrpcService", "getBuyerMerchantSummary", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    @Override
    public void getBuyerOrdersForSeller(GetBuyerOrdersForSellerRequest request,
                                        StreamObserver<GetBuyerOrdersForSellerResponse> responseObserver) {
        try {
            int page = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int size = ProtoConversionUtils.parseIntOrDefault(request.getSize(), 20);
            int offset = (page - 1) * size;

            long total = orderRepository.countBuyerOrdersForSeller(
                    request.getBuyerId(), request.getStoreId());
            var orders = orderRepository.findBuyerOrdersForSeller(
                    request.getBuyerId(), request.getStoreId(), size, offset);

            String cdnBaseUrl = getCdnBaseUrl();

            var builder = GetBuyerOrdersForSellerResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setTotal(String.valueOf(total))
                    .setPage(String.valueOf(page))
                    .setPageSize(String.valueOf(size));

            for (var o : orders) {
                // 時間格式統一使用 ISO 8601（UTC）
                String createTimeIso = o.getCreatedAt() != null
                        ? DateTimeUtils.formatIso8601(o.getCreatedAt())
                        : "";
                var recordBuilder = BuyerSellerOrderRecord.newBuilder()
                        .setOrderNo(o.getOrderNo())
                        .setCreateTime(createTimeIso)
                        .setActualAmount(o.getTotalActualAmount() != null
                                ? o.getTotalActualAmount().toBigInteger().toString() : "0")
                        .setDiscountAmount(o.getDiscountAmount() != null
                                ? o.getDiscountAmount().toBigInteger().toString() : "0")
                        .setShippingAddress(o.getShippingAddress() != null ? o.getShippingAddress() : "")
                        .setRecipientPhone(o.getRecipientPhone() != null ? o.getRecipientPhone() : "")
                        .setShippingFee(ProtoConversionUtils.formatAmount(o.getShippingFee()))
                        .setRefundAmount(ProtoConversionUtils.formatAmount(o.getRefundTotalAmount()));

                // 商品明細（復用 OrderItemDetail）
                List<OrderItemEntity> items = orderService.getOrderItems(o.getOrderNo());
                recordBuilder.addAllItems(OrderProtoConverter.toItemDetails(items, cdnBaseUrl));

                builder.addRecords(recordBuilder.build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetBuyerOrdersForSellerResponse.newBuilder(), e, "OrderGrpcService", "getBuyerOrdersForSeller", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 賣家儀表板 ====================

    @Override
    public void getSellerOrderStatusCounts(GetSellerOrderStatusCountsRequest request,
                                           StreamObserver<GetSellerOrderStatusCountsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            long[] counts = orderService.getSellerOrderStatusCounts(storeId);
            responseObserver.onNext(GetSellerOrderStatusCountsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setPendingShipmentCount(counts[0])
                    .setInProgressCount(counts[1])
                    // counts[2] = 「已結束」桶（含 CANCELLED(80)/FULLY_REFUNDED(70)，非成交數）；
                    // 欄名 completed_count 為既有 wire 契約保留，語意見 OrderService#getSellerOrderStatusCounts。
                    .setCompletedCount(counts[2])
                    .setAftersalesCount(counts[3])
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerOrderStatusCountsResponse.newBuilder(), e, "OrderGrpcService", "getSellerOrderStatusCounts", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 推薦商家：營收排名 ====================

    @Override
    public void getTopSellersByRevenue(GetTopSellersByRevenueRequest request,
                                       StreamObserver<GetTopSellersByRevenueResponse> responseObserver) {
        try {
            int days = request.getDays() > 0 ? request.getDays() : 30;
            int limit = request.getLimit() > 0 ? request.getLimit() : 10;

            var results = orderService.getTopSellersByRevenue(days, limit);
            var builder = GetTopSellersByRevenueResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (var r : results) {
                builder.addSellers(SellerRevenue.newBuilder()
                        .setStoreId(ProtoConversionUtils.formatLong(r.getStoreId()))
                        .setTotalRevenue(r.getTotalRevenue() != null
                                ? r.getTotalRevenue().longValue() : 0L)
                        .setOrderCount(r.getOrderCount())
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetTopSellersByRevenueResponse.newBuilder(), e, "OrderGrpcService", "getTopSellersByRevenue", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 推薦商家：各店訂單統計 ====================

    private static final int STORE_ORDER_STATS_MAX_DAYS = 90;
    private static final int STORE_ORDER_STATS_DEFAULT_PAGE_SIZE = 500;
    private static final int STORE_ORDER_STATS_MAX_PAGE_SIZE = 1000;

    /** 空字串＝從頭（0）；非十進位數字或負數回 -1（呼叫端視為 INVALID_PARAM）。 */
    private static long parseStoreOrderStatsCursor(String raw) {
        if (raw.isEmpty()) {
            return 0L;
        }
        try {
            long v = Long.parseLong(raw);
            return v < 0 ? -1L : v;
        } catch (NumberFormatException e) {
            return -1L;
        }
    }

    @Override
    public void listStoreOrderStats(ListStoreOrderStatsRequest request,
                                    StreamObserver<ListStoreOrderStatsResponse> responseObserver) {
        try {
            int days = request.getDays();
            int pageSize = request.getPageSize() == 0
                    ? STORE_ORDER_STATS_DEFAULT_PAGE_SIZE : request.getPageSize();
            long afterStoreId = parseStoreOrderStatsCursor(request.getAfterStoreId());
            if (days < 1 || days > STORE_ORDER_STATS_MAX_DAYS
                    || pageSize < 1 || pageSize > STORE_ORDER_STATS_MAX_PAGE_SIZE
                    || afterStoreId < 0) {
                responseObserver.onNext(ListStoreOrderStatsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(GrpcResponseCode.INVALID_PARAM.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            var page = orderService.listStoreOrderStats(days, afterStoreId, pageSize);
            var builder = ListStoreOrderStatsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setNextAfterStoreId(page.nextAfterStoreId() == 0L
                            ? "" : ProtoConversionUtils.formatLong(page.nextAfterStoreId()));
            for (var r : page.rows()) {
                builder.addStats(StoreOrderStats.newBuilder()
                        .setStoreId(ProtoConversionUtils.formatLong(r.getStoreId()))
                        .setDistinctBuyers(r.getDistinctBuyers())
                        .setQualifyingOrders(r.getQualifyingOrders())
                        .setSellerCancelledOrders(r.getSellerCancelledOrders())
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListStoreOrderStatsResponse.newBuilder(), e, "OrderGrpcService", "listStoreOrderStats", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 單體賣家儀表板：單店分時間窗的訂單統計 ====================

    private static final int STORE_PERIOD_STATS_MAX_WINDOWS = 4;
    private static final long STORE_PERIOD_STATS_MAX_WINDOW_MS = 90L * 24 * 60 * 60 * 1000;

    /** Unix ms 數字字串；不是十進位正整數回 -1（呼叫端視為 INVALID_PARAM）。 */
    private static long parseEpochMillis(String raw) {
        try {
            long v = Long.parseLong(raw);
            return v > 0 ? v : -1L;
        } catch (NumberFormatException e) {
            return -1L;
        }
    }

    /** 驗證並轉成 [start, end) 的毫秒對；任何一窗不合法回 null。 */
    private static List<long[]> parseStatsWindows(List<StatsWindow> windows) {
        if (windows.isEmpty() || windows.size() > STORE_PERIOD_STATS_MAX_WINDOWS) {
            return null;
        }
        List<long[]> out = new ArrayList<>(windows.size());
        for (StatsWindow w : windows) {
            long start = parseEpochMillis(w.getStartTime());
            long end = parseEpochMillis(w.getEndTime());
            if (start < 0 || end < 0 || end <= start || end - start > STORE_PERIOD_STATS_MAX_WINDOW_MS) {
                return null;
            }
            out.add(new long[]{start, end});
        }
        return out;
    }

    @Override
    public void getStorePeriodOrderStats(GetStorePeriodOrderStatsRequest request,
                                         StreamObserver<GetStorePeriodOrderStatsResponse> responseObserver) {
        try {
            List<long[]> windows = request.getStoreId() > 0 ? parseStatsWindows(request.getWindowsList()) : null;
            if (windows == null) {
                responseObserver.onNext(GetStorePeriodOrderStatsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(GrpcResponseCode.INVALID_PARAM.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            var results = orderService.getStorePeriodOrderStats(request.getStoreId(), windows);
            var builder = GetStorePeriodOrderStatsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            for (var r : results) {
                builder.addWindows(StorePeriodOrderStats.newBuilder()
                        .setQualifyingOrders(ProtoConversionUtils.formatLong(r.getQualifyingOrders()))
                        .setDistinctBuyers(ProtoConversionUtils.formatLong(r.getDistinctBuyers()))
                        .setRevenue(ProtoConversionUtils.formatLong(r.getRevenue()))
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetStorePeriodOrderStatsResponse.newBuilder(), e, "OrderGrpcService", "getStorePeriodOrderStats", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 快照付款資訊 ====================

    /**
     * 快照付款資訊到訂單（CreatePayment 成功後由 Gateway 呼叫）
     *
     * 將 trade_no、payment_method、ATM 虛擬帳號等資訊寫入 orders 表，
     * 讓訂單明細 API 可直接從 DB 讀取，無需聚合 Payment Service。
     */
    @Override
    public void snapshotPaymentInfo(SnapshotPaymentInfoRequest request,
                                     StreamObserver<SnapshotPaymentInfoResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            if (orderNo.isEmpty()) {
                responseObserver.onNext(SnapshotPaymentInfoResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(GrpcResponseCode.INVALID_PARAM.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 使用自訂 SQL 只更新快照欄位，避免 updateById 全欄位覆蓋導致 race condition
            // （與 PaymentEventHandler 的 @Transactional updateById 併發時，會把 status 覆蓋回舊值）
            orderRepository.snapshotPaymentInfo(
                    orderNo,
                    request.getTradeNo(),
                    request.getPaymentMethod(),
                    request.getAtmBankCode(),
                    request.getAtmVirtualAccount(),
                    request.getAtmExpireDate()
            );
            log.info("OrderGrpcService.snapshotPaymentInfo.success",
                    "付款資訊已快照: orderNo=" + orderNo + ", tradeNo=" + request.getTradeNo());

            responseObserver.onNext(SnapshotPaymentInfoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, SnapshotPaymentInfoResponse.newBuilder(), e, "OrderGrpcService", "snapshotPaymentInfo", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 服務間呼叫（aftersales-service）====================

    /**
     * 售後用：查詢訂單詳情（含買家地址、金額、送達時間）
     * 內部呼叫（x-internal-service），無權限檢查
     */
    @Override
    public void getOrderForAfterSales(GetOrderForAfterSalesRequest request,
                                       StreamObserver<GetOrderForAfterSalesResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            log.info("OrderGrpcService.getOrderForAfterSales.start", "查詢售後訂單資訊",
                    "order_no", orderNo);

            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) {
                responseObserver.onNext(GetOrderForAfterSalesResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<OrderItemEntity> items = orderItemRepository.findByOrderNo(orderNo);
            String cdnBaseUrl = getCdnBaseUrl();

            responseObserver.onNext(GetOrderForAfterSalesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(OrderProtoConverter.toAfterSalesView(order, items, cdnBaseUrl))
                    .build());
            responseObserver.onCompleted();

            log.info("OrderGrpcService.getOrderForAfterSales.done", "查詢售後訂單資訊完成",
                    "order_no", orderNo);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetOrderForAfterSalesResponse.newBuilder(), e, "OrderGrpcService", "getOrderForAfterSales", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * 售後用：內部取消訂單（CAS status IN (0,10,20)）
     * 內部呼叫（x-internal-service），無權限檢查
     *
     * 1. CAS 更新狀態 → CANCELLED(80)
     * 2. 若 0 rows → 回傳 CANCEL_NOT_ALLOWED
     * 3. 發布庫存歸還事件（best-effort）
     * 4. 釋放優惠券（best-effort）
     */
    @Override
    public void cancelOrderInternal(CancelOrderInternalRequest request,
                                     StreamObserver<CancelOrderInternalResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            String cancelReason = request.getCancelReason();
            String cancelRemark = request.getCancelRemark();
            String cancelByRole = request.getCancelByRole();

            log.info("OrderGrpcService.cancelOrderInternal.start", "售後內部取消訂單",
                    "order_no", orderNo);

            // 03 §5.3.2：寫入前 validator 強制檢查
            if (cancelReason != null && !cancelReason.isBlank()) {
                io.github.will413028.hawkr.common.utils.code.CodeMetaValidator.requireValidValue(
                        io.github.will413028.hawkr.common.utils.code.OrderCancelReason.class, cancelReason, "cancelReason");
            }
            io.github.will413028.hawkr.common.utils.code.CodeMetaValidator.requireValidValue(
                    io.github.will413028.hawkr.common.utils.code.OrderCancelByRole.class, cancelByRole, "cancelByRole");

            // CAS 狀態更新：PENDING_PAYMENT(0)/PAID(10)/PENDING_SHIPMENT(20) → CANCELLED(80)
            // #1368：狀態變更 + 庫存歸還 order.cancelled outbox 於 OrderService 同事務落地，
            // 由 outbox scheduler 保證至少一次投遞，取代原 CAS auto-commit + TX 外 best-effort 直發
            int updated = orderService.cancelOrderInternalInTx(orderNo, cancelReason, cancelRemark, cancelByRole);
            if (updated == 0) {
                responseObserver.onNext(CancelOrderInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_STATUS.code())
                        .setMessage(GrpcResponseCode.INVALID_STATUS.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // TX 外：釋放優惠券（best-effort）
            releaseCouponForOrder(orderNo);

            // [B-207] #1229：賣家工作區 realtime — 售後內部取消訂單 → CANCELLED(80)（eager，本 handler 非 @Transactional；03 §2.1 emit 矩陣 #10）
            // previousStatus 傳 null：CAS WHERE status IN ('0','10','20') 多前置，與物流推進路徑一致不精確回填
            Long storeId = orderRepository.findStoreIdByOrderNo(orderNo);
            eventPublisher.emitSellerOrderUpdated(
                    storeId, orderNo, OrderEventPublisher.SRC_SYSTEM,
                    null, io.github.will413028.hawkr.common.utils.code.OrderStatus.CANCELLED.value(), null);

            responseObserver.onNext(CancelOrderInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("訂單取消成功")
                    .build());
            responseObserver.onCompleted();

            log.info("OrderGrpcService.cancelOrderInternal.done", "售後內部取消訂單完成",
                    "order_no", orderNo);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CancelOrderInternalResponse.newBuilder(), e, "OrderGrpcService", "cancelOrderInternal", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * 賣家停權 IM 訂單關聯回填（14 號設計書 §6.5.1 路徑②、46 號 §4.4）：
     * 反查買賣雙方之間是否存在「關聯」訂單（status NOT IN (0,80)，§6.5.2）。
     * 內部呼叫（x-internal-service），無權限檢查。
     */
    @Override
    public void hasOrderBetween(io.github.will413028.hawkr.order.grpc.HasOrderBetweenRequest request,
                                 StreamObserver<io.github.will413028.hawkr.order.grpc.HasOrderBetweenResponse> responseObserver) {
        try {
            long buyerId = request.getBuyerId();
            long storeId = request.getStoreId();

            String latestOrderNo = orderRepository.findLatestRelatedOrderNoBetween(buyerId, storeId);

            responseObserver.onNext(io.github.will413028.hawkr.order.grpc.HasOrderBetweenResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setHasOrder(latestOrderNo != null)
                    .setLatestOrderNo(latestOrderNo != null ? latestOrderNo : "")
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, io.github.will413028.hawkr.order.grpc.HasOrderBetweenResponse.newBuilder(), e, "OrderGrpcService", "hasOrderBetween", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    // ==================== 服務間：發票快照（58 號 §7.1.2 步驟 5 ①）====================

    /**
     * invoice-service 的 {@code order.paid} consumer 專用：取開票所需的訂單不可變快照。
     *
     * <p><b>唯讀</b>——不改任何狀態、不寫任何表。
     *
     * <p>🔴 <b>查無訂單、或該訂單沒有發票快照列 ⇒ 回非 0 業務碼，不得回 code=0 ＋ 空 data</b>
     * （proto 註解逐字）。呼叫端唯一安全的行為是讓事件重試三次後進 DLQ；空 data 會被當成
     * 「這張訂單沒有品項」而開出一張金額為 0 的發票，而發票不可回收。
     *
     * <p><b>型別</b>：所有數值（TSID、金額、數量、版本號）一律字串。
     * {@code protonumericlint} 走訪任何 RPC output type 可達的 message，response 側寫
     * int64 會直接被 {@code make proto-lint-numeric} 擋下。
     *
     * <p><b>{@code discount_funding} 恆為空字串</b>：判別欄是 coupon-service 的
     * {@code coupon_templates.issuer_type}，而 {@code CouponValidateResult} 不帶它
     * （58 號 U-7）⇒ order-service 判不出來。呼叫端必須把空值當 <b>fail-closed</b>
     * （轉 AWAITING_MANUAL ＋ PLATFORM_FUNDED_DISCOUNT），不得當成 "SELLER"——
     * 把平台補貼那一段從賣家銷售額扣掉＝賣家短漏報銷售額。
     */
    @Override
    public void getOrderInvoiceSnapshotInternal(GetOrderInvoiceSnapshotInternalRequest request,
                                               StreamObserver<GetOrderInvoiceSnapshotInternalResponse> responseObserver) {
        try {
            // §5.2 internal-only：發票側服務間呼叫，無使用者情境。簽章由 server interceptor 驗
            // （52 號 HMAC 四 header），這裡只讀 interceptor 的判定結果。
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("GetOrderInvoiceSnapshotInternal 僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            String orderNo = request.getOrderNo();
            OrderEntity order = orderRepository.findByOrderNo(orderNo);
            if (order == null) {
                log.warn("OrderGrpcService.getOrderInvoiceSnapshotInternal.orderNotFound",
                        "查無訂單", "order_no", orderNo);
                responseObserver.onNext(GetOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("查無此訂單: " + orderNo)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            OrderInvoiceSnapshotEntity snapshot =
                    orderInvoiceSnapshotRepository.findLatestByOrderNo(orderNo);
            if (snapshot == null) {
                // V12 之前建立的訂單沒有快照。fail-loud 是刻意的：靜默補一個「無載具 B2C」
                // 會讓 cutover 之前的舊訂單被自動開票，而那些訂單的買家從來沒有做過發票選擇。
                log.warn("OrderGrpcService.getOrderInvoiceSnapshotInternal.snapshotNotFound",
                        "訂單無發票快照（V12 之前建立的訂單）", "order_no", orderNo);
                responseObserver.onNext(GetOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("訂單無發票快照: " + orderNo)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<OrderItemEntity> items = orderItemRepository.findByOrderNo(orderNo);

            OrderInvoiceSnapshotData.Builder data = OrderInvoiceSnapshotData.newBuilder()
                    .setOrderNo(orderNo)
                    .setSnapshotVersion(String.valueOf(snapshot.getSnapshotVersion()))
                    .setInvoiceKind(nullToEmpty(snapshot.getInvoiceKind()))
                    .setCarrierSelectionMode(nullToEmpty(snapshot.getCarrierSelectionMode()))
                    .setBuyerInvoicePreferenceId(snapshot.getBuyerInvoicePreferenceId() == null
                            ? "" : String.valueOf(snapshot.getBuyerInvoicePreferenceId()))
                    .setBuyerTaxId(nullToEmpty(snapshot.getBuyerTaxId()))
                    .setBuyerTitle(nullToEmpty(snapshot.getBuyerTitle()))
                    .setDonationCode(nullToEmpty(snapshot.getDonationCode()))
                    .setNotifyEmail(nullToEmpty(snapshot.getNotifyEmail()))
                    .setTotalOriginalAmount(amountStr(order.getTotalOriginalAmount()))
                    .setShippingFee(amountStr(order.getShippingFee()))
                    .setDiscountAmount(amountStr(order.getDiscountAmount()))
                    .setTotalActualAmount(amountStr(order.getTotalActualAmount()))
                    // 見 javadoc：判不出來就送空字串，讓呼叫端 fail-closed
                    .setDiscountFunding("")
                    .setIsConsolidatedOrder(isConsolidatedOrder(order.getMerId()));

            for (OrderItemEntity item : items) {
                data.addItems(OrderInvoiceSnapshotItem.newBuilder()
                        .setOrderItemId(String.valueOf(item.getId()))
                        .setSkuId(item.getSkuId() == null ? "" : String.valueOf(item.getSkuId()))
                        .setProductName(nullToEmpty(item.getProductName()))
                        .setSkuName(nullToEmpty(item.getSkuName()))
                        .setQuantity(item.getQuantity() == null ? "0" : String.valueOf(item.getQuantity()))
                        .setUnitPrice(amountStr(item.getUnitPrice()))
                        .build());
            }

            responseObserver.onNext(GetOrderInvoiceSnapshotInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data.build())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetOrderInvoiceSnapshotInternalResponse.newBuilder(), e,
                    "OrderGrpcService", "getOrderInvoiceSnapshotInternal", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * 買家於開立前更正發票資料（58 號 §6.2.5、§7.3.5 R03）。
     *
     * <p><b>order-service 是 {@code snapshot_version} 的唯一寫入者</b>（I-50）。
     * invoice-service 只能複製本 RPC 回傳的版本號、不得自行 +1。
     *
     * <p><b>append-only</b>：成功時 INSERT 一列 v(n+1)，<b>絕不 UPDATE</b> 既有列——
     * 「下單當下宣告了什麼」是爭議發生時唯一的憑據，原地改會讓它消失。
     *
     * <p><b>{@code notify_email} 不在本 RPC 的可更正欄位內</b>（§6.2.5 定版 7）：
     * {@code snapshot_version} 是內容軸，只承載發票上會印出來的東西；通知信箱只決定
     * 通知寄到哪。讓它遞增內容軸，「同內容重開一張票」與「內容真的變了」在冪等鍵上就
     * 無法區分，而那正是 {@code issue_attempt_seq} 實體軸存在的理由。本方法把前一版的
     * 值原樣帶過來。
     *
     * <p><b>三道防線，方向一致（都是「寧可失敗不要覆蓋」）</b>：
     * <ol>
     *   <li>冪等鍵命中 ⇒ 回同一個版本，不新增列</li>
     *   <li>{@code expectedSnapshotVersion} 與最新版不符 ⇒ 9308，不寫入</li>
     *   <li>並行時兩邊都算出同一個 v(n+1) ⇒ {@code uk_order_invoice_snapshots} 擋下，
     *       {@link DuplicateKeyException} 轉 9308 讓呼叫端重讀重試</li>
     * </ol>
     *
     * <p>🔴 <b>本方法刻意不加 {@code @Transactional}</b>：全程只有一句 INSERT，而
     * PostgreSQL 的交易內任一句失敗會讓整個交易 aborted（SQLSTATE 25P02）——包在交易裡
     * 再 catch {@code DuplicateKeyException}，後面任何一句 SQL 都會炸成 500。
     * auto-commit 下捕捉它才是安全的（{@code .claude/rules/10-engineering-contracts.md} §11.1）。
     */
    @Override
    public void reviseOrderInvoiceSnapshotInternal(ReviseOrderInvoiceSnapshotInternalRequest request,
                                                   StreamObserver<ReviseOrderInvoiceSnapshotInternalResponse> responseObserver) {
        try {
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("ReviseOrderInvoiceSnapshotInternal 僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            String orderNo = request.getOrderNo();
            String idempotencyKey = request.getIdempotencyKey();
            if (orderNo.isBlank() || idempotencyKey.isBlank()) {
                responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("order_no 與 idempotency_key 皆為必填")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 防線 1：這把鑰匙先前已經寫成功過 ⇒ 回同一個版本。
            // 放在最前面：重送時連 order 都不必查，且不會因為期間有別人更正過而誤判版本衝突。
            OrderInvoiceSnapshotEntity replayed =
                    orderInvoiceSnapshotRepository.findByRevisionIdempotencyKey(orderNo, idempotencyKey);
            if (replayed != null) {
                log.info("OrderGrpcService.reviseOrderInvoiceSnapshotInternal.idempotentReplay",
                        "冪等重送，回既有版本 v" + replayed.getSnapshotVersion(), "order_no", orderNo);
                responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("success")
                        .setSnapshotVersion(String.valueOf(replayed.getSnapshotVersion()))
                        .build());
                responseObserver.onCompleted();
                return;
            }

            OrderInvoiceSnapshotEntity latest =
                    orderInvoiceSnapshotRepository.findLatestByOrderNo(orderNo);
            if (latest == null) {
                // 沒有 v1 就沒有可更正的對象。fail-loud 與 Get 那一支同理：靜默補一列
                // 會讓從未做過發票選擇的舊訂單被自動開票。
                log.warn("OrderGrpcService.reviseOrderInvoiceSnapshotInternal.snapshotNotFound",
                        "訂單無發票快照", "order_no", orderNo);
                responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("訂單無發票快照: " + orderNo)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 防線 2：樂觀鎖。沒有它，兩筆並行更正是後寫的靜默覆蓋前者，而兩筆都顯示成功。
            String expected = request.getExpectedSnapshotVersion();
            if (!String.valueOf(latest.getSnapshotVersion()).equals(expected)) {
                log.warn("OrderGrpcService.reviseOrderInvoiceSnapshotInternal.versionConflict",
                        "版本不符：expected=" + expected + " actual=" + latest.getSnapshotVersion(),
                        "order_no", orderNo);
                responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.code())
                        .setMessage(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // §8：對應 CodeMeta Enum 的輸入欄位，寫入 DB 前必須走共用 validator。
            // （這與 §6.2.5 定版 4「不做發票業務驗證」不衝突：那條講的是三選一、金額這類
            //   業務規則，交給 chk_ois_exclusive 等 DB 約束；碼值合法性是另一層，§8 強制。）
            CodeMetaValidator.requireValidValue(InvoiceDocumentKind.class,
                    request.getInvoiceKind(), "invoiceKind");
            CodeMetaValidator.requireValidValue(InvoiceCarrierSelectionMode.class,
                    request.getCarrierSelectionMode(), "carrierSelectionMode");

            OrderInvoiceSnapshotEntity next = new OrderInvoiceSnapshotEntity();
            next.setId(TsidUtil.generate());
            next.setOrderNo(orderNo);
            next.setSnapshotVersion(latest.getSnapshotVersion() + 1);
            next.setInvoiceKind(request.getInvoiceKind());
            next.setCarrierSelectionMode(request.getCarrierSelectionMode());
            next.setBuyerInvoicePreferenceId(emptyToNullLong(request.getBuyerInvoicePreferenceId()));
            next.setBuyerTaxId(emptyToNull(request.getBuyerTaxId()));
            next.setBuyerTitle(emptyToNull(request.getBuyerTitle()));
            next.setDonationCode(emptyToNull(request.getDonationCode()));
            // 見 javadoc：通知信箱不是內容軸，原樣沿用前一版。
            next.setNotifyEmail(latest.getNotifyEmail());
            next.setRevisionIdempotencyKey(idempotencyKey);

            try {
                orderInvoiceSnapshotRepository.insert(next);
            } catch (DuplicateKeyException e) {
                // 防線 3：並行的兩邊都通過了防線 2、都算出同一個 v(n+1)。
                // uk_order_invoice_snapshots 擋下後者；回 9308 讓它重讀重試，
                // 不要吞掉——吞掉就是「回報成功但什麼都沒寫」。
                log.warn("OrderGrpcService.reviseOrderInvoiceSnapshotInternal.raced",
                        "並行更正撞唯一鍵 v" + next.getSnapshotVersion(), "order_no", orderNo);
                responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.code())
                        .setMessage(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            log.info("OrderGrpcService.reviseOrderInvoiceSnapshotInternal.revised",
                    "發票快照已更正為 v" + next.getSnapshotVersion(), "order_no", orderNo);
            responseObserver.onNext(ReviseOrderInvoiceSnapshotInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setSnapshotVersion(String.valueOf(next.getSnapshotVersion()))
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ReviseOrderInvoiceSnapshotInternalResponse.newBuilder(), e,
                    "OrderGrpcService", "reviseOrderInvoiceSnapshotInternal", GrpcResponseCode.INTERNAL_ERROR);
        }
    }

    /**
     * 和單判定（58 號 §7.1.3 fail-closed 三格之一）。
     *
     * <p>和單會把收款方換成代理商 MerID ⇒ 開票法源前提不成立。order-service 沒有專門的
     * 「是否和單」欄位，{@code orders.mer_id == PAYUNI_AGENT_MER_ID} 是唯一可用的訊號。
     * {@code agentMerId} 為空（本環境未設定和單）⇒ 一律 false，而不是「每張訂單都是和單」
     * ——空字串比對到空 merId 會讓後者成真。
     */
    private boolean isConsolidatedOrder(String orderMerId) {
        return agentMerId != null && !agentMerId.isBlank() && agentMerId.equals(orderMerId);
    }

    /** 金額／數量：null 一律送 "0"（DB 上這些欄位都有值，null 只可能來自極舊的殘列）。 */
    private static String amountStr(Long v) {
        return v == null ? "0" : String.valueOf(v);
    }

    // ==================== Admin 撥款重發（收斂規範 §8.4）====================

    @Override
    public void retryGrant(io.github.will413028.hawkr.order.grpc.RetryGrantRequest request,
                            StreamObserver<io.github.will413028.hawkr.order.grpc.RetryGrantResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            String orderNo = request.getOrderNo();
            if (orderNo == null || orderNo.isBlank()) {
                responseObserver.onNext(io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("order_no 為必填")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity task =
                    grantPaymentTaskRepository.selectOne(
                            new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<
                                    io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity>()
                                    .eq(io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity::getOrderNo, orderNo)
                                    .last("LIMIT 1"));
            if (task == null) {
                responseObserver.onNext(io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("該訂單無撥款任務紀錄")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if ("DONE".equals(task.getStatus())) {
                responseObserver.onNext(io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder()
                        .setCode(GrpcResponseCode.CONFLICT.code())
                        .setMessage("撥款任務已完成，不可重發")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            int previousRetryCount = task.getRetryCount() != null ? task.getRetryCount() : 0;
            int rows = grantPaymentTaskRepository.resetForRetry(orderNo);
            if (rows == 0) {
                responseObserver.onNext(io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder()
                        .setCode(GrpcResponseCode.CONFLICT.code())
                        .setMessage("撥款任務非 FAILED / MANUAL_INTERVENTION 狀態，無需重發")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            log.info("OrderGrpcService.retryGrant.reset",
                    "Admin 重設撥款任務 orderNo=" + orderNo
                            + " operatorId=" + request.getOperatorId()
                            + " reason=" + request.getOperatorReason()
                            + " prevRetryCount=" + previousRetryCount);

            responseObserver.onNext(io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(io.github.will413028.hawkr.order.grpc.RetryGrantData.newBuilder()
                            .setOrderNo(orderNo)
                            .setTaskStatus("PENDING")
                            .setPreviousRetryCount(String.valueOf(previousRetryCount))
                            .build())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.order.grpc.RetryGrantResponse.newBuilder(),
                    e, "OrderGrpcService", "retryGrant", GrpcResponseCode.INTERNAL_ERROR);
        }
    }
}
