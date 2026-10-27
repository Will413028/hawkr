package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerEntryEntity;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * {@code ledger_entries} 的 mapper。
 *
 * <p>🔴 {@code amount} 恆正（{@code chk_ledgentry_amount}）；方向由 {@code direction} 表達。
 *
 * <p>🔴 {@code trg_ledger_balanced} 是 {@code DEFERRABLE INITIALLY DEFERRED} 的 CONSTRAINT
 * TRIGGER ⇒ 借貸不平衡會在 <b>COMMIT 當下</b>被擋，而不是這一句 INSERT。
 * ⚠️ 這代表「INSERT 成功」<b>不是</b>「這筆分錄合法」的證據——呼叫端必須把
 * {@code LedgerTxn.validate()} 當第一道、把 commit 的例外當第二道，兩者缺一都會讓
 * 不平衡的分錄在某條路徑上落庫。
 */
@Mapper
public interface LedgerEntryMapper extends BaseMapper<LedgerEntryEntity> {

    /**
     * 寫入一筆分錄。
     *
     * <p>⚠️ 不帶 {@code id}：本表主鍵是 {@code bigserial}，由 DB 產生。
     */
    @Insert("""
            INSERT INTO ledger_entries (
                txn_id, line_seq, account_code, owner_type, owner_ref_id, direction, amount, memo
            ) VALUES (
                #{txnId}, #{lineSeq}, #{accountCode}, #{ownerType}, #{ownerRefId},
                #{direction}, #{amount}, #{memo,jdbcType=VARCHAR}
            )
            """)
    int insertEntry(@Param("txnId") Long txnId,
                    @Param("lineSeq") Short lineSeq,
                    @Param("accountCode") String accountCode,
                    @Param("ownerType") String ownerType,
                    @Param("ownerRefId") Long ownerRefId,
                    @Param("direction") String direction,
                    @Param("amount") Long amount,
                    @Param("memo") String memo);

    @Select("""
            SELECT * FROM ledger_entries
            WHERE txn_id = #{txnId}
            ORDER BY line_seq
            """)
    List<LedgerEntryEntity> listByTxnId(@Param("txnId") Long txnId);

    /**
     * 稽核用：某科目在某商流下的借方合計。
     *
     * <p>🔴 <b>必須帶 {@code commerce_model} 條件</b>：{@code chk_ledgtxn_model} 放寬之後，
     * 舊商流的 dormant 帳本與新商流共用同一張表，不帶條件會把兩套帳混算（70 §13 R5）。
     *
     * <p>{@code storeId} 傳 0 代表不限賣家。
     */
    @Select("""
            SELECT COALESCE(SUM(e.amount) FILTER (WHERE e.direction = #{debit}), 0)
            FROM ledger_entries e
            JOIN ledger_transactions t ON t.id = e.txn_id
            WHERE e.account_code = #{accountCode}
              AND t.commerce_model = #{commerceModel}
              AND (#{storeId} = 0 OR t.store_id = #{storeId})
            """)
    long sumDebitByAccount(@Param("accountCode") String accountCode,
                           @Param("commerceModel") String commerceModel,
                           @Param("storeId") Long storeId,
                           @Param("debit") String debitDirection);

    /** 同 {@link #sumDebitByAccount}，貸方合計。 */
    @Select("""
            SELECT COALESCE(SUM(e.amount) FILTER (WHERE e.direction = #{credit}), 0)
            FROM ledger_entries e
            JOIN ledger_transactions t ON t.id = e.txn_id
            WHERE e.account_code = #{accountCode}
              AND t.commerce_model = #{commerceModel}
              AND (#{storeId} = 0 OR t.store_id = #{storeId})
            """)
    long sumCreditByAccount(@Param("accountCode") String accountCode,
                            @Param("commerceModel") String commerceModel,
                            @Param("storeId") Long storeId,
                            @Param("credit") String creditDirection);
}
