package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.product.grpc.Certification;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalRequest;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse;
import io.github.will413028.hawkr.product.grpc.DescriptionBlock;
import io.github.will413028.hawkr.product.grpc.DraftViolation;
import io.github.will413028.hawkr.product.grpc.ProductDescription;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ProductImageImport;
import io.github.will413028.hawkr.product.grpc.ProductSkuImport;
import io.github.will413028.hawkr.product.grpc.ShippingMethod;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemErrorEntity;
import io.github.will413028.hawkr.productimport.helper.ProductDraftBuilder;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportCommitAttemptEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportCommitAttemptMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CommitItemEvent;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.ImportJobCommitFinishedEvent;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * CommitItemListener 業務邏輯（40 號設計書 §5.7）。
 *
 * <p>呼叫 Product CreateProductFromImportInternal，成功則 UPDATE item COMMITTED + asset LINKED。
 */
@Service
public class CommitItemService {

    private static final ObsLogger log = ObsLogger.forClass(CommitItemService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemSkuMapper skuMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemErrorMapper errorMapper;
    private final ProductImportCommitAttemptMapper attemptMapper;
    private final ProductImportJobMapper jobMapper;
    private final ProductGrpcClient productClient;
    private final ObsRabbitPublisher publisher;
    // 40 §2.5 Tx 載體規定:同 class self-invocation 方法用 TransactionTemplate 程式式包覆
    // (Spring AOP CGLIB proxy 不攔截 this.fooTx() 會讓 @Transactional annotation 失效)
    private final TransactionTemplate txTemplate;

    public CommitItemService(ProductImportItemMapper itemMapper,
                             ProductImportItemSkuMapper skuMapper,
                             ProductImportItemAssetMapper assetMapper,
                             ProductImportItemErrorMapper errorMapper,
                             ProductImportCommitAttemptMapper attemptMapper,
                             ProductImportJobMapper jobMapper,
                             ProductGrpcClient productClient,
                             ObsRabbitPublisher publisher,
                             PlatformTransactionManager txManager) {
        this.itemMapper = itemMapper;
        this.skuMapper = skuMapper;
        this.assetMapper = assetMapper;
        this.errorMapper = errorMapper;
        this.attemptMapper = attemptMapper;
        this.jobMapper = jobMapper;
        this.productClient = productClient;
        this.publisher = publisher;
        this.txTemplate = new TransactionTemplate(txManager);
    }

    public void commitOne(CommitItemEvent event) {
        ProductImportItemEntity item = itemMapper.selectById(event.importItemId());
        if (item == null) {
            log.warn("CommitItemService.skip.item_missing", "itemId=" + event.importItemId());
            return;
        }
        if (item.getCommitRevision() == null
                || !item.getCommitRevision().equals(event.commitRevision())) {
            log.info("CommitItemService.skip.stale_revision",
                    "itemId=" + event.importItemId());
            return;
        }
        String s = item.getStatus();
        if (!ImportItemStatus.COMMITTING.value().equals(s)
                && !ImportItemStatus.READY.value().equals(s)
                && !ImportItemStatus.COMMIT_FAILED.value().equals(s)) {
            log.info("CommitItemService.skip.status",
                    "itemId=" + event.importItemId() + ", status=" + s);
            return;
        }

        // 40 §2.5 / §5.7:程式式 tx 包覆 — markCommitting body 多筆寫入 mid-flight throw 必須 rollback
        final ProductImportItemEntity itemFinal = item;
        txTemplate.executeWithoutResult(status -> markCommittingBody(itemFinal));
        List<ProductImportItemSkuEntity> skus = skuMapper.selectList(
                new QueryWrapper<ProductImportItemSkuEntity>()
                        .eq("import_item_id", item.getId())
                        .orderByAsc("sort_order"));
        List<ProductImportItemAssetEntity> assets = assetMapper.selectList(
                new QueryWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", item.getId()));

        // 40 §4.3a / §5.7 step 4.5：commit final guard — tx 外呼叫 ValidateProductDraft(IMPORT_COMMIT)。
        // Draft 來源用 DB 當前 row（已 SELECT 完整 item + sku + asset）。
        // 若 has_blocking_error → 視為 commit 失敗（VALIDATION_DRIFT），同事務 wipe 舊 source=VALIDATE_PRODUCT_DRAFT
        // row + INSERT 本次 violations + 重算 blocking_error_count + 標 item COMMIT_FAILED + attempt FAILED。
        try {
            ProductDraft draft = ProductDraftBuilder.fromDbCurrent(item, skus, assets, event.targetStatus());
            ValidateProductDraftResponse validateResp = productClient.validateProductDraft(
                    event.storeId(), draft, ValidateProductDraftRequest.ValidationContext.IMPORT_COMMIT);
            if (validateResp != null && validateResp.getHasBlockingError()) {
                final List<DraftViolation> violations = validateResp.getViolationsList();
                txTemplate.executeWithoutResult(status -> applyValidationDriftBody(
                        item.getId(), item.getImportJobId(), item.getFirstCsvLine(),
                        event, violations));
                txTemplate.executeWithoutResult(status ->
                        maybeFinalizeJobBody(item.getImportJobId(), event.storeId()));
                return;
            }
        } catch (Exception e) {
            // RPC 異常視同 commit 失敗 — 避免「validator 掛 → 跳過 final guard → product 表落了規則漂移的資料」
            log.warn("CommitItemService.commit.validateDraftFailed",
                    "itemId=" + item.getId() + ", reason=" + e.getMessage());
            txTemplate.executeWithoutResult(status -> applyFailureBody(item.getId(), event,
                    "validate_product_draft RPC failed: " + e.getMessage(), 13));
            txTemplate.executeWithoutResult(status ->
                    maybeFinalizeJobBody(item.getImportJobId(), event.storeId()));
            return;
        }

        CreateProductFromImportInternalRequest req =
                buildRequest(event, item, skus, assets);

        try {
            CreateProductFromImportInternalResponse resp = productClient.createProductFromImport(req);
            // Product service 將 business error 包進 onNext+code（ProductGrpcService 跨 endpoint 一致契約），
            // 不會拋 StatusRuntimeException — 必須檢查 resp.code 才能正確反映成功/失敗，
            // 否則 import 端會把 INVALID_PARAM / CATEGORY_NOT_FOUND 等失敗誤標為 SUCCEEDED。
            int code = resp.getCode();
            String productId = resp.getProductId();
            if (code != GrpcResponseCode.SUCCESS.code() || productId == null || productId.isBlank()) {
                log.warn("CommitItemService.commit.businessFailed",
                        "itemId=" + item.getId() + ", code=" + code
                                + ", productId=" + productId + ", msg=" + resp.getMessage());
                final String failureMsg = resp.getMessage() == null || resp.getMessage().isBlank()
                        ? ("product service returned code=" + code + " without productId")
                        : resp.getMessage();
                txTemplate.executeWithoutResult(status ->
                        applyFailureBody(item.getId(), event, failureMsg, code));
            } else {
                log.info("CommitItemService.commit.success",
                        "itemId=" + item.getId() + ", productId=" + productId
                                + ", idemStatus=" + resp.getIdempotencyStatus());
                final String pid = productId;
                txTemplate.executeWithoutResult(status ->
                        applySuccessBody(item.getId(), pid, event));
            }
        } catch (StatusRuntimeException sre) {
            log.warn("CommitItemService.commit.grpcFailed",
                    "itemId=" + item.getId() + ", status=" + sre.getStatus());
            txTemplate.executeWithoutResult(status -> applyFailureBody(item.getId(), event,
                    sre.getStatus().toString(), sre.getStatus().getCode().value()));
        } catch (Exception e) {
            log.error("CommitItemService.commit.unexpected",
                    "itemId=" + item.getId() + ", error=" + e.getMessage(), e);
            txTemplate.executeWithoutResult(status ->
                    applyFailureBody(item.getId(), event, e.getMessage(), 13));
        }
        txTemplate.executeWithoutResult(status ->
                maybeFinalizeJobBody(item.getImportJobId(), event.storeId()));
    }

    // 40 §2.5 / §5.7:tx body — 必須由 caller 包進 txTemplate。
    // 禁止標 @Transactional(self-invocation 會繞過 Spring AOP proxy)。
    private void markCommittingBody(ProductImportItemEntity item) {
        item.setStatus(ImportItemStatus.COMMITTING.value());
        itemMapper.updateById(item);
        // Round 5 Obs #1：commit replay 同 idempotency_key 不可再 INSERT —
        // attempt 表有 uk_pica_idempotency UNIQUE，replay 時 INSERT 會撞 SQLException 中斷整個 flow，
        // 導致 item.status 卡 COMMITTING（9206）沒推進 COMMITTED（9207）。
        // 改為先查存在；存在則 reuse 不 INSERT（attempt 既有 result=IN_PROGRESS / SUCCEEDED / FAILED
        // 任一狀態都不需重新初始化）。
        String idempotencyKey = "import:" + item.getId() + ":rev:" + item.getCommitRevision();
        ProductImportCommitAttemptEntity existing = attemptMapper.selectOne(
                new QueryWrapper<ProductImportCommitAttemptEntity>()
                        .eq("idempotency_key", idempotencyKey));
        if (existing != null) {
            log.info("CommitItemService.markCommitting.replay",
                    "itemId=" + item.getId() + ", idempotencyKey=" + idempotencyKey
                            + ", existingResult=" + existing.getResult());
            return;
        }
        ProductImportCommitAttemptEntity attempt = new ProductImportCommitAttemptEntity();
        attempt.setImportItemId(item.getId());
        attempt.setImportJobId(item.getImportJobId());
        attempt.setRevision(item.getCommitRevision());
        attempt.setIdempotencyKey(idempotencyKey);
        attempt.setStartedAt(DateTimeUtils.nowUtc());
        attempt.setResult("IN_PROGRESS");
        attemptMapper.insert(attempt);
    }

    // 40 §2.5 / §5.7:tx body — 必須由 caller 包進 txTemplate。
    private void applySuccessBody(Long itemId, String productId, CommitItemEvent event) {
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null) return;
        item.setStatus(ImportItemStatus.COMMITTED.value());
        item.setFormalProductId(parseLongOrNull(productId));
        item.setCommittedAt(DateTimeUtils.nowUtc());
        item.setCommitFailedReason(null);
        itemMapper.updateById(item);

        assetMapper.update(null,
                new UpdateWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", itemId)
                        .eq("status", ImportAssetLocalStatus.UPLOADED.value())
                        .set("status", ImportAssetLocalStatus.LINKED.value()));

        attemptMapper.update(null,
                new UpdateWrapper<ProductImportCommitAttemptEntity>()
                        .eq("import_item_id", itemId)
                        .eq("idempotency_key", "import:" + itemId + ":rev:" + event.commitRevision())
                        .set("result", "SUCCEEDED")
                        .set("formal_product_id", parseLongOrNull(productId))
                        .set("finished_at", DateTimeUtils.nowUtc()));
    }

    /**
     * 40 §4.3a / §5.7 step 4.5：commit final guard 失敗（VALIDATION_DRIFT）的同事務處理。
     * <ol>
     *   <li>DELETE 既有 source=VALIDATE_PRODUCT_DRAFT 的 row（避免新舊 violations 重疊計入 blocking）</li>
     *   <li>INSERT 本次 validator 回傳的所有 violations + 額外一筆 error_type=VALIDATION_DRIFT 摘要</li>
     *   <li>UPDATE item.blocking_error_count = (SELECT COUNT WHERE severity='ERROR')</li>
     *   <li>UPDATE item.status=COMMIT_FAILED, commit_failed_reason='VALIDATION_DRIFT: {first.message}'</li>
     *   <li>UPDATE commit_attempt.result=FAILED, grpc_code=FAILED_PRECONDITION, failure_reason=VALIDATION_DRIFT</li>
     * </ol>
     */
    // 40 §2.5 / §5.7:tx body — 必須由 caller 包進 txTemplate。
    private void applyValidationDriftBody(Long itemId, Long jobId, Integer csvLine,
                                          CommitItemEvent event, List<DraftViolation> violations) {
        // (a) wipe 舊 VALIDATE_PRODUCT_DRAFT row
        errorMapper.delete(new QueryWrapper<ProductImportItemErrorEntity>()
                .eq("import_item_id", itemId)
                .eq("source", ProductImportItemErrorEntity.SOURCE_VALIDATE_PRODUCT_DRAFT));
        // (b) INSERT 本次 violations
        String firstErrMsg = "";
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
            if (firstErrMsg.isEmpty() && "ERROR".equals(e.getSeverity())) {
                firstErrMsg = e.getMessage() == null ? "" : e.getMessage();
            }
        }
        // (b2) 額外寫一筆 VALIDATION_DRIFT 摘要（便於前端統一顯示「規則漂移」事件）
        ProductImportItemErrorEntity drift = new ProductImportItemErrorEntity();
        drift.setImportItemId(itemId);
        drift.setImportJobId(jobId);
        drift.setCsvLine(csvLine == null ? 0 : csvLine);
        drift.setErrorType(ImportItemErrorType.VALIDATION_DRIFT.value());
        drift.setSeverity("ERROR");
        drift.setField("");
        drift.setMessage("上架規則已變更，請重新檢視商品內容後再 commit");
        drift.setSource(ProductImportItemErrorEntity.SOURCE_VALIDATE_PRODUCT_DRAFT);
        errorMapper.insert(drift);

        // (c) 重算 blocking_error_count
        itemMapper.recalcBlockingErrorCount(itemId);

        // (d) item 標 COMMIT_FAILED
        ProductImportItemEntity it = itemMapper.selectById(itemId);
        if (it != null) {
            it.setStatus(ImportItemStatus.COMMIT_FAILED.value());
            String reason = "VALIDATION_DRIFT" + (firstErrMsg.isBlank() ? "" : ": " + firstErrMsg);
            it.setCommitFailedReason(reason.substring(0, Math.min(500, reason.length())));
            itemMapper.updateById(it);
        }

        // (e) attempt 標 FAILED
        attemptMapper.update(null,
                new UpdateWrapper<ProductImportCommitAttemptEntity>()
                        .eq("import_item_id", itemId)
                        .eq("idempotency_key", "import:" + itemId + ":rev:" + event.commitRevision())
                        .set("result", "FAILED")
                        .set("failure_reason", "VALIDATION_DRIFT" + (firstErrMsg.isBlank() ? "" : ": " + firstErrMsg))
                        .set("grpc_code", io.grpc.Status.Code.FAILED_PRECONDITION.value())
                        .set("finished_at", DateTimeUtils.nowUtc()));
    }

