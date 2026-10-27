package io.github.will413028.hawkr.coupon.event;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

import java.time.Instant;
import java.util.HashMap;
import java.util.UUID;

/**
 * 優惠券發放事件（coupon.issued）。
 *
 * <p>{@code notifyBuyerAccountId}：由 {@code buyerId}（買家業務值）經
 * {@code BuyerGrpcClient.resolvePrincipals} 解析出的登入 account_id，供未來
 * notification-service 路由使用（鏡像 order-service {@code OrderOutboxPublisher.enrich}
 * / coupon-service {@code CouponExpiringWarningJob} 對 notify_buyer_account_id 的解析
 * 模式：保留原欄位 {@code buyerId} 的業務值不變，新增此欄位供 notification routing 專用）。
 * 解析失敗時帶 0（notification-service 端 skip）。目前尚無 notification_templates row
 * 消費本事件，此欄位暫為休眠但結構性存在的欄位。
 */
@Data
@EqualsAndHashCode(callSuper = true)
@SuperBuilder
@NoArgsConstructor
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public class CouponIssuedEvent extends BaseEvent {

    private long buyerId;
    private long notifyBuyerAccountId;
    private long couponId;
    private String couponName;
    private String couponImageUrl;
    private String expireAt;

    public CouponIssuedEvent(long buyerId, long notifyBuyerAccountId, long couponId, String couponName,
                             String couponImageUrl, String expireAt) {
        setEventId(UUID.randomUUID().toString());
        setEventType("COUPON_ISSUED");
        setVersion("1.0");
        setTimestamp(Instant.now());
        setSource("coupon-service");
        setMetadata(new HashMap<>());
        this.buyerId = buyerId;
        this.notifyBuyerAccountId = notifyBuyerAccountId;
        this.couponId = couponId;
        this.couponName = couponName;
        this.couponImageUrl = couponImageUrl;
        this.expireAt = expireAt;
    }
}
