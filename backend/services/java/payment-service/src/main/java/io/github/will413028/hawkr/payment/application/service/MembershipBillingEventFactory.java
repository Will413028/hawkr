package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.common.utils.code.MembershipEventType;
import io.github.will413028.hawkr.membership.billing.grpc.ProviderChargeResultData;
import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import io.github.will413028.hawkr.membership.grpc.SubjectRef;
import com.google.protobuf.Any;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import java.time.LocalDateTime;
import java.util.UUID;

/** Builds the single canonical membership event envelope emitted by payment-service. */
public final class MembershipBillingEventFactory {

    private MembershipBillingEventFactory() {
    }

    public static MembershipEventEnvelope build(
            MembershipProviderSubscription subscription,
            ProviderChargeResult result,
            String canonicalEventId,
            LocalDateTime occurredAt
    ) {
        if (subscription == null || result == null || occurredAt == null
                || result.status() == null || result.status() == ProviderChargeStatus.CONFLICT) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "a canonical event requires a subscription, terminal result and event time");
        }
        if (!isUuidV4(canonicalEventId)) {
            throw new BillingValidationException(
                    MembershipBillingErrorCode.PROVIDER_RESULT_CONFLICT,
                    "canonical event id must be a UUID v4");
        }

        String eventType = eventType(subscription, result.status());
        LocalDateTime effectiveAt = result.validFrom() == null ? occurredAt : result.validFrom();
        ProviderChargeResultData data = ProviderChargeResultData.newBuilder()
                .setProvider(nullToEmpty(result.provider()))
                .setProviderAccountRef(nullToEmpty(result.providerAccountRef()))
                .setProviderSubscriptionId(nullToEmpty(result.providerSubscriptionId()))
                .setProviderPeriodId(nullToEmpty(result.providerPeriodId()))
                .setPeriodSubscriptionId(firstNonBlank(result.periodSubscriptionId(), String.valueOf(subscription.id())))
                .setPeriodOrderNo(nullToEmpty(result.periodOrderNo()))
                .setTradeNo(nullToEmpty(result.tradeNo()))
                .setAmountMinor(String.valueOf(result.amountMinor()))
                .setCurrency(firstNonBlank(result.currency(), subscription.currency()))
                .setThisPeriod(result.thisPeriod() == null ? "" : String.valueOf(result.thisPeriod()))
                .setStatus(result.status().name())
                .setResCode(nullToEmpty(result.resCode()))
                .setResCodeMsg(nullToEmpty(result.resCodeMsg()))
                .setValidFrom(DateTimeUtils.formatIso8601(result.validFrom()))
                .setValidUntil(DateTimeUtils.formatIso8601(result.validUntil()))
                .setCycleStartedAt(DateTimeUtils.formatIso8601(result.cycleStartedAt()))
                .setCycleUntil(DateTimeUtils.formatIso8601(result.cycleUntil()))
                .setIsInitialAuthorization(result.initialAuthorization())
                .setBillingQuoteId(firstNonBlank(result.billingQuoteId(), subscription.billingQuoteId()))
                .setQuoteSnapshotHash(firstNonBlank(result.quoteSnapshotHash(), subscription.quoteSnapshotHash()))
                .build();

        return MembershipEventEnvelope.newBuilder()
                .setEventId(canonicalEventId)
                .setEventType(eventType)
                .setSchemaVersion("1")
                .setProducer("payment-service")
                .setSubject(SubjectRef.newBuilder()
                        .setSubjectType(subscription.subject().subjectType().name())
                        .setSubjectId(subscription.subject().subjectId())
                        .build())
                .setMembershipId(subscription.membershipId())
                // payment-service deliberately does not own or invent a domain period id
                .setMembershipPeriodId("")
                .setOfferId(subscription.offerId())
                .setEffectiveAt(DateTimeUtils.formatIso8601(effectiveAt))
                .setOccurredAt(DateTimeUtils.formatIso8601(occurredAt))
                .setIdempotencyKey(nullToEmpty(result.providerEventKey()))
                .setData(Any.pack(data))
                .setMembershipChangeRequestId(nullToEmpty(subscription.membershipChangeRequestId()))
                .build();
    }

    private static String eventType(MembershipProviderSubscription subscription, ProviderChargeStatus status) {
        boolean buyer = subscription.subject().subjectType().name().equals("BUYER");
        if (buyer && status == ProviderChargeStatus.SUCCEEDED) {
            return MembershipEventType.PAYMENT_MEMBERSHIP_BUYER_SUCCESS.value();
        }
        if (buyer) {
            return MembershipEventType.PAYMENT_MEMBERSHIP_BUYER_FAILED.value();
        }
        if (status == ProviderChargeStatus.SUCCEEDED) {
            return MembershipEventType.PAYMENT_MEMBERSHIP_SELLER_SUCCESS.value();
        }
        return MembershipEventType.PAYMENT_MEMBERSHIP_SELLER_FAILED.value();
    }

    private static String firstNonBlank(String value, String fallback) {
        return value == null || value.isBlank() ? nullToEmpty(fallback) : value;
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    private static boolean isUuidV4(String value) {
        if (value == null || value.isBlank()) {
            return false;
        }
        try {
            return UUID.fromString(value).version() == 4;
        } catch (IllegalArgumentException ignored) {
            return false;
        }
    }
}
