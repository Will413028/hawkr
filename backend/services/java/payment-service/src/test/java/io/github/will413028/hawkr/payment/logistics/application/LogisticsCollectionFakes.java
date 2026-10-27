package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeGrossUp;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeRatePolicy;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeService;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import java.time.LocalDate;
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
 * 線 B 收款測試用的替身。
 *
 * <h2>🔴 這些替身<b>真的維護狀態，而且鏡射 V9 的述詞</b></h2>
 * 只記 {@code calls} 的替身在狀態機／租約測試上<b>零鑑別力</b>：它對任何轉移都回「成功」，
 * 於是「allowed-from 漏了一個起點」「claim 沒有租約述詞」這種 bug 會全綠，
 * 而它們在真實 DB 上的症狀分別是 CAS {@code rows=0}（呼叫端多半讀成「別人先做過了」）
 * 與<b>同一筆應收被兩個 pod 各收一次錢</b>。
 *
 * <p>逐項鏡射的述詞（每一條都指得回 V9 或 mapper 的原句）：
 * <ul>
 *   <li>{@code uq_logrecv_idem} —— 冪等鍵衝突回「沒建」，<b>不是</b>錯誤</li>
 *   <li>{@code uq_logrecv_delivery_ref}（V15）—— <b>單欄</b> UNIQUE（不含 provider）：
 *       一條 provider leg 全域只能有一筆應收。🔴 <b>與上一條是兩道獨立約束</b>：
 *       idem 鍵含 provider、本索引不含 ⇒ 同 ref 不同 provider 會通過前者、被後者擋下</li>
 *   <li>{@code transitionStatusTo} 的 {@code AND status IN (allowed_from)} ——
 *       不在 allowed-from 回 <b>rows=0</b>，來源是 {@link ReceivableStatusRules}（同一份，不另抄）</li>
 *   <li>{@code claimForSubmitByStatus} 的
 *       {@code status='PENDING' AND (submit_lease_until IS NULL OR submit_lease_until <= NOW())}</li>
 *   <li>{@code claimForQuery} 的
 *       {@code fund_date IS NULL AND submitted_at IS NOT NULL AND (query_lease_until IS NULL OR ... <= NOW())}</li>
 *   <li>{@code uq_logcoll_trade_no} —— 撞號直接拋（那一句<b>刻意沒有</b> ON CONFLICT）</li>
 *   <li>{@code recordQueryResult} 的 {@code AND fund_date IS NULL}</li>
 *   <li>{@code chk_sla_credit} —— {@code credit_balance} 不得為負</li>
 *   <li>{@code ledger_transactions.idempotency_key} 的 {@code ON CONFLICT DO NOTHING}</li>
 * </ul>
 *
 * <p>⚠️ <b>它們不鏡射什麼（誠實標註）</b>：這裡沒有交易。{@code LogisticsCollectionTxService}
 * 在單元測試裡是直接 new 出來的，<b>沒有 Spring proxy ⇒ {@code @Transactional} 不生效</b>
 * ⇒ 「任一步失敗整組 rollback」這條<b>本檔驗不到</b>。本檔改為驗一件更弱但確實成立的事：
 * 失敗的守衛擺在所有副作用<b>之前</b>，所以即使沒有 rollback 也不會留下半套資料。
 * 真正的 rollback 行為需要帶 DB 的整合測試，屬另一波。
 */
public final class LogisticsCollectionFakes {

    private LogisticsCollectionFakes() {
    }

    /** 可控時鐘：租約與寬限期都要它，且不得用 {@code Thread.sleep} 造時間（10 檔 §21）。 */
    public static final class TestClock {
        private LocalDateTime now;

        public TestClock(LocalDateTime start) {
            this.now = start;
        }

        public LocalDateTime now() {
            return now;
        }

        public void advanceSeconds(long seconds) {
            now = now.plusSeconds(seconds);
        }
    }

    /** 單調遞增的 id 產生器（取代 TSID，讓斷言可以硬寫字面值）。 */
    public static final class SeqIds {
        private final AtomicLong seq;

        public SeqIds(long start) {
            this.seq = new AtomicLong(start);
        }

        public long next() {
            return seq.getAndIncrement();
        }
    }

