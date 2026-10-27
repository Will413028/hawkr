package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 購物車內單筆 SKU 「不可購買」的原因。由 `common-core/utils/code/helper/ProductPurchasabilityRules.evaluate(...)` 從 `is_active` / `stock_qty` / `requested_qty` 推導，供 cart-service `GetCart`、order-service `CheckoutValidate` / `CheckoutPreview` / `CheckoutConfirm` 統一回傳。詳見 [04 §5.2](../詳細設計書/04-購物車微服務（Cart%20Service）詳細設計書.md#52-查詢購物車-getcart---lazy-loading--grouping) 與 [20 §4.14](../詳細設計書/20-結帳模組（Checkout%20Module）詳細設計書.md)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CartItemUnavailableReason implements CodeMeta {

    DELISTED("DELISTED", "DELISTED", "已下架", "商品已被賣家下架或軟刪除（`product.status != ACTIVE` 或 `deleted_at IS NOT NULL`），不會自動恢復"),
    OUT_OF_STOCK("OUT_OF_STOCK", "OUT_OF_STOCK", "已售完", "SKU 即時 `stock_qty <= 0`，賣家補貨後下次查詢自動恢復"),
    INSUFFICIENT_STOCK("INSUFFICIENT_STOCK", "INSUFFICIENT_STOCK", "庫存不足", "`0 < stock_qty < requested_qty`（庫存有但不夠勾選量）；買家可手動調降數量重試"),
    LOOKUP_FAILED("LOOKUP_FAILED", "LOOKUP_FAILED", "暫時無法確認", "product-service 查詢失敗（cache miss + RPC timeout / SkuCartInfo 為 null）；前端文案應引導買家稍後再試而非「下架」");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CartItemUnavailableReason(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, CartItemUnavailableReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CartItemUnavailableReason::code, e -> e));

    private static final Map<String, CartItemUnavailableReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CartItemUnavailableReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CartItemUnavailableReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CartItemUnavailableReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
