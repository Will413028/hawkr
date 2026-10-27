package io.github.will413028.hawkr.payment.logistics.domain;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

/**
 * 驗 ADR-011 的收款金額鏈。
 * <p>
 * 🔴 <b>斷言的期望值一律硬寫字面值</b>，不呼叫被測方法也不引用它用到的常數：
 * 期望值跟著被測物一起變的話，這些測試驗不到任何東西
 * （03-delegation-templates.md §0.3 第二種形態）。
 */
class LogisticsAmountsTest {

    /**
     * 模擬 payment adapter 持有的通道費率政策。
     * <p>
     * 🔴 它<b>刻意住在測試裡而不是 domain</b>：ADR-011 的核心就是「domain 不得知道費率」。
     * 把它放進 logistics.domain 會讓這條規則在第一次 import 時就破了。
     *
     * @param rateNumerator   以整數表達費率的分子（例：1/100 ＝ 1%），避免浮點
     * @param rateDenominator 分母
     * @param roundDown       true ＝ 通道手續費無條件捨去到元；false ＝ 無條件進位。
     *                        🔴 <b>PayUni 真正的 rounding 規則是 EV-P6，至今未知</b>。
     *                        本檔刻意把它做成參數而不是寫死一種：寫死等於在測試裡替一個
     *                        未確認的外部事實下結論，而那正是 ADR-011 Enablement Gate
     *                        擋住自動收款的理由。
     *                        ⚠️ 取整方向會實質改變 residual —— 見 {@code ceilStrategy*}。
     */
    private record FeePolicy(long rateNumerator, long rateDenominator, boolean roundDown) {

        static FeePolicy roundUp(long numerator, long denominator) {
            return new FeePolicy(numerator, denominator, false);
        }

        /** 送出 gross 時通道實扣多少。 */
        long fee(long gross) {
            long num = gross * rateNumerator;
            long q = num / rateDenominator;
            if (!roundDown && num % rateDenominator != 0) {
                q++;
            }
            return q;
        }

        /**
         * 求<b>滿足 {@code G − fee(G) >= target} 的最小整數 G</b>。
         * <p>
         * 🔴 取整策略是契約的一部分，不是實作細節（ADR-011 後果 3）：
         * 用 {@code ceil(N/(1−r))} 會每期多收 1 元 ⇒ residual 恆為 −1
         * ⇒ 因為「residual ≠ 0 不得標記已收訖」⇒ <b>每一筆應收永遠關不掉</b>。
         */
        long grossUp(long target) {
            if (target <= 0) {
                return 0L;
            }
            // 從 target 起線性上探。實際實作應用閉式解 ＋ 微調，這裡求的是「正確」不是「快」。
            for (long g = target; ; g++) {
                if (g - fee(g) >= target) {
                    return g;
                }
            }
        }
    }

    // ---------------------------------------------------------------- 認列

    @Test
    void accrualExcludesCarriedResidual() {
        // 🔴 本期認列 = 成本 − 補貼 ± 責任調整，**不含結轉**。
        // 含了就是重複認列收入：那筆錢前期已經認列過，而現金並沒有多進來。
        assertEquals(120L, LogisticsAmounts.accrualAmount(120, 0, 0));

        // target_net 才含結轉。
        assertEquals(181L, LogisticsAmounts.targetNet(120, 61));

        // 而認列金額不受結轉影響——同樣的輸入仍是 120。
        assertEquals(120L, LogisticsAmounts.accrualAmount(120, 0, 0));
    }

    @Test
    void accrualSubtractsPlatformSubsidy() {
        assertEquals(70L, LogisticsAmounts.accrualAmount(120, 50, 0));
    }

    @Test
    void accrualAppliesResponsibilityAdjustmentInBothDirections() {
        assertEquals(150L, LogisticsAmounts.accrualAmount(120, 0, 30));
        assertEquals(90L, LogisticsAmounts.accrualAmount(120, 0, -30));
    }

    @Test
    void accrualFailsLoudWhenSubsidyExceedsCost() {
        // 🔴 夾到 0 會讓一筆算錯的補貼永遠不被發現。
        assertThrows(IllegalArgumentException.class,
                () -> LogisticsAmounts.accrualAmount(120, 200, 0));
    }

    @Test
    void accrualRejectsNegativeInputs() {
        assertThrows(IllegalArgumentException.class,
                () -> LogisticsAmounts.accrualAmount(-1, 0, 0));
        assertThrows(IllegalArgumentException.class,
                () -> LogisticsAmounts.accrualAmount(120, -1, 0));
    }

