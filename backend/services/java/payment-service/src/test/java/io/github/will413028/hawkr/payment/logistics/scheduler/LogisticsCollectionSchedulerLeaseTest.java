package io.github.will413028.hawkr.payment.logistics.scheduler;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionFakes;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionTxService;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.LocalDateTime;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * {@link LogisticsCollectionScheduler} 的<b>租約搶佔</b>與<b>Enablement Gate</b>。
 *
 * <h2>它守的是什麼</h2>
 * payment 今天是 {@code replicas: 2}。沒有租約時，兩個 pod 會同時撈到同一列 PENDING、
 * 各自生一個 {@code trade_no}、各自 pre-commit、<b>各自呼叫 PayUni</b>
 * ⇒ 同一筆應收被收兩次錢，而且事後對不出來（官方標 TradeNo 可重複、查詢回單筆非陣列）。
 *
 * <p>🔴 {@code uq_logcoll_trade_no} 與 {@code PENDING → SUBMITTED} 的 CAS <b>都擋不住</b>：
 * 前者因為兩個 pod 的 trade_no 本來就不同，後者因為它發生在呼叫 PayUni <b>之後</b>。
 * ⇒ 只有 claim 的租約述詞擋得住，所以本檔用「第二個 pod 在第一個 pod 還在打 PayUni 時進來」
 * 把那個窗口真的重現一次，而不是只斷言「claim 有被呼叫」。
 */
@DisplayName("線 B 收款 worker 的租約與 gate")
class LogisticsCollectionSchedulerLeaseTest {

