package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * 付款後建單兜底事件 Payload（03 §8.11）。
 *
 * <p>同時用於 {@code order.shipment.create_requested}（請 Shipping 重試建單）
 * 與 {@code order.shipment.creation_failed}（請 AfterSales 取消+退款）。
 * 兩者皆只需 {@code orderNo}：Shipping 以 order_no 冪等建單；AfterSales 的
 * {@code cancelShipmentBySystem} 會以 orderNo 重新查詢訂單取得賣家/買家資訊。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderShipmentBackstopPayload(String eventId, String orderNo) {
    public OrderShipmentBackstopPayload(String orderNo) {
        this(UUID.randomUUID().toString(), orderNo);
    }
}
