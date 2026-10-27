package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.LocalDateTime;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import org.junit.jupiter.api.Test;

class BillingQuoteCanonicalizerTest {

    @Test
    void canonicalJsonUsesFixedOrderAndUtcRfc3339Values() {
        BillingQuote quote = quote("ignored-hash");

        assertEquals(
                "{\"subject_type\":\"BUYER\",\"subject_id\":\"123456789\","
                        + "\"membership_id\":\"membership-1\",\"membership_change_request_id\":\"change-1\","
                        + "\"offer_id\":\"offer-1\",\"offer_version_no\":\"7\","
                        + "\"unit_amount_minor\":19900,\"currency\":\"TWD\","
                        + "\"interval_unit\":\"MONTH\",\"interval_count\":1,"
                        + "\"provider_account_ref\":\"payuni-platform-sandbox\","
                        + "\"first_period_date\":\"2026-09-02T00:00:00Z\","
                        + "\"expires_at\":\"2026-09-02T00:10:00Z\"}",
                BillingQuoteCanonicalizer.canonicalJson(quote));
    }

    @Test
    void sha256IsStableAndLowercaseHex() {
        BillingQuote quote = quote("ignored-hash");

        assertEquals(
                "0894ef1b9d2196b7b72a38e9c52c1a797bd7147b0c4d764d9fa53843ad846eb2",
                BillingQuoteCanonicalizer.sha256Hex(quote));
        assertEquals(64, BillingQuoteCanonicalizer.sha256Hex(quote).length());
        assertEquals(
                BillingQuoteCanonicalizer.sha256Hex(quote),
                BillingQuoteCanonicalizer.sha256Hex(quote).toLowerCase());
    }

    @Test
    void nullOrInvalidQuotesAreRejectedByCanonicalizer() {
        BillingQuote invalid = new BillingQuote(
                "q-1", null, "membership-1", "change-1", "offer-1", "7", 19900,
                "TWD", BillingIntervalUnit.MONTH, 1, "payuni-platform-sandbox",
                LocalDateTime.of(2026, 9, 2, 0, 0), LocalDateTime.of(2026, 9, 2, 0, 10), "");

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> BillingQuoteCanonicalizer.canonicalJson(invalid));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
    }

    private static BillingQuote quote(String snapshotHash) {
        return new BillingQuote(
                "q-1",
                new SubscriptionSubject(SubjectType.BUYER, "123456789"),
                "membership-1",
                "change-1",
                "offer-1",
                "7",
                19900,
                "TWD",
                BillingIntervalUnit.MONTH,
                1,
                "payuni-platform-sandbox",
                LocalDateTime.of(2026, 9, 2, 0, 0),
                LocalDateTime.of(2026, 9, 2, 0, 10),
                snapshotHash);
    }
}
