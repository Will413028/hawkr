package io.github.will413028.hawkr.payment.agentfee;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * {@code ADR-011} 的 gross-up 契約測試。
 *
 * <p>🔴 <b>斷言的期望值一律硬寫字面值</b>，不呼叫被測物也不引用它的常數：
 * 期望值跟著被測物一起變的話，這些測試驗不到任何東西。
 */
class AgentFeeGrossUpTest {

    private static AgentFeeRatePolicy policy(long num, long den, AgentFeeRatePolicy.Rounding r) {
        AgentFeeRatePolicy p = new AgentFeeRatePolicy();
        p.setRateNumerator(num);
        p.setRateDenominator(den);
        p.setRounding(r);
        p.setVersion("test-v1");
        return p;
    }

    @Test
    @DisplayName("🔴 費率未查證時 fail-closed：不得退回任何假設值（EV-P6 Enablement Gate）")
    void unverifiedPolicyFailsClosed() {
        AgentFeeRatePolicy p = new AgentFeeRatePolicy();
        // 只給費率、不給取整方向 —— 這是最容易發生的「查了一半」。
        p.setRateNumerator(1L);
        p.setRateDenominator(100L);
        p.setVersion("half-known");

        assertThat(p.isVerified()).isFalse();
        assertThatThrownBy(() -> new AgentFeeGrossUp(p).solve(120L))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("EV-P6");
    }

    @Test
    @DisplayName("捨去制 1%／目標 120 ⇒ 送出 121（最小可行整數），residual 為 0")
    void minimalFeasibleIntegerUnderRoundDown() {
        AgentFeeGrossUp g = new AgentFeeGrossUp(policy(1, 100, AgentFeeRatePolicy.Rounding.DOWN));

        AgentFeeGrossUp.Result r = g.solve(120L);

        // G=121 ⇒ fee=floor(1.21)=1 ⇒ net=120 ⇒ 剛好打平。
        assertThat(r.grossAmount()).isEqualTo(121L);
        assertThat(r.estimatedFee()).isEqualTo(1L);
        assertThat(r.grossAmount() - r.estimatedFee()).isEqualTo(120L);
    }

    @Test
    @DisplayName("🔴 ceil(N/(1−r)) 不是正解：它在同一組輸入下給 122 ⇒ 平台每期多收 1 元")
    void ceilFormulaOverCollectsComparedToMinimalFeasible() {
        AgentFeeRatePolicy p = policy(1, 100, AgentFeeRatePolicy.Rounding.DOWN);
        AgentFeeGrossUp g = new AgentFeeGrossUp(p);

        long minimal = g.solve(120L).grossAmount();
        // ceil(120 × 100 / 99) = ceil(121.21…) = 122
        long ceilFormula = 122L;

        assertThat(minimal).isEqualTo(121L);
        assertThat(ceilFormula - p.feeFor(ceilFormula) - 120L)
                .as("ceil 策略的實收超出目標淨額 ⇒ residual 為負 ⇒ 應收永遠關不掉")
                .isEqualTo(1L);
        assertThat(minimal).isLessThan(ceilFormula);
    }

    @Test
    @DisplayName("進位制 1%／目標 120 ⇒ 送出 122；實收剛好覆蓋目標，且沒有更小的可行值")
    void minimalFeasibleIntegerUnderRoundUp() {
        AgentFeeRatePolicy p = policy(1, 100, AgentFeeRatePolicy.Rounding.UP);
        AgentFeeGrossUp g = new AgentFeeGrossUp(p);

        AgentFeeGrossUp.Result r = g.solve(120L);

        assertThat(r.grossAmount()).isEqualTo(122L);
        assertThat(r.grossAmount() - r.estimatedFee()).isGreaterThanOrEqualTo(120L);
        // 121 不可行：fee=ceil(1.21)=2 ⇒ net=119 < 120。
        assertThat(121L - p.feeFor(121L)).isEqualTo(119L);
    }

