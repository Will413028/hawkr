package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.mq.MqConstants;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 退款結果 MQ 消費者
 *
 * 監聽 Queue: order.refund.result
 * 綁定 Routing Key: refund.succeeded / refund.failed
 *
 * DLQ 機制：超過 MAX_REDELIVERY_ATTEMPTS 次重試後拒絕，訊息進入死信佇列
 */
@Component
public class RefundResultConsumer {

    private static final ObsLogger log = ObsLogger.forClass(RefundResultConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_REFUND_RESULT, containerFactory = "manualAckContainerFactory")
    public void onRefundResult(org.springframework.amqp.core.Message message, Channel channel,
                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        String routingKey = message.getMessageProperties().getReceivedRoutingKey();

        log.info("RefundResultConsumer.received", "收到退款結果事件", "routingKey", routingKey);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String refundNo = payload.path("refund_no").asText("");

            switch (routingKey) {
                case MqConstants.ROUTING_KEY_REFUND_SUCCEEDED ->
                        log.info("RefundResultConsumer.succeeded", "退款成功通知",
                                "refundNo", refundNo);
                case MqConstants.ROUTING_KEY_REFUND_FAILED ->
                        log.info("RefundResultConsumer.failed", "退款失敗通知",
                                "refundNo", refundNo);
                default ->
                        log.info("RefundResultConsumer.unknown", "未知路由鍵", "routingKey", routingKey);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("RefundResultConsumer.error", "處理退款結果事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "RefundResultConsumer", e);
        }
    }
}
