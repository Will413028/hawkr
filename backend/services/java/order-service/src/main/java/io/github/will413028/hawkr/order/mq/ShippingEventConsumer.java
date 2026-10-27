package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ShippingStatusRules;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.dto.OrderDeliveredPayload;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.util.OrderShippingProgress;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 物流事件 MQ 消費者
 *
 * 監聯三個 Queue：
 * 1. order.shipping.created    — 物流單建立（routing key: shipment.created）
 * 2. order.shipping.status     — 物流狀態變更（routing key: shipment.status.changed）
 * 3. order.shipping.delivered  — 物流送達（routing key: shipment.delivered）
 *
 * 手動 ACK，at-least-once，冪等處理
 */
@Component
public class ShippingEventConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShippingEventConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final OrderConfig orderConfig;
    private final OrderEventPublisher eventPublisher;
    private final OrderOutboxMapper orderOutboxMapper;
    /** 送達路徑的短事務載體（狀態 CAS ＋ outbox INSERT 原子）——self-invocation 故走程式式包覆。 */
    private final TransactionTemplate txTemplate;

    public ShippingEventConsumer(OrderRepository orderRepository,
                                 OrderConfig orderConfig,
                                 OrderEventPublisher eventPublisher,
                                 OrderOutboxMapper orderOutboxMapper,
                                 PlatformTransactionManager txManager) {
        this.orderRepository = orderRepository;
        this.orderConfig = orderConfig;
        this.eventPublisher = eventPublisher;
        this.orderOutboxMapper = orderOutboxMapper;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    /**
     * 物流單建立事件
     *
     * 更新 orders：shipment_no、ship_time、shipping_status=UNPROCESSED(91)、label_url
     * 訂單狀態維持 PAID(10)，不推進。後續由物流狀態變更事件推進。
     * 冪等：shipment_no 已有值則跳過
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPPING_CREATED, containerFactory = "manualAckContainerFactory")
    public void onShipmentCreated(org.springframework.amqp.core.Message message, Channel channel,
                                  @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            String shipTradeNo = payload.path("ship_trade_no").asText("");
            String labelUrl = payload.path("label_key").asText("");
            String processingDeadlineStr = payload.path("processing_deadline").asText("");

            if (orderNo.isEmpty()) {
                log.error("ShippingEventConsumer.shipmentCreated.invalid",
                        "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // §5.9 解析賣家處理期限（ISO 8601 UTC，Uber 為空字串時寫 NULL）
            LocalDateTime processingDeadline = processingDeadlineStr.isBlank()
                    ? null
                    : DateTimeUtils.parseIso8601(processingDeadlineStr);

            String shippingStatusValue = ShippingStatus.UNPROCESSED.value();
            int rows = orderRepository.updateShipmentCreated(
                    orderNo, shipTradeNo, shippingStatusValue, labelUrl, processingDeadline);

            if (rows == 0) {
                log.info("ShippingEventConsumer.shipmentCreated.idempotent",
                        "訂單已有物流單號，跳過", "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.shipmentCreated.success",
                        "物流建單事件處理完成，shipTradeNo=" + shipTradeNo, "orderNo", orderNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.shipmentCreated.error", "處理物流建單事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * §5.9 賣家完成處理事件（CVS 取號 / 宅配叫件成功）→ 清空 orders.processing_deadline
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPMENT_PROCESSING_CLEARED, containerFactory = "manualAckContainerFactory")
    public void onProcessingCleared(org.springframework.amqp.core.Message message, Channel channel,
                                    @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("orderNo").asText("");

            if (orderNo.isEmpty()) {
                log.error("ShippingEventConsumer.processingCleared.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            int rows = orderRepository.clearProcessingDeadline(orderNo);
            if (rows == 0) {
                log.info("ShippingEventConsumer.processingCleared.idempotent",
                        "processing_deadline 已為 NULL，跳過", "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.processingCleared.success",
                        "賣家完成處理事件處理完成", "orderNo", orderNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.processingCleared.error", "處理賣家完成處理事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * 超商出貨便代碼回填事件
     *
     * 更新 orders：partner_id、odno、validation_no
     * 冪等：partner_id 已有值則跳過
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPPING_LABEL, containerFactory = "manualAckContainerFactory")
    public void onLabelUpdated(org.springframework.amqp.core.Message message, Channel channel,
                                @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            String partnerId = payload.path("partner_id").asText("");
            String odno = payload.path("odno").asText("");
            String validationNo = payload.path("validation_no").asText("");
            boolean isReturn = payload.path("is_return").asBoolean(false);

            if (orderNo.isEmpty()) {
                log.error("ShippingEventConsumer.labelUpdated.invalid",
                        "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 退貨事件由 AfterSales Service 消費快照至 aftersale_cases，Order Service 跳過避免覆寫 orders（§5.3.6）
            if (isReturn) {
                log.info("ShippingEventConsumer.labelUpdated.skipReturn",
                        "退貨事件交由 AfterSales 處理，Order Service 跳過", "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            int rows = orderRepository.updateLabelInfo(orderNo, partnerId, odno, validationNo);

            if (rows == 0) {
                log.info("ShippingEventConsumer.labelUpdated.idempotent",
                        "訂單已有出貨便代碼，跳過", "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.labelUpdated.success",
                        "出貨便代碼快照完成，odno=" + odno, "orderNo", orderNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.labelUpdated.error", "處理出貨便代碼回填事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * 出貨照片更新事件
     *
     * 覆蓋更新 orders.shipment_photo_keys（JSONB string array，存 objectKey）
     * 無冪等防護需求——照片覆蓋寫入，重複接收同一組照片無副作用
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPPING_PHOTO, containerFactory = "manualAckContainerFactory")
    public void onShipmentPhotoUpdated(org.springframework.amqp.core.Message message, Channel channel,
                                       @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("ShippingEventConsumer.photoUpdated.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            JsonNode keysNode = payload.path("shipment_photo_keys");
            if (!keysNode.isArray() || keysNode.isEmpty()) {
                log.info("ShippingEventConsumer.photoUpdated.skip",
                        "照片陣列為空，跳過", "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            String photoKeysJson = MAPPER.writeValueAsString(keysNode);
            int rows = orderRepository.updateShipmentPhotoKeys(orderNo, photoKeysJson);

            if (rows == 0) {
                log.info("ShippingEventConsumer.photoUpdated.noMatch",
                        "訂單不存在或更新失敗", "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.photoUpdated.success",
                        "出貨照片快照完成，count=" + keysNode.size(), "orderNo", orderNo);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.photoUpdated.error", "處理出貨照片更新事件失敗: " + e.getMessage(), e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * 物流狀態變更事件
     *
     * 更新 orders.shipping_status，並依 PayUni 狀態碼推進訂單狀態（目標態）：
     * - 91/92/21/920（isPendingShipment）→ PENDING_SHIPMENT(20)
     * - 210/921（isConfirmedShipment）、31（IN_TRANSIT）→ IN_TRANSIT(30)
     * - 32（WAITING_PICKUP）→ WAITING_PICKUP(32)
     * - 33/43/44/46/51~56/81/82（isAbnormal）→ SHIPPING_ABNORMAL(35)
     * - 98/22/41 → 不推進訂單狀態（僅更新 shipping_status）
     *
     * 各推進分支的合法前置狀態（含自 PAID(10) 起點、自 SHIPPING_ABNORMAL(35) 恢復）由
     * OrderShippingProgress 統一產生（單調前進 + 異常恢復），見 03 設計書「物流推進守門單調前進與異常恢復」。
     * order.status 推進為 CAS（前置不符則 rows=0 冪等 no-op）。物流送達（PayUni 11）由獨立的
     * shipment.delivered 事件處理（見 onShipmentDelivered）。
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPPING_STATUS, containerFactory = "manualAckContainerFactory")
    public void onShipmentStatusChanged(org.springframework.amqp.core.Message message, Channel channel,
                                        @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");
            String statusValue = payload.path("status").asText("");

            if (orderNo.isEmpty() || statusValue.isEmpty()) {
                log.error("ShippingEventConsumer.statusChanged.invalid",
                        "orderNo 或 status 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            String toStatus = statusValue;
            ShippingStatus shippingStatus = ShippingStatus.fromValue(statusValue);
            String statusDesc = payload.path("status_desc").asText("");
            String labelUrl = payload.path("label_key").asText("");
            String labelExpireAt = payload.path("label_expire_at").asText("");
            // [B-207] #1181：記錄訂單狀態是否真的推進（僅變更時才發 seller.order.updated）
            String newOrderStatus = null;

            // 更新物流狀態 + statusDesc + labelUrl + labelExpireAt
            int rows;
            if (!labelUrl.isEmpty()) {
                LocalDateTime expireAt = null;
                if (!labelExpireAt.isEmpty()) {
                    try { expireAt = LocalDateTime.parse(labelExpireAt); } catch (Exception ignored) {}
                }
                rows = orderRepository.updateShippingStatusAndLabel(orderNo, toStatus, statusDesc, labelUrl, expireAt);
            } else {
                rows = orderRepository.updateShippingStatus(orderNo, toStatus, statusDesc);
            }

            // 依物流狀態碼推進訂單狀態
            if (ShippingStatusRules.isPendingShipment(shippingStatus)) {
                // 91/92/21/920 → 訂單 PAID(10) → PENDING_SHIPMENT(20)（合法前置由 OrderShippingProgress 統一）
                int statusRows = orderRepository.updateStatusIfMatchAny(
                        orderNo, OrderStatus.PENDING_SHIPMENT.value(),
                        OrderShippingProgress.allowedFromFor(OrderStatus.PENDING_SHIPMENT.value()));
                if (statusRows > 0) {
                    newOrderStatus = OrderStatus.PENDING_SHIPMENT.value();
                    log.info("ShippingEventConsumer.statusChanged.orderPendingShipment",
                            "訂單狀態推進為待出貨", "orderNo", orderNo);
                }
            } else if (ShippingStatusRules.isConfirmedShipment(shippingStatus)) {
                // 210(宅配叫件) / 921(超商確認出貨) → 訂單 IN_TRANSIT(30)
                // 合法前置由 OrderShippingProgress 統一（與 PayUni 31 分支一致，含自 35 恢復）
                int statusRows = orderRepository.updateStatusIfMatchAny(
                        orderNo, OrderStatus.IN_TRANSIT.value(),
                        OrderShippingProgress.allowedFromFor(OrderStatus.IN_TRANSIT.value()));
                if (statusRows > 0) {
                    newOrderStatus = OrderStatus.IN_TRANSIT.value();
                    log.info("ShippingEventConsumer.statusChanged.orderInTransit",
                            "訂單狀態推進為配送中（" + shippingStatus.value() + "）", "orderNo", orderNo);
                }
            } else if (shippingStatus == ShippingStatus.IN_TRANSIT) {
                // PayUni 31 → 訂單 IN_TRANSIT(30)
                // 含自 35 恢復：物流商回報恢復正常配送，合法前置由 OrderShippingProgress 統一（03 設計書「35 恢復轉移」）
                int statusRows = orderRepository.updateStatusIfMatchAny(
                        orderNo, OrderStatus.IN_TRANSIT.value(),
                        OrderShippingProgress.allowedFromFor(OrderStatus.IN_TRANSIT.value()));
                if (statusRows > 0) {
                    newOrderStatus = OrderStatus.IN_TRANSIT.value();
                    log.info("ShippingEventConsumer.statusChanged.orderInTransit",
                            "訂單狀態推進為配送中", "orderNo", orderNo);
                }
            } else if (shippingStatus == ShippingStatus.WAITING_PICKUP) {
                // PayUni 32 → 訂單 WAITING_PICKUP(32)
                // 含自 35 恢復：合法前置由 OrderShippingProgress 統一（03 設計書「35 恢復轉移」）
                int statusRows = orderRepository.updateStatusIfMatchAny(
                        orderNo, OrderStatus.WAITING_PICKUP.value(),
                        OrderShippingProgress.allowedFromFor(OrderStatus.WAITING_PICKUP.value()));
                if (statusRows > 0) {
                    newOrderStatus = OrderStatus.WAITING_PICKUP.value();
                    log.info("ShippingEventConsumer.statusChanged.orderWaitingPickup",
                            "訂單狀態推進為待取貨", "orderNo", orderNo);
                }
            } else if (ShippingStatusRules.isAbnormal(shippingStatus)) {
                // PayUni 33/43/44/46/51~56/81/82 → 訂單 → SHIPPING_ABNORMAL(35)
                // 合法前置由 OrderShippingProgress 統一（在途主線態，不含已送達 40）
                int statusRows = orderRepository.updateStatusIfMatchAny(
                        orderNo, OrderStatus.SHIPPING_ABNORMAL.value(),
                        OrderShippingProgress.allowedFromForAbnormal());
                if (statusRows > 0) {
                    newOrderStatus = OrderStatus.SHIPPING_ABNORMAL.value();
                    log.info("ShippingEventConsumer.statusChanged.orderAbnormal",
                            "訂單狀態推進為物流異常，PayUni=" + statusValue, "orderNo", orderNo);
                }
            }
            // PayUni 98/22/41 → 不推進訂單狀態

            if (rows == 0) {
                log.info("ShippingEventConsumer.statusChanged.idempotent",
                        "物流狀態已為最新或更高，跳過，toStatus=" + statusValue, "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.statusChanged.success",
                        "物流狀態更新完成，toStatus=" + statusValue, "orderNo", orderNo);
            }

            // [B-207] #1181：僅當訂單狀態真的推進時發 seller.order.updated（eager，consumer 非 @Transactional）
            if (newOrderStatus != null) {
                Long storeId = orderRepository.findStoreIdByOrderNo(orderNo);
                eventPublisher.emitSellerOrderUpdated(
                        storeId, orderNo, OrderEventPublisher.SRC_SHIPPING_WEBHOOK,
                        null, newOrderStatus, statusValue);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.statusChanged.error", "處理物流狀態變更事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * 物流送達事件
     *
     * 更新訂單狀態：物流在途態（PAID/PENDING_SHIPMENT/IN_TRANSIT/WAITING_PICKUP/SHIPPING_ABNORMAL）→ DELIVERED(40)，設定 auto_complete_at
     * 發布 order.delivered 事件（通知前端/推播）
     * 冪等：合法前置由 OrderShippingProgress.allowedFromFor(40) 統一（單調前進：所有更早在途主線態 + 異常恢復）
     */
    @RabbitListener(queues = MqConstants.QUEUE_ORDER_SHIPPING_DELIVERED, containerFactory = "manualAckContainerFactory")
    public void onShipmentDelivered(org.springframework.amqp.core.Message message, Channel channel,
                                    @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        try {
            JsonNode payload = MAPPER.readTree(body);
            String orderNo = payload.path("order_no").asText("");

            if (orderNo.isEmpty()) {
                log.error("ShippingEventConsumer.delivered.invalid", "orderNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            String deliveredValue = ShippingStatus.PICKED_UP.value();
            LocalDateTime autoCompleteAt = DateTimeUtils.nowUtc().plusHours(orderConfig.getAutoCompleteHours());

            // 狀態 CAS 與 order.delivered 的 outbox 落地必須同生共死（**例外 A**，
            // 《多副本排程搶佔規範》§四）。原本兩者各自 auto-commit、publish 又走
            // OrderEventPublisher.publish（吞掉所有例外）⇒ MQ 短暫不可用時「訂單已 DELIVERED、
            // 事件沒送出」，而重投時這句 CAS 的 allowed-from 已不含 DELIVERED ⇒ rows=0 冪等 skip
            // ⇒ 送達通知與 order.delivered 的其他消費者**永久收不到**。
            int rows = deliveredInTx(orderNo, deliveredValue, autoCompleteAt);

            if (rows == 0) {
                log.info("ShippingEventConsumer.delivered.idempotent",
                        "訂單非待出貨/配送中/待取貨狀態或已處理，跳過", "orderNo", orderNo);
            } else {
                log.info("ShippingEventConsumer.delivered.success",
                        "物流送達事件處理完成，autoCompleteAt=" + autoCompleteAt, "orderNo", orderNo);

                // [B-207] #1181：賣家工作區 realtime — 已送達（eager）
                // 刻意留在 tx **之外**：它是 best-effort（漏推有前端 REST 全刷兜底），
                // 不該讓已落地的送達狀態因為 realtime 推播失敗而 rollback。
                // （原本還有第二個理由「emit 自己會打一次 seller gRPC」——那次身分反解已於
                //  2026-09-03 移除，payload 現在直接帶 store_id，見 OrderEventPublisher 的 javadoc。
                //  第一個理由仍然成立，位置不動。）
                Long storeId = orderRepository.findStoreIdByOrderNo(orderNo);
                eventPublisher.emitSellerOrderUpdated(
                        storeId, orderNo, OrderEventPublisher.SRC_SHIPPING_WEBHOOK,
                        null, OrderStatus.DELIVERED.value(), deliveredValue);
            }

            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ShippingEventConsumer.delivered.error", "處理物流送達事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingEventConsumer", e);
        }
    }

    /**
     * 送達的短事務 body：狀態 CAS ＋ order.delivered 落 outbox（例外 A）。
     *
     * <p>同 class self-invocation ⇒ 用 {@link org.springframework.transaction.support.TransactionTemplate}
     * 程式式包覆，不能標 {@code @Transactional}（CGLIB proxy 不攔截 {@code this.xxx()}）。
     *
     * <p>tx 內<b>零網路呼叫</b>：{@code notify_buyer_account_id} 的 gRPC 反解已移到派送時
     * （{@code OrderOutboxPublisher.enrichDelivered}），與 order.completed（#2294 B8）同一手法。
     *
     * @return CAS 影響筆數（0 = 冪等 skip，未寫 outbox）
     */
    private int deliveredInTx(String orderNo, String deliveredValue, LocalDateTime autoCompleteAt) {
        List<String> deliveredAllowed =
                OrderShippingProgress.allowedFromFor(OrderStatus.DELIVERED.value());
        Integer rows = txTemplate.execute(status -> {
            int affected = orderRepository.updateOrderDelivered(
                    orderNo, OrderStatus.DELIVERED.value(), deliveredValue,
                    ShippingStatus.PICKED_UP.label(), autoCompleteAt, deliveredAllowed);
            if (affected == 0) {
                return 0;
            }
            // buyer_id 一律查自己 order 記錄取得，不採信上游 shipping-service payload 同名欄位
            // （跨服務身份欄位不可信；鏡像 OrderService.completeOrder / OrderOutboxPublisher.enrich）
            Long buyerId = orderRepository.findBuyerIdByOrderNo(orderNo);
            // notifyBuyerAccountId 帶 null 佔位，由 OrderOutboxPublisher.enrichDelivered 於派送前反解
            // （鏡像 order.paid / order.completed 的 core payload 寫法）
            OrderDeliveredPayload core = new OrderDeliveredPayload(
                    orderNo,
                    buyerId != null ? String.valueOf(buyerId) : null,
                    null,
                    autoCompleteAt.toString());
            OrderOutboxEntity outbox = new OrderOutboxEntity();
            outbox.setAggregateId(orderNo);
            outbox.setEventType(OrderEventType.ORDER_DELIVERED.value());
            outbox.setExchange(MqConstants.EXCHANGE_ORDER);
            outbox.setRoutingKey(MqConstants.ROUTING_KEY_ORDER_DELIVERED);
            outbox.setStatus(OutboxEventStatus.PENDING.value());
            outbox.setRetryCount(0);
            try {
                outbox.setPayload(MAPPER.writeValueAsString(core));
            } catch (com.fasterxml.jackson.core.JsonProcessingException e) {
                // 固定結構序列化理論上不會失敗；拋出讓事務 rollback，避免狀態已轉但事件遺失
                throw new IllegalStateException("order.delivered payload 序列化失敗 orderNo=" + orderNo, e);
            }
            orderOutboxMapper.insert(outbox);
            return affected;
        });
        return rows == null ? 0 : rows;
    }
}
