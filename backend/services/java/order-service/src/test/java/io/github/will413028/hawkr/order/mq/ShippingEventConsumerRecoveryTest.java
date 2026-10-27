package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 訂單卡 35（SHIPPING_ABNORMAL）恢復轉移守衛（03 §「35 SHIPPING_ABNORMAL 的恢復轉移」，issue #1327）
 * 與物流推進起點自 PAID(10)（issue #1644）。
 *
 * <p>驗證 onShipmentStatusChanged 對 PayUni 貨態 31（IN_TRANSIT）/ 32（WAITING_PICKUP）的
 * CAS 允許 from 集合已含 SHIPPING_ABNORMAL(35)，讓卡在 35 的訂單能被物流恢復事件推回 30/32。
 *
 * <p>#1644：allowed-from 由 {@link io.github.will413028.hawkr.order.util.OrderShippingProgress} 統一產生後含 PAID(10) 起點，
 * 驗證 shipment.delivered 與物流異常事件能自 PAID 直接推進 DELIVERED(40) / SHIPPING_ABNORMAL(35)，
 * 修復「付款後未經中間貨態即送達 / 異常時訂單卡在 PAID」的主症狀。
 */
@ExtendWith(MockitoExtension.class)
class ShippingEventConsumerRecoveryTest {

    @Mock private OrderRepository orderRepository;
    @Mock private OrderConfig orderConfig;
    @Mock private OrderEventPublisher eventPublisher;
    @Mock private OrderOutboxMapper orderOutboxMapper;
    @Mock private Channel channel;

    private ShippingEventConsumer consumer;

    @BeforeEach
    void setUp() {
        consumer = new ShippingEventConsumer(
                orderRepository, orderConfig, eventPublisher, orderOutboxMapper, new CountingTxManager());
    }

    private Message message(String json) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue("order.shipping.status");
        return new Message(json.getBytes(StandardCharsets.UTF_8), props);
    }

    @Test
    @DisplayName("PayUni 31（IN_TRANSIT）恢復守衛：允許 from 含 SHIPPING_ABNORMAL(35)，可 35→30")
    void payUni31_allowedFromContainsShippingAbnormal() throws Exception {
        when(orderRepository.updateShippingStatus(eq("O123"), eq("31"), anyString())).thenReturn(1);
        when(orderRepository.updateStatusIfMatchAny(eq("O123"), eq(OrderStatus.IN_TRANSIT.value()), anyList()))
                .thenReturn(1);

        consumer.onShipmentStatusChanged(message("{\"order_no\":\"O123\",\"status\":\"31\"}"), channel, 7L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(orderRepository).updateStatusIfMatchAny(
                eq("O123"), eq(OrderStatus.IN_TRANSIT.value()), captor.capture());
        assertThat(captor.getValue()).contains(
                OrderStatus.SHIPPING_ABNORMAL.value(),   // 35 恢復軌道
                OrderStatus.PENDING_SHIPMENT.value());   // 20 正常推進不變
        verify(channel).basicAck(7L, false);
    }

    @Test
    @DisplayName("PayUni 32（WAITING_PICKUP）恢復守衛：允許 from 含 SHIPPING_ABNORMAL(35)，可 35→32")
    void payUni32_allowedFromContainsShippingAbnormal() throws Exception {
        when(orderRepository.updateShippingStatus(eq("O456"), eq("32"), anyString())).thenReturn(1);
        when(orderRepository.updateStatusIfMatchAny(eq("O456"), eq(OrderStatus.WAITING_PICKUP.value()), anyList()))
                .thenReturn(1);

        consumer.onShipmentStatusChanged(message("{\"order_no\":\"O456\",\"status\":\"32\"}"), channel, 8L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(orderRepository).updateStatusIfMatchAny(
                eq("O456"), eq(OrderStatus.WAITING_PICKUP.value()), captor.capture());
        assertThat(captor.getValue()).contains(
                OrderStatus.SHIPPING_ABNORMAL.value(),   // 35 恢復軌道
                OrderStatus.IN_TRANSIT.value());         // 30 正常推進不變
        verify(channel).basicAck(8L, false);
    }

    @Test
    @DisplayName("shipment.delivered：allowed-from 含 PAID(10)，訂單自 PAID 直達 DELIVERED(40)（#1644 主症狀修復）")
    void onShipmentDelivered_fromPaid_advancesToDelivered() throws Exception {
        // allowed 含 PAID(10) → DB CAS（status IN(...)）命中 → rows=1，模擬訂單自 PAID 成功推進送達
        when(orderRepository.updateOrderDelivered(
                eq("O789"), eq(OrderStatus.DELIVERED.value()),
                anyString(), anyString(), any(), anyList())).thenReturn(1);

        consumer.onShipmentDelivered(message("{\"order_no\":\"O789\"}"), channel, 9L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(orderRepository).updateOrderDelivered(
                eq("O789"), eq(OrderStatus.DELIVERED.value()),
                anyString(), anyString(), any(), captor.capture());
        assertThat(captor.getValue()).contains(
                OrderStatus.PAID.value(),                 // 10 起點——#1644 修復核心（PAID 直接送達不再卡住）
                OrderStatus.SHIPPING_ABNORMAL.value());   // 35 異常恢復軌道
        // rows>0 成功路徑：order.delivered 同事務落 outbox（例外 A，不再 tx 外 best-effort 直發）+ ACK
        verify(orderOutboxMapper).insert(any());
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("PayUni 33（ABNORMAL）：allowed-from 含 PAID(10)，訂單自 PAID 推進 SHIPPING_ABNORMAL(35)")
    void onShipmentStatusChanged_isAbnormal_fromPaid_advancesToAbnormal() throws Exception {
        when(orderRepository.updateShippingStatus(eq("O777"), eq("33"), anyString())).thenReturn(1);
        when(orderRepository.updateStatusIfMatchAny(
                eq("O777"), eq(OrderStatus.SHIPPING_ABNORMAL.value()), anyList())).thenReturn(1);

        consumer.onShipmentStatusChanged(message("{\"order_no\":\"O777\",\"status\":\"33\"}"), channel, 10L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(orderRepository).updateStatusIfMatchAny(
                eq("O777"), eq(OrderStatus.SHIPPING_ABNORMAL.value()), captor.capture());
        assertThat(captor.getValue()).contains(
                OrderStatus.PAID.value(),                 // 10 起點——付款後未出貨即遇物流異常也能推進
                OrderStatus.PENDING_SHIPMENT.value());    // 20 在途主線
        assertThat(captor.getValue()).doesNotContain(
                OrderStatus.DELIVERED.value(),            // 40 已送達不被異常事件拖回
                OrderStatus.SHIPPING_ABNORMAL.value());   // 35 自身不列（防自迴圈）
        verify(channel).basicAck(10L, false);
    }
}
