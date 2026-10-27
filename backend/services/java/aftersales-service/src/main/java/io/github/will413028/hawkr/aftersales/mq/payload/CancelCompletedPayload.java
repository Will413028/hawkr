package io.github.will413028.hawkr.aftersales.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * aftersales.cancel.completed MQ 事件 Payload。
 * 賣家取消出貨完成（未付款訂單直接完成）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record CancelCompletedPayload(
        String eventId,
        String orderNo,
        String cancelId,
        String occurredAt
) {
    public CancelCompletedPayload(String orderNo, String cancelId, String occurredAt) {
        this(UUID.randomUUID().toString(), orderNo, cancelId, occurredAt);
    }
}
