package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.payment.application.service.MembershipChargeResultApplicationService;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderResultDecisionType;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.RecurringWebhookNormalizer;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import java.util.Map;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class PayUniRecurringWebhookConsumerTest {

    @Mock
    private RecurringWebhookNormalizer normalizer;
    @Mock
    private MembershipChargeResultApplicationService chargeService;
    @Mock
    private Channel channel;

    private PayUniRecurringWebhookConsumer consumer;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        consumer = new PayUniRecurringWebhookConsumer(normalizer, chargeService);
    }

    @Test
    void acknowledgesOnlyAfterChargeServiceAcceptsRecurringEnvelope() throws Exception {
        ProviderChargeResult result = org.mockito.Mockito.mock(ProviderChargeResult.class);
        when(normalizer.normalize(any(), eq("event-1"), eq("HAWK9:PT001:T001:1:1")))
                .thenReturn(result);
        when(chargeService.apply(result)).thenReturn(
                new MembershipChargeResultApplicationService.ApplyResult(
                        ProviderResultDecisionType.AUTHORITATIVE, null, result, "canonical", null, "route"));

        Message message = message(Map.of(
                "event_id", "event-1",
                "webhook_type", "8257",
                "event_key", "HAWK9:PT001:T001:1:1",
                "payload", Map.of(
                        "MerID", "HAWK9",
                        "PeriodTradeNo", "PT001",
                        "PeriodOrderNo", "M123-S1_1",
                        "TradeNo", "T001",
                        "ThisPeriod", "1",
                        "TradeStatus", "1",
                        "TradeAmt", "1")));

        consumer.onRecurring(message, channel, 42L);

        verify(chargeService).apply(result);
        verify(channel).basicAck(42L, false);
    }

    @Test
    void barePayloadGetsDeterministicFallbackProviderEventKey() throws Exception {
        ProviderChargeResult result = org.mockito.Mockito.mock(ProviderChargeResult.class);
        when(normalizer.normalize(any(), eq(""), eq("HAWK9:PT001:T001:1:2")))
                .thenReturn(result);
        when(chargeService.apply(result)).thenReturn(
                new MembershipChargeResultApplicationService.ApplyResult(
                        ProviderResultDecisionType.DUPLICATE, null, result, null, null, null));

        Message message = message(Map.of(
                "MerID", "HAWK9",
                "PeriodTradeNo", "PT001",
                "PeriodOrderNo", "M123-S1_1",
                "TradeNo", "T001",
                "ThisPeriod", "1",
                "TradeStatus", "2"));

        consumer.onRecurring(message, channel, 43L);

        verify(normalizer).normalize(any(), eq(""), eq("HAWK9:PT001:T001:1:2"));
        verify(channel).basicAck(43L, false);
    }

    private static Message message(Object body) throws Exception {
        byte[] bytes = new ObjectMapper().writeValueAsBytes(body);
        MessageProperties properties = new MessageProperties();
        properties.setConsumerQueue("payment.payuni.membership.recurring");
        return new Message(bytes, properties);
    }
}
