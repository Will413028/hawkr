package io.github.will413028.hawkr.payment.provider.payuni;

import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAction;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderCommand;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

class PayUniSubscriptionProviderAdapterTest {

    private final PayUniSubscriptionProviderAdapter adapter = new PayUniSubscriptionProviderAdapter();

    @Test
    void recurringLifecycleIsFailClosedUntilSandboxContractIsVerified() {
        BillingValidationException create = assertThrows(BillingValidationException.class,
                () -> adapter.create(null));
        assertEquals("PROVIDER_CAPABILITY_UNSUPPORTED", create.errorCode().code());

        BillingValidationException suspend = assertThrows(BillingValidationException.class,
                () -> adapter.suspend((SubscriptionProviderAction) null));
        assertEquals("PROVIDER_CAPABILITY_UNSUPPORTED", suspend.errorCode().code());
    }

    @Test
    void providerIdentityIsPayUni() {
        assertEquals("PAYUNI", adapter.provider());
    }
}
