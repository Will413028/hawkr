package io.github.will413028.hawkr.shipping.service.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

import java.util.UUID;

/**
 * 物流 MQ 事件酬載（發布至 Order Service / Notification Service / AfterSales Service 等下游）
 *
 * <p>{@code eventId} 由 publisher 端自動產生 UUID v4
 * （36 §7.4 Idempotency Conformance Contract）。
 *
 * <p>{@code sellerName} / {@code sellerAvatarKey} 為 notification-service buyer
 * 推播文案上下文，由 publisher 端用 SellerGrpcClient 取得後填入。
 * sellerAvatarKey 為 objectKey（49 §6.3/S3：MQ payload 只帶 key、名 *Key；
 * 消費端 notification worker 組 URL，S5），未設頭像則為 null。
 *
 * <p>{@code buyerId} 是持久化在 ShipmentEntity 上的買家 buyer_id（PK，出貨建立時
 * 從 order 複製、write-once）。
 *
 * <p>{@code notifyBuyerAccountId} 才是 notification-service 用 target_user_field 反查
 * 買家推播收件人（device_tokens）的依據；由 publisher 端透過 BuyerGrpcClient.resolvePrincipals
 * 把 buyerId(PK) 解析為登入 principal 後填入（鏡像 aftersales-service
 * {@code AfterSalesEventPublisherImpl.resolveNotifyBuyerAccountId}）。
 * 解析失敗或無買家上下文時為 0L（安全的「查無此人」路由值，notification-service 會跳過投遞，
 * 不可為 null——null 會讓 target_user_field lookup 直接炸掉）。
 *
 * <p>{@code sellerAccountId} 供 notification-service {@code shipment.delivered.seller}
 * template 以 target_user_field 反查賣家推播收件人（gateway 通道）。
 * sellerAccountId 必須是解析後的賣家登入 principal（非 store_id 原始值）——
 * 賣家關店重開後新 store_id 不等於登入身分，直接透傳 store_id 會誤投或投不到推播；
 * 呼叫端須在建構本 payload 前透過 SellerGrpcClient.batchGetSellerMeta 解析
 * store_id（見 ShippingService.resolveSellerAccountId）。
 * 非 delivered 路徑（status.changed 等）不需定位賣家時可為 null。
 *
 * <p>{@code providerStatus} / {@code providerMessage} 為 PayUni 原始狀態碼與解密訊息原文
 * 來源是 {@code shipments.provider_status} / {@code shipments.provider_message}
 * AfterSales Service 於 RETURN_FAILED(28) 狀態時持久化至 aftersale_cases.return_provider_*
 * 並回傳前端顯示。正向訂單 consumer 可消費也可暫不消費。
 *
 * <p>本 record 同時供 {@code publishShipmentDelivered}（PICKED_UP）與
 * {@code publishShipmentShipped}（IN_TRANSIT）使用，routing key 由 publisher
 * 方法決定，payload 結構共用。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ShippingEventPayload(
        String eventId,
        String orderNo,
        String shipmentNo,
        String shippingMethod,
        String status,
        String statusCode,
        String statusDesc,
        String labelKey,
        String shipTradeNo,
        String labelExpireAt,
        boolean isReturn,
        String sellerName,
        String sellerAvatarKey,
        Long buyerId,
        Long notifyBuyerAccountId,
        Long sellerAccountId,
        String providerStatus,
        String providerMessage,
        Long storeId
) {
    /**
     * 18-arg constructor（不帶 eventId）— 自動產生 UUID v4。
     */
    public ShippingEventPayload(String orderNo, String shipmentNo, String shippingMethod,
                                 String status, String statusCode, String statusDesc,
                                 String labelKey, String shipTradeNo, String labelExpireAt,
                                 boolean isReturn, String sellerName, String sellerAvatar,
                                 Long buyerId, Long notifyBuyerAccountId, Long sellerAccountId,
                                 String providerStatus, String providerMessage, Long storeId) {
        this(UUID.randomUUID().toString(), orderNo, shipmentNo, shippingMethod,
                status, statusCode, statusDesc, labelKey, shipTradeNo, labelExpireAt,
                isReturn, sellerName, sellerAvatar, buyerId, notifyBuyerAccountId, sellerAccountId,
                providerStatus, providerMessage, storeId);
    }

    /**
     * 相容舊呼叫端（不帶 isReturn / sellerName / sellerAvatar / buyerId / notifyBuyerAccountId
     * / sellerAccountId / provider*）。notifyBuyerAccountId 固定 0L——本 constructor 未帶買家
     * 上下文，沒有 buyer_id 可解析。
     */
    public ShippingEventPayload(String orderNo, String shipmentNo, String shippingMethod,
                                 String status, String statusCode, String statusDesc,
                                 String labelKey, String shipTradeNo, String labelExpireAt) {
        this(orderNo, shipmentNo, shippingMethod, status, statusCode, statusDesc,
                labelKey, shipTradeNo, labelExpireAt, false, null, null, null, 0L, null, null, null, null);
    }

    /**
     * 相容含 isReturn 但不帶 seller 上下文 / buyerId / notifyBuyerAccountId / sellerAccountId
     * / provider* 的呼叫端。notifyBuyerAccountId 固定 0L（同上，無買家上下文）。
     */
    public ShippingEventPayload(String orderNo, String shipmentNo, String shippingMethod,
                                 String status, String statusCode, String statusDesc,
                                 String labelKey, String shipTradeNo, String labelExpireAt,
                                 boolean isReturn) {
        this(orderNo, shipmentNo, shippingMethod, status, statusCode, statusDesc,
                labelKey, shipTradeNo, labelExpireAt, isReturn, null, null, null, 0L, null, null, null, null);
    }

    /**
     * 相容 14-arg 呼叫端（不帶 sellerAccountId / provider*；用於正向出貨等 happy path）。
     * notifyBuyerAccountId 由呼叫端明確傳入——這些路徑從未真正推播到買家（狀態或
     * payload_filter 不匹配買家推播 template），呼叫端固定傳 0L，不為此額外打
     * buyer-service RPC；唯一需要真正解析的路徑是 ShippingService.updateStatus
     * 中央分派點（見該處呼叫）。
     */
    public ShippingEventPayload(String orderNo, String shipmentNo, String shippingMethod,
                                 String status, String statusCode, String statusDesc,
                                 String labelKey, String shipTradeNo, String labelExpireAt,
                                 boolean isReturn, String sellerName, String sellerAvatar,
                                 Long buyerId, Long notifyBuyerAccountId) {
        this(orderNo, shipmentNo, shippingMethod, status, statusCode, statusDesc,
                labelKey, shipTradeNo, labelExpireAt, isReturn,
                sellerName, sellerAvatar, buyerId, notifyBuyerAccountId, null, null, null, null);
    }
}
