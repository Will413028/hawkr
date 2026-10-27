package io.github.will413028.hawkr.payment.domain;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;

/**
 * §8.5 RefundStatus 業務狀態規則 helper。
 * <p>
 * CodeMeta enum (RefundStatus) 為跨語言公共型別，不得包含業務輔助邏輯
 * （docs/開發規範/Java 微服務開發規範.md §6）。本 helper 將原本寫在 enum 內的 isTerminal() /
 * canTransitionTo() 移到此處，作為 payment-service 內部業務規則。
 * <p>
 * 狀態轉換規則：
 * <ul>
 *   <li>REQUESTED → PROCESSING, FAILED</li>
 *   <li>PROCESSING → SUCCEEDED, FAILED</li>
 *   <li>終態：SUCCEEDED, FAILED</li>
 * </ul>
 */
public final class RefundStatusRules {

    private RefundStatusRules() {
    }

    /**
     * 是否為終態。
     */
    public static boolean isTerminal(RefundStatus status) {
        return status == RefundStatus.SUCCEEDED || status == RefundStatus.FAILED;
    }

    /**
     * 檢查是否可以轉換到目標狀態。
     */
    public static boolean canTransitionTo(RefundStatus from, RefundStatus to) {
        return switch (from) {
            case REQUESTED -> to == RefundStatus.PROCESSING || to == RefundStatus.FAILED;
            case PROCESSING -> to == RefundStatus.SUCCEEDED || to == RefundStatus.FAILED;
            default -> false;
        };
    }
}
