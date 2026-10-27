package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.context.ObsCarrier;
import io.github.will413028.hawkr.common.obs.context.ObsScope;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
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
import java.util.Set;
import java.util.TreeSet;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code order.paid} 的兩個發票欄位（58 號 §6.3.2.1、§6.2.4 order-service ⑤）。
 *
 * <p><b>correlation_id 為什麼非有不可</b>：invoice-service 的
 * {@code invoice_status_history.correlation_id} 與 {@code invoice_admin_audit_logs.correlation_id}
 * 存在的理由逐字是「ADM 稽核 UI 要一鍵對到 Loki 與 Tempo」。開票是主路徑，它的 correlation 若恆為空，
 * 那兩欄在最常查的那條鏈上就是死的——而且<b>不會有任何訊號</b>：欄位可為空、寫入不報錯、測試不紅，
 * 只有真的有人去查一筆發票的來龍去脈時才會發現查不到。本檔就是那個訊號。
 *
 * <p><b>不能改走 AMQP header</b>：{@code pkg/mq} 的 consumer 不從 header 取 correlation
 * （{@code grep -n correlation pkg/mq/*.go} 只命中出向 {@code BaseEvent} 的欄位定義）
 * ⇒ 讀 payload 是目前唯一可行的機制。
 *
 * <p><b>鑑別力</b>：把 {@code correlationId} 從 payload 拿掉（或改成永遠傳 null）
 * ⇒ {@link #paidPayloadCarriesCorrelationIdFromTheConsumingChain} 轉紅；
 * 把 {@code invoiceSnapshotId} 拿掉 ⇒ {@link #paidPayloadCarriesInvoiceSnapshotId} 轉紅；
 * 只改 {@code enrich} 而讓兩欄在派送時被抹掉 ⇒ {@link #enrichKeepsBothInvoiceFields} 轉紅。
 */
class OrderPaidInvoiceFieldsTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private OrderRepository orderRepository;
    private OrderItemRepository orderItemRepository;
    private PaymentEventDedupRepository dedupRepository;
    private OrderOutboxMapper outboxMapper;
    private OrderInvoiceSnapshotRepository invoiceSnapshotRepository;
    private ObsContext obsContext;
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
        invoiceSnapshotRepository = mock(OrderInvoiceSnapshotRepository.class);
        obsContext = new ObsContext();

        handler = new PaymentEventHandler(
                orderRepository, orderItemRepository, dedupRepository,
                mock(FinancialLogRepository.class), mock(AutoRefundTaskRepository.class),
                mock(CouponConsumptionTaskRepository.class), mock(OrderEventPublisher.class),
                mock(PaymentGrpcClient.class), outboxMapper, invoiceSnapshotRepository,
                mock(OrderService.class), obsContext);

        when(dedupRepository.insertIdempotent(eq("T1"), eq("O1"), anyString(), anyString())).thenReturn(1);
        OrderEntity order = new OrderEntity();
        order.setOrderNo("O1");
        order.setStatus(OrderStatus.PENDING_PAYMENT.value());
        order.setBuyerId(2623900520L);
        order.setStoreId(2685914682L);
        order.setTotalActualAmount(960L);
        order.setShippingMethod("3002");
        when(orderRepository.findByOrderNoForUpdate("O1")).thenReturn(order);
        when(orderRepository.updatePaymentConfirmed(
                eq("O1"), anyString(), anyString(), anyString(), any(), anyString())).thenReturn(1);
        when(orderItemRepository.findByOrderNo("O1")).thenReturn(List.of());
    }

    private JsonNode payload() throws Exception {
        return MAPPER.readTree("{\"trade_no\":\"T1\",\"order_id\":\"O1\",\"amount\":960,"
                + "\"pay_time\":\"2026-08-30 10:00:00\",\"payment_type\":\""
                + PaymentMethod.CREDIT_CARD.value() + "\"}");
    }

    private void stubSnapshot(Long id) {
        if (id == null) {
            when(invoiceSnapshotRepository.findLatestByOrderNo("O1")).thenReturn(null);
            return;
        }
        OrderInvoiceSnapshotEntity s = new OrderInvoiceSnapshotEntity();
        s.setId(id);
        s.setOrderNo("O1");
        s.setSnapshotVersion(1);
        when(invoiceSnapshotRepository.findLatestByOrderNo("O1")).thenReturn(s);
    }

    private JsonNode capturePaidPayload() throws Exception {
        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        OrderOutboxEntity row = captor.getValue();
        assertThat(row.getEventType()).isEqualTo(OrderEventType.ORDER_PAID.value());
        return MAPPER.readTree(row.getPayload());
    }

    @Test
    @DisplayName("payload 帶消費這條鏈的 correlation_id（不是新生成的假 id）")
    void paidPayloadCarriesCorrelationIdFromTheConsumingChain() throws Exception {
        stubSnapshot(2649605557L);

        try (ObsScope ignored = obsContext.open(new ObsCarrier("corr-from-payment-chain", null, null))) {
            handler.handlePaymentSuccess(payload(), "{}", "h1");
        }

        assertThat(capturePaidPayload().path("correlation_id").asText())
                .isEqualTo("corr-from-payment-chain");
    }

    @Test
    @DisplayName("payload 帶 invoice_snapshot_id，且是數字**字串**（int64 進 generic-map consumer 會被 float64 截斷）")
    void paidPayloadCarriesInvoiceSnapshotId() throws Exception {
        stubSnapshot(2649605557L);

        handler.handlePaymentSuccess(payload(), "{}", "h2");

        JsonNode p = capturePaidPayload();
        assertThat(p.path("invoice_snapshot_id").isTextual())
                .as("必須是 string").isTrue();
        assertThat(p.path("invoice_snapshot_id").asText()).isEqualTo("2649605557");
    }

    @Test
    @DisplayName("沒有 correlation 時送 null，不自己補一個 UUID（假 id 對不到任何 log，比空值更難查）")
    void missingCorrelationIsNullNotAFabricatedUuid() throws Exception {
        stubSnapshot(2649605557L);

        // ObsContext 的狀態掛在 ThreadLocal 上，而 surefire 預設同一個執行緒跑完整個 class
        // ⇒ 「這條鏈沒有 correlation」只有在乾淨的執行緒上才成立。開一條新執行緒是唯一
        // 不依賴測試執行順序的做法（清 ThreadLocal 需要 ObsContext 沒有提供的 API）。
        Thread t = new Thread(() -> {
            try {
                handler.handlePaymentSuccess(payload(), "{}", "h3");
            } catch (Exception e) {
                throw new IllegalStateException(e);
            }
        });
        t.start();
        t.join();

        assertThat(capturePaidPayload().path("correlation_id").isNull()).isTrue();
    }

    @Test
    @DisplayName("V12 之前的舊訂單沒有快照 → invoice_snapshot_id 為 null，但付款流程照常完成（不 fail）")
    void missingSnapshotDoesNotBreakPaymentFlow() throws Exception {
        stubSnapshot(null);

        handler.handlePaymentSuccess(payload(), "{}", "h4");

        JsonNode p = capturePaidPayload();
        assertThat(p.path("invoice_snapshot_id").isNull()).isTrue();
        assertThat(p.path("order_no").asText()).isEqualTo("O1");
    }

    @Test
    @DisplayName("欄位集合＝既有 12 欄 ＋ 本次新增 2 欄（多一欄少一欄都是契約漂移）")
    void paidPayloadFieldSetIsExact() throws Exception {
        stubSnapshot(2649605557L);

        handler.handlePaymentSuccess(payload(), "{}", "h5");

        Set<String> actual = new TreeSet<>();
        capturePaidPayload().fieldNames().forEachRemaining(actual::add);
        assertThat(actual).containsExactlyInAnyOrder(
                "event_id", "order_no", "shipping_method", "occurred_at",
                "buyer_id", "notify_buyer_account_id", "store_id", "total_actual_amount",
                "seller_name", "seller_avatar_key", "buyer_nickname", "buyer_avatar_key",
                "correlation_id", "invoice_snapshot_id");
    }

    @Test
    @DisplayName("enrich（派送前補 seller/buyer meta）不得抹掉這兩欄——它跑在另一輪 tick，那裡沒有付款鏈的 ObsContext")
    void enrichKeepsBothInvoiceFields() throws Exception {
        long buyerId = 2623900520L;
        long storeId = 2685914682L;

        io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload core =
                new io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload(
                        "O1", "3002", "2026-08-30T01:00:00Z",
                        buyerId, null, storeId, 960L,
                        null, null, null, null,
                        "corr-from-payment-chain", "2649605557");

        OrderOutboxEntity event = new OrderOutboxEntity();
        event.setId(1L);
        event.setAggregateId("O1");
        event.setEventType(OrderEventType.ORDER_PAID.value());
        event.setExchange(io.github.will413028.hawkr.common.mq.MqConstants.EXCHANGE_ORDER);
        event.setRoutingKey(io.github.will413028.hawkr.common.mq.MqConstants.ROUTING_KEY_ORDER_PAID);
        event.setStatus(io.github.will413028.hawkr.common.utils.code.OutboxEventStatus.PENDING.value());
        event.setRetryCount(0);
        event.setPayload(MAPPER.writeValueAsString(core));

        io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher rabbitPublisher =
                mock(io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher.class);
        io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient sellerGrpcClient =
                mock(io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient.class);
        io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient buyerGrpcClient =
                mock(io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient.class);
        when(sellerGrpcClient.getSellerMeta(storeId)).thenThrow(new RuntimeException("meta unavailable"));
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId)))
                .thenThrow(new RuntimeException("meta unavailable"));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(io.github.will413028.hawkr.common.grpc.PrincipalResolution.allResolved(
                        java.util.Map.of(buyerId, 55L)));

        OrderOutboxPublisher publisher = new OrderOutboxPublisher(
                rabbitPublisher, outboxMapper, sellerGrpcClient, buyerGrpcClient);
        assertThat(publisher.publish(event)).isTrue();

        ArgumentCaptor<byte[]> body = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(
                eq(io.github.will413028.hawkr.common.mq.MqConstants.EXCHANGE_ORDER),
                eq(io.github.will413028.hawkr.common.mq.MqConstants.ROUTING_KEY_ORDER_PAID),
                body.capture(), eq(OrderEventType.ORDER_PAID.value()));

        JsonNode sent = MAPPER.readTree(body.getValue());
        assertThat(sent.path("correlation_id").asText()).isEqualTo("corr-from-payment-chain");
        assertThat(sent.path("invoice_snapshot_id").asText()).isEqualTo("2649605557");
        // 鑑別力錨：enrich 確實跑過（notify 欄位被解析出來），所以上面兩欄是「被保留」而非「沒被碰」
        assertThat(sent.path("notify_buyer_account_id").asLong()).isEqualTo(55L);
    }
}
