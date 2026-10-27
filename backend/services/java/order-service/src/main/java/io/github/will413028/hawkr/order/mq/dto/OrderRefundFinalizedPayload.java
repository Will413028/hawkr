package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * {@code order.refund.finalized} MQ 事件 Payload（58 §6.3.2.3）。
 *
 * <p><b>訂單層唯一權威退款事件</b>：invoice-service 據此判定「已開立的發票該作廢還是開折讓」。
 * 三個發布點皆在 order-service 的既有交易內落 {@code order_outbox}，由
 * {@code OrderOutboxPublisherScheduler} 派送。
 *
 * <p><b>為什麼不沿用既有三條退款事件</b>（58 §6.3.2.3 逐條答覆）：
 * {@code aftersales.refund.completed} 的 {@code event_id} 每小時換一次
 * （{@code RefundSyncWorker} 重發時 payload 整個重建）、{@code FULL_REFUND} 語意不等於全額、
 * 且不涵蓋賣家取消出貨的 {@code cancel_id} path；{@code refund.succeeded} 是支付管道層事實；
 * {@code payment.rescue.refund_completed} 退的是從未開過票的重複付款。
 *
 * <p><b>型別紀律</b>：金額與 TSID 一律 {@code String}（{@code OrderPaidPayload} 用 {@code Long}
 * 是既有債，新事件不跟進）——int64 進 generic-map consumer 會被 float64 截斷。
 * 金額單位是「元」的整數，禁止 BigDecimal / Double。
 *
 * <p><b>{@code refundRef}</b>：＝ {@code financial_logs.external_txn_id}（{@code VARCHAR(128)}，
 * 該表有 {@code UNIQUE (type, external_txn_id)}）⇒ 天生是「這筆退款」的全域唯一識別。
 * 下游的冪等鍵是 {@code REFUND:{refund_ref}}，<b>絕對不是 {@code eventId}</b>。
 *
 * <p><b>{@code cumulativeRefundAmount}</b>：同一交易內、{@code insertIdempotent} 之後由
 * {@code FinancialLogRepository.sumAmountByType(orderNo, REFUND_ORIGINAL)} 算出。
 * <b>不能用 {@code orders.refund_total_amount}</b>——那個欄位只有 PARTIAL 路徑維護，
 * 全額退款場景讀它會拿到 0，發票端就會判成「沒退過」而不作廢。
 * 它可能 &gt; {@code orderTotalActualAmount}（先 PARTIAL 後，第二張售後單的 refundable_amount
 * 仍是整張訂單金額），consumer 端一律 {@code min(cumulative, invoiced_total)} 並告警。
 *
 * <p><b>{@code refundOutcome}</b> 僅 {@code AFTERSALE_CASE} 來源有值（FULL_REFUND / PARTIAL_REFUND）；
 * {@code caseNo} 與 {@code cancelId} 兩者至多一個有值，依 {@code refundSource} 而定。
 *
 * <p><b>{@code occurredAt} / {@code refundedAt}</b>：ISO 8601 UTC 帶 {@code Z}
 * （{@code DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc())}），與 {@code order.paid} 走
 * {@code LocalDateTime.toString()} 的既有債<b>刻意不同</b>——新事件從第一天就 RFC3339 可解。
 *
 * <p><b>{@code correlationId}</b>（58 §6.3.2.1）：落 outbox 當下的 obs correlation_id。
 * 與 {@code order.paid} 同一條理由——invoice-service 的
 * {@code invoice_status_history.correlation_id} 若恆為空，ADM 稽核在最常查的那條鏈上
 * 就是死的，而且<b>沒有任何訊號</b>（欄位可空、寫入不報錯、測試不紅）。取不到時為 null。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderRefundFinalizedPayload(
        String eventId,
        String orderNo,
        String occurredAt,
        String storeId,
        String buyerId,
        String refundRef,
        String refundSource,
        String refundOutcome,
        String refundAmount,
        String orderTotalActualAmount,
        String cumulativeRefundAmount,
        String caseNo,
        String cancelId,
        String paymentMethod,
        String refundedAt,
        String correlationId
) {
    /** 便利建構子：{@code eventId} 由 publisher 端自動產生 UUID v4（36 §7.4.1）。 */
    public OrderRefundFinalizedPayload(
            String orderNo,
            String occurredAt,
            String storeId,
            String buyerId,
            String refundRef,
            String refundSource,
            String refundOutcome,
            String refundAmount,
            String orderTotalActualAmount,
            String cumulativeRefundAmount,
            String caseNo,
            String cancelId,
            String paymentMethod,
            String refundedAt,
            String correlationId
    ) {
        this(UUID.randomUUID().toString(), orderNo, occurredAt, storeId, buyerId,
                refundRef, refundSource, refundOutcome, refundAmount,
                orderTotalActualAmount, cumulativeRefundAmount,
                caseNo, cancelId, paymentMethod, refundedAt, correlationId);
    }
}