    private static final long STORE_ID = 2685914682L;
    private static final String MER_ID = "HAWK2685914682";
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 14, 3, 0, 0);

    private static final int BATCH_SIZE = 5;
    private static final int LEASE_SECONDS = 600;
    private static final int GRACE_HOURS = 24;

    private LogisticsCollectionFakes.TestClock clock;
    private LogisticsCollectionFakes.FakeReceivables receivables;
    private LogisticsCollectionFakes.FakeAttempts attempts;
    private LogisticsCollectionFakes.FakeAccounts accounts;
    private LogisticsCollectionFakes.FakeLedger ledger;
    private LogisticsCollectionFakes.FakeAgentFee agentFee;
    private LogisticsCollectionFakes.SeqIds ids;
    private LogisticsCollectionTxService txService;
    private SimpleMeterRegistry meters;

    /** 解析得出 MerID 的 resolver（gate ② 打開）。 */
    private final LogisticsCollectionService.SellerMerchantIdResolver resolver =
            storeId -> storeId == STORE_ID ? Optional.of(MER_ID) : Optional.empty();

    @BeforeEach
    void setUp() {
        clock = new LogisticsCollectionFakes.TestClock(NOW);
        receivables = new LogisticsCollectionFakes.FakeReceivables(clock);
        attempts = new LogisticsCollectionFakes.FakeAttempts(clock);
        accounts = new LogisticsCollectionFakes.FakeAccounts();
        ledger = new LogisticsCollectionFakes.FakeLedger();
        agentFee = new LogisticsCollectionFakes.FakeAgentFee(MER_ID);
        ids = new LogisticsCollectionFakes.SeqIds(7001L);
        meters = new SimpleMeterRegistry();
        txService = new LogisticsCollectionTxService(receivables, attempts, accounts, ledger,
                new LogisticsCollectionFakes.SeqIds(9001L)::next, clock::now);
    }

    private LogisticsCollectionService service(boolean autoEnabled) {
        return new LogisticsCollectionService(receivables, attempts, txService, agentFee,
                LogisticsCollectionFakes.verifiedGrossUp(), autoEnabled, GRACE_HOURS,
                ids::next, clock::now);
    }

    private LogisticsCollectionScheduler scheduler(
            boolean autoEnabled,
            Optional<LogisticsCollectionService.SellerMerchantIdResolver> merchantIds) {
        return new LogisticsCollectionScheduler(receivables, attempts, service(autoEnabled),
                merchantIds, meters, BATCH_SIZE, LEASE_SECONDS, LEASE_SECONDS, GRACE_HOURS, 0L);
    }

    private void seedPending(long estimated, long subsidy) {
        assertTrue(service(true).createReceivable(new LogisticsCollectionService.CostIncurred(
                STORE_ID, "O20260912001", 900001L, "3003", "uber-del-1",
                estimated, subsidy, NOW)));
    }

    private double counter(String phase, String outcome) {
        var c = meters.find(LogisticsCollectionScheduler.METRIC_OUTCOME)
                .tags("phase", phase, "outcome", outcome).counter();
        return c == null ? 0.0d : c.count();
    }

    // =====================================================================
    // Enablement Gate
    // =====================================================================

    @Test
    @DisplayName("🔴 費率未查證（auto-submit 關）→ **連 claim 都不做**（不白白壓住一輪租約）")
    void gateClosedByFeePolicyDoesNotEvenClaim() {
        seedPending(1200, 200);

        scheduler(false, Optional.of(resolver)).runSubmit();

        assertEquals(0, receivables.claimCalls, "gate 關著就不該碰候選集合");
        assertEquals(0, agentFee.submitCalls);
    }

    @Test
    @DisplayName("🔴 MerID resolver 沒有 bean → 連 claim 都不做（不猜一個 MerID 送出去）")
    void gateClosedByMissingResolverDoesNotEvenClaim() {
        seedPending(1200, 200);

        scheduler(true, Optional.empty()).runSubmit();

        assertEquals(0, receivables.claimCalls);
        assertEquals(0, agentFee.submitCalls);
    }

    @Test
    @DisplayName("gauge 反映兩道 gate：都開才是 1（worker「完全沒在跑」只有它看得見）")
    void gaugeReflectsBothGates() {
        assertFalse(scheduler(false, Optional.of(resolver)).gatesOpen());
        assertFalse(scheduler(true, Optional.empty()).gatesOpen());
        assertTrue(scheduler(true, Optional.of(resolver)).gatesOpen());

        // ⚠️ 每個 gauge 各用一個全新的 registry：同名無 tag 的 meter 在同一個 registry 裡
        //    會被 Micrometer 依 id 去重，**先註冊的那個贏**——共用 registry 會讓這支測試
        //    量到第一個（gate 關著的）scheduler，而不是它宣稱要量的那個。
        //    ⚠️ 也必須抓著 scheduler 的強參考：Gauge 對量測對象是弱參考，被 GC 掉就回 NaN。
        assertEquals(0.0d, gaugeValueFor(false, Optional.of(resolver)));
        assertEquals(0.0d, gaugeValueFor(true, Optional.empty()));
        assertEquals(1.0d, gaugeValueFor(true, Optional.of(resolver)));
    }

    private double gaugeValueFor(boolean autoEnabled,
                                 Optional<LogisticsCollectionService.SellerMerchantIdResolver> merchantIds) {
        SimpleMeterRegistry fresh = new SimpleMeterRegistry();
        LogisticsCollectionScheduler held = new LogisticsCollectionScheduler(
                receivables, attempts, service(autoEnabled), merchantIds, fresh,
                BATCH_SIZE, LEASE_SECONDS, LEASE_SECONDS, GRACE_HOURS, 0L);
        double value = fresh.find(LogisticsCollectionScheduler.METRIC_GATE_OPEN).gauge().value();
        // 明確用一次 held，讓「弱參考被 GC」不可能在讀值之前發生。
        assertEquals(value == 1.0d, held.gatesOpen());
        return value;
    }

    @Test
    @DisplayName("🔴 解析不出某個賣家的 MerID → 跳過那一筆，不退回預設值")
    void unresolvedMerchantIsSkippedNotDefaulted() {
        // resolver 只認得 STORE_ID，這一筆用別的賣家。
        assertTrue(service(true).createReceivable(new LogisticsCollectionService.CostIncurred(
                999L, "O-OTHER", 900002L, "3003", "uber-del-2", 1200, 200, NOW)));

        scheduler(true, Optional.of(resolver)).runSubmit();

        assertEquals(0, agentFee.submitCalls, "🔴 猜一個 MerID 會從別人的商店扣錢，而 log 全部顯示成功");
        assertEquals(1.0d, counter("submit", "merchant_unresolved"));
    }

    // =====================================================================
    // 租約搶佔（M-2）
    // =====================================================================

    /**
     * 🔴 <b>M-2 的主偵測點</b>：把「第二個 pod 在第一個 pod 還在打 PayUni 時進來」這個窗口
     * 真的重現一次。
     *
     * <p>此刻那一列<b>還是 PENDING</b>（狀態要等 PayUni 回來才推進），所以
     * {@code trade_no} 的 UNIQUE 與 {@code PENDING → SUBMITTED} 的 CAS 都不會出手。
     * 唯一擋得住的是 claim 的租約述詞
     * （{@code submit_lease_until IS NULL OR submit_lease_until <= NOW()}）。
     *
     * <p>拿掉那個述詞 ⇒ pod B 撈到同一列 ⇒ {@code submitCalls} 變 2 ⇒ 轉紅。
     */
    @Test
    @DisplayName("🔴 第一個 pod 還在打 PayUni 時，第二個 pod 搶不到同一列（M-2）")
    void secondPodCannotClaimTheRowWhileFirstIsCallingPayUni() {
        seedPending(1200, 200);
        LogisticsCollectionScheduler podA = scheduler(true, Optional.of(resolver));
        LogisticsCollectionScheduler podB = scheduler(true, Optional.of(resolver));
        agentFee.willAccept();

        boolean[] reentered = {false};
        agentFee.onSubmit((tradeNo, gross) -> {
            if (reentered[0]) {
                return;
            }
            reentered[0] = true;
            // pod B 在 pod A 的 PayUni 呼叫「進行中」時跑一輪。
            podB.runSubmit();
        });

        podA.runSubmit();

        assertTrue(reentered[0], "pod B 沒被觸發＝這支測試什麼都沒量到");
        assertEquals(1, agentFee.submitCalls,
                "🔴 同一筆應收被送出兩次 ＝ 賣家被收兩次錢，而官方 API 事後對不出有幾筆");
        assertEquals(1, attempts.size(), "🔴 也不該留下第二筆 attempt（第二個 trade_no）");
    }

    /**
     * 第二條路徑：一筆<b>狀態沒有改變</b>的列（{@code target_net = 0}）在同一個租約內
     * 不得被重複處理。
     *
     * <p>它與上一支互補——上一支測的是「呼叫進行中」的窗口，這一支測的是「處理完但狀態沒動」
     * 的情形，而後者正是租約（相對於「改狀態就自然離開候選集合」）唯一能守的東西。
     */
    @Test
    @DisplayName("🔴 租約未到期 → 同一輪租約內不重複處理同一列（M-2）")
    void leaseHeldPreventsReprocessingWithinTheSameLease() {
        seedPending(300, 300); // accrual = 0 ⇒ target_net = 0 ⇒ 處理後仍是 PENDING
        LogisticsCollectionScheduler s = scheduler(true, Optional.of(resolver));

        s.runSubmit();
        s.runSubmit();
        s.runSubmit();

        assertEquals(LogisticsReceivableStatus.PENDING.value(), receivables.statusOf(7001L));
        assertEquals(1.0d, counter("submit", "skipped_zero_target"),
                "🔴 三個 tick 只該處理到一次——租約壓著它");
    }

    /**
     * 🔴 租約是<b>租約</b>不是永久旗標（10 檔 §29）：到期後那一列必須自己回到候選集合。
     *
     * <p>反面很具體：pod 在副作用中途崩潰時，永久旗標會讓該筆<b>永久卡死且無告警</b>。
     */
    @Test
    @DisplayName("🔴 租約到期 → 該列自動回到候選集合（證明它是 lease 不是永久旗標）")
    void expiredLeaseReturnsTheRowToCandidates() {
        seedPending(300, 300);
        LogisticsCollectionScheduler s = scheduler(true, Optional.of(resolver));

        s.runSubmit();
        assertEquals(1.0d, counter("submit", "skipped_zero_target"));

        clock.advanceSeconds(LEASE_SECONDS + 1);
        s.runSubmit();

        assertEquals(2.0d, counter("submit", "skipped_zero_target"),
                "租約到期後必須重新可搶——否則崩掉的 pod 會讓這一列永久卡死");
    }

    @Test
    @DisplayName("送出途中拋例外 → 不歸還租約（退避＝一個租約長度，不對剛打爆的接口連續重打）")
    void exceptionDoesNotReleaseTheLease() {
        seedPending(1200, 200);
        LogisticsCollectionScheduler s = scheduler(true, Optional.of(resolver));
        agentFee.onSubmit((tradeNo, gross) -> {
            throw new IllegalStateException("PayUni 掛了");
        });
        agentFee.willAccept();

        s.runSubmit();
        assertEquals(1.0d, counter("submit", "error"));

        s.runSubmit();
        assertEquals(1, agentFee.submitCalls, "🔴 立刻重打＝把退避變成 0 秒");

        clock.advanceSeconds(LEASE_SECONDS + 1);
        s.runSubmit();
        assertEquals(2, agentFee.submitCalls, "退避一個租約之後才重試");
    }

    // =====================================================================
    // 查詢 worker 的租約
    // =====================================================================

    @Test
    @DisplayName("🔴 查詢也走租約：寬限期內查不到的那一筆，同一輪租約內不重複打 PayUni")
    void queryTickAlsoHoldsALease() {
        seedPending(1200, 200);
        agentFee.willAccept();
        LogisticsCollectionScheduler s = scheduler(true, Optional.of(resolver));
        s.runSubmit();
        assertEquals(1, attempts.size());

        agentFee.willQuery(LogisticsCollectionFakes.notYetFunded(1010L));
        s.runQuery();
        s.runQuery();

        assertEquals(1, agentFee.queryCalls, "🔴 租約沒守住 ⇒ 每個 tick 都對 PayUni 多打一次");
        assertEquals(1.0d, counter("query", "pending_fund"));

        clock.advanceSeconds(LEASE_SECONDS + 1);
        s.runQuery();
        assertEquals(2, agentFee.queryCalls);
    }

    @Test
    @DisplayName("gate 關著 → 查詢 tick 也不 claim")
    void queryTickRespectsTheGate() {
        scheduler(false, Optional.of(resolver)).runQuery();
        assertEquals(0, attempts.claimCalls);
    }

    // =====================================================================
    // 租約長度的上下界（開機即拋）
    // =====================================================================

    /**
     * 🔴 §29 的不變式做成<b>開機即拋</b>，而不是註解裡的承諾。
     *
     * <p>它擋的是最容易發生的一種漂移：為了吞吐量把 {@code batch-size} 調大而租約沒動。
     * 那一刻起，批次跑到後面幾筆時租約已經過期，另一個 pod 會<b>合法</b>搶走同一列
     * 並再打一次 PayUni——而那個失效<b>沒有任何錯誤訊息</b>。
     */
    @Test
    @DisplayName("🔴 batchSize × 單筆最壞耗時 ≥ 租約 → 開機就拋（調大 batch 忘了調租約）")
    void leaseLowerBoundIsEnforcedAtStartup() {
        // 5 × 100 = 500 < 600 ✅
        LogisticsCollectionScheduler.validateLeaseBounds(5, 600, 600, 24);

        // 6 × 100 = 600，不是「< 600」⇒ 必須拋
        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> LogisticsCollectionScheduler.validateLeaseBounds(6, 600, 600, 24));
        assertTrue(ex.getMessage().contains("submit-lease-seconds"), ex.getMessage());

        assertThrows(IllegalStateException.class,
                () -> LogisticsCollectionScheduler.validateLeaseBounds(50, 600, 600, 24));
    }

    @Test
    @DisplayName("🔴 租約超過上界 → 開機就拋（崩掉的 pod 會把該列擋住整段租約）")
    void leaseUpperBoundIsEnforcedAtStartup() {
        assertThrows(IllegalStateException.class,
                () -> LogisticsCollectionScheduler.validateLeaseBounds(
                        1, LogisticsCollectionScheduler.MAX_LEASE_SECONDS + 1, 600, 24));
    }

    /**
     * 查詢側的第二個（更緊的）上界：候選窗口本身不過期，但
     * {@code expected_fund_date + grace} 是一個更緊的判定時點——租約接近寬限期時，
     * 一個崩掉的持有者會實質延後 UNCONFIRMED 的判定。
     */
    @Test
    @DisplayName("🔴 查詢租約 ≥ 寬限期 → 開機就拋（崩掉的持有者會延後 UNCONFIRMED 判定）")
    void queryLeaseMustBeMuchShorterThanTheGraceWindow() {
        // grace = 1 小時 = 3600 秒；查詢租約 3600 ⇒ 相等，必須拋
        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> LogisticsCollectionScheduler.validateLeaseBounds(1, 600, 3600, 1));
        assertTrue(ex.getMessage().contains("query-lease-seconds"), ex.getMessage());

        // 預設值：600s vs 24h = 86400s ⇒ 0.7%，遠小於 ✅
        LogisticsCollectionScheduler.validateLeaseBounds(BATCH_SIZE, LEASE_SECONDS, LEASE_SECONDS, GRACE_HOURS);
    }

    @Test
    @DisplayName("batchSize < 1 → 開機就拋")
    void batchSizeMustBePositive() {
        assertThrows(IllegalStateException.class,
                () -> LogisticsCollectionScheduler.validateLeaseBounds(0, 600, 600, 24));
    }

    @Test
    @DisplayName("bounded batch：候選多於 batch-size 時，一輪只處理 batch-size 筆")
    void batchIsBounded() {
        for (int i = 0; i < BATCH_SIZE + 3; i++) {
            assertTrue(service(true).createReceivable(new LogisticsCollectionService.CostIncurred(
                    STORE_ID, "O-" + i, 900100L + i, "3003", "uber-" + i,
                    300, 300, NOW)));
        }
        scheduler(true, Optional.of(resolver)).runSubmit();

        assertEquals((double) BATCH_SIZE, counter("submit", "skipped_zero_target"));
    }
}
