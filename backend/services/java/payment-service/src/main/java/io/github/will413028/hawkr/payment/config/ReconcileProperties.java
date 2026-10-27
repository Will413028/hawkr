package io.github.will413028.hawkr.payment.config;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

/**
 * L2 補救路徑（PendingPaymentReconcileScheduler）配置
 *
 * 設計參考：docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md §5.7.1
 */
@Component
@ConfigurationProperties(prefix = "hawkr.payment.reconcile.pending")
public class ReconcileProperties {

    /**
     * 是否啟用 reconcile 排程
     */
    private boolean enabled = true;

    /**
     * 排程觸發間隔（毫秒）
     */
    private long fixedDelayMs = 300_000L; // 5 min

    /**
     * 單次 tick 處理筆數上限
     */
    private int batchSize = 50;

    /**
     * Initial delay：交易建立後多少分鐘才允許 reconcile（給 webhook 主路徑優先窗口）
     */
    private int initialDelayMinutes = 3;

    /**
     * Retry interval：同一筆交易最少間隔多少分鐘才再次 reconcile
     */
    private int retryIntervalMinutes = 5;

    /**
     * Grace 視窗：交易過期後仍保留多少分鐘可被 reconcile
     */
    private int graceMinutes = 30;

    /**
     * 單筆呼叫間 sleep（毫秒），對 PayUni 友善
     */
    private long perItemSleepMs = 100L;

    public boolean isEnabled() { return enabled; }
    public void setEnabled(boolean enabled) { this.enabled = enabled; }

    public long getFixedDelayMs() { return fixedDelayMs; }
    public void setFixedDelayMs(long fixedDelayMs) { this.fixedDelayMs = fixedDelayMs; }

    public int getBatchSize() { return batchSize; }
    public void setBatchSize(int batchSize) { this.batchSize = batchSize; }

    public int getInitialDelayMinutes() { return initialDelayMinutes; }
    public void setInitialDelayMinutes(int initialDelayMinutes) { this.initialDelayMinutes = initialDelayMinutes; }

    public int getRetryIntervalMinutes() { return retryIntervalMinutes; }
    public void setRetryIntervalMinutes(int retryIntervalMinutes) { this.retryIntervalMinutes = retryIntervalMinutes; }

    public int getGraceMinutes() { return graceMinutes; }
    public void setGraceMinutes(int graceMinutes) { this.graceMinutes = graceMinutes; }

    public long getPerItemSleepMs() { return perItemSleepMs; }
    public void setPerItemSleepMs(long perItemSleepMs) { this.perItemSleepMs = perItemSleepMs; }
}
