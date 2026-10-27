package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.Optional;
import java.util.function.Consumer;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class RescueRefundConsumerPayloadTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private PaymentClient paymentClient;
    @Mock private OutboxEventMapper outboxMapper;
    @Mock private TransactionTemplate transactionTemplate;
    @Mock private Channel channel;

    @Test
    void rescueCompletedOutboxUsesCanonicalBuyerIdOnly() throws Exception {
        ObjectMapper objectMapper = new ObjectMapper();
        PaymentTransaction target = PaymentTransaction.builder()
                .id(1L)
                .tradeNo("TRD-RESCUE-1")
                .orderId("ORD-RESCUE-1")
                .merId("MER-1")
                .buyerId("BUYER-42")
                .paymentType(io.github.will413028.hawkr.common.utils.code.PaymentMethod.CREDIT_CARD)
                .amount(500L)
                .status(io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus.PAID)
                .providerTradeNo("PROVIDER-1")
                .version(0)
                .build();

        when(transactionRepository.findByProviderTradeNo("PROVIDER-1"))
                .thenReturn(Optional.of(target));
        when(paymentClient.refund("MER-1", "PROVIDER-1", "TRD-RESCUE-1-rescue", 500L))
                .thenReturn(new PayUniApiResponse("SUCCESS", "退款成功", Map.of()));
        when(transactionRepository.markRescueRefunded(eq(1L), any())).thenReturn(1);
        doAnswer(invocation -> {
            Consumer<TransactionStatus> callback = invocation.getArgument(0);
            callback.accept(null);
            return null;
        }).when(transactionTemplate).executeWithoutResult(any());

        RescueRefundConsumer consumer = new RescueRefundConsumer(
                transactionRepository, paymentClient, objectMapper, outboxMapper, transactionTemplate);
        String body = """
                {"trade_no":"TRD-RESCUE-1","order_id":"ORD-RESCUE-1","mer_id":"MER-1",
                 "provider_trade_no":"PROVIDER-1","amount":500,"payment_type":"CREDIT_CARD",
                 "rescue_reason":"TEST"}
                """;

        consumer.onRescueRefundInitiated(
                new Message(body.getBytes(StandardCharsets.UTF_8)), channel, 7L);

        ArgumentCaptor<OutboxEventEntity> outboxCaptor = ArgumentCaptor.forClass(OutboxEventEntity.class);
        verify(outboxMapper).insert(outboxCaptor.capture());
        JsonNode wire = objectMapper.readTree(outboxCaptor.getValue().getPayload());
        assertEquals("BUYER-42", wire.path("buyer_id").asText());
        assertFalse(wire.has("buyerAccountId"), "retired camelCase buyer alias must not be emitted");
    }
}
