package io.github.will413028.hawkr.payment.logistics.application;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.ReconcileReport;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.ReconciliationPolicy;
import io.github.will413028.hawkr.payment.logistics.application.ProviderStatementImporter.StatementLine;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * 🔴 <b>多期現金守恆（N = 10）</b>：連續十期 accrual → collect → true-up，
 * 斷言<b>平台累計現金差額恆為 0</b>。
 *
 * <h2>為什麼一定要多期</h2>
 * 線 B 的每一個機制都會把錢<b>推到下一期</b>：未收足的併進下一筆 {@code target_net}、
 * 補收的 true-up 進賣家下一筆應收、多收的掛在貸記餘額。⇒ <b>單期會全綠的錯誤</b>有一整類：
 * <ul>
 *   <li>true-up 的補收記到<b>貸記</b>而不是應收 —— 單期看「錢有記到某個地方」是對的，
 *       下一期的 {@code target_net} 才會少那一塊</li>
 *   <li>結轉被<b>認列兩次</b>（帳本把 carried 也記成收入）—— 單期的收入看起來只是大了一點</li>
 *   <li>多收同時走「負的應收」與「貸記」兩條路 —— 單期兩個數字各自都對，
 *       第二期的 {@code target_net} 才會被扣兩次</li>
 * </ul>
 *
 * <h2>🔴 斷言的期望值全部是硬寫的字面值</h2>
 * 一個都不從被測物讀回來（{@code 03-delegation-templates.md} §0.3 第二形態：
 * 期望值跟著被測物一起變 ⇒ 它驗不到任何東西）。下面那張情境表與三個期望陣列
 * 是從 W4 規格逐期手算出來的，數字寫死。
 *
 * <h2>入口</h2>
 * 全程走<b>正式入口</b>：{@code createReceivable} → {@code submitOne} → {@code queryOne}
 * → {@code importManual} → {@code reconcileStatement}。不直接改狀態、不直接呼叫中間層。
 */
@DisplayName("線 B 多期現金守恆（N=10 期 accrual → collect → true-up）")
class LogisticsMultiPeriodCashConservationTest {

