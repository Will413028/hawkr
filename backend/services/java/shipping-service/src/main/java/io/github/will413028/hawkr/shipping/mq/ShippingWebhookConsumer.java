package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.PayUniWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.io.IOException;
import java.util.List;
import java.util.Map;

/**
 * PayUni 物流 Webhook 消費者
 *
 * 監聽 Queue: shipping.payuni.webhook
 * 來源: payuni-webhook-service 解密後轉發的物流狀態回報
 *
 * 手動 ACK，at-least-once + 冪等（dedup 表）
 */
@Component
public class ShippingWebhookConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShippingWebhookConsumer.class);
    private static final String PROCESS_STATUS_PROCESSED = "PROCESSED";
    private final ShippingService shippingService;
    private final ShippingWebhookDedupMapper dedupMapper;
    private final CvsPrintWaitingRegistry cvsPrintWaitingRegistry;

    public ShippingWebhookConsumer(ShippingService shippingService,
                                    ShippingWebhookDedupMapper dedupMapper,
                                    CvsPrintWaitingRegistry cvsPrintWaitingRegistry) {
        this.shippingService = shippingService;
        this.dedupMapper = dedupMapper;
        this.cvsPrintWaitingRegistry = cvsPrintWaitingRegistry;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK, containerFactory = "manualAckContainerFactory")
    public void onShippingWebhook(org.springframework.amqp.core.Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {

        log.info("ShippingWebhookConsumer.received", "收到物流狀態 Webhook");

        try {
            PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(message.getBody());
            java.util.Map<String, String> rawPayload = env.payload() == null
                    ? java.util.Map.of() : env.payload();

            String shipTradeNo = env.payloadField("ShipTradeNo");
            if (shipTradeNo.isEmpty()) {
                log.warn("ShippingWebhookConsumer.invalid",
                        "缺少必要欄位，shipTradeNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            java.util.Optional<InboundCapability> inbound = shippingService.inboundFor(shipTradeNo);
            if (inbound.isEmpty()) {
                shippingService.processWebhookStatusUpdate(shipTradeNo,
                        env.payloadField("ShipStatus"),
                        env.payloadField("ShipStatusDesc"),
                        env.payloadField("ShipStatusTime"));
                log.warn("ShippingWebhookConsumer.noInbound",
                        "該 provider 無回調能力，已走相容入口保存原始值",
                        "shipTradeNo", shipTradeNo);
                channel.basicAck(deliveryTag, false);
                return;
            }
            InboundCapability ingress = inbound.get();

            InboundPayload payload = InboundPayload.ofFormParams(rawPayload);

            String eventKey = ingress.dedupKey(payload);
            if (isDuplicate(eventKey)) {
                log.info("ShippingWebhookConsumer.duplicate",
                        "重複事件，跳過處理", "eventKey", eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            java.util.Optional<InboundCapability.InboundLabelEvent> labelEvent =
                    ingress.isLabelEvent(payload)
                            ? ingress.parseLabelEvent(payload)
                            : java.util.Optional.empty();

            if (labelEvent.isPresent()) {
                java.util.Map<String, String> segments = labelEvent.get().segmentCodes();
                shippingService.processWebhookPrint(shipTradeNo, segments);
                recordDedup(eventKey, shipTradeNo);

                cvsPrintWaitingRegistry.complete(shipTradeNo,
                        new CvsPrintWaitingRegistry.CvsPrintResult(
                                segments.get(LabelCapability.SEGMENT_PARTNER_ID),
                                segments.get(LabelCapability.SEGMENT_ODNO),
                                segments.get(LabelCapability.SEGMENT_VALIDATION_NO)));

                log.info("ShippingWebhookConsumer.print.success",
                        "Print Webhook 處理完成", "shipTradeNo", shipTradeNo);
            } else {
                shippingService.applyInboundStatus(shipTradeNo, ingress.parseStatusEvent(payload));
                recordDedup(eventKey, shipTradeNo);
            }

            channel.basicAck(deliveryTag, false);

            log.info("ShippingWebhookConsumer.success",
                    "Webhook 處理完成", "shipTradeNo", shipTradeNo);

        } catch (Exception e) {
            log.error("ShippingWebhookConsumer.error", "Webhook 處理失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingWebhookConsumer", e);
        }
    }

    private boolean isDuplicate(String eventKey) {
        Long count = dedupMapper.selectCount(
                new LambdaQueryWrapper<ShippingWebhookDedupEntity>()
                        .eq(ShippingWebhookDedupEntity::getEventKey, eventKey));
        return count != null && count > 0;
    }

    private void recordDedup(String eventKey, String shipTradeNo) {
        ShippingWebhookDedupEntity entity = new ShippingWebhookDedupEntity();
        entity.setEventKey(eventKey);
        entity.setShipTradeNo(shipTradeNo);
        entity.setProcessStatus(PROCESS_STATUS_PROCESSED);
        entity.setProcessedAt(DateTimeUtils.nowUtc());
        dedupMapper.insert(entity);
    }
}
