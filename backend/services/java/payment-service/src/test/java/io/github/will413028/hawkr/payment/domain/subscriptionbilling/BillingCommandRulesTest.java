package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import org.junit.jupiter.api.Test;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;

class BillingCommandRulesTest {

    @Test
    void firstCommandIsNew() {
        BillingCommandIdentity command = command("hash-a");

        assertEquals(
                BillingCommandDecision.NEW,
                BillingCommandRules.decide(null, command));
    }

    @Test
    void sameOperationKeyAndHashIsReplay() {
        BillingCommandIdentity existing = command("hash-a");
        BillingCommandIdentity incoming = command("hash-a");

        assertEquals(BillingCommandDecision.REPLAY, BillingCommandRules.decide(existing, incoming));
    }

    @Test
    void sameOperationKeyWithDifferentHashIsConflict() {
        BillingCommandIdentity existing = command("hash-a");
        BillingCommandIdentity incoming = command("hash-b");

        assertEquals(BillingCommandDecision.CONFLICT, BillingCommandRules.decide(existing, incoming));
    }

    @Test
    void operationAndIdempotencyKeyAreRequired() {
        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> new BillingCommandIdentity("", "request-1", "hash-a"));
        assertEquals(MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT, exception.errorCode());
    }

    private static BillingCommandIdentity command(String hash) {
        return new BillingCommandIdentity("CREATE_SUBSCRIPTION", "request-1", hash);
    }
}
