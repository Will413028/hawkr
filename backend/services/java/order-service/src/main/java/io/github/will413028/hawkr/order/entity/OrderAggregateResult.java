package io.github.will413028.hawkr.order.entity;

import java.math.BigDecimal;

/**
 * MyBatis-Plus 投影：CRM 訂單聚合結果（issue #1519）
 * 用於 OrderRepository.findOrderAggregates
 */
public class OrderAggregateResult {
    private long storeId;
    private long buyerId;
    private long orderCount;
    private BigDecimal totalNetAmount;

    public long getStoreId() { return storeId; }
    public void setStoreId(long storeId) { this.storeId = storeId; }

    public long getBuyerId() { return buyerId; }
    public void setBuyerId(long buyerId) { this.buyerId = buyerId; }

    public long getOrderCount() { return orderCount; }
    public void setOrderCount(long orderCount) { this.orderCount = orderCount; }

    public BigDecimal getTotalNetAmount() { return totalNetAmount; }
    public void setTotalNetAmount(BigDecimal totalNetAmount) { this.totalNetAmount = totalNetAmount; }
}
