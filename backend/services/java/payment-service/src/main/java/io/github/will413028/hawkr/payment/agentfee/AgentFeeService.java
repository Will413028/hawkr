package io.github.will413028.hawkr.payment.agentfee;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import java.util.Map;
import java.util.Optional;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

/**
 * PayUni 多功能收付（{@code agent_fee}）的**通道執行層**。
 *
 * <h2>它負責什麼、不負責什麼</h2>
 * {@code ADR-009} 把物流財務切成四個 owner，本類是其中的「收款執行」那一格：
 * <ul>
 *   <li>✅ 依 {@code ADR-011} 把 domain 給的<b>目標淨額</b>反推成送出金額（gross-up）</li>
 *   <li>✅ 呼叫 PayUni、解讀回應、把 {@code 0000-00-00} 這種哨兵值翻成「未設」</li>
 *   <li>❌ <b>不</b>持有應收、不決定要不要收、不做去重
 *       —— 那些是同服務內 {@code io.github.will413028.hawkr.payment.logistics} 模組的事
 *       （70 §14.2 逐字「本 RPC 不負責去重」；2026-09-12 架構合併前那個模組是獨立服務）</li>
 * </ul>
 *
 * <h2>🔴 呼叫端必須先落地 trade_no 才呼叫本類</h2>
 * PayUni 官方三處逐字標 {@code TradeNo}「可重複」，且 {@code agent_fee_query}
 * 回<b>單筆結構非陣列</b> ⇒ 真的送重複了，用官方 API 也對不出有幾筆。
 * 順序顛倒的後果：HTTP 逾時 ⇒ 我方沒有紀錄 ⇒ 重試產生第二個 {@code TradeNo}
 * ⇒ <b>重複扣款且事後查不出來</b>。
 */
@Service
public class AgentFeeService {

    private static final ObsLogger log = ObsLogger.forClass(AgentFeeService.class);

    /** PayUni 用來表示「尚未撥款」的哨兵值。 */
    static final String UNSETTLED_FUND_DATE = "0000-00-00";

    private final PaymentClient paymentClient;
    private final PayUniConfig payUniConfig;
    private final AgentFeeGrossUp grossUp;

    /**
     * 代理商出入帳商店代號（選填）。
     *
     * <p>🔴 <b>空字串與未設定不是同一件事</b>：官方標它是條件必填，未帶＝依平台預設，
     * 而空字串塞進 EncryptInfo 等於指定一個不存在的商店。
     */
    private final String agentMerId;

    public AgentFeeService(PaymentClient paymentClient,
                           PayUniConfig payUniConfig,
                           AgentFeeGrossUp grossUp,
                           @Value("${payuni.agent.mer-id:}") String agentMerId) {
        this.paymentClient = paymentClient;
        this.payUniConfig = payUniConfig;
        this.grossUp = grossUp;
        this.agentMerId = agentMerId;
    }

    /**
     * 送出一筆多功能收付。
     *
     * @param merId     被扣款／收款的合作商店 MerID
     * @param tradeNo   🔴 <b>呼叫端已落地</b>的自訂單號
     * @param targetNet 🔴 <b>目標淨額</b>（平台要淨收多少），<b>不是</b>送出金額
     * @param feeType   費用類別碼（物流＝4）
     * @param balType   1＝正向（向賣家收）、2＝負向（退還賣家）
     */
    public SubmitOutcome submit(String merId, String tradeNo, long targetNet, int feeType, int balType) {
        // 🔴 這個多載**在方法內才解算 gross**，所以呼叫端沒有機會在送出前把它持久化。
        //    需要「送出前原子持久化 gross」的呼叫端（線 B 的收款）必須改用
        //    {@link #submit(String, String, AgentFeeGrossUp.Result, int, int)}。
        //    保留本多載是因為 gRPC adapter（PaymentGrpcService.submitAgentFee）是
        //    無狀態轉譯、沒有 attempt 列可落，對它而言目前這個形狀是正確的。
        return submit(merId, tradeNo, grossUp.solve(targetNet), feeType, balType);
    }

