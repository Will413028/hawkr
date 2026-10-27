package io.github.will413028.hawkr.payment.logistics.domain;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.github.will413028.hawkr.payment.logistics.domain.LedgerValidationException.Kind;
import java.util.ArrayList;
import java.util.List;
import java.util.function.Consumer;
import java.util.stream.Stream;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/**
 * {@link LedgerTxn} 的不變式測試（Go 版 {@code ledger_test.go} 的 1:1 移植）。
 *
 * <p>🔴 <b>期望值一律硬寫字面值</b>（科目名、商流名、金額），不從被測物或它引用的常數取 ——
 * 期望值跟著被測物一起變的話，這些斷言驗不到任何東西。
 */
class LedgerTxnTest {

    private static final long STORE_ID = 2685914682L;

    // ── 可變的測試規格（對應 Go 的 mutate func(*LedgerTxn)）────────────────────

    /** 可變的分錄規格；record 是不可變的，所以測試用這個做「改一欄」。 */
    private static final class EntrySpec {
        short lineSeq;
        String accountCode;
        String ownerType;
        long ownerRefId;
        String direction;
        long amount;

        EntrySpec(int lineSeq, String accountCode, String ownerType, long ownerRefId,
                  String direction, long amount) {
            this.lineSeq = (short) lineSeq;
            this.accountCode = accountCode;
            this.ownerType = ownerType;
            this.ownerRefId = ownerRefId;
            this.direction = direction;
            this.amount = amount;
        }

        LedgerEntry build() {
            return new LedgerEntry(lineSeq, accountCode, ownerType, ownerRefId, direction, amount, null);
        }
    }

    /** 可變的表頭規格。 */
    private static final class TxnSpec {
        String txnType = "ORDER_ACCRUAL";
        long storeId = STORE_ID;
        String orderNo = "O100000001";
        String commerceModel = "PLATFORM_CONSIGNMENT";
        String settlementPeriod = "202609";
        String sourceType = "ORDER";
        String sourceRef = "O100000001";
        List<EntrySpec> entries = orderAccrualEntries();

        LedgerTxn build() {
            List<LedgerEntry> es = new ArrayList<>(entries.size());
            for (EntrySpec e : entries) {
                es.add(e.build());
            }
            return new LedgerTxn(txnType, storeId, orderNo, commerceModel,
                    settlementPeriod, sourceType, sourceRef, es);
        }
    }

    /**
     * §5A.2 的 ORDER_ACCRUAL 五行分錄。
     *
     * <pre>
     * 商品本金 3000、運費 60、平台負擔折扣 200、賣家負擔折扣 100
     *   ⇒ 買家實付 3000+60−200−100 = 2760
     *   DEBIT  2760 + 200 + 100 = 3060
     *   CREDIT 3000 + 60        = 3060
     * </pre>
     */
    private static List<EntrySpec> orderAccrualEntries() {
        List<EntrySpec> es = new ArrayList<>();
        es.add(new EntrySpec(1, "PLATFORM_COLLECTED_CASH", "PLATFORM", 0L, "DEBIT", 2760L));
        es.add(new EntrySpec(2, "PLATFORM_DISCOUNT_SUBSIDY", "PLATFORM", 0L, "DEBIT", 200L));
        es.add(new EntrySpec(3, "SELLER_DISCOUNT_BORNE", "SELLER", STORE_ID, "DEBIT", 100L));
        es.add(new EntrySpec(4, "SELLER_PAYABLE", "SELLER", STORE_ID, "CREDIT", 3000L));
        es.add(new EntrySpec(5, "PLATFORM_LOGISTICS_RECOVERY", "PLATFORM", 0L, "CREDIT", 60L));
        return es;
    }

    private static LedgerTxn orderAccrualTxn() {
        return new TxnSpec().build();
    }

    private static LedgerTxn mutated(Consumer<TxnSpec> mutate) {
        TxnSpec spec = new TxnSpec();
        mutate.accept(spec);
        return spec.build();
    }

    // ── INV-L1 ────────────────────────────────────────────────────────────────

