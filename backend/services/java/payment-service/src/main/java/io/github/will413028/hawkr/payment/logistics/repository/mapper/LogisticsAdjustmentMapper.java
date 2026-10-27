package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * {@code logistics_adjustments} 的 mapper。
 *
 * <p>🔴 本表<b>沒有 UPDATE</b>：調整單是憑據，寫錯就再開一筆反向的，不改原列
 * （與 {@code ledger_transactions} 同一個理由）。
 */
@Mapper
public interface LogisticsAdjustmentMapper extends BaseMapper<LogisticsAdjustmentEntity> {

    @Insert("""
            INSERT INTO logistics_adjustments (
                id, receivable_id, store_id, provider_delivery_ref, kind, amount,
                responsibility_cause, statement_line_id, created_by, reason
            ) VALUES (
                #{e.id}, #{e.receivableId,jdbcType=BIGINT}, #{e.storeId},
                #{e.providerDeliveryRef,jdbcType=VARCHAR}, #{e.kind}, #{e.amount},
                #{e.responsibilityCause,jdbcType=VARCHAR}, #{e.statementLineId,jdbcType=BIGINT},
                #{e.createdBy,jdbcType=BIGINT}, #{e.reason}
            )
            """)
    int insertAdjustment(@Param("e") LogisticsAdjustmentEntity entity);

    @Select("""
            SELECT * FROM logistics_adjustments
            WHERE receivable_id = #{receivableId}
            ORDER BY created_at
            """)
    List<LogisticsAdjustmentEntity> listByReceivable(@Param("receivableId") Long receivableId);

    @Select("""
            SELECT * FROM logistics_adjustments
            WHERE store_id = #{storeId}
            ORDER BY created_at DESC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<LogisticsAdjustmentEntity> listByStore(@Param("storeId") Long storeId,
                                                @Param("limit") int limit,
                                                @Param("offset") int offset);

    @Select("""
            SELECT * FROM logistics_adjustments
            WHERE statement_line_id = #{statementLineId}
            ORDER BY created_at
            """)
    List<LogisticsAdjustmentEntity> listByStatementLine(@Param("statementLineId") Long statementLineId);
}
