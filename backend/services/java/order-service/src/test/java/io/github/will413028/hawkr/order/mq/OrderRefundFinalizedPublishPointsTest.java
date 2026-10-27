package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.RefundSource;
import io.github.will413028.hawkr.order.entity.AutoRefundTaskEntity;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.worker.AutoRefundTxHandler;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code order.refund.finalized} 三個發布點（58 §6.3.2.3 E1/E2/E3）。
 *
 * <p>本事件是 invoice-service 判定「已開立的發票該作廢還是開折讓」的<b>唯一</b>輸入。
 * 缺一個發布點 ⇒ 該路徑的退款不會作廢也不會折讓 ⇒ 賣家對一筆已退款的訂單仍申報了銷售額，
 * 而且沒有任何訊號。多發一則 ⇒ 多一張無法撤回的折讓單。兩個方向都要守。
 *
 * <p><b>鑑別力</b>（變異測試已實跑，見 PR 說明）：
 * <ul>
 *   <li>拿掉 E1 的 {@code duplicate == false} 守衛 ⇒ {@link E1AfterSaleCase#duplicateDelivery_writesNoOutboxRow} 轉紅</li>
 *   <li>把 outbox 寫入改成直接 publish／移出交易 ⇒ 三個 {@code emits*} 全轉紅
 *       （並由 {@code OrderRefundFinalizedOutboxTxIT} 在真 DB 上守交易邊界）</li>
 * </ul>
 */
class OrderRefundFinalizedPublishPointsTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static final String ORDER_NO = "O2026083000001";
    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final long TOTAL_ACTUAL = 1200L;

    private OrderRepository orderRepository;
    private FinancialLogRepository financialLogRepository;
    private OrderOutboxMapper outboxMapper;
    private OrderRefundFinalizedOutboxWriter writer;

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
        financialLogRepository = mock(FinancialLogRepository.class);
        outboxMapper = mock(OrderOutboxMapper.class);
        writer = new OrderRefundFinalizedOutboxWriter(
                orderRepository, financialLogRepository, outboxMapper,
                new io.github.will413028.hawkr.common.obs.context.ObsContext());
        when(orderRepository.findByOrderNo(ORDER_NO)).thenReturn(order());
    }

    private static OrderEntity order() {
        OrderEntity o = new OrderEntity();
        o.setOrderNo(ORDER_NO);
        o.setStoreId(STORE_ID);
        o.setBuyerId(BUYER_ID);
        o.setTotalActualAmount(TOTAL_ACTUAL);
        o.setPaymentMethod("CREDIT_CARD");
        // 刻意設成 0：全額退款路徑從不維護這一欄，讀它會把「已全退」誤判成「沒退過」
        o.setRefundTotalAmount(0L);
        return o;
    }

    private JsonNode capturePayload() throws Exception {
        ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);
        verify(outboxMapper).insert(captor.capture());
        OrderOutboxEntity row = captor.getValue();
        assertThat(row.getExchange()).isEqualTo(MqConstants.EXCHANGE_ORDER);
        assertThat(row.getRoutingKey()).isEqualTo(MqConstants.ROUTING_KEY_ORDER_REFUND_FINALIZED);
        assertThat(row.getEventType()).isEqualTo(OrderEventType.ORDER_REFUND_FINALIZED.value());
        assertThat(row.getStatus()).isEqualTo(OutboxEventStatus.PENDING.value());
        assertThat(row.getAggregateId()).isEqualTo(ORDER_NO);
        return MAPPER.readTree(row.getPayload());
    }

    // ==================== E1：售後退款完成 ====================

    @Nested
    @DisplayName("E1 AfterSalesRefundEventHandler.handleRefundCompleted")
    class E1AfterSaleCase {

        private AfterSalesRefundEventHandler handler;

        @BeforeEach
        void setUpE1() {
            handler = new AfterSalesRefundEventHandler(
                    orderRepository, financialLogRepository,
                    mock(OrderEventPublisher.class), writer);
        }

        @Test
        @DisplayName("首次投遞（duplicate == false）：恰好一列 outbox，refund_source = AFTERSALE_CASE")
        void firstDelivery_emitsExactlyOnce() throws Exception {
            when(financialLogRepository.insertIdempotent(anyLong(), eq(ORDER_NO), anyString(), anyLong(), anyString()))
                    .thenReturn(1);
            when(financialLogRepository.sumAmountByType(ORDER_NO, FinancialLogType.REFUND_ORIGINAL.value()))
                    .thenReturn(500L);

            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.PARTIAL_REFUND, 500L, "RFD_A1", "AS_9001");

            JsonNode p = capturePayload();
            assertThat(p.get("refund_source").asText()).isEqualTo(RefundSource.AFTERSALE_CASE.value());
            assertThat(p.get("refund_ref").asText()).isEqualTo("RFD_A1");
            assertThat(p.get("refund_outcome").asText())
                    .isEqualTo(AfterSalePlatformDecisionOutcome.PARTIAL_REFUND.value());
            assertThat(p.get("case_no").asText()).isEqualTo("AS_9001");
            assertThat(p.get("cancel_id").isNull()).isTrue();
            assertThat(p.get("refund_amount").asText()).isEqualTo("500");
            assertThat(p.get("cumulative_refund_amount").asText()).isEqualTo("500");
        }

        /**
         * 🔴 本檔最貴的一條。aftersales 的 {@code RefundSyncWorker}（{@code @Scheduled} 每小時）
         * 把任何 1 小時前的 DONE refund task 無限重發 ⇒ 守衛被拿掉的話，
         * 同一筆退款每小時多發一則 ⇒ 下游每小時多開一張<b>無法撤回</b>的折讓單。
         */
        @Test
        @DisplayName("重複投遞（duplicate == true）：零列 outbox")
        void duplicateDelivery_writesNoOutboxRow() {
            // logRows == 0 ＝ financial_logs 的 (type, external_txn_id) 已存在 ＝ 重複投遞
            when(financialLogRepository.insertIdempotent(anyLong(), eq(ORDER_NO), anyString(), anyLong(), anyString()))
                    .thenReturn(0);

            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.FULL_REFUND, 1200L, "RFD_A1", "AS_9001");

            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }

        @Test
        @DisplayName("externalRefId 為空時 refund_ref 沿用 AS_REFUND_{caseNo}（該檔既有算式）")
        void blankExternalRefId_fallsBackToCaseNoRef() throws Exception {
            when(financialLogRepository.insertIdempotent(anyLong(), eq(ORDER_NO), anyString(), anyLong(), anyString()))
                    .thenReturn(1);
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(1200L);

            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.FULL_REFUND, 1200L, "", "AS_9002");

            assertThat(capturePayload().get("refund_ref").asText()).isEqualTo("AS_REFUND_AS_9002");
        }

        @Test
        @DisplayName("refundAmount == 0：沒有 financial_log、沒有 refund_ref ⇒ 不發")
        void zeroAmount_writesNoOutboxRow() {
            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.FULL_REFUND, 0L, "RFD_A1", "AS_9003");

            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }

        @Test
        @DisplayName("cumulative 取 financial_logs 的 SUM，不取 orders.refund_total_amount（全額退款讀它會拿到 0）")
        void cumulativeComesFromFinancialLogsNotOrdersColumn() throws Exception {
            when(financialLogRepository.insertIdempotent(anyLong(), eq(ORDER_NO), anyString(), anyLong(), anyString()))
                    .thenReturn(1);
            when(financialLogRepository.sumAmountByType(ORDER_NO, FinancialLogType.REFUND_ORIGINAL.value()))
                    .thenReturn(1200L);

            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.FULL_REFUND, 1200L, "RFD_A2", "AS_9004");

            // order().refundTotalAmount == 0；讀那一欄的話這裡會是 "0" ⇒ 發票端判成「沒退過」而不作廢
            assertThat(capturePayload().get("cumulative_refund_amount").asText()).isEqualTo("1200");
        }

        @Test
        @DisplayName("順序契約：先 insertIdempotent 再算 cumulative（反過來會少算本筆）")
        void computesCumulativeAfterInsert() {
            when(financialLogRepository.insertIdempotent(anyLong(), eq(ORDER_NO), anyString(), anyLong(), anyString()))
                    .thenReturn(1);
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(500L);

            handler.handleRefundCompleted(ORDER_NO,
                    AfterSalePlatformDecisionOutcome.PARTIAL_REFUND, 500L, "RFD_A3", "AS_9005");

            InOrder order = inOrder(financialLogRepository, outboxMapper);
            order.verify(financialLogRepository)
                    .insertIdempotent(anyLong(), anyString(), anyString(), anyLong(), anyString());
            order.verify(financialLogRepository).sumAmountByType(anyString(), anyString());
            order.verify(outboxMapper).insert(any(OrderOutboxEntity.class));
        }
    }

    // ==================== E2：訂單已取消卻收到付款 → 自動退款 ====================

    @Nested
    @DisplayName("E2 AutoRefundTxHandler.completeAutoRefund")
    class E2AutoRefund {

        private AutoRefundTxHandler handler;

        @BeforeEach
        void setUpE2() {
            handler = new AutoRefundTxHandler(
                    mock(AutoRefundTaskRepository.class), financialLogRepository, writer);
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(1200L);
        }

        @Test
        @DisplayName("emits：refund_source = AUTO_REFUND，refund_ref = PayUni refundNo")
        void emitsWithAutoRefundSource() throws Exception {
            AutoRefundTaskEntity task = new AutoRefundTaskEntity();
            task.setTaskId(1L);
            task.setOrderNo(ORDER_NO);
            task.setAmount(1200L);

            handler.completeAutoRefund(task, "PAYUNI_RFD_77");

            JsonNode p = capturePayload();
            assertThat(p.get("refund_source").asText()).isEqualTo(RefundSource.AUTO_REFUND.value());
            assertThat(p.get("refund_ref").asText()).isEqualTo("PAYUNI_RFD_77");
            assertThat(p.get("refund_amount").asText()).isEqualTo("1200");
            // 僅 E1 有 refund_outcome；E2/E3 留 null（58 §6.3.2.3 payload 欄位表）
            assertThat(p.get("refund_outcome").isNull()).isTrue();
            assertThat(p.get("case_no").isNull()).isTrue();
            assertThat(p.get("cancel_id").isNull()).isTrue();
        }

        /**
         * E2 走的是 {@code handleCancelledOrderPayment}、從未發出過 {@code order.paid} ⇒ 沒有發票。
         * 仍然要發：「每一筆落定的退款恰好發出一則」比「除了這一種以外都發」容易驗證得多。
         */
        @Test
        @DisplayName("沒有發票的訂單也照發（不做『這筆有沒有票』的判斷，那是下游的事）")
        void emitsEvenWhenOrderNeverPaidEvent() {
            AutoRefundTaskEntity task = new AutoRefundTaskEntity();
            task.setTaskId(2L);
            task.setOrderNo(ORDER_NO);
            task.setAmount(1200L);

            handler.completeAutoRefund(task, "PAYUNI_RFD_78");

            verify(outboxMapper).insert(any(OrderOutboxEntity.class));
        }
    }

    // ==================== E3：賣家取消出貨 ====================

    @Nested
    @DisplayName("E3 SellerCancelRefundTxHandler.handleCancelCompleted")
    class E3SellerCancel {

        private SellerCancelRefundTxHandler handler;

        @BeforeEach
        void setUpE3() {
            handler = new SellerCancelRefundTxHandler(orderRepository, financialLogRepository, writer);
        }

        @Test
        @DisplayName("補上了從來沒寫過的 financial_log：REFUND_ORIGINAL / AS_CANCEL_{cancelId} / 訂單全額")
        void writesTheFinancialLogThatWasOnlyEverInJavadoc() {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(TOTAL_ACTUAL);

            handler.handleCancelCompleted(ORDER_NO, "CXL_555");

            verify(financialLogRepository).insertIdempotent(anyLong(), eq(ORDER_NO),
                    eq(FinancialLogType.REFUND_ORIGINAL.value()), eq(TOTAL_ACTUAL), eq("AS_CANCEL_CXL_555"));
        }

        @Test
        @DisplayName("emits：refund_source = SELLER_CANCEL，金額取 orders.total_actual_amount")
        void emitsWithSellerCancelSource() throws Exception {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(TOTAL_ACTUAL);

            handler.handleCancelCompleted(ORDER_NO, "CXL_555");

            JsonNode p = capturePayload();
            assertThat(p.get("refund_source").asText()).isEqualTo(RefundSource.SELLER_CANCEL.value());
            assertThat(p.get("refund_ref").asText()).isEqualTo("AS_CANCEL_CXL_555");
            assertThat(p.get("cancel_id").asText()).isEqualTo("CXL_555");
            assertThat(p.get("case_no").isNull()).isTrue();
            assertThat(p.get("refund_amount").asText()).isEqualTo(String.valueOf(TOTAL_ACTUAL));
        }

        @Test
        @DisplayName("need_refund_page 的清除收進同一事務（原本在 consumer 內、非交易性）")
        void clearsNeedRefundPageInSameTx() {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(TOTAL_ACTUAL);

            handler.handleCancelCompleted(ORDER_NO, "CXL_556");

            verify(orderRepository).updateNeedRefundPage(ORDER_NO, false);
        }

        @Test
        @DisplayName("cancel_id 為空 → fail-loud（它是 refund_ref 的唯一組成，共用鍵會讓多筆退款被當成同一筆）")
        void blankCancelId_failsLoud() {
            assertThatThrownBy(() -> handler.handleCancelCompleted(ORDER_NO, ""))
                    .isInstanceOf(IllegalStateException.class)
                    .hasMessageContaining("cancel_id");
            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }
    }

    // ==================== payload 契約（58 §6.3.2.3 欄位表） ====================

    @Nested
    @DisplayName("payload 契約")
    class PayloadContract {

        @Test
        @DisplayName("欄位集合恰好等於 58 §6.3.2.3 的 15 欄（多一欄少一欄都轉紅）")
        void fieldSetMatchesSpecExactly() throws Exception {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(300L);
            writer.write(ORDER_NO, RefundSource.AFTERSALE_CASE, "RFD_X", 300L,
                    AfterSalePlatformDecisionOutcome.PARTIAL_REFUND.value(), "AS_1", null);

            JsonNode p = capturePayload();
            Set<String> actual = new java.util.TreeSet<>();
            p.fieldNames().forEachRemaining(actual::add);
            assertThat(actual).containsExactlyInAnyOrder(
                    "event_id", "order_no", "occurred_at", "store_id", "buyer_id",
                    "refund_ref", "refund_source", "refund_outcome", "refund_amount",
                    "order_total_actual_amount", "cumulative_refund_amount",
                    "case_no", "cancel_id", "payment_method", "refunded_at",
                    // 58 §6.3.2.1：invoice-service 的 invoice_status_history.correlation_id 靠它，
                    // 恆空時 ADM 稽核在最常查的那條鏈上就是死的，而且沒有任何訊號
                    "correlation_id");
        }

        @Test
        @DisplayName("金額與 TSID 一律 string（int64 進 generic-map consumer 會被 float64 截斷）")
        void amountsAndTsidsAreStrings() throws Exception {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(300L);
            writer.write(ORDER_NO, RefundSource.AFTERSALE_CASE, "RFD_X", 300L, null, "AS_1", null);

            JsonNode p = capturePayload();
            for (String key : new String[]{"store_id", "buyer_id", "refund_amount",
                    "order_total_actual_amount", "cumulative_refund_amount"}) {
                assertThat(p.get(key).isTextual()).as("%s 必須是 string", key).isTrue();
            }
        }

        @Test
        @DisplayName("occurred_at / refunded_at 是 RFC3339 帶 Z（order.paid 的 LocalDateTime.toString() 是既有債，不跟進）")
        void timestampsAreRfc3339Utc() throws Exception {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(300L);
            writer.write(ORDER_NO, RefundSource.AUTO_REFUND, "RFD_X", 300L, null, null, null);

            JsonNode p = capturePayload();
            for (String key : new String[]{"occurred_at", "refunded_at"}) {
                String v = p.get(key).asText();
                // ⚠️ DateTimeUtils.formatIso8601 用 ISO_OFFSET_DATE_TIME，秒與奈秒同時為 0 時
                // 會省略秒（"…T08:30Z"，非 RFC3339）。nowUtc() 帶微秒 ⇒ 實務上打不到，
                // 但那個下界不是本層守得住的，已在交付回報列為設計書缺口。
                assertThat(v).as("%s 必須帶 Z 且可被 RFC3339 解析", key)
                        .matches("\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(\\.\\d+)?Z");
                assertThat(java.time.OffsetDateTime.parse(v)).isNotNull();
            }
        }

        @Test
        @DisplayName("event_id 每次不同（它是 metadata，正因為不穩定所以不能當冪等鍵）")
        void eventIdIsFreshEachTime() throws Exception {
            when(financialLogRepository.sumAmountByType(anyString(), anyString())).thenReturn(300L);
            ArgumentCaptor<OrderOutboxEntity> captor = ArgumentCaptor.forClass(OrderOutboxEntity.class);

            writer.write(ORDER_NO, RefundSource.AUTO_REFUND, "RFD_SAME", 300L, null, null, null);
            writer.write(ORDER_NO, RefundSource.AUTO_REFUND, "RFD_SAME", 300L, null, null, null);

            verify(outboxMapper, org.mockito.Mockito.times(2)).insert(captor.capture());
            String a = MAPPER.readTree(captor.getAllValues().get(0).getPayload()).get("event_id").asText();
            String b = MAPPER.readTree(captor.getAllValues().get(1).getPayload()).get("event_id").asText();
            assertThat(a).isNotEqualTo(b);
            // 而 refund_ref 是穩定的 ⇒ 下游的 REFUND:{refund_ref} 去重才擋得住重發
            assertThat(MAPPER.readTree(captor.getAllValues().get(0).getPayload()).get("refund_ref").asText())
                    .isEqualTo(MAPPER.readTree(captor.getAllValues().get(1).getPayload()).get("refund_ref").asText());
        }

        @Test
        @DisplayName("refund_ref 為空 → fail-loud（下游只剩 event_id 可用，而它每小時換一次）")
        void blankRefundRef_failsLoud() {
            assertThatThrownBy(() -> writer.write(ORDER_NO, RefundSource.AUTO_REFUND, "  ", 300L, null, null, null))
                    .isInstanceOf(IllegalStateException.class)
                    .hasMessageContaining("refund_ref");
            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }

        @Test
        @DisplayName("refund_ref 超過 128 → fail-loud（financial_logs.external_txn_id 的上界）")
        void overlongRefundRef_failsLoud() {
            String tooLong = "R".repeat(129);
            assertThatThrownBy(() -> writer.write(ORDER_NO, RefundSource.AUTO_REFUND, tooLong, 300L, null, null, null))
                    .isInstanceOf(IllegalStateException.class);
            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }

        @Test
        @DisplayName("訂單不存在 → fail-loud（靜默補零會讓下游把發票開給錯的賣家）")
        void missingOrder_failsLoud() {
            when(orderRepository.findByOrderNo("O_MISSING")).thenReturn(null);
            assertThatThrownBy(() -> writer.write("O_MISSING", RefundSource.AUTO_REFUND, "R1", 1L, null, null, null))
                    .isInstanceOf(IllegalStateException.class);
            verify(outboxMapper, never()).insert(any(OrderOutboxEntity.class));
        }
    }
}
