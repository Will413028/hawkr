package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.BuyerEventType;
import io.github.will413028.hawkr.product.service.FavoriteCountService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * 收藏計數同步消費者（02 §5.14）。
 *
 * 消費 buyer-service 發布的 {@code buyer.favorite.added} / {@code buyer.favorite.removed} 事件，
 * 透過 {@link FavoriteCountService#applyCountUpdate} 同事務維護 {@code product.favorite_count}
 * 反正規化欄位。
 *
 * <h3>異常處理</h3>
 * <ul>
 *   <li>Payload JSON parse 失敗 / UUID / productId 格式錯 → 記 WARN + ACK（不重試，避免 retry 風暴）</li>
 *   <li>eventType 與 routing key 不一致 → 記 WARN + ACK</li>
 *   <li>DB 例外（Service 端 transaction rollback） → NACK 進 DLQ</li>
 *   <li>重複事件 / 商品不存在 → Service 端 return false，consumer 端 ACK 不重試</li>
 * </ul>
 *
 * <h3>為何把 transactional 業務抽到 Service</h3>
 * Spring CGLIB {@code @Transactional} proxy 對 self-invocation 失效，因此本 class 內**不**標 {@code @Transactional}，
 * 由 {@link FavoriteCountService}（注入式）提供事務邊界（跨 bean 呼叫才會經過 proxy）。
 */
@Component
public class FavoriteCountConsumer {

    private static final ObsLogger log = ObsLogger.forClass(FavoriteCountConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final FavoriteCountService favoriteCountService;

    public FavoriteCountConsumer(FavoriteCountService favoriteCountService) {
        this.favoriteCountService = favoriteCountService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_ADDED,
        containerFactory = "manualAckContainerFactory"
    )
    public void onFavoriteAdded(Message message, Channel channel) throws IOException {
        handle(message, channel, +1, BuyerEventType.BUYER_FAVORITE_ADDED.value());
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED,
        containerFactory = "manualAckContainerFactory"
    )
    public void onFavoriteRemoved(Message message, Channel channel) throws IOException {
        handle(message, channel, -1, BuyerEventType.BUYER_FAVORITE_REMOVED.value());
    }

    private void handle(Message message, Channel channel, int delta, String expectedEventType) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        String body = new String(message.getBody());

        try {
            JsonNode payload = MAPPER.readTree(body);
            String eventIdStr   = textOrEmpty(payload, "event_id");
            String eventType    = textOrEmpty(payload, "event_type");
            String productIdStr = textOrEmpty(payload, "product_id");

            if (eventIdStr.isEmpty() || productIdStr.isEmpty()) {
                log.warn("FavoriteCountConsumer.payload.missing",
                        "缺少必要欄位 body=" + body);
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!BuyerEventType.isValidValue(eventType) || !expectedEventType.equals(eventType)) {
                log.warn("FavoriteCountConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + eventType
                                + " expected=" + expectedEventType);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 驗證 UUID 格式但繼續以字串傳遞（::uuid cast 於 SQL）
            java.util.UUID.fromString(eventIdStr);
            Long productId = Long.parseLong(productIdStr);

            favoriteCountService.applyCountUpdate(eventIdStr, eventType, productId, delta);
            channel.basicAck(deliveryTag, false);

        } catch (IllegalArgumentException e) {
            log.warn("FavoriteCountConsumer.parse.fail",
                    "payload 格式錯誤 body=" + body + " error=" + e.getMessage());
            channel.basicAck(deliveryTag, false);   // 不重試
        } catch (Exception e) {
            log.error("FavoriteCountConsumer.handle.fail",
                    "處理失敗，進 DLQ body=" + body, e);
            channel.basicNack(deliveryTag, false, false);
        }
    }

    private static String textOrEmpty(JsonNode node, String key) {
        JsonNode v = node.get(key);
        return v == null || v.isNull() ? "" : v.asText("");
    }
}
