package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class RabbitMqConfig {

    // ==================== 消費端（來自 Webhook Service）====================

    @Bean
    public TopicExchange payuniWebhookExchange() {
        // payuni.events 由 webhook-service 宣告，此處 declare=false 避免覆蓋
        TopicExchange exchange = new TopicExchange(MqConstants.EXCHANGE_PAYUNI_WEBHOOK, true, false);
        exchange.setShouldDeclare(false);
        return exchange;
    }

    @Bean
    public TopicExchange shippingWebhookDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SHIPPING_WEBHOOK_DLX, true, false);
    }

    @Bean
    public Queue shippingPayuniWebhookQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_SHIPPING_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK_DLQ)
                .build();
    }

    @Bean
    public Queue shippingPayuniWebhookDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK_DLQ).build();
    }

    @Bean
    public Binding bindPayuniShippingStatus(Queue shippingPayuniWebhookQueue, TopicExchange payuniWebhookExchange) {
        return BindingBuilder.bind(shippingPayuniWebhookQueue)
                .to(payuniWebhookExchange)
                .with(MqConstants.ROUTING_KEY_PAYUNI_SHIPPING_STATUS);
    }

    @Bean
    public Binding bindShippingWebhookDlq(Queue shippingPayuniWebhookDlq, TopicExchange shippingWebhookDlxExchange) {
        return BindingBuilder.bind(shippingPayuniWebhookDlq)
                .to(shippingWebhookDlxExchange)
                .with(MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK_DLQ);
    }

    // ==================== 發布端（通知 Order Service）====================

    @Bean
    public TopicExchange shippingEventsExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SHIPPING, true, false);
    }

    @Bean
    public TopicExchange shippingDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SHIPPING_DLX, true, false);
    }

    // ==================== 消費端（來自 Order Service）====================

    @Bean
    public TopicExchange orderExchange() {
        // hawkr.order 由 order-service 宣告，此處 declare=false 避免覆蓋
        TopicExchange exchange = new TopicExchange(MqConstants.EXCHANGE_ORDER, true, false);
        exchange.setShouldDeclare(false);
        return exchange;
    }

    @Bean
    public Queue shippingOrderReceiptConfirmedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_SHIPPING_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED_DLQ)
                .build();
    }

    @Bean
    public Queue shippingOrderReceiptConfirmedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED_DLQ).build();
    }

    @Bean
    public Binding bindOrderReceiptConfirmed(Queue shippingOrderReceiptConfirmedQueue, TopicExchange orderExchange) {
        return BindingBuilder.bind(shippingOrderReceiptConfirmedQueue)
                .to(orderExchange)
                .with(MqConstants.ROUTING_KEY_ORDER_RECEIPT_CONFIRMED);
    }

    @Bean
    public Binding bindShippingOrderReceiptConfirmedDlq(Queue shippingOrderReceiptConfirmedDlq, TopicExchange shippingWebhookDlxExchange) {
        return BindingBuilder.bind(shippingOrderReceiptConfirmedDlq)
                .to(shippingWebhookDlxExchange)
                .with(MqConstants.QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED_DLQ);
    }

    // ==================== 消費端（來自 Order Service — 付款成功自動建單）====================

    @Bean
    public Queue shippingOrderPaidQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_ORDER_PAID)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_SHIPPING_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_SHIPPING_ORDER_PAID_DLQ)
                .build();
    }

    @Bean
    public Queue shippingOrderPaidDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_ORDER_PAID_DLQ).build();
    }

    @Bean
    public Binding bindOrderPaid(Queue shippingOrderPaidQueue, TopicExchange orderExchange) {
        return BindingBuilder.bind(shippingOrderPaidQueue)
                .to(orderExchange)
                .with(MqConstants.ROUTING_KEY_ORDER_PAID);
    }

    @Bean
    public Binding bindShippingOrderPaidDlq(Queue shippingOrderPaidDlq, TopicExchange shippingWebhookDlxExchange) {
        return BindingBuilder.bind(shippingOrderPaidDlq)
                .to(shippingWebhookDlxExchange)
                .with(MqConstants.QUEUE_SHIPPING_ORDER_PAID_DLQ);
    }

    // ==================== 消費端（來自 Order Service — Backstop 重試建單，03 §8.11）====================

    @Bean
    public Queue shippingShipmentCreateRequestedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_SHIPPING_WEBHOOK_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED_DLQ)
                .build();
    }

    @Bean
    public Queue shippingShipmentCreateRequestedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED_DLQ).build();
    }

    @Bean
    public Binding bindShipmentCreateRequested(Queue shippingShipmentCreateRequestedQueue, TopicExchange orderExchange) {
        return BindingBuilder.bind(shippingShipmentCreateRequestedQueue)
                .to(orderExchange)
                .with(MqConstants.ROUTING_KEY_ORDER_SHIPMENT_CREATE_REQUESTED);
    }

    @Bean
    public Binding bindShippingShipmentCreateRequestedDlq(Queue shippingShipmentCreateRequestedDlq, TopicExchange shippingWebhookDlxExchange) {
        return BindingBuilder.bind(shippingShipmentCreateRequestedDlq)
                .to(shippingWebhookDlxExchange)
                .with(MqConstants.QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED_DLQ);
    }

}
