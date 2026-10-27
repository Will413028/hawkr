package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * PayUni Webhook MQ 配置
 *
 * 消費 payuni-webhook-service 發布到 payuni.events 的 payuni.payment.uop 事件
 *
 * DLQ 機制：主 Queue 設定 x-dead-letter-exchange / x-dead-letter-routing-key，
 * 超過最大重試次數後訊息由 Consumer reject 進入 DLQ。
 * 注意：Queue arguments 變更需刪除舊 Queue 重建。
 */
@Configuration
public class PayUniWebhookMqConfig {

    // ==================== Exchange（聲明，因為可能尚未建立） ====================

    @Bean
    public TopicExchange payuniWebhookExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYUNI_WEBHOOK, true, false);
    }

    // ==================== 主 Queue（含 DLQ 設定） ====================

    @Bean
    public Queue paymentPayuniWebhookQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_WEBHOOK)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_WEBHOOK_DLQ)
                .build();
    }

    // ==================== 主 Binding ====================

    @Bean
    public Binding bindPayuniPaymentUop(Queue paymentPayuniWebhookQueue, TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(paymentPayuniWebhookQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_PAYMENT_UOP);
    }

    /**
     * Shared membership billing ingress.  This is intentionally a separate queue from the
     * legacy one-time payment queue: recurring payloads have a different identity and must never
     * be handed to the UOP state machine.
     */
    @Bean
    public Queue paymentPayuniRecurringQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_RECURRING)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_PAYUNI_RECURRING_DLQ)
                .build();
    }

    @Bean
    public Binding bindPayuniPaymentRecurring(Queue paymentPayuniRecurringQueue,
                                               TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(paymentPayuniRecurringQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_PAYMENT_RECURRING);
    }

    @Bean
    public Queue paymentPayuniRecurringDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_RECURRING_DLQ).build();
    }

    @Bean
    public Binding paymentPayuniRecurringDlqBinding() {
        return BindingBuilder.bind(paymentPayuniRecurringDlq())
                .to(paymentWebhookDlxExchange())
                .with(MqConstants.QUEUE_PAYMENT_PAYUNI_RECURRING_DLQ);
    }

    // ==================== DLX Exchange ====================

    @Bean
    public DirectExchange paymentWebhookDlxExchange() {
        return new DirectExchange(MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX);
    }

    // ==================== DLQ Queue ====================

    @Bean
    public Queue paymentWebhookDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_WEBHOOK_DLQ).build();
    }

    // ==================== DLQ Binding ====================

    @Bean
    public Binding paymentWebhookDlqBinding() {
        return BindingBuilder.bind(paymentWebhookDlq())
                .to(paymentWebhookDlxExchange())
                .with(MqConstants.QUEUE_PAYMENT_WEBHOOK_DLQ);
    }

    // ==================== ATM 退款結果 Queue（收斂規範 §9.5）====================

    @Bean
    public Queue paymentPayuniRefundNotifyQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY_DLQ)
                .build();
    }

    @Bean
    public Binding bindPayuniRefundNotify(Queue paymentPayuniRefundNotifyQueue, TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(paymentPayuniRefundNotifyQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_REFUND_NOTIFY);
    }

    @Bean
    public Queue paymentPayuniRefundNotifyDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY_DLQ).build();
    }

    @Bean
    public Binding paymentPayuniRefundNotifyDlqBinding() {
        return BindingBuilder.bind(paymentPayuniRefundNotifyDlq())
                .to(paymentWebhookDlxExchange())
                .with(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY_DLQ);
    }

    // ==================== ATM 退款「受理」Queue（§6.6 async 副本，有 EstimatedRefundDate 無 RefundStatus）====================

    @Bean
    public Queue paymentPayuniRefundAcceptedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED_DLQ)
                .build();
    }

    @Bean
    public Binding bindPayuniRefundAccepted(Queue paymentPayuniRefundAcceptedQueue, TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(paymentPayuniRefundAcceptedQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_REFUND_ACCEPTED);
    }

    @Bean
    public Queue paymentPayuniRefundAcceptedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED_DLQ).build();
    }

    @Bean
    public Binding paymentPayuniRefundAcceptedDlqBinding() {
        return BindingBuilder.bind(paymentPayuniRefundAcceptedDlq())
                .to(paymentWebhookDlxExchange())
                .with(MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED_DLQ);
    }

    // ==================== §2.2 業務 pivot (2026-05-14) UNKNOWN webhook fallback Queue ====================
    // 與 aftersales 同 routing key 並行訂閱（各服務獨立 queue）；payment 端嘗試找 PENDING transaction stamp message

    @Bean
    public Queue paymentPayuniUnknownFallbackQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK_DLQ)
                .build();
    }

    @Bean
    public Binding bindPayuniUnknownFallback(Queue paymentPayuniUnknownFallbackQueue,
                                              TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(paymentPayuniUnknownFallbackQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_UNKNOWN_FALLBACK);
    }

    @Bean
    public Queue paymentPayuniUnknownFallbackDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK_DLQ).build();
    }

    @Bean
    public Binding paymentPayuniUnknownFallbackDlqBinding() {
        return BindingBuilder.bind(paymentPayuniUnknownFallbackDlq())
                .to(paymentWebhookDlxExchange())
                .with(MqConstants.QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK_DLQ);
    }
}
