package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentEventType;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.mq.OutboxEventFactory;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;

/**
 * §2A.6.5 Rescue 退款消費者。
 * <p>
 * 監聽 {@code payment.rescue.refund_initiated} event，
 * 對 refund target row 執行退款：
 * <ul>
 *   <li>信用卡 / Apple Pay / Google Pay → 呼叫 PayUni {@code /api/trade/close}（即時退款）→ 更新副 row status=CLOSED</li>
 *   <li>ATM → 不打 PayUni（需使用者填銀行資訊），log warn 並 ACK，等 P11/P12 PaymentNotificationService 寄通知信</li>
 * </ul>
 * <p>
 * MVP 範圍：不做重試計數，失敗 → DLQ。
 * 不寫 refund_transactions row（用 row.status 翻轉表達退款結果，與既有 PaymentTransaction.CLOSED 語意對齊）。
 */
@Component
public class RescueRefundConsumer {

    private static final ObsLogger log = ObsLogger.forClass(RescueRefundConsumer.class);

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentClient paymentClient;
    private final ObjectMapper objectMapper;
    private final OutboxEventMapper outboxMapper;
    // §05 §2A.6.5 Tx 載體規定（對應 §40 §2.5）：persistRefundResultBody 必須由 caller
    // 透過 txTemplate 包覆，禁止用 @Transactional 標於同 class self-invocation method
    private final TransactionTemplate transactionTemplate;

