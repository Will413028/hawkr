package io.github.will413028.hawkr.payment.application.dto;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;

import java.time.LocalDateTime;
import java.time.LocalDate;

/**
 * 支付詳情回應 DTO
 */
public record PaymentDetailResponse(
        String tradeNo,
        String orderId,
        String merId,
        String buyerId,
        PaymentMethod paymentType,
        long amount,
        String productName,
        PaymentTransactionStatus status,
        String providerTradeNo,
        String providerStatus,
        String providerMessage,
        String atmBankCode,
        String atmVirtualAccount,
        LocalDate atmExpireDate,
        LocalDateTime payTime,
        LocalDateTime closedAt,
        LocalDateTime expireAt,
        LocalDateTime createdAt,
        LocalDateTime updatedAt,
        // §2A.6 重複付款顯示
        boolean isDuplicate,
        Integer duplicateSequence,
        // 撥款欄位（收斂規範 §6.2 封閉規則：僅 Admin API 可暴露，App DTO 必須過濾）
        LocalDateTime grantedAt,
        String grantStatus,
        String grantMessage
) {

    /**
     * 從 Domain Model 建立
     */
    public static PaymentDetailResponse from(PaymentTransaction transaction) {
        return new PaymentDetailResponse(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.merId(),
                transaction.buyerId(),
                transaction.paymentType(),
                transaction.amount(),
                transaction.productName(),
                transaction.status(),
                transaction.providerTradeNo(),
                transaction.providerStatus(),
                transaction.providerMessage(),
                transaction.atmBankCode(),
                transaction.atmVirtualAccount(),
                transaction.atmExpireDate(),
                transaction.payTime(),
                transaction.closedAt(),
                transaction.expireAt(),
                transaction.createdAt(),
                transaction.updatedAt(),
                transaction.isDuplicate(),
                transaction.duplicateSequence(),
                transaction.grantedAt(),
                transaction.grantStatus(),
                transaction.grantMessage()
        );
    }
}