    @Test
    @DisplayName("平衡的 ORDER_ACCRUAL 通過驗證")
    void balancedOrderAccrualIsValid() {
        assertThatCode(() -> orderAccrualTxn().validate()).doesNotThrowAnyException();
        assertThat(orderAccrualTxn().totalDebit()).isEqualTo(3060L);
        assertThat(orderAccrualTxn().totalCredit()).isEqualTo(3060L);
    }

    @Test
    @DisplayName("🔴 INV-L1 鑑別力：逐行各加 1 元都必須被擋（只挑一行的話，「只檢查第一行」的實作也會全綠）")
    void unbalancedIsRejectedOnEveryLine() {
        int lines = orderAccrualEntries().size();
        assertThat(lines).isEqualTo(5);
        for (int i = 0; i < lines; i++) {
            final int idx = i;
            LedgerTxn txn = mutated(spec -> spec.entries.get(idx).amount += 1L);
            assertThatThrownBy(txn::validate)
                    .as("第 %d 行金額 +1 之後必須拒絕", idx + 1)
                    .isInstanceOfSatisfying(LedgerValidationException.class,
                            e -> assertThat(e.kind()).isEqualTo(Kind.UNBALANCED));
        }
    }

    @Test
    @DisplayName("借貸總額相同但方向全錯（全部改成 DEBIT）時不得放行")
    void allSameDirectionIsRejected() {
        LedgerTxn txn = mutated(spec -> spec.entries.forEach(e -> e.direction = "DEBIT"));
        assertThatThrownBy(txn::validate)
                .isInstanceOfSatisfying(LedgerValidationException.class,
                        e -> assertThat(e.kind()).isEqualTo(Kind.UNBALANCED));
    }

    // ── 形狀 ──────────────────────────────────────────────────────────────────

