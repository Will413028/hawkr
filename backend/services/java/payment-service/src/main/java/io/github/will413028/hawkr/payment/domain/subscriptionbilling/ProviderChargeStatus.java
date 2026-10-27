package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Terminal status normalized from a provider recurring charge. */
public enum ProviderChargeStatus {
    SUCCEEDED,
    FAILED,
    CONFLICT
}
