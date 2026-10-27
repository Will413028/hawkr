package io.github.will413028.hawkr.productimport.helper;

import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.ImportJobParsedEvent;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.util.UUID;

/**
 * 40 號設計書 §5.4.3 / §5.5a：asset 個別 settle 後的 job 完成度檢查 + 一次性
 * ImportJobParsedEvent publish + item / job status 推進。
 *
 * <p>由 {@code AssetDownloadService.download()}(同步 settle 路徑) 與
 * {@code MediaCallbackListener.applyCallback()}(callback fan-out 路徑) 共用,
 * 兩條路徑透過 {@code parsed_event_published_at IS NULL} CAS guard 確保 publish 只發一次。
 *
 * <p>caller 負責先 settle 個別 asset row (DOWNLOADING → UPLOADED / FAILED) 再呼叫
 * {@link #onAssetSettled(Long)},本 helper 不碰個別 asset 狀態。
 */
@Component
public class AssetSettleHelper {

    private static final ObsLogger log = ObsLogger.forClass(AssetSettleHelper.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ImportJobProgressCalculator progressCalculator;
    private final ObsRabbitPublisher publisher;

    public AssetSettleHelper(ProductImportJobMapper jobMapper,
                             ProductImportItemMapper itemMapper,
                             ImportJobProgressCalculator progressCalculator,
                             ObsRabbitPublisher publisher) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.progressCalculator = progressCalculator;
        this.publisher = publisher;
    }

    /**
     * caller 在同事務內呼叫；若 job 內所有 asset 都已 settled，推進 item / job status，
     * 並（首次完成時）publish ImportJobParsedEvent。
     *
     * <p>設計書 §4.2 / §5.4.3：parsed_event_published_at CAS guard <b>只 gate event publish 不重發</b>，
     * 不可用來 gate item.status / job.status 推進——否則編輯後重下載完成時 item 會卡在 ASSET_DOWNLOADING
     * 永遠不推進。
     *
     * @param jobId import_job.id
     */
    public void onAssetSettled(Long jobId) {
        ImportJobProgressCalculator.Counters counters = progressCalculator.counters(jobId);
        boolean allAssetsSettled = counters.assetsTotal() > 0
                && (counters.assetsPending() == 0 && counters.assetsDownloading() == 0);
        if (!allAssetsSettled) {
            return;
        }

        // 推進 item.status: ASSET_DOWNLOADING(9202) → READY(9203)。
        // blocking_error_count > 0 的 item 仍是 READY，由 needs_fix/counters 呈現「需修正」。
        // 每次 settled 都跑（idempotent — WHERE status=9202 命中才更新，否則 no-op）
        itemMapper.update(null,
                new UpdateWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("status", ImportItemStatus.ASSET_DOWNLOADING.value())
                        .set("status", ImportItemStatus.READY.value())
                        .setSql("updated_at = NOW()"));

        // 推進 job.status: PARSING(9102) → READY(9104)
        // 同樣 idempotent — 已 READY 為 no-op
        jobMapper.update(null,
                new UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .eq("status", ImportJobStatus.PARSING.value())
                        .set("status", ImportJobStatus.READY.value())
                        .setSql("updated_at = NOW()"));

        // 一次性 publish gate — CAS 設 parsed_event_published_at，只有首次成功才 publish event
        // 編輯後重下載 settled 時 CAS 失敗，跳過 publish 但 item / job status 推進已在上方完成
        int updated = jobMapper.update(null,
                new UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .isNull("parsed_event_published_at")
                        .set("parsed_event_published_at", DateTimeUtils.nowUtc())
                        .setSql("updated_at = NOW()"));
        if (updated == 0) {
            return; // 已 publish 過，但 status 推進已完成
        }

        ProductImportJobEntity job = jobMapper.selectById(jobId);
        ImportJobParsedEvent parsed = new ImportJobParsedEvent(
                UUID.randomUUID().toString(),
                jobId,
                job.getStoreId(),
                Math.toIntExact(counters.itemsTotal()),
                Math.toIntExact(counters.itemsReady()),
                0,
                Math.toIntExact(counters.assetsFailed()),
                DateTimeUtils.nowUtc().toString()
        );
        try {
            publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                    MqConstants.RK_PRODUCT_IMPORT_JOB_PARSED,
                    MAPPER.writeValueAsBytes(parsed),
                    ProductImportEventType.PRODUCT_IMPORT_JOB_PARSED.value());
            log.info("AssetSettleHelper.jobParsed.published",
                    "ImportJobParsedEvent published. jobId=" + jobId);
        } catch (Exception e) {
            throw new IllegalStateException("publish job.parsed failed", e);
        }
    }
}
