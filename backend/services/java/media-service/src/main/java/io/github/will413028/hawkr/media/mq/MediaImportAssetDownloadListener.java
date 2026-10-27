package io.github.will413028.hawkr.media.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.mq.event.InternalDownloadTaskEvent;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

/**
 * Media 內部下載 worker pool 消費端（08 號設計書 §7.2a）。
 *
 * <p>對應 queue：{@code media.import.asset.download.internal}
 * （exchange={@code hawkr.media}, routing key={@code import.asset.download.internal}）。</p>
 *
 * <p>Manual ACK：</p>
 * <ul>
 *   <li>handler 完成（含「業務失敗已落 FAILED 並 publish callback」）→ ACK</li>
 *   <li>系統例外（DB 失敗 / 反序列化失敗）→ NACK requeue=false → DLQ</li>
 * </ul>
 *
 * <p>finally / ack 區塊明確置於 service 事務之外（ACK/NACK 不可落在
 * {@code @Transactional} 內）。</p>
 */
@Component
public class MediaImportAssetDownloadListener {

    private static final ObsLogger log = ObsLogger.forClass(MediaImportAssetDownloadListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ImportAssetService importAssetService;

    public MediaImportAssetDownloadListener(ImportAssetService importAssetService) {
        this.importAssetService = importAssetService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL,
        containerFactory = "manualAckContainerFactory"
    )
    public void onDownloadTask(Message message, Channel channel,
                                @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String importAssetId = null;
        boolean handled = false;
        try {
            InternalDownloadTaskEvent event = MAPPER.readValue(
                message.getBody(), InternalDownloadTaskEvent.class);
            importAssetId = event.importAssetId();

            handled = importAssetService.processInternalDownload(importAssetId);

        } catch (IOException e) {
            // 反序列化失敗 → 進 DLQ（不要重投，否則 infinite loop）
            log.error("MediaImportAssetDownloadListener.parseFailed",
                "InternalDownloadTaskEvent 反序列化失敗，將 NACK 進 DLQ", e);
            channel.basicNack(deliveryTag, false, false);
            return;
        } catch (RuntimeException e) {
            // service 端已自己 catch 並回 false；此處兜底
            log.error("MediaImportAssetDownloadListener.unexpected",
                "處理 import_asset 出現未預期例外 import_asset_id=" + importAssetId, e);
            channel.basicNack(deliveryTag, false, false);
            return;
        }

        // ACK / NACK 區塊脫離 service 事務（finally 等價，ACK/NACK 必須在 @Transactional 之外）
        if (handled) {
            channel.basicAck(deliveryTag, false);
        } else {
            channel.basicNack(deliveryTag, false, false);
        }
    }
}
