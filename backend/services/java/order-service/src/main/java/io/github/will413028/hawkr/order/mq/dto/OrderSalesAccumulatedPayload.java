package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.List;
import java.util.UUID;

/**
 * order.sales.accumulated MQ 事件 Payload
 *
 * <p>付款成功後發布（{@code PaymentEventHandler.handlePendingPaymentOrder} 的 afterCommit），
 * 由 product-service 消費並逐筆累計 {@code product.sales_count}。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>{@code items} 已由 publisher 端依 {@code productId} group by 加總，consumer 端
 * 無需重複合併，可直接逐筆 UPDATE。
 *
 * <p>詳見 35 設計書（商品列表排序與銷量反正規化）§5。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderSalesAccumulatedPayload(
        String eventId,
        String orderNo,
        String occurredAt,                  // ISO 8601 UTC
        List<ProductSalesItem> items
) {
    public OrderSalesAccumulatedPayload(String orderNo, String occurredAt, List<ProductSalesItem> items) {
        this(UUID.randomUUID().toString(), orderNo, occurredAt, items);
    }

    @JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
    public record ProductSalesItem(
            Long productId,
            Integer quantity
    ) {}
}
