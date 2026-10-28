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
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.ReconcileCommand;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.ReconcileOutcome;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.ReconcileResult;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.TrueUpDecision;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsReconciliationService.TrueUpInput;
import io.github.will413028.hawkr.payment.logistics.domain.Bearer;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.lang.reflect.RecordComponent;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Locale;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

/**
 * W4 —— 對帳 true-up。
 *
 * <h2>🔴 這一支測的是「差額去哪了」，不是「有沒有呼叫某個方法」</h2>
 * 每一格都斷言<b>四個去向</b>（補收賣家／退還賣家／平台吸收／進爭議）的實際金額，
 * 因為 W4 的失效模式全是「錢跑到別格去了」而不是「程式炸掉」：
 * 少收被吃掉、買家原因被算成平台負擔、物流商原因被自動向賣家收 ——
 * 三者的共同症狀都是<b>帳面上一切正常</b>。
 */
@DisplayName("W4 對帳 true-up（LogisticsReconciliationService）")
class LogisticsReconciliationServiceTest {

    private static final long STORE_ID = 5_000_001L;
    private static final String MER_ID = "HAWK5000001";
    private static final String PROVIDER = "3003";
    private static final String STATEMENT_REF = "UBER-STMT-202609-A";
    private static final String STATEMENT_PERIOD = "2026-09";
    private static final String FUND_DATE = "2026-09-13";
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 12, 3, 0);
    private static final long ACTOR = 2_600_000_001L;

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

    private long nextLineId = 9_000_001L;

    @BeforeEach
    void setUp() {
        clock = new LogisticsCollectionFakes.TestClock(NOW);
        ids = new LogisticsCollectionFakes.SeqIds(7001L);
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
    }

    // =========================================================================
    // 1. 純判定（W4-1 ~ W4-4）
    // =========================================================================

    @Nested
    @DisplayName("判定表：差額必須落到四個去向之一，且加總等於 |diff|")
    class DecisionTable {

        @Test
        @DisplayName("actual = estimated ⇒ 什麼都不做")
        void noDifference() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1000L, null, 0L, 0L));
            assertTrue(d.isNoop());
            assertEquals(0L, d.diff());
            assertEquals(0L, d.sellerChargeable());
            assertEquals(0L, d.sellerCredit());
            assertEquals(0L, d.platformAbsorbed());
            assertEquals(0L, d.disputed());
        }

        @Test
        @DisplayName("🔴 W4-1：actual > estimated 且無責任判定 ⇒ 補收賣家（additional receivable）")
        void actualAboveEstimatedChargesSeller() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 0L, 0L));
            assertEquals(200L, d.diff());
            assertEquals(200L, d.sellerChargeable());
            assertEquals(0L, d.platformAbsorbed());
            assertEquals(0L, d.disputed());
            assertEquals(Bearer.SELLER, d.bearer());
        }

        @Test
        @DisplayName("W4-3 SELLER_CAUSED ⇒ 賣家負擔")
        void sellerCaused() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, ResponsibilityCause.SELLER_CAUSED, 0L, 0L));
            assertEquals(200L, d.sellerChargeable());
            assertEquals(Bearer.SELLER, d.bearer());
        }

        /**
         * 🔴 <b>本檔最容易被讀反的一格</b>：買家原因<b>仍歸賣家</b>的物流成本。
         * 買家已經完成一次付款，平台<b>不得</b>向買家二次收款（ADR-004）。
         * ⚠️ 變異「BUYER_CAUSED → Platform」必須讓這一支轉紅。
         */
        @Test
        @DisplayName("🔴 W4-3 BUYER_CAUSED ⇒ 仍先歸賣家物流成本（平台不直接二次收買家）")
        void buyerCausedStillChargesSeller() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, ResponsibilityCause.BUYER_CAUSED, 0L, 0L));
            assertEquals(200L, d.sellerChargeable(), "🔴 BUYER_CAUSED 的物流成本仍由賣家負擔");
            assertEquals(0L, d.platformAbsorbed(), "🔴 平台不得吸收買家原因的成本");
            assertEquals(0L, d.disputed());
            assertEquals(Bearer.SELLER, d.bearer());
        }

        @Test
        @DisplayName("🔴 W4-3 PLATFORM_CAUSED ⇒ 平台全額吸收，不得轉嫁賣家")
        void platformCausedAbsorbedByPlatform() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, ResponsibilityCause.PLATFORM_CAUSED, 0L, 0L));
            assertEquals(0L, d.sellerChargeable(), "🔴 不得轉嫁賣家");
            assertEquals(200L, d.platformAbsorbed());
            assertEquals(Bearer.PLATFORM, d.bearer());
        }

        /** ⚠️ 變異「PROVIDER_CAUSED 自動建 receivable」必須讓這一支轉紅。 */
        @Test
        @DisplayName("🔴 W4-3 PROVIDER_CAUSED ⇒ 進爭議，一毛都不向賣家收")
        void providerCausedGoesToDispute() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, ResponsibilityCause.PROVIDER_CAUSED, 0L, 0L));
            assertEquals(0L, d.sellerChargeable(), "🔴 不得自動向賣家收");
            assertEquals(0L, d.platformAbsorbed());
            assertEquals(200L, d.disputed());
            assertEquals(Bearer.DISPUTE, d.bearer());
        }

        /** ⚠️ 變異「少收就吃掉（不建 credit）」必須讓這一支轉紅。 */
        @Test
        @DisplayName("🔴 W4-2：actual < estimated ⇒ 建立 Seller Credit，不得吃掉")
        void actualBelowEstimatedCreatesSellerCredit() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 800L, null, 0L, 0L));
            assertEquals(-200L, d.diff());
            assertEquals(200L, d.sellerCredit(), "🔴 多收的 200 要還給賣家");
            assertEquals(0L, d.sellerChargeable());
            assertEquals(0L, d.platformAbsorbed(), "🔴 不得變成 platform margin");
            assertEquals(0L, d.disputed());
        }

        @Test
        @DisplayName("🔴 W4-2 的無條件性：即使責任在平台／物流商，多收一樣退賣家")
        void overCollectionAlwaysGoesBackRegardlessOfCause() {
            for (ResponsibilityCause cause : ResponsibilityCause.values()) {
                TrueUpDecision d = LogisticsReconciliationService.decide(
                        new TrueUpInput(1000L, 700L, cause, 500L, 500L));
                assertEquals(300L, d.sellerCredit(),
                        "🔴 多收的錢本來就不是平台的，" + cause.value() + " 也不能留著");
                assertEquals(0L, d.platformAbsorbed(), cause.value());
                assertEquals(0L, d.disputed(), cause.value());
            }
        }

        @Test
        @DisplayName("W4-1 例外：platform subsidy 抵掉補收，餘額才向賣家收")
        void platformSubsidyReducesSellerCharge() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 50L, 0L));
            assertEquals(150L, d.sellerChargeable());
            assertEquals(50L, d.platformAbsorbed());
        }

        @Test
        @DisplayName("platform subsidy 超過差額 ⇒ 只吸收到差額為止，不倒貼")
        void platformSubsidyCappedAtDiff() {
            TrueUpDecision d = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 9999L, 0L));
            assertEquals(0L, d.sellerChargeable());
            assertEquals(200L, d.platformAbsorbed());
        }

        /**
         * 🔴 <b>W4-4 的陰性測試</b>：seller subsidy 是賣家對<b>買家</b>的運費補貼（線 A），
         * <b>不得</b>減少賣家欠平台的物流成本。賣家替買家出運費，provider 不會因此少收平台一毛錢。
         */
        @Test
        @DisplayName("🔴 W4-4：seller subsidy 一毛都不得抵掉賣家的物流應收")
        void sellerSubsidyDoesNotReduceSellerCharge() {
            TrueUpDecision withSellerSubsidy = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 0L, 9999L));
            TrueUpDecision without = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 0L, 0L));
            assertEquals(200L, withSellerSubsidy.sellerChargeable(),
                    "🔴 sellerSubsidy 不是平台補貼，不得抵掉應收");
            assertEquals(without.sellerChargeable(), withSellerSubsidy.sellerChargeable());
            assertEquals(0L, withSellerSubsidy.platformAbsorbed(),
                    "🔴 賣家補貼不得被記成平台吸收");
            assertEquals(9999L, withSellerSubsidy.sellerSubsidy(), "原封帶出供憑據記錄");
        }

        @Test
        @DisplayName("🔴 W4-4：兩種補貼是兩個欄位，合成一個數就測不出上一條")
        void twoSubsidyFieldsAreNeverMerged() {
            TrueUpDecision platformOnly = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 60L, 0L));
            TrueUpDecision sellerOnly = LogisticsReconciliationService.decide(
                    new TrueUpInput(1000L, 1200L, null, 0L, 60L));
            assertEquals(140L, platformOnly.sellerChargeable());
            assertEquals(200L, sellerOnly.sellerChargeable());
            assertFalse(platformOnly.sellerChargeable() == sellerOnly.sellerChargeable(),
                    "🔴 兩個欄位若被合成一個數，這兩個結果會相同");
        }

        /**
         * 🔴 <b>W4-4 逐字：「Buyer Shipping Charge = 0 不代表 Platform 負擔」的機械形式。</b>
         * {@code TrueUpInput} 沒有買家運費這個輸入 ⇒ 從它推導平台補貼在結構上不可能發生。
         */
        @Test
        @DisplayName("🔴 W4-4：判定的輸入裡沒有「買家運費」，所以推不出平台補貼")
        void decisionInputHasNoBuyerShippingCharge() {
            List<String> components = java.util.Arrays.stream(TrueUpInput.class.getRecordComponents())
                    .map(RecordComponent::getName).toList();
            assertEquals(List.of("estimatedCost", "actualCost", "cause",
                            "platformSubsidy", "sellerSubsidy"), components,
                    "TrueUpInput 的欄位集合變了 —— 新增買家運費類輸入會讓 W4-4 的保證消失");
            for (String name : components) {
                assertFalse(name.toLowerCase(Locale.ROOT).contains("buyer"),
                        "🔴 判定不得看買家側任何欄位：" + name);
                assertFalse(name.toLowerCase(Locale.ROOT).contains("shippingcharge"), name);
            }
        }

        @Test
        @DisplayName("金額為負 ⇒ fail-loud，不靜默夾到 0")
        void negativeAmountsThrow() {
            assertThrows(IllegalArgumentException.class, () -> LogisticsReconciliationService.decide(
                    new TrueUpInput(-1L, 100L, null, 0L, 0L)));
            assertThrows(IllegalArgumentException.class, () -> LogisticsReconciliationService.decide(
                    new TrueUpInput(100L, -1L, null, 0L, 0L)));
            assertThrows(IllegalArgumentException.class, () -> LogisticsReconciliationService.decide(
                    new TrueUpInput(100L, 100L, null, -1L, 0L)));
            assertThrows(IllegalArgumentException.class, () -> LogisticsReconciliationService.decide(
                    new TrueUpInput(100L, 100L, null, 0L, -1L)));
        }

        @Test
        @DisplayName("🔴 四個去向的加總恆等於 |diff|（差額被吃掉在結構上測得出來）")
        void allocationIsExhaustive() {
            long[] actuals = {0L, 1L, 500L, 999L, 1000L, 1001L, 5000L};
            for (long actual : actuals) {
                for (ResponsibilityCause cause : causesIncludingNull()) {
                    for (long subsidy : new long[] {0L, 1L, 100L, 100_000L}) {
                        TrueUpDecision d = LogisticsReconciliationService.decide(
                                new TrueUpInput(1000L, actual, cause, subsidy, 7L));
                        assertEquals(Math.abs(d.diff()),
                                d.sellerChargeable() + d.sellerCredit()
                                        + d.platformAbsorbed() + d.disputed(),
                                "actual=" + actual + " cause=" + cause + " subsidy=" + subsidy);
                    }
                }
            }
        }
    }

    // =========================================================================
    // 2. 落地（帳本／調整單／餘額／狀態／帳單列）
    // =========================================================================

    @Nested
    @DisplayName("落地：兩個方向都要有分錄，一邊都不得漏")
    class Persistence {

        @Test
        @DisplayName("🔴 actual > estimated ⇒ 調整單 ＋ TRUE_UP 分錄 ＋ 應收餘額 ＋ CLOSED")
        void chargeableTrueUpWritesEverything() {
            long receivableId = settle("O-1", 900_001L, "UBER-1", 1000L, 0L);
            long lineId = importLine("UBER-1", 1200L);

            ReconcileResult res = reconcile(lineId, "UBER-1", 1200L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.RECONCILED, res.outcome());
            assertEquals(200L, res.decision().sellerChargeable());
            // 賣家軸
            assertEquals(200L, accounts.receivableBalance(STORE_ID), "補收的 200 進賣家下一筆應收");
            assertEquals(0L, accounts.creditBalance(STORE_ID));
            assertEquals(1, ledger.postedOfType(LedgerTxnType.LOGISTICS_TRUE_UP.code()).size());
            assertEquals(0, ledger.postedOfType(LedgerTxnType.LOGISTICS_CREDIT_ISSUE.code()).size());
            // 憑據
            List<LogisticsAdjustmentEntity> adj = adjustments.listByReceivable(receivableId);
            assertEquals(1, adj.size());
            assertEquals(200L, adj.get(0).getAmount());
            assertEquals("RECONCILIATION", adj.get(0).getKind());
            // 線 C ＋ 帳單列
            assertEquals(1, ledger.postedOfType(LedgerTxnType.LOGISTICS_COST_ACCRUAL.code()).size());
            assertEquals("MATCHED", lines.reconStatusOf(lineId));
            assertNotNull(lines.ledgerTxnNoOf(lineId), "🔴 MATCHED ⇔ 指得出一筆分錄");
            // 應收
            LogisticsReceivableEntity r = receivables.findById(receivableId).orElseThrow();
            assertEquals(LogisticsReceivableStatus.CLOSED.value(), r.getStatus());
            assertEquals(1200L, r.getActualCost());
            assertEquals(lineId, r.getStatementLineId());
        }

        @Test
        @DisplayName("🔴 actual < estimated ⇒ 貸記分錄 ＋ credit_balance；應收餘額不得被推成負數")
        void creditTrueUpWritesCreditNotNegativeReceivable() {
            long receivableId = settle("O-2", 900_002L, "UBER-2", 1000L, 0L);
            long lineId = importLine("UBER-2", 800L);

            ReconcileResult res = reconcile(lineId, "UBER-2", 800L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.RECONCILED, res.outcome());
            assertEquals(200L, res.decision().sellerCredit());
            assertEquals(200L, accounts.creditBalance(STORE_ID), "🔴 多收的 200 掛在貸記科目");
            assertEquals(0L, accounts.receivableBalance(STORE_ID),
                    "🔴 應收與貸記是兩個科目：不得把多收記成負的應收（ADR-008 第 1 條）");
            assertEquals(1, ledger.postedOfType(LedgerTxnType.LOGISTICS_CREDIT_ISSUE.code()).size());
            assertEquals(0, ledger.postedOfType(LedgerTxnType.LOGISTICS_TRUE_UP.code()).size(),
                    "🔴 貸記與負向 true-up 同時記 ⇒ 同一筆多收被沖銷兩次");
            assertEquals(LogisticsReceivableStatus.CLOSED.value(),
                    receivables.findById(receivableId).orElseThrow().getStatus());
        }

        /**
         * 🔴 <b>rule 3 的陰性測試</b>：PROVIDER_CAUSED 不得產生任何 receivable。
         * 陽性驗證（有 dispute case）人人記得，<b>「不該出現的沒出現」才是這條規則的實質內容</b>。
         */
        @Test
        @DisplayName("🔴 PROVIDER_CAUSED ⇒ 零 receivable、零貸記、零分錄，只有爭議與案件")
        void providerCausedCreatesNoReceivable() {
            long receivableId = settle("O-3", 900_003L, "UBER-3", 1000L, 0L);
            long balanceBefore = accounts.receivableBalance(STORE_ID);
            long lineId = importLine("UBER-3", 1500L);

            ReconcileResult res = reconcile(lineId, "UBER-3", 1500L,
                    ResponsibilityCause.PROVIDER_CAUSED, 0L, 0L);

            assertEquals(ReconcileOutcome.DISPUTED, res.outcome());
            assertEquals(500L, res.decision().disputed());
            // ── 陰性：這些都必須「沒有發生」 ───────────────────────────────────
            assertEquals(balanceBefore, accounts.receivableBalance(STORE_ID),
                    "🔴 不得自動向賣家收：應收餘額一毛都不能動");
            assertEquals(0L, accounts.creditBalance(STORE_ID));
            assertTrue(adjustments.all().isEmpty(), "🔴 不自動補列：連調整單都不該有");
            assertEquals(0, ledger.postedOfType(LedgerTxnType.LOGISTICS_TRUE_UP.code()).size());
            assertEquals(0, ledger.postedOfType(LedgerTxnType.LOGISTICS_COST_ACCRUAL.code()).size(),
                    "🔴 爭議中的金額不得先認列成平台成本");
            LogisticsReceivableEntity r = receivables.findById(receivableId).orElseThrow();
            assertNull(r.getActualCost(),
                    "🔴 actual_cost 是 write-once：爭議結果出來時改不回來，所以現在不能寫");
            assertNull(r.getStatementLineId());
            assertEquals(LogisticsReceivableStatus.SETTLED.value(), r.getStatus(),
                    "🔴 爭議未決不得 CLOSED —— CLOSED 是終態，它會讓這筆從待辦裡消失");
            // ── 陽性：爭議留痕 ─────────────────────────────────────────────
            assertEquals("DISPUTED", lines.reconStatusOf(lineId));
            assertNull(lines.ledgerTxnNoOf(lineId), "🔴 非 MATCHED ⇒ ledger_txn_no 必須為 NULL");
            assertEquals(1, cases.countOfCategory(
                    LogisticsReconciliationService.CASE_CATEGORY_PROVIDER_DISPUTE));
        }

        @Test
        @DisplayName("🔴 PLATFORM_CAUSED ⇒ 賣家軸不動，但兩列相消的調整單留痕（不得默默吸收）")
        void platformCausedLeavesAuditTrail() {
            long receivableId = settle("O-4", 900_004L, "UBER-4", 1000L, 0L);
            long lineId = importLine("UBER-4", 1300L);

            ReconcileResult res = reconcile(lineId, "UBER-4", 1300L,
                    ResponsibilityCause.PLATFORM_CAUSED, 0L, 0L);

            assertEquals(ReconcileOutcome.RECONCILED, res.outcome());
            assertEquals(300L, res.decision().platformAbsorbed());
            assertEquals(0L, accounts.receivableBalance(STORE_ID), "🔴 不得轉嫁賣家");
            assertEquals(0L, accounts.creditBalance(STORE_ID));
            List<LogisticsAdjustmentEntity> adj = adjustments.listByReceivable(receivableId);
            assertEquals(2, adj.size(), "🔴 差額一列、平台吸收一列 —— 合成一列淨額就看不出吸收了多少");
            assertEquals(300L, adj.get(0).getAmount());
            assertEquals(-300L, adj.get(1).getAmount());
            assertEquals(0L, adjustments.amountSum(), "兩列相消 ⇒ 賣家實際被收 0");
            assertEquals(ResponsibilityCause.PLATFORM_CAUSED.value(), adj.get(1).getResponsibilityCause());
            assertEquals(LedgerAccountCode.PLATFORM_LOGISTICS_COST.code(),
                    ledger.postedOfType(LedgerTxnType.LOGISTICS_COST_ACCRUAL.code())
                            .get(0).entries().get(0).accountCode());
            assertEquals(LogisticsReceivableStatus.CLOSED.value(),
                    receivables.findById(receivableId).orElseThrow().getStatus());
        }

        @Test
        @DisplayName("actual = estimated ⇒ 賣家軸零分錄，但帳單列仍 MATCHED（雙向等式要有分錄可指）")
        void zeroDiffStillMatchesLine() {
            long receivableId = settle("O-5", 900_005L, "UBER-5", 1000L, 0L);
            long lineId = importLine("UBER-5", 1000L);

            ReconcileResult res = reconcile(lineId, "UBER-5", 1000L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.RECONCILED, res.outcome());
            assertTrue(res.decision().isNoop());
            assertTrue(adjustments.all().isEmpty());
            assertEquals(0L, accounts.receivableBalance(STORE_ID));
            assertEquals("MATCHED", lines.reconStatusOf(lineId));
            assertNotNull(lines.ledgerTxnNoOf(lineId));
            assertEquals(LogisticsReceivableStatus.CLOSED.value(),
                    receivables.findById(receivableId).orElseThrow().getStatus());
        }

        @Test
        @DisplayName("🔴 一趟配送多列（DELIVERY ＋ WAITING）⇒ 每一列都要被推走，不得只對第一列")
        void everyLineOfTheDeliveryGetsMatched() {
            long receivableId = settle("O-6", 900_006L, "UBER-6", 1000L, 0L);
            long deliveryLine = importLine("UBER-6", 1000L, "DELIVERY");
            long waitingLine = importLine("UBER-6", 150L, "WAITING");

            ReconcileResult res = reconciliation.reconcileOne(new ReconcileCommand(
                    List.of(deliveryLine, waitingLine), PROVIDER, STATEMENT_REF, STATEMENT_PERIOD,
                    "UBER-6", 1150L, null, 0L, 0L, ACTOR));

            assertEquals(ReconcileOutcome.RECONCILED, res.outcome());
            assertEquals(150L, res.decision().sellerChargeable(), "actual = 1000 + 150");
            assertEquals("MATCHED", lines.reconStatusOf(deliveryLine));
            assertEquals("MATCHED", lines.reconStatusOf(waitingLine), "🔴 第二列不得留在 UNMATCHED");
            assertEquals(Math.min(deliveryLine, waitingLine),
                    receivables.findById(receivableId).orElseThrow().getStatementLineId(),
                    "憑據欄取最小的 id（呼叫端順序不保證穩定）");
        }

        @Test
        @DisplayName("帳單列對不到唯一應收 ⇒ 進爭議並開案，🔴 不猜、不自動補列")
        void unmatchedLineGoesToDispute() {
            long lineId = importLine("UBER-NOBODY", 500L);

            ReconcileResult res = reconcile(lineId, "UBER-NOBODY", 500L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.DISPUTED, res.outcome());
            assertNull(res.decision());
            assertEquals("DISPUTED", lines.reconStatusOf(lineId));
            assertEquals(1, cases.countOfCategory(
                    LogisticsReconciliationService.CASE_CATEGORY_STATEMENT_UNMATCHED));
            assertTrue(adjustments.all().isEmpty());
        }

        @Test
        @DisplayName("🔴 同一個 delivery_ref 命中多筆應收 ⇒ 一樣進爭議（不取第一筆）")
        void ambiguousMatchGoesToDispute() {
            // 🔴 這個狀態在 V15 之後**真 DB 建不出來**：uq_logrecv_delivery_ref 是單欄 UNIQUE
            //    （不含 provider）⇒ 一條 provider leg 全域只能有一筆應收，換 provider 也不行。
            //
            //    那為什麼還要測？因為 reconcileOne 的 `matched.size() != 1 ⇒ 進爭議`
            //    是**防禦分支**：它防的不是今天，而是哪天有人移除那道索引、或改寫了
            //    findByProviderDeliveryRef 的查詢。**防禦分支沒有測試就等於沒有防禦。**
            //
            //    ⚠️ 因此這裡刻意用 forceInsertBypassingUniqueness 直接塞第二列，
            //    而不是走 settle()——後者會被 fake 鏡射的 UNIQUE 正確擋下（回 false 不建列），
            //    那正是它該有的行為，不是這支測試的缺陷。
            settle("O-7a", 900_007L, "UBER-DUP", 1000L, 0L);
            LogisticsReceivableEntity duplicate = new LogisticsReceivableEntity();
            duplicate.setId(999_007L);
            duplicate.setStoreId(STORE_ID);
            duplicate.setOrderNo("O-7b");
            duplicate.setProvider(PROVIDER);
            duplicate.setProviderDeliveryRef("UBER-DUP");
            duplicate.setIdempotencyKey("logistics_receivable:" + PROVIDER + ":UBER-DUP-forced");
            duplicate.setEstimatedCost(1000L);
            receivables.forceInsertBypassingUniqueness(duplicate);

            long lineId = importLine("UBER-DUP", 1000L);

            ReconcileResult res = reconcile(lineId, "UBER-DUP", 1000L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.DISPUTED, res.outcome());
            assertEquals("DISPUTED", lines.reconStatusOf(lineId));
        }

        @Test
        @DisplayName("應收還沒 SETTLED（帳單早到）⇒ 本輪不處理，帳單列留在 UNMATCHED 等下一輪")
        void statementBeforeCollectionIsDeferred() {
            collection.createReceivable(new LogisticsCollectionService.CostIncurred(
                    STORE_ID, "O-8", 900_009L, PROVIDER, "UBER-8", 1000L, 0L, NOW));
            long lineId = importLine("UBER-8", 1200L);

            ReconcileResult res = reconcile(lineId, "UBER-8", 1200L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.NOT_YET_SETTLED, res.outcome());
            assertEquals("UNMATCHED", lines.reconStatusOf(lineId), "🔴 留在候選集合裡，不是對完了");
            assertTrue(adjustments.all().isEmpty());
            assertTrue(cases.all().isEmpty());
        }

        @Test
        @DisplayName("🔴 已結清後又來一列費用 ⇒ 進爭議（write-once 的 actual_cost 併不進去，不得靜默丟掉）")
        void lateLineAfterCloseGoesToDispute() {
            settle("O-9", 900_010L, "UBER-9", 1000L, 0L);
            long lineId = importLine("UBER-9", 1200L);
            reconcile(lineId, "UBER-9", 1200L, null, 0L, 0L);
            long balanceAfterFirst = accounts.receivableBalance(STORE_ID);
            int adjAfterFirst = adjustments.all().size();

            long lateLine = importLine("UBER-9", 300L, "ADJUSTMENT");
            ReconcileResult res = reconcile(lateLine, "UBER-9", 300L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.DISPUTED, res.outcome());
            assertEquals("DISPUTED", lines.reconStatusOf(lateLine),
                    "🔴 這一列必須有歸宿，不得永遠停在 UNMATCHED");
            assertEquals(balanceAfterFirst, accounts.receivableBalance(STORE_ID),
                    "🔴 不得偷偷再開一筆應收");
            assertEquals(adjAfterFirst, adjustments.all().size());
            assertEquals(1, cases.countOfCategory(
                    LogisticsReconciliationService.CASE_CATEGORY_STATEMENT_UNMATCHED));
        }

        @Test
        @DisplayName("同一列重跑（帳單重對）⇒ 什麼都不做，也不再開案")
        void rerunningTheSameLineIsNoop() {
            settle("O-10", 900_011L, "UBER-10", 1000L, 0L);
            long lineId = importLine("UBER-10", 1200L);
            reconcile(lineId, "UBER-10", 1200L, null, 0L, 0L);
            long balanceAfterFirst = accounts.receivableBalance(STORE_ID);
            int adjAfterFirst = adjustments.all().size();

            ReconcileResult res = reconcile(lineId, "UBER-10", 1200L, null, 0L, 0L);

            assertEquals(ReconcileOutcome.ALREADY_RECONCILED, res.outcome());
            assertEquals(balanceAfterFirst, accounts.receivableBalance(STORE_ID),
                    "🔴 重跑不得再補收一次");
            assertEquals(adjAfterFirst, adjustments.all().size());
            assertEquals("MATCHED", lines.reconStatusOf(lineId), "已配對的列不得被退回爭議");
            assertTrue(cases.all().isEmpty());
        }
    }

    // =========================================================================
    // 3. 邊界輸入
    // =========================================================================

    @Nested
    @DisplayName("輸入值域：錯的就拋，不猜也不補")
    class InputValidation {

        @Test
        @DisplayName("statement_period 必須是 'YYYY-MM'")
        void settlementPeriodFormat() {
            assertEquals("202609", LogisticsReconciliationService.toSettlementPeriod("2026-09"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.toSettlementPeriod("202609"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.toSettlementPeriod("2026-9"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.toSettlementPeriod("20xx-09"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.toSettlementPeriod(null));
        }

        @Test
        @DisplayName("🔴 provider 必須是 ShippingMethod 的 value —— 直接 parse 會讓任何數字通過")
        void providerMustBeAKnownShippingMethod() {
            assertEquals(3003L, LogisticsReconciliationService.providerRefId("3003"));
            assertEquals(3005L, LogisticsReconciliationService.providerRefId("3005"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.providerRefId("3004"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.providerRefId("9999"));
            assertThrows(IllegalArgumentException.class,
                    () -> LogisticsReconciliationService.providerRefId("UBER"));
        }

        @Test
        @DisplayName("statementLineIds 不得為空（沒有列就沒有憑據）")
        void statementLineIdsRequired() {
            assertThrows(IllegalArgumentException.class, () -> new ReconcileCommand(
                    List.of(), PROVIDER, STATEMENT_REF, STATEMENT_PERIOD, "UBER-X",
                    100L, null, 0L, 0L, ACTOR));
        }
    }

    // =========================================================================
    // 測試工具
    // =========================================================================

    private static List<ResponsibilityCause> causesIncludingNull() {
        List<ResponsibilityCause> out = new java.util.ArrayList<>();
        out.add(null);
        out.addAll(List.of(ResponsibilityCause.values()));
        return out;
    }

    /** 走 W3 的<b>真實</b>收款鏈把一筆應收推到 SETTLED（不直接改狀態冒充）。 */
    private long settle(String orderNo, long shipmentId, String deliveryRef,
                        long estimatedCost, long platformSubsidy) {
        collection.createReceivable(new LogisticsCollectionService.CostIncurred(
                STORE_ID, orderNo, shipmentId, PROVIDER, deliveryRef,
                estimatedCost, platformSubsidy, NOW));
        List<LogisticsReceivableEntity> pending =
                receivables.listByStatus(List.of(LogisticsReceivableStatus.PENDING), 50);
        LogisticsReceivableEntity r = pending.get(pending.size() - 1);

        agentFee.willAccept();
        collection.submitOne(r, MER_ID);

        List<io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity> claimed =
                attempts.claimForQuery(600, 50);
        var attempt = claimed.get(claimed.size() - 1);
        long gross = attempt.getGrossAmount();
        // 🔴 費率政策是 1%／DOWN（verifiedGrossUp）⇒ PayUni 收整整 1% 時 residual = 0。
        //    這是**情境事實**（我方設定的 fake 通道行為），不是從被測物讀回來的期望值。
        long actualFee = gross / 100L;
        agentFee.willQuery(LogisticsCollectionFakes.settled(FUND_DATE, actualFee, gross));
        collection.queryOne(receivables.findById(r.getId()).orElseThrow(), attempt, MER_ID);

        assertEquals(LogisticsReceivableStatus.SETTLED.value(), receivables.statusOf(r.getId()),
                "前置條件沒成立：收款鏈沒把它推到 SETTLED");
        return r.getId();
    }

    private long importLine(String deliveryRef, long amount) {
        return importLine(deliveryRef, amount, "DELIVERY");
    }

    private long importLine(String deliveryRef, long amount, String feeType) {
        var e = new io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity();
        long id = nextLineId++;
        e.setId(id);
        e.setProvider(PROVIDER);
        e.setStatementRef(STATEMENT_REF);
        e.setStatementPeriod(STATEMENT_PERIOD);
        e.setProviderShipmentRef(deliveryRef);
        e.setFeeType(feeType);
        e.setAmount(amount);
        e.setReconStatus("UNMATCHED");
        e.setImportedBy(ACTOR);
        assertTrue(lines.importIfAbsent(e), "帳單列前置條件沒建起來");
        return id;
    }

    private ReconcileResult reconcile(long lineId, String deliveryRef, long actualCost,
                                      ResponsibilityCause cause,
                                      long platformSubsidy, long sellerSubsidy) {
        return reconciliation.reconcileOne(new ReconcileCommand(
                List.of(lineId), PROVIDER, STATEMENT_REF, STATEMENT_PERIOD, deliveryRef,
                actualCost, cause, platformSubsidy, sellerSubsidy, ACTOR));
    }
}
