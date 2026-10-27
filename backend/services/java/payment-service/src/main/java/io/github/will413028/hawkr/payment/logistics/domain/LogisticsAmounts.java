package io.github.will413028.hawkr.payment.logistics.domain;

/**
 * 線 B（賣家物流財務）的金額鏈（ADR-011）。
 * <p>
 * <pre>
 * domain（本類別）
 *   本期認列（帳本） = estimated_cost − platform_subsidy ± 責任調整   ← 🔴 不含 residual
 *   target_net       = 本期認列 + carried_residual                    ← Collection Target Net
 *   🔴 domain 到此為止。它**不知道**要送多少給通道，也**不得**出現任何費率。
 *
 * payment adapter（PayUni 專屬）
 *   gross_amount = 依 verified fee policy 反推，使 gross − fee(gross) ≥ target_net
 *
 * 回報
 *   actual_fee    ← agent_fee_query 的 Fee
 *   net_proceeds  = gross_amount − actual_fee                         ← 平台真正收到
 *   residual      = target_net − net_proceeds                         ← 可正可負
 *   carried_residual(下一期) = residual
 * </pre>
 * <p>
 * 🔴 <b>三個數字分開，缺一個就會混淆不同性質的錢</b>：{@code 本期認列}（帳本收入）／
 * {@code target_net}（要淨收多少）／{@code gross_amount}（送出多少）。前期 residual
 * <b>已經在前期認列過收入</b>，本期再認列一次就是重複認列 ⇒ 帳本只認列本期新增那一塊，
 * 而 target_net 才含 residual。
 * <p>
 * 🔴 <b>residual 不是「上期的手續費」</b>——是「預估與實際的差」。費率估準時它恆為 0；
 * 估不準時它併入下一期的 target_net 後<b>整體再 gross-up 一次</b>，
 * <b>不是</b>對 fee 本身再收一次 fee（那是被推翻的初版，有 fee-on-fee 遞迴）。
 * <p>
 * <b>型別與失敗處置</b>
 * <ul>
 *   <li>金額一律 {@code long}「元」整數（10 檔 §4）。本類別沒有任何浮點數。</li>
 *   <li>不變式違反 ⇒ 丟 {@link IllegalArgumentException}（Go 版回 error 的對應物）。
 *       🔴 一律 fail-loud，<b>不得</b>靜默夾到 0：夾到 0 會讓一筆算錯的補貼永遠不被發現。</li>
 *   <li>算術溢位 ⇒ 由 {@code Math.addExact} / {@code Math.subtractExact} / {@code Math.negateExact}
 *       丟 {@link ArithmeticException}。🔴 這是為了讓「壞資料」以錯誤現形，而不是靜默算出一個
 *       環繞過的負數——負的金額在下游看起來就只是一筆小金額，沒有任何東西會叫。</li>
 * </ul>
 */
public final class LogisticsAmounts {

    private LogisticsAmounts() {
    }

    /**
     * 本期要認列進帳本的收入金額。
     * <p>
     * 🔴 <b>不含 carriedResidual</b>：結轉是前期已認列、尚未收足的餘額，再認列一次就是
     * 重複認列收入——帳上會出現一筆不存在的收入，而現金並沒有多進來。
     *
     * @param responsibilityAdjustment 可正可負（ADR-004 的責任歸屬調整）
     * @throws IllegalArgumentException 成本／補貼為負，或認列金額算出來是負的
     */
    public static long accrualAmount(long estimatedCost, long platformSubsidy, long responsibilityAdjustment) {
        if (estimatedCost < 0) {
            throw new IllegalArgumentException("logistics: estimatedCost 不得為負，got " + estimatedCost);
        }
        if (platformSubsidy < 0) {
            throw new IllegalArgumentException("logistics: platformSubsidy 不得為負，got " + platformSubsidy);
        }
        long accrual = Math.addExact(Math.subtractExact(estimatedCost, platformSubsidy), responsibilityAdjustment);
        if (accrual < 0) {
            // 🔴 補貼或調整超過成本 ⇒ 認列金額為負。這不是「應收 0」，而是設計上不該發生的
            //    組合（平台補貼超過實際成本代表補貼算錯了）。fail-loud，不要靜默夾到 0。
            throw new IllegalArgumentException(String.format(
                    "logistics: 本期認列為負（estimated=%d, subsidy=%d, adjustment=%d）——補貼或責任調整超過成本",
                    estimatedCost, platformSubsidy, responsibilityAdjustment));
        }
        return accrual;
    }

