package io.github.will413028.hawkr.product.domain.text;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;

import java.nio.charset.StandardCharsets;
import java.util.List;

/**
 * Searchable_text 拼接器（02 設計書 §3.9）。
 *
 * <p>拼接優先序（總長超過 8 KB 時僅可截 (5) {@code pure_description} 尾段）：
 * <ol>
 *   <li>name（必保）</li>
 *   <li>所有 hashtag canonical name（依 position 升序，必保）</li>
 *   <li>所有 product_sku.sku_name（必保）</li>
 *   <li>cat_l1_name + cat_l2_name + cat_l3_name（必保）</li>
 *   <li>pure_description（可截斷尾段）</li>
 * </ol>
 *
 * <p>單空格分隔，UTF-8 編碼後總長 ≤ 8 KB；截斷時記 INFO log。
 */
public final class SearchableTextBuilder {

    public static final int MAX_BYTES = 8192;

    private static final ObsLogger log = ObsLogger.forClass(SearchableTextBuilder.class);

    private SearchableTextBuilder() {}

    public static String build(
            Long productId,
            String name,
            List<String> hashtagCanonicalNames,
            List<String> skuNames,
            String catL1Name,
            String catL2Name,
            String catL3Name,
            String pureDescription) {

        StringBuilder mandatory = new StringBuilder();
        appendIfNotEmpty(mandatory, name);
        if (hashtagCanonicalNames != null) {
            for (String h : hashtagCanonicalNames) appendIfNotEmpty(mandatory, h);
        }
        if (skuNames != null) {
            for (String s : skuNames) appendIfNotEmpty(mandatory, s);
        }
        appendIfNotEmpty(mandatory, catL1Name);
        appendIfNotEmpty(mandatory, catL2Name);
        appendIfNotEmpty(mandatory, catL3Name);

        String mandatoryPart = mandatory.toString();
        int mandatoryBytes = mandatoryPart.getBytes(StandardCharsets.UTF_8).length;

        String pureDesc = pureDescription == null ? "" : pureDescription.trim();
        if (pureDesc.isEmpty()) {
            return mandatoryPart;
        }

        // 必保部分加一個空格與 pureDesc 起始
        int reserved = mandatoryBytes + (mandatoryBytes > 0 ? 1 : 0);
        int budget = MAX_BYTES - reserved;
        if (budget <= 0) {
            // 必保部分已逼近或超過上限：不夾帶 pure_description
            return mandatoryPart;
        }

        byte[] descBytes = pureDesc.getBytes(StandardCharsets.UTF_8);
        if (descBytes.length <= budget) {
            return mandatoryPart.isEmpty() ? pureDesc : mandatoryPart + " " + pureDesc;
        }

        // 截斷：依 budget 切到 char 邊界（避免切壞 UTF-8 多 byte 字元）
        String truncated = truncateToByteBudget(pureDesc, budget);
        int truncatedBytes = truncated.getBytes(StandardCharsets.UTF_8).length;
        log.info("searchableTextTruncated",
                "product_id=" + productId
                        + ", original_bytes=" + descBytes.length
                        + ", truncated_bytes=" + truncatedBytes
                        + ", removed_bytes=" + (descBytes.length - truncatedBytes));
        return mandatoryPart.isEmpty() ? truncated : mandatoryPart + " " + truncated;
    }

    private static void appendIfNotEmpty(StringBuilder sb, String s) {
        if (s == null) return;
        String t = s.trim();
        if (t.isEmpty()) return;
        if (sb.length() > 0) sb.append(' ');
        sb.append(t);
    }

    /** 將字串截到 UTF-8 byte 預算內，並對齊 char 邊界以避免切壞多 byte 字元。 */
    private static String truncateToByteBudget(String s, int byteBudget) {
        if (byteBudget <= 0) return "";
        byte[] bytes = s.getBytes(StandardCharsets.UTF_8);
        if (bytes.length <= byteBudget) return s;
        // 二分逼近：找最大 char 長度使 UTF-8 編碼 ≤ budget
        int lo = 0;
        int hi = s.length();
        while (lo < hi) {
            int mid = (lo + hi + 1) >>> 1;
            int b = s.substring(0, mid).getBytes(StandardCharsets.UTF_8).length;
            if (b <= byteBudget) lo = mid;
            else hi = mid - 1;
        }
        return s.substring(0, lo);
    }
}
