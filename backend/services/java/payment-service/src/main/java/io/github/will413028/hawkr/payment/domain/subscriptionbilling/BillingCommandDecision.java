package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Outcome of looking up an operation-scoped idempotency key. */
public enum BillingCommandDecision {
    NEW,
    REPLAY,
    CONFLICT
}
