package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.RefundSource;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * 售後退款完成事件的交易性處理（03 §5.7.1，ORD-02 雙 queue 收斂）。
 *
 * 從 AfterSalesEventConsumer 獨立出來，確保 Spring @Transactional proxy 正常運作。
 *
 * 同一事務內依序：
 * 1. financial_log 冪等寫入（36 號策略 1：txnId 冪等 = 本 handler 的重複投遞守門，
 *    衝突時不得重複累加 refund_total_amount）
 * 2. 依 refundOutcome 分流：FULL → CAS 轉 FULLY_REFUNDED(70)；
 *    PARTIAL → 單一原子 UPDATE 累加 refund_total_amount（僅 COMPLETED(50) 轉 60，
 *    進行中訂單 10~40 不改狀態，物流/完成路徑照走）
 *
 * 倒數恢復（tryResume）刻意<b>不在</b>本事務內：其內含 AfterSales gRPC 反查（不應被 DB 事務
 * 包住），由 consumer 於本方法返回（= 事務 commit）後呼叫，見 AfterSalesEventConsumer。
 */
@Service
public class AfterSalesRefundEventHandler {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesRefundEventHandler.class);

    private final OrderRepository orderRepository;
    private final FinancialLogRepository financialLogRepository;
    private final OrderEventPublisher eventPublisher;
    private final OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter;

    public AfterSalesRefundEventHandler(OrderRepository orderRepository,
                                        FinancialLogRepository financialLogRepository,
                                        OrderEventPublisher eventPublisher,
                                        OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter) {
        this.orderRepository = orderRepository;
        this.financialLogRepository = financialLogRepository;
        this.eventPublisher = eventPublisher;
        this.refundFinalizedOutboxWriter = refundFinalizedOutboxWriter;
    }

    /**
     * @param outcome 已由 consumer 驗證過的 FULL_REFUND / PARTIAL_REFUND（fail-loud 在 consumer 層）
     */
    @Transactional
    public void handleRefundCompleted(String orderNo, AfterSalePlatformDecisionOutcome outcome,
                                      long refundAmount, String externalRefId, String caseNo) {
        boolean duplicate = false;
        String txnId = null;
        if (refundAmount > 0) {
            txnId = externalRefId.isEmpty() ? "AS_REFUND_" + caseNo : externalRefId;
            int logRows = financialLogRepository.insertIdempotent(
                    TsidUtil.generate(),
                    orderNo,
                    FinancialLogType.REFUND_ORIGINAL.value(),
                    refundAmount,
                    txnId);
            duplicate = logRows == 0;
            if (duplicate) {
                log.info("AfterSalesRefundEventHandler.duplicate",
                        "退款完成事件重複投遞（financial_log txnId 已存在），冪等 skip, txnId=" + txnId,
                        "orderNo", orderNo);
            } else {
                log.info("AfterSalesRefundEventHandler.financialLog",
                        "退款流水已記錄, amount=" + refundAmount, "orderNo", orderNo);
            }
        }

        if (!duplicate) {
            switch (outcome) {
                case FULL_REFUND -> applyFullRefund(orderNo);
                case PARTIAL_REFUND -> applyPartialRefund(orderNo, refundAmount);
                default -> throw new IllegalStateException(
                        "未處理的 outcome: " + outcome + ", orderNo=" + orderNo);
            }

            if (txnId != null) {
                refundFinalizedOutboxWriter.write(orderNo, RefundSource.AFTERSALE_CASE, txnId,
                        refundAmount, outcome.value(), caseNo, null);
            }
        }
    }

    /** FULL outcome：多前置 CAS → FULLY_REFUNDED(70)（沿用既有 updateStatusIfMatchAny）。 */
    private void applyFullRefund(String orderNo) {
        List<String> allowedStatuses = List.of(
                OrderStatus.PAID.value(),
                OrderStatus.PENDING_SHIPMENT.value(),
                OrderStatus.IN_TRANSIT.value(),
                OrderStatus.WAITING_PICKUP.value(),
                OrderStatus.SHIPPING_ABNORMAL.value(),
                OrderStatus.DELIVERED.value(),
                OrderStatus.COMPLETED.value(),
                OrderStatus.PARTIALLY_REFUNDED.value());

        String targetStatus = OrderStatus.FULLY_REFUNDED.value();
        int statusRows = orderRepository.updateStatusIfMatchAny(orderNo, targetStatus, allowedStatuses);
        if (statusRows > 0) {
            log.info("AfterSalesRefundEventHandler.fullRefund.statusUpdated",
                    "訂單狀態更新為 " + targetStatus, "orderNo", orderNo);
            eventPublisher.emitSellerOrderUpdated(orderNo, OrderEventPublisher.SRC_AFTER_SALE,
                    null, targetStatus);
        } else {
            log.info("AfterSalesRefundEventHandler.fullRefund.skip",
                    "訂單不在合法前置狀態，冪等 skip", "orderNo", orderNo);
        }
    }

    /**
     * PARTIAL outcome：單一原子 UPDATE（03 §5.7.1 強制）——累加 refund_total_amount，
     * 僅 COMPLETED(50) 同步轉 PARTIALLY_REFUNDED(60)，進行中訂單（10~40）狀態不變。
     */
    private void applyPartialRefund(String orderNo, long refundAmount) {
        int affected = orderRepository.applyPartialRefund(orderNo, refundAmount);
        if (affected == 0) {
            log.info("AfterSalesRefundEventHandler.partialRefund.skip",
                    "訂單不存在或已處終態（70/80），不累加", "orderNo", orderNo);
            return;
        }
        log.info("AfterSalesRefundEventHandler.partialRefund.accumulated",
                "refund_total_amount 已累加 " + refundAmount, "orderNo", orderNo);
        eventPublisher.emitSellerOrderUpdated(orderNo, OrderEventPublisher.SRC_AFTER_SALE, null, null);
    }
}
