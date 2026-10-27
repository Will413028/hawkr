package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.StoreOrderStatsResult;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/** listStoreOrderStats 的口徑與分頁：成立訂單的狀態集合與營收排名同一組（時間窗在 SQL，見 ListStoreOrderStatsIT），多取一筆判斷下一頁。 */
class OrderServiceStoreOrderStatsTest {

    private static StoreOrderStatsResult row(long storeId) {
        StoreOrderStatsResult r = new StoreOrderStatsResult();
        r.setStoreId(storeId);
        return r;
    }

    @Test
    @DisplayName("成立訂單狀態＝營收排名同一組八個狀態；賣家取消＝CANCELLED 加 SELLER")
    void usesRevenueStatusSetAndSellerCancellation() {
        OrderRepository repo = mock(OrderRepository.class);
        when(repo.findStoreOrderStats(anyInt(), anyLong(), anyInt(), anyList(), anyString(), anyString()))
                .thenReturn(List.of());
        OrderService service = OrderServiceTestHarness.withOrderRepository(repo);

        service.listStoreOrderStats(30, 0L, 500);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> statuses = ArgumentCaptor.forClass(List.class);
        verify(repo).findStoreOrderStats(eq(30), eq(0L), eq(501), statuses.capture(),
                eq(OrderStatus.CANCELLED.value()), eq("SELLER"));
        assertThat(statuses.getValue()).containsExactlyInAnyOrder(
                OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value(),
                OrderStatus.IN_TRANSIT.value(), OrderStatus.WAITING_PICKUP.value(),
                OrderStatus.SHIPPING_ABNORMAL.value(), OrderStatus.DELIVERED.value(),
                OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value());
    }

    @Test
    @DisplayName("多取到 pageSize+1 筆：截成 pageSize 並以最後一筆當游標")
    void fullPageReturnsCursor() {
        OrderRepository repo = mock(OrderRepository.class);
        when(repo.findStoreOrderStats(anyInt(), anyLong(), anyInt(), anyList(), anyString(), anyString()))
                .thenReturn(List.of(row(1L), row(2L), row(3L)));
        OrderService service = OrderServiceTestHarness.withOrderRepository(repo);

        OrderService.StoreOrderStatsPage page = service.listStoreOrderStats(30, 0L, 2);

        assertThat(page.rows()).extracting(StoreOrderStatsResult::getStoreId).containsExactly(1L, 2L);
        assertThat(page.nextAfterStoreId()).isEqualTo(2L);
    }

    @Test
    @DisplayName("剛好 pageSize 筆或更少：最後一頁，游標為空")
    void lastPageHasEmptyCursor() {
        OrderRepository repo = mock(OrderRepository.class);
        when(repo.findStoreOrderStats(anyInt(), anyLong(), anyInt(), anyList(), anyString(), anyString()))
                .thenReturn(List.of(row(1L), row(2L)));
        OrderService service = OrderServiceTestHarness.withOrderRepository(repo);

        OrderService.StoreOrderStatsPage page = service.listStoreOrderStats(30, 0L, 2);

        assertThat(page.rows()).hasSize(2);
        assertThat(page.nextAfterStoreId()).isZero();
    }
}
