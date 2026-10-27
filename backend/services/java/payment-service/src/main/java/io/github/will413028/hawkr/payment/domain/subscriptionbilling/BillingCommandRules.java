package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;

/** Pure rules for the unique (operation, idempotency_key) command boundary. */
public final class BillingCommandRules {

    private BillingCommandRules() {
    }

    public static BillingCommandDecision decide(
            BillingCommandIdentity existing,
            BillingCommandIdentity incoming
    ) {
        if (incoming == null) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT,
                    "incoming billing command is required");
        }
        if (existing == null) {
            return BillingCommandDecision.NEW;
        }
        if (!existing.operation().equals(incoming.operation())
                || !existing.idempotencyKey().equals(incoming.idempotencyKey())) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT,
                    "command lookup scope does not match the stored identity");
        }
        return existing.requestHash().equals(incoming.requestHash())
                ? BillingCommandDecision.REPLAY
                : BillingCommandDecision.CONFLICT;
    }
}
