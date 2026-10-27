package io.github.will413028.hawkr.cart.scheduler;

import io.github.will413028.hawkr.cart.repository.CartItemRepository;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 鎖定過期解鎖 Job（設計書 §7.1）
 *
 * - 每 1 分鐘執行一次
 * - 解鎖 status=LOCKED 且 lock_expire_time 已過期的項目
 * - 防止 Order Service 崩潰導致商品永久鎖定
 * - 與 CartService.LOCK_TTL_MINUTES=2 搭配：最壞情況下使用者等待 ≤ 3 分鐘（TTL 2min + 掃描週期 1min）
 */
@Component
public class UnlockExpiredItemsJob {

    private static final ObsLogger log = ObsLogger.forClass(UnlockExpiredItemsJob.class);
    private final CartItemRepository cartItemRepository;

    @Value("${cart.scheduler.unlock-expired.enabled:true}")
    private boolean enabled;

    public UnlockExpiredItemsJob(CartItemRepository cartItemRepository) {
        this.cartItemRepository = cartItemRepository;
    }

    @Scheduled(cron = "${cart.scheduler.unlock-expired.cron:0 */1 * * * ?}")
    public void execute() {
        if (!enabled) {
            return;
        }

        int count = cartItemRepository.unlockExpiredItems();
        if (count > 0) {
            log.info("UnlockExpiredItemsJob.execute.success",
                    "解鎖過期項目完成", "count", String.valueOf(count));
        }
    }
}
