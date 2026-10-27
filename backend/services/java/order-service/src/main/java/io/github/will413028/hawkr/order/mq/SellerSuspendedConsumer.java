package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.SellerEventType;
import io.github.will413028.hawkr.order.mq.dto.SellerSuspensionEventPayload;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderExpiryCancellationService;
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
import java.util.List;

/**
 * 賣家停權批次取消未付款單消費者（03 §5.1a.2 / 36 §3.6f）。
 *
 * <p>消費 seller-service 發布的 {@code seller.suspended} 事件 → 以事件的 {@code store_id}
 * （非 {@code accountId}——帳號重開店後兩者會分岔，orders.store_id 存的是 resolved store_id，
 * 比照 coupon-service 同款修復）掃描該賣家全部 {@code PENDING_PAYMENT(0)} 訂單號 → 逐筆透過
 * {@link OrderExpiryCancellationService#cancelPendingOrder} 複用付款逾時取消的既有防護
 * （悲觀鎖 CAS + 付款 SoT gate + TX 外 best-effort 補償），cancel_reason 帶
 * {@code OrderCancelReason.SELLER_SUSPENDED}（9104）。
 *
 * <p>order-service 不消費 {@code seller.unsuspended}——批次取消為不可逆動作（訂單一旦取消
 * 不會因解除停權而恢復），與 product/coupon 的可逆下架/停用語意不同。
 *
 * <h3>冪等</h3>
 * 掃描結果僅為候選；每筆最終取消動作走 CAS（{@code status = PENDING_PAYMENT}），已被其他流程
 * 處理（已付款/已取消）的訂單會在 CAS 階段 0 副作用跳過。單筆失敗記錄續跑其餘，不中斷整批。
 */
@Component
public class SellerSuspendedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerSuspendedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final OrderExpiryCancellationService orderExpiryCancellationService;

    public SellerSuspendedConsumer(OrderRepository orderRepository,
                                   OrderExpiryCancellationService orderExpiryCancellationService) {
        this.orderRepository = orderRepository;
        this.orderExpiryCancellationService = orderExpiryCancellationService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SELLER_SUSPENDED,
                    containerFactory = "manualAckContainerFactory")
    public void onSellerSuspended(Message message, Channel channel,
                                  @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode raw = MAPPER.readTree(body);
            SellerSuspensionEventPayload payload = SellerSuspensionEventPayload.fromJson(raw);

            if (payload == null) {
                log.warn("SellerSuspendedConsumer.payload.missing",
                        "缺少 suspensionId/accountId/store_id 或格式錯誤 body=" + body);
                channel.basicAck(deliveryTag, false);
                return;
            }
            if (!SellerEventType.SELLER_SUSPENDED.value().equals(payload.eventType())) {
                log.warn("SellerSuspendedConsumer.eventType.invalid",
                        "eventType 與 routing key 不一致 eventType=" + payload.eventType());
                channel.basicAck(deliveryTag, false);
                return;
            }

            List<String> orderNos = orderRepository.findPendingPaymentOrderNosBySeller(
                    payload.storeId(), OrderStatus.PENDING_PAYMENT.value());

            int cancelled = 0;
            for (String orderNo : orderNos) {
                try {
                    if (orderExpiryCancellationService.cancelPendingOrder(
                            orderNo, OrderCancelReason.SELLER_SUSPENDED.value())) {
                        cancelled++;
                    }
                } catch (Exception e) {
                    // 單筆失敗記錄續跑其餘，不因單一訂單異常中斷整批（03 §5.1a.2）
                    log.error("SellerSuspendedConsumer.cancelOrder.error",
                            "取消單筆訂單失敗，繼續處理其餘: orderNo=" + orderNo, e);
                }
            }

            log.info("SellerSuspendedConsumer.done",
                    "賣家停權批次取消完成 accountId=" + payload.accountId()
                            + " storeId=" + payload.storeId()
                            + " suspensionId=" + payload.suspensionId()
                            + " candidates=" + orderNos.size() + " cancelled=" + cancelled);
            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("SellerSuspendedConsumer.handle.fail", "處理 seller.suspended 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "SellerSuspendedConsumer", e);
        }
    }
}
