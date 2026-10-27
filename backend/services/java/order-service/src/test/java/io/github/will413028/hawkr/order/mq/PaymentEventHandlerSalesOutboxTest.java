package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.dto.OrderSalesAccumulatedPayload;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.PaymentEventDedupRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code order.sales.accumulated} 改走**例外 A**（與 {@code order.paid} 同一事務落 order_outbox）。
 *
 * <p>修復前的形狀：dedup INSERT 在事務內（正確），但 sales.accumulated 走
 * {@code afterCommit} ＋ {@code OrderEventPublisher.publish}（吞掉所有例外）
 * ⇒ commit 之後 publish 失敗、事件遺失，而同一則 {@code payment.success} 重投一律被 dedup 擋掉
 * ⇒ {@code product.sales_count} <b>永久少算</b>，且 docs/開發規範/後端服務責任邊界規範.md 禁止寫 reconciliation job ⇒ 無救濟。
 *
 * <p>鑑別力：把 sales.accumulated 改回 {@code afterCommit + publishOrderSalesAccumulated}
 * ⇒ outbox 只會被 insert 一次（order.paid）⇒ 本檔第一支測試轉紅。
 */
class PaymentEventHandlerSalesOutboxTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private OrderRepository orderRepository;
    private OrderItemRepository orderItemRepository;
    private PaymentEventDedupRepository dedupRepository;
    private OrderOutboxMapper outboxMapper;
    private PaymentEventHandler handler;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (IllegalStateException ignored) {
            // 已初始化
        }
    }

    @BeforeEach
    void setUp() {
        orderRepository = mock(OrderRepository.class);
        orderItemRepository = mock(OrderItemRepository.class);
        dedupRepository = mock(PaymentEventDedupRepository.class);
        outboxMapper = mock(OrderOutboxMapper.class);
        handler = new PaymentEventHandler(
                orderRepository,
                orderItemRepository,
                dedupRepository,
                mock(FinancialLogRepository.class),
                mock(AutoRefundTaskRepository.class),
                mock(CouponConsumptionTaskRepository.class),
                mock(OrderEventPublisher.class),
                mock(PaymentGrpcClient.class),
                outboxMapper,
                mock(io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository.class),
                mock(OrderService.class),
                new io.github.will413028.hawkr.common.obs.context.ObsContext());
    }

    private JsonNode payload() throws Exception {
        return MAPPER.readTree("{\"trade_no\":\"T1\",\"order_id\":\"O1\",\"amount\":1000,"
                + "\"pay_time\":\"2026-08-08 10:00:00\",\"payment_type\":\""
                + PaymentMethod.CREDIT_CARD.value() + "\"}");
    }

    private void stubPaidOrder() {
        when(dedupRepository.insertIdempotent(eq("T1"), eq("O1"), anyString(), anyString())).thenReturn(1);
        OrderEntity order = new OrderEntity();
        order.setOrderNo("O1");
        order.setStatus(OrderStatus.PENDING_PAYMENT.value());
        order.setBuyerId(3001L);
        order.setStoreId(7001L);
        order.setTotalActualAmount(1000L);
        order.setShippingMethod("3001");
        when(orderRepository.findByOrderNoForUpdate("O1")).thenReturn(order);
        when(orderRepository.updatePaymentConfirmed(
                eq("O1"), anyString(), anyString(), anyString(), any(), anyString())).thenReturn(1);
    }

    private OrderItemEntity item(long productId, int qty) {
        OrderItemEntity it = new OrderItemEntity();
        it.setOrderNo("O1");
        it.setProductId(productId);
        it.setQuantity(qty);
        return it;
    }

    @Test
    @DisplayName("付款成功事務內落兩筆 outbox：order.paid ＋ order.sales.accumulated（不再走 afterCommit best-effort）")
    void salesAccumulatedGoesThroughOutboxInSameTransaction() throws Exception {
        stubPaidOrder();
        when(orderItemRepository.findByOrderNo("O1"))
                .thenReturn(List.of(item(9001L, 2), item(9001L, 1), item(9002L, 5)));

        handler.handlePaymentSuccess(payload(), "{}", "hash1");

        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper, org.mockito.Mockito.times(2)).insert(captor.capture());

        List<OrderOutboxEntity> rows = captor.getAllValues();
        assertThat(rows).extracting(OrderOutboxEntity::getEventType)
                .containsExactly(OrderEventType.ORDER_PAID.value(),
                        OrderEventType.ORDER_SALES_ACCUMULATED.value());

        OrderOutboxEntity sales = rows.get(1);
        assertThat(sales.getExchange()).isEqualTo(MqConstants.EXCHANGE_ORDER);
        assertThat(sales.getRoutingKey()).isEqualTo(MqConstants.ROUTING_KEY_ORDER_SALES_ACCUMULATED);
        assertThat(sales.getStatus()).isEqualTo(OutboxEventStatus.PENDING.value());
        assertThat(sales.getAggregateId()).isEqualTo("O1");

        // payload 形狀不變：publisher 端已 group by productId
        OrderSalesAccumulatedPayload parsed =
                MAPPER.readValue(sales.getPayload(), OrderSalesAccumulatedPayload.class);
        assertThat(parsed.orderNo()).isEqualTo("O1");
        assertThat(parsed.eventId()).isNotBlank();
        assertThat(parsed.items())
                .extracting(OrderSalesAccumulatedPayload.ProductSalesItem::productId,
                        OrderSalesAccumulatedPayload.ProductSalesItem::quantity)
                .containsExactlyInAnyOrder(
                        org.assertj.core.groups.Tuple.tuple(9001L, 3),
                        org.assertj.core.groups.Tuple.tuple(9002L, 5));
    }

    @Test
    @DisplayName("無訂單明細 → 只落 order.paid，不落空的 sales.accumulated")
    void noItems_writesOnlyPaidOutbox() throws Exception {
        stubPaidOrder();
        when(orderItemRepository.findByOrderNo("O1")).thenReturn(List.of());

        handler.handlePaymentSuccess(payload(), "{}", "hash2");

        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        assertThat(captor.getValue().getEventType()).isEqualTo(OrderEventType.ORDER_PAID.value());
    }

    @Test
    @DisplayName("dedup 命中（重複事件）→ 一筆 outbox 都不落")
    void duplicateEvent_writesNoOutbox() throws Exception {
        when(dedupRepository.insertIdempotent(eq("T1"), eq("O1"), anyString(), anyString())).thenReturn(0);

        handler.handlePaymentSuccess(payload(), "{}", "hash3");

        verify(outboxMapper, org.mockito.Mockito.never()).insert(any());
        verify(orderRepository, org.mockito.Mockito.never()).findByOrderNoForUpdate(anyString());
        verify(orderItemRepository, org.mockito.Mockito.never()).findByOrderNo(anyString());
    }
}
