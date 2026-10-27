package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipChargeAttempt;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultDecisionType;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultConflictHandler;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import io.github.will413028.hawkr.payment.infrastructure.mq.MembershipBillingOutboxWriter;
import java.time.LocalDateTime;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class MembershipChargeResultApplicationServiceTest {

    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 3, 0, 1);
    private static final String HASH = "0894ef1b9d2196b7b72a38e9c52c1a797bd7147b0c4d764d9fa53843ad846eb2";

    @Mock
    private MembershipBillingRepository repository;
    @Mock
    private MembershipBillingOutboxWriter outboxWriter;
    @Mock
    private ProviderResultConflictHandler conflictHandler;

    private MembershipChargeResultApplicationService service;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        when(repository.findSubscriptionByProviderIdentity("PAYUNI", "PT001"))
                .thenReturn(Optional.of(subscription()));
        when(repository.findChargeAttemptByProviderEventKey(any())).thenReturn(Optional.empty());
        when(repository.findAuthoritativeChargeAttempt(1L, null, "M123456789-S1_1"))
                .thenReturn(Optional.empty());
        service = new MembershipChargeResultApplicationService(repository, outboxWriter,
                conflictHandler, () -> NOW, () -> 2L);
    }

    @Test
    void firstTerminalResultIsAuthoritativeAndEmitsOneCanonicalEvent() {
        var outcome = service.apply(success("event-1", "key-1"));

        assertEquals(ProviderResultDecisionType.AUTHORITATIVE, outcome.decisionType());
        assertEquals("PAYMENT_MEMBERSHIP_BUYER_SUCCESS", outcome.event().getEventType());
        assertEquals(4, UUID.fromString(outcome.canonicalEventId()).version());
        verify(repository).insertChargeAttempt(any(MembershipChargeAttempt.class));
        verify(outboxWriter).enqueueAndDispatch(any(MembershipEventEnvelope.class),
                org.mockito.ArgumentMatchers.eq("payment.membership.buyer.success"));
    }

    @Test
    void exactProviderDuplicateIsRecordedWithoutSecondDomainEvent() {
        ProviderChargeResult existingResult = success("event-0", "key-0");
        when(repository.findChargeAttemptByProviderEventKey("key-1"))
                .thenReturn(Optional.of(new MembershipChargeAttempt(
                        1L, 1L, subscription().subject(), "membership-1", "change-1",
                        existingResult, true, "membership-billing:winner", NOW)));

        var outcome = service.apply(success("event-1", "key-1"));

        assertEquals(ProviderResultDecisionType.DUPLICATE, outcome.decisionType());
        verify(outboxWriter, never()).enqueueAndDispatch(any(), any());
        verify(repository, never()).insertChargeAttempt(any(MembershipChargeAttempt.class));
    }

    @Test
    void contradictoryTerminalResultAppendsConflictWithoutTransition() {
        ProviderChargeResult existingResult = success("event-0", "key-0");
        when(repository.findAuthoritativeChargeAttempt(1L, null, "M123456789-S1_1"))
                .thenReturn(Optional.of(new MembershipChargeAttempt(
                        1L, 1L, subscription().subject(), "membership-1", "change-1",
                        existingResult, true, "membership-billing:winner", NOW)));

        var outcome = service.apply(failure("event-1", "key-1"));

        assertEquals(ProviderResultDecisionType.CONFLICT, outcome.decisionType());
        assertEquals("CONFLICT", outcome.persistedResult().status().name());
        verify(outboxWriter, never()).enqueueAndDispatch(any(), any());
        ArgumentCaptor<MembershipChargeAttempt> captor = ArgumentCaptor.forClass(MembershipChargeAttempt.class);
        verify(repository).insertChargeAttempt(captor.capture());
        assertEquals(ProviderChargeStatus.CONFLICT, captor.getValue().result().status());
        verify(conflictHandler).onConflict(any(), any(), any());
    }

    @Test
    void providerAmountMustMatchImmutableSubscriptionSnapshot() {
        ProviderChargeResult invalid = new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", "PT001", null, null, "M123456789-S1_1",
                "T001", 2, "TWD", 1, ProviderChargeStatus.SUCCEEDED, "", "", null, null,
                null, null, false, "quote-1", HASH, "event-1", "key-1");

        BillingValidationException exception = assertThrows(BillingValidationException.class,
                () -> service.apply(invalid));
        assertEquals(MembershipBillingErrorCode.BILLING_AMOUNT_MISMATCH, exception.errorCode());
        verify(outboxWriter, never()).enqueueAndDispatch(any(), any());
    }

    @Test
    void unknownProviderSubscriptionIsRejectedBeforeLedgerWrite() {
        when(repository.findSubscriptionByProviderIdentity("PAYUNI", "UNKNOWN"))
                .thenReturn(Optional.empty());
        BillingValidationException exception = assertThrows(BillingValidationException.class,
                () -> service.apply(successFor("UNKNOWN", "event-1", "key-1")));
        assertEquals(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT, exception.errorCode());
        verify(repository, never()).insertChargeAttempt(any());
    }

    private static ProviderChargeResult success(String eventId, String key) {
        return successFor("PT001", eventId, key);
    }

    private static ProviderChargeResult successFor(String providerSubscriptionId,
                                                    String eventId, String key) {
        return new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", providerSubscriptionId, null, null,
                "M123456789-S1_1", "T001", 1, "TWD", 1, ProviderChargeStatus.SUCCEEDED,
                "", "", LocalDateTime.of(2026, 9, 3, 0, 0), LocalDateTime.of(2026, 10, 3, 0, 0),
                null, null, false, "quote-1", HASH, eventId, key);
    }

    private static ProviderChargeResult failure(String eventId, String key) {
        return new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", "PT001", null, null,
                "M123456789-S1_1", "T001", 1, "TWD", 1, ProviderChargeStatus.FAILED,
                "DECLINED", "declined", null, null, null, null, false, "quote-1", HASH,
                eventId, key);
    }

    private static MembershipProviderSubscription subscription() {
        return new MembershipProviderSubscription(
                1L, new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1",
                "quote-1", "BASE_MEMBERSHIP", "payuni-platform-sandbox", "PAYUNI", "PT001", 1,
                "M123456789-S1", "offer-1", "7", HASH, NOW.plusMinutes(10), "TWD", 1,
                BillingIntervalUnit.MONTH.name(), 1, NOW, null, ProviderSubscriptionStatus.ACTIVE);
    }
}
