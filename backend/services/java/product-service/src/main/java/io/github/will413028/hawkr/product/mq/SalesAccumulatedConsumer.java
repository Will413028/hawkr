package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.SalesCountEventDedupRepository;
import io.github.will413028.hawkr.product.service.ProductCacheService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;

/**
 * 消費 order.sales.accumulated 事件，累計商品銷量（sales_count）。
 *
 * <p>依 35 設計書 §五、§七：
 * <ul>
 *   <li>同事務先以 order_no claim 去重表（INSERT ON CONFLICT DO NOTHING），rowsAffected=1 才累計；
 *       redelivery（MaxRetries requeue / 連線中斷重投 / 手動重放）造成的重複累加由此擋下（比照 FavoriteCountService）</li>
 *   <li>同事務內逐筆 UPDATE，by productId ASC 排序避免死鎖（docs/開發規範/後端服務責任邊界規範.md §三）</li>
 *   <li>UPDATE 是 DB-side atomic increment（{@code sales_count = sales_count + ?}），無需 SELECT FOR UPDATE</li>
 *   <li>publisher 端已 group by productId，consumer 不需重複合併</li>
 * </ul>
 *
 * <p>失敗策略：order_no 缺失 → 無去重鍵不可累計，直接 reject 進 DLQ（不重試）；
 * parse / UPDATE 異常 → rollback + 進 DLQ（沿用 OrderCancelledConsumer pattern）。
 */
@Component
public class SalesAccumulatedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SalesAccumulatedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final SalesAccumulator accumulator;
    private final ProductCacheService cacheService;
    private final ProductRepository productRepository;

    public SalesAccumulatedConsumer(SalesAccumulator accumulator,
                                    ProductCacheService cacheService,
                                    ProductRepository productRepository) {
        this.accumulator = accumulator;
        this.cacheService = cacheService;
        this.productRepository = productRepository;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED,
                    containerFactory = "manualAckContainerFactory")
    public void onSalesAccumulated(Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        String orderNo = "unknown";

        try {
            JsonNode root = MAPPER.readTree(body);
            orderNo = root.path("order_no").asText("");

            // order_no 是唯一去重鍵：缺失即無法保證冪等，禁止無 guard 累加 → 直接 reject 進 DLQ（重試也永遠缺鍵）。
            if (orderNo.isBlank()) {
                log.error("SalesAccumulatedConsumer.orderNo.missing",
                        "缺少 order_no，無法冪等去重，reject 進 DLQ body=" + body);
                channel.basicReject(deliveryTag, false);
                return;
            }

            JsonNode itemsNode = root.path("items");
            List<SalesItem> items = new ArrayList<>();
            for (JsonNode item : itemsNode) {
                Long productId = item.path("product_id").asLong();
                int quantity = item.path("quantity").asInt();
                if (productId <= 0 || quantity <= 0) continue;
                items.add(new SalesItem(productId, quantity));
            }

            if (items.isEmpty()) {
                log.warn("SalesAccumulatedConsumer.empty",
                        "items 為空或全部不合法，跳過", "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // by productId ASC 排序避免併發死鎖（§22.1）
            items.sort(Comparator.comparingLong(SalesItem::productId));

            // 同事務 claim order_no 去重 + 累計；重複事件回傳 false（未寫入 → 不需 evict）。
            boolean applied = accumulator.applyAll(orderNo, items);
            if (!applied) {
                log.info("SalesAccumulatedConsumer.duplicate",
                        "重複事件，跳過累計，orderNo=" + orderNo, "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // sales_count 反正規化欄位出現在 ProductDetail 與 ProductListItemDTO（35 設計書）→ 提交後清快取。
            // applyAll 為 @Transactional bean 方法，回傳即代表已 commit，於此 evict = post-commit（#1426）。
            evictProductCache(items);

            log.info("SalesAccumulatedConsumer.success",
                    "銷量累計完成，orderNo=" + orderNo + ", items=" + items.size(),
                    "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("SalesAccumulatedConsumer.error",
                    "銷量累計失敗，orderNo=" + orderNo, e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "SalesAccumulatedConsumer", e);
        }
    }

    /**
     * 銷量更新後清除受影響商品的 detail + list 快取（best-effort，失敗不阻斷 ack）。
     * <p>detail 逐 product 清；list 反查 owner（store_id）後批次清 owner bucket + 公開聚合 bucket 0。
     */
    private void evictProductCache(List<SalesItem> items) {
        try {
            List<Long> productIds = items.stream()
                    .map(SalesItem::productId)
                    .distinct()
                    .toList();
            productIds.forEach(pid -> cacheService.evictProductDetail(String.valueOf(pid)));
            List<Long> ownerIds = productRepository.findProductSellerRefs(productIds).stream()
                    .map(ProductSellerRef::sellerId)
                    .filter(java.util.Objects::nonNull)
                    .toList();
            cacheService.evictProductListForWriteBatch(ownerIds);
        } catch (Exception e) {
            log.warn("SalesAccumulatedConsumer.evict.skip",
                    "銷量快取清除失敗（不影響業務）: " + e.getMessage());
        }
    }

    /**
     * 同事務去重 claim + 逐筆 UPDATE。
     *
     * <p>抽到獨立 Component 以利 Spring `@Transactional` proxy 攔截（自呼叫無 proxy）。
     * dedup claim 與累計必須同生共死：UPDATE 拋例外 → 整個 transaction rollback → dedup 也回滾 →
     * 下次同事件可重試（比照 {@code FavoriteCountService}）。
     */
    @Component
    public static class SalesAccumulator {
        private final ProductRepository productRepository;
        private final SalesCountEventDedupRepository dedupRepository;

        public SalesAccumulator(ProductRepository productRepository,
                                SalesCountEventDedupRepository dedupRepository) {
            this.productRepository = productRepository;
            this.dedupRepository = dedupRepository;
        }

        /**
         * @return true = 本次首次處理、已實際累計；false = 重複事件（已被去重表擋下，未累計）
         */
        @Transactional
        public boolean applyAll(String orderNo, List<SalesItem> sortedItems) {
            if (dedupRepository.insertIfAbsent(orderNo) == 0) {
                return false;   // 重複事件：dedup INSERT NO-OP → 不累計
            }
            for (SalesItem it : sortedItems) {
                productRepository.incrementSalesCount(it.productId(), it.quantity());
            }
            return true;
        }
    }

    public record SalesItem(Long productId, int quantity) {}
}
