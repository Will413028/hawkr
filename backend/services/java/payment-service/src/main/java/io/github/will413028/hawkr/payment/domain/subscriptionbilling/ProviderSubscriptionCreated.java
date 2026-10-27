package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.time.LocalDateTime;

/** Normalized provider response for initial hosted checkout/subscription creation. */
public record ProviderSubscriptionCreated(
        String providerSubscriptionId,
        String redirectUrl,
        String status,
        LocalDateTime firstPeriodDate,
        LocalDateTime providerAnchorAt
) {
}
