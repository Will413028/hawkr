package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/**
 * Canonical commercial snapshot serializer. Field order and numeric representation are part of
 * the contract; do not replace this with an unordered map or a floating-point serializer.
 */
public final class BillingQuoteCanonicalizer {

    private BillingQuoteCanonicalizer() {
    }

    public static String canonicalJson(BillingQuote quote) {
        if (quote == null || quote.subject() == null) {
            throw invalid("quote and subject are required");
        }

        StringBuilder json = new StringBuilder(512);
        json.append('{');
        appendString(json, "subject_type", quote.subject().subjectType().name());
        appendString(json, "subject_id", quote.subject().subjectId());
        appendString(json, "membership_id", required(quote.membershipId(), "membership_id"));
        appendString(json, "membership_change_request_id",
                required(quote.membershipChangeRequestId(), "membership_change_request_id"));
        appendString(json, "offer_id", required(quote.offerId(), "offer_id"));
        appendString(json, "offer_version_no", required(quote.offerVersionNo(), "offer_version_no"));
        appendNumber(json, "unit_amount_minor", quote.unitAmountMinor());
        appendString(json, "currency", required(quote.currency(), "currency"));
        appendString(json, "interval_unit",
                quote.intervalUnit() == null ? null : quote.intervalUnit().name());
        if (quote.intervalUnit() == null) {
            throw invalid("interval_unit is required");
        }
        appendNumber(json, "interval_count", quote.intervalCount());
        appendString(json, "provider_account_ref",
                required(quote.providerAccountRef(), "provider_account_ref"));
        appendString(json, "first_period_date", DateTimeUtils.formatIso8601(required(
                quote.firstPeriodDate(), "first_period_date")));
        appendString(json, "expires_at", DateTimeUtils.formatIso8601(required(
                quote.expiresAt(), "expires_at")));
        json.append('}');
        return json.toString();
    }

    public static String sha256Hex(BillingQuote quote) {
        byte[] bytes = canonicalJson(quote).getBytes(StandardCharsets.UTF_8);
        try {
            return HexFormatHolder.FORMAT.formatHex(MessageDigest.getInstance("SHA-256").digest(bytes));
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is required by the JVM", e);
        }
    }

    private static void appendString(StringBuilder json, String key, String value) {
        if (value == null) {
            throw invalid(key + " is required");
        }
        if (json.length() > 1) {
            json.append(',');
        }
        appendJsonString(json, key);
        json.append(':');
        appendJsonString(json, value);
    }

    private static void appendNumber(StringBuilder json, String key, long value) {
        if (json.length() > 1) {
            json.append(',');
        }
        appendJsonString(json, key);
        json.append(':').append(value);
    }

    private static void appendJsonString(StringBuilder json, String value) {
        json.append('"');
        for (int i = 0; i < value.length(); i++) {
            char character = value.charAt(i);
            switch (character) {
                case '"' -> json.append("\\\"");
                case '\\' -> json.append("\\\\");
                case '\b' -> json.append("\\b");
                case '\f' -> json.append("\\f");
                case '\n' -> json.append("\\n");
                case '\r' -> json.append("\\r");
                case '\t' -> json.append("\\t");
                default -> {
                    if (character < 0x20) {
                        json.append(String.format("\\u%04x", (int) character));
                    } else {
                        json.append(character);
                    }
                }
            }
        }
        json.append('"');
    }

    private static String required(String value, String field) {
        if (value == null || value.isBlank()) {
            throw invalid(field + " is required");
        }
        return value;
    }

    private static <T> T required(T value, String field) {
        if (value == null) {
            throw invalid(field + " is required");
        }
        return value;
    }

    private static BillingValidationException invalid(String message) {
        return new BillingValidationException(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, message);
    }

    private static final class HexFormatHolder {
        private static final java.util.HexFormat FORMAT = java.util.HexFormat.of();

        private HexFormatHolder() {
        }
    }
}
