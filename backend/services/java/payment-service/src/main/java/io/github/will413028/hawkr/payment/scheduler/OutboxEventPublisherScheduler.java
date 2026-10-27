package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxPublisher;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * §2A.6 Outbox 事件發布排程（payment-service）。
 * <p>
 * 每 5 秒掃描 outbox_events 中 PENDING / FAILED（且已到重試時間）的記錄，
 * 透過 {@link PaymentOutboxPublisher} 派送到 RabbitMQ。
 * 超過 max-retries 則標 DEAD，停止重試（需人工告警處理）。
 *
 * <p><b>搶佔語意（issue #2294 H2）</b>：多副本 claim 閘門<b>不在這裡</b>，在
 * {@link PaymentOutboxPublisher#publish} 的第一行——因為 outbox 有兩條進入點
 * （本排程 ＋ {@code PaymentOutboxWriter} 的 afterCommit 即時投遞），閘門必須放在兩者的
 * 共同收斂點。詳見 {@code OutboxEventMapper.claimForDispatch} 與
 * {@code docs/開發規範/多副本排程搶佔規範.md}。
 */
@Component
public class OutboxEventPublisherScheduler {

    private static final ObsLogger log = ObsLogger.forClass(OutboxEventPublisherScheduler.class);
    private static final int BATCH_SIZE = 50;

    private final OutboxEventMapper outboxMapper;
    private final PaymentOutboxPublisher publisher;
    private final int maxRetries;

    public OutboxEventPublisherScheduler(
            OutboxEventMapper outboxMapper,
            PaymentOutboxPublisher publisher,
            @Value("${payment.outbox.max-retries:5}") int maxRetries
    ) {
        this.outboxMapper = outboxMapper;
        this.publisher = publisher;
        this.maxRetries = maxRetries;
    }

    @Scheduled(fixedDelay = 5000)
    public void publishPendingEvents() {
        List<OutboxEventEntity> events = outboxMapper.findPendingEvents(BATCH_SIZE);
        if (events.isEmpty()) {
            return;
        }

        log.info("OutboxEventPublisherScheduler.poll.found",
                "待派送 outbox 事件 count=" + events.size());

        for (OutboxEventEntity event : events) {
            int retry = event.getRetryCount() == null ? 0 : event.getRetryCount();
            if (retry >= maxRetries) {
                markDead(event);
                continue;
            }
            publisher.publish(event);
        }
    }

    /**
     * 超過重試上限 → DEAD（issue #2294 H2：具名守衛 UPDATE，取代 {@code updateById} 全欄覆寫）。
     */
    private void markDead(OutboxEventEntity event) {
        outboxMapper.markDeadGuarded(event.getId(), "超過最大重試次數 " + maxRetries);
        log.error("OutboxEventPublisherScheduler.markDead.done",
                "outbox 事件已標 DEAD id=" + event.getId() + " routing_key=" + event.getRoutingKey());
    }
}
