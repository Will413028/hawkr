package io.github.will413028.hawkr.payuni.webhook.event;

/**
 * PayUni Webhook 事件基底介面（sealed）
 * <p>
 * 所有 Webhook 事件類型的共同介面，使用 sealed 限制實作類別。
 */
public sealed interface PayUniWebhookEvent
        permits PaymentUopEvent, ShippingStatusEvent, ShippingPrintEvent, AgentNotifyEvent,
                RefundNotifyEvent, RefundAcceptedEvent {
}
