package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.service.SellerCancelService;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
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
 * §5.9 賣家處理逾期 MQ 消費者。
 *
 * 監聯 Shipping Service 發布的 shipment.processing.timeout 事件，
 * 委派 SellerCancelService.cancelShipmentBySystem() 執行系統自動取消+退款。
 */
@Component
public class ProcessingTimeoutConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ProcessingTimeoutConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final SellerCancelService sellerCancelService;

    public ProcessingTimeoutConsumer(SellerCancelService sellerCancelService) {
        this.sellerCancelService = sellerCancelService;
    }

    @RabbitListener(queues = AfterSalesEventMqConfig.QUEUE_PROCESSING_TIMEOUT,
                    containerFactory = "manualAckContainerFactory")
    public void onProcessingTimeout(org.springframework.amqp.core.Message message, Channel channel,
                                     @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        log.info("ProcessingTimeoutConsumer.received", "收到賣家逾期事件");

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("orderNo").asText("");

            if (orderNo.isEmpty()) {
                log.warn("ProcessingTimeoutConsumer.invalid", "orderNo 為空，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            sellerCancelService.cancelShipmentBySystem(orderNo, OrderCancelReason.SHIPMENT_TIMEOUT);

            channel.basicAck(deliveryTag, false);

            log.info("ProcessingTimeoutConsumer.success",
                    "逾期自動取消完成", "orderNo", orderNo);

        } catch (Exception e) {
            log.error("ProcessingTimeoutConsumer.error", "逾期自動取消失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ProcessingTimeoutConsumer", e);
        }
    }
}
