package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.ProviderFeeType;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerEntry;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsAdjustmentRepository;
import io.github.will413028.hawkr.payment.logistics.repository.ProviderSettlementLineRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicLong;

/**
 * W4（對帳 true-up）測試用的替身。
 *
 * <h2>🔴 這些替身<b>真的維護狀態，而且鏡射 V9 的述詞</b></h2>
 * 只記 {@code calls} 的替身在本軸上<b>零鑑別力</b>：它對任何寫入都回「成功」，
 * 於是「帳單列沒被推走」「調整單金額是 0」「MATCHED 卻指不出分錄」這種 bug 會全綠，
 * 而它們在真實 DB 上的症狀分別是 <b>帳單列永遠停在 UNMATCHED</b>、
 * {@code chk_logadj_amount_nonzero} 的 23514，與 {@code chk_provsettle_ledger} 的 23514。
 *
 * <p>逐項鏡射的述詞（每一條都指得回 V9 或 mapper 的原句）：
 * <ul>
 *   <li>{@code uq_provsettle_line} —— 🔴 <b>含 {@code provider_shipment_ref}，而 PostgreSQL 的
 *       NULL 互不相等 ⇒ 彙總列（該欄 NULL）不會被去重</b>。這是本檔最重要的一條：
 *       照抄「UNIQUE 就會去重」的替身會讓匯入層的彙總列閘門測不出來</li>
 *   <li>{@code chk_provsettle_ledger} —— {@code MATCHED} ⇔ {@code ledger_txn_no} 非 NULL（雙向）</li>
 *   <li>{@code markMatched}／{@code markUnmatchedOutcome} 的 {@code AND recon_status IN (allowed_from)}</li>
 *   <li>{@code chk_logadj_amount_nonzero}／{@code chk_logadj_kind}／
 *       {@code chk_logadj_responsibility_needs_cause}／{@code chk_logadj_cause}／{@code chk_logadj_reason}</li>
 *   <li>{@code uq_exc_open_dedup} —— partial unique，只在 {@code OPEN}／{@code IN_PROGRESS} 之間去重</li>
 *   <li>{@code chk_exc_owner} —— {@code OPEN} ⇒ {@code owner_account_id} 必須為 NULL</li>
 *   <li>{@code chk_exc_detail_object}／{@code chk_exc_actions_array} —— JSONB 型別</li>
 * </ul>
 *
 * <p>⚠️ <b>不鏡射什麼（誠實標註）</b>：這裡沒有交易。{@code LogisticsReconciliationService}
 * 在單元測試裡是直接 new 出來的，<b>沒有 Spring proxy ⇒ {@code @Transactional} 不生效</b>
 * ⇒ 「任一步失敗整組 rollback」本檔<b>驗不到</b>。本檔改驗一件更弱但確實成立的事：
 * 失敗的守衛擺在副作用之前。真正的 rollback 需要帶 DB 的整合測試。
 */
public final class LogisticsReconciliationFakes {

    private LogisticsReconciliationFakes() {
    }

    // =========================================================================

    public static final class FakeSettlementLines extends ProviderSettlementLineRepository {

        private final Map<Long, ProviderSettlementLineEntity> rows = new LinkedHashMap<>();

        public FakeSettlementLines() {
            super(null);
        }

        /**
         * 鏡射 {@code uq_provsettle_line}
         * （{@code provider, statement_ref, provider_shipment_ref, fee_type}）。
         *
         * <p>🔴 {@code provider_shipment_ref} 為 NULL 的列<b>永遠不會衝突</b>——
         * PostgreSQL 的 NULL 互不相等。這不是替身偷懶，這正是真實行為，
         * 也正是匯入層必須自己擋彙總列的理由。
         */
        @Override
        public boolean importIfAbsent(ProviderSettlementLineEntity e) {
            if (e.getAmount() == null || e.getAmount() <= 0L) {
                throw new IllegalStateException("chk_provsettle_amount 違反：amount=" + e.getAmount());
            }
            if (!ProviderFeeType.isValidCode(e.getFeeType())) {
                throw new IllegalStateException("chk_provsettle_fee_type 違反：" + e.getFeeType());
            }
            requireLedgerPair(e.getReconStatus(), e.getLedgerTxnNo());
            if (e.getProviderShipmentRef() != null) {
                for (ProviderSettlementLineEntity row : rows.values()) {
                    if (row.getProvider().equals(e.getProvider())
                            && row.getStatementRef().equals(e.getStatementRef())
                            && e.getProviderShipmentRef().equals(row.getProviderShipmentRef())
                            && row.getFeeType().equals(e.getFeeType())) {
                        return false;
                    }
                }
            }
            rows.put(e.getId(), copy(e));
            return true;
        }

