package io.github.will413028.hawkr.coupon.constant;

/**
 * Redis Key 常數定義
 */
public final class CacheKeyConstants {

    /** Template 詳情：coupon:tpl:{templateId}，TTL 30 分鐘 */
    public static final String TEMPLATE_PREFIX = "coupon:tpl:";

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
