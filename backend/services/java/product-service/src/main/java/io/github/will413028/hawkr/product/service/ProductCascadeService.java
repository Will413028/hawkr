package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * Product cascade reflow Service（02 設計書 §5.11.4 / §5.11.5）。
 *
 * <p>用途：當 category 寫入路徑（改名 / 批次更換）影響 product 的 {@code cat_*_name} 與
 * {@code searchable_text} 反正規化欄位時，**同事務內**重算受影響 product。
 *
 * <p>避免 {@link CategoryService} 與 {@link ProductService} 循環依賴，由本 Service 居中執行。
 * 對 {@link ProductService} 用 {@link Lazy} 切斷 startup-time 依賴環。
 */
@Service
public class ProductCascadeService {

    private static final ObsLogger log = ObsLogger.forClass(ProductCascadeService.class);

    private final ProductService productService;
    private final ProductRepository productRepository;

    /**
     * @param productService 用 {@link Lazy} 避開三方循環依賴
     *                      （CategoryService → ProductCascadeService → ProductService → CategoryService）
     */
    public ProductCascadeService(
            @Lazy ProductService productService,
            ProductRepository productRepository) {
        this.productService = productService;
        this.productRepository = productRepository;
    }

    /** 取掛在指定 category（依層級）下的所有 product id。 */
    public List<Long> findProductIdsByCategoryLevel(Long categoryId, int level) {
        if (categoryId == null || level < 1 || level > 3) return List.of();
        return productRepository.findIdsByCategoryLevel(categoryId, level);
    }

    /**
     * 對受影響 product 同事務內逐筆 reflow（02 §5.11.4 / §5.11.5）。
     *
     * @param productIds 受影響 ID
     * @param taskType   log 用的事件名稱（如 {@code CATEGORY_RENAME_CASCADE} / {@code BATCH_CHANGE_CATEGORY}）
     * @return 實際更新筆數
     */
    @Transactional(propagation = Propagation.REQUIRED)
    public int reflow(List<Long> productIds, String taskType) {
        if (productIds == null || productIds.isEmpty()) return 0;
        int updated = productService.reflowSearchableForProductIds(productIds);
        log.info("cascadeReflow", "task=" + taskType + ", affected=" + productIds.size() + ", updated=" + updated);
        return updated;
    }
}
