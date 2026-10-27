package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.SellerOrderQuery;
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

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Discriminating regression test for the getSellerOrders/getSellerOrderDetail
 * store_id-vs-account_id bug (Plan C Phase 2). login account_id and the
 * gateway-resolved store surrogate are deliberately different values in both
 * tests below — a same-value test cannot distinguish the bug (reads
 * GrpcAuthContext) from the fix (reads request.getStoreId()).
 *
 * Mocking/context-attach pattern follows the established convention in
 * product-service's ProductGrpcServiceGetProductOwnerTest (GrpcAuthContext has
 * no test-only setters; auth values are injected via io.grpc.Context.attach()).
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class OrderGrpcServiceSellerOrdersTest {

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
    @DisplayName("getSellerOrders uses request.getStoreId(), not the raw JWT login account_id")
    void getSellerOrders_usesResolvedStoreId_notLoginAccountId() {
        Context ctx = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 99L)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.SELLER.code());
        Context previous = ctx.attach();
        try {
            SellerOrderQuery query = new SellerOrderQuery(7001L, 0L, List.of(), null, null, null);
            when(orderService.sellerOrderQuery(anyLong(), anyLong(), anyString(), anyString(), anyString(), anyString()))
                    .thenReturn(query);
            when(orderService.getSellerOrders(any(SellerOrderQuery.class), anyInt(), anyInt()))
                    .thenReturn(List.of());
            when(orderService.countSellerOrders(any(SellerOrderQuery.class))).thenReturn(0L);

            OrderGrpcService service = buildService();

            GetSellerOrdersRequest request = GetSellerOrdersRequest.newBuilder()
                    .setStoreId(7001L)
                    .setPage(PageRequest.newBuilder().setPage("1").setSize("20").build())
                    .build();
            @SuppressWarnings("unchecked")
            StreamObserver<GetSellerOrdersResponse> observer = mock(StreamObserver.class);

            service.getSellerOrders(request, observer);

            ArgumentCaptor<Long> sellerIdCaptor = ArgumentCaptor.forClass(Long.class);
            verify(orderService).sellerOrderQuery(sellerIdCaptor.capture(), anyLong(), anyString(),
                    anyString(), anyString(), anyString());
            assertEquals(7001L, sellerIdCaptor.getValue(),
                    "must key off request.getStoreId()=7001 (resolved surrogate), not login account_id=99");
        } finally {
            ctx.detach(previous);
        }
    }

    @Test
    @DisplayName("getSellerOrderDetail denies access when order.store_id != request.getStoreId(), even if login account_id equals the order's resolved store_id")
    void getSellerOrderDetail_ownershipCheck_usesResolvedStoreId() {
        // login account_id (7001) deliberately equals the order's resolved store_id —
        // the OLD buggy code (GrpcAuthContext.requireAccountId()) would have read 7001
        // here too and WRONGLY ALLOWED (7001 == 7001). The fix must reject anyway
        // because it compares against request.getStoreId() (999), not this context value.
        Context ctx = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 7001L)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.SELLER.code());
        Context previous = ctx.attach();
        try {
            OrderEntity order = new OrderEntity();
            order.setOrderNo("ORD1");
            order.setStoreId(7001L); // resolved store surrogate
            // Fixture completeness note: status/buyerId are set only so that IF the
            // ownership check were bypassed (the bug), execution could reach a clean 200
            // SUCCESS response instead of an incidental NPE on an unrelated unset field —
            // this makes the bug-vs-fix divergence (ALLOW vs DENY) unambiguous in the RED run.
            // "80" (CANCELLED) is deliberately outside all three provider-status source sets
            // in resolveProviderStatusMap (no extra gRPC aggregation) and outside the
            // COMPLETED-only review-deadline branch in OrderProtoConverter.
            order.setStatus("80");
            order.setBuyerId(500L);

            when(orderService.getOrderByNo("ORD1")).thenReturn(order);

            OrderGrpcService service = buildService();

            GetSellerOrderDetailRequest request = GetSellerOrderDetailRequest.newBuilder()
                    .setStoreId(999L) // attacker's own resolved store id, does NOT match the order's actual store_id
                    .setOrderNo("ORD1")
                    .build();
            @SuppressWarnings("unchecked")
            StreamObserver<GetSellerOrderDetailResponse> observer = mock(StreamObserver.class);

            service.getSellerOrderDetail(request, observer);

            ArgumentCaptor<GetSellerOrderDetailResponse> responseCaptor = ArgumentCaptor.forClass(GetSellerOrderDetailResponse.class);
            verify(observer).onNext(responseCaptor.capture());
            assertEquals(404, responseCaptor.getValue().getCode(),
                    "must anti-enumeration 404 when request.getStoreId() != order.store_id, even though login "
                            + "account_id happens to equal the order's store_id — proves the check uses "
                            + "request.getStoreId(), not the raw JWT account_id");
        } finally {
            ctx.detach(previous);
        }
    }
}
