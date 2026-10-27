package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse;
import io.github.will413028.hawkr.product.grpc.DraftViolation;
import io.github.will413028.hawkr.product.grpc.ExistingProductByName;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemErrorEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.productimport.helper.CodeMetaNormalizer;
import io.github.will413028.hawkr.productimport.helper.CsvParser;
import io.github.will413028.hawkr.productimport.helper.HashtagInputSplitter;
import io.github.will413028.hawkr.productimport.helper.HtmlToBlocksConverter;
import io.github.will413028.hawkr.productimport.helper.ProductDraftBuilder;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.AssetDownloadEvent;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.HtmlFetchEvent;
import io.github.will413028.hawkr.productimport.storage.ProductImportStorage;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingMethodView;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.math.BigDecimal;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * CsvParseListener 業務邏輯（40 號設計書 §5.2）。
 *
 * <p>事務拆分：
 * <ul>
 *   <li>tx 外：下載 CSV / 解析 / RPC 預取（category / shipping / name dup）</li>
 *   <li>per-item 短 tx：INSERT item + skus + assets + errors</li>
 *   <li>最後一個短 tx：UPDATE job 為 READY / ASSET_DOWNLOADING + publish events</li>
 * </ul>
 */
@Service
public class CsvParseService {

    private static final ObsLogger log = ObsLogger.forClass(CsvParseService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    // ShippingMethod CodeMeta value（"3001" / "3002" / "3003"）— 對齊 docs/開發規範/Java 微服務開發規範.md §2.1 / §6.4
    // 「DB / RPC 一律存 value」。shippingEnabled map 的 key 也是用 SellerShippingMethodView.getShippingMethod()
    // 拿到 value（"3001" 等），lookup 必須用 value 才能對得上。
    // BUG #11（Round 5 QA）：早期用 NAME 字串（"TAQBIN"）lookup 永遠 miss，所有 import 永遠 SHIPPING_NOT_ENABLED。
    private static final String SHIPPING_VALUE_TAQBIN = ShippingMethod.TAQBIN.value();
    private static final String SHIPPING_VALUE_SEVEN = ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    private static final String SHIPPING_VALUE_UBER = ShippingMethod.UBER_INSTANT.value();

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemSkuMapper skuMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemErrorMapper errorMapper;
    private final ProductImportStorage csvStorage;
    private final ProductGrpcClient productClient;
    private final ShippingGrpcClient shippingClient;
    private final CsvParser csvParser;
    private final HtmlToBlocksConverter htmlConverter;
    private final ObsRabbitPublisher publisher;
    // 40 §2.5 Tx 載體規定:同 class self-invocation 用程式式包覆。
    // requiresNewTxTemplate 對應 persistOneItemBody — per-item 短 tx 隔離,sibling item 失敗互不影響(§5.2)
    // txTemplate (REQUIRED) 對應 persistDraftViolations / finalizeJob / markJobParseFailed
    private final TransactionTemplate txTemplate;
    private final TransactionTemplate requiresNewTxTemplate;

    public CsvParseService(ProductImportJobMapper jobMapper,
                           ProductImportItemMapper itemMapper,
                           ProductImportItemSkuMapper skuMapper,
                           ProductImportItemAssetMapper assetMapper,
                           ProductImportItemErrorMapper errorMapper,
                           ProductImportStorage csvStorage,
                           ProductGrpcClient productClient,
                           ShippingGrpcClient shippingClient,
                           CsvParser csvParser,
                           HtmlToBlocksConverter htmlConverter,
                           ObsRabbitPublisher publisher,
                           PlatformTransactionManager txManager) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.skuMapper = skuMapper;
        this.assetMapper = assetMapper;
        this.errorMapper = errorMapper;
        this.csvStorage = csvStorage;
        this.productClient = productClient;
        this.shippingClient = shippingClient;
        this.csvParser = csvParser;
        this.htmlConverter = htmlConverter;
        this.publisher = publisher;
        this.txTemplate = new TransactionTemplate(txManager);
        this.requiresNewTxTemplate = new TransactionTemplate(txManager);
        this.requiresNewTxTemplate.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }

