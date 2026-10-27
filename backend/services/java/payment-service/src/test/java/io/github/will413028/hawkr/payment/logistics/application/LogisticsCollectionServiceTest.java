package io.github.will413028.hawkr.payment.logistics.application;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.LedgerTxnType;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerEntry;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

/**
 * 線 B 收款編排（{@link LogisticsCollectionService} ＋ {@link LogisticsCollectionTxService}）。
 *
 * <p>🔴 <b>期望值一律硬寫字面值</b>，不從被測物或它引用的常數反推：
 * gross 寫 {@code 1010} 而不是 {@code grossUp.solve(1000).grossAmount()}，
 * allowed-from 的斷言寫「應該拋」而不是「等於 ReceivableStatusRules 說的」。
 * 期望值若跟著被測物一起變，這支測試就驗不到任何東西。
 *
 * <p>費率取 <b>1%／DOWN</b>（{@link LogisticsCollectionFakes#verifiedGrossUp}），
 * 於是 {@code targetNet=1000} 的算式全程可以手算：
 * <pre>
 *   solve(1000) → 最小 G 使 G − floor(G/100) ≥ 1000
 *                 G=1010：fee=10，net=1000 ≥ 1000 ✅
 *                 G=1009：fee=10，net= 999 &lt; 1000 ❌  ⇒ gross = 1010
 * </pre>
 */
@DisplayName("線 B 收款編排")
class LogisticsCollectionServiceTest {

    private static final long STORE_ID = 2685914682L;
    private static final String MER_ID = "HAWK2685914682";
    private static final long SHIPMENT_ID = 900001L;
    private static final String ORDER_NO = "O20260912001";

