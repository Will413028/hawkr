package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.health.ComponentReadiness;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.DeliveryZoneKey;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.shipping.entity.ShippingFeeTierEntity;
import io.github.will413028.hawkr.shipping.entity.ShippingMethodConfigEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingFeeTierMapper;
import io.github.will413028.hawkr.shipping.mapper.ShippingMethodConfigMapper;
import jakarta.annotation.PostConstruct;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.stream.Collectors;

/**
 * 運費費率快取服務
 * 啟動時全量載入 DB，每小時自動重新整理
 *
 * 實作 {@link ComponentReadiness}：cache 首次載入完成前 readiness 報 DOWN，
 * K8s readinessProbe 失敗，pod 不接結帳流量。詳見 21 §3.1。
 */
@Service
public class ShippingFeeService implements ComponentReadiness {

    private static final ObsLogger log = ObsLogger.forClass(ShippingFeeService.class);

    private final ShippingMethodConfigMapper configMapper;
    private final ShippingFeeTierMapper tierMapper;

    /** configs 快取：key = "METHOD:TEMP" */
    private volatile Map<String, ShippingMethodConfigEntity> configCache = new ConcurrentHashMap<>();

    /** tiers 快取：key = "METHOD:TEMP:ZONE"，value 按 max_dimension_cm 升序 */
    private volatile Map<String, List<ShippingFeeTierEntity>> tierCache = new ConcurrentHashMap<>();

    /** 所有啟用的 configs（供 GetShippingFeeConfigs 回傳） */
    private volatile List<ShippingMethodConfigEntity> allConfigs = List.of();

    /** 所有啟用的 tiers（供 GetShippingFeeConfigs 回傳） */
    private volatile List<ShippingFeeTierEntity> allTiers = List.of();

    /** 首次載入完成標記 — 供 ComponentReadiness 用 */
    private final AtomicBoolean cacheLoaded = new AtomicBoolean(false);

    public ShippingFeeService(ShippingMethodConfigMapper configMapper,
                               ShippingFeeTierMapper tierMapper) {
        this.configMapper = configMapper;
        this.tierMapper = tierMapper;
    }

    @PostConstruct
    public void init() {
        loadCache();
        cacheLoaded.set(true);
        log.info("ShippingFeeService.init", "運費快取載入完成, configs=" + configCache.size() + ", tierGroups=" + tierCache.size());
    }

    /** 每小時重新整理快取 */
    @Scheduled(fixedRate = 3600000)
    public void refresh() {
        try {
            loadCache();
            log.info("ShippingFeeService.refresh", "運費快取重新整理完成");
        } catch (Exception e) {
            log.error("ShippingFeeService.refresh.error", "運費快取重新整理失敗: " + e.getMessage(), e);
        }
    }

    @Override
    public boolean isReady() {
        return cacheLoaded.get();
    }

    @Override
    public String name() {
        return "shipping-fee-cache";
    }

    private void loadCache() {
        // 載入 configs
        List<ShippingMethodConfigEntity> configs = configMapper.selectList(
                new LambdaQueryWrapper<ShippingMethodConfigEntity>()
                        .eq(ShippingMethodConfigEntity::getEnabled, true));

        Map<String, ShippingMethodConfigEntity> newConfigCache = new ConcurrentHashMap<>();
        for (ShippingMethodConfigEntity c : configs) {
            newConfigCache.put(configKey(c.getShippingMethod(), c.getDeliveryTemp()), c);
        }

        // 載入 tiers
        List<ShippingFeeTierEntity> tiers = tierMapper.selectList(
                new LambdaQueryWrapper<ShippingFeeTierEntity>()
                        .eq(ShippingFeeTierEntity::getEnabled, true)
                        .orderByAsc(ShippingFeeTierEntity::getMaxDimensionCm));

        Map<String, List<ShippingFeeTierEntity>> newTierCache = tiers.stream()
                .collect(Collectors.groupingBy(
                        t -> tierKey(t.getShippingMethod(), t.getDeliveryTemp(), t.getDeliveryZone()),
                        ConcurrentHashMap::new,
                        Collectors.toList()));

        // 原子替換
        this.configCache = newConfigCache;
        this.tierCache = newTierCache;
        this.allConfigs = List.copyOf(configs);
        this.allTiers = List.copyOf(tiers);
    }

    // ==================== 查詢 API ====================

