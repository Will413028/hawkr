package io.github.will413028.hawkr.payment.agentfee;

import java.math.BigInteger;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

/**
 * PayUni 多功能收付（{@code agent_fee}）的<b>已查證費率政策</b>。
 *
 * <h2>它為什麼住在 payment adapter 而不是 domain</h2>
 * {@code ADR-005} 明令「不得把 1% 寫成商業常數」，而 {@code ADR-011} 把該禁令的**範圍**釘清楚：
 * 禁的是「business domain 硬編費率」，不是「整個系統不准知道費率」。
 * 費率是<b>通道的技術參數</b>（可依合約調整），它本來就該住在這一層；
 * 物流財務的 domain 只說一句「我要淨收 N」，其餘一概不知道。
 *
 * <h2>🔴 fail-closed：沒有查證過的費率，就不准送出任何一筆</h2>
 * 四個欄位全部**沒有預設值**，這是刻意的：
 * <ul>
 *   <li>給 1% 當預設 ⇒ 「未確認」變成一個看不見的狀態，而金額會照樣送出去</li>
 *   <li>取整方向猜錯 ⇒ 整數制下會<b>系統性</b>短收或多收，而不是偶爾差一元</li>
 *   <li>gross／net 語意搞反 ⇒ 「扣 1% 後撥款」與「另外收 1%」差一個 {@code 1/(1−r)}</li>
 * </ul>
 * 以上每一項在 70 §10.3 都登記成 <b>{@code EV-P6}</b>，而它是
 * <b>Enablement Blocker 不是 Implementation Blocker</b>：schema／狀態機／對帳／residual
 * 機制全部可以先做完，缺的只是「可以自動送出」這個開關。
 *
 * <p>⚠️ 本類<b>不</b>決定開關要不要打開——那是同服務內 logistics 模組的
 * {@code SETTLEMENT_AUTO_COLLECTION_ENABLED}。本類只保證：**沒配置就算不出金額**。
 */
@Component
@ConfigurationProperties(prefix = "hawkr.payment.agent-fee.rate")
public class AgentFeeRatePolicy {

    /**
     * 費率的分子與分母（例：1% ⇒ 1 / 100；1.5% ⇒ 15 / 1000）。
     *
     * <p>🔴 <b>用整數分數而不是小數</b>：金額全程是元整數，而浮點在反推 gross 時會
     * 在邊界上偶發地差一元 —— 那種錯誤每一筆都很小、累計起來才現形，正是
     * {@code ADR-011} 初版沒抓到 fee-on-fee 的同一種盲點。
     */
    private Long rateNumerator;
    private Long rateDenominator;

    /**
     * 通道計算手續費時的取整方向。
     *
     * <p>🔴 <b>沒有預設值</b>。合法值：{@code UP}（無條件進位）／{@code DOWN}（無條件捨去）／
     * {@code HALF_UP}（四捨五入）。
     * ⚠️ 它不是實作細節：同一個費率在不同取整方向下，最小可行 gross 會差一元，
     * 而「residual ≠ 0 不得標記已收訖」會把那一元變成**永遠關不掉的應收**。
     */
    private Rounding rounding;

    /** 最低手續費（元）。官方未載 ⇒ null 代表「無下限」，非 0。 */
    private Long minFee;

    /** 最高手續費（元）。官方未載 ⇒ null 代表「無上限」。 */
    private Long maxFee;

    /**
     * 費率政策版本識別，寫進收款紀錄供事後對帳說明「當時用的是哪一版費率」。
     *
     * <p>🔴 沒有它，費率調整之後就再也解釋不了歷史那幾筆為什麼是那個金額。
     */
    private String version;

    /** 取整方向。 */
    public enum Rounding {
        UP,
        DOWN,
        HALF_UP
    }

    /**
     * 這份政策是否已完整查證（{@code EV-P6}）。
     *
     * <p>🔴 呼叫端在算任何金額之前必須先問這一句；回 false 時<b>不得</b>退回某個假設值。
     */
    public boolean isVerified() {
        return rateNumerator != null && rateNumerator >= 0
                && rateDenominator != null && rateDenominator > 0
                && rounding != null
                && version != null && !version.isBlank();
    }

    /**
     * 依本政策算出送出 {@code gross} 時通道實扣多少（元整數）。
     *
     * @throws IllegalStateException 政策未查證時 —— fail-closed，不猜
     */
    public long feeFor(long gross) {
        if (!isVerified()) {
            throw new IllegalStateException(
                    "agent_fee 費率政策未查證（EV-P6）：hawkr.payment.agent-fee.rate.* 必須完整設定。"
                            + " 缺少任一項時不得推導送出金額——猜一個值出來，每一筆的金額都是錯的。");
        }
        BigInteger num = BigInteger.valueOf(gross).multiply(BigInteger.valueOf(rateNumerator));
        BigInteger den = BigInteger.valueOf(rateDenominator);
        BigInteger[] qr = num.divideAndRemainder(den);
        long fee = qr[0].longValueExact();
        long remainder = qr[1].longValueExact();
        switch (rounding) {
            case UP -> {
                if (remainder != 0) {
                    fee++;
                }
            }
            case HALF_UP -> {
                // remainder * 2 >= den ⇒ 進位。用乘法比較避免除法造成的第二次取整。
                if (remainder * 2 >= rateDenominator) {
                    fee++;
                }
            }
            case DOWN -> {
                // 已經是 floor
            }
        }
        if (minFee != null && fee < minFee) {
            fee = minFee;
        }
        if (maxFee != null && fee > maxFee) {
            fee = maxFee;
        }
        return fee;
    }

    public Long getRateNumerator() {
        return rateNumerator;
    }

    public void setRateNumerator(Long rateNumerator) {
        this.rateNumerator = rateNumerator;
    }

    public Long getRateDenominator() {
        return rateDenominator;
    }

    public void setRateDenominator(Long rateDenominator) {
        this.rateDenominator = rateDenominator;
    }

    public Rounding getRounding() {
        return rounding;
    }

    public void setRounding(Rounding rounding) {
        this.rounding = rounding;
    }

    public Long getMinFee() {
        return minFee;
    }

    public void setMinFee(Long minFee) {
        this.minFee = minFee;
    }

    public Long getMaxFee() {
        return maxFee;
    }

    public void setMaxFee(Long maxFee) {
        this.maxFee = maxFee;
    }

    public String getVersion() {
        return version;
    }

    public void setVersion(String version) {
        this.version = version;
    }
}
