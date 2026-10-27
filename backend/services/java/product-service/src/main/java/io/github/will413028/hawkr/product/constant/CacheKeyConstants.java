package io.github.will413028.hawkr.product.constant;

/**
 * Redis Cache Key 常數
 * 依據設計書定義
 */
public final class CacheKeyConstants {
    private CacheKeyConstants() {}

    /**
     * 商品詳情快取 Key 前綴
     * 格式: prod:detail:{product_id}:{onboarding_sig}（02 §2.7.2）
     */
    public static final String PRODUCT_DETAIL_PREFIX = "prod:detail:";

    /**
     * 商品列表快取 Key 前綴
     * 格式: prod:list:{store_id}:{query_hash}:{list_onboarding_sig}（02 §2.7.2）
     */
    public static final String PRODUCT_LIST_PREFIX = "prod:list:";

    /**
     * 分類表整表快取 Key
     * 格式: cat:all
     */
    public static final String CATEGORY_ALL = "cat:all";

    /**
     * 建立商品詳情快取 Key（含 onboarding 指紋）
     *
     * @param productId 商品 ID
     * @param onboardingSig 賣家 onboarding 指紋（{@code SellerShippingFilter.onboardingSig}）
     * @return 快取 Key
     */
    public static String detailKey(String productId, String onboardingSig) {
        return PRODUCT_DETAIL_PREFIX + productId + ":" + onboardingSig;
    }

    /**
     * 商品詳情 wildcard pattern（按 productId 批次清除，無論 onboarding_sig）
     */
    public static String detailKeyPattern(String productId) {
        return PRODUCT_DETAIL_PREFIX + productId + ":*";
    }

    /**
     * 取得列表快取的 pattern（用於批次刪除）
     *
     * @param storeId 賣家 store 代理鍵（可為 null）
     * @return 快取 Key Pattern
     */
    public static String listKeyPattern(Long storeId) {
        if (storeId != null) {
            return PRODUCT_LIST_PREFIX + storeId + ":*";
        }
        return PRODUCT_LIST_PREFIX + "*";
    }

    /**
     * 建立商品詳情快取 Key（別名方法）
     */
    public static String productDetailKey(String productId, String onboardingSig) {
        return detailKey(productId, onboardingSig);
    }

    /**
     * 建立商品列表快取 Key（含 list onboarding 指紋；02 §2.7.2）
     */
    public static String productListKey(Long storeId, String queryHash, String listOnboardingSig) {
        long storeIdValue = storeId != null ? storeId : 0L;
        return String.format("%s%d:%s:%s", PRODUCT_LIST_PREFIX, storeIdValue, queryHash, listOnboardingSig);
    }

    /**
     * 建立公開產地聚合快取 Key（02 §5.13）。
     * 格式：prod:origins:summary:{categoryId|"all"}:{limit}
     */
    public static String productOriginsSummaryKey(Long categoryId, int limit) {
        return String.format("prod:origins:summary:%s:%d",
                categoryId != null ? categoryId : "all", limit);
    }

    /**
     * 取得賣家 store 的列表快取 pattern
     */
    public static String productListPattern(Long storeId) {
        return listKeyPattern(storeId);
    }
}
