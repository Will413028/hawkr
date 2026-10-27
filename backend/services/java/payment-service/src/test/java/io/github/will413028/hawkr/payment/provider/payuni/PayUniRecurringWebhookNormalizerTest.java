package io.github.will413028.hawkr.payment.provider.payuni;

import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import java.util.HashMap;
import java.util.Map;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

class PayUniRecurringWebhookNormalizerTest {

    private final PayUniRecurringWebhookNormalizer normalizer =
            new PayUniRecurringWebhookNormalizer(merId -> "payuni-platform-sandbox", "TWD");

    @Test
    void normalizesDocumentedRecurringIdentityAndTerminalStatus() {
        Map<String, String> payload = new HashMap<>();
        payload.put("MerID", "HAWK9");
        payload.put("PeriodTradeNo", "PT001");
        payload.put("PeriodOrderNo", "M123-S1_1");
        payload.put("ThisPeriod", "1");
        payload.put("TradeNo", "P20260216003");
        payload.put("TradeStatus", "1");
        payload.put("TradeAmt", "990");

        var result = normalizer.normalize(payload, "provider-event-1", "HAWK9:PT001:P20260216003:1:1");

        assertEquals("PAYUNI", result.provider());
        assertEquals("payuni-platform-sandbox", result.providerAccountRef());
        assertEquals("PT001", result.providerSubscriptionId());
        assertEquals("M123-S1_1", result.periodOrderNo());
        assertEquals(1, result.thisPeriod());
        assertEquals(990L, result.amountMinor());
        assertEquals(ProviderChargeStatus.SUCCEEDED, result.status());
        assertEquals("provider-event-1", result.providerEventId());
    }

    @Test
    void rejectsUnknownTerminalStatus() {
        Map<String, String> payload = validPayload();
        payload.put("TradeStatus", "0");

        BillingValidationException exception = assertThrows(BillingValidationException.class,
                () -> normalizer.normalize(payload, "event", "key"));

        assertEquals("PROVIDER_RESULT_CONFLICT", exception.errorCode().code());
    }

    @Test
    void rejectsMissingProviderSubscriptionIdentity() {
        Map<String, String> payload = validPayload();
        payload.remove("PeriodTradeNo");

        assertThrows(BillingValidationException.class,
                () -> normalizer.normalize(payload, "event", "key"));
    }

    @Test
    void rejectsMalformedPositiveNumbersAndBounds() {
        Map<String, String> invalidPeriod = validPayload();
        invalidPeriod.put("ThisPeriod", "0");
        assertThrows(BillingValidationException.class,
                () -> normalizer.normalize(invalidPeriod, "event", "key"));

        Map<String, String> invalidAmount = validPayload();
        invalidAmount.put("TradeAmt", "not-a-number");
        assertThrows(BillingValidationException.class,
                () -> normalizer.normalize(invalidAmount, "event", "key"));
    }

    @Test
    void acceptsExplicitUtcBoundsOnly() {
        Map<String, String> payload = validPayload();
        payload.put("ValidFrom", "2026-09-03T00:00:00Z");
        payload.put("ValidUntil", "2026-10-03T00:00:00Z");

        var result = normalizer.normalize(payload, "event", "key");
        assertEquals("2026-09-03T00:00", result.validFrom().toString());
        assertEquals("2026-10-03T00:00", result.validUntil().toString());

        payload.put("ValidUntil", "2026-10-03T00:00:00+08:00");
        assertThrows(BillingValidationException.class,
                () -> normalizer.normalize(payload, "event", "key"));
    }

    private static Map<String, String> validPayload() {
        Map<String, String> payload = new HashMap<>();
        payload.put("MerID", "HAWK9");
        payload.put("PeriodTradeNo", "PT001");
        payload.put("PeriodOrderNo", "M123-S1_1");
        payload.put("ThisPeriod", "1");
        payload.put("TradeNo", "P20260216003");
        payload.put("TradeStatus", "1");
        payload.put("TradeAmt", "990");
        return payload;
    }
}
