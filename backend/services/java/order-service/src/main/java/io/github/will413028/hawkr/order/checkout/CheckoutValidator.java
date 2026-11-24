package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import org.springframework.stereotype.Component;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 結帳驗證器
 *
 * 驗證：賣家一致性、溫層交集
 */
@Component
public class CheckoutValidator {

    /**
     * 執行結帳驗證
     *
     * @param items 已解析的結帳商品列表
     * @return 驗證結果
     */
    public ValidationResult validate(List<ResolvedItem> items) {
        List<ValidationError> errors = new ArrayList<>();

        if (items == null || items.isEmpty()) {
            errors.add(new ValidationError("EMPTY_ITEMS", "結帳商品不可為空"));
            return new ValidationResult(false, Set.of(), errors);
        }

        // 1. 賣家一致性
        long firstSeller = items.get(0).storeId();
        boolean sameSeller = items.stream().allMatch(i -> i.storeId() == firstSeller);
        if (!sameSeller) {
            errors.add(new ValidationError("DIFFERENT_SELLER", "結帳商品必須來自同一賣家"));
            return new ValidationResult(false, Set.of(), errors);
        }

        // 庫存 / 下架 / lookup 失敗檢查移到 caller（CheckoutService）以 ProductPurchasabilityRules
        // 統一收集進 unavailable_items[]（20 §4.14 / §4.15）。Validator 內不再收 row 級
        // PRODUCT_UNAVAILABLE。

        // 2. 溫層交集（每個 SKU 支援的配送溫層取交集）
        Set<String> tempIntersection = null;
        for (ResolvedItem item : items) {
            Set<String> itemTemps = item.shippingOptions().stream()
                    .map(SkuShippingOption::temperatureCode)
                    .collect(Collectors.toSet());
            if (tempIntersection == null) {
                tempIntersection = new HashSet<>(itemTemps);
            } else {
                tempIntersection.retainAll(itemTemps);
            }
        }
        if (tempIntersection == null || tempIntersection.isEmpty()) {
            errors.add(new ValidationError("TEMP_ZONE_INCOMPATIBLE", "商品的配送溫層不相容，無法合併配送"));
            return new ValidationResult(false, Set.of(), errors);
        }

        return new ValidationResult(true, tempIntersection, List.of());
    }

    // ==================== 內部資料結構 ====================

    /**
     * 已解析的結帳商品（由 CheckoutService 從 Product Service 回應轉換而來）
     */
    public record ResolvedItem(
            long skuId,
            String skuName,
            int quantity,
            long unitPrice,        // 實際成交單價（無特價=listPrice，有特價=當時 sale_price）
            long listPrice,        // 原價快照（永遠 = SKU.price）
            long stockQty,
            long storeId,
            String productId,
            String productName,
            List<SkuShippingOption> shippingOptions,
            String imageObjectKey,
            boolean isActive        // product.status == ACTIVE 且 deleted_at IS NULL（02 §5.5.2）。
                                    // 「不可購買」攔截由 caller 在 ProductPurchasabilityRules 階段收進 unavailable_items。
    ) {
        /**
         * 推斷存儲溫層（供 Preview 顯示）
         * AMBIENT 配送 → 產品為常溫（只有常溫產品可用常溫配送）
         * 僅 CHILLED → 冷藏產品
         * 僅 FROZEN → 冷凍產品
         */
        public String inferredTemperature() {
            boolean hasAmbient = shippingOptions.stream()
                    .anyMatch(o -> ShippingTemperature.AMBIENT.value().equals(o.temperatureCode()));
            if (hasAmbient) return ShippingTemperature.AMBIENT.value();
            boolean hasChilled = shippingOptions.stream()
                    .anyMatch(o -> ShippingTemperature.CHILLED.value().equals(o.temperatureCode()));
            if (hasChilled) return ShippingTemperature.CHILLED.value();
            return ShippingTemperature.FROZEN.value();
        }
    }

    /** 商品支援的配送溫層（溫層欄位屬於 B4／B6，這裡只保留溫層）。 */
    public record SkuShippingOption(
            String temperatureCode
    ) {}

    public record ValidationResult(
            boolean valid,
            Set<String> deliveryTempIntersection,
            List<ValidationError> errors
    ) {}

    public record ValidationError(
            String errorCode,
            String errorMessage
    ) {}

    /**
     * row 級不可購買項目（CartItemUnavailableReason）。
     * 對應 order.proto UnavailableItem，由 caller 在 resolveItems 階段透過
     * {@link io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules} 推導。
     * 20 設計書 §3.2 / §3.4 / §3.5。
     */
    public record UnavailableItemInfo(
            String skuId,
            String skuName,
            String reason,        // CartItemUnavailableReason value
            String requestedQty,
            String availableQty   // INSUFFICIENT_STOCK 時 0<available<requested；OUT_OF_STOCK 時 "0"；其他 ""
    ) {}

    /**
     * resolveItems 的回傳：含可組裝 validator 的 resolved items 與 row 級 unavailable items。
     * LOOKUP_FAILED 的 item 不進 resolved（缺 shipping_methods，無法跑 validator），
     * 純 unavailable_items 表達；其餘（DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK）的 item
     * 同樣不進 resolved。
     */
    public record ResolveResult(
            List<ResolvedItem> resolved,
            List<UnavailableItemInfo> unavailable
    ) {}
}
