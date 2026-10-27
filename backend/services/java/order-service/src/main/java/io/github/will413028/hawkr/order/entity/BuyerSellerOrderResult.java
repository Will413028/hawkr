package io.github.will413028.hawkr.order.entity;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * MyBatis-Plus 投影：買家在某賣家的單筆消費紀錄
 * 用於 OrderRepository.findBuyerOrdersForSeller
 */
public class BuyerSellerOrderResult {
    private String orderNo;
    private LocalDateTime createdAt;
    private BigDecimal totalActualAmount;
    private BigDecimal discountAmount;
    private String shippingAddress;
    private String recipientPhone;
    private Long shippingFee;
    /** orders.refund_total_amount：本筆已完成退款金額（元）；0 = 未退款 */
    private Long refundTotalAmount;

    public String getOrderNo() { return orderNo; }
    public void setOrderNo(String orderNo) { this.orderNo = orderNo; }

    public LocalDateTime getCreatedAt() { return createdAt; }
    public void setCreatedAt(LocalDateTime createdAt) { this.createdAt = createdAt; }

    public BigDecimal getTotalActualAmount() { return totalActualAmount; }
    public void setTotalActualAmount(BigDecimal totalActualAmount) { this.totalActualAmount = totalActualAmount; }

    public BigDecimal getDiscountAmount() { return discountAmount; }
    public void setDiscountAmount(BigDecimal discountAmount) { this.discountAmount = discountAmount; }

    public String getShippingAddress() { return shippingAddress; }
    public void setShippingAddress(String shippingAddress) { this.shippingAddress = shippingAddress; }

    public String getRecipientPhone() { return recipientPhone; }
    public void setRecipientPhone(String recipientPhone) { this.recipientPhone = recipientPhone; }

    public Long getShippingFee() { return shippingFee; }
    public void setShippingFee(Long shippingFee) { this.shippingFee = shippingFee; }

    public Long getRefundTotalAmount() { return refundTotalAmount; }
    public void setRefundTotalAmount(Long refundTotalAmount) { this.refundTotalAmount = refundTotalAmount; }
}