    // 40 §2.5 / §5.7:tx body — 必須由 caller 包進 txTemplate。
    private void applyFailureBody(Long itemId, CommitItemEvent event, String reason, int grpcCode) {
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null) return;
        item.setStatus(ImportItemStatus.COMMIT_FAILED.value());
        item.setCommitFailedReason(reason == null ? "unknown" : reason.substring(0, Math.min(500, reason.length())));
        itemMapper.updateById(item);
        attemptMapper.update(null,
                new UpdateWrapper<ProductImportCommitAttemptEntity>()
                        .eq("import_item_id", itemId)
                        .eq("idempotency_key", "import:" + itemId + ":rev:" + event.commitRevision())
                        .set("result", "FAILED")
                        .set("failure_reason", item.getCommitFailedReason())
                        .set("grpc_code", grpcCode)
                        .set("finished_at", DateTimeUtils.nowUtc()));
    }

    // 40 §2.5 / §5.7:tx body — 必須由 caller 包進 txTemplate。
    // MQ publish 保留在 tx 內(原意):publish 失敗 → IllegalStateException 觸發 rollback,
    // 避免「job 標 COMMITTED / event_id 已設但 event 沒發」的不一致。
    private void maybeFinalizeJobBody(Long jobId, Long storeId) {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null) return;
        if (!ImportJobStatus.COMMITTING.value().equals(job.getStatus())) return;

        Long committingCount = itemMapper.selectCount(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("status", ImportItemStatus.COMMITTING.value()));
        if (committingCount != null && committingCount > 0) return;

        Long committedCount = itemMapper.selectCount(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("status", ImportItemStatus.COMMITTED.value()));
        Long readyCount = itemMapper.selectCount(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("status", ImportItemStatus.READY.value()));
        Long failedCount = itemMapper.selectCount(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("import_job_id", jobId)
                        .eq("status", ImportItemStatus.COMMIT_FAILED.value()));

        String finalStatus = (hasCount(readyCount) || hasCount(failedCount))
                ? ImportJobStatus.READY.value()
                : ImportJobStatus.COMMITTED.value();
        job.setStatus(finalStatus);
        if (ImportJobStatus.COMMITTED.value().equals(finalStatus)) {
            job.setCommittedAt(DateTimeUtils.nowUtc());
        }
        jobMapper.updateById(job);

        String eventId = UUID.randomUUID().toString();
        int updated = jobMapper.update(null,
                new UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .set("commit_finished_event_published_at", DateTimeUtils.nowUtc())
                        .set("commit_finished_event_id", eventId));
        if (updated == 0) return;

        ImportJobCommitFinishedEvent ev = new ImportJobCommitFinishedEvent(
                eventId,
                jobId,
                storeId,
                finalStatus,
                committedCount == null ? 0 : committedCount.intValue(),
                failedCount == null ? 0 : failedCount.intValue(),
                0,
                DateTimeUtils.nowUtc().toString()
        );
        try {
            publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                    MqConstants.RK_PRODUCT_IMPORT_JOB_COMMIT_FINISHED,
                    MAPPER.writeValueAsBytes(ev),
                    ProductImportEventType.PRODUCT_IMPORT_JOB_COMMIT_FINISHED.value());
        } catch (Exception e) {
            throw new IllegalStateException("publish job.commit_finished failed", e);
        }
    }

    private CreateProductFromImportInternalRequest buildRequest(CommitItemEvent event,
                                                                ProductImportItemEntity item,
                                                                List<ProductImportItemSkuEntity> skus,
                                                                List<ProductImportItemAssetEntity> assets) {
        CreateProductFromImportInternalRequest.Builder b = CreateProductFromImportInternalRequest.newBuilder()
                .setIdempotencyKey(event.idempotencyKey())
                .setStoreId(event.storeId())
                .setImportJobId(String.valueOf(item.getImportJobId()))
                .setImportItemId(String.valueOf(item.getId()))
                .setName(item.getName() == null ? "" : item.getName())
                .setCategoryId(item.getCategoryId() == null ? "" : String.valueOf(item.getCategoryId()))
                .setOrigin(item.getOrigin() == null ? "" : item.getOrigin())
                .setTargetStatus(event.targetStatus() == null ? "" : event.targetStatus());

        ProductDescription.Builder desc = ProductDescription.newBuilder().setVersion("1");
        if (item.getDescriptionJson() != null) {
            JsonNode root = item.getDescriptionJson();
            JsonNode blocks = root.path("blocks");
            if (blocks.isArray()) {
                for (JsonNode bn : blocks) {
                    DescriptionBlock.Builder blk = DescriptionBlock.newBuilder()
                            .setType(bn.path("type").asText(""))
                            .setId(bn.path("id").asText(""))
                            .setContent(bn.path("content").asText(""))
                            .setStyleJson(bn.path("styleJson").asText(""));
                    // inline-run 模型欄位（runs / tableJson / listJson）— 否則 commit 後描述丟失樣式 / table / list（#1255/#1256/#1258）
                    io.github.will413028.hawkr.productimport.helper.ProductDraftBuilder.applyInlineRunFields(bn, blk);
                    if ("image".equals(bn.path("type").asText())) {
                        Integer blkIdx = parseIntegerSafe(bn.path("id").asText(""));
                        if (blkIdx != null) {
                            assets.stream()
                                    .filter(a -> "DESCRIPTION".equals(a.getUsage())
                                            && a.getBlockIndex() != null
                                            && a.getBlockIndex().equals(blkIdx))
                                    .findFirst()
                                    .ifPresent(a -> {
                                        if (a.getMediaImportAssetId() != null) {
                                            blk.setImportAssetId(a.getMediaImportAssetId());
                                        }
                                    });
                        }
                    }
                    desc.addBlocks(blk.build());
                }
            }
        }
        b.setDescription(desc.build());

        assets.stream()
                .filter(a -> "GALLERY".equals(a.getUsage())
                        && a.getMediaImportAssetId() != null)
                .sorted(Comparator.comparing(a -> a.getSortOrder() == null ? 0 : a.getSortOrder()))
                .forEach(a -> b.addImages(ProductImageImport.newBuilder()
                        .setImportAssetId(a.getMediaImportAssetId())
                        .setSortOrder(a.getSortOrder() == null ? 0 : a.getSortOrder())
                        .build()));

        if (item.getCertifications() != null) {
            for (Map<String, Object> c : item.getCertifications()) {
                b.addCertifications(Certification.newBuilder()
                        .setName(String.valueOf(c.getOrDefault("name", "")))
                        .setCertNo(String.valueOf(c.getOrDefault("certNo", "")))
                        .build());
            }
        }
        if (item.getHashtags() != null) {
            for (String h : item.getHashtags()) b.addHashtags(h);
        }

        for (ProductImportItemSkuEntity sku : skus) {
            ProductSkuImport.Builder s = ProductSkuImport.newBuilder()
                    .setSkuName(s2(sku.getSkuName()))
                    .setPrice(sku.getPrice() == null ? "" : String.valueOf(sku.getPrice()))
                    .setSalePrice(sku.getSalePrice() == null ? "" : String.valueOf(sku.getSalePrice()))
                    .setStockQty(sku.getStockQty() == null ? "" : String.valueOf(sku.getStockQty()))
                    .setWeight(sku.getWeight() == null ? "" : sku.getWeight().toPlainString())
                    .setUnit(s2(sku.getUnit()))
                    .setLengthCm(sku.getLengthCm() == null ? "" : sku.getLengthCm().toPlainString())
                    .setWidthCm(sku.getWidthCm() == null ? "" : sku.getWidthCm().toPlainString())
                    .setHeightCm(sku.getHeightCm() == null ? "" : sku.getHeightCm().toPlainString());
            if (sku.getShippingMethods() != null) {
                for (Map<String, Object> m : sku.getShippingMethods()) {
                    s.addShippingMethods(ShippingMethod.newBuilder()
                            .setType(String.valueOf(m.getOrDefault("method", "")))
                            .setTemperature(String.valueOf(m.getOrDefault("temperature", "")))
                            .setFreeThreshold(String.valueOf(m.getOrDefault("freeThreshold", "")))
                            .setFeePayer(String.valueOf(m.getOrDefault("feePayer", "")))
                            .build());
                }
            }
            b.addSkus(s.build());
        }
        return b.build();
    }

    private static Integer parseIntegerSafe(String s) {
        if (s == null) return null;
        if (s.startsWith("blk-")) s = s.substring(4);
        try { return Integer.parseInt(s); } catch (NumberFormatException e) { return null; }
    }

    private static String s2(String v) { return v == null ? "" : v; }

    private static boolean hasCount(Long count) {
        return count != null && count > 0;
    }

    private static Long parseLongOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return Long.parseLong(v); } catch (NumberFormatException e) { return null; }
    }
}
