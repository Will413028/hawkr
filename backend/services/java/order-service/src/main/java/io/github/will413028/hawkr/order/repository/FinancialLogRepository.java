package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.entity.FinancialLogEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

/**
 * 資金流水表 Mapper
 *
 * 冪等寫入：(type, external_txn_id) UNIQUE，衝突時 DO NOTHING
 */
public interface FinancialLogRepository extends BaseMapper<FinancialLogEntity> {

    /**
     * 冪等插入：若 (type, external_txn_id) 重複則忽略
     */
    @Insert("""
        INSERT INTO financial_logs (log_id, source_ref_no, type, amount, external_txn_id, created_at)
        VALUES (#{logId}, #{sourceRefNo}, #{type}, #{amount}, #{externalTxnId}, NOW())
        ON CONFLICT (type, external_txn_id) DO NOTHING
    """)
    int insertIdempotent(
            @Param("logId") long logId,
            @Param("sourceRefNo") String sourceRefNo,
            @Param("type") String type,
            @Param("amount") long amount,
            @Param("externalTxnId") String externalTxnId
    );

    /**
     * 一筆訂單某一類流水的累計金額（{@code order.refund.finalized} 的
     * {@code cumulative_refund_amount} 唯一來源，58 §6.3.2.3）。
     *
     * <p><b>為什麼不讀 {@code orders.refund_total_amount}</b>：那個欄位只有 PARTIAL 路徑
     * （{@code applyPartialRefund}）維護，全額退款走的是 {@code updateStatusIfMatchAny}、
     * 一分錢都不會累加 ⇒ 全額退款場景讀它會拿到 {@code 0}，發票端就會判成「沒退過」而不作廢。
     *
     * <p>呼叫端必須在同一交易內、{@link #insertIdempotent} <b>之後</b>呼叫，否則少算本筆。
     * 只加總傳入的那一個 type（實務上是 {@code REFUND_ORIGINAL}）——{@code REFUND_COUPON}
     * 全 repo 零寫入點，日後真的開始寫時「退成優惠券算不算折讓基準」是稅務判斷，不是加總判斷。
     */
    @Select("""
        SELECT COALESCE(SUM(amount), 0) FROM financial_logs
        WHERE source_ref_no = #{sourceRefNo} AND type = #{type}
    """)
    long sumAmountByType(@Param("sourceRefNo") String sourceRefNo, @Param("type") String type);

    /**
     * 一筆訂單的資金流水（管理端訂單詳情的對帳明細，依 List 順序逐列渲染）。
     *
     * <p>收尾補主鍵是因為 created_at 取 NOW()＝交易開始時刻：同一次付款／退款在同一交易內寫入的
     * 多筆流水時間戳完全相同，只靠 created_at 沒有全序，明細列的先後會在重新整理時自己對調。
     */
    @Select("SELECT * FROM financial_logs WHERE source_ref_no = #{sourceRefNo} ORDER BY created_at ASC, log_id ASC")
    List<FinancialLogEntity> findBySourceRefNo(@Param("sourceRefNo") String sourceRefNo);
}
