package io.github.will413028.hawkr.payment.domain.service;

import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.Optional;

/**
 * 退款領域服務
 *
 * 負責處理退款相關的業務邏輯和狀態轉換規則
 */
public class RefundDomainService {

    /**
     * 判斷交易是否可以申請退款
     *
     * 規則：只有 PAID 狀態的交易才能申請退款
     */
    public boolean canRequestRefund(PaymentTransaction transaction) {
        return transaction.status() == PaymentTransactionStatus.PAID;
    }

    /**
     * 判斷退款是否可以標記為處理中
     *
     * 規則：只有 REQUESTED 狀態可以轉換到 PROCESSING
     */
    public boolean canMarkAsProcessing(RefundTransaction refund) {
        return refund.status() == RefundStatus.REQUESTED;
    }

    /**
     * 判斷退款是否可以標記為成功
     *
     * 規則：REQUESTED 或 PROCESSING 狀態可以轉換到 SUCCEEDED
     */
    public boolean canMarkAsSucceeded(RefundTransaction refund) {
        return refund.status() == RefundStatus.REQUESTED ||
               refund.status() == RefundStatus.PROCESSING;
    }

    /**
     * 判斷退款是否可以標記為失敗
     *
     * 規則：REQUESTED 或 PROCESSING 狀態可以轉換到 FAILED
     */
    public boolean canMarkAsFailed(RefundTransaction refund) {
        return refund.status() == RefundStatus.REQUESTED ||
               refund.status() == RefundStatus.PROCESSING;
    }

    /**
     * 驗證退款金額
     *
     * @param refundAmount    退款金額
     * @param transactionAmount 原交易金額
     * @param alreadyRefunded 已退款金額
     * @return 驗證結果（成功返回 empty，失敗返回錯誤訊息）
     */
    public Optional<String> validateRefundAmount(long refundAmount, long transactionAmount, long alreadyRefunded) {
        if (refundAmount <= 0) {
            return Optional.of("退款金額必須大於 0");
        }
        if (refundAmount > transactionAmount - alreadyRefunded) {
            return Optional.of("退款金額超過可退款餘額");
        }
        return Optional.empty();
    }

    /**
     * 建立退款成功事件（擴充：payuniStatus / payuniMessage / estimatedRefundDate / bankDate）
     *
     * @param refund 退款
     * @param payuniStatus PayUni 原始狀態碼（收斂規範 §4.4：sync=response.Status / async=inner RefundStatus）
     * @param payuniMessage PayUni 原始訊息
     * @param estimatedRefundDate ATM sync success 時 PayUni EstimatedRefundDate；其他情境傳 null
     * @param bankDate ATM async RefundStatus=1 時 webhook BankDate；其他情境傳 null
     * @param completedAt 退款完成時間
     */
    public PaymentDomainEvent.RefundSucceeded createRefundSucceededEvent(
            RefundTransaction refund,
            String payuniStatus,
            String payuniMessage,
            LocalDate estimatedRefundDate,
            LocalDate bankDate,
            LocalDateTime completedAt
    ) {
        return new PaymentDomainEvent.RefundSucceeded(
                refund.refundNo(),
                refund.tradeNo(),
                refund.orderId(),
                refund.amount(),
                refund.providerRefundNo(),
                payuniStatus,
                payuniMessage,
                estimatedRefundDate,
                bankDate,
                completedAt,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 建立退款失敗事件（擴充：payuniStatus）
     */
    public PaymentDomainEvent.RefundFailed createRefundFailedEvent(
            RefundTransaction refund,
            String payuniStatus,
            String payuniMessage
    ) {
        return new PaymentDomainEvent.RefundFailed(
                refund.refundNo(),
                refund.tradeNo(),
                refund.orderId(),
                refund.amount(),
                payuniStatus,
                payuniMessage,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 解析 PayUni 退款狀態到內部狀態
     *
     * PayUni 退款狀態對應：
     * - 1: 退款成功 → SUCCEEDED
     * - 2: 退款失敗 → FAILED
     * - 0: 處理中 → PROCESSING
     */
    public Optional<RefundStatus> parseProviderStatus(String providerStatus) {
        if (providerStatus == null) {
            return Optional.empty();
        }
        return switch (providerStatus) {
            case "1" -> Optional.of(RefundStatus.SUCCEEDED);
            case "2" -> Optional.of(RefundStatus.FAILED);
            case "0" -> Optional.of(RefundStatus.PROCESSING);
            default -> Optional.empty();
        };
    }
}
