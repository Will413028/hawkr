package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderExpiryCancellationService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.io.IOException;
import java.util.Collections;

import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 賣家停權批次取消掃描必須用事件的 store_id，不是 accountId（帳號重開店後兩者會分岔）——
 * 比照 coupon-service 同名測試（Plan C Phase 1 Task 2 已修過的同款 bug，order-service 當時漏掉）。
 */
@ExtendWith(MockitoExtension.class)
class SellerSuspendedConsumerTest {

    @Mock OrderRepository orderRepository;
    @Mock OrderExpiryCancellationService orderExpiryCancellationService;
    @Mock Channel channel;

    private SellerSuspendedConsumer consumer;

    private Message msg(String body) {
        return new Message(body.getBytes(), new MessageProperties());
    }

    @Test
    @DisplayName("停權事件：以 store_id（非 accountId）掃描候選訂單 + ack — 兩者分岔時證明修對值")
    void scanUsesStoreIdNotAccountId() throws IOException {
        consumer = new SellerSuspendedConsumer(orderRepository, orderExpiryCancellationService);
        when(orderRepository.findPendingPaymentOrderNosBySeller(eq(555L), anyString()))
                .thenReturn(Collections.emptyList());

        consumer.onSellerSuspended(
                msg("{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\",\"store_id\":\"555\"}"),
                channel, 9L);

        verify(orderRepository).findPendingPaymentOrderNosBySeller(eq(555L), anyString());
        verify(orderRepository, never()).findPendingPaymentOrderNosBySeller(eq(100L), anyString());
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("store_id 欄位缺失 → 丟棄訊息，不呼叫 repository（比照 coupon-service 的 fail-closed 防呆）")
    void missingStoreIdDropsMessage() throws IOException {
        consumer = new SellerSuspendedConsumer(orderRepository, orderExpiryCancellationService);

        consumer.onSellerSuspended(
                msg("{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\"}"),
                channel, 9L);

        verify(orderRepository, never()).findPendingPaymentOrderNosBySeller(anyLong(), anyString());
        verify(channel).basicAck(9L, false);
    }
}
