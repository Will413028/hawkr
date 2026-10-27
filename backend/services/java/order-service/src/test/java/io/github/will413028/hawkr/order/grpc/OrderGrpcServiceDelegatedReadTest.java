package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.grpc.DelegationCapability;
import io.github.will413028.hawkr.common.grpc.DelegationContext;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 代行讀取訂單（56 §6.1.10 / §8.1.5）。
 *
 * <p>三件事：遮罩由 order-service 執行而非 Gateway、履約與金流欄位不得出現在代行回應、
 * PII 揭露必須帶 audit_log_id。三者漏掉都不會有錯誤訊號——回應照樣 200。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class OrderGrpcServiceDelegatedReadTest {

    private static final long STORE_ID = 7001L;

    @Mock private OrderService orderService;
    @Mock private CheckoutService checkoutService;
    @Mock private CheckoutCouponGrpcClient checkoutCouponGrpcClient;
    @Mock private OrderRepository orderRepository;
    @Mock private OrderItemRepository orderItemRepository;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private MediaGrpcClient mediaGrpcClient;
    @Mock private MediaUrlProperties mediaUrlProperties;
    @Mock private GrantPaymentTaskRepository grantPaymentTaskRepository;
    @Mock private OrderEventPublisher eventPublisher;

    private OrderGrpcService buildService() {
        return new OrderGrpcService(orderService, checkoutService, checkoutCouponGrpcClient,
                orderRepository, orderItemRepository, shippingGrpcClient, paymentGrpcClient,
                sellerGrpcClient, buyerGrpcClient, mediaGrpcClient, mediaUrlProperties,
                grantPaymentTaskRepository,
                org.mockito.Mockito.mock(io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository.class),
                eventPublisher, "");
    }

    private static DelegationContext.Builder delegation() {
        return DelegationContext.newBuilder()
                .setSessionId("2701445588392001")
                .setOperatorAdminId("42")
                .setCapability(DelegationCapability.ORDERS_READ);
    }

    private OrderEntity seedOrder() {
        OrderEntity order = new OrderEntity();
        order.setOrderNo("ORD1");
        order.setStoreId(STORE_ID);
        // 80 = CANCELLED，刻意選在所有 provider 狀態聚合來源之外，讓斷言只反映投影本身
        order.setStatus("80");
        order.setBuyerId(500L);
        order.setRecipientName("王小明");
        order.setRecipientPhone("0912341234");
        order.setShippingAddress("台北市大安區信義路四段 123 號 5 樓之 3");
        order.setTransactionId("TXN-0001");
        order.setAtmVirtualAccount("9988776655");
        order.setLabelUrl("labels/ORD1.pdf");
        order.setOdno("12345678");
        when(orderService.getOrderByNo("ORD1")).thenReturn(order);
        return order;
    }

    private OrderDetailView detailOf(GetSellerOrderDetailRequest request) {
        @SuppressWarnings("unchecked")
        StreamObserver<GetSellerOrderDetailResponse> observer = mock(StreamObserver.class);
        buildService().getSellerOrderDetail(request, observer);
        ArgumentCaptor<GetSellerOrderDetailResponse> captor =
                ArgumentCaptor.forClass(GetSellerOrderDetailResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(0, captor.getValue().getCode(), captor.getValue().getMessage());
        return captor.getValue().getData();
    }

    @Test
    @DisplayName("代行詳情：手機與地址遮罩，姓名不遮")
    void delegatedDetail_masksRecipientContact() {
        seedOrder();
        OrderDetailView detail = detailOf(GetSellerOrderDetailRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setOrderNo("ORD1")
                .setDelegation(delegation().build())
                .build());

        assertEquals("王小明", detail.getRecipientName());
        assertEquals("0912-***-234", detail.getRecipientPhone());
        assertEquals("台北市大安區", detail.getShippingAddress());
    }

    @Test
    @DisplayName("代行詳情：履約與金流欄位一律淨空（路徑禁字斷言擋不到 DTO 層）")
    void delegatedDetail_clearsFulfillmentAndPaymentFields() {
        seedOrder();
        OrderDetailView detail = detailOf(GetSellerOrderDetailRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setOrderNo("ORD1")
                .setDelegation(delegation().build())
                .build());

        assertTrue(detail.getTransactionId().isEmpty(), "金流交易明細不得外流");
        assertTrue(detail.getAtmVirtualAccount().isEmpty(), "ATM 虛擬帳號不得外流");
        assertTrue(detail.getLabelKey().isEmpty(), "託運單不得外流");
        assertTrue(detail.getOdno().isEmpty(), "出貨單編號不得外流");
    }

    @Test
    @DisplayName("賣家本人路徑逐位元不變：不帶 delegation 一律回原值")
    void sellerOwnPath_unchanged() {
        seedOrder();
        // 不帶 delegation 走的仍是 requireAccountType(SELLER)，身分由 gRPC Context 注入
        Context ctx = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, STORE_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.SELLER.code());
        Context previous = ctx.attach();
        try {
            OrderDetailView detail = detailOf(GetSellerOrderDetailRequest.newBuilder()
                    .setStoreId(STORE_ID)
                    .setOrderNo("ORD1")
                    .build());

            assertEquals("0912341234", detail.getRecipientPhone());
            assertEquals("台北市大安區信義路四段 123 號 5 樓之 3", detail.getShippingAddress());
            assertEquals("TXN-0001", detail.getTransactionId());
            assertEquals("labels/ORD1.pdf", detail.getLabelKey());
        } finally {
            ctx.detach(previous);
        }
    }

    @Test
    @DisplayName("reveal_recipient_contact=true 且帶 audit_log_id ⇒ 回完整收件資訊")
    void reveal_withAuditLogId_returnsFullContact() {
        seedOrder();
        OrderDetailView detail = detailOf(GetSellerOrderDetailRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setOrderNo("ORD1")
                .setRevealRecipientContact(true)
                .setDelegation(delegation().setAuditLogId("2701445588392099").build())
                .build());

        assertEquals("0912341234", detail.getRecipientPhone());
        assertEquals("台北市大安區信義路四段 123 號 5 樓之 3", detail.getShippingAddress());
        // 揭露的是收件資訊，不是履約與金流
        assertTrue(detail.getTransactionId().isEmpty());
        assertTrue(detail.getLabelKey().isEmpty());
    }

    @Test
    @DisplayName("reveal_recipient_contact=true 但 audit_log_id 為空 ⇒ 9260，且不揭露")
    void reveal_withoutAuditLogId_rejected() {
        seedOrder();
        @SuppressWarnings("unchecked")
        StreamObserver<GetSellerOrderDetailResponse> observer = mock(StreamObserver.class);
        buildService().getSellerOrderDetail(GetSellerOrderDetailRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setOrderNo("ORD1")
                .setRevealRecipientContact(true)
                .setDelegation(delegation().build())
                .build(), observer);

        ArgumentCaptor<GetSellerOrderDetailResponse> captor =
                ArgumentCaptor.forClass(GetSellerOrderDetailResponse.class);
        verify(observer).onNext(captor.capture());
        // #2744：兩者的主體都是「Gateway 組錯 DelegationContext」，不是操作者填錯（9260）
        // 也不是他權限不足（9262）—— 後者還會讓稽核列指控一位沒做錯事的管理員。
        assertEquals(9289, captor.getValue().getCode());
        assertTrue(captor.getValue().getData().getRecipientPhone().isEmpty());
    }

    @Test
    @DisplayName("capability 不是 orders.read ⇒ 9262")
    void wrongCapability_denied() {
        seedOrder();
        @SuppressWarnings("unchecked")
        StreamObserver<GetSellerOrderDetailResponse> observer = mock(StreamObserver.class);
        buildService().getSellerOrderDetail(GetSellerOrderDetailRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setOrderNo("ORD1")
                .setDelegation(delegation().setCapability(DelegationCapability.PRODUCTS_READ).build())
                .build(), observer);

        ArgumentCaptor<GetSellerOrderDetailResponse> captor =
                ArgumentCaptor.forClass(GetSellerOrderDetailResponse.class);
        verify(observer).onNext(captor.capture());
        // #2744：兩者的主體都是「Gateway 組錯 DelegationContext」，不是操作者填錯（9260）
        // 也不是他權限不足（9262）—— 後者還會讓稽核列指控一位沒做錯事的管理員。
        assertEquals(9289, captor.getValue().getCode());
    }
}
