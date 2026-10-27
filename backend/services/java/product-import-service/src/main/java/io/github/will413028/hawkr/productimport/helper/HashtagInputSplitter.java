package io.github.will413028.hawkr.productimport.helper;

import java.text.Normalizer;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;

/**
 * Hashtag 輸入切分（40 §11.3 + 33 §3.1.1 caller split 責任契約）。
 *
 * <p>對外契約：將賣家原始輸入（CSV cell 或 UI raw string）切成單 token list，
 * 再交給 product-service `HashtagInputValidator` 嚴格規則接收。
 *
 * <p>切分規則：
 * <ol>
 *     <li>用 regex {@code [#,]+} split — 同時接受 `#` 連寫、半形逗號、混用</li>
 *     <li>每 token: trim → strip 開頭連續 `#` → Unicode NFC</li>
 *     <li>過濾空 token（不留 placeholder）</li>
 * </ol>
 *
 * <p>本 helper **不**做字元 / 長度 / 數量驗證 — 切分後由 caller（L2ContentValidator
 * 或 commit 階段 `HashtagInputValidator`）驗。
 */
public final class HashtagInputSplitter {

    private static final Pattern DELIMITER = Pattern.compile("[#,]+");
    private static final Pattern LEADING_HASH = Pattern.compile("^#+");

    private HashtagInputSplitter() {}

    /**
     * 切分 CSV cell 等單一字串輸入。
     *
     * @param raw 原始輸入；null / blank 視為空輸入
     * @return 切分 + normalize 後的單 token list；過濾空 token；保留輸入序
     */
    public static List<String> split(String raw) {
        if (raw == null || raw.isBlank()) return new ArrayList<>();
        List<String> out = new ArrayList<>();
        for (String piece : DELIMITER.split(raw)) {
            String tok = normalizeToken(piece);
            if (!tok.isEmpty()) out.add(tok);
        }
        return out;
    }

    /**
     * 對 list of raw（如 UI / Proto 已分欄但 element 仍可能含 `#` 連寫）展平 + split。
     */
    public static List<String> splitAll(List<String> rawList) {
        if (rawList == null || rawList.isEmpty()) return new ArrayList<>();
        List<String> out = new ArrayList<>();
        for (String raw : rawList) {
            out.addAll(split(raw));
        }
        return out;
    }

    private static String normalizeToken(String s) {
        if (s == null) return "";
        String t = s.trim();
        if (t.isEmpty()) return "";
        t = LEADING_HASH.matcher(t).replaceFirst("");
        if (t.isEmpty()) return "";
        return Normalizer.normalize(t, Normalizer.Form.NFC);
    }

    /** 與 product-service {@code HashtagInputValidator.MAX_HASHTAG_LENGTH} 對齊（33 §3.2）。 */
    public static final int MAX_TOKEN_LENGTH = 40;

    /** 與 product-service {@code HashtagInputValidator.MAX_HASHTAGS_PER_PRODUCT} 對齊（33 §3.2）。 */
    public static final int MAX_TOKENS_PER_PRODUCT = 10;

    /** 字元 / 長度檢查結果。對應 ImportItemErrorType hashtag 系列。 */
    public enum TokenIssue { NONE, EMPTY, INVALID_CHAR, LENGTH_EXCEEDED }

    /**
     * 對切分後的單一 token 驗證字元 + 長度（與 product-service {@code HashtagInputValidator} 同規則）。
     *
     * @param token 已 split + normalize 過的 token
     * @return 違規類型，{@link TokenIssue#NONE} 表示合法
     */
    public static TokenIssue checkToken(String token) {
        if (token == null || token.isEmpty()) return TokenIssue.EMPTY;
        int n = token.length();
        int i = 0;
        int cpCount = 0;
        while (i < n) {
            int cp = token.codePointAt(i);
            if (!isAllowedCodePoint(cp)) return TokenIssue.INVALID_CHAR;
            i += Character.charCount(cp);
            cpCount++;
        }
        if (cpCount < 1 || cpCount > MAX_TOKEN_LENGTH) return TokenIssue.LENGTH_EXCEEDED;
        return TokenIssue.NONE;
    }

    /**
     * 與 product-service {@code HashtagInputValidator.isAllowedCodePoint} 完全一致：
     * ASCII letter / digit + CJK Unified Ideographs（基本 + 擴展 A-G + 相容區）。
     */
    private static boolean isAllowedCodePoint(int cp) {
        if ((cp >= '0' && cp <= '9') || (cp >= 'a' && cp <= 'z') || (cp >= 'A' && cp <= 'Z')) {
            return true;
        }
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
