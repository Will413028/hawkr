package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.RefundSource;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.dto.OrderRefundFinalizedPayload;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

/**
 * {@code order.refund.finalized} 的<b>唯一</b>寫入點（58 §6.3.2.3）。
 *
 * <p>三個發布點（E1 售後退款完成／E2 自動退款／E3 賣家取消出貨）共用本 writer，
 * 目的是讓「payload 怎麼組」只有一份：三份各自組裝的話，欄位漂移不會編譯錯、不會測試紅，
 * 只會讓其中一條路徑的折讓金額算錯。
 *
 * <p><b>本類別沒有自己的 {@code @Transactional}</b>——它必須參與呼叫端<b>既有</b>的交易
 * （Spring 預設 {@code REQUIRED}）。這是刻意的：直接 publish 會讓「交易 rollback 但事件已發出」
 * 變成可能，而退款事件一旦多發一則，下游就是一張無法撤回的折讓單。
 *
 * <p><b>呼叫順序契約</b>：必須在 {@code FinancialLogRepository.insertIdempotent} <b>之後</b>呼叫
 * ——{@code cumulative_refund_amount} 由同一交易內的 {@code sumAmountByType} 算出，
 * 提前呼叫會少算本筆。
 */
@Component
public class OrderRefundFinalizedOutboxWriter {

    private static final ObsLogger log = ObsLogger.forClass(OrderRefundFinalizedOutboxWriter.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    /** {@code financial_logs.external_txn_id} 是 {@code VARCHAR(128)}，refund_ref 與它同一個值。 */
    static final int REFUND_REF_MAX_LEN = 128;

    private final OrderRepository orderRepository;
    private final FinancialLogRepository financialLogRepository;
    private final OrderOutboxMapper orderOutboxMapper;
    private final ObsContext obsContext;

    public OrderRefundFinalizedOutboxWriter(OrderRepository orderRepository,
                                            FinancialLogRepository financialLogRepository,
                                            OrderOutboxMapper orderOutboxMapper,
                                            ObsContext obsContext) {
        this.orderRepository = orderRepository;
        this.financialLogRepository = financialLogRepository;
        this.orderOutboxMapper = orderOutboxMapper;
        this.obsContext = obsContext;
    }

    /**
     * 在呼叫端的交易內落一列 {@code order_outbox}。
     *
     * @param refundRef   ＝ {@code financial_logs.external_txn_id}；下游冪等鍵 {@code REFUND:{refundRef}}
     * @param refundOutcome FULL_REFUND / PARTIAL_REFUND，僅 {@link RefundSource#AFTERSALE_CASE} 有值，其餘傳 null
     * @param caseNo      售後單號，僅 {@link RefundSource#AFTERSALE_CASE} 有值
     * @param cancelId    取消單號，僅 {@link RefundSource#SELLER_CANCEL} 有值
     */
    public void write(String orderNo,
                      RefundSource refundSource,
                      String refundRef,
                      long refundAmount,
                      String refundOutcome,
                      String caseNo,
                      String cancelId) {
        // fail-loud：空的 refund_ref = 下游拿不到冪等鍵 ⇒ 只能退回 event_id，而 event_id 每小時換一次
        // ⇒ 同一筆退款每小時開一張新折讓單。寧可 rollback 重試也不能發出一則無鍵事件。
        if (refundRef == null || refundRef.isBlank()) {
            throw new IllegalStateException(
                    "order.refund.finalized 的 refund_ref 為空, orderNo=" + orderNo
                            + ", refundSource=" + refundSource);
        }
        if (refundRef.length() > REFUND_REF_MAX_LEN) {
            throw new IllegalStateException(
                    "order.refund.finalized 的 refund_ref 超過 " + REFUND_REF_MAX_LEN
                            + " 字元（financial_logs.external_txn_id 上界）, orderNo=" + orderNo
                            + ", len=" + refundRef.length());
        }

        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            // fail-loud：訂單不存在＝資料完整性問題，靜默補零會讓下游把發票開給錯的賣家
            throw new IllegalStateException(
                    "order.refund.finalized 找不到訂單, orderNo=" + orderNo
                            + ", refundRef=" + refundRef);
        }

        // 累計退款一律由 financial_logs 重算（僅 REFUND_ORIGINAL）。
        // 不能讀 orders.refund_total_amount——那欄只有 PARTIAL 路徑維護，全額退款讀它會拿到 0。
        long cumulative = financialLogRepository.sumAmountByType(
                orderNo, FinancialLogType.REFUND_ORIGINAL.value());

        // occurred_at 與 refunded_at 同值：order-service 是「退款落定」這個事實的權威，
        // 落定時點就是本交易。上游 aftersales 的 occurred_at 不可用——RefundSyncWorker 重發時
        // 整個 payload 重建，那個時間是「重發時刻」而非「退款時刻」。
        String nowIso = DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc());
        OrderRefundFinalizedPayload payload = new OrderRefundFinalizedPayload(
                orderNo, nowIso,
                str(order.getStoreId()), str(order.getBuyerId()),
                refundRef, refundSource.value(), blankToNull(refundOutcome),
                String.valueOf(refundAmount),
                str(order.getTotalActualAmount()),
                String.valueOf(cumulative),
                blankToNull(caseNo), blankToNull(cancelId),
                order.getPaymentMethod(), nowIso,
                // 58 §6.3.2.1：與 order.paid 同一條理由；取不到就是 null，不補假 UUID
                obsContext.getCorrelationIdOrNull());

        OrderOutboxEntity outbox = new OrderOutboxEntity();
        outbox.setAggregateId(orderNo);
        outbox.setEventType(OrderEventType.ORDER_REFUND_FINALIZED.value());
        outbox.setExchange(MqConstants.EXCHANGE_ORDER);
        outbox.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_REFUND_FINALIZED);
        outbox.setStatus(OutboxEventStatus.PENDING.value());
        outbox.setRetryCount(0);
        try {
            outbox.setPayload(MAPPER.writeValueAsString(payload));
        } catch (JsonProcessingException e) {
            // 固定結構序列化理論上不會失敗；拋出讓事務 rollback，避免退款已記但事件遺失
            throw new IllegalStateException(
                    "order.refund.finalized payload 序列化失敗 orderNo=" + orderNo, e);
        }
        orderOutboxMapper.insert(outbox);

        log.info("OrderRefundFinalizedOutboxWriter.enqueued",
                "退款落定事件已入 outbox, refundSource=" + refundSource.value()
                        + ", refundRef=" + refundRef
                        + ", refundAmount=" + refundAmount
                        + ", cumulativeRefundAmount=" + cumulative,
                "orderNo", orderNo);
    }

    private static String str(Long v) {
        return v == null ? null : String.valueOf(v);
    }

    private static String blankToNull(String v) {
        return (v == null || v.isBlank()) ? null : v;
    }
}
