package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderEventType;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.mq.dto.OrderCompletedPayload;
import io.github.will413028.hawkr.order.mq.dto.OrderReceiptConfirmedPayload;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 訂單完成的交易性處理（03 §5.4 階段 1）。
 *
 * 從 OrderService 獨立出來，確保 Spring @Transactional proxy 正常運作
 * （原 OrderService.completeOrder → this.completeOrderInTx 為 self-invocation，
 * 不經 proxy，@Transactional 實際失效——「CAS + 建撥款任務」曾非原子）。
 *
 * 同一事務內：DELIVERED(40) → COMPLETED(50) CAS + INSERT grant_payment_tasks（PENDING），
 * 兩者原子；PayUni 接管分流不在建立時判斷（一律 PENDING），由 Worker 依 payment-service 回應的
 * grant_outcome=PROVIDER_TOOK_OVER 收斂（03 §3.11.2 單一收斂點）。
 *
 * <p><b>搶佔語意（issue #2294 B8 — 例外 A：同 tx outbox）</b>
 *
 * <p>{@code completeDeliveredOrderByStatus}（{@code SET status='50' WHERE order_no=? AND
 * status='40'}）是<b>業務狀態機的永久轉換</b>：{@code '50'} 是終態，CAS 成功後該列永久離開
 * {@code findAutoCompleteOrders} 的候選集合，沒有任何時間欄能讓它回來 ⇒
 * 依《多副本排程搶佔規範》§二 通則 0 第 4 步，這是永久旗標，必須改。
 *
 * <p>三條出路中選 <b>例外 A（同 tx outbox）</b>：
 * <ul>
 *   <li><b>例外 B 不適用</b>：被移到 publish 之後的那一句是業務狀態轉換本身（規範 §四
 *       「例外 B 的適用邊界」）——換順序會在 CAS 回 0 時發出一則「訂單已完成」但訂單其實
 *       還在 DELIVERED 的事件，且不可收斂。</li>
 *   <li><b>改 lease 不適用</b>：狀態不能「租」。</li>
 *   <li>order-service <b>已有</b> {@code order_outbox}（{@code order.cancelled} 已經走它）
 *       ⇒ 例外 A 零基建成本、零 migration，且有同 repo 的現成前例可抄。</li>
 * </ul>
 *
 * <p>⚠️ <b>舊實作的破口</b>：{@code order.completed} 與 {@code order.receipt_confirmed}
 * 原本在 {@code OrderService.completeOrder} 的 <b>tx 之外</b> best-effort 直發，
 * 而 {@code OrderEventPublisher.publish} <b>吞掉所有例外</b>
 * ⇒ 訂單已是 COMPLETED、錢照撥（撥款任務在 tx 內建立所以安全），但
 * <b>買家沒收到通知、CRM 統計短少、物流單永遠沒被標記收貨確認</b>，且無告警無重試。
 * order-service prod {@code replicas=2}、{@code AutoCompleteScheduler} 1 小時一跑。
 *
 * <p><b>tx 內零 gRPC</b>：兩筆事件都只寫「core payload」（本地事實），
 * {@code notifyBuyerAccountId} / {@code notifySellerAccountId} 的身分反解<b>延到派送時</b>
 * 由 {@link io.github.will413028.hawkr.order.mq.OrderOutboxPublisher} enrich —— 與
 * {@code order.paid} / {@code order.cancelled} 完全同形（規範 §四 出路 4 的硬性邊界：
 * tx 內不得有任何網路呼叫）。
 */
@Service
public class OrderCompletionTxHandler {

