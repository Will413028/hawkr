package io.github.will413028.hawkr.payment.logistics.domain;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.github.will413028.hawkr.payment.logistics.domain.LogisticsLedgerBuilders.Input;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Supplier;
import java.util.stream.Stream;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/**
 * 70 §3.8 的「守恆自檢」與科目對照表（Go 版 {@code logistics_ledger_test.go} 的 1:1 移植）。
 *
 * <p>🔴 <b>三條斷言守的是三條不同的軸，一條斷言只看得到一條</b>：
 * <pre>
 *   C1 期末應收 ＝ residual        → 收款有沒有記全
 *   C2 收入只認列本期              → 不重複認列收入
 *   C3 平台零費用且現金不短列      → ADR-005（手續費由賣家負擔）
 * </pre>
 * 只驗其中一條的話，另外兩條被破壞時會全綠。
 *
 * <p>期望值（科目名、金額）一律硬寫字面值，不從被測物取。
 */
class LogisticsLedgerBuildersTest {

    private static final long STORE_ID = 2685914682L;
    private static final long PROVIDER_REF = 3003L;

    private static Input input(String ref) {
        return new Input(STORE_ID, "O" + ref, "202609", "ORDER", ref);
    }

    private static Input adminInput(String ref) {
        return new Input(STORE_ID, "O" + ref, "202609", "ADMIN", ref);
    }

    /** 把一組分錄按科目彙總成「借 − 貸」的淨額。 */
    private static Map<String, Long> sumByAccount(List<LedgerTxn> txns) {
        Map<String, Long> out = new HashMap<>();
        for (LedgerTxn t : txns) {
            for (LedgerEntry e : t.entries()) {
                long delta = e.isDebit() ? e.amount() : -e.amount();
                out.merge(e.accountCode(), delta, Long::sum);
            }
        }
        return out;
    }

    private static long net(Map<String, Long> sums, String account) {
        return sums.getOrDefault(account, 0L);
    }

    // ── 守恆自檢 ──────────────────────────────────────────────────────────────

    @Test
    @DisplayName("C1+C2+C3：費率估準時，一整筆收款走完之後三條斷言同時成立")
    void conservationWhenRateEstimatedCorrectly() {
        Input in = input("SHP-1");

        // 認列：成本 120、無補貼 ⇒ 本期認列 120。
        LedgerTxn accrual = LogisticsLedgerBuilders.logisticsReceivableAccrual(in, 120L);
        // 收款：費率估準 ⇒ net_proceeds 正好 120。
        LedgerTxn settled = LogisticsLedgerBuilders.logisticsCollectionSettled(in, 120L);

        Map<String, Long> sums = sumByAccount(List.of(accrual, settled));

        // C1：期末應收 ＝ residual（估準時為 0）。
        assertThat(net(sums, "SELLER_LOGISTICS_RECEIVABLE"))
                .as("C1 期末應收，費率估準時必須為 0（不為 0 代表收款沒記全）")
                .isEqualTo(0L);
        // C2：收入只認列本期 120（貸方 ⇒ 淨額為 −120）。
        assertThat(net(sums, "PLATFORM_LOGISTICS_SERVICE_INCOME"))
                .as("C2 收入淨額必須是 −120（只認列本期）")
                .isEqualTo(-120L);
        // 🔴 C3：平台零手續費費用，且現金不短列。
        assertThat(net(sums, "PLATFORM_COLLECTION_FEE_EXPENSE"))
                .as("C3 平台手續費費用：正常路徑必須為 0 —— 手續費由賣家負擔（ADR-005），"
                        + "記成平台費用會讓損益表多一筆不存在的支出")
                .isEqualTo(0L);
        assertThat(net(sums, "PLATFORM_OPERATING_CASH"))
                .as("C3 平台現金必須等於實收 120")
                .isEqualTo(120L);
    }

    @Test
    @DisplayName("🔴 費率估不準時，期末應收必須恰好等於 residual 而不是被沖平（ADR-011 結轉）")
    void shortfallStaysInReceivable() {
        Input in = input("SHP-2");

        LedgerTxn accrual = LogisticsLedgerBuilders.logisticsReceivableAccrual(in, 12000L);
        // 預估 1%、實際 1.5% ⇒ net_proceeds 11940 ⇒ residual 60。
        LedgerTxn settled = LogisticsLedgerBuilders.logisticsCollectionSettled(in, 11940L);

        Map<String, Long> sums = sumByAccount(List.of(accrual, settled));

        // 照直覺補一筆「DEBIT 手續費費用／CREDIT 應收」會讓這裡變成 0 ⇒ 缺口消失 ⇒ 下期收不回來。
        assertThat(net(sums, "SELLER_LOGISTICS_RECEIVABLE"))
                .as("期末應收必須 = residual 60；被沖平代表結轉機制在帳上失效")
                .isEqualTo(60L);
        assertThat(net(sums, "PLATFORM_COLLECTION_FEE_EXPENSE"))
                .as("平台手續費費用在正常路徑必須為 0")
                .isEqualTo(0L);
    }