    // ------------------------------------------------------------ target_net

    @Test
    void targetNetClampsToZeroWhenCarriedResidualOutweighsAccrual() {
        // 上期多收超過本期應收 ⇒ 不該發動收款（呼叫端據此不建 attempt）。
        assertEquals(0L, LogisticsAmounts.targetNet(100, -150));
        // 剛好打平也是 0。
        assertEquals(0L, LogisticsAmounts.targetNet(100, -100));
    }

    // ------------------------------------------------------------- 溢位保護

    @Test
    void overflowSurfacesAsArithmeticExceptionNotWrapAround() {
        // 🔴 靜默環繞會算出一個負的金額，而負的金額在下游看起來只是「一筆小錢」，
        //    沒有任何東西會叫。這幾個斷言就是「壞資料必須以錯誤現形」的機械形式。
        assertThrows(ArithmeticException.class,
                () -> LogisticsAmounts.accrualAmount(Long.MAX_VALUE, 0, 1));
        assertThrows(ArithmeticException.class,
                () -> LogisticsAmounts.targetNet(Long.MAX_VALUE, 1));
        assertThrows(ArithmeticException.class,
                () -> LogisticsAmounts.residual(Long.MIN_VALUE, 1));
        assertThrows(ArithmeticException.class,
                () -> LogisticsAmounts.creditFromResidual(Long.MIN_VALUE));
    }

    // ----------------------------------------------------------- net proceeds

    @Test
    void netProceedsRejectsGrossOutsidePayUniDomain() {
        // PayUni 的 Amt 值域是 1~9999999 的正整數，不接受 0 或負數。
        assertThrows(IllegalArgumentException.class, () -> LogisticsAmounts.netProceeds(0, 0));
        assertThrows(IllegalArgumentException.class, () -> LogisticsAmounts.netProceeds(-1, 0));
        assertThrows(IllegalArgumentException.class, () -> LogisticsAmounts.netProceeds(100, -1));
    }

    @Test
    void netProceedsSubtractsActualFee() {
        // 🔴 M-U4 變異的斷言：把 netProceeds 改成 `= gross` 必須讓這一條轉紅。
        assertEquals(118L, LogisticsAmounts.netProceeds(120, 2));
        assertNotEquals(120L, LogisticsAmounts.netProceeds(120, 2));
    }

    // ------------------------------------------------------ 70 §11.1 Case 1~4

    /** Case 1：正常收款，費率已知。 */
    @Test
    void case1NormalCollectionHasNoSilentShortfall() {
        FeePolicy p = FeePolicy.roundUp(1, 100); // 1%
        long target = 120L;

        long gross = p.grossUp(target);
        assertTrue(gross > target, "gross = " + gross + "，必須 > target " + target + "（否則手續費沒有人付）");

        long actualFee = p.fee(gross);
        assertTrue(actualFee > 0, "actualFee = " + actualFee + "，必須 > 0");

        long net = LogisticsAmounts.netProceeds(gross, actualFee);
        assertEquals(gross - actualFee, net, "net 必須等於 gross − fee");

        // 🔴 無 silent shortfall：實收必須覆蓋目標淨額。
        assertTrue(net >= target, "net = " + net + " < target = " + target + " ⇒ 平台短收而沒有任何訊號");

        // 這組數字的字面值（1% 無條件進位、target=120）：gross=122、fee=2、net=120。
        assertEquals(122L, gross);
        assertEquals(2L, actualFee);
        assertEquals(120L, net);
        assertEquals(0L, LogisticsAmounts.residual(target, net));
    }

    /**
     * Case 2：實際費率 ≠ 預估 ⇒ 必須產生 residual &gt; 0 且不得結清。
     * <p>
     * 🔴 這是整組的核心：費率永遠估得準的話 residual 恆為 0、這個欄位就不需要存在。
     * <b>它存在就是為了 Case 2。</b>
     */
    @Test
    void case2ActualRateExceedsEstimateLeavesUnsettledResidual() {
        FeePolicy est = FeePolicy.roundUp(1, 100);    // 預估 1%
        FeePolicy act = FeePolicy.roundUp(15, 1000);  // 實際 1.5%
        // 🔴 金額必須大到「0.5% 的費率差 > 1 元」，否則整數取整會把差額整個吃掉。
        //    第一版寫 target=120：兩種費率的實扣都是 2 元 ⇒ residual 恆為 0
        //    ⇒ 這支測試看起來全綠，實際上**對它要驗的東西零鑑別力**。
        long target = 12000L;

        long gross = est.grossUp(target); // adapter 用**預估**費率反推
        long actualFee = act.fee(gross);  // 通道用**實際**費率扣
        long net = LogisticsAmounts.netProceeds(gross, actualFee);
        long residual = LogisticsAmounts.residual(target, net);

        assertTrue(residual > 0, "residual = " + residual + "，實際費率高於預估時必須為正（代表尚未收足）");
        assertFalse(LogisticsAmounts.isFullyCollected(residual),
                "residual != 0 卻被判為已收訖 —— 這一筆會被錯誤地標成 CLOSED");
        // 缺口原樣併入下一期。
        assertEquals(residual, LogisticsAmounts.carryForward(residual));

        // 字面值：gross=12122、actualFee=182、net=11940、residual=60。
        assertEquals(12122L, gross);
        assertEquals(182L, actualFee);
        assertEquals(11940L, net);
        assertEquals(60L, residual);
    }

