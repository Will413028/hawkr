package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse;
import io.github.will413028.hawkr.product.grpc.ProductBriefInternal;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemErrorEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.entity.projection.ImportItemBriefProjection;
import io.github.will413028.hawkr.productimport.entity.projection.SkuPriceRangeProjection;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.grpc.Status;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@Service
public class ItemQueryService {

    private static final ObsLogger log = ObsLogger.forClass(ItemQueryService.class);

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemSkuMapper skuMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportItemErrorMapper errorMapper;
    private final ProductGrpcClient productClient;

    public ItemQueryService(ProductImportJobMapper jobMapper,
                            ProductImportItemMapper itemMapper,
                            ProductImportItemSkuMapper skuMapper,
                            ProductImportItemAssetMapper assetMapper,
                            ProductImportItemErrorMapper errorMapper,
                            ProductGrpcClient productClient) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.skuMapper = skuMapper;
        this.assetMapper = assetMapper;
        this.errorMapper = errorMapper;
        this.productClient = productClient;
    }

    public ListResult listItems(long storeId, long jobId, List<String> statusFilter,
                                boolean onlyWithErrors, boolean includeRemoved,
                                int page, int pageSize) {
        ensureJobOwnership(storeId, jobId);
        int limit = Math.max(1, Math.min(pageSize, 100));
        int offset = Math.max(0, (page - 1) * limit);
        List<ImportItemBriefProjection> briefs =
                itemMapper.listBrief(jobId, includeRemoved, limit, offset);
        QueryWrapper<ProductImportItemEntity> cqw = new QueryWrapper<>();
        cqw.eq("import_job_id", jobId);
        if (!includeRemoved) cqw.ne("status", ImportItemStatus.REMOVED.value());
        Long total = itemMapper.selectCount(cqw);

        List<Long> ids = briefs.stream().map(ImportItemBriefProjection::importItemId).toList();
        Map<Long, long[]> priceRange = new HashMap<>();
        Map<Long, String> firstImage = new HashMap<>();
        if (!ids.isEmpty()) {
            for (SkuPriceRangeProjection pr : skuMapper.priceRangeByItems(ids)) {
                priceRange.put(pr.importItemId(),
                        new long[]{pr.priceMin() == null ? 0L : pr.priceMin(),
                                   pr.priceMax() == null ? 0L : pr.priceMax()});
            }
            QueryWrapper<ProductImportItemAssetEntity> aqw = new QueryWrapper<>();
            aqw.in("import_item_id", ids).eq("usage", ImportAssetUsage.GALLERY.value())
                    .eq("sort_order", 0);
            for (ProductImportItemAssetEntity a : assetMapper.selectList(aqw)) {
                firstImage.putIfAbsent(a.getImportItemId(),
                        a.getPreviewUrl() != null ? a.getPreviewUrl()
                                : (a.getSourceUrl() != null ? a.getSourceUrl() : ""));
            }
        }
        // 40 §3.5 / §4.2.6：COMMITTED item batch RPC 查 product-service GetProductsBriefInternal 取
        // formal_product_status / formal_product_status_label（不落地反正規化）。
        BriefLookup formalLookup = batchFetchCommittedProductsBrief(storeId, briefs);
        Map<Long, ProductBriefInternal> formalById = formalLookup.byId();

        List<BriefView> records = new ArrayList<>();
        for (ImportItemBriefProjection p : briefs) {
            if (onlyWithErrors && (p.blockingErrorCount() == null || p.blockingErrorCount() <= 0)) continue;
            if (statusFilter != null && !statusFilter.isEmpty() && !statusFilter.contains(p.status())) continue;
            long[] rng = priceRange.getOrDefault(p.importItemId(), new long[]{0L, 0L});
            ProductBriefInternal formal = p.formalProductId() == null ? null : formalById.get(p.formalProductId());
            String formalStatus = formal == null ? "" : formal.getStatus();
            String formalStatusLabel = formal == null ? "" : formal.getStatusLabel();
            String formalPublicationState = formal == null ? "" : formal.getPublicationState();
            // 「查不到」只在**這個 item 真的需要查**（COMMITTED ＋ 有正式商品 id）時才成立。
            // 沒有 formalProductId 的 item 本來就沒有狀態可言，不該被標成查詢失敗。
            boolean needsFormal = ImportItemStatus.COMMITTED.value().equals(p.status())
                    && p.formalProductId() != null;
            boolean formalUnavailable = needsFormal && formal == null;
            records.add(new BriefView(p, rng[0], rng[1], firstImage.getOrDefault(p.importItemId(), ""),
                    formalStatus, formalStatusLabel, formalPublicationState, formalUnavailable));
        }
        return new ListResult(records, total == null ? 0L : total);
    }

    /**
     * 批次查詢的結果：**查得的資料**與**查詢是否失敗**分開表達。
     *
     * <p>兩者獨立（#2819）：`byId` 可能已有部分結果，同時 `lookupFailed=true`。之前用
     * 單一個 `Map.of()` 同時表達「RPC 失敗」與「這批都查無此商品」，呼叫端因此無從分辨，
     * 兩種語意南轅北轍的情況壓成同一個空字串。
     */
    private record BriefLookup(Map<Long, ProductBriefInternal> byId, boolean lookupFailed) {}

    /**
     * 40 §4.3b：對該頁 COMMITTED 的 item 批次查 product-service GetProductsBriefInternal。
     * RPC 失敗不阻擋列表回應，但**必須讓呼叫端知道失敗了**——回傳 {@link BriefLookup}。
     */
    private BriefLookup batchFetchCommittedProductsBrief(long storeId,
                                                          List<ImportItemBriefProjection> briefs) {
        List<String> ids = new ArrayList<>();
        for (ImportItemBriefProjection p : briefs) {
            if (ImportItemStatus.COMMITTED.value().equals(p.status())
                    && p.formalProductId() != null) {
                ids.add(String.valueOf(p.formalProductId()));
            }
        }
        if (ids.isEmpty()) return new BriefLookup(Map.of(), false);
        try {
            GetProductsBriefInternalResponse resp = productClient.getProductsBriefInternal(storeId, ids);
            Map<Long, ProductBriefInternal> map = new HashMap<>();
            for (ProductBriefInternal b : resp.getRecordsList()) {
                if (b.getProductId() == null || b.getProductId().isBlank()) continue;
                try {
                    map.put(Long.parseLong(b.getProductId()), b);
                } catch (NumberFormatException nfe) {
                    log.warn("ItemQueryService.batchFetchCommitted.bad_id",
                            "non-numeric product_id=" + b.getProductId());
                }
            }
            return new BriefLookup(map, false);
        } catch (Exception e) {
            log.warn("ItemQueryService.batchFetchCommitted.failed",
                    "ids=" + ids + ", reason=" + e.getMessage());
            return new BriefLookup(Map.of(), true);
        }
    }

    public DetailView getItemDetail(long storeId, long jobId, long itemId) {
        ProductImportItemEntity item = itemMapper.selectById(itemId);
        if (item == null || !item.getImportJobId().equals(jobId)
                || !item.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import item not found").asRuntimeException();
        }
        if (ImportItemStatus.COMMITTED.value().equals(item.getStatus())) {
            throw Status.NOT_FOUND.withDescription("import item not found").asRuntimeException();
        }
        List<ProductImportItemSkuEntity> skus = skuMapper.selectList(
                new QueryWrapper<ProductImportItemSkuEntity>()
                        .eq("import_item_id", itemId).orderByAsc("sort_order"));
        List<ProductImportItemAssetEntity> assets = assetMapper.selectList(
                new QueryWrapper<ProductImportItemAssetEntity>()
                        .eq("import_item_id", itemId).orderByAsc("sort_order"));
        List<ProductImportItemErrorEntity> errors = errorMapper.selectList(
                new QueryWrapper<ProductImportItemErrorEntity>().eq("import_item_id", itemId));
        long[] rng = priceRangeOf(itemId);
        String firstImg = assets.stream()
                .filter(a -> ImportAssetUsage.GALLERY.value().equals(a.getUsage())
                        && Integer.valueOf(0).equals(a.getSortOrder()))
                .findFirst()
                .map(a -> a.getPreviewUrl() != null ? a.getPreviewUrl()
                        : (a.getSourceUrl() != null ? a.getSourceUrl() : ""))
                .orElse("");
        // #2819：這裡原本有一段「COMMITTED item 即時查 product-service 取 formal_product_status」，
        // 已刪除 —— 它是**死碼**：上方 :164 的守衛對 COMMITTED item 一律丟 NOT_FOUND，
        // 那段程式碼的唯一進入條件是 status == COMMITTED，因此永遠跑不到。
        // git 佐證：該區塊在 65cdf806a（2026-06-19）加入，守衛在 649130831（2026-06-20）
        // 隔天加入，守衛把它變成死碼而無人察覺。既有測試
        // getItemDetailReturnsNotFoundForCommittedItem 的 verifyNoInteractions(productClient)
        // 其實一直在記錄這件事。
        //
        // 正式商品狀態只在**列表**路徑有意義（listItems），已在該處修好三態區分。
        return new DetailView(item, skus, assets, errors, rng[0], rng[1], firstImg);
    }

    public PreviewListingResult previewListing(long storeId, long jobId, boolean includePending,
                                                int page, int pageSize) {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import job not found").asRuntimeException();
        }
        QueryWrapper<ProductImportItemEntity> qw = new QueryWrapper<>();
        qw.eq("import_job_id", jobId).eq("blocking_error_count", 0);
        if (includePending) {
            qw.in("status", ImportItemStatus.READY.value(),
                    ImportItemStatus.COMMIT_FAILED.value(),
                    ImportItemStatus.ASSET_DOWNLOADING.value());
        } else {
            qw.in("status", ImportItemStatus.READY.value(), ImportItemStatus.COMMIT_FAILED.value());
        }
        qw.orderByAsc("first_csv_line");
        Page<ProductImportItemEntity> p = new Page<>(page, Math.min(pageSize, 100));
        Page<ProductImportItemEntity> result = itemMapper.selectPage(p, qw);

        QueryWrapper<ProductImportItemEntity> eligibleQw = new QueryWrapper<>();
        eligibleQw.eq("import_job_id", jobId)
                .eq("blocking_error_count", 0)
                .in("status", ImportItemStatus.READY.value(), ImportItemStatus.COMMIT_FAILED.value());
        Long eligible = itemMapper.selectCount(eligibleQw);

        QueryWrapper<ProductImportItemEntity> pendingQw = new QueryWrapper<>();
        pendingQw.eq("import_job_id", jobId)
                .eq("status", ImportItemStatus.ASSET_DOWNLOADING.value());
        Long pending = itemMapper.selectCount(pendingQw);

        QueryWrapper<ProductImportItemEntity> removedQw = new QueryWrapper<>();
        removedQw.eq("import_job_id", jobId)
                .eq("status", ImportItemStatus.REMOVED.value());
        Long removed = itemMapper.selectCount(removedQw);

        QueryWrapper<ProductImportItemEntity> needsFixQw = new QueryWrapper<>();
        needsFixQw.eq("import_job_id", jobId).gt("blocking_error_count", 0);
        Long needsFix = itemMapper.selectCount(needsFixQw);

        return new PreviewListingResult(result.getRecords(), result.getTotal(),
                eligible == null ? 0L : eligible,
                needsFix == null ? 0L : needsFix,
                pending == null ? 0L : pending,
                removed == null ? 0L : removed);
    }

    public DetailView getItemPreview(long storeId, long jobId, long itemId) {
        return getItemDetail(storeId, jobId, itemId);
    }

    private long[] priceRangeOf(long itemId) {
        List<SkuPriceRangeProjection> list = skuMapper.priceRangeByItems(List.of(itemId));
        if (list.isEmpty()) return new long[]{0L, 0L};
        SkuPriceRangeProjection pr = list.get(0);
        return new long[]{pr.priceMin() == null ? 0L : pr.priceMin(),
                pr.priceMax() == null ? 0L : pr.priceMax()};
    }

    private void ensureJobOwnership(long storeId, long jobId) {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import job not found").asRuntimeException();
        }
    }

    public record BriefView(ImportItemBriefProjection brief, long priceMin, long priceMax,
                            String firstImageUrl,
                            /* 40 §4.2.6 / §4.3b：COMMITTED item 用 GetProductsBriefInternal 即時查得 */
                            String formalProductStatus,
                            String formalProductStatusLabel,
                            /* #2819：ProductPublicationState；4014 = 已移除 */
                            String formalProductPublicationState,
                            /* #2819：true = 我方查不到（RPC 失敗或無此列），與「已移除」是兩件事 */
                            boolean formalProductStatusUnavailable) {}

    public record DetailView(ProductImportItemEntity item, List<ProductImportItemSkuEntity> skus,
                             List<ProductImportItemAssetEntity> assets,
                             List<ProductImportItemErrorEntity> errors,
                             long priceMin, long priceMax, String firstImageUrl) {}

    public record ListResult(List<BriefView> records, long total) {}

    public record PreviewListingResult(List<ProductImportItemEntity> items, long total,
                                       long eligible, long needsFix, long pending, long removed) {}
}
