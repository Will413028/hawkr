package io.github.will413028.hawkr.payment.constant;

/**
 * PayUni 業務語意常數（payment-service 內部使用）
 *
 * 注意：API 通訊層的常數（PATH_*、VERSION_*、所有外層欄位名）已統一搬至
 * {@code shared/payuni-java-client} 的 {@link io.github.will413028.hawkr.payuni.payment.PayUniPaymentEndpoint}，
 * payment-service 不再保留任何重複定義。
 *
 * 此 class 只保留**業務邏輯層**會用到的：
 * - PayUni 回應欄位名（用於 webhook 解密後 Map 取值）
 * - PayUni 狀態碼語意（CloseStatus / TradeStatus 對應業務邏輯分支）
 *
 * 對應審計報告 §10：移除所有與 client 通訊相關的硬編碼常數，徹底依賴 shared SDK。
 */
public final class PayUniConstants {

    private PayUniConstants() {}

    // ===== PayUni API 回應狀態值 =====

    /** API 呼叫成功的回應 Status 值 */
    public static final String RESPONSE_SUCCESS = "SUCCESS";

    /**
     * §5.6 撥款去重硬化 A：撥款回應「模稜兩可」時的 grant_status 哨兵值。
     * <p>
     * 裸字串（14 字元 &lt; VARCHAR(20)），<b>非 CodeMeta</b>——{@code grant_status} 欄位是
     * 「{@code SUCCESS} ∪ PayUni 錯誤碼」的異質欄位，不套 CodeMeta（見 05 §5.6 / 收斂規範 §8.2）。
     * 標此值者代表「PayUni 可能已收到撥款指示但回應不可信」，不自動重試，且因非 SUCCESS
     * 自動進 Admin 異常清單，下輪重試由 grant_query 對帳把關（去重硬化 B）。
     */
    public static final String PENDING_CONFIRM = "PENDING_CONFIRM";

    // ===== PayUni 回應欄位名（用於 webhook 解密後 Map.get） =====

    /** 商店訂單編號（系統交易編號） */
    public static final String FIELD_MER_TRADE_NO = "MerTradeNo";

    /** PayUni 交易編號 */
    public static final String FIELD_TRADE_NO = "TradeNo";

    /** 交易狀態碼 */
    public static final String FIELD_TRADE_STATUS = "TradeStatus";

    /** 信用卡請款狀態欄位名 */
    public static final String FIELD_CLOSE_STATUS = "CloseStatus";

    /** 回應訊息欄位 */
    public static final String FIELD_MESSAGE = "Message";

    /** §10.5 信用卡 webhook 授權日期（yyyyMMdd UTC+8） */
    public static final String FIELD_AUTH_DAY = "AuthDay";
    /** §10.5 信用卡 webhook 授權時間（HHmmss UTC+8） */
    public static final String FIELD_AUTH_TIME = "AuthTime";
    /** §10.5 ATM/愛金卡/LinePay query webhook 付款時間（yyyy-MM-dd HH:mm:ss UTC+8） */
    public static final String FIELD_PAYMENT_DAY = "PaymentDay";
    /** §10.5 PayUni 交易建立時間（yyyy-MM-dd HH:mm:ss UTC+8，query API 才有） */
    public static final String FIELD_CREATE_DAY = "CreateDay";
    /** §10.5 PayUni 付款方式代碼（webhook payload，1=信用卡 2=ATM ...） */
    public static final String FIELD_PAYMENT_TYPE = "PaymentType";

    // ===== 信用卡請款狀態語意（PayUni CloseStatus）=====

    /** 1=請款申請中 */
    public static final String CLOSE_STATUS_APPLYING = "1";
    /** 2=請款成功 */
    public static final String CLOSE_STATUS_SUCCESS = "2";
    /** 3=請款取消 */
    public static final String CLOSE_STATUS_CANCELED = "3";
    /** 7=請款處理中 */
    public static final String CLOSE_STATUS_PROCESSING = "7";
    /** 9=未申請 */
    public static final String CLOSE_STATUS_NOT_APPLIED = "9";

    // ===== PayUni TradeStatus 狀態碼語意 =====

    /** 1=交易成功 */
    public static final int TRADE_STATUS_SUCCESS = 1;
}
