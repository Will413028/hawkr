package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.LedgerOwnerType;
import io.github.will413028.hawkr.common.utils.code.LedgerTxnType;
import java.util.List;

/**
 * 70 §3.8 分錄對照表（{@code SELLER_DIRECT} 分支）的程式形式。
 *
 * <h2>🔴 為什麼要有它</h2>
 * {@code ledger_entries} 有 {@code CONSTRAINT TRIGGER trg_ledger_balanced}
 * （Σ DEBIT = Σ CREDIT，{@code DEFERRABLE INITIALLY DEFERRED}）⇒ 科目對猜錯就是
 * <b>COMMIT 當下 500</b>，而症狀會出現在呼叫端（收款流程／帳單匯入），看起來像那條路徑壞了。
 * 把七種分錄寫成具名建構式，讓「這一筆該記哪兩個科目」只有一個答案。
 *
 * <p>⚠️ <b>同一個 {@code txn_type} 在兩個商流下是同名的兩種分錄</b>：科目對由
 * {@code commerce_model} 決定，不由 {@code txn_type} 單獨決定。本類只產出
 * {@code SELLER_DIRECT} 那一側。
 */
public final class LogisticsLedgerBuilders {

    private LogisticsLedgerBuilders() {
    }

    /**
     * 七種分錄共用的識別資料。
     *
     * @param storeId          store surrogate
     * @param orderNo          訂單編號，可為 null
     * @param settlementPeriod {@code 'YYYYMM'}
     * @param sourceType       {@code ORDER} / {@code REFUND} / {@code PAYOUT} / {@code ADMIN}
     * @param sourceRef        指得回的單據編號（INV-L3）
     */
    public record Input(
            long storeId,
            String orderNo,
            String settlementPeriod,
            String sourceType,
            String sourceRef) {
    }

    /**
     * 物流應收認列。
     *
     * <pre>DEBIT  SELLER_LOGISTICS_RECEIVABLE ／ CREDIT PLATFORM_LOGISTICS_SERVICE_INCOME</pre>
     *
     * <p>🔴 <b>金額 ＝ 本期成本 − 平台補貼，不含前期結轉</b>：結轉是前期已認列、尚未收足的餘額，
     * 再認列一次就是<b>重複認列收入</b> —— 帳上多一筆不存在的收入，而現金並沒有多進來。
     */
    public static LedgerTxn logisticsReceivableAccrual(Input in, long accrual) {
        requirePositive(accrual, "應收認列金額");
        return build(in, LedgerTxnType.LOGISTICS_RECEIVABLE_ACCRUAL.code(),
                LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE.code(), seller(in),
                LedgerAccountCode.PLATFORM_LOGISTICS_SERVICE_INCOME.code(), PLATFORM,
                accrual, "物流應收認列（本期成本 − 平台補貼）");
    }

    /**
     * 物流收款入帳。
     *
     * <pre>DEBIT  PLATFORM_OPERATING_CASH ／ CREDIT SELLER_LOGISTICS_RECEIVABLE</pre>
     *
     * <p>🔴 <b>金額 ＝ netProceeds（gross − actualFee），不是 gross</b>：手續費是 PayUni
     * 從撥款裡扣掉的，平台從來沒收到那筆錢。用 gross 會讓帳上的現金比銀行多。
     *
     * <p>🔴 <b>未收足的部分不需要任何分錄</b>：它自然留在
     * {@code SELLER_LOGISTICS_RECEIVABLE} 的餘額裡，那就是 ADR-011 要結轉、下一期併收的東西。
     * ⚠️ 照直覺補一筆「DEBIT 手續費費用／CREDIT 應收」會把餘額沖平 ⇒ <b>結轉機制在帳上失效</b>；
     * 而且同時違反 ADR-005（把賣家該負擔的手續費認列成平台自己的費用）。
     */
    public static LedgerTxn logisticsCollectionSettled(Input in, long netProceeds) {
        requirePositive(netProceeds, "實收金額");
        return build(in, LedgerTxnType.LOGISTICS_COLLECTION_SETTLED.code(),
                LedgerAccountCode.PLATFORM_OPERATING_CASH.code(), PLATFORM,
                LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE.code(), seller(in),
                netProceeds, "平台實收（gross − actual_fee）");
    }

