package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.util.IdGenerator;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.StockTxPhase;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.entity.projection.RestockTransitionProjection;
import io.github.will413028.hawkr.product.entity.projection.SkuProductRef;
import io.github.will413028.hawkr.product.mq.ProductEventPublisher;
import io.github.will413028.hawkr.product.mq.dto.ProductRestockPayload;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.repository.StockTransactionRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;
import java.util.stream.Collectors;

/**
 * 庫存操作 Service
 *
 * 提供原子性庫存扣減/歸還，搭配冪等性保證（stock_tx_id）
 */
@Service
public class StockService {

    private static final ObsLogger log = ObsLogger.forClass(StockService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductSkuRepository skuRepository;
    private final StockTransactionRepository stockTxRepository;
    private final ProductRepository productRepository;
    private final ProductCacheService cacheService;
    private final ProductEventPublisher productEventPublisher;

    private final ProductRevalidationService productRevalidationService;
    private final io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;

    public StockService(ProductSkuRepository skuRepository,
                        StockTransactionRepository stockTxRepository,
                        ProductRepository productRepository,
                        ProductCacheService cacheService,
                        ProductEventPublisher productEventPublisher,
                        ProductRevalidationService productRevalidationService,
                        io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository) {
        this.skuRepository = skuRepository;
        this.stockTxRepository = stockTxRepository;
        this.productRepository = productRepository;
        this.cacheService = cacheService;
        this.productEventPublisher = productEventPublisher;
        this.productRevalidationService = productRevalidationService;
        this.storeExposureRepository = storeExposureRepository;
    }

    /**
     * 對庫存變動波及到的 product 觸發 frontend cache bust（後端服務責任邊界規範 §9.3）。
     * 庫存 0↔有貨改變 storefront 商品卡的可購買狀態，但不改變「賣家有哪些公開商品」
     * → 只發 {@code product:<id>}×N，列表面由 web expansion（{@code product:<id> → products:list}）
     * 涵蓋；不共發 {@code seller:<id>}（賣家層 surface 不嵌商品欄位，共發只會霰彈失效推薦賣家 rail）。
     */
    private void revalidateAffectedProducts(List<StockItemParam> sortedItems) {
        try {
            List<Long> skuIds = sortedItems.stream().map(StockItemParam::skuId).toList();
            List<SkuProductRef> refs = skuRepository.findSkuProductRefs(skuIds);
            java.util.Set<Long> productIds = new java.util.LinkedHashSet<>();
            for (SkuProductRef r : refs) {
                if (r.productId() != null) productIds.add(r.productId());
            }
            productRevalidationService.revalidateProducts(productIds);
        } catch (Exception e) {
            log.warn("StockService.revalidate.skip", "skip bust on " + e.getMessage());
        }
    }

    /**
     * 庫存項目參數
     */
    public record StockItemParam(Long skuId, int quantity) {}

    /**
     * 庫存操作結果
     */
    public enum StockResult {
        SUCCESS,
        INSUFFICIENT_STOCK,
        ALREADY_APPLIED,
        NOOP_NOT_FOUND,
        SYSTEM_ERROR
    }

    /**
     * 原子扣減庫存（全部成功或全部失敗）
     *
     * 冪等性：相同 (stockTxId, DEDUCT) 重複呼叫回傳 ALREADY_APPLIED
     * 死鎖防護：按 skuId 升序加鎖
     */
    @Transactional
    public StockResult deductStock(String stockTxId, List<StockItemParam> items) {
        if (stockTxRepository.existsByStockTxIdAndOperation(stockTxId, StockTxPhase.DEDUCT.value())) {
            log.info("StockService.deductStock.idempotent", "重複請求，忽略", "stockTxId", stockTxId);
            return StockResult.ALREADY_APPLIED;
        }

        List<StockItemParam> sorted = items.stream()
                .sorted(Comparator.comparingLong(StockItemParam::skuId))
                .toList();

        for (StockItemParam item : sorted) {
            int affected = skuRepository.deductStock(item.skuId(), item.quantity());
            if (affected == 0) {
                log.warn("StockService.deductStock.insufficient",
                        "庫存不足", "skuId", String.valueOf(item.skuId()));
                throw new InsufficientStockException(item.skuId(), item.quantity());
            }
        }

        cascadeProductStock(sorted, -1);

        String payload = toJson(sorted);
        stockTxRepository.insertIfAbsent(stockTxId, StockTxPhase.DEDUCT.value(), payload);

        revalidateAffectedProducts(sorted);

        log.info("StockService.deductStock.success", "庫存扣減成功",
                "stockTxId", stockTxId);
        return StockResult.SUCCESS;
    }

    /**
     * 原子歸還庫存（補償/取消）
     *
     * 冪等性：相同 (stockTxId, RESTORE) 重複呼叫回傳 ALREADY_APPLIED
     */
    @Transactional
    public StockResult restoreStock(String stockTxId, List<StockItemParam> items) {
        if (stockTxRepository.existsByStockTxIdAndOperation(stockTxId, StockTxPhase.RESTORE.value())) {
            log.info("StockService.restoreStock.idempotent", "重複請求，忽略", "stockTxId", stockTxId);
            return StockResult.ALREADY_APPLIED;
        }

        if (!stockTxRepository.existsByStockTxIdAndOperation(stockTxId, StockTxPhase.DEDUCT.value())) {
            log.warn("StockService.restoreStock.noDeduct",
                    "無對應 DEDUCT 記錄，視為補償成功 NOOP", "stockTxId", stockTxId);
            return StockResult.NOOP_NOT_FOUND;
        }

        List<StockItemParam> sorted = items.stream()
                .sorted(Comparator.comparingLong(StockItemParam::skuId))
                .toList();

        for (StockItemParam item : sorted) {
            int affected = skuRepository.restoreStock(item.skuId(), item.quantity());
            if (affected == 0) {
                log.warn("StockService.restoreStock.notFound",
                        "SKU 不存在", "skuId", String.valueOf(item.skuId()));
            }
        }

        cascadeProductStock(sorted, +1);

        String payload = toJson(sorted);
        stockTxRepository.insertIfAbsent(stockTxId, StockTxPhase.RESTORE.value(), payload);

        revalidateAffectedProducts(sorted);

        log.info("StockService.restoreStock.success", "庫存歸還成功",
                "stockTxId", stockTxId);
        return StockResult.SUCCESS;
    }

    /**
     * 反正規化欄位 cascade（02 §3.2 / §5.8）：將 SKU 層級 stock_qty 變動同步到 product 層級。
     * <p>
     * 按 productId ASC 排序避免多訂單併發死鎖（docs/開發規範/後端服務責任邊界規範.md §三）。
     *
     * @param items SKU 變動清單（已按 skuId ASC 排序）
     * @param sign  +1（restore）或 -1（deduct）
     */
    private void cascadeProductStock(List<StockItemParam> items, int sign) {
        List<Long> skuIds = items.stream().map(StockItemParam::skuId).toList();
        List<SkuProductRef> skus = skuRepository.findSkuProductRefs(skuIds);
        Map<Long, Long> skuToProduct = skus.stream()
                .collect(Collectors.toMap(SkuProductRef::skuId, SkuProductRef::productId));

        TreeSet<Long> productIds = new TreeSet<>();
        for (StockItemParam item : items) {
            Long productId = skuToProduct.get(item.skuId());
            if (productId == null) continue;
            productIds.add(productId);
        }
        if (productIds.isEmpty()) return;

        Map<Long, RestockTransitionProjection> projectionMap;
        if (sign > 0) {
            projectionMap = productRepository.findRestockTransitionFields(List.copyOf(productIds)).stream()
                    .collect(Collectors.toMap(RestockTransitionProjection::productId, p -> p));
        } else {
            projectionMap = Map.of();
        }

        for (Long productId : productIds) {
            productRepository.reaggregateStockFromSku(productId);

            RestockTransitionProjection proj = projectionMap.get(productId);
            if (proj != null
                    && proj.stockQty() != null && proj.stockQty() == 0L
                    && ProductStatus.ACTIVE.value().equals(proj.status())
                    && proj.deletedAt() == null
                    && storeExposureRepository.isExposable(proj.storeId())) {
                Long newStock = productRepository.findStockQtyById(productId);
                if (newStock != null && newStock > 0L) {
                    productEventPublisher.publishRestockAfterCommit(buildRestockPayload(proj));
                }
            }
        }

        List<Long> ownerIds = productRepository.findProductSellerRefs(List.copyOf(productIds)).stream()
                .map(ProductSellerRef::sellerId)
                .filter(java.util.Objects::nonNull)
                .toList();

        registerCacheEvictionAfterCommit(productIds, ownerIds);
    }

    /**
     * 將受影響 product 的快取失效（detail + list）註冊為 afterCommit callback（#1455）。
     * <p>
     * 捕獲 productIds / ownerIds 為區域 final 快照（避免跨交易污染）；於交易 commit 後執行一次
     * detail evict + list batch evict。快取清除失敗不影響業務（僅記 warn）。
     * <p>
     * 防禦：庫存寫路徑皆 {@code @Transactional}，理論上必有 active synchronization；若無（
     * {@code isSynchronizationActive()==false}）則 fallback 直接 evict，避免快取靜默陳舊。
     */
    private void registerCacheEvictionAfterCommit(java.util.Collection<Long> productIds, List<Long> ownerIds) {
        final List<Long> productIdsSnapshot = List.copyOf(productIds);
        final List<Long> ownerIdsSnapshot = List.copyOf(ownerIds);

        Runnable evictTask = () -> {
            try {
                productIdsSnapshot.forEach(pid -> cacheService.evictProductDetail(pid.toString()));
                cacheService.evictProductListForWriteBatch(ownerIdsSnapshot);
                log.info("StockService.evictCache.success",
                        "庫存快取清除完成（afterCommit），productCount=" + productIdsSnapshot.size());
            } catch (Exception e) {
                log.warn("StockService.evictCache.failed",
                        "庫存快取清除失敗（不影響業務）: " + e.getMessage());
            }
        };

        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    evictTask.run();
                }
            });
        } else {
            evictTask.run();
        }
    }

    /**
     * 將 transition projection 組裝為 product.restock MQ payload（02 §5.13b）。
     * imageKey 帶商品主圖 objectKey（49 §6.3，顯示 URL 由消費端組）；無圖則為空字串。
     */
    private ProductRestockPayload buildRestockPayload(RestockTransitionProjection proj) {
        String imageKey = proj.primaryObjectKey() == null ? "" : proj.primaryObjectKey();
        return new ProductRestockPayload(
                IdGenerator.uuidV4Lower(),
                ProductEventType.PRODUCT_RESTOCK.value(),
                String.valueOf(proj.productId()),
                proj.name(),
                imageKey,
                proj.storeId(),
                DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()));
    }

    /**
     * 批量查詢庫存
     */
    public Map<Long, Integer> getStock(List<Long> skuIds) {
        List<SkuProductRef> skus = skuRepository.findSkuProductRefs(skuIds);
        return skus.stream()
                .collect(Collectors.toMap(
                        SkuProductRef::skuId,
                        s -> s.stockQty() != null ? s.stockQty() : 0
                ));
    }

    private String toJson(List<StockItemParam> items) {
        try {
            return MAPPER.writeValueAsString(items);
        } catch (JsonProcessingException e) {
            return "[]";
        }
    }

    /**
     * 庫存不足例外（觸發 @Transactional 回滾）
     */
    public static class InsufficientStockException extends RuntimeException {
        private final Long skuId;
        private final int requestedQuantity;

        public InsufficientStockException(Long skuId, int requestedQuantity) {
            super("SKU " + skuId + " 庫存不足，請求數量: " + requestedQuantity);
            this.skuId = skuId;
            this.requestedQuantity = requestedQuantity;
        }

        public Long getSkuId() { return skuId; }
        public int getRequestedQuantity() { return requestedQuantity; }
    }
}
