package io.github.will413028.hawkr.order.entity;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * MyBatis-Plus 投影：買家在某賣家的累計消費摘要
 * 用於 OrderRepository.findBuyerMerchantSummary
 */
public class BuyerMerchantSummaryResult {
    private long storeId;
    private BigDecimal totalSpending;
    private LocalDateTime lastOrderTime;
    /** 同一批訂單的 SUM(orders.refund_total_amount)（元）；淨消費 = totalSpending - totalRefund */
    private BigDecimal totalRefund;

    public long getStoreId() { return storeId; }
    public void setStoreId(long storeId) { this.storeId = storeId; }

    public BigDecimal getTotalSpending() { return totalSpending; }
    public void setTotalSpending(BigDecimal totalSpending) { this.totalSpending = totalSpending; }

    public LocalDateTime getLastOrderTime() { return lastOrderTime; }
    public void setLastOrderTime(LocalDateTime lastOrderTime) { this.lastOrderTime = lastOrderTime; }

    public BigDecimal getTotalRefund() { return totalRefund; }
    public void setTotalRefund(BigDecimal totalRefund) { this.totalRefund = totalRefund; }
}
