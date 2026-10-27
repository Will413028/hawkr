package io.github.will413028.hawkr.payment.provider.payuni;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderActionResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionCreated;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAction;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAdapter;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderCommand;
import org.springframework.stereotype.Component;

/**
 * PayUni provider seam. The recurring API payload and lifecycle semantics still require a real
 * sandbox verification, so every side-effecting operation is deliberately disabled. Keeping a
 * bean in the graph makes the capability boundary explicit and prevents an unconfigured provider
 * from being accidentally selected by the shared application service.
 */
@Component
public final class PayUniSubscriptionProviderAdapter implements SubscriptionProviderAdapter {

    private static final String DISABLED_MESSAGE =
            "PayUni recurring adapter is disabled until the official sandbox contract is verified";

    @Override
    public String provider() {
        return PayUniRecurringWebhookNormalizer.PROVIDER;
    }

    @Override
    public ProviderSubscriptionCreated create(SubscriptionProviderCommand command) {
        throw unsupported();
    }

    @Override
    public ProviderActionResult suspend(SubscriptionProviderAction command) {
        throw unsupported();
    }

    @Override
    public ProviderActionResult restart(SubscriptionProviderAction command) {
        throw unsupported();
    }

    @Override
    public ProviderActionResult end(SubscriptionProviderAction command) {
        throw unsupported();
    }

    private static BillingValidationException unsupported() {
        return new BillingValidationException(
                MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED, DISABLED_MESSAGE);
    }
}
