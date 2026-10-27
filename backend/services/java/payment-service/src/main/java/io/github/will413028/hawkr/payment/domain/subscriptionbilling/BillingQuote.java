package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.time.LocalDateTime;

/**
 * Immutable commercial snapshot issued by a buyer/seller control plane.
 *
 * <p>The record intentionally does not recalculate price or entitlement. Validation is kept
 * in {@link BillingQuoteValidator} so adapters can distinguish malformed input from an
 * already persisted immutable snapshot.</p>
 */
public record BillingQuote(
        String quoteId,
        SubscriptionSubject subject,
        String membershipId,
        String membershipChangeRequestId,
        String offerId,
        String offerVersionNo,
        long unitAmountMinor,
        String currency,
        BillingIntervalUnit intervalUnit,
        int intervalCount,
        String providerAccountRef,
        LocalDateTime firstPeriodDate,
        LocalDateTime expiresAt,
        String snapshotHash
) {
}
