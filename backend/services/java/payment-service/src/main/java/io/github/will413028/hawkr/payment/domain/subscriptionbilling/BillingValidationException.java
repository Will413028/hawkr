package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;

/**
 * Domain-facing error for shared membership billing validation.
 *
 * <p>The generated CodeMeta enum is the source of truth for the machine symbol. This
 * exception keeps pure domain rules independent from gRPC and persistence concerns.</p>
 */
public final class BillingValidationException extends IllegalArgumentException {

    private final MembershipBillingErrorCode errorCode;

    public BillingValidationException(MembershipBillingErrorCode errorCode, String message) {
        super(message);
        this.errorCode = errorCode;
    }

    public MembershipBillingErrorCode errorCode() {
        return errorCode;
    }
}
