package io.github.will413028.hawkr.payment.agentfee;

import org.springframework.stereotype.Component;

/**
 * {@code ADR-011} 的 gross-up：把 domain 給的<b>目標淨額</b>反推成<b>實際要送出的金額</b>。
 *
 * <pre>
 * business domain（物流財務結算）
 *   │  只說一句：我要淨收 Collection Target Net = N
 *   ▼
 * payment collection adapter（本類）
 *   │  依 verified fee policy 反推 Gross Amount G  s.t.  G − fee(G) ≥ N
 *   ▼
 * agent_fee(Amt = G)
 * </pre>
 *
 * <h2>🔴 為什麼一定要 gross-up（而不是「照成本送出、事後補收差額」）</h2>
 * 官方逐字：{@code BalType=1} ＝「扣合作商店款項，<b>扣 1% 手續費後</b>撥款給代理商」
 * ⇒ 賣家被扣 {@code Amt}、平台只收到 {@code Amt × (1−r)}
 * ⇒ <b>手續費的現金負擔落在平台</b>，而 {@code ADR-005} 定案它由賣家負擔。
 *
 * <p>本 ADR 的初版曾改成「不 gross-up、以實際 Fee 後收、缺口結轉下一筆」，那個模型有
 * <b>fee-on-fee 遞迴</b>：每一輪都再生一筆新的 fee，是幾何級數、數學上永不到 0；
 * 而且在真實世界連第一輪都走不完 —— {@code Amt} 值域是 1~9999999 的整數，
 * 殘留的 1.21 元只能送 1 元，剩下的 0.21 元<b>小於最小可送金額，永遠送不出去</b>。
 *
 * <h2>🔴 取整策略是契約的一部分，不是實作細節</h2>
 * {@code Amt} 是整數元 ⇒ 必須取<b>滿足 {@code G − fee(G) ≥ N} 的最小整數 G</b>。
 *
 * <table border="1">
 *   <caption>兩種取整策略在 {@code C=120}／{@code r=1%}（捨去制）下的差別</caption>
 *   <tr><th>策略</th><th>每期 residual</th><th>後果</th></tr>
 *   <tr><td>{@code ceil(N/(1−r))}</td><td><b>−1（恆定）</b></td>
 *       <td>🔴 平台每期多收 1 元；而「{@code residual ≠ 0} 不得標記已收訖」
 *           ⇒ <b>每一筆應收永遠關不掉</b>，並持續觸發 SETTLED 卡住偵測</td></tr>
 *   <tr><td><b>求解最小可行整數</b></td><td><b>0</b></td><td>✅</td></tr>
 * </table>
 *
 * <p>⚠️ 這是「兩條規則各自正確、組合起來互相抵銷」的實例：「取整隨便」與
 * 「{@code residual≠0} 不得結清」單獨看都對，合起來讓應收永不結清。
 */
@Component
public class AgentFeeGrossUp {

    /** PayUni {@code Amt} 的官方值域上限（長度 7）。 */
    static final long MAX_AMT = 9_999_999L;

    private final AgentFeeRatePolicy policy;

    public AgentFeeGrossUp(AgentFeeRatePolicy policy) {
        this.policy = policy;
    }

    /**
     * 反推送出金額。
     *
     * @param targetNet 平台要<b>淨收</b>的金額（元整數，必須 ≥ 1）
     * @return 滿足 {@code gross − fee(gross) >= targetNet} 的<b>最小</b>整數
     * @throws IllegalStateException    費率政策未查證（{@code EV-P6}）—— fail-closed
     * @throws IllegalArgumentException targetNet 不在合法範圍，或反推結果超出官方值域
     */
    public Result solve(long targetNet) {
        if (!policy.isVerified()) {
            throw new IllegalStateException(
                    "agent_fee 費率政策未查證（EV-P6）⇒ 不得推導送出金額。"
                            + " 這是 ADR-011 的 Enablement Gate：設計與程式都可以先完成，"
                            + " 缺的只是『可以自動送出』這個開關。");
        }
        if (targetNet < 1L) {
            // 🔴 官方不接受 0 或負數的 Amt ⇒ 目標淨額 <= 0 時**不該有這一次呼叫**。
            //    在這裡擋，比讓它變成一筆送出必失敗的請求好——後者的失敗看起來像通道問題。
            throw new IllegalArgumentException(
                    "targetNet 必須 >= 1（PayUni Amt 值域 1~9999999），got " + targetNet);
        }

        // 閉式解當起點：G0 ＝ ceil(N × den / (den − num))，再往下收斂到最小可行值。
        //
        // ⚠️ 直接從 targetNet 線性上探也會得到同一個答案，但在費率大／金額大時會跑很多輪。
        // 用閉式解起跳 ＋ 向下收斂，結果與線性上探**逐值相同**（由測試釘住），而不是近似。
        long num = policy.getRateNumerator();
        long den = policy.getRateDenominator();
        long denom = den - num;
        if (denom <= 0) {
            // 費率 >= 100% ⇒ 無論送多少都淨收不到正數。這是設定錯誤，不是邊界情況。
            throw new IllegalStateException(
                    "agent_fee 費率 >= 100%（" + num + "/" + den + "）⇒ 無解。請覆核費率設定。");
        }
        long candidate = ceilDiv(Math.multiplyExact(targetNet, den), denom);
        if (candidate < targetNet) {
            candidate = targetNet;
        }

        // 向上補足（取整方向為 DOWN 時閉式解可能略低）
        while (candidate - policy.feeFor(candidate) < targetNet) {
            if (candidate >= MAX_AMT) {
                throw new IllegalArgumentException(
                        "反推的送出金額超出 PayUni Amt 上限 " + MAX_AMT + "（targetNet=" + targetNet + "）");
            }
            candidate++;
        }
        // 向下收斂到**最小**可行值（閉式解在取整方向為 UP／HALF_UP 時可能偏高 1）
        while (candidate - 1 >= targetNet
                && (candidate - 1) - policy.feeFor(candidate - 1) >= targetNet) {
            candidate--;
        }

        if (candidate > MAX_AMT) {
            throw new IllegalArgumentException(
                    "反推的送出金額超出 PayUni Amt 上限 " + MAX_AMT + "（targetNet=" + targetNet + "）");
        }
        long estimatedFee = policy.feeFor(candidate);
        return new Result(candidate, estimatedFee, policy.getVersion());
    }

    private static long ceilDiv(long a, long b) {
        long q = a / b;
        return (a % b == 0) ? q : q + 1;
    }

    /**
     * 反推結果。
     *
     * @param grossAmount      實際要送出的 {@code Amt}
     * @param estimatedFee     依**預估**費率算出的手續費。
     *                         🔴 <b>它不是事後帳務憑據</b>——真正的手續費只有
     *                         {@code agent_fee_query} 的 {@code Fee} 說了算，
     *                         兩者的差就是 {@code residual} 要吸收的東西。
     * @param feePolicyVersion 反推時採用的費率政策版本
     */
    public record Result(long grossAmount, long estimatedFee, String feePolicyVersion) {}
}