    /** Case 3：查詢尚未 settled ⇒ 不得提前視為已收款。 */
    @Test
    void case3NotYetSettledIsNullNotZero() {
        // 這條的機械形式在 schema（net_proceeds 可為 NULL）與 repository（Long 而非 long）。
        // domain 這一側能驗的是：0 與「尚未查詢」是兩件事——
        // 🔴 把未查詢當成 net=0 會讓 residual = target（看起來完全沒收到），
        //    而真相是「還不知道」。
        Long notQueried = null;
        Long queriedZero = 0L;

        assertNull(notQueried, "fixture 寫錯");
        assertNotNull(queriedZero);
        assertNotEquals(queriedZero, notQueried, "0 與 null 必須可分辨");

        // 真的查到 0（送出 1 元、手續費 1 元）與沒查過，residual 的語意完全不同。
        long residualIfActuallyZero = LogisticsAmounts.residual(12000L, 0L);
        assertEquals(12000L, residualIfActuallyZero);
    }

    /** Case 4：賣家貸記 50 ⇒ 賣家必須實收 50，通道成本不得侵蝕本金。 */
    @Test
    void case4SellerCreditPrincipalIsNotErodedByChannelFee() {
        // residual = −50 代表平台多收 50。
        long credit = LogisticsAmounts.creditFromResidual(-50);
        assertEquals(50L, credit);

        // 🔴 PayUni 負向是「扣代理商款項**含該金額 1% 手續費**、**足額**撥款給合作商店」
        //    ⇒ 賣家收到 50，平台被扣 50 × (1+r)。通道成本落在平台，
        //    **不得**從 credit 裡扣成 49.x。本斷言把「賣家實收 ＝ 本金」釘死。
        long sellerReceives = credit;
        assertEquals(50L, sellerReceives, "賣家實收必須足額 50");

        // 正的 residual 不產生貸記。
        assertEquals(0L, LogisticsAmounts.creditFromResidual(61), "residual > 0（未收足）不得產生貸記");
        assertEquals(0L, LogisticsAmounts.creditFromResidual(0));
    }

    // ------------------------------------------------- 70 §11.2 平台現金流不變式

    /**
     * 平台現金流不變式，<b>必須跨多期驗</b>。
     * <p>
     * 🔴 被推翻的初版（fee-on-fee）每期只短收 C·rⁿ（1.2 → 0.012 → 0.00012…），
     * 單看一期幾乎看不出來，只有<b>累計</b>才會現形。所以本測試跑 12 期並斷言累計值。
     */
    @Test
    void multiPeriodPlatformCashConservationWhenRateEstimatedCorrectly() {
        final int periods = 12;
        final long costPerPeriod = 120L;
        FeePolicy p = FeePolicy.roundUp(1, 100);

        long cumulativeNetProceeds = 0L;
        long cumulativePaidToProvider = 0L;
        long carried = 0L;

        for (int i = 0; i < periods; i++) {
            long accrual = LogisticsAmounts.accrualAmount(costPerPeriod, 0, 0);
            long target = LogisticsAmounts.targetNet(accrual, carried);
            long gross = p.grossUp(target);
            long fee = p.fee(gross);
            long net = LogisticsAmounts.netProceeds(gross, fee);
            carried = LogisticsAmounts.carryForward(LogisticsAmounts.residual(target, net));

            cumulativeNetProceeds += net;
            cumulativePaidToProvider += costPerPeriod;
        }

        // 🔴 費率估準時：累計實收 − 累計付給 provider == 0。
        //    初版的 fee-on-fee 會讓這個差額穩定在 −1.2121×periods 的量級。
        assertEquals(0L, cumulativeNetProceeds - cumulativePaidToProvider,
                "平台累計現金差額必須為 0（實收 " + cumulativeNetProceeds
                        + "，付出 " + cumulativePaidToProvider + "）");
        // 費率估準 ⇒ 穩態 residual 恆為 0。
        assertEquals(0L, carried, "期末 carried residual 在費率估準時必須為 0");
        // 字面值：12 期 × 120 元。
        assertEquals(1440L, cumulativeNetProceeds);
    }