    /**
     * 暫估 → 實際的<b>雙向</b>差額結算。
     *
     * <p>{@code diff > 0}（實際 &gt; 暫估）⇒ DEBIT 應收／CREDIT 收入（補收）；
     * {@code diff < 0}（實際 &lt; 暫估）⇒ 🔴 <b>方向相反，同一個 txn_type</b>。
     */
    public static LedgerTxn logisticsTrueUp(Input in, long diff) {
        if (diff == 0L) {
            throw LedgerValidationException.invalid("true-up 差額為 0，不該產生分錄");
        }
        String txnType = LedgerTxnType.LOGISTICS_TRUE_UP.code();
        if (diff > 0L) {
            return build(in, txnType,
                    LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE.code(), seller(in),
                    LedgerAccountCode.PLATFORM_LOGISTICS_SERVICE_INCOME.code(), PLATFORM,
                    diff, "true-up 補收（實際 > 暫估）");
        }
        return build(in, txnType,
                LedgerAccountCode.PLATFORM_LOGISTICS_SERVICE_INCOME.code(), PLATFORM,
                LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE.code(), seller(in),
                -diff, "true-up 退還（實際 < 暫估）");
    }

    /**
     * 物流貸記發生（多收成立貸記）。
     *
     * <pre>DEBIT  PLATFORM_LOGISTICS_SERVICE_INCOME ／ CREDIT SELLER_LOGISTICS_CREDIT</pre>
     *
     * <p>⚠️ <b>借「收入」而不是借「費用」</b>：多收的那一筆當初是貸記收入，現在要還
     * ⇒ <b>沖減當初認列的收入</b>。記成費用會讓損益表上同時有「虛增的收入」與
     * 「不存在的費用」，兩者金額相同但都是假的。
     */
    public static LedgerTxn logisticsCreditIssue(Input in, long amount) {
        requirePositive(amount, "貸記金額");
        return build(in, LedgerTxnType.LOGISTICS_CREDIT_ISSUE.code(),
                LedgerAccountCode.PLATFORM_LOGISTICS_SERVICE_INCOME.code(), PLATFORM,
                LedgerAccountCode.SELLER_LOGISTICS_CREDIT.code(), seller(in),
                amount, "貸記發生（平台欠賣家）");
    }

    /**
     * 物流貸記抵扣下一筆應收。
     *
     * <pre>DEBIT  SELLER_LOGISTICS_CREDIT ／ CREDIT SELLER_LOGISTICS_RECEIVABLE</pre>
     *
     * <p>⚠️ 貸記<b>優先抵扣</b>、現金返還為後備（抵扣不經通道 ⇒ 零通道成本），
     * 🔴 但<b>不得因成本而拒絕返還</b>（ADR-008 第 4 條）。
     */
    public static LedgerTxn logisticsCreditApply(Input in, long amount) {
        requirePositive(amount, "貸記抵扣金額");
        return build(in, LedgerTxnType.LOGISTICS_CREDIT_APPLY.code(),
                LedgerAccountCode.SELLER_LOGISTICS_CREDIT.code(), seller(in),
                LedgerAccountCode.SELLER_LOGISTICS_RECEIVABLE.code(), seller(in),
                amount, "貸記抵扣應收");
    }

