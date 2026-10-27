package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * order.receipt.confirmed MQ 事件 Payload
 * 確認收貨後發布，通知 Shipping Service 標記 receipt_confirmed。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderReceiptConfirmedPayload(
        String eventId,
        String orderNo,
        String confirmType
) {
    public OrderReceiptConfirmedPayload(String orderNo, String confirmType) {
        this(UUID.randomUUID().toString(), orderNo, confirmType);
    }
}
