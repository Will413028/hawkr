package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import java.util.List;
import org.junit.jupiter.api.Test;

class SubscriptionProviderRegistryTest {

    @Test
    void resolvesProviderByStableProviderCode() {
        SubscriptionProviderAdapter payUni = new StubAdapter("PAYUNI");

        SubscriptionProviderRegistry registry = new SubscriptionProviderRegistry(List.of(payUni));

        assertEquals(payUni, registry.require("PAYUNI"));
    }

    @Test
    void duplicateProviderCodesAreRejectedAtCompositionBoundary() {
        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> new SubscriptionProviderRegistry(List.of(
                        new StubAdapter("PAYUNI"), new StubAdapter("PAYUNI"))));

        assertEquals(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED, exception.errorCode());
    }

    private static final class StubAdapter implements SubscriptionProviderAdapter {
        private final String provider;

        private StubAdapter(String provider) {
            this.provider = provider;
        }

        @Override
        public String provider() {
            return provider;
        }

        @Override
        public ProviderSubscriptionCreated create(SubscriptionProviderCommand command) {
            return null;
        }

        @Override
        public ProviderActionResult suspend(SubscriptionProviderAction command) {
            return null;
        }

        @Override
        public ProviderActionResult restart(SubscriptionProviderAction command) {
            return null;
        }

        @Override
        public ProviderActionResult end(SubscriptionProviderAction command) {
            return null;
        }
    }
}
