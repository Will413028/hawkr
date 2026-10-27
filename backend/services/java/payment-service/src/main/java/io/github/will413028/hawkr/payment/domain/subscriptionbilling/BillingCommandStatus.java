package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Persisted command processing status. */
public enum BillingCommandStatus {
    PROCESSING,
    SUCCEEDED,
    FAILED,
    /** Provider outcome is unknown; the same idempotency key must be reconciled before retry. */
    UNKNOWN
}
