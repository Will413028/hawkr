package io.github.will413028.hawkr.shipping.domain;

import io.github.will413028.hawkr.common.utils.code.OnboardingStatus;

/**
 * §8.5 OnboardingStatus 業務規則 helper。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 將 PayUni SetStatus 字串對應內部 OnboardingStatus 的解析邏輯，
 * 從 enum 內移到 shipping-service 內部（單一服務使用）。
 */
public final class OnboardingStatusRules {

    private OnboardingStatusRules() {
    }

    /**
     * 將 PayUni SetStatus 映射為內部狀態。
     * <ul>
     *   <li>1 → ACTIVE</li>
     *   <li>2 → REJECTED</li>
     *   <li>3 → CLOSED</li>
     *   <li>4 → PERMANENTLY_CLOSED</li>
     *   <li>其他（含 9 未處理 / 7 重新送審 / 8 上傳審核中 / null）→ PENDING_REVIEW</li>
     * </ul>
     */
    public static OnboardingStatus fromPayUniSetStatus(String setStatus) {
        if (setStatus == null) {
            return OnboardingStatus.PENDING_REVIEW;
        }
        return switch (setStatus) {
            case "1" -> OnboardingStatus.ACTIVE;
            case "2" -> OnboardingStatus.REJECTED;
            case "3" -> OnboardingStatus.CLOSED;
            case "4" -> OnboardingStatus.PERMANENTLY_CLOSED;
            default -> OnboardingStatus.PENDING_REVIEW;
        };
    }
}
