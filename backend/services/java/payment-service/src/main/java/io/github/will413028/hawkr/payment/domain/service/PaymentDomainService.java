package io.github.will413028.hawkr.payment.domain.service;

import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.Optional;

/**
 * 支付領域服務
 *
 * 負責處理跨聚合的業務邏輯和狀態轉換規則
 */
public class PaymentDomainService {

    /**
     * 判斷交易是否可以更新為付款成功
     *
     * 規則：只有 INIT 狀態可以轉換到 PAID
     */
    public boolean canMarkAsPaid(PaymentTransaction transaction) {
        return transaction.status() == PaymentTransactionStatus.INIT;
    }

    /**
     * 判斷交易是否可以更新為付款失敗
     *
     * 規則：只有 INIT 狀態可以轉換到 FAILED
     */
    public boolean canMarkAsFailed(PaymentTransaction transaction) {
        return transaction.status() == PaymentTransactionStatus.INIT;
    }

    /**
     * 判斷交易是否可以關閉
     *
     * 規則：只有 INIT 狀態可以轉換到 CLOSED
     */
    public boolean canClose(PaymentTransaction transaction) {
        return transaction.status() == PaymentTransactionStatus.INIT;
    }

    /**
     * 判斷交易是否可以標記為逾期
     *
     * 規則：只有 INIT 狀態且已超過逾期時間
     */
    public boolean canMarkAsExpired(PaymentTransaction transaction) {
        if (transaction.status() != PaymentTransactionStatus.INIT) {
            return false;
        }
        if (transaction.expireAt() == null) {
            return false;
        }
        return DateTimeUtils.nowUtc().isAfter(transaction.expireAt());
    }

    /**
     * 判斷交易是否可以標記為衝突
     *
     * 規則：只有 PAID 或 FAILED 可以轉換到 CONFLICT
     * 適用場景：Apple Pay / Google Pay 同步終態與 Webhook 結果矛盾
     */
    public boolean canMarkAsConflict(PaymentTransaction transaction) {
        return transaction.status() == PaymentTransactionStatus.PAID
                || transaction.status() == PaymentTransactionStatus.FAILED;
    }

    /**
     * 檢測同步 API 與 Webhook 結果是否衝突
     *
     * @param syncStatus    同步 API 回傳的狀態
     * @param webhookStatus Webhook 回傳的狀態
     * @return 如果衝突返回 true
     */
    public boolean isConflict(PaymentTransactionStatus syncStatus, PaymentTransactionStatus webhookStatus) {
        // 兩者都是終態但不一致，視為衝突
        if (PaymentStatusRules.isTerminal(syncStatus) && PaymentStatusRules.isTerminal(webhookStatus)) {
            return syncStatus != webhookStatus;
        }
        return false;
    }

    /**
     * 建立付款成功事件
     */
    public PaymentDomainEvent.PaymentSucceeded createPaymentSucceededEvent(
            PaymentTransaction transaction,
            LocalDateTime payTime
    ) {
        return new PaymentDomainEvent.PaymentSucceeded(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.buyerId(),
                transaction.amount(),
                transaction.providerTradeNo(),
                transaction.paymentType() != null ? transaction.paymentType().value() : "",
                payTime,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 建立付款失敗事件
     */
    public PaymentDomainEvent.PaymentFailed createPaymentFailedEvent(
            PaymentTransaction transaction,
            String providerMessage
    ) {
        return new PaymentDomainEvent.PaymentFailed(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.buyerId(),
                transaction.amount(),
                providerMessage,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 建立付款逾期事件
     */
    public PaymentDomainEvent.PaymentExpired createPaymentExpiredEvent(PaymentTransaction transaction) {
        return new PaymentDomainEvent.PaymentExpired(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.buyerId(),
                transaction.amount(),
                transaction.expireAt(),
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 建立交易關閉事件
     */
    public PaymentDomainEvent.PaymentClosed createPaymentClosedEvent(
            PaymentTransaction transaction,
            LocalDateTime closedAt
    ) {
        return new PaymentDomainEvent.PaymentClosed(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.buyerId(),
                transaction.amount(),
                closedAt,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 建立狀態衝突事件
     */
    public PaymentDomainEvent.PaymentConflict createPaymentConflictEvent(
            PaymentTransaction transaction,
            String syncResult,
            String webhookResult
    ) {
        return new PaymentDomainEvent.PaymentConflict(
                transaction.tradeNo(),
                transaction.orderId(),
                transaction.buyerId(),
                transaction.amount(),
                syncResult,
                webhookResult,
                DateTimeUtils.nowUtc()
        );
    }

    /**
     * 解析 PayUni TradeStatus 到內部狀態
     *
     * PayUni TradeStatus 對應：
     * - 1: 交易成功 → PAID
     * - 2: 交易失敗 → FAILED
     * - 0: 處理中 → INIT（保持原狀）
     */
    public Optional<PaymentTransactionStatus> parseProviderStatus(String providerStatus) {
        if (providerStatus == null) {
            return Optional.empty();
        }
        return switch (providerStatus) {
            case "1" -> Optional.of(PaymentTransactionStatus.PAID);
            case "2" -> Optional.of(PaymentTransactionStatus.FAILED);
            case "0" -> Optional.of(PaymentTransactionStatus.INIT);
            default -> Optional.empty();
        };
    }

    // §10.4 D3 修正：calculateAtmExpireTime() 已刪除，改用 PayUni 回的 ExpireDate（單一真實來源）
}
