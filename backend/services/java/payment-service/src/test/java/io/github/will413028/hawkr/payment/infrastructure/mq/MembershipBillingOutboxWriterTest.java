package io.github.will413028.hawkr.payment.infrastructure.mq;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.verifyNoInteractions;

import io.github.will413028.hawkr.membership.grpc.MembershipEventEnvelope;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class MembershipBillingOutboxWriterTest {

    @Test
    void refusesToInsertOutsideAnActiveTransaction() {
        OutboxEventMapper mapper = Mockito.mock(OutboxEventMapper.class);
        PaymentOutboxPublisher publisher = Mockito.mock(PaymentOutboxPublisher.class);
        MembershipBillingOutboxWriter writer = new MembershipBillingOutboxWriter(mapper, publisher);

        assertThrows(IllegalStateException.class,
                () -> writer.enqueueAndDispatch(MembershipEventEnvelope.getDefaultInstance(), "payment.test"));
        verifyNoInteractions(mapper, publisher);
    }
}
