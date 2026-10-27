package io.github.will413028.hawkr.payment.application.dto;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;

import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * 退款詳情回應 DTO
 */
public record RefundDetailResponse(
        String refundNo,
        String tradeNo,
        String orderId,
        long amount,
        RefundStatus status,
        String reason,
        String providerRefundNo,
        String providerStatus,
        String providerMessage,
        LocalDate estimatedRefundDate,
        LocalDate bankDate,
        LocalDateTime completedAt,
        LocalDateTime createdAt
) {

    /**
     * 從 Domain Model 建立
     */
    public static RefundDetailResponse from(RefundTransaction refund) {
        return new RefundDetailResponse(
                refund.refundNo(),
                refund.tradeNo(),
                refund.orderId(),
                refund.amount(),
                refund.status(),
                refund.reason(),
                refund.providerRefundNo(),
                refund.providerStatus(),
                refund.providerMessage(),
                refund.estimatedRefundDate(),
                refund.bankDate(),
                refund.completedAt(),
                refund.createdAt()
        );
    }
}
