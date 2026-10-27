package io.github.will413028.hawkr.cart.mq;

import io.github.will413028.hawkr.cart.service.CartCacheService;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.SellerEventType;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * 賣家資料變更 → evict {@code seller:meta} 快取消費者（#1426 方案 A）。
 *
 * <p>消費 seller-service 發布的 {@code seller.profile.changed}（{@code SellerProfileChangedEvent}，
 * snake_case wire，{@code store_id} 為 string）。賣家改店名 / logo / 封面後，主動 evict cart 的
 * {@code seller:meta:{storeId}}，取代等 24h TTL safety net；下次 GetCart miss 經 gRPC lazy 回填新值。
 *
 * <p><b>賣家身份 key 只有 {@code store_id} 一個，不要再加回任何舊 key 的 fallback。</b>
 * account_id vs store_id audit 的 MQ 契約 3-PR 遷移已於 2026-07-15 完成（consumer 先容忍雙 key →
 * publisher dual-write → 本檔移除舊 key fallback），移除前已在 wire 層直接驗證過 dual-write 正確
 * 運作、queue 與 DLQ 皆無積壓；publisher 端（{@code pkg/mq/events.go} 的
 * {@code SellerProfileChangedEvent}）現在也只序列化這一個 key。舊 key 的名字由本服務
 * {@code db/migration} 底下 V2 那支 rename 遷移永久保存（檔名即舊名）。
 *
 * <h3>冪等</h3>
 * evict（delete key）天然冪等——重複 / 亂序訊息重放同 key 無副作用，故<b>不需 dedup 表</b>
 * （同 {@link io.github.will413028.hawkr.cart.config.RabbitMqConfig} 說明 / 對齊 product SellerDeletedConsumer）。
 *
 * <h3>異常處理</h3>
 * <ul>
 *   <li>payload 缺 {@code store_id} / eventType 不符 / 數字格式錯 → WARN + ACK（不重試）</li>
 *   <li>其他未預期例外 → NACK 進 DLQ（{@code cart.seller.profile.changed.dlq}），不 infinite requeue</li>
 * </ul>
 * 註：{@link CartCacheService#evictSellerMeta} 內部已吞 Redis 例外（best-effort），Redis 短暫不可用
 * 時本 consumer 仍 ACK（TTL 為 safety net），不會 DLQ 堆積。
 */
@Component
public class SellerProfileChangedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerProfileChangedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final CartCacheService cartCacheService;

    public SellerProfileChangedConsumer(CartCacheService cartCacheService) {
        this.cartCacheService = cartCacheService;
    }

    @RabbitListener(
            queues = MqConstants.QUEUE_CART_SELLER_PROFILE_CHANGED,
            containerFactory = "manualAckContainerFactory"
    )
    public void onSellerProfileChanged(Message message, Channel channel) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        String body = new String(message.getBody());

        try {
            JsonNode payload = MAPPER.readTree(body);
            String eventType = textOrEmpty(payload, "event_type");
            String storeIdStr = textOrEmpty(payload, "store_id");

            if (storeIdStr.isEmpty()) {
                log.warn("SellerProfileChangedConsumer.payload.missing",
                        "缺少 store_id body=" + body);
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!SellerEventType.SELLER_PROFILE_CHANGED.value().equals(eventType)) {
                log.warn("SellerProfileChangedConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + eventType);
                channel.basicAck(deliveryTag, false);
                return;
            }

            Long storeId = Long.parseLong(storeIdStr);
            cartCacheService.evictSellerMeta(storeId);
            channel.basicAck(deliveryTag, false);

        } catch (IllegalArgumentException e) {
            log.warn("SellerProfileChangedConsumer.parse.fail",
                    "payload 格式錯誤 body=" + body + " error=" + e.getMessage());
            channel.basicAck(deliveryTag, false);   // 不重試
        } catch (Exception e) {
            log.error("SellerProfileChangedConsumer.handle.fail",
                    "處理失敗，進 DLQ body=" + body, e);
            channel.basicNack(deliveryTag, false, false);
        }
    }

    private static String textOrEmpty(JsonNode node, String key) {
        JsonNode v = node.get(key);
        return v == null || v.isNull() ? "" : v.asText("");
    }
}
