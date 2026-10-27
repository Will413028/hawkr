package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.membership.billing.grpc.CreateMembershipSubscriptionRequest;
import io.github.will413028.hawkr.membership.billing.grpc.CreateMembershipSubscriptionResponse;
import io.github.will413028.hawkr.membership.billing.grpc.MembershipBillingServiceGrpc;
import io.github.will413028.hawkr.membership.billing.grpc.MembershipSubscriptionActionRequest;
import io.github.will413028.hawkr.membership.billing.grpc.MembershipSubscriptionActionResponse;
import io.github.will413028.hawkr.payment.application.service.MembershipBillingApplicationService;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuote;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.time.LocalDateTime;
import net.devh.boot.grpc.server.service.GrpcService;

/** Internal gRPC adapter for the buyer/seller shared billing plane. */
@GrpcService
public class MembershipBillingGrpcService
        extends MembershipBillingServiceGrpc.MembershipBillingServiceImplBase {

    private final MembershipBillingApplicationService applicationService;

    public MembershipBillingGrpcService(MembershipBillingApplicationService applicationService) {
        this.applicationService = applicationService;
    }

    @Override
    public void createMembershipSubscription(
            CreateMembershipSubscriptionRequest request,
            StreamObserver<CreateMembershipSubscriptionResponse> responseObserver
    ) {
        requireInternalCaller();
        if (request == null || !request.hasSubject() || !request.hasBillingQuote()) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    "subject and billing_quote are required");
        }
        if (request.getMembershipId().isBlank()
                || request.getMembershipChangeRequestId().isBlank()
                || request.getRequestId().isBlank()) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    "membership IDs and request_id are required");
        }
        SubscriptionSubject subject = toSubject(request.getSubject().getSubjectType(), request.getSubject().getSubjectId());
        BillingQuote quote = toQuote(request, subject);
        MembershipBillingApplicationService.CreateResult result = applicationService.create(
                new MembershipBillingApplicationService.CreateCommand(
                        subject,
                        request.getMembershipId(),
                        request.getMembershipChangeRequestId(),
                        quote,
                        request.getRequestId()));

        responseObserver.onNext(CreateMembershipSubscriptionResponse.newBuilder()
                .setPeriodSubscriptionId(result.periodSubscriptionId())
                .setProviderSubscriptionId(result.providerSubscriptionId())
                .setRedirectUrl(result.redirectUrl())
                .setMerTradeNo(result.merTradeNo())
                .setStatus(result.status())
                .build());
        responseObserver.onCompleted();
    }

    @Override
    public void suspendMembershipSubscription(
            MembershipSubscriptionActionRequest request,
            StreamObserver<MembershipSubscriptionActionResponse> responseObserver
    ) {
        responseObserver.onNext(actionResponse(
                applicationService.suspend(toAction(request))));
        responseObserver.onCompleted();
    }

    @Override
    public void restartMembershipSubscription(
            MembershipSubscriptionActionRequest request,
            StreamObserver<MembershipSubscriptionActionResponse> responseObserver
    ) {
        requireInternalCaller();
        responseObserver.onNext(actionResponse(
                applicationService.restart(toAction(request))));
        responseObserver.onCompleted();
    }

    @Override
    public void endMembershipSubscription(
            MembershipSubscriptionActionRequest request,
            StreamObserver<MembershipSubscriptionActionResponse> responseObserver
    ) {
        requireInternalCaller();
        responseObserver.onNext(actionResponse(
                applicationService.end(toAction(request))));
        responseObserver.onCompleted();
    }

    private MembershipBillingApplicationService.ActionCommand toAction(
            MembershipSubscriptionActionRequest request
    ) {
        requireInternalCaller();
        if (request == null || request.getPeriodSubscriptionId().isBlank() || request.getRequestId().isBlank()) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "period_subscription_id and request_id are required");
        }
        return new MembershipBillingApplicationService.ActionCommand(
                request.getPeriodSubscriptionId(), request.getRequestId(), request.getReason());
    }

    private static MembershipSubscriptionActionResponse actionResponse(
            MembershipBillingApplicationService.ActionResult result
    ) {
        return MembershipSubscriptionActionResponse.newBuilder()
                .setPeriodSubscriptionId(result.periodSubscriptionId())
                .setProviderSubscriptionId(result.providerSubscriptionId())
                .setStatus(result.status())
                .build();
    }

    private static SubscriptionSubject toSubject(String subjectTypeRaw, String subjectId) {
        try {
            return new SubscriptionSubject(SubjectType.valueOf(subjectTypeRaw), subjectId);
        } catch (IllegalArgumentException e) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBJECT_INVALID,
                    "subject_type must be BUYER or SELLER and subject_id must be a positive TSID");
        }
    }

    private static BillingQuote toQuote(
            CreateMembershipSubscriptionRequest request,
            SubscriptionSubject subject
    ) {
        io.github.will413028.hawkr.membership.billing.grpc.BillingQuote proto = request.getBillingQuote();
        long amount = parsePositiveLong(proto.getUnitAmountMinor(), "unit_amount_minor");
        int intervalCount = parsePositiveInt(proto.getIntervalCount(), "interval_count");
        BillingIntervalUnit intervalUnit;
        try {
            intervalUnit = BillingIntervalUnit.valueOf(proto.getIntervalUnit());
        } catch (IllegalArgumentException e) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, "interval_unit is invalid");
        }
        LocalDateTime firstPeriodDate = parseTimestamp(proto.getFirstPeriodDate(), "first_period_date");
        LocalDateTime expiresAt = parseTimestamp(proto.getExpiresAt(), "expires_at");
        return new BillingQuote(
                proto.getQuoteId(),
                subject,
                request.getMembershipId(),
                request.getMembershipChangeRequestId(),
                proto.getOfferId(),
                proto.getOfferVersionNo(),
                amount,
                proto.getCurrency(),
                intervalUnit,
                intervalCount,
                proto.getProviderAccountRef(),
                firstPeriodDate,
                expiresAt,
                proto.getSnapshotHash());
    }

    private static LocalDateTime parseTimestamp(String raw, String field) {
        // API contract is RFC3339 UTC with an explicit upper-case Z. DateTimeUtils intentionally
        // accepts legacy offset/no-zone forms for other integrations, so this boundary must be
        // strict and reject an offset before handing the value to the domain canonicalizer.
        if (raw == null || raw.isBlank() || !raw.endsWith("Z")) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    field + " must be RFC3339 UTC");
        }
        LocalDateTime value = DateTimeUtils.parseIso8601(raw);
        if (value == null) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    field + " must be RFC3339 UTC");
        }
        return value;
    }

    private static long parsePositiveLong(String raw, String field) {
        try {
            long value = Long.parseLong(raw);
            if (value > 0) {
                return value;
            }
        } catch (NumberFormatException ignored) {
            // Normalize malformed protobuf numerics to one domain error below.
        }
        throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, field + " must be positive");
    }

    private static int parsePositiveInt(String raw, String field) {
        long value = parsePositiveLong(raw, field);
        if (value > Integer.MAX_VALUE) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, field + " is out of range");
        }
        return (int) value;
    }

    private static void requireInternalCaller() {
        if (!GrpcAuthContext.isInternalCall()) {
            throw Status.PERMISSION_DENIED
                    .withDescription("Membership billing RPC is internal-only")
                    .asRuntimeException();
        }
    }

    private static BillingValidationException invalid(MembershipBillingErrorCode code, String message) {
        return new BillingValidationException(code, message);
    }
}
