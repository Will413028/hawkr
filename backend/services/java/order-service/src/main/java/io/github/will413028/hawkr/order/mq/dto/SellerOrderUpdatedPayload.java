package io.github.will413028.hawkr.order.mq.dto;

import java.util.UUID;

/**
 * seller.order.updated MQ 事件 Payload（exchange {@code hawkr.seller_workspace}）。
 *
 * <p>賣家工作區 realtime invalidation 事件：order-service 所有 order 狀態 commit 後發布，
 * 唯一 consumer 為 gateway SSE → 推到賣家 {@code notification:user:{sellerId}} topic（僅 web 賣家中心）。
 * 36 §3.6i / 41 §四之二，[B-207] #1181。
 *
 * <p><b>命名為 camelCase</b>（不加 {@code @JsonNaming}），對齊既有 SSE payload 與 Go 端
 * {@code pkgmq.SellerOrderUpdatedEvent}（standalone，無 BaseEvent embedding，比照 ProductRestockPayload）。
 * {@code sellerId} 於 MQ wire 為 JSON number（gateway 解析 topic）；gateway 於 SSE 輸出時序列化為 string。
 *
 * <p><b>{@code sellerId} 的身分空間是 store_id surrogate，不是登入 account_id。</b>
 * 訂閱端 {@code notification_inbox.go#resolveNotificationUserID} 自 2026-08-30 起對賣家
 * 回傳 current store_id，發布端必須用同一個空間，否則 {@code store_id != account_id} 的賣家
 * 會落在兩個永不相等的 topic 上（欄名沿用 {@code sellerId} 是 re-key ② 命名收斂的過渡狀態，
 * 見 CLAUDE.md §6）。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4，供前端去重；{@code changedAt} 為 ISO 8601 UTC。
 * 狀態欄位（previous/status/shipment）為 order/shipment status code 字串（OrderStatus.value()），可為 null。
 */
public record SellerOrderUpdatedPayload(
        String eventId,
        Long sellerId,
        String orderNo,
        String source,
        String previousStatus,
        String status,
        String shipmentStatus,
        String changedAt
) {
    public SellerOrderUpdatedPayload(
            Long sellerId,
            String orderNo,
            String source,
            String previousStatus,
            String status,
            String shipmentStatus,
            String changedAt
    ) {
        this(UUID.randomUUID().toString(), sellerId, orderNo, source,
                previousStatus, status, shipmentStatus, changedAt);
    }
}
