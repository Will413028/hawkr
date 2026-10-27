package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * Uber 呼叫失敗的分類 —— 66 §4.5.4 錯誤碼映射表的程式形式。
 *
 * <h2>🔴 {@link #DETERMINISTIC} 與 {@link #NON_DETERMINISTIC} 的分流是本檔唯一真正重要的一格</h2>
 * 66 §4.5.4 逐字：「把 5xx 當 990 會導致『其實建成功了但我們以為失敗』⇒ Admin 重建
 * ⇒ <b>兩趟車、重複計費</b>」（陷阱 A8）。兩者的差別<b>不是</b>嚴重程度，是
 * 「對方那邊到底有沒有產生副作用」：
 * <ul>
 *   <li>{@code DETERMINISTIC}（400）：請求被對方看懂了並拒絕 ⇒ <b>確定沒有建出單</b>
 *       ⇒ 重送同一份 payload 只會再錯一次 ⇒ {@code CREATE_FAILED(990)}</li>
 *   <li>{@code NON_DETERMINISTIC}（5xx／timeout／IO）：<b>不知道</b>對方收到了沒
 *       ⇒ {@code CREATE_UNKNOWN(991)} ＋ 回查收斂（66 §7.7）</li>
 * </ul>
 *
 * <p>⚠️ 這個 enum <b>刻意不提供</b>「kind → ShippingStatus」的轉換方法。
 * 「這次失敗要把單子寫成哪個狀態」是業務判斷，屬狀態機；adapter 的硬規則 1 是
 * 「只做翻譯，不做業務判斷」。轉換寫在 {@link UberDirectAdapter} 的建單路徑上，
 * 且只有建單那一條路徑需要它（取消／回查／POD 的失敗都是「留原狀態，下個 tick 再來」）。
 */
public enum UberFailureKind {

    /**
     * 其他 400：確定性失敗，不重試（66 §4.5.4「其他 400 ⇒ 確定性 ⇒ 不重試 ⇒ CREATE_FAILED(990)」）。
     */
    DETERMINISTIC,

    /**
     * 404：查無此件。
     *
     * <p>兩個來源：① E6 取消時訂單已在終態或從未存在（API 總覽 §4 官方逐字）
     * ② 🔴 {@code 991} 誤用 E4 回查（沒有 {@code delivery_id}）⇒ 每次 404、永不收斂（66 陷阱 B2）。
     * 分出來而不是併進 {@link #DETERMINISTIC}，是因為 66 §7.7 對「查到不存在」有專屬處置（→ 990）。
     */
    NOT_FOUND,

    /**
     * {@code customer_blocked}（403）：66 §4.5.4 ⇒ 寫 {@code uber_store_mapping.blocked=true}／
     * {@code tax_form_required=true} ＋ 建 case ＋ 通知賣家。
     *
     * <p>⚠️ 與 {@link #AUTH} 分開：它是<b>這個門市</b>被擋，不是我方憑證有問題；
     * 併在一起會讓「某個賣家要補稅務表單」長得像「我們被停權」。
     */
    CUSTOMER_BLOCKED,

    /**
     * {@code noncancelable_delivery}（400）：取消失敗、<b>留原狀態</b>（不寫 {@code 992 → 41}），
     * 回 65 {@code 9522}（66 §4.5.4／§7.6）。
     *
     * <p>🔴 不得因為它是 400 就併進 {@link #DETERMINISTIC}：後者在建單路徑上會寫 {@code 990}，
     * 而這一格的正確處置是<b>什麼都不寫</b>——貨還在送。
     */
    NONCANCELABLE,

    /**
     * 401／其他 403：66 §4.5.4 ⇒ 斷路器計數 ⇒ 選項 {@code available=false}。
     */
    AUTH,

    /** 429：退避重試；連續發生開斷路器（66 §4.5.4）。 */
    RATE_LIMITED,

    /**
     * 5xx／timeout／IO：<b>非確定性</b> ⇒ 重試耗盡後 {@code CREATE_UNKNOWN(991)}，<b>不是 990</b>。
     */
    NON_DETERMINISTIC;

    /**
     * 這一類失敗值不值得重試。
     *
     * <p>🔴 {@link #NON_DETERMINISTIC} 重試的是「有副作用的呼叫」——重試次數因此必須小
     * （66 §0.3：Create 2 次／2s・6s），而且要搭配我方 pre-commit 鍵（{@code ADR-014}），
     * <b>不依賴 Uber 官方冪等</b>（官方文件對 idempotency 零命中）。
     */
    public boolean retryable() {
        return this == NON_DETERMINISTIC || this == RATE_LIMITED;
    }
}
