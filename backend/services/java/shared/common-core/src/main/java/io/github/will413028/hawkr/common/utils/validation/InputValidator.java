package io.github.will413028.hawkr.common.utils.validation;

import com.ibm.icu.text.Transliterator;

import java.util.regex.Pattern;

/**
 * 通用輸入驗證工具類
 *
 * 集中管理各種格式驗證與錯誤訊息，供所有微服務在 gRPC 邊界或 Service 層使用。
 * 驗證失敗統一拋出 IllegalArgumentException，由上層 catch 轉為 gRPC INVALID_ARGUMENT。
 *
 * 使用範例：
 * <pre>
 * // 尺寸（支援小數，四捨五入為整數 cm，必須 > 0）
 * int lengthCm = InputValidator.requirePositiveCm("15.50", "包裝長度");
 *
 * // 正整數
 * int qty = InputValidator.requirePositiveInt("3", "quantity");
 *
 * // 正整數金額
 * long amount = InputValidator.requirePositiveAmount("500", "price");
 * </pre>
 */
public final class InputValidator {

    private InputValidator() {}

    // ==================== 尺寸驗證（支援小數，四捨五入為整數 cm） ====================

    /**
     * 解析尺寸字串為正整數 cm（支援小數，四捨五入）
     *
     * @param value     尺寸字串（如 "15.50"、"30"）
     * @param fieldName 欄位名稱（用於錯誤訊息）
     * @return 四捨五入後的整數 cm（> 0）
     * @throws IllegalArgumentException 空值、非數字、或 <= 0
     */
    public static int requirePositiveCm(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        double parsed;
        try {
            parsed = Double.parseDouble(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 格式不合法: " + value, e);
        }
        if (parsed <= 0) {
            throw new IllegalArgumentException(fieldName + " 必須大於 0");
        }
        int result = (int) Math.round(parsed);
        if (result <= 0) {
            throw new IllegalArgumentException(fieldName + " 四捨五入後必須至少為 1 cm");
        }
        return result;
    }

    // ==================== 正整數驗證 ====================

    /**
     * 解析字串為正整數（必須 > 0）
     *
     * @param value     整數字串（如 "3"、"10"）
     * @param fieldName 欄位名稱
     * @return 正整數（> 0）
     */
    public static int requirePositiveInt(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        int parsed;
        try {
            parsed = Integer.parseInt(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 必須為整數: " + value, e);
        }
        if (parsed <= 0) {
            throw new IllegalArgumentException(fieldName + " 必須大於 0");
        }
        return parsed;
    }

    /**
     * 解析字串為非負整數（>= 0）
     *
     * @param value     整數字串
     * @param fieldName 欄位名稱
     * @return 非負整數（>= 0）
     */
    public static int requireNonNegativeInt(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        int parsed;
        try {
            parsed = Integer.parseInt(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 必須為整數: " + value, e);
        }
        if (parsed < 0) {
            throw new IllegalArgumentException(fieldName + " 不可為負數");
        }
        return parsed;
    }

    // ==================== 金額驗證（Long，單位新台幣元） ====================

    /**
     * 解析金額字串為正整數 Long（必須 > 0）
     *
     * @param value     金額字串（如 "500"、"1200"）
     * @param fieldName 欄位名稱
     * @return 正整數金額（> 0）
     */
    public static long requirePositiveAmount(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        long parsed;
        try {
            parsed = Long.parseLong(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 必須為整數金額: " + value, e);
        }
        if (parsed <= 0) {
            throw new IllegalArgumentException(fieldName + " 必須大於 0");
        }
        return parsed;
    }

    /**
     * 解析金額字串為非負整數 Long（>= 0）
     *
     * @param value     金額字串
     * @param fieldName 欄位名稱
     * @return 非負金額（>= 0）
     */
    public static long requireNonNegativeAmount(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        long parsed;
        try {
            parsed = Long.parseLong(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 必須為整數金額: " + value, e);
        }
        if (parsed < 0) {
            throw new IllegalArgumentException(fieldName + " 不可為負數");
        }
        return parsed;
    }

    // ==================== 手機號碼驗證 ====================

    private static final java.util.regex.Pattern MOBILE_PATTERN =
            java.util.regex.Pattern.compile("^09\\d{8}$");

    /**
     * 驗證台灣手機號碼（09 開頭，10 碼純數字）
     *
     * @param value     手機號碼字串（如 "0912345678"）
     * @param fieldName 欄位名稱
     * @return 通過驗證的手機號碼（原值回傳）
     */
    public static String requireMobile(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        if (!MOBILE_PATTERN.matcher(value).matches()) {
            throw new IllegalArgumentException(fieldName + " 必須為 09 開頭的 10 碼數字: " + value);
        }
        return value;
    }

    // ==================== 地址驗證 ====================

    /**
     * 僅允許繁體中文 + 阿拉伯數字 + 地址常用符號（-/之）
     */
    private static final Pattern ADDRESS_ALLOWED_CHARS =
            Pattern.compile("^[\\u4e00-\\u9fff0-9\\-之]+$");

    /**
     * ICU4J 簡體→繁體轉換器（用於偵測簡體中文）
     * 若輸入經轉換後與原文不同，代表含有簡體字
     */
    private static final Transliterator SIMPLIFIED_TO_TRADITIONAL =
            Transliterator.getInstance("Simplified-Traditional");

    /**
     * 驗證地址格式（PayUni 物流規範）
     *
     * 規則：
     * - 僅允許繁體中文、阿拉伯數字、地址符號（-、之）
     * - 禁止英文字母、簡體中文、特殊符號
     * - 必須包含門牌號碼（號）
     *
     * @param value     地址字串
     * @param fieldName 欄位名稱
     * @return 通過驗證的地址（原值回傳）
     */
    public static String requireAddress(String value, String fieldName) {
        requireNotEmpty(value, fieldName);
        // 僅允許繁體中文 + 阿拉伯數字 + 地址符號
        if (!ADDRESS_ALLOWED_CHARS.matcher(value).matches()) {
            throw new IllegalArgumentException(fieldName + " 僅能使用繁體中文及數字填寫，不可包含英文或特殊符號");
        }
        // ICU4J 簡繁轉換偵測：轉成繁體後若不同，代表含簡體字
        String traditional = SIMPLIFIED_TO_TRADITIONAL.transliterate(value);
        if (!traditional.equals(value)) {
            // 找出第一個簡體字，提示使用者
            for (int i = 0; i < value.length(); i++) {
                if (value.charAt(i) != traditional.charAt(i)) {
                    throw new IllegalArgumentException(fieldName + " 包含簡體中文「"
                            + value.charAt(i) + "」，請使用繁體中文「"
                            + traditional.charAt(i) + "」");
                }
            }
        }
        // 必須包含「號」（最基本的地址結構）
        if (!value.contains("號")) {
            throw new IllegalArgumentException(fieldName + " 格式不完整，必須包含門牌號碼（號）");
        }
        return value;
    }

    // ==================== 顯示長度（PayUni 計長：英文/半形 1、中文/全形 2） ====================

    /**
     * 計算字串的顯示長度，對齊 PayUni 物流欄位計長規則。
     *
     * <p>規則：ASCII（code point {@code <= 0x7F}，含英文字母、數字、半形符號）算 1 位，
     * 其餘（中文、全形字符）算 2 位。以 code point 逐字累加，正確處理 surrogate pair。
     *
     * @param value 待計算字串（null 視為 0）
     * @return 顯示長度
     */
    public static int displayLength(String value) {
        if (value == null || value.isEmpty()) {
            return 0;
        }
        int len = 0;
        int i = 0;
        while (i < value.length()) {
            int cp = value.codePointAt(i);
            len += (cp <= 0x7F) ? 1 : 2;
            i += Character.charCount(cp);
        }
        return len;
    }

    /**
     * 驗證字串顯示長度（半形 1、全形 2）落在 {@code [min, max]} 區間，否則拒絕。
     *
     * <p>用於 PayUni 物流「前端可控」欄位（收件人姓名 / 地址）的源頭驗證，
     * 從結帳路徑攔截不符合 provider 約束的輸入，避免建單階段才失敗。
     *
     * @param value     待驗證字串
     * @param min       顯示長度下限（含）
     * @param max       顯示長度上限（含）
     * @param fieldName 欄位名稱
     * @return 通過驗證的原字串
     * @throws IllegalArgumentException 空值、顯示長度 &lt; min、顯示長度 &gt; max
     */
    public static String requireDisplayLengthBetween(String value, int min, int max, String fieldName) {
        requireNotEmpty(value, fieldName);
        int len = displayLength(value);
        if (len < min) {
            throw new IllegalArgumentException(
                    fieldName + " 長度過短，至少需 " + min + " 個字（中文 1 字算 2、英文 1 字算 1）");
        }
        if (len > max) {
            throw new IllegalArgumentException(fieldName + " 長度過長，最多 " + max + " 個字");
        }
        return value;
    }

    /**
     * 依顯示長度（半形 1、全形 2）截斷字串至上限 {@code max}，不切半個全形字。
     *
     * <p>用於 PayUni 物流「系統背景填寫」欄位（商品名稱）：來源非前端可控、無法回頭要使用者修正，
     * 故送 PayUni 前截斷適配而非拒絕。截斷屬資料適配，不拋例外。
     *
     * @param value 待截斷字串（null / 空字串原樣回傳）
     * @param max   顯示長度上限
     * @return 截斷後字串（未超長則原值回傳）
     */
    public static String truncateByDisplayLength(String value, int max) {
        if (value == null || value.isEmpty() || max <= 0) {
            return value;
        }
        int len = 0;
        int i = 0;
        while (i < value.length()) {
            int cp = value.codePointAt(i);
            int w = (cp <= 0x7F) ? 1 : 2;
            if (len + w > max) {
                return value.substring(0, i);
            }
            len += w;
            i += Character.charCount(cp);
        }
        return value;
    }

    // ==================== 共用工具 ====================

    /**
     * 驗證字串非空（null 或空字串皆拒絕）
     *
     * @param value     待驗證字串
     * @param fieldName 欄位名稱
     */
    public static void requireNotEmpty(String value, String fieldName) {
        if (value == null || value.isEmpty()) {
            throw new IllegalArgumentException(fieldName + " 為必填欄位");
        }
    }

    private static final java.util.regex.Pattern HTML_TAG_PATTERN =
            java.util.regex.Pattern.compile("<\\w[^>]*>|</\\w+\\s*>");

    /**
     * 拒絕含 HTML tag 的字串（純文字檢驗）。
     *
     * <p>用途：商品描述 text block content 寫入前驗證，保證 pure_description 萃取結果為純文字。
     * 偵測規則：任何子序列匹配 {@code <\w[^>]*>}（startTag）或 {@code </\w+\s*>}（endTag）即拒絕。
     *
     * @param value     待驗證字串（null 或空字串視為通過）
     * @param fieldName 欄位名稱
     * @return 驗證通過的原字串
     */
    public static String requireNoHtmlTags(String value, String fieldName) {
        if (value == null || value.isEmpty()) {
            return value;
        }
        if (HTML_TAG_PATTERN.matcher(value).find()) {
            throw new IllegalArgumentException(fieldName + " 不可包含 HTML 標籤");
        }
        return value;
    }
}
