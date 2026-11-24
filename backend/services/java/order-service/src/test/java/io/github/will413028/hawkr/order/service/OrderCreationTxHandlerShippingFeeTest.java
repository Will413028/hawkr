package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.checkout.OrderPolicyResolver;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Java 訂單在 B8 出貨重建前的運費固定為 0。期望值一律硬寫字面 0，
 * 不引用 {@code OrderConstants.CHECKOUT_SHIPPING_FEE}：用常數當期望值會讓常數與斷言一起變。
 */
class OrderCreationTxHandlerShippingFeeTest {

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (Exception ignored) {
            // 已初始化
        }
    }

    @Test
    @DisplayName("gRPC CreateOrder 路徑（createOrderInTx）：寫進 insert 的訂單運費是字面 0")
    void createOrderInTxPersistsZeroShippingFee() {
        OrderRepository orderRepository = mock(OrderRepository.class);
        OrderConfig orderConfig = mock(OrderConfig.class);
        when(orderConfig.getPaymentDeadlineSeconds()).thenReturn(259200L);
        OrderCreationTxHandler handler = new OrderCreationTxHandler(
                orderRepository,
                mock(OrderItemRepository.class),
                mock(StockTxRecordRepository.class),
                mock(OrderInvoiceSnapshotRepository.class),
                mock(OrderNoGenerator.class),
                orderConfig);

        handler.createOrderInTx(100L, 2001L,
                List.of(new OrderService.CreateOrderItemParam(
                        5001L, "Apple", 2, 500L, 500L, 0L, "key/a.jpg", 9001L, "Apple Box")),
                1000L, 1000L, "STX1", "CHK1", "3201", new OrderPolicyResolver().resolve());

        ArgumentCaptor<OrderEntity> captor = ArgumentCaptor.forClass(OrderEntity.class);
        verify(orderRepository).insert(captor.capture());
        assertThat(captor.getValue().getShippingFee()).isEqualTo(0L);
    }
}
