package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.MediaGrpcClient;
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
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

/**
 * backend#3262 P0：撥款閘門的 <b>deadline / grace 邊界矩陣</b>，以及
 * 「grace 已被降級成安全餘裕」這件事的機械斷言。
 *
 * <p><b>兩件事要分清楚，本檔各自有斷言</b>：
 * <ul>
 *   <li><b>時間邊界</b>決定「什麼時候<i>可以考慮</i>放行」——那仍然是 grace 說了算，
 *       因為它是時鐘偏移預算（{@link #graceBoundary_beforeAndAfter}）。</li>
 *   <li><b>能不能<i>真的</i>放行</b>由持鎖探測決定，與 grace 的長度無關
 *       （{@link #atEveryReleasableInstant_lockProbeStillDecides}）。</li>
 * </ul>
 *
 * <p><b>為什麼「窗內」用 ±1ms 而「窗外」也用 1ms、但「剛好在窗內」用 2 秒</b>：
 * 本 service 讀的是真實 wall clock（{@code DateTimeUtils.nowUtc()}），測試無法注入時鐘。
 * 服務讀到的 {@code now} 必然<b>晚於</b>測試計算基準點數微秒到數毫秒 ⇒
 * 「已經在窗外 1ms」是<b>單調安全</b>的（只會更晚、更確定在外）；
 * 「還差 1ms 才出窗」則會被那點漂移翻面 ⇒ 該方向一律取 2 秒，
 * 讓斷言只在「行為錯了」時轉紅，不在「機器慢了」時轉紅。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSalePayoutHoldBoundaryTest {

    private static final String ORDER_NO = "O3262BND";
    private static final String CASE_NO = "AS-BND";
    private static final int DEADLINE_HOURS = 72;
    private static final int GRACE_MINUTES = 15;

    @Mock private AfterSaleCaseMapper caseMapper;

    private AfterSalePayoutHoldService service;

    @BeforeEach
    void setUp() {
        service = newService(GRACE_MINUTES);
    }

    private AfterSalePayoutHoldService newService(int graceMinutes) {
        AfterSalesProperties props = new AfterSalesProperties();
        props.getAppeal().setDeadlineHours(DEADLINE_HOURS);
        props.getAppeal().setPayoutHoldGraceMinutes(graceMinutes);
        return new AfterSalePayoutHoldService(caseMapper, props);
    }

    /**
     * 讓 {@code deadline + grace}（＝閘門的放行時點）落在「現在 + offsetMillis」。
     * offsetMillis 為正 ＝ 還沒到放行時點；為負 ＝ 已過。
     */
    private void givenReleaseInstantAt(long offsetMillis, int graceMinutes) {
        LocalDateTime lastRejectedAt = DateTimeUtils.nowUtc()
                .plusNanos(offsetMillis * 1_000_000L)
                .minusMinutes(graceMinutes)
                .minusHours(DEADLINE_HOURS);
        AfterSaleCaseEntity c = new AfterSaleCaseEntity();
        c.setCaseNo(CASE_NO);
        c.setOrderNo(ORDER_NO);
        c.setStatus(AfterSaleCaseStatus.REJECTED.value());
        c.setAppealState(AfterSaleAppealState.NONE.value());
        c.setLastRejectedAt(lastRejectedAt);
        when(caseMapper.selectList(any())).thenReturn(List.of(c));
        current = c;
    }

    /** {@link #givenReleaseInstantAt} 建出的那一列，供持鎖重讀回傳（＝重讀結果與快照一致）。 */
    private AfterSaleCaseEntity current;

    private void nobodyHoldsLock() {
        when(caseMapper.tryLockCaseForPayoutRelease(anyString()))
                .thenAnswer(inv -> current);
    }

    // ==================== deadline 附近：三個點都還在 grace 之內，必須擋 ====================

    @Test
    @DisplayName("deadline − 1ms / deadline / deadline + 1ms ⇒ 三者皆在 grace 內 ⇒ 一律有時間界 hold")
    void aroundDeadline_alwaysHeldWithBound() {
        nobodyHoldsLock();
        for (long deltaMillis : new long[]{-1L, 0L, +1L}) {
            // 放行時點 = deadline + grace ⇒ 距今 = grace − delta，恆為正且達分鐘級 ⇒ 不受時鐘漂移影響
            givenReleaseInstantAt(GRACE_MINUTES * 60_000L - deltaMillis, GRACE_MINUTES);

            PayoutHold hold = service.getPayoutHold(ORDER_NO);
            assertTrue(hold.hold(), "deadline 偏移 " + deltaMillis + "ms：仍在 grace 內，必須擋");
            assertEquals(HoldReason.APPEAL_WINDOW_OPEN, hold.reason());
            assertTrue(hold.holdUntil() != null, "窗內的 hold 必須帶時間界（否則賣家款項無到期日）");
        }
    }

    // ==================== grace 邊界：跨過去才允許考慮放行 ====================

    @Test
    @DisplayName("grace 邊界：尚未跨過 ⇒ 有時間界 hold；已跨過 ⇒ 才輪到持鎖探測決定")
    void graceBoundary_beforeAndAfter() {
        nobodyHoldsLock();

        // 還差 2 秒才到放行時點（見類別 javadoc：此方向取秒級避免時鐘漂移翻面）
        givenReleaseInstantAt(2_000L, GRACE_MINUTES);
        PayoutHold before = service.getPayoutHold(ORDER_NO);
        assertTrue(before.hold(), "尚未跨過 deadline+grace ⇒ 必須擋");
        assertEquals(HoldReason.APPEAL_WINDOW_OPEN, before.reason());

        // 已過放行時點 1ms（單調安全方向）
        givenReleaseInstantAt(-1L, GRACE_MINUTES);
        PayoutHold after = service.getPayoutHold(ORDER_NO);
        assertFalse(after.hold(), "已跨過 deadline+grace 且無人持鎖 ⇒ 放行，不得永久卡住賣家款項");
        assertEquals(HoldReason.NONE, after.reason());
    }

    // ==================== grace 的角色降級：correctness 不再由它承擔 ====================

    @Test
    @DisplayName("在每一個『可放行』的時點上，真正決定放不放行的都是持鎖探測，不是 grace 的長度")
    void atEveryReleasableInstant_lockProbeStillDecides() {
        // 涵蓋剛過放行時點、過很久、以及 grace=0 的極端設定
        for (int grace : new int[]{0, 1, GRACE_MINUTES, 24 * 60}) {
            for (long pastMillis : new long[]{-1L, -60_000L, -30L * 24 * 3600_000L}) {
                AfterSalePayoutHoldService svc = newService(grace);
                givenReleaseInstantAt(pastMillis, grace);

                // 有人持鎖（＝可能有在途申訴）⇒ 不論 grace 多長多短，一律擋
                when(caseMapper.tryLockCaseForPayoutRelease(anyString())).thenReturn(null);
                PayoutHold locked = svc.getPayoutHold(ORDER_NO);
                assertTrue(locked.hold(),
                        "grace=" + grace + "m、超出 " + (-pastMillis) + "ms：持鎖時必須擋");
                assertEquals(HoldReason.APPEAL_IN_FLIGHT, locked.reason());
                assertNull(locked.holdUntil());

                // 無人持鎖 ⇒ 同一個時點就放行 ⇒ 證明前一段不是恆真
                when(caseMapper.tryLockCaseForPayoutRelease(anyString())).thenAnswer(inv -> current);
                assertFalse(svc.getPayoutHold(ORDER_NO).hold(),
                        "grace=" + grace + "m：無人持鎖時必須放行（斷言要有鑑別力）");
            }
        }
    }

    // ==================== 交易內同步依賴的上界（defense-in-depth，非 correctness 依據）====================

    @Test
    @DisplayName("submitAppeal 交易內的 media RPC 必須有有限 deadline——否則 case row lock 持有時間無上界")
    void mediaCallInsideAppealTransactionIsBounded() {
        assertTrue(MediaGrpcClient.MEDIA_DEADLINE_SEC > 0,
                "MEDIA_DEADLINE_SEC 必須為正數（0 或負數 = 無界）。"
                        + "EvidenceService.finalizeAndInsert 跑在 submitAppeal 的 @Transactional 內，"
                        + "且位於『申訴資格已確立（case row 已持鎖）→ commit』之間："
                        + "無 deadline ⇒ 鎖持有時間無上界 ⇒ 撥款被無限期推遲。");
        assertTrue(MediaGrpcClient.MEDIA_DEADLINE_SEC <= 60,
                "調大之前先想清楚：這個值直接決定『撥款最多被一筆在途申訴拖延多久』的上界。");
    }
}
