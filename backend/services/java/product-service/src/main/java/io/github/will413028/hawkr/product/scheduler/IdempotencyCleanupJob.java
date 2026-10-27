package io.github.will413028.hawkr.product.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.service.IdempotencyService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 冪等記錄清理排程
 *
 * 定期清理過期的 idempotency_request 記錄，避免表無限膨脹。
 */
@Component
public class IdempotencyCleanupJob {

    private static final ObsLogger log = ObsLogger.forClass(IdempotencyCleanupJob.class);

    private final IdempotencyService idempotencyService;

    @Value("${product.cleanup.idempotency.enabled:true}")
    private boolean enabled;

    public IdempotencyCleanupJob(IdempotencyService idempotencyService) {
        this.idempotencyService = idempotencyService;
    }

    /**
     * 每日凌晨 02:00 清理過期冪等記錄
     */
    @Scheduled(cron = "${product.cleanup.idempotency.cron:0 0 2 * * ?}")
    public void cleanup() {
        if (!enabled) {
            return;
        }

        try {
            int deleted = idempotencyService.cleanupExpired();
            if (deleted > 0) {
                log.info("IdempotencyCleanupJob.done", "清理過期冪等記錄 " + deleted + " 筆");
            }
        } catch (Exception e) {
            log.error("IdempotencyCleanupJob.failed", "清理失敗", e);
        }
    }
}