    /**
     * 同一條不變式，在<b>費率估不準</b>時：累計短收必須恰好等於期末未收的 residual。
     * <p>
     * 🔴 兩側相等才叫正確——「那筆錢確實還沒收到」與「帳上顯示還沒收到」是兩個獨立的事實，
     * 只驗其中一個會讓一個永久短收的模型全綠。
     */
    @Test
    void multiPeriodShortfallEqualsOutstandingResidualWhenRateUnderestimated() {
        final int periods = 12;
        // 🔴 同 Case 2：金額要大到費率差不被整數取整吃掉（見該測試的註解）。
        final long costPerPeriod = 12000L;
        FeePolicy est = FeePolicy.roundUp(1, 100);
        FeePolicy act = FeePolicy.roundUp(15, 1000);

        long cumulativeNetProceeds = 0L;
        long cumulativePaidToProvider = 0L;
        long carried = 0L;

        for (int i = 0; i < periods; i++) {
            long accrual = LogisticsAmounts.accrualAmount(costPerPeriod, 0, 0);
            long target = LogisticsAmounts.targetNet(accrual, carried);
            long gross = est.grossUp(target);
            long fee = act.fee(gross);
            long net = LogisticsAmounts.netProceeds(gross, fee);
            carried = LogisticsAmounts.carryForward(LogisticsAmounts.residual(target, net));

            cumulativeNetProceeds += net;
            cumulativePaidToProvider += costPerPeriod;
        }

        long shortfall = cumulativePaidToProvider - cumulativeNetProceeds;
        assertEquals(carried, shortfall,
                "累計短收 " + shortfall + " ≠ 期末未收 residual " + carried
                        + " —— 兩側不等代表有一筆錢在帳上消失了");
        assertTrue(carried > 0, "實際費率高於預估時，期末必須留有正的 residual，got " + carried);
    }

    /** 🔴 取整策略的鑑別力：用 {@code ceil(N/(1−r))} 會讓 residual 恆為負 ⇒ 應收永遠關不掉。 */
    @Test
    void ceilStrategyOverCollectsEveryPeriod() {
        // 🔴 **取整策略是契約的一部分，不是實作細節**（ADR-011 後果 3）。
        //
        // ⚠️ 本測試用「通道手續費捨去到元」的模型 —— ADR-011 表格那組數字
        // （ceil 策略 residual 恆為 −1）就是在這個模型下成立的。
        // 真實的 PayUni rounding 是 EV-P6、至今未知；本測試要釘死的不是那個絕對數字，
        // 而是**兩種取整策略會給出不同的 residual** —— 那才是「策略屬於契約」的理由。
        FeePolicy p = new FeePolicy(1, 100, true);
        long target = 120L;

        // 正解：滿足 G − fee(G) >= target 的最小整數。
        // G=121 ⇒ fee=floor(1.21)=1 ⇒ net=120 ⇒ 剛好打平。
        long minimal = p.grossUp(target);
        assertEquals(121L, minimal, "最小可行整數");
        assertEquals(0L, LogisticsAmounts.residual(target, minimal - p.fee(minimal)),
                "最小可行整數的 residual");

        // 反例：ceil(N/(1−r)) = ceil(12000/99) = 122 ⇒ net = 122 − 1 = 121 ⇒ residual = −1。
        // 🔴 後果不是「多收一元」而已：因為「residual ≠ 0 不得標記已收訖」，
        //    **每一筆應收都永遠關不掉**，並持續觸發 SETTLED 卡住偵測。
        //    這是「兩條規則各自正確、組合起來互相抵銷」的實例。
        long ceilGross = 122L;
        long residualCeil = LogisticsAmounts.residual(target, ceilGross - p.fee(ceilGross));
        assertEquals(-1L, residualCeil, "ceil 策略的 residual");
        assertFalse(LogisticsAmounts.isFullyCollected(residualCeil),
                "ceil 策略下 residual ≠ 0 卻被判為已收訖 ⇒ 應收永遠關不掉");
        // 而且它會產生一筆本不該存在的貸記。
        assertEquals(1L, LogisticsAmounts.creditFromResidual(residualCeil));
    }
}
