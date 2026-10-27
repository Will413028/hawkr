package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * {@code GetOrderInvoiceSnapshotInternal}（58 號 §7.1.2 步驟 5 ①）。
 *
 * <p>三件事在守：
 * <ol>
 *   <li><b>internal-only</b>：它把整張訂單的品項與買家發票選擇一次吐出來，沒有任何買家授權檢查
 *       （因為呼叫端不是使用者）。少了 {@code isInternalCall()} 這道閘門，任何持有 JWT 的買家
 *       都能用別人的 order_no 讀走別人的訂單明細——<b>而它不會報錯</b>。</li>
 *   <li><b>查無訂單／查無快照一律回非 0</b>：proto 註解逐字要求。回 {@code code=0} ＋ 空 data
 *       會讓 invoice-service 把它當成「這張訂單沒有品項」而開出一張金額為 0 的發票，
 *       而發票開出去不可回收。</li>
 *   <li><b>22 個欄位逐項對帳</b>：所有數值是字串、金額取自 {@code orders}（不是請求）、
 *       {@code discount_funding} 恆空（order-service 判不出來，呼叫端必須 fail-closed）。</li>
 * </ol>
 */
class OrderInvoiceSnapshotInternalTest {

    private static final String ORDER_NO = "O20260830001";
    private static final String AGENT_MER_ID = "HAWKAGENT01";

    private OrderRepository orderRepository;
    private OrderItemRepository orderItemRepository;
    private OrderInvoiceSnapshotRepository invoiceSnapshotRepository;