    public RescueRefundConsumer(PaymentTransactionRepository transactionRepository,
                                PaymentClient paymentClient,
                                ObjectMapper objectMapper,
                                OutboxEventMapper outboxMapper,
                                TransactionTemplate transactionTemplate) {
        this.transactionRepository = transactionRepository;
        this.paymentClient = paymentClient;
        this.objectMapper = objectMapper;
        this.outboxMapper = outboxMapper;
        this.transactionTemplate = transactionTemplate;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_RESCUE_REFUND,
            containerFactory = "manualAckContainerFactory")
    public void onRescueRefundInitiated(Message message, Channel channel,
                                         @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("RescueRefundConsumer.received", "收到 rescue 退款 event body=" + body);

        try {
            @SuppressWarnings("unchecked")
            Map<String, Object> payload = objectMapper.readValue(body, Map.class);

            // payload key 一律 snake_case（與全專案 PaymentDomainEvent / PaymentEventConsumer 慣例一致）
            String tradeNo = asString(payload.get("trade_no"));
            String orderId = asString(payload.get("order_id"));
            String merId = asString(payload.get("mer_id"));
            String providerTradeNo = asString(payload.get("provider_trade_no"));
            String paymentTypeValue = asString(payload.get("payment_type"));
            long amount = asLong(payload.get("amount"), "amount");
            String rescueReason = asString(payload.get("rescue_reason"));

            log.info("RescueRefundConsumer.parsed",
                    "trade_no=" + tradeNo + " refund_target_provider_trade_no=" + providerTradeNo
                            + " amount=" + amount + " type=" + paymentTypeValue + " reason=" + rescueReason);

            // 依 provider_trade_no 精準鎖定要退款的 row（refundTarget 是新主或副，視情況）
            Optional<PaymentTransaction> targetOpt = transactionRepository.findByProviderTradeNo(providerTradeNo);
            if (targetOpt.isEmpty()) {
                log.error("RescueRefundConsumer.target.notFound",
                        "找不到 refund target row provider_trade_no=" + providerTradeNo + " trade_no=" + tradeNo);
                channel.basicAck(deliveryTag, false);  // 邏輯錯誤不重投
                return;
            }
            PaymentTransaction target = targetOpt.get();

            PaymentMethod paymentType = target.paymentType();
            if (paymentType == PaymentMethod.ATM) {
                // ATM 退款需使用者填銀行資訊，本 MVP 不寄通知信，log warn 並 ACK
                log.warn("RescueRefundConsumer.atm.requiresManual",
                        "ATM rescue 退款需使用者填銀行資訊（PaymentNotificationService 待 P11 實作） trade_no="
                                + tradeNo + " row_id=" + target.id());
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 信用卡 / Apple Pay / Google Pay → 呼叫 PayUni close
            String effMerId = merId != null && !merId.isBlank() ? merId : target.merId();
            executeCreditRefund(target, effMerId, providerTradeNo, amount, rescueReason);
            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("RescueRefundConsumer.process.failed",
                    "rescue 退款處理失敗 err=" + e.getMessage(), e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "RescueRefundConsumer", e);
        }
    }

    // §05 §2A.6.5 步驟 3-6 entry：
    // - 步驟 3 PayUni /api/trade/close 在 tx 外（外部 HTTP RPC 不應佔 DB 連線）
    // - 步驟 3.5 rescue.refund_completed payload 序列化在 tx 外（純 CPU 工作不應佔 DB 連線）；
    //   序列化失敗直接 throw 由 onRescueRefundInitiated outer catch → MqErrorHandler → DLQ retry
    // - 步驟 4-5 markRescueRefunded UPDATE + outbox INSERT 透過 txTemplate 包覆為單一短 tx
    //   保證原子化（修 #1122：避免 tx body 內 catch+return 造成 UPDATE 落地但 outbox INSERT 漏寫）
    private void executeCreditRefund(PaymentTransaction target,
                                     String merId,
                                     String providerTradeNo,
                                     long amount,
                                     String rescueReason) {
        log.info("RescueRefundConsumer.payuniClose.start",
                "呼叫 PayUni /api/trade/close trade_no=" + target.tradeNo()
                        + " provider_trade_no=" + providerTradeNo + " amount=" + amount);

        // PayUni close API（CloseType=2 退款），audit logging 由 PayUniHttpClient 自動寫入 payment_logs
        // refundNo 用 trade_no + "-rescue" 做冪等鍵，避免 PayUni 端同 refundNo 重複退款
        String refundNo = target.tradeNo() + "-rescue";
        PayUniApiResponse response = paymentClient.refund(merId, providerTradeNo, refundNo, amount);

        // §2A.6.5 PayUni 已退款的冪等處理：
        // PayUni 對「同 trade_no 重複呼叫 /api/trade/close」會回失敗訊息含「目前已退款」。
        // 此情境視為冪等成功（PayUni 端已是退款完成狀態），本地照樣標 row CLOSED。
        boolean alreadyRefunded = !response.isSuccess()
                && response.getErrorMessage() != null
                && response.getErrorMessage().contains("已退款");

        if (response.isSuccess() || alreadyRefunded) {
            LocalDateTime closedAt = DateTimeUtils.nowUtc();
            // §2A.6.5 步驟 3.5：序列化在 tx 外，失敗 throw → outer MqErrorHandler → DLQ
            String payloadJson = serializeRescueCompletedPayload(target, rescueReason, closedAt);
            transactionTemplate.executeWithoutResult(status ->
                    persistRefundResultBody(target, rescueReason, closedAt, payloadJson));
        } else {
            log.error("RescueRefundConsumer.payuniClose.failed",
                    "PayUni close 失敗 trade_no=" + target.tradeNo()
                            + " errMsg=" + response.getErrorMessage());
            throw new RuntimeException("PayUni close failed: " + response.getErrorMessage());
        }
    }

    // §05 §2A.6.5 Tx body — 由 caller 透過 txTemplate.executeWithoutResult(...) 包覆。
    // 禁止標 @Transactional（同 class self-invocation 會繞過 Spring AOP CGLIB proxy 讓 annotation 失效）。
    // 內含 markRescueRefunded UPDATE + outbox INSERT 兩個 DB write，需原子化。
    // payloadJson 為步驟 3.5 預序列化結果，tx body 內不再做任何 serialize / catch+return。
    private void persistRefundResultBody(PaymentTransaction target,
                                         String rescueReason,
                                         LocalDateTime closedAt,
                                         String payloadJson) {
        // §2A.6.5 用 markRescueRefunded 而非 updateToClosed：
        // updateToClosed WHERE status='0' 對 PAID row 不適用
        // markRescueRefunded WHERE id=? AND status='1' 才能正確標 PAID → CLOSED
        int updated = transactionRepository.markRescueRefunded(target.id(), closedAt);
        log.info("RescueRefundConsumer.payuniClose.success",
                "rescue 退款成功 trade_no=" + target.tradeNo() + " row_id=" + target.id() + " updated=" + updated);
        if (updated == 0) {
            log.error("RescueRefundConsumer.markClosed.failed",
                    "PayUni close 成功但 row 標 CLOSED 失敗 row_id=" + target.id()
                            + "（人工介入：PayUni 已退但本地未記錄，不發 completed event 避免幻覺通知）");
            return;
        }
        // §2A.6.6 B-2：rescue 退款完成 → INSERT outbox_events 通知 notification-service
        // 走 outbox pattern：與 markRescueRefunded UPDATE 同 tx 原子化 + 由 OutboxEventPublisherScheduler 統一派送
        insertRescueCompletedOutbox(target, closedAt, payloadJson, rescueReason);
    }

    /**
     * §2A.6.6 B-2 序列化 rescue 退款完成事件 payload（在 tx 外執行）。
     * <p>
     * 失敗（{@link JsonProcessingException}）直接 throw {@link RuntimeException}，
     * 由 {@link #onRescueRefundInitiated} outer catch 接 → {@link MqErrorHandler#handleFailure} → DLQ retry。
     * <p>
     * 禁止在 tx body 內 serialize + catch + return（會造成 markRescueRefunded UPDATE 落地但 outbox INSERT 漏寫）。
     * <p>
     * Payload 結構見 05 號設計書 §2A.6.6。
     */
    private String serializeRescueCompletedPayload(PaymentTransaction target,
                                                   String rescueReason,
                                                   LocalDateTime refundedAt) {
        Map<String, Object> payload = new LinkedHashMap<>();
        // 業務欄位（snake_case，與 rescue 系列其他事件 payload 對齊：DuplicatePaymentService.publishRescueRefundInitiated）
        payload.put("trade_no", target.tradeNo());
        payload.put("order_id", target.orderId());
        payload.put("buyer_id", target.buyerId());
        payload.put("mer_id", target.merId());
        payload.put("provider_trade_no", target.providerTradeNo());
        payload.put("amount", target.amount());
        payload.put("payment_type", target.paymentType() != null ? target.paymentType().value() : "");
        payload.put("rescue_reason", rescueReason);
        payload.put("refunded_at", refundedAt.toString());
        payload.put("orderNo", target.orderId());
        // 事件 envelope metadata（36 §7.4 Conformance Contract：event_id 為 UUID v4 必含欄位）
        OutboxEventFactory.applyEnvelopeMetadata(payload,
                PaymentEventType.PAYMENT_RESCUE_REFUND_COMPLETED.value(), "payment",
                target.tradeNo(), refundedAt.toString());

        try {
            return objectMapper.writeValueAsString(payload);
        } catch (JsonProcessingException e) {
            // 在 tx 外 throw，避免 tx body 內 catch+return 造成原子化破口
            throw new RuntimeException("rescue completed payload serialize failed trade_no=" + target.tradeNo(), e);
        }
    }

    /**
     * §2A.6.6 B-2 rescue 退款完成事件 outbox INSERT（在 tx 內執行）。
     * <p>
     * 與 markRescueRefunded UPDATE 同 tx 原子化；
     * 由 {@link io.github.will413028.hawkr.payment.scheduler.OutboxEventPublisherScheduler}
     * 5 秒輪詢派送到 RabbitMQ {@code payment.events / payment.rescue.refund_completed}，
     * notification-service 訂閱後推播給使用者。
     */
    private void insertRescueCompletedOutbox(PaymentTransaction target,
                                             LocalDateTime refundedAt,
                                             String payloadJson,
                                             String rescueReason) {
        OutboxEventEntity entity = OutboxEventFactory.newPendingEntity(
                MqConstants.EXCHANGE_PAYMENT,
                PaymentEventType.PAYMENT_RESCUE_REFUND_COMPLETED.value(),
                MqConstants.ROUTING_KEY_PAYMENT_RESCUE_REFUND_COMPLETED,
                payloadJson, refundedAt);
        outboxMapper.insert(entity);

        log.info("RescueRefundConsumer.completed.queued",
                "rescue 完成 outbox 已建立 trade_no=" + target.tradeNo()
                        + " outbox_id=" + entity.getId()
                        + " target_id=" + target.id() + " reason=" + rescueReason);
    }

    private static String asString(Object o) {
        return o == null ? null : String.valueOf(o);
    }

    /**
     * fail-loud：缺失 / 非數字一律拋例外（由 onRescueRefundInitiated outer catch →
     * {@link MqErrorHandler} → DLQ retry）。禁止吞成 0——本值直接作為退款金額送 PayUni 退款 API，
     * 吞 0 會對真實交易發出金額 0 的退款請求。
     */
    private static long asLong(Object o, String field) {
        if (o == null) {
            throw new IllegalStateException("rescue 退款 payload 缺少必要數值欄位: " + field);
        }
        if (o instanceof Number n) return n.longValue();
        try {
            return Long.parseLong(o.toString());
        } catch (NumberFormatException e) {
            throw new IllegalStateException(
                    "rescue 退款 payload 欄位 " + field + " 非合法數字: '" + o + "'", e);
        }
    }
}
