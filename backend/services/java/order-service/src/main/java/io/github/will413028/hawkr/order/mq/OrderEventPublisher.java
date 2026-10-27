package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.SellerWorkspaceEventType;
import io.github.will413028.hawkr.order.mq.dto.PayoutCompletedPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderPaidPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderShipmentBackstopPayload;
import io.github.will413028.hawkr.order.mq.dto.SellerOrderUpdatedPayload;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/**
 * 訂單事件 MQ 發布者
 *
 * 發布事件：
 * 1. order.paid              — 付款成功（觸發 Shipping Service 自動建單；改走 order_outbox 可靠投遞）
 * 2. payout.completed        — 撥款完成（#1286）
 * 3. order.shipment.create_requested / creation_failed — 建單兜底（03 §8.11）
 * 4. seller.order.updated    — 賣家工作區 realtime（41 §四之二）
 *
 * 已改走 order_outbox（例外 A）而本類不再持有對應方法者：
 * order.cancelled（#1368，OrderService.insertCancelledOutbox）、
 * order.completed / order.receipt.confirmed（issue #2294 B8，OrderCompletionTxHandler）、
 * order.delivered（ShippingEventConsumer.deliveredInTx）、
 * order.sales.accumulated（PaymentEventHandler.handlePendingPaymentOrder）——見下方移除註記。
 */
