package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * {@code N16 fulfillment.responsibility.finalized} 在 payment-service（Logistics Finance）
 * 側的訂閱意圖宣告（36 §3.3.1）。
 *
 * <p>⚠️ <b>dev/prod 已切 passive declare</b>（{@code SPRING_RABBITMQ_DYNAMIC=false}）
 * ⇒ 本檔的 {@code @Bean} 只是<b>意圖宣告</b>，broker 上的實際資源由
 * {@code infra/mq/topology/topology.yaml}（Terraform）佈建。
 * 🔴 <b>兩邊都要有</b>：漏了 topology 條目的症狀是<b>訂閱靜默失效</b>——
 * 服務不 crash、訊息不進 DLQ、不報錯（10 檔 §10.7）。
 *
 * <p>🔴 <b>DLX 用 {@code payment.events.dlx} 而不是 {@code hawkr.payment.dlx}</b>：
 * 36 §3.3.1 明訂本 queue 的 DLQ 走前者。payment-service 有兩個 DLX 並存（兩者都已佈建、
 * 各有既有 queue 掛著），憑「payment 的 DLX 應該是哪個」猜會讓死信落到另一個 exchange，
 * 而該 queue 的死信查詢會回**空的**——看起來像「沒有死信」，其實是找錯地方。
 *
 * <p>🔴 <b>DLX 跟著 consumer 服務走，不跟著來源 exchange 走</b>：本事件來自
 * {@code hawkr.shipping}，但它的死信落 payment 自己的 DLX。
 * 既有先例：{@code aftersales.payment.refund.result} 的 binding 來自 {@code payment.events}，
 * 而它的 {@code via} 是 {@code aftersales.events.dlx}。
 * 這樣每個服務才能在<b>自己的</b> DLX 上看齊自己全部的死信，
 * 而不必為了查自己的死信去翻別人的 exchange。
 */
@Configuration
public class LogisticsResponsibilityMqConfig {

    /**
     * 🔴 來源 exchange 是 {@code hawkr.shipping}，而 <b>payment-service 是它的第二個
     * publisher</b>（N16／N17 由本服務發出，因為責任兩張表由它擁有）。
     * 36 §1.4 禁止的是「同一條 <b>routing key</b> 對應多個 publisher」，不是「一個 exchange
     * 只能有一個 publisher」——既有先例 {@code hawkr.product}、
     * {@code hawkr.seller_workspace}。
     */
    @Bean
    public TopicExchange logisticsShippingExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SHIPPING, true, false);
    }

    @Bean
    public TopicExchange paymentEventsDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYMENT_EVENTS_DLX, true, false);
    }

    @Bean
    public Queue paymentFulfillmentResponsibilityFinalizedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PAYMENT_FULFILLMENT_RESPONSIBILITY_FINALIZED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_EVENTS_DLX)
                .withArgument("x-dead-letter-routing-key",
                        MqConstants.QUEUE_PAYMENT_FULFILLMENT_RESPONSIBILITY_FINALIZED_DLQ)
                .build();
    }

    @Bean
    public Binding bindPaymentFulfillmentResponsibilityFinalized(
            Queue paymentFulfillmentResponsibilityFinalizedQueue,
            TopicExchange logisticsShippingExchange) {
        return BindingBuilder.bind(paymentFulfillmentResponsibilityFinalizedQueue)
                .to(logisticsShippingExchange)
                .with(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED);
    }
}