    /**
     * 送出一筆多功能收付，<b>使用呼叫端已經解算並且已經持久化的 gross</b>。
     *
     * <h2>🔴 它為什麼必須存在（架構裁決 DEC-20260912-184025 第 7 條）</h2>
     * {@code agent_fee_query} 的查詢鍵是 <b>{@code (MerID, TradeNo, Amt)} 三元組</b>——
     * 少了 {@code Amt} 就查不到那一筆。而 {@code Amt} 送出去的值是 gross 不是 target net。
     *
     * <p>於是有一個具體的失效窗口：<b>送出成功、但回應遺失</b>（逾時／連線中斷）。
     * 此時 PayUni 可能已經受理，我方卻不知道自己送了多少 ⇒ 後續 query 帶不出正確的
     * {@code Amt} ⇒ 查無此筆 ⇒ 一筆已經在對方系統裡的收款，在我方永遠停在不確定狀態。
     *
     * <p>🔴 <b>不得靠 response 才取得查詢鍵</b>（裁決第 7 條逐字）。
     * ⇒ 正確順序是：呼叫端先 {@link AgentFeeGrossUp#solve} → 把 gross 與 attempt 列
     * <b>在同一個交易裡</b>落地 → 才呼叫本方法送出。
     *
     * <h2>⚠️ 順序不能顛倒：先 solve 再建列，不是先建列再 solve</h2>
     * {@code solve()} 在費率未查證時<b>會拋</b>（fail-closed）。先建列的話，
     * 拋出來的時候已經有一筆 gross 不明的 attempt 落地了——而
     * {@code chk_logcoll_gross} 要求 {@code >= 1}，那筆列根本寫不進去或寫進一個假值。
     *
     * <h2>⚠️ 也不要用「先落佔位值、回應後覆寫」那種兩段式</h2>
     * 那是跨服務時代的產物：當 gross 要等下游回應才知道時才需要佔位。
     * ADR-011 的 gross 是在 HTTP 呼叫<b>之前</b>就算好的，同進程直接拿得到
     * ⇒ <b>可以一次寫對</b>。佔位值的代價是「這一欄現在是真值還是佔位」變成要靠
     * 另一個欄位才分得出來，而那個判斷遲早會有人漏掉。
     *
     * @param solved 呼叫端已 {@code solve()} 並且<b>已經落地</b>的結果
     */
    public SubmitOutcome submit(String merId, String tradeNo, AgentFeeGrossUp.Result solved,
                                int feeType, int balType) {
        AgentFeeGrossUp.Result reversed = solved;

        try {
            PayUniApiResponse resp = paymentClient.agentFee(
                    payUniConfig.getAgentId(), merId, tradeNo,
                    reversed.grossAmount(), feeType, balType, agentMerId);

            String status = firstNonBlank(resp.status(), "");
            String message = firstNonBlank(resp.message(), "");
            log.info("AgentFeeService.submit.responded",
                    "agent_fee 已回應 status=" + status, "tradeNo", tradeNo);
            return new SubmitOutcome(status, message, reversed.grossAmount(),
                    reversed.estimatedFee(), reversed.feePolicyVersion(), true);

        } catch (Exception e) {
            // 🔴 **逾時／連線失敗不是失敗**：PayUni 可能已經受理了。
            //    回 providerResponded=false 讓呼叫端走「不確定窗口」的處置，
            //    而不是自動重送——自動重送的期望值是「把一次不確定變成一次確定的重複扣款」。
            log.warn("AgentFeeService.submit.unknown",
                    "agent_fee 呼叫未取得明確回應（可能已受理）：" + e.getMessage(), "tradeNo", tradeNo);
            return new SubmitOutcome("", e.getMessage() == null ? "" : e.getMessage(),
                    reversed.grossAmount(), reversed.estimatedFee(), reversed.feePolicyVersion(), false);
        }
    }

