package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.repository.FavoriteCountEventDedupRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * 收藏計數同事務維護（02 §3.5.1 / §5.14）。
 *
 * 抽為獨立 {@link Service} 而非寫在 {@link io.github.will413028.hawkr.product.mq.FavoriteCountConsumer} 內，
 * 以避免 Spring {@code @Transactional} self-invocation 失效（CGLIB proxy 不代理同 class 內呼叫）。
 *
 * 同事務語意：
 * <ul>
 *   <li>INSERT dedup（{@code ON CONFLICT DO NOTHING}）+ UPDATE product.favorite_count 必須同生共死</li>
 *   <li>UPDATE 拋例外 → 整個 transaction rollback → dedup 也回滾 → 下次同 event 可重試</li>
 *   <li>重複 event：dedup INSERT NO-OP → 直接 return → 不執行 UPDATE</li>
 * </ul>
 */
@Service
public class FavoriteCountService {

    private static final ObsLogger log = ObsLogger.forClass(FavoriteCountService.class);

    private final FavoriteCountEventDedupRepository dedupRepository;
    private final ProductRepository productRepository;
    private final ProductCacheService cacheService;

    public FavoriteCountService(FavoriteCountEventDedupRepository dedupRepository,
                                ProductRepository productRepository,
                                ProductCacheService cacheService) {
        this.dedupRepository = dedupRepository;
        this.productRepository = productRepository;
        this.cacheService = cacheService;
    }

    /**
     * 同事務 INSERT dedup + UPDATE product.favorite_count ± delta。
     *
     * @param eventId   UUID v4 字串（buyer publisher 生成）
     * @param eventType BuyerEventType.value（UPPER_SNAKE）
     * @param productId 目標商品 ID
     * @param delta     +1 / -1
     * @return true = 本次有實際 +/-1；false = 重複 event 或商品不存在 / 已軟刪除 / favorite_count 已為 0
     */
    @Transactional
    public boolean applyCountUpdate(String eventId, String eventType, Long productId, int delta) {
        int dedupAffected = dedupRepository.insertIfAbsent(eventId, eventType, productId);
        if (dedupAffected == 0) {
            log.info("FavoriteCountService.duplicate",
                    "重複事件，跳過 eventId=" + eventId + " productId=" + productId);
            return false;
        }
        int updated = productRepository.incrementFavoriteCount(productId, delta);
        if (updated == 0) {
            log.warn("FavoriteCountService.noop",
                    "商品不存在 / 已軟刪除 / favorite_count 已為 0 productId="
                            + productId + " delta=" + delta);
            return false;
        }
        // 反正規化欄位更新後 evict detail cache，避免讀路徑看到 stale favorite_count（02 §5.14）
        cacheService.evictProductDetail(productId.toString());
        // favorite_count 亦出現在 ProductListItemDTO → 列表快取同步失效（owner bucket + 公開 bucket 0，#1426）。
        Long ownerId = productRepository.findProductSellerRefs(List.of(productId)).stream()
                .findFirst().map(ProductSellerRef::sellerId).orElse(null);
        if (ownerId != null) {
            cacheService.evictProductListForWrite(ownerId);
        }
        return true;
    }
}
