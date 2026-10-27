package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.SellerCaseClassification;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 賣家工作台 Dashboard 四桶 count 的命名契約（03 §2.1 SSOT）。
 * <p>
 * 本測試釘的不是數字，是**第三桶裝了什麼**：它含 {@code CANCELLED(80)} 與
 * {@code FULLY_REFUNDED(70)}，所以賣家面不能把它叫「已完成」（那是 OrderStatus 50 單一狀態的
 * CodeMeta Label）。前端標籤側各有一支對應測試（web {@code tests/seller-order-tab-labels.test.ts}、
 * App {@code test/features/order/data/order_tab_mapper_test.dart}）。三支合起來守同一條契約：
 * 桶的成員與桶的名字必須對得上。
 */
class OrderServiceSellerStatusCountsTest {

    private static final long STORE_ID = 2685914682L;

    @Test
    void thirdBucketIncludesCancelledAndFullyRefunded_soItIsNotACompletedSalesCount() {
        OrderRepository orderRepository = mock(OrderRepository.class);
        AfterSalesGrpcClient afterSalesGrpcClient = mock(AfterSalesGrpcClient.class);
        when(afterSalesGrpcClient.getSellerCaseClassification(STORE_ID))
                .thenReturn(new SellerCaseClassification(0L, List.of(), List.of()));

        OrderService orderService =
                OrderServiceTestHarness.withOrderRepository(orderRepository, afterSalesGrpcClient);

        orderService.getSellerOrderStatusCounts(STORE_ID);

        // 三桶各打一次 countBySellerStatusExcludingOrders；第三次才是「已結束」桶。
        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> statuses = ArgumentCaptor.forClass(List.class);
        verify(orderRepository, times(3))
                .countBySellerStatusExcludingOrders(anyLong(), statuses.capture(), any());

        List<String> closedBucket = statuses.getAllValues().get(2);
        assertThat(closedBucket)
                .as("第三桶含已取消與全額退款 → 它是「已結束」（賣家無需再動作），不是成交數")
                .contains(OrderStatus.CANCELLED.value(), OrderStatus.FULLY_REFUNDED.value())
                .containsExactlyInAnyOrder(
                        OrderStatus.COMPLETED.value(),
                        OrderStatus.PARTIALLY_REFUNDED.value(),
                        OrderStatus.FULLY_REFUNDED.value(),
                        OrderStatus.CANCELLED.value());
    }
}