    /**
     * 取得配送方式設定
     * @return null 表示該組合不可用
     */
    public ShippingMethodConfigEntity getConfig(String method, String temp) {
        return configCache.get(configKey(method, temp));
    }

    /**
     * 判定該配送方式在系統層級是否有任一可用組合（任一溫層 enabled=true）。
     *
     * 用途:
     * - listSellerShippingMethods：method 全系統暫停（如 Uber Phase 1 預留） → 對所有賣家統一回 available=false
     * - validateProductShipping：method 完全無 config 時，先報 METHOD_NOT_AVAILABLE 而非誤導為 METHOD_NOT_ONBOARDED / METHOD_NOT_SUPPORTED
     *
     * configCache 載入時已 filter enabled=true，cache 內鍵代表系統開放的 (method, temp) 組合。
     *
     * @return true 表示該 method 至少有一個溫層的 config row enabled
     */
    public boolean hasAnyEnabledConfig(String shippingMethod) {
        if (shippingMethod == null || shippingMethod.isBlank()) {
            return false;
        }
        String prefix = shippingMethod + ":";
        for (String key : configCache.keySet()) {
            if (key.startsWith(prefix)) {
                return true;
            }
        }
        return false;
    }

    /**
     * 查找運費
     * @param dimensionSum 材積總和（cm）
     * @return null 表示超過最大級距或不可用
     */
    public Long lookupFee(String method, String temp, String zone, int dimensionSum) {
        List<ShippingFeeTierEntity> tiers = tierCache.get(tierKey(method, temp, zone));
        if (tiers == null || tiers.isEmpty()) {
            return null;
        }

        for (ShippingFeeTierEntity tier : tiers) {
            // maxDimensionCm=0 表示統一價
            if (tier.getMaxDimensionCm() == 0 || dimensionSum <= tier.getMaxDimensionCm()) {
                return tier.getFee();
            }
        }

        // 超過最大級距
        return null;
    }

    /**
     * 取得全部啟用的 configs（供 gRPC GetShippingFeeConfigs）
     */
    public List<ShippingMethodConfigEntity> getAllConfigs() {
        return allConfigs;
    }

    /**
     * 取得全部啟用的 tiers（供 gRPC GetShippingFeeConfigs）
     */
    public List<ShippingFeeTierEntity> getAllTiers() {
        return allTiers;
    }

    /**
     * 計算可用配送選項
     * 給定包裹尺寸與重量，回傳所有可用配送方式（按溫層分組）
     */
    /**
     * 計算可用配送選項與運費
     *
     * @param sellerAvailableMethods 賣家已開通的配送方式集合（"METHOD:GOODS_TYPE"），null 表示不過濾
     */
    public CalculateResult calculateShippingOptions(int lengthCm, int widthCm, int heightCm,
                                                     long weightGrams, String deliveryZone,
                                                     Set<String> sellerAvailableMethods) {
        int dimensionSum = lengthCm + widthCm + heightCm;
        int longestSide = Math.max(lengthCm, Math.max(widthCm, heightCm));

        if (deliveryZone == null || deliveryZone.isEmpty()) {
            deliveryZone = DeliveryZoneKey.DOMESTIC.value();
        }

        // 依溫層分組
        Map<ShippingTemperature, List<ShippingOptionItem>> grouped = new LinkedHashMap<>();

        for (ShippingTemperature temp : ShippingTemperature.values()) {
            List<ShippingOptionItem> methods = new ArrayList<>();

            for (ShippingMethod method : List.of(ShippingMethod.TAQBIN, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP)) {
                // 賣家開通狀態過濾（物流設定獨立化）
                if (sellerAvailableMethods != null) {
                    // 宅配不分溫層，key 為 "TAQBIN:"
                    // 超商分溫層，key 為 "SEVEN_ELEVEN_STORE_PICKUP:AMBIENT" 或 "SEVEN_ELEVEN_STORE_PICKUP:FROZEN"
                    String filterKey = method == ShippingMethod.TAQBIN
                            ? method.value() + ":"
                            : method.value() + ":" + temp.value();
                    if (!sellerAvailableMethods.contains(filterKey)) {
                        continue; // 賣家未開通此配送方式
                    }
                }

                ShippingMethodConfigEntity config = getConfig(method.value(), temp.value());
                if (config == null) {
                    continue; // 該組合不可用（如 7-11 冷藏）
                }

                // 檢查限制
                if (config.getMaxDimensionSumCm() > 0 && dimensionSum > config.getMaxDimensionSumCm()) {
                    continue;
                }
                if (config.getMaxSingleSideCm() > 0 && longestSide > config.getMaxSingleSideCm()) {
                    continue;
                }
                if (config.getMaxWeightGrams() > 0 && weightGrams > config.getMaxWeightGrams()) {
                    continue;
                }

                // 店到店固定 DOMESTIC
                String zone = method == ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP
                        ? DeliveryZoneKey.DOMESTIC.value()
                        : deliveryZone;

                Long fee = lookupFee(method.value(), temp.value(), zone, dimensionSum);
                if (fee == null) {
                    continue; // 無對應費率
                }

                methods.add(new ShippingOptionItem(
                        method.value(),
                        method.label(),
                        fee,
                        config.getEstimatedDays(),
                        config.getSettlementDays(),
                        config.getMaxDimensionSumCm(),
                        config.getMaxSingleSideCm(),
                        config.getMaxWeightGrams()
                ));
            }

            if (!methods.isEmpty()) {
                grouped.put(temp, methods);
            }
        }

        return new CalculateResult(grouped, weightGrams, dimensionSum, longestSide);
    }

