package io.github.will413028.hawkr.order.entity;

/**
 * MyBatis-Plus 投影：單店在一個時間窗內的成立訂單統計（成立訂單數、不重複買家數、營收）
 * 用於 OrderRepository.findStorePeriodOrderStats
 */
public class StorePeriodOrderStatsResult {
    private long qualifyingOrders;
    private long distinctBuyers;
    private long revenue;

    public long getQualifyingOrders() { return qualifyingOrders; }
    public void setQualifyingOrders(long qualifyingOrders) { this.qualifyingOrders = qualifyingOrders; }

    public long getDistinctBuyers() { return distinctBuyers; }
    public void setDistinctBuyers(long distinctBuyers) { this.distinctBuyers = distinctBuyers; }

    public long getRevenue() { return revenue; }
    public void setRevenue(long revenue) { this.revenue = revenue; }
}