    @Test
    @DisplayName("🔴 閉式解起跳必須與線性上探逐值相同——不是近似")
    void closedFormMatchesLinearSearchAcrossRange() {
        for (AgentFeeRatePolicy.Rounding mode : AgentFeeRatePolicy.Rounding.values()) {
            for (long[] rate : new long[][] {{1, 100}, {15, 1000}, {3, 100}, {7, 1000}}) {
                AgentFeeRatePolicy p = policy(rate[0], rate[1], mode);
                AgentFeeGrossUp g = new AgentFeeGrossUp(p);
                for (long target = 1; target <= 400; target++) {
                    long expected = linearSearch(p, target);
                    assertThat(g.solve(target).grossAmount())
                            .as("mode=%s rate=%d/%d target=%d", mode, rate[0], rate[1], target)
                            .isEqualTo(expected);
                }
            }
        }
    }

    /** 定義式：由 target 起逐一往上找第一個可行值。慢，但它就是「最小可行整數」的定義。 */
    private static long linearSearch(AgentFeeRatePolicy p, long target) {
        for (long gross = target; gross <= AgentFeeGrossUp.MAX_AMT; gross++) {
            if (gross - p.feeFor(gross) >= target) {
                return gross;
            }
        }
        throw new AssertionError("無解");
    }

    @Test
    @DisplayName("🔴 實收永遠覆蓋目標淨額：跨四種費率 × 三種取整 × 400 個金額，無一例外")
    void netProceedsAlwaysCoversTargetNet() {
        for (AgentFeeRatePolicy.Rounding mode : AgentFeeRatePolicy.Rounding.values()) {
            for (long[] rate : new long[][] {{1, 100}, {15, 1000}, {3, 100}, {7, 1000}}) {
                AgentFeeRatePolicy p = policy(rate[0], rate[1], mode);
                AgentFeeGrossUp g = new AgentFeeGrossUp(p);
                for (long target = 1; target <= 400; target++) {
                    AgentFeeGrossUp.Result r = g.solve(target);
                    assertThat(r.grossAmount() - r.estimatedFee())
                            .as("mode=%s rate=%d/%d target=%d ⇒ 平台短收而沒有任何訊號",
                                    mode, rate[0], rate[1], target)
                            .isGreaterThanOrEqualTo(target);
                }
            }
        }
    }

    @Test
    @DisplayName("目標淨額 <= 0 必須被擋在我方，而不是變成一筆送出必失敗的請求")
    void nonPositiveTargetIsRejectedLocally() {
        AgentFeeGrossUp g = new AgentFeeGrossUp(policy(1, 100, AgentFeeRatePolicy.Rounding.UP));
        assertThatThrownBy(() -> g.solve(0L)).isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> g.solve(-5L)).isInstanceOf(IllegalArgumentException.class);
    }

    @Test
    @DisplayName("反推結果超出 PayUni Amt 上限（9999999）時 fail-loud")
    void overflowBeyondOfficialAmountCeilingFailsLoud() {
        AgentFeeGrossUp g = new AgentFeeGrossUp(policy(1, 100, AgentFeeRatePolicy.Rounding.UP));
        assertThatThrownBy(() -> g.solve(9_999_999L))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("9999999");
    }

    @Test
    @DisplayName("費率 >= 100% 無解，必須當成設定錯誤 fail-loud 而不是無限迴圈")
    void rateAtOrAboveHundredPercentHasNoSolution() {
        AgentFeeGrossUp g = new AgentFeeGrossUp(policy(100, 100, AgentFeeRatePolicy.Rounding.DOWN));
        assertThatThrownBy(() -> g.solve(120L))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("100%");
    }

    @Test
    @DisplayName("min／max fee 是官方未載的選配：null 代表沒有限制，不是 0")
    void minAndMaxFeeAreOptionalAndNullMeansUnbounded() {
        AgentFeeRatePolicy p = policy(1, 100, AgentFeeRatePolicy.Rounding.DOWN);
        assertThat(p.feeFor(100L)).isEqualTo(1L);

        p.setMinFee(5L);
        assertThat(p.feeFor(100L)).as("低於下限時墊到下限").isEqualTo(5L);

        p.setMinFee(null);
        p.setMaxFee(0L);
        assertThat(p.feeFor(100L)).as("上限 0 代表真的不收費，與 null 是兩件事").isEqualTo(0L);
    }
}
