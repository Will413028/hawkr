package io.github.will413028.hawkr.coupon.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.SellerEventType;
import io.github.will413028.hawkr.coupon.mq.dto.SellerSuspensionEventPayload;
import io.github.will413028.hawkr.coupon.service.CouponTemplateService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 賣家解除停權模板還原消費者（07 §5.20，36 §3.6f）。
 *
 * <p>監聽 Queue: {@code coupon.seller.unsuspended}（綁定 {@code hawkr.seller} /
 * {@code seller.unsuspended}）。只還原 {@code disabled_by_suspension=TRUE} 的模板，
 * 不誤還原賣家本來就自行停用 / 被 admin 停用的模板。
 */
@Component
public class SellerUnsuspendedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerUnsuspendedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final CouponTemplateService templateService;

    public SellerUnsuspendedConsumer(CouponTemplateService templateService) {
        this.templateService = templateService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_COUPON_SELLER_UNSUSPENDED,
                    containerFactory = "manualAckContainerFactory")
    public void onSellerUnsuspended(Message message, Channel channel,
                                    @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("SellerUnsuspendedConsumer.received", "收到 seller.unsuspended 事件");

        try {
            JsonNode raw = MAPPER.readTree(body);
            SellerSuspensionEventPayload payload = SellerSuspensionEventPayload.fromJson(raw);

            if (payload == null) {
                log.warn("SellerUnsuspendedConsumer.payload.missing",
                        "缺少 suspensionId/accountId 或格式錯誤，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!SellerEventType.SELLER_UNSUSPENDED.value().equals(payload.eventType())) {
                log.warn("SellerUnsuspendedConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + payload.eventType());
                channel.basicAck(deliveryTag, false);
                return;
            }

            int affected = templateService.restoreTemplatesFromSuspension(payload.storeId());
            channel.basicAck(deliveryTag, false);

            log.info("SellerUnsuspendedConsumer.success",
                    "已還原模板，storeId=" + payload.storeId() + " affected=" + affected,
                    "storeId", String.valueOf(payload.storeId()));

        } catch (Exception e) {
            log.error("SellerUnsuspendedConsumer.error", "處理 seller.unsuspended 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "SellerUnsuspendedConsumer", e);
        }
    }
}
