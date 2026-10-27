package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.shipping.mapper.PayUniAuditLogMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;

/**
 * PayUni 互動稽核專表清理排程（payuni_audit_logs）。
 * <p>
 * 保留 7 年（2555 天，可配置），每日 3:30 AM DELETE 保留期外記錄（見 06 §10.7）。
 * 與 DedupCleanupScheduler（7 天冪等表）語意不同，故獨立成 scheduler。
 */
@Component
public class PayUniAuditCleanupScheduler {

    private static final ObsLogger log = ObsLogger.forClass(PayUniAuditCleanupScheduler.class);

    private final PayUniAuditLogMapper mapper;

    @Value("${shipping.cleanup.payuni-audit-enabled:true}")
    private boolean enabled;

    // 物流爭議追溯下限 7 年（2555 天）
    @Value("${shipping.cleanup.payuni-audit-retention-days:2555}")
    private int retentionDays;

    public PayUniAuditCleanupScheduler(PayUniAuditLogMapper mapper) {
        this.mapper = mapper;
    }

    @Scheduled(cron = "${shipping.cleanup.payuni-audit-cron:0 30 3 * * ?}")
    public void cleanup() {
        if (!enabled) {
            return;
        }
        try {
            LocalDateTime threshold = DateTimeUtils.nowUtc().minusDays(retentionDays);
            int deleted = mapper.deleteBefore(threshold);
            if (deleted > 0) {
                log.info("PayUniAuditCleanupScheduler.done",
                        "清理過期 PayUni 稽核記錄，刪除 " + deleted + " 筆");
            }
        } catch (Exception e) {
            log.error("PayUniAuditCleanupScheduler.failed", "PayUni 稽核清理失敗", e);
        }
    }
}