    public void parseAndPersist(Long jobId, Long storeId) throws IOException {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null) {
            log.warn("CsvParseService.skip.job_missing", "job missing, jobId=" + jobId);
            return;
        }
        if (!ImportJobStatus.PARSING.value().equals(job.getStatus())) {
            log.warn("CsvParseService.skip.status",
                    "job not in PARSING, jobId=" + jobId + ", status=" + job.getStatus());
            return;
        }

        byte[] csvBytes = csvStorage.downloadCsv(job.getSourceObjectKey());
        if (csvBytes == null) {
            log.error("CsvParseService.csv_missing",
                    "Cannot fetch CSV from OSS, key=" + job.getSourceObjectKey(), null);
            // 40 §2.5:程式式 tx 包覆
            txTemplate.executeWithoutResult(status -> markJobParseFailedBody(jobId, "CSV fetch failed"));
            return;
        }

        List<CsvParser.ParsedRow> rows = csvParser.parse(csvBytes);
        Map<String, List<CsvParser.ParsedRow>> groups = new LinkedHashMap<>();
        for (CsvParser.ParsedRow r : rows) {
            groups.computeIfAbsent(r.productKey(), k -> new ArrayList<>()).add(r);
        }

        Map<String, ValidateLeafCategoryInternalResponse> categoryCache = new HashMap<>();
        for (String cat : uniqueValues(rows, CsvParser.ParsedRow::categoryId)) {
            if (cat == null || cat.isBlank()) continue;
            try {
                categoryCache.put(cat, productClient.validateLeafCategory(cat));
            } catch (Exception e) {
                log.warn("CsvParseService.category_rpc_failed", "category=" + cat + ", reason=" + e.getMessage());
            }
        }

        List<String> uniqueNames = new ArrayList<>(uniqueValues(rows, CsvParser.ParsedRow::productName));
        Map<String, ExistingProductByName> existingNames = new HashMap<>();
        if (!uniqueNames.isEmpty()) {
            try {
                BatchCheckSellerProductNamesResponse resp =
                        productClient.batchCheckSellerProductNames(storeId, uniqueNames);
                for (ExistingProductByName e : resp.getExistingList()) {
                    existingNames.put(e.getName(), e);
                }
            } catch (Exception e) {
                log.warn("CsvParseService.name_check_failed", "reason=" + e.getMessage());
            }
        }

        Map<String, SellerShippingMethodView> shippingEnabled = new HashMap<>();
        try {
            for (SellerShippingMethodView v : shippingClient.listSellerShippingMethods(storeId)) {
                shippingEnabled.put(v.getShippingMethod(), v);
            }
        } catch (Exception e) {
            log.warn("CsvParseService.shipping_list_failed", "reason=" + e.getMessage());
        }

        Set<String> inJobNames = new HashSet<>();
        List<AssetDownloadEvent> pendingAssetEvents = new ArrayList<>();
        List<HtmlFetchEvent> pendingHtmlEvents = new ArrayList<>();
        int totalAssets = 0;

        for (Map.Entry<String, List<CsvParser.ParsedRow>> entry : groups.entrySet()) {
            List<CsvParser.ParsedRow> productRows = entry.getValue();
            CsvParser.ParsedRow first = productRows.get(0);

            // 40 §2.5 / §5.2:REQUIRES_NEW per-item 短 tx,sibling item 失敗互不影響
            final String productKey = entry.getKey();
            final List<CsvParser.ParsedRow> rowsFinal = productRows;
            final Set<String> inJobNamesFinal = inJobNames;
            PersistResult res = requiresNewTxTemplate.execute(status -> persistOneItemBody(
                    jobId, storeId, productKey, rowsFinal,
                    categoryCache, existingNames, shippingEnabled, inJobNamesFinal));
            inJobNames.add(first.productName());
            totalAssets += res.assetCount();
            pendingAssetEvents.addAll(res.assetDownloadEvents());
            pendingHtmlEvents.addAll(res.htmlFetchEvents());

            // 40 §4.3a / §5.2 step 5.2.g：tx 外呼叫 product-service ValidateProductDraft（CSV_PARSE context）。
            // 失敗不阻擋 parse 流程（賣家可在編輯頁修），僅把 violations 寫入 product_import_item_error
            // （source=VALIDATE_PRODUCT_DRAFT），blocking_error_count 同事務重算。
            if (res.importItemId() != null) {
                try {
                    runValidateProductDraftForParse(res.importItemId(), storeId, jobId, first);
                } catch (Exception e) {
                    log.warn("CsvParseService.validateDraft.parse.failed",
                            "itemId=" + res.importItemId() + ", reason=" + e.getMessage());
                }
            }
        }

