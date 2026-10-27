package io.github.will413028.hawkr.webhook.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.webhook.infrastructure.mq.OutboxPublisher;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Outbox 事件發布排程
 *
 * 每 5 秒掃描 outbox_events 中 PENDING 或 FAILED（且已到重試時間）的記錄，
 * 透過 OutboxPublisher 發布到 RabbitMQ。超過最大重試次數則標記為 DEAD。
 *
 * <p><b>搶佔語意（issue #2294 H3）</b>：本排程是 outbox 的<b>唯一</b>進入點
 * （webhook 側沒有 payment 那種 afterCommit 即時投遞），因此 claim 閘門放在這裡、
 * 在 {@link OutboxPublisher#publish} <b>之前</b>。閘門為 <b>lease on {@code next_retry_at}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒），不是永久旗標——租約到期該列自動回到候選集合。
 */
@Component
public class OutboxPublisherScheduler {

    private static final ObsLogger log = ObsLogger.forClass(OutboxPublisherScheduler.class);
    private static final int BATCH_SIZE = 50;

    /**
     * issue #2294 H3：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：候選述詞
     * （{@code OutboxEventMapper.findPendingEvents}）只有 {@code status IN (PENDING, FAILED)}
     * ＋ {@code next_retry_at <= NOW()}，<b>沒有任何時間上界</b> ⇒ 候選窗口寬度 <b>∞</b>
     * ⇒ 任何租約長度都合格；cron 週期 5 秒遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步：租約必須 &gt; 單筆工作的最壞耗時）：claim 之後唯一的長工作是
     * {@code RabbitTemplate.send}（非同步送出、不等 broker confirm），量級為毫秒。
     * 取 <b>60 秒</b>，遠大於最壞耗時，同時遠小於 ∞ 的窗口。
     *
     * <p>⚠️ <b>這個 60 秒是「每一筆」的，不是「整批」的</b>（R4-P1-2，2026-08-08）：
     * 租約基準必須在<b>迴圈內</b>取 {@code DateTimeUtils.nowUtc()}。本服務單筆是毫秒級、
     * {@code BATCH_SIZE(50)} 的批次上界遠小於 60 秒，所以現況不會踩到；
     * 但這是<b>參數巧合</b>不是結構保證（改 BATCH_SIZE 或給 send 加上同步 confirm 就會踩到），
     * 故與 order 側採同一寫法。
     */
    static final long CLAIM_LEASE_SEC = 60L;

    private final OutboxEventMapper outboxMapper;
    private final OutboxPublisher outboxPublisher;
    private final int maxRetries;

    public OutboxPublisherScheduler(OutboxEventMapper outboxMapper,
                                     OutboxPublisher outboxPublisher,
                                     @Value("${webhook.outbox.max-retries:5}") int maxRetries) {
        this.outboxMapper = outboxMapper;
        this.outboxPublisher = outboxPublisher;
        this.maxRetries = maxRetries;
    }

    @Scheduled(fixedDelay = 5000)
    public void publishPendingEvents() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        List<OutboxEvent> events = outboxMapper.findPendingEvents(BATCH_SIZE);
        if (events.isEmpty()) {
            return;
        }

        log.info("OutboxPublisherScheduler.poll.found",
                "待發布事件", "count", String.valueOf(events.size()));

        for (OutboxEvent event : events) {
            if (event.getRetryCount() >= maxRetries) {
                markDead(event);
                continue;
            }
            // issue #2294 H3：搶佔閘門必須在副作用（MQ 投遞）之前，且是 lease 不是永久旗標。
            // claim 回 0 ⇒ 另一個 pod 已認領（或狀態已變）⇒ 一個副作用都不做。
            //
            // ⚠️ R4-P1-2（2026-08-08）：租約基準取 **claim 當下**而非 tick 起點的 now
            // （用 tick 起點時第 k 筆的有效租約會被前 k−1 筆的耗時吃掉）。
            // 述詞用的 now 維持 tick 起點：那個方向讓候選集合更小，是保守的。
            int claimed = outboxMapper.claimForDispatch(
                    event.getId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC), now);
            if (claimed == 0) {
                continue;
            }
            outboxPublisher.publish(event);
        }
    }

    /**
     * 超過最大重試次數 → DEAD（具名守衛 UPDATE，取代 {@code updateById} 全欄覆寫）。
     */
    private void markDead(OutboxEvent event) {
        outboxMapper.markDeadGuarded(event.getId(), "超過最大重試次數 " + maxRetries);
        log.error("OutboxPublisherScheduler.markDead.done",
                "事件已標記為 DEAD: " + event.getMessageKey());
    }
}