@Component
public class OrderEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(OrderEventPublisher.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ObsRabbitPublisher obsRabbitPublisher;
    private final OrderRepository orderRepository;

    public OrderEventPublisher(ObsRabbitPublisher obsRabbitPublisher, OrderRepository orderRepository) {
        this.obsRabbitPublisher = obsRabbitPublisher;
        this.orderRepository = orderRepository;
    }

    /**
     * 發布訂單付款成功事件（觸發 Shipping Service 自動建單）
     */
    public void publishOrderPaid(OrderPaidPayload payload) {
        publish(MqConstants.EXCHANGE_ORDER,
                MqConstants.ROUTING_KEY_ORDER_PAID,
                payload,
                OrderEventType.ORDER_PAID.value(),
                payload.orderNo());
    }

    // order.cancelled 改走 order_outbox 可靠投遞（#1368）：取消狀態變更同事務落 outbox，
    // 由 OrderOutboxPublisherScheduler 保證至少一次投遞（見 OrderService.insertCancelledOutbox +
    // OrderOutboxPublisher.enrichCancelled）。原事務外 best-effort 直發已移除。

    // order.completed / order.receipt.confirmed 的 best-effort 直發已移除（issue #2294 B8）：
    // 兩者改走例外 A（狀態變更同事務落 order_outbox，OrderCompletionTxHandler → OrderOutboxPublisher），
    // 移除當下全 repo 零呼叫端（含測試）。
    // ⚠️ 不要「為了對稱」把它們加回來：本類的 publish 吞掉所有例外（見本檔最下方），
    //    任何走回這條路的狀態變更就是「狀態已轉、事件 tx 外 best-effort 直發、失敗被吞」——
    //    正是 B8 要消滅的形狀。要發這兩個事件請寫 outbox，不要呼叫這裡。

    /**
     * 發布撥款完成事件（#1286）——平台撥款指示成功後通知賣家款項已入帳。
     */
    public void publishPayoutCompleted(PayoutCompletedPayload payload) {
        publish(MqConstants.EXCHANGE_ORDER,
                MqConstants.ROUTING_KEY_PAYOUT_COMPLETED,
                payload,
                OrderEventType.PAYOUT_COMPLETED.value(),
                payload.orderNo());
    }

    // order.delivered / order.sales.accumulated 的 best-effort 直發已移除（副作用失敗無訊號修復）：
    // 兩者改走**例外 A**（狀態變更同事務落 order_outbox）——
    //   order.delivered        → ShippingEventConsumer.deliveredInTx + OrderOutboxPublisher.enrichDelivered
    //   order.sales.accumulated→ PaymentEventHandler.handlePendingPaymentOrder（與 order.paid 同一事務）
    // 移除當下全 repo 零生產呼叫端。
    // ⚠️ 不要「為了對稱」把它們加回來：本類的 publish 吞掉所有例外（見本檔最下方），
    //    而這兩個事件的前置狀態轉換都是永久的（DELIVERED 的 allowed-from 不含自己、
    //    sales.accumulated 由 payment dedup 表擋住重投）——走回這條路就是永久遺失。

    /**
     * 發布重試建單事件（Backstop：付款後逾期仍無物流單，請 Shipping 冪等重建，03 §8.11）
     */
    public void publishShipmentCreateRequested(String orderNo) {
        publish(MqConstants.EXCHANGE_ORDER,
                MqConstants.ROUTING_KEY_ORDER_SHIPMENT_CREATE_REQUESTED,
                new OrderShipmentBackstopPayload(orderNo),
                OrderEventType.ORDER_SHIPMENT_CREATE_REQUESTED.value(),
                orderNo);
    }

    /**
     * 發布建單逾時取消事件（Backstop：重試上限仍無物流單，請 AfterSales 系統取消+退款，03 §8.11）
     */
    public void publishShipmentCreationFailed(String orderNo) {
        publish(MqConstants.EXCHANGE_ORDER,
                MqConstants.ROUTING_KEY_ORDER_SHIPMENT_CREATION_FAILED,
                new OrderShipmentBackstopPayload(orderNo),
                OrderEventType.ORDER_SHIPMENT_CREATION_FAILED.value(),
                orderNo);
    }

    // ==================== Seller Workspace realtime（[B-207] #1181） ====================

    /** seller.order.updated 事件來源（seller workspace SSE，41 §4b.5 source 欄位） */
    public static final String SRC_PAYUNI_WEBHOOK = "payuni_webhook";
    public static final String SRC_SHIPPING_WEBHOOK = "shipping_webhook";
    public static final String SRC_AFTER_SALE = "after_sale";
    public static final String SRC_SYSTEM = "system";

    /**
     * 發布 seller.order.updated 事件（賣家工作區 realtime invalidation，41 §四之二）。
     *
     * <ul>
     *   <li>事務同步啟用（@Transactional 路徑）→ 註冊 afterCommit hook，「DB commit 成功後才發」，
     *       避免事務 rollback 仍發幽靈事件。</li>
     *   <li>事務同步未啟用（MQ consumer manual-ack 路徑，狀態已由 mapper 各自 commit）→ eager publish。</li>
     * </ul>
     *
     * best-effort：失敗僅記 error log，不中斷主流程；漏推由前端 {@code ready.bufferMiss} + REST 全刷兜底，
     * 不引入 application-level outbox（docs/開發規範/後端服務責任邊界規範.md）。
     *
     * @param storeId 賣家 store_id surrogate —— <b>原樣進 payload，不反解析</b>。gateway SSE 的
     *                訂閱端（notification_inbox.go 的 resolveNotificationUserID）自 2026-08-30 起
     *                對賣家回傳 current store_id，topic 因此是 {@code notification:user:{store_id}}。
     * @param orderNo         訂單號
     * @param source          事件來源（SRC_* 常數）
     * @param previousStatus  前一個 order status code（可為 null）
     * @param status          最新 order status code（可為 null）
     * @param shipmentStatus  最新 shipment status code（可為 null；物流來源時帶）
     */
    /**
     * by-orderNo 便利 overload：內部反查 store_id 後 emit（供新增碼避免各處手抄
     * 「查 seller + emit」組合；既有 caller 已持有 storeId 者仍用完整版）。
     */
    public void emitSellerOrderUpdated(String orderNo, String source,
                                       String previousStatus, String status) {
        Long storeId = orderRepository.findStoreIdByOrderNo(orderNo);
        emitSellerOrderUpdated(storeId, orderNo, source, previousStatus, status, null);
    }

    /**
     * @param storeId 賣家目前的 store_id surrogate（Plan C Phase 2 起，orders.store_id 值）——
     *                <b>原樣放進 payload 的 sellerId</b>。
     *                <p>2026-08-30（commit {@code 57cefff9f}）起 gateway 的 SSE 訂閱端
     *                {@code notification_inbox.go#resolveNotificationUserID} 對賣家改回傳 current
     *                store_id，訂閱 topic 因此是 {@code notification:user:{store_id}}。本方法原本
     *                會先把 storeId 反解析成 login account_id 再組 payload，而
     *                {@code seller_workspace_channel.go#ResolveTopic} 直接拿 payload 的 sellerId 組
     *                發布 topic ⇒ 對 {@code store_id != account_id} 的賣家兩個 topic 永不相等，
     *                事件 100% 送不到且無任何 log。反解析已移除，兩端同為 store_id 空間。
     */
    public void emitSellerOrderUpdated(Long storeId, String orderNo, String source,
                                       String previousStatus, String status, String shipmentStatus) {
        if (storeId == null || storeId <= 0 || orderNo == null || orderNo.isEmpty()) {
            log.warn("OrderEventPublisher.emitSellerOrderUpdated.skip",
                    "缺 storeId 或 orderNo，跳過 seller.order.updated"
                            + " storeId=" + storeId + " orderNo=" + orderNo);
            return;
        }
        // sellerId 直接帶 store_id：訂閱端 topic 也是 store_id 空間（見上方 @param）。
        // 這裡曾經反解析成 login account_id，並為此付一次 seller-service gRPC —— 那次呼叫
        // 連同它的 fail-closed 降級一併移除，因為它現在是「把值轉換到一個沒有人訂閱的空間」。
        SellerOrderUpdatedPayload payload = new SellerOrderUpdatedPayload(
                storeId, orderNo, source, previousStatus, status, shipmentStatus,
                DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()));
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    publishSellerOrderUpdated(payload);
                }
            });
        } else {
            publishSellerOrderUpdated(payload);
        }
    }

    private void publishSellerOrderUpdated(SellerOrderUpdatedPayload payload) {
        publish(MqConstants.EXCHANGE_SELLER_WORKSPACE,
                MqConstants.ROUTING_KEY_SELLER_ORDER_UPDATED,
                payload,
                SellerWorkspaceEventType.SELLER_ORDER_UPDATED.value(),
                payload.orderNo());
    }

    private void publish(String exchange, String routingKey, Object payload, String eventType, String orderNo) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(payload);
            obsRabbitPublisher.publish(exchange, routingKey, body, eventType);
            log.info("OrderEventPublisher.publish.success",
                    "事件發布成功，eventType=" + eventType, "orderNo", orderNo);
        } catch (Exception e) {
            // best-effort：失敗僅記錄，不影響主流程
            log.error("OrderEventPublisher.publish.error",
                    "事件發布失敗，eventType=" + eventType + ", orderNo=" + orderNo, e);
        }
    }
}
