package io.github.will413028.hawkr.product.constant;

/**
 * Redis Cache Key 常數
 * 依據設計書定義
 */
public final class CacheKeyConstants {
    private CacheKeyConstants() {}

    /**
     * 商品詳情快取 Key 前綴
     * 格式: prod:detail:{product_id}
     */
    public static final String PRODUCT_DETAIL_PREFIX = "prod:detail:";

    /**
     * 商品列表快取 Key 前綴
     * 格式: prod:list:{store_id}:{query_hash}
     */
    public static final String PRODUCT_LIST_PREFIX = "prod:list:";

    /**
     * 分類表整表快取 Key
     * 格式: cat:all
     */
    public static final String CATEGORY_ALL = "cat:all";

    /**
     * 建立商品詳情快取 Key
     *
     * @param productId 商品 ID
     * @return 快取 Key
     */
    public static String detailKey(String productId) {
        return PRODUCT_DETAIL_PREFIX + productId;
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
     * 建立商品列表快取 Key
     */
    public static String productListKey(Long storeId, String queryHash) {
        long storeIdValue = storeId != null ? storeId : 0L;
        return String.format("%s%d:%s", PRODUCT_LIST_PREFIX, storeIdValue, queryHash);
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
