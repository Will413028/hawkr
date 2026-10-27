package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderCountdownService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * refund.completed consumer 行為（03 §5.7.1）：
 * fail-loud 守門（金額非數字 / PARTIAL 金額<=0 不得進交易段）與
 * 「交易段（handler）→ post-commit 倒數恢復（tryResume）」順序。
 */
@ExtendWith(MockitoExtension.class)
class AfterSalesEventConsumerRefundCompletedTest {

    @Mock private OrderRepository orderRepository;
    @Mock private AfterSalesRefundEventHandler refundEventHandler;
    @Mock private SellerCancelRefundTxHandler sellerCancelRefundTxHandler;
    @Mock private OrderCountdownService countdownService;
    @Mock private Channel channel;

    private AfterSalesEventConsumer consumer;

    @BeforeEach
    void setUp() {
        consumer = new AfterSalesEventConsumer(orderRepository, refundEventHandler,
                sellerCancelRefundTxHandler, countdownService);
    }

    private Message message(String json) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue("order.aftersales.refund.completed");
        return new Message(json.getBytes(StandardCharsets.UTF_8), props);
    }

    @Test
    @DisplayName("PARTIAL happy path：交易段 handler 先執行，返回後才 tryResume，最後 ack")
    void partial_happyPath_handlerThenResumeThenAck() throws Exception {
        consumer.onRefundCompleted(message("""
                {"order_no":"O123","refund_outcome":"PARTIAL_REFUND","refund_amount":"100",
                 "external_ref_id":"RFD_1","case_no":"AS_1"}
                """), channel, 7L);

        var inOrder = org.mockito.Mockito.inOrder(refundEventHandler, countdownService, channel);
        inOrder.verify(refundEventHandler).handleRefundCompleted(
                eq("O123"), eq(AfterSalePlatformDecisionOutcome.PARTIAL_REFUND),
                eq(100L), eq("RFD_1"), eq("AS_1"));
        inOrder.verify(countdownService).tryResume(eq("O123"), anyString());
        inOrder.verify(channel).basicAck(7L, false);
    }

    @Test
    @DisplayName("fail-loud：refund_amount 非數字 → 不進交易段、不 tryResume（走 MqErrorHandler）")
    void invalidAmount_failLoud() throws Exception {
        consumer.onRefundCompleted(message("""
                {"order_no":"O123","refund_outcome":"PARTIAL_REFUND","refund_amount":"abc",
                 "external_ref_id":"RFD_1","case_no":"AS_1"}
                """), channel, 7L);

        verify(refundEventHandler, never()).handleRefundCompleted(any(), any(), anyLong(), any(), any());
        verify(countdownService, never()).tryResume(anyString(), anyString());
    }

    @Test
    @DisplayName("fail-loud：PARTIAL 且 refund_amount<=0 → 不進交易段（事件矛盾，走 MqErrorHandler）")
    void partialWithNonPositiveAmount_failLoud() throws Exception {
        consumer.onRefundCompleted(message("""
                {"order_no":"O123","refund_outcome":"PARTIAL_REFUND","refund_amount":"0",
                 "external_ref_id":"RFD_1","case_no":"AS_1"}
                """), channel, 7L);

        verify(refundEventHandler, never()).handleRefundCompleted(any(), any(), anyLong(), any(), any());
        verify(countdownService, never()).tryResume(anyString(), anyString());
    }

    @Test
    @DisplayName("fail-loud：REJECT_APPEAL outcome → 不進交易段（走 MqErrorHandler）")
    void rejectAppealOutcome_failLoud() throws Exception {
        consumer.onRefundCompleted(message("""
                {"order_no":"O123","refund_outcome":"REJECT_APPEAL","refund_amount":"100",
                 "external_ref_id":"RFD_1","case_no":"AS_1"}
                """), channel, 7L);

        verify(refundEventHandler, never()).handleRefundCompleted(any(), any(), anyLong(), any(), any());
        verify(countdownService, never()).tryResume(anyString(), anyString());
    }
}
