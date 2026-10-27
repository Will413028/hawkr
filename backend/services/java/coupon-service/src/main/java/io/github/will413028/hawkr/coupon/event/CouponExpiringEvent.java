package io.github.will413028.hawkr.coupon.event;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * 優惠券即將到期預警事件（issue #1176 PR-4）。
 *
 * <p>wire 命名 snake_case（36 號 §7.4.1 L1 定案）：{@code event_id} / {@code buyer_id} /
 * {@code coupon_instance_id} 等。notification template（coupon.expiring）以 snake_case read-key 對照。
 *
 * <p>{@code notifyBuyerAccountId} / {@code notify_buyer_account_id}（buyer_id rekey Phase 5，
 * 2026-07-18 新增）：由 {@code buyerId}（買家業務值 buyer_id，來自
 * {@code coupon_instances.owner_buyer_id}）經 {@code BuyerGrpcClient.resolvePrincipals}
 * 解析出的登入 account_id。notification-service 的 {@code device_tokens} 是用登入
 * account_id 鍵的，直接拿 buyer_id 當路由值在買家分岔（重新開業）後會找不到裝置。
 * 鏡像 order-service Phase 1 的 {@code notify_buyer_account_id} 手法：保留原欄位
 * （{@code buyer_id}）的業務值不變，新增這個欄位供 notification routing 專用。
 * 解析失敗時帶 0（notification-service 端 skip），見
 * {@code CouponExpiringWarningJob.execute()}。
 *
 * <p>冪等：notification 端以 reference_id {@code expiring_{coupon_instance_id}_{date_key}}
 * （notifications UNIQUE，36 號 §7.5 策略 3）保證同券同日只推一次；本 job 每日重跑 / 多 pod 併發天然去重，
 * 不引入 dedup 表（docs/開發規範/後端服務責任邊界規範.md）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record CouponExpiringEvent(
        String eventId,
        String eventType,
        String occurredAt,
        String buyerId,
        String notifyBuyerAccountId,
        String couponInstanceId,
        String couponName,
        String couponImageUrl,
        String expireAt,
        String dateKey
) {
    public static CouponExpiringEvent of(long buyerId, long notifyBuyerAccountId, long couponInstanceId,
                                         String couponName, String couponImageUrl, String expireAt, String dateKey,
                                         String occurredAt) {
        return new CouponExpiringEvent(
                UUID.randomUUID().toString(),
                "COUPON_EXPIRING",
                occurredAt,
                String.valueOf(buyerId),
                String.valueOf(notifyBuyerAccountId),
                String.valueOf(couponInstanceId),
                couponName,
                couponImageUrl == null ? "" : couponImageUrl,
                expireAt,
                dateKey
        );
    }
}
