package io.github.will413028.hawkr.product.entity.projection;

/**
 * SKU → product 反查 projection。
 * <p>
 * 用於庫存 cascade 與 cache eviction（StockService.cascadeProductStock）等
 * 只需取 SKU 對應 product_id 與
 * 庫存值的場景，避免回傳完整 ProductSkuEntity 但 SQL 只 SELECT 部分欄位的
 * partial-projection 反模式（見 docs/dev-guidelines/java-microservices-development-guidelines.md §9.2）。
 */
public record SkuProductRef(Long skuId, Long productId, Integer stockQty) {}
