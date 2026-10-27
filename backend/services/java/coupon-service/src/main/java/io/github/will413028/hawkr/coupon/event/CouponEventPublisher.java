package io.github.will413028.hawkr.coupon.event;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.CouponEventType;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
public class CouponEventPublisher {

    private static final Logger log = LoggerFactory.getLogger(CouponEventPublisher.class);

    private final RabbitTemplate rabbitTemplate;
    private final BuyerGrpcClient buyerGrpcClient;

    public CouponEventPublisher(RabbitTemplate rabbitTemplate, BuyerGrpcClient buyerGrpcClient) {
        this.rabbitTemplate = rabbitTemplate;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    public void publishCouponIssued(long buyerId, long couponId, String couponName,
                                    String couponImageUrl, String expireAt) {
        // buyer_id -> 登入 account_id 解析（notification routing 用，鏡像
        // OrderOutboxPublisher.enrich 對 notifyBuyerAccountId 的解析模式）。
        // 失敗時帶 0（notification-service 端 skip），不阻斷本次 coupon.issued 派送主流程。
        long notifyBuyerAccountId;
        try {
            // 通知類：未解析帶 0（略過收件人），不回吐 buyer_id 冒充 account_id。
            notifyBuyerAccountId = buyerGrpcClient.resolvePrincipals(List.of(buyerId))
                    .principalOrZero(buyerId, "buyer_id");
        } catch (Exception e) {
            log.error("Failed to resolve notifyBuyerAccountId for coupon.issued: buyerId={}", buyerId, e);
            notifyBuyerAccountId = 0L;
        }
        try {
            CouponIssuedEvent event = new CouponIssuedEvent(buyerId, notifyBuyerAccountId, couponId, couponName,
                    couponImageUrl, expireAt);
            rabbitTemplate.convertAndSend(MqConstants.EXCHANGE_MARKETING,
                    CouponEventType.COUPON_ISSUED.value(), event);
            log.info("Published coupon.issued event: buyerId={}, notifyBuyerAccountId={}, couponId={}",
                    buyerId, notifyBuyerAccountId, couponId);
        } catch (Exception e) {
            log.error("Failed to publish coupon.issued event: buyerId={}, couponId={}", buyerId, couponId, e);
        }
    }

    /**
     * issue #1176 PR-4：發布優惠券即將到期預警（snake_case wire，36 號 §7.4.1）。
     */
    public void publishCouponExpiring(CouponExpiringEvent event) {
        try {
            rabbitTemplate.convertAndSend(MqConstants.EXCHANGE_MARKETING,
                    CouponEventType.COUPON_EXPIRING.value(), event);
        } catch (Exception e) {
            log.error("Failed to publish coupon.expiring event: buyerId={}, notifyBuyerAccountId={}, couponInstanceId={}",
                    event.buyerId(), event.notifyBuyerAccountId(), event.couponInstanceId(), e);
        }
    }

}
