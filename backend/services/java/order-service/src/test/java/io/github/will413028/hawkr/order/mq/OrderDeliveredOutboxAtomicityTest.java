package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code order.delivered} 的**例外 A**（同事務 outbox）原子性回歸。
 *
 * <p>修復前的形狀：{@code updateOrderDelivered} CAS auto-commit ⇒ 之後才走
 * {@code OrderEventPublisher.publish}（吞掉所有例外）。MQ 短暫不可用時「訂單已 DELIVERED、
 * 事件沒送出」；而重投時 {@code OrderShippingProgress.allowedFromFor(DELIVERED)} 不含
 * DELIVERED 自身 ⇒ CAS rows=0 冪等 skip ⇒ 送達通知與 {@code order.delivered} 的其他
 * 消費者<b>永久收不到</b>，且零告警。
 */
class OrderDeliveredOutboxAtomicityTest {

    private OrderRepository orderRepository;
    private OrderOutboxMapper outboxMapper;
    private OrderEventPublisher eventPublisher;
    private CountingTxManager txManager;
    private ShippingEventConsumer consumer;
    private Channel channel;

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        outboxMapper = mock(OrderOutboxMapper.class);
        eventPublisher = mock(OrderEventPublisher.class);
        txManager = new CountingTxManager();
        channel = mock(Channel.class);
        consumer = new ShippingEventConsumer(
                orderRepository, mock(OrderConfig.class), eventPublisher, outboxMapper, txManager);
    }

    private Message message(String orderNo) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue(MqConstants.QUEUE_ORDER_SHIPPING_DELIVERED);
        return new Message(("{\"order_no\":\"" + orderNo + "\"}").getBytes(StandardCharsets.UTF_8), props);
    }

    @Test
    @DisplayName("CAS 命中 → 同一個 tx 內落 order_outbox（PENDING），且 tx commit")
    void casHit_writesOutboxInSameTransaction() throws Exception {
        when(orderRepository.updateOrderDelivered(
                eq("O1"), eq(OrderStatus.DELIVERED.value()), anyString(), anyString(), any(), anyList()))
                .thenReturn(1);
        when(orderRepository.findBuyerIdByOrderNo("O1")).thenReturn(3001L);

        consumer.onShipmentDelivered(message("O1"), channel, 1L);

        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        OrderOutboxEntity row = captor.getValue();
        assertThat(row.getEventType()).isEqualTo(OrderEventType.ORDER_DELIVERED.value());
        assertThat(row.getExchange()).isEqualTo(MqConstants.EXCHANGE_ORDER);
        assertThat(row.getRoutingKey()).isEqualTo(MqConstants.ROUTING_KEY_ORDER_DELIVERED);
        assertThat(row.getStatus()).isEqualTo(OutboxEventStatus.PENDING.value());
        assertThat(row.getAggregateId()).isEqualTo("O1");

        assertThat(txManager.beginCount).as("CAS + outbox 必須在同一個 tx 內").isEqualTo(1);
        assertThat(txManager.commitCount).isEqualTo(1);
        assertThat(txManager.rollbackCount).isEqualTo(0);
        verify(channel).basicAck(1L, false);
    }

    @Test
    @DisplayName("outbox INSERT 失敗 → 整個 tx rollback（狀態不得單獨落地）")
    void outboxInsertFailure_rollsBackStatusChange() throws Exception {
        when(orderRepository.updateOrderDelivered(
                eq("O2"), eq(OrderStatus.DELIVERED.value()), anyString(), anyString(), any(), anyList()))
                .thenReturn(1);
        when(orderRepository.findBuyerIdByOrderNo("O2")).thenReturn(3001L);
        doThrow(new RuntimeException("simulated outbox insert failure"))
                .when(outboxMapper).insert(any());

        consumer.onShipmentDelivered(message("O2"), channel, 2L);

        assertThat(txManager.rollbackCount)
                .as("outbox 寫不進去時，DELIVERED 狀態必須跟著 rollback")
                .isEqualTo(1);
        assertThat(txManager.commitCount).isEqualTo(0);
        // best-effort 的賣家 realtime 也不該在失敗路徑上發出去
        verify(eventPublisher, never()).emitSellerOrderUpdated(
                any(), anyString(), anyString(), any(), any(), any());
    }

    @Test
    @DisplayName("CAS rows=0（冪等重投）→ 不寫 outbox、不發 realtime")
    void casMiss_writesNothing() throws Exception {
        when(orderRepository.updateOrderDelivered(
                eq("O3"), eq(OrderStatus.DELIVERED.value()), anyString(), anyString(), any(), anyList()))
                .thenReturn(0);

        consumer.onShipmentDelivered(message("O3"), channel, 3L);

        verify(outboxMapper, never()).insert(any());
        verify(eventPublisher, never()).emitSellerOrderUpdated(
                any(), anyString(), anyString(), any(), any(), any());
        verify(channel).basicAck(3L, false);
    }

    @Test
    @DisplayName("tx 內零 gRPC：consumer 不再自己反解 buyer principal（改由 dispatch 時 enrich）")
    void noGrpcInsideTransaction() throws Exception {
        when(orderRepository.updateOrderDelivered(
                eq("O4"), eq(OrderStatus.DELIVERED.value()), anyString(), anyString(), any(), anyList()))
                .thenReturn(1);
        when(orderRepository.findBuyerIdByOrderNo("O4")).thenReturn(3001L);

        consumer.onShipmentDelivered(message("O4"), channel, 4L);

        // 建構子已不再收 BuyerGrpcClient（編譯層保證），這裡再從行為面確認：
        // 落進 outbox 的 core payload 的 notify 欄位為 null ⇒ 沒有在 tx 內做過反解。
        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        assertThat(captor.getValue().getPayload())
                .as("notify_buyer_account_id 必須留給 OrderOutboxPublisher.enrichDelivered")
                .contains("\"notify_buyer_account_id\":null");
    }
}
