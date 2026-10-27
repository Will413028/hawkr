package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.MembershipChargeAttemptEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

@Mapper
public interface MembershipChargeAttemptMapper extends BaseMapper<MembershipChargeAttemptEntity> {

    @Select("SELECT * FROM membership_charge_attempts WHERE provider_event_key = #{providerEventKey}")
    MembershipChargeAttemptEntity findByProviderEventKey(@Param("providerEventKey") String providerEventKey);

    @Select("""
            SELECT * FROM membership_charge_attempts
            WHERE provider_subscription_ref = #{subscriptionRef}
              AND is_authoritative = TRUE
              AND (
                   (#{providerPeriodId,jdbcType=VARCHAR} IS NOT NULL
                       AND provider_period_id = #{providerPeriodId,jdbcType=VARCHAR})
                   OR (#{periodOrderNo,jdbcType=VARCHAR} IS NOT NULL
                       AND period_order_no = #{periodOrderNo,jdbcType=VARCHAR})
              )
            ORDER BY received_at ASC
            LIMIT 1
            FOR UPDATE
            """)
    MembershipChargeAttemptEntity findAuthoritative(
            @Param("subscriptionRef") Long subscriptionRef,
            @Param("providerPeriodId") String providerPeriodId,
            @Param("periodOrderNo") String periodOrderNo
    );
}
