package io.github.will413028.hawkr.payment.application.service;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommand;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandHasher;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuote;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuoteCanonicalizer;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderAccountProfile;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderActionResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionCreated;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionOperation;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAdapter;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderCommand;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionStatus;
import java.time.LocalDateTime;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;

class MembershipBillingApplicationServiceTest {

    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 3, 0, 0);

    @Mock
    private MembershipBillingRepository repository;
    @Mock
    private SubscriptionProviderAdapter adapter;

    private MembershipBillingApplicationService service;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        when(adapter.provider()).thenReturn("PAYUNI");
        when(repository.findProviderAccount("payuni-platform-sandbox"))
                .thenReturn(Optional.of(new ProviderAccountProfile(
                        "PAYUNI", "payuni-platform-sandbox", "SANDBOX", "secret://payuni",
                        "{\"RECURRING_CHARGE\":true,\"SUSPEND\":true,\"RESTART\":true,\"END\":true}", "ACTIVE")));
        when(repository.nextSubscriptionSequence(any())).thenReturn(1);
        when(repository.insertCommand(any())).thenReturn(true);
        service = new MembershipBillingApplicationService(repository, adapter, () -> NOW, () -> 1L);
    }

    @Test
    void createsPendingSubscriptionFromServerIssuedQuote() {
        BillingQuote quote = quote();
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenReturn(new ProviderSubscriptionCreated("provider-sub-1", "https://payuni.test/checkout", "PENDING", null, null));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("1", result.periodSubscriptionId());
        assertEquals("provider-sub-1", result.providerSubscriptionId());
        assertEquals("PENDING", result.status());
        verify(repository).insertSubscription(any(MembershipProviderSubscription.class));
        verify(repository).markCommandSucceeded(any(), anyLong(), any(), any(), any(), any());
    }

    @Test
    void claimsCommandBeforeSubscriptionAndBindsResultAfterParentInsert() {
        BillingQuote quote = quote();
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenReturn(new ProviderSubscriptionCreated("provider-sub-1", "https://payuni.test/checkout", "PENDING", null, null));

        service.create(new MembershipBillingApplicationService.CreateCommand(
                quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-order"));

        ArgumentCaptor<BillingCommand> commandCaptor = ArgumentCaptor.forClass(BillingCommand.class);
        InOrder order = org.mockito.Mockito.inOrder(repository);
        order.verify(repository).insertCommand(commandCaptor.capture());
        order.verify(repository).insertSubscription(any(MembershipProviderSubscription.class));
        order.verify(repository).bindCommandSubscription(commandCaptor.getValue().id(), 1L);
        assertNull(commandCaptor.getValue().resultSubscriptionId());
    }

    @Test
    void activeProviderSubscriptionMustHaveProviderIdentity() {
        BillingQuote quote = quote();
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenReturn(new ProviderSubscriptionCreated("", "", "ACTIVE", null, null));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));

        assertEquals(MembershipBillingErrorCode.PROVIDER_REJECTED, exception.errorCode());
        verify(repository).markCommandFailed(any(), org.mockito.ArgumentMatchers.eq(
                MembershipBillingErrorCode.PROVIDER_REJECTED.code()), any());
        verify(adapter).create(any());
    }

    @Test
    void sameIdempotencyKeyAndHashReplaysWithoutCallingProviderAgain() {
        BillingQuote quote = quote();
        BillingCommand existing = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.SUCCEEDED, null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));
        when(repository.findSubscriptionById(55L)).thenReturn(Optional.of(subscription(55L)));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("55", result.periodSubscriptionId());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void idempotentCreateReplayReturnsTheOriginalHostedCheckoutSnapshot() {
        BillingQuote quote = quote();
        BillingCommand existing = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.SUCCEEDED, null, null,
                "provider-sub-1", "https://payuni.test/checkout", "M123456789-S1", "PENDING", null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("https://payuni.test/checkout", result.redirectUrl());
        assertEquals("provider-sub-1", result.providerSubscriptionId());
        assertEquals("M123456789-S1", result.merTradeNo());
        assertEquals("PENDING", result.status());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void providerTransportFailureLeavesCommandUnknownAndSubscriptionRetryable() {
        BillingQuote quote = quote();
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenThrow(new RuntimeException("connection reset"));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));

        assertEquals(MembershipBillingErrorCode.BILLING_UNAVAILABLE, exception.errorCode());
        verify(repository).markCommandUnknown(any(), org.mockito.ArgumentMatchers.eq(
                MembershipBillingErrorCode.BILLING_UNAVAILABLE.code()), any());
        verify(repository, org.mockito.Mockito.never()).markCommandFailed(any(), any(), any());
        verify(repository, org.mockito.Mockito.never()).updateSubscriptionStatus(anyLong(), any());
    }

    @Test
    void unknownCreateRetryMustWinAnAtomicReconciliationClaimBeforeProviderCall() {
        BillingQuote quote = quote();
        java.util.UUID commandId = java.util.UUID.randomUUID();
        BillingCommand existing = new BillingCommand(
                commandId, SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.UNKNOWN,
                MembershipBillingErrorCode.BILLING_UNAVAILABLE.code(), "provider outcome is unknown",
                null, null, null, null, null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));
        when(repository.findSubscriptionById(55L)).thenReturn(Optional.of(subscription(55L)));
        when(repository.claimUnknownCommand(commandId)).thenReturn(true);
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenReturn(new ProviderSubscriptionCreated(
                        "provider-sub-1", "https://payuni.test/checkout", "PENDING", null, null));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("PENDING", result.status());
        verify(repository).claimUnknownCommand(commandId);
        verify(adapter).create(any(SubscriptionProviderCommand.class));
        verify(repository).markCommandSucceeded(any(), anyLong(), any(), any(), any(), any());
    }

    @Test
    void processingReplayDoesNotInvokeProviderASecondTime() {
        BillingQuote quote = quote();
        BillingCommand existing = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.PROCESSING, null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));

        assertEquals(MembershipBillingErrorCode.BILLING_UNAVAILABLE, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).create(any());
        verify(repository, org.mockito.Mockito.never()).claimUnknownCommand(any());
    }

    @Test
    void expiredProcessingLeaseCanRecoverWithTheSameProviderIdempotencyKey() {
        BillingQuote quote = quote();
        java.util.UUID commandId = java.util.UUID.randomUUID();
        BillingCommand existing = new BillingCommand(
                commandId, SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.PROCESSING, null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));
        when(repository.claimStaleProcessingCommand(commandId)).thenReturn(true);
        when(repository.findSubscriptionById(55L)).thenReturn(Optional.of(subscription(55L)));
        when(adapter.create(any(SubscriptionProviderCommand.class)))
                .thenReturn(new ProviderSubscriptionCreated(
                        "provider-sub-1", "https://payuni.test/checkout", "PENDING", null, null));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("PENDING", result.status());
        verify(repository).claimStaleProcessingCommand(commandId);
        verify(adapter).create(any(SubscriptionProviderCommand.class));
    }

    @Test
    void sameIdempotencyKeyWithDifferentRequestHashIsConflict() {
        BillingQuote quote = quote();
        BillingCommand existing = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", "different-hash", quote.quoteId(), null, BillingCommandStatus.PROCESSING, null);
        when(repository.findCommand(any(), any())).thenReturn(Optional.of(existing));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));
        assertEquals(MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT, exception.errorCode());
    }

    @Test
    void concurrentIdempotencyInsertReplaysTheCommandClaimedByTheOtherTransaction() {
        BillingQuote quote = quote();
        BillingCommand existing = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.SUCCEEDED, null);
        when(repository.insertCommand(any())).thenReturn(false);
        when(repository.findCommand(SubscriptionOperation.CREATE_SUBSCRIPTION.value(), "request-1"))
                .thenReturn(Optional.of(existing));
        when(repository.findSubscriptionById(55L)).thenReturn(Optional.of(subscription(55L)));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1"));

        assertEquals("55", result.periodSubscriptionId());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void invalidGeneratedSubscriptionIdFailsBeforeProviderInvocation() {
        BillingQuote quote = quote();
        service = new MembershipBillingApplicationService(repository, adapter, () -> NOW, () -> 0L);

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));

        assertEquals(MembershipBillingErrorCode.BILLING_UNAVAILABLE, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void blankCreateRequestIdIsRejectedBeforeRepositoryLookup() {
        BillingQuote quote = quote();

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, " ")));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
        verify(repository, org.mockito.Mockito.never()).findCommand(any(), any());
    }

    @Test
    void quoteReplayMustMatchTheOriginalSubscriptionSnapshot() {
        BillingQuote quote = quote();
        MembershipProviderSubscription existing = subscription(55L);
        when(repository.findSubscriptionByQuoteId("quote-1")).thenReturn(Optional.of(existing));

        BillingQuote unsignedTampered = new BillingQuote(
                quote.quoteId(), quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(),
                quote.offerId(), quote.offerVersionNo(), quote.unitAmountMinor() + 1, quote.currency(),
                quote.intervalUnit(), quote.intervalCount(), quote.providerAccountRef(), quote.firstPeriodDate(),
                quote.expiresAt(), "");
        BillingQuote tampered = new BillingQuote(
                unsignedTampered.quoteId(), unsignedTampered.subject(), unsignedTampered.membershipId(),
                unsignedTampered.membershipChangeRequestId(), unsignedTampered.offerId(),
                unsignedTampered.offerVersionNo(), unsignedTampered.unitAmountMinor(), unsignedTampered.currency(),
                unsignedTampered.intervalUnit(), unsignedTampered.intervalCount(),
                unsignedTampered.providerAccountRef(), unsignedTampered.firstPeriodDate(),
                unsignedTampered.expiresAt(), BillingQuoteCanonicalizer.sha256Hex(unsignedTampered));
        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        tampered.subject(), tampered.membershipId(), tampered.membershipChangeRequestId(),
                        tampered, "request-2")));
        assertEquals(MembershipBillingErrorCode.BILLING_QUOTE_INVALID, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void sameQuoteWithDifferentRequestIdReplaysTheOriginalHostedCheckoutSnapshot() {
        BillingQuote quote = quote();
        MembershipProviderSubscription existingSubscription = subscription(55L);
        BillingCommand original = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.CREATE_SUBSCRIPTION.value(), quote.subject(),
                "request-1", BillingCommandHasher.hashCreate(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote),
                quote.quoteId(), 55L, BillingCommandStatus.SUCCEEDED, null, null,
                "provider-sub-1", "https://payuni.test/checkout", "M123456789-S1", "PENDING", null);
        when(repository.findSubscriptionByQuoteId(quote.quoteId())).thenReturn(Optional.of(existingSubscription));
        when(repository.findCreateCommandByQuoteId(quote.quoteId())).thenReturn(Optional.of(original));

        MembershipBillingApplicationService.CreateResult result = service.create(
                new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-2"));

        assertEquals("https://payuni.test/checkout", result.redirectUrl());
        assertEquals("provider-sub-1", result.providerSubscriptionId());
        assertEquals("M123456789-S1", result.merTradeNo());
        assertEquals("PENDING", result.status());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void blankActionRequestIdIsRejectedBeforeRepositoryLookup() {
        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.suspend(new MembershipBillingApplicationService.ActionCommand("9", " ", "reason")));
        assertEquals(MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT, exception.errorCode());
        verify(repository, org.mockito.Mockito.never()).findCommand(any(), any());
    }

    @Test
    void unsupportedActionAfterCommandCreationIsMarkedFailedNotLeftProcessing() {
        MembershipProviderSubscription pendingWithoutProvider = new MembershipProviderSubscription(
                9L, new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1", "quote-1",
                "BASE_MEMBERSHIP", "payuni-platform-sandbox", "PAYUNI", null, 1, "M123456789-S1",
                "offer-1", "7", quote().snapshotHash(),
                NOW.plusMinutes(10), "TWD", 19900, "MONTH", 1, NOW, null,
                ProviderSubscriptionStatus.PENDING);
        when(repository.findSubscriptionById(9L)).thenReturn(Optional.of(pendingWithoutProvider));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.suspend(new MembershipBillingApplicationService.ActionCommand("9", "suspend-2", "reason")));
        assertEquals(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED, exception.errorCode());
        verify(repository).markCommandFailed(any(), org.mockito.ArgumentMatchers.eq(
                MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED.code()), any());
    }

    @Test
    void existingActiveBaseSubscriptionIsRejectedBeforeProviderCall() {
        BillingQuote quote = quote();
        when(repository.findActiveSubscription(quote.subject()))
                .thenReturn(Optional.of(subscription(88L)));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));
        assertEquals(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void recurringCapabilityMustBeExplicitlyEnabledOnProviderAccount() {
        BillingQuote quote = quote();
        when(repository.findProviderAccount("payuni-platform-sandbox"))
                .thenReturn(Optional.of(new ProviderAccountProfile(
                        "PAYUNI", "payuni-platform-sandbox", "SANDBOX", "secret://payuni",
                        "{\"SUSPEND\":true}", "ACTIVE")));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.create(new MembershipBillingApplicationService.CreateCommand(
                        quote.subject(), quote.membershipId(), quote.membershipChangeRequestId(), quote, "request-1")));

        assertEquals(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).create(any());
    }

    @Test
    void suspendActionIsIdempotentAndChangesOnlySharedSubscriptionStatus() {
        MembershipProviderSubscription subscription = subscription(9L);
        when(repository.findSubscriptionById(9L)).thenReturn(Optional.of(subscription));
        when(adapter.suspend(any())).thenReturn(new ProviderActionResult("SUSPENDED"));

        MembershipBillingApplicationService.ActionResult result = service.suspend(
                new MembershipBillingApplicationService.ActionCommand("9", "suspend-1", "payment failed"));

        assertEquals("SUSPENDED", result.status());
        verify(repository).updateSubscriptionStatus(9L, ProviderSubscriptionStatus.SUSPENDED);
        assertTrue(result.periodSubscriptionId().equals("9"));
    }

    @Test
    void anotherLifecycleRequestIsBlockedByTheSubscriptionCommandLease() {
        MembershipProviderSubscription subscription = subscription(9L);
        BillingCommand active = new BillingCommand(
                java.util.UUID.randomUUID(), SubscriptionOperation.SUSPEND_SUBSCRIPTION.value(),
                subscription.subject(), "suspend-1",
                BillingCommandHasher.hashAction(SubscriptionOperation.SUSPEND_SUBSCRIPTION, "9", "payment failed"),
                null, 9L, BillingCommandStatus.PROCESSING, null);
        when(repository.findSubscriptionById(9L)).thenReturn(Optional.of(subscription));
        when(repository.findActiveCommandBySubscriptionId(9L)).thenReturn(Optional.of(active));

        BillingValidationException exception = assertThrows(
                BillingValidationException.class,
                () -> service.suspend(new MembershipBillingApplicationService.ActionCommand(
                        "9", "suspend-2", "another reason")));

        assertEquals(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT, exception.errorCode());
        verify(adapter, org.mockito.Mockito.never()).suspend(any());
        verify(repository, org.mockito.Mockito.never()).insertCommand(any());
    }

    private static BillingQuote quote() {
        BillingQuote unsigned = new BillingQuote(
                "quote-1", new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1",
                "offer-1", "7", 19900, "TWD",
                io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit.MONTH, 1,
                "payuni-platform-sandbox", NOW, NOW.plusMinutes(10), "");
        return new BillingQuote(
                unsigned.quoteId(), unsigned.subject(), unsigned.membershipId(), unsigned.membershipChangeRequestId(),
                unsigned.offerId(), unsigned.offerVersionNo(), unsigned.unitAmountMinor(), unsigned.currency(),
                unsigned.intervalUnit(), unsigned.intervalCount(), unsigned.providerAccountRef(),
                unsigned.firstPeriodDate(), unsigned.expiresAt(), BillingQuoteCanonicalizer.sha256Hex(unsigned));
    }

    private static MembershipProviderSubscription subscription(long id) {
        return new MembershipProviderSubscription(
                id, new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1", "quote-1",
                "BASE_MEMBERSHIP", "payuni-platform-sandbox", "PAYUNI", "provider-sub-1", 1, "M123456789-S1",
                "offer-1", "7", quote().snapshotHash(),
                NOW.plusMinutes(10), "TWD", 19900, "MONTH", 1, NOW, null, ProviderSubscriptionStatus.PENDING);
    }
}
