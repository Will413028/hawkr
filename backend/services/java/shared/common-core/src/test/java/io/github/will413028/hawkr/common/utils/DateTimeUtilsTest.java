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


    @Test
    void parsePayUniDateTimeToUtc_shouldConvertTaipeiToUtc() {
        LocalDateTime utc = DateTimeUtils.parsePayUniDateTimeToUtc("2026-03-31 08:30:00");
        assertEquals(LocalDateTime.of(2026, 3, 31, 0, 30, 0), utc);
    }

    @Test
    void parsePayUniDateTimeToUtc_shouldHandleCrossDayBoundary() {
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


    @Test
    void parsePayUniDateTimeToUtc_dashSentinel_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("-"));
    }

    @Test
    void parsePayUniDateTimeToUtc_dashSentinelWithSpaces_shouldReturnNull() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("  -  "));
    }

    @Test
    void parsePayUniDateTimeToUtc_unknownFormat_shouldReturnNullNotThrow() {
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("2026/09/10 12:00:00"));
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("not-a-date"));
        assertNull(DateTimeUtils.parsePayUniDateTimeToUtc("2026-09-10"));
    }

    @Test
    void parsePayUniDateTimeToUtc_validInput_stillConverts() {
        LocalDateTime utc = DateTimeUtils.parsePayUniDateTimeToUtc("2026-09-10 16:30:00");
        assertEquals(LocalDateTime.of(2026, 9, 10, 8, 30, 0), utc);
    }


    @Test
    void parsePayUniAuthDayTimeToUtc_shouldConvertTaipeiToUtc() {
        LocalDateTime utc = DateTimeUtils.parsePayUniAuthDayTimeToUtc("20260331", "083000");
        assertEquals(LocalDateTime.of(2026, 3, 31, 0, 30, 0), utc);
    }

    @Test
    void parsePayUniAuthDayTimeToUtc_shouldHandleCrossDayBoundary() {
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


    @Test
    void payUniDateEndOfDayToUtc_shouldReturn235959Taipei_asUtc() {
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
