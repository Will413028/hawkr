package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.mq.dto.OrderDeliveredPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * BuyerGrpcClient.resolvePrincipals 有 3 個真實生產程式碼呼叫點：
 * OrderOutboxPublisher.enrichCancelled（order.cancelled，已由
 * {@link OrderOutboxPublisherCancelledEnrichTest} 覆蓋）、OrderService.completeOrder
 * （order.completed，已由 OrderServiceNotificationPayloadTest 覆蓋）、以及本檔覆蓋的另兩條：
 * OrderOutboxPublisher.enrich（order.paid dispatch-time enrich）與
 * ShippingEventConsumer.onShipmentDelivered（order.delivered，buyer_id 一律查自己 order
 * 記錄取得，不採信上游 shipping-service payload 同名欄位）。
 *
 * <p>兩個測試都驅動真正的生產方法（publisher.publish / consumer.onShipmentDelivered），
 * 斷言實際派送出去的 payload 欄位值，而非只驗證 BuyerGrpcClient mock 自己的 stub 設定
 * （diverged buyer_id -> 不同的 login account_id，證明 resolve 步驟真的被呼叫到）。
 */
class OrderNotifyBuyerFieldTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    @DisplayName("OrderOutboxPublisher dispatch enrich: order.paid buyer_id(3001)->login(55) resolves "
            + "into notify_buyer_account_id, buyerId stays raw (discriminating)")
    void enrichOrderPaid_resolvesNotifyBuyerAccountId() throws Exception {
        long buyerId = 3001L;
        long resolvedBuyer = 55L;
        long storeId = 7001L;

        // core payload：dispatch 前尚未 enrich，notify 欄位為 null（鏡像 order.paid 初次落 outbox 的形狀）
        OrderPaidPayload core = new OrderPaidPayload(
                "ORD1", "HOME_DELIVERY", "2026-08-01T00:00:00Z",
                buyerId, null, storeId, 1000L,
                null, null, null, null,
                "corr-notify-test", "9001");
        String coreJson = MAPPER.writeValueAsString(core);

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(1L);
        event.setAggregateId("ORD1");
        event.setEventType(OrderEventType.ORDER_PAID.value());
        event.setExchange(MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_PAID);
        event.setStatus(OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(coreJson);

        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        // seller/buyer meta 皆 best-effort：拋例外驗證不阻斷派送（sellerName/buyerNickname 留 null）
        when(sellerGrpcClient.getSellerMeta(storeId)).thenThrow(new RuntimeException("meta unavailable"));
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId)))
                .thenThrow(new RuntimeException("meta unavailable"));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyer)));

        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);

        boolean ok = publisher.publish(event);
        assertTrue(ok, "publish should succeed");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_PAID),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_PAID.value()));

        OrderPaidPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderPaidPayload.class);
        assertEquals(buyerId, sent.buyerId(),
                "buyerId must stay raw buyer_id (3001)");
        assertEquals(resolvedBuyer, sent.notifyBuyerAccountId(),
                "notifyBuyerAccountId must be resolved login account_id (55), not the raw buyer_id echoed back");
    }

    @Test
    @DisplayName("order.delivered 端到端: consumer 落 outbox 帶 raw buyer_id(3001)、dispatch enrich 解成 login(55) "
            + "填進 notify_buyer_account_id (discriminating)")
    void onShipmentDelivered_resolvesNotifyBuyerAccountId() throws Exception {
        long buyerId = 3001L;
        long resolvedBuyer = 55L;

        OrderRepository orderRepository = mock(OrderRepository.class);
        when(orderRepository.updateOrderDelivered(
                eq("O789"), eq(OrderStatus.DELIVERED.value()), any(), any(), any(), anyList()))
                .thenReturn(1);
        when(orderRepository.findBuyerIdByOrderNo("O789")).thenReturn(buyerId);

        OrderConfig orderConfig = mock(OrderConfig.class);
        OrderEventPublisher eventPublisher = mock(OrderEventPublisher.class);
        OrderOutboxMapper outboxMapper = mock(OrderOutboxMapper.class);

        ShippingEventConsumer consumer = new ShippingEventConsumer(
                orderRepository, orderConfig, eventPublisher, outboxMapper, new CountingTxManager());

        MessageProperties props = new MessageProperties();
        Message message = new Message("{\"order_no\":\"O789\"}".getBytes(StandardCharsets.UTF_8), props);
        Channel channel = mock(Channel.class);

        consumer.onShipmentDelivered(message, channel, 1L);

        // ① consumer 側：事務內寫進 outbox 的 core payload 帶 raw buyer_id、notify 欄位留白
        ArgumentCaptor<OrderOutboxEntity> outboxCaptor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(outboxCaptor.capture());
        OrderOutboxEntity written = outboxCaptor.getValue();
        assertEquals(OrderEventType.ORDER_DELIVERED.value(), written.getEventType());
        OrderDeliveredPayload core = MAPPER.readValue(written.getPayload(), OrderDeliveredPayload.class);
        assertEquals(String.valueOf(buyerId), core.buyerId(),
                "buyerId must come from own order record's buyer_id (3001), "
                        + "not any external shipping-service payload field");
        assertNull(core.notifyBuyerAccountId(),
                "consumer 的短事務內不得有 gRPC —— notify 欄位必須留給 dispatch 時 enrich");
        verify(channel).basicAck(1L, false);

        // ② dispatch 側：enrichDelivered 把 buyer_id 反解成登入 account_id
        ObsRabbitPublisher rabbitPublisher = mock(ObsRabbitPublisher.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyer)));
        written.setId(2L);
        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, mock(SellerGrpcClient.class), buyerGrpcClient);

        assertTrue(publisher.publish(written), "publish should succeed");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(eq(MqConstants.EXCHANGE_ORDER), eq(MqConstants.ROUTING_KEY_ORDER_DELIVERED),
                bodyCaptor.capture(), eq(OrderEventType.ORDER_DELIVERED.value()));
        OrderDeliveredPayload sent = MAPPER.readValue(bodyCaptor.getValue(), OrderDeliveredPayload.class);
        assertEquals(String.valueOf(buyerId), sent.buyerId(), "buyerId must stay raw buyer_id (3001)");
        assertEquals(String.valueOf(resolvedBuyer), sent.notifyBuyerAccountId(),
                "notifyBuyerAccountId must be resolved login account_id (55), not the raw buyer_id echoed back");
        assertEquals(core.eventId(), sent.eventId(),
                "enrich 必須保留 core 的 eventId（跨重試冪等鍵穩定）");
    }
}
