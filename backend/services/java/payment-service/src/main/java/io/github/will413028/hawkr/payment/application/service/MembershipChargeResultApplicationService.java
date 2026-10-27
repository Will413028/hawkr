package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipChargeAttempt;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultDecision;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultDecisionType;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultAdjudicator;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultConflictHandler;
import io.github.will413028.hawkr.payment.infrastructure.mq.MembershipBillingOutboxWriter;
import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import java.time.LocalDateTime;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Applies a normalized provider result to the payment-side append-only ledger.
 *
 * <p>The subscription row is locked by the repository before adjudication. This serializes
 * competing notifications for one provider subscription while the database unique keys protect
 * retries across processes. Only the authoritative winner creates an outbox event.</p>
 */
@Service
public class MembershipChargeResultApplicationService {

    private final MembershipBillingRepository repository;
    private final MembershipBillingOutboxWriter outboxWriter;
    private final ProviderResultConflictHandler conflictHandler;
    private final Supplier<LocalDateTime> nowUtc;
    private final Supplier<Long> idGenerator;

    @Autowired
    public MembershipChargeResultApplicationService(MembershipBillingRepository repository,
                                                    MembershipBillingOutboxWriter outboxWriter,
                                                    ProviderResultConflictHandler conflictHandler) {
        this(repository, outboxWriter, conflictHandler, DateTimeUtils::nowUtc, TsidUtil::generate);
    }

    public MembershipChargeResultApplicationService(MembershipBillingRepository repository,
                                                    MembershipBillingOutboxWriter outboxWriter,
                                                    Supplier<LocalDateTime> nowUtc,
                                                    Supplier<Long> idGenerator) {
        this(repository, outboxWriter, (subscription, authoritative, conflicting) -> { }, nowUtc, idGenerator);
    }

    public MembershipChargeResultApplicationService(MembershipBillingRepository repository,
                                                    MembershipBillingOutboxWriter outboxWriter,
                                                    ProviderResultConflictHandler conflictHandler,
                                                    Supplier<LocalDateTime> nowUtc,
                                                    Supplier<Long> idGenerator) {
        this.repository = repository;
        this.outboxWriter = outboxWriter;
        this.conflictHandler = conflictHandler;
        this.nowUtc = nowUtc;
        this.idGenerator = idGenerator;
    }

