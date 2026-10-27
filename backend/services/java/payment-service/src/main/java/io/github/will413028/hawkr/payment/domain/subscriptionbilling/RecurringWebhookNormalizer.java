package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.util.Map;

/** Converts provider-specific webhook fields into the shared billing result contract. */
public interface RecurringWebhookNormalizer {

    ProviderChargeResult normalize(Map<String, String> payload, String providerEventId,
                                    String providerEventKey);
}
