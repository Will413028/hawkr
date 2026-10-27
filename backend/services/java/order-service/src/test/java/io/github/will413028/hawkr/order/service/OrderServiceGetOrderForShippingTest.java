package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.exception.OrderNotFoundException;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * getOrderForShipping 為純內部讀（#1366：移除 storeId==0 哨兵繞歸屬）。
 * 授權由 HMAC internal-auth + netpol 保證，方法本身不再做歸屬校驗——
 * 任何 store_id 的訂單都應原樣回傳，僅在訂單不存在時拋 OrderNotFoundException。
 */
class OrderServiceGetOrderForShippingTest {

    @Test
    @DisplayName("getOrderForShipping returns the order regardless of store_id (pure internal read, no ownership check)")
    void getOrderForShipping_returnsOrderWithoutOwnershipCheck() {
        OrderEntity order = new OrderEntity();
        order.setOrderNo("ORD1");
        order.setStoreId(7001L);

        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.findByOrderNo("ORD1")).thenReturn(order);

        OrderService service = OrderServiceTestHarness.withOrderRepository(orderRepository);

        OrderEntity result = service.getOrderForShipping("ORD1");
        assertEquals("ORD1", result.getOrderNo());
        assertEquals(7001L, result.getStoreId());
    }

    @Test
    @DisplayName("getOrderForShipping throws OrderNotFoundException for unknown order_no")
    void getOrderForShipping_orderNotFound() {
        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.findByOrderNo("NOPE")).thenReturn(null);

        OrderService service = OrderServiceTestHarness.withOrderRepository(orderRepository);

        assertThrows(OrderNotFoundException.class, () -> service.getOrderForShipping("NOPE"));
    }
}
