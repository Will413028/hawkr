package io.github.will413028.hawkr.aftersales.config;

/**
 * 售後服務內部常數。
 * 僅 PayUni 第三方協定字串保留於此；業務語意值請使用對應 CodeMeta enum：
 * - 稽核日誌操作者：{@code io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor}
 * - 稽核日誌動作：{@code io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction}
 * - 證據類型：{@code io.github.will413028.hawkr.common.utils.code.AfterSaleEvidenceType}
 * - 退款結果：{@code io.github.will413028.hawkr.common.utils.code.RefundStatus}
 */
public final class AfterSalesConstants {

    private AfterSalesConstants() {}

    // ==================== 逆向 leg 分類器（已遷出）====================
    // 🔴 5 個 PAYUNI_RETURN_* 常數於 2026-09-12 依 65 附錄 ADR-013 **整組刪除**。
    //
    // 刪除理由不是「多了一個 Uber 值」，而是那個前綴從一開始就是謊話：它們比對的是
    // **已正規化的 ShippingStatus.value()**，不是 PayUni wire code（PayUni 的映射恰好恆等，
    // 所以謊話至今隱形）。⇒ 加一套 UBER_RETURN_* 是架構上錯誤的修法——Uber 的原生值
    // 在 shipping-service 就被轉成 930／933，**永遠到不了本服務**。
    //
    // 現在的落點：common-core 的 helper/ReturnShippingRules（與正向 leg 的
    // ShippingStatusRules 同層同宿，入參型別是 ShippingStatus 枚舉、對值域全覆蓋）。

    // ==================== 超商退貨識別 ====================

    /** 超商 7-11 C2C 退貨方式 value（ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP）；只有此 method 會走 26 狀態 */
    public static final String RETURN_SHIPPING_METHOD_CVS = "3002";
    /** 黑貓宅配退貨方式 value（ShippingMethod.TAQBIN）；直接 25→27 */
    public static final String RETURN_SHIPPING_METHOD_TAQBIN = "3001";
}
