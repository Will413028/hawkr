package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;

/**
 * 物流額外成本的責任歸屬規則（ADR-004）。
 * <p>
 * 🔴 <b>碼值本身住在 CodeMeta</b>（{@link ResponsibilityCause}）。本類別只放歸屬表——
 * CodeMeta enum 不得承載業務輔助邏輯（10 檔 §8.5）。
 * <p>
 * 🔴 依成因判定，不依費用名稱——同一筆「取消費」在賣家改地址與外送員失聯兩種情況下
 * 歸屬完全不同。判定原料不足時一律落 {@code PROVIDER_CAUSED}（先進爭議），
 * <b>不是</b>落「先不記帳」：後者讓一筆真實發生的成本從帳上消失。
 */
public final class ResponsibilityRules {

    private ResponsibilityRules() {
    }

    /**
     * ADR-004 的責任歸屬表，<b>exhaustive</b>。
     * <p>
     * 🔴 {@code BUYER_CAUSED} 仍歸賣家的物流成本：買家已經完成一次付款，平台<b>不得</b>向買家
     * 二次收款（ADR-004）。這一格最容易被直覺讀反。
     * <p>
     * ⚠️ 這裡刻意用<b>沒有 default 的 switch expression</b>：CodeMeta 新增一個成因而未在此
     * 指派負擔方時，payment-service 會在<b>編譯期</b>失敗（同一個 reactor 一起 build），
     * 而不是靜默落到某個預設值。就算只換 jar 不重編，JVM 也會丟
     * {@link MatchException}——兩條路都是 fail-loud，沒有一條是沉默的。
     *
     * @throws IllegalArgumentException {@code cause} 為 null
     */
    public static Bearer bearerOf(ResponsibilityCause cause) {
        if (cause == null) {
            throw new IllegalArgumentException("logistics: responsibilityCause 不得為 null");
        }
        return switch (cause) {
            case SELLER_CAUSED -> Bearer.SELLER;
            case BUYER_CAUSED -> Bearer.SELLER;
            case PLATFORM_CAUSED -> Bearer.PLATFORM;
            case PROVIDER_CAUSED -> Bearer.DISPUTE;
        };
    }

    /**
     * 這個成因的額外成本是否可以自動向賣家收。
     * <p>
     * 🔴 只有 {@link Bearer#SELLER} 為 true。{@code DISPUTE} 回 false 是本規則的實質內容——
     * 「判定原料不足就先進爭議」如果沒有人擋住自動收款，那條 fail-closed 等於不存在。
     */
    public static boolean chargeableToSeller(ResponsibilityCause cause) {
        return bearerOf(cause) == Bearer.SELLER;
    }
}
