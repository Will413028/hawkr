package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.helper.AfterSaleCaseStatusRules.PayoutBlock;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.EnumSet;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

/**
 * backend#3262 撥款阻擋分類（{@link AfterSaleCaseStatusRules#payoutBlock}）。
 *
 * <p>本測試的鑑別力來自兩件事：
 * <ol>
 *   <li><b>窮舉</b>：對 {@code AfterSaleCaseStatus} 的<b>每一個</b> enum 值都斷言，
 *       不是挑幾個代表。新增碼值而沒有想過撥款語意時，這裡會直接紅
 *       （放行集合寫死成字面清單，新值落入 else ⇒ INDEFINITE ⇒ 與清單比對時被抓）。</li>
 *   <li><b>deny-by-default 的反面</b>：斷言「不在放行清單裡的一律不是 NONE」，
 *       而不只是斷言放行清單裡的是 NONE——後者對「多放行了一個」零鑑別力。</li>
 * </ol>
 */
class AfterSaleCaseStatusRulesPayoutBlockTest {

    private static final String NONE_STATE = AfterSaleAppealState.NONE.value();        // "0"
    private static final String APPEALED = AfterSaleAppealState.APPEALED.value();      // "10"
    private static final String INVESTIGATING = AfterSaleAppealState.INVESTIGATING.value(); // "20"
    private static final String RESOLVED = AfterSaleAppealState.RESOLVED.value();      // "30"

    /** 確定不會再生出退款的三個終態。 */
    private static final Set<AfterSaleCaseStatus> RELEASED = EnumSet.of(
            AfterSaleCaseStatus.CLOSED,                        // 50
            AfterSaleCaseStatus.CANCELLED,                     // 70
            AfterSaleCaseStatus.CANCELLED_BY_CONFIRM_RECEIPT); // 71

    /** 尚可申訴（有時間界）的兩個狀態。 */
    private static final Set<AfterSaleCaseStatus> APPEALABLE = EnumSet.of(
            AfterSaleCaseStatus.REJECTED,                              // 60
            AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT);// 33

    @Test
    @DisplayName("窮舉全部 status × appeal_state=NONE：只有 50/70/71 放行，60/33 有時間界，其餘無時間界")
    void exhaustiveWithNoAppeal() {
        for (AfterSaleCaseStatus s : AfterSaleCaseStatus.values()) {
            PayoutBlock actual = AfterSaleCaseStatusRules.payoutBlock(s, NONE_STATE);
            PayoutBlock expected = RELEASED.contains(s) ? PayoutBlock.NONE
                    : APPEALABLE.contains(s) ? PayoutBlock.UNTIL_APPEAL_DEADLINE
                    : PayoutBlock.INDEFINITE;
            assertEquals(expected, actual, s.name() + "(" + s.value() + ")");
        }
    }

    @Test
    @DisplayName("deny-by-default：放行清單以外的 status 一律不得回 NONE（多放行一個就紅）")
    void nothingOutsideReleaseSetIsReleased() {
        for (AfterSaleCaseStatus s : AfterSaleCaseStatus.values()) {
            if (RELEASED.contains(s)) {
                continue;
            }
            for (String state : new String[]{NONE_STATE, APPEALED, INVESTIGATING, RESOLVED, null, "999"}) {
                assertNotEquals(PayoutBlock.NONE, AfterSaleCaseStatusRules.payoutBlock(s, state),
                        s.name() + " × appealState=" + state + " 不得放行撥款");
            }
        }
    }

    @Test
    @DisplayName("60/33 一旦申訴 ⇒ 無時間界（等 Admin 裁決，不能靠時間自動放行）")
    void appealedRejectedIsIndefinite() {
        for (AfterSaleCaseStatus s : APPEALABLE) {
            assertEquals(PayoutBlock.INDEFINITE, AfterSaleCaseStatusRules.payoutBlock(s, APPEALED),
                    s.name() + " 申訴中應無時間界");
        }
    }

    @Test
    @DisplayName("appeal_state 為 INVESTIGATING(20)（Phase 2 保留值，目前不寫入）也必須擋——deny-by-default")
    void investigatingIsIndefinite() {
        for (AfterSaleCaseStatus s : APPEALABLE) {
            assertEquals(PayoutBlock.INDEFINITE, AfterSaleCaseStatusRules.payoutBlock(s, INVESTIGATING),
                    s.name() + " × INVESTIGATING 應擋（未來 Phase 2 啟用時不得靜默放行）");
        }
    }

    @Test
    @DisplayName("appeal_state=null 視同 NONE（DB 歷史列可能為 null，不得因此被當成已申訴或被誤放行）")
    void nullAppealStateTreatedAsNone() {
        assertEquals(PayoutBlock.UNTIL_APPEAL_DEADLINE,
                AfterSaleCaseStatusRules.payoutBlock(AfterSaleCaseStatus.REJECTED, null));
        assertEquals(PayoutBlock.NONE,
                AfterSaleCaseStatusRules.payoutBlock(AfterSaleCaseStatus.CLOSED, null));
    }

    @Test
    @DisplayName("status=null（碼值讀不出來）⇒ 擋，不放行")
    void nullStatusIsBlocked() {
        assertEquals(PayoutBlock.INDEFINITE, AfterSaleCaseStatusRules.payoutBlock(null, NONE_STATE));
        assertEquals(PayoutBlock.INDEFINITE, AfterSaleCaseStatusRules.payoutBlock(null, null));
    }

    @Test
    @DisplayName("與 isActive 的分歧點就是本票的洞：60/33＋未申訴 → isActive=false 但撥款必須擋")
    void divergesFromIsActiveExactlyAtTheBugSite() {
        for (AfterSaleCaseStatus s : APPEALABLE) {
            assertEquals(false, AfterSaleCaseStatusRules.isActive(s, NONE_STATE),
                    s.name() + " 在 isActive 下不活躍（訂單該完成、倒數該恢復）");
            assertEquals(PayoutBlock.UNTIL_APPEAL_DEADLINE,
                    AfterSaleCaseStatusRules.payoutBlock(s, NONE_STATE),
                    s.name() + " 但撥款必須擋到申訴窗關閉——兩個判定不是同一個問題");
        }
    }
}
