package io.github.will413028.hawkr.order.entity;

import java.math.BigDecimal;

/**
 * MyBatis-Plus 投影：賣家營收排名結果
 * 用於 OrderRepository.findTopSellersByRevenue
 */
public class SellerRevenueResult {
    private long storeId;
    private BigDecimal totalRevenue;
    private int orderCount;

    public long getStoreId() { return storeId; }
    public void setStoreId(long storeId) { this.storeId = storeId; }

    public BigDecimal getTotalRevenue() { return totalRevenue; }
    public void setTotalRevenue(BigDecimal totalRevenue) { this.totalRevenue = totalRevenue; }

    public int getOrderCount() { return orderCount; }
    public void setOrderCount(int orderCount) { this.orderCount = orderCount; }
}
