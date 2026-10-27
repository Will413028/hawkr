package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.StorePeriodOrderStatsResult;
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
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/** GetStorePeriodOrderStats 的參數驗證與欄位映射。 */
class OrderGrpcServiceStorePeriodOrderStatsTest {

    private static final long DAY_MS = 24L * 60 * 60 * 1000;

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

    private static StatsWindow window(String start, String end) {
        return StatsWindow.newBuilder().setStartTime(start).setEndTime(end).build();
    }

    @SuppressWarnings("unchecked")
    private GetStorePeriodOrderStatsResponse call(long storeId, StatsWindow... windows) {
        StreamObserver<GetStorePeriodOrderStatsResponse> observer = mock(StreamObserver.class);
        buildService().getStorePeriodOrderStats(GetStorePeriodOrderStatsRequest.newBuilder()
                .setStoreId(storeId).addAllWindows(List.of(windows)).build(), observer);
        ArgumentCaptor<GetStorePeriodOrderStatsResponse> captor =
                ArgumentCaptor.forClass(GetStorePeriodOrderStatsResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        return captor.getValue();
    }

    @ParameterizedTest(name = "start={0} end={1} → INVALID_PARAM")
    @CsvSource({"abc,2000", "1000,abc", "0,2000", "-5,2000", "1000,1000", "2000,1000", "1.5,2000", "'',2000", "1000,99999999999999999999"})
    @DisplayName("起訖不是正的 Unix ms、或終點不晚於起點：INVALID_PARAM，不查詢")
    void rejectsBadWindow(String start, String end) {
        GetStorePeriodOrderStatsResponse resp = call(7L, window(start, end));

        assertThat(resp.getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        verify(orderService, never()).getStorePeriodOrderStats(anyLong(), anyList());
    }

    @Test
    @DisplayName("窗長超過 90 天、窗數為 0 或超過 4、store_id 不是正數：INVALID_PARAM，不查詢")
    void rejectsBadShape() {
        assertThat(call(7L, window("1000", String.valueOf(1000 + 90 * DAY_MS + 1))).getCode())
                .isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(call(7L).getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        StatsWindow w = window("1000", "2000");
        assertThat(call(7L, w, w, w, w, w).getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(call(0L, w).getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        assertThat(call(-1L, w).getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        verify(orderService, never()).getStorePeriodOrderStats(anyLong(), anyList());
    }

    @Test
    @DisplayName("邊界：窗長剛好 90 天、4 個窗通過")
    void acceptsBoundaryValues() {
        when(orderService.getStorePeriodOrderStats(anyLong(), anyList()))
                .thenReturn(List.of(new StorePeriodOrderStatsResult()));

        assertThat(call(7L, window("1000", String.valueOf(1000 + 90 * DAY_MS))).getCode())
                .isEqualTo(GrpcResponseCode.SUCCESS.code());
    }

    @Test
    @DisplayName("欄位映射：窗以毫秒傳給 service，三個值以字串依窗序回傳")
    void mapsWindowsAndValues() {
        StorePeriodOrderStatsResult a = new StorePeriodOrderStatsResult();
        a.setQualifyingOrders(7);
        a.setDistinctBuyers(4);
        a.setRevenue(12345);
        when(orderService.getStorePeriodOrderStats(anyLong(), anyList()))
                .thenReturn(List.of(a, new StorePeriodOrderStatsResult()));

        GetStorePeriodOrderStatsResponse resp = call(2685914682L, window("1000", "2000"), window("500", "1000"));

        assertThat(resp.getCode()).isEqualTo(GrpcResponseCode.SUCCESS.code());
        assertThat(resp.getWindowsList()).hasSize(2);
        assertThat(resp.getWindows(0).getQualifyingOrders()).isEqualTo("7");
        assertThat(resp.getWindows(0).getDistinctBuyers()).isEqualTo("4");
        assertThat(resp.getWindows(0).getRevenue()).isEqualTo("12345");
        assertThat(resp.getWindows(1).getQualifyingOrders()).isEqualTo("0");
        assertThat(resp.getWindows(1).getRevenue()).isEqualTo("0");
        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<long[]>> windows = ArgumentCaptor.forClass(List.class);
        verify(orderService).getStorePeriodOrderStats(org.mockito.ArgumentMatchers.eq(2685914682L), windows.capture());
        assertThat(windows.getValue()).hasSize(2);
        assertThat(windows.getValue().get(0)).containsExactly(1000L, 2000L);
        assertThat(windows.getValue().get(1)).containsExactly(500L, 1000L);
    }
}
