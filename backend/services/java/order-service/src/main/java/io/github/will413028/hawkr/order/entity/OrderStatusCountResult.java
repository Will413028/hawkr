package io.github.will413028.hawkr.order.entity;

/**
 * MyBatis-Plus 投影：買家各 OrderStatus 訂單筆數
 * 用於 OrderRepository.countMyOrdersGroupByStatus（買家訂單 tab 筆數 rollup，SSOT 見 03 §2.1）
 */
public class OrderStatusCountResult {
    private String status;
    private long count;

    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }

    public long getCount() { return count; }
    public void setCount(long count) { this.count = count; }
}
