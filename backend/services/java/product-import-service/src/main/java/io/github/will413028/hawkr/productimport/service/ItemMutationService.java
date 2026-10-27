package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.media.grpc.BindStagedAssetResponse;
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
import io.github.will413028.hawkr.productimport.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.helper.CodeMetaNormalizer;
import io.github.will413028.hawkr.productimport.helper.HashtagInputSplitter;
import io.github.will413028.hawkr.productimport.helper.ProductDraftBuilder;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.AssetDownloadEvent;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Status;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.math.BigDecimal;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

@Service
public class ItemMutationService {

    private static final ObsLogger log = ObsLogger.forClass(ItemMutationService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String GALLERY_REF_TYPE = AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value();
    private static final Set<String> EDITABLE_STATUS = Set.of(
            ImportItemStatus.PARSED.value(),
            ImportItemStatus.ASSET_DOWNLOADING.value(),
            ImportItemStatus.READY.value(),
            ImportItemStatus.COMMIT_FAILED.value());
    private static final Set<String> ALLOWED_TARGET_STATUS = Set.of("ACTIVE", "DRAFT", "PENDING_REVIEW");

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemSkuMapper skuMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemErrorMapper errorMapper;
    private final ProductGrpcClient productClient;
    private final MediaGrpcClient mediaClient;
    private final ObsRabbitPublisher publisher;
    // 40 §2.5 Tx 載體規定:同 class self-invocation 方法用 TransactionTemplate 程式式包覆
    // (而非 @Transactional annotation;Spring AOP CGLIB proxy 不攔截 this.fooTx() 會讓 annotation 失效)
    private final TransactionTemplate txTemplate;

    public ItemMutationService(ProductImportJobMapper jobMapper,
                               ProductImportItemMapper itemMapper,
                               ProductImportItemSkuMapper skuMapper,
                               ProductImportItemAssetMapper assetMapper,
                               ProductImportItemErrorMapper errorMapper,
                               ProductGrpcClient productClient,
                               MediaGrpcClient mediaClient,
                               ObsRabbitPublisher publisher,
                               PlatformTransactionManager txManager) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.skuMapper = skuMapper;
        this.assetMapper = assetMapper;
        this.errorMapper = errorMapper;
        this.productClient = productClient;
        this.mediaClient = mediaClient;
        this.publisher = publisher;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    /** UpdateImportItem 主流程。 */
    public ProductImportItemEntity update(long storeId, long jobId, long itemId,
                                          Integer expectedVersion,
                                          UpdateInput input) {
        if (input.skusSet && (input.skus == null || input.skus.isEmpty())) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("skus must contain at least one entry when skusSet=true")
                    .asRuntimeException();
        }
        ProductImportItemEntity snapshot = itemMapper.selectById(itemId);
        if (snapshot == null || !snapshot.getImportJobId().equals(jobId)
                || !snapshot.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import item not found").asRuntimeException();
        }
        // 40 §2.2 / §4.6：COMMITTED item 為終態 immutable，明確拒絕並回 PRODUCT_IMPORT_ITEM_ALREADY_COMMITTED
        // 訊息（caller 端可顯示「該商品已併入正式商品，無法再編輯或移除」）。
        if (ImportItemStatus.COMMITTED.value().equals(snapshot.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("IMPORT_ITEM_ALREADY_COMMITTED: item already committed, immutable")
                    .asRuntimeException();
        }
        if (!EDITABLE_STATUS.contains(snapshot.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("item not editable in status=" + snapshot.getStatus())
                    .asRuntimeException();
        }

        // 40 號設計書 §5.5 階段 1.2 / Round 5 Obs #2：
        // 全無變動（例：rename 為原本 name）→ 直接回 200 + 原 snapshot，commit_revision 不變。
        // 比對 item-level *Set 欄位是否真有差異；SKU / asset / category 變動視為實質編輯（保守側）。
        if (isItemLevelNoop(snapshot, input)) {
            log.info("ItemMutationService.update.noop",
                    "itemId=" + itemId + " — all touched fields equal current snapshot, "
                            + "commit_revision stays at " + snapshot.getCommitRevision()
                            + ", rerun ValidateProductDraft");
            try {
                runValidateProductDraftForUpdate(snapshot, storeId);
            } catch (Exception e) {
                log.warn("ItemMutationService.validateDraft.noop.failed",
                        "itemId=" + itemId + ", reason=" + e.getMessage());
            }
            ProductImportItemEntity latest = itemMapper.selectById(itemId);
            return latest != null ? latest : snapshot;
        }

        ValidateLeafCategoryInternalResponse catResp = null;
        if (input.categoryIdSet && input.categoryId != null && !input.categoryId.isBlank()) {
            try {
                catResp = productClient.validateLeafCategory(input.categoryId);
            } catch (Exception e) {
                log.warn("ItemMutationService.cat_rpc_failed", "categoryId=" + input.categoryId);
            }
        }
        ExistingProductByName existing = null;
        if (input.nameSet && input.name != null && !input.name.isBlank()
                && !input.name.equals(snapshot.getName())) {
            try {
                BatchCheckSellerProductNamesResponse r =
                        productClient.batchCheckSellerProductNames(storeId, List.of(input.name));
                if (!r.getExistingList().isEmpty()) existing = r.getExistingList().get(0);
            } catch (Exception e) {
                log.warn("ItemMutationService.name_check_failed", "name=" + input.name);
            }
        }

        List<BindStagedResult> bindResults = new ArrayList<>();
        if (input.galleryImagesSet) {
            for (int i = 0; i < input.galleryImages.size(); i++) {
                ImageInput img = input.galleryImages.get(i);
                if (img.stagedToken != null && !img.stagedToken.isBlank()) {
                    try {
                        BindStagedAssetResponse br = mediaClient.bindStagedAsset(
                                img.stagedToken, storeId, jobId, itemId, GALLERY_REF_TYPE);
                        if (br.getCode() != GrpcResponseCode.SUCCESS.code()) {
                            // Media bizError 透過正常 response 回（非 gRPC StatusException）；
                            // 不檢查會靜默落入空 importAssetId / objectKey。對齊既有 catch 區塊語意，拋 INVALID_ARGUMENT
                            log.warn("ItemMutationService.bind_staged_bizError",
                                    "stagedToken=" + img.stagedToken
                                            + ", code=" + br.getCode() + ", msg=" + br.getMessage());
                            throw Status.INVALID_ARGUMENT
                                    .withDescription("BindStagedAsset bizError: code="
                                            + br.getCode() + " msg=" + br.getMessage())
                                    .asRuntimeException();
                        }
                        bindResults.add(new BindStagedResult(i, img.stagedToken,
                                br.getImportAssetId(), br.getObjectKey(), br.getPreviewUrl()));
                    } catch (Exception e) {
                        log.warn("ItemMutationService.bind_staged_failed",
                                "stagedToken=" + img.stagedToken + ", reason=" + e.getMessage());
                        throw Status.INVALID_ARGUMENT
                                .withDescription("BindStagedAsset failed: " + e.getMessage())
                                .asRuntimeException();
                    }
                }
            }
        }

        // 40 §2.5 / §5.5 階段 3:程式式 tx 包覆,任何 mid-flight throw(樂觀鎖、SKU validator、
        // CodeMetaNormalizer)都會 rollback,避免 DELETE 已執行 / INSERT 未補的破壞性副作用
        final ValidateLeafCategoryInternalResponse catRespFinal = catResp;
        final ExistingProductByName existingFinal = existing;
        ProductImportItemEntity updated = txTemplate.execute(status ->
                applyTxBody(storeId, jobId, itemId, expectedVersion, input,
                        catRespFinal, existingFinal, bindResults));

        // 40 §4.3a / §5.5 step 2.7：tx 外呼叫 ValidateProductDraft（IMPORT_UPDATE context），
        // 把 violations 落入 product_import_item_error（source=VALIDATE_PRODUCT_DRAFT），
        // 同事務先 wipe 舊 source=VALIDATE_PRODUCT_DRAFT row 再 INSERT。
        // 失敗不阻擋儲存（即使 ERROR 仍允許儲存，由 §5.7 commit 階段阻擋）。
        runValidateProductDraftForItems(storeId, List.of(updated.getId()));
        // 重新拿一次 entity（含 wipe 後重算的 blocking_error_count + status）
        ProductImportItemEntity latest = itemMapper.selectById(itemId);
        return latest != null ? latest : updated;
    }

    private void runValidateProductDraftForItems(long storeId, List<Long> itemIds) {
        if (itemIds == null || itemIds.isEmpty()) return;
        for (Long id : itemIds) {
            if (id == null) continue;
            ProductImportItemEntity item = itemMapper.selectById(id);
            if (item == null || ImportItemStatus.COMMITTED.value().equals(item.getStatus())) {
                continue;
            }
            try {
                runValidateProductDraftForUpdate(item, storeId);
            } catch (Exception e) {
                log.warn("ItemMutationService.validateDraft.items.failed",
                        "itemId=" + id + ", reason=" + e.getMessage());
            }
        }
    }

    private void runValidateProductDraftForUpdate(ProductImportItemEntity item, long storeId) {
        List<ProductImportItemSkuEntity> skus = skuMapper.selectList(
                new QueryWrapper<ProductImportItemSkuEntity>()
                        .eq("import_item_id", item.getId()).orderByAsc("sort_order"));
        List<ProductImportItemAssetEntity> assets = assetMapper.selectList(
                new QueryWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", item.getId()));
        ProductDraft draft = ProductDraftBuilder.fromUpdateSnapshot(item, skus, assets, null);
        ValidateProductDraftResponse resp = productClient.validateProductDraft(
                storeId, draft, ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE);
        // 40 §2.5 / §5.5c:程式式 tx 包覆。被 update() / batchUpdate* 二路 caller 共用 —
        // update() 路徑無 outer tx,txTemplate 開短 tx;batchUpdate* 路徑帶 outer @Transactional,
        // PROPAGATION_REQUIRED 自動 join,失敗會把 outer 標 rollback-only(更嚴謹的行為)
        final List<DraftViolation> finalViolations = resp == null ? List.of() : resp.getViolationsList();
        txTemplate.executeWithoutResult(status ->
                wipeAndPersistDraftViolationsBody(item.getId(), item.getImportJobId(),
                        item.getFirstCsvLine(), finalViolations));
    }

    // 40 §2.5 / §5.5c:tx body — 必須由 caller 包進 txTemplate。
    // 禁止標 @Transactional(self-invocation 會繞過 Spring AOP proxy)。
    private void wipeAndPersistDraftViolationsBody(long itemId, long jobId, Integer csvLine,
                                                   List<DraftViolation> violations) {
        // §4.3a wipe scope：先 DELETE 既有 source=VALIDATE_PRODUCT_DRAFT row 再 INSERT
        errorMapper.delete(new QueryWrapper<ProductImportItemErrorEntity>()
                .eq("import_item_id", itemId)
                .eq("source", ProductImportItemErrorEntity.SOURCE_VALIDATE_PRODUCT_DRAFT));
        for (DraftViolation v : violations) {
            ProductImportItemErrorEntity e = new ProductImportItemErrorEntity();
            e.setImportItemId(itemId);
            e.setImportJobId(jobId);
            e.setCsvLine(csvLine == null ? 0 : csvLine);
            e.setErrorType(v.getCode() == null ? "" : v.getCode());
            e.setSeverity(v.getSeverity() == null ? "ERROR" : v.getSeverity());
            e.setField(v.getField() == null ? "" : v.getField());
            e.setMessage(v.getMessage() == null ? "" : v.getMessage());
            e.setRawValue(v.getRawValue() == null ? "" : v.getRawValue());
            e.setSource(ProductImportItemErrorEntity.SOURCE_VALIDATE_PRODUCT_DRAFT);
            errorMapper.insert(e);
        }
        itemMapper.recalcBlockingErrorCount(itemId);
        itemMapper.recalcValidationCounts(itemId);
    }

    // 40 §2.5 / §5.5 階段 3:tx body — 必須由 caller 包進 txTemplate.execute(...)。
    // 禁止標 @Transactional(self-invocation 會繞過 Spring AOP CGLIB proxy 讓 annotation 失效)。
    private ProductImportItemEntity applyTxBody(long storeId, long jobId, long itemId,
                                                Integer expectedVersion,
                                                UpdateInput input,
                                                ValidateLeafCategoryInternalResponse catResp,
                                                ExistingProductByName existing,
                                                List<BindStagedResult> bindResults) {
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null) {
            throw Status.NOT_FOUND.withDescription("import item not found").asRuntimeException();
        }
        if (expectedVersion != null && !expectedVersion.equals(item.getVersion())) {
            throw Status.ABORTED
                    .withDescription("optimistic lock failed, expected=" + expectedVersion
                            + ", actual=" + item.getVersion())
                    .asRuntimeException();
        }

        if (input.nameSet) item.setName(input.name);
        if (input.originSet) item.setOrigin(input.origin);
        if (input.targetStatusSet) {
            if (input.targetStatus == null || input.targetStatus.isBlank()) {
                item.setTargetStatus(null);
            } else if (!ALLOWED_TARGET_STATUS.contains(input.targetStatus)) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("invalid target_status: " + input.targetStatus)
                        .asRuntimeException();
            } else {
                item.setTargetStatus(input.targetStatus);
            }
        }
        if (input.hashtagsSet) item.setHashtags(HashtagInputSplitter.splitAll(input.hashtags));
        if (input.certificationsSet) item.setCertifications(normalizeCertifications(input.certifications));
        if (input.descriptionSet) {
            try {
                item.setDescriptionJson(input.description == null || input.description.isBlank()
                        ? null
                        : MAPPER.readTree(input.description));
            } catch (Exception e) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("description JSON parse failed: " + e.getMessage())
                        .asRuntimeException();
            }
        }
        if (input.allowDuplicateNameSet) item.setAllowDuplicateName(input.allowDuplicateName);

        if (input.categoryIdSet) {
            if (input.categoryId == null || input.categoryId.isBlank()) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("categoryId is required when categoryIdSet=true")
                        .asRuntimeException();
            }
            if (catResp == null || !catResp.getIsValid()) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("category invalid"
                                + (catResp == null ? "" : ": " + catResp.getInvalidReason()))
                        .asRuntimeException();
            }
            item.setCategoryId(parseLongOrNull(catResp.getCatL3Id()));
            item.setCatL3Name(catResp.getCatL3Name());
            item.setCatL2Id(parseLongOrNull(catResp.getCatL2Id()));
            item.setCatL2Name(catResp.getCatL2Name());
            item.setCatL1Id(parseLongOrNull(catResp.getCatL1Id()));
            item.setCatL1Name(catResp.getCatL1Name());
            // 40 §4.2.8 LayerJob ERROR wipe（issue #1035 #4）：
            // catResp valid 已確認上面新 categoryId 是合法葉節點，clean 舊 category 全家族 stale row。
            // 涵蓋 CSV parse 寫入的 CATEGORY_INVALID（source=L2_VALIDATOR）與
            // ProductPublishValidator 寫入的 CATEGORY_NOT_FOUND / CATEGORY_NOT_LEAF
            // （source=VALIDATE_PRODUCT_DRAFT；雖然後續 wipeAndPersistDraftViolations 會以 source 重建，
            // 此處先清避免短暫狀態漏算）。
            deleteCategoryErrors(List.of(itemId));
        }

        // 僅清除本次 update 會重新計算的 error type（DUPLICATE_NAME_VS_EXISTING、CATEGORY_INVALID — 已於上方 categoryId block 處理）。
        // 其他 error（SHIPPING_*、SKU 級驗證等）由 CSV parse / SKU 編輯路徑各自維護，
        // 不可整列 wipe 否則賣家只改 name 會把 SKU 級 blocking errors 異常清零。
        if (input.nameSet) {
            errorMapper.delete(new QueryWrapper<ProductImportItemErrorEntity>()
                    .eq("import_item_id", itemId)
                    .eq("error_type", ImportItemErrorType.DUPLICATE_NAME_VS_EXISTING.value()));
        }
        int duplicateBlocking = 0;
        if (input.nameSet) {
            if (existing != null) {
                ProductImportItemErrorEntity e = buildError(jobId, itemId, item.getFirstCsvLine(),
                        "product_name", ImportItemErrorType.DUPLICATE_NAME_VS_EXISTING.value(), "ERROR",
                        "與既有商品同名（可勾選 allow_duplicate_name 化解）", input.name);
                errorMapper.insert(e);
                item.setDuplicateExistingProductId(parseLongOrNull(existing.getProductId()));
                if (Boolean.FALSE.equals(item.getAllowDuplicateName())) duplicateBlocking++;
            } else {
                item.setDuplicateExistingProductId(null);
            }
        }

        // blocking_error_count 重算延後到 skusSet block 之後（line 380+），
        // 確保 SHIPPING_ALL_DOWNGRADED 的 wipe + 重驗 insert 都已完成才 SELECT COUNT。
        // 注意 DUPLICATE_NAME_VS_EXISTING 在 allow_duplicate_name=true 時不算 blocking，
        // 故 SELECT 時 ne 該 type。

        List<AssetDownloadEvent> assetEvents = new ArrayList<>();
        if (input.galleryImagesSet) {
            // 40 號設計書 §5.5 階段 3.6：content-key based diff，禁止整列 wipe + 重插。
            // 既有非 ORPHAN GALLERY asset 依 (sourceType, key) 建 map：
            //   STAGED → key = stagedToken
            //   URL    → key = sourceUrlHash
            // 命中 input list 的既有 asset 保留 row（更新 sort_order），不重 INSERT、不 emit event。
            // 不在 input list 中的既有 asset 標 ORPHAN。
            List<ProductImportItemAssetEntity> existingGallery = assetMapper.selectList(
                    new QueryWrapper<ProductImportItemAssetEntity>()
                            .eq("import_item_id", itemId)
                            .eq("usage", ImportAssetUsage.GALLERY.value())
                            .ne("status", ImportAssetLocalStatus.ORPHAN.value()));
            Map<String, ProductImportItemAssetEntity> existingByKey = new LinkedHashMap<>();
            for (ProductImportItemAssetEntity ex : existingGallery) {
                String key = galleryContentKey(ex.getSourceType(), ex.getStagedToken(), ex.getSourceUrlHash());
                if (key != null) {
                    existingByKey.putIfAbsent(key, ex);
                }
            }
            Set<Long> keptAssetIds = new HashSet<>();
            for (int i = 0; i < input.galleryImages.size(); i++) {
                ImageInput img = input.galleryImages.get(i);
                final int idx = i;
                boolean isStaged = img.stagedToken != null && !img.stagedToken.isBlank();
                String inputKey = isStaged
                        ? "S:" + img.stagedToken
                        : "U:" + sha256Hex(img.sourceUrl.getBytes());
                ProductImportItemAssetEntity hit = existingByKey.get(inputKey);
                if (hit != null) {
                    // 命中既有 asset：更新 sort_order 與必要 metadata，保留 status / media_import_asset_id
                    if (hit.getSortOrder() == null || hit.getSortOrder() != i) {
                        ProductImportItemAssetEntity upd = new ProductImportItemAssetEntity();
                        upd.setId(hit.getId());
                        upd.setSortOrder(i);
                        assetMapper.updateById(upd);
                    }
                    keptAssetIds.add(hit.getId());
                    continue;
                }
                // 沒命中 → INSERT 新 asset
                long assetId = TsidUtil.generate();
                ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
                asset.setId(assetId);
                asset.setImportItemId(itemId);
                asset.setImportJobId(jobId);
                asset.setUsage(ImportAssetUsage.GALLERY.value());
                asset.setSortOrder(i);
                asset.setRetryCount(0);
                if (isStaged) {
                    BindStagedResult br = bindResults.stream()
                            .filter(b -> b.index == idx)
                            .findFirst().orElse(null);
                    asset.setSourceType("STAGED");
                    asset.setStagedToken(img.stagedToken);
                    if (br != null) {
                        asset.setMediaImportAssetId(br.importAssetId);
                        asset.setObjectKey(br.objectKey);
                        asset.setPreviewUrl(br.previewUrl);
                    }
                    asset.setStatus(ImportAssetLocalStatus.UPLOADED.value());
                } else {
                    asset.setSourceType("URL");
                    asset.setSourceUrl(img.sourceUrl);
                    asset.setSourceUrlHash(sha256Hex(img.sourceUrl.getBytes()));
                    asset.setStatus(ImportAssetLocalStatus.PENDING.value());
                }
                assetMapper.insert(asset);
                keptAssetIds.add(assetId);
                if ("URL".equals(asset.getSourceType())) {
                    assetEvents.add(new AssetDownloadEvent(itemId, jobId, storeId, assetId,
                            img.sourceUrl, ImportAssetUsage.GALLERY.value(),
                            item.getCommitRevision() + 1));
                }
            }
            // 既有但 input 沒命中的 asset → 標 ORPHAN
            List<Long> orphanIds = new ArrayList<>();
            for (ProductImportItemAssetEntity ex : existingGallery) {
                if (!keptAssetIds.contains(ex.getId())) {
                    orphanIds.add(ex.getId());
                }
            }
            if (!orphanIds.isEmpty()) {
                assetMapper.update(null,
                        new com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper<ProductImportItemAssetEntity>()
                                .in("id", orphanIds)
                                .set("status", ImportAssetLocalStatus.ORPHAN.value()));
            }
        }

        if (input.skusSet) {
            // 40 §5.5 階段 3.7 業務驗證：input.skus 為 authoritative full list，
            // 「reconciliation 後 SKU = 0」必須在 mutation 前先擋，給賣家明確錯誤訊息
            // 而非無聲刪光 SKU 然後在 ValidateProductDraft 才出現 SKUS_REQUIRED。
            // 註：txTemplate 包覆下 mid-flight throw 仍會 rollback；保留 pre-check 純粹是 UX
            // 與「early reject 不做 DB work」考量。預測規則：input.skus 中 !delete &&
            // (skuId == null || DB 上 row 屬於本 item) 視為「會保留 1 筆」。
            int predictedKeep = 0;
            for (SkuInput s : input.skus) {
                if (s.delete) continue;
                if (s.skuId == null) {
                    predictedKeep++;
                    continue;
                }
                ProductImportItemSkuEntity existingSku = skuMapper.selectById(s.skuId);
                if (existingSku != null && itemId == existingSku.getImportItemId()) {
                    predictedKeep++;
                }
            }
            if (predictedKeep == 0) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("skus must contain at least one entry that survives reconciliation "
                                + "(non-delete + skuId either null or pointing to a valid existing row)")
                        .asRuntimeException();
            }

            // 40 §4.2.8 LayerJob ERROR wipe（issue #1035 #4）：
            // 賣家透過 UpdateImportItem 修 SKU shipping_methods 後，CSV parse 階段寫入的
            // shipping stale rows 必須清除，否則 blocking_error_count / warning_count
            // 仍含舊 row。重驗於 SKU upsert 完成後由 ValidateProductDraft 共同尾端重建。
            deleteShippingErrors(List.of(itemId));

            // 40 號設計書 §5.5 階段 3.7：input.skus 為 authoritative full list。
            // 既有 SKU 不在 input.sku_id list 中 → 視為刪除（行為對齊 §3.6 gallery）。
            // sku_count 用處理後實際保留的 row 數（禁止用 input.skus.size，因 input 可能含 delete=true entries）。
            List<Long> existingSkuIds = skuMapper.selectList(
                    new QueryWrapper<ProductImportItemSkuEntity>()
                            .select("id")
                            .eq("import_item_id", itemId)
            ).stream().map(ProductImportItemSkuEntity::getId).toList();
            Set<Long> keptIds = new HashSet<>();
            int order = 0;
            int allDowngradedSkus = 0;
            for (SkuInput s : input.skus) {
                if (s.delete) {
                    if (s.skuId != null) {
                        skuMapper.deleteById(s.skuId);
                    }
                    continue;
                }
                Long skuId = s.skuId;
                ProductImportItemSkuEntity sku;
                if (skuId == null) {
                    sku = new ProductImportItemSkuEntity();
                    sku.setId(TsidUtil.generate());
                    sku.setImportItemId(itemId);
                    sku.setCsvLine(0);
                } else {
                    sku = skuMapper.selectById(skuId);
                    if (sku == null || !sku.getImportItemId().equals(itemId)) continue;
                }
                sku.setSkuName(s.skuName);
                sku.setPrice(parseLongOrNull(s.price));
                sku.setSalePrice(parseLongOrNull(s.salePrice));
                sku.setStockQty(parseIntOrNull(s.stockQty));
                sku.setWeight(parseBigDecimalOrNull(s.weight));
                // §4.1 / §8.4 / §11.1a：API 邊界 strict — invalid 立即 INVALID_ARGUMENT，
                // 不再 fallback null 讓 commit 階段才暴露錯誤
                sku.setUnit(CodeMetaNormalizer.requireValidWeightUnit(s.unit, "sku.unit"));
                sku.setLengthCm(parseBigDecimalOrNull(s.lengthCm));
                sku.setWidthCm(parseBigDecimalOrNull(s.widthCm));
                sku.setHeightCm(parseBigDecimalOrNull(s.heightCm));
                List<Map<String, Object>> normalizedShipping = normalizeShippingMethods(s.shippingMethods);
                sku.setShippingMethods(normalizedShipping);
                if (normalizedShipping == null || normalizedShipping.isEmpty()) {
                    allDowngradedSkus++;
                }
                sku.setSortOrder(order++);
                if (skuId == null) {
                    skuMapper.insert(sku);
                    keptIds.add(sku.getId());
                } else {
                    skuMapper.updateById(sku);
                    keptIds.add(skuId);
                }
            }
            // Reconcile：既有 SKU 不在 keptIds（也未在 input 顯式 delete）→ 視為刪除
            List<Long> toDelete = new ArrayList<>();
            for (Long existingId : existingSkuIds) {
                if (!keptIds.contains(existingId)) {
                    toDelete.add(existingId);
                }
            }
            if (!toDelete.isEmpty()) {
                skuMapper.deleteBatchIds(toDelete);
            }
            // 40 §5.5 final guard 已移到 SKU loop 之前作為 pre-check（見上方 predictedKeep）。
            // 純為 UX：early reject 不做 DB work。tx 安全已由 caller 的 txTemplate 包覆提供。
            item.setSkuCount(order);

            // 重驗後若仍有 SKU 全降級 → 寫一筆 item 級 SHIPPING_ALL_DOWNGRADED ERROR
            // （與 CSV parse 階段一致，避免 blocking_error_count 重算後反而被誤判可 commit）
            if (allDowngradedSkus > 0) {
                ProductImportItemErrorEntity e = buildError(jobId, itemId, item.getFirstCsvLine(),
                        "sku_shipping", ImportItemErrorType.SHIPPING_ALL_DOWNGRADED.value(), "ERROR",
                        allDowngradedSkus + " 個 SKU 所有 shipping 皆未開通 / 無效", "");
                errorMapper.insert(e);
            }
        }

        // 統一在所有 wipe / insert 之後重算 blocking_error_count：
        // DB 上現存 ERROR 級 errors（已 wipe 過 CATEGORY_INVALID / SHIPPING_ALL_DOWNGRADED stale row + 含本次 insert 新值）
        // + 本次 update 新加入的 duplicate-name blocking。
        Long currentErrors = errorMapper.selectCount(new QueryWrapper<ProductImportItemErrorEntity>()
                .eq("import_item_id", itemId)
                .eq("severity", "ERROR")
                .ne("error_type", ImportItemErrorType.DUPLICATE_NAME_VS_EXISTING.value()));
        int blocking = (currentErrors == null ? 0 : currentErrors.intValue()) + duplicateBlocking;
        item.setBlockingErrorCount(blocking);

        item.setCommitRevision((item.getCommitRevision() == null ? 1 : item.getCommitRevision()) + 1);
        if (assetEvents.isEmpty() && blocking == 0
                && !ImportItemStatus.COMMITTED.value().equals(item.getStatus())) {
            item.setStatus(ImportItemStatus.READY.value());
        } else if (!assetEvents.isEmpty()) {
            item.setStatus(ImportItemStatus.ASSET_DOWNLOADING.value());
        }
        itemMapper.updateById(item);

        for (AssetDownloadEvent ev : assetEvents) {
            try {
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD,
                        MAPPER.writeValueAsBytes(ev),
                        ProductImportEventType.PRODUCT_IMPORT_ASSET_DOWNLOAD.value());
            } catch (Exception e) {
                throw new IllegalStateException("publish asset.download failed", e);
            }
        }
        return item;
    }

    @Transactional
    public int removeItems(long storeId, long jobId, List<Long> itemIds) {
        ensureJobOwnership(storeId, jobId);
        if (itemIds == null || itemIds.isEmpty()) return 0;
        // 40 §2.2：COMMITTED item 屬唯讀，從 batch 中剔除；既有 notIn 已過濾，
        // 此處 SELECT 後比對 input 數量供 caller 知曉「有 N 個因已 COMMITTED 被剔除」。
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId).in("id", itemIds)
                .notIn("status", ImportItemStatus.COMMITTED.value(),
                        ImportItemStatus.COMMITTING.value(),
                        ImportItemStatus.REMOVED.value());
        List<ProductImportItemEntity> items = itemMapper.selectList(qw);
        int n = 0;
        for (ProductImportItemEntity it : items) {
            it.setStatus(ImportItemStatus.REMOVED.value());
            it.setRemovedAt(DateTimeUtils.nowUtc());
            itemMapper.updateById(it);
            n++;
        }
        return n;
    }

    @Transactional
    public int restoreItems(long storeId, long jobId, List<Long> itemIds) {
        ensureJobOwnership(storeId, jobId);
        if (itemIds == null || itemIds.isEmpty()) return 0;
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId).in("id", itemIds)
                .eq("status", ImportItemStatus.REMOVED.value());
        List<ProductImportItemEntity> items = itemMapper.selectList(qw);
        int n = 0;
        for (ProductImportItemEntity it : items) {
            String newStatus = it.getBlockingErrorCount() != null && it.getBlockingErrorCount() > 0
                    ? ImportItemStatus.PARSED.value()
                    : ImportItemStatus.READY.value();
            // 用 UpdateWrapper 強制 set removed_at = NULL —
            // MyBatis-Plus updateById 預設 FieldStrategy.NOT_NULL 會忽略 null 欄位，
            // 造成 status 改了但 removed_at 仍保留舊時間戳，eligible filter 看到後拒 commit。
            itemMapper.update(null,
                    new com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper<ProductImportItemEntity>()
                            .eq("id", it.getId())
                            .eq("version", it.getVersion())
                            .set("status", newStatus)
                            .set("removed_at", null)
                            .setSql("version = version + 1, updated_at = NOW()"));
            n++;
        }
        return n;
    }

    @Transactional
    public int retryAssets(long storeId, long jobId, long itemId, List<Long> assetIds) {
        ensureJobOwnership(storeId, jobId);
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null || !item.getImportJobId().equals(jobId)) {
            throw Status.NOT_FOUND.withDescription("import item not found").asRuntimeException();
        }
        // 40 §2.2：COMMITTED item 唯讀，asset 已 LINKED，不可重試下載
        if (ImportItemStatus.COMMITTED.value().equals(item.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("IMPORT_ITEM_ALREADY_COMMITTED: item already committed, immutable")
                    .asRuntimeException();
        }
        QueryWrapper<ProductImportItemAssetEntity> qw = new QueryWrapper<>();
        qw.eq("import_item_id", itemId).eq("status", ImportAssetLocalStatus.FAILED.value());
        if (assetIds != null && !assetIds.isEmpty()) qw.in("id", assetIds);
        List<ProductImportItemAssetEntity> failedAssets = assetMapper.selectList(qw);
        int n = 0;
        for (ProductImportItemAssetEntity a : failedAssets) {
            a.setStatus(ImportAssetLocalStatus.PENDING.value());
            a.setRetryCount(0);
            a.setFailureReason(null);
            a.setFailureDetail(null);
            assetMapper.updateById(a);
            try {
                AssetDownloadEvent ev = new AssetDownloadEvent(itemId, jobId, storeId, a.getId(),
                        a.getSourceUrl(), a.getUsage(), item.getCommitRevision());
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD,
                        MAPPER.writeValueAsBytes(ev),
                        ProductImportEventType.PRODUCT_IMPORT_ASSET_DOWNLOAD.value());
                n++;
            } catch (Exception e) {
                log.error("ItemMutationService.retryAssets.publish", "publish failed", e);
            }
        }
        if (n > 0 && ImportItemStatus.READY.value().equals(item.getStatus())) {
            item.setStatus(ImportItemStatus.ASSET_DOWNLOADING.value());
            itemMapper.updateById(item);
        }
        return n;
    }

    @Transactional
    public int batchUpdateTargetStatus(long storeId, long jobId, List<Long> itemIds, boolean applyAll,
                                       String targetStatus) {
        if (!ALLOWED_TARGET_STATUS.contains(targetStatus)) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("invalid target_status: " + targetStatus).asRuntimeException();
        }
        ensureJobOwnership(storeId, jobId);
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId)
                .notIn("status", ImportItemStatus.COMMITTED.value(),
                        ImportItemStatus.COMMITTING.value(),
                        ImportItemStatus.REMOVED.value());
        if (!applyAll && itemIds != null && !itemIds.isEmpty()) qw.in("id", itemIds);
        List<ProductImportItemEntity> items = itemMapper.selectList(qw);
        for (ProductImportItemEntity it : items) {
            it.setTargetStatus(targetStatus);
            it.setCommitRevision((it.getCommitRevision() == null ? 1 : it.getCommitRevision()) + 1);
            itemMapper.updateById(it);
        }
        runValidateProductDraftForItems(storeId, items.stream().map(ProductImportItemEntity::getId).toList());
        return items.size();
    }

    @Transactional
    public BatchCategoryResult batchUpdateCategory(long storeId, long jobId, List<Long> itemIds,
                                                   boolean applyAll, String categoryId) {
        ensureJobOwnership(storeId, jobId);
        ValidateLeafCategoryInternalResponse cat;
        try {
            cat = productClient.validateLeafCategory(categoryId);
        } catch (Exception e) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("category validate failed: " + e.getMessage())
                    .asRuntimeException();
        }
        if (cat == null || !cat.getIsValid()) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("category invalid" + (cat == null ? "" : ": " + cat.getInvalidReason()))
                    .asRuntimeException();
        }
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId)
                .notIn("status", ImportItemStatus.COMMITTED.value(),
                        ImportItemStatus.COMMITTING.value(),
                        ImportItemStatus.REMOVED.value());
        if (!applyAll && itemIds != null && !itemIds.isEmpty()) qw.in("id", itemIds);
        List<ProductImportItemEntity> items = itemMapper.selectList(qw);
        for (ProductImportItemEntity it : items) {
            it.setCategoryId(parseLongOrNull(cat.getCatL3Id()));
            it.setCatL3Name(cat.getCatL3Name());
            it.setCatL2Id(parseLongOrNull(cat.getCatL2Id()));
            it.setCatL2Name(cat.getCatL2Name());
            it.setCatL1Id(parseLongOrNull(cat.getCatL1Id()));
            it.setCatL1Name(cat.getCatL1Name());
            it.setCommitRevision((it.getCommitRevision() == null ? 1 : it.getCommitRevision()) + 1);
            itemMapper.updateById(it);
        }
        if (!items.isEmpty()) {
            List<Long> affectedItemIds = items.stream().map(ProductImportItemEntity::getId).toList();
            // 清 category 全家族 stale row（CATEGORY_INVALID/CATEGORY_NOT_FOUND/CATEGORY_NOT_LEAF），
            // 再以 ValidateProductDraft 重驗。對齊 §5.5 update categoryIdSet path 的清理範圍，
            // 避免只清 CATEGORY_INVALID 導致 ProductPublishValidator stale row 殘留。
            deleteCategoryErrors(affectedItemIds);
            runValidateProductDraftForItems(storeId, affectedItemIds);
        }
        return new BatchCategoryResult(items.size(), cat);
    }

    @Transactional
    public BatchShippingResult batchUpdateShipping(long storeId, long jobId, List<Long> itemIds,
                                                   boolean applyAll,
                                                   List<ShippingMethodInputEntry> methods) {
        ensureJobOwnership(storeId, jobId);
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId)
                .notIn("status", ImportItemStatus.COMMITTED.value(),
                        ImportItemStatus.COMMITTING.value(),
                        ImportItemStatus.REMOVED.value());
        if (!applyAll && itemIds != null && !itemIds.isEmpty()) qw.in("id", itemIds);
        List<ProductImportItemEntity> items = itemMapper.selectList(qw);
        int affectedSkus = 0;
        List<Map<String, Object>> serializedMethods = new ArrayList<>();
        for (ShippingMethodInputEntry e : methods) {
            String feePayer = (e.freeThreshold == null || e.freeThreshold.isBlank()) ? "BUYER" : "SELLER";
            Map<String, Object> m = new LinkedHashMap<>();
            // §4.1 / §8.4 / §11.1a：method 也 strict 正規化為 ShippingMethod value（"3001" 等），
            // 對齊 §3.3 ShippingMethodListTypeHandler 「method 為 ShippingMethod value」契約
            m.put("method", CodeMetaNormalizer.requireValidShippingMethod(
                    e.type, "shipping_methods.method"));
            // §4.1 / §8.4 / §11.1a：API 邊界 strict — invalid 立即拋 INVALID_ARGUMENT
            // 避免靜默存 null 後在 commit 階段才以「temperature=null」訊息誤導賣家
            m.put("temperature", CodeMetaNormalizer.requireValidShippingTemperature(
                    e.temperature, "shipping_methods.temperature"));
            m.put("freeThreshold", e.freeThreshold == null ? "" : e.freeThreshold);
            m.put("feePayer", feePayer);
            serializedMethods.add(m);
        }
        for (ProductImportItemEntity it : items) {
            List<ProductImportItemSkuEntity> skus = skuMapper.selectList(
                    new QueryWrapper<ProductImportItemSkuEntity>().eq("import_item_id", it.getId()));
            for (ProductImportItemSkuEntity s : skus) {
                s.setShippingMethods(serializedMethods);
                skuMapper.updateById(s);
                affectedSkus++;
            }
            it.setCommitRevision((it.getCommitRevision() == null ? 1 : it.getCommitRevision()) + 1);
            itemMapper.updateById(it);
        }
        List<Long> affectedItemIds = items.stream().map(ProductImportItemEntity::getId).toList();
        deleteShippingErrors(affectedItemIds);
        runValidateProductDraftForItems(storeId, affectedItemIds);
        return new BatchShippingResult(items.size(), affectedSkus);
    }

    private void deleteShippingErrors(List<Long> itemIds) {
        if (itemIds == null || itemIds.isEmpty()) return;
        errorMapper.delete(new QueryWrapper<ProductImportItemErrorEntity>()
                .in("import_item_id", itemIds)
                .in("error_type", List.of(
                        ImportItemErrorType.SHIPPING_AT_LEAST_ONE_REQUIRED.value(),
                        ImportItemErrorType.SHIPPING_DIMENSION_MISSING.value(),
                        ImportItemErrorType.SHIPPING_WEIGHT_EXCEEDED.value(),
                        ImportItemErrorType.SHIPPING_DIMENSION_EXCEEDED.value(),
                        ImportItemErrorType.SHIPPING_TEMP_INCOMPATIBLE.value(),
                        ImportItemErrorType.SHIPPING_NOT_ENABLED.value(),
                        ImportItemErrorType.SHIPPING_ALL_DOWNGRADED.value())));
    }

    /**
     * 清 category 全家族 stale error row。涵蓋三條來源：
     * <ul>
     *   <li>CSV parse / L2 validator 寫入：{@link ImportItemErrorType#CATEGORY_INVALID}</li>
     *   <li>ProductPublishValidator 寫入（source=VALIDATE_PRODUCT_DRAFT）：
     *       {@link ImportItemErrorType#CATEGORY_NOT_FOUND} / {@link ImportItemErrorType#CATEGORY_NOT_LEAF}</li>
     * </ul>
     * 設計：source-agnostic，只看 error_type；新 category 套用後重驗會由
     * {@code wipeAndPersistDraftViolations} 以 source=VALIDATE_PRODUCT_DRAFT 重建合法狀態，
     * 此 helper 確保「ValidateProductDraft 跑前」UI 看到的 brief 計數已歸零。
     */
    private void deleteCategoryErrors(List<Long> itemIds) {
        if (itemIds == null || itemIds.isEmpty()) return;
        errorMapper.delete(new QueryWrapper<ProductImportItemErrorEntity>()
                .in("import_item_id", itemIds)
                .in("error_type", List.of(
                        ImportItemErrorType.CATEGORY_INVALID.value(),
                        ImportItemErrorType.CATEGORY_NOT_FOUND.value(),
                        ImportItemErrorType.CATEGORY_NOT_LEAF.value())));
    }

    private void ensureJobOwnership(long storeId, long jobId) {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import job not found").asRuntimeException();
        }
        if (ImportJobStatus.COMMITTING.value().equals(job.getStatus())
                || ImportJobStatus.COMMITTED.value().equals(job.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("job not editable in status=" + job.getStatus())
                    .asRuntimeException();
        }
    }

    private ProductImportItemErrorEntity buildError(long jobId, long itemId, Integer csvLine,
                                                    String field, String type, String severity,
                                                    String message, String raw) {
        ProductImportItemErrorEntity e = new ProductImportItemErrorEntity();
        e.setImportItemId(itemId);
        e.setImportJobId(jobId);
        e.setCsvLine(csvLine == null ? 0 : csvLine);
        e.setErrorType(type);
        e.setSeverity(severity);
        e.setField(field);
        e.setMessage(message);
        e.setRawValue(raw);
        return e;
    }

    /**
     * Round 5 Obs #2 / §5.5：判定是否「真正無變動」。
     *
     * <p>保守側 — 任一以下情境視為「實質變動」（不短路）：
     * <ul>
     *   <li>SKU 編輯（input.skusSet）</li>
     *   <li>Gallery 圖片編輯（input.galleryImagesSet 且 {@link #isGalleryNoop} 判定有變動 — 含新 staged_token / 與既有 content-key set 不同）</li>
     *   <li>分類 ID 變動（input.categoryIdSet + 與 snapshot.categoryId 不同）</li>
     *   <li>description 編輯（input.descriptionSet — 純比較 JSON 字串等價較複雜，視為變動）</li>
     * </ul>
     * 其餘 item-level 純量欄位逐一比對；若所有觸碰欄位皆等於 snapshot，回 true。
     */
    private boolean isItemLevelNoop(ProductImportItemEntity snapshot, UpdateInput input) {
        if (input.skusSet) return false;
        // 40 §5.5 階段 1.2 gallery noop 深度防禦（issue #1053）：
        // 前端把 preview_url 誤當 identity 送 source_url 會讓 galleryImagesSet=true 但內容等價，
        // 後端不能只看 *Set boolean 短路；必須比對 content-key set 是否完全相同。
        if (input.galleryImagesSet && !isGalleryNoop(snapshot.getId(), input.galleryImages)) return false;
        if (input.descriptionSet) return false;
        if (input.categoryIdSet) {
            String currentCatId = snapshot.getCategoryId() == null
                    ? null : String.valueOf(snapshot.getCategoryId());
            if (!equalsNullSafe(emptyToNull(input.categoryId), currentCatId)) return false;
        }
        if (input.nameSet && !equalsNullSafe(emptyToNull(input.name), snapshot.getName())) return false;
        if (input.originSet && !equalsNullSafe(emptyToNull(input.origin), snapshot.getOrigin())) return false;
        if (input.targetStatusSet) {
            String inTs = (input.targetStatus == null || input.targetStatus.isBlank()) ? null : input.targetStatus;
            if (!equalsNullSafe(inTs, snapshot.getTargetStatus())) return false;
        }
        if (input.allowDuplicateNameSet) {
            Boolean inFlag = input.allowDuplicateName;
            Boolean curFlag = snapshot.getAllowDuplicateName();
            if (!equalsNullSafe(inFlag, curFlag)) return false;
        }
        if (input.hashtagsSet) {
            List<String> inTags = HashtagInputSplitter.splitAll(input.hashtags);
            List<String> curTags = snapshot.getHashtags() == null ? List.of() : snapshot.getHashtags();
            if (!inTags.equals(curTags)) return false;
        }
        if (input.certificationsSet) {
            // normalizeCertifications 內含 strict 驗證；noop check 階段只取 normalized 結果做比對
            List<Map<String, Object>> normalized = normalizeCertifications(input.certifications);
            List<Map<String, Object>> current = snapshot.getCertifications() == null
                    ? List.of() : snapshot.getCertifications();
            List<Map<String, Object>> normSafe = normalized == null ? List.of() : normalized;
            if (!certificationsEquivalent(normSafe, current)) return false;
        }
        // 走到這 = 全部觸碰欄位都等於 snapshot；任何 *Set 為 false 的欄位本來就不會被改
        return true;
    }

    private static String emptyToNull(String s) {
        return (s == null || s.isBlank()) ? null : s;
    }

    private static boolean equalsNullSafe(Object a, Object b) {
        return a == null ? b == null : a.equals(b);
    }

    private static boolean certificationsEquivalent(List<Map<String, Object>> input,
                                                    List<Map<String, Object>> current) {
        if (input.size() != current.size()) return false;
        for (int i = 0; i < input.size(); i++) {
            Map<String, Object> ie = input.get(i);
            Map<String, Object> ce = current.get(i);
            String iName = ie.get("name") == null ? null : String.valueOf(ie.get("name"));
            String iCertNo = ie.get("certNo") == null ? null : String.valueOf(ie.get("certNo"));
            String cName = ce.get("name") == null ? null : String.valueOf(ce.get("name"));
            String cCertNo = ce.get("certNo") == null ? null : String.valueOf(ce.get("certNo"));
            if (!equalsNullSafe(iName, cName)) return false;
            if (!equalsNullSafe(iCertNo, cCertNo)) return false;
        }
        return true;
    }

    /**
     * 40 §5.5 階段 1.2 gallery noop 深度防禦（issue #1053）。
     *
     * <p>比對 input gallery 與 DB 既有非 ORPHAN gallery 的 content-key list（含順序）：
     * 完全相同 → 視為 gallery 無實質變動，可走 noop 短路。
     * 任一條件不符（含新 staged_token / 數量差 / key 不同 / 順序不同）→ 視為實質變動。
     *
     * <p>此檢查獨立於前端 dirty 判定，防止前端把 preview_url 誤當 identity 送回。
     */
    private boolean isGalleryNoop(long itemId, List<ImageInput> inputs) {
        if (inputs == null) inputs = List.of();
        // 任一 input 帶 stagedToken = 新上傳，必定實質變動
        for (ImageInput img : inputs) {
            if (img.stagedToken != null && !img.stagedToken.isBlank()) return false;
        }
        List<ProductImportItemAssetEntity> existing = assetMapper.selectList(
                new QueryWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", itemId)
                        .eq("usage", ImportAssetUsage.GALLERY.value())
                        .ne("status", ImportAssetLocalStatus.ORPHAN.value())
                        .orderByAsc("sort_order"));
        if (existing.size() != inputs.size()) return false;
        for (int i = 0; i < inputs.size(); i++) {
            ImageInput inp = inputs.get(i);
            ProductImportItemAssetEntity ex = existing.get(i);
            if (inp.sourceUrl == null || inp.sourceUrl.isBlank()) return false;
            if (!"URL".equals(ex.getSourceType())) return false;
            // 同 index 視為對應 asset：input.sourceUrl 等於 existing.sourceUrl 或 existing.previewUrl 即視為同一圖
            // （後者兜底舊前端 / 未更新 client 把 preview_url 誤當 identity 送回的情境，issue #1053）
            boolean sameSourceUrl = inp.sourceUrl.equals(ex.getSourceUrl());
            boolean samePreviewUrl = ex.getPreviewUrl() != null && inp.sourceUrl.equals(ex.getPreviewUrl());
            if (!sameSourceUrl && !samePreviewUrl) return false;
        }
        return true;
    }

    /**
     * 40 號設計書 §5.5 階段 3.6：gallery asset 的 content-key。
     * STAGED → "S:" + staged_token；URL → "U:" + source_url_hash。
     * 命中 input map → 保留既有 asset，不重新 INSERT、不 emit AssetDownloadEvent。
     */
    private static String galleryContentKey(String sourceType, String stagedToken, String sourceUrlHash) {
        if ("STAGED".equals(sourceType) && stagedToken != null && !stagedToken.isBlank()) {
            return "S:" + stagedToken;
        }
        if ("URL".equals(sourceType) && sourceUrlHash != null && !sourceUrlHash.isBlank()) {
            return "U:" + sourceUrlHash;
        }
        return null;
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
        try { return Long.parseLong(v); } catch (NumberFormatException e) { return null; }
    }

    private static Integer parseIntOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return Integer.parseInt(v); } catch (NumberFormatException e) { return null; }
    }

    private static BigDecimal parseBigDecimalOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return new BigDecimal(v); } catch (NumberFormatException e) { return null; }
    }

    /**
     * 把前端送來的 shipping_methods（temperature 可能是 NAME 或 value）統一為 DB value。
     * §11.1a strict：non-blank invalid → INVALID_ARGUMENT；blank 略過該欄位（保留既有語意）。
     */
    private static List<Map<String, Object>> normalizeShippingMethods(List<Map<String, Object>> raw) {
        if (raw == null || raw.isEmpty()) return raw;
        List<Map<String, Object>> out = new ArrayList<>(raw.size());
        for (Map<String, Object> m : raw) {
            Map<String, Object> n = new LinkedHashMap<>(m);
            Object methodIn = n.get("method");
            if (methodIn != null) {
                // method 也走 strict 正規化（NAME / value 都接），與 batchUpdateShipping 一致
                n.put("method", CodeMetaNormalizer.requireValidShippingMethod(
                        String.valueOf(methodIn), "shipping_methods.method"));
            }
            Object t = n.get("temperature");
            if (t != null) {
                n.put("temperature", CodeMetaNormalizer.requireValidShippingTemperature(
                        String.valueOf(t), "shipping_methods.temperature"));
            }
            out.add(n);
        }
        return out;
    }

    /**
     * 把前端送來的 certifications（name 可能是 NAME 或 value）統一為 DB value。
     * §11.1a strict：non-blank invalid → INVALID_ARGUMENT；blank（含空 entry）跳過該 entry。
     */
    private static List<Map<String, Object>> normalizeCertifications(List<Map<String, Object>> raw) {
        if (raw == null || raw.isEmpty()) return raw;
        List<Map<String, Object>> out = new ArrayList<>(raw.size());
        for (Map<String, Object> m : raw) {
            Map<String, Object> n = new LinkedHashMap<>(m);
            Object name = n.get("name");
            if (name == null || String.valueOf(name).isBlank()) continue;
            String v = CodeMetaNormalizer.requireValidCertificationType(
                    String.valueOf(name), "certifications.name");
            n.put("name", v);
            out.add(n);
        }
        return out;
    }

    public static class UpdateInput {
        public boolean nameSet;
        public String name;
        public boolean categoryIdSet;
        public String categoryId;
        public boolean originSet;
        public String origin;
        public boolean targetStatusSet;
        public String targetStatus;
        public boolean descriptionSet;
        public String description;
        public boolean hashtagsSet;
        public List<String> hashtags = List.of();
        public boolean certificationsSet;
        public List<Map<String, Object>> certifications = List.of();
        public boolean galleryImagesSet;
        public List<ImageInput> galleryImages = List.of();
        public boolean skusSet;
        public List<SkuInput> skus = List.of();
        public boolean allowDuplicateNameSet;
        public boolean allowDuplicateName;
    }

    public static class ImageInput {
        public String sourceUrl;
        public String stagedToken;
        public ImageInput(String sourceUrl, String stagedToken) {
            this.sourceUrl = sourceUrl;
            this.stagedToken = stagedToken;
        }
    }

    public static class SkuInput {
        public Long skuId;
        public String skuName;
        public String price;
        public String salePrice;
        public String stockQty;
        public String weight;
        public String unit;
        public String lengthCm;
        public String widthCm;
        public String heightCm;
        public List<Map<String, Object>> shippingMethods = List.of();
        public boolean delete;
    }

    public static class ShippingMethodInputEntry {
        public String type;
        public String temperature;
        public String freeThreshold;
        public ShippingMethodInputEntry(String type, String temperature, String freeThreshold) {
            this.type = type;
            this.temperature = temperature;
            this.freeThreshold = freeThreshold;
        }
    }

    private record BindStagedResult(int index, String stagedToken, String importAssetId,
                                     String objectKey, String previewUrl) {}

    public record BatchCategoryResult(int affectedItems, ValidateLeafCategoryInternalResponse cat) {}

    public record BatchShippingResult(int affectedItems, int affectedSkus) {}
}
