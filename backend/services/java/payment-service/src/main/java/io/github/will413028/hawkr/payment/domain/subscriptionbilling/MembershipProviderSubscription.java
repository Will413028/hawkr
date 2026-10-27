package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.time.LocalDateTime;

/** Read model of payment-service-owned provider subscription facts. */
public record MembershipProviderSubscription(
        long id,
        SubscriptionSubject subject,
        String membershipId,
        String membershipChangeRequestId,
        String billingQuoteId,
        String subscriptionKind,
        String providerAccountRef,
        String provider,
        String providerSubscriptionId,
        int subscriptionSeq,
        String merTradeNo,
        String offerId,
        String offerVersionNo,
        String quoteSnapshotHash,
        LocalDateTime quoteExpiresAt,
        String currency,
        long unitAmountMinor,
        String intervalUnit,
        int intervalCount,
        LocalDateTime firstPeriodDate,
        LocalDateTime providerAnchorAt,
        ProviderSubscriptionStatus status
) {
}
