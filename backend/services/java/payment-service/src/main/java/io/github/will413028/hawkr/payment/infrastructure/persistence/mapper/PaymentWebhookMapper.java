package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentWebhookEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;

/**
 * Webhook 記錄 Mapper
 *
 * 支援依 webhook_type + idempotency_key 進行冪等檢查
 */
@Mapper
public interface PaymentWebhookMapper extends BaseMapper<PaymentWebhookEntity> {

    /**
     * 檢查是否已處理過（冪等性檢查）
     */
    @Select("""
            SELECT EXISTS(
                SELECT 1 FROM payment_webhooks
                WHERE webhook_type = #{webhookType}
                  AND idempotency_key = #{idempotencyKey}
                  AND is_processed = TRUE
            )
            """)
    boolean existsProcessed(
            @Param("webhookType") String webhookType,
            @Param("idempotencyKey") String idempotencyKey
    );

    /**
     * 標記為已處理
     */
    @Update("""
            UPDATE payment_webhooks
            SET is_processed = TRUE,
                process_status = #{processStatus},
                error_message = #{errorMessage},
                processed_at = #{processedAt}
            WHERE id = #{id}
            """)
    int markAsProcessed(
            @Param("id") Long id,
            @Param("processStatus") String processStatus,
            @Param("errorMessage") String errorMessage,
            @Param("processedAt") LocalDateTime processedAt
    );

    /**
     * 刪除指定時間前已處理的 Webhook 記錄
     */
    @Delete("""
            DELETE FROM payment_webhooks
            WHERE is_processed = TRUE
              AND processed_at < #{before}
            """)
    int deleteProcessedBefore(@Param("before") LocalDateTime before);
}
