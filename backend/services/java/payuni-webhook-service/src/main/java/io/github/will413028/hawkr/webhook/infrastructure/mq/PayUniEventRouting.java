package io.github.will413028.hawkr.webhook.infrastructure.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;


/**
 * PayUni Webhook 事件 → MQ exchange / routing key 對應。
 *
 * 此類別承載 MQ 拓樸決策（屬於 payuni-webhook-service 內部關注），
 * 與 CodeMeta enum 解耦（CodeMeta enum 不得含業務輔助方法，見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public final class PayUniEventRouting {

    private PayUniEventRouting() {}

    public static String exchange(PayUniEventType type) {
        return MqConstants.EXCHANGE_PAYUNI_WEBHOOK;
    }

    public static String topic(PayUniEventType type) {
        return switch (type) {
            case SHIPPING_STATUS -> MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS;
            case SHIPPING_PRINT  -> MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS;
            case AGENT_NOTIFY    -> throw new IllegalStateException(
                    "AGENT_NOTIFY 走 seller.review.result，不經 payuni.events");
            case UNKNOWN         -> throw new IllegalStateException(
                    "UNKNOWN 由 WebhookReceiveService 標 IGNORED，不投遞 MQ");
        };
    }
}
