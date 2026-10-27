package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.BillingCommandEntity;
import java.time.LocalDateTime;
import java.util.UUID;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

@Mapper
public interface BillingCommandMapper extends BaseMapper<BillingCommandEntity> {

    @Insert("""
            INSERT INTO billing_commands (
                id, operation, subject_type, subject_id, idempotency_key, request_hash,
                billing_quote_id, result_subscription_id, status, error_code, error_message,
                result_provider_subscription_id, result_redirect_url, result_mer_trade_no,
                result_status, action_reason, lease_expires_at
            ) VALUES (
                #{id,typeHandler=io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.UuidTypeHandler},
                #{operation}, #{subjectType}, #{subjectId}, #{idempotencyKey}, #{requestHash},
                #{billingQuoteId,jdbcType=VARCHAR}, #{resultSubscriptionId,jdbcType=BIGINT},
                #{status}, #{errorCode,jdbcType=VARCHAR}, #{errorMessage,jdbcType=VARCHAR},
                #{resultProviderSubscriptionId,jdbcType=VARCHAR}, #{resultRedirectUrl,jdbcType=VARCHAR},
                #{resultMerTradeNo,jdbcType=VARCHAR}, #{resultStatus,jdbcType=VARCHAR},
                #{actionReason,jdbcType=VARCHAR}, #{leaseExpiresAt,jdbcType=TIMESTAMP}
            )
            ON CONFLICT DO NOTHING
            """)
    int insertIfAbsent(BillingCommandEntity entity);

    @Update("""
            UPDATE billing_commands
            SET result_subscription_id = #{subscriptionId}, updated_at = #{updatedAt}
            WHERE id = #{commandId}
              AND status = 'PROCESSING'
              AND result_subscription_id IS NULL
            """)
    int bindSubscription(
            @Param("commandId") UUID commandId,
            @Param("subscriptionId") Long subscriptionId,
            @Param("updatedAt") LocalDateTime updatedAt
    );

    @Select("""
            SELECT * FROM billing_commands
            WHERE operation = #{operation} AND idempotency_key = #{idempotencyKey}
            """)
    BillingCommandEntity findByOperationAndKey(
            @Param("operation") String operation,
            @Param("idempotencyKey") String idempotencyKey
    );

    @Select("""
            SELECT * FROM billing_commands
            WHERE operation = 'CREATE_SUBSCRIPTION' AND billing_quote_id = #{billingQuoteId}
            ORDER BY created_at ASC
            LIMIT 1
            """)
    BillingCommandEntity findCreateByQuoteId(@Param("billingQuoteId") String billingQuoteId);

    @Select("""
            SELECT * FROM billing_commands
            WHERE result_subscription_id = #{subscriptionId}
              AND status IN ('PROCESSING', 'UNKNOWN')
            ORDER BY created_at ASC
            LIMIT 1
            """)
    BillingCommandEntity findActiveBySubscriptionId(@Param("subscriptionId") Long subscriptionId);

    @Update("""
            UPDATE billing_commands
            SET status = 'PROCESSING', lease_expires_at = #{leaseExpiresAt}, updated_at = #{updatedAt}
            WHERE id = #{commandId} AND status = 'UNKNOWN'
            """)
    int claimUnknown(
            @Param("commandId") UUID commandId,
            @Param("updatedAt") LocalDateTime updatedAt,
            @Param("leaseExpiresAt") LocalDateTime leaseExpiresAt
    );

    @Update("""
            UPDATE billing_commands
            SET status = 'PROCESSING', lease_expires_at = #{leaseExpiresAt}, updated_at = #{updatedAt}
            WHERE id = #{commandId}
              AND status = 'PROCESSING'
              AND (lease_expires_at IS NULL OR lease_expires_at <= #{updatedAt})
            """)
    int claimStaleProcessing(
            @Param("commandId") UUID commandId,
            @Param("updatedAt") LocalDateTime updatedAt,
            @Param("leaseExpiresAt") LocalDateTime leaseExpiresAt
    );

    @Update("""
            UPDATE billing_commands
            SET status = 'SUCCEEDED', result_subscription_id = #{subscriptionId}, error_code = NULL,
                error_message = NULL,
                result_provider_subscription_id = #{providerSubscriptionId,jdbcType=VARCHAR},
                result_redirect_url = #{redirectUrl,jdbcType=VARCHAR},
                result_mer_trade_no = #{merTradeNo,jdbcType=VARCHAR},
                result_status = #{status}, lease_expires_at = NULL,
                updated_at = #{updatedAt}
            WHERE id = #{commandId} AND status = 'PROCESSING'
            """)
    int markSucceeded(
            @Param("commandId") UUID commandId,
            @Param("subscriptionId") Long subscriptionId,
            @Param("providerSubscriptionId") String providerSubscriptionId,
            @Param("redirectUrl") String redirectUrl,
            @Param("merTradeNo") String merTradeNo,
            @Param("status") String status,
            @Param("updatedAt") LocalDateTime updatedAt
    );

    @Update("""
            UPDATE billing_commands
            SET status = 'FAILED', error_code = #{errorCode}, error_message = #{errorMessage},
                lease_expires_at = NULL, updated_at = #{updatedAt}
            WHERE id = #{commandId} AND status = 'PROCESSING'
            """)
    int markFailed(
            @Param("commandId") UUID commandId,
            @Param("errorCode") String errorCode,
            @Param("errorMessage") String errorMessage,
            @Param("updatedAt") LocalDateTime updatedAt
    );

    @Update("""
            UPDATE billing_commands
            SET status = 'UNKNOWN', error_code = #{errorCode}, error_message = #{errorMessage},
                lease_expires_at = NULL, updated_at = #{updatedAt}
            WHERE id = #{commandId} AND status = 'PROCESSING'
            """)
    int markUnknown(
            @Param("commandId") UUID commandId,
            @Param("errorCode") String errorCode,
            @Param("errorMessage") String errorMessage,
            @Param("updatedAt") LocalDateTime updatedAt
    );
}
