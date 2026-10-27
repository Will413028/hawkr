package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.UUID;

/**
 * §2A.6 / §5.10.1 Outbox 事件組裝集中處。
 * <p>
 * 集中原分散於 {@code PaymentOutboxWriter}、{@code PaymentEventPublisher}、
 * {@code DuplicatePaymentService}、{@code RescueRefundConsumer} 的兩段重複組裝：
 * <ul>
 *   <li>{@link #applyEnvelopeMetadata}：36 §7.4 Conformance Contract envelope
 *       metadata（{@code event_id} 為 UUID v4 必含欄位，另含 {@code event_type} /
 *       {@code aggregate_type} / {@code aggregate_id} / {@code occurred_at}）；</li>
 *   <li>{@link #newPendingEntity}：{@code outbox_events} row 骨架
 *       （{@code exchange} 必填、{@code status=PENDING}、{@code retry_count=0}）。</li>
 * </ul>
 * 純函式、無狀態；不負責 {@code insert} 與投遞策略（afterCommit / 純 INSERT 由各 caller 決定）。
 */
public final class OutboxEventFactory {

    private OutboxEventFactory() {
    }

    /**
     * 將 36 §7.4 envelope metadata 就地補進 payload map。
     * {@code event_id} 每次產生新 UUID v4；其餘四欄由 caller 提供，語意與各事件既有值一致。
     */
    public static void applyEnvelopeMetadata(Map<String, Object> payload,
                                             String eventType,
                                             String aggregateType,
                                             String aggregateId,
                                             String occurredAt) {
        payload.put("event_id", UUID.randomUUID().toString());
        payload.put("event_type", eventType);
        payload.put("aggregate_type", aggregateType);
        payload.put("aggregate_id", aggregateId);
        payload.put("occurred_at", occurredAt);
    }

    /**
     * 建立 {@code PENDING} 狀態的 outbox_events entity 骨架（不 insert）。
     * {@code status=PENDING}、{@code retry_count=0}；其餘欄位由 caller 決定。
     *
     * @param exchange 本列要投遞到的 exchange，<b>必填</b>（36 §3.3.1 explicit persisted exchange）。
     *                 刻意設為必填參數而非 setter：漏傳就是<b>編譯錯</b>，不會靜默落 DB DEFAULT。
     *                 null／空白一律 fail-fast，讓壞列連 insert 都到不了
     *                 （publisher 端另有 fail-closed 守衛，兩層都要，見 {@code PaymentOutboxPublisher}）。
     */
    public static OutboxEventEntity newPendingEntity(String exchange,
                                                     String eventType,
                                                     String routingKey,
                                                     String payloadJson,
                                                     LocalDateTime createdAt) {
        // isBlank() 涵蓋 space／tab／CR／LF 等全部 whitespace，與 V11 的 CHECK (exchange ~ '\S') 同語意。
        if (exchange == null || exchange.isBlank()) {
            throw new IllegalArgumentException(
                    "outbox exchange 為必填且不得為空白（event_type=" + eventType
                            + " routing_key=" + routingKey + "）");
        }
        OutboxEventEntity entity = new OutboxEventEntity();
        entity.setExchange(exchange);
        entity.setEventType(eventType);
        entity.setRoutingKey(routingKey);
        entity.setPayload(payloadJson);
        entity.setStatus(OutboxEventStatus.PENDING.value());
        entity.setRetryCount(0);
        entity.setCreatedAt(createdAt);
        return entity;
    }
}
