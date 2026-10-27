package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.PayUniWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentWebhookEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Map;

/**
 * PayUni ATM 退款結果 webhook 消費者（收斂規範 §9.5）。
 *
 * <p>消費 {@code payuni.events} 交換機上 routing key {@code payuni.refund.notify} 的訊息，
 * 更新 {@code payment_refunds} SoT 並在 {@code payment.events} 交換機上發佈退款結果 event
 * （routing key {@code refund.succeeded} / {@code refund.failed}）給 AfterSales 訂閱者。
 * AfterSales 端的 queue 名為 {@code aftersales.payment.refund.result}，那是 queue 不是 routing key。</p>
 *
 * <p>payload 欄位（由 SDK {@code PayUniWebhookParser} 解析為原始欄位）：
 * <ul>
 *   <li>{@code MerTradeNo} — 商店退款關聯的原付款 trade_no</li>
 *   <li>{@code RefundStatus} — 1=成功到帳 / 2=退件失敗（收斂規範 §4.4 雙階段雙寫：async 寫 inner）</li>
 *   <li>{@code BankDate} — YYYY-MM-DD 或 "-"（失敗時）</li>
 *   <li>{@code Message} — 失敗原因描述</li>
 * </ul>
 * </p>
 *
 * <p>冪等：與 PayUniWebhookConsumer 共用 payment_webhooks 表，idempotency_key 加 {@code REFUND:} 前綴避免衝突。</p>
 */
@Component
public class PayUniRefundNotifyConsumer {

    private static final ObsLogger log = ObsLogger.forClass(PayUniRefundNotifyConsumer.class);
    private static final String IDEMPOTENCY_PREFIX = "REFUND:";

    private final RefundApplicationService refundService;
    private final PayUniAuditLogger auditLogger;
    private final PaymentWebhookMapper webhookMapper;
    private final ObjectMapper objectMapper;

    public PayUniRefundNotifyConsumer(RefundApplicationService refundService,
                                      PayUniAuditLogger auditLogger,
                                      PaymentWebhookMapper webhookMapper,
                                      ObjectMapper objectMapper) {
        this.refundService = refundService;
        this.auditLogger = auditLogger;
        this.webhookMapper = webhookMapper;
        this.objectMapper = objectMapper;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY,
                    containerFactory = "manualAckContainerFactory")
    public void onRefundNotify(org.springframework.amqp.core.Message message, Channel channel,
                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String rawBody = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("PayUniRefundNotifyConsumer.received", "收到 PayUni 退款 webhook 事件");

        try {
            PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(message.getBody());
            Map<String, String> payload = env.payload();

            if (payload.isEmpty()) {
                log.warn("PayUniRefundNotifyConsumer.payload.missing", "Envelope 缺少 payload");
                channel.basicAck(deliveryTag, false);
                return;
            }

            String tradeNo = payload.get("MerTradeNo");
            String providerTradeNo = payload.get("TradeNo");
            String refundStatus = payload.get("RefundStatus");
            String bankDate = payload.get("BankDate");
            String msg = payload.getOrDefault("Message", "");

            // 稽核：留下 webhook 原始軌跡（即使 dedup hit 也寫，不阻斷主流程）
            try {
                auditLogger.recordInboundWebhook(tradeNo, providerTradeNo, payload);
            } catch (Exception auditEx) {
                log.warn("PayUniRefundNotifyConsumer.audit.failed",
                        "入向稽核寫入失敗 err=" + auditEx.getMessage());
            }

            if (tradeNo == null || tradeNo.isBlank()) {
                log.warn("PayUniRefundNotifyConsumer.noTradeNo",
                        "Payload 缺少 MerTradeNo，無法定位退款 row，丟棄");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 冪等鍵：REFUND:MerTradeNo:TradeNo:RefundStatus:BankDate
            String idempotencyKey = buildIdempotencyKey(tradeNo, providerTradeNo, refundStatus, bankDate);
            if (webhookMapper.existsProcessed(WebhookProvider.PAYUNI.value(), idempotencyKey)) {
                log.info("PayUniRefundNotifyConsumer.idempotent.skip",
                        "退款 webhook 已處理過（冪等跳過）", "key", idempotencyKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 先寫 inbox（事務外）
            Long webhookId = saveInbox(tradeNo, providerTradeNo, idempotencyKey, payload, rawBody);

            try {
                refundService.handleAtmRefundWebhook(tradeNo, refundStatus, bankDate, msg);
                webhookMapper.markAsProcessed(webhookId, WebhookProcessStatus.SUCCESS.value(),
                        null, DateTimeUtils.nowUtc());
            } catch (Exception handleEx) {
                webhookMapper.markAsProcessed(webhookId, WebhookProcessStatus.FAILED.value(),
                        handleEx.getMessage(), DateTimeUtils.nowUtc());
                throw handleEx;
            }

            log.info("PayUniRefundNotifyConsumer.process.success",
                    "ATM 退款 webhook 處理完成 refundStatus=" + refundStatus,
                    "tradeNo", tradeNo);

            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("PayUniRefundNotifyConsumer.process.failed",
                    "處理 PayUni 退款 webhook 事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "PayUniRefundNotifyConsumer", e);
        }
    }

    private String buildIdempotencyKey(String tradeNo, String providerTradeNo,
                                        String refundStatus, String bankDate) {
        return IDEMPOTENCY_PREFIX
                + safe(tradeNo) + ":"
                + safe(providerTradeNo) + ":"
                + safe(refundStatus) + ":"
                + safe(bankDate);
    }

    private String safe(String v) {
        return v == null ? "" : v;
    }

    private Long saveInbox(String tradeNo, String providerTradeNo, String idempotencyKey,
                            Map<String, String> payload, String rawBody) {
        PaymentWebhookEntity entity = new PaymentWebhookEntity();
        entity.setWebhookType(WebhookProvider.PAYUNI.value());
        entity.setTradeNo(tradeNo);
        entity.setProviderTradeNo(providerTradeNo);
        entity.setIdempotencyKey(idempotencyKey);
        entity.setRawRequest(rawBody);
        entity.setDecryptedPayload(toJson(payload));
        entity.setIsValid(true);
        entity.setIsProcessed(false);
        entity.setProcessStatus(WebhookProcessStatus.PENDING.value());
        entity.setReceivedAt(DateTimeUtils.nowUtc());
        entity.setSourceIp("MQ:" + MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY);
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
