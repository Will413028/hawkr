package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.media.grpc.ImportDownloadAssetResponse;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.productimport.helper.AssetSettleHelper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.AssetDownloadEvent;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.security.MessageDigest;
import java.util.HexFormat;

/**
 * AssetDownloadListener 業務邏輯（40 號設計書 §5.4）。
 *
 * <p>呼叫 Media {@code ImportDownloadAsset} (async)。狀態變更同事務內完成。
 */
@Service
public class AssetDownloadService {

    private static final ObsLogger log = ObsLogger.forClass(AssetDownloadService.class);
    private static final int MAX_RETRY = 3;
    private static final String GALLERY_REF_TYPE = AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value();
    private static final String DESC_REF_TYPE = AssetReferenceType.PRODUCT_IMPORT_PREVIEW_DESC.value();

    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportJobMapper jobMapper;
    private final MediaGrpcClient mediaClient;
    private final AssetSettleHelper settleHelper;

    public AssetDownloadService(ProductImportItemAssetMapper assetMapper,
                                ProductImportItemMapper itemMapper,
                                ProductImportJobMapper jobMapper,
                                MediaGrpcClient mediaClient,
                                AssetSettleHelper settleHelper) {
        this.assetMapper = assetMapper;
        this.itemMapper = itemMapper;
        this.jobMapper = jobMapper;
        this.mediaClient = mediaClient;
        this.settleHelper = settleHelper;
    }

    @Transactional
    public void download(AssetDownloadEvent event) {
        ProductImportItemAssetEntity asset = assetMapper.selectById(event.assetId());
        if (asset == null) {
            log.warn("AssetDownloadService.skip.asset_missing", "assetId=" + event.assetId());
            return;
        }
        String status = asset.getStatus();
        boolean retryable = ImportAssetLocalStatus.PENDING.value().equals(status)
                || (ImportAssetLocalStatus.FAILED.value().equals(status)
                    && (asset.getRetryCount() == null || asset.getRetryCount() < MAX_RETRY));
        if (!retryable) {
            log.info("AssetDownloadService.skip.status", "assetId=" + event.assetId() + ", status=" + status);
            return;
        }

        ProductImportJobEntity job = jobMapper.selectById(event.importJobId());
        String expiresAt = job != null && job.getExpiresAt() != null
                ? DateTimeUtils.formatIso8601(job.getExpiresAt())
                : DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc().plusDays(7));
        String idempotencyKey = sha256Hex(event.sourceUrl().getBytes()) + ":" + event.importJobId();
        String refType = "DESCRIPTION".equals(event.usage()) ? DESC_REF_TYPE : GALLERY_REF_TYPE;

        try {
            ImportDownloadAssetResponse resp = mediaClient.importDownloadAsset(
                    refType,
                    event.sourceUrl(),
                    event.storeId(),
                    event.importJobId(),
                    event.importItemId(),
                    idempotencyKey,
                    expiresAt,
                    event.assetId());
            if (resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
                // Media bizError（deterministic 入參錯誤，如 ref_type / source_url 違法）
                // 不走 MQ retry queue（同樣參數無窮重打），落 FAILED + retry_count++ 終態
                String failureReason = resp.getFailureReason() != null && !resp.getFailureReason().isBlank()
                        ? resp.getFailureReason()
                        : "MEDIA_DISPATCH_FAILED";
                asset.setStatus(ImportAssetLocalStatus.FAILED.value());
                asset.setFailureReason(failureReason);
                asset.setFailureDetail("media code=" + resp.getCode() + " msg=" + resp.getMessage());
                asset.setRetryCount((asset.getRetryCount() == null ? 0 : asset.getRetryCount()) + 1);
                assetMapper.updateById(asset);
                log.warn("AssetDownloadService.media.bizError",
                        "assetId=" + event.assetId() + ", code=" + resp.getCode() + ", reason=" + failureReason);
                return;
            }
            // 40 號 §5.4.1 step 4 依 response.status 分流(8 號 §7.2 dedup 命中時 echo settled metadata)。
            // dedup 命中且 Media 已 settled 時直接同步 settle 自己 row,不依賴 callback (解決
            // 「dispatch 落在 callback 之後」的 race window — BUG #15)。
            asset.setMediaImportAssetId(resp.getImportAssetId());
            asset.setRetryCount((asset.getRetryCount() == null ? 0 : asset.getRetryCount()) + 1);

            String mediaStatus = resp.getStatus();
            boolean settledNow;
            if (ImportAssetMediaStatus.UPLOADED.value().equals(mediaStatus)
                    || ImportAssetMediaStatus.FINALIZED.value().equals(mediaStatus)) {
                asset.setStatus(ImportAssetLocalStatus.UPLOADED.value());
                asset.setObjectKey(resp.getObjectKey());
                asset.setPreviewUrl(resp.getPreviewUrl());
                settledNow = true;
                log.info("AssetDownloadService.sync_settle.uploaded",
                        "Media RPC echo 已 settled (UPLOADED/FINALIZED),同步 settle 自己 row. assetId="
                                + event.assetId() + ", mediaStatus=" + mediaStatus);
            } else if (ImportAssetMediaStatus.FAILED.value().equals(mediaStatus)) {
                asset.setStatus(ImportAssetLocalStatus.FAILED.value());
                asset.setFailureReason(resp.getFailureReason());
                asset.setFailureDetail(resp.getFailureDetail());
                settledNow = true;
                log.info("AssetDownloadService.sync_settle.failed",
                        "Media RPC echo 已 settled (FAILED),同步 settle 自己 row. assetId=" + event.assetId());
            } else if (ImportAssetMediaStatus.EXPIRED.value().equals(mediaStatus)) {
                // 罕見: dedup 命中但 Media row 已過期 — 視為終態 FAILED,賣家可走 RetryImportItemAssets 重派
                asset.setStatus(ImportAssetLocalStatus.FAILED.value());
                asset.setFailureReason("CALLBACK_TIMEOUT");
                settledNow = true;
                log.warn("AssetDownloadService.sync_settle.expired",
                        "Media RPC echo EXPIRED,標 FAILED 待賣家 retry. assetId=" + event.assetId());
            } else {
                // DOWNLOADING(首次 INSERT 或 dedup 命中但仍下載中) — 等 callback fan-out 收斂
                asset.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
                settledNow = false;
            }
            assetMapper.updateById(asset);

            // settled 路徑同樣推進 item.status PARSED → ASSET_DOWNLOADING (與 DOWNLOADING 路徑一致進入 §5.4.3 helper);
            // 同步 settle 走完後立即呼叫 helper 觸發 job 完成度判斷 (與 callback 路徑共用一次性 guard)。
            ProductImportItemEntity item = itemMapper.selectById(event.importItemId());
            if (item != null && ImportItemStatus.PARSED.value().equals(item.getStatus())) {
                item.setStatus(ImportItemStatus.ASSET_DOWNLOADING.value());
                itemMapper.updateById(item);
            }

            if (settledNow) {
                settleHelper.onAssetSettled(event.importJobId());
            }
        } catch (Exception e) {
            asset.setStatus(ImportAssetLocalStatus.FAILED.value());
            asset.setFailureReason("FETCH_ERROR");
            asset.setFailureDetail(e.getMessage());
            asset.setRetryCount((asset.getRetryCount() == null ? 0 : asset.getRetryCount()) + 1);
            assetMapper.updateById(asset);
            throw e;
        }
    }

    private static String sha256Hex(byte[] content) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(md.digest(content));
        } catch (Exception e) {
            throw new IllegalStateException("SHA-256 not available", e);
        }
    }
}
