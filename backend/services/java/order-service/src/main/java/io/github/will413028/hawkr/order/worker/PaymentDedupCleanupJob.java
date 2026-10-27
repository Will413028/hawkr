package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.repository.PaymentEventDedupRepository;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 支付事件冪等表清理排程
 *
 * 每日凌晨 03:00 執行，刪除 7 天前的冪等記錄
 * 避免 payment_event_dedup 表無限膨脹
 */
@Component
public class PaymentDedupCleanupJob {

    private static final ObsLogger log = ObsLogger.forClass(PaymentDedupCleanupJob.class);

    private final PaymentEventDedupRepository dedupRepository;

    public PaymentDedupCleanupJob(PaymentEventDedupRepository dedupRepository) {
        this.dedupRepository = dedupRepository;
    }

    /**
     * 每日凌晨 03:00 執行
     */
    @Scheduled(cron = "0 0 3 * * ?")
    public void cleanup() {
        log.info("PaymentDedupCleanupJob.start", "開始清理過期冪等記錄");

        int deleted = dedupRepository.deleteExpired();

        log.info("PaymentDedupCleanupJob.done", "清理完成", "deletedCount", String.valueOf(deleted));
    }
}
