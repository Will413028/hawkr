package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.LocalDateTime;

/** Validates an immutable domain-issued quote before provider execution. */
public final class BillingQuoteValidator {

    // Hashes are persisted in lowercase and included in the canonical contract. Accepting
    // uppercase here would make a quote valid in Java but fail the PostgreSQL CHECK constraint.
    private static final String SHA256_HEX = "[0-9a-f]{64}";
    private static final String CURRENCY = "[A-Z]{3}";

    private BillingQuoteValidator() {
    }

    public static void validate(BillingQuote quote) {
        validate(quote, DateTimeUtils.nowUtc());
    }

    public static void validate(BillingQuote quote, LocalDateTime nowUtc) {
        if (quote == null || nowUtc == null) {
            throw invalid("quote and validation time are required");
        }
        required(quote.quoteId(), "quote_id");
        if (quote.subject() == null) {
            throw invalid("subject is required");
        }
        required(quote.membershipId(), "membership_id");
        required(quote.membershipChangeRequestId(), "membership_change_request_id");
        required(quote.offerId(), "offer_id");
        required(quote.offerVersionNo(), "offer_version_no");
        if (quote.unitAmountMinor() <= 0) {
            throw invalid("unit_amount_minor must be positive");
        }
        if (quote.currency() == null || !quote.currency().matches(CURRENCY)) {
            throw invalid("currency must be an ISO 4217 uppercase code");
        }
        if (quote.intervalUnit() == null || quote.intervalCount() <= 0) {
            throw invalid("interval_unit and positive interval_count are required");
        }
        required(quote.providerAccountRef(), "provider_account_ref");
        if (quote.firstPeriodDate() == null || quote.expiresAt() == null
                || !quote.expiresAt().isAfter(nowUtc)) {
            throw invalid("first_period_date and an unexpired expires_at are required");
        }
        if (quote.snapshotHash() == null || !quote.snapshotHash().matches(SHA256_HEX)) {
            throw invalid("snapshot_hash must be a SHA-256 hex string");
        }

        String expected = BillingQuoteCanonicalizer.sha256Hex(quote);
        if (!MessageDigest.isEqual(
                expected.getBytes(StandardCharsets.US_ASCII),
                quote.snapshotHash().getBytes(StandardCharsets.US_ASCII))) {
            throw invalid("snapshot_hash does not match the canonical quote");
        }
    }

    private static void required(String value, String field) {
        if (value == null || value.isBlank()) {
            throw invalid(field + " is required");
        }
    }

    private static BillingValidationException invalid(String message) {
        return new BillingValidationException(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, message);
    }
}
