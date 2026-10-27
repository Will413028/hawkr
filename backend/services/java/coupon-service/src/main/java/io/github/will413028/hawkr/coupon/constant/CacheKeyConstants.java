package io.github.will413028.hawkr.coupon.constant;

/**
 * Redis Key 常數定義
 */
public final class CacheKeyConstants {

    /** Template 詳情：coupon:tpl:{templateId}，TTL 30 分鐘 */
    public static final String TEMPLATE_PREFIX = "coupon:tpl:";

    // TODO(#1426): coupon:avail 快取目前為 dead code —— availableListKey / AVAILABLE_LIST_PREFIX
    // 無任何 put / get caller，僅 CouponCacheService 的 evict-all（AVAILABLE_LIST_PATTERN）會掃到（永遠掃空）。
    // 待確認是否曾規劃可領取列表快取；若確定不啟用，後續連同 evict-all 一併移除（暫不刪以免誤傷）。
    /** 可領取列表：coupon:avail:{queryHash}，TTL 5 分鐘 */
    public static final String AVAILABLE_LIST_PREFIX = "coupon:avail:";

    /** Template 快取 TTL（30 分鐘，秒） */
    public static final long TEMPLATE_TTL_SECONDS = 30L * 60;

    /** 可領取列表快取 TTL（5 分鐘，秒） */
    public static final long AVAILABLE_LIST_TTL_SECONDS = 5L * 60;

    /** 可領取列表 key pattern（用於批次清除） */
    public static final String AVAILABLE_LIST_PATTERN = "coupon:avail:*";

    private CacheKeyConstants() {}

    public static String templateKey(Long templateId) {
        return TEMPLATE_PREFIX + templateId;
    }

    public static String availableListKey(String queryHash) {
        return AVAILABLE_LIST_PREFIX + queryHash;
    }
}
