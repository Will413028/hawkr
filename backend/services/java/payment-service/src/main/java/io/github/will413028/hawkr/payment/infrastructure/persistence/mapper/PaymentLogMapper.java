package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentLogEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 支付日誌 Mapper
 */
@Mapper
public interface PaymentLogMapper extends BaseMapper<PaymentLogEntity> {

    /**
     * 依交易編號查詢日誌
     */
    @Select("SELECT * FROM payment_logs WHERE trade_no = #{tradeNo} ORDER BY created_at DESC")
    List<PaymentLogEntity> findByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * 依退款編號查詢日誌
     */
    @Select("SELECT * FROM payment_logs WHERE refund_no = #{refundNo} ORDER BY created_at DESC")
    List<PaymentLogEntity> findByRefundNo(@Param("refundNo") String refundNo);

    /**
     * 查詢衝突日誌
     */
    @Select("""
            SELECT * FROM payment_logs
            WHERE log_type = 'CONFLICT'
            ORDER BY created_at DESC
            LIMIT #{limit}
            """)
    List<PaymentLogEntity> findConflictLogs(@Param("limit") int limit);

    /**
     * 刪除指定時間前的非 ERROR 日誌
     */
    @Delete("""
            DELETE FROM payment_logs
            WHERE created_at < #{before}
              AND log_level IN ('INFO', 'WARNING')
            """)
    int deleteNonErrorBefore(@Param("before") LocalDateTime before);

    /**
     * 刪除指定時間前的 ERROR 日誌
     */
    @Delete("DELETE FROM payment_logs WHERE created_at < #{before} AND log_level = 'ERROR'")
    int deleteErrorBefore(@Param("before") LocalDateTime before);
}
