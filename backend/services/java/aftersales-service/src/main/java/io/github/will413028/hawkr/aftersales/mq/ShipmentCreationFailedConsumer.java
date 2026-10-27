package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.service.SellerCancelService;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
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
 * Backstop 建單逾時取消 MQ 消費者（03 §8.11）。
 *
 * 監聽 Order Service 發布的 order.shipment.creation_failed 事件，
 * 委派 {@link SellerCancelService#cancelShipmentBySystem} 以 SHIPMENT_CREATE_TIMEOUT
 * 執行系統自動取消 + 退款（冪等：已有取消記錄則跳過）。
 */
@Component
public class ShipmentCreationFailedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentCreationFailedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final SellerCancelService sellerCancelService;

    public ShipmentCreationFailedConsumer(SellerCancelService sellerCancelService) {
        this.sellerCancelService = sellerCancelService;
    }

    @RabbitListener(queues = AfterSalesEventMqConfig.QUEUE_SHIPMENT_CREATION_FAILED,
                    containerFactory = "manualAckContainerFactory")
    public void onCreationFailed(org.springframework.amqp.core.Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        log.info("ShipmentCreationFailedConsumer.received", "收到建單逾時取消事件");

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");

            if (orderNo.isEmpty()) {
                log.warn("ShipmentCreationFailedConsumer.invalid", "orderNo 為空，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // #1871 OBS-09：order_no 進 MDC，讓這個 handler 內所有 log 行都能被 Loki structured
            // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
            MDC.put("order_no", orderNo);

            sellerCancelService.cancelShipmentBySystem(orderNo, OrderCancelReason.SHIPMENT_CREATE_TIMEOUT);

            channel.basicAck(deliveryTag, false);

            log.info("ShipmentCreationFailedConsumer.success", "建單逾時自動取消完成", "orderNo", orderNo);

        } catch (Exception e) {
            log.error("ShipmentCreationFailedConsumer.error", "建單逾時自動取消失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShipmentCreationFailedConsumer", e);
        } finally {
            MDC.remove("order_no");
        }
    }
}
