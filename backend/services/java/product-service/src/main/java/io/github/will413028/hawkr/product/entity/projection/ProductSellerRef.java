package io.github.will413028.hawkr.product.entity.projection;

/**
 * 商品 → 所屬賣家（store_id）反查 projection。
 *
 * <p>用於 frontend cache revalidate（後端服務責任邊界規範 §9.3）：批次 mutation
 * 影響多筆商品、可能橫跨多賣家時，反查每筆的 sellerId 以組出 {@code product:<id>} +
 * {@code seller:<sellerId>} 完整 bust 清單。避免回傳完整 Entity（docs/開發規範/Java 微服務開發規範.md §9.2）。
 */
public record ProductSellerRef(Long productId, Long sellerId) {}