    /** 費率已查證的真實 gross-up：1%、DOWN、版本 TEST。🔴 不做替身，直接跑真的反推。 */
    public static AgentFeeGrossUp verifiedGrossUp() {
        AgentFeeRatePolicy policy = new AgentFeeRatePolicy();
        policy.setRateNumerator(1L);
        policy.setRateDenominator(100L);
        policy.setRounding(AgentFeeRatePolicy.Rounding.DOWN);
        policy.setVersion("TEST-1%");
        return new AgentFeeGrossUp(policy);
    }

    // =========================================================================

    public static final class FakeReceivables extends LogisticsReceivableRepository {

        private final Map<Long, LogisticsReceivableEntity> rows = new LinkedHashMap<>();
        private final Set<String> idempotencyKeys = new LinkedHashSet<>();
        /** {@code uq_logrecv_delivery_ref}（V15）的鏡射：一條 provider leg 全域一筆。 */
        private final Set<String> deliveryRefs = new LinkedHashSet<>();
        private final TestClock clock;

        /** claim 被呼叫幾次。用來斷言「gate 關著時連 claim 都不做」。 */
        public int claimCalls;

        public FakeReceivables(TestClock clock) {
            super(null);
            this.clock = clock;
        }

        @Override
        public boolean createIfAbsent(LogisticsReceivableEntity entity) {
            // 🔴 uq_logrecv_idem：衝突 ＝ 送達事件重放 ⇒ 什麼都不做，回 false（不是錯誤）。
            if (!idempotencyKeys.add(entity.getIdempotencyKey())) {
                return false;
            }
            // 🔴 uq_logrecv_delivery_ref（V15）：**單欄** UNIQUE，不含 provider
            //    ⇒ 一條 provider leg 全域只能有一筆應收，換 provider 也建不出第二筆。
            //    ⚠️ 它與 uq_logrecv_idem 是**兩道獨立的約束**，不是同一件事的兩種寫法：
            //    idem 鍵含 provider、本索引不含 ⇒ 同 ref 不同 provider 會通過前者、被後者擋下。
            //    少了這一條，fake 會放行真 DB 擋下的狀態——那種假綠正是 70 §3.1.2 要防的
            //    「兩套 identity truth」：測試全綠，上線才在 INSERT 當下炸。
            String deliveryRef = entity.getProviderDeliveryRef();
            if (deliveryRef != null && !deliveryRef.isBlank() && !deliveryRefs.add(deliveryRef)) {
                return false;
            }
            LogisticsReceivableEntity row = copy(entity);
            row.setStatus(LogisticsReceivableStatus.PENDING.value());
            rows.put(row.getId(), row);
            return true;
        }

        /**
         * 🔴 <b>繞過 {@code uq_logrecv_delivery_ref} 直接塞一列。唯一合法用途：
         * 測試「索引失效時的防禦分支」。</b>
         *
         * <p>V15 之後「同一個 {@code provider_delivery_ref} 有多筆應收」在真 DB 層<b>不可能發生</b>，
         * 但 {@code LogisticsReconciliationService.reconcileOne} 仍保留
         * {@code matched.size() != 1 ⇒ 進爭議} 的防禦——它防的不是今天，而是
         * <b>哪天有人移除那道索引、或改寫了查詢</b>。防禦分支沒有測試就等於沒有防禦。
         *
         * <p>⚠️ <b>不得用它來造「正常」的測試資料。</b> 走 {@link #createIfAbsent} 才會受
         * 兩道 UNIQUE 拘束；用本方法造出來的狀態<b>在 production 造不出來</b>，
         * 拿它當前提寫出來的斷言會是對一個不存在情境的保證。
         */
        public void forceInsertBypassingUniqueness(LogisticsReceivableEntity entity) {
            LogisticsReceivableEntity row = copy(entity);
            row.setStatus(LogisticsReceivableStatus.PENDING.value());
            rows.put(row.getId(), row);
        }

        @Override
        public Optional<LogisticsReceivableEntity> findById(long id) {
            return Optional.ofNullable(rows.get(id)).map(FakeReceivables::copy);
        }

