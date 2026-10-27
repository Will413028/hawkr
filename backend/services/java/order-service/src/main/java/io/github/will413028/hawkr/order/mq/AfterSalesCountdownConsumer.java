package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.order.service.OrderCountdownService;
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

/**
 * 售後事件 → 訂單 60h 自動完成倒數 暫停 / 恢復（03 §5.8）。
 *
 * 訂閱（同一 queue 綁多個 routing key）：
 * <pre>
 * 暫停：
 *   - aftersales.case.applied
 *   - aftersales.appeal.submitted
 *   - aftersales.case.auto_escalated
 * 恢復（嘗試）：
 *   - aftersales.case.rejected
 *   - aftersales.case.auto_rejected
 *   - aftersales.case.cancelled_by_buyer
 *   - aftersales.case.coupon_compensated
 *   - aftersales.platform_decision.made
 *   - aftersales.case.auto_completed_return_pending_timeout (2026-06-15 新增；待寄回逾時自動關案，與 cancelled/auto_rejected 同性質)
 *
 * aftersales.refund.completed 已收斂至 AfterSalesEventConsumer.onRefundCompleted 同事務處理
 * （ORD-02 雙 queue 收斂，03 §5.7.1），本 queue 的 binding 已移除，不在此消費。
 * </pre>
 *
 * 手動 ACK；任何例外都 ACK（不重試），由對帳 worker 兜底。
 */
@Component
public class AfterSalesCountdownConsumer {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesCountdownConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderCountdownService countdownService;

    public AfterSalesCountdownConsumer(OrderCountdownService countdownService) {
        this.countdownService = countdownService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_ORDER_AFTERSALES_COUNTDOWN, containerFactory = "manualAckContainerFactory")
    public void onCountdownEvent(Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag,
                                 @Header(AmqpHeaders.RECEIVED_ROUTING_KEY) String routingKey) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            String orderNo = extractOrderNo(body);
            if (orderNo.isEmpty()) {
                log.warn("AfterSalesCountdownConsumer.invalid",
                        "payload 缺 orderNo，ACK skip", "routingKey", routingKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            if (!AfterSalesEventType.isValidValue(routingKey)) {
                log.warn("AfterSalesCountdownConsumer.unknownRoutingKey",
                        "未預期的 routing key，ACK skip", "routingKey", routingKey);
            } else {
                switch (AfterSalesEventType.fromValue(routingKey)) {
                    // 暫停
                    case CASE_APPLIED, APPEAL_SUBMITTED, CASE_AUTO_ESCALATED ->
                            countdownService.pause(orderNo, routingKey);
                    // 恢復（嘗試）；REFUND_COMPLETED 已收斂至 AfterSalesEventConsumer（ORD-02），
                    // broker 舊 binding 殘留的訊息落 default 分支 ACK skip
                    case CASE_REJECTED, CASE_AUTO_REJECTED, CASE_CANCELLED_BY_BUYER,
                         CASE_COUPON_COMPENSATED, PLATFORM_DECISION_MADE,
                         CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT ->
                            countdownService.tryResume(orderNo, routingKey);
                    default -> log.warn("AfterSalesCountdownConsumer.unhandledRoutingKey",
                            "已知但不處理的 routing key，ACK skip", "routingKey", routingKey);
                }
            }
        } catch (Exception e) {
            log.error("AfterSalesCountdownConsumer.error",
                    "處理失敗 routingKey=" + routingKey + " body=" + body, e);
        } finally {
            channel.basicAck(deliveryTag, false);
        }
    }

    /**
     * payload 取 order_no（snake_case wire；對齊 36 §7.4 Conformance Contract）。
     * 兼容頂層 / data.order_no 兩種包裝。
     */
    private String extractOrderNo(String body) throws IOException {
        JsonNode root = MAPPER.readTree(body);
        String orderNo = root.path("order_no").asText("");
        if (orderNo.isEmpty()) {
            orderNo = root.path("data").path("order_no").asText("");
        }
        return orderNo;
    }
}
