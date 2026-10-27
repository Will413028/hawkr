package io.github.will413028.hawkr.productimport.listener;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.AssetDownloadEvent;
import io.github.will413028.hawkr.productimport.service.AssetDownloadService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Component
public class AssetDownloadListener {

    private static final ObsLogger log = ObsLogger.forClass(AssetDownloadListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final AssetDownloadService assetDownloadService;

    public AssetDownloadListener(AssetDownloadService assetDownloadService) {
        this.assetDownloadService = assetDownloadService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD,
                    containerFactory = "manualAckContainerFactory")
    public void onAssetDownload(Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        try {
            AssetDownloadEvent event = MAPPER.readValue(message.getBody(), AssetDownloadEvent.class);
            log.info("AssetDownloadListener.received", "assetId=" + event.assetId());
            assetDownloadService.download(event);
            channel.basicAck(deliveryTag, false);
            log.info("AssetDownloadListener.success", "assetId=" + event.assetId());
        } catch (Exception e) {
            log.error("AssetDownloadListener.error", "Failed asset.download", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "AssetDownloadListener", e);
        }
    }
}
