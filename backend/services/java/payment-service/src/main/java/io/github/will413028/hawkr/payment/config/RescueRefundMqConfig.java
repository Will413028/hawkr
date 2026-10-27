package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * §2A.6 Rescue 退款 MQ 配置。
 * <p>
 * 監聽 payment.events / payment.rescue.refund_initiated。
 * 由 RescueRefundConsumer 處理：對副 row 呼叫 PayUni close API 退款。
 */
@Configuration
public class RescueRefundMqConfig {

    @Bean
    public TopicExchange paymentEventsExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYMENT, true, false);
    }

    @Bean
    public Queue paymentRescueRefundQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_RESCUE_REFUND)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PAYMENT_RESCUE_REFUND_DLQ)
                .build();
    }

    @Bean
    public Binding bindPaymentRescueRefund(Queue paymentRescueRefundQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(paymentRescueRefundQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_RESCUE_REFUND_INITIATED);
    }

    @Bean
    public TopicExchange paymentDlxExchange() {
        // 既有 hawkr.payment.dlx 是 topic，沿用避免 inequivalent type 衝突
        return new TopicExchange(MqConstants.EXCHANGE_PAYMENT_DLX, true, false);
    }

    @Bean
    public Queue paymentRescueRefundDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_RESCUE_REFUND_DLQ).build();
    }

    @Bean
    public Binding bindPaymentRescueRefundDlq(Queue paymentRescueRefundDlq, TopicExchange paymentDlxExchange) {
        return BindingBuilder.bind(paymentRescueRefundDlq)
                .to(paymentDlxExchange)
                .with(MqConstants.QUEUE_PAYMENT_RESCUE_REFUND_DLQ);
    }
}
