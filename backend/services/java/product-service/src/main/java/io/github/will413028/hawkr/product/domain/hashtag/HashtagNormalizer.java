package io.github.will413028.hawkr.product.domain.hashtag;

import java.text.Normalizer;
import java.util.Locale;

/**
 * Hashtag 規範化工具（02 設計書 §3.9）。
 *
 * <p>核心 pipeline：
 * <ol>
 *     <li>trim 兩端空白</li>
 *     <li>strip 開頭連續 {@code #}</li>
 *     <li>Unicode NFC 正規化</li>
 *     <li>canonical 路徑：ASCII letter 轉 lower（中文/全形不變）</li>
 *     <li>display 路徑：保留原大小寫</li>
 * </ol>
 */
public final class HashtagNormalizer {

    private HashtagNormalizer() {}

    /**
     * 對單一 raw 輸入跑雙 pipeline；不檢查字元集與長度（由 {@link HashtagInputValidator} 做）。
     *
     * @param raw 賣家原始輸入
     * @return 規範化結果；當 raw 為 null/空字串或經 strip 後為空時回傳 {@code null}
     */
    public static NormalizedHashtag normalize(String raw) {
        if (raw == null) {
            return null;
        }
        String trimmed = raw.trim();
        if (trimmed.isEmpty()) {
            return null;
        }
        String stripped = stripLeadingHash(trimmed);
        if (stripped.isEmpty()) {
            return null;
        }
        String nfc = Normalizer.normalize(stripped, Normalizer.Form.NFC);
        String canonical = nfc.toLowerCase(Locale.ROOT);
        return new NormalizedHashtag(canonical, nfc);
    }

    private static String stripLeadingHash(String s) {
        int i = 0;
        while (i < s.length() && s.charAt(i) == '#') {
            i++;
        }
        return s.substring(i);
    }
}
