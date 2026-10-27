package io.github.will413028.hawkr.media.util;

import java.util.regex.Pattern;

/**
 * 資產檔名的消毒與顯示解析（issue #3149，設計書 08 §4.9）。
 *
 * <p><b>為什麼是 static utility 而不是 Spring bean</b>：做成 bean 會迫使四支既有測試
 * （{@code BatchGetAssetMetadataTest}／{@code ServerReadSecureAssetTest}／
 * {@code ServerUploadSecureEncryptionTest}／{@code SecureMediaControllerTest}）的顯式
 * 建構子全部改簽章，而本類別沒有任何狀態或依賴（設計書 §4.9.6）。
 *
 * <p>🔴 <b>本類別只處理「顯示用的檔名」，不碰 object_key</b>。同一個 filename 在系統裡
 * 有三個去處，只有寫進 {@code original_filename} 那一個要消毒——另兩個（
 * {@code generateObjectKey} 取副檔名、兩支後端直傳組 key 末段）必須拿<b>原值</b>。
 * 詳見設計書 §4.9.3〈作用範圍〉；順序上 {@link #sanitize} 一律排在 key 生成之後，
 * 反過來會讓 {@code AbstractStorageService.sanitizedExtension} 對路徑穿越的拒絕失效。
 */
public final class AssetFilename {

    /**
     * 對齊 {@code original_filename VARCHAR(255)}。
     *
     * <p>PostgreSQL 的 {@code VARCHAR(n)} 計<b>字元</b>不計位元組，所以這裡也用字元數。
     * ⚠️ 不要改用 {@code InputValidator.truncateByDisplayLength}——那支算的是顯示寬度
     * （CJK 記 2），會把 200 字的中文檔名砍成 127 字。
     */
    private static final int MAX_LENGTH = 255;

    /**
     * per-attempt object key 的尾綴（issue #2759）。
     *
     * <p>{@code SecureMediaController} 每次機敏上傳都用 {@code session.objectKey + ".attempt-" + UUID}，
     * CAS 贏家的 key 會被寫回 session 並由 {@code generateFinalObjectKey} 原樣沿用 ⇒
     * 舊資料回退取尾段時會拿到 {@code {uuid}{ext}.attempt-{uuid}}，連副檔名都不對。
     * 剝掉它<b>不是</b>本 issue 的解法（剝完仍是一串 uuid），只是讓回退值不那麼糟。
     */
    private static final Pattern ATTEMPT_SUFFIX =
            Pattern.compile("\\.attempt-[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$");

    /**
     * 必須從顯示檔名移除的字元：所有 C0／C1 控制字元（{@code \p{Cc}} 涵蓋 0x00-0x1F、
     * 0x7F-0x9F）、雙引號與反斜線。
     *
     * <p>⚠️ 三者之中**真正靠這個 pattern 擋住的是雙引號**。反斜線在實務上先被
     * {@link #basenameOf} 當成路徑分隔符取走（實測 30 萬組隨機輸入，加不加 {@code \\}
     * 到這個集合的結果差異為 0）⇒ 它在這裡是**縱深防禦**而不是唯一防線。
     * 寫下這一點是為了避免後人以為 {@code basenameOf} 可以拿掉。
     *
     * <p>🔴 <b>引號與反斜線必須在這裡移除，不能指望 {@code ContentDisposition} builder</b>
     * ——實測 spring-web 6.1.2：只有<b>不帶 charset</b> 的 {@code filename(String)} 多載會走
     * {@code encodeQuotedPairs} 逃逸它們；我們用的 {@code filename(String, Charset)} 走
     * {@code encodeQuotedPrintableFilename}，而它的 PRINTABLE 位元集只排除 {@code =}／{@code ?}／
     * {@code _}，{@code "}(34) 與 {@code \}(92) <b>原樣輸出</b>：
     * <pre>
     * a"b.pdf → attachment; filename="=?UTF-8?Q?a"b.pdf?="; filename*=UTF-8''a%22b.pdf
     *                                            ^ 引號在這裡把 quoted-string 提前收尾
     * </pre>
     * ⚠️ 這條註解取代了一個曾經寫在設計書裡、但<b>從未被查證</b>的前提
     * （「builder 會逃逸引號」）。佐證：invoice-service 的 Go 端
     * {@code infrastructure/file_server.go} 的 {@code sanitizeFileName} 也是自己剝掉
     * {@code "} 與 {@code \} 的。
     */
    private static final Pattern UNSAFE_CHARS = Pattern.compile("[\\p{Cc}\"\\\\]");

    private AssetFilename() {
    }

    /**
     * 把使用者可控的原始檔名消毒成可以落庫、可以進 header 的顯示值。
     *
     * <p><b>清洗而不是拒絕</b>：與 {@code sanitizedExtension} 的分工是——後者守的是 object_key
     * （路徑穿越 ⇒ 直接丟 {@code INVALID_PARAM}），本函式守的是顯示值，而顯示值壞掉不該讓
     * 整個上傳失敗。
     *
     * <p>控制字元採<b>白名單語意</b>（移除整個 {@code Cc} 類別）而不是點名清除 CR／LF：
     * 點名會漏掉 {@code NUL}，而 {@code %00} 經 URL path 解碼後是真的 {@code 0x00}，
     * PostgreSQL 的 {@code VARCHAR} 直接拒收它 ⇒ 那會是一條新引入的 INSERT 失敗路徑。
     * 引號與反斜線一併移除，理由見 {@link #UNSAFE_CHARS}。
     *
     * @return 消毒後的檔名；輸入為 null、或清洗後為空時回 {@code null}（讓讀取端走回退）
     */
    public static String sanitize(String raw) {
        if (raw == null) {
            return null;
        }
        String name = basenameOf(raw);
        name = UNSAFE_CHARS.matcher(name).replaceAll("").trim();
        if (name.isEmpty()) {
            return null;
        }
        return truncateKeepingExtension(name);
    }

