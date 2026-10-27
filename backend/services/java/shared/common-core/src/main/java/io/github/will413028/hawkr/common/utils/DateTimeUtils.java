package io.github.will413028.hawkr.common.utils;

import java.time.Instant;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.LocalTime;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;

/**
 * UTC 時間工具類。
 * <p>
 * 全系統統一使用 UTC 儲存與傳輸，前端依使用者時區自行轉換。
 * 禁止裸用 {@code LocalDateTime.now()}，必須使用本工具類取得 UTC 時間。
 * </p>
 */
public final class DateTimeUtils {

    private DateTimeUtils() {
    }

    // ==================== 在地時區常數 ====================

    /**
     * 台灣在地時區。
     *
     * <p><b>只用於「與外部在地日期資料做語意比較」</b>：外部系統（PayUni、物流、政府開放資料）
     * 給的是不帶時區的台灣在地日期／時間，要正確解讀它就必須用這個時區。
     *
     * <p>⚠️ 這與「時間一律 UTC」的規範不衝突：那條管的是<b>儲存與傳輸</b>
     * （DB 欄位、事件時間、API 輸出仍一律 UTC）。<b>不得</b>拿它來取代 {@link #nowUtc()} 寫入 DB。
     */
    public static final ZoneId TAIWAN_ZONE = ZoneId.of("Asia/Taipei");

    // ==================== PayUni 時區常數與 formatter ====================

    /** PayUni / 物流 系統的時區（台灣）；＝ {@link #TAIWAN_ZONE}。 */
    public static final ZoneId PAYUNI_ZONE = TAIWAN_ZONE;

    /** 占位符：PayUni query API 對未付款 trade 回傳的 PaymentDay 值。 */
    private static final String PAYUNI_NULL_DATETIME_PLACEHOLDER = "0000-00-00 00:00:00";

    /**
     * PayUni 的另一個「無值」表示法。
     *
     * <p>🔴 它與 {@link #PAYUNI_NULL_DATETIME_PLACEHOLDER} 是<b>兩個不同的哨兵</b>，
     * 而 PayUni 兩種都會回。全 repo 已有四處各自處理過這個 {@code "-"}
     * （{@code PayUniCvsAdapter}、{@code PayUniHomeAdapter}、
     * {@code RefundApplicationService}、{@code RefundResultConsumer}），
     * 唯獨這支共用 parser 沒有——那正是它會炸的原因。
     */
    private static final String PAYUNI_DASH_PLACEHOLDER = "-";

    private static final DateTimeFormatter PAYUNI_DATE_FMT =
            DateTimeFormatter.ofPattern("yyyy-MM-dd");
    private static final DateTimeFormatter PAYUNI_DATETIME_FMT =
            DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");
    private static final DateTimeFormatter PAYUNI_AUTH_DAY_FMT =
            DateTimeFormatter.ofPattern("yyyyMMdd");
    private static final DateTimeFormatter PAYUNI_AUTH_TIME_FMT =
            DateTimeFormatter.ofPattern("HHmmss");

    // ===== 我方 → PayUni（送出日期）=====

    /** 取得目前在台北時區的日期（送 PayUni 用，避免 UTC 容器跨日邊界錯一天）。 */
    public static LocalDate todayInTaipei() {
        return LocalDate.now(PAYUNI_ZONE);
    }

    /** 取得 N 天後的台北日期。 */
    public static LocalDate taipeiPlusDays(int days) {
        return todayInTaipei().plusDays(days);
    }

    /** 格式化為 PayUni {@code yyyy-MM-dd} 字串。 */
    public static String formatPayUniDate(LocalDate date) {
        return date.format(PAYUNI_DATE_FMT);
    }

    /** 格式化為 PayUni 物流 {@code yyyyMMdd} 字串。 */
    public static String formatShippingDate(LocalDate date) {
        return date.format(PAYUNI_AUTH_DAY_FMT);
    }

    // ===== PayUni → 我方（解析回傳）=====

