package io.github.will413028.hawkr.order.util;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * 物流推進守門規則（單調前進 + 異常恢復）——order.status 物流段推進的合法前置狀態計算。
 *
 * <p>SSOT：03 號設計書「物流推進守門單調前進與異常恢復」。CodeMeta enum 禁含業務邏輯（§8.5），
 * 故本 helper 集中「什麼狀態能推進到 X」的判定，取代 ShippingEventConsumer 各分支寫死的 allowed-from 清單。
 *
 * <p>物流主線序位（rank）：PAID(10)=0 → PENDING_SHIPMENT(20)=1 → IN_TRANSIT(30)=2 →
 * WAITING_PICKUP(32)=3 → DELIVERED(40)=4；SHIPPING_ABNORMAL(35) 為異常旁支。
 * 非物流態（0/50/60/70/80）不在序位表，物流事件對其一律 no-op（終端態自動保護）。
 */
public final class OrderShippingProgress {

    private OrderShippingProgress() {
    }

    /** 物流主線序位：value → rank（僅含物流推進主線態；異常/終端/起點態不在此表）。 */
    private static final Map<String, Integer> RANK = Map.of(
            OrderStatus.PAID.value(), 0,
            OrderStatus.PENDING_SHIPMENT.value(), 1,
            OrderStatus.IN_TRANSIT.value(), 2,
            OrderStatus.WAITING_PICKUP.value(), 3,
            OrderStatus.DELIVERED.value(), 4);

    private static final int DELIVERED_RANK = RANK.get(OrderStatus.DELIVERED.value());
    private static final int IN_TRANSIT_RANK = RANK.get(OrderStatus.IN_TRANSIT.value());
    private static final String ABNORMAL = OrderStatus.SHIPPING_ABNORMAL.value();

    /**
     * 推進到正常主線態 {@code targetValue} 的合法前置狀態集合（CAS allowed-from）。
     * = 所有序位早於 target 的主線態；若 target 為異常可恢復目標（rank ≥ IN_TRANSIT，即 30/32/40）
     * 額外納入 SHIPPING_ABNORMAL（異常恢復）。待出貨(20) 不納入異常，杜絕 35→20 倒退。
     *
     * @param targetValue OrderStatus.value()，須為物流主線態（20/30/32/40）
     * @return 合法前置 value 清單；target 非主線態時回空清單（防呆）
     */
    public static List<String> allowedFromFor(String targetValue) {
        Integer targetRank = RANK.get(targetValue);
        if (targetRank == null) {
            return List.of();
        }
        List<String> allowed = new ArrayList<>();
        for (Map.Entry<String, Integer> entry : RANK.entrySet()) {
            if (entry.getValue() < targetRank) {
                allowed.add(entry.getKey());
            }
        }
        if (targetRank >= IN_TRANSIT_RANK) {
            allowed.add(ABNORMAL);
        }
        return allowed;
    }

    /**
     * 推進到異常態 SHIPPING_ABNORMAL(35) 的合法前置狀態集合。
     * = 所有序位早於 DELIVERED 的在途主線態（PAID/PENDING_SHIPMENT/IN_TRANSIT/WAITING_PICKUP）；
     * 不含 DELIVERED(40)（已送達不因遲到異常事件拖回）、不含 SHIPPING_ABNORMAL 自身。
     */
    public static List<String> allowedFromForAbnormal() {
        List<String> allowed = new ArrayList<>();
        for (Map.Entry<String, Integer> entry : RANK.entrySet()) {
            if (entry.getValue() < DELIVERED_RANK) {
                allowed.add(entry.getKey());
            }
        }
        return allowed;
    }
}