    /**
     * Collection Target Net：這次要讓平台<b>淨收</b>多少。
     * <p>
     * 🔴 它<b>不是</b>送給通道的金額。送出額由 payment adapter 依費率反推（ADR-011），
     * 而 domain 不得知道費率。
     * <p>
     * {@code carriedResidual} 可負（上期多收）。合計為負時代表平台上期多收超過本期應收 ⇒
     * 這一筆不該發動收款，而是該產生貸記；回 0 並由呼叫端據此不建 attempt。
     */
    public static long targetNet(long accrual, long carriedResidual) {
        long target = Math.addExact(accrual, carriedResidual);
        return target < 0 ? 0L : target;
    }

    /**
     * Agent Net Proceeds ＝ 平台<b>真正收到</b>的錢。
     * <p>
     * 🔴 {@code gross − actualFee}，不是 gross。手續費是 PayUni 從撥款裡扣掉的，
     * 平台從來沒收到那筆錢——把 gross 當實收，帳上的現金就會比銀行多。
     * （M-U4 變異：把本式改成 {@code = gross} 必須轉紅。）
     *
     * @throws IllegalArgumentException grossAmount &lt; 1（PayUni 的 Amt 值域是 1~9999999 的
     *                                  正整數，不接受 0 或負數）或 actualFee 為負
     */
    public static long netProceeds(long grossAmount, long actualFee) {
        if (grossAmount < 1) {
            throw new IllegalArgumentException("logistics: grossAmount 必須 >= 1，got " + grossAmount);
        }
        if (actualFee < 0) {
            throw new IllegalArgumentException("logistics: actualFee 不得為負，got " + actualFee);
        }
        return Math.subtractExact(grossAmount, actualFee);
    }

    /**
     * Collection Residual ＝ {@code targetNet − netProceeds}（可正可負）。
     * <ul>
     *   <li>residual &gt; 0 ⇒ 平台<b>尚未收足</b> ⇒ 🔴 不得標記為已收訖，缺口併入下一期的 target_net</li>
     *   <li>residual &lt; 0 ⇒ 平台<b>多收</b> ⇒ 🔴 必須產生 Seller Logistics Credit，<b>不得</b>變成 platform margin</li>
     *   <li>residual = 0 ⇒ 正好（費率估準時的正常結果）</li>
     * </ul>
     */
    public static long residual(long targetNet, long netProceeds) {
        return Math.subtractExact(targetNet, netProceeds);
    }

    /**
     * 這一次收款是否已收足。
     * <p>
     * 🔴 <b>residual ≠ 0 不得標記已收訖</b>（M-U5 變異：把 residual≠0 的收款直接標 CLOSED
     * 必須轉紅）。⚠️ 注意 residual &lt; 0（多收）同樣<b>不是</b>「收足了就好」——
     * 它要走貸記，所以本方法只在 residual == 0 時回 true。
     */
    public static boolean isFullyCollected(long residual) {
        return residual == 0L;
    }

    /**
     * 把負的 residual 轉成應發給賣家的貸記本金（正數）。
     * <p>
     * 🔴 貸記本金<b>不得被通道手續費侵蝕</b>：PayUni 負向（BalType=2）是「扣代理商款項
     * 含該金額 1% 手續費、<b>足額</b>撥款給合作商店」⇒ 賣家收到的是足額，
     * 平台額外承擔 {@code credit × r}。那筆通道成本是平台履行退還義務的執行成本，
     * <b>不得</b>從 credit 裡扣，也不得默默記成「賣家少拿」。
     * <p>
     * residual &gt;= 0 時回 0（沒有多收就沒有貸記）。
     */
    public static long creditFromResidual(long residual) {
        if (residual >= 0) {
            return 0L;
        }
        return Math.negateExact(residual);
    }

    /**
     * 下一期要帶入的 carried_residual。
     * <p>
     * 🔴 <b>不是「上期的手續費」</b>：它是 targetNet 與實收的差。費率估準時恆為 0。
     * 正值代表未收足（下期併收），負值代表多收（下期抵扣，或走貸記現金返還）。
     */
    public static long carryForward(long residual) {
        return residual;
    }
}