    private OrderGrpcService buildService(String agentMerId) {
        return new OrderGrpcService(
                mock(OrderService.class), mock(CheckoutService.class), mock(CheckoutCouponGrpcClient.class),
                orderRepository, orderItemRepository, mock(ShippingGrpcClient.class),
                mock(PaymentGrpcClient.class), mock(SellerGrpcClient.class), mock(BuyerGrpcClient.class),
                mock(MediaGrpcClient.class), mock(MediaUrlProperties.class),
                mock(GrantPaymentTaskRepository.class), invoiceSnapshotRepository,
                mock(OrderEventPublisher.class), agentMerId);
    }

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        orderItemRepository = mock(OrderItemRepository.class);
        invoiceSnapshotRepository = mock(OrderInvoiceSnapshotRepository.class);
    }

    // ==================== fixtures ====================

    private static OrderEntity order(String merId) {
        OrderEntity o = new OrderEntity();
        o.setOrderNo(ORDER_NO);
        o.setBuyerId(2623900520L);
        o.setStoreId(2685914682L);
        o.setTotalOriginalAmount(1000L);
        o.setShippingFee(60L);
        o.setDiscountAmount(100L);
        o.setTotalActualAmount(960L);
        o.setMerId(merId);
        return o;
    }

    private static OrderInvoiceSnapshotEntity snapshot() {
        OrderInvoiceSnapshotEntity s = new OrderInvoiceSnapshotEntity();
        s.setId(2649605557L);
        s.setOrderNo(ORDER_NO);
        s.setSnapshotVersion(1);
        s.setInvoiceKind(InvoiceDocumentKind.B2B.value());
        s.setCarrierSelectionMode(InvoiceCarrierSelectionMode.NONE.value());
        s.setBuyerTaxId("12345678");
        s.setBuyerTitle("範例生鮮股份有限公司");
        s.setNotifyEmail("buyer@example.com");
        return s;
    }

    private static OrderItemEntity item(long id, long skuId, String product, String sku, int qty, long unitPrice) {
        OrderItemEntity i = new OrderItemEntity();
        i.setId(id);
        i.setOrderNo(ORDER_NO);
        i.setSkuId(skuId);
        i.setProductName(product);
        i.setSkuName(sku);
        i.setQuantity(qty);
        i.setUnitPrice(unitPrice);
        // actual_pay_amount / share_discount 刻意不設：本 RPC 不得回它們（浮點分攤器的結果）
        return i;
    }

    // ==================== 呼叫工具 ====================

    private record Result(GetOrderInvoiceSnapshotInternalResponse response, Throwable error) {
    }

    /** 以「內部呼叫」情境跑一次；internal=false 時模擬使用者面呼叫。 */
    private Result call(OrderGrpcService service, boolean internal) {
        AtomicReference<GetOrderInvoiceSnapshotInternalResponse> resp = new AtomicReference<>();
        List<Throwable> errors = new ArrayList<>();
        StreamObserver<GetOrderInvoiceSnapshotInternalResponse> observer =
                new StreamObserver<>() {
                    @Override
                    public void onNext(GetOrderInvoiceSnapshotInternalResponse value) {
                        resp.set(value);
                    }

                    @Override
                    public void onError(Throwable t) {
                        errors.add(t);
                    }

                    @Override
                    public void onCompleted() {
                    }
                };

        Context ctx = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, internal);
        ctx.run(() -> service.getOrderInvoiceSnapshotInternal(
                GetOrderInvoiceSnapshotInternalRequest.newBuilder().setOrderNo(ORDER_NO).build(),
                observer));

        return new Result(resp.get(), errors.isEmpty() ? null : errors.get(0));
    }

    // ==================== 測試 ====================

    @Test
    @DisplayName("非內部呼叫一律 PERMISSION_DENIED（少了這道閘門＝任何買家可讀走任何訂單的明細，且不報錯）")
    void rejectsNonInternalCallers() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order(null));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(snapshot());

        Result r = call(buildService(""), false);

        assertThat(r.response()).as("被拒絕時不得吐出任何 data").isNull();
        assertThat(r.error()).isInstanceOf(StatusRuntimeException.class);
        assertThat(Status.fromThrowable(r.error()).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED);
    }

    @Test
    @DisplayName("查無訂單 → 非 0 業務碼且不帶 data（回 code=0 ＋ 空 data 會讓對方開出一張金額為 0 的發票）")
    void missingOrderReturnsNonZeroCode() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(null);

        Result r = call(buildService(""), true);

        assertThat(r.response().getCode()).isNotZero().isEqualTo(GrpcResponseCode.NOT_FOUND.code());
        assertThat(r.response().hasData()).isFalse();
    }

    @Test
    @DisplayName("訂單在但沒有快照（V12 之前的舊訂單）→ 非 0 業務碼；不得靜默補一個「無載具 B2C」")
    void missingSnapshotReturnsNonZeroCode() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order(null));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(null);

        Result r = call(buildService(""), true);

        assertThat(r.response().getCode()).isNotZero().isEqualTo(GrpcResponseCode.NOT_FOUND.code());
        assertThat(r.response().hasData()).isFalse();
    }

    @Test
    @DisplayName("22 欄逐項對帳：買家選擇取自快照、金額取自 orders、品項取自 order_items，全部字串")
    void mapsAllTwentyTwoFields() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order("HAWK2685914682"));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(snapshot());
        when(orderItemRepository.findByOrderNo(ORDER_NO)).thenReturn(List.of(
                item(7001L, 5001L, "蘋果禮盒", "大", 2, 400L),
                item(7002L, 5002L, "梨子", "", 1, 200L)));

        Result r = call(buildService(AGENT_MER_ID), true);

        assertThat(r.response().getCode()).isZero();
        OrderInvoiceSnapshotData d = r.response().getData();

        // --- 快照 9 欄 ---
        assertThat(d.getOrderNo()).isEqualTo(ORDER_NO);
        assertThat(d.getSnapshotVersion()).isEqualTo("1");
        assertThat(d.getInvoiceKind()).isEqualTo(InvoiceDocumentKind.B2B.value());
        assertThat(d.getCarrierSelectionMode()).isEqualTo(InvoiceCarrierSelectionMode.NONE.value());
        assertThat(d.getBuyerInvoicePreferenceId()).as("NULL 送空字串，不是 \"0\"").isEmpty();
        assertThat(d.getBuyerTaxId()).isEqualTo("12345678");
        assertThat(d.getBuyerTitle()).isEqualTo("範例生鮮股份有限公司");
        assertThat(d.getDonationCode()).isEmpty();
        assertThat(d.getNotifyEmail()).isEqualTo("buyer@example.com");

        // --- 金額 4 欄（取自 orders，不是請求）---
        assertThat(d.getTotalOriginalAmount()).isEqualTo("1000");
        assertThat(d.getShippingFee()).isEqualTo("60");
        assertThat(d.getDiscountAmount()).isEqualTo("100");
        assertThat(d.getTotalActualAmount()).isEqualTo("960");

        // --- fail-closed 兩欄 ---
        assertThat(d.getDiscountFunding())
                .as("CouponValidateResult 不帶 issuer_type（58 U-7）⇒ 判不出來就送空字串，"
                        + "呼叫端必須 fail-closed 轉 AWAITING_MANUAL，不得當成 SELLER")
                .isEmpty();
        assertThat(d.getIsConsolidatedOrder())
                .as("orders.mer_id 是賣家自己的 merId ⇒ 非和單")
                .isFalse();

        // --- 品項 6 欄 × 2 ---
        assertThat(d.getItemsCount()).isEqualTo(2);
        OrderInvoiceSnapshotItem first = d.getItems(0);
        assertThat(first.getOrderItemId()).isEqualTo("7001");
        assertThat(first.getSkuId()).isEqualTo("5001");
        assertThat(first.getProductName()).isEqualTo("蘋果禮盒");
        assertThat(first.getSkuName()).isEqualTo("大");
        assertThat(first.getQuantity()).isEqualTo("2");
        assertThat(first.getUnitPrice()).isEqualTo("400");
        assertThat(d.getItems(1).getSkuName()).as("skuName 為空時原樣送空字串").isEmpty();
    }

    @Test
    @DisplayName("品項依 order_items.id 升冪（line_seq 的決定性來源；repository 的 ORDER BY 是唯一保證）")
    void itemsFollowRepositoryOrder() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order(null));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(snapshot());
        when(orderItemRepository.findByOrderNo(ORDER_NO)).thenReturn(List.of(
                item(7001L, 5001L, "A", "", 1, 100L),
                item(7002L, 5002L, "B", "", 1, 100L),
                item(7003L, 5003L, "C", "", 1, 100L)));

        OrderInvoiceSnapshotData d = call(buildService(""), true).response().getData();

        assertThat(d.getItemsList()).extracting(OrderInvoiceSnapshotItem::getOrderItemId)
                .containsExactly("7001", "7002", "7003");
    }

    @Test
    @DisplayName("和單（orders.mer_id ＝ 代理商 MerID）→ is_consolidated_order = true，讓 invoice 端 fail-closed")
    void consolidatedOrderIsFlagged() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order(AGENT_MER_ID));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(snapshot());
        when(orderItemRepository.findByOrderNo(ORDER_NO)).thenReturn(List.of());

        assertThat(call(buildService(AGENT_MER_ID), true).response().getData().getIsConsolidatedOrder())
                .isTrue();
    }

    @Test
    @DisplayName("未設定 PAYUNI_AGENT_MER_ID 時，merId 為空的訂單不得被判成和單（空字串比對到空 merId 會全部成真）")
    void blankAgentMerIdNeverMarksConsolidated() {
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order(""));
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(snapshot());
        when(orderItemRepository.findByOrderNo(ORDER_NO)).thenReturn(List.of());

        assertThat(call(buildService(""), true).response().getData().getIsConsolidatedOrder())
                .isFalse();
    }
}
