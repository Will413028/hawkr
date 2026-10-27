package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderCountdownService;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 倒數對帳兜底（03 §5.8.7）。
 * 每日掃 paused 超過 {@code STALLED_DAYS} 天的訂單，反查 AfterSales 確認是否需要恢復。
 * 防止 MQ 事件遺失導致訂單永久卡暫停。
 */
@Component
public class CountdownRecoveryWorker {

    private static final ObsLogger log = ObsLogger.forClass(CountdownRecoveryWorker.class);

    private static final int STALLED_DAYS = 7;
    private static final int BATCH_LIMIT = 200;

    private final OrderRepository orderRepository;
    private final OrderCountdownService countdownService;

    public CountdownRecoveryWorker(OrderRepository orderRepository,
                                   OrderCountdownService countdownService) {
        this.orderRepository = orderRepository;
        this.countdownService = countdownService;
    }

    /**
     * 每日 03:00 UTC 掃描。
     */
    @Scheduled(cron = "${hawkr.order.countdown-recovery-scan-cron:0 0 3 * * ?}", zone = "UTC")
    public void scan() {
        LocalDateTime threshold = DateTimeUtils.nowUtc().minusDays(STALLED_DAYS);
        List<String> orderNos = orderRepository.scanStalledPausedOrders(threshold, BATCH_LIMIT);
        if (orderNos.isEmpty()) {
            return;
        }
        log.info("CountdownRecoveryWorker.scan.found",
                "暫停超過 " + STALLED_DAYS + " 天的訂單", "count", String.valueOf(orderNos.size()));
        for (String orderNo : orderNos) {
            try {
                countdownService.tryResume(orderNo, "stalled-reconcile");
            } catch (Exception e) {
                log.error("CountdownRecoveryWorker.resume.error",
                        "對帳恢復失敗: " + orderNo, e);
            }
        }
    }
}
