package io.github.will413028.hawkr.productimport.entity.projection;

/**
 * Partial projection — 即時計算 item 的 SKU 價格範圍（§3.10）。
 */
public record SkuPriceRangeProjection(
        Long importItemId,
        Long priceMin,
        Long priceMax) {
}
