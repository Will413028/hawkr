package io.github.will413028.hawkr.coupon.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
import io.github.will413028.hawkr.coupon.service.CouponTemplateService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 自動過期排程
 * 1. Template：status=ACTIVE 且 end_at < NOW() → EXPIRED
 * 2. Instance：status=UNUSED 且所屬 Template 已過期 → EXPIRED
 */
@Component
public class AutoExpireJob {

    private static final ObsLogger log = ObsLogger.forClass(AutoExpireJob.class);

    @Value("${coupon.scheduler.auto-expire.enabled:true}")
    private boolean enabled;

    private final CouponTemplateService templateService;
    private final CouponInstanceService instanceService;

    public AutoExpireJob(CouponTemplateService templateService, CouponInstanceService instanceService) {
        this.templateService = templateService;
        this.instanceService = instanceService;
    }

    @Scheduled(cron = "${coupon.scheduler.auto-expire.cron:0 */5 * * * ?}")
    public void execute() {
        if (!enabled) {
            return;
        }
        try {
            // 1. 過期模板
            int expiredTemplates = templateService.expireTemplates();
            if (expiredTemplates > 0) {
                log.info("AutoExpireJob.expireTemplates.done", "自動過期模板",
                        "expiredCount", String.valueOf(expiredTemplates));
            }

            // 2. 過期未使用的 Instance（依已過期模板批次處理）
            int expiredInstances = instanceService.expireInstancesByTemplate();
            if (expiredInstances > 0) {
                log.info("AutoExpireJob.expireInstances.done", "自動過期優惠券",
                        "expiredCount", String.valueOf(expiredInstances));
            }
        } catch (Exception e) {
            log.error("AutoExpireJob.execute.failed", "自動過期排程異常", e);
        }
    }
}
