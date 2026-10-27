package io.github.will413028.hawkr.productimport.grpc;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemErrorEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.entity.projection.ImportItemBriefProjection;
import io.github.will413028.hawkr.productimport.helper.ImportErrorMessageCatalog;
import io.github.will413028.hawkr.productimport.helper.ImportJobProgressCalculator;
import io.github.will413028.hawkr.productimport.service.CategoryTokenService;
import io.github.will413028.hawkr.productimport.service.CommitJobService;
import io.github.will413028.hawkr.productimport.service.ImportJobService;
import io.github.will413028.hawkr.productimport.service.ItemMutationService;
import io.github.will413028.hawkr.productimport.service.ItemQueryService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.protobuf.ByteString;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Product Import gRPC service（40 號設計書 §4.2 — 18 RPC 全實作）。
 */
@GrpcService
public class ProductImportServiceImpl extends ProductImportServiceGrpc.ProductImportServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(ProductImportServiceImpl.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ImportJobService jobService;
    private final CommitJobService commitJobService;
    private final ItemQueryService itemQueryService;
    private final ItemMutationService itemMutationService;
    private final CategoryTokenService categoryTokenService;
    private final ImportJobProgressCalculator progressCalculator;

    public ProductImportServiceImpl(ImportJobService jobService,
                                    CommitJobService commitJobService,
                                    ItemQueryService itemQueryService,
                                    ItemMutationService itemMutationService,
                                    CategoryTokenService categoryTokenService,
                                    ImportJobProgressCalculator progressCalculator) {
        this.jobService = jobService;
        this.commitJobService = commitJobService;
        this.itemQueryService = itemQueryService;
        this.itemMutationService = itemMutationService;
        this.categoryTokenService = categoryTokenService;
        this.progressCalculator = progressCalculator;
    }

    // ========================================================================
    // Job 級
    // ========================================================================

    @Override
    public void createImportJob(CreateImportJobRequest request,
                                 StreamObserver<CreateImportJobResponse> responseObserver) {
        try {
            ImportJobService.CreateJobResult r = jobService.createJob(
                    request.getStoreId(),
                    request.getFilename(),
                    request.getContent().toByteArray(),
                    request.getTargetStatus());
            CreateImportJobResponse.Builder b = CreateImportJobResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setImportJobId(String.valueOf(r.importJobId()))
                    .setStatus(r.status())
                    .setDedupHit(r.dedupHit());
            appendValidationErrors(r.validationErrors(), b::addValidationErrors);
            responseObserver.onNext(b.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            log.error("createImportJob.error", "createImportJob error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void reuploadImportJobCsv(ReuploadImportJobCsvRequest request,
                                      StreamObserver<ReuploadImportJobCsvResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            ImportJobService.ReuploadResult r = jobService.reuploadCsv(
                    request.getStoreId(), jobId,
                    request.getFilename(), request.getContent().toByteArray());
            ReuploadImportJobCsvResponse.Builder b = ReuploadImportJobCsvResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setImportJobId(String.valueOf(r.importJobId()))
                    .setStatus(r.status());
            appendValidationErrors(r.validationErrors(), b::addValidationErrors);
            responseObserver.onNext(b.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            log.error("reuploadImportJobCsv.error", "reupload error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void listImportJobs(ListImportJobsRequest request,
                                StreamObserver<ListImportJobsResponse> responseObserver) {
        try {
            int page = parsePositiveInt(request.getPage(), 1);
            int pageSize = parsePositiveInt(request.getPageSize(), 20);
            ImportJobService.PageResult<ProductImportJobEntity> result =
                    jobService.listJobs(request.getStoreId(),
                            request.getStatusList(), page, pageSize);
            ListImportJobsResponse.Builder b = ListImportJobsResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setTotal(String.valueOf(result.total()))
                    .setPage(String.valueOf(page))
                    .setPageSize(String.valueOf(pageSize));
            for (ProductImportJobEntity job : result.records()) {
                ImportJobProgressCalculator.Counters c = progressCalculator.counters(job.getId());
                b.addRecords(ImportJobBrief.newBuilder()
                        .setImportJobId(String.valueOf(job.getId()))
                        .setStatus(orEmpty(job.getStatus()))
                        .setSourceFilename(orEmpty(job.getSourceFilename()))
                        .setTotalItems(job.getTotalItems() == null ? "0" : String.valueOf(job.getTotalItems()))
                        .setCommittedCount(String.valueOf(c.itemsCommitted()))
                        .setFailedCount(String.valueOf(c.itemsCommitFailed()))
                        .setPendingCount(String.valueOf(c.itemsReady() + c.itemsParsed()))
                        .setCreatedAt(DateTimeUtils.formatIso8601(job.getCreatedAt()))
                        .setExpiresAt(DateTimeUtils.formatIso8601(job.getExpiresAt()))
                        .build());
            }
            responseObserver.onNext(b.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            log.error("listImportJobs.error", "listImportJobs error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void getImportJob(GetImportJobRequest request,
                              StreamObserver<GetImportJobResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            ProductImportJobEntity job = jobService.getJob(request.getStoreId(), jobId);
            if (job == null) {
                responseObserver.onError(Status.NOT_FOUND
                        .withDescription("import job not found: " + request.getImportJobId())
                        .asRuntimeException());
                return;
            }
            ImportJobProgressCalculator.Counters c = progressCalculator.counters(jobId);
            ImportJobProgressCalculator.Progress p = progressCalculator.progress(c);
            ImportJobDetail.Builder data = ImportJobDetail.newBuilder()
                    .setImportJobId(String.valueOf(job.getId()))
                    .setStatus(job.getStatus())
                    .setSourceFilename(orEmpty(job.getSourceFilename()))
                    .setTotalRows(job.getTotalRows() == null ? "0" : String.valueOf(job.getTotalRows()))
                    .setTotalItems(job.getTotalItems() == null ? "0" : String.valueOf(job.getTotalItems()))
                    .setTargetStatus(orEmpty(job.getTargetStatus()))
                    .setParseError(orEmpty(job.getParseError()))
                    .setCreatedAt(DateTimeUtils.formatIso8601(job.getCreatedAt()))
                    .setUpdatedAt(DateTimeUtils.formatIso8601(job.getUpdatedAt()))
                    .setExpiresAt(DateTimeUtils.formatIso8601(job.getExpiresAt()))
                    .setCommittedAt(job.getCommittedAt() == null ? "" : DateTimeUtils.formatIso8601(job.getCommittedAt()))
                    .setCancelledAt(job.getCancelledAt() == null ? "" : DateTimeUtils.formatIso8601(job.getCancelledAt()))
                    .setCounters(toProtoCounters(c))
                    .setProgress(toProtoProgress(p));
            appendValidationErrors(job.getValidationErrors(), data::addValidationErrors);
            responseObserver.onNext(GetImportJobResponse.newBuilder()
                    .setCode(0).setMessage("success").setData(data.build()).build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            log.error("getImportJob.error", "getImportJob error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void getImportJobPreviewListing(GetImportJobPreviewListingRequest request,
                                            StreamObserver<GetImportJobPreviewListingResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            int page = parsePositiveInt(request.getPage(), 1);
            int pageSize = parsePositiveInt(request.getPageSize(), 20);
            ItemQueryService.PreviewListingResult r = itemQueryService.previewListing(
                    request.getStoreId(), jobId, request.getIncludePending(), page, pageSize);
            GetImportJobPreviewListingData.Builder data = GetImportJobPreviewListingData.newBuilder()
                    .setImportJobId(request.getImportJobId())
                    .setTotalEligible(String.valueOf(r.eligible()))
                    .setTotalNeedsFix(String.valueOf(r.needsFix()))
                    .setTotalPending(String.valueOf(r.pending()))
                    .setTotalRemoved(String.valueOf(r.removed()))
                    .setTotal(String.valueOf(r.total()))
                    .setPage(String.valueOf(page))
                    .setPageSize(String.valueOf(pageSize));
            for (ProductImportItemEntity it : r.items()) {
                ItemQueryService.DetailView dv = itemQueryService.getItemDetail(request.getStoreId(), jobId, it.getId());
                ProductListPresentation.Builder pl = ProductListPresentation.newBuilder()
                        .setId("")
                        .setName(orEmpty(it.getName()))
                        .setRating("0")
                        .setCategoryId(it.getCategoryId() == null ? "" : String.valueOf(it.getCategoryId()))
                        .setCategoryName(orEmpty(it.getCatL3Name()))
                        .setOrigin(orEmpty(it.getOrigin()))
                        .setPriceMin(String.valueOf(dv.priceMin()))
                        .setPriceMax(String.valueOf(dv.priceMax()))
                        .setReviewCount("0")
                        .setSkuCount(it.getSkuCount() == null ? "0" : String.valueOf(it.getSkuCount()))
                        .setImageUrl(orEmpty(dv.firstImageUrl()));
                buildGallery(dv.assets()).forEach(pl::addGallery);
                data.addRecords(ImportItemPreviewListingRecord.newBuilder()
                        .setImportItemId(String.valueOf(it.getId()))
                        .setImportStatus(orEmpty(it.getStatus()))
                        .setPreviewData(pl.build())
                        .build());
            }
            responseObserver.onNext(GetImportJobPreviewListingResponse.newBuilder()
                    .setCode(0).setMessage("success").setData(data.build()).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            log.error("getImportJobPreviewListing.error", "preview listing error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void cancelImportJob(CancelImportJobRequest request,
                                 StreamObserver<CancelImportJobResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            boolean ok = jobService.cancelJob(request.getStoreId(), jobId);
            if (!ok) {
                responseObserver.onError(Status.NOT_FOUND
                        .withDescription("import job not found").asRuntimeException());
                return;
            }
            responseObserver.onNext(CancelImportJobResponse.newBuilder()
                    .setCode(0).setMessage("success").setStatus("CANCELLED").build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void commitImportJob(CommitImportJobRequest request,
                                 StreamObserver<CommitImportJobResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            List<Long> includeIds = new ArrayList<>();
            for (String s : request.getIncludeItemIdsList()) includeIds.add(parseLong(s));
            CommitJobService.CommitResult r = commitJobService.commitJob(request.getStoreId(),
                    jobId, request.getRetryFailedOnly(), includeIds);
            // 40 §4.2.5 partial-success：把 skipped item id + reason 帶回 caller
            CommitImportJobResponse.Builder rb = CommitImportJobResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setStatus(r.status())
                    .setQueuedCount(String.valueOf(r.queuedCount()))
                    .setCommitOutcome(toCommitOutcome(r.outcome()));
            for (CommitJobService.SkipEntry s : r.skipped()) {
                rb.addSkippedItemIds(String.valueOf(s.importItemId()));
                rb.addSkipReasons(CommitSkipReason.newBuilder()
                        .setImportItemId(String.valueOf(s.importItemId()))
                        .setReasonCode(s.reasonCode())
                        .setReasonMessage(s.reasonMessage())
                        .build());
            }
            responseObserver.onNext(rb.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    private CommitOutcome toCommitOutcome(CommitJobService.CommitOutcome outcome) {
        return switch (outcome) {
            case QUEUED -> CommitOutcome.QUEUED;
            case PARTIAL_SUCCESS -> CommitOutcome.PARTIAL_SUCCESS;
            case ALL_SKIPPED -> CommitOutcome.ALL_SKIPPED;
        };
    }

    // ========================================================================
    // Item 級
    // ========================================================================

    @Override
    public void listImportItems(ListImportItemsRequest request,
                                 StreamObserver<ListImportItemsResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            int page = parsePositiveInt(request.getPage(), 1);
            int pageSize = parsePositiveInt(request.getPageSize(), 20);
            ItemQueryService.ListResult r = itemQueryService.listItems(request.getStoreId(), jobId,
                    request.getStatusList(), request.getOnlyWithErrors(),
                    request.getIncludeRemoved(), page, pageSize);
            ListImportItemsResponse.Builder b = ListImportItemsResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setTotal(String.valueOf(r.total()))
                    .setPage(String.valueOf(page))
                    .setPageSize(String.valueOf(pageSize));
            for (ItemQueryService.BriefView v : r.records()) {
                b.addRecords(toProtoBrief(v));
            }
            responseObserver.onNext(b.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void getImportItem(GetImportItemRequest request,
                               StreamObserver<GetImportItemResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            Long itemId = parseLong(request.getImportItemId());
            ItemQueryService.DetailView dv = itemQueryService.getItemDetail(request.getStoreId(), jobId, itemId);
            responseObserver.onNext(GetImportItemResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setData(toProtoDetail(dv))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void getImportItemPreview(GetImportItemPreviewRequest request,
                                      StreamObserver<GetImportItemPreviewResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            Long itemId = parseLong(request.getImportItemId());
            ItemQueryService.DetailView dv = itemQueryService.getItemPreview(request.getStoreId(), jobId, itemId);
            ProductImportItemEntity it = dv.item();
            ProductDetailPresentation.Builder pd = ProductDetailPresentation.newBuilder()
                    .setId("")
                    .setName(orEmpty(it.getName()))
                    .setRating("0")
                    .setStatus("")
                    .setCategoryId(it.getCategoryId() == null ? "" : String.valueOf(it.getCategoryId()))
                    .setOrigin(orEmpty(it.getOrigin()))
                    .setDescription(it.getDescriptionJson() == null ? "" : it.getDescriptionJson().toString())
                    .setPriceMin(String.valueOf(dv.priceMin()))
                    .setPriceMax(String.valueOf(dv.priceMax()))
                    .setCatL1Id(it.getCatL1Id() == null ? "" : String.valueOf(it.getCatL1Id()))
                    .setCatL1Name(orEmpty(it.getCatL1Name()))
                    .setCatL2Id(it.getCatL2Id() == null ? "" : String.valueOf(it.getCatL2Id()))
                    .setCatL2Name(orEmpty(it.getCatL2Name()))
                    .setCatL3Id(it.getCategoryId() == null ? "" : String.valueOf(it.getCategoryId()))
                    .setCatL3Name(orEmpty(it.getCatL3Name()));
            buildGallery(dv.assets()).forEach(pd::addGallery);
            for (Map<String, Object> c : ifNull(it.getCertifications())) {
                pd.addCertifications(CertificationDto.newBuilder()
                        .setName(String.valueOf(c.getOrDefault("name", "")))
                        .setCertNo(String.valueOf(c.getOrDefault("certNo", "")))
                        .build());
            }
            for (ProductImportItemSkuEntity sku : dv.skus()) {
                pd.addSkus(toProtoSku(sku));
            }
            ImportItemPreview.Builder p = ImportItemPreview.newBuilder()
                    .setImportJobId(request.getImportJobId())
                    .setImportItemId(request.getImportItemId())
                    .setImportStatus(orEmpty(it.getStatus()))
                    .setFormalProductId(it.getFormalProductId() == null ? "" : String.valueOf(it.getFormalProductId()))
                    .setPreviewData(pd.build());
            for (ProductImportItemErrorEntity e : dv.errors()) {
                ImportItemErrorDto.Builder eb = toProtoError(e);
                if ("ERROR".equals(e.getSeverity())) p.addValidationErrors(eb.build());
                else p.addWarnings(eb.build());
            }
            responseObserver.onNext(GetImportItemPreviewResponse.newBuilder()
                    .setCode(0).setMessage("success").setData(p.build()).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void updateImportItem(UpdateImportItemRequest request,
                                  StreamObserver<UpdateImportItemResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            Long itemId = parseLong(request.getImportItemId());
            Integer expectedVersion = request.getExpectedVersion().isBlank()
                    ? null : Integer.parseInt(request.getExpectedVersion());

            ItemMutationService.UpdateInput input = new ItemMutationService.UpdateInput();
            input.nameSet = request.getNameSet();
            input.name = request.getName();
            input.categoryIdSet = request.getCategoryIdSet();
            input.categoryId = request.getCategoryId();
            input.originSet = request.getOriginSet();
            input.origin = request.getOrigin();
            input.targetStatusSet = request.getTargetStatusSet();
            input.targetStatus = request.getTargetStatus();
            input.descriptionSet = request.getDescriptionSet();
            input.description = request.getDescription();
            input.hashtagsSet = request.getHashtagsSet();
            input.hashtags = new ArrayList<>(request.getHashtagsList());
            input.certificationsSet = request.getCertificationsSet();
            input.certifications = new ArrayList<>();
            for (CertificationDto c : request.getCertificationsList()) {
                Map<String, Object> m = new LinkedHashMap<>();
                m.put("name", c.getName());
                m.put("certNo", c.getCertNo());
                input.certifications.add(m);
            }
            input.galleryImagesSet = request.getGalleryImagesSet();
            input.galleryImages = new ArrayList<>();
            for (ImportItemImageInput img : request.getGalleryImagesList()) {
                input.galleryImages.add(new ItemMutationService.ImageInput(
                        img.getSourceUrl(), img.getStagedToken()));
            }
            input.skusSet = request.getSkusSet();
            input.skus = new ArrayList<>();
            for (UpdateSkuInput s : request.getSkusList()) {
                ItemMutationService.SkuInput si = new ItemMutationService.SkuInput();
                si.skuId = s.getSkuId().isBlank() ? null : Long.parseLong(s.getSkuId());
                si.skuName = s.getSkuName();
                si.price = s.getPrice();
                si.salePrice = s.getSalePrice();
                si.stockQty = s.getStockQty();
                si.weight = s.getWeight();
                si.unit = s.getUnit();
                si.lengthCm = s.getLengthCm();
                si.widthCm = s.getWidthCm();
                si.heightCm = s.getHeightCm();
                si.delete = s.getDelete();
                si.shippingMethods = new ArrayList<>();
                for (ShippingMethodDto sm : s.getShippingMethodsList()) {
                    Map<String, Object> m = new LinkedHashMap<>();
                    m.put("method", sm.getMethod());
                    m.put("temperature", sm.getTemperature());
                    si.shippingMethods.add(m);
                }
                input.skus.add(si);
            }
            input.allowDuplicateNameSet = request.getAllowDuplicateNameSet();
            input.allowDuplicateName = request.getAllowDuplicateName();

            itemMutationService.update(request.getStoreId(), jobId, itemId, expectedVersion, input);
            ItemQueryService.DetailView dv = itemQueryService.getItemDetail(request.getStoreId(), jobId, itemId);
            responseObserver.onNext(UpdateImportItemResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setData(toProtoDetail(dv)).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            log.error("updateImportItem.error", "update error", e);
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void removeImportItems(RemoveImportItemsRequest request,
                                   StreamObserver<RemoveImportItemsResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            List<Long> ids = new ArrayList<>();
            for (String s : request.getImportItemIdsList()) ids.add(parseLong(s));
            int n = itemMutationService.removeItems(request.getStoreId(), jobId, ids);
            responseObserver.onNext(RemoveImportItemsResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setAffectedCount(String.valueOf(n)).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void restoreImportItems(RestoreImportItemsRequest request,
                                    StreamObserver<RestoreImportItemsResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            List<Long> ids = new ArrayList<>();
            for (String s : request.getImportItemIdsList()) ids.add(parseLong(s));
            int n = itemMutationService.restoreItems(request.getStoreId(), jobId, ids);
            responseObserver.onNext(RestoreImportItemsResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setAffectedCount(String.valueOf(n)).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void retryImportItemAssets(RetryImportItemAssetsRequest request,
                                       StreamObserver<RetryImportItemAssetsResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            Long itemId = parseLong(request.getImportItemId());
            List<Long> ids = new ArrayList<>();
            for (String s : request.getAssetIdsList()) ids.add(parseLong(s));
            int n = itemMutationService.retryAssets(request.getStoreId(), jobId, itemId, ids);
            responseObserver.onNext(RetryImportItemAssetsResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setQueuedCount(String.valueOf(n)).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    // ========================================================================
    // 批次
    // ========================================================================

    @Override
    public void batchUpdateItemsShipping(BatchUpdateItemsShippingRequest request,
                                          StreamObserver<BatchUpdateItemsShippingResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            List<Long> ids = new ArrayList<>();
            for (String s : request.getImportItemIdsList()) ids.add(parseLong(s));
            List<ItemMutationService.ShippingMethodInputEntry> methods = new ArrayList<>();
            for (ShippingMethodInput m : request.getShippingMethodsList()) {
                methods.add(new ItemMutationService.ShippingMethodInputEntry(
                        m.getType(), m.getTemperature(), m.getFreeThreshold()));
            }
            ItemMutationService.BatchShippingResult r = itemMutationService.batchUpdateShipping(
                    request.getStoreId(), jobId, ids, request.getApplyAll(), methods);
            responseObserver.onNext(BatchUpdateItemsShippingResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setAffectedItems(String.valueOf(r.affectedItems()))
                    .setAffectedSkus(String.valueOf(r.affectedSkus()))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void batchUpdateItemsTargetStatus(BatchUpdateItemsTargetStatusRequest request,
                                              StreamObserver<BatchUpdateItemsTargetStatusResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            List<Long> ids = new ArrayList<>();
            for (String s : request.getImportItemIdsList()) ids.add(parseLong(s));
            int n = itemMutationService.batchUpdateTargetStatus(request.getStoreId(),
                    jobId, ids, request.getApplyAll(), request.getTargetStatus());
            responseObserver.onNext(BatchUpdateItemsTargetStatusResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setAffectedItems(String.valueOf(n)).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void batchUpdateItemsCategory(BatchUpdateItemsCategoryRequest request,
                                          StreamObserver<BatchUpdateItemsCategoryResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            String tokenCat = categoryTokenService.consume(jobId, request.getConfirmationToken());
            if (tokenCat == null || !tokenCat.equals(request.getCategoryId())) {
                responseObserver.onError(Status.INVALID_ARGUMENT
                        .withDescription("confirmation_token invalid or category mismatch")
                        .asRuntimeException());
                return;
            }
            List<Long> ids = new ArrayList<>();
            for (String s : request.getImportItemIdsList()) ids.add(parseLong(s));
            ItemMutationService.BatchCategoryResult r = itemMutationService.batchUpdateCategory(
                    request.getStoreId(), jobId, ids, request.getApplyAll(), request.getCategoryId());
            responseObserver.onNext(BatchUpdateItemsCategoryResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setAffectedItems(String.valueOf(r.affectedItems()))
                    .setCatL1Id(orEmpty(r.cat().getCatL1Id()))
                    .setCatL1Name(orEmpty(r.cat().getCatL1Name()))
                    .setCatL2Id(orEmpty(r.cat().getCatL2Id()))
                    .setCatL2Name(orEmpty(r.cat().getCatL2Name()))
                    .setCatL3Id(orEmpty(r.cat().getCatL3Id()))
                    .setCatL3Name(orEmpty(r.cat().getCatL3Name()))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void getCategoryConfirmationToken(GetCategoryConfirmationTokenRequest request,
                                              StreamObserver<GetCategoryConfirmationTokenResponse> responseObserver) {
        try {
            Long jobId = parseLong(request.getImportJobId());
            ProductImportJobEntity job = jobService.getJob(request.getStoreId(), jobId);
            if (job == null) {
                responseObserver.onError(Status.NOT_FOUND
                        .withDescription("import job not found").asRuntimeException());
                return;
            }
            ItemMutationService.BatchCategoryResult validate;
            try {
                validate = itemMutationService.batchUpdateCategory(
                        request.getStoreId(), jobId, List.of(), false, request.getCategoryId());
            } catch (StatusRuntimeException sre) {
                // re-throw category-related but skip "no items to update" precondition (we're just validating)
                if (sre.getStatus().getCode() == Status.Code.INVALID_ARGUMENT) {
                    throw sre;
                }
                validate = new ItemMutationService.BatchCategoryResult(0,
                        productGrpcValidateCategoryFallback(request.getCategoryId()));
            }
            CategoryTokenService.IssuedToken issued = categoryTokenService.issue(jobId, request.getCategoryId());
            responseObserver.onNext(GetCategoryConfirmationTokenResponse.newBuilder()
                    .setCode(0).setMessage("success")
                    .setConfirmationToken(issued.token())
                    .setExpiresAt(DateTimeUtils.formatIso8601(
                            DateTimeUtils.nowUtc().plusSeconds(issued.ttlSeconds())))
                    .setCatL1Id(orEmpty(validate.cat().getCatL1Id()))
                    .setCatL1Name(orEmpty(validate.cat().getCatL1Name()))
                    .setCatL2Id(orEmpty(validate.cat().getCatL2Id()))
                    .setCatL2Name(orEmpty(validate.cat().getCatL2Name()))
                    .setCatL3Id(orEmpty(validate.cat().getCatL3Id()))
                    .setCatL3Name(orEmpty(validate.cat().getCatL3Name()))
                    .build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException sre) {
            responseObserver.onError(sre);
        } catch (Exception e) {
            responseObserver.onError(Status.INTERNAL.withCause(e).withDescription(e.getMessage()).asRuntimeException());
        }
    }

    private io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse productGrpcValidateCategoryFallback(String categoryId) {
        return io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse.newBuilder()
                .setIsValid(true)
                .setCatL3Id(categoryId)
                .build();
    }

    // ========================================================================
    // Proto helpers
    // ========================================================================

    private ImportJobCounters toProtoCounters(ImportJobProgressCalculator.Counters c) {
        return ImportJobCounters.newBuilder()
                .setItemsTotal(String.valueOf(c.itemsTotal()))
                .setItemsParsed(String.valueOf(c.itemsParsed()))
                .setItemsAssetDownloading(String.valueOf(c.itemsAssetDownloading()))
                .setItemsReady(String.valueOf(c.itemsReady()))
                .setItemsNeedsFix(String.valueOf(c.itemsNeedsFix()))
                .setItemsRemoved(String.valueOf(c.itemsRemoved()))
                .setItemsCommitting(String.valueOf(c.itemsCommitting()))
                .setItemsCommitted(String.valueOf(c.itemsCommitted()))
                .setItemsCommitFailed(String.valueOf(c.itemsCommitFailed()))
                .setAssetsTotal(String.valueOf(c.assetsTotal()))
                .setAssetsPending(String.valueOf(c.assetsPending()))
                .setAssetsDownloading(String.valueOf(c.assetsDownloading()))
                .setAssetsUploaded(String.valueOf(c.assetsUploaded()))
                .setAssetsFailed(String.valueOf(c.assetsFailed()))
                .setAssetsLinked(String.valueOf(c.assetsLinked()))
                .build();
    }

    private ImportJobProgress toProtoProgress(ImportJobProgressCalculator.Progress p) {
        return ImportJobProgress.newBuilder()
                .setOverallPercentage(String.valueOf(p.overallPercentage()))
                .setParsePercentage(String.valueOf(p.parsePercentage()))
                .setAssetDownloadPercentage(String.valueOf(p.assetDownloadPercentage()))
                .setCommitPercentage(String.valueOf(p.commitPercentage()))
                .setParseCompleted(p.parseCompleted())
                .setAssetDownloadCompleted(p.assetDownloadCompleted())
                .setCommitCompleted(p.commitCompleted())
                .build();
    }

    private ImportItemBrief toProtoBrief(ItemQueryService.BriefView v) {
        ImportItemBriefProjection p = v.brief();
        return ImportItemBrief.newBuilder()
                .setImportItemId(String.valueOf(p.importItemId()))
                .setProductKey(orEmpty(p.productKey()))
                .setStatus(orEmpty(p.status()))
                .setName(orEmpty(p.name()))
                .setCategoryId(p.categoryId() == null ? "" : String.valueOf(p.categoryId()))
                .setCatL3Name(orEmpty(p.catL3Name()))
                .setSkuCount(p.skuCount() == null ? "0" : String.valueOf(p.skuCount()))
                .setPriceMin(String.valueOf(v.priceMin()))
                .setPriceMax(String.valueOf(v.priceMax()))
                .setFirstImageUrl(orEmpty(v.firstImageUrl()))
                .setBlockingErrorCount(p.blockingErrorCount() == null ? "0" : String.valueOf(p.blockingErrorCount()))
                .setWarningCount(p.warningCount() == null ? "0" : String.valueOf(p.warningCount()))
                .setCommitFailedReason(orEmpty(p.commitFailedReason()))
                .setFormalProductId(p.formalProductId() == null ? "" : String.valueOf(p.formalProductId()))
                .setNeedsFix(p.blockingErrorCount() != null && p.blockingErrorCount() > 0)
                .setHasDuplicateExisting(p.duplicateExistingProductId() != null)
                .setCommitRevision(p.commitRevision() == null ? "1" : String.valueOf(p.commitRevision()))
                .setVersion(p.version() == null ? "0" : String.valueOf(p.version()))
                // 40 §4.2.6：同名既有商品（has_duplicate_existing=true 時填）
                .setExistingProductId(p.existingProductId() == null ? "" : String.valueOf(p.existingProductId()))
                .setExistingProductName(orEmpty(p.existingProductName()))
                .setExistingProductStatusLabel(orEmpty(p.existingProductStatusLabel()))
                // 40 §4.2.6 / §4.3b：COMMITTED 時 product-service 即時查得
                .setFormalProductStatus(orEmpty(v.formalProductStatus()))
                .setFormalProductStatusLabel(orEmpty(v.formalProductStatusLabel()))
                .setFormalProductPublicationState(orEmpty(v.formalProductPublicationState()))
                .setFormalProductStatusUnavailable(v.formalProductStatusUnavailable())
                .build();
    }

    private ImportItemDetail toProtoDetail(ItemQueryService.DetailView dv) {
        ProductImportItemEntity it = dv.item();
        ImportItemBrief.Builder brief = ImportItemBrief.newBuilder()
                .setImportItemId(String.valueOf(it.getId()))
                .setProductKey(orEmpty(it.getProductKey()))
                .setStatus(orEmpty(it.getStatus()))
                .setName(orEmpty(it.getName()))
                .setCategoryId(it.getCategoryId() == null ? "" : String.valueOf(it.getCategoryId()))
                .setCatL3Name(orEmpty(it.getCatL3Name()))
                .setSkuCount(it.getSkuCount() == null ? "0" : String.valueOf(it.getSkuCount()))
                .setPriceMin(String.valueOf(dv.priceMin()))
                .setPriceMax(String.valueOf(dv.priceMax()))
                .setFirstImageUrl(orEmpty(dv.firstImageUrl()))
                .setBlockingErrorCount(it.getBlockingErrorCount() == null ? "0"
                        : String.valueOf(it.getBlockingErrorCount()))
                .setWarningCount(it.getWarningCount() == null ? "0" : String.valueOf(it.getWarningCount()))
                .setCommitFailedReason(orEmpty(it.getCommitFailedReason()))
                .setFormalProductId(it.getFormalProductId() == null ? "" : String.valueOf(it.getFormalProductId()))
                .setNeedsFix(it.getBlockingErrorCount() != null && it.getBlockingErrorCount() > 0)
                .setHasDuplicateExisting(it.getDuplicateExistingProductId() != null)
                .setCommitRevision(it.getCommitRevision() == null ? "1" : String.valueOf(it.getCommitRevision()))
                .setVersion(it.getVersion() == null ? "0" : String.valueOf(it.getVersion()))
                ;
        // #2819：詳情路徑不填 formal_product_* —— getItemDetail 對 COMMITTED item 一律
        // NOT_FOUND，非 COMMITTED item 沒有正式商品可言。原本的映射讀的是永遠為空的欄位。
        // 40 §3.5：詳情頁的 existing_product_* 從同 item 的 DUPLICATE_NAME_VS_EXISTING error row 取
        for (ProductImportItemErrorEntity er : dv.errors()) {
            if ("DUPLICATE_NAME_VS_EXISTING".equals(er.getErrorType())
                    && er.getExistingProductId() != null) {
                brief.setExistingProductId(String.valueOf(er.getExistingProductId()));
                brief.setExistingProductName(orEmpty(er.getExistingProductName()));
                brief.setExistingProductStatusLabel(orEmpty(er.getExistingProductStatusLabel()));
                break;
            }
        }

        ImportItemDetail.Builder b = ImportItemDetail.newBuilder()
                .setBrief(brief.build())
                .setDescriptionJson(it.getDescriptionJson() == null ? "" : it.getDescriptionJson().toString())
                .setCommitRevision(it.getCommitRevision() == null ? "1" : String.valueOf(it.getCommitRevision()))
                .setOrigin(orEmpty(it.getOrigin()))
                .setTargetStatus(orEmpty(it.getTargetStatus()));
        for (String h : ifNull(it.getHashtags())) b.addHashtags(h);
        for (Map<String, Object> c : ifNull(it.getCertifications())) {
            b.addCertifications(CertificationDto.newBuilder()
                    .setName(String.valueOf(c.getOrDefault("name", "")))
                    .setCertNo(String.valueOf(c.getOrDefault("certNo", "")))
                    .build());
        }
        for (ProductImportItemSkuEntity sku : dv.skus()) b.addSkus(toProtoSku(sku));
        for (ProductImportItemAssetEntity asset : dv.assets()) {
            ImportItemAssetDto.Builder ab = ImportItemAssetDto.newBuilder()
                    .setAssetId(String.valueOf(asset.getId()))
                    .setUsage(orEmpty(asset.getUsage()))
                    .setSortOrder(asset.getSortOrder() == null ? "0" : String.valueOf(asset.getSortOrder()))
                    .setBlockIndex(asset.getBlockIndex() == null ? "" : String.valueOf(asset.getBlockIndex()))
                    .setSourceUrl(orEmpty(asset.getSourceUrl()))
                    .setStatus(orEmpty(asset.getStatus()))
                    .setPreviewUrl(orEmpty(asset.getPreviewUrl()))
                    .setFailureReason(orEmpty(asset.getFailureReason()))
                    .setFailureDetail(orEmpty(asset.getFailureDetail()))
                    .setRetryCount(asset.getRetryCount() == null ? "0" : String.valueOf(asset.getRetryCount()));
            if ("GALLERY".equals(asset.getUsage())) b.addGalleryAssets(ab.build());
            else b.addDescriptionAssets(ab.build());
        }
        for (ProductImportItemErrorEntity e : dv.errors()) {
            ImportItemErrorDto.Builder eb = toProtoError(e);
            if ("ERROR".equals(e.getSeverity())) b.addErrors(eb.build());
            else b.addWarnings(eb.build());
        }
        return b.build();
    }

    private ImportItemErrorDto.Builder toProtoError(ProductImportItemErrorEntity e) {
        String message = ImportErrorMessageCatalog.displayMessage(
                e.getField(), e.getErrorType(), e.getMessage());
        return ImportItemErrorDto.newBuilder()
                .setErrorType(orEmpty(e.getErrorType()))
                .setSeverity(orEmpty(e.getSeverity()))
                .setField(orEmpty(e.getField()))
                .setMessage(orEmpty(message))
                .setCsvLine(e.getCsvLine() == null ? "0" : String.valueOf(e.getCsvLine()));
    }

    private ImportItemSkuDto toProtoSku(ProductImportItemSkuEntity sku) {
        ImportItemSkuDto.Builder b = ImportItemSkuDto.newBuilder()
                .setSkuId(String.valueOf(sku.getId()))
                .setCsvLine(sku.getCsvLine() == null ? "0" : String.valueOf(sku.getCsvLine()))
                .setSkuName(orEmpty(sku.getSkuName()))
                .setPrice(sku.getPrice() == null ? "" : String.valueOf(sku.getPrice()))
                .setSalePrice(sku.getSalePrice() == null ? "" : String.valueOf(sku.getSalePrice()))
                .setStockQty(sku.getStockQty() == null ? "0" : String.valueOf(sku.getStockQty()))
                .setWeight(sku.getWeight() == null ? "" : sku.getWeight().toPlainString())
                .setUnit(orEmpty(sku.getUnit()))
                .setLengthCm(sku.getLengthCm() == null ? "" : sku.getLengthCm().toPlainString())
                .setWidthCm(sku.getWidthCm() == null ? "" : sku.getWidthCm().toPlainString())
                .setHeightCm(sku.getHeightCm() == null ? "" : sku.getHeightCm().toPlainString());
        if (sku.getShippingMethods() != null) {
            for (Map<String, Object> m : sku.getShippingMethods()) {
                b.addShippingMethods(ShippingMethodDto.newBuilder()
                        .setMethod(String.valueOf(m.getOrDefault("method", "")))
                        .setTemperature(String.valueOf(m.getOrDefault("temperature", "")))
                        .build());
            }
        }
        return b.build();
    }

    private List<GalleryItem> buildGallery(List<ProductImportItemAssetEntity> assets) {
        List<GalleryItem> list = new ArrayList<>();
        int order = 0;
        for (ProductImportItemAssetEntity a : assets) {
            if (!"GALLERY".equals(a.getUsage())) continue;
            String url = a.getPreviewUrl() != null && !a.getPreviewUrl().isBlank()
                    ? a.getPreviewUrl()
                    : (a.getSourceUrl() == null ? "" : a.getSourceUrl());
            list.add(GalleryItem.newBuilder()
                    .setUrl(url)
                    .setSortOrder(String.valueOf(order++))
                    .build());
        }
        return list;
    }

    private void appendValidationErrors(List<Map<String, Object>> errors,
                                        java.util.function.Consumer<ValidationError> appender) {
        if (errors == null) return;
        for (Map<String, Object> m : errors) {
            appender.accept(ValidationError.newBuilder()
                    .setCsvLine(String.valueOf(m.getOrDefault("csv_line", "0")))
                    .setField(String.valueOf(m.getOrDefault("field", "")))
                    .setErrorType(String.valueOf(m.getOrDefault("error_type", "")))
                    .setMessage(String.valueOf(m.getOrDefault("message", "")))
                    .setRawValue(String.valueOf(m.getOrDefault("raw_value", "")))
                    .build());
        }
    }

    private static <T> List<T> ifNull(List<T> v) {
        return v == null ? List.of() : v;
    }

    private static String orEmpty(String s) {
        return s == null ? "" : s;
    }

    private static long parseLong(String s) {
        if (s == null || s.isBlank()) {
            throw Status.INVALID_ARGUMENT.withDescription("numeric id required").asRuntimeException();
        }
        try {
            return Long.parseLong(s);
        } catch (NumberFormatException nfe) {
            throw Status.INVALID_ARGUMENT.withDescription("invalid numeric id: " + s).asRuntimeException();
        }
    }

    private static int parsePositiveInt(String s, int defaultVal) {
        if (s == null || s.isBlank()) return defaultVal;
        try {
            int v = Integer.parseInt(s);
            return v > 0 ? v : defaultVal;
        } catch (NumberFormatException nfe) {
            return defaultVal;
        }
    }
}
