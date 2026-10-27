package io.github.will413028.hawkr.product.domain.hashtag;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Hashtag 集合驗證工具（02 設計書 §3.9 / 26-輸入驗證 §2.3 R9）。
 *
 * <p>校驗規則：
 * <ul>
 *     <li>normalize 後為空 → {@code HASHTAG_EMPTY_AFTER_NORMALIZE}（3018）</li>
 *     <li>含空白/#/emoji/標點/控制字元 → {@code HASHTAG_INVALID_CHAR}（3015）</li>
 *     <li>length（Unicode code point）超出 [1, 40] → {@code HASHTAG_LENGTH_EXCEEDED}（3016）</li>
 *     <li>canonical 去重後 list size > 10 → {@code HASHTAG_COUNT_EXCEEDED}（3017）</li>
 * </ul>
 *
 * <p>有效字元：中文 / ASCII letter / digit。
 */
public final class HashtagInputValidator {

    /** 每商品 hashtag 上限（02 §3.9）。 */
    public static final int MAX_HASHTAGS_PER_PRODUCT = 10;

    /** 單一 hashtag 長度上限（Unicode code points）。 */
    public static final int MAX_HASHTAG_LENGTH = 40;

    private HashtagInputValidator() {}

    /**
     * 對 raw hashtag 列表跑完整校驗 + normalize + 去重。
     *
     * @param rawList 賣家輸入；null 視為空 list（合法）
     * @return 規範化後依輸入序保留首次出現的去重結果
     * @throws ProductBusinessException 任一規則違反
     */
    public static List<NormalizedHashtag> requireHashtagSet(List<String> rawList) {
        if (rawList == null || rawList.isEmpty()) {
            return List.of();
        }
        Map<String, NormalizedHashtag> dedup = new LinkedHashMap<>();
        for (String raw : rawList) {
            NormalizedHashtag norm = HashtagNormalizer.normalize(raw);
            if (norm == null) {
                throw new ProductBusinessException(
                        GrpcResponseCode.HASHTAG_EMPTY_AFTER_NORMALIZE,
                        GrpcResponseCode.HASHTAG_EMPTY_AFTER_NORMALIZE.message());
            }
            String canonical = norm.canonical();
            requireValidChars(canonical);
            requireValidLength(canonical);
            // 同 canonical 取輸入序首個 display
            dedup.putIfAbsent(canonical, norm);
        }
        if (dedup.size() > MAX_HASHTAGS_PER_PRODUCT) {
            throw new ProductBusinessException(
                    GrpcResponseCode.HASHTAG_COUNT_EXCEEDED,
                    GrpcResponseCode.HASHTAG_COUNT_EXCEEDED.message());
        }
        return new ArrayList<>(dedup.values());
    }

    private static void requireValidChars(String canonical) {
        int n = canonical.length();
        int i = 0;
        while (i < n) {
            int cp = canonical.codePointAt(i);
            if (!isAllowedCodePoint(cp)) {
                throw new ProductBusinessException(
                        GrpcResponseCode.HASHTAG_INVALID_CHAR,
                        GrpcResponseCode.HASHTAG_INVALID_CHAR.message());
            }
            i += Character.charCount(cp);
        }
    }

    private static void requireValidLength(String canonical) {
        int codePointCount = canonical.codePointCount(0, canonical.length());
        if (codePointCount < 1 || codePointCount > MAX_HASHTAG_LENGTH) {
            throw new ProductBusinessException(
                    GrpcResponseCode.HASHTAG_LENGTH_EXCEEDED,
                    GrpcResponseCode.HASHTAG_LENGTH_EXCEEDED.message());
        }
    }

    /**
     * 允許的字元：ASCII 字母（lower 後）、ASCII 數字、中日韓表意字符（CJK Unified Ideographs 與相關區塊）。
     * 拒絕：空白、#、ASCII 標點/符號、emoji、其他控制字元。
     */
    private static boolean isAllowedCodePoint(int cp) {
        // ASCII letter / digit
        if ((cp >= '0' && cp <= '9') || (cp >= 'a' && cp <= 'z') || (cp >= 'A' && cp <= 'Z')) {
            return true;
        }
        // CJK Unified Ideographs（基本 + 擴展 A/B/C/D/E/F/G/H）
        Character.UnicodeBlock block = Character.UnicodeBlock.of(cp);
        if (block == null) return false;
        return block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_A
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_B
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_C
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_D
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_E
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_F
                || block == Character.UnicodeBlock.CJK_COMPATIBILITY_IDEOGRAPHS
                || block == Character.UnicodeBlock.CJK_COMPATIBILITY_IDEOGRAPHS_SUPPLEMENT;
    }
}
