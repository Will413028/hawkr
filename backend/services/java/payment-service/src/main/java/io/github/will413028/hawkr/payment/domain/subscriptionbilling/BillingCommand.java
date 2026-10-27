package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.util.UUID;

/** Read model of a shared billing command idempotency row. */
public record BillingCommand(
        UUID id,
        String operation,
        SubscriptionSubject subject,
        String idempotencyKey,
        String requestHash,
        String billingQuoteId,
        Long resultSubscriptionId,
        BillingCommandStatus status,
        String errorCode,
        String errorMessage,
        String resultProviderSubscriptionId,
        String resultRedirectUrl,
        String resultMerTradeNo,
        String resultStatus,
        String actionReason
) {

    /** Source-compatible constructor for command rows written before response snapshots existed. */
    public BillingCommand(
            UUID id,
            String operation,
            SubscriptionSubject subject,
            String idempotencyKey,
            String requestHash,
            String billingQuoteId,
            Long resultSubscriptionId,
            BillingCommandStatus status,
            String errorCode
    ) {
        this(id, operation, subject, idempotencyKey, requestHash, billingQuoteId, resultSubscriptionId, status,
                errorCode, null, null, null, null, null, null);
    }
}
