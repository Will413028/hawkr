package io.github.will413028.hawkr.product.entity.projection;

/**
 * 商品 → SKU 名稱 projection。
 * <p>
 * 用於商品列表填充 SKU 名稱聚合（ProductService 列表查詢）等只需 (productId, skuName)
 * 二元組的場景，避免回傳完整 ProductSkuEntity 但 SQL 只 SELECT 部分欄位的
 * partial-projection 反模式（見 docs/開發規範/Java 微服務開發規範.md §9.2）。
 */
public record SkuNameProjection(Long productId, String skuName) {}