        final int totalItemsFinal = groups.size();
        final boolean hasAssetsFinal = totalAssets > 0;
        // 40 §2.5:程式式 tx 包覆
        txTemplate.executeWithoutResult(status -> finalizeJobBody(jobId, totalItemsFinal, hasAssetsFinal));
        for (AssetDownloadEvent ev : pendingAssetEvents) publish(MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD, ev,
                ProductImportEventType.PRODUCT_IMPORT_ASSET_DOWNLOAD.value());
        for (HtmlFetchEvent ev : pendingHtmlEvents) publish(MqConstants.RK_PRODUCT_IMPORT_HTML_FETCH, ev,
                ProductImportEventType.PRODUCT_IMPORT_HTML_FETCH.value());
    }

    // 40 §2.5 / §5.2:tx body — 必須由 caller 包進 requiresNewTxTemplate(per-item 短 tx 隔離)。
    // 禁止標 @Transactional(self-invocation 會繞過 Spring AOP proxy)。
    private PersistResult persistOneItemBody(Long jobId, Long storeId, String productKey,
                                             List<CsvParser.ParsedRow> rows,
                                             Map<String, ValidateLeafCategoryInternalResponse> categoryCache,
                                             Map<String, ExistingProductByName> existingNames,
                                             Map<String, SellerShippingMethodView> shippingEnabled,
                                             Set<String> inJobNames) {
        CsvParser.ParsedRow first = rows.get(0);

        // Issue #1029：listener requeue（per-item tx partial 失敗後 republish）會讓本方法二次進入。
        // 既有 row 表示前次已 atomic commit（REQUIRES_NEW per-item tx，item+sku+asset+errors+status 全 in），
        // reuse 既有 row 避免撞 uk_pii_job_key UNIQUE；同模式與 CommitItemService.markCommitting（40 號設計書 §5.4）對齊。
        ProductImportItemEntity existing = itemMapper.selectOne(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("product_key", productKey));
        if (existing != null) {
            log.info("CsvParseService.persistOneItem.replay_reuse",
                    "jobId=" + jobId + ", productKey=" + productKey + ", itemId=" + existing.getId());
            return rebuildPersistResultFromExisting(existing, first, storeId);
        }

        long itemId = TsidUtil.generate();

        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(itemId);
        item.setImportJobId(jobId);
        item.setStoreId(storeId);
        item.setProductKey(productKey);
        item.setStatus(ImportItemStatus.PARSED.value());
        item.setCommitRevision(1);
        item.setName(first.productName());
        item.setOrigin(first.origin());
        item.setTargetStatus(first.targetStatus().isBlank() ? null : first.targetStatus());
        item.setCertifications(parseCertifications(first.certifications()));
        item.setHashtags(parseHashtags(first.hashtags()));
        item.setFirstCsvLine(first.csvLine());
        item.setSkuCount(rows.size());
        item.setWarningCount(0);
        item.setBlockingErrorCount(0);
        item.setAllowDuplicateName(false);

        int blockingCount = 0;
        int warningCount = 0;
        List<ProductImportItemErrorEntity> errors = new ArrayList<>();

        ValidateLeafCategoryInternalResponse cat = categoryCache.get(first.categoryId());
        if (cat == null || !cat.getIsValid()) {
            errors.add(buildError(jobId, itemId, first.csvLine(), "category_id",
                    ImportItemErrorType.CATEGORY_INVALID.value(), "ERROR",
                    "分類驗證失敗" + (cat == null ? "" : ": " + cat.getInvalidReason()),
                    first.categoryId()));
            blockingCount++;
        } else {
            item.setCategoryId(parseLongOrNull(cat.getCatL3Id()));
            item.setCatL3Name(cat.getCatL3Name());
            item.setCatL2Id(parseLongOrNull(cat.getCatL2Id()));
            item.setCatL2Name(cat.getCatL2Name());
            item.setCatL1Id(parseLongOrNull(cat.getCatL1Id()));
            item.setCatL1Name(cat.getCatL1Name());
        }

        if (inJobNames.contains(first.productName())) {
            errors.add(buildError(jobId, itemId, first.csvLine(), "product_name",
                    ImportItemErrorType.DUPLICATE_NAME_IN_JOB.value(), "ERROR",
                    "同 job 內 product_name 重複", first.productName()));
            blockingCount++;
        }
        ExistingProductByName dup = existingNames.get(first.productName());
        if (dup != null) {
            // 40 §3.5：DUPLICATE_NAME_VS_EXISTING 同事務寫入 existing_product_* 反正規化欄位，
            // 供前端 DuplicateNameWarning 渲染（不另發 GetProductsBriefInternal RPC）。
            ProductImportItemErrorEntity dupErr = buildError(jobId, itemId, first.csvLine(), "product_name",
                    ImportItemErrorType.DUPLICATE_NAME_VS_EXISTING.value(), "ERROR",
                    "與既有商品同名（可勾選 allow_duplicate_name 化解）", first.productName());
            dupErr.setExistingProductId(parseLongOrNull(dup.getProductId()));
            dupErr.setExistingProductName(dup.getName());
            dupErr.setExistingProductStatusLabel(extractDupStatusLabel(dup));
            errors.add(dupErr);
            blockingCount++;
            item.setDuplicateExistingProductId(parseLongOrNull(dup.getProductId()));
        }

        boolean descIsUrl = first.description().toLowerCase().startsWith("http://")
                || first.description().toLowerCase().startsWith("https://");
        HtmlFetchEvent htmlEvent = null;
        if (!first.description().isBlank() && !descIsUrl) {
            HtmlToBlocksConverter.Result r = htmlConverter.convert(first.description());
            item.setDescriptionJson(r.descriptionJson());
            // §12.5a：HTML 轉換 warning 收集起來，與其他 errors 一起 INSERT
            for (HtmlToBlocksConverter.ConversionWarning w : r.warnings()) {
                ProductImportItemErrorEntity ew = new ProductImportItemErrorEntity();
                ew.setImportItemId(itemId);
                ew.setImportJobId(jobId);
                ew.setCsvLine(first.csvLine());
                ew.setErrorType(ImportItemErrorType.HTML_PARSE_WARNING.value());
                ew.setSeverity("WARNING");
                ew.setField("description");
                ew.setMessage(w.message());
                String detail = w.warningEnum() + ": " + (w.detail() == null ? "" : w.detail());
                ew.setRawValue(detail.length() > 500 ? detail.substring(0, 500) : detail);
                // 40 §3.5：HTML→JSON 轉換 warning 屬 HTML_PARSE source
                ew.setSource(ProductImportItemErrorEntity.SOURCE_HTML_PARSE);
                errors.add(ew);
                warningCount++;
            }
        }
        itemMapper.insert(item);

        List<CsvParser.ParsedRow> rowsForShipping = rows;
        int skuOrder = 0;
        for (CsvParser.ParsedRow row : rowsForShipping) {
            ProductImportItemSkuEntity sku = new ProductImportItemSkuEntity();
            sku.setId(TsidUtil.generate());
            sku.setImportItemId(itemId);
            sku.setCsvLine(row.csvLine());
            sku.setSkuName(row.skuName());
            sku.setPrice(parseLongOrNull(row.skuPrice()));
            sku.setSalePrice(parseLongOrNull(row.skuSalePrice()));
            sku.setStockQty(parseIntOrNull(row.skuStockQty()));
            sku.setWeight(parseBigDecimalOrNull(row.skuWeight()));
            // docs/開發規範/Java 微服務開發規範.md §2.1 / §6.4：DB 與 gRPC 契約一律存 enum value（CSV 寫 NAME 為了人類可讀，
            // L2 已驗合法 NAME，此處轉成 value 落地）。
            sku.setUnit(CodeMetaNormalizer.weightUnitToValue(row.skuUnit()));
            sku.setLengthCm(parseBigDecimalOrNull(row.skuLengthCm()));
            sku.setWidthCm(parseBigDecimalOrNull(row.skuWidthCm()));
            sku.setHeightCm(parseBigDecimalOrNull(row.skuHeightCm()));

            List<Map<String, Object>> shippingMethods = new ArrayList<>();
            String freeThreshold = row.skuFreeThreshold().isBlank() ? null : row.skuFreeThreshold();
            String feePayer = freeThreshold == null ? "BUYER" : "SELLER";
            boolean anyEnabled = false;
            anyEnabled |= appendShipping(shippingMethods, SHIPPING_VALUE_TAQBIN,
                    row.skuShippingTaqbin(), freeThreshold, feePayer, shippingEnabled, errors, jobId, itemId, row.csvLine());
            anyEnabled |= appendShipping(shippingMethods, SHIPPING_VALUE_SEVEN,
                    row.skuShippingSeven(), freeThreshold, feePayer, shippingEnabled, errors, jobId, itemId, row.csvLine());
            anyEnabled |= appendShipping(shippingMethods, SHIPPING_VALUE_UBER,
                    row.skuShippingUber(), freeThreshold, feePayer, shippingEnabled, errors, jobId, itemId, row.csvLine());
            if (!anyEnabled) {
                errors.add(buildError(jobId, itemId, row.csvLine(), "sku_shipping",
                        ImportItemErrorType.SHIPPING_ALL_DOWNGRADED.value(), "ERROR",
                        "本 row 所有 shipping 皆未開通 / 無效", ""));
                blockingCount++;
            }
            sku.setShippingMethods(shippingMethods);
            sku.setSortOrder(skuOrder++);
            skuMapper.insert(sku);
        }

        int totalAssets = 0;
        List<AssetDownloadEvent> assetEvents = new ArrayList<>();
        int sortOrder = 0;
        // 40 §4.3a / §5.2 step 5.2.f：gallery 上限與正常上架一致 = 9 張（ProductPublishConstants.MAX_GALLERY_IMAGES）。
        // CSV 來源圖片數 > 9 時截前 9 張保留，並寫一筆 WARNING `GALLERY_TRUNCATED_EXCEEDS_LIMIT`
        // 通知賣家後續可在編輯頁加回前面遺漏的圖。當前 CSV header 限 9 欄，結構上不會超過。
        for (int idx = 1; idx <= CsvParser.ParsedRow.MAX_GALLERY_IMAGES; idx++) {
            String url = first.productImage(idx);
            if (url == null || url.isBlank()) continue;
            long assetId = TsidUtil.generate();
            ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
            asset.setId(assetId);
            asset.setImportItemId(itemId);
            asset.setImportJobId(jobId);
            asset.setUsage(ImportAssetUsage.GALLERY.value());
            asset.setSortOrder(sortOrder++);
            asset.setSourceType("URL");
            asset.setSourceUrl(url);
            asset.setSourceUrlHash(sha256Hex(url.getBytes()));
            asset.setStatus(ImportAssetLocalStatus.PENDING.value());
            asset.setRetryCount(0);
            assetMapper.insert(asset);
            assetEvents.add(new AssetDownloadEvent(itemId, jobId, storeId, assetId, url,
                    ImportAssetUsage.GALLERY.value(), item.getCommitRevision()));
            totalAssets++;
        }
        // CSV 結構上限 = 9（CsvParser.ParsedRow.MAX_GALLERY_IMAGES）；目前 schema 不會達到 >9 場景，
        // 截斷 WARNING 留作未來擴充 CSV header（image_10+）時的提示路徑。若哪天 ParsedRow 加到 10+ 才會
        // 真正觸發。當前 collectedNonBlank ≤ 9 — 不寫 WARNING 屬正常。

        for (ProductImportItemErrorEntity e : errors) {
            errorMapper.insert(e);
        }
        item.setBlockingErrorCount(blockingCount);
        item.setWarningCount(warningCount);
        if (totalAssets == 0) {
            item.setStatus(ImportItemStatus.READY.value());
        } else {
            item.setStatus(ImportItemStatus.ASSET_DOWNLOADING.value());
        }
        itemMapper.updateById(item);

        List<HtmlFetchEvent> htmlEvents = new ArrayList<>();
        if (descIsUrl) {
            htmlEvents.add(new HtmlFetchEvent(itemId, jobId, storeId,
                    first.description(), item.getCommitRevision()));
        }
        return new PersistResult(totalAssets, assetEvents, htmlEvents, itemId);
    }

    /**
     * Issue #1029 replay 守護：listener requeue 時 reuse 既有 row。
     *
     * <p>既有 row 表示前次 REQUIRES_NEW per-item tx 已 atomic commit；本方法重建：
     * <ul>
     *   <li>AssetDownloadEvent：對 status=PENDING(9301) 的 asset 重派（DOWNLOADING+ 已派工，不重派）</li>
     *   <li>HtmlFetchEvent：existing.descriptionJson IS NULL AND in-memory first.description() 為 URL 時重派；
     *       即使與 html.fetch worker 寫回發生極短 race，HtmlFetchListener 以 commitRevision 守衛 dedup（§5.3）</li>
     * </ul>
     */
    private PersistResult rebuildPersistResultFromExisting(ProductImportItemEntity existing,
                                                            CsvParser.ParsedRow first,
                                                            Long storeId) {
        List<ProductImportItemAssetEntity> pendingAssets = assetMapper.selectList(
                new QueryWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", existing.getId())
                        .eq("status", ImportAssetLocalStatus.PENDING.value()));
        List<AssetDownloadEvent> assetEvents = new ArrayList<>(pendingAssets.size());
        for (ProductImportItemAssetEntity a : pendingAssets) {
            assetEvents.add(new AssetDownloadEvent(
                    existing.getId(),
                    existing.getImportJobId(),
                    storeId,
                    a.getId(),
                    a.getSourceUrl(),
                    a.getUsage(),
                    existing.getCommitRevision()));
        }

        List<HtmlFetchEvent> htmlEvents = new ArrayList<>();
        String desc = first.description();
        boolean descIsUrl = desc != null && (desc.toLowerCase().startsWith("http://")
                || desc.toLowerCase().startsWith("https://"));
        if (existing.getDescriptionJson() == null && descIsUrl) {
            htmlEvents.add(new HtmlFetchEvent(
                    existing.getId(),
                    existing.getImportJobId(),
                    storeId,
                    desc,
                    existing.getCommitRevision()));
        }
        return new PersistResult(assetEvents.size(), assetEvents, htmlEvents, existing.getId());
    }

    private boolean appendShipping(List<Map<String, Object>> methods, String methodValue, String temperature,
                                   String freeThreshold, String feePayer,
                                   Map<String, SellerShippingMethodView> enabled,
                                   List<ProductImportItemErrorEntity> errors,
                                   long jobId, long itemId, int csvLine) {
        if (temperature == null || temperature.isBlank()) return false;
        SellerShippingMethodView view = enabled.get(methodValue);
        if (view == null || !view.getAvailable()) {
            // 賣家錯誤訊息以 ShippingMethod.label() 呈現（「黑貓宅急便」），不暴露 enum value（"3001"）
            String label = ShippingMethod.isValidValue(methodValue)
                    ? ShippingMethod.fromValue(methodValue).label()
                    : methodValue;
            errors.add(buildError(jobId, itemId, csvLine, methodValue,
                    ImportItemErrorType.SHIPPING_NOT_ENABLED.value(), "WARNING",
                    "賣家未開通 " + label, temperature));
            return false;
        }
        Map<String, Object> m = new LinkedHashMap<>();
        // method 欄位也存 enum value（與 DB / RPC 契約一致；前端展示時用 ShippingMethod.fromValue().label()）
        m.put("method", methodValue);
        // docs/開發規範/Java 微服務開發規範.md §2.1 / §6.4：JSONB temperature 一律存 ShippingTemperature value（3201/3202/3203）
        m.put("temperature", CodeMetaNormalizer.shippingTemperatureToValue(temperature));
        m.put("freeThreshold", freeThreshold == null ? "" : freeThreshold);
        m.put("feePayer", feePayer);
        methods.add(m);
        return true;
    }

    /**
     * 40 §4.3a / §5.2 g：tx 外呼叫 ValidateProductDraft（CSV_PARSE context），把 violations 寫入
     * product_import_item_error。本方法包含「先 tx 外 RPC → 後短 tx 落 violations + 重算 blocking」。
     */
    private void runValidateProductDraftForParse(long itemId, long storeId, long jobId,
                                                  CsvParser.ParsedRow first) {
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null) return;
        List<ProductImportItemSkuEntity> skus = skuMapper.selectList(
                new com.baomidou.mybatisplus.core.conditions.query.QueryWrapper<ProductImportItemSkuEntity>()
                        .eq("import_item_id", itemId).orderByAsc("sort_order"));
        List<String> galleryUrls = new ArrayList<>();
        for (int i = 1; i <= CsvParser.ParsedRow.MAX_GALLERY_IMAGES; i++) {
            String u = first.productImage(i);
            if (u != null && !u.isBlank()) galleryUrls.add(u);
        }
        ProductDraft draft = ProductDraftBuilder.fromCsvParseSnapshot(item, skus, galleryUrls, null);
        ValidateProductDraftResponse resp = productClient.validateProductDraft(
                storeId, draft, ValidateProductDraftRequest.ValidationContext.CSV_PARSE);
        if (resp == null || resp.getViolationsCount() == 0) return;
        final List<DraftViolation> violations = resp.getViolationsList();
        // 40 §2.5:程式式 tx 包覆
        txTemplate.executeWithoutResult(status ->
                persistDraftViolationsBody(itemId, jobId, first.csvLine(), violations));
    }

    // 40 §2.5 / §5.2:tx body
    private void persistDraftViolationsBody(long itemId, long jobId, int csvLine,
                                            List<DraftViolation> violations) {
        // §4.3a wipe scope（parse 是 INSERT-only，不 wipe；但仍重算 blocking_error_count 以含本次新增 ERROR）
        for (DraftViolation v : violations) {
            ProductImportItemErrorEntity e = new ProductImportItemErrorEntity();
            e.setImportItemId(itemId);
            e.setImportJobId(jobId);
            e.setCsvLine(csvLine);
            e.setErrorType(v.getCode() == null ? "" : v.getCode());
            e.setSeverity(v.getSeverity() == null ? "ERROR" : v.getSeverity());
            e.setField(v.getField() == null ? "" : v.getField());
            e.setMessage(v.getMessage() == null ? "" : v.getMessage());
            e.setRawValue(v.getRawValue() == null ? "" : v.getRawValue());
            e.setSource(ProductImportItemErrorEntity.SOURCE_VALIDATE_PRODUCT_DRAFT);
            errorMapper.insert(e);
        }
        itemMapper.recalcBlockingErrorCount(itemId);
    }

    // 40 §2.5 / §5.2:tx body
    private void finalizeJobBody(Long jobId, int totalItems, boolean hasAssets) {
        com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper<ProductImportJobEntity> uw =
                new com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .set("total_items", totalItems);
        if (!hasAssets) {
            uw.set("status", ImportJobStatus.READY.value());
        }
        jobMapper.update(null, uw);
    }

    // 40 §2.5 / §5.2:tx body
    private void markJobParseFailedBody(Long jobId, String reason) {
        jobMapper.update(null,
                new com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .set("status", ImportJobStatus.PARSE_FAILED.value())
                        .set("parse_error", reason));
    }

    private void publish(String routingKey, Object event, String eventType) {
        try {
            publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT, routingKey,
                    MAPPER.writeValueAsBytes(event), eventType);
        } catch (Exception e) {
            log.error("CsvParseService.publish.error",
                    "Failed to publish event=" + routingKey + ", reason=" + e.getMessage(), e);
        }
    }

    private Set<String> uniqueValues(List<CsvParser.ParsedRow> rows,
                                     java.util.function.Function<CsvParser.ParsedRow, String> extractor) {
        Set<String> set = new HashSet<>();
        for (CsvParser.ParsedRow r : rows) {
            String v = extractor.apply(r);
            if (v != null && !v.isBlank()) set.add(v);
        }
        return set;
    }

    private List<Map<String, Object>> parseCertifications(String raw) {
        if (raw == null || raw.isBlank()) return new ArrayList<>();
        List<Map<String, Object>> list = new ArrayList<>();
        for (String part : raw.split(",")) {
            String s = part.trim();
            if (s.isEmpty()) continue;
            int colon = s.indexOf(':');
            if (colon <= 0 || colon == s.length() - 1) continue;
            // docs/開發規範/Java 微服務開發規範.md §2.1 / §6.4：cert.name 一律存 CertificationType value（3701~3706）
            String nameValue = CodeMetaNormalizer.certificationTypeToValue(s.substring(0, colon).trim());
            if (nameValue == null) continue;
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("name", nameValue);
            m.put("certNo", s.substring(colon + 1).trim());
            list.add(m);
        }
        return list;
    }

    private List<String> parseHashtags(String raw) {
        return HashtagInputSplitter.split(raw);
    }

    private ProductImportItemErrorEntity buildError(long jobId, long itemId, int csvLine, String field,
                                                    String type, String severity, String message, String raw) {
        ProductImportItemErrorEntity e = new ProductImportItemErrorEntity();
        e.setImportItemId(itemId);
        e.setImportJobId(jobId);
        e.setCsvLine(csvLine);
        e.setErrorType(type);
        e.setSeverity(severity);
        e.setField(field);
        e.setMessage(message);
        e.setRawValue(raw);
        // 40 §3.5：CSV parse 階段 L2 規則 / shipping / cert / duplicate-name 等預設來源
        e.setSource(ProductImportItemErrorEntity.SOURCE_L2_VALIDATOR);
        return e;
    }

    /**
     * 40 §3.5：將 ExistingProductByName.status（ProductStatus value，如 "4004"）轉為中文 label。
     * 若 value 不在 enum 內回原字串（避免投出例外影響主流程）。
     */
    private static String extractDupStatusLabel(ExistingProductByName dup) {
        String status = dup.getStatus();
        if (status == null || status.isBlank()) return "";
        try {
            return io.github.will413028.hawkr.common.utils.code.ProductStatus.fromValue(status).label();
        } catch (IllegalArgumentException ex) {
            return status;
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

    private static Long parseLongOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return Long.parseLong(v.trim()); } catch (NumberFormatException e) { return null; }
    }

    private static Integer parseIntOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return Integer.parseInt(v.trim()); } catch (NumberFormatException e) { return null; }
    }

    private static BigDecimal parseBigDecimalOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return new BigDecimal(v.trim()); } catch (NumberFormatException e) { return null; }
    }

    public record PersistResult(int assetCount,
                                List<AssetDownloadEvent> assetDownloadEvents,
                                List<HtmlFetchEvent> htmlFetchEvents,
                                Long importItemId) {}
}
