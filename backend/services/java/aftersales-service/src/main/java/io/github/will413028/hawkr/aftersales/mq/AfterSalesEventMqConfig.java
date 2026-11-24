package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * 售後服務 MQ 拓撲配置。
 * 定義 Exchange、Queue、Binding，包含 DLQ。
 */
@Configuration
public class AfterSalesEventMqConfig {

    // ==================== Consumer Queue 常數（引用 MqConstants）====================

    public static final String QUEUE_REFUND_RESULT = MqConstants.QUEUE_AFTERSALES_REFUND_RESULT;

    // ==================== Exchange Beans ====================

    @Bean
    public TopicExchange aftersalesExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_AFTERSALES, true, false);
    }

    @Bean
    public TopicExchange aftersalesDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_AFTERSALES_DLX, true, false);
    }

    /**
     * 賣家工作區 realtime exchange（[B-207] #1181）。
     * aftersales-service publish seller.after_sale.updated；gateway SSE 消費。宣告確保全新
     * RabbitMQ 上 producer publish 前 exchange 已存在（與 gateway 端宣告 idempotent）。
     */
    @Bean
    public TopicExchange sellerWorkspaceExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_WORKSPACE, true, false);
    }

    // ==================== DLQ ====================

    @Bean
    public Queue aftersalesDlq() {
        return QueueBuilder.durable("aftersales.events.dlq").build();
    }

    @Bean
    public Binding dlqBinding(Queue aftersalesDlq, TopicExchange aftersalesDlxExchange) {
        return BindingBuilder.bind(aftersalesDlq).to(aftersalesDlxExchange).with("dlq");
    }

    // ==================== Consumer Queue: 退款結果（綁定 succeeded + failed）====================

    @Bean
    public Queue refundResultQueue() {
        return QueueBuilder.durable(QUEUE_REFUND_RESULT)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding refundSucceededBinding(Queue refundResultQueue) {
        return BindingBuilder.bind(refundResultQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_PAYMENT, true, false))
                .with(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED);
    }

    @Bean
    public Binding refundFailedBinding(Queue refundResultQueue) {
        return BindingBuilder.bind(refundResultQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_PAYMENT, true, false))
                .with(MqConstants.ROUTING_KEY_REFUND_FAILED);
    }

    // ==================== Consumer Queue: §2.2 業務 pivot (2026-05-14) PayUni UNKNOWN webhook fallback ====================
    // 綁定 payuni.events / payuni.unknown.fallback，與 payment-service 同 routing key 並行訂閱（各服務獨立 queue）。
    // 用途：UNKNOWN webhook 含 MerID 時，aftersales 嘗試找最近 awaiting refund task，找到則退回 5 stamp message。

    @Bean
    public Queue aftersalesPayuniUnknownFallbackQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_AFTERSALES_PAYUNI_UNKNOWN_FALLBACK)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding aftersalesPayuniUnknownFallbackBinding(Queue aftersalesPayuniUnknownFallbackQueue) {
        return BindingBuilder.bind(aftersalesPayuniUnknownFallbackQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_PAYUNI_WEBHOOK, true, false))
                .with(MqConstants.ROUTING_KEY_PAYUNI_UNKNOWN_FALLBACK);
    }
}
