package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.membership.billing.grpc.ProviderChargeResultData;
import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import com.google.protobuf.util.JsonFormat;
import com.google.protobuf.TypeRegistry;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import java.time.LocalDateTime;
import java.util.UUID;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MembershipBillingEventFactoryTest {

    @Test
    void buildsDirectEnvelopeWithVersionedAnyPayload() throws Exception {
        MembershipProviderSubscription subscription = subscription();
        ProviderChargeResult result = result(ProviderChargeStatus.SUCCEEDED);

        MembershipEventEnvelope envelope = MembershipBillingEventFactory.build(
                subscription, result, "8d7f6a4c-0d91-4b21-9f6e-0d9f7a1c3e52", LocalDateTime.of(2026, 9, 3, 0, 1));

        assertEquals("8d7f6a4c-0d91-4b21-9f6e-0d9f7a1c3e52", envelope.getEventId());
        assertEquals(4, UUID.fromString(envelope.getEventId()).version());
        assertEquals("PAYMENT_MEMBERSHIP_BUYER_SUCCESS", envelope.getEventType());
        assertEquals("1", envelope.getSchemaVersion());
        assertEquals("payment-service", envelope.getProducer());
        assertEquals("BUYER", envelope.getSubject().getSubjectType());
        assertEquals("123456789", envelope.getSubject().getSubjectId());
        assertEquals("membership-1", envelope.getMembershipId());
        assertEquals("change-1", envelope.getMembershipChangeRequestId());
        assertTrue(envelope.getData().is(ProviderChargeResultData.class));

        ProviderChargeResultData payload = envelope.getData().unpack(ProviderChargeResultData.class);
        assertEquals("SUCCEEDED", payload.getStatus());
        assertEquals("1", payload.getAmountMinor());
        assertEquals("1", payload.getThisPeriod());
        assertTrue(envelope.getMembershipPeriodId().isBlank(),
                "payment event must not invent a domain membership period id");

        String wire = JsonFormat.printer()
                .usingTypeRegistry(TypeRegistry.newBuilder()
                        .add(ProviderChargeResultData.getDescriptor()).build())
                .preservingProtoFieldNames()
                .omittingInsignificantWhitespace().print(envelope);
        assertTrue(wire.contains("\"@type\":\"type.googleapis.com/hawkr.membership.billing.v1.ProviderChargeResultData\""));
        assertTrue(wire.contains("\"provider_subscription_id\""));
        assertFalse(wire.contains("BillingEvent"));
    }

    @Test
    void failureUsesDomainScopedFailureEventType() {
        MembershipEventEnvelope envelope = MembershipBillingEventFactory.build(
                subscription(), result(ProviderChargeStatus.FAILED), "2b1e3c5d-4f67-48a9-b0c1-d2e3f4a5b6c7",
                LocalDateTime.of(2026, 9, 3, 0, 1));
        assertEquals("PAYMENT_MEMBERSHIP_BUYER_FAILED", envelope.getEventType());
    }

    private static MembershipProviderSubscription subscription() {
        return new MembershipProviderSubscription(
                1L, new SubscriptionSubject(SubjectType.BUYER, "123456789"), "membership-1", "change-1",
                "quote-1", "BASE_MEMBERSHIP", "payuni-platform-sandbox", "PAYUNI", "PT001", 1,
                "M123456789-S1", "offer-1", "7",
                "0894ef1b9d2196b7b72a38e9c52c1a797bd7147b0c4d764d9fa53843ad846eb2",
                LocalDateTime.of(2026, 9, 3, 0, 10), "TWD", 1, "MONTH", 1,
                LocalDateTime.of(2026, 9, 3, 0, 0), null, ProviderSubscriptionStatus.ACTIVE);
    }

    private static ProviderChargeResult result(ProviderChargeStatus status) {
        return new ProviderChargeResult(
                "PAYUNI", "payuni-platform-sandbox", "PT001", null, "1", "M123456789-S1_1",
                "T001", 1, "TWD", 1, status, "", "", LocalDateTime.of(2026, 9, 3, 0, 0),
                LocalDateTime.of(2026, 10, 3, 0, 0), null, null, false, "quote-1",
                "0894ef1b9d2196b7b72a38e9c52c1a797bd7147b0c4d764d9fa53843ad846eb2",
                "provider-event", "provider-key");
    }
}
