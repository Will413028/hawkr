package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.SellerEventType;
import io.github.will413028.hawkr.product.service.SellerProductOffShelfService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * 賣家刪除下架消費者。
 *
 * <p>消費 seller-service 發布的 {@code seller.deleted} 事件（{@code SellerDeletedEvent}），
 * 透過 {@link SellerProductOffShelfService#offShelfAllByMerchant} 把該賣家所有 ACTIVE 商品
 * 下架為 INACTIVE，避免已刪除賣家的商品繼續出現在公開面向。</p>
 *
 * <p>事件無條件在賣家刪除時發布（即使底層帳號因仍有買家身分而未軟刪），因此這是比
 * {@code account.deleted} 更正確的觸發點。</p>
 *
 * <h3>異常處理（同 {@link FavoriteCountConsumer}）</h3>
 * <ul>
 *   <li>Payload parse 失敗 / 缺 store_id/accountId / eventType 不符 → 記 WARN + ACK（不重試）</li>
 *   <li>DB 例外（Service 端 transaction rollback） → NACK 進 DLQ</li>
 * </ul>
 */
@Component
public class SellerDeletedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerDeletedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final SellerProductOffShelfService offShelfService;

    public SellerDeletedConsumer(SellerProductOffShelfService offShelfService) {
        this.offShelfService = offShelfService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_PRODUCT_SELLER_DELETED,
        containerFactory = "manualAckContainerFactory"
    )
    public void onSellerDeleted(Message message, Channel channel) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        String body = new String(message.getBody());

        try {
            JsonNode payload = MAPPER.readTree(body);
            String eventType = textOrEmpty(payload, "event_type");
            // ② re-key：owner 識別優先用 store_id（store 代理鍵，seller-service 於刪除 tx 內
            // snapshot）；舊格式 in-flight 事件只有 accountId — fallback 安全
            // （pre-incarnation surrogate == account_id）。
            //
            // D7 Stage 2 PR-B（store_id rename）done 2026-08-02（Phase 2 account_id 命名債
            // campaign Task 3）：seller-service 的 outbound SELLER_DELETED payload key 已從
            // sellerAccountId 正名為 store_id，本消費端同一 PR 原子更新。仍保留對舊鍵的相容
            // fallback 一個 rollout 窗口——若收到部署過渡期間、由尚未更新的舊版
            // seller-service 發出的事件（JSON 仍帶 sellerAccountId），照樣能解析，不落入
            // missing-field 分支。待確認兩邊都已完成部署且無舊格式 backlog 後可再移除。
            String sellerIdStr = textOrEmpty(payload, "store_id");
            if (sellerIdStr.isEmpty()) {
                sellerIdStr = textOrEmpty(payload, "sellerAccountId"); // rollout-window 相容：舊版 publisher
            }
            if (sellerIdStr.isEmpty()) {
                sellerIdStr = textOrEmpty(payload, "accountId");
            }

            if (sellerIdStr.isEmpty()) {
                log.warn("SellerDeletedConsumer.payload.missing",
                        "缺少 store_id/accountId body=" + body);
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!SellerEventType.SELLER_DELETED.value().equals(eventType)) {
                log.warn("SellerDeletedConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + eventType);
                channel.basicAck(deliveryTag, false);
                return;
            }

            Long storeId = Long.parseLong(sellerIdStr);
            offShelfService.offShelfAllByMerchant(storeId);
            channel.basicAck(deliveryTag, false);

        } catch (IllegalArgumentException e) {
            log.warn("SellerDeletedConsumer.parse.fail",
                    "payload 格式錯誤 body=" + body + " error=" + e.getMessage());
            channel.basicAck(deliveryTag, false);   // 不重試
        } catch (Exception e) {
            log.error("SellerDeletedConsumer.handle.fail",
                    "處理失敗，進 DLQ body=" + body, e);
            channel.basicNack(deliveryTag, false, false);
        }
    }

    private static String textOrEmpty(JsonNode node, String key) {
        JsonNode v = node.get(key);
        return v == null || v.isNull() ? "" : v.asText("");
    }
}
