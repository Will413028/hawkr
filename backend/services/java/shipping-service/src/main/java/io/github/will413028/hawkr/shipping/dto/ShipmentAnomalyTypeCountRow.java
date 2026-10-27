package io.github.will413028.hawkr.shipping.dto;

/**
 * §5.21 配送異常統計 — 依 anomaly_type 分組筆數。
 */
public class ShipmentAnomalyTypeCountRow {
    private String anomalyType;
    private long cnt;

    public String getAnomalyType() { return anomalyType; }
    public void setAnomalyType(String anomalyType) { this.anomalyType = anomalyType; }

    public long getCnt() { return cnt; }
    public void setCnt(long cnt) { this.cnt = cnt; }
}
