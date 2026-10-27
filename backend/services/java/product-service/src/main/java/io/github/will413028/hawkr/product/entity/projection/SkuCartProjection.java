package io.github.will413028.hawkr.product.entity.projection;

import java.time.LocalDateTime;

/**
 * 購物車批次查詢 projection（02 §5.5.3）。
 * <p>
 * 透過單階段 LEFT JOIN 一次拿 SKU 與其 product 的必要欄位（一個 PG snapshot），
 * 取代既有「findSkusByIds + findProductsByIds」兩階段非交易查詢，避免並行
 * {@code UpdateProductStatus} / {@code DeductStock} 期間出現 sku/product
 * snapshot 不一致的 race window。
 * <p>
 * 對應 SQL：
 * <pre>
 * SELECT s.id, s.product_id, s.stock_qty, s.price, s.sale_price,
 *        s.shipping_method::text AS shipping_method_json,
 *        p.status, p.deleted_at
 * FROM product_sku s
 * LEFT JOIN product p ON p.id = s.product_id
 * WHERE s.id IN (...)
 * </pre>
 * <p>
 * 當 sku 找得到但 product 找不到（{@code productStatus} 為 null）時，代表 product
 * orphan（data integrity issue：sku.product_id 指向不存在 / 已 hard-delete 的 product）。
 * caller 必須**從 response map 排除該 sku**（caller 走 LOOKUP_FAILED），**禁止**
 * 把 product 缺失靜默 collapse 成 {@code is_active=false}（會產生假 DELISTED 回給買家）。
 */
public record SkuCartProjection(
        Long skuId,
        Long productId,
        Integer stockQty,
        Long price,
        Long salePrice,
        String shippingMethodJson,
        String productStatus,
        LocalDateTime productDeletedAt
) {}
