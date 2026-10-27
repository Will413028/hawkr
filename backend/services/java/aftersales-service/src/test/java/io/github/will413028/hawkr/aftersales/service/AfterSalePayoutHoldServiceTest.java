package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.HoldReason;
import io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.PayoutHold;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * backend#3262 撥款資格閘門（{@link AfterSalePayoutHoldService}）。
 *
 * <p>對應 issue 的 Regression Case：
 * <ul>
 *   <li><b>Case A</b> 售後被拒 → 不申訴 → 合法窗口結束 → 撥款放行（{@link #caseA_windowClosed_released}）</li>
 *   <li><b>Case B</b> 合法 appeal → 無時間界 hold，撥款不得先於裁決（{@link #caseB_appealed_indefiniteHold}）</li>
 *   <li><b>Case D</b> AutoComplete 路徑（33 待寄回逾時關案）與 appeal window 的關係
 *       （{@link #caseD_autoClosed33_holdsUntilAppealDeadline}）</li>
 *   <li><b>Case E</b> 窗到期 ⇒ 最終必然放行，不得形成永久 block（{@link #caseE_neverPermanentlyBlocked}）</li>
 * </ul>
 *
 * <p>時間一律用相對 {@link DateTimeUtils#nowUtc()} 的偏移，不 sleep、不依賴本機 timezone。
 */
@ExtendWith(MockitoExtension.class)
class AfterSalePayoutHoldServiceTest {

    private static final String ORDER_NO = "O20260909001";
    private static final int DEADLINE_HOURS = 72;
    private static final int GRACE_MINUTES = 15;

    @Mock private AfterSaleCaseMapper caseMapper;

    private AfterSalePayoutHoldService service;

    @BeforeEach
    void setUp() {
        AfterSalesProperties props = new AfterSalesProperties();
        props.getAppeal().setDeadlineHours(DEADLINE_HOURS);
        props.getAppeal().setPayoutHoldGraceMinutes(GRACE_MINUTES);
        service = new AfterSalePayoutHoldService(caseMapper, props);
    }

    private AfterSaleCaseEntity aCase(String caseNo, AfterSaleCaseStatus status, String appealState) {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(caseNo);
        c.setOrderNo(ORDER_NO);
        c.setStatus(status.value());
        c.setAppealState(appealState);
        return c;
    }

    private void givenCases(AfterSaleCaseEntity... cases) {
        when(caseMapper.selectList(any())).thenReturn(List.of(cases));
    }

    /**
     * backend#3262 P0：模擬「該 case 當下無人持鎖」＝沒有在途申訴。
     *
     * <p>刻意<b>不</b>做成預設行為：Mockito 對未 stub 的方法回 null，而 null ＝「被持鎖」
     * ＝ fail-closed 不放行。於是任何「期望放行」的測試都<b>必須顯式</b>宣告這件事，
     * 忘了寫就會轉紅——這正是我們要的方向（漏掉探測的預設結果是擋，不是放）。
     */
    private void givenNobodyHoldsLock(AfterSaleCaseEntity fresh) {
        when(caseMapper.tryLockCaseForPayoutRelease(fresh.getCaseNo())).thenReturn(fresh);
    }

    // ==================== 無 case：絕大多數訂單 ====================

    @Test
    @DisplayName("訂單無售後 case ⇒ 不 hold（撥款 cadence 完全不受本閘門影響）")
    void noCases_released() {
        when(caseMapper.selectList(any())).thenReturn(List.of());
        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertFalse(hold.hold());
        assertEquals(HoldReason.NONE, hold.reason());
        assertNull(hold.holdUntil());
    }

    @Test
    @DisplayName("selectList 回 null（防禦）⇒ 不 hold")
    void nullCaseList_released() {
        when(caseMapper.selectList(any())).thenReturn(null);
        assertFalse(service.getPayoutHold(ORDER_NO).hold());
    }

    // ==================== Case A / E：窗關了就放行，不會永久卡死 ====================

    @Test
    @DisplayName("Case A/E：REJECTED(60) 未申訴、last_rejected_at 已超過 72h+grace ⇒ 放行")
    void caseA_windowClosed_released() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS + 1));
        givenCases(c);
        givenNobodyHoldsLock(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertFalse(hold.hold(), "申訴窗已過，買家再也不能申訴 ⇒ 必須放行，否則賣家款項永久卡死");
        assertEquals(HoldReason.NONE, hold.reason());
    }

    @Test
    @DisplayName("Case E：dev 上停在 60/未申訴/窗已過的那 32 筆形狀，一律放行（C5③ 原寫法會全部卡死）")
    void caseE_neverPermanentlyBlocked() {
        for (AfterSaleCaseStatus s : List.of(AfterSaleCaseStatus.REJECTED,
                AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT)) {
            AfterSaleCaseEntity c = aCase("AS-" + s.value(), s, AfterSaleAppealState.NONE.value());
            LocalDateTime longAgo = DateTimeUtils.nowUtc().minusDays(60);
            c.setLastRejectedAt(longAgo);
            c.setClosedAt(longAgo);
            givenCases(c);
            givenNobodyHoldsLock(c);
            assertFalse(service.getPayoutHold(ORDER_NO).hold(),
                    s.name() + " 窗已過必須放行");
        }
    }

    // ==================== 風險窗內：本票要關的洞 ====================

    @Test
    @DisplayName("REJECTED(60) 未申訴、仍在 72h 窗內 ⇒ hold 到 deadline+grace（有時間界）")
    void rejectedWithinWindow_holdsUntilDeadlinePlusGrace() {
        LocalDateTime rejectedAt = DateTimeUtils.nowUtc().minusHours(1);
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(rejectedAt);
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(HoldReason.APPEAL_WINDOW_OPEN, hold.reason());
        assertEquals(rejectedAt.plusHours(DEADLINE_HOURS).plusMinutes(GRACE_MINUTES), hold.holdUntil());
        assertEquals("AS1", hold.caseNo());
    }

    @Test
    @DisplayName("grace 有作用：deadline 剛過但仍在 grace 內 ⇒ 還在 hold（關掉 submitAppeal 的 TOCTOU）")
    void withinGrace_stillHeld() {
        // last_rejected_at = now - 72h - 1min ⇒ deadline 已過 1 分鐘，但 grace 15 分鐘還沒走完
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS).minusMinutes(1));
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(), "deadline 已過但 grace 未走完 ⇒ 仍須 hold，否則跨越截止的申訴交易會被撥款超車");
        assertEquals(HoldReason.APPEAL_WINDOW_OPEN, hold.reason());
    }

    @Test
    @DisplayName("Case D：AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33) 以 closed_at 起算，窗內 ⇒ hold")
    void caseD_autoClosed33_holdsUntilAppealDeadline() {
        LocalDateTime closedAt = DateTimeUtils.nowUtc().minusHours(2);
        AfterSaleCaseEntity c = aCase("AS33", AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT,
                AfterSaleAppealState.NONE.value());
        c.setClosedAt(closedAt);
        // last_rejected_at 為 null——33 的起算點必須是 closed_at，取錯欄位會靜默放行
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(closedAt.plusHours(DEADLINE_HOURS).plusMinutes(GRACE_MINUTES), hold.holdUntil());
    }

    // ==================== Case B：申訴中 ====================

    @Test
    @DisplayName("Case B：60＋APPEALED ⇒ 無時間界 hold（等 Admin 裁決，不得靠時間自動放行）")
    void caseB_appealed_indefiniteHold() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.APPEALED.value());
        // 即使 last_rejected_at 早就超過窗，只要申訴已成立就不能因為「時間到了」而撥款
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusDays(30));
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(HoldReason.ACTIVE_CASE, hold.reason());
        assertNull(hold.holdUntil(), "申訴審理無可預期時點 ⇒ 不得帶時間界（帶了就會到期自動撥款）");
    }

    @Test
    @DisplayName("Admin 裁決 FULL_REFUND 後 case 進 REFUND_PROCESSING(40) ⇒ 退款完成前仍 hold")
    void refundProcessing_indefiniteHold() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REFUND_PROCESSING,
                AfterSaleAppealState.RESOLVED.value());
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(HoldReason.ACTIVE_CASE, hold.reason());
        assertNull(hold.holdUntil());
    }

    @Test
    @DisplayName("Admin 駁回申訴 → CLOSED(50) ⇒ 放行（賣家該拿到錢）")
    void appealRejectedThenClosed_released() {
        givenCases(aCase("AS1", AfterSaleCaseStatus.CLOSED, AfterSaleAppealState.RESOLVED.value()));
        assertFalse(service.getPayoutHold(ORDER_NO).hold());
    }

    // ==================== 時間戳缺漏與多 case ====================

    @Test
    @DisplayName("REJECTED 但 last_rejected_at 為 null ⇒ submitAppeal 的 A3 也會拒絕 ⇒ 不 hold")
    void rejectedWithoutTimestamp_released() {
        givenCases(aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value()));
        assertFalse(service.getPayoutHold(ORDER_NO).hold(),
                "申訴起算點缺漏 ⇒ 申訴不可能成立 ⇒ 沒有 hold 的理由");
    }

    @Test
    @DisplayName("多 case：無時間界者支配有時間界者")
    void indefiniteDominatesBounded() {
        AfterSaleCaseEntity bounded = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        bounded.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(1));
        AfterSaleCaseEntity indefinite = aCase("AS2", AfterSaleCaseStatus.REFUND_PROCESSING,
                AfterSaleAppealState.RESOLVED.value());
        givenCases(bounded, indefinite);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(HoldReason.ACTIVE_CASE, hold.reason());
        assertNull(hold.holdUntil());
    }

    @Test
    @DisplayName("多 case 都有時間界 ⇒ 取最晚的那個時點（取最早會提前撥款）")
    void boundedTakesLatestDeadline() {
        LocalDateTime early = DateTimeUtils.nowUtc().minusHours(10);
        LocalDateTime late = DateTimeUtils.nowUtc().minusHours(1);
        AfterSaleCaseEntity c1 = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c1.setLastRejectedAt(early);
        AfterSaleCaseEntity c2 = aCase("AS2", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c2.setLastRejectedAt(late);
        givenCases(c1, c2);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertEquals(late.plusHours(DEADLINE_HOURS).plusMinutes(GRACE_MINUTES), hold.holdUntil());
        assertEquals("AS2", hold.caseNo());
    }

    @Test
    @DisplayName("已終結的 case 與窗內 case 並存 ⇒ 仍 hold（不得被 CANCELLED 的那張稀釋）")
    void terminalCaseDoesNotReleaseAnotherHeldCase() {
        AfterSaleCaseEntity done = aCase("AS0", AfterSaleCaseStatus.CANCELLED, AfterSaleAppealState.NONE.value());
        AfterSaleCaseEntity held = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        held.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(1));
        givenCases(done, held);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals("AS1", hold.caseNo());
    }

    @Test
    @DisplayName("status 碼值讀不出來（髒資料 / 未來新碼）⇒ 擋，不放行")
    void unknownStatusValue_blocked() {
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo("ASX");
        c.setOrderNo(ORDER_NO);
        c.setStatus("999");
        c.setAppealState(AfterSaleAppealState.NONE.value());
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(), "看不到就擋，不是看不到就放行");
        assertEquals(HoldReason.ACTIVE_CASE, hold.reason());
        assertNotNull(hold.caseNo());
    }

    // ==================== backend#3262 P0：在途申訴探測（QA FAIL 後新增）====================
    //
    // 這一組取代 grace 成為 correctness 的來源。判準不是「15 分鐘夠不夠久」，
    // 而是「撥款放行」與「一筆已通過 A3 的申訴落地」在順序上不可能同時成立。
    // submitAppeal 在做任何資格判定之前先 SELECT … FOR UPDATE 鎖住該 case（持有到 commit），
    // 本閘門在因窗已過而放行之前先以 FOR UPDATE SKIP LOCKED 探測同一列，探不到就擋。

    @Test
    @DisplayName("🔴 P0：窗（含 grace）已過但該 case 正被持鎖 ⇒ fail-closed 不放行（在途申訴）")
    void windowClosedButRowLocked_holdsFailClosed() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        // 駁回於 73 小時前：deadline 已過 1 小時、deadline+grace 也已過 45 分
        // ——舊版在這一刻無條件放行，而 markAppealed 的 WHERE 沒有任何時間述詞
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS + 1));
        givenCases(c);
        when(caseMapper.tryLockCaseForPayoutRelease("AS1")).thenReturn(null);   // 被別的交易持鎖

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(),
                "P0：有交易正壓在這一列上，可能是一筆已通過 A3、尚未 commit 的申訴 ⇒ 不得放行撥款");
        assertEquals(HoldReason.APPEAL_IN_FLIGHT, hold.reason());
        assertNull(hold.holdUntil(),
                "在途申訴沒有可預期的結束時點 ⇒ 無時間界，呼叫端推遲後重問（不是永久卡死）");
        assertEquals("AS1", hold.caseNo());
    }

    @Test
    @DisplayName("🔴 P0：grace = 0 時仍然安全 ⇒ 證明 correctness 不再建立在 grace 的長度上")
    void graceZero_stillFailsClosedWhenRowLocked() {
        AfterSalesProperties zeroGrace = new AfterSalesProperties();
        zeroGrace.getAppeal().setDeadlineHours(DEADLINE_HOURS);
        zeroGrace.getAppeal().setPayoutHoldGraceMinutes(0);
        AfterSalePayoutHoldService noGrace = new AfterSalePayoutHoldService(caseMapper, zeroGrace);

        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        // 只超過 deadline 1 秒：舊版在 grace=0 下會立刻放行
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS).minusSeconds(1));
        givenCases(c);
        when(caseMapper.tryLockCaseForPayoutRelease("AS1")).thenReturn(null);

        PayoutHold hold = noGrace.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(), "grace 歸零也不得誤放行——安全來自持鎖順序，不是來自 grace 的長度");
        assertEquals(HoldReason.APPEAL_IN_FLIGHT, hold.reason());
    }

    @Test
    @DisplayName("P0：探測只發生在『因窗已過而放行』這一步——窗內本來就擋，不必也不該去探鎖")
    void withinWindow_doesNotProbeLock() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(1));
        givenCases(c);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(HoldReason.APPEAL_WINDOW_OPEN, hold.reason());
        // 未 stub tryLock 仍不失敗 ⇒ 證明該路徑沒去探；同時避免探測成為每次查詢的固定成本
        verify(caseMapper, never()).tryLockCaseForPayoutRelease(anyString());
    }

    @Test
    @DisplayName("🔴 P0：閘門的第一次讀是舊快照（申訴在那之後才 commit）⇒ 持鎖重讀必須抓到，不得放行")
    void staleSnapshotThenAppealCommitted_holdsFailClosed() {
        // 交錯：① selectList 讀到 appeal_state='0'（當時申訴交易還持鎖未 commit）
        //       ② 申訴 commit、鎖釋放
        //       ③ 探測「鎖得到」——若只看可鎖性就會放行，而申訴其實已經成立
        AfterSaleCaseEntity snapshot = aCase("AS1", AfterSaleCaseStatus.REJECTED,
                AfterSaleAppealState.NONE.value());
        snapshot.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS + 1));
        givenCases(snapshot);

        AfterSaleCaseEntity fresh = aCase("AS1", AfterSaleCaseStatus.REJECTED,
                AfterSaleAppealState.APPEALED.value());     // ← 重讀時已是申訴中
        fresh.setLastRejectedAt(snapshot.getLastRejectedAt());
        when(caseMapper.tryLockCaseForPayoutRelease("AS1")).thenReturn(fresh);

        PayoutHold hold = service.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(),
                "P0：鎖拿得到不代表沒事——必須用重讀到的新鮮那一列重跑判定");
        assertEquals(HoldReason.APPEAL_IN_FLIGHT, hold.reason());
        assertNull(hold.holdUntil());
    }

    @Test
    @DisplayName("P0：持鎖重讀後狀態未變（仍 60/未申訴、窗已過）⇒ 正常放行（重讀不是恆擋）")
    void freshRowUnchanged_stillReleases() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED,
                AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(DateTimeUtils.nowUtc().minusHours(DEADLINE_HOURS + 1));
        givenCases(c);
        givenNobodyHoldsLock(c);

        assertFalse(service.getPayoutHold(ORDER_NO).hold(),
                "重讀結果與快照一致且窗已過 ⇒ 必須放行，否則賣家款項永久卡死");
    }

    @Test
    @DisplayName("P0：deadline 起算時間戳缺漏（申訴本來就不可能成立）⇒ 直接放行，不必探鎖")
    void nullDeadline_releasedWithoutProbe() {
        AfterSaleCaseEntity c = aCase("AS1", AfterSaleCaseStatus.REJECTED, AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(null);
        givenCases(c);

        assertFalse(service.getPayoutHold(ORDER_NO).hold());
        verify(caseMapper, never()).tryLockCaseForPayoutRelease(anyString());
    }
}
