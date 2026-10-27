package io.github.will413028.hawkr.shipping.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.shipping.entity.HolidayEntity;
import io.github.will413028.hawkr.shipping.entity.projection.HolidayYearCoverageRef;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Select;

import java.util.List;

@Mapper
public interface HolidayMapper extends BaseMapper<HolidayEntity> {

    /**
     * 各年度覆蓋現況（issue #1359）：年份 / 該年筆數 / 該年最大假日日期。
     */
    @Select("SELECT year, COUNT(*) AS count, MAX(holiday_date) AS max_holiday_date "
            + "FROM holidays GROUP BY year ORDER BY year")
    List<HolidayYearCoverageRef> listYearCoverage();

    /**
     * 全域剩餘覆蓋天數 = MAX(holiday_date) - CURRENT_DATE，空表回 -1。
     * 公式與 #898 告警 metric（monitoring/sql-exporter/sql_exporter.yml:60）同源。
     */
    @Select("SELECT COALESCE(MAX(holiday_date) - CURRENT_DATE, -1) FROM holidays")
    Integer selectCoverageDays();

    /**
     * 取一段閉區間內的假日日期（升冪）。
     *
     * <p>invoice-service 的行事曆鏡像每日整段覆蓋時用（58 §5.1.14）。回的是
     * <b>日期本身</b>而不是 Entity：呼叫端只需要日期集合，而回 Entity 會踩到
     * §11.3 的 partial projection 禁令（本查詢不打算 SELECT 全欄）。
     */
    @Select("SELECT holiday_date FROM holidays "
            + "WHERE holiday_date >= #{fromDate} AND holiday_date <= #{toDate} "
            + "ORDER BY holiday_date")
    List<java.time.LocalDate> selectHolidayDatesBetween(
            @org.apache.ibatis.annotations.Param("fromDate") java.time.LocalDate fromDate,
            @org.apache.ibatis.annotations.Param("toDate") java.time.LocalDate toDate);
}
