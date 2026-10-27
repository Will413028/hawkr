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
            case PAYMENT_UOP     -> MqConstants.ROUTING_KEY_PAYUNI_PAYMENT_UOP;
            case PAYMENT_RECURRING -> MqConstants.ROUTING_KEY_PAYUNI_PAYMENT_RECURRING;
            case SHIPPING_STATUS -> MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS;
            // 超商列印與貨態共用 routing key，由下游按 ApiType 分流
            case SHIPPING_PRINT  -> MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS;
            // AGENT_NOTIFY 已收斂為單通道（#1757）：不經 payuni.events，改由 buildRoutes 直接發
            // seller.review.result。case 保留以維持窮舉，但不應被呼叫到。
            case AGENT_NOTIFY    -> throw new IllegalStateException(
                    "AGENT_NOTIFY 走 seller.review.result，不經 payuni.events");
            case REFUND_NOTIFY   -> MqConstants.ROUTING_KEY_PAYUNI_REFUND_NOTIFY;
            case REFUND_ACCEPTED -> MqConstants.ROUTING_KEY_PAYUNI_REFUND_ACCEPTED;
            // §2.2 業務 pivot (2026-05-14)：UNKNOWN 若 payload 含 MerID 投遞 fallback 由 aftersales/payment consumer 接手；
            // 若 payload 無 MerID 由 WebhookReceiveService 在 T2 前 markIgnored 終態，不會走到此 routing。
            case UNKNOWN -> MqConstants.ROUTING_KEY_PAYUNI_UNKNOWN_FALLBACK;
        };
    }
}
