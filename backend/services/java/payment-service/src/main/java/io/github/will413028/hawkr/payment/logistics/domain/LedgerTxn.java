package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.LedgerTxnType;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * 一筆帳本交易（表頭 ＋ 全部分錄），對應 {@code ledger_transactions} ＋ {@code ledger_entries}。
 *
 * <p>🔴 寫入只有一支函式（repository 的 post），且必須在業務 tx 內；
 * 本型別只負責「這筆交易長得對不對」，不碰 DB。
 *
 * @param txnType          CodeMeta {@link LedgerTxnType} 的 code
 * @param storeId          store surrogate（非 account_id）
 * @param orderNo          可為 null／空（{@code MANUAL_ADJUSTMENT}）
 * @param commerceModel    CodeMeta {@link CommerceModel} 的 code；ADR-010 的互斥表以它為索引
 * @param settlementPeriod {@code 'YYYYMM'}（UTC 曆月）
 * @param sourceType       {@code ORDER} / {@code REFUND} / {@code PAYOUT} / {@code ADMIN}
 * @param sourceRef        INV-L3 的「指得回一張單據」，不得為空
 * @param entries          分錄，至少兩列
 */
public record LedgerTxn(
        String txnType,
        long storeId,
        String orderNo,
        String commerceModel,
        String settlementPeriod,
        String sourceType,
        String sourceRef,
        List<LedgerEntry> entries) {

    public LedgerTxn {
        // 防禦性複製：帳本交易一旦建好就不該被呼叫端從外面改掉。
        // 不用 List.copyOf —— 它對 null 元素直接 NPE，會把「分錄寫錯」變成一個
        // 與帳務語意無關的錯誤訊息；留給 validate() 用業務語言講。
        entries = entries == null ? null : Collections.unmodifiableList(new ArrayList<>(entries));
    }

    /** 合法的 source_type（與 {@code chk_ledgtxn_source} 同一份清單）。 */
    private static final Set<String> SOURCE_TYPES =
            Collections.unmodifiableSet(new HashSet<>(List.of("ORDER", "REFUND", "PAYOUT", "ADMIN")));

    /**
     * 兩套商流各自合法的 txn_type（ADR-010；與 DB 的 {@code chk_ledgtxn_model_type} 是同一份清單）。
     *
     * <p>🔴 <b>{@code LOGISTICS_COST_ACCRUAL} 與 {@code PROVIDER_SETTLEMENT} 兩邊都有，那不是筆誤</b>：
     * 物流成本這件事本身與商流無關（兩種商流都會付錢給 provider），差別只在<b>對手科目</b> ——
     * 舊商流是 {@code PLATFORM_COLLECTED_CASH}（平台代收），新商流是
     * {@code PLATFORM_OPERATING_CASH}（{@code SELLER_DIRECT} 下平台沒有代收，前者無語意）。
     * ⇒ txn_type 的歸屬看「這個動作在哪個商流下發生」，不看「它是誰引入的」。
     *
     * <p>⚠️ 這份清單與 {@code V9__add_logistics_finance_tables.sql} 的
     * {@code chk_ledgtxn_model_type} <b>必須逐字一致</b>：兩邊漂移時，domain 放行而 DB 擋下
     * ⇒ 症狀是呼叫端 500 而不是「這筆不該寫」。
     */
    private static final Map<String, Set<String>> MODEL_TXN_TYPES = buildModelTxnTypes();

    private static Map<String, Set<String>> buildModelTxnTypes() {
        Map<String, Set<String>> m = new HashMap<>();
        m.put(CommerceModel.PLATFORM_CONSIGNMENT.code(), Set.of(
                "ORDER_ACCRUAL", "REFUND_REVERSAL_GOODS", "REFUND_REVERSAL_SHIPPING",
                "PAYOUT_FEE", "PLATFORM_SERVICE_FEE", "HOLD_APPLY", "HOLD_RELEASE",
                "PAYOUT_SETTLE", "MANUAL_ADJUSTMENT",
                "LOGISTICS_COST_ACCRUAL", "PROVIDER_SETTLEMENT"));
        m.put(CommerceModel.SELLER_DIRECT.code(), Set.of(
                "LOGISTICS_COST_ACCRUAL", "PROVIDER_SETTLEMENT",
                "LOGISTICS_RECEIVABLE_ACCRUAL", "LOGISTICS_COLLECTION_SETTLED",
                "LOGISTICS_TRUE_UP", "LOGISTICS_CREDIT_ISSUE", "LOGISTICS_CREDIT_APPLY",
                "COLLECTION_FEE_ACCRUAL", "MANUAL_ADJUSTMENT"));
        return Collections.unmodifiableMap(m);
    }

    /**
     * 守住「代銷專屬的 txn_type 不得在 {@code SELLER_DIRECT} 下寫入」，反向亦然。
     *
     * <p>🔴 它<b>不取代</b>撥款的 G0 軸守門（{@code PayoutEligibility} 的 commerce_model 判定）：
     * 兩道防線方向一致、不互相取代 —— 前者擋「寫進帳本」，後者擋「發動撥款」。
     *
     * @throws LedgerValidationException 商流未知，或 txn_type 不屬於該商流
     */
    static void validateModelTxnTypeCompatibility(String commerceModel, String txnType) {
        Set<String> allowed = commerceModel == null ? null : MODEL_TXN_TYPES.get(commerceModel);
        if (allowed == null) {
            throw LedgerValidationException.invalid("未知的 commerce_model %s", commerceModel);
        }
        if (txnType == null || !allowed.contains(txnType)) {
            throw LedgerValidationException.invalid(
                    "txn_type %s 不屬於 commerce_model %s（ADR-010 互斥表）", txnType, commerceModel);
        }
    }

    /**
     * {@code ledger_transactions.idempotency_key} 的組成。
     *
     * <p>⚠️ 寫入一律 {@code ON CONFLICT DO NOTHING}，不得靠「讓 DB 拋唯一鍵錯、catch 之後繼續查」
     * —— PostgreSQL 交易內任一句失敗會讓整個交易 aborted（25P02），catch 得到但下一句
     * SELECT 必炸成 500。
     */
    public String idempotencyKey() {
        return txnType + ":" + sourceType + ":" + sourceRef;
    }

    /**
     * 檢查一筆帳本交易是否可以落庫。
     *
     * <p>它守的是 INV-L1（Σ DEBIT ＝ Σ CREDIT）／INV-L2（整數元）／INV-L3（source_ref
     * 指得回單據）／ADR-010（商流 × txn_type 互斥）。
     *
     * <p>⚠️ <b>這不取代 {@code trg_ledger_balanced}</b>：DB 的 deferred constraint trigger
     * 是唯一擋得住「繞過本方法的寫入路徑」的東西（維運 SQL、未來的第二條 code path）。
     * 兩道都要 —— 本方法讓錯誤在寫入前就帶著業務語意現形，trigger 讓它在任何情況下都不會落庫。
     *
     * @throws LedgerValidationException 不可落庫；{@link LedgerValidationException#kind()} 分辨原因
     */
    public void validate() {
        // 🔴 ADR-010：商流是值域不是單值，互斥由 txn_type × commerce_model 相容表守。
        // 放寬而不補相容性檢查就退化成「什麼都能寫」。
        validateModelTxnTypeCompatibility(commerceModel, txnType);

        if (!LedgerTxnType.isValidCode(txnType)) {
            throw LedgerValidationException.invalid("未知的 txn_type %s", txnType);
        }
        if (sourceType == null || !SOURCE_TYPES.contains(sourceType)) {
            throw LedgerValidationException.invalid("未知的 source_type %s", sourceType);
        }
        // INV-L3：每個減項都要指得回一張單據。
        if (sourceRef == null || sourceRef.isBlank()) {
            throw LedgerValidationException.invalid("source_ref 不得為空（INV-L3）");
        }
        if (storeId <= 0L) {
            throw LedgerValidationException.invalid("store_id 必須為正，實得 %d", storeId);
        }
        if (!isSettlementPeriod(settlementPeriod)) {
            throw LedgerValidationException.invalid(
                    "settlement_period %s 不是 YYYYMM", settlementPeriod);
        }
        // 一筆交易至少要有借貸兩行；單行的「交易」不可能平衡。
        if (entries == null || entries.size() < 2) {
            throw LedgerValidationException.invalid(
                    "分錄至少兩行，實得 %d", entries == null ? 0 : entries.size());
        }

        long debit = 0L;
        long credit = 0L;
        Set<Short> seen = new HashSet<>(entries.size());
        for (int i = 0; i < entries.size(); i++) {
            LedgerEntry e = entries.get(i);
            if (e == null) {
                throw LedgerValidationException.invalid("entries[%d] 為 null", i);
            }
            e.validate(i);

            if (!seen.add(e.lineSeq())) {
                throw LedgerValidationException.invalid("line_seq %d 重複", e.lineSeq());
            }

            if (e.isDebit()) {
                if (debit > Long.MAX_VALUE - e.amount()) {
                    throw LedgerValidationException.overflow("借方合計溢位");
                }
                debit += e.amount();
            } else {
                if (credit > Long.MAX_VALUE - e.amount()) {
                    throw LedgerValidationException.overflow("貸方合計溢位");
                }
                credit += e.amount();
            }
        }

        // INV-L1。
        if (debit != credit) {
            throw LedgerValidationException.unbalanced(debit, credit);
        }
    }

    /** 借方合計；只在已通過 {@link #validate()} 的交易上有意義。 */
    public long totalDebit() {
        return entries.stream().filter(LedgerEntry::isDebit).mapToLong(LedgerEntry::amount).sum();
    }

    /** 貸方合計。 */
    public long totalCredit() {
        return entries.stream().filter(LedgerEntry::isCredit).mapToLong(LedgerEntry::amount).sum();
    }

    /** 檢查 {@code 'YYYYMM'}（與 {@code chk_ledgtxn_period} 的 regex 同義）。 */
    private static boolean isSettlementPeriod(String s) {
        if (s == null || s.length() != 6) {
            return false;
        }
        for (int i = 0; i < 6; i++) {
            char c = s.charAt(i);
            if (c < '0' || c > '9') {
                return false;
            }
        }
        return true;
    }
}
