package io.github.will413028.hawkr.common.mq;

import com.rabbitmq.client.AMQP;
import com.rabbitmq.client.Channel;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.io.IOException;
import java.util.HashMap;
import java.util.Map;

/**
 * MQ 消費失敗統一處理工具
 * <p>
 * 解決 basicNack(requeue=true) 搭配 x-death header 計數不生效的問題。
 * 透過自訂 {@code x-retry-count} header 追蹤重試次數：
 * <ul>
 *   <li>重試次數 < 3：ACK 原訊息，重新發布到同一 queue 並遞增 x-retry-count</li>
 *   <li>重試次數 ≥ 3：basicReject 進 DLQ，不再重試</li>
 * </ul>
 * <p>
 * 所有使用 manualAckContainerFactory 的 consumer catch 區塊統一呼叫此工具。
 */
public final class MqErrorHandler {

    private static final Logger log = LoggerFactory.getLogger(MqErrorHandler.class);
    private static final String RETRY_COUNT_HEADER = "x-retry-count";
    private static final int MAX_RETRY_COUNT = 3;

    // Set once at startup by MqErrorHandlerMetricsAutoConfiguration. Nullable so
    // handleFailure stays usable where no Spring MeterRegistry bean exists (e.g.
    // plain unit tests calling this static method directly).
    private static volatile MeterRegistry meterRegistry;

    private MqErrorHandler() {}

    public static void setMeterRegistry(MeterRegistry registry) {
        meterRegistry = registry;
    }

    /**
     * 處理消費失敗（預設最大重試 3 次）
     *
     * @param message      原始訊息
     * @param channel      RabbitMQ channel
     * @param deliveryTag  delivery tag
     * @param consumerName 消費者名稱（用於日誌）
     * @param e            異常
     */
    public static void handleFailure(Message message, Channel channel, long deliveryTag,
                                     String consumerName, Exception e) throws IOException {
        int retryCount = getRetryCount(message);
        String queueName = message.getMessageProperties().getConsumerQueue();

        if (retryCount >= MAX_RETRY_COUNT) {
            // 超過最大重試次數，送入 DLQ
            log.error("[{}] 超過最大重試次數({})，訊息進入 DLQ: {}",
                    consumerName, MAX_RETRY_COUNT, e.getMessage());
            recordOutcome(queueName, "dlq");
            channel.basicReject(deliveryTag, false);
        } else {
            // ACK 原訊息，重新發布到同一 queue 並遞增 retry count
            log.warn("[{}] 消費失敗，將重試({}/{})：{}",
                    consumerName, retryCount + 1, MAX_RETRY_COUNT, e.getMessage());
            recordOutcome(queueName, "retry");
            channel.basicAck(deliveryTag, false);
            republishWithRetryCount(channel, queueName, message, retryCount + 1);
        }
    }

    /**
     * #1871 OBS-08：handler 級直接信號，互補 infra-alerts.yml 既有的 queue 級間接
     * 信號（RabbitMQDLQMessages/RabbitMQNoConsumers/RabbitMQQueueNoConsumersStalled）。
     * 這裡是全 repo 唯一集中的 retry/dlq 決策點，但成功 ack 分散在各 @RabbitListener
     * 自行呼叫，沒有集中的成功計數可當比值分母——service-alerts.yml 的
     * MQJavaConsumerFailureHigh 因此用絕對速率而非失敗率比值。
     */
    private static void recordOutcome(String queueName, String outcome) {
        MeterRegistry registry = meterRegistry;
        if (registry == null) {
            return;
        }
        Counter.builder("mq_java_consume_failure_total")
                .tag("queue", queueName == null ? "unknown" : queueName)
                .tag("outcome", outcome)
                .register(registry)
                .increment();
    }

    private static int getRetryCount(Message message) {
        Object header = message.getMessageProperties().getHeader(RETRY_COUNT_HEADER);
        if (header instanceof Number) {
            return ((Number) header).intValue();
        }
        return 0;
    }

    private static void republishWithRetryCount(Channel channel, String queueName,
                                                 Message originalMessage, int retryCount) throws IOException {
        // 複製原始 headers 並加入 retry count
        MessageProperties originalProps = originalMessage.getMessageProperties();
        Map<String, Object> headers = new HashMap<>();
        if (originalProps.getHeaders() != null) {
            headers.putAll(originalProps.getHeaders());
        }
        headers.put(RETRY_COUNT_HEADER, retryCount);

        AMQP.BasicProperties props = new AMQP.BasicProperties.Builder()
                .contentType(originalProps.getContentType())
                .deliveryMode(2)  // persistent
                .headers(headers)
                .build();

        // 透過 default exchange（""）直接發送到指定 queue
        channel.basicPublish("", queueName, props, originalMessage.getBody());
    }
}
