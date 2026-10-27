package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.product.domain.hashtag.HashtagInputValidator;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.entity.projection.SellerProductNameProjection;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalRequest;
import io.github.will413028.hawkr.product.grpc.ProductImageImport;
import io.github.will413028.hawkr.product.grpc.ProductSkuImport;
import io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker;
import io.github.will413028.hawkr.product.service.validator.ProductPublishValidator;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;

/**
 * 商品匯入內部服務（02 §9）。
 *
 * <p>承載三個對 Product Import Service 開放的內部 RPC 業務邏輯：
 * <ul>
 *   <li>{@link #validateLeafCategory(String)} — 驗證 category 為合法葉節點並回傳三層反正規化（§9.1）</li>
 *   <li>{@link #createProductFromImport(CreateProductFromImportInternalRequest)} — Import commit
 *       階段建立正式 product + product_sku，含 idempotency 與 audit 欄位（§9.2 / §9.4）</li>
 *   <li>{@link #batchCheckSellerProductNames(Long, List)} — 批次偵測賣家既有同名商品（§9.5）</li>
 * </ul>
 *
 * <p>與既有 {@link ProductService#createProduct} 嚴格分離（docs/開發規範/Java 微服務開發規範.md §4.1）：不共用 RPC、不走 flag 分流。
 * INSERT 流程相似但語意不同（storeId 來源、idempotency key 格式、audit 欄位、圖片承載），
 * 因此在本服務獨立組裝 entity，僅重用 {@link ProductImportAssetProcessor} / {@link HashtagService} /
 * {@link CategoryService} 等下層元件。
 */
@Service
public class ProductImportService {

    private static final ObsLogger log = ObsLogger.forClass(ProductImportService.class);

    /** §9.5 上限：超過拋 INVALID_ARGUMENT。 */
    static final int MAX_NAMES_PER_BATCH = 500;

    private final CategoryService categoryService;
    private final CategoryCacheService categoryCacheService;
    private final IdempotencyService idempotencyService;
    private final ProductRepository productRepository;
    private final ProductSkuRepository productSkuRepository;
    private final ProductImportAssetProcessor importAssetProcessor;
    private final HashtagService hashtagService;
    private final ProductCacheService cacheService;
    private final ProductPublishValidator productPublishValidator;
    private final ProductMediaReferenceLinker mediaReferenceLinker;
    private final ProductRevalidationService productRevalidationService;
    /** 認證申請收斂點（19 號設計書 §6.1 W5）。獨立 bean，跨 bean 呼叫。 */
    private final ProductCertificationApplicationService certificationApplicationService;

    public ProductImportService(CategoryService categoryService,
                                CategoryCacheService categoryCacheService,
                                IdempotencyService idempotencyService,
                                ProductRepository productRepository,
                                ProductSkuRepository productSkuRepository,
                                ProductImportAssetProcessor importAssetProcessor,
                                HashtagService hashtagService,
                                ProductCacheService cacheService,
                                ProductPublishValidator productPublishValidator,
                                ProductMediaReferenceLinker mediaReferenceLinker,
                                ProductRevalidationService productRevalidationService,
                                ProductCertificationApplicationService certificationApplicationService) {
        this.categoryService = categoryService;
        this.categoryCacheService = categoryCacheService;
        this.idempotencyService = idempotencyService;
        this.productRepository = productRepository;
        this.productSkuRepository = productSkuRepository;
        this.importAssetProcessor = importAssetProcessor;
        this.hashtagService = hashtagService;
        this.cacheService = cacheService;
        this.productPublishValidator = productPublishValidator;
        this.mediaReferenceLinker = mediaReferenceLinker;
        this.productRevalidationService = productRevalidationService;
        this.certificationApplicationService = certificationApplicationService;
    }

    // ==================== §9.1 ValidateLeafCategoryInternal ====================

    /**
     * 驗證 category 為合法葉節點，回傳三層反正規化（02 §9.1）。
     *
     * <p>本方法**不拋 gRPC 例外**：驗證失敗（NOT_FOUND / NOT_LEAF / DISABLED）以
     * {@code is_valid=false + invalid_reason} 形式回應，由呼叫端決定是否升級為錯誤。
     */
    public ValidateLeafCategoryInternalResponse validateLeafCategory(String categoryIdStr) {
        ValidateLeafCategoryInternalResponse.Builder builder =
                ValidateLeafCategoryInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("success");

        Long categoryId = parseCategoryId(categoryIdStr);
        if (categoryId == null) {
            return builder
                    .setIsValid(false)
                    .setInvalidReason("NOT_FOUND")
                    .build();
        }

        Optional<CategoryEntity> opt = categoryCacheService.findById(categoryId);
        if (opt.isEmpty()) {
            return builder
                    .setIsValid(false)
                    .setInvalidReason("NOT_FOUND")
                    .build();
        }

        CategoryEntity leaf = opt.get();
        if (!io.github.will413028.hawkr.common.utils.code.CommonStatus.ACTIVE.value().equals(leaf.getStatus())) {
            return builder
                    .setIsValid(false)
                    .setInvalidReason("DISABLED")
                    .build();
        }

        int level = categoryService.detectCategoryLevel(categoryId);
        if (level != 3) {
            return builder
                    .setIsValid(false)
                    .setInvalidReason("NOT_LEAF")
                    .build();
        }

        // L3 → L2 → L1 反查
        Long lv2Id = leaf.getParentId();
        Long lv1Id = (lv2Id != null) ? categoryService.getParentId(lv2Id) : null;
        CategoryEntity lv2 = (lv2Id != null) ? categoryCacheService.findById(lv2Id).orElse(null) : null;
        CategoryEntity lv1 = (lv1Id != null) ? categoryCacheService.findById(lv1Id).orElse(null) : null;

        if (lv2 == null || lv1 == null) {
            // 理論上 detectCategoryLevel=3 才會走到此，缺父屬資料異常
            return builder
                    .setIsValid(false)
                    .setInvalidReason("NOT_FOUND")
                    .build();
        }

        return builder
                .setIsValid(true)
                .setCatL3Id(String.valueOf(leaf.getId()))
                .setCatL3Name(nullSafe(leaf.getName()))
                .setCatL2Id(String.valueOf(lv2.getId()))
                .setCatL2Name(nullSafe(lv2.getName()))
                .setCatL1Id(String.valueOf(lv1.getId()))
                .setCatL1Name(nullSafe(lv1.getName()))
                .build();
    }