        @Override
        public boolean transition(long id, LogisticsReceivableStatus to) {
            LogisticsReceivableEntity row = rows.get(id);
            if (row == null) {
                return false;
            }
            // 🔴 鏡射 SQL 的 `AND status IN (allowed_from)`：不在 allowed-from ⇒ rows=0。
            //    來源刻意用 ReceivableStatusRules 本身，不在測試裡另抄一份（兩份必然漂移）。
            LogisticsReceivableStatus from = LogisticsReceivableStatus.fromValue(row.getStatus());
            if (!ReceivableStatusRules.canTransition(from, to)) {
                return false;
            }
            row.setStatus(to.value());
            return true;
        }

        /**
         * 鏡射 {@code claimForSubmitByStatus}：
         * {@code status='PENDING' AND (submit_lease_until IS NULL OR submit_lease_until <= NOW())}，
         * 租約 {@code NOW() + leaseSeconds} 在 UPDATE 內寫回。
         */
        @Override
        public List<LogisticsReceivableEntity> claimPendingForSubmit(int leaseSeconds, int batchSize) {
            claimCalls++;
            List<LogisticsReceivableEntity> claimed = new ArrayList<>();
            for (LogisticsReceivableEntity row : rows.values()) {
                if (claimed.size() >= batchSize) {
                    break;
                }
                if (!LogisticsReceivableStatus.PENDING.value().equals(row.getStatus())) {
                    continue;
                }
                LocalDateTime lease = row.getSubmitLeaseUntil();
                if (lease != null && lease.isAfter(clock.now())) {
                    continue;
                }
                row.setSubmitLeaseUntil(clock.now().plusSeconds(leaseSeconds));
                claimed.add(copy(row));
            }
            return claimed;
        }

        /**
         * 🔴 <b>無租約</b>的列表，只給 ADM 例外佇列用。
         *
         * <p>它存在的理由是變異測試：把 {@link #claimPendingForSubmit} 換成它，
         * 就是「拿掉 claim 的租約述詞」那個變異的機械形式——而那必須讓測試轉紅。
         */
        @Override
        public List<LogisticsReceivableEntity> listByStatus(List<LogisticsReceivableStatus> statuses, int limit) {
            List<LogisticsReceivableEntity> out = new ArrayList<>();
            for (LogisticsReceivableEntity row : rows.values()) {
                if (out.size() >= limit) {
                    break;
                }
                for (LogisticsReceivableStatus s : statuses) {
                    if (s.value().equals(row.getStatus())) {
                        out.add(copy(row));
                        break;
                    }
                }
            }
            return out;
        }

        /**
         * 鏡射 {@code findByProviderDeliveryRef}：該欄<b>沒有 UNIQUE</b>（V9 只有 partial index）
         * ⇒ 回全部命中，不在這裡挑「第一筆」。W4 的對帳用它判「命中不是 1 筆 ⇒ 進爭議」。
         */
        @Override
        public List<LogisticsReceivableEntity> findByProviderDeliveryRef(String providerDeliveryRef) {
            if (providerDeliveryRef == null || providerDeliveryRef.isBlank()) {
                return List.of();
            }
            List<LogisticsReceivableEntity> out = new ArrayList<>();
            for (LogisticsReceivableEntity row : rows.values()) {
                if (providerDeliveryRef.equals(row.getProviderDeliveryRef())) {
                    out.add(copy(row));
                }
            }
            return out;
        }

        /**
         * 鏡射 {@code applyTrueUp}：
         * <ul>
         *   <li>{@code AND actual_cost IS NULL} —— write-once，回填過就回 0 列（不是錯誤）</li>
         *   <li>{@code chk_logrecv_recon_pair} —— {@code actual_cost} 與 {@code statement_line_id}
         *       是<b>雙向等式</b>，單邊寫會被 DB 擋下 ⇒ 這裡同樣拒絕</li>
         *   <li>{@code chk_logrecv_actual} —— {@code actual_cost >= 0}</li>
         * </ul>
         */
        @Override
        public boolean applyTrueUp(long id, long actualCost, long statementLineId) {
            LogisticsReceivableEntity row = rows.get(id);
            if (row == null || row.getActualCost() != null) {
                return false;
            }
            if (actualCost < 0L) {
                throw new IllegalStateException("chk_logrecv_actual 違反：actual_cost=" + actualCost);
            }
            if (statementLineId <= 0L) {
                throw new IllegalStateException(
                        "chk_logrecv_recon_pair 違反：statement_line_id 必須與 actual_cost 同時有值");
            }
            row.setActualCost(actualCost);
            row.setStatementLineId(statementLineId);
            return true;
        }

