package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.entity.ReviewSnapshot;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

/**
 * 評論事件 MQ 消費者
 *
 * 監聽 Queue：order.review.created（payload 為 review-service ReviewCreatedEvent）
 * 處理：
 *   1. 將訂單 is_reviewed 標記為 TRUE（冪等，任一商品已評即 true）
 *   2. 反正規化訂單層級評論快照至 orders.review（JSONB），most-recent-wins
 *      （取該訂單最近一筆商品評論，供訂單詳情單一評論顯示）
 *
 * 手動 ACK，at-least-once。評論不可改/刪 → 重送覆寫同值，無副作用（03 設計書 §6.1 / §8.9）。
 */
@Component
public class ReviewEventConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ReviewEventConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;

    public ReviewEventConsumer(OrderRepository orderRepository) {
        this.orderRepository = orderRepository;
    }

    @RabbitListener(queues = MqConstants.QUEUE_ORDER_REVIEW_CREATED, containerFactory = "manualAckContainerFactory")
    public void onReviewCreated(org.springframework.amqp.core.Message message, Channel channel,
                                @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("ReviewEventConsumer.reviewCreated.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 1. 訂單層級已評標記（冪等：已 TRUE 不重複更新）
            orderRepository.markReviewed(orderNo);

            // 2. 訂單層級評論快照（most-recent-wins，僅較新評論覆寫）
            // rating 範圍防呆：僅顯示用快照（低危），範圍外跳過覆寫 + warn，不拋例外
            // （markReviewed 已完成，照常 ack；權威評分在 review-service）
            String createdAt = payload.path("created_at").asText("");
            int rating = payload.path("rating").asInt(0);
            if (rating < 1 || rating > 5) {
                log.warn("ReviewEventConsumer.reviewCreated.ratingOutOfRange",
                        "rating 超出 1-5 範圍，跳過評論快照覆寫: rating=" + rating, "orderNo", orderNo);
            } else {
                // review.created payload key = image_keys，值為 objectKey（49 §5.3/§6.3）；
                // 落入 ReviewSnapshot.imageKeys，顯示 URL 由 Gateway 組
                ReviewSnapshot snapshot = new ReviewSnapshot(
                        rating,
                        payload.path("content").asText(""),
                        toStringList(payload.path("image_keys")),
                        createdAt
                );
                orderRepository.updateOrderReview(orderNo, snapshot, createdAt);
            }

            log.info("ReviewEventConsumer.reviewCreated.success", "已標記並更新訂單評論快照", "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ReviewEventConsumer.reviewCreated.error", "處理評論事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ReviewEventConsumer", e);
        }
    }

    private static List<String> toStringList(JsonNode node) {
        List<String> result = new ArrayList<>();
        if (node != null && node.isArray()) {
            for (JsonNode n : node) {
                result.add(n.asText());
            }
        }
        return result;
    }
}