        @Override
        public Optional<ProviderSettlementLineEntity> findById(long id) {
            return Optional.ofNullable(rows.get(id)).map(FakeSettlementLines::copy);
        }

        @Override
        public List<ProviderSettlementLineEntity> listByStatementRef(String provider, String statementRef) {
            List<ProviderSettlementLineEntity> out = new ArrayList<>();
            for (ProviderSettlementLineEntity row : rows.values()) {
                if (row.getProvider().equals(provider) && row.getStatementRef().equals(statementRef)) {
                    out.add(copy(row));
                }
            }
            return out;
        }

        @Override
        public List<ProviderSettlementLineEntity> listByReconStatus(List<String> statuses, int limit) {
            List<ProviderSettlementLineEntity> out = new ArrayList<>();
            for (ProviderSettlementLineEntity row : rows.values()) {
                if (out.size() >= limit) {
                    break;
                }
                if (statuses.contains(row.getReconStatus())) {
                    out.add(copy(row));
                }
            }
            return out;
        }

        /** 🔴 CAS ＋ {@code chk_provsettle_ledger}（MATCHED ⇔ ledger_txn_no 非 NULL）。 */
        @Override
        public boolean markMatched(long id, String matchedStatus, String ledgerTxnNo,
                                   List<String> allowedFrom) {
            ProviderSettlementLineEntity row = rows.get(id);
            if (row == null || !allowedFrom.contains(row.getReconStatus())) {
                return false;
            }
            requireLedgerPair(matchedStatus, ledgerTxnNo);
            row.setReconStatus(matchedStatus);
            row.setLedgerTxnNo(ledgerTxnNo);
            return true;
        }

        /** 🔴 離開 MATCHED 時 {@code ledger_txn_no} 必須一起清空（同一條雙向等式）。 */
        @Override
        public boolean markUnmatchedOutcome(long id, String toStatus, List<String> allowedFrom) {
            ProviderSettlementLineEntity row = rows.get(id);
            if (row == null || !allowedFrom.contains(row.getReconStatus())) {
                return false;
            }
            requireLedgerPair(toStatus, null);
            row.setReconStatus(toStatus);
            row.setLedgerTxnNo(null);
            return true;
        }

        @Override
        public boolean bindExceptionCase(long id, long exceptionCaseId) {
            ProviderSettlementLineEntity row = rows.get(id);
            if (row == null || row.getExceptionCaseId() != null) {
                return false;
            }
            row.setExceptionCaseId(exceptionCaseId);
            return true;
        }

        public String reconStatusOf(long id) {
            return rows.get(id).getReconStatus();
        }

        public String ledgerTxnNoOf(long id) {
            return rows.get(id).getLedgerTxnNo();
        }

        public int size() {
            return rows.size();
        }

        /** 測試專用：直接塞一列（繞過匯入），用來造「已經 MATCHED 的列」等前置狀態。 */
        public void seed(ProviderSettlementLineEntity e) {
            rows.put(e.getId(), copy(e));
        }

        private static void requireLedgerPair(String reconStatus, String ledgerTxnNo) {
            boolean matched = "MATCHED".equals(reconStatus);
            if (matched != (ledgerTxnNo != null)) {
                throw new IllegalStateException(
                        "chk_provsettle_ledger 違反：recon_status=" + reconStatus
                                + " ledger_txn_no=" + ledgerTxnNo);
            }
        }

