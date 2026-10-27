package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.service.StockService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * 消費 order.cancelled 事件，非同步歸還庫存
 *
 * 冪等保證：StockService.restoreStock() 內部以 (stockTxId, RESTORE) 複合鍵檢查，
 * 重複消費回傳 ALREADY_APPLIED，不會重複歸還。
 */
@Component
public class OrderCancelledConsumer {

    private static final ObsLogger log = ObsLogger.forClass(OrderCancelledConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private final StockService stockService;

    public OrderCancelledConsumer(StockService stockService) {
        this.stockService = stockService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_ORDER_CANCELLED,
                    containerFactory = "manualAckContainerFactory")
    public void onOrderCancelled(Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        String orderNo = "unknown";

        try {
            JsonNode root = MAPPER.readTree(body);
            orderNo = root.path("order_no").asText("unknown");
            String stockTxId = root.path("stock_tx_id").asText();

            if (stockTxId.isEmpty()) {
                log.warn("OrderCancelledConsumer.invalid", "stockTxId 為空，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }

            JsonNode itemsNode = root.path("items");
            List<StockService.StockItemParam> items = new ArrayList<>();
            for (JsonNode item : itemsNode) {
                long skuId = Long.parseLong(item.path("sku_id").asText());
                int quantity = Integer.parseInt(item.path("quantity").asText());
                items.add(new StockService.StockItemParam(skuId, quantity));
            }

            StockService.StockResult result = stockService.restoreStock(stockTxId, items);
            log.info("OrderCancelledConsumer.onOrderCancelled.success",
                    "庫存歸還完成，orderNo=" + orderNo + ", result=" + result,
                    "orderNo", orderNo);

            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("OrderCancelledConsumer.onOrderCancelled.error",
                    "庫存歸還失敗，orderNo=" + orderNo, e);

            MqErrorHandler.handleFailure(message, channel, deliveryTag, "OrderCancelledConsumer", e);
        }
    }
}
