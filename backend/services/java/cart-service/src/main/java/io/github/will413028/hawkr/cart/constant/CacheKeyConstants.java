package io.github.will413028.hawkr.cart.constant;

/**
 * Redis Key 常數定義（設計書 §3.2）
 */
public final class CacheKeyConstants {

    /** 會員購物車 Hash：cart:b:{buyerId}，Field=skuId，Value=JSON。字尾 b=buyer */
    public static final String CART_BUYER_PREFIX = "cart:b:";

    /** 訪客購物車 Hash：cart:g:{guestId}，Field=skuId，Value=JSON */
    public static final String CART_GUEST_PREFIX = "cart:g:";

    /** 賣家資訊快取 String：seller:meta:{storeId} */
    public static final String SELLER_META_PREFIX = "seller:meta:";

    /** MergeCart 互斥鎖：merge_lock:{guestId}:{buyerId} */
    public static final String MERGE_LOCK_PREFIX = "merge_lock:";

    /** 購物車 Hash TTL（7 天，秒） */
    public static final long CART_TTL_SECONDS = 7L * 24 * 60 * 60;

    /**
     * 賣家資訊 TTL（1 天，秒）—— safety net。
     * <p>一致性主要靠 <b>event-driven evict</b>：seller-service 改店名 / logo 後 emit
     * {@code seller.profile.changed}，cart-service consumer 主動 evict {@code seller:meta:{id}}
     * （#1426 方案 A）。此 TTL 僅作為 event 遺失 / consumer 停機時的最終保險（miss 時經 gRPC lazy 回填）。
     */
    public static final long SELLER_META_TTL_SECONDS = 24L * 60 * 60;

    /** MergeCart 互斥鎖 TTL（10 秒） */
    public static final long MERGE_LOCK_TTL_SECONDS = 10L;

    private CacheKeyConstants() {}

    public static String cartKey(Long buyerId, String guestId) {
        if (buyerId != null) {
            return CART_BUYER_PREFIX + buyerId;
        }
        return CART_GUEST_PREFIX + guestId;
    }

    public static String sellerMetaKey(Long storeId) {
        return SELLER_META_PREFIX + storeId;
    }

    public static String mergeLockKey(String guestId, Long buyerId) {
        return MERGE_LOCK_PREFIX + guestId + ":" + buyerId;
    }
}
