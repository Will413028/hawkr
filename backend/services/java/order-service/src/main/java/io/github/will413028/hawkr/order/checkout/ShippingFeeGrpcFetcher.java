package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.shipping.grpc.GetShippingFeeConfigsResponse;
import io.github.will413028.hawkr.shipping.grpc.ShippingFeeTierProto;
import io.github.will413028.hawkr.shipping.grpc.ShippingMethodConfigProto;
import io.github.resilience4j.circuitbreaker.annotation.CircuitBreaker;
import org.springframework.stereotype.Component;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * 取 Shipping Service 全量運費設定的 gRPC 包裝層。
 *
 * 獨立於 {@link ShippingFeeCacheClient} 的理由：Resilience4j {@code @CircuitBreaker} 透過 Spring AOP proxy
 * 介入呼叫，self-invocation（同 class 內部呼叫）會繞過 proxy 使 CB 失效。
 * 把 gRPC 呼叫獨立到本 class 後，cache client 透過注入此 bean 呼叫即會經 proxy。
 *
 * CB instance 名稱 {@code shipping-fee-service} 與 order-service application.yml 的
 * {@code resilience4j.circuitbreaker.instances.shipping-fee-service} 對齊。
 *
 * 詳見 21 §3.2、§8.3。
 */
@Component
public class ShippingFeeGrpcFetcher {

    private final ShippingGrpcClient shippingGrpcClient;

    public ShippingFeeGrpcFetcher(ShippingGrpcClient shippingGrpcClient) {
        this.shippingGrpcClient = shippingGrpcClient;
    }

    /**
     * 從 shipping-service 拉全量 configs + tiers。
     *
     * 失敗會拋 StatusRuntimeException（或 R4j CB 拒絕時的 CallNotPermittedException），
     * 呼叫方需以 try/catch 包覆並決定是否走 stale fallback。
     */
    @CircuitBreaker(name = "shipping-fee-service")
    public Snapshot fetchAll() {
        GetShippingFeeConfigsResponse response = shippingGrpcClient.getShippingFeeConfigs();
        return buildSnapshot(response);
    }

    private Snapshot buildSnapshot(GetShippingFeeConfigsResponse response) {
        Map<String, ShippingFeeCacheClient.ConfigEntry> newConfigs = new ConcurrentHashMap<>();
        for (ShippingMethodConfigProto c : response.getMethodConfigsList()) {
            newConfigs.put(configKey(c.getShippingMethod(), c.getDeliveryTemp()),
                    new ShippingFeeCacheClient.ConfigEntry(
                            c.getShippingMethod(),
                            c.getDeliveryTemp(),
                            parseIntSafe(c.getMaxDimensionSumCm()),
                            parseIntSafe(c.getMaxSingleSideCm()),
                            parseLongSafe(c.getMaxWeightGrams()),
                            parseIntSafe(c.getEstimatedDays()),
                            parseIntSafe(c.getSettlementDays())));
        }

        List<ShippingFeeCacheClient.TierEntry> allTiers = response.getFeeTiersList().stream()
                .map(t -> new ShippingFeeCacheClient.TierEntry(
                        t.getShippingMethod(),
                        t.getDeliveryTemp(),
                        t.getDeliveryZone(),
                        parseIntSafe(t.getMaxDimensionCm()),
                        parseLongSafe(t.getFee())))
                .sorted(Comparator.comparingInt(ShippingFeeCacheClient.TierEntry::maxDimensionCm))
                .toList();

        Map<String, List<ShippingFeeCacheClient.TierEntry>> newTiers = allTiers.stream()
                .collect(Collectors.groupingBy(
                        t -> tierKey(t.shippingMethod(), t.deliveryTemp(), t.deliveryZone()),
                        ConcurrentHashMap::new,
                        Collectors.toList()));

        return new Snapshot(newConfigs, newTiers);
    }

    private static String configKey(String method, String temp) {
        return method + ":" + temp;
    }

    private static String tierKey(String method, String temp, String zone) {
        return method + ":" + temp + ":" + zone;
    }

    private static int parseIntSafe(String value) {
        if (value == null || value.isEmpty()) return 0;
        try { return (int) Math.round(Double.parseDouble(value)); } catch (NumberFormatException e) { return 0; }
    }

    private static long parseLongSafe(String value) {
        if (value == null || value.isEmpty()) return 0;
        try { return Math.round(Double.parseDouble(value)); } catch (NumberFormatException e) { return 0; }
    }

    public record Snapshot(
            Map<String, ShippingFeeCacheClient.ConfigEntry> configs,
            Map<String, List<ShippingFeeCacheClient.TierEntry>> tiers) {}
}
