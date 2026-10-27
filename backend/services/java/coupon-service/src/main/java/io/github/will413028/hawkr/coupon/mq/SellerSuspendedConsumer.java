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
 * 賣家停權連動模板停用消費者（07 §5.20，36 §3.6f）。
 *
 * <p>監聽 Queue: {@code coupon.seller.suspended}（綁定 {@code hawkr.seller} /
 * {@code seller.suspended}）。把該賣家自建的 ACTIVE 模板批次停用為 INACTIVE 並標記
 * {@code disabled_by_suspension=TRUE}，供解除停權時精準還原。
 *
 * <p>失敗策略：DB 錯誤拋出 → {@link MqErrorHandler} 走 retry / DLQ。
 * Malformed payload / eventType 不符 → log Warn + ack，避免 poison。
 */
@Component
public class SellerSuspendedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerSuspendedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final CouponTemplateService templateService;

    public SellerSuspendedConsumer(CouponTemplateService templateService) {
        this.templateService = templateService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_COUPON_SELLER_SUSPENDED,
                    containerFactory = "manualAckContainerFactory")
    public void onSellerSuspended(Message message, Channel channel,
                                  @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("SellerSuspendedConsumer.received", "收到 seller.suspended 事件");

        try {
            JsonNode raw = MAPPER.readTree(body);
            SellerSuspensionEventPayload payload = SellerSuspensionEventPayload.fromJson(raw);

            if (payload == null) {
                log.warn("SellerSuspendedConsumer.payload.missing",
                        "缺少 suspensionId/accountId 或格式錯誤，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!SellerEventType.SELLER_SUSPENDED.value().equals(payload.eventType())) {
                log.warn("SellerSuspendedConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + payload.eventType());
                channel.basicAck(deliveryTag, false);
                return;
            }

            int affected = templateService.disableTemplatesBySuspension(payload.storeId());
            channel.basicAck(deliveryTag, false);

            log.info("SellerSuspendedConsumer.success",
                    "已批量停用模板，storeId=" + payload.storeId() + " affected=" + affected,
                    "storeId", String.valueOf(payload.storeId()));

        } catch (Exception e) {
            log.error("SellerSuspendedConsumer.error", "處理 seller.suspended 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "SellerSuspendedConsumer", e);
        }
    }
}
