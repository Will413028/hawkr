package io.github.will413028.hawkr.payment.provider.payuni;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.RecurringWebhookNormalizer;
import java.time.LocalDateTime;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.time.format.DateTimeParseException;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.function.Function;

/**
 * Maps the small, provider-specific recurring webhook vocabulary into the shared result contract.
 *
 * <p>The resolver is deliberately injected: a PayUni {@code MerID} is not silently treated as a
 * domain subject or as an account alias. Until the real sandbox payload and account mapping are
 * verified, the production adapter remains disabled and this mapper fails closed on ambiguity.</p>
 */
public final class PayUniRecurringWebhookNormalizer implements RecurringWebhookNormalizer {

    public static final String PROVIDER = "PAYUNI";
    public static final String DEFAULT_CURRENCY = "TWD";

    private final Function<String, String> accountRefResolver;
    private final String defaultCurrency;

    public PayUniRecurringWebhookNormalizer(Function<String, String> accountRefResolver,
                                             String defaultCurrency) {
        this.accountRefResolver = Objects.requireNonNull(accountRefResolver, "accountRefResolver");
        this.defaultCurrency = requireCurrency(defaultCurrency);
    }

    @Override
    public ProviderChargeResult normalize(Map<String, String> payload,
                                           String providerEventId,
                                           String providerEventKey) {
        if (payload == null || payload.isEmpty()) {
            throw conflict("recurring webhook payload is empty");
        }
        required(payload, "MerID");
        requiredValue(providerEventKey, "provider_event_key");

        String providerSubscriptionId = required(payload, "PeriodTradeNo");
        String periodOrderNo = required(payload, "PeriodOrderNo");
        int thisPeriod = parsePositiveInt(required(payload, "ThisPeriod"), "ThisPeriod");
        String tradeNo = required(payload, "TradeNo");
        ProviderChargeStatus status = parseStatus(required(payload, "TradeStatus"));

        String accountRef = accountRefResolver.apply(payload.get("MerID"));
        requiredValue(accountRef, "provider_account_ref");

        long amountMinor = parseAmount(payload.getOrDefault("TradeAmt", ""), status);
        LocalDateTime validFrom = parseUtc(payload.get("ValidFrom"), "ValidFrom");
        LocalDateTime validUntil = parseUtc(payload.get("ValidUntil"), "ValidUntil");
        validateBounds(validFrom, validUntil, "valid bounds");
        LocalDateTime cycleStartedAt = parseUtc(payload.get("CycleStartedAt"), "CycleStartedAt");
        LocalDateTime cycleUntil = parseUtc(payload.get("CycleUntil"), "CycleUntil");
        validateBounds(cycleStartedAt, cycleUntil, "cycle bounds");

        return new ProviderChargeResult(
                PROVIDER,
                accountRef,
                providerSubscriptionId,
                null,
                null,
                periodOrderNo,
                tradeNo,
                amountMinor,
                payload.getOrDefault("Currency", defaultCurrency),
                thisPeriod,
                status,
                payload.getOrDefault("ResCode", ""),
                firstNonBlank(payload.get("ResCodeMsg"), payload.get("Message")),
                validFrom,
                validUntil,
                cycleStartedAt,
                cycleUntil,
                parseInitialAuthorization(payload.get("InitialAuthorization")),
                payload.getOrDefault("BillingQuoteId", ""),
                payload.getOrDefault("QuoteSnapshotHash", ""),
                firstNonBlank(payload.get("EventId"), providerEventId),
                providerEventKey);
    }

    private static ProviderChargeStatus parseStatus(String raw) {
        return switch (raw.trim().toUpperCase(Locale.ROOT)) {
            case "1", "SUCCESS", "SUCCEEDED" -> ProviderChargeStatus.SUCCEEDED;
            case "2", "FAIL", "FAILED" -> ProviderChargeStatus.FAILED;
            default -> throw conflict("unsupported recurring TradeStatus: " + raw);
        };
    }

    private static long parseAmount(String raw, ProviderChargeStatus status) {
        if (raw == null || raw.isBlank()) {
            if (status == ProviderChargeStatus.FAILED) {
                return 0L;
            }
            throw conflict("successful recurring result must contain TradeAmt");
        }
        try {
            long value = Long.parseLong(raw.trim());
            if (value >= 0 && (status == ProviderChargeStatus.FAILED || value > 0)) {
                return value;
            }
        } catch (NumberFormatException ignored) {
            // Normalize all malformed provider numerics to one machine-readable error below.
        }
        throw conflict("TradeAmt must be a non-negative integer");
    }

    private static int parsePositiveInt(String raw, String field) {
        try {
            long value = Long.parseLong(raw.trim());
            if (value > 0 && value <= Integer.MAX_VALUE) {
                return (int) value;
            }
        } catch (NumberFormatException ignored) {
            // Normalize malformed provider numerics to one machine-readable error below.
        }
        throw conflict(field + " must be a positive integer");
    }

    private static LocalDateTime parseUtc(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        if (!raw.endsWith("Z")) {
            throw conflict(field + " must be RFC3339 UTC with Z suffix");
        }
        try {
            return OffsetDateTime.parse(raw, DateTimeFormatter.ISO_OFFSET_DATE_TIME)
                    .withOffsetSameInstant(ZoneOffset.UTC)
                    .toLocalDateTime();
        } catch (DateTimeParseException e) {
            throw conflict(field + " is not a valid RFC3339 timestamp");
        }
    }

    private static void validateBounds(LocalDateTime from, LocalDateTime until, String label) {
        if ((from == null) != (until == null)
                || (from != null && !until.isAfter(from))) {
            throw conflict(label + " must be complete and ordered");
        }
    }

    private static boolean parseInitialAuthorization(String raw) {
        if (raw == null || raw.isBlank()) {
            return false;
        }
        return switch (raw.trim().toLowerCase(Locale.ROOT)) {
            case "1", "true" -> true;
            case "0", "false" -> false;
            default -> throw conflict("InitialAuthorization must be 0/1 or true/false");
        };
    }

    private static String required(Map<String, String> payload, String field) {
        return requiredValue(payload.get(field), field);
    }

    private static String requiredValue(String value, String field) {
        if (value == null || value.isBlank()) {
            throw conflict(field + " is required");
        }
        return value.trim();
    }

    private static String firstNonBlank(String preferred, String fallback) {
        return preferred == null || preferred.isBlank() ? (fallback == null ? "" : fallback) : preferred;
    }

    private static String requireCurrency(String raw) {
        if (raw == null || !raw.matches("[A-Z]{3}")) {
            throw conflict("default currency must be an uppercase ISO 4217 code");
        }
        return raw;
    }

    private static BillingValidationException conflict(String message) {
        return new BillingValidationException(MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT, message);
    }
}
