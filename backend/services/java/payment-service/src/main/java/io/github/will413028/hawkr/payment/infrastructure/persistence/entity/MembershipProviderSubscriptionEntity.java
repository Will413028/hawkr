package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/** Payment-owned provider subscription lifecycle and immutable commercial snapshot. */
@Data
@TableName("membership_provider_subscriptions")
public class MembershipProviderSubscriptionEntity {

    @TableId(type = IdType.INPUT)
    private Long id;
    private String subjectType;
    private String subjectId;
    private String membershipId;
    private String membershipChangeRequestId;
    private String billingQuoteId;
    private String subscriptionKind;
    private String providerAccountRef;
    private String provider;
    private String providerCustomerId;
    private String providerSubscriptionId;
    private Integer subscriptionSeq;
    private String merTradeNo;
    private String offerId;
    private String offerVersionNo;
    private String quoteSnapshotHash;
    private LocalDateTime quoteExpiresAt;
    private String currency;
    private Long unitAmountMinor;
    private String intervalUnit;
    private Integer intervalCount;
    private LocalDateTime firstPeriodDate;
    private LocalDateTime providerAnchorAt;
    private String status;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
