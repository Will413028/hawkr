package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.CartItemUnavailableReason;

/**
 * §8.5 商品可購買性判定 helper（跨 cart-service / order-service 共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 從 SKU 即時 is_active / stock_qty / 買家勾選量推導
 * {@link CartItemUnavailableReason}，供：
 * <ul>
 *   <li>cart-service GetCart 組裝 CartItemView.purchasable / unavailable_reason</li>
 *   <li>order-service CheckoutValidate / Preview / Confirm 收集 unavailable_items[]</li>
 * </ul>
 * 集中實作避免跨服務判定漂移（詳見 20 設計書 §4.14 / §4.15）。
 * <p>
 * <b>判定鏈執行順序</b>（程式碼層面，先命中先回，非業務嚴重度）：
 * <ol>
 *   <li>LOOKUP_FAILED — caller 在拿到 null SkuCartInfo / ProductForCheckoutInfo 時呼叫
 *       {@link #lookupFailed()} 處理；helper 本身不處理 null（簽章為 primitive）</li>
 *   <li>DELISTED — {@code !isActive}</li>
 *   <li>OUT_OF_STOCK — {@code stockQty <= 0}</li>
 *   <li>INSUFFICIENT_STOCK — {@code 0 < stockQty < requestedQty}</li>
 *   <li>{@link Purchasability#OK} — 可購買</li>
 * </ol>
 */
public final class ProductPurchasabilityRules {

    private ProductPurchasabilityRules() {
    }

    /**
     * 可購買性判定結果。
     *
     * @param purchasable 本筆 row 是否可購買
     * @param reason      不可購買原因（{@link CartItemUnavailableReason#value()}）；可購買時為空字串
     */
    public record Purchasability(boolean purchasable, String reason) {

        /** 可購買（purchasable=true, reason=""）。 */
        public static final Purchasability OK = new Purchasability(true, "");

        /** 以指定 reason 建立「不可購買」結果。 */
        public static Purchasability of(CartItemUnavailableReason r) {
            return new Purchasability(false, r.value());
        }
    }

    /**
     * 從 SKU 即時欄位推導可購買性。
     * <p>
     * caller 須先處理 lookup 失敗（null SkuCartInfo / ProductForCheckoutInfo / batch
     * response 缺 key）並改呼叫 {@link #lookupFailed()}；本方法假設 caller 已拿到合法 SKU 資料。
     *
     * @param isActive     product.status == ACTIVE 且 deleted_at IS NULL
     * @param stockQty     SKU 即時庫存
     * @param requestedQty 買家當下勾選量（cart_items.quantity）
     */
    public static Purchasability evaluate(boolean isActive, long stockQty, long requestedQty) {
        if (!isActive) {
            return Purchasability.of(CartItemUnavailableReason.DELISTED);
        }
        if (stockQty <= 0) {
            return Purchasability.of(CartItemUnavailableReason.OUT_OF_STOCK);
        }
        if (stockQty < requestedQty) {
            return Purchasability.of(CartItemUnavailableReason.INSUFFICIENT_STOCK);
        }
        return Purchasability.OK;
    }

    /**
     * Product Service lookup 失敗 / batch response 缺 key 時使用。
     * <p>
     * 對應判定鏈第 1 步：LOOKUP_FAILED 在程式判定鏈中最先命中，避免在 null 上判 is_active / stock_qty
     * 造成 NPE。業務嚴重度上 LOOKUP_FAILED 低於 DELISTED（前端文案需引導買家稍後重試而非「下架」）。
     */
    public static Purchasability lookupFailed() {
        return Purchasability.of(CartItemUnavailableReason.LOOKUP_FAILED);
    }
}