    private static final long STORE_ID = 5_000_003L;
    private static final String MER_ID = "HAWK5000003";
    private static final String PROVIDER = "3003";
    private static final String PERIOD = "2026-09";
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 12, 3, 0);
    private static final long ACTOR = 2_600_000_003L;

    /**
     * 情境表（手寫）：每一期的暫估成本／平台補貼／帳單實際成本。
     *
     * <p>刻意涵蓋三個方向各數次：{@code actual = estimated}（第 1、4、5、10 期）、
     * {@code actual > estimated}（第 2、6、8 期 ⇒ 補收，會結轉到下一期）、
     * {@code actual < estimated}（第 3、7、9 期 ⇒ 貸記）。
     */
    private static final long[] ESTIMATED = {100, 100, 100, 200, 150, 150, 150, 300, 300, 100};
    private static final long[] SUBSIDY = {0, 0, 0, 0, 50, 50, 50, 0, 0, 0};
    private static final long[] ACTUAL = {100, 120, 80, 200, 150, 170, 130, 301, 299, 100};

    /** 每一期送出時的 {@code target_net}（＝本期認列 ＋ 前期結轉）。手算。 */
    private static final long[] EXPECTED_TARGET_NET = {100, 100, 120, 200, 100, 100, 120, 300, 301, 100};
    /** 每一期 true-up 之後的 {@code receivable_balance}。手算。 */
    private static final long[] EXPECTED_RECEIVABLE_AFTER = {0, 20, 0, 0, 0, 20, 0, 1, 0, 0};
    /** 每一期 true-up 之後的 {@code credit_balance}（🔴 只增不減：貸記永不過期）。手算。 */
    private static final long[] EXPECTED_CREDIT_AFTER = {0, 0, 20, 20, 20, 20, 40, 40, 41, 41};

    /** Σ 各期實收（＝平台真正進帳的現金）。手算：100+100+120+200+100+100+120+300+301+100。 */
    private static final long TOTAL_NET_PROCEEDS = 1541L;
    /** Σ 各期帳單實際成本（＝平台要付給 provider 的錢）。手算。 */
    private static final long TOTAL_ACTUAL_COST = 1650L;
    /** Σ 平台補貼（平台自願承擔的那一塊）。手算。 */
    private static final long TOTAL_SUBSIDY = 150L;

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
    private ProviderStatementImporter importer;

    private final List<Long> netProceedsByPeriod = new ArrayList<>();

    @BeforeEach
    void setUp() {
        clock = new LogisticsCollectionFakes.TestClock(NOW);
        ids = new LogisticsCollectionFakes.SeqIds(6001L);
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
        LogisticsReconciliationService reconciliation = new LogisticsReconciliationService(
                receivables, lines, adjustments, accounts, ledger, cases, ids::next, clock::now);
        importer = new ProviderStatementImporter(lines, reconciliation, false, ids::next);
    }

    @Test
    @DisplayName("🔴 十期跑完：平台累計現金差額 = 0，且每一期的結轉都對得上")
    void cashIsConservedAcrossTenPeriods() {
        for (int i = 0; i < 10; i++) {
            runPeriod(i);

            assertEquals(EXPECTED_RECEIVABLE_AFTER[i], accounts.receivableBalance(STORE_ID),
                    "第 " + (i + 1) + " 期結束後的應收餘額（＝下一期要結轉的數字）");
            assertEquals(EXPECTED_CREDIT_AFTER[i], accounts.creditBalance(STORE_ID),
                    "第 " + (i + 1) + " 期結束後的貸記餘額（🔴 只增不減）");
        }

        long sumNet = netProceedsByPeriod.stream().mapToLong(Long::longValue).sum();
        assertEquals(TOTAL_NET_PROCEEDS, sumNet, "Σ 實收與情境表對不上 —— 前置條件就壞了");

        long receivableBalance = accounts.receivableBalance(STORE_ID);
        long creditBalance = accounts.creditBalance(STORE_ID);

        // 🔴 平台累計現金差額：
        //    收到的錢 − 要付給 provider 的錢 + 平台自願承擔的補貼
        //      + 賣家還欠平台的 − 平台還欠賣家的   ==  0
        //    每一項都是**情境事實或最終餘額**，沒有一項是從被測物的判定結果讀回來的。
        long cashDifference = sumNet
                - TOTAL_ACTUAL_COST
                + TOTAL_SUBSIDY
                + receivableBalance
                - creditBalance;
        assertEquals(0L, cashDifference, String.format(
                "🔴 平台累計現金差額必須為 0（Σnet=%d Σactual=%d Σsubsidy=%d recv=%d credit=%d）",
                sumNet, TOTAL_ACTUAL_COST, TOTAL_SUBSIDY, receivableBalance, creditBalance));

        assertEquals(0L, receivableBalance, "十期跑完，賣家不欠平台");
        assertEquals(41L, creditBalance, "十期跑完，平台欠賣家 41（三次多收 20＋20＋1）");
    }

    /**
     * 同一件事從<b>帳本</b>再驗一次。
     *
     * <p>🔴 它不是上一支的複製：上一支看的是 {@code seller_logistics_accounts} 的兩個餘額
     * （delta 累加），這一支看的是 {@code ledger_entries} 的逐科目借貸。
     * 兩者由<b>不同的寫入路徑</b>維護 —— 只驗一邊的話，「餘額動了但分錄沒記」
     * 或反過來，都會全綠。
     */
    @Test
    @DisplayName("🔴 十期跑完：帳本逐科目的借貸與情境事實逐項對得上")
    void ledgerMatchesTheFactsAcrossTenPeriods() {
        for (int i = 0; i < 10; i++) {
            runPeriod(i);
        }

        assertEquals(ledger.totalDebit(), ledger.totalCredit(), "INV-L1 的整體形式");

        assertEquals(TOTAL_NET_PROCEEDS, ledger.netDebit(LedgerAccountCode.PLATFORM_OPERATING_CASH),
                "🔴 平台自有現金 = Σ 實收（gross − actual_fee），不是 Σ gross");
        assertEquals(TOTAL_ACTUAL_COST, ledger.netDebit(LedgerAccountCode.PLATFORM_LOGISTICS_COST),
                "平台物流成本 = Σ 帳單實際成本");
        assertEquals(TOTAL_ACTUAL_COST, ledger.netCredit(LedgerAccountCode.PROVIDER_PAYABLE),
                "應付物流商 = Σ 帳單實際成本（本波沒有付款那一半，見類別註解）");
        assertEquals(0L, ledger.netDebit(LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE),
                "🔴 賣家物流應收結清 —— 若多收被記成負的應收，這裡會是負數");
        assertEquals(41L, ledger.netCredit(LedgerAccountCode.SELLER_LOGISTICS_CREDIT),
                "🔴 賣家物流貸記 = 三次多收的合計，不得被吃掉");

        // 🔴 平台的損益恆等式：服務費收入 = Σ 實際成本 − Σ 補貼。
        //    平台是物流成本的**過手方**，賺的只有「不補貼的那一塊」。
        assertEquals(TOTAL_ACTUAL_COST - TOTAL_SUBSIDY,
                ledger.netCredit(LedgerAccountCode.PLATFORM_LOGISTICS_SERVICE_INCOME),
                "🔴 收入 = Σ實際成本 − Σ補貼；多認列一次結轉就會讓這個數字變大");
    }

    // =========================================================================

    /** 一期完整跑完：應收成立 → 送出 → 入帳 → 帳單匯入 → true-up。 */
    private void runPeriod(int i) {
        int no = i + 1;
        String orderNo = "O-P" + no;
        String deliveryRef = "UBER-P" + no;
        String statementRef = "UBER-STMT-P" + no;

        // ── 1. 應收成立（送達事件） ────────────────────────────────────────────
        collection.createReceivable(new LogisticsCollectionService.CostIncurred(
                STORE_ID, orderNo, 920_000L + no, PROVIDER, deliveryRef,
                ESTIMATED[i], SUBSIDY[i], NOW));
        List<LogisticsReceivableEntity> pending =
                receivables.listByStatus(List.of(LogisticsReceivableStatus.PENDING), 50);
        assertEquals(1, pending.size(), "第 " + no + " 期應該只有一筆 PENDING");
        LogisticsReceivableEntity r = pending.get(0);
        assertEquals(EXPECTED_TARGET_NET[i], r.getTargetNet(),
                "🔴 第 " + no + " 期的 target_net（本期認列 ＋ 前期結轉）");

        // ── 2. 送出 ──────────────────────────────────────────────────────────
        agentFee.willAccept();
        assertEquals(LogisticsCollectionService.SubmitResult.SUBMITTED,
                collection.submitOne(r, MER_ID));

        // ── 3. 查詢入帳 ───────────────────────────────────────────────────────
        List<LogisticsCollectionAttemptEntity> claimed = attempts.claimForQuery(600, 50);
        assertEquals(1, claimed.size(), "第 " + no + " 期應該只有一筆待查 attempt");
        LogisticsCollectionAttemptEntity attempt = claimed.get(0);
        long gross = attempt.getGrossAmount();
        // 🔴 情境事實：PayUni 收整整 1%（與 verifiedGrossUp 的政策一致）⇒ residual = 0。
        //    這是我方設定的 fake 通道行為，不是從被測物讀回來的期望值。
        long actualFee = gross / 100L;
        agentFee.willQuery(LogisticsCollectionFakes.settled("2026-09-13", actualFee, gross));
        assertEquals(LogisticsCollectionService.QueryResult.SETTLED,
                collection.queryOne(receivables.findById(r.getId()).orElseThrow(), attempt, MER_ID));

        LogisticsCollectionAttemptEntity settled = attempts.raw(attempt.getId());
        assertEquals(0L, settled.getResidual(),
                "🔴 第 " + no + " 期的 residual 必須是 0，否則本檔的守恆式少了一項");
        long netProceeds = settled.getNetProceeds();
        assertEquals(EXPECTED_TARGET_NET[i], netProceeds, "實收應等於 target_net");
        netProceedsByPeriod.add(netProceeds);

        // ── 4. 帳單匯入（人工路徑） ────────────────────────────────────────────
        importer.importManual(List.of(new StatementLine(
                PROVIDER, statementRef, PERIOD, deliveryRef, orderNo, "DELIVERY", ACTUAL[i])), ACTOR);

        // ── 5. true-up ───────────────────────────────────────────────────────
        ReconcileReport report = importer.reconcileStatement(
                PROVIDER, statementRef, ReconciliationPolicy.none(), ACTOR);
        assertEquals(0, report.failures(), "第 " + no + " 期對帳不得有失敗");
        assertEquals(1, report.countOf(
                LogisticsReconciliationService.ReconcileOutcome.RECONCILED),
                "第 " + no + " 期應該對成功一趟配送");
        assertEquals(LogisticsReceivableStatus.CLOSED.value(), receivables.statusOf(r.getId()),
                "true-up 完成 ⇒ CLOSED");
        assertTrue(cases.all().isEmpty(), "第 " + no + " 期不該開任何爭議案件");
    }
}
