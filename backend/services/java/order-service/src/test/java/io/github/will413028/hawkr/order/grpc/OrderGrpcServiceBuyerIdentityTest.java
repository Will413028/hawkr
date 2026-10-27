package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
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
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Discriminating regression test for the getMyOrders/getOrder buyer_id-vs-account_id
 * bug (buyer_id 8-service rekey, Phase 1). Login account_id (GrpcAuthContext) and the
 * gateway-resolved buyer_id (request.getBuyerId()) are deliberately different values
 * in every test below — a same-value test cannot distinguish the bug (reads
 * GrpcAuthContext) from the fix (reads request.getBuyerId()).
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class OrderGrpcServiceBuyerIdentityTest {

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

    @Test
    @DisplayName("getMyOrders uses request.getBuyerId() (resolved buyer_id), not the raw JWT login account_id")
    void getMyOrders_usesResolvedBuyerId_notLoginAccountId() {
        Context ctx = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 99L) // pre-rollover login account_id — must NOT be used
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.BUYER.code());
        Context previous = ctx.attach();
        try {
            when(orderService.getMyOrders(anyLong(), anyString(), anyInt(), anyInt()))
                    .thenReturn(java.util.List.of());
            when(orderService.countMyOrders(anyLong(), anyString())).thenReturn(0L);

            OrderGrpcService service = buildService();

            GetMyOrdersRequest request = GetMyOrdersRequest.newBuilder()
                    .setBuyerId(7001L) // gateway-resolved current buyer_id — MUST be used
                    .setPage(PageRequest.newBuilder().setPage("1").setSize("20").build())
                    .build();
            @SuppressWarnings("unchecked")
            StreamObserver<GetMyOrdersResponse> observer = mock(StreamObserver.class);

            service.getMyOrders(request, observer);

            ArgumentCaptor<Long> buyerIdCaptor = ArgumentCaptor.forClass(Long.class);
            verify(orderService).getMyOrders(buyerIdCaptor.capture(), anyString(), anyInt(), anyInt());
            assertEquals(7001L, buyerIdCaptor.getValue(),
                    "must key off request.getBuyerId()=7001 (resolved buyer_id), not login account_id=99");
        } finally {
            ctx.detach(previous);
        }
    }

    @Test
    @DisplayName("getOrder ownership check uses request.getBuyerId(), even when login account_id equals the order's buyer_id")
    void getOrder_ownershipCheck_usesResolvedBuyerId() {
        // login account_id (500) deliberately equals the order's buyer_id — the OLD
        // buggy code (GrpcAuthContext.requireAccountId()) would read 500 here too and
        // WRONGLY ALLOW (500 == 500). The fix must reject anyway because it compares
        // against request.getBuyerId() (999), not this context value.
        Context ctx = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 500L)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.BUYER.code());
        Context previous = ctx.attach();
        try {
            OrderEntity order = new OrderEntity();
            order.setOrderNo("ORD1");
            order.setBuyerId(500L); // the order actually belongs to buyer_id=500

            when(orderService.getOrderByNo("ORD1")).thenReturn(order);

            OrderGrpcService service = buildService();

            GetOrderRequest request = GetOrderRequest.newBuilder()
                    .setBuyerId(999L) // attacker's own resolved buyer_id, does NOT match the order's buyer_id
                    .setOrderNo("ORD1")
                    .build();
            @SuppressWarnings("unchecked")
            StreamObserver<GetOrderResponse> observer = mock(StreamObserver.class);

            service.getOrder(request, observer);

            ArgumentCaptor<GetOrderResponse> responseCaptor = ArgumentCaptor.forClass(GetOrderResponse.class);
            verify(observer).onNext(responseCaptor.capture());
            assertEquals(404, responseCaptor.getValue().getCode(),
                    "must anti-enumeration 404 when request.getBuyerId() != order.buyer_id, even though login "
                            + "account_id happens to equal the order's buyer_id — proves the check uses "
                            + "request.getBuyerId(), not the raw JWT account_id");
            verify(orderService, never()).getOrderItems(org.mockito.ArgumentMatchers.anyString());
        } finally {
            ctx.detach(previous);
        }
    }
}
