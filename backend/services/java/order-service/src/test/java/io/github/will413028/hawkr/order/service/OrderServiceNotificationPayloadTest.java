package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.dto.OrderCancelledPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderCompletedPayload;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import io.github.will413028.hawkr.order.entity.StockTxRecordEntity;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class OrderServiceNotificationPayloadTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @org.junit.jupiter.api.BeforeAll
    static void initTsid() {
        // completeOrderInTx 內建撥款任務會 TsidUtil.generate()
        try {
            io.github.will413028.hawkr.common.utils.TsidUtil.init(300, 0);
        } catch (IllegalStateException ignored) {
            // 已由其他測試初始化
        }
    }

    @Test
    @DisplayName("insertCancelledOutbox writes ORDER_CANCELLED core payload to outbox in-tx: "
            + "storeId holds raw store_id, notify fields deferred to dispatch-time "
            + "enrich, buyerId stays raw buyer_id (#1368)")
    void insertCancelledOutbox_writesCorePayload() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;

        OrderEntity order = new OrderEntity();
        order.setOrderNo("ORD1");
        order.setBuyerId(buyerId);
        order.setStoreId(storeId);
        order.setCancelReason("SELLER_SUSPENDED");

        StockTxRecordEntity deductRecord = new StockTxRecordEntity();
        deductRecord.setStockTxId("TX1");
        deductRecord.setStatus(io.github.will413028.hawkr.order.constant.OrderConstants.STOCK_TX_STATUS_SUCCESS);
        deductRecord.setPayloadSnapshot("[]");

        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.findByOrderNo("ORD1")).thenReturn(order);
        StockTxRecordRepository stockTxRecordRepository = mock(StockTxRecordRepository.class);
        when(stockTxRecordRepository.findDeductByOrderNo("ORD1")).thenReturn(deductRecord);

        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);
        OrderEventPublisher eventPublisher = mock(OrderEventPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);

        OrderService service = OrderServiceTestHarness.withCollaborators(
                orderRepository, stockTxRecordRepository, sellerGrpcClient, buyerGrpcClient,
                eventPublisher, outboxMapper);

        service.insertCancelledOutbox("ORD1");

        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        OrderOutboxEntity outbox = captor.getValue();
        assertEquals("order.cancelled", outbox.getRoutingKey(),
                "outbox routing key must be order.cancelled");
        assertEquals("ORD1", outbox.getAggregateId());

        OrderCancelledPayload core = MAPPER.readValue(outbox.getPayload(), OrderCancelledPayload.class);
        assertEquals(storeId, core.storeId(),
                "core payload storeId holds the raw store_id (7001) — dispatch-time enrichCancelled "
                        + "reads it to resolve the login account_id");
        assertEquals(buyerId, core.buyerId(),
                "core payload buyerId must stay raw buyer_id (3001)");
        assertNull(core.notifyBuyerAccountId(),
                "notifyBuyerAccountId deferred to dispatch-time enrich (null at insert)");
        assertEquals("TX1", core.stockTxId());
        assertEquals("SELLER_SUSPENDED", core.cancelReason());
    }

    /**
     * issue #2294 B8：{@code order.completed} 從「tx 外 best-effort 直發」改成「tx 內落 order_outbox」。
     *
     * <p>本測試取代原本兩支對 {@code OrderService.completeOrder} 的
     * {@code verify(eventPublisher).publishOrderCompleted(...)} 斷言——那條路徑已經不存在。
     * 這裡改驗 tx 內寫出的 <b>core payload</b>：{@code storeId} / {@code buyerId} 維持原始業務鍵，
     * notify 欄位留 null 交給派送時 enrich（與 {@code order.cancelled} 同形）。
     */
    @Test
    @DisplayName("completeOrderInTx writes order.completed core payload to outbox in-tx: storeId/buyerId "
            + "stay raw, notify fields deferred to dispatch-time enrich (#2294 B8)")
    void completeOrderInTx_writesCompletedCorePayload() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;

        OrderEntity order = new OrderEntity();
        order.setOrderNo("ORD1");
        order.setBuyerId(buyerId);
        order.setStoreId(storeId);
        order.setTotalActualAmount(1000L);

        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.findByOrderNo("ORD1")).thenReturn(order);
        when(orderRepository.completeDeliveredOrder(eq("ORD1"), eq("MANUAL"), any())).thenReturn(1);

        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        OrderCompletionTxHandler handler = new OrderCompletionTxHandler(
                orderRepository,
                mock(io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository.class),
                mock(OrderEventPublisher.class),
                outboxMapper);

        assertTrue(handler.completeOrderInTx("ORD1", "MANUAL", null));

        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper, org.mockito.Mockito.times(2)).insert(captor.capture());
        List<OrderOutboxEntity> rows = captor.getAllValues();

        OrderOutboxEntity completedRow = rows.stream()
                .filter(r -> "order.completed".equals(r.getRoutingKey()))
                .findFirst()
                .orElseThrow(() -> new AssertionError("order.completed outbox row missing"));
        assertEquals("ORD1", completedRow.getAggregateId());

        OrderCompletedPayload core = MAPPER.readValue(completedRow.getPayload(), OrderCompletedPayload.class);
        assertEquals(String.valueOf(storeId), core.storeId(),
                "core payload storeId must stay the raw store_id (7001) — crm stats and "
                        + "GetOrderAggregates reconcile key by store_id, not login account_id");
        assertEquals(String.valueOf(buyerId), core.buyerId(),
                "core payload buyerId must stay the raw buyer_id (3001)");
        assertNull(core.notifySellerAccountId(),
                "notifySellerAccountId deferred to dispatch-time enrich (null at insert)");
        assertNull(core.notifyBuyerAccountId(),
                "notifyBuyerAccountId deferred to dispatch-time enrich (null at insert)");
        assertEquals("MANUAL", core.confirmType());

        OrderOutboxEntity receiptRow = rows.stream()
                .filter(r -> "order.receipt.confirmed".equals(r.getRoutingKey()))
                .findFirst()
                .orElseThrow(() -> new AssertionError("order.receipt.confirmed outbox row missing"));
        assertEquals("ORD1", receiptRow.getAggregateId());
    }

    /**
     * issue #2294 B8 鑑別力測試：CAS 回 0（另一個 pod 已完成該訂單）⇒ <b>一筆 outbox 都不寫</b>。
     *
     * <p>沒有這一支，把 {@code if (rows == 0) return false;} 拿掉也不會有測試轉紅
     * —— 那會讓兩個 pod 各發一次 {@code order.completed}／{@code order.receipt_confirmed}。
     */
    @Test
    @DisplayName("completeOrderInTx: CAS returns 0 (another pod won) → no outbox row at all (#2294 B8)")
    void completeOrderInTx_casLost_writesNoOutbox() {
        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.completeDeliveredOrder(eq("ORD1"), eq("AUTO"), any())).thenReturn(0);

        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        OrderCompletionTxHandler handler = new OrderCompletionTxHandler(
                orderRepository,
                mock(io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository.class),
                mock(OrderEventPublisher.class),
                outboxMapper);

        org.junit.jupiter.api.Assertions.assertFalse(handler.completeOrderInTx("ORD1", "AUTO", null));

        verify(outboxMapper, org.mockito.Mockito.never()).insert(any());
        verify(orderRepository, org.mockito.Mockito.never()).findByOrderNo(any());
    }

    /**
     * issue #2294 B8：{@code order.completed} 的身分反解析改在<b>派送時</b>做（tx 內零 gRPC）。
     *
     * <p>對照組是 {@code OrderOutboxPublisherCancelledEnrichTest}——這裡驗
     * {@code OrderOutboxPublisher} 對 ORDER_COMPLETED 也做同樣的 enrich：
     * {@code storeId(7001) → notifySellerAccountId(42)}、{@code buyerId(3001) → notifyBuyerAccountId(55)}，
     * 而 {@code storeId} / {@code buyerId} 原值不動、{@code eventId} 保留。
     */
    @Test
    @DisplayName("dispatch enrich for order.completed: store_id(7001)->login(42), buyer_id(3001)->login(55), "
            + "raw keys unchanged, eventId preserved (#2294 B8)")
    void dispatchEnrich_completedResolvesPrincipals() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;

        OrderCompletedPayload core = new OrderCompletedPayload(
                "ORD1", String.valueOf(buyerId), null, String.valueOf(storeId), null,
                "1000", "AUTO", "2026-08-08T00:00:00Z");

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(9L);
        event.setAggregateId("ORD1");
        event.setEventType(io.github.will413028.hawkr.common.utils.code.OrderEventType.ORDER_COMPLETED.value());
        event.setExchange(io.github.will413028.hawkr.common.mq.MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(io.github.will413028.hawkr.common.mq.MqConstants.ROUTING_KEY_ORDER_COMPLETED);
        event.setStatus(io.github.will413028.hawkr.common.utils.code.OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(MAPPER.writeValueAsString(core));

        io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher rabbitPublisher =
                mock(io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId))).thenReturn(PrincipalResolution.allResolved(Map.of(storeId, 42L)));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, 55L)));

        io.github.will413028.hawkr.order.mq.OrderOutboxPublisher publisher =
                new io.github.will413028.hawkr.order.mq.OrderOutboxPublisher(
                        rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);

        assertTrue(publisher.publish(event));

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(
                eq(io.github.will413028.hawkr.common.mq.MqConstants.EXCHANGE_ORDER),
                eq(io.github.will413028.hawkr.common.mq.MqConstants.ROUTING_KEY_ORDER_COMPLETED),
                bodyCaptor.capture(),
                eq(io.github.will413028.hawkr.common.utils.code.OrderEventType.ORDER_COMPLETED.value()));

        OrderCompletedPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderCompletedPayload.class);
        assertEquals("42", sent.notifySellerAccountId(),
                "notifySellerAccountId must be the resolved login account_id (42), not store_id");
        assertEquals("55", sent.notifyBuyerAccountId(),
                "notifyBuyerAccountId must be the resolved login account_id (55), not buyer_id");
        assertEquals(String.valueOf(storeId), sent.storeId(), "storeId must reach the wire unchanged");
        assertEquals(String.valueOf(buyerId), sent.buyerId(), "buyerId must reach the wire unchanged");
        assertEquals(core.eventId(), sent.eventId(),
                "eventId must be preserved from core payload (stable idempotency key across retries)");
    }
}
