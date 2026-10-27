package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Composition boundary for provider adapters.
 *
 * <p>The billing application service depends on this registry instead of a single PayUni bean, so
 * adding another provider does not require changing lifecycle or idempotency code. Provider codes
 * are case-sensitive contract values and duplicate registrations fail during application startup.
 */
public final class SubscriptionProviderRegistry {

    private final Map<String, SubscriptionProviderAdapter> adapters;

    public SubscriptionProviderRegistry(List<? extends SubscriptionProviderAdapter> providers) {
        Map<String, SubscriptionProviderAdapter> registered = new LinkedHashMap<>();
        if (providers != null) {
            for (SubscriptionProviderAdapter adapter : providers) {
                if (adapter == null || adapter.provider() == null || adapter.provider().isBlank()) {
                    throw unsupported("provider adapter must declare a provider code");
                }
                if (registered.putIfAbsent(adapter.provider(), adapter) != null) {
                    throw unsupported("duplicate provider adapter: " + adapter.provider());
                }
            }
        }
        this.adapters = Collections.unmodifiableMap(registered);
    }

    public SubscriptionProviderAdapter require(String provider) {
        SubscriptionProviderAdapter adapter = provider == null ? null : adapters.get(provider);
        if (adapter == null) {
            throw unsupported("no adapter is registered for provider " + provider);
        }
        return adapter;
    }

    private static BillingValidationException unsupported(String message) {
        return new BillingValidationException(
                MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED, message);
    }
}
