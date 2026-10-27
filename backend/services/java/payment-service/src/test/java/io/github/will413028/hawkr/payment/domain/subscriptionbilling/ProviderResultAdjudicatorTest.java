package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.LocalDateTime;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import org.junit.jupiter.api.Test;

class ProviderResultAdjudicatorTest {

    @Test
    void firstTerminalResultBecomesAuthoritative() {
        ProviderChargeResult incoming = result(ProviderChargeStatus.SUCCEEDED, "trade-1");

        ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(null, incoming);

        assertEquals(ProviderResultDecisionType.AUTHORITATIVE, decision.type());
        assertEquals(incoming, decision.result());
    }

    @Test
    void exactRetryOfAuthoritativeResultIsDuplicateAndDoesNotTransitionAgain() {
        ProviderChargeResult existing = result(ProviderChargeStatus.SUCCEEDED, "trade-1");
        ProviderChargeResult retry = result(ProviderChargeStatus.SUCCEEDED, "trade-1");

        ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(existing, retry);

        assertEquals(ProviderResultDecisionType.DUPLICATE, decision.type());
        assertEquals(existing, decision.result());
    }

    @Test
    void oppositeTerminalResultIsConflictAndExistingWinnerRemains() {
        ProviderChargeResult existing = result(ProviderChargeStatus.FAILED, "trade-1");
        ProviderChargeResult contradictory = result(ProviderChargeStatus.SUCCEEDED, "trade-2");

        ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(existing, contradictory);

        assertEquals(ProviderResultDecisionType.CONFLICT, decision.type());
        assertEquals(existing, decision.result());
    }

    @Test
    void changedProviderPeriodOrdinalIsAConflictEvenWhenOtherTermsMatch() {
        ProviderChargeResult existing = result(ProviderChargeStatus.SUCCEEDED, "trade-1");
        ProviderChargeResult changedOrdinal = new ProviderChargeResult(
                existing.provider(), existing.providerAccountRef(), existing.providerSubscriptionId(),
                existing.providerPeriodId(), existing.periodSubscriptionId(), existing.periodOrderNo(),
                existing.tradeNo(), existing.amountMinor(), existing.currency(), 2, existing.status(),
                existing.resCode(), existing.resCodeMsg(), existing.validFrom(), existing.validUntil(),
                existing.cycleStartedAt(), existing.cycleUntil(), existing.initialAuthorization(),
                existing.billingQuoteId(), existing.quoteSnapshotHash(), "event-2", "event-key-2");

        ProviderResultDecision decision = ProviderResultAdjudicator.adjudicate(existing, changedOrdinal);

        assertEquals(ProviderResultDecisionType.CONFLICT, decision.type());
    }

    @Test
    void providerBoundsArePreservedAndMustBeOrdered() {
        ProviderChargeResult incoming = result(ProviderChargeStatus.SUCCEEDED, "trade-1");
        assertEquals(LocalDateTime.of(2026, 9, 2, 0, 0), incoming.validFrom());
        assertEquals(LocalDateTime.of(2026, 10, 2, 0, 0), incoming.validUntil());

        ProviderChargeResult invalid = new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", "ps-1", "period-1", "subscription-1", "period-order-1",
                "trade-1", 19900, "TWD", 1, ProviderChargeStatus.SUCCEEDED, "00", "ok",
                LocalDateTime.of(2026, 10, 2, 0, 0), LocalDateTime.of(2026, 9, 2, 0, 0),
                LocalDateTime.of(2026, 9, 2, 0, 0), LocalDateTime.of(2026, 10, 2, 0, 0), true,
                "q-1", "hash", "event-1", "event-key-1");

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> ProviderResultAdjudicator.adjudicate(null, invalid));
        assertEquals(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT, exception.errorCode());
    }

    private static ProviderChargeResult result(ProviderChargeStatus status, String tradeNo) {
        return new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", "ps-1", "period-1", "subscription-1", "period-order-1",
                tradeNo, 19900, "TWD", 1, status, "00", "ok",
                LocalDateTime.of(2026, 9, 2, 0, 0), LocalDateTime.of(2026, 10, 2, 0, 0),
                LocalDateTime.of(2026, 9, 2, 0, 0), LocalDateTime.of(2026, 10, 2, 0, 0), true,
                "q-1", "hash", "event-1", "event-key-1");
    }
}
