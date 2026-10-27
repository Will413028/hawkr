package io.github.will413028.hawkr.product.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.product.repository.FavoriteCountEventDedupRepository;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * favorite_count_event_dedup 排程清理（02 §3.5.1 / §8.4）。
 *
 * 預設每日 02:30 清理 30 天前的 row，避免表無限膨脹。
 */
@Component
public class FavoriteEventDedupCleanupJob {

    private static final ObsLogger log = ObsLogger.forClass(FavoriteEventDedupCleanupJob.class);

    private final FavoriteCountEventDedupRepository repository;

    @Value("${product.cleanup.favorite-dedup.enabled:true}")
    private boolean enabled;

    @Value("${product.cleanup.favorite-dedup.retention-days:30}")
    private int retentionDays;

    public FavoriteEventDedupCleanupJob(FavoriteCountEventDedupRepository repository) {
        this.repository = repository;
    }

    @Scheduled(cron = "${product.cleanup.favorite-dedup.cron:0 30 2 * * ?}")
    public void cleanup() {
        if (!enabled) {
            return;
        }
        try {
            int deleted = repository.deleteBefore(DateTimeUtils.nowUtc().minusDays(retentionDays));
            if (deleted > 0) {
                log.info("FavoriteEventDedupCleanupJob.done",
                        "清理收藏計數事件去重 " + deleted + " 筆 (>" + retentionDays + " 天)");
            }
        } catch (Exception e) {
            log.error("FavoriteEventDedupCleanupJob.failed", "清理失敗", e);
        }
    }
}
