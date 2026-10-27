package io.github.will413028.hawkr.shipping.mq;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.ProviderWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.Optional;

/**
 * Uber Direct 回調消費者（66 §6.4.1）。
 *
 * <p>監聽 {@code shipping.uber.webhook}，來源是 payuni-webhook-service 的 uber slice。
 * 與 {@code ShippingWebhookConsumer} 同一個中央分派點（{@code applyInboundStatus} →
 * {@code updateStatus}），<b>不另開寫入路徑</b>。
 *
 * <h2>🔴 raw bytes 逐位元組送達 adapter</h2>
 * {@link ProviderWebhookEnvelope#rawBody()} 還原出來的位元組與 Uber 送來的完全相同
 * ——驗簽是 HMAC-SHA256 over raw body，中間任何一次 parse／re-serialize 都會讓它<b>偶發</b>
 * 失敗（66 §6.3.1 陷阱 A2）。
 * 這與 {@code ShippingWebhookConsumer} 對 PayUni 的處置相反（那邊 rawBody 一律留 null，
 * 因為 PayUni 的 MQ body 是 envelope JSON、<b>不是</b>對方簽過的那一份）。
 *
 * <h2>去重是兩層，不是一層</h2>
 * ① ingress 的 {@code webhook_inbox}（{@code uq_inbox_event}）擋「Uber 重送同一個 event_id」；
 * ② 本 consumer 的 {@code shipping_webhook_dedup} 擋「MQ at-least-once 的重複投遞」。
 * 兩層防的是不同的重複來源，<b>缺任一層都會讓另一種重複穿過去</b>。
 *
 * <h2>亂序</h2>
 * consumer <b>不</b>無條件覆蓋狀態：正規化後交給 {@code applyInboundStatus}，
 * 由既有的 {@code ShipmentStatusMapper.isValidTransition} 判定（66 §6.3.5 第 ② 道）。
 */
@Component
public class UberWebhookConsumer {

    private static final ObsLogger log = ObsLogger.forClass(UberWebhookConsumer.class);
    private static final String PROCESS_STATUS_PROCESSED = "PROCESSED";

    private final ShippingService shippingService;
    private final ShippingWebhookDedupMapper dedupMapper;

    public UberWebhookConsumer(ShippingService shippingService,
                                ShippingWebhookDedupMapper dedupMapper) {
        this.shippingService = shippingService;
        this.dedupMapper = dedupMapper;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_UBER_WEBHOOK,
            containerFactory = "manualAckContainerFactory")
    public void onUberWebhook(org.springframework.amqp.core.Message message, Channel channel,
                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {

        log.info("UberWebhookConsumer.received", "收到 Uber 回調");

        try {
            ProviderWebhookEnvelope env = ProviderWebhookEnvelope.parse(message.getBody());

            byte[] rawBody = env.rawBody();
            if (rawBody == null || rawBody.length == 0) {
                // 🔴 這不是「內容為空」而是「契約壞了」：JSON provider 的信封一定帶 raw body。
                //    丟出去 → 重試 → DLQ，而不是靜默 ACK —— 靜默 ACK 的症狀是「事件消失且無人知曉」。
                throw new IllegalStateException("Uber envelope 缺少 raw body，eventKey=" + env.eventKey());
            }
            InboundPayload payload = InboundPayload.ofJson(rawBody, env.headers());

            // 我方識別碼 → 定位配送單 → 取該 provider 的 inbound 能力（與 PayUni 同一個入口）。
            String externalId = UberWebhookCorrelation.externalIdOf(rawBody);
            if (externalId.isEmpty()) {
                throw new IllegalStateException(
                        "Uber 回調找不到我方識別碼（external_id／manifest.reference／meta.external_order_id 皆空），"
                                + "eventKey=" + env.eventKey());
            }

            Optional<InboundCapability> inbound = shippingService.inboundFor(externalId);
            if (inbound.isEmpty()) {
                // 兩種可能：① 配送單還沒建（回調早於建單的 race）② 該 provider 沒有 inbound 能力。
                // 🔴 都<b>不</b>能靜默 ACK：①要重試、②是我方漏接線。
                //    交給 MqErrorHandler：重試 3 次仍不行就進 DLQ（可 re-drive、且 DLQ 有告警）。
                throw new IllegalStateException(
                        "查無配送單或該 provider 無 inbound 能力，externalId=" + externalId
                                + " eventKey=" + env.eventKey());
            }
            InboundCapability ingress = inbound.get();

            String eventKey = ingress.dedupKey(payload);
            if (isDuplicate(eventKey)) {
                log.info("UberWebhookConsumer.duplicate", "重複事件，跳過處理", "eventKey", eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 正規化由 adapter 做；未知值的 fail-closed 由 ShippingService 統一處置。
            // 狀態轉移合法性（含亂序）由 applyInboundStatus 內的 isValidTransition 判定。
            shippingService.applyInboundStatus(externalId, ingress.parseStatusEvent(payload));
            recordDedup(eventKey, externalId);

            channel.basicAck(deliveryTag, false);
            log.info("UberWebhookConsumer.success", "Uber 回調處理完成", "eventKey", eventKey);

        } catch (Exception e) {
            log.error("UberWebhookConsumer.error", "Uber 回調處理失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "UberWebhookConsumer", e);
        }
    }

    /**
     * ⚠️ 與 {@code ShippingWebhookConsumer} 共用 {@code shipping_webhook_dedup} 一張表。
     * 去重鍵由各 provider 的 adapter 產生 ⇒ <b>跨 provider 必須不會撞</b>
     * （PayUni 的一律以 {@code merId} 開頭、Uber 的是 provider event id）。
     * 接第三個 provider 時要重新確認這一點，否則症狀是「另一個 provider 的事件被當成重複丟棄」。
     */
    private boolean isDuplicate(String eventKey) {
        Long count = dedupMapper.selectCount(
                new LambdaQueryWrapper<ShippingWebhookDedupEntity>()
                        .eq(ShippingWebhookDedupEntity::getEventKey, eventKey));
        return count != null && count > 0;
    }

    private void recordDedup(String eventKey, String shipTradeNo) {
        ShippingWebhookDedupEntity entity = new ShippingWebhookDedupEntity();
        entity.setEventKey(eventKey);
        entity.setShipTradeNo(shipTradeNo);
        entity.setProcessStatus(PROCESS_STATUS_PROCESSED);
        entity.setProcessedAt(DateTimeUtils.nowUtc());
        dedupMapper.insert(entity);
    }
}
