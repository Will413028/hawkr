package io.github.will413028.hawkr.aftersales.mq.payload;

import java.util.UUID;

/**
 * seller.after_sale.updated MQ 事件 Payload（exchange {@code hawkr.seller_workspace}）。
 *
 * <p>賣家工作區 realtime invalidation：aftersales-service case 狀態流轉後發布，
 * 唯一 consumer 為 gateway SSE → 推到賣家 {@code notification:user:{sellerId}} topic（僅 web 賣家中心）。
 * 36 §3.6i / 41 §四之二，[B-207] #1181。
 *
 * <p><b>命名為 camelCase</b>（不加 {@code @JsonNaming}），對齊既有 SSE payload 與 Go 端
 * {@code pkgmq.SellerAfterSaleUpdatedEvent}。{@code sellerId} 於 MQ wire 為 JSON number
 * （gateway 解析 topic）；gateway 於 SSE 輸出時序列化為 string。
 */
public record SellerAfterSaleUpdatedPayload(
        String eventId,
        Long sellerId,
        String orderNo,
        String afterSaleCaseNo,
        String source,
        String status,
        String changedAt
) {
    public SellerAfterSaleUpdatedPayload(
            Long sellerId,
            String orderNo,
            String afterSaleCaseNo,
            String source,
            String status,
            String changedAt
    ) {
        this(UUID.randomUUID().toString(), sellerId, orderNo, afterSaleCaseNo, source, status, changedAt);
    }
}
