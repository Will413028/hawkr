package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.slf4j.MDC;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 訂單付款成功 MQ 消費者
 *
 * 監聽 Queue: shipping.order.paid
 * 來源: Order Service 付款成功後發布的 order.paid 事件
 *
 * 手動 ACK，at-least-once + 冪等（ShippingService 內部 DB 檢查）
 */
@Component
public class OrderPaidConsumer {

    private static final ObsLogger log = ObsLogger.forClass(OrderPaidConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private final ShippingService shippingService;

    public OrderPaidConsumer(ShippingService shippingService) {
        this.shippingService = shippingService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_ORDER_PAID, containerFactory = "manualAckContainerFactory")
    public void onOrderPaid(org.springframework.amqp.core.Message message, Channel channel,
                            @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        log.info("OrderPaidConsumer.received", "收到 order.paid 事件");

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
            // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
            MDC.put("order_no", orderNo);

            if (orderNo.isEmpty()) {
                log.warn("OrderPaidConsumer.invalid", "orderNo 為空，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            shippingService.createShipmentFromOrder(orderNo);

            channel.basicAck(deliveryTag, false);

            log.info("OrderPaidConsumer.success",
                    "自動建單完成", "order_no", orderNo);

        } catch (Exception e) {
            log.error("OrderPaidConsumer.error", "自動建單失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "OrderPaidConsumer", e);
        } finally {
            MDC.remove("order_no");
        }
    }
}
