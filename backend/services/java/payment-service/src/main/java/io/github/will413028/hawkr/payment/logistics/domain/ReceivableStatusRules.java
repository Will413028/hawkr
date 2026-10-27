package io.github.will413028.hawkr.payment.logistics.domain;

import java.util.Comparator;
import java.util.EnumMap;
import java.util.EnumSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;

/**
 * 賣家物流應收狀態機的業務規則（70 §3.2）。
 * <p>
 * 🔴 <b>碼值本身住在 CodeMeta</b>（{@link LogisticsReceivableStatus}，SSOT ＝
 * {@code docs/代碼管理（CodeMeta）/業務狀態碼.md}）。本類別只放狀態機規則——
 * CodeMeta enum 不得承載業務輔助邏輯（10 檔 §8.5），所以 allowed-from／終態判定
 * 落在服務側的 {@code *Rules} helper，與 {@code PaymentStatusRules} 同一個形狀。
 * <p>
 * 🔴 兩個最容易做錯的詞，寫在規則旁邊而不是只寫在設計書裡：
 * <ul>
 *   <li>{@code SETTLED} <b>不是終態</b>：收到錢不等於這筆帳結清了，還要等 provider 帳單
 *       確定 {@code actual_cost} 才能 true-up。把它當終態的後果是「actual &gt; estimated 的差額
 *       永遠不會被收，而帳面上看起來一切正常」。</li>
 *   <li>{@code UNCONFIRMED} <b>不是失敗</b>：它的語意是「可能收了也可能沒收」。PayUni 沒有
 *       status lifecycle，唯一訊號是 FundDate 二元值，「還沒到次日」與「次日執行失敗」
 *       長得一模一樣。把它當失敗 ⇒ 自動重送 ⇒ 重複扣款且事後查不出來。</li>
 * </ul>
 */
public final class ReceivableStatusRules {

    private ReceivableStatusRules() {
    }

    /**
     * 70 §3.2 的 allowed-from 欄，<b>這裡是程式側的唯一權威</b>。
     * <p>
     * 🔴 每一條邊都必須答得出「誰把它推走」（06-no-small-tails.md §3.13 第 ② 問）。
     * 答不出來的邊不要留：那是一條沒有執行者的邊，而它的症狀是 CAS rows=0，
     * 呼叫端多半把 rows=0 當成「別人先做過了」而靜默放行。
     */
    private static final Map<LogisticsReceivableStatus, Set<LogisticsReceivableStatus>> ALLOWED_FROM;

    static {
        Map<LogisticsReceivableStatus, Set<LogisticsReceivableStatus>> m =
                new EnumMap<>(LogisticsReceivableStatus.class);

        // 提交 worker 送出成功。
        // ⚠️ 含 FAILED：退避重送成功的目標就是 SUBMITTED，漏了它會讓每一次
        //    重送成功都 rows=0。
        m.put(LogisticsReceivableStatus.SUBMITTED, EnumSet.of(
                LogisticsReceivableStatus.PENDING, LogisticsReceivableStatus.FAILED));

        // 查詢 worker 查到 FundDate 有值；或 ADM 到 PayUni 後台核對後確認「其實收到了」。
        // ⚠️ 含 UNCONFIRMED：那是 UNCONFIRMED 三條出邊裡**最可能**的一條，
        //    少了它，查證為真的那條路沒有邊、只剩認賠。
        m.put(LogisticsReceivableStatus.SETTLED, EnumSet.of(
                LogisticsReceivableStatus.SUBMITTED, LogisticsReceivableStatus.UNCONFIRMED));

        // 超過 expected_fund_date ＋ 寬限，FundDate 仍為 '0000-00-00'。
        m.put(LogisticsReceivableStatus.UNCONFIRMED, EnumSet.of(
                LogisticsReceivableStatus.SUBMITTED));

        // 🔴 **只會從 PENDING 來**：FAILED 的語意是「送出當下回 Status ≠ SUCCESS」。
        //    原稿的 allowed-from 含 SUBMITTED，那是一條沒有執行者的邊。
        m.put(LogisticsReceivableStatus.FAILED, EnumSet.of(
                LogisticsReceivableStatus.PENDING));

        // ADM 確認 PayUni 未受理 ⇒ 退回 PENDING 開新 attempt ＋ 新 trade_no。
        // ⚠️ 用舊 trade_no 重送等於賭 PayUni 真的沒受理；開新的一筆即使原來其實受理了，
        //    也是兩筆可查的紀錄，而不是一筆分不清的。
        m.put(LogisticsReceivableStatus.PENDING, EnumSet.of(
                LogisticsReceivableStatus.UNCONFIRMED));

        // true-up 完成。
        m.put(LogisticsReceivableStatus.CLOSED, EnumSet.of(
                LogisticsReceivableStatus.SETTLED));

        // 人工核銷（強制填理由）。
        m.put(LogisticsReceivableStatus.WRITTEN_OFF, EnumSet.of(
                LogisticsReceivableStatus.UNCONFIRMED, LogisticsReceivableStatus.FAILED));

        ALLOWED_FROM = Map.copyOf(m);
    }