        @Override
        public void releaseSubmitLease(long id) {
            LogisticsReceivableEntity row = rows.get(id);
            if (row != null) {
                row.setSubmitLeaseUntil(null);
            }
        }

        /** 測試專用：直接改狀態，模擬「另一個執行者在我手上這份快照之後動了它」。 */
        public void forceStatus(long id, LogisticsReceivableStatus status) {
            rows.get(id).setStatus(status.value());
        }

        public String statusOf(long id) {
            return rows.get(id).getStatus();
        }

        public LogisticsReceivableEntity raw(long id) {
            return rows.get(id);
        }

        public int size() {
            return rows.size();
        }

        public static LogisticsReceivableEntity copy(LogisticsReceivableEntity src) {
            LogisticsReceivableEntity c = new LogisticsReceivableEntity();
            c.setId(src.getId());
            c.setStoreId(src.getStoreId());
            c.setOrderNo(src.getOrderNo());
            c.setShipmentId(src.getShipmentId());
            c.setProvider(src.getProvider());
            c.setProviderDeliveryRef(src.getProviderDeliveryRef());
            c.setEstimatedCost(src.getEstimatedCost());
            c.setActualCost(src.getActualCost());
            c.setPlatformSubsidy(src.getPlatformSubsidy());
            c.setTargetNet(src.getTargetNet());
            c.setCarriedResidual(src.getCarriedResidual());
            c.setStatus(src.getStatus());
            c.setChargeBasisFrozenAt(src.getChargeBasisFrozenAt());
            c.setIdempotencyKey(src.getIdempotencyKey());
            c.setInvoiceDocumentId(src.getInvoiceDocumentId());
            c.setSubmitLeaseUntil(src.getSubmitLeaseUntil());
            c.setStatementLineId(src.getStatementLineId());
            return c;
        }
    }

    // =========================================================================

    public static final class FakeAttempts extends LogisticsCollectionAttemptRepository {

        private final Map<Long, LogisticsCollectionAttemptEntity> rows = new LinkedHashMap<>();
        private final Set<String> tradeNos = new LinkedHashSet<>();
        private final TestClock clock;

        /** claim 被呼叫幾次。用來斷言「gate 關著時連 claim 都不做」。 */
        public int claimCalls;

        public FakeAttempts(TestClock clock) {
            super(null);
            this.clock = clock;
        }

        @Override
        public void preCommit(long id, long receivableId, String tradeNo, long grossAmount) {
            // 🔴 鏡射 chk_logcoll_gross：PayUni Amt 值域 1~9999999，佔位值寫不進來。
            if (grossAmount < 1L || grossAmount > 9_999_999L) {
                throw new IllegalStateException("chk_logcoll_gross 違反：gross_amount=" + grossAmount);
            }
            // 🔴 那一句刻意沒有 ON CONFLICT：撞號代表 id 產生器或呼叫端邏輯壞了，必須 fail-loud。
            if (!tradeNos.add(tradeNo)) {
                throw new IllegalStateException("uq_logcoll_trade_no 違反：" + tradeNo);
            }
            LogisticsCollectionAttemptEntity row = new LogisticsCollectionAttemptEntity();
            row.setId(id);
            row.setReceivableId(receivableId);
            row.setTradeNo(tradeNo);
            row.setGrossAmount(grossAmount);
            rows.put(id, row);
        }

        @Override
        public boolean markSubmitted(long id, String submitStatus, String submitMessage,
                                     long grossAmount, LocalDateTime submittedAt, LocalDate expectedFundDate) {
            LogisticsCollectionAttemptEntity row = rows.get(id);
            if (row == null) {
                return false;
            }
            row.setSubmitStatus(submitStatus);
            row.setSubmitMessage(submitMessage);
            row.setGrossAmount(grossAmount);
            row.setSubmittedAt(submittedAt);
            row.setExpectedFundDate(expectedFundDate);
            return true;
        }

