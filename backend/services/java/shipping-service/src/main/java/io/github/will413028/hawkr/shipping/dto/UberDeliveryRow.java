package io.github.will413028.hawkr.shipping.dto;

import java.time.LocalDateTime;

/**
 * Uber 配送查詢結果行（66；Stage 8B W6-3／W6-4）。
 *
 * <p>對應 {@link io.github.will413028.hawkr.shipping.mapper.UberDeliveryQueryMapper} 的 SELECT 欄位。
 * 獨立 POJO 而非 Entity：本行跨兩張表（{@code shipments} ＋ {@code uber_deliveries}），
 * 任何一張的 Entity 都裝不下它，而回半填的 Entity 是明令禁止的（10 檔 §11.3）。
 *
 * <p>🔴 <b>{@code status} 與 {@code providerStatus} 是兩個不同的東西</b>：前者是我方正規化狀態
 * （{@code shipments.status}，ShippingStatus value），後者是 Uber 原生 7 值照抄。
 * 合併成一欄會讓「未知 provider 值 fail-closed」這條不變式失去意義（INV-U7）。
 */
public class UberDeliveryRow {

    // ── shipments 側（歸屬、正規化狀態、失敗證據的 SSOT）────────────────
    private Long shipmentId;
    private String shipmentNo;
    private String orderNo;
    private Long storeId;
    private Long buyerId;
    private String shippingMethod;
    private Boolean isReturn;
    private String status;
    private String providerErrorCode;
    private String providerErrorMessage;
    private LocalDateTime providerErrorAt;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    // ── uber_deliveries 側（provider 事實）；建單失敗／未知時整組為 null ──
    private String deliveryId;
    private String quoteId;
    private String providerStatus;
    private Boolean courierImminent;
    private String courierName;
    /** 🔴 遮罩後的值。明碼不落庫（66 §5.1.5／§8.6），因此本欄沒有明碼對應欄位。 */
    private String courierPhoneMasked;
    private String courierLocation;
    private String relatedDeliveries;
    private LocalDateTime etaDropoffAt;
    private String undeliverableReason;
    private String cancelationReason;
    private String outcome;

    /** 可定址鍵：{@code delivery_id} 非空時取它，否則退回 {@code shipment_no}（見 mapper 註解）。 */
    private String deliveryNo;

    /** 已抓回（{@code fetched_at} 非空）的 POD 件數。 */
    private Long podArtifactCount;

    public Long getShipmentId() { return shipmentId; }
    public void setShipmentId(Long shipmentId) { this.shipmentId = shipmentId; }

    public String getShipmentNo() { return shipmentNo; }
    public void setShipmentNo(String shipmentNo) { this.shipmentNo = shipmentNo; }

    public String getOrderNo() { return orderNo; }
    public void setOrderNo(String orderNo) { this.orderNo = orderNo; }

    public Long getStoreId() { return storeId; }
    public void setStoreId(Long storeId) { this.storeId = storeId; }

    public Long getBuyerId() { return buyerId; }
    public void setBuyerId(Long buyerId) { this.buyerId = buyerId; }

    public String getShippingMethod() { return shippingMethod; }
    public void setShippingMethod(String shippingMethod) { this.shippingMethod = shippingMethod; }

    public Boolean getIsReturn() { return isReturn; }
    public void setIsReturn(Boolean isReturn) { this.isReturn = isReturn; }

    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }

    public String getProviderErrorCode() { return providerErrorCode; }
    public void setProviderErrorCode(String providerErrorCode) { this.providerErrorCode = providerErrorCode; }

    public String getProviderErrorMessage() { return providerErrorMessage; }
    public void setProviderErrorMessage(String providerErrorMessage) { this.providerErrorMessage = providerErrorMessage; }

    public LocalDateTime getProviderErrorAt() { return providerErrorAt; }
    public void setProviderErrorAt(LocalDateTime providerErrorAt) { this.providerErrorAt = providerErrorAt; }

    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }

    public LocalDateTime getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(LocalDateTime updatedAt) { this.updatedAt = updatedAt; }

    public String getDeliveryId() { return deliveryId; }
    public void setDeliveryId(String deliveryId) { this.deliveryId = deliveryId; }

    public String getQuoteId() { return quoteId; }
    public void setQuoteId(String quoteId) { this.quoteId = quoteId; }

    public String getProviderStatus() { return providerStatus; }
    public void setProviderStatus(String providerStatus) { this.providerStatus = providerStatus; }

    public Boolean getCourierImminent() { return courierImminent; }
    public void setCourierImminent(Boolean courierImminent) { this.courierImminent = courierImminent; }

    public String getCourierName() { return courierName; }
    public void setCourierName(String courierName) { this.courierName = courierName; }

    public String getCourierPhoneMasked() { return courierPhoneMasked; }
    public void setCourierPhoneMasked(String courierPhoneMasked) { this.courierPhoneMasked = courierPhoneMasked; }

    public String getCourierLocation() { return courierLocation; }
    public void setCourierLocation(String courierLocation) { this.courierLocation = courierLocation; }

    public String getRelatedDeliveries() { return relatedDeliveries; }
    public void setRelatedDeliveries(String relatedDeliveries) { this.relatedDeliveries = relatedDeliveries; }

    public LocalDateTime getEtaDropoffAt() { return etaDropoffAt; }
    public void setEtaDropoffAt(LocalDateTime etaDropoffAt) { this.etaDropoffAt = etaDropoffAt; }

    public String getUndeliverableReason() { return undeliverableReason; }
    public void setUndeliverableReason(String undeliverableReason) { this.undeliverableReason = undeliverableReason; }

    public String getCancelationReason() { return cancelationReason; }
    public void setCancelationReason(String cancelationReason) { this.cancelationReason = cancelationReason; }

    public String getOutcome() { return outcome; }
    public void setOutcome(String outcome) { this.outcome = outcome; }

    public String getDeliveryNo() { return deliveryNo; }
    public void setDeliveryNo(String deliveryNo) { this.deliveryNo = deliveryNo; }

    public Long getPodArtifactCount() { return podArtifactCount; }
    public void setPodArtifactCount(Long podArtifactCount) { this.podArtifactCount = podArtifactCount; }
}
