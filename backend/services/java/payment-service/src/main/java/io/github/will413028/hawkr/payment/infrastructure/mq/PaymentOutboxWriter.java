package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentEventType;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.util.HashMap;
import java.util.Map;

/**
 * §5.10.1 付款結果事件統一寫入器（issue #1349 L1'）。
 * <p>
 * 六個 {@code payment.success} / {@code payment.failed} / {@code payment.conflict}
 * 發布點經兩個天然收斂點（A={@code PaymentNotifyHandler}、
 * B={@code PaymentApplicationService.publishEvent}）統一走本 writer，將
 * 「MQ 事件發布」由事務外/後直發改為 transactional outbox with immediate dispatch：
 * <ol>
 *   <li>事務內 {@code INSERT outbox_events(status=PENDING)}——與交易狀態落地、
 *       Inbox {@code markAsProcessed} 原子（rollback 三者皆不落地、commit 一併落地）；</li>
 *   <li>註冊 {@code afterCommit} synchronization，於事務 commit 成功後同執行緒即時
 *       {@link PaymentOutboxPublisher#publish} 投遞（publisher-confirm，正常路徑即時履約）；</li>
 *   <li>afterCommit dispatch 失敗（nack / timeout / 例外）交
 *       {@code OutboxEventPublisherScheduler} 每 5s 兜底重投。</li>
 * </ol>
 * envelope 組裝集中於此（統一補齊收斂點 A 原缺的 {@code event_id}），payload JSON
 * 於 INSERT 時凍結一次 → afterCommit 送與排程器兜底送同一份 bytes，at-least-once
 * 重送完全一致；下游以 {@code trade_no} 單欄冪等吸收。
 */
@Component
public class PaymentOutboxWriter {

    private static final ObsLogger log = ObsLogger.forClass(PaymentOutboxWriter.class);

    private final OutboxEventMapper outboxMapper;
    private final ObjectMapper objectMapper;
    private final PaymentOutboxPublisher paymentOutboxPublisher;
    // issue #1369 業務健康 metric：付款終態結果計數（此 writer 是 payment.success /
    // payment.failed 六發布點的兩收斂點統一入口，於 afterCommit 計數 = 只計已提交的業務事件）。
    // Prometheus 對 Counter 自動補 _total → 暴露為 hawkr_payment_total /
    // hawkr_payment_failed_total，供 business-alerts.yml PaymentFailureSpike 規則。
    private final Counter paymentTotalCounter;
    private final Counter paymentFailedCounter;

    public PaymentOutboxWriter(OutboxEventMapper outboxMapper,
                               ObjectMapper objectMapper,
                               PaymentOutboxPublisher paymentOutboxPublisher,
                               MeterRegistry meterRegistry) {
        this.outboxMapper = outboxMapper;
        this.objectMapper = objectMapper;
        this.paymentOutboxPublisher = paymentOutboxPublisher;
        this.paymentTotalCounter = Counter.builder("hawkr_payment")
                .description("付款終態結果總數（success + failed），issue #1369")
                .register(meterRegistry);
        this.paymentFailedCounter = Counter.builder("hawkr_payment_failed")
                .description("付款失敗總數，issue #1369")
                .register(meterRegistry);
    }

