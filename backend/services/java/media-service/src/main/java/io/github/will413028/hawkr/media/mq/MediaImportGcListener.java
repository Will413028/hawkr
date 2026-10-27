package io.github.will413028.hawkr.media.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.mq.event.ExpireJobEvent;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * 跨服務 GC 消費端（08 號設計書 §7.5）。
 *
 * <p>訂閱 product-import-service publish 的 {@code gc.expire_job} 事件
 * （exchange={@code hawkr.product.import}, routing key={@code gc.expire_job}）。</p>
 *
 * <p>處理邏輯：同 {@code import_job_id} 下所有非終態 import_asset 標 EXPIRED + 刪 OSS tmp。
 * UPDATE 條件 {@code status NOT IN (FINALIZED, EXPIRED)} 天然 idempotent。</p>
 */
@Component
public class MediaImportGcListener {

    private static final ObsLogger log = ObsLogger.forClass(MediaImportGcListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ImportAssetService importAssetService;

    public MediaImportGcListener(ImportAssetService importAssetService) {
        this.importAssetService = importAssetService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB,
        containerFactory = "manualAckContainerFactory"
    )
    public void onExpireJob(Message message, Channel channel,
                            @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        Long importJobId = null;
        try {
            // 兼容兩種 payload：(a) record-style {"importJobId": 123}；(b) snake-case {"import_job_id": 123}
            JsonNode node = MAPPER.readTree(message.getBody());
            JsonNode field = node.path("importJobId");
            if (field.isMissingNode() || field.isNull()) {
                field = node.path("import_job_id");
            }
            if (!field.isMissingNode() && !field.isNull()) {
                importJobId = field.asLong();
            } else {
                // 還是嘗試以 ExpireJobEvent record 反序列化（最後一道）
                ExpireJobEvent event = MAPPER.treeToValue(node, ExpireJobEvent.class);
                importJobId = event.importJobId();
            }

            if (importJobId == null || importJobId <= 0) {
                log.error("MediaImportGcListener.invalidPayload",
                    "缺少 importJobId，NACK 進 DLQ payload=" + node.toString());
                channel.basicNack(deliveryTag, false, false);
                return;
            }

            importAssetService.expireByJobId(importJobId);

        } catch (IOException e) {
            log.error("MediaImportGcListener.parseFailed",
                "ExpireJobEvent 反序列化失敗，NACK 進 DLQ", e);
            channel.basicNack(deliveryTag, false, false);
            return;
        } catch (RuntimeException e) {
            log.error("MediaImportGcListener.unexpected",
                "處理 ExpireJobEvent 出現未預期例外 import_job_id=" + importJobId, e);
            channel.basicNack(deliveryTag, false, false);
            return;
        }

        // ACK 區塊脫離 service 事務（ACK 必須在 @Transactional 之外）
        channel.basicAck(deliveryTag, false);
    }
}
