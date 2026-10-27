package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * {@code uber_quotes.status} 的值域（66 §5.1.4，DB {@code chk_uber_quotes_status} 守）。
 *
 * <h2>🔴 為什麼是常數而不是 CodeMeta enum</h2>
 * 8 檔 §8.5 要求<b>共享</b>代碼值走 CodeMeta。這五個值<b>不跨服務</b>：
 * 它們只出現在 shipping-service 的 {@code uber_quotes} 那一欄，
 * 沒有任何前端、Gateway 或其他服務會看到或送出它們
 * （對外暴露的是報價本身與它的有效期，不是這個內部狀態）。
 * ⇒ 放進 CodeMeta 會讓四份前端拷貝多出五個永遠用不到的值。
 *
 * <p>⚠️ 判準是「這個值會不會離開 shipping-service」。哪天它出現在 Gateway 的回應裡，
 * 這段註解就失效，該搬進 CodeMeta。
 *
 * <h2>allowed-from（66 §5.1.4）</h2>
 * <ul>
 *   <li>{@link #CHECKOUT_ACCEPTED} ← 只允許 {@link #CREATED}</li>
 *   <li>{@link #CONSUMED} ← 🔴 <b>只允許</b> {@link #CHECKOUT_ACCEPTED}
 *       ——from {@link #CREATED} 代表跳過了結帳確認（66 §4.1 ②）</li>
 *   <li>{@link #SUPERSEDED} ← 只允許 {@link #CREATED}</li>
 *   <li>{@link #EXPIRED} ← 只允許 {@link #CREATED}（TTL 掃描，P3）</li>
 * </ul>
 */
public final class UberQuoteStatus {

    /** 報價成立，尚未被結帳確認。 */
    public static final String CREATED = "CREATED";

    /** 結帳確認通過（TTL ＋ hash 重驗），可被建單消費。 */
    public static final String CHECKOUT_ACCEPTED = "CHECKOUT_ACCEPTED";

    /** TTL 到期（P3 掃描）。🔴 只改 status 不刪列——帳務與客訴要查得到報價歷史。 */
    public static final String EXPIRED = "EXPIRED";

    /** 建單成功，已消費。 */
    public static final String CONSUMED = "CONSUMED";

    /** 購物車變動後重報，舊的換手。 */
    public static final String SUPERSEDED = "SUPERSEDED";

    private UberQuoteStatus() {
    }
}
