package io.github.will413028.hawkr.webhook.infrastructure.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

class PayUniEventRoutingTest {

    @Test
    void recurringPaymentUsesDedicatedPayUniRoute() {
        assertEquals(MqConstants.EXCHANGE_PAYUNI_WEBHOOK,
                PayUniEventRouting.exchange(PayUniEventType.PAYMENT_RECURRING));
        assertEquals(MqConstants.ROUTING_KEY_PAYUNI_PAYMENT_RECURRING,
                PayUniEventRouting.topic(PayUniEventType.PAYMENT_RECURRING));
    }
}
