package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;

/** Canonical identity used by the shared billing command idempotency boundary. */
public record BillingCommandIdentity(String operation, String idempotencyKey, String requestHash) {

    public BillingCommandIdentity {
        if (operation == null || operation.isBlank()
                || idempotencyKey == null || idempotencyKey.isBlank()
                || requestHash == null || requestHash.isBlank()) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT,
                    "operation, idempotency_key and request_hash are required");
        }
    }
}
