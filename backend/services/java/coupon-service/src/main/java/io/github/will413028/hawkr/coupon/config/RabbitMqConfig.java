package io.github.will413028.hawkr.coupon.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Coupon Service RabbitMQ 配置（07 §5.19 → W2/W3 設計書 §3.2 D6 遷移，2026-07-16）
 *
 * 訂閱 buyer-service 發布的 buyer.deleted 事件（BuyerDeletedEvent），
 * 對該 buyerId 全部 UNUSED Instance 批量作廢。取代原本掛在 account.deleted
 * 的綁定——account.deleted 只在帳號「最後一個」active role 被移除時才發，
 * 買家角色單獨被移除（帳號仍有賣家角色存活）時不會觸發，導致舊優惠券在
 * 買家重新註冊後原樣冒回來；buyer.deleted 無條件於買家角色自己的 DELETE
 * transition 發布，修正此漏洞。
 */
@Configuration
public class RabbitMqConfig {

    // ==================== 消費端：來自 buyer-service ====================

    /**
     * hawkr.buyer exchange。由 coupon-service（consumer）宣告，原因同
     * product-service 既有的 {@code buyerExchange()}：buyer-service 透過
     * {@code pkg/mq.Publisher.Publish} 發訊息，內部不會呼叫 ExchangeDeclare；
     * consumer 端宣告才能保證 exchange 存在（consumer 啟動順序通常早於
     * first publish，且 consumer 一定會宣告 queue）。
     */
    @Bean
    public TopicExchange buyerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_BUYER, true, false);
    }

    /**
     * Coupon Service 私有 DLX（與其他服務 DLX 隔離）。
     */
    @Bean
    public TopicExchange couponDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_COUPON_DLX, true, false);
    }

    @Bean
    public Queue couponBuyerDeletedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_BUYER_DELETED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_COUPON_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_COUPON_BUYER_DELETED_DLQ)
                .build();
    }

    @Bean
    public Queue couponBuyerDeletedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_BUYER_DELETED_DLQ).build();
    }

    @Bean
    public Binding bindCouponBuyerDeleted(Queue couponBuyerDeletedQueue, TopicExchange buyerExchange) {
        return BindingBuilder.bind(couponBuyerDeletedQueue)
                .to(buyerExchange)
                .with(MqConstants.ROUTING_KEY_BUYER_DELETED);
    }

    @Bean
    public Binding bindCouponBuyerDeletedDlq(Queue couponBuyerDeletedDlq, TopicExchange couponDlxExchange) {
        return BindingBuilder.bind(couponBuyerDeletedDlq)
                .to(couponDlxExchange)
                .with(MqConstants.QUEUE_COUPON_BUYER_DELETED_DLQ);
    }

    // ==================== 消費端：來自 seller-service（賣家停權連動模板停用/還原，07 §5.20 / 36 §3.6f） ====================

    /**
     * hawkr.seller exchange（既有 {@link MqConstants#EXCHANGE_SELLER_BIZ}，product-service 已宣告過）。
     * coupon-service 作為 consumer 端亦自行宣告，原因同 {@link #buyerExchange()}：
     * seller-service 透過 pkg/mq.Publisher 發訊息不會 ExchangeDeclare，consumer 端宣告才能保證 exchange 存在
     * （多個 Java consumer 各自宣告同一 topic exchange 是 RabbitMQ 冪等操作，非衝突）。
     */
    @Bean
    public TopicExchange sellerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_BIZ, true, false);
    }

    @Bean
    public Queue couponSellerSuspendedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_SELLER_SUSPENDED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_COUPON_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_COUPON_SELLER_SUSPENDED_DLQ)
                .build();
    }

    @Bean
    public Queue couponSellerSuspendedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_SELLER_SUSPENDED_DLQ).build();
    }

    @Bean
    public Binding bindCouponSellerSuspended(Queue couponSellerSuspendedQueue, TopicExchange sellerExchange) {
        return BindingBuilder.bind(couponSellerSuspendedQueue)
                .to(sellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_SUSPENDED);
    }

    @Bean
    public Binding bindCouponSellerSuspendedDlq(Queue couponSellerSuspendedDlq, TopicExchange couponDlxExchange) {
        return BindingBuilder.bind(couponSellerSuspendedDlq)
                .to(couponDlxExchange)
                .with(MqConstants.QUEUE_COUPON_SELLER_SUSPENDED_DLQ);
    }

    @Bean
    public Queue couponSellerUnsuspendedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_SELLER_UNSUSPENDED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_COUPON_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_COUPON_SELLER_UNSUSPENDED_DLQ)
                .build();
    }

    @Bean
    public Queue couponSellerUnsuspendedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_COUPON_SELLER_UNSUSPENDED_DLQ).build();
    }

    @Bean
    public Binding bindCouponSellerUnsuspended(Queue couponSellerUnsuspendedQueue, TopicExchange sellerExchange) {
        return BindingBuilder.bind(couponSellerUnsuspendedQueue)
                .to(sellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_UNSUSPENDED);
    }

    @Bean
    public Binding bindCouponSellerUnsuspendedDlq(Queue couponSellerUnsuspendedDlq, TopicExchange couponDlxExchange) {
        return BindingBuilder.bind(couponSellerUnsuspendedDlq)
                .to(couponDlxExchange)
                .with(MqConstants.QUEUE_COUPON_SELLER_UNSUSPENDED_DLQ);
    }
}
