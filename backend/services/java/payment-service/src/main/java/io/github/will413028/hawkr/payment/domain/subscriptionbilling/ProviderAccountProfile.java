package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

/** Provider account alias and adapter routing metadata; raw credentials never enter this type. */
public record ProviderAccountProfile(
        String provider,
        String accountRef,
        String environment,
        String credentialRef,
        String capabilitiesJson,
        String status
) {

    private static final ObjectMapper JSON = new ObjectMapper();

    public boolean isActive() {
        return "ACTIVE".equals(status);
    }

    /**
     * Capability metadata is an allowlist, not a default. Malformed or non-boolean entries fail
     * closed so a deployment cannot accidentally enable a provider operation by omission.
     */
    public boolean supports(String capability) {
        if (capability == null || capability.isBlank() || capabilitiesJson == null
                || capabilitiesJson.isBlank()) {
            return false;
        }
        try {
            JsonNode root = JSON.readTree(capabilitiesJson);
            JsonNode value = root == null ? null : root.get(capability);
            return root != null && root.isObject() && value != null && value.isBoolean()
                    && value.booleanValue();
        } catch (Exception ignored) {
            return false;
        }
    }
}
