package io.github.will413028.hawkr.product.service.helper;

import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity.ShippingMethodItem;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * product 反正規化欄位聚合規則（02 §5.13a）。
 *
 * 供 CreateProduct / UpdateProduct 在同事務內計算 {@code product.stock_qty} 與
 * {@code product.shipping_methods}，作為 {@link io.github.will413028.hawkr.product.entity.ProductEntity} 反正規化來源。
 *
 * 設計參考：docs/開發規範/Java 微服務開發規範.md §6 helper class 慣例、docs/開發規範/後端服務責任邊界規範.md §五 cascade 同事務原則。
 */
public final class ProductAggregateRules {

    private ProductAggregateRules() {}

    /**
     * stock_qty 聚合：所有 SKU stock_qty 加總（02 §5.13a）。
     *
     * @param skus 全部 SKU 列表（含新增 / 更新後的最終集合）
     * @return 加總後庫存；空集合 → 0
     */
    public static long aggregateStock(List<ProductSkuEntity> skus) {
        if (skus == null || skus.isEmpty()) {
            return 0L;
        }
        long total = 0L;
        for (ProductSkuEntity sku : skus) {
            if (sku == null || sku.getStockQty() == null) continue;
            total += sku.getStockQty();
        }
        return total;
    }

    /**
     * shipping_methods 聯集（02 §5.13a）。
     *
     * 規則：
     * <ul>
     *   <li>去重維度：(type, temperature)</li>
     *   <li>free_threshold 衝突：取 MIN（最寬鬆，行銷友善）；空字串 / "0" 視為「無門檻」，不參與比較</li>
     *   <li>fee_payer 衝突：取 sku.id ASC 第一筆（穩定序）</li>
     *   <li>輸出排序：(type ASC, temperature ASC)</li>
     * </ul>
     *
     * @param skus 全部 SKU 列表
     * @return 聯集後運送方式；空集合 → empty list
     */
    public static List<ShippingMethodItem> mergeShippingMethods(List<ProductSkuEntity> skus) {
        if (skus == null || skus.isEmpty()) {
            return new ArrayList<>();
        }
        // 1. sku.id ASC 排序（穩定序，避免 fee_payer 因順序漂移）
        List<ProductSkuEntity> sorted = new ArrayList<>(skus);
        sorted.sort(Comparator.comparing(
            ProductSkuEntity::getId,
            Comparator.nullsLast(Comparator.naturalOrder())
        ));

        // 2. 按 (type, temperature) 聚合
        Map<String, ShippingMethodItem> merged = new LinkedHashMap<>();
        for (ProductSkuEntity sku : sorted) {
            if (sku == null || sku.getShippingMethod() == null) continue;
            for (ShippingMethodItem m : sku.getShippingMethod()) {
                if (m == null || isBlank(m.getType()) || isBlank(m.getTemperature())) continue;
                String key = m.getType() + "|" + m.getTemperature();
                ShippingMethodItem existing = merged.get(key);
                if (existing == null) {
                    merged.put(key, copy(m));  // 第一次出現的 fee_payer 勝出
                } else {
                    mergeFreeThreshold(existing, m);
                    // fee_payer 保持 existing（已是 sku.id 最小者）
                }
            }
        }

        // 3. 輸出排序 (type ASC, temperature ASC)
        List<ShippingMethodItem> result = new ArrayList<>(merged.values());
        result.sort(Comparator
            .comparing(ShippingMethodItem::getType, Comparator.nullsLast(Comparator.naturalOrder()))
            .thenComparing(ShippingMethodItem::getTemperature, Comparator.nullsLast(Comparator.naturalOrder())));
        return result;
    }

    private static void mergeFreeThreshold(ShippingMethodItem existing, ShippingMethodItem incoming) {
        Long curr = parseThreshold(existing.getFreeThreshold());
        Long inc  = parseThreshold(incoming.getFreeThreshold());
        if (inc == null) return;                 // incoming 無門檻，不影響
        if (curr == null || inc < curr) {        // existing 無門檻 → 採 inc；inc 較低 → 採 inc
            existing.setFreeThreshold(incoming.getFreeThreshold());
        }
    }

    private static Long parseThreshold(String s) {
        if (isBlank(s)) return null;
        try {
            long v = Long.parseLong(s);
            return v > 0 ? v : null;       // <= 0 視為「無門檻」
        } catch (NumberFormatException ignored) {
            return null;
        }
    }

    private static ShippingMethodItem copy(ShippingMethodItem src) {
        ShippingMethodItem c = new ShippingMethodItem();
        c.setType(src.getType());
        c.setTemperature(src.getTemperature());
        c.setFreeThreshold(src.getFreeThreshold());
        c.setFeePayer(src.getFeePayer());
        return c;
    }

    private static boolean isBlank(String s) {
        return s == null || s.isEmpty();
    }
}
