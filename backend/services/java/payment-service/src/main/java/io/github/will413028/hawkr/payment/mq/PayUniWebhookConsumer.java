package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.PayUniWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.WebhookHandleResult;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.WebhookNotifyHandler;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentWebhookEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * PayUni Webhook MQ 消費者
 *
 * 消費 payuni-webhook-service 發布到 payuni.events 的 payuni.payment.uop 事件，
 * 委託現有的 PaymentNotifyHandler 處理業務邏輯。
 *
 * incoming 可靠性：MQ message 本身提供 at-least-once delivery，失敗時 NACK 重投，
 * handler 冪等保證不重複處理；超過 MAX_REDELIVERY_ATTEMPTS 次後 reject 進 DLQ。
 *
 * outgoing 付款結果事件（§5.10.1 L1'）：由 PaymentNotifyHandler 在事務內經
 * PaymentOutboxWriter.enqueueAndDispatch 寫入 outbox（與交易狀態落地、markAsProcessed
 * 原子），afterCommit 即時投遞 + 排程器兜底。
 */
@Component
public class PayUniWebhookConsumer {

    private static final ObsLogger log = ObsLogger.forClass(PayUniWebhookConsumer.class);

    private final Map<WebhookProvider, WebhookNotifyHandler> handlerMap;
    private final PaymentWebhookMapper webhookMapper;
    private final ObjectMapper objectMapper;
    private final TransactionTemplate transactionTemplate;
    private final PayUniAuditLogger auditLogger;

    public PayUniWebhookConsumer(
            List<WebhookNotifyHandler> handlers,
            PaymentWebhookMapper webhookMapper,
            ObjectMapper objectMapper,
            TransactionTemplate transactionTemplate,
            PayUniAuditLogger auditLogger
    ) {
        this.webhookMapper = webhookMapper;
        this.objectMapper = objectMapper;
        this.transactionTemplate = transactionTemplate;
        this.auditLogger = auditLogger;

        this.handlerMap = new HashMap<>();
        for (WebhookNotifyHandler handler : handlers) {
            this.handlerMap.put(handler.webhookType(), handler);
        }
    }

    /**
     * 消費 payuni.payment.uop 事件
     *
     * Event Envelope 格式（wire 鍵一律 snake_case，36 §7.4 Conformance Contract）：
     * {
     *   "event_id": "uuid",
     *   "provider": "PAYUNI",
     *   "webhook_type": "PAYMENT_UOP",
     *   "mer_id": "HAWK9",
     *   "event_key": "HAWK9:TRD...:1:1000",
     *   "occurred_at": "2026-02-16T12:34:56",
     *   "payload": { "MerTradeNo": "...", "TradeNo": "...", ... },
     *   "meta": { "version": "1.0" }
     * }
     *
     * 解析一律走 {@code PayUniWebhookEnvelope.parse}（record accessor 為 camelCase），
     * 不要自行 readTree 取 "eventId" 這類 camelCase 鍵——wire 上不存在。
     */
    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_PAYUNI_WEBHOOK,
                   containerFactory = "manualAckContainerFactory")
    public void onPayUniPaymentUop(org.springframework.amqp.core.Message message, Channel channel,
                                    @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        log.info("PayUniWebhookConsumer.received", "收到 PayUni Webhook MQ 事件");

        try {
            // 1. 統一解析 envelope（兼容標準 envelope 與裸 payload）
            PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(message.getBody());
            Map<String, String> decryptedData = env.payload();

            if (decryptedData.isEmpty()) {
                log.warn("PayUniWebhookConsumer.payload.missing", "Event Envelope 缺少 payload");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // §11.4 入向稽核：解密後 + dedup check 之前寫 payment_logs
            // 即使 webhook 被 dedup hit 也留稽核軌跡（PayUni retry 行為追蹤）
            try {
                String auditTradeNo = decryptedData.get(PayUniConstants.FIELD_MER_TRADE_NO);
                String auditProviderTradeNo = decryptedData.get(PayUniConstants.FIELD_TRADE_NO);
                auditLogger.recordInboundWebhook(auditTradeNo, auditProviderTradeNo, decryptedData);
            } catch (Exception auditEx) {
                // 雙重保護：稽核失敗不阻斷主流程
                log.warn("PayUniWebhookConsumer.audit.failed",
                        "入向稽核寫入失敗 err=" + auditEx.getMessage());
            }

            // 2. 判斷 handler（付款通知使用 PAYUNI_PAYMENT）
            WebhookNotifyHandler handler = handlerMap.get(WebhookProvider.PAYUNI);
            if (handler == null) {
                log.error("PayUniWebhookConsumer.handler.notFound", "找不到 PAYUNI_PAYMENT Handler");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 3. 提取冪等鍵
            String idempotencyKey = handler.extractIdempotencyKey(decryptedData);

            // 4. 冪等檢查
            if (idempotencyKey != null &&
                    webhookMapper.existsProcessed(WebhookProvider.PAYUNI.value(), idempotencyKey)) {
                log.info("PayUniWebhookConsumer.idempotent.skip",
                        "Webhook 已處理過（冪等跳過）", "key", idempotencyKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 5. 儲存 inbox 記錄（Transaction 外）
            Long webhookId = saveInbox(decryptedData, idempotencyKey, body);

            // 6. Transaction 內：handler → 交易狀態落地 + outbox 事件寫入 + mark processed（三者原子）
            //    §5.10.1 L1'：付款結果事件由 handler 經 PaymentOutboxWriter 在此事務內寫入 outbox
            //    並註冊 afterCommit 即時投遞
            transactionTemplate.execute(status -> {
                WebhookHandleResult r = handler.handle(decryptedData);
                webhookMapper.markAsProcessed(webhookId, r.processStatus(),
                        r.message(), DateTimeUtils.nowUtc());
                return r;
            });

            log.info("PayUniWebhookConsumer.process.success",
                    "MQ Webhook 處理完成", "key", idempotencyKey);

            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("PayUniWebhookConsumer.process.failed", "處理 PayUni Webhook MQ 事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "PayUniWebhookConsumer", e);
        }
    }

    /**
     * 儲存 inbox 記錄到 payment_webhooks
     */
    private Long saveInbox(Map<String, String> decryptedData,
                            String idempotencyKey, String rawBody) {
        String tradeNo = decryptedData.get("MerTradeNo");
        String providerTradeNo = decryptedData.get("TradeNo");

        PaymentWebhookEntity entity = new PaymentWebhookEntity();
        entity.setWebhookType(WebhookProvider.PAYUNI.value());
        entity.setTradeNo(tradeNo);
        entity.setProviderTradeNo(providerTradeNo);
        entity.setIdempotencyKey(idempotencyKey);
        entity.setRawRequest(rawBody);
        entity.setDecryptedPayload(toJson(decryptedData));
        entity.setIsValid(true);
        entity.setIsProcessed(false);
        entity.setProcessStatus(WebhookProcessStatus.PENDING.value());
        entity.setReceivedAt(DateTimeUtils.nowUtc());
        entity.setSourceIp("MQ:" + MqConstants.QUEUE_PAYMENT_PAYUNI_WEBHOOK);

        webhookMapper.insert(entity);
        return entity.getId();
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            return "{}";
        }
    }
}
