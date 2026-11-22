package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.health.ComponentReadiness;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.shipping.entity.HolidayEntity;
import io.github.will413028.hawkr.shipping.mapper.HolidayMapper;
import jakarta.annotation.PostConstruct;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.time.LocalDate;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * 節假日管理服務
 * - 記憶體快取（啟動載入 + 每月排程 refresh）
 * - 工作天驗證與查詢
 *
 * 實作 {@link ComponentReadiness}：cache 首次載入完成前 readiness 報 DOWN，
 * 避免 generate_home_label 在假日 cache 空白時誤判 shipDate / deliveryDate。
 * 詳見 24 §3.3。
 */
@Service
public class HolidayService implements ComponentReadiness {

    private static final ObsLogger log = ObsLogger.forClass(HolidayService.class);

    private final HolidayMapper holidayMapper;

    /** 記憶體快取：holiday_date → name */
    private final Map<LocalDate, String> holidayCache = new ConcurrentHashMap<>();

    /** 首次載入完成標記 — 供 ComponentReadiness 用 */
    private final AtomicBoolean cacheLoaded = new AtomicBoolean(false);

    public HolidayService(HolidayMapper holidayMapper) {
        this.holidayMapper = holidayMapper;
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
}
