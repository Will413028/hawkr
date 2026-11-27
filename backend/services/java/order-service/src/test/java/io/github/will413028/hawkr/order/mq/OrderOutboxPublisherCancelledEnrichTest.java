package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.mq.dto.OrderCancelledPayload;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #1368：order.cancelled 改走 outbox 後，notification 用的 seller/buyer 反解析移到派送時 enrich。
 * 本測試坐實 {@link OrderOutboxPublisher} 派送 ORDER_CANCELLED 時，把 core payload 自帶的 store_id
 * 反解析成登入 account_id 填入 notifySellerAccountId、buyer_id 反解析成 notifyBuyerAccountId，
 * 且保留 core 的 eventId（冪等鍵穩定）。2026-08-06 Phase C 移除 sellerAccountId 舊欄位與其對應的
 * legacy-row fallback（storeId 缺失時退回讀 sellerAccountId）——見下面 fails-closed 測試記錄拆除後
 * 的降級行為。
 */
class OrderOutboxPublisherCancelledEnrichTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    @DisplayName("dispatch enrich: store_id(7001)->login(42) as notifySellerAccountId, buyer_id(3001)->login(55) "
            + "as notifyBuyerAccountId, storeId/buyerId stay raw, eventId preserved (discriminating)")
    void enrichCancelled_resolvesPrincipalsAtDispatch() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;
        long resolvedSeller = 42L;
        long resolvedBuyer = 55L;

        OrderCancelledPayload core = new OrderCancelledPayload(
                "ORD1", "TX1", List.of(new OrderCancelledPayload.StockItemPayload(java.util.UUID.fromString("0192f3a4-7b1c-7d2e-8f3a-000000000011"), "2")),
                "2026-07-22T00:00:00Z", buyerId, null, storeId, null, null, null, "SELLER_SUSPENDED");
        String coreJson = MAPPER.writeValueAsString(core);

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(1L);
        event.setAggregateId("ORD1");
        event.setEventType(OrderEventType.ORDER_CANCELLED.value());
        event.setExchange(MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
        event.setStatus(OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(coreJson);

        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.getSellerMeta(storeId)).thenThrow(new RuntimeException("meta unavailable"));
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId))).thenReturn(PrincipalResolution.allResolved(Map.of(storeId, resolvedSeller)));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyer)));

        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);

        boolean ok = publisher.publish(event);
        org.junit.jupiter.api.Assertions.assertTrue(ok, "publish should succeed");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_CANCELLED),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_CANCELLED.value()));

        OrderCancelledPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderCancelledPayload.class);
        assertEquals(resolvedSeller, sent.notifySellerAccountId(),
                "notifySellerAccountId must be resolved login account_id (42), not store_id (7001)");
        assertEquals(storeId, sent.storeId(),
                "storeId must reach the wire unchanged (7001) — it is the raw store surrogate, "
                        + "mirroring OrderPaidPayload/OrderCompletedPayload's store_id");
        assertEquals(buyerId, sent.buyerId(),
                "buyerId must stay raw buyer_id (3001)");
        assertEquals(resolvedBuyer, sent.notifyBuyerAccountId(),
                "notifyBuyerAccountId must be resolved login account_id (55)");
        assertEquals(core.eventId(), sent.eventId(),
                "eventId must be preserved from core payload (stable idempotency key across retries)");
        assertEquals("TX1", sent.stockTxId());
        assertEquals("SELLER_SUSPENDED", sent.cancelReason());
    }

    @Test
    @DisplayName("row missing store_id key entirely -> fails closed (notifySellerAccountId=0, seller meta "
            + "stays null) instead of retrying to DEAD — Phase C removed the legacy sellerAccountId->storeId "
            + "fallback; this is the accepted residual behavior if the 'old rows already drained' assumption "
            + "is ever wrong (discriminating)")
    void enrichCancelled_rowMissingStoreId_failsClosedInsteadOfCrashing() throws Exception {
        long buyerId = 3001L;
        long resolvedBuyer = 55L;

        String noStoreIdJson = """
                {"event_id":"no-store-id-evt","order_no":"ORD1","stock_tx_id":"TX1",
                 "items":[{"sku_id":"0192f3a4-7b1c-7d2e-8f3a-000000000011","quantity":"2"}],"occurred_at":"2026-07-22T00:00:00Z",
                 "buyer_id":3001,"notify_buyer_account_id":null,
                 "seller_name":null,"seller_avatar_key":null,"cancel_reason":"SELLER_SUSPENDED"}
                """;

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(2L);
        event.setAggregateId("ORD1");
        event.setEventType(OrderEventType.ORDER_CANCELLED.value());
        event.setExchange(MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
        event.setStatus(OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(noStoreIdJson);

        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyer)));

        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);

        org.junit.jupiter.api.Assertions.assertTrue(publisher.publish(event),
                "row without store_id must still publish — must not retry to DEAD");
        verify(outboxMapper).markSent(eq(2L), org.mockito.ArgumentMatchers.any());
        verify(outboxMapper, org.mockito.Mockito.never())
                .markFailedGuarded(org.mockito.ArgumentMatchers.anyLong(),
                        org.mockito.ArgumentMatchers.anyInt(), org.mockito.ArgumentMatchers.any());

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_CANCELLED),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_CANCELLED.value()));

        OrderCancelledPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderCancelledPayload.class);
        assertEquals(0L, sent.notifySellerAccountId(),
                "no store_id to resolve from -> notifySellerAccountId must fail-closed to 0, "
                        + "not throw and not silently stay some stale/wrong value");
        assertEquals(resolvedBuyer, sent.notifyBuyerAccountId(),
                "buyer-side resolution is independent of the missing store_id and must still succeed");
        assertEquals("no-store-id-evt", sent.eventId(), "eventId preserved (stable idempotency key)");
    }

    @Test
    @DisplayName("rollback-safety: a row carrying a key this version does not declare (the now-removed "
            + "seller_account_id, plus a hypothetical future notify_channel_id) must still dispatch instead "
            + "of going DEAD (discriminating)")
    void enrichCancelled_rowWithUnknownKey_stillDispatches() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;
        long resolvedSeller = 42L;
        long resolvedBuyer = 55L;

        String forwardShapedCoreJson = """
                {"event_id":"future-evt-1","order_no":"ORD1","stock_tx_id":"TX1",
                 "items":[{"sku_id":"0192f3a4-7b1c-7d2e-8f3a-000000000011","quantity":"2"}],"occurred_at":"2026-07-22T00:00:00Z",
                 "buyer_id":3001,"notify_buyer_account_id":null,"store_id":7001,
                 "seller_account_id":null,"seller_name":null,"seller_avatar_key":null,
                 "cancel_reason":"SELLER_SUSPENDED","notify_channel_id":999}
                """;

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(3L);
        event.setAggregateId("ORD1");
        event.setEventType(OrderEventType.ORDER_CANCELLED.value());
        event.setExchange(MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
        event.setStatus(OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(forwardShapedCoreJson);

        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.getSellerMeta(storeId)).thenThrow(new RuntimeException("meta unavailable"));
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId))).thenReturn(PrincipalResolution.allResolved(Map.of(storeId, resolvedSeller)));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyer)));

        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);

        org.junit.jupiter.api.Assertions.assertTrue(publisher.publish(event),
                "a row with an undeclared key must still publish — otherwise it retries to DEAD and "
                        + "product-service never restores that cancelled order's stock");
        verify(outboxMapper).markSent(eq(3L), org.mockito.ArgumentMatchers.any());
        verify(outboxMapper, org.mockito.Mockito.never())
                .markFailedGuarded(org.mockito.ArgumentMatchers.anyLong(),
                        org.mockito.ArgumentMatchers.anyInt(), org.mockito.ArgumentMatchers.any());

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_CANCELLED),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_CANCELLED.value()));

        OrderCancelledPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderCancelledPayload.class);
        assertEquals(storeId, sent.storeId(),
                "declared fields must still be read normally (store_id 7001)");
        assertEquals(resolvedSeller, sent.notifySellerAccountId(),
                "store_id must still resolve to login account_id 42, even though the input row's "
                        + "seller_account_id key is now unrecognized and ignored");
        assertEquals(resolvedBuyer, sent.notifyBuyerAccountId());
        assertEquals("future-evt-1", sent.eventId(),
                "eventId preserved (stable idempotency key)");
    }

    @Test
    @DisplayName("store_id 與 buyer_id 查無存活列（unresolved_ids）：notify 欄位帶 0，不回吐 store_id／buyer_id 冒充 account_id")
    void enrichCancelled_unresolvedPrincipalsAreNotEchoedAsAccounts() throws Exception {
        long storeId = 7001L;
        long buyerId = 3001L;
        OrderCancelledPayload core = new OrderCancelledPayload(
                "ORD1", "TX1", List.of(new OrderCancelledPayload.StockItemPayload(java.util.UUID.fromString("0192f3a4-7b1c-7d2e-8f3a-000000000011"), "2")),
                "2026-07-22T00:00:00Z", buyerId, null, storeId, null, null, null, "SELLER_SUSPENDED");

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(3L);
        event.setAggregateId("ORD1");
        event.setEventType(OrderEventType.ORDER_CANCELLED.value());
        event.setExchange(MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_CANCELLED);
        event.setStatus(OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(MAPPER.writeValueAsString(core));

        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);
        when(sellerGrpcClient.getSellerMeta(storeId)).thenThrow(new RuntimeException("meta unavailable"));
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId)))
                .thenReturn(PrincipalResolution.of(Map.of(storeId, storeId), List.of(storeId)));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.of(Map.of(), List.of(buyerId)));

        new OrderOutboxPublisher(rabbitPublisher, mock(OrderOutboxMapper.class), sellerGrpcClient, buyerGrpcClient).publish(event);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_CANCELLED),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_CANCELLED.value()));
        OrderCancelledPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderCancelledPayload.class);
        assertEquals(0L, sent.notifySellerAccountId());
        assertEquals(0L, sent.notifyBuyerAccountId());
        assertEquals(storeId, sent.storeId());
        assertEquals(buyerId, sent.buyerId());
    }
}
