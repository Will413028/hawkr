package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultConflictHandler;
import org.springframework.stereotype.Component;

/** Default observability hook; reconciliation automation can replace or decorate it later. */
@Component
public final class MembershipBillingConflictAlert implements ProviderResultConflictHandler {

    private static final ObsLogger log = ObsLogger.forClass(MembershipBillingConflictAlert.class);

    @Override
    public void onConflict(MembershipProviderSubscription subscription,
                           ProviderChargeResult authoritative,
                           ProviderChargeResult conflicting) {
        String providerEventKey = conflicting == null ? "" : conflicting.providerEventKey();
        String period = conflicting == null ? "" : firstNonBlank(
                conflicting.providerPeriodId(), conflicting.periodOrderNo());
        String message = "contradictory provider terminal result; subscription_id="
                + (subscription == null ? "" : subscription.id())
                + " period=" + period
                + " authoritative_status=" + (authoritative == null ? "" : authoritative.status())
                + " conflicting_status=" + (conflicting == null ? "" : conflicting.status());
        log.error("MembershipBillingConflictAlert.providerResultConflict",
                message, "provider_event_key", providerEventKey);
    }

    private static String firstNonBlank(String preferred, String fallback) {
        return preferred == null || preferred.isBlank()
                ? (fallback == null ? "" : fallback) : preferred;
    }
}
