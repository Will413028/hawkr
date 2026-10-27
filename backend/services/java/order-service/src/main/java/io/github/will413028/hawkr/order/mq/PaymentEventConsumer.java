package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.order.util.HashUtil;
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
 * 支付事件 MQ 消費者
 *
 * 監聽 Queue: order.payment.result
 * 綁定 Routing Key: payment.success / payment.failed / payment.expired
 *
 * 手動 ACK，at-least-once
 * DLQ 機制：超過 MAX_REDELIVERY_ATTEMPTS 次重試後拒絕，訊息進入死信佇列
 *
 * 交易性邏輯委託 PaymentEventHandler（獨立 @Service），
 * 確保 Spring @Transactional proxy 正常運作。
 */
@Component
public class PaymentEventConsumer {

    private static final ObsLogger log = ObsLogger.forClass(PaymentEventConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private final PaymentEventHandler paymentEventHandler;

    public PaymentEventConsumer(PaymentEventHandler paymentEventHandler) {
        this.paymentEventHandler = paymentEventHandler;
    }

    @RabbitListener(queues = MqConstants.QUEUE_ORDER_PAYMENT_RESULT, containerFactory = "manualAckContainerFactory")
    public void onPaymentEvent(org.springframework.amqp.core.Message message, Channel channel,
                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        String routingKey = message.getMessageProperties().getReceivedRoutingKey();

        log.info("PaymentEventConsumer.received", "收到支付事件", "routingKey", routingKey);

        try {
            JsonNode payload = MAPPER.readTree(body);

            switch (routingKey) {
                case MqConstants.ROUTING_KEY_PAYMENT_SUCCESS -> {
                    String payloadHash = HashUtil.sha256Hex(body);
                    paymentEventHandler.handlePaymentSuccess(payload, body, payloadHash);
                }
                case MqConstants.ROUTING_KEY_PAYMENT_EXPIRED -> handlePaymentExpired(payload);
                case MqConstants.ROUTING_KEY_PAYMENT_FAILED -> {
                    String orderId = payload.path("order_id").asText("");
                    paymentEventHandler.handlePaymentFailed(orderId);
                }
                case MqConstants.ROUTING_KEY_PAYMENT_CLOSED ->
                        log.info("PaymentEventConsumer.paymentClosed", "交易關閉通知",
                                "orderId", payload.path("order_id").asText(""));
                case MqConstants.ROUTING_KEY_PAYMENT_CONFLICT ->
                        log.warn("PaymentEventConsumer.paymentConflict", "狀態衝突，需人工介入",
                                "orderId", payload.path("order_id").asText(""));
                default -> log.info("PaymentEventConsumer.unknown", "未知路由鍵，忽略", "routingKey", routingKey);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("PaymentEventConsumer.error", "處理支付事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "PaymentEventConsumer", e);
        }
    }

    /**
     * 支付過期處理
     * PayUni 回報的逾期，不取消訂單，
     * 統一由 PaymentDeadlineScheduler 排程處理超時取消
     */
    private void handlePaymentExpired(JsonNode payload) {
        String orderId = payload.path("order_id").asText("");
        if (orderId.isEmpty()) {
            log.error("PaymentEventConsumer.paymentExpired.invalid", "order_id 為空");
            return;
        }

        log.info("PaymentEventConsumer.paymentExpired.info",
                "PayUni 回報付款逾期，訂單維持待付款",
                "orderNo", orderId);
    }

}
