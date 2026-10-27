package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.membership.billing.grpc.ProviderChargeResultData;
import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import com.google.protobuf.TypeRegistry;
import com.google.protobuf.util.JsonFormat;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/**
 * Transactional outbox writer for provider-result membership events.
 *
 * <p>It deliberately reuses payment-service's existing outbox table and publisher. The payload is
 * protobuf JSON with preserved snake_case names and a registered Any type; no second event wrapper
 * or direct RabbitTemplate publish is allowed.</p>
 */
@Component
public final class MembershipBillingOutboxWriter {

    private static final ObsLogger log = ObsLogger.forClass(MembershipBillingOutboxWriter.class);
    private static final TypeRegistry TYPE_REGISTRY = TypeRegistry.newBuilder()
            .add(ProviderChargeResultData.getDescriptor())
            .build();

    private final OutboxEventMapper outboxMapper;
    private final PaymentOutboxPublisher paymentOutboxPublisher;

    public MembershipBillingOutboxWriter(OutboxEventMapper outboxMapper,
                                         PaymentOutboxPublisher paymentOutboxPublisher) {
        this.outboxMapper = outboxMapper;
        this.paymentOutboxPublisher = paymentOutboxPublisher;
    }

    /** Must be called inside the same transaction as the charge-attempt insert. */
    public void enqueueAndDispatch(MembershipEventEnvelope envelope, String routingKey) {
        if (envelope == null || routingKey == null || routingKey.isBlank()) {
            throw new IllegalArgumentException("membership event envelope and routing key are required");
        }
        if (!TransactionSynchronizationManager.isSynchronizationActive()) {
            throw new IllegalStateException("membership billing outbox requires an active transaction");
        }
        String payloadJson = toJson(envelope);
        OutboxEventEntity entity = OutboxEventFactory.newPendingEntity(
                MqConstants.EXCHANGE_PAYMENT,
                envelope.getEventType(), routingKey, payloadJson, DateTimeUtils.nowUtc());
        outboxMapper.insert(entity);

        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                try {
                    paymentOutboxPublisher.publish(entity);
                } catch (Throwable throwable) {
                    log.error("MembershipBillingOutboxWriter.afterCommit.dispatchFailed",
                            "membership event dispatch failed; scheduler will retry outbox_id=" + entity.getId(),
                            throwable);
                }
            }
        });
    }

    static String toJson(MembershipEventEnvelope envelope) {
        try {
            return JsonFormat.printer()
                    .usingTypeRegistry(TYPE_REGISTRY)
                    .preservingProtoFieldNames()
                    .omittingInsignificantWhitespace()
                    .print(envelope);
        } catch (Exception e) {
            throw new IllegalStateException("membership event serialization failed", e);
        }
    }
}
