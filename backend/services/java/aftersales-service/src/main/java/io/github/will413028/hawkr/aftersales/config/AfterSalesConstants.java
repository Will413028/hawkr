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



    /** 超商 7-11 C2C 退貨方式 value（ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP）；只有此 method 會走 26 狀態 */
    public static final String RETURN_SHIPPING_METHOD_CVS = "3002";
    /** 黑貓宅配退貨方式 value（ShippingMethod.TAQBIN）；直接 25→27 */
    public static final String RETURN_SHIPPING_METHOD_TAQBIN = "3001";
}
