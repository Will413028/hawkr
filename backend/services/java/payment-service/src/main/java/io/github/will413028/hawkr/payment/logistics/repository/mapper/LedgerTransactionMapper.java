package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * {@code ledger_transactions} 的 mapper。
 *
 * <p>🔴 本表 <b>append-only</b>：本介面<b>刻意沒有任何 UPDATE 或 DELETE</b>。更正一律開一筆
 * 反向的 {@code MANUAL_ADJUSTMENT}。（{@link BaseMapper} 仍帶著 {@code updateById}／
 * {@code deleteById}，那是繼承來的通用方法——本表禁止使用它們，review 時以此註解為據。）
 */
@Mapper
public interface LedgerTransactionMapper extends BaseMapper<LedgerTransactionEntity> {

    /**
     * 取下一個 {@code txn_no}：{@code 'LT' + yyyyMMdd + LPAD(nextval, 8, '0')}。
     *
     * <p>🔴 sequence 建在 V9 的手寫 DDL 裡 ⇒ 有人 DROP 它不會有任何東西轉紅，只會在這一句炸。
     */
    @Select("""
            SELECT 'LT' || to_char(NOW() AT TIME ZONE 'UTC', 'YYYYMMDD')
                || LPAD(nextval('ledger_txn_no_seq')::text, 8, '0')
            """)
    String nextTxnNo();

    /**
     * 寫入表頭，以 {@code idempotency_key} 去重。
     *
     * <p>🔴 衝突時回零列（{@code ON CONFLICT DO NOTHING}），呼叫端據此判定「已經認列過」——
     * 那不是錯誤，MQ 至少一次投遞讓重放是常態。
     *
     * @return 1 = 新建；0 = 已認列過
     */
    @Insert("""
            INSERT INTO ledger_transactions (
                id, txn_no, txn_type, store_id, order_no, commerce_model,
                settlement_period, source_type, source_ref, idempotency_key, posted_at
            ) VALUES (
                #{e.id}, #{e.txnNo}, #{e.txnType}, #{e.storeId}, #{e.orderNo,jdbcType=VARCHAR},
                #{e.commerceModel}, #{e.settlementPeriod}, #{e.sourceType}, #{e.sourceRef},
                #{e.idempotencyKey}, #{e.postedAt}
            )
            ON CONFLICT (idempotency_key) DO NOTHING
            """)
    int insertIfAbsent(@Param("e") LedgerTransactionEntity entity);

    @Select("SELECT * FROM ledger_transactions WHERE idempotency_key = #{key}")
    LedgerTransactionEntity findByIdempotencyKey(@Param("key") String idempotencyKey);

    @Select("SELECT * FROM ledger_transactions WHERE id = #{id}")
    LedgerTransactionEntity findById(@Param("id") Long id);

    @Select("SELECT * FROM ledger_transactions WHERE txn_no = #{txnNo}")
    LedgerTransactionEntity findByTxnNo(@Param("txnNo") String txnNo);
}
