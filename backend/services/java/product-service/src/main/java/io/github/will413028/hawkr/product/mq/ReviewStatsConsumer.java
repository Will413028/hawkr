package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.service.ReviewStatsService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.math.BigDecimal;
import java.math.RoundingMode;

/**
 * 評論聚合同步消費者（02 §5.14a，#1156）。
 *
 * 消費 review-service 發布的 {@code review.created} / {@code review.hidden} 事件，
 * 透過 {@link ReviewStatsService#applyReviewStats} 將 event 攜帶的商品層級權威統計
 * full-recompute 覆寫到 {@code product.rating} / {@code product.review_count}。
 *
 * <p>單一 {@code onReviewStatsChanged} 同時綁定兩個 routing key（兩事件 payload 皆含
 * {@code productId} / {@code productAvgRating} / {@code productReviewCount}）。覆寫天然冪等，
 * 無需 dedup 表。</p>
 *
 * <h3>異常處理</h3>
 * <ul>
 *   <li>Payload JSON parse 失敗 / 缺 productId → 記 WARN + ACK（不可恢復，不重試）</li>
 *   <li>商品不存在 / 已軟刪除 → Service 端 return false，consumer 端 ACK 不重試</li>
 *   <li>DB 例外 → NACK 進 DLQ（{@code product.review.stats.dlq}）</li>
 * </ul>
 */
@Component
public class ReviewStatsConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ReviewStatsConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ReviewStatsService reviewStatsService;

    public ReviewStatsConsumer(ReviewStatsService reviewStatsService) {
        this.reviewStatsService = reviewStatsService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_PRODUCT_REVIEW_STATS,
        containerFactory = "manualAckContainerFactory"
    )
    public void onReviewStatsChanged(Message message, Channel channel) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        String body = new String(message.getBody());

        try {
            JsonNode payload = MAPPER.readTree(body);
            String productIdStr = textOrEmpty(payload, "product_id");
            if (productIdStr.isEmpty() || "0".equals(productIdStr)) {
                log.warn("ReviewStatsConsumer.payload.missing", "缺少 productId body=" + body);
                channel.basicAck(deliveryTag, false);
                return;
            }
            Long productId = Long.parseLong(productIdStr);
            // fail-loud：統計欄位缺失或非數字 → IllegalStateException → 下方 catch(Exception) NACK 進 DLQ。
            // 不得以 0 預設值覆寫 product.rating / review_count（malformed event 會把權威統計清零）；
            // 「真 0」（無評論）是合法 JSON 數字值，isNumber() 放行。
            // publisher 為 pkg/mq ReviewCreatedEvent / ReviewHiddenEvent（float64 / int64 → JSON number）。
            JsonNode ratingNode = payload.get("product_avg_rating");
            JsonNode countNode = payload.get("product_review_count");
            if (ratingNode == null || !ratingNode.isNumber()) {
                throw new IllegalStateException("product_avg_rating 缺失或非數字: " + ratingNode
                        + ", productId=" + productId);
            }
            if (countNode == null || !countNode.isNumber()) {
                throw new IllegalStateException("product_review_count 缺失或非數字: " + countNode
                        + ", productId=" + productId);
            }
            // productAvgRating 為 float（0~5）；product.rating 為 decimal(3,2)，四捨五入 2 位
            BigDecimal rating = BigDecimal.valueOf(ratingNode.asDouble())
                    .setScale(2, RoundingMode.HALF_UP);
            long reviewCount = countNode.asLong();

            reviewStatsService.applyReviewStats(productId, rating, reviewCount);
            channel.basicAck(deliveryTag, false);

        } catch (IllegalArgumentException e) {
            log.warn("ReviewStatsConsumer.parse.fail",
                    "payload 格式錯誤 body=" + body + " error=" + e.getMessage());
            channel.basicAck(deliveryTag, false);   // 不重試
        } catch (Exception e) {
            log.error("ReviewStatsConsumer.handle.fail", "處理失敗，進 DLQ body=" + body, e);
            channel.basicNack(deliveryTag, false, false);
        }
    }

    private static String textOrEmpty(JsonNode node, String key) {
        JsonNode v = node.get(key);
        return v == null || v.isNull() ? "" : v.asText("");
    }
}
