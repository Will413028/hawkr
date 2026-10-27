package io.github.will413028.hawkr.coupon.mq;

import io.github.will413028.hawkr.coupon.service.CouponTemplateService;
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

import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

@ExtendWith(MockitoExtension.class)
class SellerUnsuspendedConsumerTest {

    @Mock CouponTemplateService templateService;
    @Mock Channel channel;
    @InjectMocks SellerUnsuspendedConsumer consumer;

    private Message msg(String body) {
        return new Message(body.getBytes(), new MessageProperties());
    }

    @Test
    @DisplayName("解除停權事件：以 store_id（非 accountId）批量還原模板 + ack — 兩者分岔時證明修對值")
    void restoreTemplatesUsesStoreIdNotAccountId() throws IOException {
        consumer.onSellerUnsuspended(
                msg("{\"event_type\":\"SELLER_UNSUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"store_id\":\"555\"}"),
                channel, 9L);

        verify(templateService).restoreTemplatesFromSuspension(555L);
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("store_id 欄位缺失 → 丟棄訊息，不呼叫 service（Phase 0 尚未上線的防呆）")
    void missingStoreIdDropsMessage() throws IOException {
        consumer.onSellerUnsuspended(
                msg("{\"event_type\":\"SELLER_UNSUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\"}"),
                channel, 9L);

        verify(templateService, never()).restoreTemplatesFromSuspension(anyLong());
        verify(channel).basicAck(9L, false);
    }
}
