package io.github.will413028.hawkr.common.mq;

import com.rabbitmq.client.Channel;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * #1871 OBS-08：驗證 {@link MqErrorHandler#handleFailure} 在 retry / dlq 兩個分支
 * 都正確記錄 {@code mq_java_consume_failure_total}——這是全 repo 唯一集中的
 * Java 側 consumer 失敗計數點（32 個 {@code @RabbitListener} 各自呼叫本方法）。
 */
class MqErrorHandlerTest {

    @AfterEach
    void resetMeterRegistry() {
        // handleFailure 讀的是 static 欄位；每個 test 結束後清掉，避免污染其他測試檔
        // 在同一個 JVM 內對 MqErrorHandler 的呼叫（static 狀態跨測試類別共享）。
        MqErrorHandler.setMeterRegistry(null);
    }

    @Test
    void retryBranchIncrementsRetryOutcomeAndRepublishes() throws Exception {
        SimpleMeterRegistry registry = new SimpleMeterRegistry();
        MqErrorHandler.setMeterRegistry(registry);
        Channel channel = mock(Channel.class);
        Message message = buildMessage("order.paid", 0);

        MqErrorHandler.handleFailure(message, channel, 1L, "TestConsumer", new RuntimeException("boom"));

        verify(channel).basicAck(1L, false);
        verify(channel).basicPublish(any(), any(), any(), any());
        verify(channel, never()).basicReject(anyLong(), anyBoolean());
        assertThat(registry.counter("mq_java_consume_failure_total", "queue", "order.paid", "outcome", "retry")
                .count()).isEqualTo(1.0);
    }

    @Test
    void dlqBranchIncrementsDlqOutcomeAndRejects() throws Exception {
        SimpleMeterRegistry registry = new SimpleMeterRegistry();
        MqErrorHandler.setMeterRegistry(registry);
        Channel channel = mock(Channel.class);
        Message message = buildMessage("order.paid", 3);

        MqErrorHandler.handleFailure(message, channel, 2L, "TestConsumer", new RuntimeException("still failing"));

        verify(channel).basicReject(2L, false);
        assertThat(registry.counter("mq_java_consume_failure_total", "queue", "order.paid", "outcome", "dlq")
                .count()).isEqualTo(1.0);
    }

    @Test
    void noMeterRegistrySetDoesNotThrow() throws Exception {
        // setMeterRegistry 從未被呼叫過（@AfterEach 保證每個 test 都是 null 起手）——
        // 對應「沒有 Spring context 時直接呼叫這個 static 方法」的情境。
        Channel channel = mock(Channel.class);
        Message message = buildMessage("order.paid", 3);

        MqErrorHandler.handleFailure(message, channel, 3L, "TestConsumer", new RuntimeException("still failing"));

        verify(channel).basicReject(3L, false);
    }

    private Message buildMessage(String queue, int retryCount) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue(queue);
        props.setContentType("application/json");
        if (retryCount > 0) {
            props.setHeader("x-retry-count", retryCount);
        }
        return new Message("{}".getBytes(StandardCharsets.UTF_8), props);
    }
}