    static Stream<Arguments> invalidShapes() {
        return Stream.of(
                // 🔴 原本這一格斷言的是「非受託代銷一律不得進帳本」（INV-P8），2026-09-12
                // 依 ADR-010 改判：商流改成 SELLER_DIRECT 之後那條不變式讓整張帳本寫不進去。
                // 取代它的不是「什麼都能寫」，而是互斥 —— 代銷專屬的 txn_type 仍然寫不進去。
                Arguments.of("代銷專屬的 txn_type 不得在 SELLER_DIRECT 下寫入",
                        (Consumer<TxnSpec>) s -> s.commerceModel = "SELLER_DIRECT"),
                // 反向：只驗單向的話，一個「對所有組合都放行」的 bug 會有一半看不見。
                Arguments.of("物流財務的 txn_type 不得在 PLATFORM_CONSIGNMENT 下寫入",
                        (Consumer<TxnSpec>) s -> s.txnType = "LOGISTICS_RECEIVABLE_ACCRUAL"),
                Arguments.of("未知的 commerce_model 一律拒絕（放寬成值域不等於什麼都能寫）",
                        (Consumer<TxnSpec>) s -> s.commerceModel = "SOMETHING_ELSE"),
                Arguments.of("commerce_model 為 null",
                        (Consumer<TxnSpec>) s -> s.commerceModel = null),
                // INV-L3：每個減項指得回一張單據。
                Arguments.of("source_ref 為空",
                        (Consumer<TxnSpec>) s -> s.sourceRef = ""),
                // 🔴 舊值 REFUND_REVERSAL 已拆成 _GOODS／_SHIPPING；碼表裡沒有它。
                Arguments.of("txn_type 是已廢除的 REFUND_REVERSAL",
                        (Consumer<TxnSpec>) s -> s.txnType = "REFUND_REVERSAL"),
                Arguments.of("source_type 不在值域內",
                        (Consumer<TxnSpec>) s -> s.sourceType = "SYSTEM"),
                Arguments.of("settlement_period 不是 YYYYMM",
                        (Consumer<TxnSpec>) s -> s.settlementPeriod = "2026-09"),
                Arguments.of("store_id 非正",
                        (Consumer<TxnSpec>) s -> s.storeId = 0L),
                Arguments.of("只有一行分錄",
                        (Consumer<TxnSpec>) s -> s.entries = new ArrayList<>(s.entries.subList(0, 1))),
                Arguments.of("account_code 不在 CodeMeta 值域內",
                        (Consumer<TxnSpec>) s -> s.entries.get(0).accountCode = "PLATFORM_MYSTERY_CASH"),
                Arguments.of("owner_type 不在 CodeMeta 值域內",
                        (Consumer<TxnSpec>) s -> s.entries.get(0).ownerType = "BUYER"),
                // 與 chk_ledgentry_ref 同一條規則。
                Arguments.of("PLATFORM 的 owner_ref_id 不是 0",
                        (Consumer<TxnSpec>) s -> s.entries.get(0).ownerRefId = 12345L),
                Arguments.of("SELLER 的 owner_ref_id 是 0",
                        (Consumer<TxnSpec>) s -> s.entries.get(2).ownerRefId = 0L),
                // 金額恆為正、方向由 direction 表達。允許負數 ⇒ 同一筆金額有兩種寫法。
                Arguments.of("金額為負", (Consumer<TxnSpec>) s -> {
                    s.entries.get(0).amount = -2760L;
                    s.entries.get(3).amount = -3000L;
                }),
                Arguments.of("金額為 0（借貸仍平衡，所以這一格驗的是逐列檢查而不是 INV-L1）",
                        (Consumer<TxnSpec>) s -> {
                            s.entries.get(1).amount = 0L;
                            s.entries.get(4).amount = 0L;
                            s.entries.get(0).amount = 2860L;
                            s.entries.get(3).amount = 2960L;
                        }),
                Arguments.of("line_seq 小於 1",
                        (Consumer<TxnSpec>) s -> s.entries.get(0).lineSeq = 0),
                // uq_ledgentry_txn_seq 的應用層對應。
                Arguments.of("line_seq 重複",
                        (Consumer<TxnSpec>) s -> s.entries.get(1).lineSeq = 1),
                Arguments.of("direction 是未知值",
                        (Consumer<TxnSpec>) s -> s.entries.get(0).direction = "REVERSAL"));
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("invalidShapes")
    void invalidShapesAreRejected(String name, Consumer<TxnSpec> mutate) {
        LedgerTxn txn = mutated(mutate);
        assertThatThrownBy(txn::validate)
                .as(name)
                .isInstanceOfSatisfying(LedgerValidationException.class,
                        e -> assertThat(e.kind()).isEqualTo(Kind.INVALID));
    }

    @Test
    @DisplayName("PROVIDER 的 owner_ref_id 是 ShippingMethod 碼值，必須為正且合法")
    void providerOwnerIsAccepted() {
        LedgerTxn txn = new LedgerTxn(
                "LOGISTICS_COST_ACCRUAL", STORE_ID, "O100000001", "PLATFORM_CONSIGNMENT",
                "202609", "ORDER", "O100000001",
                List.of(
                        new LedgerEntry((short) 1, "PLATFORM_LOGISTICS_COST", "PLATFORM", 0L, "DEBIT", 85L, null),
                        new LedgerEntry((short) 2, "PROVIDER_PAYABLE", "PROVIDER", 3003L, "CREDIT", 85L, null)));

        assertThatCode(txn::validate).doesNotThrowAnyException();
    }

    @Test
    @DisplayName("idempotency_key ＝ {txn_type}:{source_type}:{source_ref}")
    void idempotencyKey() {
        assertThat(orderAccrualTxn().idempotencyKey()).isEqualTo("ORDER_ACCRUAL:ORDER:O100000001");
    }

    @Test
    @DisplayName("退款沖銷拆成 _GOODS／_SHIPPING 之後兩個新值都必須被接受")
    void refundReversalSplitIsAccepted() {
        // 少了任一個，運費退款就只能記進商品科目，而那是「賣家替一筆他從未收到的運費買單」。
        for (String txnType : List.of("REFUND_REVERSAL_GOODS", "REFUND_REVERSAL_SHIPPING")) {
            LedgerTxn txn = new LedgerTxn(
                    txnType, STORE_ID, "O100000001", "PLATFORM_CONSIGNMENT",
                    "202609", "REFUND", "R100000001",
                    List.of(
                            new LedgerEntry((short) 1, "SELLER_PAYABLE", "SELLER", STORE_ID, "DEBIT", 500L, null),
                            new LedgerEntry((short) 2, "PLATFORM_COLLECTED_CASH", "PLATFORM", 0L, "CREDIT", 500L, null)));
            assertThatCode(txn::validate).as(txnType).doesNotThrowAnyException();
        }
    }

    // ── ADR-010 互斥表 ─────────────────────────────────────────────────────────

    static Stream<Arguments> modelTxnTypeCompatibility() {
        return Stream.of(
                // 兩個商流都合法（物流成本與 provider 結算與商流無關）。
                Arguments.of("PLATFORM_CONSIGNMENT", "LOGISTICS_COST_ACCRUAL", true),
                Arguments.of("SELLER_DIRECT", "LOGISTICS_COST_ACCRUAL", true),
                Arguments.of("PLATFORM_CONSIGNMENT", "PROVIDER_SETTLEMENT", true),
                Arguments.of("SELLER_DIRECT", "PROVIDER_SETTLEMENT", true),
                // 代銷專屬 —— SELLER_DIRECT 下必須被擋（撥款鏈不得因放寬而復活）。
                Arguments.of("PLATFORM_CONSIGNMENT", "PAYOUT_SETTLE", true),
                Arguments.of("SELLER_DIRECT", "PAYOUT_SETTLE", false),
                Arguments.of("PLATFORM_CONSIGNMENT", "HOLD_APPLY", true),
                Arguments.of("SELLER_DIRECT", "HOLD_APPLY", false),
                Arguments.of("PLATFORM_CONSIGNMENT", "ORDER_ACCRUAL", true),
                Arguments.of("SELLER_DIRECT", "ORDER_ACCRUAL", false),
                // 物流財務專屬 —— 受託代銷下必須被擋。
                Arguments.of("SELLER_DIRECT", "LOGISTICS_RECEIVABLE_ACCRUAL", true),
                Arguments.of("PLATFORM_CONSIGNMENT", "LOGISTICS_RECEIVABLE_ACCRUAL", false),
                Arguments.of("SELLER_DIRECT", "LOGISTICS_COLLECTION_SETTLED", true),
                Arguments.of("PLATFORM_CONSIGNMENT", "LOGISTICS_COLLECTION_SETTLED", false),
                Arguments.of("SELLER_DIRECT", "LOGISTICS_CREDIT_ISSUE", true),
                Arguments.of("PLATFORM_CONSIGNMENT", "LOGISTICS_CREDIT_ISSUE", false),
                // MANUAL_ADJUSTMENT 兩邊都要有（ADM 可稽核調整與商流無關）。
                Arguments.of("PLATFORM_CONSIGNMENT", "MANUAL_ADJUSTMENT", true),
                Arguments.of("SELLER_DIRECT", "MANUAL_ADJUSTMENT", true));
    }

    /**
     * 🔴 陰性驗證是這一組的重點：只驗「合法組合會過」的話，一個「對所有組合都放行」的 bug
     * 會全綠 —— 而那正是「放寬 CHECK 卻忘了補相容性條件」的症狀。
     */
    @ParameterizedTest(name = "{0} × {1} ⇒ ok={2}")
    @MethodSource("modelTxnTypeCompatibility")
    void modelTxnTypeCompatibilityIsMutuallyExclusive(String model, String txnType, boolean ok) {
        if (ok) {
            assertThatCode(() -> LedgerTxn.validateModelTxnTypeCompatibility(model, txnType))
                    .as("%s × %s 應合法", model, txnType)
                    .doesNotThrowAnyException();
        } else {
            assertThatThrownBy(() -> LedgerTxn.validateModelTxnTypeCompatibility(model, txnType))
                    .as("%s × %s 應被拒；放行代表互斥失效，撥款專屬分錄會寫進新商流", model, txnType)
                    .isInstanceOf(LedgerValidationException.class);
        }
    }

    @Test
    @DisplayName("不平衡的例外帶業務碼 9566（LEDGER_TRANSACTION_UNBALANCED）")
    void unbalancedCarriesBizCode9566() {
        LedgerTxn txn = mutated(spec -> spec.entries.get(0).amount += 1L);
        assertThatThrownBy(txn::validate)
                .isInstanceOfSatisfying(LedgerValidationException.class,
                        e -> assertThat(e.code()).isEqualTo(9566));
    }
}