    @Transactional
    public ApplyResult apply(ProviderChargeResult incoming) {
        // Validate provider identity before touching the database. This also rejects a caller
        // attempting to inject a pre-adjudicated CONFLICT as a domain transition.
        validateResult(incoming);

        MembershipProviderSubscription subscription = repository
                .findSubscriptionByProviderIdentity(incoming.provider(), incoming.providerSubscriptionId())
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                        "provider subscription identity is not registered"));
        ProviderChargeResult normalized = bindToSubscription(subscription, incoming);

        Optional<MembershipChargeAttempt> sameProviderEvent = repository
                .findChargeAttemptByProviderEventKey(normalized.providerEventKey());
        if (sameProviderEvent.isPresent()) {
            if (sameProviderEvent.get().result().status() == ProviderChargeStatus.CONFLICT) {
                return new ApplyResult(ProviderResultDecisionType.DUPLICATE, subscription,
                        sameProviderEvent.get().result(), sameProviderEvent.get().canonicalEventId(), null, null);
            }
            ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(
                    sameProviderEvent.get().result(), normalized);
            if (decision.type() == ProviderResultDecisionType.DUPLICATE) {
                return new ApplyResult(ProviderResultDecisionType.DUPLICATE, subscription,
                        sameProviderEvent.get().result(), sameProviderEvent.get().canonicalEventId(), null, null);
            }
            // provider_event_key is itself unique. A contradictory payload reusing that key is
            // not appendable without destroying the exact dedup contract, so fail closed and send
            // it to the provider reconciliation/DLQ path.
            notifyConflict(subscription, sameProviderEvent.get().result(), normalized);
            throw invalid(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider event key was reused with a different terminal result");
        }

        Optional<MembershipChargeAttempt> authoritative = repository.findAuthoritativeChargeAttempt(
                subscription.id(), normalized.providerPeriodId(), normalized.periodOrderNo());
        ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(
                authoritative.map(MembershipChargeAttempt::result).orElse(null), normalized);

        boolean winner = decision.type() == ProviderResultDecisionType.AUTHORITATIVE;
        ProviderChargeResult persistedResult = winner
                ? normalized
                : decision.type() == ProviderResultDecisionType.CONFLICT
                ? withStatus(normalized, ProviderChargeStatus.CONFLICT)
                : normalized;
        // This is the wire event_id, not the semantic deduplication key. Generate it once for
        // the authoritative attempt and persist it with the append-only row; duplicate provider
        // notifications reload the same value. The provider-period hash remains the separate
        // first-terminal-wins/idempotency identity.
        String canonicalEventId = winner ? UUID.randomUUID().toString() : null;
        MembershipChargeAttempt attempt = new MembershipChargeAttempt(
                nextId(), subscription.id(), subscription.subject(), subscription.membershipId(),
                subscription.membershipChangeRequestId(), persistedResult, winner, canonicalEventId, nowUtc.get());
        repository.insertChargeAttempt(attempt);

        if (!winner) {
            notifyConflict(subscription, authoritative.map(MembershipChargeAttempt::result).orElse(null), normalized);
            return new ApplyResult(decision.type(), subscription, persistedResult, null, null, null);
        }

        MembershipEventEnvelope event = MembershipBillingEventFactory.build(
                subscription, normalized, canonicalEventId, attempt.receivedAt());
        String route = routeFor(subscription, normalized.status());
        outboxWriter.enqueueAndDispatch(event, route);
        return new ApplyResult(decision.type(), subscription, persistedResult, canonicalEventId, event, route);
    }

    private long nextId() {
        Long value = idGenerator.get();
        if (value == null || value <= 0) {
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "charge attempt id generator returned an invalid value");
        }
        return value;
    }

    private static void validateResult(ProviderChargeResult result) {
        ProviderResultAdjudicator.adjudicate(null, result);
        if (result.providerEventKey() == null || result.providerEventKey().isBlank()) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider event key is required for exact dedup");
        }
    }

    private static ProviderChargeResult bindToSubscription(
            MembershipProviderSubscription subscription,
            ProviderChargeResult incoming
    ) {
        if (!subscription.provider().equals(incoming.provider())
                || !subscription.providerSubscriptionId().equals(incoming.providerSubscriptionId())
                || !subscription.providerAccountRef().equals(incoming.providerAccountRef())) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "provider result identity does not match the payment subscription");
        }
        if (incoming.periodSubscriptionId() != null && !incoming.periodSubscriptionId().isBlank()
                && !String.valueOf(subscription.id()).equals(incoming.periodSubscriptionId())) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "period_subscription_id does not match the provider subscription");
        }
        if (incoming.billingQuoteId() != null && !incoming.billingQuoteId().isBlank()
                && !subscription.billingQuoteId().equals(incoming.billingQuoteId())) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider result quote id does not match the payment snapshot");
        }
        if (incoming.quoteSnapshotHash() != null && !incoming.quoteSnapshotHash().isBlank()
                && !subscription.quoteSnapshotHash().equalsIgnoreCase(incoming.quoteSnapshotHash())) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider result quote hash does not match the payment snapshot");
        }
        if ((incoming.amountMinor() != 0 && incoming.amountMinor() != subscription.unitAmountMinor())
                || (incoming.status() == ProviderChargeStatus.SUCCEEDED && incoming.amountMinor() <= 0)) {
            throw invalid(MembershipBillingErrorCode.BILLING_AMOUNT_MISMATCH,
                    "provider amount does not match the immutable subscription snapshot");
        }
        if (!subscription.currency().equals(incoming.currency())) {
            throw invalid(MembershipBillingErrorCode.BILLING_AMOUNT_MISMATCH,
                    "provider currency does not match the immutable subscription snapshot");
        }
        return new ProviderChargeResult(
                incoming.provider(), incoming.providerAccountRef(), incoming.providerSubscriptionId(),
                incoming.providerPeriodId(), String.valueOf(subscription.id()), incoming.periodOrderNo(),
                incoming.tradeNo(), incoming.amountMinor(), incoming.currency(), incoming.thisPeriod(),
                incoming.status(), incoming.resCode(), incoming.resCodeMsg(), incoming.validFrom(),
                incoming.validUntil(), incoming.cycleStartedAt(), incoming.cycleUntil(),
                incoming.initialAuthorization(), subscription.billingQuoteId(),
                subscription.quoteSnapshotHash(), incoming.providerEventId(), incoming.providerEventKey());
    }

    private static ProviderChargeResult withStatus(ProviderChargeResult result, ProviderChargeStatus status) {
        return new ProviderChargeResult(
                result.provider(), result.providerAccountRef(), result.providerSubscriptionId(),
                result.providerPeriodId(), result.periodSubscriptionId(), result.periodOrderNo(), result.tradeNo(),
                result.amountMinor(), result.currency(), result.thisPeriod(), status, result.resCode(),
                result.resCodeMsg(), result.validFrom(), result.validUntil(), result.cycleStartedAt(),
                result.cycleUntil(), result.initialAuthorization(), result.billingQuoteId(),
                result.quoteSnapshotHash(), result.providerEventId(), result.providerEventKey());
    }

    private static String routeFor(MembershipProviderSubscription subscription, ProviderChargeStatus status) {
        boolean buyer = subscription.subject().subjectType().name().equals("BUYER");
        if (buyer && status == ProviderChargeStatus.SUCCEEDED) {
            return MqConstants.ROUTING_KEY_PAYMENT_MEMBERSHIP_BUYER_SUCCESS;
        }
        if (buyer) {
            return MqConstants.ROUTING_KEY_PAYMENT_MEMBERSHIP_BUYER_FAILED;
        }
        if (status == ProviderChargeStatus.SUCCEEDED) {
            return MqConstants.ROUTING_KEY_PAYMENT_MEMBERSHIP_SELLER_SUCCESS;
        }
        return MqConstants.ROUTING_KEY_PAYMENT_MEMBERSHIP_SELLER_FAILED;
    }

    private static BillingValidationException invalid(MembershipBillingErrorCode code, String message) {
        return new BillingValidationException(code, message);
    }

    private void notifyConflict(MembershipProviderSubscription subscription,
                                ProviderChargeResult authoritative,
                                ProviderChargeResult conflicting) {
        try {
            conflictHandler.onConflict(subscription, authoritative, conflicting);
        } catch (RuntimeException ignored) {
            // Alerting/reconciliation must not make an already persisted conflict disappear or
            // turn a provider retry into a second transition.
        }
    }

    /** Result returned to the MQ consumer; only AUTHORITATIVE carries an event to publish. */
    public record ApplyResult(
            ProviderResultDecisionType decisionType,
            MembershipProviderSubscription subscription,
            ProviderChargeResult persistedResult,
            String canonicalEventId,
            MembershipEventEnvelope event,
            String routingKey
    ) {
    }

}
