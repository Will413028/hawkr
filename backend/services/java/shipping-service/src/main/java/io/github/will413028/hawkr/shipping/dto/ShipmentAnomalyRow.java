package io.github.will413028.hawkr.shipping.dto;

import java.time.LocalDateTime;

/**
 * §5.21 配送異常 Dashboard 查詢結果行。
 *
 * <p>對應 {@link io.github.will413028.hawkr.shipping.mapper.ShipmentAnomalyQueryMapper} 推導 columns。
 * 獨立 POJO 避免觸發 JsonbMap TypeHandler。
 */
public class ShipmentAnomalyRow {
    private Long shipmentId;
    private String orderNo;
    private Long storeId;
    private String shipmentStatus;
    private String shippingMethod;
    private String anomalyType;
    private String severity;
    private LocalDateTime detectedAt;
    private String ackAction;
    private LocalDateTime actedAt;
    // Admin 端揭露：賣家處置（seller_ack）；賣家 dashboard 查詢不填此兩欄
    private String sellerAckAction;
    private LocalDateTime sellerActedAt;

    public Long getShipmentId() { return shipmentId; }
    public void setShipmentId(Long shipmentId) { this.shipmentId = shipmentId; }

    public String getOrderNo() { return orderNo; }
    public void setOrderNo(String orderNo) { this.orderNo = orderNo; }

    public Long getStoreId() { return storeId; }
    public void setStoreId(Long storeId) { this.storeId = storeId; }

    public String getShipmentStatus() { return shipmentStatus; }
    public void setShipmentStatus(String shipmentStatus) { this.shipmentStatus = shipmentStatus; }

    public String getShippingMethod() { return shippingMethod; }
    public void setShippingMethod(String shippingMethod) { this.shippingMethod = shippingMethod; }

    public String getAnomalyType() { return anomalyType; }
    public void setAnomalyType(String anomalyType) { this.anomalyType = anomalyType; }

    public String getSeverity() { return severity; }
    public void setSeverity(String severity) { this.severity = severity; }

    public LocalDateTime getDetectedAt() { return detectedAt; }
    public void setDetectedAt(LocalDateTime detectedAt) { this.detectedAt = detectedAt; }

    public String getAckAction() { return ackAction; }
    public void setAckAction(String ackAction) { this.ackAction = ackAction; }

    public LocalDateTime getActedAt() { return actedAt; }
    public void setActedAt(LocalDateTime actedAt) { this.actedAt = actedAt; }

    public String getSellerAckAction() { return sellerAckAction; }
    public void setSellerAckAction(String sellerAckAction) { this.sellerAckAction = sellerAckAction; }

    public LocalDateTime getSellerActedAt() { return sellerActedAt; }
    public void setSellerActedAt(LocalDateTime sellerActedAt) { this.sellerActedAt = sellerActedAt; }
}