        private static ProviderSettlementLineEntity copy(ProviderSettlementLineEntity s) {
            ProviderSettlementLineEntity c = new ProviderSettlementLineEntity();
            c.setId(s.getId());
            c.setProvider(s.getProvider());
            c.setStatementRef(s.getStatementRef());
            c.setStatementPeriod(s.getStatementPeriod());
            c.setProviderShipmentRef(s.getProviderShipmentRef());
            c.setOrderNo(s.getOrderNo());
            c.setFeeType(s.getFeeType());
            c.setAmount(s.getAmount());
            c.setReconStatus(s.getReconStatus());
            c.setLedgerTxnNo(s.getLedgerTxnNo());
            c.setExceptionCaseId(s.getExceptionCaseId());
            c.setImportedBy(s.getImportedBy());
            c.setImportedAt(s.getImportedAt());
            return c;
        }
    }

    // =========================================================================

    public static final class FakeAdjustments extends LogisticsAdjustmentRepository {

        private final List<LogisticsAdjustmentEntity> rows = new ArrayList<>();

        public FakeAdjustments() {
            super(null);
        }

        /** 🔴 鏡射 V9 對 {@code logistics_adjustments} 的五條 CHECK。 */
        @Override
        public void create(LogisticsAdjustmentEntity e) {
            if (e.getAmount() == null || e.getAmount() == 0L) {
                throw new IllegalStateException("chk_logadj_amount_nonzero 違反：amount=" + e.getAmount());
            }
            if (!List.of("RESPONSIBILITY", "RECONCILIATION").contains(e.getKind())) {
                throw new IllegalStateException("chk_logadj_kind 違反：" + e.getKind());
            }
            if ("RESPONSIBILITY".equals(e.getKind()) && e.getResponsibilityCause() == null) {
                throw new IllegalStateException("chk_logadj_responsibility_needs_cause 違反");
            }
            if (e.getResponsibilityCause() != null
                    && !ResponsibilityCause.isValidValue(e.getResponsibilityCause())) {
                throw new IllegalStateException("chk_logadj_cause 違反：" + e.getResponsibilityCause());
            }
            if (e.getReason() == null || e.getReason().isEmpty()) {
                throw new IllegalStateException("chk_logadj_reason 違反：reason 長度必須 >= 1");
            }
            if (e.getStoreId() == null || e.getStoreId() <= 0L) {
                throw new IllegalStateException("chk_logadj_store 違反：store_id=" + e.getStoreId());
            }
            rows.add(e);
        }

        @Override
        public List<LogisticsAdjustmentEntity> listByReceivable(long receivableId) {
            return rows.stream().filter(r -> r.getReceivableId() != null
                    && r.getReceivableId() == receivableId).toList();
        }

        @Override
        public List<LogisticsAdjustmentEntity> listByStatementLine(long statementLineId) {
            return rows.stream().filter(r -> r.getStatementLineId() != null
                    && r.getStatementLineId() == statementLineId).toList();
        }

        public List<LogisticsAdjustmentEntity> all() {
            return List.copyOf(rows);
        }

        /** 所有調整單金額的合計 —— 「平台吸收」那一列有沒有真的相消，看這個數字。 */
        public long amountSum() {
            return rows.stream().mapToLong(LogisticsAdjustmentEntity::getAmount).sum();
        }
    }

    // =========================================================================

    public static final class FakeCases extends FulfillmentExceptionCaseRepository {

        private final List<FulfillmentExceptionCaseEntity> rows = new ArrayList<>();
        private final Set<String> openDedupKeys = new LinkedHashSet<>();
        private final AtomicLong seq = new AtomicLong(1);

        public FakeCases() {
            super(null);
        }

        @Override
        public String nextCaseNo() {
            return "EX20260912" + String.format("%08d", seq.getAndIncrement());
        }

