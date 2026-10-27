package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.StorePeriodOrderStatsResult;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/** getStorePeriodOrderStats 的口徑與窗序：狀態集合與各店統計、營收排名共用同一份常數，逐窗查詢並依輸入順序回傳（時間窗與營收在 SQL，見 StorePeriodOrderStatsIT）。 */
class OrderServiceStorePeriodOrderStatsTest {

    private static StorePeriodOrderStatsResult row(long orders, long buyers, long revenue) {
        StorePeriodOrderStatsResult r = new StorePeriodOrderStatsResult();
        r.setQualifyingOrders(orders);
        r.setDistinctBuyers(buyers);
        r.setRevenue(revenue);
        return r;
    }

    @Test
    @DisplayName("成立訂單狀態＝共用常數，與營收排名同一組八個狀態")
    void usesTheSharedQualifyingStatusSet() {
        OrderRepository repo = mock(OrderRepository.class);
        when(repo.findStorePeriodOrderStats(anyLong(), anyLong(), anyLong(), anyList())).thenReturn(row(0, 0, 0));
        OrderService service = OrderServiceTestHarness.withOrderRepository(repo);

        service.getStorePeriodOrderStats(7L, List.<long[]>of(new long[]{1000L, 2000L}));

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> statuses = ArgumentCaptor.forClass(List.class);
        verify(repo).findStorePeriodOrderStats(eq(7L), eq(1000L), eq(2000L), statuses.capture());
        assertThat(statuses.getValue()).isSameAs(OrderService.QUALIFYING_ORDER_STATUSES);
        assertThat(statuses.getValue()).containsExactlyInAnyOrder(
                OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value(),
                OrderStatus.IN_TRANSIT.value(), OrderStatus.WAITING_PICKUP.value(),
                OrderStatus.SHIPPING_ABNORMAL.value(), OrderStatus.DELIVERED.value(),
                OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value());
    }

    @Test
    @DisplayName("逐窗查詢，結果依輸入順序，不因窗的先後而對調")
    void returnsOneResultPerWindowInRequestOrder() {
        OrderRepository repo = mock(OrderRepository.class);
        when(repo.findStorePeriodOrderStats(eq(7L), eq(100L), eq(200L), anyList())).thenReturn(row(1, 1, 10));
        when(repo.findStorePeriodOrderStats(eq(7L), eq(10L), eq(20L), anyList())).thenReturn(row(2, 2, 20));
        OrderService service = OrderServiceTestHarness.withOrderRepository(repo);

        List<StorePeriodOrderStatsResult> out = service.getStorePeriodOrderStats(
                7L, List.of(new long[]{100L, 200L}, new long[]{10L, 20L}));

        assertThat(out).extracting(StorePeriodOrderStatsResult::getRevenue).containsExactly(10L, 20L);
    }
}
