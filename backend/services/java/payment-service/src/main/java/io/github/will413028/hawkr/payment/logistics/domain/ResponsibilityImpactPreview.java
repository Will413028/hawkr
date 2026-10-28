package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import java.time.LocalDateTime;

/**
 * 責任判定的財務影響預覽（WP2A-R2 PM Decision）。
 *
 * <p>🔴 <b>刻意只有一個金額</b>。65 的 A-4 原本列了三個
 * （{@code refundAmount}／{@code platformFeeDelta}／{@code sellerNetDelta}），
 * 但後兩者在現行商流下<b>沒有 canonical 算法，而且語意不成立</b>：
 * <ul>
 *   <li>唯一權威公式 {@code PayoutInstructionFee} 只存在於 65 的文件虛擬碼，
 *       該章自述「零消費者」，整條代銷／撥款軸已由 {@code DEC-20260912-184025} 判定 dormant</li>
 *   <li>{@code OrderPolicyResolver.resolve()} 寫死回 {@code SELLER_DIRECT}，
 *       而該商流下<b>平台沒有代收</b> ⇒ 訂單層的「平台費差額」根本不存在</li>
 *   <li>費率未查證（EV-P6／U-2），{@code AgentFeeRatePolicy.feeFor()} 會直接 throw</li>
 * </ul>
 * ⇒ 它們<b>不是 0，而是不屬於本 contract</b>。把它們填 0 會讓覆核者看到一個
 * 「平台與賣家都不受影響」的假結論，比不顯示更糟。
 *
 * <p>🔴 <b>{@code computedAt} 不參與 stale 比較</b>：重算必然產生新的時間戳，
 * 把它納入 equality 會讓每一次覆核都判定 stale（那道守衛就永遠是狼來了）。
 * 它只回答「這份快照是什麼時候算的」。
 *
 * @param refundAmount 退款給買家的金額，單位「元」的整數（禁浮點，CLAUDE.md §4）
 * @param computedAt   快照計算時間（UTC）
 */
public record ResponsibilityImpactPreview(long refundAmount, LocalDateTime computedAt) {

    /**
     * 依責任成因與權威退款基準算出預覽。
     *
     * <p>🔴 <b>規則是二元的，不是公式</b>（65 F 逐字：「沒有第三種拆分公式…
     * 本設計不引入 refund component model」）：
     * <table border="1">
     *   <caption>成因 → 退款金額</caption>
     *   <tr><td>{@code BUYER_CAUSED}</td><td><b>0</b>（走 {@code DELIVERY_FAILED_NO_REFUND(54)}）</td></tr>
     *   <tr><td>其餘三個 resolved 成因</td><td><b>全額</b>（商品款 ＋ 買方運費）</td></tr>
     * </table>
     *
     * @param cause            四個 resolved 成因之一（由 maker 提案時主張並持久化，V13）
     * @param fullRefundBasis  權威全額退款基準 ＝ {@code orders.total_actual_amount}
     * @param computedAt       計算時間（UTC）
     */
    public static ResponsibilityImpactPreview of(String cause, long fullRefundBasis,
                                                 LocalDateTime computedAt) {
        if (!ResponsibilityCaseRules.isFinalizableCause(cause)) {
            // fail-closed：判不出成因就算不出金額，不得默默回 0
            throw new IllegalArgumentException(
                    "responsibility: 無法以非 resolved 成因計算影響預覽（收到 " + cause + "）");
        }
        if (fullRefundBasis < 0) {
            throw new IllegalArgumentException(
                    "responsibility: 退款基準不得為負（收到 " + fullRefundBasis + "）");
        }
        long refund = ResponsibilityCause.BUYER_CAUSED.value().equals(cause) ? 0L : fullRefundBasis;
        return new ResponsibilityImpactPreview(refund, computedAt);
    }

    /**
     * stale 比較：<b>只比業務影響值</b>，不比 {@link #computedAt}。
     *
     * <p>🔴 這是 65 硬約束 2 的實質：「覆核者看到的金額與實際執行的金額不同，
     * 就是 maker-checker 在保護的那件事本身失效」。
     */
    public boolean sameBusinessImpactAs(ResponsibilityImpactPreview other) {
        return other != null && this.refundAmount == other.refundAmount;
    }
}