        /** 🔴 鏡射 {@code uq_exc_open_dedup}（partial）＋ {@code chk_exc_*}。 */
        @Override
        public boolean openIfAbsent(FulfillmentExceptionCaseEntity e) {
            if (!List.of("shipping", "order", "payment").contains(e.getSourceService())) {
                throw new IllegalStateException("chk_exc_source 違反：" + e.getSourceService());
            }
            if (!List.of("P0", "P1", "P2").contains(e.getSeverity())) {
                throw new IllegalStateException("chk_exc_severity 違反：" + e.getSeverity());
            }
            if (!List.of("ORDER", "SHIPMENT", "SHIPMENT_BATCH", "CONFIG").contains(e.getSubjectType())) {
                throw new IllegalStateException("chk_exc_subject 違反：" + e.getSubjectType());
            }
            if (!"OPEN".equals(e.getStatus())) {
                throw new IllegalStateException("開案的 status 只能是 OPEN，實得 " + e.getStatus());
            }
            // 🔴 chk_exc_owner：OPEN ⇒ owner_account_id 必須為 NULL。
            if (e.getOwnerAccountId() != null) {
                throw new IllegalStateException("chk_exc_owner 違反：OPEN 不得有承辦人");
            }
            if (e.getDetail() == null || !e.getDetail().startsWith("{")) {
                throw new IllegalStateException("chk_exc_detail_object 違反：detail 必須是 JSON object");
            }
            if (e.getProposedActionCodes() == null || !e.getProposedActionCodes().startsWith("[")) {
                throw new IllegalStateException("chk_exc_actions_array 違反");
            }
            if (e.getOccurredAt() == null) {
                throw new IllegalStateException("occurred_at 為 NOT NULL");
            }
            if (!openDedupKeys.add(e.getDedupKey())) {
                return false;
            }
            rows.add(e);
            return true;
        }

        public List<FulfillmentExceptionCaseEntity> all() {
            return List.copyOf(rows);
        }

        public long countOfCategory(String category) {
            return rows.stream().filter(r -> category.equals(r.getCategory())).count();
        }
    }

    // =========================================================================

    /**
     * {@link LogisticsCollectionFakes.FakeLedger} ＋ 兩件 W4 需要的東西：
     * <ul>
     *   <li>{@code findByIdempotencyKey} —— 成本應計重放時要取回<b>原來那一筆</b>的 {@code txn_no}；
     *       沿用剛產生的新號碼會讓 {@code ledger_txn_no} 指向一筆不存在的表頭</li>
     *   <li>逐科目借貸合計 —— 多期現金守恆的斷言就靠它</li>
     * </ul>
     */
    public static final class RecordingLedger extends LogisticsCollectionFakes.FakeLedger {

        private final Map<String, String> txnNoByIdempotencyKey = new LinkedHashMap<>();

        @Override
        public boolean post(long txnId, LedgerTxn txn, String txnNo, LocalDateTime postedAt) {
            boolean created = super.post(txnId, txn, txnNo, postedAt);
            if (created) {
                txnNoByIdempotencyKey.put(txn.idempotencyKey(), txnNo);
            }
            return created;
        }

        @Override
        public Optional<LedgerTransactionEntity> findByIdempotencyKey(String idempotencyKey) {
            String txnNo = txnNoByIdempotencyKey.get(idempotencyKey);
            if (txnNo == null) {
                return Optional.empty();
            }
            LedgerTransactionEntity e = new LedgerTransactionEntity();
            e.setTxnNo(txnNo);
            e.setIdempotencyKey(idempotencyKey);
            return Optional.of(e);
        }

        /** 某科目的<b>借方 − 貸方</b>。資產科目為正代表平台手上有這麼多。 */
        public long netDebit(LedgerAccountCode account) {
            long net = 0L;
            for (LedgerTxn txn : posted()) {
                for (LedgerEntry e : txn.entries()) {
                    if (!e.accountCode().equals(account.code())) {
                        continue;
                    }
                    net += e.isDebit() ? e.amount() : -e.amount();
                }
            }
            return net;
        }

        /** 某科目的<b>貸方 − 借方</b>。負債／收入科目為正代表平台欠這麼多／賺了這麼多。 */
        public long netCredit(LedgerAccountCode account) {
            return -netDebit(account);
        }

        /** 🔴 對照組用：全部分錄的借方合計必等於貸方合計（INV-L1 的整體形式）。 */
        public long totalDebit() {
            return posted().stream().mapToLong(LedgerTxn::totalDebit).sum();
        }

        public long totalCredit() {
            return posted().stream().mapToLong(LedgerTxn::totalCredit).sum();
        }
    }
}