        /**
         * 鏡射 {@code claimForQuery}：
         * {@code fund_date IS NULL AND submitted_at IS NOT NULL AND (lease IS NULL OR lease <= NOW())}。
         */
        @Override
        public List<LogisticsCollectionAttemptEntity> claimForQuery(int leaseSeconds, int batchSize) {
            claimCalls++;
            List<LogisticsCollectionAttemptEntity> claimed = new ArrayList<>();
            for (LogisticsCollectionAttemptEntity row : rows.values()) {
                if (claimed.size() >= batchSize) {
                    break;
                }
                if (row.getFundDate() != null || row.getSubmittedAt() == null) {
                    continue;
                }
                LocalDateTime lease = row.getQueryLeaseUntil();
                if (lease != null && lease.isAfter(clock.now())) {
                    continue;
                }
                row.setQueryLeaseUntil(clock.now().plusSeconds(leaseSeconds));
                claimed.add(copy(row));
            }
            return claimed;
        }

        /** 🔴 無租約版本，變異測試用（見 {@link FakeReceivables#listByStatus} 同一理由）。 */
        public List<LogisticsCollectionAttemptEntity> listQueryableIgnoringLease(int batchSize) {
            List<LogisticsCollectionAttemptEntity> out = new ArrayList<>();
            for (LogisticsCollectionAttemptEntity row : rows.values()) {
                if (out.size() >= batchSize) {
                    break;
                }
                if (row.getFundDate() == null && row.getSubmittedAt() != null) {
                    out.add(copy(row));
                }
            }
            return out;
        }

        @Override
        public boolean recordQueryResult(long id, LocalDate fundDate, long actualFee,
                                         long netProceeds, long residual) {
            LogisticsCollectionAttemptEntity row = rows.get(id);
            // 🔴 鏡射 `AND fund_date IS NULL`：另一個持有者先寫了就回 0 列。
            if (row == null || row.getFundDate() != null) {
                return false;
            }
            // 🔴 鏡射 chk_logcoll_net：net = gross − fee，DB 層守著同一式。
            if (netProceeds != row.getGrossAmount() - actualFee) {
                throw new IllegalStateException("chk_logcoll_net 違反");
            }
            row.setFundDate(fundDate);
            row.setActualFee(actualFee);
            row.setNetProceeds(netProceeds);
            row.setResidual(residual);
            row.setQueryLeaseUntil(null);
            return true;
        }

        @Override
        public Optional<LogisticsCollectionAttemptEntity> findByTradeNo(String tradeNo) {
            return rows.values().stream()
                    .filter(r -> tradeNo.equals(r.getTradeNo()))
                    .findFirst()
                    .map(FakeAttempts::copy);
        }

        @Override
        public List<LogisticsCollectionAttemptEntity> listByReceivable(long receivableId) {
            return rows.values().stream()
                    .filter(r -> r.getReceivableId() == receivableId)
                    .map(FakeAttempts::copy)
                    .toList();
        }

        public LogisticsCollectionAttemptEntity raw(long id) {
            return rows.get(id);
        }

        public int size() {
            return rows.size();
        }

        public static LogisticsCollectionAttemptEntity copy(LogisticsCollectionAttemptEntity src) {
            LogisticsCollectionAttemptEntity c = new LogisticsCollectionAttemptEntity();
            c.setId(src.getId());
            c.setReceivableId(src.getReceivableId());
            c.setTradeNo(src.getTradeNo());
            c.setGrossAmount(src.getGrossAmount());
            c.setNetProceeds(src.getNetProceeds());
            c.setResidual(src.getResidual());
            c.setSubmitStatus(src.getSubmitStatus());
            c.setSubmitMessage(src.getSubmitMessage());
            c.setSubmittedAt(src.getSubmittedAt());
            c.setExpectedFundDate(src.getExpectedFundDate());
            c.setFundDate(src.getFundDate());
            c.setActualFee(src.getActualFee());
            c.setQueryLeaseUntil(src.getQueryLeaseUntil());
            return c;
        }
    }

    // =========================================================================

    public static final class FakeAccounts extends SellerLogisticsAccountRepository {

        private final Map<Long, SellerLogisticsAccountEntity> rows = new LinkedHashMap<>();

        public FakeAccounts() {
            super(null);
        }

