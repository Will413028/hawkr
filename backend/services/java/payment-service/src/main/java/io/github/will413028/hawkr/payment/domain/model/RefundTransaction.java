package io.github.will413028.hawkr.payment.domain.model;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.domain.RefundStatusRules;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.Accessors;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;

import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * 退款交易領域模型
 *
 * 狀態轉換規則：
 * - REQUESTED → PROCESSING, FAILED
 * - PROCESSING → SUCCEEDED, FAILED
 * - 終態：SUCCEEDED, FAILED
 */
@Getter
@Accessors(fluent = true)
@AllArgsConstructor
@Builder
public class RefundTransaction {

    private Long id;
    private String refundNo;
    /** §2A.3.6 對應 payment_transactions(id) 的 PK，FK 改指 PK */
    private Long paymentId;
    private String tradeNo;
    private String orderId;

    private long amount;
    private RefundStatus status;
    private String reason;
    private String idempotencyKey;

    private String providerRefundNo;
    private String providerStatus;
    private String providerMessage;

    /** ATM sync（頁面版受理）時 PayUni 回報的預計扣款日；信用卡恆空。收斂規範 §3.4 里程碑。 */
    private LocalDate estimatedRefundDate;

    /** ATM async webhook RefundStatus=1 時 PayUni 回報的實際到帳日；信用卡恆空。 */
    private LocalDate bankDate;

    private LocalDateTime completedAt;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    private int version;

    /**
     * 檢查是否可以轉換到目標狀態
     */
    public boolean canTransitionTo(RefundStatus targetStatus) {
        return RefundStatusRules.canTransitionTo(status, targetStatus);
    }

    /**
     * 是否為終態
     */
    public boolean isTerminal() {
        return RefundStatusRules.isTerminal(status);
    }

    /**
     * 是否退款成功
     */
    public boolean isSucceeded() {
        return status == RefundStatus.SUCCEEDED;
    }

    /**
     * 建立退款申請的工廠方法
     */
    public static RefundTransaction create(
            Long paymentId,
            String tradeNo,
            String orderId,
            long amount,
            String reason) {
        return create(paymentId, tradeNo, orderId, amount, reason, null);
    }

    /**
     * 建立退款申請的工廠方法（含冪等鍵）
     *
     * @param paymentId      對應的 payment_transactions.id（§2A.3.6 FK 改指 PK）
     * @param idempotencyKey 冪等鍵（AfterSales 傳入 case_no 或 cancel_id），可為 null
     */
    public static RefundTransaction create(
            Long paymentId,
            String tradeNo,
            String orderId,
            long amount,
            String reason,
            String idempotencyKey) {

        return RefundTransaction.builder()
                .id(TsidUtil.generate())
                .refundNo(generateRefundNo())
                .paymentId(paymentId)
                .tradeNo(tradeNo)
                .orderId(orderId)
                .amount(amount)
                .status(RefundStatus.REQUESTED)
                .reason(reason)
                .idempotencyKey(idempotencyKey)
                .createdAt(DateTimeUtils.nowUtc())
                .updatedAt(DateTimeUtils.nowUtc())
                .version(0)
                .build();
    }

    /**
     * 產生退款編號
     * 格式：RFD{yyyyMMddHHmmss}{random4}
     */
    private static String generateRefundNo() {
        var now = DateTimeUtils.nowUtc();
        var formatter = java.time.format.DateTimeFormatter.ofPattern("yyyyMMddHHmmss");
        var random = String.format("%04d", (int) (Math.random() * 10000));
        return "RFD" + now.format(formatter) + random;
    }
}
