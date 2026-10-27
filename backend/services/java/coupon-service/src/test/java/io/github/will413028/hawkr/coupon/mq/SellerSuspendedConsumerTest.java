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

/**
 * Plan C Phase 1 Task 2：停權連動停用模板必須用事件的 store_id，不是 accountId
 * （帳號重開店後兩者會分岔）。比照 {@code BuyerDeletedConsumerTest} 慣例：
 * 純 JUnit5 + MockitoExtension，直接呼叫 consumer 方法。
 */
@ExtendWith(MockitoExtension.class)
class SellerSuspendedConsumerTest {

    @Mock CouponTemplateService templateService;
    @Mock Channel channel;
    @InjectMocks SellerSuspendedConsumer consumer;

    private Message msg(String body) {
        return new Message(body.getBytes(), new MessageProperties());
    }

    @Test
    @DisplayName("停權事件：以 store_id（非 accountId）批量停用模板 + ack — 兩者分岔時證明修對值")
    void disableTemplatesUsesStoreIdNotAccountId() throws IOException {
        consumer.onSellerSuspended(
                msg("{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\",\"store_id\":\"555\"}"),
                channel, 9L);

        verify(templateService).disableTemplatesBySuspension(555L);
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("store_id 欄位缺失 → 丟棄訊息，不呼叫 service（Phase 0 尚未上線的防呆）")
    void missingStoreIdDropsMessage() throws IOException {
        consumer.onSellerSuspended(
                msg("{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\"}"),
                channel, 9L);

        verify(templateService, never()).disableTemplatesBySuspension(anyLong());
        verify(channel).basicAck(9L, false);
    }
}
