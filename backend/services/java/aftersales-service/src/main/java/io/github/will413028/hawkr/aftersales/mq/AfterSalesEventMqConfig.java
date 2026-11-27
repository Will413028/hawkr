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


    public static final String QUEUE_RETURN_STATUS = MqConstants.QUEUE_AFTERSALES_RETURN_SHIPMENT_STATUS;
    public static final String QUEUE_PROCESSING_TIMEOUT = MqConstants.QUEUE_AFTERSALES_SHIPMENT_PROCESSING_TIMEOUT;
    public static final String QUEUE_SHIPMENT_LABEL = MqConstants.QUEUE_AFTERSALES_SHIPMENT_LABEL;
    public static final String QUEUE_SHIPMENT_CREATION_FAILED = MqConstants.QUEUE_AFTERSALES_SHIPMENT_CREATION_FAILED;


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


    @Bean
    public Queue aftersalesDlq() {
        return QueueBuilder.durable("aftersales.events.dlq").build();
    }

    @Bean
    public Binding dlqBinding(Queue aftersalesDlq, TopicExchange aftersalesDlxExchange) {
        return BindingBuilder.bind(aftersalesDlq).to(aftersalesDlxExchange).with("dlq");
    }


    @Bean
    public Queue returnStatusQueue() {
        return QueueBuilder.durable(QUEUE_RETURN_STATUS)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding returnStatusBinding(Queue returnStatusQueue) {
        return BindingBuilder.bind(returnStatusQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_SHIPPING, true, false))
                .with(MqConstants.ROUTING_KEY_RETURN_SHIPMENT_STATUS_CHANGED);
    }


    @Bean
    public Queue processingTimeoutQueue() {
        return QueueBuilder.durable(QUEUE_PROCESSING_TIMEOUT)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding processingTimeoutBinding(Queue processingTimeoutQueue) {
        return BindingBuilder.bind(processingTimeoutQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_SHIPPING, true, false))
                .with(MqConstants.ROUTING_KEY_SHIPMENT_PROCESSING_TIMEOUT);
    }


    @Bean
    public Queue shipmentCreationFailedQueue() {
        return QueueBuilder.durable(QUEUE_SHIPMENT_CREATION_FAILED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding shipmentCreationFailedBinding(Queue shipmentCreationFailedQueue) {
        return BindingBuilder.bind(shipmentCreationFailedQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_ORDER, true, false))
                .with(MqConstants.ROUTING_KEY_ORDER_SHIPMENT_CREATION_FAILED);
    }


    @Bean
    public Queue shipmentLabelQueue() {
        return QueueBuilder.durable(QUEUE_SHIPMENT_LABEL)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_AFTERSALES_DLX)
                .withArgument("x-dead-letter-routing-key", "dlq")
                .build();
    }

    @Bean
    public Binding shipmentLabelBinding(Queue shipmentLabelQueue) {
        return BindingBuilder.bind(shipmentLabelQueue)
                .to(new TopicExchange(MqConstants.EXCHANGE_SHIPPING, true, false))
                .with(MqConstants.ROUTING_KEY_SHIPMENT_LABEL_UPDATED);
    }
}