    /**
     * 查詢一筆多功能收付的撥款結果。
     *
     * @param grossAmount 🔴 <b>當初送出的那個金額</b>（查詢 key 是 MerID+TradeNo+Amt 三元組），
     *                    不是目標淨額
     */
    public QueryOutcome query(String merId, String tradeNo, long grossAmount) {
        PayUniApiResponse resp = paymentClient.agentFeeQuery(
                payUniConfig.getAgentId(), merId, tradeNo, grossAmount);

        Map<String, String> data = resp.decryptedData();
        if (data == null || data.isEmpty()) {
            // 🔴 查無此筆與「尚未撥款」是**兩件事**：前者代表 PayUni 根本沒有這筆
            //    （帶錯 trade_no／amt，或送出其實沒受理），後者代表受理了但還沒執行。
            //    混在一起的話，呼叫端會把「沒受理」誤讀成「還沒到次日」而一直等下去。
            log.warn("AgentFeeService.query.notFound",
                    "agent_fee_query 查無此筆", "tradeNo", tradeNo);
            return QueryOutcome.notFound();
        }

        String rawFundDate = data.getOrDefault("FundDate", "");
        Optional<String> fundDate = (rawFundDate.isBlank() || UNSETTLED_FUND_DATE.equals(rawFundDate))
                ? Optional.empty()
                : Optional.of(rawFundDate);

        // 🔴 Fee 只在真的撥款之後才有意義。未撥款時 PayUni 可能回 0 或空字串，
        //    而 0 與「還沒有」在帳上是完全不同的兩件事：前者代表這筆沒收手續費，
        //    後者代表我們還不知道。⇒ 未撥款一律回 empty，不把 0 當事實。
        Optional<Long> actualFee = fundDate.isPresent()
                ? parseLong(data.get("Fee"))
                : Optional.empty();

        return new QueryOutcome(
                true,
                data.getOrDefault("MerID", ""),
                data.getOrDefault("TradeNo", ""),
                parseLong(data.get("Amt")).orElse(grossAmount),
                data.getOrDefault("FeeType", ""),
                data.getOrDefault("BalType", ""),
                data.getOrDefault("TradeDate", ""),
                fundDate,
                actualFee,
                blankToEmpty(data.get("AgentMerID")));
    }

    private static Optional<Long> parseLong(String v) {
        if (v == null || v.isBlank()) {
            return Optional.empty();
        }
        try {
            return Optional.of(Long.parseLong(v.trim()));
        } catch (NumberFormatException e) {
            // 🔴 解析不出來就回 empty，**不要退回 0**：0 是一個合法的金額，
            //    用它當「解析失敗」的代表值會讓錯誤在帳上變成一筆看起來正常的數字。
            return Optional.empty();
        }
    }

    private static Optional<String> blankToEmpty(String v) {
        return (v == null || v.isBlank()) ? Optional.empty() : Optional.of(v);
    }

    private static String firstNonBlank(String v, String fallback) {
        return (v == null || v.isBlank()) ? fallback : v;
    }

    /**
     * 送出結果。
     *
     * @param providerStatus    PayUni 的 {@code Status}（{@code SUCCESS} ＝ 受理）
     * @param providerMessage   PayUni 的 {@code Message}
     * @param grossAmount       實際送出的 {@code Amt}
     * @param estimatedFee      依預估費率算出的手續費（🔴 不是帳務憑據）
     * @param feePolicyVersion  反推時採用的費率政策版本
     * @param providerResponded 🔴 false ＝ 逾時／連線失敗，<b>呼叫端不得視為失敗</b>
     */
    public record SubmitOutcome(String providerStatus,
                                String providerMessage,
                                long grossAmount,
                                long estimatedFee,
                                String feePolicyVersion,
                                boolean providerResponded) {

        /** PayUni 明確受理。 */
        public boolean accepted() {
            return providerResponded && "SUCCESS".equalsIgnoreCase(providerStatus);
        }

        /**
         * PayUni 明確拒絕（有回應、但不是 SUCCESS）。
         *
         * <p>🔴 與「未取得回應」是**兩件事**：明確拒絕代表沒有受理、不存在
         * 「可能已經扣了」的窗口 ⇒ 可以自動退避重送；未取得回應則不可以。
         */
        public boolean rejected() {
            return providerResponded && !accepted();
        }
    }

    /**
     * 查詢結果。
     *
     * @param found     🔴 false ＝ PayUni 沒有這筆，與「尚未撥款」是兩件事
     * @param fundDate  撥款日；empty ＝ 尚未撥款（{@code 0000-00-00} 已在此翻譯掉）
     * @param actualFee 實際手續費；🔴 僅在已撥款時有值，未撥款一律 empty 而非 0
     */
    public record QueryOutcome(boolean found,
                               String merId,
                               String tradeNo,
                               long grossAmount,
                               String feeType,
                               String balType,
                               String tradeDate,
                               Optional<String> fundDate,
                               Optional<Long> actualFee,
                               Optional<String> agentMerId) {

        static QueryOutcome notFound() {
            return new QueryOutcome(false, "", "", 0L, "", "", "",
                    Optional.empty(), Optional.empty(), Optional.empty());
        }

        /** 已確認入帳（{@code FundDate} 有值）。 */
        public boolean settled() {
            return found && fundDate.isPresent();
        }
    }
}