    /**
     * 在當前事務內寫入 outbox 事件並註冊 afterCommit 即時投遞。
     * <p>
     * 必須在 Spring 管理的 active transaction 內呼叫（收斂點 A 在
     * {@code transactionTemplate.execute}、收斂點 B 在 {@code @Transactional}），
     * 否則 afterCommit synchronization 不會生效。
     * <p>
     * 序列化失敗即拋例外讓事務 rollback（§5.10.1 / §2A.6.5：禁止 catch + return，
     * 否則造成狀態 UPDATE 落地但 outbox 漏寫的原子化破口）。
     */
    public void enqueueAndDispatch(PaymentDomainEvent event) {
        // 組 envelope：event.payload() + 36 §7.4 Conformance Contract metadata
        // （event_id 為 UUID v4 必含欄位；統一補齊收斂點 A 原缺的 event_id）
        Map<String, Object> payload = new HashMap<>(event.payload());
        OutboxEventFactory.applyEnvelopeMetadata(payload,
                event.eventType().value(), event.aggregateType(),
                event.aggregateId(), event.occurredAt().toString());

        String payloadJson;
        try {
            payloadJson = objectMapper.writeValueAsString(payload);
        } catch (JsonProcessingException e) {
            // 禁止 catch + return：必須讓事務 rollback，避免狀態落地但事件漏寫
            throw new IllegalStateException(
                    "payment 事件 payload 序列化失敗: " + event.eventType().value() + " - " + e.getMessage(), e);
        }

        // payment 事件一律落 payment.events（36 §3.3.1 explicit persisted exchange）。
        // exchange 由本 payment-specific wrapper 決定並顯式傳入，generic path 自己不知道要發去哪。
        OutboxEventEntity entity = enqueue(MqConstants.EXCHANGE_PAYMENT,
                event.eventType().value(), event.eventType().value(), payloadJson);

        // issue #1369：付款終態業務 metric 於 afterCommit 計數（僅計已提交事件，rollback 不計）。
        // 只認 payment.success / payment.failed 為「付款終態結果」；rescue / conflict 不計入分母。
        final PaymentEventType eventType = event.eventType();

        // afterCommit 即時投遞：事務 commit 成功後於原執行緒立即送。
        // 例外全吞不 rethrow——afterCommit 上拋會傳到 commit 呼叫方（付款 gRPC caller），
        // 造成付款已成功卻對前端回錯；confirm 失敗只留 PENDING / FAILED 交排程器兜底。
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                // 業務 metric 先計（與投遞成敗無關：事件已提交即代表業務結果已發生）
                if (eventType == PaymentEventType.PAYMENT_SUCCESS || eventType == PaymentEventType.PAYMENT_FAILED) {
                    paymentTotalCounter.increment();
                    if (eventType == PaymentEventType.PAYMENT_FAILED) {
                        paymentFailedCounter.increment();
                    }
                }
                try {
                    paymentOutboxPublisher.publish(entity);
                } catch (Throwable t) {
                    log.error("PaymentOutboxWriter.afterCommit.dispatchFailed",
                            "afterCommit 即時投遞失敗，交排程器兜底 outbox_id=" + entity.getId()
                                    + " routing_key=" + entity.getRoutingKey(), t);
                }
            }
        });

        log.info("PaymentOutboxWriter.enqueued",
                "付款結果事件已寫入 outbox outbox_id=" + entity.getId()
                        + " routing_key=" + entity.getRoutingKey());
    }
    /**
     * generic enqueue：在當前交易內寫入一列 {@code outbox_events}（{@code status=PENDING}），
     * <b>不</b>決定投遞時機、<b>不</b>含任何 payment 業務 metric。
     * <p>
     * 🔴 <b>本方法不得遞增 {@code hawkr_payment_total} / {@code hawkr_payment_failed_total}</b>
     * （36 §3.3.1 / 65）：那兩個 Counter 餵 {@code business-alerts.yml} 的
     * {@code PaymentFailureSpike}，非 payment 事件若進入分母，付款失敗率會被稀釋
     * ⇒ 告警在最需要的時候不響。業務 metric 一律留在 {@link #enqueueAndDispatch} 這個
     * payment-specific wrapper 的 afterCommit 內。
     * <p>
     * 投遞時機由 caller 自行決定（註冊 afterCommit 即時投遞，或純 INSERT 交
     * {@code OutboxEventPublisherScheduler} 每 5s 兜底）——本方法刻意不替 caller 選，
     * 既有四條寫入路徑的 timing 語意因此不受 genericize 影響。
     *
     * 🔴 <b>必須在 active DB transaction 內呼叫</b>，否則在任何副作用之前 fail-fast。
     * 理由見方法內註解——這是 transactional outbox 的存在意義本身。
     *
     * @param exchange 目標 exchange，必填；null／空白由 {@link OutboxEventFactory} fail-fast
     * @return 已 insert 的 entity（id 已回填），供 caller 決定後續投遞
     * @throws IllegalStateException 不在 active DB transaction 內
     */
    public OutboxEventEntity enqueue(String exchange, String eventType, String routingKey, String payloadJson) {
        // 🔴 transaction boundary 閘門：必須在 factory construction／insert／任何副作用**之前**。
        //
        // transactional outbox 的不變式是「domain transition ＋ outbox INSERT 在同一個交易」：
        // 業務 rollback 時事件必須一起消失，否則會發出一個「沒有發生過的事」。
        // 少了這道閘門，未來的 caller 忘記開交易時，outbox 列會**獨立落庫**——
        // 而那個失敗是完全靜默的：insert 成功、scheduler 照常撈到、事件照常發出去。
        //
        // 🔴 用 isActualTransactionActive() 而**不是** isSynchronizationActive()：
        // 後者只表示「這條 thread 上有 synchronization 機制」，可以在沒有任何實際 DB 交易的
        // 情況下為 true（例如有人單獨呼叫 initSynchronization()，或 read-only 的
        // synchronization context）⇒ 拿它當閘門會放行真正危險的那一種情形。
        if (!TransactionSynchronizationManager.isActualTransactionActive()) {
            throw new IllegalStateException(
                    "outbox enqueue requires an active transaction"
                            + "（outbox INSERT 必須與觸發它的業務寫入在同一個交易內；"
                            + "exchange=" + exchange + " event_type=" + eventType
                            + " routing_key=" + routingKey + "）");
        }

        OutboxEventEntity entity = OutboxEventFactory.newPendingEntity(
                exchange, eventType, routingKey, payloadJson, DateTimeUtils.nowUtc());
        outboxMapper.insert(entity);
        return entity;
    }
}
