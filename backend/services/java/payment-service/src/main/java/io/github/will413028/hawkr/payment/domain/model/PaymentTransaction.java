package io.github.will413028.hawkr.payment.domain.model;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.Accessors;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;

import java.time.LocalDateTime;

/**
 * 支付交易領域模型
 *
 * 狀態轉換規則：
 * - INIT → PAID, FAILED, EXPIRED, CONFLICT, CLOSED
 * - 所有終態不可再變更
 */
@Getter
@Accessors(fluent = true)
@AllArgsConstructor
@Builder
public class PaymentTransaction {

    private Long id;
    private String tradeNo;
    private String orderId;
    private int attemptSeq;
    private String merId;
    private String buyerId;

    private PaymentMethod paymentType;
    private long amount;
    private String productName;
    private PaymentTransactionStatus status;

    private String providerTradeNo;
    private String providerStatus;
    private String providerMessage;

    // 撥款欄位（收斂規範 §6.2 封閉規則：僅 Admin API 可暴露）
    private LocalDateTime grantedAt;
    private String grantStatus;
    private String grantMessage;

    // ATM 專用欄位
    private String atmBankCode;
    private String atmVirtualAccount;
    private java.time.LocalDate atmExpireDate;

    private LocalDateTime payTime;
    private LocalDateTime closedAt;
    private LocalDateTime expireAt;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    // L2 補救路徑欄位（PendingPaymentReconcileScheduler）
    private LocalDateTime lastReconciledAt;
    private LocalDateTime rescuedAt;
    private String rescueAction; // REFUNDED / REFUND_PENDING_MANUAL

    // §2A.6 重複付款處理欄位
    private boolean isDuplicate;
    private Long duplicateOfPaymentId;
    private Integer duplicateSequence;

    // §10.5 PayUni 真實時間軸
    private LocalDateTime providerCreateAt;
    private LocalDateTime providerPaidAt;

    // §2A.5.4 ATM 復活機制
    private LocalDateTime reactivatedAt;

    // §5.9 PayUni 回調最終出口路由（CodeMeta ClientType / AppType value）
    private String clientType;
    private String appType;

    private int version;

    /**
     * 是否為幕後即時付款方式（同步返回結果）
     */
    public boolean isDirectPayment() {
        return paymentType == PaymentMethod.APPLE_PAY || paymentType == PaymentMethod.GOOGLE_PAY;
    }

    /**
     * 是否為幕前導頁付款方式
     */
    public boolean isRedirectPayment() {
        return paymentType == PaymentMethod.CREDIT_CARD;
    }

    /**
     * 是否為幕後取號付款方式
     */
    public boolean isVoucherPayment() {
        return paymentType == PaymentMethod.ATM;
    }

    /**
     * 檢查是否可以轉換到目標狀態
     */
    public boolean canTransitionTo(PaymentTransactionStatus targetStatus) {
        return PaymentStatusRules.canTransitionTo(status, targetStatus);
    }

    /**
     * 是否已付款
     */
    public boolean isPaid() {
        return status == PaymentTransactionStatus.PAID;
    }

    /**
     * 是否為終態
     */
    public boolean isTerminal() {
        return PaymentStatusRules.isTerminal(status);
    }

    /**
     * 是否可退款（僅 PAID 狀態可退款）
     */
    public boolean isRefundable() {
        return status == PaymentTransactionStatus.PAID;
    }

    /**
     * 是否可關閉（僅 INIT 狀態可關閉）
     */
    public boolean isCloseable() {
        return status == PaymentTransactionStatus.INIT;
    }

    /**
     * 建立新交易的工廠方法
     *
     * tradeNo = orderId + attemptSeq（3位序號），作為 PayUni MerTradeNo。
     * 每次付款嘗試使用不同 MerTradeNo，繞開 PayUni C1 約束。
     * 詳見 §2A.3.6 MerTradeNo 序列化。
     */
    /**
     * @param expireAt 過期時間。信用卡/Apple Pay/Google Pay 由 ApplicationService 設 nowUtc + 600s；
     *                 ATM 由 PayUni /api/atm response 的 ExpireDate 後續更新；此處可傳 null。
     */
    public static PaymentTransaction create(
            String orderId,
            int attemptSeq,
            String merId,
            String buyerId,
            PaymentMethod paymentType,
            long amount,
            String productName,
            LocalDateTime expireAt,
            String clientType,
            String appType) {

        String tradeNo = orderId + String.format("%03d", attemptSeq);
        LocalDateTime now = DateTimeUtils.nowUtc();

        return PaymentTransaction.builder()
                .id(TsidUtil.generate())
                .tradeNo(tradeNo)
                .orderId(orderId)
                .attemptSeq(attemptSeq)
                .merId(merId)
                .buyerId(buyerId)
                .paymentType(paymentType)
                .amount(amount)
                .productName(productName)
                .status(PaymentTransactionStatus.INIT)
                .expireAt(expireAt)
                .clientType(clientType)
                .appType(appType)
                .createdAt(now)
                .updatedAt(now)
                .version(0)
                .build();
    }

    /**
     * 建立零元訂單交易（§2A.1）
     * <p>
     * 全額折抵後實付 0 元，不呼叫 PayUni，直接以 PAID 狀態落地。
     * paymentType 固定為 ZERO_AMOUNT，amount 固定為 0，無 provider_trade_no。
     */
    public static PaymentTransaction createZeroAmount(
            String orderId,
            int attemptSeq,
            String merId,
            String buyerId,
            String productName,
            String clientType,
            String appType) {

        String tradeNo = orderId + String.format("%03d", attemptSeq);
        LocalDateTime now = DateTimeUtils.nowUtc();
        return PaymentTransaction.builder()
                .id(TsidUtil.generate())
                .tradeNo(tradeNo)
                .orderId(orderId)
                .attemptSeq(attemptSeq)
                .merId(merId)
                .buyerId(buyerId)
                .paymentType(PaymentMethod.ZERO_AMOUNT)
                .amount(0L)
                .productName(productName)
                .status(PaymentTransactionStatus.PAID)
                .payTime(now)
                .clientType(clientType)
                .appType(appType)
                .createdAt(now)
                .updatedAt(now)
                .version(0)
                .build();
    }
}
