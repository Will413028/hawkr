package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class ProviderAccountProfileTest {

    @Test
    void capabilityAllowlistRequiresBooleanTrue() {
        ProviderAccountProfile profile = new ProviderAccountProfile(
                "PAYUNI", "sandbox", "SANDBOX", "secret://ref",
                "{\"RECURRING_CHARGE\":true,\"SUSPEND\":false,\"END\":\"true\"}", "ACTIVE");

        assertTrue(profile.supports("RECURRING_CHARGE"));
        assertFalse(profile.supports("SUSPEND"));
        assertFalse(profile.supports("END"));
        assertFalse(profile.supports("UNKNOWN"));
    }

    @Test
    void malformedCapabilitiesFailClosed() {
        ProviderAccountProfile profile = new ProviderAccountProfile(
                "PAYUNI", "sandbox", "SANDBOX", "secret://ref", "not-json", "ACTIVE");

        assertFalse(profile.supports("RECURRING_CHARGE"));
    }
}
