package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Outbox + Inbox status 的原子事務（T2，REQUIRES_NEW）。
 * <p>
 * 設計書 §4.5.3：T2 在單一事務內完成
 * 「outbox INSERT（一或兩筆，replay-safe via ON CONFLICT）+ inbox status UPDATE 為 ENQUEUED」。
 * 任一步失敗 → 整段 rollback → outbox 與 inbox 都不會留下半成品。
 */
@Service
public class WebhookOutboxTxOps {

    private static final ObsLogger log = ObsLogger.forClass(WebhookOutboxTxOps.class);

    private final OutboxEventMapper outboxMapper;
    private final WebhookInboxMapper inboxMapper;

    public WebhookOutboxTxOps(OutboxEventMapper outboxMapper, WebhookInboxMapper inboxMapper) {
        this.outboxMapper = outboxMapper;
        this.inboxMapper = inboxMapper;
    }

    /**
     * 路由描述（一個 eventKey 可能要投到多個 exchange/topic，例如 AGENT_NOTIFY 雙通道）。
     */
    public record OutboxRoute(String eventKey, String exchange, String topic, String payloadJson) {}

    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void enqueueAndMark(Long inboxId, List<OutboxRoute> routes) {
        enqueueAndMark(inboxId, WebhookProvider.PAYUNI.value(), routes);
    }

    /**
     * 同上，但由呼叫端指定 {@code aggregate_type}。
     *
     * <p>：Uber webhook 共用本服務的 inbox／outbox 兩張表，
     * 而 {@code aggregate_type} 是「這批 outbox 列屬於哪個 provider」的唯一線索
     * —— 原本的單參數多載把它寫死成 PayUni，Uber 的列會被標成 PayUni 的，
     * 而那正是之後查問題時第一個會被相信的欄位。
     *
     * <p>⚠️ 去重仍由 {@code uq_outbox_aggregate_route (aggregate_id, exchange, topic)} 守，
     * <b>不含 aggregate_type</b> ⇒ aggregate_id（＝ eventKey）必須跨 provider 唯一。
     * PayUni 的 eventKey 一律以 {@code merId} 開頭、Uber 的是 provider event id（uuid 形狀），
     * 兩者不會撞；新增第三個 provider 時要重新確認這一點。
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void enqueueAndMark(Long inboxId, String aggregateType, List<OutboxRoute> routes) {
        LocalDateTime now = DateTimeUtils.nowUtc();

        for (OutboxRoute route : routes) {
            int affected = outboxMapper.insertIfAbsent(
                    aggregateType,
                    route.eventKey(),
                    route.exchange(),
                    route.topic(),
                    route.eventKey(),
                    route.payloadJson(),
                    OutboxEventStatus.PENDING.value(),
                    0,
                    now);

            if (affected == 0) {
                // §4.5.3 replay-safe：前次 T2 commit 後在某處中斷；本次重放時 outbox 已存在
                log.warn("WebhookOutboxTxOps.outbox.alreadyExists",
                        "outbox 已存在，視為 replay-safe 跳過 aggregateId=" + route.eventKey()
                                + " exchange=" + route.exchange() + " topic=" + route.topic(),
                        "aggregateId", route.eventKey());
            }
        }

        WebhookInbox patch = new WebhookInbox();
        patch.setId(inboxId);
        patch.setStatus(WebhookInboxStatus.ENQUEUED.value());
        patch.setProcessedAt(now);
        inboxMapper.updateById(patch);
    }
}
