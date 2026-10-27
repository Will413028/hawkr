package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

/**
 * Provider adapter seam. Implementations own provider-specific pages, fields, errors and
 * capability checks; the billing application service never imports PayUni types. Every provider
 * implementation must honor the command/action idempotency key: retrying the same key after an
 * unknown network outcome must return the original provider result without creating a second
 * subscription or applying a lifecycle action twice.
 */
public interface SubscriptionProviderAdapter {

    String provider();

    ProviderSubscriptionCreated create(SubscriptionProviderCommand command);

    ProviderActionResult suspend(SubscriptionProviderAction command);

    ProviderActionResult restart(SubscriptionProviderAction command);

    ProviderActionResult end(SubscriptionProviderAction command);
}
