package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.CertificationType;

/**
 * §8.5 CertificationType 業務規則 helper。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 將 CertificationType 內原有的 {@code isVerifiable()} 線上驗證
 * 判斷移到 product-service 內部（單一服務使用）。
 */
public final class CertificationTypeRules {

    private CertificationTypeRules() {
    }

    /**
     * 是否支援線上驗證（農業部 API 可查詢）。
     */
    public static boolean isVerifiable(CertificationType type) {
        return type == CertificationType.ORGANIC
                || type == CertificationType.TAP
                || type == CertificationType.CAS;
    }
}
