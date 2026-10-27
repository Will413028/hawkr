package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.slf4j.MDC;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * 訂單確認收貨事件消費者
 *
 * 監聽 Queue: shipping.order.receipt.confirmed
 * 來源: Order Service 確認收貨（買家主動或排程自動完成）後發布
 *
 * 收到事件後將 shipments.receipt_confirmed 標記為 TRUE
 * MANUAL ACK + DLQ：parse 失敗或 DB 異常 → NACK + DLQ（docs/開發規範/MQ 與事件管理規範.md §1.3，2026-05-20）
 */
@Component
public class OrderReceiptConfirmedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(OrderReceiptConfirmedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ShipmentMapper shipmentMapper;

    public OrderReceiptConfirmedConsumer(ShipmentMapper shipmentMapper) {
        this.shipmentMapper = shipmentMapper;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED,
            containerFactory = "manualAckContainerFactory")
    public void onOrderReceiptConfirmed(Message message, Channel channel,
                                         @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        byte[] body = message.getBody();
        String orderNo = null;
        try {
            JsonNode node = MAPPER.readTree(body);
            orderNo = node.path("order_no").asText("");
            String confirmType = node.path("confirm_type").asText("");
            // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
            // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
            MDC.put("order_no", orderNo);

            if (orderNo.isEmpty()) {
                log.error("OrderReceiptConfirmedConsumer.parse.error",
                        "缺少 orderNo，ACK skip");
                channel.basicAck(deliveryTag, false);
                return;
            }

            int rows = shipmentMapper.markReceiptConfirmed(orderNo);
            if (rows > 0) {
                log.info("OrderReceiptConfirmedConsumer.marked",
                        "已標記收貨確認，confirmType=" + confirmType, "order_no", orderNo);
            } else {
                log.info("OrderReceiptConfirmedConsumer.skipped",
                        "物流單已標記或不存在", "order_no", orderNo);
            }
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("OrderReceiptConfirmedConsumer.error",
                    "處理收貨確認事件失敗，orderNo=" + orderNo, e);
            channel.basicNack(deliveryTag, false, false);
        } finally {
            MDC.remove("order_no");
        }
    }
}