    /** 2026-09-14 是週一。跨週末的斷言另外在 {@link NextBusinessDay} 裡做。 */
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 14, 3, 0, 0);

    private LogisticsCollectionFakes.TestClock clock;
    private LogisticsCollectionFakes.FakeReceivables receivables;
    private LogisticsCollectionFakes.FakeAttempts attempts;
    private LogisticsCollectionFakes.FakeAccounts accounts;
    private LogisticsCollectionFakes.FakeLedger ledger;
    private LogisticsCollectionFakes.FakeAgentFee agentFee;
    private LogisticsCollectionFakes.SeqIds ids;
    private LogisticsCollectionFakes.SeqIds ledgerIds;
    private LogisticsCollectionTxService txService;

    @BeforeEach
    void setUp() {
        clock = new LogisticsCollectionFakes.TestClock(NOW);
        receivables = new LogisticsCollectionFakes.FakeReceivables(clock);
        attempts = new LogisticsCollectionFakes.FakeAttempts(clock);
        accounts = new LogisticsCollectionFakes.FakeAccounts();
        ledger = new LogisticsCollectionFakes.FakeLedger();
        agentFee = new LogisticsCollectionFakes.FakeAgentFee(MER_ID);
        // 🔴 兩個號段刻意分開：帳本 txn id 若與編排共用，
        //    「應收 id / attempt id 是幾號」就會隨「這一筆有沒有認列帳本」而漂移。
        ids = new LogisticsCollectionFakes.SeqIds(7001L);
        ledgerIds = new LogisticsCollectionFakes.SeqIds(9001L);
        txService = new LogisticsCollectionTxService(
                receivables, attempts, accounts, ledger, ledgerIds::next, clock::now);
    }

    private LogisticsCollectionService service(boolean autoEnabled) {
        return new LogisticsCollectionService(receivables, attempts, txService, agentFee,
                LogisticsCollectionFakes.verifiedGrossUp(), autoEnabled, 24, ids::next, clock::now);
    }

    private LogisticsCollectionService.CostIncurred cost(long shipmentId, long estimated, long subsidy) {
        return new LogisticsCollectionService.CostIncurred(
                STORE_ID, ORDER_NO, shipmentId, "3003", "uber-del-" + shipmentId,
                estimated, subsidy, NOW);
    }

    /** 建一筆 PENDING 應收並回傳呼叫端會拿到的那份快照（＝worker claim 出來的形狀）。 */
    private LogisticsReceivableEntity seedPending(long estimated, long subsidy) {
        assertTrue(service(true).createReceivable(cost(SHIPMENT_ID, estimated, subsidy)));
        long id = receivables.raw(firstReceivableId()).getId();
        return receivables.findById(id).orElseThrow();
    }

    private long firstReceivableId() {
        return 7001L; // SeqIds 起點；應收是第一個被配號的實體
    }

    // =====================================================================
    // 1. 應收成立
    // =====================================================================

    @Nested
    @DisplayName("應收成立")
    class CreateReceivable {

        @Test
        @DisplayName("送達事件重放（同 idempotency_key）→ 回 false，且帳本與餘額都不再動一次")
        void replayIsIdempotent() {
            LogisticsCollectionService svc = service(true);
            assertTrue(svc.createReceivable(cost(SHIPMENT_ID, 1200, 200)));
            assertEquals(1000L, accounts.receivableBalance(STORE_ID));
            assertEquals(1, ledger.posted().size());

            // 同一個 provider ＋ 同一個 provider_delivery_ref ⇒ 同一把冪等鍵。
            assertFalse(svc.createReceivable(cost(SHIPMENT_ID, 1200, 200)));

            assertEquals(1, receivables.size(), "重放不得建出第二筆應收");
            assertEquals(1000L, accounts.receivableBalance(STORE_ID), "重放不得再加一次餘額");
            assertEquals(1, ledger.posted().size(), "重放不得再認列一次收入");
        }

        @Test
        @DisplayName("正逆向是兩把冪等鍵 → 同一張訂單可以有兩筆應收")
        void forwardAndReturnAreSeparateKeys() {
            LogisticsCollectionService svc = service(true);
            assertTrue(svc.createReceivable(cost(SHIPMENT_ID, 1200, 200)));
            // 🔴 逆向是 provider 端另一條 fulfillment leg ⇒ 另一個 provider_delivery_ref。
            //    shipment_id 相同而鍵不同，正是新錨要保住的那筆「真實成本」（否則漏收）。
            assertTrue(svc.createReceivable(new LogisticsCollectionService.CostIncurred(
                    STORE_ID, ORDER_NO, SHIPMENT_ID, "3003", "uber-del-" + SHIPMENT_ID + "-return",
                    800, 0, NOW)));
            assertEquals(2, receivables.size());
        }

        @Test
        @DisplayName("🔴 carried 進 target_net、但**不進**帳本：結轉再認列一次就是重複認列收入")
        void carriedGoesToTargetNetNotToLedger() {
            LogisticsCollectionService svc = service(true);
            // 第一筆：accrual = 1200 − 200 = 1000，carried = 0
            assertTrue(svc.createReceivable(cost(SHIPMENT_ID, 1200, 200)));
            // 第二筆：accrual = 500，carried = 1000（第一筆留下的餘額）
            assertTrue(svc.createReceivable(cost(SHIPMENT_ID + 1, 500, 0)));

            LogisticsReceivableEntity second = receivables.findById(7002L).orElseThrow();
            assertEquals(1500L, second.getTargetNet(), "target_net = 本期認列 500 + carried 1000");
            assertEquals(1000L, second.getCarriedResidual());

            List<LedgerTxn> accruals =
                    ledger.postedOfType(LedgerTxnType.LOGISTICS_RECEIVABLE_ACCRUAL.code());
            assertEquals(2, accruals.size());
            assertEquals(500L, accruals.get(1).entries().get(0).amount(),
                    "🔴 帳本只認列本期新增那一塊（500），不是 target_net（1500）");
            assertEquals(1500L, accounts.receivableBalance(STORE_ID));
        }

        @Test
        @DisplayName("charge_basis_frozen_at 取事件的業務時間，不是 now()")
        void chargeBasisFrozenAtComesFromTheEvent() {
            LocalDateTime occurred = LocalDateTime.of(2026, 9, 10, 1, 2, 3);
            service(true).createReceivable(new LogisticsCollectionService.CostIncurred(
                    STORE_ID, ORDER_NO, SHIPMENT_ID, "3003", "uber-del-1", 1200, 200, occurred));
            assertEquals(occurred, receivables.findById(7001L).orElseThrow().getChargeBasisFrozenAt());
        }

        @Test
        @DisplayName("補貼超過成本 → fail-loud，不靜默夾到 0，且不留下任何半套資料")
        void subsidyOverCostFailsLoud() {
            assertThrows(IllegalArgumentException.class,
                    () -> service(true).createReceivable(cost(SHIPMENT_ID, 100, 300)));
            assertEquals(0, receivables.size());
            assertEquals(0, ledger.posted().size());
        }

        @Test
        @DisplayName("accrual = 0（成本恰等於補貼）→ 建應收但不認列帳本（金額 0 的分錄沒有語意）")
        void zeroAccrualCreatesReceivableWithoutLedger() {
            assertTrue(service(true).createReceivable(cost(SHIPMENT_ID, 300, 300)));
            assertEquals(1, receivables.size());
            assertEquals(0, ledger.posted().size());
            assertEquals(0L, accounts.receivableBalance(STORE_ID));
        }
    }

    // =====================================================================
    // 2. 送出
    // =====================================================================

    @Nested
    @DisplayName("送出")
    class SubmitOne {

        /**
         * 🔴 <b>M-1 的偵測點</b>（裁決第 7 條）：gross 在送出 PayUni 之前就與 attempt 一起持久化。
         *
         * <p>量的時點是 <b>PayUni 被呼叫的那一瞬間</b>（{@code onSubmit} 的 inspector 讀當下的
         * DB 狀態），不是事後補量。把持久化移到送出之後 ⇒ inspector 讀不到那一列 ⇒ 轉紅。
         *
         * <p>為什麼這一格非守不可：{@code agent_fee_query} 的查詢鍵是 {@code (MerID, TradeNo, Amt)}
         * <b>三元組</b>，而 Amt 是 gross。「送出成功、回應遺失」時若 gross 沒先落地，
         * 後續 query 帶不出正確 Amt ⇒ 一筆已經在對方系統裡的收款，在我方永遠停在不確定狀態。
         */
        @Test
        @DisplayName("🔴 呼叫 PayUni 的那一瞬間，attempt 與 gross 都已經落地（M-1）")
        void grossIsPersistedBeforeThePayUniCall() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willAccept();
            boolean[] seen = {false};
            agentFee.onSubmit((tradeNo, gross) -> {
                LogisticsCollectionAttemptEntity row = attempts.findByTradeNo(tradeNo).orElse(null);
                assertNotNull(row, "🔴 呼叫 PayUni 之前 attempt 就必須已經落地");
                assertEquals(1010L, row.getGrossAmount(),
                        "🔴 落地的必須是實際要送出的 gross，不是 target_net 佔位值");
                assertEquals(1010L, gross, "送出的 Amt 與落地的 gross 必須是同一個數");
                seen[0] = true;
            });

            assertEquals(LogisticsCollectionService.SubmitResult.SUBMITTED,
                    service(true).submitOne(r, MER_ID));

            assertTrue(seen[0], "inspector 沒被呼叫＝這支測試什麼都沒量到");
            assertEquals(1, agentFee.submitCalls);
            assertEquals(LogisticsReceivableStatus.SUBMITTED.value(), receivables.statusOf(r.getId()));
        }

        @Test
        @DisplayName("trade_no 由我方生成（uq_logcoll_trade_no 是唯一的去重保證），長度不超過 20")
        void tradeNoIsOursAndWithinPayUniLimit() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willAccept();
            service(true).submitOne(r, MER_ID);

            String tradeNo = agentFee.submittedTradeNos.get(0);
            assertEquals("LF7002", tradeNo);
            assertTrue(tradeNo.length() <= 20);
        }

        /**
         * 🔴 未取得回應 ≠ 失敗。PayUni 可能已經受理了 ⇒ 不重送、推進 SUBMITTED，
         * 讓查詢 worker 拿這個 trade_no 去問對方到底有沒有受理。
         */
        @Test
        @DisplayName("🔴 逾時／連線失敗 → 推進 SUBMITTED（不重送），且 gross 是真值不是佔位值")
        void noResponseGoesToSubmittedWithRealGross() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willNotRespond();

            assertEquals(LogisticsCollectionService.SubmitResult.UNKNOWN,
                    service(true).submitOne(r, MER_ID));

            assertEquals(LogisticsReceivableStatus.SUBMITTED.value(), receivables.statusOf(r.getId()));
            LogisticsCollectionAttemptEntity a = attempts.raw(7002L);
            assertEquals(1010L, a.getGrossAmount(),
                    "🔴 gross 在 HTTP 呼叫之前就算好並落地，逾時這一格照樣拿得到查詢鍵");
            assertNotNull(a.getExpectedFundDate(), "SUBMITTED 必須有期望撥款日，否則 UNCONFIRMED 判不出來");
            assertEquals(1, agentFee.submitCalls, "🔴 不得自動重送");
        }

        @Test
        @DisplayName("明確拒絕 → FAILED，且 expected_fund_date 留空（沒有受理就沒有期望撥款日）")
        void rejectedGoesToFailedWithoutExpectedFundDate() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willReject("FAIL", "餘額不足");

            assertEquals(LogisticsCollectionService.SubmitResult.REJECTED,
                    service(true).submitOne(r, MER_ID));

            assertEquals(LogisticsReceivableStatus.FAILED.value(), receivables.statusOf(r.getId()));
            LogisticsCollectionAttemptEntity a = attempts.raw(7002L);
            assertEquals("FAIL", a.getSubmitStatus());
            assertNull(a.getExpectedFundDate(),
                    "🔴 填了期望撥款日，查詢 worker 會把一筆根本沒受理的收款當成在等撥款");
        }

        @Test
        @DisplayName("🔴 target_net < 1 → 不送出（PayUni Amt 值域 1~9999999），連 attempt 都不建")
        void zeroTargetIsSkippedWithoutAnyAttempt() {
            LogisticsReceivableEntity r = seedPending(300, 300); // accrual = 0 ⇒ target_net = 0

            assertEquals(LogisticsCollectionService.SubmitResult.SKIPPED_ZERO_TARGET,
                    service(true).submitOne(r, MER_ID));

            assertEquals(0, agentFee.submitCalls, "不得送出一筆必失敗的請求（那種失敗看起來像通道問題）");
            assertEquals(0, attempts.size());
            assertEquals(LogisticsReceivableStatus.PENDING.value(), receivables.statusOf(r.getId()));
        }

        @Test
        @DisplayName("🔴 自動收款未啟用（EV-P6）→ 什麼都不做，連 attempt 都不建")
        void disabledDoesNothing() {
            LogisticsReceivableEntity r = seedPending(1200, 200);

            assertEquals(LogisticsCollectionService.SubmitResult.DISABLED,
                    service(false).submitOne(r, MER_ID));

            assertEquals(0, agentFee.submitCalls);
            assertEquals(0, attempts.size());
        }

        @Test
        @DisplayName("非 PENDING → fail-loud（呼叫順序或狀態機出問題，不是可以忽略的情形）")
        void nonPendingThrows() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            receivables.forceStatus(r.getId(), LogisticsReceivableStatus.SUBMITTED);
            LogisticsReceivableEntity stale = receivables.findById(r.getId()).orElseThrow();

            assertThrows(IllegalStateException.class, () -> service(true).submitOne(stale, MER_ID));
            assertEquals(0, agentFee.submitCalls);
        }

        /**
         * 🔴 <b>M-3 的偵測點</b>：CAS 的 allowed-from。
         *
         * <p>情境是真實的競態：worker 撈到一份 PENDING 的<b>快照</b>，在它打 PayUni 的期間
         * 另一個執行者把那一列推到了別處。收尾的 {@code PENDING → SUBMITTED} 於是 rows=0。
         *
         * <p>🔴 rows=0 <b>不得</b>讀成「別人先做過了」就靜默放行——它代表這筆已經不在
         * allowed-from 裡，而症狀會是「應收永遠停在某一格，而所有 log 都顯示處理成功」。
         *
         * <p>兩種變異都由本測試接住：
         * ① {@code ReceivableStatusRules.canTransition} 改成恆真（＝allowed-from 換成無條件 UPDATE）；
         * ② {@code LogisticsCollectionTxService.requireTransition} 的 fail-loud 拿掉。
         */
        @Test
        @DisplayName("🔴 送出途中狀態被別人改走 → CAS rows=0 必須 fail-loud，不得靜默放行（M-3）")
        void concurrentStatusChangeDuringSubmitFailsLoud() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willAccept();
            // SETTLED 不在 SUBMITTED 的 allowed-from（{PENDING, FAILED}）內。
            agentFee.onSubmit((tradeNo, gross) ->
                    receivables.forceStatus(r.getId(), LogisticsReceivableStatus.SETTLED));

            IllegalStateException ex = assertThrows(IllegalStateException.class,
                    () -> service(true).submitOne(r, MER_ID));
            assertTrue(ex.getMessage().contains("SUBMITTED"), "錯誤訊息要指得出是哪一條邊：" + ex.getMessage());
            assertEquals(LogisticsReceivableStatus.SETTLED.value(), receivables.statusOf(r.getId()),
                    "🔴 不得覆蓋別人剛寫進去的狀態");
        }

        @Test
        @DisplayName("MerID 帶錯 → 通道替身直接拒絕（不讓「解析錯 MerID」靜默通過）")
        void wrongMerIdIsRejectedByChannel() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willAccept();
            assertThrows(IllegalStateException.class,
                    () -> service(true).submitOne(r, "HAWK0000000000"));
        }
    }

    // =====================================================================
    // 3. 查詢與入帳
    // =====================================================================

    @Nested
    @DisplayName("查詢與入帳")
    class QueryOne {

        private LogisticsReceivableEntity submitted() {
            LogisticsReceivableEntity r = seedPending(1200, 200);
            agentFee.willAccept();
            service(true).submitOne(r, MER_ID);
            return receivables.findById(r.getId()).orElseThrow();
        }

        private LogisticsCollectionAttemptEntity attempt() {
            return LogisticsCollectionFakes.FakeAttempts.copy(attempts.raw(7002L));
        }

        @Test
        @DisplayName("🔴 查詢帶的是 gross（查詢鍵三元組的第三格），不是 target_net")
        void queryUsesGrossAsKey() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.notYetFunded(1010L));

            service(true).queryOne(r, attempt(), MER_ID);

            assertEquals(List.of(1010L), agentFee.queriedAmt,
                    "帶 target_net(1000) 去查會查無此筆，而那看起來像「沒受理」");
        }

        @Test
        @DisplayName("🔴 帳本記 net_proceeds 不是 gross：手續費平台從來沒收到")
        void settledPostsNetProceedsNotGross() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-15", 10L, 1010L));

            assertEquals(LogisticsCollectionService.QueryResult.SETTLED,
                    service(true).queryOne(r, attempt(), MER_ID));

            List<LedgerTxn> settledTxns =
                    ledger.postedOfType(LedgerTxnType.LOGISTICS_COLLECTION_SETTLED.code());
            assertEquals(1, settledTxns.size());
            LedgerEntry debit = settledTxns.get(0).entries().get(0);
            assertEquals(LedgerAccountCode.PLATFORM_OPERATING_CASH.code(), debit.accountCode());
            assertEquals(1000L, debit.amount(), "🔴 記的是 1010 − 10 = 1000，不是 gross 1010");

            assertEquals(LogisticsReceivableStatus.SETTLED.value(), receivables.statusOf(r.getId()));
            assertEquals(0L, accounts.receivableBalance(STORE_ID), "應收 1000 收足 ⇒ 餘額歸零");
            assertEquals(0L, accounts.creditBalance(STORE_ID));
            LogisticsCollectionAttemptEntity a = attempts.raw(7002L);
            assertEquals(1000L, a.getNetProceeds());
            assertEquals(0L, a.getResidual());
            assertEquals(LocalDate.of(2026, 9, 15), a.getFundDate());
        }

        @Test
        @DisplayName("🔴 收不足（residual > 0）→ 餘額留著缺口，且 SETTLED 不是終態")
        void underCollectionLeavesResidualOnTheBalance() {
            LogisticsReceivableEntity r = submitted();
            // 實際手續費 20 ＞ 預估 10 ⇒ net = 990，residual = 1000 − 990 = 10
            agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-15", 20L, 1010L));

            service(true).queryOne(r, attempt(), MER_ID);

            assertEquals(10L, attempts.raw(7002L).getResidual());
            assertEquals(10L, accounts.receivableBalance(STORE_ID), "未收足的 10 元留在應收餘額，下期結轉");
            assertEquals(0L, accounts.creditBalance(STORE_ID));
            assertEquals(0, ledger.postedOfType(LedgerTxnType.LOGISTICS_CREDIT_ISSUE.code()).size());
        }

        @Test
        @DisplayName("🔴 多收（residual < 0）→ 產生貸記，**不得**變成 platform margin")
        void overCollectionIssuesCredit() {
            LogisticsReceivableEntity r = submitted();
            // 實際手續費 5 ＜ 預估 10 ⇒ net = 1005，residual = 1000 − 1005 = −5
            agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-15", 5L, 1010L));

            service(true).queryOne(r, attempt(), MER_ID);

            assertEquals(-5L, attempts.raw(7002L).getResidual());
            assertEquals(5L, accounts.creditBalance(STORE_ID), "🔴 多收的 5 元要還給賣家");
            assertEquals(-5L, accounts.receivableBalance(STORE_ID));
            List<LedgerTxn> credits = ledger.postedOfType(LedgerTxnType.LOGISTICS_CREDIT_ISSUE.code());
            assertEquals(1, credits.size());
            assertEquals(5L, credits.get(0).entries().get(0).amount());
        }

        @Test
        @DisplayName("🔴 已撥款但沒有 Fee → 一列都不寫（不得用預估值冒充實際手續費）")
        void settledWithoutFeeWritesNothing() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.settledWithoutFee("2026-09-15", 1010L));

            assertEquals(LogisticsCollectionService.QueryResult.SETTLED_WITHOUT_FEE,
                    service(true).queryOne(r, attempt(), MER_ID));

            assertEquals(LogisticsReceivableStatus.SUBMITTED.value(), receivables.statusOf(r.getId()));
            assertNull(attempts.raw(7002L).getFundDate());
            assertEquals(1, ledger.posted().size(), "只有應收成立那一筆，不得多出入帳分錄");
        }

        @Test
        @DisplayName("寬限期內尚未撥款 → 什麼都不做，下一輪再查")
        void withinGraceDoesNothing() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.notYetFunded(1010L));

            assertEquals(LogisticsCollectionService.QueryResult.PENDING_FUND,
                    service(true).queryOne(r, attempt(), MER_ID));
            assertEquals(LogisticsReceivableStatus.SUBMITTED.value(), receivables.statusOf(r.getId()));
        }

        /**
         * 🔴 {@code found=false}（PayUni 沒有這筆）與「尚未撥款」是<b>兩件事</b>，
         * 但在寬限期內兩者的<b>處置相同</b>：都不推狀態機。
         * 把 found=false 當成失敗而重送，就是「把一次不確定變成一次確定的重複扣款」。
         */
        @Test
        @DisplayName("🔴 寬限期內查無此筆 → 一樣什麼都不做，不得當成失敗重送")
        void notFoundWithinGraceIsNotAFailure() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.notFound());

            assertEquals(LogisticsCollectionService.QueryResult.PENDING_FUND,
                    service(true).queryOne(r, attempt(), MER_ID));
            assertEquals(LogisticsReceivableStatus.SUBMITTED.value(), receivables.statusOf(r.getId()));
            assertEquals(1, agentFee.submitCalls, "🔴 不得自動重送");
        }

        @Test
        @DisplayName("🔴 超過期望撥款日＋寬限仍查不到 → UNCONFIRMED，且**不自動重送**")
        void pastGraceGoesToUnconfirmed() {
            LogisticsReceivableEntity r = submitted();
            // 送出日 2026-09-14（週一）⇒ expected = 09-15；寬限 24h ⇒ 09-16 00:00 之後才過期。
            clock.advanceSeconds(3L * 24 * 3600);
            agentFee.willQuery(LogisticsCollectionFakes.notYetFunded(1010L));

            assertEquals(LogisticsCollectionService.QueryResult.UNCONFIRMED,
                    service(true).queryOne(r, attempt(), MER_ID));

            assertEquals(LogisticsReceivableStatus.UNCONFIRMED.value(), receivables.statusOf(r.getId()));
            assertEquals(1, agentFee.submitCalls, "🔴 UNCONFIRMED 一律進人工，不得自動重送");
        }

        /**
         * {@code SETTLED} 的 allowed-from 逐字含 {@code UNCONFIRMED}——那是 UNCONFIRMED
         * 三條出邊裡<b>最可能</b>的一條。不走它的話，一筆「其實收到了」只能靠人工發現。
         */
        @Test
        @DisplayName("UNCONFIRMED 之後其實撥款了 → 自動收斂回 SETTLED（allowed-from 含這條邊）")
        void unconfirmedCanStillSettle() {
            LogisticsReceivableEntity r = submitted();
            clock.advanceSeconds(3L * 24 * 3600);
            agentFee.willQuery(LogisticsCollectionFakes.notYetFunded(1010L));
            service(true).queryOne(r, attempt(), MER_ID);
            assertEquals(LogisticsReceivableStatus.UNCONFIRMED.value(), receivables.statusOf(r.getId()));

            LogisticsReceivableEntity nowUnconfirmed = receivables.findById(r.getId()).orElseThrow();
            agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-16", 10L, 1010L));

            assertEquals(LogisticsCollectionService.QueryResult.SETTLED,
                    service(true).queryOne(nowUnconfirmed, attempt(), MER_ID));
            assertEquals(LogisticsReceivableStatus.SETTLED.value(), receivables.statusOf(r.getId()));
        }

        @Test
        @DisplayName("應收已核銷／已結清 → 這筆 attempt 不再追，且不打 PayUni")
        void terminalReceivableIsSkipped() {
            LogisticsReceivableEntity r = submitted();
            receivables.forceStatus(r.getId(), LogisticsReceivableStatus.WRITTEN_OFF);
            LogisticsReceivableEntity terminal = receivables.findById(r.getId()).orElseThrow();

            assertEquals(LogisticsCollectionService.QueryResult.SKIPPED_NOT_QUERYABLE,
                    service(true).queryOne(terminal, attempt(), MER_ID));
            assertEquals(0, agentFee.queryCalls);
        }

        /**
         * 🔴 {@code recordQueryResult} 回 0 列 ＝ 另一個持有者已經寫過 fund_date。
         * 放行會讓帳本與餘額被記第二次——帳本的 idempotency_key 擋得住重複 txn，
         * <b>擋不住 {@code addReceivable} 這種 delta 累加</b>。
         */
        @Test
        @DisplayName("🔴 重複入帳 → fail-loud，且守衛在所有副作用之前（沒有半套資料）")
        void duplicateSettlementFailsLoudBeforeAnySideEffect() {
            LogisticsReceivableEntity r = submitted();
            agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-15", 10L, 1010L));
            service(true).queryOne(r, attempt(), MER_ID);
            long balanceAfterFirst = accounts.receivableBalance(STORE_ID);
            int ledgerAfterFirst = ledger.posted().size();

            // 第二個持有者拿著同一份快照再入帳一次。
            LogisticsReceivableEntity stale = LogisticsCollectionFakes.FakeReceivables.copy(r);
            stale.setStatus(LogisticsReceivableStatus.SUBMITTED.value());
            assertThrows(IllegalStateException.class,
                    () -> service(true).queryOne(stale, attempt(), MER_ID));

            assertEquals(balanceAfterFirst, accounts.receivableBalance(STORE_ID));
            assertEquals(ledgerAfterFirst, ledger.posted().size());
        }
    }

    // =====================================================================
    // 4. 純判定
    // =====================================================================

    @Nested
    @DisplayName("expected_fund_date")
    class NextBusinessDay {

        @Test
        @DisplayName("平日 → 隔天")
        void weekdayGoesToNextDay() {
            assertEquals(LocalDate.of(2026, 9, 15),
                    LogisticsCollectionService.nextBusinessDay(LocalDateTime.of(2026, 9, 14, 3, 0)));
        }

        @Test
        @DisplayName("🔴 週五 → 跳過週末落到週一（PayUni 次日撥款不在週末執行）")
        void fridaySkipsTheWeekend() {
            // 2026-09-18 是週五。
            assertEquals(LocalDate.of(2026, 9, 21),
                    LogisticsCollectionService.nextBusinessDay(LocalDateTime.of(2026, 9, 18, 23, 0)));
        }

        @Test
        @DisplayName("週六 → 週一")
        void saturdayGoesToMonday() {
            assertEquals(LocalDate.of(2026, 9, 21),
                    LogisticsCollectionService.nextBusinessDay(LocalDateTime.of(2026, 9, 19, 3, 0)));
        }
    }
}
