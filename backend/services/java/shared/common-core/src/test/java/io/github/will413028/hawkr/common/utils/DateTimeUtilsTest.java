package io.github.will413028.hawkr.common.utils;

import org.junit.jupiter.api.Test;

import java.time.LocalDate;
import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.*;

/**
 * DateTimeUtils 單元測試。
 *
 * 覆蓋：PayUni 時區處理、UTC+8 ↔ UTC 轉換、占位符、null/blank、跨日邊界。
 */
class DateTimeUtilsTest {

    // ========== nowUtc / 既有 ==========

    @Test
    void nowUtc_shouldReturnNonNull() {
        assertNotNull(DateTimeUtils.nowUtc());
    }

    @Test
    void formatIso8601_shouldAppendZ() {
        LocalDateTime t = LocalDateTime.of(2026, 3, 31, 8, 30, 0);
        assertEquals("2026-03-31T08:30:00Z", DateTimeUtils.formatIso8601(t));
    }

    @Test
    void formatIso8601_null_shouldReturnEmpty() {
        assertEquals("", DateTimeUtils.formatIso8601(null));
    }

    // ========== todayInTaipei / taipeiPlusDays ==========

    @Test
    void todayInTaipei_shouldNotBeNull() {
        assertNotNull(DateTimeUtils.todayInTaipei());
    }

    @Test
    void taipeiPlusDays_shouldOffsetByDays() {
        LocalDate base = DateTimeUtils.todayInTaipei();
        assertEquals(base.plusDays(2), DateTimeUtils.taipeiPlusDays(2));
    }

    @Test
    void formatPayUniDate_shouldUseDashFormat() {
        assertEquals("2026-03-31", DateTimeUtils.formatPayUniDate(LocalDate.of(2026, 3, 31)));
    }

    @Test
    void formatShippingDate_shouldUseCompactFormat() {
        assertEquals("20260331", DateTimeUtils.formatShippingDate(LocalDate.of(2026, 3, 31)));
    }

    // ========== parsePayUniDateTimeToUtc ==========

    @Test
    void parsePayUniDateTimeToUtc_shouldConvertTaipeiToUtc() {
        // 台北 2026-03-31 08:30:00 → UTC 2026-03-31 00:30:00
        LocalDateTime utc = DateTimeUtils.parsePayUniDateTimeToUtc("2026-03-31 08:30:00");
        assertEquals(LocalDateTime.of(2026, 3, 31, 0, 30, 0), utc);
    }

    @Test
    void parsePayUniDateTimeToUtc_shouldHandleCrossDayBoundary() {
        // 台北 2026-03-31 07:00:00 → UTC 2026-03-30 23:00:00（跨日）
        LocalDateTime utc = DateTimeUtils.parsePayUniDateTimeToUtc("2026-03-31 07:00:00");
        assertEquals(LocalDateTime.of(2026, 3, 30, 23, 0, 0), utc);
    }

    @Test
    void parsePayUniDateTimeToUtc_placeholder_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("0000-00-00 00:00:00"));
    }

    @Test
    void parsePayUniDateTimeToUtc_null_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc(null));
    }

    @Test
    void parsePayUniDateTimeToUtc_blank_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("   "));
    }

    // ── PayUni 的第二個哨兵 "-"（2026-09-10 dev 實證的 P0）──────────────────
    //
    // PayUni 對「尚未開始配送」的物流單，ShipStatusTime 回的是 "-" 而不是
    // "0000-00-00 00:00:00"。本方法原本只認後者 ⇒ "-" 走到 LocalDateTime.parse
    // 拋 DateTimeParseException ⇒ 物流追蹤端點對這類單一律 HTTP 500。
    //
    // 🔴 這幾支斷言的是**回 null**而不是「不拋例外」：兩者不等價——
    //    回一個「像的」時間（例如 now()）同樣不拋例外，但會讓 provider_status_time
    //    落一個假值，而假值比缺值更難發現。

    @Test
    void parsePayUniDateTimeToUtc_dashSentinel_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("-"));
    }

    @Test
    void parsePayUniDateTimeToUtc_dashSentinelWithSpaces_shouldReturnNull() {
        // 實際 payload 可能帶空白；trim 之後才比對。
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("  -  "));
    }

    @Test
    void parsePayUniDateTimeToUtc_unknownFormat_shouldReturnNullNotThrow() {
        // 未知格式（PayUni 改格式、或欄位被塞了別的東西）也必須回 null。
        // 呼叫端全都寫成 `if (result == null) { 兜底/告警 }`，是照 javadoc 的承諾寫的；
        // 拋例外會讓每一個正確使用它的呼叫端變成故障點。
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("2026/09/10 12:00:00"));
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("not-a-date"));
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("2026-09-10"));
    }

    @Test
    void parsePayUniDateTimeToUtc_validInput_stillConverts() {
        // 陰性對照：修掉哨兵與例外之後，正常路徑不能跟著壞掉。
        // 沒有這一條，把整個方法改成 `return null;` 也會讓上面三支全綠。
        LocalDateTime utc = DateTimeUtils.parsePayUniDateTimeToUtc("2026-09-10 16:30:00");
        assertEquals(LocalDateTime.of(2026, 9, 10, 8, 30, 0), utc);
    }

    // ========== parsePayUniAuthDayTimeToUtc ==========

    @Test
    void parsePayUniAuthDayTimeToUtc_shouldConvertTaipeiToUtc() {
        // 台北 2026-03-31 08:30:00 → UTC 2026-03-31 00:30:00
        LocalDateTime utc = DateTimeUtils.parsePayUniAuthDayTimeToUtc("20260331", "083000");
        assertEquals(LocalDateTime.of(2026, 3, 31, 0, 30, 0), utc);
    }

    @Test
    void parsePayUniAuthDayTimeToUtc_shouldHandleCrossDayBoundary() {
        // 台北 2026-03-31 06:59:59 → UTC 2026-03-30 22:59:59
        LocalDateTime utc = DateTimeUtils.parsePayUniAuthDayTimeToUtc("20260331", "065959");
        assertEquals(LocalDateTime.of(2026, 3, 30, 22, 59, 59), utc);
    }

    @Test
    void parsePayUniAuthDayTimeToUtc_nullAuthDay_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniAuthDayTimeToUtc(null, "083000"));
    }

    @Test
    void parsePayUniAuthDayTimeToUtc_blankAuthTime_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniAuthDayTimeToUtc("20260331", ""));
    }

    // ========== payUniDateEndOfDayToUtc ==========

    @Test
    void payUniDateEndOfDayToUtc_shouldReturn235959Taipei_asUtc() {
        // 台北 2026-03-31 23:59:59 → UTC 2026-03-31 15:59:59
        LocalDateTime utc = DateTimeUtils.payUniDateEndOfDayToUtc("2026-03-31");
        assertEquals(LocalDateTime.of(2026, 3, 31, 15, 59, 59), utc);
    }

    @Test
    void payUniDateEndOfDayToUtc_null_shouldReturnNull() {
        assertNull(DateTimeUtils.payUniDateEndOfDayToUtc(null));
    }

    @Test
    void payUniDateEndOfDayToUtc_blank_shouldReturnNull() {
        assertNull(DateTimeUtils.payUniDateEndOfDayToUtc("  "));
    }
}
