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
 * Backstop 重試建單事件消費者（03 §8.11）。
 *
 * <p>監聽 Queue: shipping.shipment.create_requested（routing key: order.shipment.create_requested）。
 * 來源：Order Service Backstop 掃描到付款後逾期仍無物流單時發出。
 * 與 {@link OrderPaidConsumer} 共用 {@code createShipmentFromOrder}（以 order_no 冪等），
 * 故重複/與 order.paid 並發皆安全。手動 ACK。
 */
@Component
public class ShipmentCreateRequestedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentCreateRequestedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private final ShippingService shippingService;

    public ShipmentCreateRequestedConsumer(ShippingService shippingService) {
        this.shippingService = shippingService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED, containerFactory = "manualAckContainerFactory")
    public void onCreateRequested(org.springframework.amqp.core.Message message, Channel channel,
                                  @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        log.info("ShipmentCreateRequestedConsumer.received", "收到 Backstop 重試建單事件");

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
            // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
            MDC.put("order_no", orderNo);

            if (orderNo.isEmpty()) {
                log.warn("ShipmentCreateRequestedConsumer.invalid", "orderNo 為空，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            shippingService.createShipmentFromOrder(orderNo);

            channel.basicAck(deliveryTag, false);

            log.info("ShipmentCreateRequestedConsumer.success", "重試建單完成", "order_no", orderNo);

        } catch (Exception e) {
            log.error("ShipmentCreateRequestedConsumer.error", "重試建單失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShipmentCreateRequestedConsumer", e);
        } finally {
            MDC.remove("order_no");
        }
    }
}
