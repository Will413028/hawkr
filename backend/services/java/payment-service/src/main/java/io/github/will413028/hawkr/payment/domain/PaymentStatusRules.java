package io.github.will413028.hawkr.payment.domain;

import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;

/**
 * §2A.3.6 / §8.5 PaymentTransactionStatus 業務狀態規則 helper。
 * <p>
 * CodeMeta enum (PaymentTransactionStatus) 為跨語言（Java / Go / Flutter）公共型別，
 * 不得包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。本 helper 將原本寫在 enum 內的
 * isTerminal() / canTransitionTo() 移到此處，作為 payment-service 內部業務規則。
 * <p>
 * 注意：CLOSED_BY_SWITCH 在語意上即終態（不可轉回 INIT/PAID），必須包含在
 * isTerminal() 內，否則 §2A.6 webhook 段 1.b「切換後舊管道完成」分支永遠無法
 * 進入（live e2e 已證實，見 commit 599cdff4 / 1ffc09b0）。
 */
public final class PaymentStatusRules {

    private PaymentStatusRules() {
    }

    /**
     * 是否為終態（不可再變更）。
     */
    public static boolean isTerminal(PaymentTransactionStatus status) {
        return status == PaymentTransactionStatus.PAID
                || status == PaymentTransactionStatus.FAILED
                || status == PaymentTransactionStatus.EXPIRED
                || status == PaymentTransactionStatus.CONFLICT
                || status == PaymentTransactionStatus.CLOSED
                || status == PaymentTransactionStatus.CLOSED_BY_SWITCH;
    }

    /**
     * 檢查是否可以轉換到目標狀態。
     * <p>
     * PAID/FAILED → CONFLICT：Apple Pay / Google Pay 同步結果與 Webhook 結果矛盾時觸發。
     */
    public static boolean canTransitionTo(PaymentTransactionStatus from, PaymentTransactionStatus to) {
        return switch (from) {
            case INIT -> true;
            case PAID, FAILED -> to == PaymentTransactionStatus.CONFLICT;
            default -> false;
        };
    }
}
