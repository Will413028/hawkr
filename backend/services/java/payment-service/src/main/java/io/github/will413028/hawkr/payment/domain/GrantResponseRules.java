package io.github.will413028.hawkr.payment.domain;

import java.util.Set;
import java.util.function.BooleanSupplier;

/**
 * §5.6 撥款去重硬化（A：收緊重試 / B：grant_query 對帳）純函式 helper。
 * <p>
 * 依 docs/開發規範/Java 微服務開發規範.md §6：分類/比對邏輯不放進 CodeMeta enum、不含 I/O、無狀態、無 side effect；
 * 對齊既有 {@link PaymentStatusRules} / {@link RefundStatusRules} 慣例。
 */
public final class GrantResponseRules {

    private GrantResponseRules() {
    }

    /**
     * 撥款失敗回應的三分類（§5.6 去重硬化 A + §5.6.1 退款接管）。
     */
    public enum GrantResponseClass {
        /**
         * 明確未發動：PayUni 回傳「可信的簽章業務拒絕（拒絕碼 / 參數錯）」→ PayUni 未受理該撥款
         * → 可安全重試（§8.1 最多一次）。
         */
        DEFINITELY_NOT_SENT,
        /**
         * 模稜兩可：transport 失敗（timeout / IO / HTTP_ERROR / EMPTY_RESPONSE）或不可信回應
         * （HASH_INVALID 驗簽失敗 / PARSE_ERROR 解析失敗）→ PayUni <b>可能已收到撥款指示</b>
         * → 不自動重試、標 {@code PENDING_CONFIRM}，下輪由 grant_query 對帳把關。
         */
        AMBIGUOUS,
        /**
         * 已由 PayUni 接管（§5.6.1）：可信的 PayUni 拒絕 <b>且</b> 該交易存在已完成退款
         * （主判準，碼不可知；GRANT30023/30018/30019 僅為線索碼）→ PayUni 已對退款交易主動撥款、
         * 不受理補發指示 → <b>不重試、不進 {@code PENDING_CONFIRM} 對帳迴圈</b>，
         * {@code grant_status} 落 PayUni 回應碼原樣、gRPC 以<b>正常回應</b>帶
         * {@code grant_outcome=PROVIDER_TOOK_OVER}；Order 端 Worker 據此收斂 task 為
         * {@code PROVIDER_AUTO}（03 §3.11.2）。
         * 與 {@code ReconcileOutcome.HIT_ALREADY_GRANTED}（grant_query 對帳結果分類）為不同機制、不合流。
         */
        PROVIDER_TOOK_OVER
    }

    /** gRPC 回應 {@code grant_outcome}：撥款指示成功發動（proto {@code GrantPaymentResponse.grant_outcome}）。 */
    public static final String GRANT_OUTCOME_GRANTED = "GRANTED";

    /** gRPC 回應 {@code grant_outcome}：PayUni 已對退款交易接管撥款（§5.6.1，Order 端據此收斂 PROVIDER_AUTO）。 */
    public static final String GRANT_OUTCOME_PROVIDER_TOOK_OVER = "PROVIDER_TOOK_OVER";

    /**
     * 「模稜兩可」代碼集合（皆為 transport 層失敗或不可信回應，PayUni 可能已收到指示）：
     * <ul>
     *   <li>{@code IO_ERROR} / {@code IO_EXCEPTION} / {@code HTTP_ERROR} / {@code EMPTY_RESPONSE}
     *       —— {@code PayUniApiException.getErrorCode()}（timeout / 連線中斷 / 非 2xx / 空回應）</li>
     *   <li>{@code HASH_INVALID} / {@code PARSE_ERROR}
     *       —— {@code PayUniHttpClient.parseResponse} 合成的不可信 status（驗簽 / 解析失敗）</li>
     * </ul>
     */
    private static final Set<String> AMBIGUOUS_CODES = Set.of(
            "IO_ERROR", "IO_EXCEPTION", "HTTP_ERROR", "EMPTY_RESPONSE",
            "PARSE_ERROR", "HASH_INVALID");

    /**
     * 依失敗代碼分類撥款回應（§5.6 去重硬化 A + §5.6.1 退款接管第三類）。
     * <p>
     * code 來源：非成功 {@code PayUniApiResponse.status()}（PARSE_ERROR / HASH_INVALID / PayUni 錯誤碼），
     * 或捕捉的 {@code PayUniApiException.getErrorCode()}（IO / HTTP_ERROR / EMPTY_RESPONSE）。
     * <p>
     * 判準（依序）：
     * <ol>
     *   <li>空 / transport / 不可信碼（{@link #AMBIGUOUS_CODES}）→ 模稜兩可（無可信的 PayUni 決定，
     *       不能斷言接管；fail-safe 不重試，交 grant_query 對帳）——<b>不評估</b> supplier</li>
     *   <li>可信的 PayUni 拒絕 + 該交易存在已完成退款（<b>碼不可知主判準</b>，§5.6.1）→ 已由 PayUni
     *       接管（不重試）。判準以 {@link BooleanSupplier} <b>延遲取得</b>：僅遇「確定拒絕候選碼」
     *       才評估（由呼叫端查 payment_refunds），本類自身仍不含 I/O</li>
     *   <li>可信的 PayUni 拒絕 + 無已完成退款 → 明確未發動（可重試；涵蓋 GRANT30023 的
     *       「付款當日請款批次未結」時間性混淆場景）</li>
     * </ol>
     *
     * @param hasCompletedRefund 延遲判準：該交易是否存在已完成退款（payment_refunds 有 SUCCEEDED 記錄）
     */
    public static GrantResponseClass classifyFailure(String code, BooleanSupplier hasCompletedRefund) {
        if (code == null || code.isBlank()
                || AMBIGUOUS_CODES.contains(code.trim().toUpperCase())) {
            return GrantResponseClass.AMBIGUOUS;
        }
        return hasCompletedRefund.getAsBoolean()
                ? GrantResponseClass.PROVIDER_TOOK_OVER
                : GrantResponseClass.DEFINITELY_NOT_SENT;
    }

