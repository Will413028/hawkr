package io.github.will413028.hawkr.order.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * RabbitMQ 配置
 *
 * 訂單服務消費：payment.events Exchange 的 payment.success / payment.expired / refund.succeeded / refund.failed
 * 含 DLQ 配置：毒訊息超過重試上限後進入死信佇列
 */
@Configuration
public class RabbitMqConfig {

    // ==================== Exchange ====================

    @Bean
    public TopicExchange paymentEventsExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYMENT, true, false);
    }

    @Bean
    public TopicExchange paymentDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYMENT_DLX, true, false);
    }

    // ==================== Queue（含 DLQ 配置） ====================

    @Bean
    public Queue orderPaymentResultQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_PAYMENT_RESULT)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_PAYMENT_RESULT_DLQ)
                .build();
    }

    @Bean
    public Queue orderRefundResultQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_REFUND_RESULT)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PAYMENT_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_REFUND_RESULT_DLQ)
                .build();
    }

    @Bean
    public Queue orderPaymentResultDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_PAYMENT_RESULT_DLQ).build();
    }

    @Bean
    public Queue orderRefundResultDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_REFUND_RESULT_DLQ).build();
    }

    // ==================== Binding ====================

    @Bean
    public Binding bindPaymentSuccess(Queue orderPaymentResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderPaymentResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_SUCCESS);
    }

    @Bean
    public Binding bindPaymentExpired(Queue orderPaymentResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderPaymentResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_EXPIRED);
    }

    @Bean
    public Binding bindPaymentFailed(Queue orderPaymentResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderPaymentResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_FAILED);
    }

    @Bean
    public Binding bindPaymentClosed(Queue orderPaymentResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderPaymentResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_CLOSED);
    }

    @Bean
    public Binding bindPaymentConflict(Queue orderPaymentResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderPaymentResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_PAYMENT_CONFLICT);
    }

    @Bean
    public Binding bindRefundSucceeded(Queue orderRefundResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderRefundResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED);
    }

    @Bean
    public Binding bindRefundFailed(Queue orderRefundResultQueue, TopicExchange paymentEventsExchange) {
        return BindingBuilder.bind(orderRefundResultQueue)
                .to(paymentEventsExchange)
                .with(MqConstants.ROUTING_KEY_REFUND_FAILED);
    }

    // ==================== Order DLX ====================

    @Bean
    public TopicExchange orderDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_ORDER_DLX, true, false);
    }

    // ==================== Order Exchange（發布確認收貨事件用）====================

    @Bean
    public TopicExchange orderExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_ORDER, true, false);
    }

    /**
     * 賣家工作區 realtime exchange（[B-207] #1181）。
     * order-service publish seller.order.updated；gateway SSE 消費。宣告確保
     * 全新 RabbitMQ 上 producer publish 前 exchange 已存在（與 gateway 端宣告 idempotent）。
     */
    @Bean
    public TopicExchange sellerWorkspaceExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_WORKSPACE, true, false);
    }

    // ==================== AfterSales Events ====================

    @Bean
    public TopicExchange aftersalesExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_AFTERSALES, true, false);
    }

    @Bean
    public Queue orderAftersalesRefundCompletedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED_DLQ)
                .build();
    }

    @Bean
    public Queue orderAftersalesCancelCompletedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED_DLQ)
                .build();
    }

    @Bean
    public Queue orderAftersalesAwaitingBuyerPageQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE_DLQ)
                .build();
    }

    @Bean
    public Binding bindAftersalesRefundCompleted(Queue orderAftersalesRefundCompletedQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesRefundCompletedQueue)
                .to(aftersalesExchange)
                .with(AfterSalesEventType.REFUND_COMPLETED.value());
    }

    @Bean
    public Binding bindAftersalesCancelCompleted(Queue orderAftersalesCancelCompletedQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCancelCompletedQueue)
                .to(aftersalesExchange)
                .with(AfterSalesEventType.CANCEL_COMPLETED.value());
    }

    @Bean
    public Binding bindAftersalesAwaitingBuyerPage(Queue orderAftersalesAwaitingBuyerPageQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesAwaitingBuyerPageQueue)
                .to(aftersalesExchange)
                .with(AfterSalesEventType.AWAITING_BUYER_PAGE.value());
    }

    // 60h 自動完成倒數的暫停/恢復事件（03 §5.8）
    @Bean
    public Queue orderAftersalesCountdownQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_COUNTDOWN)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_AFTERSALES_COUNTDOWN_DLQ)
                .build();
    }

    @Bean
    public Binding bindAftersalesCountdownCaseApplied(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_APPLIED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownAppealSubmitted(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.APPEAL_SUBMITTED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownAutoEscalated(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_AUTO_ESCALATED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownCaseRejected(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_REJECTED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownAutoRejected(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_AUTO_REJECTED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownCancelledByBuyer(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_CANCELLED_BY_BUYER.value());
    }

    @Bean
    public Binding bindAftersalesCountdownCouponCompensated(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_COUPON_COMPENSATED.value());
    }

    @Bean
    public Binding bindAftersalesCountdownPlatformDecisionMade(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.PLATFORM_DECISION_MADE.value());
    }

    // aftersales.refund.completed 不再 binding 本 queue（ORD-02 雙 queue 收斂，03 §5.7.1）：
    // 倒數恢復已併入 AfterSalesEventConsumer.onRefundCompleted 同事務處理。
    // 注意：移除 @Bean 不會刪 broker 上既存 binding，需維運手動移除（見 36 號）。

    /**
     * 2026-06-15 §5.3.8 待寄回逾時自動關案（status 20→33）→ order-service 嘗試恢復倒數。
     * 與 cancelled_by_buyer / auto_rejected 同性質（案件已終結 + 無退款）。
     */
    @Bean
    public Binding bindAftersalesCountdownAutoCompletedReturnPendingTimeout(Queue orderAftersalesCountdownQueue, TopicExchange aftersalesExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownQueue).to(aftersalesExchange).with(AfterSalesEventType.CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.value());
    }

    // ==================== AfterSales DLQ ====================

    @Bean
    public Queue orderAftersalesRefundCompletedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED_DLQ).build();
    }

    @Bean
    public Queue orderAftersalesCancelCompletedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED_DLQ).build();
    }

    @Bean
    public Queue orderAftersalesAwaitingBuyerPageDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE_DLQ).build();
    }

    @Bean
    public Queue orderAftersalesCountdownDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_AFTERSALES_COUNTDOWN_DLQ).build();
    }

    @Bean
    public Binding bindOrderAftersalesRefundCompletedDlq(Queue orderAftersalesRefundCompletedDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderAftersalesRefundCompletedDlq).to(orderDlxExchange).with(MqConstants.QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED_DLQ);
    }

    @Bean
    public Binding bindOrderAftersalesCancelCompletedDlq(Queue orderAftersalesCancelCompletedDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderAftersalesCancelCompletedDlq).to(orderDlxExchange).with(MqConstants.QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED_DLQ);
    }

    @Bean
    public Binding bindOrderAftersalesAwaitingBuyerPageDlq(Queue orderAftersalesAwaitingBuyerPageDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderAftersalesAwaitingBuyerPageDlq).to(orderDlxExchange).with(MqConstants.QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE_DLQ);
    }

    @Bean
    public Binding bindOrderAftersalesCountdownDlq(Queue orderAftersalesCountdownDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderAftersalesCountdownDlq).to(orderDlxExchange).with(MqConstants.QUEUE_ORDER_AFTERSALES_COUNTDOWN_DLQ);
    }

    // ==================== Review Events ====================

    @Bean
    public TopicExchange reviewExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_REVIEW, true, false);
    }

    @Bean
    public Queue orderReviewCreatedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_REVIEW_CREATED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_REVIEW_CREATED_DLQ)
                .build();
    }

    @Bean
    public Queue orderReviewCreatedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_REVIEW_CREATED_DLQ).build();
    }

    @Bean
    public Binding bindReviewCreated(Queue orderReviewCreatedQueue, TopicExchange reviewExchange) {
        return BindingBuilder.bind(orderReviewCreatedQueue)
                .to(reviewExchange)
                .with(MqConstants.ROUTING_KEY_REVIEW_CREATED);
    }

    @Bean
    public Binding bindOrderReviewCreatedDlq(Queue orderReviewCreatedDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderReviewCreatedDlq).to(orderDlxExchange).with(MqConstants.QUEUE_ORDER_REVIEW_CREATED_DLQ);
    }

    // ==================== 消費端（來自 Seller Service — 賣家停權連動批次取消未付款單，03 §5.1a.2 / 36 §3.6f） ====================

    /**
     * hawkr.seller exchange（既有 {@link MqConstants#EXCHANGE_SELLER_BIZ}，product-service 已宣告過）。
     * order-service 作為 consumer 端亦自行宣告，原因同 product-service 註解：
     * seller-service 透過 pkg/mq.Publisher 發訊息不會 ExchangeDeclare，consumer 端宣告才能保證 exchange 存在
     * （多個 Java consumer 各自宣告同一 topic exchange 是 RabbitMQ 冪等操作，非衝突）。
     */
    @Bean
    public TopicExchange sellerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER_BIZ, true, false);
    }

    @Bean
    public Queue orderSellerSuspendedQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_SELLER_SUSPENDED)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_ORDER_DLX)
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_ORDER_SELLER_SUSPENDED_DLQ)
                .build();
    }

    @Bean
    public Queue orderSellerSuspendedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_ORDER_SELLER_SUSPENDED_DLQ).build();
    }

    @Bean
    public Binding bindSellerSuspended(Queue orderSellerSuspendedQueue, TopicExchange sellerExchange) {
        return BindingBuilder.bind(orderSellerSuspendedQueue)
                .to(sellerExchange)
                .with(MqConstants.ROUTING_KEY_SELLER_SUSPENDED);
    }

    @Bean
    public Binding bindOrderSellerSuspendedDlq(Queue orderSellerSuspendedDlq, TopicExchange orderDlxExchange) {
        return BindingBuilder.bind(orderSellerSuspendedDlq)
                .to(orderDlxExchange)
                .with(MqConstants.QUEUE_ORDER_SELLER_SUSPENDED_DLQ);
    }

    // ==================== DLQ Binding ====================

    @Bean
    public Binding bindPaymentResultDlq(Queue orderPaymentResultDlq, TopicExchange paymentDlxExchange) {
        return BindingBuilder.bind(orderPaymentResultDlq)
                .to(paymentDlxExchange)
                .with(MqConstants.QUEUE_ORDER_PAYMENT_RESULT_DLQ);
    }

    @Bean
    public Binding bindRefundResultDlq(Queue orderRefundResultDlq, TopicExchange paymentDlxExchange) {
        return BindingBuilder.bind(orderRefundResultDlq)
                .to(paymentDlxExchange)
                .with(MqConstants.QUEUE_ORDER_REFUND_RESULT_DLQ);
    }

}