    // ==================== §9.2 CreateProductFromImportInternal ====================

    /**
     * 由 Product Import Service commit 階段呼叫，將暫存 item 轉成正式 product + product_sku。
     *
     * <p>流程（02 §9.2 / §9.4）：
     * <ol>
     *   <li>idempotency_key acquireLock — endpoint=CreateProductFromImportInternal，
     *       SUCCEEDED 直接回 cached、FAILED 自動允許 retry</li>
     *   <li>重驗證 leaf category（避免 csv.parse 後 admin 軟刪除分類）</li>
     *   <li>重驗證 SKU 欄位（WeightUnit / ShippingTemperature / 數值範圍）</li>
     *   <li>INSERT product（含 importJobId / importItemId / catL1~3）</li>
     *   <li>INSERT product_sku（多筆）</li>
     *   <li>處理 gallery import asset（FinalizeImportAsset → 回填 assetId / objectKey）</li>
     *   <li>處理 description blocks import asset（同上，PRODUCT_DESC）</li>
     *   <li>反正規化彙總（price range / stock / shipping_methods / cat_*_name / searchable_text）+
     *       updateById 落盤</li>
     *   <li>markSucceeded + 回傳 product_id</li>
     * </ol>
     *
     * <p>所有寫入同一事務；任一步驟失敗 rollback 後標 FAILED（同 key 允許後續 retry）。
     */
    @Transactional
    public CreateProductFromImportInternalResponseDTO createProductFromImport(
            CreateProductFromImportInternalRequest request) {

        String idempotencyKey = request.getIdempotencyKey();
        Long storeId = request.getStoreId();
        if (idempotencyKey == null || idempotencyKey.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "idempotency_key 為必填欄位");
        }
        if (storeId == null || storeId <= 0) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "store_id 為必填欄位");
        }

        String requestHash = idempotencyService.calculateHash(request.toByteArray());

        Optional<CreateProductFromImportInternalResponseDTO> cached = idempotencyService.acquireLock(
                idempotencyKey,
                storeId,
                "CreateProductFromImportInternal",
                requestHash,
                CreateProductFromImportInternalResponseDTO.class);
        if (cached.isPresent()) {
            CreateProductFromImportInternalResponseDTO dto = cached.get();
            log.info("ProductImportService.createProductFromImport.duplicated",
                    "回傳先前成功結果，key=" + idempotencyKey + "，product_id=" + dto.getProductId());
            return new CreateProductFromImportInternalResponseDTO(
                    dto.getProductId(),
                    "DUPLICATED",
                    dto.getCreatedAt(),
                    dto.getStatus());
        }

        try {
            CreateProductFromImportInternalResponseDTO result = doCreate(request);
            idempotencyService.markSucceeded(idempotencyKey, storeId, result);
            return result;
        } catch (Exception e) {
            log.error("ProductImportService.createProductFromImport.failed",
                    "商品轉正失敗，key=" + idempotencyKey + "，error=" + e.getMessage(), e);
            try {
                idempotencyService.markFailed(idempotencyKey, storeId);
            } catch (Exception markEx) {
                log.error("ProductImportService.createProductFromImport.markFailedError",
                        "標記冪等失敗時出錯: " + markEx.getMessage());
            }
            throw e;
        }
    }

    private CreateProductFromImportInternalResponseDTO doCreate(CreateProductFromImportInternalRequest request) {
        Long storeId = request.getStoreId();

        // 0. Defense-in-depth：commit final guard 跑 ProductPublishValidator（40 §4.3a）
        //    呼叫端（product-import-service §5.7）理論上已驗過，但 DB 落地前再驗一次防 drift。
        //    違規視為「validator drift」直接拋 INVALID_PARAM，由 caller 端落 product_import_item_error。
        io.github.will413028.hawkr.product.grpc.ProductDraft draft = buildDraftFromRequest(request);
        ProductPublishValidator.ValidationResult vr = productPublishValidator.validate(
                storeId == null ? 0L : storeId,
                draft,
                io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest.ValidationContext.IMPORT_COMMIT);
        if (vr.hasBlockingError()) {
            String summary = vr.violations().stream()
                    .filter(v -> "ERROR".equals(v.severity()))
                    .map(v -> v.field() + ":" + v.code())
                    .limit(5)
                    .toList()
                    .toString();
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "validation drift: " + summary + (vr.errorCount() > 5 ? " ...+" + (vr.errorCount() - 5) : ""));
        }

        // 1. 驗證 target_status
        String targetStatus = parseTargetStatus(request.getTargetStatus());

        // 2. 重驗證 leaf category（02 §9.2）
        Long categoryId = parseCategoryId(request.getCategoryId());
        if (categoryId == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "category_id 不可為空且必須為數字");
        }
        CategoryEntity leaf = categoryCacheService.findById(categoryId)
                .orElseThrow(() -> new ProductBusinessException(
                        GrpcResponseCode.CATEGORY_NOT_FOUND,
                        "category_id=" + categoryId + " 不存在"));
        if (!io.github.will413028.hawkr.common.utils.code.CommonStatus.ACTIVE.value().equals(leaf.getStatus())) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND,
                    "category_id=" + categoryId + " 已停用");
        }
        if (categoryService.detectCategoryLevel(categoryId) != 3) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "category_id=" + categoryId + " 非葉節點");
        }
        Long categoryLv2 = leaf.getParentId();
        Long categoryLv1 = categoryService.getParentId(categoryLv2);
        if (categoryLv2 == null || categoryLv1 == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "category 層級資料異常，category_id=" + categoryId);
        }

        // 3. Hashtag fail-fast
        List<String> hashtagsRaw = new ArrayList<>(request.getHashtagsList());
        HashtagInputValidator.requireHashtagSet(hashtagsRaw);

        // 4. 重驗證 SKU & description / images 互斥
        validateSkus(request.getSkusList());
        // description text-block HTML 檢查（image block 走 import_asset_id 不檢 HTML）
        validateDescriptionTextBlocks(request.getDescription());

        // 5. INSERT product 主體
        ProductEntity entity = new ProductEntity();
        entity.setId(TsidUtil.generate());
        entity.setName(requireText(request.getName(), "name"));
        entity.setCategory(categoryId);
        entity.setCategoryLv2(categoryLv2);
        entity.setCategoryLv1(categoryLv1);
        entity.setStatus(targetStatus);
        entity.setOrigin(emptyToNull(request.getOrigin()));
        entity.setCreatedBy(storeId);          // audit
        entity.setStoreId(storeId);            // owner：store 代理鍵
        entity.setImportJobId(parseLongOrNull(request.getImportJobId()));
        entity.setImportItemId(parseLongOrNull(request.getImportItemId()));
        entity.setDescription(toDescriptionEntity(request.getDescription()));
        // 認證一律以空陣列落地：投影只由收斂點的 rebuildProjection() 產生（19 §6.1）。
        // 匯入的認證在步驟 12.1 走收斂點建成 SUBMITTED 申請。
        entity.setCertifications(new ArrayList<>());
        productRepository.insert(entity);
        Long productId = entity.getId();

        log.info("ProductImportService.createProductFromImport.entityCreated",
                "商品主體已建立，product_id=" + productId + "，import_item_id=" + request.getImportItemId());

        // 6. 處理 SKU
        List<ProductSkuEntity> skus = new ArrayList<>();
        for (ProductSkuImport skuProto : request.getSkusList()) {
            ProductSkuEntity sku = toSkuEntity(skuProto, productId);
            productSkuRepository.insert(sku);
            skus.add(sku);
        }

        // 7. 處理 gallery import asset（按 sort_order 排序）
        List<ProductImportAssetProcessor.ImportGalleryImage> galleryImports = request.getImagesList().stream()
                .filter(img -> img.getImportAssetId() != null && !img.getImportAssetId().isBlank())
                .sorted(Comparator.comparingInt(ProductImageImport::getSortOrder))
                .map(img -> new ProductImportAssetProcessor.ImportGalleryImage(
                        img.getImportAssetId(), img.getSortOrder()))
                .toList();
        List<ProductEntity.GalleryItem> gallery = importAssetProcessor.processGallery(
                productId, galleryImports, storeId);
        entity.setGallery(gallery);

        // 8. 處理 description blocks import asset
        List<ProductImportAssetProcessor.ImportDescriptionBlock> descImports = collectDescriptionImports(
                entity.getDescription(), request.getDescription());
        importAssetProcessor.processDescription(productId, descImports, storeId);

        // 9. 反正規化彙總
        entity.setStockQty(io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.aggregateStock(skus));
        entity.setShippingMethods(
                io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.mergeShippingMethods(skus));
        updatePriceRange(entity, skus);

        // 10. Hashtag upsert + apply
        List<HashtagService.UpsertedHashtag> upserted = hashtagService.upsertHashtags(hashtagsRaw);
        boolean isActive = ProductStatus.ACTIVE.value().equals(targetStatus);
        hashtagService.applyProductHashtags(productId, upserted);

        // 11. 反正規化文字欄位
        enrichSearchableFields(entity, upserted, skus);

        // 12. 更新 entity（gallery / description / price range / searchable_text / stock / shipping_methods）
        //     entity 是本事務內剛 INSERT 的，version 在記憶體中為 null，OptimisticLockerInnerInterceptor
        //     對 null version 直接略過整段樂觀鎖 ⇒ 0 列不可能是版本衝突，只可能是「剛寫進去的列不見了」。
        //     不判就是回 CREATED 而 gallery / searchable_text 全部沒落盤（商品破圖、搜尋查不到）。
        if (productRepository.updateById(entity) == 0) {
            throw new IllegalStateException("Import 商品轉正補寫影響 0 列，product_id=" + productId);
        }

        // 12.1 認證收斂點（19 §6.1 W5）— 必須在步驟 12 的 updateById 之後，否則記憶體中的空投影
        //      會把剛自動核准的標章覆蓋掉。source = PRODUCT_IMPORT；submitted_by_account_id 用
        //      storeId 之外沒有更好的來源，匯入是賣家自己上傳的 CSV，故沿用本路徑既有的 storeId。
        //      🔴 非法認證（type 不在 3701–3706／certNo 不合規）由收斂點剔除、不拋例外
        //      ——L2 與 parse 兩層維持既有行為不動（§6.5），而 ProductPublishValidator 已從
        //      error() 降為 warning()，這裡再拋例外等於把那次降級抵銷掉、匯入照樣整筆失敗。
        certificationApplicationService.syncFromProductWrite(
                new ProductCertificationApplicationService.CertificationSyncCommand(
                        productId,
                        storeId,
                        // 🔴 submittedByAccountId 留 null：storeId 是營運鍵不是登入 principal，
                        //    頂替會讓 dev 與 fixture 全綠、只有 re-key 過的店家錯（CLAUDE.md §6）。
                        //    匯入來源由 source = PRODUCT_IMPORT 表達。
                        null,
                        io.github.will413028.hawkr.common.utils.code.ProductCertificationSource.PRODUCT_IMPORT.value(),
                        false,
                        toCertificationInputs(request.getCertificationsList())));
        productRepository.findByIdIncludingDeleted(productId)
                .ifPresent(fresh -> entity.setCertifications(fresh.getCertifications()));

        // 12.5 建立 Media 引用（asset_reference）— Finalize → DB 落盤 → Link 順序（02 §9.3a / 08 §7.3）。
        //      ★ 與互動式 createProduct 的 best-effort 不同：import commit 不可逆，link 失敗讓例外傳播
        //      → @Transactional rollback → CommitItemService 標 item COMMIT_FAILED。
        //      漏此步驟會使 media_asset 無引用，7 天後被 Media 孤兒 GC 連同 OSS 物件刪除 → 商品永久破圖（#1203）。
        mediaReferenceLinker.linkGallery(productId, gallery);
        mediaReferenceLinker.linkDescription(productId, entity.getDescription());

        // 13. 清快取
        cacheService.evictProductDetail(productId.toString());
        cacheService.evictProductListForWrite(storeId);

        // 14. frontend cache bust（§9.3）：CSV 匯入轉正＝新商品加入賣家商品集合，
        //     必須 bust seller:<storeId>（列表）+ product:<id>（詳情），否則賣家 storefront
        //     stale 到 TTL（原「首頁看得到、賣家頁看不到匯入商品」缺口，#1271）。
        productRevalidationService.revalidateProductAndSeller(productId, storeId);

        log.info("ProductImportService.createProductFromImport.success",
                "Import 商品轉正成功，product_id=" + productId
                        + "，import_item_id=" + request.getImportItemId()
                        + "，status=" + targetStatus);

        String createdAtIso = DateTimeUtils.formatIso8601(
                entity.getCreatedAt() != null ? entity.getCreatedAt() : DateTimeUtils.nowUtc());
        return new CreateProductFromImportInternalResponseDTO(
                String.valueOf(productId),
                "CREATED",
                createdAtIso,
                targetStatus);
    }

    // ==================== §9.5 BatchCheckSellerProductNames ====================

    /**
     * 批次偵測賣家既有商品中是否有同名商品（02 §9.5）。
     *
     * <p>純查詢，無事務需求；走 partial projection（docs/開發規範/Java 微服務開發規範.md §9.2）僅取 (id, name, status)。
     */
    @Transactional(propagation = Propagation.SUPPORTS, readOnly = true)
    public List<SellerProductNameProjection> batchCheckSellerProductNames(Long storeId, List<String> names) {
        if (storeId == null || storeId <= 0) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "store_id 為必填欄位");
        }
        if (names == null || names.isEmpty()) {
            return List.of();
        }
        if (names.size() > MAX_NAMES_PER_BATCH) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "names 上限為 " + MAX_NAMES_PER_BATCH + "，當前=" + names.size());
        }

        // 去重 + 過濾空字串，避免 SQL IN 多餘參數
        Set<String> normalized = new HashSet<>();
        for (String n : names) {
            if (n != null && !n.isBlank()) {
                normalized.add(n);
            }
        }
        if (normalized.isEmpty()) {
            return List.of();
        }

        return productRepository.findSellerProductNameProjections(storeId, normalized);
    }

    // ==================== 私有 helper ====================

    private static String parseTargetStatus(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "target_status 為必填（ACTIVE / DRAFT / PENDING_REVIEW）");
        }
        // 接受 code（ACTIVE / DRAFT / PENDING_REVIEW）或 value（4001 / 4002 / 4004）
        if (ProductStatus.isValidCode(raw)) {
            ProductStatus ps = ProductStatus.fromCode(raw);
            if (ps == ProductStatus.INACTIVE) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "target_status 不可為 INACTIVE（Import 不支援上架後立即下架）");
            }
            return ps.value();
        }
        if (ProductStatus.isValidValue(raw)) {
            ProductStatus ps = ProductStatus.fromValue(raw);
            if (ps == ProductStatus.INACTIVE) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "target_status 不可為 INACTIVE（Import 不支援上架後立即下架）");
            }
            return ps.value();
        }
        throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                "target_status 不合法：" + raw);
    }

    private static Long parseCategoryId(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Long.parseLong(raw.trim());
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static Long parseLongOrNull(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Long.parseLong(raw.trim());
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static String requireText(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    fieldName + " 不可為空");
        }
        return raw.trim();
    }

    private static String emptyToNull(String raw) {
        return (raw == null || raw.isBlank()) ? null : raw;
    }

    private static String nullSafe(String raw) {
        return raw == null ? "" : raw;
    }

    /**
     * 將 {@link CreateProductFromImportInternalRequest} 映射為 {@link io.github.will413028.hawkr.product.grpc.ProductDraft}，
     * 給 {@link ProductPublishValidator} 驗證用。
     *
     * <p>注意：
     * <ul>
     *   <li>gallery 來源為 {@code images}（{@link ProductImageImport}），映射為 {@link io.github.will413028.hawkr.product.grpc.GalleryItem}
     *       時將 {@code import_asset_id} 寫入 {@code asset_id} 欄位；commit 階段 validator 對
     *       ASSET_NOT_READY 的嚴格檢核目前 product-service 端尚未 wire media client，僅留 TODO。</li>
     *   <li>description_json 為 {@link io.github.will413028.hawkr.product.grpc.ProductDescription} 序列化後 bytes。</li>
     * </ul>
     */
    private static io.github.will413028.hawkr.product.grpc.ProductDraft buildDraftFromRequest(
            CreateProductFromImportInternalRequest request) {
        io.github.will413028.hawkr.product.grpc.ProductDraft.Builder b = io.github.will413028.hawkr.product.grpc.ProductDraft.newBuilder()
                .setName(nullSafe(request.getName()))
                .setCategoryId(nullSafe(request.getCategoryId()))
                .setOrigin(nullSafe(request.getOrigin()))
                .setTargetStatus(nullSafe(request.getTargetStatus()))
                .addAllSkus(request.getSkusList())
                .addAllCertifications(request.getCertificationsList())
                .addAllHashtags(request.getHashtagsList());
        for (ProductImageImport img : request.getImagesList()) {
            b.addGallery(io.github.will413028.hawkr.product.grpc.GalleryItem.newBuilder()
                    .setAssetId(nullSafe(img.getImportAssetId()))
                    .build());
        }
        if (request.hasDescription()) {
            b.setDescriptionJson(request.getDescription().toByteString());
        }
        return b.build();
    }

    private void validateSkus(List<ProductSkuImport> skus) {
        if (skus == null || skus.isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "skus 至少 1 筆");
        }
        for (ProductSkuImport sku : skus) {
            CodeMetaValidator.requireValidValue(WeightUnit.class, sku.getUnit(), "sku.unit");
            // ShippingMethod 內 temperature 對應 ShippingTemperature
            for (io.github.will413028.hawkr.product.grpc.ShippingMethod sm : sku.getShippingMethodsList()) {
                if (sm.getTemperature() != null && !sm.getTemperature().isBlank()) {
                    CodeMetaValidator.requireValidValue(
                            ShippingTemperature.class, sm.getTemperature(), "sku.shipping_methods.temperature");
                }
            }
            requirePositiveLong(sku.getPrice(), "sku.price");
            if (sku.getSalePrice() != null && !sku.getSalePrice().isBlank()) {
                long sale = parseLong(sku.getSalePrice(), "sku.sale_price");
                long price = parseLong(sku.getPrice(), "sku.price");
                if (sale <= 0 || sale >= price) {
                    throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                            "sku.sale_price 必須 > 0 且 < sku.price");
                }
            }
            if (sku.getStockQty() != null && !sku.getStockQty().isBlank()) {
                long stock = parseLong(sku.getStockQty(), "sku.stock_qty");
                if (stock < 0) {
                    throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                            "sku.stock_qty 不可為負");
                }
            }
            // 重量 / 材積必填 > 0（Import 路徑 csv 解析時必填）
            requirePositiveDecimal(sku.getWeight(), "sku.weight");
            requirePositiveDecimal(sku.getLengthCm(), "sku.length_cm");
            requirePositiveDecimal(sku.getWidthCm(), "sku.width_cm");
            requirePositiveDecimal(sku.getHeightCm(), "sku.height_cm");
        }
    }

    private void validateDescriptionTextBlocks(io.github.will413028.hawkr.product.grpc.ProductDescription description) {
        if (description == null || description.getBlocksList().isEmpty()) {
            return;
        }
        for (io.github.will413028.hawkr.product.grpc.DescriptionBlock block : description.getBlocksList()) {
            if (!"text".equals(block.getType())) {
                continue;
            }
            try {
                io.github.will413028.hawkr.common.utils.validation.InputValidator.requireNoHtmlTags(
                        block.getContent(), "商品描述");
            } catch (IllegalArgumentException e) {
                throw new ProductBusinessException(
                        GrpcResponseCode.INVALID_DESCRIPTION_HTML_FORBIDDEN, e.getMessage());
            }
        }
    }

    private ProductEntity.ProductDescription toDescriptionEntity(
            io.github.will413028.hawkr.product.grpc.ProductDescription proto) {
        if (proto == null) {
            return null;
        }
        ProductEntity.ProductDescription desc = new ProductEntity.ProductDescription();
        desc.setVersion(proto.getVersion() != null && !proto.getVersion().isBlank() ? proto.getVersion() : "1.0");
        List<ProductEntity.DescriptionBlock> blocks = new ArrayList<>();
        for (io.github.will413028.hawkr.product.grpc.DescriptionBlock blockProto : proto.getBlocksList()) {
            ProductEntity.DescriptionBlock block = new ProductEntity.DescriptionBlock();
            block.setType(blockProto.getType());
            block.setId(blockProto.getId());
            // inline-run 模型欄位（02 §3.6；#1256 text runs / #1255 table / #1257 list）— import 路徑亦須帶，
            // 否則 commit 建出的正式商品描述丟失樣式 / table / list（text block content 空、list/table 空殼）。
            if (blockProto.getRunsCount() > 0) {
                List<ProductEntity.InlineRun> runs = new ArrayList<>();
                for (io.github.will413028.hawkr.product.grpc.InlineRun rp : blockProto.getRunsList()) {
                    ProductEntity.InlineRun run = new ProductEntity.InlineRun();
                    run.setText(rp.getText());
                    if (rp.getBold()) run.setBold(true);
                    if (rp.getItalic()) run.setItalic(true);
                    if (rp.getUnderline()) run.setUnderline(true);
                    if (rp.getColor() != null && !rp.getColor().isBlank()) run.setColor(rp.getColor());
                    if (rp.getHref() != null && !rp.getHref().isBlank()) run.setHref(rp.getHref());
                    runs.add(run);
                }
                block.setRuns(runs);
            }
            if (blockProto.getTableJson() != null && !blockProto.getTableJson().isBlank()) {
                block.setTableJson(blockProto.getTableJson());
            }
            if (blockProto.getListJson() != null && !blockProto.getListJson().isBlank()) {
                block.setListJson(blockProto.getListJson());
            }
            if ("text".equals(blockProto.getType())) {
                block.setContent(blockProto.getContent());
                // styleJson 此處先不解析（與既有 ProductEntityConverter.convertDescriptionToEntity 一致：
                // import 路徑 csv 不支援 inline style，留 null）
            } else if ("image".equals(blockProto.getType())) {
                // import 路徑：assetId/objectKey 將由 processDescription 在 FinalizeImportAsset 後回填，
                // 互斥檢查：同一 block 不可同時帶 stagedToken 與 importAssetId
                boolean hasStaged = blockProto.getStagedToken() != null && !blockProto.getStagedToken().isBlank();
                boolean hasImport = blockProto.getImportAssetId() != null && !blockProto.getImportAssetId().isBlank();
                if (hasStaged && hasImport) {
                    throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                            "description block 不可同時帶 staged_token 與 import_asset_id");
                }
                // 入庫前 entity 暫不持有 importAssetId（不在 entity model）；落盤前由 processDescription 回填
                block.setWidth(parseIntOrNull(blockProto.getWidth()));
                block.setHeight(parseIntOrNull(blockProto.getHeight()));
                block.setAlt(blockProto.getAlt());
            }
            blocks.add(block);
        }
        desc.setBlocks(blocks);
        return desc;
    }

    /**
     * 從 entity blocks + proto blocks 同步收集 import asset 對應；
     * entity blocks 為「將被回填的目標」，proto blocks 為「import_asset_id 來源」。
     * 必須兩端一一對應（同 index），否則表示資料異常。
     */
    private List<ProductImportAssetProcessor.ImportDescriptionBlock> collectDescriptionImports(
            ProductEntity.ProductDescription entityDesc,
            io.github.will413028.hawkr.product.grpc.ProductDescription protoDesc) {
        List<ProductImportAssetProcessor.ImportDescriptionBlock> out = new ArrayList<>();
        if (entityDesc == null || entityDesc.getBlocks() == null || protoDesc == null) {
            return out;
        }
        List<ProductEntity.DescriptionBlock> entityBlocks = entityDesc.getBlocks();
        List<io.github.will413028.hawkr.product.grpc.DescriptionBlock> protoBlocks = protoDesc.getBlocksList();
        int n = Math.min(entityBlocks.size(), protoBlocks.size());
        for (int i = 0; i < n; i++) {
            ProductEntity.DescriptionBlock entityBlock = entityBlocks.get(i);
            io.github.will413028.hawkr.product.grpc.DescriptionBlock protoBlock = protoBlocks.get(i);
            if (!"image".equals(entityBlock.getType())) {
                continue;
            }
            String importAssetId = protoBlock.getImportAssetId();
            if (importAssetId == null || importAssetId.isBlank()) {
                continue;
            }
            out.add(new ProductImportAssetProcessor.ImportDescriptionBlock(importAssetId, entityBlock));
        }
        return out;
    }

    private static Integer parseIntOrNull(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Integer.parseInt(raw.trim());
        } catch (NumberFormatException e) {
            return null;
        }
    }

    /**
     * 匯入的認證 proto → 收斂點輸入（19 §6.1 W5）。
     *
     * <p>⚠️ <b>刻意不在這裡驗值域</b>（舊版在此 {@code CodeMetaValidator.requireValidValue} 直接拋）：
     * 那會讓一格認證違規炸掉整筆 item 的 commit，而 §6.5.3 已把 {@code ProductPublishValidator}
     * 的兩個發出點從 {@code error()} 降為 {@code warning()}（＝不擋 commit）。留著這裡的拋例外，
     * 降級就完全沒有效果。非法輸入的處置統一由收斂點負責：剔除該筆 ＋ warn log。
     */
    private List<ProductCertificationApplicationService.CertificationInput> toCertificationInputs(
            List<io.github.will413028.hawkr.product.grpc.Certification> protoList) {
        List<ProductCertificationApplicationService.CertificationInput> result = new ArrayList<>();
        if (protoList == null) {
            return result;
        }
        for (io.github.will413028.hawkr.product.grpc.Certification cert : protoList) {
            result.add(new ProductCertificationApplicationService.CertificationInput(
                    cert.getName(), cert.getCertNo()));
        }
        return result;
    }

    private ProductSkuEntity toSkuEntity(ProductSkuImport proto, Long productId) {
        ProductSkuEntity sku = new ProductSkuEntity();
        sku.setProductId(productId);
        sku.setSkuName(proto.getSkuName());
        sku.setWeight(parseDecimalOrNull(proto.getWeight()));
        sku.setUnit(proto.getUnit());
        sku.setPrice(parseLong(proto.getPrice(), "sku.price"));
        sku.setSalePrice(proto.getSalePrice() == null || proto.getSalePrice().isBlank()
                ? null
                : parseLong(proto.getSalePrice(), "sku.sale_price"));
        sku.setStockQty(proto.getStockQty() == null || proto.getStockQty().isBlank()
                ? 0
                : (int) parseLong(proto.getStockQty(), "sku.stock_qty"));
        sku.setLengthCm(parseDecimalOrNull(proto.getLengthCm()));
        sku.setWidthCm(parseDecimalOrNull(proto.getWidthCm()));
        sku.setHeightCm(parseDecimalOrNull(proto.getHeightCm()));

        List<ProductSkuEntity.ShippingMethodItem> methods = new ArrayList<>();
        for (io.github.will413028.hawkr.product.grpc.ShippingMethod sm : proto.getShippingMethodsList()) {
            ProductSkuEntity.ShippingMethodItem item = new ProductSkuEntity.ShippingMethodItem();
            item.setType(sm.getType());
            item.setTemperature(sm.getTemperature());
            // 與 ProductSkuConverter 對齊：freeThreshold 為數字字串，直接透傳（驗證在 SQL TypeHandler 前）
            if (sm.getFreeThreshold() != null && !sm.getFreeThreshold().isBlank()) {
                // 驗證為合法數字後仍以字串落盤
                parseLong(sm.getFreeThreshold(), "sku.shipping_methods.free_threshold");
                item.setFreeThreshold(sm.getFreeThreshold());
            }
            item.setFeePayer(sm.getFeePayer());
            methods.add(item);
        }
        sku.setShippingMethod(methods);
        return sku;
    }

    private static BigDecimal parseDecimalOrNull(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return new BigDecimal(raw.trim());
        } catch (NumberFormatException e) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "數值解析失敗：" + raw);
        }
    }

    private static long parseLong(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    fieldName + " 為必填");
        }
        try {
            return Long.parseLong(raw.trim());
        } catch (NumberFormatException e) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    fieldName + " 必須為數字");
        }
    }

    private static void requirePositiveLong(String raw, String fieldName) {
        long v = parseLong(raw, fieldName);
        if (v <= 0) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    fieldName + " 必須大於 0");
        }
    }

    private static void requirePositiveDecimal(String raw, String fieldName) {
        BigDecimal v = parseDecimalOrNull(raw);
        if (v == null || v.compareTo(BigDecimal.ZERO) <= 0) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                    fieldName + " 必須大於 0");
        }
    }

    /**
     * 與 {@link ProductService#updatePriceRange} 對齊的縮減版（同邏輯，獨立路徑）。
     *
     * <p>空集合分支的 {@code setXxx(null)} 不是死碼：六個欄位在 {@link ProductEntity} 上都標了
     * {@code updateStrategy = IGNORED}，null 才寫得進 DB（#2546）。兩條路徑共用同一個 entity，
     * 所以策略修一次即同時生效——但不要單邊移除這幾行，那會讓兩條路徑語意分岔。
     */
    private void updatePriceRange(ProductEntity entity, List<ProductSkuEntity> skus) {
        if (skus == null || skus.isEmpty()) {
            entity.setPriceMin(null);
            entity.setPriceMax(null);
            entity.setListPriceMin(null);
            entity.setListPriceMax(null);
            entity.setSalePriceMin(null);
            entity.setSalePriceMax(null);
            return;
        }
        Long listMin = skus.stream().map(ProductSkuEntity::getPrice).filter(Objects::nonNull).min(Long::compareTo).orElse(null);
        Long listMax = skus.stream().map(ProductSkuEntity::getPrice).filter(Objects::nonNull).max(Long::compareTo).orElse(null);
        Long saleMin = skus.stream().map(ProductSkuEntity::getSalePrice).filter(Objects::nonNull).min(Long::compareTo).orElse(null);
        Long saleMax = skus.stream().map(ProductSkuEntity::getSalePrice).filter(Objects::nonNull).max(Long::compareTo).orElse(null);
        Long effMin = skus.stream()
                .map(s -> s.getSalePrice() != null ? s.getSalePrice() : s.getPrice())
                .filter(Objects::nonNull).min(Long::compareTo).orElse(null);
        Long effMax = skus.stream()
                .map(s -> s.getSalePrice() != null ? s.getSalePrice() : s.getPrice())
                .filter(Objects::nonNull).max(Long::compareTo).orElse(null);
        entity.setPriceMin(effMin);
        entity.setPriceMax(effMax);
        entity.setListPriceMin(listMin);
        entity.setListPriceMax(listMax);
        entity.setSalePriceMin(saleMin);
        entity.setSalePriceMax(saleMax);
    }

    /** 與 {@link ProductService#enrichSearchableFields} 對齊的縮減版（同邏輯，獨立路徑）。 */
    private void enrichSearchableFields(ProductEntity entity,
                                        List<HashtagService.UpsertedHashtag> hashtags,
                                        List<ProductSkuEntity> skus) {
        Set<Long> ids = new HashSet<>();
        if (entity.getCategoryLv1() != null) ids.add(entity.getCategoryLv1());
        if (entity.getCategoryLv2() != null) ids.add(entity.getCategoryLv2());
        if (entity.getCategory() != null) ids.add(entity.getCategory());
        Map<Long, String> nameMap = ids.isEmpty() ? new HashMap<>() : categoryService.getCategoryNames(ids);
        entity.setCatL1Name(nameMap.get(entity.getCategoryLv1()));
        entity.setCatL2Name(nameMap.get(entity.getCategoryLv2()));
        entity.setCatL3Name(nameMap.get(entity.getCategory()));

        String pureDesc = io.github.will413028.hawkr.product.domain.text.DescriptionTextExtractor.extract(entity.getDescription());
        entity.setPureDescription(pureDesc);

        List<String> hashtagCanonicals = (hashtags == null) ? List.of()
                : hashtags.stream().map(HashtagService.UpsertedHashtag::canonical).toList();
        List<String> skuNames = (skus == null) ? List.of()
                : skus.stream().map(ProductSkuEntity::getSkuName).filter(Objects::nonNull).toList();
        String searchable = io.github.will413028.hawkr.product.domain.text.SearchableTextBuilder.build(
                entity.getId(),
                entity.getName(),
                hashtagCanonicals,
                skuNames,
                entity.getCatL1Name(),
                entity.getCatL2Name(),
                entity.getCatL3Name(),
                pureDesc);
        entity.setSearchableText(searchable);
    }

    // ==================== Response DTO（給 IdempotencyService 用） ====================

    /**
     * IdempotencyService 透過 JSON 序列化快取此結果；欄位順序與 setter 保持與 Jackson 預設相容。
     */
    public static class CreateProductFromImportInternalResponseDTO {
        private String productId;
        private String idempotencyStatus;
        private String createdAt;
        private String status;

        public CreateProductFromImportInternalResponseDTO() {
        }

        public CreateProductFromImportInternalResponseDTO(String productId,
                                                          String idempotencyStatus,
                                                          String createdAt,
                                                          String status) {
            this.productId = productId;
            this.idempotencyStatus = idempotencyStatus;
            this.createdAt = createdAt;
            this.status = status;
        }

        public String getProductId() { return productId; }
        public void setProductId(String productId) { this.productId = productId; }
        public String getIdempotencyStatus() { return idempotencyStatus; }
        public void setIdempotencyStatus(String idempotencyStatus) { this.idempotencyStatus = idempotencyStatus; }
        public String getCreatedAt() { return createdAt; }
        public void setCreatedAt(String createdAt) { this.createdAt = createdAt; }
        public String getStatus() { return status; }
        public void setStatus(String status) { this.status = status; }
    }
}
