package io.github.will413028.hawkr.productimport.listener;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.mq.events.MediaImportAssetDownloadedEvent;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.helper.AssetSettleHelper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportMediaCallbackInboxMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.List;

/**
 * Media callback listener（40 號設計書 §5.5 / §6.5a / 08 號 §7.2a）。
 *
 * <p>消費 {@code hawkr.media} / {@code import.asset.downloaded}（Media 端 publish）。
 *
 * <p>同事務內：
 * <ol>
 *   <li>INSERT inbox ON CONFLICT DO NOTHING（去重）</li>
 *   <li>UPDATE product_import_item_asset 為 UPLOADED / FAILED</li>
 *   <li>檢查 job 是否「最後一個 asset 完成」→ UPDATE job.parsed_event_published_at IS NULL → SET NOW()
 *       同事務內判斷影響筆數 = 1 才 publish ImportJobParsedEvent（一次性）</li>
 * </ol>
 */
@Component
public class MediaCallbackListener {

    private static final ObsLogger log = ObsLogger.forClass(MediaCallbackListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductImportMediaCallbackInboxMapper inboxMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final AssetSettleHelper settleHelper;
    // 40 §2.5 Tx 載體規定:applyCallbackBody 為 self-invocation 短 tx,用程式式包覆
    private final TransactionTemplate txTemplate;

    public MediaCallbackListener(ProductImportMediaCallbackInboxMapper inboxMapper,
                                 ProductImportItemAssetMapper assetMapper,
                                 AssetSettleHelper settleHelper,
                                 PlatformTransactionManager txManager) {
        this.inboxMapper = inboxMapper;
        this.assetMapper = assetMapper;
        this.settleHelper = settleHelper;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK,
                    containerFactory = "manualAckContainerFactory")
    public void onMediaCallback(Message message, Channel channel,
                                 @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        log.info("MediaCallbackListener.received", "Received import.asset.downloaded event");

        try {
            MediaImportAssetDownloadedEvent event = MAPPER.readValue(body, MediaImportAssetDownloadedEvent.class);

            // 40 §2.5 / §5.4.2:程式式 tx 包覆,確保 inbox dedup claim + per-asset UPDATE +
            // settleHelper.onAssetSettled(含 CAS publish)原子。
            //
            // dedup INSERT **必須與 callback body 同事務**：原本它在 tx 外自動提交，於是
            // body 失敗 → tx rollback → MqErrorHandler 原樣重投（同一個 event_id）→
            // insertIfAbsent 回 0 → ack skip ⇒ asset 永遠停在 DOWNLOADING、job 永遠不完成。
            // dedup claim 與實際工作必須同生共死（比照 product-service
            // SalesAccumulatedConsumer.SalesAccumulator.applyAll 的註解與寫法）。
            Boolean processed = txTemplate.execute(status -> {
                int inserted = inboxMapper.insertIfAbsent(
                        event.eventId(), event.importAssetId(), event.resultStatus());
                if (inserted == 0) {
                    return Boolean.FALSE;
                }
                applyCallbackBody(event);
                return Boolean.TRUE;
            });

            channel.basicAck(deliveryTag, false);
            if (!Boolean.TRUE.equals(processed)) {
                log.info("MediaCallbackListener.duplicate",
                        "Duplicate event_id, ack and skip",
                        "eventId", event.eventId());
                return;
            }
            log.info("MediaCallbackListener.success",
                    "Processed import.asset.downloaded event",
                    "eventId",
                    "eventId=" + event.eventId() + ", assetId=" + event.productImportItemAssetId());
        } catch (Exception e) {
            log.error("MediaCallbackListener.error", "Failed to process media callback", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "MediaCallbackListener", e);
        }
    }

    /**
     * UPDATE asset 並判斷是否為 job 最後一個 asset → 一次性 publish ImportJobParsedEvent。
     *
     * <p>40 §2.5 / §5.4.2:tx body — 必須由 caller 包進 txTemplate。
     * 禁止標 @Transactional(self-invocation 會繞過 Spring AOP proxy)。
     */
    private void applyCallbackBody(MediaImportAssetDownloadedEvent event) {
        // Issue #1030: Media 端以 (account, job, sourceUrlHash) dedup 同 URL，只 INSERT 一個 import_asset，
        // callback 只 echo 第一個 caller 的 productImportItemAssetId。ProductImport 端 N 個 row 共用同一個
        // media_import_asset_id（GALLERY 各 slot / 跨 item 各自 INSERT，必須保留 per-row 以對應 commit 階段 slot）。
        // 改走 fan-out：以 (import_job_id, media_import_asset_id, status=DOWNLOADING) 找全部 row 批次 UPDATE。
        // 為相容性同時涵蓋 echo 的 productImportItemAssetId（萬一該 row 還沒被 AssetDownloadService set media_import_asset_id 的極窄 race），
        // fan-out 後一併納入。
        Long jobId = event.importJobId();
        String mediaImportAssetId = event.importAssetId();
        List<ProductImportItemAssetEntity> targets;
        if (mediaImportAssetId != null && !mediaImportAssetId.isBlank() && jobId != null) {
            targets = assetMapper.selectList(
                    new QueryWrapper<ProductImportItemAssetEntity>()
                            .eq("import_job_id", jobId)
                            .eq("media_import_asset_id", mediaImportAssetId)
                            .eq("status", ImportAssetLocalStatus.DOWNLOADING.value()));
        } else {
            targets = new java.util.ArrayList<>();
        }

        // 防 race：echo 的 productImportItemAssetId 還沒被 AssetDownloadService UPDATE 到 media_import_asset_id 時，
        // 用 selectById 補一筆。
        Long echoAssetId = event.productImportItemAssetId();
        if (echoAssetId != null && targets.stream().noneMatch(a -> echoAssetId.equals(a.getId()))) {
            ProductImportItemAssetEntity echo = assetMapper.selectById(echoAssetId);
            if (echo != null && ImportAssetLocalStatus.DOWNLOADING.value().equals(echo.getStatus())) {
                targets.add(echo);
            }
        }

        if (targets.isEmpty()) {
            log.warn("MediaCallbackListener.fan_out_empty",
                    "No DOWNLOADING asset rows match callback. jobId=" + jobId
                            + ", mediaImportAssetId=" + mediaImportAssetId
                            + ", echoAssetId=" + echoAssetId);
            return;
        }

        for (ProductImportItemAssetEntity asset : targets) {
            // UPDATE asset 狀態（resultStatus 對齊 ImportAssetMediaStatus.value()，非字面值）
            if (ImportAssetMediaStatus.UPLOADED.value().equals(event.resultStatus())) {
                asset.setStatus(ImportAssetLocalStatus.UPLOADED.value());
                asset.setObjectKey(event.objectKey());
                asset.setPreviewUrl(event.previewUrl());
            } else if (ImportAssetMediaStatus.FAILED.value().equals(event.resultStatus())) {
                asset.setStatus(ImportAssetLocalStatus.FAILED.value());
                asset.setFailureReason(event.failureReason());
                asset.setFailureDetail(event.failureDetail());
            } else {
                throw new IllegalStateException(
                        "Unexpected resultStatus from MediaImportAssetDownloadedEvent: " + event.resultStatus()
                                + " (eventId=" + event.eventId() + ")");
            }
            assetMapper.updateById(asset);
        }
        log.info("MediaCallbackListener.fan_out_applied",
                "Fan-out UPDATE applied. jobId=" + jobId
                        + ", mediaImportAssetId=" + mediaImportAssetId
                        + ", rowCount=" + targets.size()
                        + ", resultStatus=" + event.resultStatus());

        // 同步 settle 路徑共用此 helper(40 號 §5.4.3): job 完成度判斷 + 一次性 publish + item/job status 推進。
        settleHelper.onAssetSettled(jobId);
    }
}
