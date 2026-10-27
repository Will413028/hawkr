package io.github.will413028.hawkr.payment.adapter.grpc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.payment.application.service.MembershipBillingApplicationService;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuote;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuoteCanonicalizer;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import io.github.will413028.hawkr.membership.billing.grpc.BillingQuote.Builder;
import io.github.will413028.hawkr.membership.billing.grpc.CreateMembershipSubscriptionRequest;
import io.github.will413028.hawkr.membership.billing.grpc.CreateMembershipSubscriptionResponse;
import io.github.will413028.hawkr.membership.billing.grpc.MembershipSubscriptionActionRequest;
import io.github.will413028.hawkr.membership.billing.grpc.MembershipSubscriptionActionResponse;
import io.github.will413028.hawkr.membership.grpc.SubjectRef;
import com.google.protobuf.Any;
import com.google.rpc.ErrorInfo;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.protobuf.StatusProto;
import io.grpc.stub.StreamObserver;
import java.time.LocalDateTime;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;

class MembershipBillingGrpcServiceTest {

    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 3, 0, 0);

    @Mock
    private MembershipBillingApplicationService applicationService;
    @Mock
    private StreamObserver<CreateMembershipSubscriptionResponse> createObserver;
    @Mock
    private StreamObserver<MembershipSubscriptionActionResponse> actionObserver;

    private AutoCloseable mocks;
    private MembershipBillingGrpcService service;

    @BeforeEach
    void setUp() {
        mocks = MockitoAnnotations.openMocks(this);
        service = new MembershipBillingGrpcService(applicationService);
    }

    @AfterEach
    void tearDown() throws Exception {
        mocks.close();
    }

    @Test
    void externalCallerCannotUseInternalBillingRpc() {
        StatusRuntimeException exception = assertThrows(
                StatusRuntimeException.class,
                () -> service.createMembershipSubscription(validRequest(), createObserver));
        assertEquals(Status.Code.PERMISSION_DENIED, exception.getStatus().getCode());
    }

    @Test
    void mapsServerIssuedQuoteAndReturnsPaymentSideReference() {
        when(applicationService.create(any())).thenReturn(
                new MembershipBillingApplicationService.CreateResult(
                        "9001", "provider-sub-1", "https://checkout.test", "M123456789-S1", "PENDING"));
        Context context = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
        Context previous = context.attach();
        try {
            service.createMembershipSubscription(validRequest(), createObserver);
        } finally {
            context.detach(previous);
        }

        verify(applicationService).create(any(MembershipBillingApplicationService.CreateCommand.class));
        verify(createObserver).onNext(CreateMembershipSubscriptionResponse.newBuilder()
                .setPeriodSubscriptionId("9001")
                .setProviderSubscriptionId("provider-sub-1")
                .setRedirectUrl("https://checkout.test")
                .setMerTradeNo("M123456789-S1")
                .setStatus("PENDING")
                .build());
    }

    @Test
    void actionMapsIdempotencyKeyAndReason() {
        when(applicationService.suspend(any())).thenReturn(
                new MembershipBillingApplicationService.ActionResult("9001", "provider-sub-1", "SUSPENDED"));
        Context context = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
        Context previous = context.attach();
        try {
            service.suspendMembershipSubscription(
                    MembershipSubscriptionActionRequest.newBuilder()
                            .setPeriodSubscriptionId("9001")
                            .setRequestId("suspend-1")
                            .setReason("failed charge")
                            .build(),
                    actionObserver);
        } finally {
            context.detach(previous);
        }
        verify(applicationService).suspend(any(MembershipBillingApplicationService.ActionCommand.class));
        verify(actionObserver).onNext(MembershipSubscriptionActionResponse.newBuilder()
                .setPeriodSubscriptionId("9001")
                .setProviderSubscriptionId("provider-sub-1")
                .setStatus("SUSPENDED")
                .build());
    }

    @Test
    void rejectsNonUtcOffsetTimestampBeforeCallingApplicationService() {
        Context context = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
        Context previous = context.attach();
        try {
            CreateMembershipSubscriptionRequest request = validRequest().toBuilder()
                    .setBillingQuote(validRequest().getBillingQuote().toBuilder()
                            .setFirstPeriodDate("2026-09-03T08:00:00+08:00")
                            .build())
                    .build();
            assertThrows(BillingValidationException.class,
                    () -> service.createMembershipSubscription(request, createObserver));
        } finally {
            context.detach(previous);
        }
        org.mockito.Mockito.verifyNoInteractions(applicationService);
    }

    @Test
    void billingErrorUsesStandardGrpcErrorInfoReason() {
        BillingValidationException domainError = new BillingValidationException(
                io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                "quote expired");

        StatusRuntimeException wireError = new GrpcExceptionAdvice()
                .handleBillingValidationException(domainError);
        com.google.rpc.Status status = StatusProto.fromThrowable(wireError);

        assertEquals(Status.Code.INVALID_ARGUMENT.value(), status.getCode());
        assertEquals("quote expired", status.getMessage());
        ErrorInfo info = status.getDetailsList().stream()
                .filter(detail -> detail.is(ErrorInfo.class))
                .findFirst()
                .map(detail -> unpackErrorInfo(detail))
                .orElseThrow();
        assertEquals("BILLING_QUOTE_INVALID", info.getReason());
        assertEquals("hawkr.membership.billing", info.getDomain());
    }

    private static ErrorInfo unpackErrorInfo(Any detail) {
        try {
            return detail.unpack(ErrorInfo.class);
        } catch (com.google.protobuf.InvalidProtocolBufferException e) {
            throw new AssertionError(e);
        }
    }

    private static CreateMembershipSubscriptionRequest validRequest() {
        BillingQuote unsigned = new BillingQuote(
                "quote-1", new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1",
                "offer-1", "7", 19900, "TWD", BillingIntervalUnit.MONTH, 1,
                "payuni-platform-sandbox", NOW, NOW.plusMinutes(10), "");
        String hash = BillingQuoteCanonicalizer.sha256Hex(unsigned);
        Builder quote = io.github.will413028.hawkr.membership.billing.grpc.BillingQuote.newBuilder()
                .setQuoteId(unsigned.quoteId())
                .setOfferId(unsigned.offerId())
                .setOfferVersionNo(unsigned.offerVersionNo())
                .setUnitAmountMinor(String.valueOf(unsigned.unitAmountMinor()))
                .setCurrency(unsigned.currency())
                .setIntervalUnit(unsigned.intervalUnit().name())
                .setIntervalCount(String.valueOf(unsigned.intervalCount()))
                .setProviderAccountRef(unsigned.providerAccountRef())
                .setFirstPeriodDate("2026-09-03T00:00:00Z")
                .setExpiresAt("2026-09-03T00:10:00Z")
                .setSnapshotHash(hash);
        return CreateMembershipSubscriptionRequest.newBuilder()
                .setSubject(SubjectRef.newBuilder().setSubjectType("BUYER").setSubjectId("123456789"))
                .setMembershipId("membership-1")
                .setMembershipChangeRequestId("change-1")
                .setBillingQuote(quote)
                .setRequestId("request-1")
                .build();
    }
}
