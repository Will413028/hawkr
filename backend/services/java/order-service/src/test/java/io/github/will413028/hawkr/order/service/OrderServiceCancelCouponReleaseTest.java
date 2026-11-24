package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 取消的 transaction 內寫入釋券任務：與狀態變更在同一個 @Transactional 方法裡、在狀態 CAS 之後呼叫，任務與取消一起提交或回滾。
 */
class OrderServiceCancelCouponReleaseTest {

    private static final String ORDER_NO = "ORD-CANCEL";

    private final OrderRepository orderRepository = mock(OrderRepository.class);
    private final CouponReleaseService couponReleaseService = mock(CouponReleaseService.class);
    private final OrderService orderService = OrderServiceTestHarness.withCouponRelease(orderRepository, couponReleaseService);

    private OrderEntity order(String couponInstanceId) {
        OrderEntity o = new OrderEntity();
        o.setOrderNo(ORDER_NO);
        o.setBuyerId(7L);
        o.setStatus(OrderStatus.PENDING_PAYMENT.value());
        o.setCouponInstanceId(couponInstanceId);
        return o;
    }

    @Test
    @DisplayName("買家取消持券訂單 → 狀態 CAS 之後、同一方法內寫入釋券任務")
    void buyerCancelEnqueuesInsideTheTransaction() {
        when(orderRepository.findByOrderNoForUpdate(ORDER_NO)).thenReturn(order("V1"));
        when(orderRepository.cancelOrderByBuyerCas(eq(ORDER_NO), anyString(), anyString())).thenReturn(1);

        orderService.cancelOrder(7L, ORDER_NO);

        InOrder o = inOrder(orderRepository, couponReleaseService);
        o.verify(orderRepository).cancelOrderByBuyerCas(eq(ORDER_NO), anyString(), anyString());
        o.verify(couponReleaseService).enqueueInTx(ORDER_NO, "V1");
    }

    @Test
    @DisplayName("買家取消沒有券的訂單 → 不寫釋券任務")
    void buyerCancelWithoutCouponDoesNotEnqueue() {
        when(orderRepository.findByOrderNoForUpdate(ORDER_NO)).thenReturn(order(null));
        when(orderRepository.cancelOrderByBuyerCas(eq(ORDER_NO), anyString(), anyString())).thenReturn(1);

        orderService.cancelOrder(7L, ORDER_NO);

        verify(couponReleaseService, never()).enqueueInTx(anyString(), any());
    }

    @Test
    @DisplayName("售後內部取消持券訂單 → CAS 成功後寫入釋券任務")
    void internalCancelEnqueuesInsideTheTransaction() {
        when(orderRepository.cancelOrderInternalCas(eq(ORDER_NO), anyString(), anyString(), anyString(), anyString())).thenReturn(1);
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order("V2"));

        orderService.cancelOrderInternalInTx(ORDER_NO, "AFTER_SALES", "remark", "ADMIN");

        verify(couponReleaseService).enqueueInTx(ORDER_NO, "V2");
    }

    @Test
    @DisplayName("售後內部取消 CAS 沒更新任何列 → 不寫釋券任務")
    void internalCancelLostCasDoesNotEnqueue() {
        when(orderRepository.cancelOrderInternalCas(eq(ORDER_NO), anyString(), anyString(), anyString(), anyString())).thenReturn(0);

        orderService.cancelOrderInternalInTx(ORDER_NO, "AFTER_SALES", "remark", "ADMIN");

        verify(couponReleaseService, never()).enqueueInTx(anyString(), any());
    }
}
