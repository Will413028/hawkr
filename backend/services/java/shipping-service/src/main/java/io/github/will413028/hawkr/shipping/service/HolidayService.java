package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.health.ComponentReadiness;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.entity.HolidayEntity;
import io.github.will413028.hawkr.shipping.entity.projection.HolidayYearCoverageRef;
import io.github.will413028.hawkr.shipping.mapper.HolidayMapper;
import jakarta.annotation.PostConstruct;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.StringReader;
import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * 節假日管理服務
 * - CSV 匯入（行政院人事行政總處 Google 行事曆格式）
 * - 記憶體快取（啟動載入 + 每月排程 refresh + 匯入後即時刷新）
 * - 工作天驗證與查詢
 *
 * 實作 {@link ComponentReadiness}：cache 首次載入完成前 readiness 報 DOWN，
 * 避免 generate_home_label 在假日 cache 空白時誤判 shipDate / deliveryDate。
 * 詳見 24 §3.3。
 */
@Service
public class HolidayService implements ComponentReadiness {

    private static final ObsLogger log = ObsLogger.forClass(HolidayService.class);
    private static final DateTimeFormatter CSV_DATE_FMT = DateTimeFormatter.ofPattern("yyyy/M/d");

    private final HolidayMapper holidayMapper;
    private final OkHttpClient httpClient;

    /** 記憶體快取：holiday_date → name */
    private final Map<LocalDate, String> holidayCache = new ConcurrentHashMap<>();

    /** 首次載入完成標記 — 供 ComponentReadiness 用 */
    private final AtomicBoolean cacheLoaded = new AtomicBoolean(false);

    public HolidayService(HolidayMapper holidayMapper) {
        this.holidayMapper = holidayMapper;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .build();
    }

    // ==================== 快取 ====================

    @PostConstruct
    public void loadCache() {
        List<HolidayEntity> all = holidayMapper.selectList(null);
        holidayCache.clear();
        for (HolidayEntity entity : all) {
            holidayCache.put(entity.getHolidayDate(), entity.getName());
        }
        cacheLoaded.set(true);
        log.info("HolidayService.loadCache.done",
                "假日快取載入完成，共 " + holidayCache.size() + " 筆");
    }

    /**
     * 月排程 refresh：每月 1 日 03:00 UTC 重新從 DB 全量載入。
     * 兜底覆蓋運維手動 SQL 更新 holidays 表後的同步需求。
     * 失敗時保留既有 cache，不影響 readiness（cacheLoaded 已為 true）。
     */
    @Scheduled(cron = "0 0 3 1 * *", zone = "UTC")
    public void scheduledRefresh() {
        try {
            loadCache();
            log.info("HolidayService.scheduledRefresh.done",
                    "假日快取月排程刷新完成，共 " + holidayCache.size() + " 筆");
        } catch (Exception e) {
            log.warn("HolidayService.scheduledRefresh.failed",
                    "假日快取月排程刷新失敗，保留既有 cache: " + e.getMessage());
        }
    }

    @Override
    public boolean isReady() {
        return cacheLoaded.get();
    }

    @Override
    public String name() {
        return "shipping-holiday";
    }

    // ==================== 工作天驗證 ====================

    /**
     * 檢查日期是否為工作天（不在 holidays 表）
     */
    public boolean isBusinessDay(LocalDate date) {
        return !holidayCache.containsKey(date);
    }

    /**
     * 取得假日名稱（非假日返回 null）
     */
    public String getHolidayName(LocalDate date) {
        return holidayCache.get(date);
    }

    /**
     * 從指定日期的下一天起算，找到第 N 個工作天
     */
    public LocalDate nextBusinessDay(LocalDate from, int n) {
        LocalDate cursor = from.plusDays(1);
        int count = 0;
        while (count < n) {
            if (isBusinessDay(cursor)) {
                count++;
                if (count == n) {
                    return cursor;
                }
            }
            cursor = cursor.plusDays(1);
        }
        return cursor;
    }

