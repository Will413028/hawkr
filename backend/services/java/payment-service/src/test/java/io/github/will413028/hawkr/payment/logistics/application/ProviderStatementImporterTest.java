package io.github.will413028.hawkr.payment.logistics.application;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.ReconcileOutcome;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.ImportReport;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.LineImportStatus;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.ReconcileReport;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.ReconciliationPolicy;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.StatementLine;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDateTime;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

/**
 * W4-5 —— statement abstraction ＋ manual import path ＋ reconciliation engine。
 *
 * <p>🔴 自動抓取那一半<b>刻意沒有做</b>（EV-U1：Uber 是否提供 statement／invoice API 完全沒有證據），
 * 旗標逐字 {@code PROVIDER AUTOMATIC STATEMENT ENABLEMENT = DISABLED}。本檔把「它真的沒有做」
 * 也測出來 —— 否則「停用」只是註解裡的一句話。
 */
@DisplayName("W4-5 provider 帳單匯入與對帳引擎")
class ProviderStatementImporterTest {

    private static final long STORE_ID = 5_000_002L;
    private static final String MER_ID = "HAWK5000002";
    private static final String PROVIDER = "3003";
    private static final String STATEMENT_REF = "UBER-STMT-202609-B";
    private static final String PERIOD = "2026-09";
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 12, 3, 0);
    private static final long ACTOR = 2_600_000_002L;

    private LogisticsCollectionFakes.TestClock clock;
    private LogisticsCollectionFakes.SeqIds ids;
    private LogisticsCollectionFakes.FakeReceivables receivables;
    private LogisticsCollectionFakes.FakeAttempts attempts;
    private LogisticsCollectionFakes.FakeAccounts accounts;
    private LogisticsCollectionFakes.FakeAgentFee agentFee;
    private LogisticsReconciliationFakes.RecordingLedger ledger;
    private LogisticsReconciliationFakes.FakeSettlementLines lines;
    private LogisticsReconciliationFakes.FakeAdjustments adjustments;
    private LogisticsReconciliationFakes.FakeCases cases;

    private LogisticsCollectionService collection;
    private LogisticsReconciliationService reconciliation;
    private ProviderStatementImporter importer;

    @BeforeEach
    void setUp() {
        clock = new LogisticsCollectionFakes.TestClock(NOW);
        ids = new LogisticsCollectionFakes.SeqIds(8001L);
        receivables = new LogisticsCollectionFakes.FakeReceivables(clock);
        attempts = new LogisticsCollectionFakes.FakeAttempts(clock);
        accounts = new LogisticsCollectionFakes.FakeAccounts();
        agentFee = new LogisticsCollectionFakes.FakeAgentFee(MER_ID);
        ledger = new LogisticsReconciliationFakes.RecordingLedger();
        lines = new LogisticsReconciliationFakes.FakeSettlementLines();
        adjustments = new LogisticsReconciliationFakes.FakeAdjustments();
        cases = new LogisticsReconciliationFakes.FakeCases();

        LogisticsCollectionTxService tx = new LogisticsCollectionTxService(
                receivables, attempts, accounts, ledger, ids::next, clock::now);
        collection = new LogisticsCollectionService(receivables, attempts, tx, agentFee,
                LogisticsCollectionFakes.verifiedGrossUp(), true, 24, ids::next, clock::now);
        reconciliation = new LogisticsReconciliationService(receivables, lines, adjustments,
                accounts, ledger, cases, ids::next, clock::now);
        importer = new ProviderStatementImporter(lines, reconciliation, false, ids::next);
    }

    // =========================================================================

    @Nested
    @DisplayName("Enablement gate：自動抓取沒有做，而且旗標打開也還是沒有")
    class AutomaticFetchDisabled {

        @Test
        @DisplayName("🔴 預設停用，且訊息逐字帶 PROVIDER AUTOMATIC STATEMENT ENABLEMENT = DISABLED")
        void disabledByDefault() {
            assertFalse(importer.isAutomaticStatementEnabled(), "🔴 預設停用是設計不是故障");
            IllegalStateException ex = assertThrows(IllegalStateException.class,
                    () -> importer.fetchFromProvider(PROVIDER, PERIOD));
            assertTrue(ex.getMessage().contains(
                            ProviderStatementImporter.PROVIDER_AUTOMATIC_STATEMENT_ENABLEMENT_DISABLED),
                    "訊息要能被人與 log 直接讀到，實得：" + ex.getMessage());
        }

        /**
         * 🔴 <b>旗標不是實作</b>。回一個空清單比拋還糟：呼叫端會把「沒有這個能力」
         * 讀成「這期沒有帳單」——那正是「對目標不存在回 SKIP 而不是 FAIL」的形狀。
         */
        @Test
        @DisplayName("🔴 旗標打開也還是拋 —— 打開開關不會生出一個不存在的能力")
        void stillThrowsEvenWhenFlagIsOn() {
            ProviderStatementImporter enabled =
                    new ProviderStatementImporter(lines, reconciliation, true, ids::next);
            assertTrue(enabled.isAutomaticStatementEnabled());
            assertThrows(IllegalStateException.class,
                    () -> enabled.fetchFromProvider(PROVIDER, PERIOD));
        }
    }

    // =========================================================================

    @Nested
    @DisplayName("人工匯入：值域先全批驗完才寫，重複列不重複建")
    class ManualImport {

        @Test
        @DisplayName("正常匯入：每列一筆，初始 recon_status = UNMATCHED 且不得帶 ledger_txn_no")
        void importsEachLine() {
            ImportReport report = importer.importManual(List.of(
                    line("UBER-A", 1000L, "DELIVERY"),
                    line("UBER-B", 1500L, "DELIVERY")), ACTOR);

            assertEquals(2, report.imported());
            assertEquals(0, report.duplicates());
            assertEquals(2, importer.lineCount(PROVIDER, STATEMENT_REF));
            long id = report.results().get(0).lineId();
            assertEquals("UNMATCHED", lines.reconStatusOf(id));
            assertEquals(null, lines.ledgerTxnNoOf(id),
                    "🔴 chk_provsettle_ledger：非 MATCHED ⇒ ledger_txn_no 必須為 NULL");
        }

        @Test
        @DisplayName("同一列重匯 ⇒ uq_provsettle_line 擋下，回 DUPLICATE（不是錯誤）")
        void duplicateDetailLineIsDeduped() {
            importer.importManual(List.of(line("UBER-A", 1000L, "DELIVERY")), ACTOR);
            ImportReport second = importer.importManual(
                    List.of(line("UBER-A", 1000L, "DELIVERY")), ACTOR);

            assertEquals(0, second.imported());
            assertEquals(1, second.duplicates());
            assertEquals(1, importer.lineCount(PROVIDER, STATEMENT_REF));
        }

        /**
         * 🔴 <b>本檔最重要的一支</b>：{@code uq_provsettle_line} 含
         * {@code provider_shipment_ref}，而 PostgreSQL 的 NULL <b>互不相等</b>
         * ⇒ 彙總列<b>不會</b>被那個 UNIQUE 去重。閘門在匯入層，所以它必須被測到。
         */
        @Test
        @DisplayName("🔴 彙總列重匯：UNIQUE 擋不住它 ⇒ 由匯入層拒收")
        void aggregateRowReimportIsRejectedByImportLayer() {
            ImportReport first = importer.importManual(List.of(aggregate(500L)), ACTOR);
            assertEquals(1, first.imported(), "第一次匯入的彙總列要收得進來");

            ImportReport second = importer.importManual(List.of(aggregate(500L)), ACTOR);

            assertEquals(0, second.imported());
            assertEquals(0, second.duplicates(), "🔴 它不是被 UNIQUE 擋下的（NULL 互不相等）");
            assertEquals(1, second.rejectedAggregates());
            assertEquals(1, importer.lineCount(PROVIDER, STATEMENT_REF),
                    "🔴 沒有這道閘門就會變成兩列");
        }

        /**
         * 🔴 對照組：把閘門的判定材料換成「邊匯邊查」會讓<b>第一次</b>匯入就出錯。
         * 這一支釘住「快照在整批開始前取一次」這件事。
         */
        @Test
        @DisplayName("🔴 同一批裡明細列在前、彙總列在後 ⇒ 彙總列仍要收得進來（快照在批次開始前取）")
        void aggregateInSameBatchAsDetailsIsStillImported() {
            ImportReport report = importer.importManual(List.of(
                    line("UBER-A", 1000L, "DELIVERY"),
                    line("UBER-B", 1500L, "DELIVERY"),
                    aggregate(120L)), ACTOR);

            assertEquals(3, report.imported(),
                    "🔴 邊匯邊查的話，彙總列會在第一次匯入就被誤判成重複");
            assertEquals(0, report.rejectedAggregates());
        }

        @Test
        @DisplayName("🔴 值域錯誤在任何一列落地之前就拋（不得留下半套帳單）")
        void validationHappensBeforeAnyWrite() {
            assertThrows(IllegalArgumentException.class, () -> importer.importManual(List.of(
                    line("UBER-A", 1000L, "DELIVERY"),
                    line("UBER-B", 1500L, "NOT_A_FEE_TYPE")), ACTOR));
            assertEquals(0, importer.lineCount(PROVIDER, STATEMENT_REF),
                    "🔴 半套帳單會讓彙總列閘門對這份帳單永久關閉");
        }

        @Test
        @DisplayName("金額必須為正／provider 必須是 ShippingMethod／期別必須是 YYYY-MM")
        void rejectsOutOfDomainValues() {
            assertThrows(IllegalArgumentException.class,
                    () -> importer.importManual(List.of(line("UBER-A", 0L, "DELIVERY")), ACTOR));
            assertThrows(IllegalArgumentException.class,
                    () -> importer.importManual(List.of(line("UBER-A", -5L, "DELIVERY")), ACTOR));
            assertThrows(IllegalArgumentException.class, () -> importer.importManual(List.of(
                    new StatementLine("9999", STATEMENT_REF, PERIOD, "UBER-A", null,
                            "DELIVERY", 100L)), ACTOR));
            assertThrows(IllegalArgumentException.class, () -> importer.importManual(List.of(
                    new StatementLine(PROVIDER, STATEMENT_REF, "202609", "UBER-A", null,
                            "DELIVERY", 100L)), ACTOR));
            assertThrows(IllegalArgumentException.class,
                    () -> importer.importManual(List.of(), ACTOR));
            assertThrows(IllegalArgumentException.class,
                    () -> importer.importManual(List.of(line("UBER-A", 100L, "DELIVERY")), 0L));
        }
    }

    // =========================================================================

    @Nested
    @DisplayName("對帳引擎：以 provider_shipment_ref 分組，彙總列不進對帳")
    class ReconcileEngine {

        @Test
        @DisplayName("一趟配送的多列合計成 actual_cost，每一列都被推走")
        void groupsLinesByDeliveryRef() {
            settle("O-A", 910_001L, "UBER-A", 1000L);
            importer.importManual(List.of(
                    line("UBER-A", 1000L, "DELIVERY"),
                    line("UBER-A", 200L, "WAITING")), ACTOR);

            ReconcileReport report = importer.reconcileStatement(
                    PROVIDER, STATEMENT_REF, ReconciliationPolicy.none(), ACTOR);

            assertEquals(1, report.deliveries().size(), "兩列同屬一趟配送 ⇒ 只對一次");
            assertEquals(1200L, report.deliveries().get(0).actualCost());
            assertEquals(1, report.countOf(ReconcileOutcome.RECONCILED));
            assertEquals(0, report.failures());
            assertEquals(200L, accounts.receivableBalance(STORE_ID), "補收 1200 − 1000");
            for (long id : report.deliveries().get(0).statementLineIds()) {
                assertEquals("MATCHED", lines.reconStatusOf(id));
                assertNotNull(lines.ledgerTxnNoOf(id));
            }
        }

        @Test
        @DisplayName("🔴 彙總列不進對帳（它對不到任何一趟配送），留在 UNMATCHED 交 ADM")
        void aggregateRowsAreNotReconciled() {
            settle("O-A", 910_002L, "UBER-A", 1000L);
            ImportReport imported = importer.importManual(List.of(
                    line("UBER-A", 1000L, "DELIVERY"),
                    aggregate(300L)), ACTOR);
            long aggregateId = imported.results().get(1).lineId();

            ReconcileReport report = importer.reconcileStatement(
                    PROVIDER, STATEMENT_REF, ReconciliationPolicy.none(), ACTOR);

            assertEquals(1, report.deliveries().size());
            assertEquals("UNMATCHED", lines.reconStatusOf(aggregateId),
                    "🔴 自動把彙總列攤到某幾筆應收上就是「自動補列」");
        }

        @Test
        @DisplayName("已 MATCHED 的列不再重對（只掃 UNMATCHED）")
        void onlyUnmatchedLinesAreProcessed() {
            settle("O-A", 910_003L, "UBER-A", 1000L);
            importer.importManual(List.of(line("UBER-A", 1000L, "DELIVERY")), ACTOR);
            importer.reconcileStatement(PROVIDER, STATEMENT_REF, ReconciliationPolicy.none(), ACTOR);

            ReconcileReport again = importer.reconcileStatement(
                    PROVIDER, STATEMENT_REF, ReconciliationPolicy.none(), ACTOR);

            assertEquals(0, again.deliveries().size());
            assertEquals(0, again.failures());
        }

        @Test
        @DisplayName("🔴 policy 帶 PROVIDER_CAUSED ⇒ 整組進爭議，賣家一毛不收")
        void policyDrivesResponsibility() {
            settle("O-A", 910_004L, "UBER-A", 1000L);
            importer.importManual(List.of(line("UBER-A", 1800L, "CANCELLATION")), ACTOR);

            ReconcileReport report = importer.reconcileStatement(PROVIDER, STATEMENT_REF,
                    ref -> new ReconciliationPolicy.Attribution(
                            ResponsibilityCause.PROVIDER_CAUSED, 0L, 0L), ACTOR);

            assertEquals(1, report.countOf(ReconcileOutcome.DISPUTED));
            assertEquals(0L, accounts.receivableBalance(STORE_ID), "🔴 不得自動向賣家收");
            assertTrue(adjustments.all().isEmpty());
        }

        @Test
        @DisplayName("🔴 一列壞資料不得擋住整份帳單，但失敗筆數必須數得出來（不是吞掉）")
        void perItemFailureIsCountedNotSwallowed() {
            settle("O-A", 910_005L, "UBER-A", 1000L);
            settle("O-B", 910_006L, "UBER-B", 1000L);
            importer.importManual(List.of(
                    line("UBER-A", 1200L, "DELIVERY"),
                    line("UBER-B", 1200L, "DELIVERY")), ACTOR);

            // UBER-A 的 policy 丟例外，模擬「這一筆的判定原料壞了」。
            ReconcileReport report = importer.reconcileStatement(PROVIDER, STATEMENT_REF, ref -> {
                if ("UBER-A".equals(ref)) {
                    throw new IllegalStateException("刻意注入：這一筆的判定原料壞了");
                }
                return ReconciliationPolicy.Attribution.NONE;
            }, ACTOR);

            assertEquals(2, report.deliveries().size());
            assertEquals(1, report.failures(), "🔴 失敗要數得出來");
            assertEquals(1, report.countOf(ReconcileOutcome.RECONCILED), "另一筆照跑");
            assertEquals(200L, accounts.receivableBalance(STORE_ID), "只有 UBER-B 被補收");
        }

        @Test
        @DisplayName("provider 必須是 ShippingMethod 的 value")
        void reconcileValidatesProvider() {
            assertThrows(IllegalArgumentException.class, () -> importer.reconcileStatement(
                    "9999", STATEMENT_REF, ReconciliationPolicy.none(), ACTOR));
        }
    }

    // =========================================================================

    private static StatementLine line(String deliveryRef, long amount, String feeType) {
        return new StatementLine(PROVIDER, STATEMENT_REF, PERIOD, deliveryRef, null, feeType, amount);
    }

    /** 🔴 彙總列：{@code provider_shipment_ref} 為 NULL。 */
    private static StatementLine aggregate(long amount) {
        return new StatementLine(PROVIDER, STATEMENT_REF, PERIOD, null, null, "ADJUSTMENT", amount);
    }

    /** 走 W3 的真實收款鏈推到 SETTLED。 */
    private void settle(String orderNo, long shipmentId, String deliveryRef, long estimatedCost) {
        collection.createReceivable(new LogisticsCollectionService.CostIncurred(
                STORE_ID, orderNo, shipmentId, PROVIDER, deliveryRef,
                estimatedCost, 0L, NOW));
        List<LogisticsReceivableEntity> pending =
                receivables.listByStatus(List.of(LogisticsReceivableStatus.PENDING), 50);
        LogisticsReceivableEntity r = pending.get(pending.size() - 1);
        agentFee.willAccept();
        collection.submitOne(r, MER_ID);
        var claimed = attempts.claimForQuery(600, 50);
        var attempt = claimed.get(claimed.size() - 1);
        long gross = attempt.getGrossAmount();
        agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-13", gross / 100L, gross));
        collection.queryOne(receivables.findById(r.getId()).orElseThrow(), attempt, MER_ID);
        assertEquals(LogisticsReceivableStatus.SETTLED.value(), receivables.statusOf(r.getId()));
    }
}
