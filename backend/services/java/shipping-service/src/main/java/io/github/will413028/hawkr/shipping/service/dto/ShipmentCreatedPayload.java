package io.github.will413028.hawkr.shipping.service.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * 物流單建立 MQ 事件酬載（{@code shipment.created}）
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>Order Service 消費後寫入 {@code orders} 表（含 §5.9 賣家處理期限快照）。
 * 與 ShippingEventPayload 拆開，因為 created 事件特有 {@code processingDeadline} 欄位
 * 是給 Order 端 {@code orders.processing_deadline} 持久化用，其他事件不需要此資訊。
 *
 * <p><b>2026-05-31 補欄</b>（賣家「新訂單」推播由 {@code order.paid} 改綁
 * {@code shipment.created}）：
 * <ul>
 *   <li>{@code sellerAccountId} / {@code buyerId} / {@code totalActualAmount}：
 *       由 {@code publishShipmentCreatedEvent} 呼叫端的 {@code OrderShippingInfo}
 *       直接取得。</li>
 *   <li>{@code buyerNickname} / {@code buyerAvatarKey}：由 shipping-service 在 publish 前
 *       透過 {@link io.github.will413028.hawkr.shipping.client.BuyerGrpcClient#batchGetBuyerDisplayByBuyerIds}
 *       best-effort enrichment 取得（與 order-service {@code OrderOutboxPublisher.enrich}
 *       同模式）。失敗時兩欄皆為 {@code null}，前端 {@code _personalize} helper
 *       會 fallback 至通用「買家」字樣（issue #686 個人化字樣保留）。
 *       **值為 display name**：買家已註銷時是墓碑字串而非真實暱稱（#2830）；
 *       出貨在訂單成立後不久，實務上打不到已註銷買家。
 *       {@code buyerAvatarKey} 為 objectKey（49 §6.3/S3：MQ payload 只帶 key、名 *Key；
 *       消費端 notification worker 組 URL，S5）。</li>
 * </ul>
 *
 * <p><b>2026-08-05 正名</b>（issue #2174）：本 payload 的買家欄位原名
 * {@code buyerAccountId}（wire key {@code buyer_account_id}），但裝的是
 * {@code orders.buyer_id} 裸 PK、從未經過解析，名字與內容不符。改名為
 * {@code buyerId} 以對齊 sibling {@link ShippingEventPayload} 已定下的雙欄位形狀
 * （{@code buyerId} = 裸 PK、{@code notifyBuyerAccountId} = 已解析登入 principal）。
 * {@code sellerAccountId} 則**確實**是已解析值（{@code ShippingService.resolveSellerAccountId}），
 * 名字正確，不動。本事件目前只有 {@code notification_templates} id=2
 * （{@code target_user_type='seller'}、{@code target_user_field='seller_account_id'}）
 * 與 order-service {@code ShippingEventConsumer.onShipmentCreated}（只讀 order_no /
 * ship_trade_no / label_key / processing_deadline）兩個消費端，皆不讀買家欄位——
 * 未來要接買家向推播時，請比照 {@code ShippingService.resolveNotifyBuyerAccountId}
 * **新增** 一個 {@code notifyBuyerAccountId} 欄位，不要直接拿 {@code buyerId} 當推播對象。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ShipmentCreatedPayload(
        String eventId,
        String orderNo,
        String shipmentNo,
        String shipTradeNo,
        String shippingMethod,
        String status,
        String labelUrl,
        String processingDeadline,
        String sellerAccountId,
        String buyerId,
        String totalActualAmount,
        String buyerNickname,
        String buyerAvatarKey,
        String storeId
) {
    public ShipmentCreatedPayload(
            String orderNo,
            String shipmentNo,
            String shipTradeNo,
            String shippingMethod,
            String status,
            String labelUrl,
            String processingDeadline,
            String sellerAccountId,
            String buyerId,
            String totalActualAmount,
            String buyerNickname,
            String buyerAvatar,
            String storeId
    ) {
        this(UUID.randomUUID().toString(), orderNo, shipmentNo, shipTradeNo, shippingMethod,
                status, labelUrl, processingDeadline, sellerAccountId, buyerId,
                totalActualAmount, buyerNickname, buyerAvatar, storeId);
    }
}