    /**
     * 物流成本應計（暫估）。
     *
     * <pre>DEBIT  PLATFORM_LOGISTICS_COST ／ CREDIT PROVIDER_PAYABLE</pre>
     *
     * @param providerRefId ShippingMethod 碼值的數值形式
     *                      （{@code chk_ledgentry_ref}：PROVIDER ⇒ ref &gt; 0）
     */
    public static LedgerTxn logisticsCostAccrual(Input in, long providerRefId, long cost) {
        requirePositive(cost, "物流成本");
        return build(in, LedgerTxnType.LOGISTICS_COST_ACCRUAL.code(),
                LedgerAccountCode.PLATFORM_LOGISTICS_COST.code(), PLATFORM,
                LedgerAccountCode.PROVIDER_PAYABLE.code(),
                new Owner(LedgerOwnerType.PROVIDER.code(), providerRefId),
                cost, "平台物流成本（暫估）");
    }

    /**
     * 月結付款給 provider。
     *
     * <pre>DEBIT  PROVIDER_PAYABLE ／ CREDIT PLATFORM_OPERATING_CASH</pre>
     *
     * <p>🔴 <b>CREDIT 與舊商流不同，這一格最容易抄錯</b>：65 寫的是
     * {@code PLATFORM_COLLECTED_CASH}（平台<b>代理商戶收到買家款</b>），而
     * {@code SELLER_DIRECT} 下平台<b>沒有代收</b> ⇒ 該科目在新商流裡沒有語意
     * ⇒ 對手科目必須是 {@code PLATFORM_OPERATING_CASH}。
     * ⚠️ 抄錯的後果不是「記到別的科目」，而是 {@code trg_ledger_balanced} 放行、
     * 但 {@code chk_ledgentry_owner} 或稽核在事後才發現兩套帳被混算。
     */
    public static LedgerTxn logisticsProviderSettlement(Input in, long providerRefId, long amount) {
        requirePositive(amount, "provider 結算金額");
        return build(in, LedgerTxnType.PROVIDER_SETTLEMENT.code(),
                LedgerAccountCode.PROVIDER_PAYABLE.code(),
                new Owner(LedgerOwnerType.PROVIDER.code(), providerRefId),
                LedgerAccountCode.PLATFORM_OPERATING_CASH.code(), PLATFORM,
                amount, "月結付款給 provider");
    }

    // ── 內部 ────────────────────────────────────────────────────────────────

    /** 分錄持有人（type ＋ ref）。 */
    private record Owner(String type, long refId) {
    }

    private static final Owner PLATFORM = new Owner(LedgerOwnerType.PLATFORM.code(), 0L);

    private static Owner seller(Input in) {
        return new Owner(LedgerOwnerType.SELLER.code(), in.storeId());
    }

    private static void requirePositive(long amount, String what) {
        if (amount <= 0L) {
            throw LedgerValidationException.invalid("%s必須為正，實得 %d", what, amount);
        }
    }

    /**
     * 組出兩行分錄的交易並<b>立刻</b> {@link LedgerTxn#validate()}。
     *
     * <p>🔴 驗證不是選配：不驗就交出去的話，錯誤會延後到 COMMIT 當下的
     * {@code trg_ledger_balanced}，而那時的堆疊裡已經沒有「是誰組錯了科目」的線索。
     */
    private static LedgerTxn build(Input in, String txnType,
                                   String debitAccount, Owner debitOwner,
                                   String creditAccount, Owner creditOwner,
                                   long amount, String memo) {
        LedgerTxn txn = new LedgerTxn(
                txnType,
                in.storeId(),
                in.orderNo(),
                // 本類只產出 SELLER_DIRECT 那一側；舊商流的同名分錄科目對不同。
                CommerceModel.SELLER_DIRECT.code(),
                in.settlementPeriod(),
                in.sourceType(),
                in.sourceRef(),
                List.of(
                        LedgerEntry.debit(1, debitAccount, debitOwner.type(), debitOwner.refId(),
                                amount, memo),
                        LedgerEntry.credit(2, creditAccount, creditOwner.type(), creditOwner.refId(),
                                amount, memo)));
        txn.validate();
        return txn;
    }
}
