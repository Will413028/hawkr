package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;

/**
 * A billing subject is deliberately namespaced; an account id must never be guessed as a
 * buyer id or store id.
 */
public record SubscriptionSubject(SubjectType subjectType, String subjectId) {

    private static final String DECIMAL_TSID = "[1-9][0-9]*";

    public SubscriptionSubject {
        if (subjectType == null || subjectId == null || !subjectId.matches(DECIMAL_TSID)) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.BILLING_SUBJECT_INVALID,
                    "subject_type and a positive decimal subject_id are required");
        }
    }
}
