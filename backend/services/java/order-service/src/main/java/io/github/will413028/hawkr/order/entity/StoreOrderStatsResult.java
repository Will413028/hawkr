package io.github.will413028.hawkr.order.entity;

/**
 * MyBatis-Plus 投影：單店近期訂單統計（成交買家數、成交訂單數、賣家取消訂單數）
 * 用於 OrderRepository.findStoreOrderStats
 */
public class StoreOrderStatsResult {
    private long storeId;
    private int distinctBuyers;
    private int qualifyingOrders;
    private int sellerCancelledOrders;

    public long getStoreId() { return storeId; }
    public void setStoreId(long storeId) { this.storeId = storeId; }

    public int getDistinctBuyers() { return distinctBuyers; }
    public void setDistinctBuyers(int distinctBuyers) { this.distinctBuyers = distinctBuyers; }

    public int getQualifyingOrders() { return qualifyingOrders; }
    public void setQualifyingOrders(int qualifyingOrders) { this.qualifyingOrders = qualifyingOrders; }

    public int getSellerCancelledOrders() { return sellerCancelledOrders; }
    public void setSellerCancelledOrders(int sellerCancelledOrders) { this.sellerCancelledOrders = sellerCancelledOrders; }
}
