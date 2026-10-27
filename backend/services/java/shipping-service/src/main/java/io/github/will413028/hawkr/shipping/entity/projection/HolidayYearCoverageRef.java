package io.github.will413028.hawkr.shipping.entity.projection;

import java.time.LocalDate;

/**
 * 假日覆蓋現況查詢投影（issue #1359）。各年度假日筆數與最大覆蓋日期。
 */
public record HolidayYearCoverageRef(
        Short year,
        Long count,
        LocalDate maxHolidayDate
) {}
