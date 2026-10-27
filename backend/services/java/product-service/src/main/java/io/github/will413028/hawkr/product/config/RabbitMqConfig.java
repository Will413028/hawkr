package io.github.will413028.hawkr.product.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class RabbitMqConfig {

    // ==================== 消費端（來自 Order Service — 訂單取消庫存歸還）====================

    /**
     * hawkr.order 由 order-service 宣告，此處 declare=false 避免覆蓋
     */
    @Bean
    public TopicExchange orderExchange() {
        TopicExchange exchange = new TopicExchange(MqConstants.EXCHANGE_ORDER, true, false);
        exchange.setShouldDeclare(false);
        return exchange;
    }

    @Bean
    public TopicExchange productDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PRODUCT_DLX, true, false);
    }

    // ==================== 發佈端（hawkr.product 由本服務宣告）====================

    /**
     * 02 §5.13b / 36 §3.6h / 39 §5.5a / #942 — 補貨 transition event 之 exchange。
     * product-service 為 producer 端，必須自行宣告 (docs/開發規範/MQ 與事件管理規範.md §1.3)，避免 buyer-service consumer
     * 尚未啟動時的首次 publish 因 NOT_FOUND 被 broker 丟棄（review #942 ultra HIGH finding）。
     */
    @Bean
    public TopicExchange productExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PRODUCT, true, false);
    }

    @Bean
    public Queue productOrderCancelledQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_ORDER_CANCELLED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_ORDER_CANCELLED_DLQ)
                .build();
    }

    @Bean
    public Queue productOrderCancelledDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_ORDER_CANCELLED_DLQ).build();
    }

    @Bean
    public Binding bindOrderCancelled(Queue productOrderCancelledQueue, TopicExchange orderExchange) {
        return BindingBuilder.bind(productOrderCancelledQueue)
                .to(orderExchange)
                .with(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
    }

    @Bean
    public Binding bindProductOrderCancelledDlq(Queue productOrderCancelledDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productOrderCancelledDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_ORDER_CANCELLED_DLQ);
    }

    // ==================== 消費端（來自 Order Service — 付款成功銷量累計，35 設計書） ====================

    @Bean
    public Queue productOrderSalesAccumulatedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED_DLQ)
                .build();
    }

    @Bean
    public Queue productOrderSalesAccumulatedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED_DLQ).build();
    }

    @Bean
    public Binding bindOrderSalesAccumulated(Queue productOrderSalesAccumulatedQueue, TopicExchange orderExchange) {
        return BindingBuilder.bind(productOrderSalesAccumulatedQueue)
                .to(orderExchange)
                .with(MqConstants.ROUTING_KEY_ORDER_SALES_ACCUMULATED);
    }

    @Bean
    public Binding bindProductOrderSalesAccumulatedDlq(Queue productOrderSalesAccumulatedDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productOrderSalesAccumulatedDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED_DLQ);
    }

    // ==================== 消費端（來自 Buyer Service — 收藏計數同步，02 §5.14） ====================

    /**
     * hawkr.buyer exchange。
     *
     * <p>選擇由 product-service（consumer）宣告而非 buyer-service（publisher）的原因：
     * buyer-service 透過 {@code pkg/mq.Publisher.Publish} 發訊息，內部不會呼叫 {@code ExchangeDeclare}。
     * 若雙方都 {@code shouldDeclare=false}，exchange 從未建立，首批 publish 會被 RabbitMQ 丟棄（不影響 publisher
     * 的 confirm，因 confirm 只代表訊息進入 broker，但無 exchange 路由的訊息會 silently drop）。
     *
     * <p>本服務作為 consumer 端宣告 exchange + queue + binding 是 RabbitMQ 推薦做法 — consumer 啟動順序通常
     * 早於 first publish，且 consumer 一定會宣告 queue。
     */
    @Bean
    public TopicExchange buyerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_BUYER, true, false);
    }

    @Bean
    public Queue productBuyerFavoriteAddedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_ADDED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_ADDED_DLQ)
                .build();
    }

    @Bean
    public Queue productBuyerFavoriteAddedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_ADDED_DLQ).build();
    }

    @Bean
    public Binding bindBuyerFavoriteAdded(Queue productBuyerFavoriteAddedQueue, TopicExchange buyerExchange) {
        return BindingBuilder.bind(productBuyerFavoriteAddedQueue)
                .to(buyerExchange)
                .with(MqConstants.ROUTING_KEY_BUYER_FAVORITE_ADDED);
    }

    @Bean
    public Binding bindProductBuyerFavoriteAddedDlq(Queue productBuyerFavoriteAddedDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productBuyerFavoriteAddedDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_ADDED_DLQ);
    }

    @Bean
    public Queue productBuyerFavoriteRemovedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED_DLQ)
                .build();
    }

    @Bean
    public Queue productBuyerFavoriteRemovedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED_DLQ).build();
    }

    @Bean
    public Binding bindBuyerFavoriteRemoved(Queue productBuyerFavoriteRemovedQueue, TopicExchange buyerExchange) {
        return BindingBuilder.bind(productBuyerFavoriteRemovedQueue)
                .to(buyerExchange)
                .with(MqConstants.ROUTING_KEY_BUYER_FAVORITE_REMOVED);
    }

    @Bean
    public Binding bindProductBuyerFavoriteRemovedDlq(Queue productBuyerFavoriteRemovedDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productBuyerFavoriteRemovedDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED_DLQ);
    }

    // ==================== 消費端（來自 Seller Service — 賣家刪除下架商品） ====================

    /**
     * hawkr.seller exchange（既有 {@link MqConstants#EXCHANGE_SELLER_BIZ}）。
     * 由 product-service（consumer）宣告，原因同 {@link #buyerExchange()}：
     * seller-service 透過 pkg/mq.Publisher 發訊息不會 ExchangeDeclare，consumer 端宣告才能保證 exchange 存在。
     */
    @Bean
    public TopicExchange sellerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_BIZ, true, false);
    }

    @Bean
    public Queue productSellerDeletedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_SELLER_DELETED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_SELLER_DELETED_DLQ)
                .build();
    }

    @Bean
    public Queue productSellerDeletedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_SELLER_DELETED_DLQ).build();
    }

    @Bean
    public Binding bindSellerDeleted(Queue productSellerDeletedQueue, TopicExchange sellerExchange) {
        return BindingBuilder.bind(productSellerDeletedQueue)
                .to(sellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_DELETED);
    }

    @Bean
    public Binding bindProductSellerDeletedDlq(Queue productSellerDeletedDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productSellerDeletedDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_SELLER_DELETED_DLQ);
    }

    // ==================== 消費端（來自 Seller Service — 店家可曝光狀態，seller.exposure.changed） ====================

    @Bean
    public Queue productSellerExposureChangedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED_DLQ)
                .build();
    }

    @Bean
    public Queue productSellerExposureChangedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED_DLQ).build();
    }

    @Bean
    public Binding bindSellerExposureChanged(Queue productSellerExposureChangedQueue, TopicExchange sellerExchange) {
        return BindingBuilder.bind(productSellerExposureChangedQueue)
                .to(sellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_EXPOSURE_CHANGED);
    }

    @Bean
    public Binding bindProductSellerExposureChangedDlq(Queue productSellerExposureChangedDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productSellerExposureChangedDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED_DLQ);
    }

    // ==================== 消費端（來自 Review Service — 評論聚合同步，02 §5.14a，#1156） ====================

    /**
     * hawkr.review exchange。
     * 由 product-service（consumer）宣告，原因同 {@link #buyerExchange()}：
     * review-service 透過 pkg/mq.Publisher 發訊息不會 ExchangeDeclare，consumer 端宣告才能保證 exchange 存在。
     */
    @Bean
    public TopicExchange reviewExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_REVIEW, true, false);
    }

    /**
     * product.review.stats queue：單一 queue 綁定 review.created + review.hidden 兩個 routing key，
     * 由 {@link io.github.will413028.hawkr.product.mq.ReviewStatsConsumer} 消費，full-recompute 覆寫
     * product.rating / review_count（02 §5.14a）。DLQ 走 product 服務自有 DLX（與其他 consumer 一致）。
     */
    @Bean
    public Queue productReviewStatsQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_REVIEW_STATS)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_REVIEW_STATS_DLQ)
                .build();
    }

    @Bean
    public Queue productReviewStatsDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_REVIEW_STATS_DLQ).build();
    }

    @Bean
    public Binding bindProductReviewCreated(Queue productReviewStatsQueue, TopicExchange reviewExchange) {
        return BindingBuilder.bind(productReviewStatsQueue)
                .to(reviewExchange)
                .with(MqConstants.ROUTING_KEY_REVIEW_CREATED);
    }

    @Bean
    public Binding bindProductReviewHidden(Queue productReviewStatsQueue, TopicExchange reviewExchange) {
        return BindingBuilder.bind(productReviewStatsQueue)
                .to(reviewExchange)
                .with(MqConstants.ROUTING_KEY_REVIEW_HIDDEN);
    }

    @Bean
    public Binding bindProductReviewStatsDlq(Queue productReviewStatsDlq, TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productReviewStatsDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_REVIEW_STATS_DLQ);
    }

    // ==================== 消費端（來自本服務自己 — 認證背景查驗，19 §12 / 36 §3.6h.1，#2755） ====================

    /**
     * product.certification.verify queue：綁<b>自家</b> exchange {@code hawkr.product} 的
     * {@code product.certification.verify.requested}，由
     * {@link io.github.will413028.hawkr.product.mq.CertificationVerifyConsumer} 消費。
     *
     * <p>🔴 本服務現有 8 條 queue 全部消費<b>別人的</b> exchange，這是第一條「自己 publish、自己
     * consume」的 queue。dev/prod 已 passive declare（{@code SPRING_RABBITMQ_DYNAMIC=false}）
     * ⇒ 這幾個 {@code @Bean} 只是<b>意圖宣告</b>，broker 上真正建立資源的唯一 owner 是
     * {@code infra/mq/topology/topology.yaml}（Terraform）。<b>漏登記 topology ＝ 訂閱靜默失效</b>：
     * 不 crash、不進 DLQ、無告警，唯一線索是連線當下印的一行 skip。
     */
    @Bean
    public Queue productCertificationVerifyQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_CERTIFICATION_VERIFY)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_CERTIFICATION_VERIFY_DLQ)
                .build();
    }

    @Bean
    public Queue productCertificationVerifyDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_CERTIFICATION_VERIFY_DLQ).build();
    }

    @Bean
    public Binding bindProductCertificationVerify(Queue productCertificationVerifyQueue,
                                                  TopicExchange productExchange) {
        return BindingBuilder.bind(productCertificationVerifyQueue)
                .to(productExchange)
                .with(MqConstants.ROUTING_KEY_PRODUCT_CERTIFICATION_VERIFY_REQUESTED);
    }

    /**
     * 🔴 死信 binding key 必須恰為 {@code {queue}.dlq}：{@code hawkr.product.dlx} 是 topic
     * exchange，{@code x-dead-letter-routing-key} 寫成別的字面會讓死信<b>沒有任何 binding 命中
     * 而被 broker 直接丟棄</b>——plan 乾淨、apply 成功、consumers ≥ 1，只有訊息無聲消失。
     */
    @Bean
    public Binding bindProductCertificationVerifyDlq(Queue productCertificationVerifyDlq,
                                                     TopicExchange productDlxExchange) {
        return BindingBuilder.bind(productCertificationVerifyDlq)
                .to(productDlxExchange)
                .with(MqConstants.QUEUE_PRODUCT_CERTIFICATION_VERIFY_DLQ);
    }

}
