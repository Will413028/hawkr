package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.dto.response.PopularHashtagItemDTO;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.repository.ProductHashtagRepository;
import io.github.will413028.hawkr.product.repository.ProductHashtagRepository.PopularHashtagRow;
import org.springframework.stereotype.Service;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;
import java.util.ArrayList;
import java.util.List;

/**
 * 區間熱門 Hashtag 查詢 Service（33-商品 Hashtag 模組詳細設計書 §5.2）。
 *
 * <p>處理：
 * <ul>
 *   <li>YYYY-MM-DD 日期字串解析</li>
 *   <li>§5.2.2 日期區間補值規則（兩端皆可空，預設近 1 個月）</li>
 *   <li>區間驗證（≤ 1 年、start ≤ end）</li>
 *   <li>內部 SQL 邊界轉換（左閉右開：endDate.plusDays(1).atStartOfDay()）</li>
 *   <li>呼叫 {@link ProductHashtagRepository#selectPopularInRange} 查詢</li>
 * </ul>
 *
 * <p>純讀 Service，不引入快取、不引入 reconciliation（33 §11 / docs/開發規範/後端服務責任邊界規範.md）。
 */
@Service
public class PopularHashtagService {

    private static final ObsLogger log = ObsLogger.forClass(PopularHashtagService.class);

    /** 預設區間長度：1 個月（兩端皆空時用） */
    private static final int DEFAULT_RANGE_MONTHS = 1;

    /** 最大區間長度：1 年（含） */
    private static final int MAX_RANGE_YEARS = 1;

    /** 預設與最大 limit */
    public static final int DEFAULT_LIMIT = 50;
    public static final int MAX_LIMIT = 50;

    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyy-MM-dd");

    private final ProductHashtagRepository productHashtagRepository;

    public PopularHashtagService(ProductHashtagRepository productHashtagRepository) {
        this.productHashtagRepository = productHashtagRepository;
    }

    /**
     * 查詢區間熱門 hashtag。
     *
     * @param startFromRaw 區間下界（YYYY-MM-DD，可空）
     * @param endToRaw     區間上界（YYYY-MM-DD，可空）
     * @param limit        筆數上限（0 或負數視為 DEFAULT_LIMIT；> MAX_LIMIT 視為 MAX_LIMIT）
     */
    public List<PopularHashtagItemDTO> getPopularHashtags(String startFromRaw, String endToRaw, int limit) {
        log.info("PopularHashtagService.query.start",
                "start_from=" + startFromRaw + ", end_to=" + endToRaw + ", limit=" + limit);

        long t0 = System.currentTimeMillis();

        LocalDate startFrom = parseDateOrNull(startFromRaw, "startFrom");
        LocalDate endTo = parseDateOrNull(endToRaw, "endTo");
        LocalDate today = DateTimeUtils.todayUtc();

        DateRange range = resolveRange(startFrom, endTo, today);

        int effectiveLimit = resolveLimit(limit);

        LocalDateTime sqlStart = range.startDate.atStartOfDay();
        LocalDateTime sqlEnd = range.endDate.plusDays(1).atStartOfDay();

        String activeStatus = ProductStatus.ACTIVE.value();

        List<PopularHashtagRow> rows = productHashtagRepository.selectPopularInRange(
                sqlStart, sqlEnd, activeStatus, effectiveLimit);

        List<PopularHashtagItemDTO> records = new ArrayList<>(rows.size());
        for (PopularHashtagRow r : rows) {
            records.add(PopularHashtagItemDTO.builder()
                    .hashtag(r.getHashtag())
                    .count(r.getCount())
                    .build());
        }

        long elapsed = System.currentTimeMillis() - t0;
        log.info("PopularHashtagService.query.success",
                "records=" + records.size() + ", sql_ms=" + elapsed
                        + ", start=" + range.startDate + ", end=" + range.endDate);

        return records;
    }

    /** §5.2.2 規則計算 [startDate, endDate] 區間（兩端皆 inclusive）。 */
    private DateRange resolveRange(LocalDate startFrom, LocalDate endTo, LocalDate today) {
        LocalDate startDate;
        LocalDate endDate;

        if (startFrom == null && endTo == null) {
            endDate = today;
            startDate = today.minusMonths(DEFAULT_RANGE_MONTHS);
        } else if (startFrom != null && endTo == null) {
            startDate = startFrom;
            LocalDate plus1y = startFrom.plusYears(MAX_RANGE_YEARS);
            endDate = plus1y.isBefore(today) ? plus1y : today;
        } else if (startFrom == null) {
            endDate = endTo;
            startDate = endTo.minusYears(MAX_RANGE_YEARS);
        } else {
            startDate = startFrom;
            endDate = endTo;
        }

        if (startDate.isAfter(endDate)) {
            rejectInvalid("startFrom must not be later than endTo", "range_order");
        }

        if (endDate.isAfter(startDate.plusYears(MAX_RANGE_YEARS))) {
            rejectInvalid("popular hashtag query range must not exceed 1 year", "range_exceeded");
        }

        return new DateRange(startDate, endDate);
    }

    private int resolveLimit(int limit) {
        if (limit < 1) return DEFAULT_LIMIT;
        return Math.min(limit, MAX_LIMIT);
    }

    private LocalDate parseDateOrNull(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) return null;
        try {
            return LocalDate.parse(raw.trim(), DATE_FMT);
        } catch (DateTimeParseException e) {
            rejectInvalid("invalid date format: " + fieldName + " must be YYYY-MM-DD", "date_format");
            return null;
        }
    }

    private void rejectInvalid(String message, String reason) {
        log.warn("PopularHashtagService.query.invalid_argument",
                "reason=" + reason + ", message=" + message);
        throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, message);
    }

    private record DateRange(LocalDate startDate, LocalDate endDate) {}
}
