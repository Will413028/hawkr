package io.github.will413028.hawkr.payuni.http;

/**
 * PayUni 互動 audit hook。
 * <p>
 * 由 payuni-java-client 在每次 PayUni exchange（含成功與各類失敗）結束時觸發**一次**，
 * 各服務提供 persist-only 實作將稽核落庫（payment / shipping 的 {@code payuni_audit_logs} 專表）。
 * <p>
 * 設計目的：金流／物流爭議稽核軌跡，可回溯「我方送了什麼、PayUni 回了什麼、耗時多久、驗簽/解密是否通過」。
 * 本 client 不依賴任何服務的 DB layer，只觸發 hook；落庫端負責遮罩與寫入。
 * <p>
 * 失敗策略：實作端任何例外必須吞掉（由 {@link PayUniHttpClient} 的 safeOnExchange 包覆），不阻斷 PayUni 主流程。
 */
public interface PayUniRequestLogger {

    /** No-op 預設實作（無 logger 注入時使用，不影響行為）。 */
    PayUniRequestLogger NOOP = ctx -> { };

    /**
     * 一次 PayUni exchange（含成功／各類失敗）恰好觸發一次。
     *
     * @param ctx 本次 exchange 的聚合 context（明文業務參數、原始回應、解密內層、傳輸層錯誤、耗時等）
     */
    void onExchange(PayUniExchangeContext ctx);
}
