package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.PayUniWebhookEnvelope;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.io.IOException;
import java.util.List;
import java.util.Map;

/**
 * PayUni 物流 Webhook 消費者
 *
 * 監聽 Queue: shipping.payuni.webhook
 * 來源: payuni-webhook-service 解密後轉發的物流狀態回報
 *
 * 手動 ACK，at-least-once + 冪等（dedup 表）
 */
@Component
public class ShippingWebhookConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ShippingWebhookConsumer.class);
    private static final String PROCESS_STATUS_PROCESSED = "PROCESSED";
    private final ShippingService shippingService;
    private final ShippingWebhookDedupMapper dedupMapper;
    private final CvsPrintWaitingRegistry cvsPrintWaitingRegistry;

    public ShippingWebhookConsumer(ShippingService shippingService,
                                    ShippingWebhookDedupMapper dedupMapper,
                                    CvsPrintWaitingRegistry cvsPrintWaitingRegistry) {
        this.shippingService = shippingService;
        this.dedupMapper = dedupMapper;
        this.cvsPrintWaitingRegistry = cvsPrintWaitingRegistry;
    }

    @RabbitListener(queues = MqConstants.QUEUE_SHIPPING_PAYUNI_WEBHOOK, containerFactory = "manualAckContainerFactory")
    public void onShippingWebhook(org.springframework.amqp.core.Message message, Channel channel,
                                   @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {

        log.info("ShippingWebhookConsumer.received", "收到物流狀態 Webhook");

        try {
            PayUniWebhookEnvelope env = PayUniWebhookEnvelope.parse(message.getBody());
            java.util.Map<String, String> rawPayload = env.payload() == null
                    ? java.util.Map.of() : env.payload();

            String shipTradeNo = env.payloadField("ShipTradeNo");
            if (shipTradeNo.isEmpty()) {
                log.warn("ShippingWebhookConsumer.invalid",
                        "缺少必要欄位，shipTradeNo 為空");
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 🔴 BLOCKER 4：解析與去重鍵一律由 **adapter 的 InboundCapability** 提供。
            //    修復前這裡自己讀 PayUni 欄名、自己組去重鍵，而 adapter 那一份是零呼叫端的死碼
            //    ⇒ 兩份 provider 知識、且已經不等價。設計書逐字說 inbound() 是
            //    「未知 provider 原生值 fail-closed」的唯一落點。
            java.util.Optional<InboundCapability> inbound = shippingService.inboundFor(shipTradeNo);
            if (inbound.isEmpty()) {
                // 查無配送單（回調早於建單的 race）或該 provider 沒有註冊 adapter。
                //
                // 🔴 **不能只 warn 就 ACK**：若配送單存在而只是解析不出 adapter，
                //    provider 回報的內容（例如「包裹遺失」）會在系統裡完全沒有痕跡。
                //    這裡改為交給 legacy 相容入口——它對「有單但無 adapter」會保存 raw 值。
                //    ⚠️ 這是**唯一**還會在 consumer 內讀 PayUni 欄名的地方，
                //    而它只在「沒有 adapter 可用」時才走得到——那本身就是異常。
                //    有 adapter 時一律走上面的 canonical path，不存在第二份解析。
                shippingService.processWebhookStatusUpdate(shipTradeNo,
                        env.payloadField("ShipStatus"),
                        env.payloadField("ShipStatusDesc"),
                        env.payloadField("ShipStatusTime"));
                log.warn("ShippingWebhookConsumer.noInbound",
                        "該 provider 無回調能力，已走相容入口保存原始值",
                        "shipTradeNo", shipTradeNo);
                channel.basicAck(deliveryTag, false);
                return;
            }
            InboundCapability ingress = inbound.get();

            // 🔴 PayUni 走 MQ 進來，這裡的 message body 是 payuni-webhook-service 的
            //    envelope JSON，**不是** PayUni 原始簽章的那份 body ⇒ rawBody 一律留 null。
            //    傳一個「看起來是 raw 但不是對方簽的那份」的位元組，會讓未來加驗簽的人
            //    拿它去驗然後永遠失敗，而症狀看起來像金鑰錯（ADR-012）。
            InboundPayload payload = InboundPayload.ofFormParams(rawPayload);

            String eventKey = ingress.dedupKey(payload);
            if (isDuplicate(eventKey)) {
                log.info("ShippingWebhookConsumer.duplicate",
                        "重複事件，跳過處理", "eventKey", eventKey);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 🔴 「這是不是標籤事件」由 adapter 回答，consumer 只拿中性結論。
            //    原本這裡自己判 `"Print".equals(ApiType)`，而 adapter 的 dedupKey
            //    **已經**在做同一個判定 ⇒ 兩份 provider 知識分屬兩層、必須手動保持同步。
            //    今天 PayUni 讀同一個欄所以不會分歧；第二個 provider 一進來，
            //    這個 if 就永遠不會把它的 label 事件路由過去（而去重鍵會用 PRINT 前綴）。
            java.util.Optional<InboundCapability.InboundLabelEvent> labelEvent =
                    ingress.isLabelEvent(payload)
                            ? ingress.parseLabelEvent(payload)
                            : java.util.Optional.empty();

            if (labelEvent.isPresent()) {
                java.util.Map<String, String> segments = labelEvent.get().segmentCodes();
                shippingService.processWebhookPrint(shipTradeNo, segments);
                recordDedup(eventKey, shipTradeNo);

                // 通知等待中的 PrintCvsLabel 請求（若有）
                cvsPrintWaitingRegistry.complete(shipTradeNo,
                        new CvsPrintWaitingRegistry.CvsPrintResult(
                                segments.get(LabelCapability.SEGMENT_PARTNER_ID),
                                segments.get(LabelCapability.SEGMENT_ODNO),
                                segments.get(LabelCapability.SEGMENT_VALIDATION_NO)));

                log.info("ShippingWebhookConsumer.print.success",
                        "Print Webhook 處理完成", "shipTradeNo", shipTradeNo);
            } else {
                // 一般物流貨態更新——正規化由 adapter 做，未知值的 fail-closed 由 service 統一處置。
                shippingService.applyInboundStatus(shipTradeNo, ingress.parseStatusEvent(payload));
                recordDedup(eventKey, shipTradeNo);
            }

            channel.basicAck(deliveryTag, false);

            log.info("ShippingWebhookConsumer.success",
                    "Webhook 處理完成", "shipTradeNo", shipTradeNo);

        } catch (Exception e) {
            log.error("ShippingWebhookConsumer.error", "Webhook 處理失敗", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "ShippingWebhookConsumer", e);
        }
    }

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
