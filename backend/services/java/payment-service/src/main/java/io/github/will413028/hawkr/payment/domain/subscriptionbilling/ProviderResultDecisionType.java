package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Outcome of applying first-terminal-result-wins to a provider period. */
public enum ProviderResultDecisionType {
    AUTHORITATIVE,
    DUPLICATE,
    CONFLICT
}
