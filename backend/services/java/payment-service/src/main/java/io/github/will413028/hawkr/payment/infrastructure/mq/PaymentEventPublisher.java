package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.PaymentEventType;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.util.Map;

/**
 * 支付事件發布器
 *
 * 使用 ObsRabbitPublisher 發布事件到 RabbitMQ
 * 支援多 Exchange 發布（payment.events、seller.events 等）
 */
@Component
public class PaymentEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(PaymentEventPublisher.class);

    private final ObsRabbitPublisher rabbitPublisher;
    private final ObjectMapper objectMapper;

    public PaymentEventPublisher(ObsRabbitPublisher rabbitPublisher, ObjectMapper objectMapper) {
        this.rabbitPublisher = rabbitPublisher;
        this.objectMapper = objectMapper;
    }

    /**
     * 發布領域事件
     */
    public void publish(PaymentDomainEvent event) {
        String routingKey = getRoutingKey(event.eventType());
        Map<String, Object> payload = event.payload();

        // 加入事件元資料（36 §7.4 Conformance Contract：event_id 為 UUID v4 必含欄位）
        payload = new java.util.HashMap<>(payload);
        OutboxEventFactory.applyEnvelopeMetadata(payload,
                event.eventType().value(), event.aggregateType(),
                event.aggregateId(), event.occurredAt().toString());

        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);

            rabbitPublisher.publish(
                    MqConstants.EXCHANGE_PAYMENT,
                    routingKey,
                    messageBytes,
                    event.eventType().value()
            );

            log.info("PaymentEventPublisher.publish.success", "事件發布成功", "event_type", event.eventType().value());

        } catch (Exception e) {
            log.error("PaymentEventPublisher.publish.failed", "事件發布失敗: " + event.eventType().value(), e);
            throw new RuntimeException("發布事件失敗: " + e.getMessage(), e);
        }
    }

    /**
     * 根據事件類型取得 Routing Key
     */
    private String getRoutingKey(PaymentEventType eventType) {
        return switch (eventType) {
            case PAYMENT_SUCCESS -> MqConstants.ROUTING_KEY_PAYMENT_SUCCESS;
            case PAYMENT_FAILED -> MqConstants.ROUTING_KEY_PAYMENT_FAILED;
            case PAYMENT_EXPIRED -> MqConstants.ROUTING_KEY_PAYMENT_EXPIRED;
            case PAYMENT_CLOSED -> MqConstants.ROUTING_KEY_PAYMENT_CLOSED;
            case PAYMENT_CONFLICT -> MqConstants.ROUTING_KEY_PAYMENT_CONFLICT;
            case REFUND_SUCCEEDED -> MqConstants.ROUTING_KEY_REFUND_SUCCEEDED;
            case REFUND_FAILED -> MqConstants.ROUTING_KEY_REFUND_FAILED;
            case PAYMENT_RESCUE_REFUND_INITIATED -> MqConstants.ROUTING_KEY_PAYMENT_RESCUE_REFUND_INITIATED;
            case PAYMENT_RESCUE_REFUND_COMPLETED -> MqConstants.ROUTING_KEY_PAYMENT_RESCUE_REFUND_COMPLETED;
        };
    }
}
