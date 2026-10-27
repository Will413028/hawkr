package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.mq.dto.SellerExposureChangedPayload;
import io.github.will413028.hawkr.product.service.StoreExposureService;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * 店家可曝光狀態消費者：消費單體 seller 發出的 {@code seller.exposure.changed}，
 * 更新 {@code store_exposure} 讀取模型，公開讀取面據此隱藏不可曝光的店的商品。
 *
 * <h3>異常處理</h3>
 * <ul>
 *   <li>Payload 缺欄位、格式錯誤、event_type 不符（毒訊息）→ 直接 NACK 進 DLQ，不重試</li>
 *   <li>DB 或其他暫時性例外 → {@link MqErrorHandler#handleFailure}：重新發布重試最多 3 次，仍失敗才進 DLQ
 *       （與 coupon、order 的 seller 事件 consumer 相同；重試沒有延遲）</li>
 *   <li>舊版本 → 正常 ACK（{@link StoreExposureService#apply} 不改任何東西）；同版本重送 → 不改列、再清一次快取</li>
 * </ul>
 *
 * <p>這是狀態傳遞的讀取模型：seller 只比對自己最後一次公告，死信的事件不會被重送，所以暫時性失敗不能直接死信。
 * 寫入由版本條件 upsert 保護，重試與 DLQ 重放都是冪等的（舊版本不改任何東西，同版本重複只再清一次快取）。
 */
@Component
public class SellerExposureChangedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(SellerExposureChangedConsumer.class);

    private final StoreExposureService storeExposureService;

    public SellerExposureChangedConsumer(StoreExposureService storeExposureService) {
        this.storeExposureService = storeExposureService;
    }

    @RabbitListener(
        queues = MqConstants.QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED,
        containerFactory = "manualAckContainerFactory"
    )
    public void onSellerExposureChanged(Message message, Channel channel) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        String body = new String(message.getBody(), StandardCharsets.UTF_8);

        SellerExposureChangedPayload payload;
        try {
            payload = SellerExposureChangedPayload.parse(body);
        } catch (IllegalArgumentException e) {
            log.warn("SellerExposureChangedConsumer.parse.fail",
                    "payload 格式錯誤，進 DLQ body=" + body + " error=" + e.getMessage());
            channel.basicNack(deliveryTag, false, false);
            return;
        }

        try {
            storeExposureService.apply(payload.storeId(), payload.exposable(), payload.exposureVersion());
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("SellerExposureChangedConsumer.handle.fail", "處理失敗 body=" + body, e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "SellerExposureChangedConsumer", e);
        }
    }
}
