package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;

/**
 * 任務表清理排程（03 §8.4）
 *
 * 定期清理已完成的 stock_tx_records、auto_refund_tasks、coupon_consumption_tasks、
 * cart_cleanup_retry_tasks、grant_payment_tasks（DONE / MANUAL_INTERVENTION / PROVIDER_AUTO），
 * 避免任務表無限膨脹。
 */
@Component
public class TaskCleanupJob {

    private static final ObsLogger log = ObsLogger.forClass(TaskCleanupJob.class);

    private final StockTxRecordRepository stockTxRecordRepository;
    private final AutoRefundTaskRepository autoRefundTaskRepository;
    private final CouponConsumptionTaskRepository couponConsumptionTaskRepository;
    private final CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository;
    private final GrantPaymentTaskRepository grantPaymentTaskRepository;

    @Value("${order.cleanup.enabled:true}")
    private boolean enabled;

    @Value("${order.cleanup.task-retention-days:30}")
    private int taskRetentionDays;

    public TaskCleanupJob(
            StockTxRecordRepository stockTxRecordRepository,
            AutoRefundTaskRepository autoRefundTaskRepository,
            CouponConsumptionTaskRepository couponConsumptionTaskRepository,
            CartCleanupRetryTaskRepository cartCleanupRetryTaskRepository,
            GrantPaymentTaskRepository grantPaymentTaskRepository
    ) {
        this.stockTxRecordRepository = stockTxRecordRepository;
        this.autoRefundTaskRepository = autoRefundTaskRepository;
        this.couponConsumptionTaskRepository = couponConsumptionTaskRepository;
        this.cartCleanupRetryTaskRepository = cartCleanupRetryTaskRepository;
        this.grantPaymentTaskRepository = grantPaymentTaskRepository;
    }

    /**
     * 每週日凌晨 02:00 執行任務表清理
     */
    @Scheduled(cron = "${order.cleanup.cron:0 0 2 * * SUN}")
    public void cleanup() {
        if (!enabled) {
            return;
        }

        try {
            // 清理已完成的任務記錄
            LocalDateTime threshold = DateTimeUtils.nowUtc().minusDays(taskRetentionDays);

            int stockDeleted = stockTxRecordRepository.deleteCompletedBefore(threshold);
            int autoRefundDeleted = autoRefundTaskRepository.deleteCompletedBefore(threshold);
            int couponDeleted = couponConsumptionTaskRepository.deleteCompletedBefore(threshold);
            int cartDeleted = cartCleanupRetryTaskRepository.deleteCompletedBefore(threshold);
            int grantDeleted = grantPaymentTaskRepository.deleteCompletedBefore(threshold);

            int total = stockDeleted + autoRefundDeleted + couponDeleted + cartDeleted + grantDeleted;
            if (total > 0) {
                log.info("TaskCleanupJob.done",
                    "任務表清理完成, stockTx=" + stockDeleted
                        + ", autoRefund=" + autoRefundDeleted
                        + ", coupon=" + couponDeleted
                        + ", cart=" + cartDeleted
                        + ", grant=" + grantDeleted);
            }
        } catch (Exception e) {
            log.error("TaskCleanupJob.failed", "任務表清理失敗", e);
        }
    }
}
