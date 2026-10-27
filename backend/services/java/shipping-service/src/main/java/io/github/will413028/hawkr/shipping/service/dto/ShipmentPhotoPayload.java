package io.github.will413028.hawkr.shipping.service.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.List;
import java.util.UUID;

/**
 * 出貨照片更新 MQ 事件酬載。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>{@code shipmentPhotoKeys}（payload key {@code shipment_photo_keys}）值為 objectKey，
 * 顯示 URL 由消費端 / Gateway 組（49 §5.3/§6.3）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ShipmentPhotoPayload(
        String eventId,
        String orderNo,
        List<String> shipmentPhotoKeys
) {
    public ShipmentPhotoPayload(String orderNo, List<String> shipmentPhotoKeys) {
        this(UUID.randomUUID().toString(), orderNo, shipmentPhotoKeys);
    }
}