    /**
     * 從明日起算，取得接下來 count 個工作天
     */
    public List<LocalDate> getNextBusinessDays(int count) {
        List<LocalDate> result = new ArrayList<>();
        // §10.4 D6: 用台北時區避免 UTC 容器跨日邊界
        LocalDate today = DateTimeUtils.todayInTaipei();
        LocalDate cursor = today.plusDays(1);
        LocalDate safetyLimit = today.plusDays(60);
        while (result.size() < count) {
            if (isBusinessDay(cursor)) {
                result.add(cursor);
            }
            cursor = cursor.plusDays(1);
            if (cursor.isAfter(safetyLimit)) {
                break;
            }
        }
        return result;
    }

    /**
     * 取一段閉區間內的國定假日（服務間用；58 §5.1.14 的行事曆鏡像每日同步）。
     *
     * <p>🔴 <b>只回假日，不回工作天</b>。呼叫端要自己扣週六日——這是刻意的：
     * 「哪些日期是週末」不需要跨服務查詢，而把它算進來會讓回應大 5 倍且與
     * 呼叫端的時區假設耦合。
     *
     * @param fromDate 起（含）
     * @param toDate   迄（含）；早於 fromDate 時回空清單而不是丟例外——空區間是
     *                 合法輸入，呼叫端的視窗參數可能算出這個結果
     */
    public List<LocalDate> getHolidaysBetween(LocalDate fromDate, LocalDate toDate) {
        if (fromDate == null || toDate == null || toDate.isBefore(fromDate)) {
            return List.of();
        }
        return holidayMapper.selectHolidayDatesBetween(fromDate, toDate);
    }

    // ==================== CSV 匯入 ====================

    /**
     * 從政府 CSV URL 匯入假日資料
     *
     * @return ImportResult 匯入統計
     */
    @Transactional
    public ImportResult importFromCsvUrl(String csvUrl) {
        // 1. 下載 CSV
        String csvContent = downloadCsv(csvUrl);

        // 2. 解析 CSV
        List<ParsedHoliday> holidays = parseCsv(csvContent);
        if (holidays.isEmpty()) {
            throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_EMPTY);
        }

        // 3. 推算年份（取第一筆有效資料）
        short year = (short) holidays.get(0).date().getYear();

