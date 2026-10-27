package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.DeliveryZoneKey;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * 運費費率本地快取（Lazy + TTL + Circuit Breaker）
 *
 * 設計沿革：
 *   v1: 啟動時 @PostConstruct 全量 warm + @Scheduled 每小時刷新 + daemon retry
 *       → dev rolling update 觀測到 ~21 秒冷啟動拒絕視窗，回 ALL_SHIPPING_EXCEEDED
 *   v2（本檔，2026-06-04+）: 移除 warm-up 與排程，改 lazy load 配 5 分鐘 TTL，
 *       gRPC 呼叫透過 {@link ShippingFeeGrpcFetcher} 的 Resilience4j @CircuitBreaker 保護。
 *       CB 開路時走 stale cache fallback；cache 為空才拋 SERVICE_UNAVAILABLE。
 *
 * 詳見 20 §4.9 與 21 §3.2、§8.3。
 */
@Component
public class ShippingFeeCacheClient {

    private static final ObsLogger log = ObsLogger.forClass(ShippingFeeCacheClient.class);

    /** Cache 有效期：超過後下個請求觸發 refresh（仍會 fallback stale 若 refresh 失敗） */
    private static final Duration TTL = Duration.ofMinutes(5);

    private final ShippingFeeGrpcFetcher fetcher;

    /** 配送方式設定快取：key = "METHOD:TEMP" */
    private volatile Map<String, ConfigEntry> configCache = new ConcurrentHashMap<>();

    /** 費率級距快取：key = "METHOD:TEMP:ZONE"，按 maxDimensionCm 升序 */
    private volatile Map<String, List<TierEntry>> tierCache = new ConcurrentHashMap<>();

    /** 最近一次成功載入時間；null 表示從未成功（cold） */
    private volatile Instant lastSuccessAt;

    public ShippingFeeCacheClient(ShippingFeeGrpcFetcher fetcher) {
        this.fetcher = fetcher;
    }

    // ==================== 查詢 API ====================

    /**
     * 取得配送方式設定
     * @return null 表示該組合不可用；cache 為空 + fetch 失敗會拋 SERVICE_UNAVAILABLE
     */
    public ConfigEntry getConfig(String method, String temp) {
        ensureFresh();
        return configCache.get(configKey(method, temp));
    }

    /**
     * 查找運費
     * @param dimensionSum 材積總和（cm）
     * @return null 表示超過最大級距或不可用；cache 為空 + fetch 失敗會拋 SERVICE_UNAVAILABLE
     */
    public Long lookupFee(String method, String temp, String zone, int dimensionSum) {
        ensureFresh();
        List<TierEntry> tiers = tierCache.get(tierKey(method, temp, zone));
        if (tiers == null || tiers.isEmpty()) {
            return null;
        }

        for (TierEntry tier : tiers) {
            // maxDimensionCm=0 表示統一價
            if (tier.maxDimensionCm() == 0 || dimensionSum <= tier.maxDimensionCm()) {
                return tier.fee();
            }
        }

        return null;
    }

    /**
     * 檢查材積/重量是否在限制內
     */
    public boolean isWithinLimit(ConfigEntry config, int totalDimSum, int maxSingleSide, long totalWeightGram) {
        if (config.maxDimensionSumCm() > 0 && totalDimSum > config.maxDimensionSumCm()) return false;
        if (config.maxSingleSideCm() > 0 && maxSingleSide > config.maxSingleSideCm()) return false;
        return config.maxWeightGrams() <= 0 || totalWeightGram <= config.maxWeightGrams();
    }

    /**
     * ShippingRoute → delivery_zone 轉換
     */
    public static String toDeliveryZone(ShippingRoute route) {
        return switch (route) {
            case DOMESTIC -> DeliveryZoneKey.DOMESTIC.value();
            case TO_OFFSHORE, FROM_OFFSHORE -> DeliveryZoneKey.OFFSHORE.value();
            case OFFSHORE_TO_OFFSHORE -> DeliveryZoneKey.INTER_ISLAND.value();
        };
    }

    // ==================== 內部 ====================

    /**
     * 確保 cache 在 TTL 內。stale 時 synchronized 觸發 refresh；
     * fetch 失敗有兩條路：
     *   - cache 非空 → log warn 用 stale
     *   - cache 為空（cold + shipping-service 不可用）→ 拋 SERVICE_UNAVAILABLE
     */
    private void ensureFresh() {
        Instant snapshotLast = this.lastSuccessAt;
        if (snapshotLast != null && Instant.now().isBefore(snapshotLast.plus(TTL))) {
            return;
        }
        synchronized (this) {
            // double-check：另一執行緒可能已 refresh
            if (this.lastSuccessAt != null && Instant.now().isBefore(this.lastSuccessAt.plus(TTL))) {
                return;
            }
            try {
                ShippingFeeGrpcFetcher.Snapshot snapshot = fetcher.fetchAll();
                this.configCache = snapshot.configs();
                this.tierCache = snapshot.tiers();
                this.lastSuccessAt = Instant.now();
                log.info("ShippingFeeCacheClient.refresh.success",
                        "運費快取刷新完成, configs=" + configCache.size() + ", tierGroups=" + tierCache.size());
            } catch (Exception e) {
                if (this.lastSuccessAt == null) {
                    log.error("ShippingFeeCacheClient.refresh.coldFailure",
                            "運費快取冷啟動載入失敗且無 stale 可用: " + e.getMessage());
                    throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE,
                            "shipping fee cache cold-load failed: " + e.getMessage());
                }
                log.warn("ShippingFeeCacheClient.refresh.degraded",
                        "運費快取刷新失敗，沿用 stale cache (" + e.getClass().getSimpleName() + "): " + e.getMessage());
                // 不更新 lastSuccessAt → 下個請求仍會嘗試 refresh（受 CB 速率保護）
            }
        }
    }

    // ==================== Key 工具方法 ====================

    private static String configKey(String method, String temp) {
        return method + ":" + temp;
    }

    private static String tierKey(String method, String temp, String zone) {
        return method + ":" + temp + ":" + zone;
    }

    // ==================== 資料結構 ====================

    public record ConfigEntry(
            String shippingMethod,
            String deliveryTemp,
            int maxDimensionSumCm,
            int maxSingleSideCm,
            long maxWeightGrams,
            int estimatedDays,
            int settlementDays
    ) {}

    public record TierEntry(
            String shippingMethod,
            String deliveryTemp,
            String deliveryZone,
            int maxDimensionCm,
            long fee
    ) {}
}
