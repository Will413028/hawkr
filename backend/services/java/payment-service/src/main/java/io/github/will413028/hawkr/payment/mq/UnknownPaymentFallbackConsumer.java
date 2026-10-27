package io.github.will413028.hawkr.payment.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
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
import java.util.Optional;

/**
 * §2.2 業務 pivot (2026-05-14) — UNKNOWN webhook 保守失敗處理（付款側）。
 *
 * <p>訂閱 {@code payuni.events} exchange 的 {@code payuni.unknown.fallback} routing key，
 * 與 aftersales 端 UnknownRefundFallbackConsumer 並行訂閱同一 event。</p>
 *
 * <p>邏輯：
 * <ul>
 *   <li>用 payload.MerID 查 {@code payment_transactions} 最近一筆 PENDING (status=0) row（過濾 is_duplicate=false）</li>
 *   <li>找到：stamp {@code provider_message}（樂觀鎖 WHERE provider_message IS NULL 避免覆蓋 reconcile）</li>
 *   <li>不主動推 status：由既有 {@link io.github.will413028.hawkr.payment.scheduler.PendingPaymentReconcileService}
 *       透過 PayUni {@code /api/trade/query} 補對齊（保留 single source of truth）</li>
 *   <li>找不到：log.warn 後 ACK</li>
 *   <li>不論結果：log.error 觸發 Obs alert</li>
 * </ul>
 *
 * <p>不違反 docs/開發規範/後端服務責任邊界規範.md：webhook-driven 一次處理，非定時 reconcile worker。</p>
 */
@Component
public class UnknownPaymentFallbackConsumer {

    private static final ObsLogger log = ObsLogger.forClass(UnknownPaymentFallbackConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final int MAX_MESSAGE_LENGTH = 500;

    private final PaymentTransactionRepository transactionRepository;

    public UnknownPaymentFallbackConsumer(PaymentTransactionRepository transactionRepository) {
        this.transactionRepository = transactionRepository;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK,
            containerFactory = "manualAckContainerFactory")
    public void onUnknownFallback(Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            JsonNode envelope = MAPPER.readTree(body);
            String merId = envelope.path("mer_id").asText("");
            String eventKey = envelope.path("event_key").asText("");
            JsonNode payload = envelope.path("payload");
            String payuniStatus = payload.path("Status").asText("");
            String payuniMessage = payload.path("Message").asText("");

            log.error("UnknownPaymentFallbackConsumer.received",
                    "收到 UNKNOWN webhook fallback merId=" + merId + " status=" + payuniStatus
                            + " message=" + payuniMessage + " eventKey=" + eventKey,
                    "eventKey", eventKey);

            if (merId == null || merId.isBlank()) {
                log.warn("UnknownPaymentFallbackConsumer.noMerId.skip",
                        "payload 無 MerID 無法綁定任何 transaction，skip eventKey=" + eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            Optional<PaymentTransaction> optTx = transactionRepository.findLatestPendingByMerId(merId);
            if (optTx.isEmpty()) {
                log.warn("UnknownPaymentFallbackConsumer.noPendingTx",
                        "MerID=" + merId + " 找不到 PENDING transaction，"
                                + "讓 aftersales fallback consumer 接手或維運處理 eventKey=" + eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            PaymentTransaction tx = optTx.get();
            // 組合 stamp 內容：Status + Message（PayUni 原話），無內容時用通用文案
            String composed = composeMessage(payuniStatus, payuniMessage);

            int rows = transactionRepository.stampUnknownFallbackMessage(tx.id(), composed);
            if (rows == 0) {
                // 已有 provider_message（reconcile 已 stamp 過）或 status 已變化 → 不覆蓋
                log.info("UnknownPaymentFallbackConsumer.stamp.skip",
                        "transaction provider_message 已存在或 status 變化，不覆蓋 txId=" + tx.id());
            } else {
                log.warn("UnknownPaymentFallbackConsumer.stamped",
                        "已 stamp provider_message 到 PENDING transaction txId=" + tx.id()
                                + " merId=" + merId + " msg=" + composed);
            }

            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("UnknownPaymentFallbackConsumer.error", "處理 UNKNOWN fallback 失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "UnknownPaymentFallbackConsumer", e);
        }
    }

    private String composeMessage(String status, String message) {
        StringBuilder sb = new StringBuilder("UNKNOWN_FALLBACK ");
        if (status != null && !status.isBlank()) {
            sb.append("status=").append(status);
        }
        if (message != null && !message.isBlank()) {
            if (sb.length() > "UNKNOWN_FALLBACK ".length()) sb.append(" ");
            sb.append("message=").append(message);
        }
        if (sb.length() == "UNKNOWN_FALLBACK ".length()) {
            sb.append("無 PayUni 訊息");
        }
        String composed = sb.toString();
        return composed.length() > MAX_MESSAGE_LENGTH
                ? composed.substring(0, MAX_MESSAGE_LENGTH)
                : composed;
    }
}
