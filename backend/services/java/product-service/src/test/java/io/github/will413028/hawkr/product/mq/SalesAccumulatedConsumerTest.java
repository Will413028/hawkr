package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.product.mq.SalesAccumulatedConsumer.SalesAccumulator;
import io.github.will413028.hawkr.product.mq.SalesAccumulatedConsumer.SalesItem;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.service.ProductCacheService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.io.IOException;
import java.util.List;

import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * SalesAccumulatedConsumer 冪等 / 失敗路徑：
 * <ul>
 *   <li>首次事件 → 累計 + ack + evict cache</li>
 *   <li>重複事件（accumulator 回傳 false）→ ack、不 evict</li>
 *   <li>缺 order_no → reject 進 DLQ、不觸及 accumulator</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class SalesAccumulatedConsumerTest {

    private static final long TAG = 9L;

    @Mock SalesAccumulator accumulator;
    @Mock ProductCacheService cacheService;
    @Mock ProductRepository productRepository;
    @Mock Channel channel;
    @InjectMocks SalesAccumulatedConsumer consumer;

    private Message msg(String body) {
        MessageProperties props = new MessageProperties();
        props.setDeliveryTag(TAG);
        return new Message(body.getBytes(), props);
    }

    @Test
    @DisplayName("首次事件 → accumulator 累計 + ack + evict cache")
    void firstEventAckAndEvict() throws IOException {
        when(accumulator.applyAll(eq("O1001"), anyList())).thenReturn(true);
        when(productRepository.findProductSellerRefs(anyList())).thenReturn(List.of());

        consumer.onSalesAccumulated(
                msg("{\"order_no\":\"O1001\",\"items\":[{\"product_id\":100,\"quantity\":2}]}"),
                channel, TAG);

        verify(accumulator).applyAll(eq("O1001"), anyList());
        verify(cacheService).evictProductDetail("100");
        verify(channel).basicAck(TAG, false);
        verify(channel, never()).basicReject(anyLong(), anyBoolean());
    }

    @Test
    @DisplayName("重複事件（applyAll=false）→ ack、不 evict")
    void duplicateEventAckNoEvict() throws IOException {
        when(accumulator.applyAll(eq("O1001"), anyList())).thenReturn(false);

        consumer.onSalesAccumulated(
                msg("{\"order_no\":\"O1001\",\"items\":[{\"product_id\":100,\"quantity\":2}]}"),
                channel, TAG);

        verify(channel).basicAck(TAG, false);
        verifyNoInteractions(cacheService);
    }

    @Test
    @DisplayName("缺 order_no → reject 進 DLQ、不累計")
    void missingOrderNoRejectsToDlq() throws IOException {
        consumer.onSalesAccumulated(
                msg("{\"items\":[{\"product_id\":100,\"quantity\":2}]}"),
                channel, TAG);

        verify(channel).basicReject(TAG, false);
        verify(accumulator, never()).applyAll(anyString(), anyList());
        verify(channel, never()).basicAck(anyLong(), anyBoolean());
    }
}
