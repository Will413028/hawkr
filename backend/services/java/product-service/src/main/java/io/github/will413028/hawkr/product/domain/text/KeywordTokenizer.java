package io.github.will413028.hawkr.product.domain.text;

import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;

/**
 * 多 token 關鍵字搜尋 tokenizer（02 設計書 §5.3.3）。
 *
 * <p>規則：
 * <ol>
 *   <li>trim 整體</li>
 *   <li>以連續 Unicode whitespace 拆分</li>
 *   <li>每 token：trim、lower (ASCII) — 中文不變</li>
 *   <li>過濾：純中文 token ≥ 1 字元；含 ASCII letter/digit token ≥ 2 字元；其餘丟棄</li>
 *   <li>去重（lower 後）</li>
 *   <li>token 數上限 5；超過取前 5</li>
 *   <li>全部被過濾 → 回空 list（呼叫端視同無 keyword）</li>
 * </ol>
 */
public final class KeywordTokenizer {

    public static final int MAX_TOKENS = 5;

    private KeywordTokenizer() {}

    public static List<String> tokenize(String keyword) {
        if (keyword == null) {
            return List.of();
        }
        String trimmed = keyword.trim();
        if (trimmed.isEmpty()) {
            return List.of();
        }
        String[] parts = trimmed.split("\\s+");
        Set<String> dedup = new LinkedHashSet<>();
        for (String part : parts) {
            String t = part.trim().toLowerCase(Locale.ROOT);
            if (t.isEmpty()) continue;
            if (!hasMinLength(t)) continue;
            dedup.add(t);
            if (dedup.size() >= MAX_TOKENS) break;
        }
        return new ArrayList<>(dedup);
    }

    /** 純中文 token 接受 ≥1 字元；含 ASCII letter/digit token 要求 ≥2 字元。 */
    private static boolean hasMinLength(String token) {
        boolean hasAsciiAlnum = false;
        int cpCount = token.codePointCount(0, token.length());
        for (int i = 0; i < token.length(); ) {
            int cp = token.codePointAt(i);
            if ((cp >= '0' && cp <= '9') || (cp >= 'a' && cp <= 'z') || (cp >= 'A' && cp <= 'Z')) {
                hasAsciiAlnum = true;
                break;
            }
            i += Character.charCount(cp);
        }
        if (hasAsciiAlnum) {
            return cpCount >= 2;
        }
        return cpCount >= 1;
    }
}
