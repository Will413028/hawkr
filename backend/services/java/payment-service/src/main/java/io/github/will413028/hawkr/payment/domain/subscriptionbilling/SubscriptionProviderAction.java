package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Provider-neutral lifecycle action command. */
public record SubscriptionProviderAction(
        MembershipProviderSubscription subscription,
        SubscriptionOperation operation,
        String reason,
        String idempotencyKey
) {

    /** Source-compatible constructor; new callers must pass the persisted provider idempotency key. */
    public SubscriptionProviderAction(
            MembershipProviderSubscription subscription,
            SubscriptionOperation operation,
            String reason
    ) {
        this(subscription, operation, reason, "");
    }
}
