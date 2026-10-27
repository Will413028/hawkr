package io.github.will413028.hawkr.coupon.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 買家刪除事件消費者（07 §5.19；2026-07-16 由 AccountDeletedConsumer 遷移，
 * W2/W3 設計書 §3.2 D6）
 *
 * <p>監聽 Queue: {@code coupon.buyer.deleted}（綁定 {@code hawkr.buyer} / {@code buyer.deleted}）。
 * 來源：buyer-service 在買家角色自己的 DELETE transition 發布的
 * {@code BuyerDeletedEvent}（{@code pkg/mq/events.go}），無條件觸發——即使帳號
 * 因仍有其他角色（例如賣家）存活而未被軟刪。取代原本掛在 {@code account.deleted}
 * 的綁定：{@code account.deleted} 只在帳號「最後一個」active role 被移除時才發，
 * 買家角色單獨被移除時不會觸發，導致舊優惠券在買家重新註冊後原樣冒回來。
 *
 * <p>觸發後對該 buyerId 全部 UNUSED Instance（status='0'）批量作廢（→ ANNUL）；
 * RESERVED / USED / EXPIRED / REVOKED / ANNUL 不動。SQL 條件本身具備冪等：
 * 同 buyerId 重跑時 affected = 0。
 *
 * <p>失敗策略：DB 錯誤拋出 → {@link MqErrorHandler} 走 retry / DLQ。
 * Malformed payload（JSON 解析失敗 / buyerId 為 0）→ log Warn + ack，避免 poison。
 */
@Component
public class BuyerDeletedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(BuyerDeletedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String VOID_REASON = "BUYER_DELETED";

    private final CouponInstanceService instanceService;

    public BuyerDeletedConsumer(CouponInstanceService instanceService) {
        this.instanceService = instanceService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_COUPON_BUYER_DELETED,
                    containerFactory = "manualAckContainerFactory")
    public void onBuyerDeleted(Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("BuyerDeletedConsumer.received", "收到 buyer.deleted 事件");

        try {
            JsonNode payload = MAPPER.readTree(body);
            long buyerId = payload.path("buyerId").asLong(0L);

            if (buyerId <= 0L) {
                log.warn("BuyerDeletedConsumer.invalid",
                        "buyerId 為 0 或缺失，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            int affected = instanceService.voidAllUnusedByOwner(buyerId, VOID_REASON);
            channel.basicAck(deliveryTag, false);

            log.info("BuyerDeletedConsumer.success",
                    "已批量作廢 UNUSED Instance，buyerId=" + buyerId + " affected=" + affected,
                    "buyerId", String.valueOf(buyerId));

        } catch (Exception e) {
            log.error("BuyerDeletedConsumer.error", "處理 buyer.deleted 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "BuyerDeletedConsumer", e);
        }
    }
}
