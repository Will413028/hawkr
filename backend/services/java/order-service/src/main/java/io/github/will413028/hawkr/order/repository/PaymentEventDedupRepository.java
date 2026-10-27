package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.entity.PaymentEventDedupEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * 支付事件冪等表 Mapper
 */
public interface PaymentEventDedupRepository extends BaseMapper<PaymentEventDedupEntity> {

    @Select("SELECT * FROM payment_event_dedup WHERE transaction_id = #{transactionId}")
    PaymentEventDedupEntity findByTransactionId(@Param("transactionId") String transactionId);

    /**
     * 嘗試插入冪等記錄，若已存在則忽略
     *
     * @return 1 表示成功插入（首次），0 表示已存在（重複）
     */
    @Insert("""
        INSERT INTO payment_event_dedup (transaction_id, order_no, event_type, payload_hash, processed_time)
        VALUES (#{transactionId}, #{orderNo}, #{eventType}, #{payloadHash}, NOW())
        ON CONFLICT (transaction_id) DO NOTHING
    """)
    int insertIdempotent(
            @Param("transactionId") String transactionId,
            @Param("orderNo") String orderNo,
            @Param("eventType") String eventType,
            @Param("payloadHash") String payloadHash
    );

    /**
     * 清理過期冪等記錄（7 天前），由排程任務呼叫
     */
    @Delete("DELETE FROM payment_event_dedup WHERE processed_time < NOW() - INTERVAL '7 days'")
    int deleteExpired();
}
