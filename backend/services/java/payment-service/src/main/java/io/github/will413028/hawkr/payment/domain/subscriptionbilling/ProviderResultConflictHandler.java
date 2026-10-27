package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/**
 * Side-effect hook for contradictory provider notifications.
 *
 * <p>The charge-attempt ledger remains the audit source of truth; an implementation may page,
 * increment metrics, or enqueue a reconciliation command, but it must never mutate the winning
 * attempt or grant entitlement.</p>
 */
@FunctionalInterface
public interface ProviderResultConflictHandler {

    void onConflict(MembershipProviderSubscription subscription,
                    ProviderChargeResult authoritative,
                    ProviderChargeResult conflicting);
}