    // ==================== 商品配送驗證 ====================

    /**
     * 驗證商品配送方式合法性（材積、重量、賣家開通狀態）
     *
     * @param items                  待驗證的 SKU 配送項目
     * @param sellerAvailableMethods 賣家已開通的配送方式集合（"METHOD:GOODS_TYPE"），null 表示不過濾
     */
    public ValidationResult validateProductShipping(List<ValidationItem> items,
                                                     Set<String> sellerAvailableMethods) {
        List<ValidationResultItem> results = new ArrayList<>();
        boolean allValid = true;

        // 反推「該 method 是否有任何溫層已開通」— 用於區分 METHOD_NOT_ONBOARDED vs TEMPERATURE_NOT_ONBOARDED
        Set<String> sellerOnboardedMethods = sellerAvailableMethods == null
                ? null
                : sellerAvailableMethods.stream()
                        .map(s -> {
                            int idx = s.indexOf(':');
                            return idx >= 0 ? s.substring(0, idx) : s;
                        })
                        .collect(Collectors.toSet());

        for (ValidationItem item : items) {
            String method = item.shippingMethod();
            String temp = item.temperature();
            String methodLabel = methodLabel(method);
            String tempLabel = temperatureLabel(temp);

            // 0. 系統層級檢查:該 method 是否有任何 enabled config(如 Uber Phase 1 預留全系統暫停)
            if (!hasAnyEnabledConfig(method)) {
                allValid = false;
                results.add(new ValidationResultItem(
                        item.skuName(), method, temp, false,
                        "METHOD_NOT_AVAILABLE",
                        "「" + methodLabel + "」服務目前暫停使用,請改選其他配送方式",
                        0, 0, 0L));
                continue;
            }

            // 1. 賣家開通狀態檢查（(method, temperature) 配對粒度）
            if (sellerAvailableMethods != null) {
                String filterKey = ShippingMethod.TAQBIN.value().equals(method)
                        ? method + ":"
                        : method + ":" + temp;
                if (!sellerAvailableMethods.contains(filterKey)) {
                    allValid = false;
                    boolean methodHasAnyOnboarded = sellerOnboardedMethods.contains(method);
                    String reason;
                    String message;
                    if (methodHasAnyOnboarded) {
                        reason = "TEMPERATURE_NOT_ONBOARDED";
                        message = "您的商品為「" + tempLabel + "」溫層，「" + methodLabel
                                + "」尚未開通此溫層，請至「物流設定 → " + methodLabel + "」新增「"
                                + tempLabel + "」設定";
                    } else {
                        reason = "METHOD_NOT_ONBOARDED";
                        message = "您尚未開通「" + methodLabel + "」配送方式，請至「物流設定」完成開通";
                    }
                    results.add(new ValidationResultItem(
                            item.skuName(), method, temp, false,
                            reason, message,
                            0, 0, 0L));
                    continue;
                }
            }

            // 2. 配送方式+溫層組合是否存在
            ShippingMethodConfigEntity config = getConfig(method, temp);
            if (config == null) {
                allValid = false;
                results.add(new ValidationResultItem(
                        item.skuName(), method, temp, false,
                        "METHOD_NOT_SUPPORTED",
                        "「" + methodLabel + "」不支援「" + tempLabel + "」溫層",
                        0, 0, 0L));
                continue;
            }

            int dimensionSum = item.lengthCm() + item.widthCm() + item.heightCm();
            int longestSide = Math.max(item.lengthCm(), Math.max(item.widthCm(), item.heightCm()));

            // 3. 材積總和檢查
            if (config.getMaxDimensionSumCm() > 0 && dimensionSum > config.getMaxDimensionSumCm()) {
                allValid = false;
                results.add(new ValidationResultItem(
                        item.skuName(), method, temp, false,
                        "DIMENSION_SUM_EXCEEDED",
                        "材積總和 " + dimensionSum + "cm 超過「" + methodLabel + "/" + tempLabel + "」上限 " + config.getMaxDimensionSumCm() + "cm",
                        config.getMaxDimensionSumCm(), config.getMaxSingleSideCm(), config.getMaxWeightGrams()));
                continue;
            }

            // 4. 單邊長度檢查
            if (config.getMaxSingleSideCm() > 0 && longestSide > config.getMaxSingleSideCm()) {
                allValid = false;
                results.add(new ValidationResultItem(
                        item.skuName(), method, temp, false,
                        "SINGLE_SIDE_EXCEEDED",
                        "最長邊 " + longestSide + "cm 超過「" + methodLabel + "/" + tempLabel + "」上限 " + config.getMaxSingleSideCm() + "cm",
                        config.getMaxDimensionSumCm(), config.getMaxSingleSideCm(), config.getMaxWeightGrams()));
                continue;
            }

            // 5. 重量檢查
            if (config.getMaxWeightGrams() > 0 && item.weightGrams() > config.getMaxWeightGrams()) {
                allValid = false;
                results.add(new ValidationResultItem(
                        item.skuName(), method, temp, false,
                        "WEIGHT_EXCEEDED",
                        "重量 " + item.weightGrams() + "g 超過「" + methodLabel + "/" + tempLabel + "」上限 " + config.getMaxWeightGrams() + "g",
                        config.getMaxDimensionSumCm(), config.getMaxSingleSideCm(), config.getMaxWeightGrams()));
                continue;
            }

            // 全部通過
            results.add(new ValidationResultItem(
                    item.skuName(), method, temp, true,
                    "", "", 0, 0, 0L));
        }

        return new ValidationResult(allValid, results);
    }