    /**
     * 推進到 {@code to} 的合法來源狀態，依 {@code value()} 字典序排序。
     * <p>
     * 🔴 呼叫端一律用它組 CAS 的 {@code allowed_from}，<b>不要在 SQL 裡各寫一份</b>：
     * 兩份必然漂移，而漂移的症狀是 rows=0 這種看起來像「別人先做過了」的靜默卡死。
     * <p>
     * 🔴 排序是契約的一部分而不是裝飾：它讓「repository 實際送出的陣列」與本表可以直接
     * 逐位元比對。
     * <p>
     * ⚠️ 對「CodeMeta 新增了一個值但這裡沒登記」的情形一律丟例外（FAIL），<b>不是</b>回空集合
     * （SKIP）——回空集合會讓那個新狀態的 CAS 永遠 rows=0，而且完全沒有訊號。
     *
     * @throws IllegalArgumentException {@code to} 為 null，或該狀態沒有登記任何入邊
     */
    public static List<LogisticsReceivableStatus> allowedFrom(LogisticsReceivableStatus to) {
        if (to == null) {
            throw new IllegalArgumentException("logistics: 目標狀態不得為 null");
        }
        Set<LogisticsReceivableStatus> from = ALLOWED_FROM.get(to);
        if (from == null) {
            throw new IllegalArgumentException("logistics: 未登記入邊的目標狀態 " + to.value());
        }
        return from.stream()
                .sorted(Comparator.comparing(LogisticsReceivableStatus::value))
                .toList();
    }

    /**
     * {@link #allowedFrom} 的字串形式，直接餵給 SQL 的 {@code varchar[]} 參數。
     */
    public static List<String> allowedFromValues(LogisticsReceivableStatus to) {
        return allowedFrom(to).stream().map(LogisticsReceivableStatus::value).toList();
    }

    /**
     * {@code from → to} 是否為合法邊。null 一律回 false（不丟例外，因為呼叫端是判斷式）。
     */
    public static boolean canTransition(LogisticsReceivableStatus from, LogisticsReceivableStatus to) {
        if (from == null || to == null) {
            return false;
        }
        Set<LogisticsReceivableStatus> sources = ALLOWED_FROM.get(to);
        return sources != null && sources.contains(from);
    }

    /**
     * 該狀態是否為終態。
     * <p>
     * 🔴 {@code SETTLED} <b>不在這裡</b>。它是本檔最容易被讀錯的一格，所以用一個具名方法
     * 把答案釘死，而不是讓每個呼叫端各自判斷。
     */
    public static boolean isTerminal(LogisticsReceivableStatus status) {
        return status == LogisticsReceivableStatus.CLOSED
                || status == LogisticsReceivableStatus.WRITTEN_OFF;
    }

    /**
     * 已登記入邊的狀態集合。測試用它驗「CodeMeta 的每個值都有登記」，
     * 讓「新增碼值卻忘了補邊」在 build 時就現形。
     */
    public static Set<LogisticsReceivableStatus> statusesWithRegisteredInboundEdges() {
        return ALLOWED_FROM.keySet();
    }
}
