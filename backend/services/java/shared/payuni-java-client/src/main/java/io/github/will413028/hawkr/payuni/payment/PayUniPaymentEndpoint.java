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

    /**
     * 多功能收付 v1.0（代理商專用 API 總覽 §2）—— 外層使用 AgentID，走 {@code callAgentApi}。
     *
     * <p>🔴 與 {@link #PATH_GRANT} 是<b>方向相反的兩件事</b>：撥款是平台付錢給賣家，
     * 本 API 的正向（{@code BalType=1}）是平台<b>向賣家收</b>。兩者共用同一個 HTTP 通道，
     * 但業務語意完全相反 —— 抄錯一個常數，錢會往反方向流。
     *
     * <p>⚠️ 官方逐字：「實際交易<b>於次日執行</b>」，且回傳<b>只有 Status／Message 兩欄</b>
     * （無單號、無受理序號）⇒ 「已受理」與「已收到錢」是兩件事，中間隔著一個真實存在的狀態。
     */
    public static final String PATH_AGENT_FEE = "/api/agent/agent_fee";
    public static final String VERSION_AGENT_FEE = "1.0";

    /**
     * 多功能收付交易查詢 v1.0（代理商專用 API 總覽 §3）—— 外層使用 AgentID，走 {@code callAgentApi}。
     *
     * <p>查詢的 key 是 <b>(MerID, TradeNo, Amt)</b> 三元組 ⇒ 必須帶<b>當初送出的金額</b>。
     * 回傳含 {@code FundDate}（{@code 0000-00-00} ＝ 尚未撥款）與 {@code Fee}（實際手續費）。
     *
     * <p>🔴 <b>回單筆結構、非陣列</b>，而 {@code TradeNo} 官方三處標「可重複」
     * ⇒ 真的送重複了，用官方 API 也對不出有幾筆。唯一的去重保證只能在我方 DB。
     */
    public static final String PATH_AGENT_FEE_QUERY = "/api/agent/agent_fee_query";
    public static final String VERSION_AGENT_FEE_QUERY = "1.0";
}