    private static String methodLabel(String methodValue) {
        if (methodValue == null || methodValue.isBlank()) {
            return "";
        }
        try {
            return ShippingMethod.fromValue(methodValue).label();
        } catch (Exception ignored) {
            return methodValue;
        }
    }

    private static String temperatureLabel(String tempValue) {
        if (tempValue == null || tempValue.isBlank()) {
            return "";
        }
        try {
            return ShippingTemperature.fromValue(tempValue).label();
        } catch (Exception ignored) {
            return tempValue;
        }
    }

    // ==================== Key 工具方法 ====================

    private static String configKey(String method, String temp) {
        return method + ":" + temp;
    }

    private static String tierKey(String method, String temp, String zone) {
        return method + ":" + temp + ":" + zone;
    }

    // ==================== 結果 DTO ====================

    public record CalculateResult(
            Map<ShippingTemperature, List<ShippingOptionItem>> temperatureGroups,
            long weightGrams,
            int dimensionSum,
            int longestSide
    ) {}

    public record ShippingOptionItem(
            String shippingMethod,
            String displayName,
            long fee,
            int estimatedDays,
            int settlementDays,
            int maxDimensionSumCm,
            int maxSingleSideCm,
            long maxWeightGrams
    ) {}

    // ==================== 驗證 DTO ====================

    public record ValidationItem(
            String skuName,
            String shippingMethod,
            String temperature,
            int lengthCm,
            int widthCm,
            int heightCm,
            long weightGrams
    ) {}

    public record ValidationResult(
            boolean allValid,
            List<ValidationResultItem> results
    ) {}

    public record ValidationResultItem(
            String skuName,
            String shippingMethod,
            String temperature,
            boolean valid,
            String rejectionReason,
            String rejectionMessage,
            int maxDimensionSumCm,
            int maxSingleSideCm,
            long maxWeightGrams
    ) {}
}
