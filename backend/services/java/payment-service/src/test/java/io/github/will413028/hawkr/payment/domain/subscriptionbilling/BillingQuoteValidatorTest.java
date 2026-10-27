package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.LocalDateTime;
import java.util.Locale;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import org.junit.jupiter.api.Test;

class BillingQuoteValidatorTest {

    @Test
    void acceptsUnexpiredQuoteWhoseSnapshotHashMatches() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 2, 0, 0);
        BillingQuote unsigned = quote(now.plusMinutes(10), "");
        BillingQuote quote = withHash(unsigned, BillingQuoteCanonicalizer.sha256Hex(unsigned));

        assertDoesNotThrow(() -> BillingQuoteValidator.validate(quote, now));
    }

    @Test
    void rejectsExpiredQuote() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 2, 0, 11);
        BillingQuote unsigned = quote(now.minusMinutes(1), "");
        BillingQuote quote = withHash(unsigned, BillingQuoteCanonicalizer.sha256Hex(unsigned));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> BillingQuoteValidator.validate(quote, now));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
    }

    @Test
    void rejectsTamperedSnapshotHash() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 2, 0, 0);
        BillingQuote quote = quote(now.plusMinutes(10), "0000000000000000000000000000000000000000000000000000000000000000");

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> BillingQuoteValidator.validate(quote, now));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
    }

    @Test
    void rejectsUppercaseSnapshotHashSoDatabaseAndWireContractsStayCanonical() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 2, 0, 0);
        BillingQuote unsigned = quote(now.plusMinutes(10), "");
        String uppercase = BillingQuoteCanonicalizer.sha256Hex(unsigned).toUpperCase(Locale.ROOT);
        BillingQuote quote = withHash(unsigned, uppercase);

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> BillingQuoteValidator.validate(quote, now));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
    }

    @Test
    void rejectsNonPositiveAmountWrongCurrencyOrWrongInterval() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 2, 0, 0);
        BillingQuote unsigned = new BillingQuote(
                "q-1", new SubscriptionSubject(SubjectType.SELLER, "987654321"), "membership-1", "change-1",
                "offer-1", "7", 0, "tw", BillingIntervalUnit.MONTH, 0, "payuni-platform-sandbox",
                now, now.plusMinutes(10), "");

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> BillingQuoteValidator.validate(unsigned, now));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
    }

    private static BillingQuote quote(LocalDateTime expiresAt, String hash) {
        return new BillingQuote(
                "q-1", new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1",
                "offer-1", "7", 19900, "TWD", BillingIntervalUnit.MONTH, 1,
                "payuni-platform-sandbox", LocalDateTime.of(2026, 9, 2, 0, 0), expiresAt, hash);
    }

    private static BillingQuote withHash(BillingQuote quote, String hash) {
        return new BillingQuote(
                quote.quoteId(), quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(),
                quote.offerId(), quote.offerVersionNo(), quote.unitAmountMinor(), quote.currency(),
                quote.intervalUnit(), quote.intervalCount(), quote.providerAccountRef(), quote.firstPeriodDate(),
                quote.expiresAt(), hash);
    }
}
