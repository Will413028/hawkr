package io.github.will413028.hawkr.shipping.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

/**
 * Webhook 冪等表定期清理排程
 * 每日凌晨 3 點清理 7 天前的 dedup 記錄
 */
@Component
public class DedupCleanupScheduler {

    private static final ObsLogger log = ObsLogger.forClass(DedupCleanupScheduler.class);
    private static final int RETENTION_DAYS = 7;

    private final ShippingWebhookDedupMapper dedupMapper;

    public DedupCleanupScheduler(ShippingWebhookDedupMapper dedupMapper) {
        this.dedupMapper = dedupMapper;
    }

    @Scheduled(cron = "0 0 3 * * ?")
    public void cleanupExpiredDedup() {
        var cutoff = DateTimeUtils.nowUtc().minusDays(RETENTION_DAYS);

        int deleted = dedupMapper.delete(
                new LambdaQueryWrapper<ShippingWebhookDedupEntity>()
                        .lt(ShippingWebhookDedupEntity::getProcessedAt, cutoff));

        if (deleted > 0) {
            log.info("DedupCleanupScheduler.cleanup.success",
                    "清理過期 dedup 記錄，刪除 " + deleted + " 筆");
        }
    }
}