    /**
     * 解析 PayUni {@code yyyy-MM-dd HH:mm:ss} UTC+8 字串為 UTC LocalDateTime。
     *
     * <h2>🔴 本方法對「解析不出來」的承諾是<b>回 null，不拋例外</b></h2>
     * 2026-09-10 dev 實證：PayUni 對「尚未開始配送」的物流單，{@code ShipStatusTime}
     * 回的是哨兵 {@code "-"}。本方法原本只認 {@code "0000-00-00 00:00:00"}
     * ⇒ {@code "-"} 一路走到 {@code LocalDateTime.parse} 拋 {@code DateTimeParseException}
     * ⇒ <b>物流追蹤端點對這類單一律 HTTP 500</b>
     * （{@code GET /app/shipments/{orderNo}/tracking}，貨態補收排程亦連帶失敗）。
     *
     * <p>⚠️ 為什麼呼叫端接不住：四個呼叫端全都寫成
     * {@code if (result == null) { 兜底 }} 的形狀——它們是照這份 javadoc 的承諾寫的。
     * 一個宣稱「回 null」卻會拋例外的方法，會讓每一個正確使用它的呼叫端都變成故障點。
     * 這裡把實作對齊承諾，而不是要四個呼叫端各自包 try-catch。
     *
     * <p>⚠️ <b>吞掉例外會不會遮蔽真問題？</b> 不會靜默：四個呼叫端在拿到 null 時
     * 都有各自的處置（{@code ShippingService.updateStatus} 與
     * {@code AbstractPayUniAdapter.parseProviderTime} 會 {@code log.warn} 並印出原始字串；
     * payment 兩處走兜底解析）。而拋例外的後果是整個請求 500 ——
     * 兩者相比，落 null ＋ 留痕才是正確的失效方向。
     *
     * @return UTC 的 LocalDateTime；若輸入為 null／blank／PayUni 的兩種占位符
     *         （{@code "0000-00-00 00:00:00"}、{@code "-"}）／格式無法解析 → 一律回 null
     */
    public static LocalDateTime parsePayUniDateTimeToUtc(String s) {
        if (s == null || s.isBlank()) {
            return null;
        }
        String trimmed = s.trim();
        if (PAYUNI_NULL_DATETIME_PLACEHOLDER.equals(trimmed)
                || PAYUNI_DASH_PLACEHOLDER.equals(trimmed)) {
            return null;
        }
        try {
            LocalDateTime taipeiLocal = LocalDateTime.parse(trimmed, PAYUNI_DATETIME_FMT);
            return taipeiLocal.atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC)
                    .toLocalDateTime();
        } catch (DateTimeParseException e) {
            // 🔴 已知的哨兵在上面就回 null 了（不會走到這裡）⇒ 進到這裡代表
            //    **PayUni 送了一個我們沒見過的格式**，那是要被看見的事。
            //    這裡不 log（本類別是純工具、不持有 logger），交由呼叫端的
            //    null 分支留痕——四個呼叫端都有。
            //
            // ⚠️ **這道與上面的哨兵判定互為冗餘，是刻意的，但要知道冗餘到什麼程度**
            //    （2026-09-10 變異實測，四種變異）：
            //      M1  只拿掉 "-" 哨兵判定        → **不轉紅**（"-" 落到這裡被接住）
            //      M2  只拿掉本 catch             → 轉紅 1 error（未知格式失去防線）
            //      M1b 兩道同時拿掉               → 轉紅 3 errors
            //      M3  整個方法直接 return null   → 轉紅 3 failures（陰性對照有效）
            //    ⇒ 單看「"-" 有沒有被正確判定」這件事，**測試抓不到**——本 catch 會遮住它。
            //    保留顯式哨兵判定的理由不是防禦，是**語意**：讓本 catch 的含義維持
            //    「PayUni 改格式了」這個罕見事件，而不是把每天都會發生的 "-" 也算進去。
            //    刪掉任何一道之前，先讓另一道的測試轉紅，否則會誤以為它沒用。
            return null;
        }
    }

    /**
     * 解析 PayUni 信用卡 webhook 的 AuthDay ({@code yyyyMMdd}) + AuthTime ({@code HHmmss}) UTC+8
     * 為 UTC LocalDateTime。
     *
     * @return UTC 的 LocalDateTime；若任一輸入為 null/blank → 回 null
     */
    public static LocalDateTime parsePayUniAuthDayTimeToUtc(String authDay, String authTime) {
        if (authDay == null || authDay.isBlank() || authTime == null || authTime.isBlank()) {
            return null;
        }
        LocalDate date = LocalDate.parse(authDay.trim(), PAYUNI_AUTH_DAY_FMT);
        LocalTime time = LocalTime.parse(authTime.trim(), PAYUNI_AUTH_TIME_FMT);
        LocalDateTime taipeiLocal = LocalDateTime.of(date, time);
        return taipeiLocal.atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC).toLocalDateTime();
    }

    /**
     * 將 PayUni {@code yyyy-MM-dd} 日期視為「該日 23:59:59 UTC+8」轉為 UTC LocalDateTime。
     * 給「ATM ExpireDate 補位」用（例如 PayUni 異常時的告警，不應在主路徑使用）。
     */
    public static LocalDateTime payUniDateEndOfDayToUtc(String yyyyMmDd) {
        if (yyyyMmDd == null || yyyyMmDd.isBlank()) return null;
        LocalDate date = LocalDate.parse(yyyyMmDd.trim(), PAYUNI_DATE_FMT);
        return date.atTime(23, 59, 59).atZone(PAYUNI_ZONE).withZoneSameInstant(ZoneOffset.UTC).toLocalDateTime();
    }

    // ==================== 取得當前時間 ====================

    /**
     * 取得 UTC 當前時間。
     * <p>
     * 替代 {@code LocalDateTime.now()}，不受 JVM 時區影響。
     */
    public static LocalDateTime nowUtc() {
        return LocalDateTime.now(ZoneOffset.UTC);
    }

    /**
     * 取得 UTC 今日日期（不含時分秒）。
     * <p>
     * 替代 {@code LocalDate.now()}，不受 JVM 時區影響。日期區間查詢用。
     */
    public static LocalDate todayUtc() {
        return LocalDate.now(ZoneOffset.UTC);
    }

    // ==================== 格式化 ====================

    /**
     * LocalDateTime(UTC) → ISO 8601 字串，帶 Z 尾綴。
     * <p>
     * 例：{@code "2026-03-31T08:30:00Z"}
     *
     * @return ISO 8601 字串，null 時回傳空字串
     */
    public static String formatIso8601(LocalDateTime utcTime) {
        if (utcTime == null) {
            return "";
        }
        return utcTime.atOffset(ZoneOffset.UTC)
                .format(DateTimeFormatter.ISO_OFFSET_DATE_TIME);
    }

    // ==================== 解析 ====================

    /**
     * ISO 8601 字串 → LocalDateTime(UTC)。
     * <p>
     * 支援格式：
     * <ul>
     *   <li>{@code "2026-03-31T08:30:00Z"}</li>
     *   <li>{@code "2026-03-31T08:30:00"} （無 Z，視為 UTC）</li>
     *   <li>{@code "2026-03-31"} （純日期，補 T00:00:00）</li>
     * </ul>
     *
     * @return LocalDateTime(UTC)，null 或不合法時回傳 null
     */
    public static LocalDateTime parseIso8601(String iso) {
        if (iso == null || iso.isBlank()) {
            return null;
        }
        String cleaned = iso.replace("Z", "").replace("z", "").trim();
        if (cleaned.length() == 10 && cleaned.charAt(4) == '-' && cleaned.charAt(7) == '-') {
            cleaned = cleaned + "T00:00:00";
        }
        try {
            return LocalDateTime.parse(cleaned, DateTimeFormatter.ISO_LOCAL_DATE_TIME);
        } catch (DateTimeParseException e) {
            return null;
        }
    }

    // ==================== Epoch 轉換 ====================

    /**
     * LocalDateTime(UTC) → epoch 毫秒。
     */
    public static long toEpochMillis(LocalDateTime utcTime) {
        return utcTime.toInstant(ZoneOffset.UTC).toEpochMilli();
    }

    /**
     * Epoch 毫秒 → LocalDateTime(UTC)。
     */
    public static LocalDateTime fromEpochMillis(long epochMillis) {
        return LocalDateTime.ofInstant(Instant.ofEpochMilli(epochMillis), ZoneOffset.UTC);
    }

    /**
     * LocalDateTime(UTC) → epoch 秒。
     */
    public static long toEpochSeconds(LocalDateTime utcTime) {
        return utcTime.toEpochSecond(ZoneOffset.UTC);
    }

    /**
     * Epoch 秒 → LocalDateTime(UTC)。
     */
    public static LocalDateTime fromEpochSeconds(long epochSeconds) {
        return LocalDateTime.ofEpochSecond(epochSeconds, 0, ZoneOffset.UTC);
    }
}
