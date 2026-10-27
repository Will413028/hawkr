package io.github.will413028.hawkr.cart.scheduler;

import io.github.will413028.hawkr.cart.repository.CartItemRepository;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

/**
 * 訪客購物車資料清理 Job（設計書 §7.2）
 *
 * - 每日離峰（凌晨 03:00）執行
 * - 清理 30 天前的訪客購物車資料
 * - 控制 DB 容量，避免訪客資料永久堆積
 */
@Component
public class GuestCartCleanupJob {

    private static final ObsLogger log = ObsLogger.forClass(GuestCartCleanupJob.class);
    private final CartItemRepository cartItemRepository;

    @Value("${cart.scheduler.guest-cleanup.enabled:true}")
    private boolean enabled;

    @Value("${cart.scheduler.guest-cleanup-days:30}")
    private int cleanupDays;

    public GuestCartCleanupJob(CartItemRepository cartItemRepository) {
        this.cartItemRepository = cartItemRepository;
    }

    @Scheduled(cron = "${cart.scheduler.guest-cleanup.cron:0 0 3 * * ?}")
    public void execute() {
        if (!enabled) {
            return;
        }

        var cutoffTime = DateTimeUtils.nowUtc().minusDays(cleanupDays);
        int count = cartItemRepository.deleteExpiredGuestItems(cutoffTime);

        log.info("GuestCartCleanupJob.execute.success",
                "訪客購物車清理完成",
                "count", String.valueOf(count));
    }
}
