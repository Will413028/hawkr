package io.github.will413028.hawkr.shipping.service.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * 超商列印 Webhook 回填出貨便代碼 MQ 事件酬載。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>單一 routing key `shipment.label.updated` 同時綁定 Order Service 與 AfterSales Service
 * 兩個 queue，消費者依 {@code isReturn} 分流：
 * <ul>
 *   <li>{@code isReturn=false} → Order Service 快照至 {@code orders}</li>
 *   <li>{@code isReturn=true}  → AfterSales Service 快照至 {@code aftersale_cases}</li>
 * </ul>
 *
 * @param eventId       UUID v4
 * @param orderNo       關聯訂單編號
 * @param shipmentNo    物流單號（便於 debug，正向為 S{9位}、退貨為 RS{9位}）
 * @param partnerId     出貨便母代碼（3 碼）
 * @param odno          出貨便單編號（8 碼）
 * @param validationNo  C2C 驗證碼（4 碼）
 * @param isReturn      是否為退貨單（直接取自 {@code shipments.is_return}）
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record LabelUpdatedPayload(
        String eventId,
        String orderNo,
        String shipmentNo,
        String partnerId,
        String odno,
        String validationNo,
        boolean isReturn
) {
    public LabelUpdatedPayload(
            String orderNo,
            String shipmentNo,
            String partnerId,
            String odno,
            String validationNo,
            boolean isReturn
    ) {
        this(UUID.randomUUID().toString(), orderNo, shipmentNo, partnerId, odno, validationNo, isReturn);
    }
}
