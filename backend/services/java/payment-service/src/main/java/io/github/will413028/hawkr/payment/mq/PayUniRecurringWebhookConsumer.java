package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.PayUniWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import io.github.will413028.hawkr.payment.application.service.MembershipChargeResultApplicationService;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.RecurringWebhookNormalizer;
import com.rabbitmq.client.Channel;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

/**
 * Consumes the dedicated PayUni recurring route. It never invokes the legacy UOP handler and it
 * ACKs only after the charge-attempt transaction (and authoritative outbox insert) commits.
 */
@Component
public final class PayUniRecurringWebhookConsumer {

    private static final ObsLogger log = ObsLogger.forClass(PayUniRecurringWebhookConsumer.class);

    private final RecurringWebhookNormalizer normalizer;
    private final MembershipChargeResultApplicationService chargeService;

    public PayUniRecurringWebhookConsumer(RecurringWebhookNormalizer normalizer,
                                          MembershipChargeResultApplicationService chargeService) {
        this.normalizer = normalizer;
        this.chargeService = chargeService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_PAYUNI_RECURRING,
            containerFactory = "manualAckContainerFactory")
    public void onRecurring(Message message, Channel channel,
                            @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        try {
            PayUniWebhookEnvelope envelope = PayUniWebhookEnvelope.parse(message.getBody());
            Map<String, String> payload = envelope.payload();
            if (payload == null || payload.isEmpty()) {
                throw new IllegalArgumentException("recurring webhook payload is empty");
            }
            if (!envelope.webhookType().isBlank()
                    && !PayUniEventType.PAYMENT_RECURRING.value().equals(envelope.webhookType())) {
                throw new IllegalArgumentException("message is not a PAYMENT_RECURRING webhook");
            }

            String providerEventKey = envelope.eventKey().isBlank()
                    ? fallbackEventKey(payload)
                    : envelope.eventKey();
            ProviderChargeResult result = normalizer.normalize(payload, envelope.eventId(), providerEventKey);
            MembershipChargeResultApplicationService.ApplyResult applied = chargeService.apply(result);

            log.info("PayUniRecurringWebhookConsumer.process.success",
                    "PayUni recurring webhook 已正規化並完成 charge-attempt adjudication",
                    "provider_event_key", applied.decisionType().name() + ":" + providerEventKey);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("PayUniRecurringWebhookConsumer.process.failed",
                    "處理 PayUni recurring webhook 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "PayUniRecurringWebhookConsumer", e);
        }
    }

    private static String fallbackEventKey(Map<String, String> payload) {
        return String.join(":",
                payload.getOrDefault("MerID", ""),
                payload.getOrDefault("PeriodTradeNo", ""),
                payload.getOrDefault("TradeNo", ""),
                payload.getOrDefault("ThisPeriod", ""),
                payload.getOrDefault("TradeStatus", ""));
    }
}
