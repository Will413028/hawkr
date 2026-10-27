package io.github.will413028.hawkr.coupon.constant;

import io.github.will413028.hawkr.common.utils.code.CouponTargetType;

import java.util.EnumSet;
import java.util.Set;

/**
 * CouponTargetType 業務輔助規則
 *
 * 依 docs/開發規範/Java 微服務開發規範.md §6：CodeMeta enum 不得內嵌業務輔助方法，
 * 此 helper 集中 coupon-service 對 target_type 的判斷邏輯。
 */
public final class CouponTargetTypeRules {

    /** 賣家可發放的目標類型（不含 COMPENSATION，補償券由售後流程發放） */
    private static final Set<CouponTargetType> SELLER_ALLOWED = EnumSet.of(
            CouponTargetType.ALL,
            CouponTargetType.MEMBER,
            CouponTargetType.DESIGNATED
    );

    /** 需要附帶買家名單的目標類型 */
    private static final Set<CouponTargetType> REQUIRES_TARGET_ACCOUNTS = EnumSet.of(
            CouponTargetType.DESIGNATED,
            CouponTargetType.COMPENSATION
    );

    private CouponTargetTypeRules() {
    }

    public static boolean isSellerAllowed(CouponTargetType type) {
        return type != null && SELLER_ALLOWED.contains(type);
    }

    public static boolean requiresTargetAccounts(CouponTargetType type) {
        return type != null && REQUIRES_TARGET_ACCOUNTS.contains(type);
    }
}
