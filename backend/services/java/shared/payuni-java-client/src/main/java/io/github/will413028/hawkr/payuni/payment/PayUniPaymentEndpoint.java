package io.github.will413028.hawkr.payuni.payment;

/**
 * PayUni 金流 API 端點與版本常數
 */
public final class PayUniPaymentEndpoint {

    private PayUniPaymentEndpoint() {}

    /** UPP 整合式支付頁 v2.0 */
    public static final String PATH_UPP = "/api/upp";
    public static final String VERSION_UPP = "2.0";

    /** ATM 虛擬帳號 v1.3 */
    public static final String PATH_ATM = "/api/atm";
    public static final String VERSION_ATM = "1.3";

    /** Apple Pay v1.3 */
    public static final String PATH_APPLE_PAY = "/api/apple";
    public static final String VERSION_APPLE_PAY = "1.3";

    /** Google Pay v1.3 */
    public static final String PATH_GOOGLE_PAY = "/api/google";
    public static final String VERSION_GOOGLE_PAY = "1.3";

    /** 交易查詢 v2.0 — 依 PayUni 逆向金流 API §2.2 */
    public static final String PATH_QUERY = "/api/trade/query";
    public static final String VERSION_QUERY = "2.0";

    /** 取消授權（信用卡已授權未請款） v1.0 */
    public static final String PATH_TRADE_CANCEL = "/api/trade/cancel";
    public static final String VERSION_TRADE_CANCEL = "1.0";

    /** 退款（關帳） v1.0 */
    public static final String PATH_TRADE_CLOSE = "/api/trade/close";
    public static final String VERSION_TRADE_CLOSE = "1.0";

    /** 非信用卡退款轉匯 v1.0 */
    public static final String PATH_OFFLINE_REFUND = "/api/trade/offline/refund";
    public static final String VERSION_OFFLINE_REFUND = "1.0";

    /** 非信用卡退款轉匯取消 v1.0 */
    public static final String PATH_OFFLINE_CANCEL_REFUND = "/api/trade/offline/cancel_refund";
    public static final String VERSION_OFFLINE_CANCEL_REFUND = "1.0";

    /** 非信用卡退款轉匯（頁面版）v1.0 — 買家自行填寫銀行資訊 */
    public static final String PATH_OFFLINE_REFUND_PAGE = "/api/offline_payment/refund";
    public static final String VERSION_OFFLINE_REFUND_PAGE = "1.0";

    /** 撥款指示 v1.0（代理商級別 API，外層使用 AgentID 而非 MerID） */
    public static final String PATH_GRANT = "/api/trade/grant";
    public static final String VERSION_GRANT = "1.0";

    /**
     * 撥款查詢 v1.0（代理商專用 API 總覽 §6）—— 外層使用 MerID + IsPlatForm=1（走 callApi，非 callAgentApi）。
     * 用於 §5.6 去重硬化 B 重試前對帳，查詢撥款當日入帳明細（每頁 500 筆、限 30 天內）。
     */
    public static final String PATH_GRANT_QUERY = "/api/trade/grant_query";
    public static final String VERSION_GRANT_QUERY = "1.0";
}
