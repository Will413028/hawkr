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

    // ==================== PayUni 退款頁面回傳狀態（第三方協定，非 CodeMeta） ====================

    public static final String PAYUNI_REFUND_PAGE_SUCCESS = "SUCCESS";
    public static final String PAYUNI_REFUND_PAGE_FAIL = "FAIL";

    /**
     * PayUni 退款錯誤碼「無足夠金額可退款」（可退餘額不足，強烈代表該筆已退過）。
     * 出現此碼不得 revert 讓買家重填，改對帳 payment 端實際狀態（issue #1288 / 09 §5.4.3.1）。
     */
    public static final String PAYUNI_REFUND_NO_BALANCE = "REF02037";
}
