package io.github.will413028.hawkr.payment.logistics.domain;

/**
 * 某個責任成因下，這筆物流額外成本最終算誰的（ADR-004）。
 * <p>
 * ⚠️ <b>刻意不進 CodeMeta</b>：它是從 {@code ResponsibilityCause} 純推導出來的值，
 * <b>沒有任何欄位存它</b>（V9 的 {@code logistics_adjustments} 只存 {@code responsibility_cause}）。
 * 另存一份碼表就會有兩個真實來源、必然漂移——與「{@code dispute_hold} 不落欄」同一條理由。
 * 哪天它真的需要落庫或跨服務暴露，那時才升格進 CodeMeta。
 */
public enum Bearer {

    /** 由賣家的物流成本承擔。 */
    SELLER,

    /** 由平台承擔，<b>不得轉嫁賣家</b>。 */
    PLATFORM,

    /** 先進爭議／對帳，🔴 <b>不得自動向賣家收</b>。 */
    DISPUTE
}
