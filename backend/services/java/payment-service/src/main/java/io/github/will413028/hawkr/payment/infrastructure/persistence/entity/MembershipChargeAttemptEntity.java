package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/** Append-only normalized provider charge result. */
@Data
@TableName("membership_charge_attempts")
public class MembershipChargeAttemptEntity {

    @TableId(type = IdType.INPUT)
    private Long id;
    private Long providerSubscriptionRef;
    private String subjectType;
    private String subjectId;
    private String membershipId;
    private String membershipChangeRequestId;
    private String billingQuoteId;
    private String quoteSnapshotHash;
    private String providerPeriodId;
    private String periodOrderNo;
    private Integer thisPeriod;
    private String tradeNo;
    private Long amountMinor;
    private String currency;
    private String providerEventId;
    private String providerEventKey;
    private String status;
    private String resCode;
    private String resCodeMsg;
    private LocalDateTime validFrom;
    private LocalDateTime validUntil;
    private LocalDateTime cycleStartedAt;
    private LocalDateTime cycleUntil;
    private Boolean isInitialAuthorization;
    private Boolean isAuthoritative;
    private String canonicalEventId;
    private LocalDateTime receivedAt;
}