        @Override
        public void ensureExists(long storeId) {
            rows.computeIfAbsent(storeId, id -> {
                SellerLogisticsAccountEntity e = new SellerLogisticsAccountEntity();
                e.setStoreId(id);
                e.setReceivableBalance(0L);
                e.setCreditBalance(0L);
                return e;
            });
        }

        @Override
        public Optional<SellerLogisticsAccountEntity> find(long storeId) {
            return Optional.ofNullable(rows.get(storeId));
        }

        @Override
        public boolean addReceivable(long storeId, long delta) {
            SellerLogisticsAccountEntity e = rows.get(storeId);
            if (e == null) {
                return false;
            }
            e.setReceivableBalance(e.getReceivableBalance() + delta);
            return true;
        }

        @Override
        public boolean addCredit(long storeId, long delta) {
            SellerLogisticsAccountEntity e = rows.get(storeId);
            if (e == null) {
                return false;
            }
            long next = e.getCreditBalance() + delta;
            // 🔴 鏡射 chk_sla_credit：credit_balance >= 0，抵扣超額被 DB 擋下而不是靜默變負。
            if (next < 0L) {
                throw new IllegalStateException("chk_sla_credit 違反：credit_balance=" + next);
            }
            e.setCreditBalance(next);
            return true;
        }

        public long receivableBalance(long storeId) {
            return rows.get(storeId).getReceivableBalance();
        }

        public long creditBalance(long storeId) {
            return rows.get(storeId).getCreditBalance();
        }
    }

    // =========================================================================

    /**
     * ⚠️ <b>刻意不是 {@code final}</b>：{@link LedgerRepository#post} 標了
     * {@code @Transactional}，所以只要測試把它註冊成 bean 且啟用了交易管理，
     * Spring 就必須 CGLIB 代理它——{@code final} 類別代理不了，症狀是
     * 「Could not generate CGLIB subclass」，而那個訊息指向的是替身而不是被測物。
     */
    public static class FakeLedger extends LedgerRepository {

        private final List<LedgerTxn> posted = new ArrayList<>();
        private final Set<String> idempotencyKeys = new LinkedHashSet<>();
        private final AtomicLong seq = new AtomicLong(1);

        public FakeLedger() {
            super(null, null);
        }

        @Override
        public String nextTxnNo() {
            return "LT20260912" + String.format("%08d", seq.getAndIncrement());
        }

        @Override
        public boolean post(long txnId, LedgerTxn txn, String txnNo, LocalDateTime postedAt) {
            // 🔴 程式側先驗（借貸平衡、商流 × txn_type 互斥）——與真實 repository 同序。
            txn.validate();
            // 🔴 鏡射 ledger_transactions.idempotency_key 的 ON CONFLICT DO NOTHING。
            if (!idempotencyKeys.add(txn.idempotencyKey())) {
                return false;
            }
            posted.add(txn);
            return true;
        }

        public List<LedgerTxn> posted() {
            return List.copyOf(posted);
        }

        public List<LedgerTxn> postedOfType(String txnType) {
            return posted.stream().filter(t -> t.txnType().equals(txnType)).toList();
        }
    }

    // =========================================================================

    /**
     * {@code agent_fee} 通道替身。
     *
     * <h2>🔴 它的鑑別力來自它<b>拒絕什麼</b></h2>
     * <ul>
     *   <li>{@code merId} 不是唯一正解 ⇒ 直接拋。這讓「MerID 解析錯了」不會靜默通過。</li>
     *   <li>每次 {@code submit} 都跑一次 {@code inspector}：它讀的是<b>當下</b>的 DB 狀態，
     *       所以「gross 是不是已經落地」是在 PayUni 被呼叫的那一瞬間量的，不是事後補量。
     *       🔴 這正是 M-1 變異（把持久化移到送出之後）的偵測點。</li>
     * </ul>
     */
    public static final class FakeAgentFee extends AgentFeeService {

        public interface Inspector {
            void inspect(String tradeNo, long grossAmount);
        }

        private final String expectedMerId;
        private AgentFeeService.SubmitOutcome nextSubmit;
        private AgentFeeService.QueryOutcome nextQuery;
        private Inspector inspector = (t, g) -> { };