    private static final ObsLogger log = ObsLogger.forClass(OrderCompletionTxHandler.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final OrderRepository orderRepository;
    private final GrantPaymentTaskRepository grantPaymentTaskRepository;
    private final OrderEventPublisher eventPublisher;
    private final OrderOutboxMapper orderOutboxMapper;

    public OrderCompletionTxHandler(OrderRepository orderRepository,
                                    GrantPaymentTaskRepository grantPaymentTaskRepository,
                                    OrderEventPublisher eventPublisher,
                                    OrderOutboxMapper orderOutboxMapper) {
        this.orderRepository = orderRepository;
        this.grantPaymentTaskRepository = grantPaymentTaskRepository;
        this.eventPublisher = eventPublisher;
        this.orderOutboxMapper = orderOutboxMapper;
    }

    /**
     * 完成訂單（TX 內）：DELIVERED(40) → COMPLETED(50) + 建立撥款任務
     * + {@code order.completed} / {@code order.receipt_confirmed} 落 outbox（同事務原子）。
     *
     * @param receiptPhotoKeysJson buyer 上傳照片 objectKey 轉為 JSON 字串（null 表示無）
     * @return true=成功完成, false=冪等跳過（非 DELIVERED 或已完成）
     */
    @Transactional
    public boolean completeOrderInTx(String orderNo, String confirmType, String receiptPhotoKeysJson) {
        int rows = orderRepository.completeDeliveredOrder(orderNo, confirmType, receiptPhotoKeysJson);
        if (rows == 0) {
            log.info("OrderCompletionTxHandler.completeOrder.idempotent",
                    "訂單非 DELIVERED 狀態或已完成，跳過", "orderNo", orderNo);
            return false;
        }

        // 建立撥款指示任務：一律 PENDING（03 §5.4 階段 1）。曾發生退款的交易由 Worker 發動撥款後
        // 依 payment-service 回應 grant_outcome=PROVIDER_TOOK_OVER 收斂為 PROVIDER_AUTO，
        // 建立時不做啟發式分流（避免與退款事件的競態產生兩套判準）。
        GrantPaymentTaskEntity task = new GrantPaymentTaskEntity();
        task.setTaskId(TsidUtil.generate());
        task.setOrderNo(orderNo);
        task.setConfirmType(confirmType);
        task.setStatus(OrderConstants.TASK_STATUS_PENDING);
        task.setRetryCount(0);
        task.setMaxRetryCount(OrderConstants.DEFAULT_MAX_RETRY_COUNT);
        grantPaymentTaskRepository.insert(task);

        // issue #2294 B8（例外 A）：兩則事件與上面的 CAS 同事務落 order_outbox，
        // 取代原本 OrderService.completeOrder 在 tx 外的 best-effort 直發。
        insertCompletionOutbox(orderNo, confirmType);

        log.info("OrderCompletionTxHandler.completeOrder.success",
                "訂單完成，撥款任務已建立，confirmType=" + confirmType, "orderNo", orderNo);
        // [B-207] #1181：賣家工作區 realtime — 完成（自動完成 + 確認收貨共用此 sink，afterCommit）
        eventPublisher.emitSellerOrderUpdated(orderNo, OrderEventPublisher.SRC_SYSTEM,
                OrderStatus.DELIVERED.value(), OrderStatus.COMPLETED.value());
        return true;
    }

    /**
     * 事務內落 {@code order.completed} ＋ {@code order.receipt_confirmed} 至發件匣（issue #2294 B8）。
     *
     * <p>只讀本地 orders 一列組「核心 payload」；notification 用的
     * {@code notifyBuyerAccountId} / {@code notifySellerAccountId} 留 null，
     * 由 {@link io.github.will413028.hawkr.order.mq.OrderOutboxPublisher} 在派送時 enrich 反解析
     * （鏡像 {@code order.paid} / {@code order.cancelled}，避免在完成事務內做 gRPC）。
     *
     * <p>序列化失敗拋例外讓事務 rollback ⇒ 狀態沒轉、撥款任務沒建、事件沒發，
     * 下一 tick（{@code AutoCompleteScheduler} 1 小時 / 買家再按一次確認收貨）重來。
     * 查無訂單同樣拋例外——CAS 剛剛才成功，此時讀不到代表資料層出了更嚴重的問題，
     * 不能靜默跳過（那會退回「狀態已轉、事件遺失」的原缺陷）。
     */
    private void insertCompletionOutbox(String orderNo, String confirmType) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            throw new IllegalStateException(
                    "訂單完成 CAS 已成功但查無訂單，無法組 order.completed payload orderNo=" + orderNo);
        }

        OrderCompletedPayload completed = new OrderCompletedPayload(
                orderNo,
                String.valueOf(order.getBuyerId()),
                null,                                   // notifyBuyerAccountId：派送時 enrich 反解析
                String.valueOf(order.getStoreId()),     // storeId：本地事實（crm 統計以此為 key）
                null,                                   // notifySellerAccountId：派送時 enrich 反解析
                String.valueOf(order.getTotalActualAmount()),
                confirmType,
                DateTimeUtils.nowUtc().toString());

        insertOutbox(orderNo, OrderEventType.ORDER_COMPLETED.value(),
                MqConstants.ROUTING_KEY_ORDER_COMPLETED, completed);

        insertOutbox(orderNo, OrderEventType.ORDER_RECEIPT_CONFIRMED.value(),
                MqConstants.ROUTING_KEY_ORDER_RECEIPT_CONFIRMED,
                new OrderReceiptConfirmedPayload(orderNo, confirmType));
    }

    private void insertOutbox(String orderNo, String eventType, String routingKey, Object corePayload) {
        OrderOutboxEntity outbox = new OrderOutboxEntity();
        outbox.setAggregateId(orderNo);
        outbox.setEventType(eventType);
        outbox.setExchange(MqConstants.EXCHANGE_ORDER);
        outbox.setRoutingKey(routingKey);
        outbox.setStatus(OutboxEventStatus.PENDING.value());
        outbox.setRetryCount(0);
        try {
            outbox.setPayload(MAPPER.writeValueAsString(corePayload));
        } catch (JsonProcessingException e) {
            // 固定結構序列化理論上不會失敗；拋出讓完成事務 rollback，避免狀態已轉但事件遺失
            throw new IllegalStateException(
                    eventType + " payload 序列化失敗 orderNo=" + orderNo, e);
        }
        orderOutboxMapper.insert(outbox);
    }
}
