package io.github.will413028.hawkr.coupon.entity.projection;

import java.time.LocalDateTime;

/**
 * 到期預警掃描投影（issue #1176 PR-4）。UNUSED 且所屬 template 即將到期的 instance。
 */
public record CouponExpiringRef(
        Long instanceId,
        Long ownerBuyerId,
        String couponName,
        LocalDateTime endAt
) {}