        public int submitCalls;
        public int queryCalls;
        public final List<Long> submittedGross = new ArrayList<>();
        public final List<String> submittedTradeNos = new ArrayList<>();
        public final List<Long> queriedAmt = new ArrayList<>();

        public FakeAgentFee(String expectedMerId) {
            super(null, null, null, "");
            this.expectedMerId = expectedMerId;
        }

        public void onSubmit(Inspector inspector) {
            this.inspector = inspector;
        }

        public void willAccept() {
            this.nextSubmit = new AgentFeeService.SubmitOutcome("SUCCESS", "", 0L, 0L, "TEST-1%", true);
        }

        public void willReject(String status, String message) {
            this.nextSubmit = new AgentFeeService.SubmitOutcome(status, message, 0L, 0L, "TEST-1%", true);
        }

        /** 逾時／連線失敗：{@code providerResponded=false}。🔴 呼叫端不得視為失敗。 */
        public void willNotRespond() {
            this.nextSubmit = new AgentFeeService.SubmitOutcome("", "timeout", 0L, 0L, "TEST-1%", false);
        }

        public void willQuery(AgentFeeService.QueryOutcome outcome) {
            this.nextQuery = outcome;
        }

        @Override
        public AgentFeeService.SubmitOutcome submit(String merId, String tradeNo,
                                                    AgentFeeGrossUp.Result solved, int feeType, int balType) {
            requireMerId(merId);
            if (feeType != 4 || balType != 1) {
                throw new IllegalStateException("預期 feeType=4（物流）／balType=1（正向），"
                        + "實得 feeType=" + feeType + " balType=" + balType);
            }
            submitCalls++;
            submittedGross.add(solved.grossAmount());
            submittedTradeNos.add(tradeNo);
            inspector.inspect(tradeNo, solved.grossAmount());
            if (nextSubmit == null) {
                throw new IllegalStateException("測試沒有設定 submit 結果");
            }
            // 回應帶回的 gross 就是實際送出的那個（與 adapter 真實行為一致）。
            return new AgentFeeService.SubmitOutcome(nextSubmit.providerStatus(), nextSubmit.providerMessage(),
                    solved.grossAmount(), solved.estimatedFee(), solved.feePolicyVersion(),
                    nextSubmit.providerResponded());
        }

        @Override
        public AgentFeeService.QueryOutcome query(String merId, String tradeNo, long grossAmount) {
            requireMerId(merId);
            queryCalls++;
            queriedAmt.add(grossAmount);
            if (nextQuery == null) {
                throw new IllegalStateException("測試沒有設定 query 結果");
            }
            return nextQuery;
        }

        private void requireMerId(String merId) {
            if (!expectedMerId.equals(merId)) {
                throw new IllegalStateException(
                        "MerID 不是唯一正解：預期 " + expectedMerId + "，實得 " + merId);
            }
        }
    }

    /** 已撥款且有 Fee。 */
    public static AgentFeeService.QueryOutcome settled(String fundDate, long actualFee, long grossAmount) {
        return new AgentFeeService.QueryOutcome(true, "", "", grossAmount, "4", "1", "",
                Optional.of(fundDate), Optional.of(actualFee), Optional.empty());
    }

    /** 🔴 受理了但還沒撥款（{@code FundDate='0000-00-00'} 已被 adapter 翻成 empty）。 */
    public static AgentFeeService.QueryOutcome notYetFunded(long grossAmount) {
        return new AgentFeeService.QueryOutcome(true, "", "", grossAmount, "4", "1", "",
                Optional.empty(), Optional.empty(), Optional.empty());
    }

    /** 🔴 PayUni 根本沒有這筆——與「尚未撥款」是兩件事。 */
    public static AgentFeeService.QueryOutcome notFound() {
        return new AgentFeeService.QueryOutcome(false, "", "", 0L, "", "", "",
                Optional.empty(), Optional.empty(), Optional.empty());
    }

    /** 已撥款但 PayUni 沒給 Fee ⇒ 算不出實收。🔴 不得用預估值冒充。 */
    public static AgentFeeService.QueryOutcome settledWithoutFee(String fundDate, long grossAmount) {
        return new AgentFeeService.QueryOutcome(true, "", "", grossAmount, "4", "1", "",
                Optional.of(fundDate), Optional.empty(), Optional.empty());
    }
}
