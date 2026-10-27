package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.OrderOutboxPublisher;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * order.paid 可靠投遞排程（03 §8.10）。
 *
 * <p>輪詢 order_outbox 中待派送（PENDING / FAILED 且已到重試時間）事件，交由
 * {@link OrderOutboxPublisher} 派送。超過上限標 DEAD 停止重試（待人工告警）。
 *
 * <p><b>搶佔語意（issue #2294 H1）</b>：本排程是 {@link OrderOutboxPublisher#publish} 的
 * <b>唯一</b>呼叫點（全 repo 實查），因此 claim 閘門放在這裡、在 publish <b>之前</b>。
 * 閘門為 <b>lease on {@code next_retry_at}</b>（{@link #CLAIM_LEASE_SEC} 秒），
 * 不是永久旗標——租約到期該列自動回到候選集合。
 */
@Component
public class OrderOutboxPublisherScheduler {

    private static final ObsLogger log = ObsLogger.forClass(OrderOutboxPublisherScheduler.class);
    private static final int BATCH_SIZE = 50;
    private static final int MAX_RETRIES = 10;

    /**
     * issue #2294 H1：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：候選述詞
     * （{@link OrderOutboxMapper#findDispatchable}）只有 {@code status IN (PENDING, FAILED)}
     * ＋ {@code next_retry_at <= now}，<b>沒有任何時間上界</b> ⇒ 候選窗口寬度 <b>∞</b>
     * ⇒ 任何租約長度都合格；cron 週期 5 秒遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步：租約必須 &gt; 單筆工作的最壞耗時）：claim 之後的工作是
     * {@code buildBody}（{@code order.paid} / {@code order.cancelled} 會做 seller/buyer 的
     * enrich gRPC，皆為 best-effort try/catch）＋ 一次 {@code rabbitPublisher.publish}。
     * enrich 的 gRPC 由呼叫點 deadline 封頂（{@code SellerGrpcClient.RESOLVE_PRINCIPALS_DEADLINE_SEC}
     * / {@code BuyerGrpcClient.RESOLVE_PRINCIPALS_DEADLINE_SEC}，各 10 秒；單筆最多 3 次往返
     * ⇒ 上界 30 秒）。取 <b>60 秒</b>（&gt; 30 秒最壞耗時，且遠小於 ∞ 的窗口）。
     *
     * <p>⚠️ <b>這個 60 秒是「每一筆」的，不是「整批」的</b>（R4-P1-2，2026-08-08）：
     * 租約基準必須在<b>迴圈內</b>取 {@code DateTimeUtils.nowUtc()}。批次上界為
     * {@code BATCH_SIZE(50) × 30 秒 ≈ 25 分鐘}，遠大於 60 秒——若拿 tick 起點當基準，
     * 排在後面的項目 claim 下去時租約早已過期。
     */
    static final long CLAIM_LEASE_SEC = 60L;

    private final OrderOutboxMapper outboxMapper;
    private final OrderOutboxPublisher publisher;

    public OrderOutboxPublisherScheduler(OrderOutboxMapper outboxMapper, OrderOutboxPublisher publisher) {
        this.outboxMapper = outboxMapper;
        this.publisher = publisher;
    }

    @Scheduled(fixedDelayString = "${hawkr.order.outbox-publish-interval-ms:5000}")
    public void publishPendingEvents() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        List<OrderOutboxEntity> events = outboxMapper.findDispatchable(now, BATCH_SIZE);
        if (events.isEmpty()) {
            return;
        }

        log.info("OrderOutboxPublisherScheduler.poll.found", "待派送 outbox 事件 count=" + events.size());

        for (OrderOutboxEntity event : events) {
            int retry = event.getRetryCount() == null ? 0 : event.getRetryCount();
            if (retry >= MAX_RETRIES) {
                markDead(event);
                continue;
            }
            // issue #2294 H1：搶佔閘門必須在副作用（MQ 投遞）之前，且是 lease 不是永久旗標。
            // claim 回 0 ⇒ 另一個 pod 已認領（或狀態已變）⇒ 一個副作用都不做。
            //
            // ⚠️ R4-P1-2（2026-08-08）：租約基準取 **claim 當下**而非 tick 起點的 now。
            // 前者才是「這一筆的工作從現在開始算 CLAIM_LEASE_SEC 秒」；用 tick 起點時，
            // 第 k 筆的有效租約 = CLAIM_LEASE_SEC − (前 k−1 筆的耗時)，批次夠長就變成負數
            // ⇒ 租約在持有者仍在工作時到期 ⇒ 另一個 pod 合法搶走 ⇒ 重複副作用。
            // 述詞用的 now 維持 tick 起點：那個方向讓候選集合更小，是保守的。
            int claimed = outboxMapper.claimForDispatch(
                    event.getId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC), now);
            if (claimed == 0) {
                continue;
            }
            publisher.publish(event);
        }
    }

    /**
     * 超過重試上限 → DEAD（issue #2294：具名守衛 UPDATE，取代 {@code updateById} 全欄覆寫）。
     */
    private void markDead(OrderOutboxEntity event) {
        outboxMapper.markDeadGuarded(event.getId());
        log.error("OrderOutboxPublisherScheduler.markDead",
                "outbox 事件超過重試上限標 DEAD id=" + event.getId() + " routing_key=" + event.getRoutingKey(),
                "orderNo", event.getAggregateId());
    }
}
