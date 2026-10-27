package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.github.will413028.hawkr.order.repository.RefundRequestRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 測試專用工廠：建構一個窄範圍可用的 {@link OrderService} 實例。
 *
 * {@code OrderService} 建構子參數多達 18 個；本類別只暴露呼叫方真正需要控制的
 * {@code orderRepository}，其餘 collaborator 一律 mock（本測試範圍不會用到，
 * 對 getOrderForShipping 這類窄方法而言是 null-safe 的 no-op stub）。
 */
final class OrderServiceTestHarness {

    private OrderServiceTestHarness() {
    }

    static OrderService withOrderRepository(OrderRepository orderRepository) {
        return withOrderRepository(orderRepository, mock(AfterSalesGrpcClient.class));
    }

    /**
     * 測試專用工廠：額外開放 {@code afterSalesGrpcClient} 可控。
     * {@code getSellerOrderStatusCounts} 必須先取得售後分類才會往下打 repository，
     * 未 stub 的 mock 回 null 會在 {@code cls.activeOrderNos()} NPE。
     */
    static OrderService withOrderRepository(OrderRepository orderRepository,
                                            AfterSalesGrpcClient afterSalesGrpcClient) {
        return new OrderService(
                orderRepository,
                mock(OrderItemRepository.class),
                mock(RefundRequestRepository.class),
                mock(FinancialLogRepository.class),
                mock(StockTxRecordRepository.class),
                mock(CartCleanupRetryTaskRepository.class),
                mock(AutoRefundTaskRepository.class),
                mock(OrderNoGenerator.class),
                mock(ProductGrpcClient.class),
                mock(CartGrpcClient.class),
                mock(OrderConfig.class),
                mock(OrderEventPublisher.class),
                mock(SellerGrpcClient.class),
                mock(BuyerGrpcClient.class),
                afterSalesGrpcClient,
                mock(OrderCompletionTxHandler.class),
                mock(OrderCreationTxHandler.class),
                mock(OrderOutboxMapper.class),
                new io.github.will413028.hawkr.order.checkout.OrderPolicyResolver(),
                new io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard(),
                new SimpleMeterRegistry()
        );
    }

    /**
     * 測試專用工廠：額外開放 {@code orderRepository} / {@code stockTxRecordRepository} /
     * {@code sellerGrpcClient} / {@code eventPublisher} 四個 collaborator 可控（Plan C Phase 2
     * notification-service reverse-resolve 測試需要），其餘一律 mock（{@code buyerGrpcClient}
     * 為預設 no-op mock）。
     */
    static OrderService withCollaborators(
            OrderRepository orderRepository,
            StockTxRecordRepository stockTxRecordRepository,
            SellerGrpcClient sellerGrpcClient,
            OrderEventPublisher eventPublisher
    ) {
        return withCollaborators(orderRepository, stockTxRecordRepository, sellerGrpcClient,
                mock(BuyerGrpcClient.class), eventPublisher);
    }

    /**
     * 測試專用工廠：額外開放 {@code orderRepository} / {@code stockTxRecordRepository} /
     * {@code sellerGrpcClient} / {@code buyerGrpcClient} / {@code eventPublisher} 五個
     * collaborator 可控（buyer_id 8-service rekey Phase 1 notify_buyer_account_id 測試需要），
     * 其餘一律 mock。
     */
    static OrderService withCollaborators(
            OrderRepository orderRepository,
            StockTxRecordRepository stockTxRecordRepository,
            SellerGrpcClient sellerGrpcClient,
            BuyerGrpcClient buyerGrpcClient,
            OrderEventPublisher eventPublisher
    ) {
        return withCollaborators(orderRepository, stockTxRecordRepository, sellerGrpcClient,
                buyerGrpcClient, eventPublisher, mock(OrderOutboxMapper.class));
    }

    /**
     * 測試專用工廠：額外開放 {@code orderOutboxMapper} 可控（#1368 order.cancelled 改走 outbox，
     * insertCancelledOutbox 測試需驗 core payload 寫入 outbox）。
     */
    static OrderService withCollaborators(
            OrderRepository orderRepository,
            StockTxRecordRepository stockTxRecordRepository,
            SellerGrpcClient sellerGrpcClient,
            BuyerGrpcClient buyerGrpcClient,
            OrderEventPublisher eventPublisher,
            OrderOutboxMapper orderOutboxMapper
    ) {
        // completeOrder() 走 orderCompletionTxHandler.completeOrderInTx 才會往下發 MQ 事件；
        // 預設 stub 為 true，讓呼叫 completeOrder(...) 的測試能到達本次要驗證的通知 payload 組裝邏輯，
        // 不因未 stub 的 mock 預設回傳 false 而提早短路。
        OrderCompletionTxHandler orderCompletionTxHandler = mock(OrderCompletionTxHandler.class);
        when(orderCompletionTxHandler.completeOrderInTx(any(), any(), any())).thenReturn(true);

        return new OrderService(
                orderRepository,
                mock(OrderItemRepository.class),
                mock(RefundRequestRepository.class),
                mock(FinancialLogRepository.class),
                stockTxRecordRepository,
                mock(CartCleanupRetryTaskRepository.class),
                mock(AutoRefundTaskRepository.class),
                mock(OrderNoGenerator.class),
                mock(ProductGrpcClient.class),
                mock(CartGrpcClient.class),
                mock(OrderConfig.class),
                eventPublisher,
                sellerGrpcClient,
                buyerGrpcClient,
                mock(AfterSalesGrpcClient.class),
                orderCompletionTxHandler,
                mock(OrderCreationTxHandler.class),
                orderOutboxMapper,
                new io.github.will413028.hawkr.order.checkout.OrderPolicyResolver(),
                new io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard(),
                new SimpleMeterRegistry()
        );
    }
}
