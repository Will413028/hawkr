package io.github.will413028.hawkr.common.utils.proto;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;

/**
 * gRPC 邊界型別轉換工具。
 * <p>
 * 統一處理 Proto string ↔ Java 原生型別的轉換，
 * 避免各微服務散落實作相同邏輯。
 * </p>
 */
public final class ProtoConversionUtils {

    private static final DateTimeFormatter ISO_FORMATTER = DateTimeFormatter.ISO_LOCAL_DATE_TIME;

    private ProtoConversionUtils() {
    }

    // ==================== Long 轉換 ====================

    /**
     * Proto string → Long（必填欄位）。
     * 空值或格式不合法時拋出 IllegalArgumentException。
     */
    public static long parseLongRequired(String value, String fieldName) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(fieldName + " 不可為空");
        }
        try {
            return Long.parseLong(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 格式不合法: " + value, e);
        }
    }

    /**
     * Proto string → Long（可選欄位）。
     * 空值返回 null，格式不合法返回 null。
     */
    public static Long parseLongOrNull(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Long.parseLong(value);
        } catch (NumberFormatException e) {
            return null;
        }
    }

    /**
     * Proto string → long（含預設值）。
     * 空值或格式不合法時返回 defaultValue。
     */
    public static long parseLongOrDefault(String value, long defaultValue) {
        if (value == null || value.isBlank()) {
            return defaultValue;
        }
        try {
            return Long.parseLong(value);
        } catch (NumberFormatException e) {
            return defaultValue;
        }
    }

    /**
     * Long → Proto string（非金額的可選數值）。
     * null 返回空字串。
     *
     * <p><b>與 {@link #formatAmount(Long)} 對 null 的處理相反，選錯就是靜默的契約錯誤</b>：
     * <ul>
     *   <li>金額欄位（價格、運費、折扣、退款額）用 {@link #formatAmount(Long)}：null → {@code "0"}。
     *       金額的 0 就是本專案表達「沒有」的方式，且下游把這些欄位當數字解析——空字串會讓
     *       帶 {@code omitempty} 的 Gateway JSON 鍵整個消失。</li>
     *   <li>非金額的可選數值（ID、計數、時間戳）用本方法：null → {@code ""}。這類欄位的 0
     *       是一個真實的值，不能拿來冒充「沒有」。</li>
     * </ul>
     */
    public static String formatLong(Long value) {
        return value == null ? "" : String.valueOf(value);
    }

    // ==================== Integer 轉換 ====================

    /**
     * Proto string → int（必填欄位）。
     */
    public static int parseIntRequired(String value, String fieldName) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(fieldName + " 不可為空");
        }
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 格式不合法: " + value, e);
        }
    }

    /**
     * Proto string → Integer（可選欄位）。
     * 空值返回 null。
     */
    public static Integer parseIntOrNull(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException e) {
            return null;
        }
    }

    /**
     * Proto string → int（含預設值）。
     */
    public static int parseIntOrDefault(String value, int defaultValue) {
        if (value == null || value.isBlank()) {
            return defaultValue;
        }
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException e) {
            return defaultValue;
        }
    }

    /**
     * Integer → Proto string。
     * null 返回空字串。
     */
    public static String formatInt(Integer value) {
        return value == null ? "" : String.valueOf(value);
    }

    // ==================== Decimal-tolerant 數字解析 ====================

    /**
     * Proto string → int，**接受十進位字串**並向 0 truncate。
     * <p>
     * 設計動機：DB 物理量欄位多為 {@code numeric(P,S)}，序列化會帶 trailing zero（"18.00"），
     * {@code parseIntRequired} 因 {@code Integer.parseInt} 嚴格而拋錯。本方法用 {@link BigDecimal}
     * 解析，吸收 "18.00"、"18.5"、"5e2" 等所有 BigDecimal 合法格式；含小數時向 0 truncate；
     * 超 int 範圍拒絕（不靜默 overflow）。詳見 22 號設計書 §2.7。
     * </p>
     *
     * @throws IllegalArgumentException value 為空、非數字、或超 int 範圍
     */
    public static int parseDecimalToInt(String value, String fieldName) {
        BigDecimal d = parseDecimal(value, fieldName);
        BigInteger big = d.toBigInteger(); // truncate towards zero
        if (big.bitLength() >= 32) {
            throw new IllegalArgumentException(fieldName + " 超出 int 範圍: " + value);
        }
        return big.intValue();
    }

    /**
     * Proto string → long，**接受十進位字串**並向 0 truncate。
     * 細節同 {@link #parseDecimalToInt(String, String)}，僅範圍延伸至 long。
     */
    public static long parseDecimalToLong(String value, String fieldName) {
        BigDecimal d = parseDecimal(value, fieldName);
        BigInteger big = d.toBigInteger(); // truncate towards zero
        if (big.bitLength() >= 64) {
            throw new IllegalArgumentException(fieldName + " 超出 long 範圍: " + value);
        }
        return big.longValue();
    }

    private static BigDecimal parseDecimal(String value, String fieldName) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(fieldName + " 不可為空");
        }
        try {
            return new BigDecimal(value.trim());
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(fieldName + " 格式不合法: " + value, e);
        }
    }

    // ==================== 時間轉換 ====================

    /**
     * LocalDateTime(UTC) → ISO 8601 字串（帶 Z 尾綴）。
     * null 返回空字串。
     * <p>
     * 委託 {@link DateTimeUtils#formatIso8601(LocalDateTime)}。
     */
    public static String formatIso8601(LocalDateTime dateTime) {
        return DateTimeUtils.formatIso8601(dateTime);
    }

    /**
     * ISO 8601 字串 → LocalDateTime（必填欄位）。
     * 空值或格式不合法時拋出 IllegalArgumentException。
     * 支援完整 ISO 8601（2026-03-24T10:00:00Z）與純日期（2026-03-24，視為當日 00:00:00）。
     */
    public static LocalDateTime parseIso8601Required(String iso, String fieldName) {
        if (iso == null || iso.isBlank()) {
            throw new IllegalArgumentException(fieldName + " 不可為空");
        }
        LocalDateTime result = parseIso8601Internal(iso);
        if (result == null) {
            throw new IllegalArgumentException(fieldName + " 時間格式不合法: " + iso);
        }
        return result;
    }

    /**
     * ISO 8601 字串 → LocalDateTime。
     * 空值或格式不合法返回 null。
     * 支援完整 ISO 8601（2026-03-24T10:00:00Z）與純日期（2026-03-24，視為當日 00:00:00）。
     */
    public static LocalDateTime parseIso8601OrNull(String iso) {
        if (iso == null || iso.isBlank()) {
            return null;
        }
        return parseIso8601Internal(iso);
    }

    private static LocalDateTime parseIso8601Internal(String iso) {
        String cleaned = iso.replace("Z", "").trim();
        // 純日期格式（YYYY-MM-DD）→ 補上 T00:00:00
        if (cleaned.length() == 10 && cleaned.charAt(4) == '-' && cleaned.charAt(7) == '-') {
            cleaned = cleaned + "T00:00:00";
        }
        try {
            return LocalDateTime.parse(cleaned, ISO_FORMATTER);
        } catch (DateTimeParseException e) {
            return null;
        }
    }

    // ==================== 金額轉換 ====================

    /**
     * Proto string → long（金額，必須 > 0）。
     */
    public static long parseAmountRequired(String amountStr, String fieldName) {
        long amount = parseLongRequired(amountStr, fieldName);
        if (amount <= 0) {
            throw new IllegalArgumentException(fieldName + " 必須大於 0，實際值: " + amount);
        }
        return amount;
    }

    /**
     * Long 金額 → Proto string。
     * null 返回 "0"。
     *
     * <p><b>與 {@link #formatLong(Long)} 對 null 的處理相反</b>：那支回空字串。金額一律用本方法，
     * 因為 0 就是本專案表達「沒有金額」的方式（入站的 {@code parsePrice} / {@code parseLong}
     * 也是把 {@code "0"} 讀回 null，雙向對稱），而下游把這些欄位當數字解析——空字串會讓帶
     * {@code omitempty} 的 Gateway JSON 鍵整個消失。改變本方法的行為是對每一筆訂單、每一件商品的
     * 破壞性契約變更，不是一個 null-handling 的口味問題。
     */
    public static String formatAmount(Long amount) {
        return amount == null ? "0" : String.valueOf(amount);
    }
}
