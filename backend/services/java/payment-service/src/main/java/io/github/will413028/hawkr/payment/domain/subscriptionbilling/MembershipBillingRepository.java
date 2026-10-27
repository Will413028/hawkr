package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.util.Optional;

/** Persistence port for payment-service-owned shared billing facts. */
public interface MembershipBillingRepository {

    Optional<ProviderAccountProfile> findProviderAccount(String accountRef);

    Optional<BillingCommand> findCommand(String operation, String idempotencyKey);

    Optional<BillingCommand> findCreateCommandByQuoteId(String billingQuoteId);

    Optional<BillingCommand> findActiveCommandBySubscriptionId(long subscriptionId);

    /**
     * Atomically claims an idempotency key. A false result means another transaction already
     * owns the same operation/key; the caller must reload that row and replay or report conflict.
     */
    boolean insertCommand(BillingCommand command);

    /**
     * Binds a claimed create command to the subscription inserted by the same transaction.
     *
     * <p>Create commands claim idempotency before the subscription row exists. Keeping the
     * result reference nullable for that first insert preserves the immediate FK while the
     * transaction establishes the parent row, then this method completes the relationship.
     */
    void bindCommandSubscription(java.util.UUID commandId, long subscriptionId);

    /** Atomically claims an UNKNOWN command for exactly one reconciliation attempt. */
    boolean claimUnknownCommand(java.util.UUID commandId);

    /** Atomically reclaims a PROCESSING command whose provider-call lease has expired. */
    boolean claimStaleProcessingCommand(java.util.UUID commandId);

    void markCommandSucceeded(
            java.util.UUID commandId,
            long subscriptionId,
            String providerSubscriptionId,
            String redirectUrl,
            String merTradeNo,
            String status
    );

    void markCommandFailed(java.util.UUID commandId, String errorCode, String errorMessage);

    void markCommandUnknown(java.util.UUID commandId, String errorCode, String errorMessage);

    Optional<MembershipProviderSubscription> findSubscriptionByQuoteId(String quoteId);

    Optional<MembershipProviderSubscription> findSubscriptionById(long subscriptionId);

    Optional<MembershipProviderSubscription> findActiveSubscription(SubscriptionSubject subject);

    int nextSubscriptionSequence(SubscriptionSubject subject);

    void insertSubscription(MembershipProviderSubscription subscription);

    void attachProviderSubscription(long subscriptionId, ProviderSubscriptionCreated created);

    void updateSubscriptionStatus(long subscriptionId, ProviderSubscriptionStatus status);

    Optional<MembershipProviderSubscription> findSubscriptionByProviderIdentity(
            String provider, String providerSubscriptionId);

    Optional<MembershipChargeAttempt> findChargeAttemptByProviderEventKey(String providerEventKey);

    Optional<MembershipChargeAttempt> findAuthoritativeChargeAttempt(
            long providerSubscriptionRef, String providerPeriodId, String periodOrderNo);

    void insertChargeAttempt(MembershipChargeAttempt attempt);
}
