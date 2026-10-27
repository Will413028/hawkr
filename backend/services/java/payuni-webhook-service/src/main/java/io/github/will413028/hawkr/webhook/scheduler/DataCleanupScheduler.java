package io.github.will413028.hawkr.webhook.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

/**
 * 資料清理排程
 *
 * 定期清除已處理的 webhook_inbox 和已發送/死信的 outbox_events
 */
@Component
@ConditionalOnProperty(name = "webhook.cleanup.enabled", havingValue = "true", matchIfMissing = true)
public class DataCleanupScheduler {

    private static final ObsLogger log = ObsLogger.forClass(DataCleanupScheduler.class);

    private final WebhookInboxMapper inboxMapper;
    private final OutboxEventMapper outboxMapper;
    private final int inboxRetentionDays;
    private final int outboxSentRetentionDays;
    private final int outboxDeadRetentionDays;

    public DataCleanupScheduler(WebhookInboxMapper inboxMapper,
                                 OutboxEventMapper outboxMapper,
                                 @Value("${webhook.cleanup.inbox-retention-days:90}") int inboxRetentionDays,
                                 @Value("${webhook.cleanup.outbox-sent-retention-days:7}") int outboxSentRetentionDays,
                                 @Value("${webhook.cleanup.outbox-dead-retention-days:30}") int outboxDeadRetentionDays) {
        this.inboxMapper = inboxMapper;
        this.outboxMapper = outboxMapper;
        this.inboxRetentionDays = inboxRetentionDays;
        this.outboxSentRetentionDays = outboxSentRetentionDays;
        this.outboxDeadRetentionDays = outboxDeadRetentionDays;
    }

    @Scheduled(cron = "${webhook.cleanup.cron:0 15 2 * * ?}")
    public void cleanup() {
        log.info("DataCleanupScheduler.cleanup.start", "開始資料清理");

        // 清理已處理的 inbox（保留 N 天）。
        // terminal 狀態：ENQUEUED（已入隊）、DEDUPED（保留未用）、IGNORED（UNKNOWN 已稽核不投遞）。
        // FAILED 不清 — 失敗 inbox 留作人工排查或 replay 路徑復原依據。
        var inboxCutoff = DateTimeUtils.nowUtc().minusDays(inboxRetentionDays);
        int inboxDeleted = inboxMapper.delete(
                new LambdaQueryWrapper<WebhookInbox>()
                        .in(WebhookInbox::getStatus,
                                WebhookInboxStatus.ENQUEUED.value(),
                                WebhookInboxStatus.DEDUPED.value(),
                                WebhookInboxStatus.IGNORED.value())
                        .lt(WebhookInbox::getReceivedAt, inboxCutoff));
        log.info("DataCleanupScheduler.inbox.cleaned",
                "Inbox 清理完成", "count", String.valueOf(inboxDeleted));

        // 清理已發送的 outbox（保留 N 天）
        var sentCutoff = DateTimeUtils.nowUtc().minusDays(outboxSentRetentionDays);
        int sentDeleted = outboxMapper.delete(
                new LambdaQueryWrapper<OutboxEvent>()
                        .eq(OutboxEvent::getStatus, OutboxEventStatus.SENT.value())
                        .lt(OutboxEvent::getSentAt, sentCutoff));
        log.info("DataCleanupScheduler.outbox.sent.cleaned",
                "已發送 Outbox 清理完成", "count", String.valueOf(sentDeleted));

        // 清理死信 outbox（保留 N 天）
        var deadCutoff = DateTimeUtils.nowUtc().minusDays(outboxDeadRetentionDays);
        int deadDeleted = outboxMapper.delete(
                new LambdaQueryWrapper<OutboxEvent>()
                        .eq(OutboxEvent::getStatus, OutboxEventStatus.DEAD.value())
                        .lt(OutboxEvent::getCreatedAt, deadCutoff));
        log.info("DataCleanupScheduler.outbox.dead.cleaned",
                "死信 Outbox 清理完成", "count", String.valueOf(deadDeleted));

        log.info("DataCleanupScheduler.cleanup.done", "資料清理完成");
    }
}
