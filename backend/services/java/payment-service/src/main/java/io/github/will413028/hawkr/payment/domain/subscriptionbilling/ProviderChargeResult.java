package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.time.LocalDateTime;

/** Provider-normalized recurring charge result; provider bounds are never recomputed here. */
public record ProviderChargeResult(
        String provider,
        String providerAccountRef,
        String providerSubscriptionId,
        String providerPeriodId,
        String periodSubscriptionId,
        String periodOrderNo,
        String tradeNo,
        long amountMinor,
        String currency,
        Integer thisPeriod,
        ProviderChargeStatus status,
        String resCode,
        String resCodeMsg,
        LocalDateTime validFrom,
        LocalDateTime validUntil,
        LocalDateTime cycleStartedAt,
        LocalDateTime cycleUntil,
        boolean initialAuthorization,
        String billingQuoteId,
        String quoteSnapshotHash,
        String providerEventId,
        String providerEventKey
) {
}
