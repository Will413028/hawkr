package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.safefetch.SafeFetchRequest;
import io.github.will413028.hawkr.common.safefetch.SafeFetchResponse;
import io.github.will413028.hawkr.common.safefetch.SafeFetcher;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ImportAssetFailureReason;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemErrorEntity;
import io.github.will413028.hawkr.productimport.helper.HtmlToBlocksConverter;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.AssetDownloadEvent;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.HtmlFetchEvent;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;

/**
 * HtmlFetchListener 業務邏輯（40 號設計書 §5.3 / §12）。
 *
 * <p>從外部 URL 拉取 HTML → 轉 ProductDescription blocks → 抽圖派 AssetDownloadEvent。
 */
@Service
public class HtmlFetchService {

    private static final ObsLogger log = ObsLogger.forClass(HtmlFetchService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String UA = "HawkrProductImport/1.0";

    @Value("${hawkr.product-import.html-fetch.max-size-bytes:5242880}")
    private long maxSizeBytes;

    @Value("${hawkr.product-import.html-fetch.connect-timeout-ms:5000}")
    private long connectTimeoutMs;

    @Value("${hawkr.product-import.html-fetch.read-timeout-ms:30000}")
    private long readTimeoutMs;

    private final SafeFetcher safeFetcher;
    private final HtmlToBlocksConverter htmlConverter;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemErrorMapper errorMapper;
    private final ObsRabbitPublisher publisher;
    // 40 §2.5 Tx 載體規定:同 class self-invocation 用程式式包覆
    private final TransactionTemplate txTemplate;

    public HtmlFetchService(SafeFetcher safeFetcher,
                            HtmlToBlocksConverter htmlConverter,
                            ProductImportItemMapper itemMapper,
                            ProductImportItemAssetMapper assetMapper,
                            ProductImportItemErrorMapper errorMapper,
                            ObsRabbitPublisher publisher,
                            PlatformTransactionManager txManager) {
        this.safeFetcher = safeFetcher;
        this.htmlConverter = htmlConverter;
        this.itemMapper = itemMapper;
        this.assetMapper = assetMapper;
        this.errorMapper = errorMapper;
        this.publisher = publisher;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    public void fetchAndPersist(HtmlFetchEvent event) {
        ProductImportItemEntity item = itemMapper.selectById(event.importItemId());
        if (item == null) {
            log.warn("HtmlFetchService.skip.item_missing", "itemId=" + event.importItemId());
            return;
        }
        if (item.getCommitRevision() == null
                || !item.getCommitRevision().equals(event.commitRevision())) {
            log.info("HtmlFetchService.skip.stale_revision",
                    "itemId=" + event.importItemId() + ", event_rev=" + event.commitRevision()
                            + ", current=" + item.getCommitRevision());
            return;
        }

        SafeFetchResponse resp;
        try {
            resp = safeFetcher.fetch(new SafeFetchRequest(
                    event.sourceUrl(),
                    Set.of("text/html", "application/xhtml+xml", "text/plain"),
                    maxSizeBytes,
                    Duration.ofMillis(connectTimeoutMs),
                    Duration.ofMillis(readTimeoutMs),
                    UA,
                    3));
        } catch (Exception e) {
            log.warn("HtmlFetchService.fetch_failed",
                    "url=" + event.sourceUrl() + ", reason=" + e.getMessage());
            // 同 in-flight redirect 行為：建一筆 FAILED DESCRIPTION asset sentinel
            // 賣家編輯介面才能看到「描述 URL 無法抓取」+ failure_reason / detail。
            final String failureReason = classifyFailureReason(e);
            final String failureDetail = e.getMessage();
            // 40 §2.5:程式式 tx 包覆
            txTemplate.executeWithoutResult(status ->
                    recordFetchFailureBody(event, failureReason, failureDetail));
            return;
        }

        String html = new String(resp.content(), StandardCharsets.UTF_8);
        HtmlToBlocksConverter.Result converted = htmlConverter.convert(html);

        List<AssetDownloadEvent> newAssetEvents = new ArrayList<>();
        // 40 §2.5:程式式 tx 包覆
        txTemplate.executeWithoutResult(status ->
                persistDescriptionBody(item, event, converted, newAssetEvents));

        for (AssetDownloadEvent ev : newAssetEvents) {
            try {
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD,
                        MAPPER.writeValueAsBytes(ev),
                        ProductImportEventType.PRODUCT_IMPORT_ASSET_DOWNLOAD.value());
            } catch (Exception e) {
                log.error("HtmlFetchService.publish.error",
                        "publish asset.download failed", e);
            }
        }
    }

    // 40 §2.5 / §5.3:tx body — 必須由 caller 包進 txTemplate。
    // 禁止標 @Transactional(self-invocation 會繞過 Spring AOP proxy)。
    private void persistDescriptionBody(ProductImportItemEntity itemSnapshot, HtmlFetchEvent event,
                                        HtmlToBlocksConverter.Result converted,
                                        List<AssetDownloadEvent> out) {
        ProductImportItemEntity item = itemMapper.selectById(event.importItemId());
        if (item == null) return;
        if (item.getCommitRevision() == null
                || !item.getCommitRevision().equals(event.commitRevision())) {
            return;
        }
        item.setDescriptionJson(converted.descriptionJson());
        itemMapper.updateById(item);

        // §12.5a：HTML 轉換 warning 持久化（severity=WARNING / error_type=HTML_PARSE_WARNING）
        for (HtmlToBlocksConverter.ConversionWarning w : converted.warnings()) {
            ProductImportItemErrorEntity e = new ProductImportItemErrorEntity();
            e.setImportItemId(event.importItemId());
            e.setImportJobId(event.importJobId());
            e.setCsvLine(item.getFirstCsvLine() == null ? 0 : item.getFirstCsvLine());
            e.setErrorType(ImportItemErrorType.HTML_PARSE_WARNING.value());
            e.setSeverity("WARNING");
            e.setField("description");
            e.setMessage(w.message());
            String detail = w.warningEnum() + ": " + (w.detail() == null ? "" : w.detail());
            e.setRawValue(detail.length() > 500 ? detail.substring(0, 500) : detail);
            errorMapper.insert(e);
        }

        for (HtmlToBlocksConverter.ExtractedImage img : converted.images()) {
            long assetId = TsidUtil.generate();
            ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
            asset.setId(assetId);
            asset.setImportItemId(event.importItemId());
            asset.setImportJobId(event.importJobId());
            asset.setUsage(ImportAssetUsage.DESCRIPTION.value());
            asset.setBlockIndex(img.blockIndex());
            asset.setSourceType("URL");
            asset.setSourceUrl(img.url());
            asset.setSourceUrlHash(sha256Hex(img.url().getBytes()));
            asset.setStatus(ImportAssetLocalStatus.PENDING.value());
            asset.setRetryCount(0);
            assetMapper.insert(asset);
            out.add(new AssetDownloadEvent(event.importItemId(), event.importJobId(),
                    event.storeId(), assetId, img.url(),
                    ImportAssetUsage.DESCRIPTION.value(), item.getCommitRevision()));
        }
        if (!converted.images().isEmpty()
                && ImportItemStatus.PARSED.value().equals(item.getStatus())) {
            item.setStatus(ImportItemStatus.ASSET_DOWNLOADING.value());
            itemMapper.updateById(item);
        }
    }

    /**
     * fetch 失敗（含 SSRF pre-flight reject）時建 sentinel asset row，對齊 in-flight 失敗的可見性。
     *
     * <p>40 §2.5 / §5.3:tx body — 必須由 caller 包進 txTemplate。
     */
    private void recordFetchFailureBody(HtmlFetchEvent event, String failureReason, String failureDetail) {
        ProductImportItemEntity item = itemMapper.selectById(event.importItemId());
        if (item == null) return;
        if (item.getCommitRevision() == null
                || !item.getCommitRevision().equals(event.commitRevision())) {
            return;
        }
        ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
        asset.setId(TsidUtil.generate());
        asset.setImportItemId(event.importItemId());
        asset.setImportJobId(event.importJobId());
        asset.setUsage(ImportAssetUsage.DESCRIPTION.value());
        // sentinel：description URL 整體失敗，無 block_index 對應
        asset.setSourceType("URL");
        asset.setSourceUrl(event.sourceUrl());
        asset.setSourceUrlHash(sha256Hex(event.sourceUrl().getBytes()));
        asset.setStatus(ImportAssetLocalStatus.FAILED.value());
        asset.setFailureReason(failureReason);
        asset.setFailureDetail(failureDetail == null
                ? null
                : failureDetail.substring(0, Math.min(500, failureDetail.length())));
        asset.setRetryCount(0);
        assetMapper.insert(asset);
    }

    /** 由 exception message 判斷 failure_reason，預設 FETCH_ERROR；命中 SSRF / 協定字串時改 SSRF_BLOCKED / PROTOCOL_NOT_ALLOWED。 */
    private static String classifyFailureReason(Exception e) {
        String msg = e.getMessage() == null ? "" : e.getMessage().toLowerCase();
        if (msg.contains("only https") || msg.contains("scheme")) {
            return ImportAssetFailureReason.PROTOCOL_NOT_ALLOWED.value();
        }
        if (msg.contains("ssrf") || msg.contains("private") || msg.contains("blocked")
                || msg.contains("redirect") || msg.contains("127.")
                || msg.contains("localhost") || msg.contains("10.")
                || msg.contains("192.168.") || msg.contains("169.254.")) {
            return ImportAssetFailureReason.SSRF_BLOCKED.value();
        }
        return ImportAssetFailureReason.FETCH_ERROR.value();
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
