package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.MembershipProviderSubscriptionEntity;
import java.time.LocalDateTime;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

@Mapper
public interface MembershipProviderSubscriptionMapper extends BaseMapper<MembershipProviderSubscriptionEntity> {

    @Select("SELECT * FROM membership_provider_subscriptions WHERE billing_quote_id = #{quoteId}")
    MembershipProviderSubscriptionEntity findByQuoteId(@Param("quoteId") String quoteId);

    @Select("SELECT * FROM membership_provider_subscriptions WHERE id = #{id} FOR UPDATE")
    MembershipProviderSubscriptionEntity findByIdForUpdate(@Param("id") Long id);

    /** Lock the payment-side subscription while adjudicating a provider period. */
    @Select("""
            SELECT * FROM membership_provider_subscriptions
            WHERE provider = #{provider}
              AND provider_subscription_id = #{providerSubscriptionId}
            FOR UPDATE
            """)
    MembershipProviderSubscriptionEntity findByProviderIdentityForUpdate(
            @Param("provider") String provider,
            @Param("providerSubscriptionId") String providerSubscriptionId
    );

    @Select("""
            SELECT * FROM membership_provider_subscriptions
            WHERE subject_type = #{subjectType}
              AND subject_id = #{subjectId}
              AND subscription_kind = 'BASE_MEMBERSHIP'
              AND status IN ('PENDING', 'ACTIVE', 'SUSPENDED')
            ORDER BY created_at DESC
            LIMIT 1
            """)
    MembershipProviderSubscriptionEntity findActive(
            @Param("subjectType") String subjectType,
            @Param("subjectId") String subjectId
    );

    @Select("""
            SELECT COALESCE(MAX(subscription_seq), 0) + 1
            FROM membership_provider_subscriptions
            WHERE subject_type = #{subjectType} AND subject_id = #{subjectId}
            """)
    Integer nextSequence(@Param("subjectType") String subjectType, @Param("subjectId") String subjectId);

    @Update("""
            UPDATE membership_provider_subscriptions
            SET provider_customer_id = #{providerCustomerId},
                provider_subscription_id = #{providerSubscriptionId},
                first_period_date = #{firstPeriodDate},
                provider_anchor_at = #{providerAnchorAt},
                status = #{status}
            WHERE id = #{id}
            """)
    int attachProviderSubscription(
            @Param("id") Long id,
            @Param("providerCustomerId") String providerCustomerId,
            @Param("providerSubscriptionId") String providerSubscriptionId,
            @Param("firstPeriodDate") LocalDateTime firstPeriodDate,
            @Param("providerAnchorAt") LocalDateTime providerAnchorAt,
            @Param("status") String status
    );

    @Update("UPDATE membership_provider_subscriptions SET status = #{status} WHERE id = #{id}")
    int updateStatus(@Param("id") Long id, @Param("status") String status);
}
