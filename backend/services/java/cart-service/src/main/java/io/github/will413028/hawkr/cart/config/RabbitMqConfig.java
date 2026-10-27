package io.github.will413028.hawkr.cart.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * cart-service RabbitMQ 宣告（#1426 方案 A，cart 首個 consumer）。
 *
 * <p>消費 seller-service 發布的 {@code seller.profile.changed}（exchange {@code hawkr.seller}），
 * 由 {@link io.github.will413028.hawkr.cart.mq.SellerProfileChangedConsumer} evict {@code seller:meta:{id}} 快取。
 *
 * <p>docs/開發規範/MQ 與事件管理規範.md §1.3：每個 {@code @RabbitListener} 對應 queue 必須有對應 {@code @Bean Queue} + Binding，
 * 服務要能在全新 RabbitMQ 上自行啟動。{@code manualAckContainerFactory} 由 common-core
 * {@code RabbitMqAutoConfiguration} 於 {@code ConnectionFactory} 上 classpath（本服務已加 amqp 依賴）時自動提供。
 */
@Configuration
public class RabbitMqConfig {

    /**
     * {@code hawkr.seller} exchange。由 consumer 端宣告——seller-service 透過 pkg/mq.Publisher
     * 發訊息不會 ExchangeDeclare，consumer 宣告才能保證 exchange 存在（同 product-service 慣例，
     * 同參數重複 declare 為冪等）。
     */
    @Bean
    public TopicExchange cartSellerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_BIZ, true, false);
    }

    /** cart 自有 DLX（死信出口）。 */
    @Bean
    public TopicExchange cartDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_CART_DLX, true, false);
    }

    @Bean
    public Queue cartSellerProfileChangedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_CART_SELLER_PROFILE_CHANGED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_CART_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_CART_SELLER_PROFILE_CHANGED_DLQ)
                .build();
    }

    @Bean
    public Queue cartSellerProfileChangedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_CART_SELLER_PROFILE_CHANGED_DLQ).build();
    }

    @Bean
    public Binding bindCartSellerProfileChanged(Queue cartSellerProfileChangedQueue, TopicExchange cartSellerExchange) {
        return BindingBuilder.bind(cartSellerProfileChangedQueue)
                .to(cartSellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_PROFILE_CHANGED);
    }

    @Bean
    public Binding bindCartSellerProfileChangedDlq(Queue cartSellerProfileChangedDlq, TopicExchange cartDlxExchange) {
        return BindingBuilder.bind(cartSellerProfileChangedDlq)
                .to(cartDlxExchange)
                .with(MqConstants.QUEUE_CART_SELLER_PROFILE_CHANGED_DLQ);
    }
}
