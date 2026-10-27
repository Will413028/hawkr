package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/** Provider-neutral create command. PayUni fields belong only in its adapter implementation. */
public record SubscriptionProviderCommand(
        BillingQuote quote,
        String membershipId,
        String membershipChangeRequestId,
        String merTradeNo,
        ProviderAccountProfile account,
        String idempotencyKey
) {

    /** Source-compatible constructor; new callers must pass the persisted provider idempotency key. */
    public SubscriptionProviderCommand(
            BillingQuote quote,
            String membershipId,
            String membershipChangeRequestId,
            String merTradeNo,
            ProviderAccountProfile account
    ) {
        this(quote, membershipId, membershipChangeRequestId, merTradeNo, account, "");
    }
}
