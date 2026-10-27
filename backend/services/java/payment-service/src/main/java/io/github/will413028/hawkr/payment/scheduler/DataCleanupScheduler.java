package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PayUniAuditLogMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.time.temporal.ChronoUnit;

/**
 * 資料清理排程
 *
 * 定期清理 payment_logs、payment_webhooks、payuni_audit_logs、outbox_events 中的過期記錄，
 * 避免輔助表無限膨脹。
 */
@Component
public class DataCleanupScheduler {

    private static final ObsLogger log = ObsLogger.forClass(DataCleanupScheduler.class);

    private final PaymentLogMapper logMapper;
    private final PaymentWebhookMapper webhookMapper;
    private final PayUniAuditLogMapper payUniAuditLogMapper;
    private final OutboxEventMapper outboxMapper;

    @Value("${payment.cleanup.enabled:true}")
    private boolean enabled;

    @Value("${payment.cleanup.log-retention-days:180}")
    private int logRetentionDays;

    @Value("${payment.cleanup.error-log-retention-days:365}")
    private int errorLogRetentionDays;

    @Value("${payment.cleanup.webhook-retention-days:90}")
    private int webhookRetentionDays;

    // PayUni 互動稽核專表：金流爭議追溯下限 7 年（2555 天），見 05 §11.9
    @Value("${payment.cleanup.payuni-audit-retention-days:2555}")
    private int payUniAuditRetentionDays;

    // outbox_events 已成功投遞（SENT）的保留天數。沿用 application.yml 既有 key，不另立設定。
    @Value("${payment.cleanup.outbox-sent-retention-days:7}")
    private int outboxSentRetentionDays;

    // 🔴 application.yml 另有 payment.cleanup.outbox-dead-retention-days，**刻意未接線**。
    // DEAD（8224）是 permanent delivery failure 的證據，也是 payment_outbox_dead_rows 告警的來源。
    // 讓 timer 自動刪它的語意是錯的：
    //     事故沒解決 → 保留期到 → 列消失 → 告警自己恢復綠
    // 未來若真的需要 DEAD retention，前提是先有明確的 operational resolution／acknowledgement／
    // replay 語意（例如「已人工補發並註記」才可清），而不是單純的時間到期。

    public DataCleanupScheduler(
            PaymentLogMapper logMapper,
            PaymentWebhookMapper webhookMapper,
            PayUniAuditLogMapper payUniAuditLogMapper,
            OutboxEventMapper outboxMapper
    ) {
        this.logMapper = logMapper;
        this.webhookMapper = webhookMapper;
        this.payUniAuditLogMapper = payUniAuditLogMapper;
        this.outboxMapper = outboxMapper;
    }

    /**
     * 每日凌晨 02:30 執行資料清理
     */
    @Scheduled(cron = "${payment.cleanup.cron:0 30 2 * * ?}")
    public void cleanup() {
        if (!enabled) {
            return;
        }

        try {
            int total = 0;
            total += cleanupLogs();
            total += cleanupWebhooks();
            total += cleanupPayUniAuditLogs();
            total += cleanupOutboxSent();

            if (total > 0) {
                log.info("DataCleanupScheduler.done", "資料清理完成，共刪除 " + total + " 筆");
            }
        } catch (Exception e) {
            log.error("DataCleanupScheduler.failed", "資料清理失敗", e);
        }
    }

    /**
     * 清理 payment_logs
     * INFO/WARNING：保留 180 天（PayUni 對話稽核需求，§11.6）；ERROR：保留 365 天
     */
    private int cleanupLogs() {
        LocalDateTime nonErrorThreshold = DateTimeUtils.nowUtc().minus(logRetentionDays, ChronoUnit.DAYS);
        LocalDateTime errorThreshold = DateTimeUtils.nowUtc().minus(errorLogRetentionDays, ChronoUnit.DAYS);

        int nonError = logMapper.deleteNonErrorBefore(nonErrorThreshold);
        int error = logMapper.deleteErrorBefore(errorThreshold);
        return nonError + error;
    }

    /**
     * 清理 payment_webhooks（已處理：保留 90 天）
     */
    private int cleanupWebhooks() {
        LocalDateTime threshold = DateTimeUtils.nowUtc().minus(webhookRetentionDays, ChronoUnit.DAYS);
        return webhookMapper.deleteProcessedBefore(threshold);
    }

    /**
     * 清理 payuni_audit_logs（金流爭議稽核專表，保留 7 年）
     */
    private int cleanupPayUniAuditLogs() {
        LocalDateTime threshold = DateTimeUtils.nowUtc().minus(payUniAuditRetentionDays, ChronoUnit.DAYS);
        return payUniAuditLogMapper.deleteBefore(threshold);
    }

    /**
     * 清理 outbox_events 中已成功投遞（SENT）且超過保留期的列。
     *
     * <p>🔴 <b>只刪 SENT</b>——PENDING／FAILED 仍在派送候選集合內（刪了等於事件蒸發），
     * DEAD 是事故證據且是告警來源（見上方欄位註解）。述詞正面列舉，見
     * {@link OutboxEventMapper#deleteSentBefore}。
     *
     * <p>retention 的起點是 {@code sent_at}（真正送出去那一刻），不是 {@code created_at}
     * ——與 {@code cleanupWebhooks()} 用 {@code processed_at} 同一條慣例。
     */
    private int cleanupOutboxSent() {
        LocalDateTime threshold = DateTimeUtils.nowUtc().minus(outboxSentRetentionDays, ChronoUnit.DAYS);
        return outboxMapper.deleteSentBefore(threshold);
    }
}
