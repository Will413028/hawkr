package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.shipping.service.dto.LogisticsCostIncurredPayload;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.service.dto.ShippingEventPayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

@Component
public class ShippingEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(ShippingEventPublisher.class);
    private final ObsRabbitPublisher rabbitPublisher;
    private final ObjectMapper objectMapper;

    public ShippingEventPublisher(ObsRabbitPublisher rabbitPublisher, ObjectMapper objectMapper) {
        this.rabbitPublisher = rabbitPublisher;
        this.objectMapper = objectMapper;
    }

    /**
     * 發布物流單建立事件
     *
     * <p>Order Service 消費後寫入 {@code orders} 表，包含 {@code processingDeadline}
     * 快照給前端展示倒數（§5.9 / §5.20）。
     */
    public void publishShipmentCreated(io.github.will413028.hawkr.shipping.service.dto.ShipmentCreatedPayload payload) {
        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_CREATED, messageBytes,
                    MqConstants.EVENT_TYPE_SHIPMENT_CREATED);
            log.info("ShippingEventPublisher.publishShipmentCreated.success",
                    "物流單建立事件發布成功", "orderNo", payload.orderNo());
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishShipmentCreated.error",
                    "物流單建立事件發布失敗: " + e.getMessage());
            throw new ShippingBusinessException(GrpcResponseCode.INTERNAL_ERROR, "MQ 事件發布失敗");
        }
    }

    /**
     * 發布物流狀態變更事件
     */
    public void publishShipmentStatusChanged(ShippingEventPayload payload) {
        publish(MqConstants.ROUTING_KEY_SHIPMENT_STATUS_CHANGED,
                MqConstants.EVENT_TYPE_SHIPMENT_STATUS_CHANGED, payload);
    }

    /**
     * 發布退貨物流狀態變更事件
     */
    public void publishReturnShipmentStatusChanged(ShippingEventPayload payload) {
        publish(MqConstants.ROUTING_KEY_RETURN_SHIPMENT_STATUS_CHANGED,
                MqConstants.EVENT_TYPE_RETURN_SHIPMENT_STATUS_CHANGED, payload);
    }

    /**
     * 發布物流已送達事件
     */
    public void publishShipmentDelivered(ShippingEventPayload payload) {
        publish(MqConstants.ROUTING_KEY_SHIPMENT_DELIVERED,
                MqConstants.EVENT_TYPE_SHIPMENT_DELIVERED, payload);
    }

    /**
     * 發布出貨照片更新事件
     */
    public void publishShipmentPhotoUpdated(io.github.will413028.hawkr.shipping.service.dto.ShipmentPhotoPayload payload) {
        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_PHOTO_UPDATED, messageBytes,
                    MqConstants.EVENT_TYPE_SHIPMENT_PHOTO_UPDATED);
            log.info("ShippingEventPublisher.publishShipmentPhotoUpdated.success",
                    "出貨照片更新事件發布成功", "orderNo", payload.orderNo());
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishShipmentPhotoUpdated.error",
                    "出貨照片更新事件發布失敗: " + e.getMessage());
        }
    }

    /**
     * 發布超商出貨便代碼回填事件
     */
    public void publishLabelUpdated(io.github.will413028.hawkr.shipping.service.dto.LabelUpdatedPayload payload) {
        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_LABEL_UPDATED, messageBytes,
                    MqConstants.EVENT_TYPE_SHIPMENT_LABEL_UPDATED);
            log.info("ShippingEventPublisher.publishLabelUpdated.success",
                    "出貨便代碼回填事件發布成功", "orderNo", payload.orderNo());
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishLabelUpdated.error",
                    "出貨便代碼回填事件發布失敗: " + e.getMessage());
        }
    }

    /**
     * §5.9 發布賣家完成處理事件（CVS 取號 / 宅配叫件成功時觸發）
     *
     * <p>Order Service 消費後清空 {@code orders.processing_deadline}，前端倒數立即消失。
     */
    public void publishProcessingCleared(String orderNo) {
        try {
            String json = objectMapper.writeValueAsString(java.util.Map.of("orderNo", orderNo));
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_PROCESSING_CLEARED,
                    json.getBytes(java.nio.charset.StandardCharsets.UTF_8),
                    MqConstants.EVENT_TYPE_SHIPMENT_PROCESSING_CLEARED);
            log.info("ShippingEventPublisher.publishProcessingCleared.success",
                    "賣家完成處理事件發布成功", "orderNo", orderNo);
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishProcessingCleared.error",
                    "賣家完成處理事件發布失敗, orderNo=" + orderNo, e);
        }
    }

    /**
     * §5.9 發布賣家處理逾期事件。
     *
     * <p>{@code sellerAccountId} 是已解析的賣家登入 account_id
     * （{@code ProcessingDeadlineScheduler.resolveSellerAccountIds}，resolve 失敗時
     * fail-open 落回原始 store_id）；{@code buyerId} 是 {@code shipments.buyer_id}
     * 裸 PK，**未經解析**——唯一消費端 aftersales-service
     * {@code ProcessingTimeoutConsumer} 只讀 {@code orderNo}，不用它定位任何人。
     * 2026-08-05 (issue #2174) 由 {@code buyerAccountId} 正名而來。
     */
    public void publishProcessingTimeout(String orderNo, Long sellerAccountId, Long buyerId) {
        try {
            String json = objectMapper.writeValueAsString(java.util.Map.of(
                    "orderNo", orderNo,
                    "sellerAccountId", String.valueOf(sellerAccountId),
                    "buyerId", String.valueOf(buyerId)
            ));
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_SHIPMENT_PROCESSING_TIMEOUT,
                    json.getBytes(java.nio.charset.StandardCharsets.UTF_8),
                    MqConstants.EVENT_TYPE_SHIPMENT_PROCESSING_TIMEOUT);
            log.info("ShippingEventPublisher.publishProcessingTimeout.success",
                    "賣家逾期事件發布成功", "orderNo", orderNo);
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishProcessingTimeout.error",
                    "賣家逾期事件發布失敗, orderNo=" + orderNo, e);
        }
    }

    /**
     * 發布 {@code logistics.cost.incurred}——provider 這條 leg 已產生成本（70 §7.1）。
     *
     * <p>🔴 <b>fail-loud，不吞例外。</b> 本服務沒有 outbox（實查：全服務無 outbox 表），
     * publisher 是交易外的 {@code ObsRabbitPublisher} ⇒ 發送失敗時這一筆成本事實<b>會丟</b>，
     * 而不是延後送達。吞掉例外等於讓「向賣家收不到這筆錢」變成一個沒有訊號的事件。
     *
     * <p>⚠️ <b>最終兜底不在這條路徑上</b>：provider 月結帳單匯入（70 §6.1）會帶來該期間的
     * <b>全部</b>成本事實並走 true-up（§6.2）⇒ 事件丟失是<b>可偵測、可補回</b>的。
     * 把 outbox 加進 shipping-service 是獨立的工程，不在本波射程內。
     */
    public void publishLogisticsCostIncurred(LogisticsCostIncurredPayload payload) {
        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING,
                    MqConstants.ROUTING_KEY_LOGISTICS_COST_INCURRED, messageBytes,
                    MqConstants.EVENT_TYPE_LOGISTICS_COST_INCURRED);
            log.info("ShippingEventPublisher.publishLogisticsCostIncurred.success",
                    "provider 成本事實發布成功: providerDeliveryRef=" + payload.providerDeliveryRef(),
                    "orderNo", payload.orderNo());
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publishLogisticsCostIncurred.error",
                    "provider 成本事實發布失敗（本服務無 outbox ⇒ 這一筆會丟，靠月結對帳補回）: "
                            + e.getMessage());
            throw new ShippingBusinessException(GrpcResponseCode.INTERNAL_ERROR, "MQ 事件發布失敗");
        }
    }

    private void publish(String routingKey, String eventType, ShippingEventPayload payload) {
        try {
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);
            rabbitPublisher.publish(MqConstants.EXCHANGE_SHIPPING, routingKey, messageBytes, eventType);
            log.info("ShippingEventPublisher.publish.success",
                    "事件發布成功，eventType=" + eventType, "orderNo", payload.orderNo());
        } catch (Exception e) {
            log.error("ShippingEventPublisher.publish.error",
                    "事件發布失敗: " + e.getMessage());
            throw new ShippingBusinessException(GrpcResponseCode.INTERNAL_ERROR, "MQ 事件發布失敗");
        }
    }
}
