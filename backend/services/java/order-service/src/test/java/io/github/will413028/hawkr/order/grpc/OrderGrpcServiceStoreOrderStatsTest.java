package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.StoreOrderStatsResult;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/** ListStoreOrderStats 的參數驗證、預設頁大小與欄位映射。 */
class OrderGrpcServiceStoreOrderStatsTest {

    private final OrderService orderService = mock(OrderService.class);

    private OrderGrpcService buildService() {
        return new OrderGrpcService(orderService, mock(CheckoutService.class),
                mock(CheckoutCouponGrpcClient.class), mock(OrderRepository.class),
                mock(OrderItemRepository.class), mock(ShippingGrpcClient.class),
                mock(PaymentGrpcClient.class), mock(SellerGrpcClient.class),
                mock(BuyerGrpcClient.class), mock(MediaGrpcClient.class),
                mock(MediaUrlProperties.class), mock(GrantPaymentTaskRepository.class),
                mock(io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository.class),
                mock(OrderEventPublisher.class), "");
    }

    @SuppressWarnings("unchecked")
    private ListStoreOrderStatsResponse call(int days, String after, int pageSize) {
        StreamObserver<ListStoreOrderStatsResponse> observer = mock(StreamObserver.class);
        buildService().listStoreOrderStats(ListStoreOrderStatsRequest.newBuilder()
                .setDays(days).setAfterStoreId(after).setPageSize(pageSize).build(), observer);
        ArgumentCaptor<ListStoreOrderStatsResponse> captor =
                ArgumentCaptor.forClass(ListStoreOrderStatsResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        return captor.getValue();
    }

    @ParameterizedTest(name = "days={0} pageSize={1} → INVALID_PARAM")
    @CsvSource({"0,10", "-1,10", "91,10", "30,-1", "30,1001"})
    @DisplayName("days 超出 1..90 或 page_size 超出 1..1000：INVALID_PARAM，不查詢")
    void rejectsOutOfRange(int days, int pageSize) {
        ListStoreOrderStatsResponse resp = call(days, "", pageSize);

        assertThat(resp.getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        verify(orderService, never()).listStoreOrderStats(anyInt(), anyLong(), anyInt());
    }

    @ParameterizedTest(name = "after_store_id={0} → INVALID_PARAM")
    @CsvSource({"abc", "-5", "1.5", "99999999999999999999"})
    @DisplayName("after_store_id 不是非負十進位整數：INVALID_PARAM，不查詢")
    void rejectsBadCursor(String cursor) {
        ListStoreOrderStatsResponse resp = call(30, cursor, 10);

        assertThat(resp.getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        verify(orderService, never()).listStoreOrderStats(anyInt(), anyLong(), anyInt());
    }

    @Test
    @DisplayName("page_size=0 用預設 500；邊界 days=90、page_size=1000 通過")
    void defaultAndBoundaryValues() {
        when(orderService.listStoreOrderStats(anyInt(), anyLong(), anyInt()))
                .thenReturn(new OrderService.StoreOrderStatsPage(List.of(), 0L));

        call(30, "", 0);
        verify(orderService).listStoreOrderStats(30, 0L, 500);
        assertThat(call(90, "", 1000).getCode()).isEqualTo(GrpcResponseCode.SUCCESS.code());
        verify(orderService).listStoreOrderStats(90, 0L, 1000);
    }

    @Test
    @DisplayName("欄位映射：store_id 字串、三個計數與 next_after_store_id")
    void mapsRowsAndCursor() {
        StoreOrderStatsResult r = new StoreOrderStatsResult();
        r.setStoreId(2685914682L);
        r.setDistinctBuyers(4);
        r.setQualifyingOrders(7);
        r.setSellerCancelledOrders(2);
        when(orderService.listStoreOrderStats(anyInt(), anyLong(), anyInt()))
                .thenReturn(new OrderService.StoreOrderStatsPage(List.of(r), 2685914682L));

        ListStoreOrderStatsResponse resp = call(30, "100", 10);

        assertThat(resp.getCode()).isEqualTo(GrpcResponseCode.SUCCESS.code());
        assertThat(resp.getNextAfterStoreId()).isEqualTo("2685914682");
        assertThat(resp.getStatsList()).singleElement().satisfies(s -> {
            assertThat(s.getStoreId()).isEqualTo("2685914682");
            assertThat(s.getDistinctBuyers()).isEqualTo(4);
            assertThat(s.getQualifyingOrders()).isEqualTo(7);
            assertThat(s.getSellerCancelledOrders()).isEqualTo(2);
        });
        verify(orderService).listStoreOrderStats(30, 100L, 10);
    }
}