        // 4. 覆寫該年份資料
        return applyHolidays(year, holidays);
    }

    /**
     * 從上傳的 CSV 內容匯入指定年份假日資料
     *
     * <p>與 URL 版共用 {@link #applyHolidays}，差異在年份由呼叫端顯式宣告，
     * 並斷言宣告年份確實存在於 CSV，避免選錯年 / 傳錯檔覆寫他年資料。
     *
     * @return ImportResult 匯入統計
     */
    @Transactional
    public ImportResult importFromCsvContent(String declaredYear, String csvContent) {
        short year = Short.parseShort(declaredYear);

        // 1. 解析 CSV
        List<ParsedHoliday> holidays = parseCsv(csvContent);
        if (holidays.isEmpty()) {
            throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_EMPTY);
        }

        // 2. 斷言宣告年份在 CSV 至少一筆（否則檔案與年份不符）
        boolean hasDeclaredYear = holidays.stream()
                .anyMatch(h -> h.date().getYear() == year);
        if (!hasDeclaredYear) {
            throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_YEAR_MISMATCH, declaredYear);
        }

        // 3. 覆寫該年份資料
        return applyHolidays(year, holidays);
    }

    /**
     * 覆寫指定年份的假日資料：過濾至該年 + 去重 → 刪除舊資料 → 批次 INSERT → 刷新快取。
     * 政府 CSV 可能跨年，過濾與去重確保僅覆寫宣告年份。於呼叫端交易內執行。
     */
    private ImportResult applyHolidays(short year, List<ParsedHoliday> holidays) {
        // 刪除該年份舊資料
        long deletedCount = holidayMapper.delete(
                new LambdaQueryWrapper<HolidayEntity>().eq(HolidayEntity::getYear, year));

        // 批次 INSERT
        int importedCount = 0;
        Set<LocalDate> seen = new HashSet<>();
        for (ParsedHoliday h : holidays) {
            // 只匯入同年份，忽略重複
            if (h.date().getYear() != year || seen.contains(h.date())) {
                continue;
            }
            seen.add(h.date());

            HolidayEntity entity = new HolidayEntity();
            entity.setHolidayDate(h.date());
            entity.setName(h.name());
            entity.setYear(year);
            holidayMapper.insert(entity);
            importedCount++;
        }

        log.info("HolidayService.import.done",
                "假日匯入完成 year=" + year + " imported=" + importedCount + " deleted=" + deletedCount);

        // 刷新快取
        loadCache();

        return new ImportResult(year, importedCount, (int) deletedCount);
    }

    // ==================== 覆蓋現況查詢 ====================

    /**
     * 查詢假日資料覆蓋現況：各年度筆數 / 最大覆蓋日期，
     * 以及全域剩餘覆蓋天數（與 #898 告警 metric 同源公式，空表回 -1）。
     */
    public HolidayCoverage getHolidayCoverage() {
        List<HolidayYearCoverageRef> years = holidayMapper.listYearCoverage();
        Integer coverageDays = holidayMapper.selectCoverageDays();
        return new HolidayCoverage(years, coverageDays != null ? coverageDays : -1);
    }

    private String downloadCsv(String csvUrl) {
        Request request = new Request.Builder().url(csvUrl).build();
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful() || response.body() == null) {
                log.error("HolidayService.downloadCsv.httpError",
                        "CSV 下載失敗 HTTP " + response.code(), "csvUrl", csvUrl);
                throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_DOWNLOAD_FAILED);
            }
            return response.body().string();
        } catch (IOException e) {
            log.error("HolidayService.downloadCsv.ioError",
                    "CSV 下載失敗 csvUrl=" + csvUrl, e);
            throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_DOWNLOAD_FAILED);
        }
    }

    private List<ParsedHoliday> parseCsv(String csvContent) {
        List<ParsedHoliday> result = new ArrayList<>();
        try (BufferedReader reader = new BufferedReader(new StringReader(csvContent))) {
            String line;
            int lineNum = 0;
            while ((line = reader.readLine()) != null) {
                lineNum++;
                // 跳過 header（第一行）
                if (lineNum == 1) continue;
                // 跳過空行
                if (line.trim().isEmpty() || line.startsWith(",")) continue;

                String[] cols = line.split(",", -1);
                if (cols.length < 2) continue;

                String subject = cols[0].trim();
                String startDate = cols[1].trim();

                if (subject.isEmpty() || startDate.isEmpty()) continue;

                // 移除 BOM
                if (subject.startsWith("\uFEFF")) {
                    subject = subject.substring(1);
                }

                try {
                    LocalDate date = LocalDate.parse(startDate, CSV_DATE_FMT);
                    result.add(new ParsedHoliday(date, subject));
                } catch (Exception e) {
                    log.error("HolidayService.parseCsv.lineError",
                            "第 " + lineNum + " 行日期解析失敗: " + startDate);
                }
            }
        } catch (IOException e) {
            log.error("HolidayService.parseCsv.ioError",
                    "CSV 解析失敗", e);
            throw new ShippingBusinessException(GrpcResponseCode.HOLIDAY_CSV_PARSE_FAILED);
        }
        return result;
    }

    // ==================== 內部型別 ====================

    public record ImportResult(short year, int importedCount, int deletedCount) {}
    public record HolidayCoverage(List<HolidayYearCoverageRef> years, int coverageDays) {}
    private record ParsedHoliday(LocalDate date, String name) {}
}