    /**
     * §5.6 去重硬化 B：判斷 grant_query 某筆 Result 是否命中本交易的撥款（已入帳）。
     * <p>
     * 以 PayUni UNi 序號（{@code TradeNo}，全域唯一）精確比對 {@code provider_trade_no}：命中即代表
     * PayUni 已對本交易執行撥款（grant_query 回傳的即撥款入帳明細；出帳必以入帳為前提，故任何一筆
     * 命中皆代表撥款已發生），因此不得重發。
     * <p>
     * <b>刻意不以 Type（入帳/出帳）與 FundsAmt（入帳淨額）為否決條件</b>：PayUni 未於官方文件明列
     * 該兩欄之代碼/淨額語意（{@code FundsAmt} 疑為扣費後淨額，{@code Type} 無代碼表），以其作為必要
     * 條件會產生 false-negative（誤判「未撥」）→ 重複撥款（本功能正要消除的最大風險）。故以唯一鍵
     * {@code TradeNo} 判定（fail-safe 傾向「已撥款、不重發」），Type / 金額由呼叫端記 log 供稽核與後續收緊。
     *
     * @param entryTradeNo     grant_query Result 的 TradeNo（UNi 序號）
     * @param providerTradeNo  本交易的 provider_trade_no（UNi 序號）
     */
    public static boolean isReconciliationHit(String entryTradeNo, String providerTradeNo) {
        return providerTradeNo != null && !providerTradeNo.isBlank()
                && providerTradeNo.equals(entryTradeNo);
    }

    /**
     * PayUni {@code grant_query} 外層 Status「查無符合撥款資料」——該查詢日無任何撥款入帳明細。
     * <p>
     * 這是 grant_query 對「窗口內某日無撥款」的<b>常態、有效空結果</b>（非失敗）：撥款「隔日入帳」，
     * 首次發動嘗試日當天對本交易本就恆空，PayUni 即以此碼回應。對帳須視為<b>該日有效空頁（count=0）</b>
     * 並續掃下一日，切勿當成查詢失敗（否則逐日對帳第一天就誤判 INCONCLUSIVE、抵達入帳日前即返回，
     * 對帳自動判定退化成永遠 PENDING_CONFIRM 需人工——issue #1329）。
     */
    public static final String GRANT_QUERY_NO_DATA = "QUERY03002";

    /**
     * PayUni {@code grant_query} 外層 Status「查詢日期須為 30 天內」——查詢窗超限。
     * <p>
     * 與 {@link #GRANT_QUERY_NO_DATA} 不同：這是<b>查詢未能有效完成</b>（區間未查完），不可推進為「未撥」，
     * 對帳須維持 INCONCLUSIVE、不盲目重發（比照 transport 錯誤）。
     */
    public static final String GRANT_QUERY_OUT_OF_WINDOW = "QUERY02004";

    /**
     * §5.6 去重硬化 B：判斷 grant_query 某查詢日「是否已有效查完」（結果可信、可據以推進對帳，無論有無命中）。
     * <p>
     * {@code true} 者代表該日已取得可信結果，呼叫端可繼續（有資料則比對 Result[]、空日則續掃下一日）：
     * <ul>
     *   <li>{@code isSuccess=true}（{@code Status=SUCCESS}）——正常回應（Result[] 有資料或空）</li>
     *   <li>{@code Status=}{@link #GRANT_QUERY_NO_DATA}（{@code QUERY03002} 查無符合撥款資料）——
     *       PayUni 對「該日無撥款」的常態回應，為<b>有效空頁（count=0）</b>，非失敗</li>
     * </ul>
     * {@code false}（{@link #GRANT_QUERY_OUT_OF_WINDOW} 超 30 天窗 / transport 錯誤 / 其他非預期 Status）
     * →「未查完」→ 呼叫端維持 INCONCLUSIVE、不盲目重發。
     *
     * @param isSuccess grant_query 回應是否 {@code Status=SUCCESS}（{@code PayUniApiResponse.isSuccess()}）
     * @param status    grant_query 回應外層 Status（非 SUCCESS 時據此分辨「有效空日」vs「未查完」）
     */
    public static boolean isGrantQueryDayComplete(boolean isSuccess, String status) {
        if (isSuccess) {
            return true;
        }
        return status != null && GRANT_QUERY_NO_DATA.equalsIgnoreCase(status.trim());
    }
}
