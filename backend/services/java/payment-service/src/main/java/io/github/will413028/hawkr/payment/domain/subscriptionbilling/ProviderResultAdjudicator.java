package io.github.will413028.hawkr.payment.domain.subscriptionbilling;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import java.time.LocalDateTime;
import java.util.Objects;

/**
 * Pure first-terminal-result-wins adjudication. Persistence decides how to append the losing
 * row; this class only decides whether a domain transition may be driven.
 */
public final class ProviderResultAdjudicator {

    private static final String CURRENCY = "[A-Z]{3}";

    private ProviderResultAdjudicator() {
    }

    public static ProviderResultDecision adjudicate(
            ProviderChargeResult existingAuthoritative,
            ProviderChargeResult incoming
    ) {
        validate(incoming);
        if (existingAuthoritative == null) {
            return new ProviderResultDecision(ProviderResultDecisionType.AUTHORITATIVE, incoming);
        }
        validate(existingAuthoritative);
        if (!sameSubscription(existingAuthoritative, incoming)
                || !samePeriod(existingAuthoritative, incoming)) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "provider result does not belong to the same subscription period");
        }
        if (sameTerminalFingerprint(existingAuthoritative, incoming)) {
            return new ProviderResultDecision(ProviderResultDecisionType.DUPLICATE, existingAuthoritative);
        }
        return new ProviderResultDecision(ProviderResultDecisionType.CONFLICT, existingAuthoritative);
    }

    private static void validate(ProviderChargeResult result) {
        if (result == null || blank(result.provider()) || blank(result.providerSubscriptionId())
                || (blank(result.providerPeriodId()) && blank(result.periodOrderNo()))
                || result.status() == null || result.status() == ProviderChargeStatus.CONFLICT) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider result identity and a SUCCEEDED/FAILED terminal status are required");
        }
        if (result.amountMinor() < 0 || result.currency() == null || !result.currency().matches(CURRENCY)) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "provider amount and currency are invalid");
        }
        validateBounds(result.validFrom(), result.validUntil(), "valid bounds");
        validateBounds(result.cycleStartedAt(), result.cycleUntil(), "cycle bounds");
    }

    private static void validateBounds(LocalDateTime from, LocalDateTime until, String label) {
        if ((from == null) != (until == null)
                || (from != null && !until.isAfter(from))) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    label + " must be complete and ordered");
        }
    }

    private static boolean sameSubscription(ProviderChargeResult left, ProviderChargeResult right) {
        return Objects.equals(left.provider(), right.provider())
                && Objects.equals(left.providerSubscriptionId(), right.providerSubscriptionId());
    }

    private static boolean samePeriod(ProviderChargeResult left, ProviderChargeResult right) {
        boolean periodIdMatches = !blank(left.providerPeriodId()) && !blank(right.providerPeriodId())
                && left.providerPeriodId().equals(right.providerPeriodId());
        boolean orderMatches = !blank(left.periodOrderNo()) && !blank(right.periodOrderNo())
                && left.periodOrderNo().equals(right.periodOrderNo());
        return periodIdMatches || orderMatches;
    }

    private static boolean sameTerminalFingerprint(ProviderChargeResult left, ProviderChargeResult right) {
        return left.status() == right.status()
                && Objects.equals(left.providerAccountRef(), right.providerAccountRef())
                && Objects.equals(left.providerPeriodId(), right.providerPeriodId())
                && Objects.equals(left.periodOrderNo(), right.periodOrderNo())
                && Objects.equals(left.thisPeriod(), right.thisPeriod())
                && Objects.equals(left.tradeNo(), right.tradeNo())
                && left.amountMinor() == right.amountMinor()
                && Objects.equals(left.currency(), right.currency())
                && Objects.equals(left.validFrom(), right.validFrom())
                && Objects.equals(left.validUntil(), right.validUntil())
                && Objects.equals(left.cycleStartedAt(), right.cycleStartedAt())
                && Objects.equals(left.cycleUntil(), right.cycleUntil())
                && left.initialAuthorization() == right.initialAuthorization()
                && Objects.equals(left.billingQuoteId(), right.billingQuoteId())
                && Objects.equals(left.quoteSnapshotHash(), right.quoteSnapshotHash());
    }

    private static boolean blank(String value) {
        return value == null || value.isBlank();
    }
}
