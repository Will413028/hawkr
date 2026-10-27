package io.github.will413028.hawkr.order.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * order.delivered MQ 事件 Payload
 * 物流送達後發布（通知前端/推播等）。
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p><b>{@code buyerId}</b>：買家的 {@code buyer_id} 業務值（原始值，非登入
 * account_id）。由 consumer 端查詢自己的 order 記錄取得（{@code orders.buyer_id}），
 * 不採信上游 shipping-service {@code shipment.delivered} payload 傳入的同名欄位——
 * 跨服務身份欄位不可信，一律以自己 DB 為準（鏡像 OrderService.completeOrder /
 * OrderOutboxPublisher.enrich 的查自己 order 記錄再解析模式）。
 *
 * <p><b>{@code notifyBuyerAccountId}</b>：由 {@code buyerId} 經
 * BuyerGrpcClient.resolvePrincipals 反解析而得的登入 account_id，供 notification-service
 * 的 {@code order.delivered} 模板路由使用（{@code target_user_field} 指向此欄位；鏡像
 * {@code OrderCompletedPayload.notifyBuyerAccountId}）。解析失敗時帶 0（notification-service
 * 端 skip），不阻斷本次 order.delivered 派送主流程。目前尚無 notification_templates row
 * 消費本事件的 buyer 欄位，此欄位暫為休眠但結構性存在的欄位。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record OrderDeliveredPayload(
        String eventId,
        String orderNo,
        String buyerId,
        String notifyBuyerAccountId,
        String autoCompleteAt
) {
    public OrderDeliveredPayload(
            String orderNo,
            String buyerId,
            String notifyBuyerAccountId,
            String autoCompleteAt
    ) {
        this(UUID.randomUUID().toString(), orderNo, buyerId, notifyBuyerAccountId, autoCompleteAt);
    }
}
