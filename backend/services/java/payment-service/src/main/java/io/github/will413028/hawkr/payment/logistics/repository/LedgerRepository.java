package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.domain.LedgerEntry;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerEntryEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LedgerEntryMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LedgerTransactionMapper;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

/**
 * 雙分錄帳本的 repository（表頭 ＋ 分錄，thin adapter）。
 *
 * <p>🔴 <b>表頭與分錄必須同一個交易</b>：{@code trg_ledger_balanced} 是
 * {@code DEFERRABLE INITIALLY DEFERRED} 的 CONSTRAINT TRIGGER，借貸平衡在 <b>COMMIT 當下</b>
 * 才求值。拆成兩個交易 ⇒ 第一個交易就會帶著一筆不平衡的表頭 commit 成功，而帳本從那一刻起
 * 永久失去可稽核性。所以 {@link #post} 標 {@code @Transactional}。
 *
 * <p>⚠️ {@code @Transactional} 的 self-invocation 禁令（10 檔 §11.1）：呼叫端必須<b>跨 bean</b>
 * 呼叫本方法才有交易；在同一個 class 內以 {@code this.} 叫它等於沒有交易。
 */
@Repository
public class LedgerRepository {

    private final LedgerTransactionMapper txnMapper;
    private final LedgerEntryMapper entryMapper;

    public LedgerRepository(LedgerTransactionMapper txnMapper, LedgerEntryMapper entryMapper) {
        this.txnMapper = txnMapper;
        this.entryMapper = entryMapper;
    }

    /** {@code 'LT' + yyyyMMdd + 8 位流水}。🔴 sequence 在 V9 的手寫 DDL 裡，被 DROP 只會在這一句炸。 */
    public String nextTxnNo() {
        return txnMapper.nextTxnNo();
    }

    /**
     * 寫入一筆帳本交易（表頭 ＋ 全部分錄），以 {@code idempotency_key} 去重。
     *
     * <p>🔴 三道防線，<b>方向一致不互相取代</b>：
     * <ol>
     *   <li>{@link LedgerTxn#validate()}（程式側，這裡先跑）——借貸不平衡、未知碼值、
     *       商流 × txn_type 互斥，全部在進 DB 之前擋掉，錯誤訊息指得回業務語意。</li>
     *   <li>表級 CHECK（{@code chk_ledgtxn_model_type} 等）——擋繞過程式側的寫入路徑。</li>
     *   <li>{@code trg_ledger_balanced}（COMMIT 當下）——擋前兩道都被繞過的情形。</li>
     * </ol>
     *
     * @return {@code true} = 這次真的認列了；{@code false} = 已經認列過（重放，不是錯誤）
     */
    @Transactional
    public boolean post(long txnId, LedgerTxn txn, String txnNo, LocalDateTime postedAt) {
        txn.validate();

        LedgerTransactionEntity header = new LedgerTransactionEntity();
        header.setId(txnId);
        header.setTxnNo(txnNo);
        header.setTxnType(txn.txnType());
        header.setStoreId(txn.storeId());
        header.setOrderNo(txn.orderNo());
        header.setCommerceModel(txn.commerceModel());
        header.setSettlementPeriod(txn.settlementPeriod());
        header.setSourceType(txn.sourceType());
        header.setSourceRef(txn.sourceRef());
        header.setIdempotencyKey(txn.idempotencyKey());
        header.setPostedAt(postedAt);

        if (txnMapper.insertIfAbsent(header) == 0) {
            // 🔴 已認列過：一列分錄都不要寫。寫了就會掛到「上一次」那筆表頭上，
            //    讓一筆本來平衡的交易變成不平衡，而症狀要到 COMMIT 才出現。
            return false;
        }

        for (LedgerEntry e : txn.entries()) {
            entryMapper.insertEntry(txnId, e.lineSeq(), e.accountCode(), e.ownerType(),
                    e.ownerRefId(), e.direction(), e.amount(), e.memo());
        }
        return true;
    }

    public Optional<LedgerTransactionEntity> findByIdempotencyKey(String idempotencyKey) {
        return Optional.ofNullable(txnMapper.findByIdempotencyKey(idempotencyKey));
    }

    public Optional<LedgerTransactionEntity> findById(long id) {
        return Optional.ofNullable(txnMapper.findById(id));
    }

    public Optional<LedgerTransactionEntity> findByTxnNo(String txnNo) {
        return Optional.ofNullable(txnMapper.findByTxnNo(txnNo));
    }

    public List<LedgerEntryEntity> listEntries(long txnId) {
        return entryMapper.listByTxnId(txnId);
    }

    /**
     * 稽核用：某科目在某商流下的借方合計。
     *
     * <p>🔴 <b>必須帶 commerceModel</b>：放寬 {@code chk_ledgtxn_model} 之後，舊商流的 dormant
     * 帳本與新商流共用同一張表，不帶條件會把兩套帳混算。
     *
     * @param storeId 傳 0 代表不限賣家
     */
    public long sumDebit(String accountCode, String commerceModel, long storeId) {
        return entryMapper.sumDebitByAccount(accountCode, commerceModel, storeId,
                LedgerEntry.DIRECTION_DEBIT);
    }

    /** 同 {@link #sumDebit}，貸方合計。 */
    public long sumCredit(String accountCode, String commerceModel, long storeId) {
        return entryMapper.sumCreditByAccount(accountCode, commerceModel, storeId,
                LedgerEntry.DIRECTION_CREDIT);
    }
}