    /**
     * 決定「這份資產顯示成什麼檔名」。<b>所有</b>檔名消費點都必須走這裡。
     *
     * <p>回退到 object_key 尾段不是技術債，是資料事實：V9 之前的列沒有原始檔名，
     * 而 R2 的 object metadata 也沒存（無從回填，設計書 §4.9.4）。
     *
     * @param originalFilename {@code media_asset}／{@code upload_session} 的該欄，可為 null
     * @param objectKey        回退來源
     * @return 檔名；兩者皆不可用時回空字串（<b>不回 null</b>——proto 的 string 欄位與
     *         {@code Content-Disposition} 都不接受 null，統一成空字串只留一種表示法）
     */
    public static String resolveDisplayName(String originalFilename, String objectKey) {
        String candidate;
        if (originalFilename != null && !originalFilename.isBlank()) {
            candidate = originalFilename;
        } else if (objectKey == null) {
            candidate = null;
        } else {
            candidate = ATTEMPT_SUFFIX.matcher(basenameOf(objectKey)).replaceFirst("");
        }
        // 🔴 **兩個分支都要過 sanitize，回退那條尤其不能省**：object_key 末段不是乾淨的。
        // 一般上傳的末段是 {uuid}{ext}，而 ext 只過 sanitizedExtension——那支只拒絕
        // `/`、`\`、`..`，**引號會原樣通過** ⇒ 使用者送 `a.pd"f` 就足以讓 key 尾段帶引號；
        // 兩支後端直傳的末段更是照 §4.9.3 用未消毒的原值。V9 之前的列沒有
        // original_filename 可用 ⇒ 一定走回退 ⇒ 那個引號會直接進 Content-Disposition。
        // sanitize 是冪等的，對已消毒的 originalFilename 再跑一次沒有代價，
        // 換來的是「本函式的回傳值恆可安全進 header」這個不依賴呼叫端紀律的保證。
        String safe = sanitize(candidate);
        return safe == null ? "" : safe;
    }

    /**
     * 從商品匯入的外部圖片 URL 推出顯示檔名（設計書 08 §4.9.3a）。
     *
     * <p>流程：{@code URI.create} → {@code getPath()} → basename → {@link #sanitize}。
     * 任何一步失敗或結果為空即回 {@code null}，讓讀取端走回退——匯入圖片沒有檔名不是錯誤。
     *
     * <p>三個實測過的坑（每一個的失敗都是<b>靜默</b>的，不會拋例外）：
     * <ol>
     *   <li><b>不得再 URL-decode 一次</b>。{@code getPath()} 回的已經是解碼後的路徑
     *       （{@code getRawPath()} 才是原始形式）。多解一次會讓 {@code 100%25_off.jpg}
     *       拋例外、{@code my+file.jpg} 的 {@code +} 變成空格、而
     *       {@code a%252F..%252F..} 被還原成真的路徑分隔符。</li>
     *   <li><b>{@code URI.create(null)} 拋的是 {@link NullPointerException} 而不是
     *       {@link IllegalArgumentException}</b>，而 {@code source_url} 可以是 NULL
     *       （STAGED 來源的 CHECK 約束強制它為 NULL）⇒ null 必須在解析前擋掉。</li>
     *   <li><b>不得直接對整串 URL 取 basename</b>。賣家的圖片 URL 常帶簽名參數，
     *       {@code img.jpg?sig=<token>} 的 basename 會把那個 token 一路寫進 DB 與
     *       {@code Content-Disposition}。{@code getPath()} 天生不含 query 與 fragment。</li>
     * </ol>
     */
    public static String fromSourceUrl(String sourceUrl) {
        if (sourceUrl == null || sourceUrl.isBlank()) {
            return null;
        }
        String path;
        try {
            // opaque URI（如 data:image/png;base64,...）的 getPath() 回 null
            path = java.net.URI.create(sourceUrl).getPath();
        } catch (IllegalArgumentException e) {
            // 未編碼空格之類的非法字元；匯入圖片拿不到檔名不是錯誤，回退即可
            return null;
        }
        if (path == null || path.isEmpty()) {
            return null;
        }
        // trailing slash（.../dir/）的 basename 為空 → sanitize 會回 null
        return sanitize(path);
    }

    /** 取最後一個 {@code /} 或 {@code \} 之後的部分；瀏覽器偶爾會送出完整路徑。 */
    private static String basenameOf(String path) {
        int cut = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
        return cut < 0 ? path : path.substring(cut + 1);
    }

    /**
     * 超長時截斷，<b>保留副檔名</b>——那是使用者辨識檔案類型的依據，砍掉它等於把
     * {@code 報告.pdf} 變成一個看不出是什麼的東西。副檔名本身就超長時退化為單純截斷。
     */
    private static String truncateKeepingExtension(String name) {
        if (name.length() <= MAX_LENGTH) {
            return name;
        }
        int dot = name.lastIndexOf('.');
        String ext = dot < 0 ? "" : name.substring(dot);
        if (ext.length() >= MAX_LENGTH) {
            return name.substring(0, MAX_LENGTH);
        }
        return (name.substring(0, MAX_LENGTH - ext.length()).stripTrailing() + ext).trim();
    }
}
