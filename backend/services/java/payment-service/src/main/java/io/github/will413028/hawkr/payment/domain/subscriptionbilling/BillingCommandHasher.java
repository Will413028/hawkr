package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/** Builds deterministic hashes for the idempotency row without including the idempotency key. */
public final class BillingCommandHasher {

    private BillingCommandHasher() {
    }

    public static String hashCreate(
            SubscriptionSubject subject,
            String membershipId,
            String membershipChangeRequestId,
            BillingQuote quote
    ) {
        String payload = SubscriptionOperation.CREATE_SUBSCRIPTION.value() + "|"
                + subject.subjectType().name() + "|" + subject.subjectId() + "|"
                + membershipId + "|" + membershipChangeRequestId + "|"
                + BillingQuoteCanonicalizer.canonicalJson(quote);
        return sha256(payload);
    }

    public static String hashAction(
            SubscriptionOperation operation,
            String subscriptionId,
            String reason
    ) {
        return sha256(operation.value() + "|" + subscriptionId + "|" + (reason == null ? "" : reason));
    }

    /** Shared deterministic SHA-256 helper for stable event identities as well as commands. */
    public static String sha256(String payload) {
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256")
                    .digest(payload.getBytes(StandardCharsets.UTF_8));
            return java.util.HexFormat.of().formatHex(digest);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is required by the JVM", e);
        }
    }
}
