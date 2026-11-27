package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.util.List;

/**
 * 評論聚合反正規化維護（02 §5.14a，#1156）。
 *
 * 消費 review-service 的 {@code review.created} / {@code review.hidden} 事件，將 review-service 權威重算的
 * 商品層級平均星等 / 評論數 <b>full-recompute 覆寫</b>到 {@code product.rating} / {@code product.review_count}。
 *
 * <p>與 {@link FavoriteCountService}（±1 增量 → 需 dedup 表）的關鍵差異：本服務採全量覆寫，
 * event 帶當前權威快照，重複 / 亂序 event 覆寫為相同 / 最後值，<b>天然冪等</b>，故不需 dedup 表、不需事務。</p>
 */
@Service
public class ReviewStatsService {

    private static final ObsLogger log = ObsLogger.forClass(ReviewStatsService.class);

    private final ProductRepository productRepository;
    private final ProductCacheService cacheService;

    public ReviewStatsService(ProductRepository productRepository, ProductCacheService cacheService) {
        this.productRepository = productRepository;
        this.cacheService = cacheService;
    }

    /**
     * 以 review-service 權威重算值覆寫商品評論聚合。
     *
     * @param productId   商品 ID
     * @param avgRating   商品平均星等（review-service 已排除 hidden / deleted 列重算）
     * @param reviewCount 商品評論數
     * @return true = 有實際覆寫；false = 商品不存在 / 已軟刪除（UPDATE rowsAffected = 0）
     */
    public boolean applyReviewStats(Long productId, BigDecimal avgRating, long reviewCount) {
        BigDecimal effectiveRating = reviewCount > 0 ? avgRating : null;
        int updated = productRepository.updateReviewStats(productId, effectiveRating, reviewCount);
        if (updated == 0) {
            log.warn("ReviewStatsService.noop",
                    "商品不存在 / 已軟刪除 productId=" + productId
                            + " rating=" + avgRating + " reviewCount=" + reviewCount);
            return false;
        }
        cacheService.evictProductDetail(productId.toString());
        Long ownerId = productRepository.findProductSellerRefs(List.of(productId)).stream()
                .findFirst().map(ProductSellerRef::sellerId).orElse(null);
        if (ownerId != null) {
            cacheService.evictProductListForWrite(ownerId);
        }
        return true;
    }
}
