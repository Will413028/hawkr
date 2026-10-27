package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.time.LocalDateTime;

/** Append-only payment-side record of one normalized provider notification. */
public record MembershipChargeAttempt(
        long id,
        long providerSubscriptionRef,
        SubscriptionSubject subject,
        String membershipId,
        String membershipChangeRequestId,
        ProviderChargeResult result,
        boolean authoritative,
        String canonicalEventId,
        LocalDateTime receivedAt
) {
}