    @Test
    @DisplayName("🔴 true-up 是雙向的：實際 < 暫估時方向必須相反（同一個 txn_type）")
    void trueUpIsBidirectional() {
        Input in = input("STM-1");

        LedgerTxn up = LogisticsLedgerBuilders.logisticsTrueUp(in, 30L);
        assertThat(net(sumByAccount(List.of(up)), "SELLER_LOGISTICS_RECEIVABLE"))
                .as("實際 > 暫估時應收應 +30")
                .isEqualTo(30L);

        LedgerTxn down = LogisticsLedgerBuilders.logisticsTrueUp(in, -30L);
        assertThat(net(sumByAccount(List.of(down)), "SELLER_LOGISTICS_RECEIVABLE"))
                .as("實際 < 暫估時應收應 −30")
                .isEqualTo(-30L);

        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsTrueUp(in, 0L))
                .as("差額為 0 不該產生分錄")
                .isInstanceOf(LedgerValidationException.class);
    }

    @Test
    @DisplayName("🔴 貸記借的是「收入」不是「費用」")
    void creditIssueReversesIncomeNotExpense() {
        // 記成費用會讓損益表上同時有「虛增的收入」與「不存在的費用」，兩者金額相同但都是假的。
        LedgerTxn txn = LogisticsLedgerBuilders.logisticsCreditIssue(adminInput("CRD-1"), 50L);

        Map<String, Long> sums = sumByAccount(List.of(txn));
        assertThat(net(sums, "PLATFORM_LOGISTICS_SERVICE_INCOME"))
                .as("貸記必須沖減當初認列的收入（借方 +50）")
                .isEqualTo(50L);
        assertThat(net(sums, "SELLER_LOGISTICS_CREDIT"))
                .as("貸記餘額（平台欠賣家）應為貸方 50")
                .isEqualTo(-50L);
        assertThat(sums)
                .as("🔴 貸記不得觸及手續費費用科目")
                .doesNotContainKey("PLATFORM_COLLECTION_FEE_EXPENSE");
    }

    @Test
    @DisplayName("🔴 PROVIDER_SETTLEMENT 在新商流的對手科目是 PLATFORM_OPERATING_CASH，不是 PLATFORM_COLLECTED_CASH")
    void providerSettlementUsesOperatingCash() {
        // SELLER_DIRECT 下平台沒有代收，PLATFORM_COLLECTED_CASH 無語意。
        LedgerTxn txn = LogisticsLedgerBuilders.logisticsProviderSettlement(
                adminInput("STM-2"), PROVIDER_REF, 120L);

        String creditAccount = txn.entries().stream()
                .filter(LedgerEntry::isCredit)
                .map(LedgerEntry::accountCode)
                .findFirst()
                .orElseThrow();

        assertThat(creditAccount).isEqualTo("PLATFORM_OPERATING_CASH");
        assertThat(creditAccount).as("🔴 抄到舊商流的代收科目了").doesNotContain("COLLECTED");
    }

    // ── 科目對照表（70 §3.8）─────────────────────────────────────────────────

    /**
     * 🔴 每一個 builder 的<b>借方／貸方科目</b>逐一硬寫。
     *
     * <p>這是「科目對猜錯 ⇒ COMMIT 當下 500」的機械防線：把任何一個 builder 的借貸方向對調，
     * 這一組就必須轉紅。只靠守恆自檢的話，方向對調在只有兩行的分錄裡仍然是平衡的
     * （Σ DEBIT = Σ CREDIT 照樣成立）⇒ {@code trg_ledger_balanced} 也看不出來。
     */
    static Stream<Arguments> accountPairs() {
        Input in = new Input(STORE_ID, "OALL-1", "202609", "ADMIN", "ALL-1");
        return Stream.of(
                Arguments.of("LOGISTICS_RECEIVABLE_ACCRUAL",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsReceivableAccrual(in, 120L),
                        "SELLER_LOGISTICS_RECEIVABLE", "PLATFORM_LOGISTICS_SERVICE_INCOME"),
                Arguments.of("LOGISTICS_COLLECTION_SETTLED",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsCollectionSettled(in, 118L),
                        "PLATFORM_OPERATING_CASH", "SELLER_LOGISTICS_RECEIVABLE"),
                Arguments.of("LOGISTICS_TRUE_UP（實際 > 暫估）",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsTrueUp(in, 5L),
                        "SELLER_LOGISTICS_RECEIVABLE", "PLATFORM_LOGISTICS_SERVICE_INCOME"),
                Arguments.of("LOGISTICS_TRUE_UP（實際 < 暫估：方向相反）",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsTrueUp(in, -5L),
                        "PLATFORM_LOGISTICS_SERVICE_INCOME", "SELLER_LOGISTICS_RECEIVABLE"),
                Arguments.of("LOGISTICS_CREDIT_ISSUE",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsCreditIssue(in, 5L),
                        "PLATFORM_LOGISTICS_SERVICE_INCOME", "SELLER_LOGISTICS_CREDIT"),
                Arguments.of("LOGISTICS_CREDIT_APPLY",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsCreditApply(in, 5L),
                        "SELLER_LOGISTICS_CREDIT", "SELLER_LOGISTICS_RECEIVABLE"),
                Arguments.of("LOGISTICS_COST_ACCRUAL",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsCostAccrual(in, PROVIDER_REF, 120L),
                        "PLATFORM_LOGISTICS_COST", "PROVIDER_PAYABLE"),
                Arguments.of("PROVIDER_SETTLEMENT",
                        (Supplier<LedgerTxn>) () -> LogisticsLedgerBuilders.logisticsProviderSettlement(in, PROVIDER_REF, 120L),
                        "PROVIDER_PAYABLE", "PLATFORM_OPERATING_CASH"));
    }

    @ParameterizedTest(name = "{0}：DEBIT {2} ／ CREDIT {3}")
    @MethodSource("accountPairs")
    void buildersUseTheDocumentedAccountPair(String name, Supplier<LedgerTxn> build,
                                             String wantDebit, String wantCredit) {
        LedgerTxn txn = build.get();

        Map<String, String> byDirection = new LinkedHashMap<>();
        for (LedgerEntry e : txn.entries()) {
            byDirection.put(e.direction(), e.accountCode());
        }

        assertThat(txn.entries()).as("%s 必須恰好兩行", name).hasSize(2);
        assertThat(byDirection.get("DEBIT")).as("%s 的借方科目", name).isEqualTo(wantDebit);
        assertThat(byDirection.get("CREDIT")).as("%s 的貸方科目", name).isEqualTo(wantCredit);
        assertThat(txn.commerceModel()).as("%s 的商流", name).isEqualTo("SELLER_DIRECT");
        assertThat(txn.totalDebit()).as("%s 借貸必須相等", name).isEqualTo(txn.totalCredit());
    }

    @Test
    @DisplayName("每一種分錄都通過 validate（含互斥表與借貸平衡），且商流恆為 SELLER_DIRECT")
    void allBuildersProduceValidTxns() {
        Input in = new Input(STORE_ID, "OALL-2", "202609", "ADMIN", "ALL-2");

        List<Supplier<LedgerTxn>> builders = List.of(
                () -> LogisticsLedgerBuilders.logisticsReceivableAccrual(in, 120L),
                () -> LogisticsLedgerBuilders.logisticsCollectionSettled(in, 118L),
                () -> LogisticsLedgerBuilders.logisticsTrueUp(in, 5L),
                () -> LogisticsLedgerBuilders.logisticsCreditIssue(in, 5L),
                () -> LogisticsLedgerBuilders.logisticsCreditApply(in, 5L),
                () -> LogisticsLedgerBuilders.logisticsCostAccrual(in, PROVIDER_REF, 120L),
                () -> LogisticsLedgerBuilders.logisticsProviderSettlement(in, PROVIDER_REF, 120L));

        assertThat(builders).hasSize(7);
        for (Supplier<LedgerTxn> build : builders) {
            LedgerTxn txn = build.get();
            assertThatCode(txn::validate).doesNotThrowAnyException();
            assertThat(txn.commerceModel()).isEqualTo("SELLER_DIRECT");
        }
    }

    @Test
    @DisplayName("金額非正時每個 builder 都必須拒絕（0 元分錄會讓 chk_ledgentry_amount 在 INSERT 當下擋下）")
    void nonPositiveAmountsAreRejected() {
        Input in = adminInput("NEG-1");

        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsReceivableAccrual(in, 0L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsCollectionSettled(in, -1L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsCreditIssue(in, 0L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsCreditApply(in, 0L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsCostAccrual(in, PROVIDER_REF, 0L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsProviderSettlement(in, PROVIDER_REF, 0L))
                .isInstanceOf(LedgerValidationException.class);
    }

    @Test
    @DisplayName("PROVIDER 的 owner_ref_id 必須為正（chk_ledgentry_ref）")
    void providerRefMustBePositive() {
        Input in = adminInput("PRV-1");

        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsCostAccrual(in, 0L, 120L))
                .isInstanceOf(LedgerValidationException.class);
        assertThatThrownBy(() -> LogisticsLedgerBuilders.logisticsProviderSettlement(in, 0L, 120L))
                .isInstanceOf(LedgerValidationException.class);
    }
}
