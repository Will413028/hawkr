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
 * PayUni ATM 退款「受理」async webhook 消費者（文件 §6.6 async 副本）。
 *
 * <p>payload 特徵：
 * <ul>
 *   <li>{@code EstimatedRefundDate} — PayUni 預計扣款日期</li>
 *   <li>{@code MerTradeNo} / {@code TradeNo} — 定位原退款交易</li>
 *   <li>**無** {@code RefundStatus}（那是 §6.7 匯款結果 webhook）</li>
 * </ul></p>
 *
 * <p>處理：依 tradeNo 找 active refund → {@link RefundApplicationService#handleAtmRefundAccepted} 冪等推進為 PROCESSING。
 * 與 sync return（{@code AfterSales.handleRefundPageReturn → ackAtmRefundAccepted}）互為備援；
 * markAtmRefundAccepted WHERE 有 {@code status = '0'} 保護，先到的寫、後到的 noop。</p>
 *
 * <p>冪等：與 {@link PayUniWebhookConsumer} / {@link PayUniRefundNotifyConsumer}
 * 共用 payment_webhooks 表，idempotency_key 加 {@code REFUND_ACCEPTED:} 前綴避免衝突。</p>
 */
@Component
public class PayUniRefundAcceptedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(PayUniRefundAcceptedConsumer.class);
    private static final String IDEMPOTENCY_PREFIX = "REFUND_ACCEPTED:";

    private final RefundApplicationService refundService;
    private final PayUniAuditLogger auditLogger;
    private final PaymentWebhookMapper webhookMapper;
    private final ObjectMapper objectMapper;

    public PayUniRefundAcceptedConsumer(RefundApplicationService refundService,
                                        PayUniAuditLogger auditLogger,
                                        PaymentWebhookMapper webhookMapper,
                                        ObjectMapper objectMapper) {
        this.refundService = refundService;
        this.auditLogger = auditLogger;
        this.webhookMapper = webhookMapper;
        this.objectMapper = objectMapper;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED,
                    containerFactory = "manualAckContainerFactory")
    public void onRefundAccepted(org.springframework.amqp.core.Message message, Channel channel,
                                  @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String rawBody = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("PayUniRefundAcceptedConsumer.received", "收到 PayUni 退款受理 webhook 事件");

        try {
            PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(message.getBody());
            Map<String, String> payload = env.payload();

            if (payload.isEmpty()) {
                log.warn("PayUniRefundAcceptedConsumer.payload.missing", "Envelope 缺少 payload");
                channel.basicAck(deliveryTag, false);
                return;
            }

            String merTradeNo = payload.get("MerTradeNo");
            String providerTradeNo = payload.get("TradeNo");
            String estimatedRefundDate = payload.get("EstimatedRefundDate");
            String msg = payload.getOrDefault("Message", "");

            try {
                auditLogger.recordInboundWebhook(merTradeNo, providerTradeNo, payload);
            } catch (Exception auditEx) {
                log.warn("PayUniRefundAcceptedConsumer.audit.failed",
                        "入向稽核寫入失敗 err=" + auditEx.getMessage());
            }

            // 用 MerTradeNo 定位（payment_refunds.trade_no 存的是 MerTradeNo，非 PayUni UNi 序號）
            if (merTradeNo == null || merTradeNo.isBlank()) {
                log.warn("PayUniRefundAcceptedConsumer.noMerTradeNo",
                        "Payload 缺少 MerTradeNo，無法定位退款 row，丟棄");
                channel.basicAck(deliveryTag, false);
                return;
            }

            String idempotencyKey = buildIdempotencyKey(merTradeNo, providerTradeNo, estimatedRefundDate);
            if (webhookMapper.existsProcessed(WebhookProvider.PAYUNI.value(), idempotencyKey)) {
                log.info("PayUniRefundAcceptedConsumer.idempotent.skip",
                        "退款受理 webhook 已處理過（冪等跳過）", "key", idempotencyKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            Long webhookId = saveInbox(merTradeNo, providerTradeNo, idempotencyKey, payload, rawBody);

            try {
                // 用 MerTradeNo lookup active refund（payment_refunds.trade_no = MerTradeNo）
                refundService.handleAtmRefundAccepted(merTradeNo, estimatedRefundDate, msg);
                webhookMapper.markAsProcessed(webhookId, WebhookProcessStatus.SUCCESS.value(),
                        null, DateTimeUtils.nowUtc());
            } catch (Exception handleEx) {
                webhookMapper.markAsProcessed(webhookId, WebhookProcessStatus.FAILED.value(),
                        handleEx.getMessage(), DateTimeUtils.nowUtc());
                throw handleEx;
            }

            log.info("PayUniRefundAcceptedConsumer.process.success",
                    "ATM 退款受理 webhook 處理完成",
                    "tradeNo", providerTradeNo);

            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("PayUniRefundAcceptedConsumer.process.failed",
                    "處理 PayUni 退款受理 webhook 事件失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "PayUniRefundAcceptedConsumer", e);
        }
    }

    private String buildIdempotencyKey(String merTradeNo, String providerTradeNo, String estimatedRefundDate) {
        return IDEMPOTENCY_PREFIX
                + safe(merTradeNo) + ":"
                + safe(providerTradeNo) + ":"
                + safe(estimatedRefundDate);
    }

    private String safe(String v) {
        return v == null ? "" : v;
    }

    private Long saveInbox(String merTradeNo, String providerTradeNo, String idempotencyKey,
                            Map<String, String> payload, String rawBody) {
        PaymentWebhookEntity entity = new PaymentWebhookEntity();
        entity.setWebhookType(WebhookProvider.PAYUNI.value());
        entity.setTradeNo(merTradeNo);
        entity.setProviderTradeNo(providerTradeNo);
        entity.setIdempotencyKey(idempotencyKey);
        entity.setRawRequest(rawBody);
        entity.setDecryptedPayload(toJson(payload));
        entity.setIsValid(true);
        entity.setIsProcessed(false);
        entity.setProcessStatus(WebhookProcessStatus.PENDING.value());
        entity.setReceivedAt(DateTimeUtils.nowUtc());
        entity.setSourceIp("MQ:" + MqConstants.QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED);
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
