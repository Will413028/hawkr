package io.github.will413028.hawkr.common.mq.payload;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * §5.21 配送異常賣家通知事件 payload。
 *
 * <p>跨服務共用（shipping-service + aftersales-service 皆 publish 至 {@code hawkr.shipping} exchange，
 * routing key {@code shipment.anomaly.detected}），故放置於 common-core。
 *
 * <p>消費端（notification-service）依 severity 決策推送節奏：
 * HIGH 即時 FCM、MEDIUM 早晚摘要、LOW 僅 inbox。
 *
 * <p>冪等去重：notification-service 端以 {@code (orderNo, anomalyType, detectedAt::date)} 作為 dedup key。
 *
 * 設計書：06 §5.21.5、36 §3.3
 *
 * @param eventId         UUID（每次 publish 新生）
 * @param orderNo         訂單編號
 * @param shipmentId      shipments.id（toString）；RETURN_STUCK 無對應 shipment 時為空字串
 * @param sellerAccountId 通知對象賣家 accountId
 * @param buyerAccountId  買家 accountId（供 notification 渲染上下文用，可為 null）
 * @param anomalyType     ShipmentAnomalyType.value
 * @param anomalyTypeLabel ShipmentAnomalyType.label（如「黑貓叫件未完成」）
 * @param severity        ShipmentAnomalySeverity.value（HIGH / MEDIUM / LOW）
 * @param detectedAt      ISO 8601 UTC（DateTimeUtils.formatIso8601）
 * @param storeId         賣家 store 代理鍵原始值（未經解析）；供未來以 store_id 為身分空間的
 *                        收件人查找使用，issue #2707 Phase A。三個 producer（shipping-service
 *                        {@code ShipmentAnomalyEventPublisher}、aftersales-service
 *                        {@code ShipmentLabelConsumer}／{@code ReturnStuckAlertScheduler}）
 *                        皆已有此值在手，純透傳，不新增解析。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ShipmentAnomalyDetectedPayload(
        String eventId,
        String orderNo,
        String shipmentId,
        Long sellerAccountId,
        Long buyerAccountId,
        String anomalyType,
        String anomalyTypeLabel,
        String severity,
        String detectedAt,
        Long storeId
) {
}
