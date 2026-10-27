package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.grpc.DelegationContextValidator;
import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.product.exception.ProductAccessDeniedException;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.dto.request.*;
import io.github.will413028.hawkr.product.dto.response.*;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.entity.projection.SkuNameProjection;
import io.github.will413028.hawkr.product.exception.ProductNotFoundException;
import io.github.will413028.hawkr.product.exception.ProductPublicNotFoundException;
import io.github.will413028.hawkr.common.utils.shipping.SellerShippingFilter;
import io.github.will413028.hawkr.common.utils.shipping.WeightConverter;
import io.github.will413028.hawkr.shipping.grpc.GetSellerShippingConfigResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingConfigItem;
import io.github.will413028.hawkr.product.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.ValidateProductShippingResponse;
import io.github.will413028.hawkr.shipping.grpc.ValidateShippingItem;
import io.github.will413028.hawkr.shipping.grpc.ValidateShippingResult;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品核心業務服務
 */
@Service
public class ProductService {

    private static final ObsLogger log = ObsLogger.forClass(ProductService.class);

    /** cascade reflow 單筆的 CAS 嘗試上限（重讀重算一次；理由見 {@code reflowOneSearchable}）。 */
    private static final int REFLOW_CAS_ATTEMPTS = 2;

    private final ProductRepository productRepository;
    private final ProductSkuRepository productSkuRepository;
    private final ProductEntityConverter productEntityConverter;
    private final ProductSkuConverter productSkuConverter;
    private final IdempotencyService idempotencyService;
    private final MediaGrpcClient mediaGrpcClient;
    private final ProductCacheService cacheService;
    private final CategoryService categoryService;
    private final SellerGrpcClient sellerGrpcClient;
    private final ShippingGrpcClient shippingGrpcClient;
    private final HashtagService hashtagService;
    private final ProductCascadeService productCascadeService;
    private final SellerInfoAssembler sellerInfoAssembler;
    private final io.github.will413028.hawkr.product.grpc.client.BuyerGrpcClient buyerGrpcClient;
    private final io.github.will413028.hawkr.product.mq.ProductEventPublisher productEventPublisher;
    private final ProductRevalidationService productRevalidationService;
    private final io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker mediaReferenceLinker;
    /**
     * 認證申請收斂點（19 號設計書 §6.7）。<b>獨立 bean，跨 bean 呼叫</b>——寫成本類別的私有
     * {@code @Transactional} 方法會讓 Spring proxy 不攔截、事務完全失效而編譯測試皆不報錯。
     */
    private final ProductCertificationApplicationService certificationApplicationService;
    private final io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;

    public ProductService(
            ProductRepository productRepository,
            ProductSkuRepository productSkuRepository,
            ProductEntityConverter productEntityConverter,
            ProductSkuConverter productSkuConverter,
            IdempotencyService idempotencyService,
            MediaGrpcClient mediaGrpcClient,
            ProductCacheService cacheService,
            CategoryService categoryService,
            SellerGrpcClient sellerGrpcClient,
            ShippingGrpcClient shippingGrpcClient,
            HashtagService hashtagService,
            ProductCascadeService productCascadeService,
            SellerInfoAssembler sellerInfoAssembler,
            io.github.will413028.hawkr.product.grpc.client.BuyerGrpcClient buyerGrpcClient,
            io.github.will413028.hawkr.product.mq.ProductEventPublisher productEventPublisher,
            ProductRevalidationService productRevalidationService,
            io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker mediaReferenceLinker,
            ProductCertificationApplicationService certificationApplicationService,
            io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository
    ) {
        this.productRepository = productRepository;
        this.productSkuRepository = productSkuRepository;
        this.productEntityConverter = productEntityConverter;
        this.productSkuConverter = productSkuConverter;
        this.idempotencyService = idempotencyService;
        this.mediaGrpcClient = mediaGrpcClient;
        this.cacheService = cacheService;
        this.categoryService = categoryService;
        this.sellerGrpcClient = sellerGrpcClient;
        this.shippingGrpcClient = shippingGrpcClient;
        this.hashtagService = hashtagService;
        this.productCascadeService = productCascadeService;
        this.sellerInfoAssembler = sellerInfoAssembler;
        this.buyerGrpcClient = buyerGrpcClient;
        this.productEventPublisher = productEventPublisher;
        this.productRevalidationService = productRevalidationService;
        this.mediaReferenceLinker = mediaReferenceLinker;
        this.certificationApplicationService = certificationApplicationService;
        this.storeExposureRepository = storeExposureRepository;
    }

    /** 商品所屬店家目前是否可曝光（{@code store_exposure}，沒有列 = false）。 */
    private boolean isStoreExposable(ProductEntity entity) {
        return storeExposureRepository.isExposable(entity.getStoreId());
    }

    /**
     * Frontend SSR cache invalidation tag helpers（後端服務責任邊界規範 §9.3）。
     * caller 須在 mutation 結束前呼叫，tx commit 後 fire-and-forget HTTP POST。
     */
    // frontend cache revalidate 統一走 ProductRevalidationService（§9，避免各處重複組 tag）
    // 共發 seller:<id> 的判準＝該 mutation 是否改變「賣家有哪些公開商品」（改變才共發）。
    private void revalidateProductAndSeller(Long productId, Long sellerId) {
        productRevalidationService.revalidateProductAndSeller(productId, sellerId);
    }

    /**
     * {@code CertificationDTO} → 收斂點輸入（19 號設計書 §6.3）。
     *
     * <p>不做任何驗證與過濾：非法輸入的處置（剔除該筆 ＋ warn ＋ 落進 warnings）是收斂點的職責，
     * 在這裡先擋掉會讓 warning 少一筆而賣家什麼都不知道。
     * {@code null} 原樣傳下去 —— 它與空清單在 Step 0 閘門的意義不同。
     */
    private List<ProductCertificationApplicationService.CertificationInput> toCertificationInputs(
            List<io.github.will413028.hawkr.product.dto.request.CertificationDTO> certifications) {
        if (certifications == null) {
            return null;
        }
        List<ProductCertificationApplicationService.CertificationInput> inputs =
                new ArrayList<>(certifications.size());
        for (io.github.will413028.hawkr.product.dto.request.CertificationDTO cert : certifications) {
            if (cert == null) {
                continue;
            }
            inputs.add(new ProductCertificationApplicationService.CertificationInput(
                    cert.getName(), cert.getCertNo()));
        }
        return inputs;
    }

    /**
     * 認證申請的 {@code source}（19 §4.1）：賣家自助與 ADM 代行是<b>不同的人</b>做同一件事，
     * 稽核必須分得出來。代行判定的唯一依據是 proto 的 {@code delegation} 語境有沒有帶。
     */
    private String certificationSourceOf(boolean delegated) {
        return delegated
                ? io.github.will413028.hawkr.common.utils.code.ProductCertificationSource.DELEGATED_ADMIN.value()
                : io.github.will413028.hawkr.common.utils.code.ProductCertificationSource.SELLER.value();
    }

    /**
     * 收斂點跑完後把 {@code certifications} 投影與 {@code version} 讀回記憶體中的 entity。
     *
     * <p>收斂點可能在同一交易內自動核准並重建投影，而重建走
     * {@code ProductRepository.updateCertificationProjection}（手寫 {@code version = version + 1}）
     * ⇒ 不讀回的話回應 DTO 會帶著<b>舊投影 ＋ 落後一版的 version</b>，
     * 前端拿它當下次 {@code expectedVersion} 會直接撞 409。
     */
    private void refreshCertificationProjection(ProductEntity entity) {
        productRepository.findByIdIncludingDeleted(entity.getId()).ifPresent(fresh -> {
            entity.setCertifications(fresh.getCertifications());
            entity.setVersion(fresh.getVersion());
        });
    }

    private void revalidateSeller(Long sellerId) {
        productRevalidationService.revalidateSeller(sellerId);
    }

    private void revalidateProduct(Long productId) {
        productRevalidationService.revalidateProduct(productId);
    }

    // 商品狀態變更是否跨 ACTIVE 邊界（上架↔下架）→ 決定賣家公開商品集合是否變動。
    private static boolean crossesActiveBoundary(String oldStatus, String newStatus) {
        boolean wasActive = ProductStatus.ACTIVE.value().equals(oldStatus);
        boolean nowActive = ProductStatus.ACTIVE.value().equals(newStatus);
        return wasActive != nowActive;
    }

    /**
     * 補貨 transition 偵測 + afterCommit 發布 product.restock（02 §5.13b）。
     * <p>
     * 條件（兩個皆需）：
     * <ol>
     *   <li>oldStatus != ACTIVE(4004) AND newStatus == ACTIVE(4004) — 上架 transition</li>
     *   <li>當下 stockQty &gt; 0 — 上架當下有庫存</li>
     * </ol>
     * <p>
     * stockQty 必須**重新從 DB 抓取**（不可用 entity.getStockQty()）：
     * 因 cascadeProductStock 由其他 tx 並行更新（restoreStock / OrderCancelledConsumer
     * 等路徑），caller 端 entity 的 stockQty 是 SELECT 當下快照可能 stale；
     * 此處 read-committed 隔離下 findById 會看到本 tx + 已 commit 的並行 tx，
     * 涵蓋「cascade 已補貨但 entity 還是 0」的競態（review #942）。
     * <p>
     * 若軟刪商品（deletedAt != null）則跳過。
     */
    private void detectAndPublishRestockOnStatusChange(ProductEntity entity, String oldStatus, String newStatus) {
        if (!ProductStatus.ACTIVE.value().equals(newStatus)) return;
        if (ProductStatus.ACTIVE.value().equals(oldStatus)) return;
        if (entity.getDeletedAt() != null) return;

        // 重新抓 fresh entity — 取得 stockQty、name、gallery、storeId 的最新值。
        // 不能只 trust entity（caller-snapshot）的 name/gallery/storeId：
        // 並行 admin tx 可能在 findById 與本 publish 之間改名 / 換主圖（review #942）。
        ProductEntity fresh = productRepository.findById(entity.getId()).orElse(null);
        if (fresh == null || fresh.getStockQty() == null || fresh.getStockQty() <= 0L) return;

        String objectKey = (fresh.getGallery() != null && !fresh.getGallery().isEmpty())
                ? fresh.getGallery().get(0).getObjectKey()
                : null;
        // imageKey 帶商品主圖 objectKey（49 §6.3，顯示 URL 由消費端組）；無圖則為空字串。
        String imageKey = (objectKey == null || objectKey.isEmpty()) ? "" : objectKey;

        io.github.will413028.hawkr.product.mq.dto.ProductRestockPayload payload =
                new io.github.will413028.hawkr.product.mq.dto.ProductRestockPayload(
                        io.github.will413028.hawkr.common.obs.util.IdGenerator.uuidV4Lower(),
                        io.github.will413028.hawkr.common.utils.code.ProductEventType.PRODUCT_RESTOCK.value(),
                        String.valueOf(fresh.getId()),
                        fresh.getName(),
                        imageKey,
                        fresh.getStoreId(),
                        io.github.will413028.hawkr.common.utils.DateTimeUtils.formatIso8601(
                                io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc()));
        productEventPublisher.publishRestockAfterCommit(payload);
    }

    /**
     * 建立商品（含冪等性）。
     *
     * <p>相容既有呼叫端的薄 wrapper；需要 {@code certificationWarnings[]} 的呼叫端
     * （gRPC {@code CreateProduct}）走 {@link #createProductWithWarnings}。
     */
    @Transactional
    public ProductDetailDTO createProduct(CreateProductDTO dto) {
        return createProductWithWarnings(dto).detail();
    }

    /**
     * 建立商品（含冪等性），並回傳本次未被採用的認證（19 號設計書 §9.5）。
     */
    @Transactional
    public io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome createProductWithWarnings(CreateProductDTO dto) {
        String idempotencyKey = dto.getIdempotencyKey();

        // 驗證 idempotency_key 必填
        if (idempotencyKey == null || idempotencyKey.isBlank()) {
            throw new IllegalArgumentException("idempotency_key 為必填欄位");
        }

        // 計算請求內容的雜湊值
        String requestHash = idempotencyService.calculateHash(dto);

        // 檢查冪等性
        Optional<ProductDetailDTO> cached = idempotencyService.acquireLock(
                idempotencyKey,
                dto.getStoreId(),
                "CreateProduct",
                requestHash,
                ProductDetailDTO.class
        );
        if (cached.isPresent()) {
            log.info("idempotent", "返回先前結果", "key", idempotencyKey);
            // 冪等重放不重跑收斂點 ⇒ 沒有新的認證警告可回（理由見 ProductWriteOutcome javadoc）
            return io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome.of(cached.get(), List.of());
        }

        try {
            // 0. Fail-fast 驗證（必須在任何 DB INSERT / Media finalize 等外部副作用之前）：
            //    - description text-block 拒絕 HTML（02 §3.6 / 26-§2.3 R8）
            //    - hashtag 字元 / 長度 / 上限 / dedup（02 §3.9 / 26-§2.3 R9）
            //    驗證走 dto，僅純記憶體運算，避免 hashtag/HTML 違規時還跑了 INSERT product + FinalizeImages
            io.github.will413028.hawkr.product.domain.hashtag.HashtagInputValidator.requireHashtagSet(dto.getHashtags());
            validateDescriptionDtoTextBlocks(dto.getDescription());

            // 1. 建立商品主體
            ProductEntity entity = productEntityConverter.toEntity(dto);

            // 1.0 賣家停權 backstop（02 §5.10 步驟 5b / 46 §6.2，首次接線）：
            // toEntity 未指定 status 時預設落地 ACTIVE（見 ProductEntityConverter.toEntity），
            // 故 CreateProduct 與 UpdateSellerProductStatus 同樣需要此收斂點檢查。
            if (ProductStatus.ACTIVE.value().equals(entity.getStatus())) {
                requireSellerCanManageProductsForActivation(dto.getStoreId());
            }

            // 1.1 自動填入 category_lv2（從第三層分類的 parent_id 取得第二層分類 ID）
            Long categoryLv2 = categoryService.getParentId(entity.getCategory());
            if (categoryLv2 == null) {
                throw new IllegalArgumentException("分類 ID " + entity.getCategory() + " 無對應的第二層分類");
            }
            entity.setCategoryLv2(categoryLv2);

            // 1.2 自動填入 category_lv1（從第二層分類的 parent_id 取得第一層分類 ID）
            Long categoryLv1 = categoryService.getLv1Id(categoryLv2);
            if (categoryLv1 == null) {
                throw new IllegalArgumentException("第二層分類 ID " + categoryLv2 + " 無對應的第一層分類");
            }
            entity.setCategoryLv1(categoryLv1);

            productRepository.insert(entity);
            Long productId = entity.getId();

            log.info("entityCreated", "商品主體已建立", "product_id", productId.toString());

            // 2. 轉正商品展示圖片（僅 FinalizeImages，不建立引用）
            List<ProductEntity.GalleryItem> gallery = processImages(productId, dto.getImages());
            entity.setGallery(gallery);

            // 2.1 轉正描述中的圖片（僅 FinalizeImages，不建立引用）
            if (entity.getDescription() != null) {
                processDescriptionImages(productId, entity.getDescription());
            }

            // 3. 驗證 SKU 欄位合法性
            if (dto.getSkus() != null) {
                for (ProductSkuDTO skuDTO : dto.getSkus()) {
                    CodeMetaValidator.requireValidValue(WeightUnit.class, skuDTO.getUnit(), "unit");
                    if (skuDTO.getPrice() == null || skuDTO.getPrice() <= 0) {
                        throw new IllegalArgumentException("商品價格 必須大於 0");
                    }
                    // 特價校驗（02 設計書 §2.8.2）：null=無特價；非 null 必須 > 0 且 < price
                    if (skuDTO.getSalePrice() != null) {
                        if (skuDTO.getSalePrice() <= 0) {
                            throw new IllegalArgumentException("特價 必須大於 0");
                        }
                        if (skuDTO.getSalePrice() >= skuDTO.getPrice()) {
                            throw new IllegalArgumentException("特價 必須低於原價");
                        }
                    }
                    if (skuDTO.getStockQty() != null && skuDTO.getStockQty() < 0) {
                        throw new IllegalArgumentException("庫存數量 不可為負數");
                    }
                    // 重量與三個材積是必填（非 optional）。理由見 requirePositive 的註解。
                    requirePositive(skuDTO.getWeight(), "商品重量");
                    requirePositive(skuDTO.getLengthCm(), "包裝長度");
                    requirePositive(skuDTO.getWidthCm(), "包裝寬度");
                    requirePositive(skuDTO.getHeightCm(), "包裝高度");
                }
            }

            // 3.1 驗證 SKU 配送方式合法性（賣家開通狀態 + 材積/重量限制）
            validateSkuShippingMethods(dto.getStoreId(), dto.getSkus());

            // 4. 建立商品 SKU
            List<ProductSkuEntity> skus = new ArrayList<>();
            if (dto.getSkus() != null && !dto.getSkus().isEmpty()) {
                for (ProductSkuDTO skuDTO : dto.getSkus()) {
                    ProductSkuEntity sku = productSkuConverter.toEntity(skuDTO, productId);
                    productSkuRepository.insert(sku);
                    skus.add(sku);
                }
            }

            // 4. 更新價格範圍
            updatePriceRange(entity, skus);

            // 4.0a 反正規化欄位 cascade（02 §3.2 / §5.13a）：
            //      stock_qty = SUM(sku.stock_qty)、shipping_methods = SKU JSONB 聯集
            //      favorite_count 維持 DEFAULT 0（由 §5.14 FavoriteCountConsumer 後續維護）
            entity.setStockQty(io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.aggregateStock(skus));
            entity.setShippingMethods(io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.mergeShippingMethods(skus));

            // 4.1 [step 0 已 fail-fast 驗證 description HTML / hashtag 字元]
            //     Hashtag UPSERT 主表 + 套用 product_hashtag（同事務）
            //     建立時 status = ACTIVE（4004），counter delta 立即生效
            List<HashtagService.UpsertedHashtag> upserted = hashtagService.upsertHashtags(dto.getHashtags());
            boolean isActive = ProductStatus.ACTIVE.value().equals(entity.getStatus());
            hashtagService.applyProductHashtags(productId, upserted);

            // 4.3 計算 pure_description + cat_*_name + searchable_text 並寫回 entity
            enrichSearchableFields(entity, upserted, skus);

            // 5. 更新商品（含 gallery、價格範圍、cat_*_name、pure_description、searchable_text、stock_qty、shipping_methods）— 業務資料落盤
            requireInsertedRowWritten(productRepository.updateById(entity), productId);

            // 5.1 認證收斂點（19 號設計書 §6.1 W1）— 跨 bean 呼叫，join 本交易（Propagation.REQUIRED）。
            //     🔴 位置必須在步驟 5 的 updateById 之後：entity.certifications 在記憶體中是空陣列
            //     （ProductEntityConverter.toEntity 不再直寫投影），先跑收斂點的話這個空陣列會被
            //     updateById 原樣覆蓋回去，把剛自動核准的標章洗掉。
            //     建立路徑沒有 update_certifications 旗標；Step 0 由「認證清單非空」決定（新商品
            //     沒有既有 APPROVED，空清單不需要也不會撤回任何東西）。
            List<io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO> certificationWarnings =
                    certificationApplicationService.syncFromProductWrite(
                            new ProductCertificationApplicationService.CertificationSyncCommand(
                                    productId,
                                    dto.getStoreId(),
                                    // 🔴 submittedByAccountId 一律 null：CreateProductRequest 沒有
                                    // account_id 欄位，而 store_id 是營運鍵不是登入 principal
                                    // （CLAUDE.md §6 明令禁止互相頂替；兩者只在首任 grandfather
                                    // 時恰好相等 ⇒ 頂替會讓 dev 與 fixture 全綠、只有 re-key 過的
                                    // 店家錯而且沒有任何告警）。代行同理：DelegationContext 的
                                    // operator_admin_id 是 admin_accounts.id，又是另一個 id space。
                                    null,
                                    certificationSourceOf(dto.isDelegated()),
                                    false,
                                    toCertificationInputs(dto.getCertifications())));
            refreshCertificationProjection(entity);

            // 6. 業務資料寫入後、交易 commit 前建立 Media 引用（#2222）
            //    順序：FinalizeImages → DB 寫入 → LinkImages（失敗即拋 → 回滾）
            //    引用集合必須恆為「業務列指著的資產」的超集，否則 GC 會刪掉在用的圖
            linkGalleryImages(productId, gallery, AssetReferenceType.PRODUCT.value());
            linkDescriptionImages(productId, entity.getDescription(), AssetReferenceType.PRODUCT_DESC.value());

            // 7. 轉換為 DTO（含 hashtag list 填充）
            ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));
            result.setHashtags(buildHashtagItemDTOs(productId));
            applyCategoryHierarchy(result, entity);

            // 8. 標記冪等成功
            idempotencyService.markSucceeded(idempotencyKey, dto.getStoreId(), result);

            // 9. 清除快取
            cacheService.evictProductDetail(productId.toString());
            cacheService.evictProductListForWrite(dto.getStoreId());

            // 10. 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）— 新建商品加入賣家 storefront。
            //     建立即 ACTIVE（上架）→ 改變賣家公開商品集合，發完整 entity tag：product:<id> 讓新品經
            //     web expansion 進 products:list（producer 自足原則），seller:<id> 刷 storefront；
            //     DRAFT 等非公開狀態不進公開貨架，只需 seller:<id>。
            if (isActive) {
                revalidateProductAndSeller(productId, dto.getStoreId());
            } else {
                revalidateSeller(dto.getStoreId());
            }

            log.info("success", "商品建立成功，name=" + dto.getName(), "product_id", productId.toString());

            return io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome.of(result, certificationWarnings);

        } catch (Exception e) {
            // 先記錄真正的錯誤原因（包含堆疊追蹤）
            log.error("failed", "商品建立失敗，key=" + idempotencyKey + "，error=" + e.getMessage(), e);
            // markFailed 在新事務中執行（Propagation.REQUIRES_NEW）
            try {
                idempotencyService.markFailed(idempotencyKey, dto.getStoreId());
            } catch (Exception markFailedException) {
                log.error("markFailedError", "標記冪等失敗時出錯: " + markFailedException.getMessage());
            }
            throw e;
        }
    }

    /**
     * 更新商品（含冪等性）。
     *
     * <p>相容既有呼叫端的薄 wrapper；需要 {@code certificationWarnings[]} 的呼叫端
     * （gRPC {@code UpdateProduct}）走 {@link #updateProductWithWarnings}。
     */
    @Transactional
    public ProductDetailDTO updateProduct(UpdateProductDTO dto) {
        return updateProductWithWarnings(dto).detail();
    }

    /**
     * 更新商品（含冪等性），並回傳本次未被採用的認證（19 號設計書 §9.5）。
     */
    @Transactional
    public io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome updateProductWithWarnings(UpdateProductDTO dto) {
        String idempotencyKey = dto.getIdempotencyKey();

        // 驗證 idempotency_key 必填
        if (idempotencyKey == null || idempotencyKey.isBlank()) {
            throw new IllegalArgumentException("idempotency_key 為必填欄位");
        }

        // 計算請求內容的雜湊值
        String requestHash = idempotencyService.calculateHash(dto);

        // 檢查冪等性
        Optional<ProductDetailDTO> cached = idempotencyService.acquireLock(
                idempotencyKey,
                dto.getStoreId(),
                "UpdateProduct",
                requestHash,
                ProductDetailDTO.class
        );
        if (cached.isPresent()) {
            log.info("idempotent", "返回先前結果", "key", idempotencyKey);
            return io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome.of(cached.get(), List.of());
        }

        try {
            // 驗證 product_id 格式（必須為 TSID 數字字串，不可為 UUID）
            String productIdStr = dto.getProductId();
            if (productIdStr.contains("-")) {
                throw new IllegalArgumentException(
                    "product_id 必須為 TSID 數字字串格式 (例如: '123456789012345678')，不可使用 UUID 格式"
                );
            }

            Long productId;
            try {
                productId = Long.parseLong(productIdStr);
            } catch (NumberFormatException e) {
                throw new IllegalArgumentException(
                    "product_id 格式錯誤: '" + productIdStr + "'，必須為純數字字串"
                );
            }

            // 1. 查詢並驗證商品存在且屬於該帳號
            ProductEntity entity = productRepository.findByIdAndStoreId(productId, dto.getStoreId())
                    .orElseThrow(() -> new ProductNotFoundException(dto.getProductId()));

            // 1.0a 樂觀鎖前置比對（56 §6.6.3）：早退避開後續 media link / SKU 寫入等副作用。
            //      真正的 CAS 由 updateById 的 OptimisticLockerInnerInterceptor 執行（見步驟 6）。
            requireExpectedVersion(entity, dto.getExpectedVersion());

            // 1.1 編輯狀態守門（02 設計書 §2.8.5）：商品必須為 DRAFT(4001) 或 INACTIVE(4003) 才允許編輯
            // 上下架狀態轉換走 updateSellerProductStatus，不受本守門影響
            String currentStatus = entity.getStatus();
            if (!ProductStatus.DRAFT.value().equals(currentStatus)
                    && !ProductStatus.INACTIVE.value().equals(currentStatus)) {
                // 02 §2.8.5：用 PRODUCT_EDIT_REQUIRES_INACTIVE (3019) 業務碼回應「狀態不允許」
                throw new ProductBusinessException(
                        GrpcResponseCode.PRODUCT_EDIT_REQUIRES_INACTIVE,
                        String.format(GrpcResponseCode.PRODUCT_EDIT_REQUIRES_INACTIVE.message(), currentStatus));
            }

            // 1.2 Fail-fast 驗證（02 §3.6 / §3.9）：在 image finalize / SKU 更新等外部副作用前先驗
            //     description text-block 拒絕 HTML / hashtag 字元 + 長度 + 上限（僅當 update_hashtags=true 才驗 hashtag）
            validateDescriptionDtoTextBlocks(dto.getDescription());
            if (dto.isUpdateHashtags()) {
                io.github.will413028.hawkr.product.domain.hashtag.HashtagInputValidator.requireHashtagSet(dto.getHashtags());
            }

            // 保存舊的 gallery 和 description（必須在 updateEntity 之前，
            // 因為 updateEntity 會修改 entity 的 description，導致 oldDescription 被覆蓋）
            List<ProductEntity.GalleryItem> oldGallery = entity.getGallery() != null
                    ? new ArrayList<>(entity.getGallery()) : new ArrayList<>();
            ProductEntity.ProductDescription oldDescription = entity.getDescription();

            // 2. 更新商品主體
            productEntityConverter.updateEntity(entity, dto);

            // 2.1 若分類有變更，同步更新 category_lv2 和 category_lv1
            if (dto.getCategoryId() != null) {
                Long categoryLv2 = categoryService.getParentId(entity.getCategory());
                if (categoryLv2 == null) {
                    throw new IllegalArgumentException("分類 ID " + entity.getCategory() + " 無對應的第二層分類");
                }
                entity.setCategoryLv2(categoryLv2);

                Long categoryLv1 = categoryService.getLv1Id(categoryLv2);
                if (categoryLv1 == null) {
                    throw new IllegalArgumentException("第二層分類 ID " + categoryLv2 + " 無對應的第一層分類");
                }
                entity.setCategoryLv1(categoryLv1);
            }

            // 3. 處理商品展示圖片更新（僅轉正，不建立/解除引用）
            if (dto.getImages() != null) {
                List<ProductEntity.GalleryItem> newGallery = processImageUpdate(
                        productId,
                        oldGallery,
                        dto.getImages()
                );
                entity.setGallery(newGallery);
            }

            // 3.1 處理描述中的圖片更新（僅轉正，不建立/解除引用）
            if (dto.getDescription() != null) {
                ProductEntity.ProductDescription newDescription = productEntityConverter.convertDescriptionToEntity(dto.getDescription());
                processDescriptionImageUpdate(productId, oldDescription, newDescription);
                entity.setDescription(newDescription);
            }

            // 4. 驗證 SKU 欄位合法性（更新時選填，有傳才驗證）
            if (dto.getSkus() != null) {
                for (ProductSkuDTO skuDTO : dto.getSkus()) {
                    CodeMetaValidator.validateValueIfPresent(WeightUnit.class, skuDTO.getUnit(), "unit");
                    if (skuDTO.getPrice() != null && skuDTO.getPrice() <= 0) {
                        throw new IllegalArgumentException("商品價格 必須大於 0");
                    }
                    // 特價校驗（02 設計書 §2.8.2）：null=清空特價；非 null 必須 > 0 且 < price
                    // 注意 price 可能由本次更新帶入（skuDTO.getPrice()）或來自 DB 既有值
                    if (skuDTO.getSalePrice() != null) {
                        if (skuDTO.getSalePrice() <= 0) {
                            throw new IllegalArgumentException("特價 必須大於 0");
                        }
                        // 比對的 price：優先用本次 DTO 的 price，否則由 service 層在 updateSkus 套用既有 SKU price
                        // 此處先做基礎校驗（> 0）；< price 的最終檢查在 updateSkus 內 sku entity 落地前確認
                        if (skuDTO.getPrice() != null && skuDTO.getSalePrice() >= skuDTO.getPrice()) {
                            throw new IllegalArgumentException("特價 必須低於原價");
                        }
                    }
                    if (skuDTO.getStockQty() != null && skuDTO.getStockQty() < 0) {
                        throw new IllegalArgumentException("庫存數量 不可為負數");
                    }
                    // 重量與三個材積是必填（非 optional）。理由見 requirePositive 的註解。
                    requirePositive(skuDTO.getWeight(), "商品重量");
                    requirePositive(skuDTO.getLengthCm(), "包裝長度");
                    requirePositive(skuDTO.getWidthCm(), "包裝寬度");
                    requirePositive(skuDTO.getHeightCm(), "包裝高度");
                }
            }

            // 4.1 驗證 SKU 配送方式合法性（賣家開通狀態 + 材積/重量限制）
            validateSkuShippingMethods(dto.getStoreId(), dto.getSkus());

            // 5. 處理 SKU 更新
            List<ProductSkuEntity> skus = updateSkus(productId, dto.getSkus());

            // 5. 更新價格範圍
            updatePriceRange(entity, skus);

            // 5.0a 反正規化欄位 cascade 重算（02 §3.2 / §5.13a）：
            //      stock_qty = SUM(sku.stock_qty)（依最終 SKU 集合，含新增 / 刪除後）
            //      shipping_methods = SKU JSONB 聯集（同組合 freeThreshold 取 MIN、feePayer 取 sku.id ASC）
            //      favorite_count 不在 UpdateProduct 維護（由 §5.14 FavoriteCountConsumer 維護）
            //
            // stock_qty 只在請求確實帶了 skus 時才寫回。沒帶 skus 的純商品編輯（改名稱 / 描述 /
            // 分類）在 updateSkus 是 no-op，這裡的 skus 就是交易開頭 findByProductId 讀到的快照；
            // 照寫等於把那個快照的加總原樣覆蓋回 product.stock_qty。而庫存異動走
            // reaggregateStockFromSku，它刻意不 bump product.version（56 §5.4 F41 第 11 列，已登記在
            // scripts/check-version-bump-coverage.py 的豁免清單）⇒ 步驟 6 的 product 層 CAS 對
            // 「表單送出期間發生的庫存異動」完全失明，該異動會被靜默覆寫回舊值；窗口是
            // findByProductId → updateById，中間還隔著 hashtag upsert 的 DB 往返。
            // SKU 才是庫存的真實來源（結帳的 deductStock 帶 stock_qty >= quantity 述詞，不會超賣），
            // 但 product.stock_qty 是 ProductPublicationPolicy.isPurchasable 的判準之一，覆寫後
            // 商品的可購買性會與實際庫存不符，直到下一次任何庫存異動把它重算回來。
            // 設成 null 讓 MyBatis-Plus 預設的 NOT_NULL 更新策略把該欄整個排除在 UPDATE 之外
            // （stockQty 沒有 @TableField，沿用全域預設）；回應 DTO 要用的值在落盤後補回。
            boolean skusProvided = dto.getSkus() != null && !dto.getSkus().isEmpty();
            long aggregatedStock = io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.aggregateStock(skus);
            entity.setStockQty(skusProvided ? aggregatedStock : null);
            entity.setShippingMethods(io.github.will413028.hawkr.product.service.helper.ProductAggregateRules.mergeShippingMethods(skus));

            // 5.1 description text-block 純文字驗證（§3.6 / 26-§2.3 R8）
            validateDescriptionTextBlocks(entity.getDescription());

            // 5.2 Hashtag 集合 diff（02 §5.2.4）：僅在 update_hashtags=true 時生效
            List<HashtagService.UpsertedHashtag> finalHashtagBindings;
            if (dto.isUpdateHashtags()) {
                List<HashtagService.UpsertedHashtag> upserted = hashtagService.upsertHashtags(dto.getHashtags());
                boolean isActive = ProductStatus.ACTIVE.value().equals(entity.getStatus());
                HashtagService.HashtagApplyResult applyResult =
                        hashtagService.applyProductHashtags(productId, upserted);
                finalHashtagBindings = applyResult.finalBindings();
            } else {
                // 維持當前綁定，從 DB 讀取 canonical names 供 searchable_text 重算
                List<HashtagService.HashtagDisplayItem> current = hashtagService.findHashtagsByProductId(productId);
                finalHashtagBindings = current.stream()
                        .map(h -> new HashtagService.UpsertedHashtag(null, h.canonicalName(), h.displayName()))
                        .toList();
            }

            // 5.3 重算反正規化文字欄位（cat_*_name + pure_description + searchable_text）
            enrichSearchableFields(entity, finalHashtagBindings, skus);

            // 6. 保存商品更新 — 業務資料落盤（version CAS）
            long versionBeforeUpdate = currentVersion(entity);
            requireRowsAffected(productRepository.updateById(entity), productId);
            entity.setVersion(versionBeforeUpdate + 1);
            if (!skusProvided) {
                // 步驟 5.0a 刻意讓 stock_qty 不進 UPDATE；補回既有 SKU 的加總供步驟 8 的回應 DTO
                // 使用（DB 值不受影響，與本次修改前回傳的是同一個數字）。
                entity.setStockQty(aggregatedStock);
            }

            // 6.1 認證收斂點（19 號設計書 §6.1 W2）— 跨 bean 呼叫，join 本交易。
            //     🔴 必須在步驟 6 的 updateById 之後：entity 帶的是從 DB 讀出的舊投影，
            //     先跑收斂點的話（a）投影會被舊值覆蓋回去（b）rebuildProjection 的 version+1
            //     會讓步驟 6 的 CAS 直接 0 列。
            //     🔴 Step 0 閘門條件是「旗標 OR 認證非空」（§6.3.0）；旗標與清單都原樣往下傳，
            //     在收斂點內判定，這裡不得自行提前 return。
            List<io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO> certificationWarnings =
                    certificationApplicationService.syncFromProductWrite(
                            new ProductCertificationApplicationService.CertificationSyncCommand(
                                    productId,
                                    dto.getStoreId(),
                                    null,   // 理由同 createProduct 步驟 5.1
                                    certificationSourceOf(dto.isDelegated()),
                                    dto.isUpdateCertifications(),
                                    toCertificationInputs(dto.getCertifications())));
            refreshCertificationProjection(entity);

            // 7. 業務資料寫入後、交易 commit 前建立/解除 Media 引用（#2222）
            //    Link 失敗即拋 → 回滾；Unlink 失敗只記 log（洩漏可事後清，實體刪除不能）
            linkAndUnlinkGalleryImages(productId, oldGallery, entity.getGallery());
            linkAndUnlinkDescriptionImages(productId, oldDescription, entity.getDescription());

            // 8. 轉換為 DTO（含 hashtag list 填充）
            ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));
            result.setHashtags(buildHashtagItemDTOs(productId));
            applyCategoryHierarchy(result, entity);

            // 9. 標記冪等成功
            idempotencyService.markSucceeded(idempotencyKey, dto.getStoreId(), result);

            // 10. 清除快取
            cacheService.evictProductDetail(productId.toString());
            cacheService.evictProductListForWrite(dto.getStoreId());

            // 11. 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）
            //     純欄位編輯不改變賣家公開商品集合 → 只發 product:<id>；storefront 列表同掛
            //     products:list，由 web expansion（product:<id> → products:list）涵蓋。
            revalidateProduct(productId);

            log.info("success", "商品更新成功", "product_id", productId.toString());

            return io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome.of(result, certificationWarnings);

        } catch (Exception e) {
            idempotencyService.markFailed(idempotencyKey, dto.getStoreId());
            log.error("failed", "商品更新失敗，key=" + idempotencyKey + "，error=" + e.getMessage());
            throw e;
        }
    }

    /**
     * 查詢商品詳情（含權限檢查）
     *
     * 權限規則：
     * - ACTIVE (4004) 狀態：任何人都可以查看
     * - DRAFT (4001) / PENDING_REVIEW (4002) 狀態：只有 store_id owner 可以查看
     * - INACTIVE (4003) 狀態：只有 store_id owner 可以查看
     *
     * @param productId 商品 ID
     * @param storeId 請求者的 store 代理鍵（0 或 null 表示匿名）
     * @return 商品詳情 DTO
     * @throws ProductNotFoundException 商品不存在
     * @throws ProductAccessDeniedException 無權存取
     */
    public ProductDetailDTO getProduct(String productId, Long storeId) {
        return getProduct(productId, storeId, false);
    }

    /**
     * 查詢商品詳情（含權限檢查），代行路徑另加 Q4 歸屬斷言。
     *
     * @param requireOwnership true = 代行路徑：商品的 store_id 必須等於 storeId，不符回 9275。
     *                         既有述詞是「ACTIVE 商品對任何 storeId 一律放行」，代行照現況接上去
     *                         等於 A 店 credential 讀得到 B 店所有上架商品，而成功讀取不落稽核
     *                         ⇒ 零痕跡跨店讀取（56 §7.1.2 Q4）
     */
    public ProductDetailDTO getProduct(String productId, Long storeId, boolean requireOwnership) {
        // 1. 查詢資料庫（不使用快取，因為需要檢查權限）
        Long id = Long.parseLong(productId);
        ProductEntity entity = productRepository.findById(id)
                .orElseThrow(() -> new ProductNotFoundException(productId));

        // 1.1 Q4 歸屬斷言（代行路徑）：必須在既有權限檢查之前，否則 ACTIVE 商品會被既有述詞先放行
        if (requireOwnership) {
            DelegationContextValidator.requireResourceOwned(
                    entity.getStoreId() != null ? entity.getStoreId() : 0L,
                    storeId != null ? storeId : 0L);
        }

        // 2. 權限檢查
        String status = entity.getStatus();
        boolean isOwner = storeId != null && storeId > 0 && storeId.equals(entity.getStoreId());
        boolean isActive = ProductStatus.ACTIVE.value().equals(status);

        // 賣家已刪除後又復活：舊商品對其隱形（等同不存在）。public 非-owner 由下方既有檢查擋；
        // admin 走 getProductForAdmin 不經此（Part A / doc 44 §11）。
        if (entity.getSellerDeletedAt() != null && isOwner && !isActive) {
            throw new ProductNotFoundException(productId);
        }

        // 允許查看的狀態（對於 store_id owner）：DRAFT / PENDING_REVIEW / ACTIVE / INACTIVE
        // 賣家必須能查看自己下架的商品，才能重新上架或編輯
        boolean isAllowedForOwner = ProductStatus.DRAFT.value().equals(status)
                || ProductStatus.PENDING_REVIEW.value().equals(status)
                || ProductStatus.INACTIVE.value().equals(status)
                || isActive;

        if (!isActive && !isOwner) {
            // 非 ACTIVE 狀態且不是 store_id owner，拒絕存取
            log.warn("accessDenied", "非 store_id owner 無法查看非上架商品，product_id=" + productId + "，status=" + status);
            throw new ProductAccessDeniedException(productId, "商品不存在或無權存取");
        }

        if (isOwner && !isAllowedForOwner) {
            // store_id owner 也無法查看某些狀態（例如 INACTIVE 由管理員控制）
            log.warn("accessDenied", "store_id owner 無法查看此狀態的商品，product_id=" + productId + "，status=" + status);
            throw new ProductAccessDeniedException(productId, "商品不存在或無權存取");
        }

        // 3. 查詢 SKU
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(id);

        // 4. 轉換為 DTO
        ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));

        // 5. 填充三層分類（不限狀態，含軟刪除分類；§2.5.2 買家破口修復）
        if (entity.getCategory() != null) {
            CategoryHierarchyDTO hierarchy = categoryService.getCategoryHierarchy(
                    entity.getCategory(), entity.getCategoryLv2());
            if (hierarchy != null) {
                result.setCategoryName(hierarchy.getLv3Name()); // 向後相容
                result.setCategoryLv3Id(hierarchy.getLv3Id());
                result.setCategoryLv3Name(hierarchy.getLv3Name());
                result.setCategoryLv2Id(hierarchy.getLv2Id());
                result.setCategoryLv2Name(hierarchy.getLv2Name());
                result.setCategoryLv1Id(hierarchy.getLv1Id());
                result.setCategoryLv1Name(hierarchy.getLv1Name());
            }
        }

        // 6. 填充商家資訊（從 Seller Service 取得 name/rate/logo/address）
        SellerInfoDTO sellerInfo = buildSellerInfo(entity.getStoreId());
        result.setSeller(sellerInfo);

        // 7. 填充評論預覽（預留呼叫點）
        result.setReviews(new ArrayList<>());
        // TODO: 接通 Review Service 後取得前兩筆評論

        // 8. 填充 hashtag list（依 position 升序，含讀取時計算的公開商品數；02 §5.4）
        result.setHashtags(buildHashtagItemDTOs(id));

        log.info("success", "查詢商品成功，product_id=" + productId + "，store_id=" + storeId);

        return result;
    }

    /**
     * 填充三層分類（不限狀態，含軟刪除分類；§2.5.2 買家破口修復）。
     *
     * 寫入路徑與讀取路徑用同一條規則：建立／更新／改狀態的回應也是一份完整商品詳情，
     * 代行工作區與賣家前台都直接拿它重繪。少填的形狀是「編輯完商品，分類欄變成空的」
     * ——請求成功、code: 0、DB 也沒被改壞，只有回應少了那一欄。
     */
    private void applyCategoryHierarchy(ProductDetailDTO result, ProductEntity entity) {
        if (entity.getCategory() == null) {
            return;
        }
        CategoryHierarchyDTO hierarchy = categoryService.getCategoryHierarchy(
                entity.getCategory(), entity.getCategoryLv2());
        if (hierarchy == null) {
            return;
        }
        result.setCategoryName(hierarchy.getLv3Name()); // 向後相容
        result.setCategoryLv3Id(hierarchy.getLv3Id());
        result.setCategoryLv3Name(hierarchy.getLv3Name());
        result.setCategoryLv2Id(hierarchy.getLv2Id());
        result.setCategoryLv2Name(hierarchy.getLv2Name());
        result.setCategoryLv1Id(hierarchy.getLv1Id());
        result.setCategoryLv1Name(hierarchy.getLv1Name());
    }

    /**
     * 管理員取得商品詳情（不檢查擁有者，不限狀態）
     *
     * 管理員可查看任何商品的完整詳情，包含所有狀態（草稿、審核中、下架、上架）。
     * 不使用快取，確保管理員看到即時資料。
     *
     * @param productId 商品 ID（TSID 數字字串）
     * @return 商品詳情 DTO
     * @throws ProductNotFoundException 商品不存在
     */
    public ProductDetailDTO getProductForAdmin(String productId) {
        // 1. 查詢資料庫（不使用快取，管理員需看到即時資料）
        Long id = Long.parseLong(productId);
        ProductEntity entity = productRepository.findById(id)
                .orElseThrow(() -> new ProductNotFoundException(productId));

        // 2. 不做權限檢查、不限狀態

        // 3. 查詢 SKU
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(id);

        // 4. 轉換為 DTO
        ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));

        // 5. 填充三層分類（不限狀態，含軟刪除分類）
        if (entity.getCategory() != null) {
            CategoryHierarchyDTO hierarchy = categoryService.getCategoryHierarchy(
                    entity.getCategory(), entity.getCategoryLv2());
            if (hierarchy != null) {
                result.setCategoryName(hierarchy.getLv3Name());
                result.setCategoryLv3Id(hierarchy.getLv3Id());
                result.setCategoryLv3Name(hierarchy.getLv3Name());
                result.setCategoryLv2Id(hierarchy.getLv2Id());
                result.setCategoryLv2Name(hierarchy.getLv2Name());
                result.setCategoryLv1Id(hierarchy.getLv1Id());
                result.setCategoryLv1Name(hierarchy.getLv1Name());
            }
        }

        // 6. 填充商家資訊
        SellerInfoDTO sellerInfo = SellerInfoDTO.builder()
                .storeId(entity.getStoreId())
                .build();
        result.setSeller(sellerInfo);

        // 7. 填充評論預覽（預留呼叫點）
        result.setReviews(new ArrayList<>());

        // 8. 填充 hashtag list（02 §5.4）
        result.setHashtags(buildHashtagItemDTOs(id));

        log.info("success", "管理員查詢商品成功，product_id=" + productId);

        return result;
    }

    /**
     * 公開商品詳情（PUBLIC 或 TEMPORARILY_UNAVAILABLE，支援 Redis 快取）
     *
     * 供買家或匿名使用者瀏覽，不做權限檢查；從未公開或已移除商品回 NotFound。
     * 使用 Cache-Aside 模式：先查 Redis，未命中再查 DB 並回寫快取。
     *
     * @param productId 商品 ID（TSID 數字字串）
     * @return 商品詳情 DTO
     * @throws ProductNotFoundException 商品不存在或非 ACTIVE 狀態
     */
    public ProductDetailDTO getPublicProduct(String productId) {
        // 0. 先查 DB 取得 seller，計算 onboarding 指紋（cache key 用，02 §2.7.2）
        Long id = Long.parseLong(productId);
        Optional<ProductEntity> found = productRepository.findByIdIncludingDeleted(id);
        if (found.isEmpty()) {
            throw new ProductPublicNotFoundException(
                    productId, io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.PRODUCT_NOT_FOUND);
        }
        ProductEntity entity = found.get();

        // 0.1 公開生命週期獨立於銷售狀態：曾公開後暫時下架仍保留穩定 PDP。
        var publicationState = io.github.will413028.hawkr.product.domain.ProductPublicationPolicy.deriveState(
                entity.getFirstPublishedAt(), entity.getStatus(),
                entity.getDeletedAt(), entity.getSellerDeletedAt(), isStoreExposable(entity));
        io.github.will413028.hawkr.product.domain.ProductPublicVisibilityPolicy
                .notFoundCode(true, publicationState)
                .ifPresent(code -> { throw new ProductPublicNotFoundException(productId, code); });

        Long storeId = entity.getStoreId();
        OnboardingSnapshot onboarding = fetchOnboardingSnapshot(storeId);
        String onboardingSig = SellerShippingFilter.onboardingSig(storeId, onboarding.availableKeys);

        // 1. 用 (productId, onboardingSig) 查 Redis 快取
        Optional<ProductDetailDTO> cached = cacheService.getProductDetail(productId, onboardingSig);
        if (cached.isPresent()) {
            return cached.get();
        }

        // 4. 查詢 SKU
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(id);

        // 5. 轉換為 DTO（含 product 原始 shipping_methods）
        ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));

        // 5.1 套用 onboarding filter（02 §2.7.2）：剔除賣家當下未啟用的 shipping methods
        applyOnboardingFilterToDetail(result, onboarding.availableKeys);

        // 6. 填充三層分類（不限狀態，§2.5.2 買家破口修復）
        if (entity.getCategory() != null) {
            CategoryHierarchyDTO hierarchy = categoryService.getCategoryHierarchy(entity.getCategory(), entity.getCategoryLv2());
            if (hierarchy != null) {
                result.setCategoryName(hierarchy.getLv3Name()); // 向後相容
                result.setCategoryLv3Id(hierarchy.getLv3Id());
                result.setCategoryLv3Name(hierarchy.getLv3Name());
                result.setCategoryLv2Id(hierarchy.getLv2Id());
                result.setCategoryLv2Name(hierarchy.getLv2Name());
                result.setCategoryLv1Id(hierarchy.getLv1Id());
                result.setCategoryLv1Name(hierarchy.getLv1Name());
            }
        }

        // 7. 填充商家資訊（從 Seller Service 取得 name/rate/logo/address）
        SellerInfoDTO sellerInfo = buildSellerInfo(entity.getStoreId());
        result.setSeller(sellerInfo);

        // 8. 填充評論預覽（預留呼叫點）
        result.setReviews(new ArrayList<>());
        // TODO: 接通 Review Service 後取得前兩筆評論

        // 8.1 填充 hashtag list（02 §5.5；快取會包含此 list，TTL 1h）
        result.setHashtags(buildHashtagItemDTOs(id));

        // 9. 寫入 Redis 快取（含 onboarding 指紋）
        cacheService.cacheProductDetail(productId, onboardingSig, result);

        log.info("success", "公開查詢商品成功，product_id=" + productId);

        return result;
    }

    // ==================== 02 §2.7.2 onboarding-aware read path 共用 ====================

    /**
     * 賣家 onboarding 狀態快照（per-request 用，包含已啟用方式 key set + 指紋）。
     */
    private static final class OnboardingSnapshot {
        final Set<String> availableKeys;
        OnboardingSnapshot(Set<String> availableKeys) {
            this.availableKeys = availableKeys;
        }
    }

    /**
     * 從 shipping-service 取得單一 seller 當下已啟用配送方式集合（02 §2.7.2）。
     *
     * <p>fail-graceful：RPC 失敗或 code!=0 時 log.warn 並回傳「entity 原 shipping_methods 視同已啟用」
     * 的兜底集合，避免公開商品頁因 shipping-service 短暫故障整體掛掉（與 03 結帳 fail-close 不同）。
     */
    private OnboardingSnapshot fetchOnboardingSnapshot(long storeId) {
        try {
            GetSellerShippingConfigResponse resp = shippingGrpcClient.getSellerShippingConfig(storeId);
            if (resp.getCode() != 0) {
                log.warn("fetchOnboardingSnapshot.nonZeroCode",
                        "shipping-service 回應非零碼 storeId=" + storeId
                                + " code=" + resp.getCode() + "；degrade to permissive set");
                return permissiveSnapshot();
            }
            Set<String> available = new HashSet<>();
            for (SellerShippingConfigItem cfg : resp.getConfigsList()) {
                if (cfg.getAvailable()) {
                    available.add(SellerShippingFilter.toMethodKey(cfg.getShippingMethod(), cfg.getGoodsType()));
                }
            }
            return new OnboardingSnapshot(available);
        } catch (Exception e) {
            log.warn("fetchOnboardingSnapshot.failed",
                    "shipping-service 取得失敗 storeId=" + storeId
                            + " err=" + e.getMessage() + "；degrade to permissive set");
            return permissiveSnapshot();
        }
    }

    /**
     * 兜底集合：含特殊標記 "*:*" 讓 filter 全放行（shipping-service 不可達時不阻擋公開瀏覽）。
     */
    private OnboardingSnapshot permissiveSnapshot() {
        return new OnboardingSnapshot(Set.of("*:*"));
    }

    /**
     * 套用 onboarding filter 至 ProductDetailDTO 的 shippingMethods 與 deliveryTemp。
     */
    private void applyOnboardingFilterToDetail(ProductDetailDTO dto, Set<String> availableKeys) {
        if (availableKeys.contains("*:*")) {
            return; // 兜底放行，shipping-service 故障時不過濾
        }
        if (dto.getShippingMethods() == null || dto.getShippingMethods().isEmpty()) {
            return;
        }
        List<ShippingMethodDTO> kept = new ArrayList<>();
        Set<String> tempUnion = new LinkedHashSet<>();
        for (ShippingMethodDTO sm : dto.getShippingMethods()) {
            if (SellerShippingFilter.isAvailable(sm.getType(), sm.getTemperature(), availableKeys)) {
                kept.add(sm);
                if (sm.getTemperature() != null && !sm.getTemperature().isEmpty()) {
                    tempUnion.add(sm.getTemperature());
                }
            }
        }
        dto.setShippingMethods(kept);
        dto.setDeliveryTemp(new ArrayList<>(tempUnion));
    }

    /**
     * 套用 onboarding filter 至 ProductListItemDTO 的 shippingMethods 與 deliveryTemp。
     */
    private void applyOnboardingFilterToListItem(ProductListItemDTO item, Set<String> availableKeys) {
        if (availableKeys.contains("*:*")) {
            return;
        }
        if (item.getShippingMethods() == null || item.getShippingMethods().isEmpty()) {
            return;
        }
        List<io.github.will413028.hawkr.product.entity.ProductSkuEntity.ShippingMethodItem> kept = new ArrayList<>();
        Set<String> tempUnion = new LinkedHashSet<>();
        for (io.github.will413028.hawkr.product.entity.ProductSkuEntity.ShippingMethodItem sm : item.getShippingMethods()) {
            if (SellerShippingFilter.isAvailable(sm.getType(), sm.getTemperature(), availableKeys)) {
                kept.add(sm);
                if (sm.getTemperature() != null && !sm.getTemperature().isEmpty()) {
                    tempUnion.add(sm.getTemperature());
                }
            }
        }
        item.setShippingMethods(kept);
        item.setDeliveryTemp(new ArrayList<>(tempUnion));
    }

    /**
     * 查詢商品列表（禁止 join product_spec）
     *
     * 狀態篩選規則由 Gateway 入口控制：
     * - 公開 API：Gateway 強制 status=4004（ACTIVE）
     * - 賣家 API：Gateway 傳入 ownerStoreId，status 可選
     */
    public ProductListResultDTO getProductList(ProductQueryDTO query) {
        // 02 §5.3.4 入口驗證：delivery_temp_filter 合法值（docs/開發規範/Java 微服務開發規範.md §6.4）
        if (query.getDeliveryTempFilter() != null) {
            for (String v : query.getDeliveryTempFilter()) {
                io.github.will413028.hawkr.common.utils.code.CodeMetaValidator.requireValidValue(
                        io.github.will413028.hawkr.common.utils.code.ShippingTemperature.class,
                        v, "delivery_temp_filter");
            }
        }

        // 商家所在地由 seller SSOT 解析；與顯式 store_ids 同時存在時取交集。
        if (query.getSellerCityCode() != null && !query.getSellerCityCode().isBlank()) {
            List<Long> cityStoreIds = sellerGrpcClient.listPublicStoreIdsByCityCode(query.getSellerCityCode());
            List<Long> intersected = io.github.will413028.hawkr.product.domain.ProductSellerCityFilterPolicy
                    .intersect(query.getStoreIds(), cityStoreIds);
            if (intersected.isEmpty()) {
                return emptyResult(query);
            }
            query.setStoreIds(intersected);
        }

        // 02 §5.3.4 followed_product_only filter 預處理：必須有 viewerBuyerId（觀看者的 buyer_id，
        // 由 gateway 解析後傳入），呼叫 buyer 拿收藏 id list
        if (query.isFollowedProductOnly()) {
            Long viewerId = query.getViewerBuyerId();
            if (viewerId == null || viewerId <= 0) {
                throw new IllegalArgumentException("followed_product_only requires login");
            }
            List<Long> favoritedIds = buyerGrpcClient.listFavoritedProductIds(viewerId);
            if (favoritedIds == null || favoritedIds.isEmpty()) {
                // 收藏為空 → 直接回空 list，不打 DB
                log.info("followedProductOnly.emptyFavorites",
                        "followed_product_only=true 但無收藏，回空 list",
                        "viewer_buyer_id", String.valueOf(viewerId));
                return emptyResult(query);
            }
            // 02 §5.3.4 上限 5000
            if (favoritedIds.size() > 5000) {
                log.warn("followedProductOnly.truncate",
                        "favorited ids 超過 5000，截斷 viewer_buyer_id=" + viewerId
                                + ", size=" + favoritedIds.size());
                favoritedIds = favoritedIds.subList(0, 5000);
            }
            query.setFollowedProductIds(favoritedIds);
        }

        // 02 §5.3.4 followed_seller_only filter 預處理：必須有 viewerBuyerId（由 gateway 解析後傳入），
        // 呼叫 buyer 拿追蹤的 seller id list，解析後與既有 storeIds 取交集（不另開 SQL 路徑，重用
        // idx_product_status_store_id 索引）
        if (query.isFollowedSellerOnly()) {
            Long viewerId = query.getViewerBuyerId();
            if (viewerId == null || viewerId <= 0) {
                throw new IllegalArgumentException("followed_seller_only requires login");
            }
            List<Long> subscribedSellerIds = buyerGrpcClient.listSubscribedSellerIds(viewerId);
            if (subscribedSellerIds == null || subscribedSellerIds.isEmpty()) {
                log.info("followedSellerOnly.emptySubscriptions",
                        "followed_seller_only=true 但無追蹤賣家，回空 list",
                        "viewer_buyer_id", String.valueOf(viewerId));
                return emptyResult(query);
            }
            // 02 §5.3.4 上限 1000
            if (subscribedSellerIds.size() > 1000) {
                log.warn("followedSellerOnly.truncate",
                        "subscribed seller ids 超過 1000，截斷 viewer_buyer_id=" + viewerId
                                + ", size=" + subscribedSellerIds.size());
                subscribedSellerIds = subscribedSellerIds.subList(0, 1000);
            }
            // 與既有 storeIds 取交集（若 query 有指定）
            List<Long> existing = query.getStoreIds();
            if (existing != null && !existing.isEmpty()) {
                java.util.Set<Long> intersect = new java.util.HashSet<>(existing);
                intersect.retainAll(subscribedSellerIds);
                if (intersect.isEmpty()) {
                    log.info("followedSellerOnly.emptyIntersect",
                            "followed_seller_only ∩ store_ids 為空，回空 list",
                            "viewer_buyer_id", String.valueOf(viewerId));
                    return emptyResult(query);
                }
                query.setStoreIds(new java.util.ArrayList<>(intersect));
            } else {
                query.setStoreIds(subscribedSellerIds);
            }
        }

        // 直接使用 Gateway 傳入的 status（由入口控制）
        String effectiveStatus = query.getStatus();

        // 生成快取鍵
        String cacheKey = cacheService.generateListCacheKey(query);

        // 1. 先查快取（cache 存 pre-filter snapshot，public 場景仍須套 onboarding filter）
        Optional<ProductListResultDTO> cached = cacheService.getProductList(cacheKey);
        if (cached.isPresent()) {
            log.info("cacheHit", "列表快取命中", "cache_key", cacheKey);
            ProductListResultDTO cachedResult = cached.get();
            if (query.isPublicApi()) {
                applyOnboardingFilterToList(cachedResult);
            }
            return cachedResult;
        }

        // 2. 分類層級偵測（cache miss 才執行；02 §5.3.1 多值聯集）：
        //    對每個 id 偵測 level，分組為 lv1/lv2/lv3 三組；SQL 三組 IN 用 OR 串接（命中任一即可）。
        //    天然支援「遞迴含子分類」（lv1=X 匹配商品 lv1=X，含其下所有 lv2/lv3）。
        List<Long> categoryLv1Ids = null;
        List<Long> categoryLv2Ids = null;
        List<Long> categoryLv3Ids = null;
        List<Long> categoryIds = query.getCategoryIds();
        if (categoryIds != null && !categoryIds.isEmpty()) {
            categoryLv1Ids = new java.util.ArrayList<>();
            categoryLv2Ids = new java.util.ArrayList<>();
            categoryLv3Ids = new java.util.ArrayList<>();
            for (Long cid : categoryIds) {
                int lvl = categoryService.detectCategoryLevel(cid);
                if (lvl == -1) {
                    throw new IllegalArgumentException("分類 ID " + cid + " 不存在");
                }
                switch (lvl) {
                    case 1 -> categoryLv1Ids.add(cid);
                    case 2 -> categoryLv2Ids.add(cid);
                    case 3 -> categoryLv3Ids.add(cid);
                    default -> throw new IllegalArgumentException("分類 ID " + cid + " 層級異常: " + lvl);
                }
            }
            if (categoryLv1Ids.isEmpty()) categoryLv1Ids = null;
            if (categoryLv2Ids.isEmpty()) categoryLv2Ids = null;
            if (categoryLv3Ids.isEmpty()) categoryLv3Ids = null;
        }

        // 3. 分頁查詢（不 join product_spec）
        com.baomidou.mybatisplus.extension.plugins.pagination.Page<ProductEntity> page =
                new com.baomidou.mybatisplus.extension.plugins.pagination.Page<>(
                        query.getPage(),
                        query.getPageSize()
                );

        // storeId 優先（公開 API 篩選指定賣家），否則用 ownerStoreId（賣家管理自己商品）
        Long ownerFilter = query.getStoreId() != null ? query.getStoreId() : query.getOwnerStoreId();

        // seller_deleted exclude 只在「賣家看自己」時套（ownerStoreId 是 owner 且非公開 storeId 路徑）；
        // 公開 storeId 篩選、admin（ownerStoreId=null）皆不 exclude（Part A / doc 44 §11）。
        boolean excludeSellerDeleted = query.getStoreId() == null && query.getOwnerStoreId() != null;

        // 多 token 關鍵字（02 §5.3.3）：tokenize 後 OR 命中 + hit_count 排序
        List<String> tokens = io.github.will413028.hawkr.product.domain.text.KeywordTokenizer.tokenize(query.getKeyword());

        // Hashtag 精確篩選（02 §3.9）：normalize 為 canonical 後比對
        List<String> canonicalHashtags = hashtagService.normalizeForFilter(query.getHashtags());

        com.baomidou.mybatisplus.core.metadata.IPage<ProductEntity> result = productRepository.findPageByCondition(
                page,
                ownerFilter,
                categoryLv1Ids,
                categoryLv2Ids,
                categoryLv3Ids,
                effectiveStatus,
                tokens,
                canonicalHashtags,
                query.isHashtagMatchAll(),
                canonicalHashtags.size(),
                query.getSort(),
                query.getDeliveryTempFilter(),
                query.getCertificationTypes(),
                query.getPriceMinFilter(),
                query.getPriceMaxFilter(),
                query.getStoreIds(),
                query.getFollowedProductIds(),
                excludeSellerDeleted,
                query.isPublicApi()
        );

        // 3. 轉換為 ListItemDTO（不含 variants）
        List<ProductListItemDTO> items = result.getRecords().stream()
                .map(productEntityConverter::toListItemDTO)
                .collect(Collectors.toList());

        // 4. 填充 categoryName
        // 公開 API 與賣家管理 API 都透過分類服務解析名稱；商品 owner scope 使用 storeId。
        Set<Long> categoryIdsForName = items.stream()
                .map(ProductListItemDTO::getCategoryId)
                .filter(Objects::nonNull)
                .collect(Collectors.toSet());
        if (!categoryIdsForName.isEmpty()) {
            // 不限狀態填充分類名稱（§2.5.2 買家破口修復：商品掛在軟刪除分類下時仍顯示分類名）
            Map<Long, String> categoryNames = categoryService.getCategoryNames(categoryIdsForName);
            for (ProductListItemDTO item : items) {
                if (item.getCategoryId() != null) {
                    item.setCategoryName(categoryNames.get(item.getCategoryId()));
                }
            }
        }

        // 4.5 填充商家資訊（公開 API 才需要；list / detail 共用 SellerInfoAssembler，02 §5.3.5）
        if (query.isPublicApi()) {
            Set<Long> storeIds = items.stream()
                    .map(ProductListItemDTO::getStoreId)
                    .filter(Objects::nonNull)
                    .collect(Collectors.toSet());
            if (!storeIds.isEmpty()) {
                Map<Long, SellerMeta> sellerMetas = sellerGrpcClient.batchGetSellerMetas(storeIds);
                for (ProductListItemDTO item : items) {
                    if (item.getStoreId() != null) {
                        SellerMeta meta = sellerMetas.get(item.getStoreId());
                        item.setSeller(sellerInfoAssembler.from(item.getStoreId(), meta));
                    }
                }
            }
        }

        // 4.6 填充 SKU 規格名稱
        List<Long> productIds = items.stream()
                .map(item -> {
                    try { return Long.parseLong(item.getId()); }
                    catch (NumberFormatException e) { return null; }
                })
                .filter(Objects::nonNull)
                .collect(Collectors.toList());
        if (!productIds.isEmpty()) {
            List<SkuNameProjection> skuRows = productSkuRepository.findSkuNames(productIds);
            Map<Long, List<String>> skuNameMap = skuRows.stream()
                    .filter(s -> s.skuName() != null && !s.skuName().isEmpty())
                    .collect(Collectors.groupingBy(
                            SkuNameProjection::productId,
                            Collectors.mapping(SkuNameProjection::skuName, Collectors.toList())
                    ));
            for (ProductListItemDTO item : items) {
                try {
                    Long pid = Long.parseLong(item.getId());
                    item.setSkuNames(skuNameMap.getOrDefault(pid, List.of()));
                } catch (NumberFormatException ignored) {
                }
            }
        }

        // 4.7 填充 hashtag list（02 §5.3）
        if (!productIds.isEmpty()) {
            Map<Long, List<HashtagService.HashtagDisplayItem>> hashtagMap =
                    hashtagService.findHashtagsByProductIds(productIds);
            for (ProductListItemDTO item : items) {
                try {
                    Long pid = Long.parseLong(item.getId());
                    List<HashtagService.HashtagDisplayItem> hs = hashtagMap.getOrDefault(pid, List.of());
                    if (hs.isEmpty()) {
                        item.setHashtags(List.of());
                    } else {
                        item.setHashtags(hs.stream()
                                .map(h -> io.github.will413028.hawkr.product.dto.response.HashtagItemDTO.builder()
                                        .name(h.canonicalName())
                                        .displayName(h.displayName())
                                        .productCount(h.productCount())
                                        .build())
                                .toList());
                    }
                } catch (NumberFormatException ignored) {
                }
            }
        }

        // deliveryTemp 派生已移到 ProductEntityConverter.toListItemDTO（02 §5.3.6 本期改為從 product.shipping_methods 派生）
        // 不再需要 product_sku.shipping_method JSONB GROUP BY pipeline

        // 5. 組裝結果（pre-filter；shipping_methods 為 product 原始 snapshot）
        ProductListResultDTO listResult = ProductListResultDTO.builder()
                .products(items)
                .total(result.getTotal())
                .page(query.getPage())
                .pageSize(query.getPageSize())
                .build();

        // 5. 寫入快取（pre-filter snapshot，避免 stale；02 §2.7.2）
        cacheService.cacheProductList(cacheKey, listResult);

        log.info("success", "查詢商品列表成功，total=" + result.getTotal() + "，page=" + query.getPage());

        // 6. 對外回傳前套 onboarding filter（02 §2.7.2 公開 read path；賣家中心不過濾）
        if (query.isPublicApi()) {
            applyOnboardingFilterToList(listResult);
        }
        return listResult;
    }

    /**
     * 套用 onboarding filter 到列表結果（02 §2.7.2）。
     * per-request memoize per-seller onboarding snapshot，避免 N+1 gRPC。
     */
    private void applyOnboardingFilterToList(ProductListResultDTO listResult) {
        if (listResult == null || listResult.getProducts() == null || listResult.getProducts().isEmpty()) {
            return;
        }
        // 不對賣家中心 / admin 視角過濾 — 只對 publicApi 過濾
        // 列表進入此 method 一律視為「公開」場景；賣家中心走的是 getSellerProducts 等另外路徑
        Map<Long, OnboardingSnapshot> perSeller = new HashMap<>();
        for (ProductListItemDTO item : listResult.getProducts()) {
            Long sellerId = item.getStoreId();
            if (sellerId == null) {
                continue;
            }
            OnboardingSnapshot snap = perSeller.computeIfAbsent(sellerId, this::fetchOnboardingSnapshot);
            applyOnboardingFilterToListItem(item, snap.availableKeys);
        }
    }

    private ProductListResultDTO emptyResult(ProductQueryDTO query) {
        return ProductListResultDTO.builder()
                .products(List.of())
                .total(0L)
                .page(query.getPage())
                .pageSize(query.getPageSize())
                .build();
    }

    /**
     * 管理員更新商品狀態（不檢查擁有者）
     *
     * @param productId 商品 ID（TSID 數字字串）
     * @param status 目標狀態碼（ProductStatus value，如 4003=下架、4004=上架）
     * @return 更新後的商品詳情 DTO
     * @throws ProductNotFoundException 商品不存在
     * @throws IllegalArgumentException 狀態碼不合法
     */
    @Transactional
    public ProductDetailDTO updateProductStatus(String productId, String status) {
        // 1. 驗證狀態值合法
        boolean validStatus = false;
        for (ProductStatus ps : ProductStatus.values()) {
            if (ps.value().equals(status)) {
                validStatus = true;
                break;
            }
        }
        if (!validStatus) {
            throw new IllegalArgumentException("無效的商品狀態碼: " + status);
        }

        // 2. 查詢商品（不檢查 owner）
        Long id = Long.parseLong(productId);
        ProductEntity entity = productRepository.findById(id)
                .orElseThrow(() -> new ProductNotFoundException(productId));

        // 賣家已刪除：禁止把 flagged 商品重新上架到 public（doc 44 §11 / Part A）。
        // admin 仍可看、可改其他狀態，就是不能轉 ACTIVE。ProductBusinessException 由 GrpcErrorHandler catch-all 映業務碼。
        if (entity.getSellerDeletedAt() != null
                && ProductStatus.ACTIVE.value().equals(status)) {
            throw new ProductBusinessException(
                    GrpcResponseCode.INVALID_STATUS, "該商品所屬賣家已刪除，不可重新上架");
        }

        // 3. 更新狀態
        String oldStatus = entity.getStatus();
        entity.setFirstPublishedAt(io.github.will413028.hawkr.product.domain.ProductPublicationPolicy.firstPublishedAtOnActivation(
                entity.getFirstPublishedAt(), status, java.time.LocalDateTime.now()));
        entity.setStatus(status);
        long versionBeforeUpdate = currentVersion(entity);
        requireAdminCasHit(productRepository.updateById(entity), id);
        entity.setVersion(versionBeforeUpdate + 1);

        // 3.2 02 §5.13b 補貨 transition：上架 + stock>0 時 afterCommit 發 product.restock
        detectAndPublishRestockOnStatusChange(entity, oldStatus, status);

        // 4. 清除快取（狀態變更影響公開列表，需一併清除）
        cacheService.evictProductDetail(productId);
        cacheService.evictProductListForWrite(entity.getStoreId());

        // 5. 查詢 SKU 並轉為 DTO（含 hashtag list 填充）
        //    toDetailDTO 讀 product 表，hashtags 在 product_hashtag ⇒ 不補這一行，
        //    回應的 hashtags 恆為空。消費者看到的是「這個商品沒有標籤」而不是「本次
        //    沒查標籤」，兩者在 DTO 上無法區分。同檔另外五條產出 ProductDetailDTO 的
        //    路徑都有這一行。
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(id);
        ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));
        result.setHashtags(buildHashtagItemDTOs(id));

        // 6. 填充三層分類
        applyCategoryHierarchy(result, entity);

        // 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）
        // 跨 ACTIVE 邊界（上架／下架）改變賣家公開商品集合 → 共發 seller:<id>；未跨界只動商品本身。
        if (crossesActiveBoundary(oldStatus, status)) {
            revalidateProductAndSeller(entity.getId(), entity.getStoreId());
        } else {
            revalidateProduct(entity.getId());
        }

        log.info("statusUpdated", "管理員更新商品狀態，product_id=" + productId
                + "，old=" + oldStatus + "，new=" + status);

        return result;
    }

    /**
     * 同步重算指定 product 集合的反正規化文字欄位（02 §5.11.4 / §5.11.5）。
     *
     * <p>使用情境：
     * <ul>
     *   <li>{@code UpdateCategoryForAdmin} 改名 → cascade 改受影響 product 的 {@code cat_l*_name} 與 {@code searchable_text}</li>
     *   <li>{@code BatchChangeProductCategoryForAdmin} 換 category id → 同事務 reflow</li>
     * </ul>
     *
     * <p>每筆獨立查 hashtag canonicals + sku names + 三層 category name，重算後 update；
     * 同事務內逐筆執行（採 {@code REQUIRED} 確保呼叫端的事務上下文延續）。
     *
     * @param productIds 受影響的商品 ID
     * @return 實際更新筆數（id 不存在或已軟刪除者跳過）
     */
    @Transactional(propagation = org.springframework.transaction.annotation.Propagation.REQUIRED)
    public int reflowSearchableForProductIds(List<Long> productIds) {
        if (productIds == null || productIds.isEmpty()) return 0;
        int updated = 0;
        for (Long pid : productIds) {
            if (reflowOneSearchable(pid)) {
                updated++;
            }
        }
        return updated;
    }

    /**
     * 單筆 reflow；CAS 未命中時重讀重算再寫一次，第二次仍未命中才拋 9266。
     *
     * <p>此處的重讀<b>不是</b> 56 §6.6.3 禁止的「再查一次拿新 version 然後覆蓋」——那條禁的是
     * 拿使用者送進來的內容蓋掉他人較新的修改。本方法寫的三個欄位全部由重讀後的當前列推導
     * （{@code cat_l*_name} / {@code pure_description} / {@code searchable_text}），併發修改者的
     * 欄位是原樣讀出再寫回，不會被吃掉。
     *
     * <p>不直接拋的理由：呼叫端是分類改名 / 批次換分類的 cascade（動輒數千筆同事務），
     * 任一筆被賣家或停權連動併發碰到就整批回滾，代價遠大於收益；而靜默跳過會讓該筆的
     * {@code searchable_text} 永久停在舊分類名（搜尋結果錯且無訊號），兩者都不可接受。
     *
     * @return true = 已寫入；false = 該 id 不存在或已軟刪除（跳過，非失敗）
     */
    private boolean reflowOneSearchable(Long pid) {
        for (int attempt = 1; attempt <= REFLOW_CAS_ATTEMPTS; attempt++) {
            ProductEntity entity = productRepository.findById(pid).orElse(null);
            if (entity == null) return false;

            // 取 hashtag canonical names（依 position）
            List<HashtagService.HashtagDisplayItem> hashtags = hashtagService.findHashtagsByProductId(pid);
            List<HashtagService.UpsertedHashtag> hashtagInputs = hashtags.stream()
                    .map(h -> new HashtagService.UpsertedHashtag(null, h.canonicalName(), h.displayName()))
                    .toList();

            // 取 sku list
            List<ProductSkuEntity> skus = productSkuRepository.findByProductId(pid);

            // 重算 cat_l*_name + pure_description + searchable_text
            enrichSearchableFields(entity, hashtagInputs, skus);

            if (productRepository.updateById(entity) > 0) {
                return true;
            }
            log.warn("reflowCasMiss",
                    "reflow 樂觀鎖 CAS 未命中，attempt=" + attempt + "，product_id=" + pid);
        }
        throw new ProductBusinessException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT);
    }

    /**
     * 將 product_id 的當前 hashtag 綁定轉為 DTO（02 §5.4 / §5.5）。
     */
    private List<io.github.will413028.hawkr.product.dto.response.HashtagItemDTO> buildHashtagItemDTOs(Long productId) {
        List<HashtagService.HashtagDisplayItem> items = hashtagService.findHashtagsByProductId(productId);
        if (items.isEmpty()) return List.of();
        return items.stream()
                .map(h -> io.github.will413028.hawkr.product.dto.response.HashtagItemDTO.builder()
                        .name(h.canonicalName())
                        .displayName(h.displayName())
                        .productCount(h.productCount())
                        .build())
                .toList();
    }

    /**
     * 賣家更新商品狀態（含擁有者驗證 + 狀態轉換驗證）
     *
     * 允許的轉換：DRAFT→ACTIVE, ACTIVE→INACTIVE, INACTIVE→ACTIVE
     * 禁止：操作 PENDING_REVIEW 狀態的商品、回退到 DRAFT
     */
    @Transactional
    public ProductDetailDTO updateSellerProductStatus(String productId, Long storeId, String status) {
        return updateSellerProductStatus(productId, storeId, status, null);
    }

    /**
     * 賣家更新商品狀態，帶樂觀鎖期望版本（56 §6.6.3）。
     *
     * @param expectedVersion 期望版本（數字字串）；null／空字串 = 不比對
     */
    @Transactional
    public ProductDetailDTO updateSellerProductStatus(String productId, Long storeId, String status,
                                                      String expectedVersion) {
        // 1. 驗證目標狀態碼合法
        if (!ProductStatus.isValidValue(status)) {
            throw new IllegalArgumentException("無效的商品狀態碼: " + status);
        }

        // 2. 查詢商品並驗證擁有者
        Long id = Long.parseLong(productId);
        ProductEntity entity = productRepository.findByIdAndStoreId(id, storeId)
                .orElseThrow(() -> new ProductNotFoundException(productId));

        // 2.1 樂觀鎖前置比對（真正的 CAS 在步驟 4 的 updateById）
        requireExpectedVersion(entity, expectedVersion);

        // 3. 驗證狀態轉換合法性
        String oldStatus = entity.getStatus();
        validateSellerStatusTransition(oldStatus, status);

        // 3.1 賣家停權 backstop（02 §5.10 步驟 5b / 46 §6.2，首次接線）：目標為 ACTIVE 時強制檢查
        if (ProductStatus.ACTIVE.value().equals(status)) {
            requireSellerCanManageProductsForActivation(storeId);
        }

        // 4. 更新狀態
        entity.setStatus(status);
        entity.setFirstPublishedAt(io.github.will413028.hawkr.product.domain.ProductPublicationPolicy.firstPublishedAtOnActivation(
                entity.getFirstPublishedAt(), status, java.time.LocalDateTime.now()));
        entity.setUpdatedBy(storeId);
        long versionBeforeUpdate = currentVersion(entity);
        requireRowsAffected(productRepository.updateById(entity), id);
        entity.setVersion(versionBeforeUpdate + 1);

        // 4.2 02 §5.13b 補貨 transition：上架 + stock>0 時 afterCommit 發 product.restock
        detectAndPublishRestockOnStatusChange(entity, oldStatus, status);

        // 5. 清除快取
        cacheService.evictProductDetail(productId);
        cacheService.evictProductListForWrite(entity.getStoreId());

        // 6. 查詢 SKU 並轉為 DTO（含 hashtag list 填充）
        //    ⚠️ 這一行曾經缺席，代價不只是「回應少一欄」：56 號代行上下架把寫入回應
        //    當成 after 快照去 diff，於是每一次上／下架的稽核列都虛報一列「主題標籤
        //    被清空」，而 product_hashtag 一列未動。稽核列是爭議時的舉證，記到假的
        //    比少記更糟。
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(id);
        ProductDetailDTO result = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));
        result.setHashtags(buildHashtagItemDTOs(id));

        // 7. 填充三層分類
        applyCategoryHierarchy(result, entity);

        // 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）
        // — DRAFT→ACTIVE / ACTIVE→INACTIVE / INACTIVE→ACTIVE 都需即時 propagate（#1035 #5 觸發來源）
        // 跨 ACTIVE 邊界（上架／下架）改變賣家公開商品集合 → 共發 seller:<id>；未跨界只動商品本身。
        if (crossesActiveBoundary(oldStatus, status)) {
            revalidateProductAndSeller(entity.getId(), storeId);
        } else {
            revalidateProduct(entity.getId());
        }

        log.info("sellerStatusUpdated", "賣家更新商品狀態，product_id=" + productId
                + "，old=" + oldStatus + "，new=" + status
                + "，store_id=" + storeId);

        return result;
    }

    /**
     * 驗證賣家允許的狀態轉換
     */
    /**
     * 批次取 before 快照與當前 version（56 §7.2.8 步驟 ②，唯讀）。
     *
     * <p>與 productIds 同序、同長度；查不到或不屬於 storeId 的一律 {@code found=false}
     * ——兩者不區分，避免用回應差異探測他店商品是否存在。
     */
    public List<ProductSnapshotDTO> getProductSnapshotsForDelegation(Long storeId, List<String> productIds) {
        return getProductSnapshotsForDelegation(storeId, productIds, false);
    }

    /**
     * 同上，{@code withDetail=true} 時另填 {@link ProductSnapshotDTO#getDetail()}。
     *
     * <p><b>批次的第一跳必須傳 true。</b> 稽核的 before_snapshot 取自這一份 detail；只回
     * 前四欄時，批次下架的 before_snapshot 只剩 {@code status} 一個鍵，而同一個操作的單筆
     * 版本存的是完整 12 欄。批次正是「一次動很多件」的高風險操作，事後要回答「平台動手當下
     * 這件商品長什麼樣」的需求只會更強，不會更弱。
     *
     * <p>逐項組 detail 是三次額外查詢（SKU／hashtag／分類階層），<b>但全部在本服務內</b>，
     * 跳數仍與 items 筆數無關（§7.2.8）。反過來讓 Gateway 逐項打 {@code GetProduct} 才是被
     * 明令禁止的形狀，而且那條路還會連帶付 N 次 seller-service 呼叫。
     */
    public List<ProductSnapshotDTO> getProductSnapshotsForDelegation(
            Long storeId, List<String> productIds, boolean withDetail) {
        List<ProductSnapshotDTO> results = new ArrayList<>(productIds.size());
        for (String productIdStr : productIds) {
            ProductSnapshotDTO.ProductSnapshotDTOBuilder builder =
                    ProductSnapshotDTO.builder().productId(productIdStr).found(false);
            Long id;
            try {
                id = Long.parseLong(productIdStr);
            } catch (NumberFormatException e) {
                results.add(builder.build());
                continue;
            }
            productRepository.findByIdAndStoreId(id, storeId).ifPresent(entity -> {
                builder.found(true)
                        .version(currentVersion(entity))
                        .status(entity.getStatus())
                        .name(entity.getName());
                if (withDetail) {
                    builder.detail(buildDelegationSnapshotDetail(entity));
                }
            });
            results.add(builder.build());
        }
        return results;
    }

    /**
     * 組出代行稽核 before_snapshot 用的商品詳情。
     *
     * <p>與 {@link #getProduct(String, Long, boolean)} 的差別只有兩點，且兩點都是刻意的：
     * <b>不查 seller、不查 reviews</b>——投影不用它們，而 seller 那一項是跨服務呼叫。其餘
     * 三段（SKU、hashtags、三層分類）一項都不能少：Gateway 對兩條路徑跑同一支投影函式，這裡
     * 少填哪一段，稽核就把該欄記成空值，而空值與「它本來就是空的」在稽核列上分不出來。
     */
    private ProductDetailDTO buildDelegationSnapshotDetail(ProductEntity entity) {
        List<ProductSkuEntity> skus = productSkuRepository.findByProductId(entity.getId());
        ProductDetailDTO detail = productEntityConverter.toDetailDTO(entity, skus, isStoreExposable(entity));
        detail.setHashtags(buildHashtagItemDTOs(entity.getId()));
        applyCategoryHierarchy(detail, entity);
        return detail;
    }

    /**
     * 樂觀鎖期望版本比對。expectedVersion 為 null／空字串時不比對（賣家自助端既有行為）。
     */
    private void requireExpectedVersion(ProductEntity entity, String expectedVersion) {
        if (expectedVersion == null || expectedVersion.isBlank()) {
            return;
        }
        long expected;
        try {
            expected = Long.parseLong(expectedVersion.trim());
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("expected_version 必須為數字字串: " + expectedVersion);
        }
        if (expected != currentVersion(entity)) {
            throw new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT);
        }
    }

    /**
     * updateById 的 CAS 結果判定。
     *
     * <p>MyBatis-Plus 的 OptimisticLockerInnerInterceptor 在 version 不符時只回 0 列、<b>不拋例外</b>
     * ⇒ 不判就是「回 200 但資料沒寫」。
     */
    private void requireRowsAffected(int rows, Long productId) {
        if (rows == 0) {
            log.warn("versionConflict", "樂觀鎖 CAS 未命中，product_id=" + productId);
            throw new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT);
        }
    }

    /**
     * admin 路徑的 CAS 結果判定。條件與 {@link #requireRowsAffected} 相同，例外型別不同。
     *
     * <p>{@code DelegationDeniedException} 不是 {@code GrpcBusinessException}（模組依賴會成環，
     * 見其 javadoc），要靠 RPC 方法自己 catch 才拿得到 code；而 admin 的
     * {@code UpdateProductStatusForAdmin} 沒有那個 catch 分支 ⇒ 丟它會落進 catch-all 的
     * 「非業務例外」分支，變成 500 系統錯誤＋ERROR 堆疊。改丟 {@link ProductBusinessException}
     * 讓 catch-all 認得，前端拿到的仍是同一個 9266。
     */
    private void requireAdminCasHit(int rows, Long productId) {
        if (rows == 0) {
            log.warn("versionConflict", "樂觀鎖 CAS 未命中（admin 路徑），product_id=" + productId);
            throw new ProductBusinessException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT);
        }
    }

    /**
     * SKU 層樂觀鎖：把 client 的 expectedVersion 蓋進 entity，交給
     * OptimisticLockerInnerInterceptor 去 CAS（#2545）。
     *
     * <p><b>為什麼一定要覆蓋</b>：{@code updateSkus} 是先 {@code findByProductId} 把 SKU 讀出來、
     * 再把 DTO 疊上去寫回。不覆蓋的話 entity 帶的是「本次交易剛讀到的最新版本」，CAS 等於拿
     * 自己比自己 ⇒ 永遠命中 ⇒ 加了 {@code @Version} 也擋不住任何東西。
     *
     * <p>expectedVersion 為 null／空字串 = 不比對（沿用 product 層
     * {@link #requireExpectedVersion} 對舊 client 的既有語意）：此時 entity 仍帶著剛讀到的版本，
     * CAS 必命中，行為與本次修改前完全相同。
     */
    private void applySkuExpectedVersion(ProductSkuEntity entity, String expectedVersion) {
        if (expectedVersion == null || expectedVersion.isBlank()) {
            return;
        }
        try {
            entity.setVersion(Long.parseLong(expectedVersion.trim()));
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("sku expected_version 必須為數字字串: " + expectedVersion);
        }
    }

    /**
     * SKU updateById 的 CAS 結果判定（#2545）。
     *
     * <p>與 {@link #requireRowsAffected} 同構：MyBatis-Plus 在 version 不符時只回 0 列、不拋例外，
     * 不判就是「回 200 但這筆 SKU 根本沒寫進去」。
     *
     * <p>expectedVersion 未提供時 entity 帶的是剛讀到的版本，理論上不會 miss；真的 miss 代表
     * 該列在本交易內被移除，同樣不能當成成功。
     *
     * <p>沿用 9266（「資料已被更新，請重新載入後再試」）而不另立碼：它是本 repo 唯一的版本衝突碼，
     * 訊息對賣家自助端與代行端都成立；為同一語意另開一個碼才是重複（§15）。
     */
    private void requireSkuCasHit(int rows, Long skuId) {
        if (rows == 0) {
            log.warn("skuVersionConflict", "SKU 樂觀鎖 CAS 未命中，sku_id=" + skuId);
            throw new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT);
        }
    }

    /**
     * 建立路徑「INSERT 後補寫」的落盤判定。
     *
     * <p>與 CAS 未命中不同：entity 是本事務內剛 INSERT 的，{@code version} 在記憶體中為 null，
     * 而 {@code OptimisticLockerInnerInterceptor} 對 null version 直接略過整段樂觀鎖
     * （不加 {@code AND version = ?}、也不 {@code +1}）⇒ 這裡的 0 列不可能是版本衝突，
     * 只可能是「剛寫進去的列不見了」。因此丟 {@code IllegalStateException} 讓它以系統錯誤
     * ＋堆疊現形並回滾，而不是回一個叫使用者「重新載入」的 9266。
     */
    private void requireInsertedRowWritten(int rows, Long productId) {
        if (rows == 0) {
            throw new IllegalStateException("商品建立後補寫影響 0 列，product_id=" + productId);
        }
    }

    private long currentVersion(ProductEntity entity) {
        return entity.getVersion() != null ? entity.getVersion() : 0L;
    }

    private void validateSellerStatusTransition(String fromStatus, String toStatus) {
        // PENDING_REVIEW 狀態的商品，賣家完全不可操作
        if (ProductStatus.PENDING_REVIEW.value().equals(fromStatus)) {
            throw new IllegalArgumentException("審核中的商品僅管理員可操作");
        }

        // 檢查允許的轉換
        boolean allowed = false;
        if (ProductStatus.DRAFT.value().equals(fromStatus) && ProductStatus.ACTIVE.value().equals(toStatus)) {
            allowed = true; // DRAFT → ACTIVE
        } else if (ProductStatus.ACTIVE.value().equals(fromStatus) && ProductStatus.INACTIVE.value().equals(toStatus)) {
            allowed = true; // ACTIVE → INACTIVE
        } else if (ProductStatus.INACTIVE.value().equals(fromStatus) && ProductStatus.ACTIVE.value().equals(toStatus)) {
            allowed = true; // INACTIVE → ACTIVE
        }

        if (!allowed) {
            throw new IllegalArgumentException(
                    "不允許的狀態轉換: " + fromStatus + " → " + toStatus);
        }
    }

    /**
     * 賣家停權 ACTIVE transition backstop（02 §5.10 步驟 5b / 46 §6.2 §11，首次接線）。
     * 商品轉 ACTIVE 的唯一收斂點檢查：{@code can_manage_products=false}（店家停權、等待刪除或已關閉）→ 擋下，回業務碼 9240。
     * {@code can_manage_products} 只看店家的營運狀態，不要求 KYC 或 PayUni 核准：賣家可以在店家可公開曝光之前先上架商品，
     * 買家看不看得到由讀取端的可曝光過濾決定。
     * 這是能力層 Gateway fail-open 窗口的唯一不可逆殘留（產生公開商品）兜底，任何重構不得移除或繞過。
     *
     * <p>fail-closed：查詢走 {@link SellerGrpcClient#getSellerMetaOrThrow}（硬依賴，不降級）；
     * gRPC 失敗或查無資料一律視為不可放行，回 SERVICE_UNAVAILABLE(503) 供前端重試，不靜默放行 ACTIVE。
     */
    private void requireSellerCanManageProductsForActivation(Long storeId) {
        SellerMeta sellerMeta;
        try {
            sellerMeta = sellerGrpcClient.getSellerMetaOrThrow(storeId);
        } catch (Exception e) {
            log.error("requireSellerCanManageProductsForActivation.queryFailed",
                    "SellerMeta 查詢失敗，fail-closed 擋下 ACTIVE transition: " + e.getMessage(), e);
            throw new ProductBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE, "賣家狀態查詢失敗，請稍後再試");
        }
        if (!sellerMeta.getCanManageProducts()) {
            throw new ProductBusinessException(GrpcResponseCode.SELLER_SUSPENDED_OPERATION_BLOCKED);
        }
    }

    // package-visible test seam：直接驗證 ACTIVE transition backstop 規則本身，
    // 不需 bootstrap 整條 updateSellerProductStatus() / createProduct() pipeline。
    void requireSellerCanManageProductsForActivationForTest(Long storeId) {
        requireSellerCanManageProductsForActivation(storeId);
    }

    // ==================== 私有輔助方法 ====================

    /**
     * 處理新建商品的圖片
     * 將 staged_token 轉換為正式 asset
     */
    private List<ProductEntity.GalleryItem> processImages(Long productId, List<ProductImageDTO> images) {
        if (images == null || images.isEmpty()) {
            return new ArrayList<>();
        }

        // 收集所有 staged_token
        List<String> stagedTokens = images.stream()
                .filter(img -> img.getStagedToken() != null && !img.getStagedToken().isEmpty())
                .map(ProductImageDTO::getStagedToken)
                .collect(Collectors.toList());

        if (stagedTokens.isEmpty()) {
            return new ArrayList<>();
        }

        // 轉正失敗一律往外拋，與業務落盤後的 linkGalleryImages 同一條原則（#2222）：
        // 拋出 → 交易回滾 → 沒有任何列宣稱擁有這些圖。吞掉則會 commit 出一個「操作者送了
        // N 張圖、回 code: 0、實際一張都沒掛上」的成功回應，而更新路徑的 images 是整份替換，
        // 連既有圖片都會一起消失（56 P0-1 的實際症狀）。
        List<MediaGrpcClient.FinalizedImage> finalized =
                mediaGrpcClient.finalizeImages(stagedTokens, AssetReferenceType.PRODUCT.value());

        // 依 staged_token 建立 mapping（media.proto:41 契約要求），避免依賴回傳順序
        Map<String, MediaGrpcClient.FinalizedImage> tokenToFinalized = finalized.stream()
                .collect(Collectors.toMap(MediaGrpcClient.FinalizedImage::getStagedToken, f -> f, (a, b) -> a));

        // 依輸入圖片順序組裝 gallery（決定 gallery 顯示位置）
        List<ProductEntity.GalleryItem> result = new ArrayList<>();
        for (ProductImageDTO img : images) {
            if (img.getStagedToken() == null || img.getStagedToken().isEmpty()) {
                continue;
            }
            MediaGrpcClient.FinalizedImage f = tokenToFinalized.get(img.getStagedToken());
            if (f == null) {
                continue;
            }
            ProductEntity.GalleryItem item = new ProductEntity.GalleryItem();
            item.setAssetId(f.getAssetId());
            item.setObjectKey(f.getObjectKey());
            result.add(item);
        }
        return result;
    }

    /**
     * 處理更新商品的圖片（混合清單）
     */
    private List<ProductEntity.GalleryItem> processImageUpdate(
            Long productId,
            List<ProductEntity.GalleryItem> existingGallery,
            List<ProductImageDTO> newImages
    ) {
        // 處理新增的圖片（有 staged_token，僅轉正，不建立引用）
        List<String> stagedTokens = newImages.stream()
                .filter(img -> img.getStagedToken() != null && !img.getStagedToken().isEmpty())
                .map(ProductImageDTO::getStagedToken)
                .collect(Collectors.toList());

        // 轉正失敗往外拋，理由同 processImages：這裡吞掉的後果最嚴重——images 是整份替換，
        // 送進來的新圖沒轉正成功時，賣家原本的圖也已經不在 result 裡了。
        List<MediaGrpcClient.FinalizedImage> newFinalized = new ArrayList<>();
        if (!stagedTokens.isEmpty()) {
            newFinalized = mediaGrpcClient.finalizeImages(stagedTokens, AssetReferenceType.PRODUCT.value());
        }

        // 依 staged_token 建立 mapping（media.proto:41 契約要求），避免依賴回傳順序
        Map<String, MediaGrpcClient.FinalizedImage> tokenToFinalized = newFinalized.stream()
                .collect(Collectors.toMap(MediaGrpcClient.FinalizedImage::getStagedToken, f -> f, (a, b) -> a));

        // 依 newImages 原始輸入順序逐項組裝 gallery，讓最終順序完全等於前端送出的陣列順序（#1424）。
        // 既有圖（assetId）與新圖（stagedToken）可交錯混用，單一迴圈避免「既有群組 → 新增群組」的兩段式重排。
        List<ProductEntity.GalleryItem> result = new ArrayList<>();
        for (ProductImageDTO img : newImages) {
            if (img.getAssetId() != null && !img.getAssetId().isEmpty()) {
                // 既有圖：從現有 gallery 以 assetId 對應保留（找不到則略過）
                if (existingGallery != null) {
                    existingGallery.stream()
                            .filter(g -> g.getAssetId() != null && g.getAssetId().equals(img.getAssetId()))
                            .findFirst()
                            .ifPresent(result::add);
                }
            } else if (img.getStagedToken() != null && !img.getStagedToken().isEmpty()) {
                // 新圖：以 staged_token 從轉正 mapping 取得（轉正失敗／查無則略過）
                MediaGrpcClient.FinalizedImage finalized = tokenToFinalized.get(img.getStagedToken());
                if (finalized != null) {
                    ProductEntity.GalleryItem item = new ProductEntity.GalleryItem();
                    item.setAssetId(finalized.getAssetId());
                    item.setObjectKey(finalized.getObjectKey());
                    result.add(item);
                }
            }
        }

        return result;
    }

    /**
     * SKU 的重量／材積必填校驗。
     *
     * <p>這四個欄位在 proto 是 {@code string}，缺席時 {@code ProductProtoConverter.parseBigDecimal}
     * 會回 {@link BigDecimal#ZERO}（不是 null）。舊寫法 {@code if (x != null && x <= 0) throw}
     * 讀起來像「可選，有給才檢查」，但 null 分支永遠進不去，實際行為是「必填且必須 > 0」。
     * 因為型別本身無法表達「未提供」，只能靠這個 helper 把真實契約寫明白：
     * <b>省略 = 送 0 = 被拒絕</b>。改成 {@code == null ||} 是為了讓讀者不會再被誤導，
     * 對外行為與改動前完全一致（dev 實測：省略 weight → 400「商品重量 必須大於 0」）。
     */
    private void requirePositive(BigDecimal value, String fieldName) {
        if (value == null || value.compareTo(BigDecimal.ZERO) <= 0) {
            throw new IllegalArgumentException(fieldName + " 必須大於 0");
        }
    }

    /**
     * 更新商品 SKU。
     *
     * <p><b>結構性不變式：回傳集合恆非空</b>，因此本方法無法把商品變成零 SKU。它不是某一行
     * 檢查出來的，而是兩條路徑交互作用的結果——空／null 清單直接原樣回傳既有 SKU（下方 no-op
     * 分支），非空清單則每一筆都會進 {@code resultSkus}，而刪除範圍只是
     * {@code existingSkus - keepIds}。配上 {@code ProductGrpcService.createProduct} 的
     * {@code skus_count} 守衛（建立側恆 ≥1），「商品至少一個 SKU」在線上就沒有可達的反例。
     *
     * <p>這件事有兩個下游倚賴它，改動前務必一起看：{@code product.price_min IS NULL} 目前
     * 等價於「零 SKU」（{@code product_sku.price} 是 {@code BIGINT NOT NULL}），以及
     * {@code purchasable} 對 {@code stock_qty > 0} 的隱性依賴。真要開放「刪到零 SKU」，
     * 得先給這兩者各自的表達方式，不能只把守衛拿掉。不變式本身由
     * {@code ProductServiceUpdateSkusEmptyListTest} 守著。
     */
    private List<ProductSkuEntity> updateSkus(Long productId, List<ProductSkuDTO> skuDTOs) {
        // 查詢現有 SKU
        List<ProductSkuEntity> existingSkus = productSkuRepository.findByProductId(productId);
        Map<String, ProductSkuEntity> existingMap = existingSkus.stream()
                .collect(Collectors.toMap(s -> s.getId().toString(), s -> s));

        List<ProductSkuEntity> resultSkus = new ArrayList<>();

        // 空清單 = 「本次不動 SKU」，不是「清空 SKU」。
        //
        // proto3 的 `repeated ProductSku skus` 無法區分「欄位缺席」與「空陣列」——前端
        // PUT /app/products/{id} 只想改商品名稱、完全不帶 skus 時，到這裡就是一個空 List。
        // 舊實作把它讀成「清空 SKU」，於是一次局部更新會實體刪光該商品所有 SKU
        // （dev 實測：帶 1 個 SKU 的商品送 {"name": "..."} → HTTP 200 → product_sku 0 rows）。
        // 商品無 SKU 是不可購買的破損狀態（createProduct 已要求至少一個 SKU），
        // 因此「清空」本來就不是合法操作，直接讓空清單成為 no-op。
        if (skuDTOs == null || skuDTOs.isEmpty()) {
            return existingSkus;
        }

        // 帶 id 的 SKU 必須確實屬於本商品，否則整個請求失敗（#2544）。
        //
        // 舊實作對 existingMap 查無的 id 是「靜默略過」：該 id 既不進 resultSkus 也不進 keepIds，
        // 於是下方的刪除步驟會把**全部**既有 SKU 算成「不在保留清單」而實體刪除，同事務接著
        // 以空集合重算 stock_qty（aggregateStock(∅) 回原生 0）→ 商品變成 0 SKU、庫存 0，HTTP 仍 200。
        // 商品歸屬已在 updateProduct 步驟 1 的 findByIdAndStoreId 驗過，所以這不是越權，
        // 但賣家送一個打錯的 skuId 就能清空自己商品的全部規格，且沒有任何錯誤訊息。
        //
        // 驗證刻意放在寫入迴圈之前（fail-fast）：拖到迴圈內才拋，前面幾個 SKU 的
        // INSERT/UPDATE 已經送出，只能倚賴外層 @Transactional 回滾；先驗完再寫，
        // 讓不合法輸入根本不產生副作用。
        for (ProductSkuDTO skuDTO : skuDTOs) {
            String skuId = skuDTO.getId();
            if (skuId != null && !skuId.isEmpty() && !existingMap.containsKey(skuId)) {
                throw new ProductBusinessException(
                        GrpcResponseCode.PRODUCT_SKU_NOT_FOUND,
                        String.format(GrpcResponseCode.PRODUCT_SKU_NOT_FOUND.message(), skuId));
            }
        }

        Set<String> keepIds = new HashSet<>();

        for (ProductSkuDTO skuDTO : skuDTOs) {
            if (skuDTO.getId() != null && !skuDTO.getId().isEmpty()) {
                // 更新現有 SKU（上方已驗證必定命中，不再有「查無就略過」的分支）
                ProductSkuEntity existing = existingMap.get(skuDTO.getId());
                productSkuConverter.updateEntity(existing, skuDTO);
                applySkuExpectedVersion(existing, skuDTO.getExpectedVersion());
                Long versionBeforeUpdate = existing.getVersion();
                requireSkuCasHit(productSkuRepository.updateById(existing), existing.getId());
                if (versionBeforeUpdate != null) {
                    existing.setVersion(versionBeforeUpdate + 1);
                }
                resultSkus.add(existing);
                keepIds.add(skuDTO.getId());
            } else {
                // 新增 SKU
                ProductSkuEntity newSku = productSkuConverter.toEntity(skuDTO, productId);
                productSkuRepository.insert(newSku);
                resultSkus.add(newSku);
            }
        }

        // 刪除不在保留清單中的 SKU
        List<Long> deleteIds = existingSkus.stream()
                .filter(s -> !keepIds.contains(s.getId().toString()))
                .map(ProductSkuEntity::getId)
                .collect(Collectors.toList());

        if (!deleteIds.isEmpty()) {
            productSkuRepository.deleteByIdIn(deleteIds);
        }

        return resultSkus;
    }

    /**
     * 更新商品價格範圍
     */
    /**
     * 驗證 SKU 配送方式合法性（賣家開通狀態 + 材積/重量限制）
     * 跳過無配送方式或缺少材積資料的 SKU
     */
    private void validateSkuShippingMethods(Long storeId, List<ProductSkuDTO> skus) {
        if (skus == null || skus.isEmpty()) {
            return;
        }

        List<ValidateShippingItem> items = new ArrayList<>();
        for (ProductSkuDTO sku : skus) {
            if (sku.getShippingMethods() == null || sku.getShippingMethods().isEmpty()) {
                continue;
            }
            // 跳過缺少材積或重量的 SKU
            if (sku.getLengthCm() == null || sku.getWidthCm() == null
                    || sku.getHeightCm() == null || sku.getWeight() == null || sku.getUnit() == null) {
                continue;
            }

            long weightGrams = WeightConverter.convertToGrams(
                    sku.getWeight().toPlainString(), WeightUnit.fromValue(sku.getUnit()));

            String skuName = sku.getSkuName() != null ? sku.getSkuName() : "未命名";
            for (ShippingMethodDTO sm : sku.getShippingMethods()) {
                if (sm.getType() == null || sm.getType().isEmpty()) {
                    continue;
                }
                items.add(ValidateShippingItem.newBuilder()
                        .setSkuName(skuName)
                        .setShippingMethod(sm.getType())
                        .setTemperature(sm.getTemperature() != null ? sm.getTemperature() : "")
                        .setLengthCm(String.valueOf(sku.getLengthCm().intValue()))
                        .setWidthCm(String.valueOf(sku.getWidthCm().intValue()))
                        .setHeightCm(String.valueOf(sku.getHeightCm().intValue()))
                        .setWeightGrams(String.valueOf(weightGrams))
                        .build());
            }
        }

        if (items.isEmpty()) {
            return;
        }

        ValidateProductShippingResponse response =
                shippingGrpcClient.validateProductShipping(storeId, items);

        if (!response.getAllValid()) {
            StringBuilder sb = new StringBuilder();
            for (ValidateShippingResult r : response.getResultsList()) {
                if (!r.getValid()) {
                    if (sb.length() > 0) {
                        sb.append("; ");
                    }
                    sb.append("SKU「").append(r.getSkuName()).append("」的 ")
                      .append(r.getShippingMethod()).append("/").append(r.getTemperature())
                      .append(" 驗證失敗：").append(r.getRejectionMessage());
                }
            }
            throw new IllegalArgumentException("配送方式驗證失敗：" + sb);
        }
    }

    /**
     * 計算商品的三組價格冗餘欄位（02 設計書 §2.8 / §3.2）：
     * - priceMin / priceMax：以「實際成交單價」彙總（effective_price = sale_price ?? price）
     * - listPriceMin / listPriceMax：以原價（SKU.price）彙總
     * - salePriceMin / salePriceMax：以特價（SKU.salePrice）彙總；皆無特價時為 null
     *
     * <p>空集合分支的 {@code setXxx(null)} 不是死碼：六個欄位在 {@link ProductEntity} 上都標了
     * {@code updateStrategy = IGNORED}，null 才寫得進 DB（#2546）。移掉任一邊都會讓 0 SKU 的商品
     * 繼續對外宣稱舊價格。
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

        // 原價區間（永遠以 SKU.price 彙總）
        Long listMin = skus.stream()
                .map(ProductSkuEntity::getPrice)
                .filter(Objects::nonNull)
                .min(Long::compareTo)
                .orElse(null);
        Long listMax = skus.stream()
                .map(ProductSkuEntity::getPrice)
                .filter(Objects::nonNull)
                .max(Long::compareTo)
                .orElse(null);

        // 特價區間（皆無特價時為 null）
        Long saleMin = skus.stream()
                .map(ProductSkuEntity::getSalePrice)
                .filter(Objects::nonNull)
                .min(Long::compareTo)
                .orElse(null);
        Long saleMax = skus.stream()
                .map(ProductSkuEntity::getSalePrice)
                .filter(Objects::nonNull)
                .max(Long::compareTo)
                .orElse(null);

        // 實際售價區間（effective_price = salePrice ?? price）
        Long effectiveMin = skus.stream()
                .map(sku -> sku.getSalePrice() != null ? sku.getSalePrice() : sku.getPrice())
                .filter(Objects::nonNull)
                .min(Long::compareTo)
                .orElse(null);
        Long effectiveMax = skus.stream()
                .map(sku -> sku.getSalePrice() != null ? sku.getSalePrice() : sku.getPrice())
                .filter(Objects::nonNull)
                .max(Long::compareTo)
                .orElse(null);

        entity.setPriceMin(effectiveMin);
        entity.setPriceMax(effectiveMax);
        entity.setListPriceMin(listMin);
        entity.setListPriceMax(listMax);
        entity.setSalePriceMin(saleMin);
        entity.setSalePriceMax(saleMax);
    }

    /**
     * 對 description.blocks 中所有 type=text 的 content 跑純文字驗證（02 §3.6 / 26-§2.3 R8）。
     * 拒絕含 HTML tag。
     */
    private void validateDescriptionTextBlocks(ProductEntity.ProductDescription description) {
        if (description == null || description.getBlocks() == null) return;
        for (ProductEntity.DescriptionBlock block : description.getBlocks()) {
            if (block == null || !"text".equals(block.getType())) continue;
            InputValidator.requireNoHtmlTags(block.getContent(), "商品描述");
        }
    }

    /** 描述表格上限（02 §3.6）。 */
    private static final int MAX_TABLE_COLS = 10;
    private static final int MAX_TABLE_ROWS = 50;
    private static final int MAX_CELL_TEXT_LEN = 200;
    private static final int MAX_TABLE_JSON_BYTES = 20 * 1024;
    /** 描述清單上限（02 §3.6；單項純文字上限複用 {@link #MAX_CELL_TEXT_LEN}）。 */
    private static final int MAX_LIST_ITEMS = 50;
    private static final int MAX_LIST_JSON_BYTES = 20 * 1024;
    private static final com.fasterxml.jackson.databind.ObjectMapper DESC_MAPPER =
            new com.fasterxml.jackson.databind.ObjectMapper();

    /**
     * Fail-fast 版本：在 dto 層驗證 description.blocks（inline-run 模型，02 §3.6）：
     * <ul>
     *   <li>text block：每個 run 的 {@code text} 禁 HTML tag（3014）、{@code href} 僅 http(s)（3020）；runs 缺時 fallback 驗 legacy content</li>
     *   <li>table block：table_json ≤ 20KB、欄 ≤ 10、列 ≤ 50、單 cell ≤ 200 字、非空表格（違反 INVALID_ARGUMENT）；cell run 同 text/href 驗證</li>
     *   <li>list block：list_json ≤ 20KB、項目 ≤ 50、單項 ≤ 200 字、非空清單（違反 INVALID_ARGUMENT）；item run 同 text/href 驗證</li>
     * </ul>
     * 避免 INSERT/FinalizeImages 後才報錯，確保前端能依 envelope.code 識別具體錯誤類型。
     */
    private void validateDescriptionDtoTextBlocks(io.github.will413028.hawkr.product.dto.request.ProductDescriptionDTO description) {
        if (description == null || description.getBlocks() == null) return;
        for (io.github.will413028.hawkr.product.dto.request.DescriptionBlockDTO block : description.getBlocks()) {
            if (block == null || block.getType() == null) continue;
            switch (block.getType()) {
                case "text" -> validateTextBlock(block);
                case "table" -> validateTableBlock(block);
                case "list" -> validateListBlock(block);
                default -> { /* image / 其他 type 無文字驗證 */ }
            }
        }
    }

    /** text block：runs 逐一驗 text 禁 HTML + href scheme；runs 缺時驗 legacy content。 */
    private void validateTextBlock(io.github.will413028.hawkr.product.dto.request.DescriptionBlockDTO block) {
        if (block.getRuns() != null && !block.getRuns().isEmpty()) {
            for (io.github.will413028.hawkr.product.dto.request.InlineRunDTO run : block.getRuns()) {
                if (run == null) continue;
                requireRunTextNoHtml(run.getText());
                requireHrefHttpScheme(run.getHref());
            }
        } else {
            requireRunTextNoHtml(block.getContent());
        }
    }

    /** table block：上限 + cell run 驗證。 */
    private void validateTableBlock(io.github.will413028.hawkr.product.dto.request.DescriptionBlockDTO block) {
        String tableJson = block.getTableJson();
        if (tableJson == null || tableJson.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格內容不可為空");
        }
        if (tableJson.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > MAX_TABLE_JSON_BYTES) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格資料過大（上限 20KB）");
        }
        final com.fasterxml.jackson.databind.JsonNode root;
        try {
            root = DESC_MAPPER.readTree(tableJson);
        } catch (Exception e) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格資料格式錯誤");
        }
        com.fasterxml.jackson.databind.JsonNode rows = root.path("rows");
        if (!rows.isArray() || rows.isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格至少需 1 列 1 欄");
        }
        if (rows.size() > MAX_TABLE_ROWS) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格列數超出上限（最多 " + MAX_TABLE_ROWS + " 列）");
        }
        for (com.fasterxml.jackson.databind.JsonNode row : rows) {
            if (!row.isArray() || row.isEmpty()) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格至少需 1 列 1 欄");
            }
            if (row.size() > MAX_TABLE_COLS) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格欄數超出上限（最多 " + MAX_TABLE_COLS + " 欄）");
            }
            for (com.fasterxml.jackson.databind.JsonNode cell : row) {
                com.fasterxml.jackson.databind.JsonNode cellRuns = cell.path("runs");
                if (!cellRuns.isArray()) continue;
                StringBuilder cellText = new StringBuilder();
                for (com.fasterxml.jackson.databind.JsonNode run : cellRuns) {
                    String t = run.path("text").asText("");
                    cellText.append(t);
                    requireRunTextNoHtml(t);
                    requireHrefHttpScheme(run.path("href").asText(null));
                }
                if (cellText.length() > MAX_CELL_TEXT_LEN) {
                    throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "表格單一儲存格字數超出上限（最多 " + MAX_CELL_TEXT_LEN + " 字）");
                }
            }
        }
    }

    /** list block：上限 + item run 驗證。 */
    private void validateListBlock(io.github.will413028.hawkr.product.dto.request.DescriptionBlockDTO block) {
        String listJson = block.getListJson();
        if (listJson == null || listJson.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單內容不可為空");
        }
        if (listJson.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > MAX_LIST_JSON_BYTES) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單資料過大（上限 20KB）");
        }
        final com.fasterxml.jackson.databind.JsonNode root;
        try {
            root = DESC_MAPPER.readTree(listJson);
        } catch (Exception e) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單資料格式錯誤");
        }
        com.fasterxml.jackson.databind.JsonNode items = root.path("items");
        if (!items.isArray() || items.isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單至少需 1 個項目");
        }
        if (items.size() > MAX_LIST_ITEMS) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單項目數超出上限（最多 " + MAX_LIST_ITEMS + " 項）");
        }
        for (com.fasterxml.jackson.databind.JsonNode item : items) {
            com.fasterxml.jackson.databind.JsonNode itemRuns = item.path("runs");
            if (!itemRuns.isArray()) continue;
            StringBuilder itemText = new StringBuilder();
            for (com.fasterxml.jackson.databind.JsonNode run : itemRuns) {
                String t = run.path("text").asText("");
                itemText.append(t);
                requireRunTextNoHtml(t);
                requireHrefHttpScheme(run.path("href").asText(null));
            }
            if (itemText.length() > MAX_CELL_TEXT_LEN) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "清單單一項目字數超出上限（最多 " + MAX_CELL_TEXT_LEN + " 字）");
            }
        }
    }

    /** run text 禁 HTML tag（3014）。 */
    private void requireRunTextNoHtml(String text) {
        try {
            InputValidator.requireNoHtmlTags(text, "商品描述");
        } catch (IllegalArgumentException e) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_DESCRIPTION_HTML_FORBIDDEN, e.getMessage());
        }
    }

    /** href 僅接受 http(s):// 絕對 URL（3020）；空值放行。 */
    private void requireHrefHttpScheme(String href) {
        if (href == null || href.isBlank()) return;
        String lower = href.trim().toLowerCase();
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
            throw new ProductBusinessException(
                    GrpcResponseCode.INVALID_DESCRIPTION_HREF,
                    "商品描述連結網址格式不支援（僅接受 http/https）");
        }
    }

    /**
     * 計算並寫入 product 反正規化文字欄位（02 §3.2 / §3.6 / §3.9）：
     * <ul>
     *   <li>{@code cat_l1/l2/l3_name}：依 entity 三層 category id 取 {@link CategoryService#getCategoryNames}</li>
     *   <li>{@code pure_description}：用 {@link io.github.will413028.hawkr.product.domain.text.DescriptionTextExtractor}</li>
     *   <li>{@code searchable_text}：用 {@link io.github.will413028.hawkr.product.domain.text.SearchableTextBuilder}（8 KB 上限）</li>
     * </ul>
     */
    private void enrichSearchableFields(
            ProductEntity entity,
            List<HashtagService.UpsertedHashtag> hashtags,
            List<ProductSkuEntity> skus) {
        // 三層分類名稱快照
        Set<Long> ids = new HashSet<>();
        if (entity.getCategoryLv1() != null) ids.add(entity.getCategoryLv1());
        if (entity.getCategoryLv2() != null) ids.add(entity.getCategoryLv2());
        if (entity.getCategory() != null) ids.add(entity.getCategory());
        Map<Long, String> nameMap = ids.isEmpty() ? Map.of() : categoryService.getCategoryNames(ids);
        entity.setCatL1Name(nameMap.get(entity.getCategoryLv1()));
        entity.setCatL2Name(nameMap.get(entity.getCategoryLv2()));
        entity.setCatL3Name(nameMap.get(entity.getCategory()));

        // pure_description
        String pureDesc = io.github.will413028.hawkr.product.domain.text.DescriptionTextExtractor.extract(entity.getDescription());
        entity.setPureDescription(pureDesc);

        // searchable_text 拼接
        List<String> hashtagCanonicals = (hashtags == null) ? List.of() :
                hashtags.stream().map(HashtagService.UpsertedHashtag::canonical).toList();
        List<String> skuNames = (skus == null) ? List.of() :
                skus.stream()
                        .map(ProductSkuEntity::getSkuName)
                        .filter(Objects::nonNull)
                        .toList();
        String searchableText = io.github.will413028.hawkr.product.domain.text.SearchableTextBuilder.build(
                entity.getId(),
                entity.getName(),
                hashtagCanonicals,
                skuNames,
                entity.getCatL1Name(),
                entity.getCatL2Name(),
                entity.getCatL3Name(),
                pureDesc);
        entity.setSearchableText(searchableText);
    }

    // ==================== Description 圖片處理方法 ====================

    /**
     * 處理新建商品的描述圖片
     * 將 description blocks 中的 staged_token 轉換為正式 asset
     *
     * @param productId 商品 ID
     * @param description 商品描述
     */
    private void processDescriptionImages(Long productId, ProductEntity.ProductDescription description) {
        if (description == null || description.getBlocks() == null || description.getBlocks().isEmpty()) {
            return;
        }

        // 收集所有 image 類型 block 的 staged_token
        List<ProductEntity.DescriptionBlock> imageBlocks = description.getBlocks().stream()
                .filter(block -> "image".equals(block.getType()))
                .filter(block -> block.getStagedToken() != null && !block.getStagedToken().isEmpty())
                .collect(Collectors.toList());

        if (imageBlocks.isEmpty()) {
            return;
        }

        List<String> stagedTokens = imageBlocks.stream()
                .map(ProductEntity.DescriptionBlock::getStagedToken)
                .collect(Collectors.toList());

        // 轉正失敗往外拋，理由同 processImages。這裡吞掉的形狀更隱蔽：block 會留著
        // staged_token 落庫，而那是一個短期憑證，過期之後那個 block 永遠指不到任何資產。
        List<MediaGrpcClient.FinalizedImage> finalized =
                mediaGrpcClient.finalizeImages(stagedTokens, AssetReferenceType.PRODUCT_DESC.value());

        // 依 staged_token 建立 mapping（media.proto:41 契約要求），避免依賴回傳順序
        Map<String, MediaGrpcClient.FinalizedImage> tokenToFinalized = finalized.stream()
                .collect(Collectors.toMap(MediaGrpcClient.FinalizedImage::getStagedToken, f -> f, (a, b) -> a));

        // 更新 description blocks
        for (ProductEntity.DescriptionBlock block : imageBlocks) {
            MediaGrpcClient.FinalizedImage img = tokenToFinalized.get(block.getStagedToken());
            if (img != null) {
                block.setAssetId(img.getAssetId());
                block.setObjectKey(img.getObjectKey());
                block.setStagedToken(null); // 清除 staged_token
            }
        }

        log.info("descriptionImagesProcessed", "描述圖片處理完成，product_id=" + productId + "，image_count=" + imageBlocks.size());
    }

    /**
     * 處理更新商品的描述圖片
     * 比對新舊 description blocks，處理新增和刪除的圖片
     *
     * @param productId 商品 ID
     * @param oldDescription 原有描述
     * @param newDescription 新描述
     */
    private void processDescriptionImageUpdate(
            Long productId,
            ProductEntity.ProductDescription oldDescription,
            ProductEntity.ProductDescription newDescription
    ) {
        if (newDescription == null) {
            return;
        }

        // 收集新描述中需要轉正的圖片（有 staged_token）
        List<ProductEntity.DescriptionBlock> newImageBlocks = new ArrayList<>();
        if (newDescription.getBlocks() != null) {
            for (ProductEntity.DescriptionBlock block : newDescription.getBlocks()) {
                if ("image".equals(block.getType())
                        && block.getStagedToken() != null && !block.getStagedToken().isEmpty()) {
                    newImageBlocks.add(block);
                }
            }
        }

        // 處理新增的圖片（僅轉正，不建立引用）。轉正失敗往外拋，理由同
        // processDescriptionImages：吞掉會把短期憑證 staged_token 留在 block 裡落庫。
        if (!newImageBlocks.isEmpty()) {
            List<String> stagedTokens = newImageBlocks.stream()
                    .map(ProductEntity.DescriptionBlock::getStagedToken)
                    .collect(Collectors.toList());

            List<MediaGrpcClient.FinalizedImage> finalized =
                    mediaGrpcClient.finalizeImages(stagedTokens, AssetReferenceType.PRODUCT_DESC.value());

            // 依 staged_token 建立 mapping（media.proto:41 契約要求），避免依賴回傳順序
            Map<String, MediaGrpcClient.FinalizedImage> tokenToFinalized = finalized.stream()
                    .collect(Collectors.toMap(MediaGrpcClient.FinalizedImage::getStagedToken, f -> f, (a, b) -> a));

            // 更新 description blocks
            for (ProductEntity.DescriptionBlock block : newImageBlocks) {
                MediaGrpcClient.FinalizedImage img = tokenToFinalized.get(block.getStagedToken());
                if (img != null) {
                    block.setAssetId(img.getAssetId());
                    block.setObjectKey(img.getObjectKey());
                    block.setStagedToken(null); // 清除 staged_token
                }
            }

            log.info("descriptionImagesFinalized", "描述圖片已確認，product_id=" + productId + "，finalized_count=" + newImageBlocks.size());
        }
    }

    // ==================== Media 引用關係方法（業務落盤後呼叫） ====================

    /**
     * 建立商品展示圖片的 Media 引用關係。
     * <p>
     * 在同一個 {@code @Transactional} 內、業務資料寫入之後呼叫，且**失敗必須往外拋**（#2222）：
     * 拋出 → 交易回滾 → product 列不存在 → 剛轉正的資產無人引用，正好落在 media GC 的孤兒
     * 回收範圍。反之吞掉錯誤就會 commit 出「product.gallery 指著 assetId、卻沒有 asset_reference」
     * 的列，7 天後 GC 把正在販售的商品圖連同 R2 物件實體刪除。
     */
    private void linkGalleryImages(Long productId, List<ProductEntity.GalleryItem> gallery, String refType) {
        List<String> assetIds = collectGalleryAssetIds(gallery);
        if (assetIds.isEmpty()) {
            return;
        }
        mediaGrpcClient.linkImages(assetIds, refType, productId.toString());
    }

    /**
     * 建立商品描述圖片的 Media 引用關係。失敗必須往外拋（理由同 {@link #linkGalleryImages}）。
     */
    private void linkDescriptionImages(Long productId, ProductEntity.ProductDescription description, String refType) {
        List<String> assetIds = collectDescriptionAssetIds(description);
        if (assetIds.isEmpty()) {
            return;
        }
        mediaGrpcClient.linkImages(assetIds, refType, productId.toString());
    }

    /**
     * 比對新舊商品展示圖片，建立/解除 Media 引用關係
     * 新增的 assetId → LinkImages，移除的 assetId → UnlinkImages
     * 在業務資料落盤後呼叫
     */
    private void linkAndUnlinkGalleryImages(
            Long productId,
            List<ProductEntity.GalleryItem> oldGallery,
            List<ProductEntity.GalleryItem> newGallery
    ) {
        Set<String> oldAssetIds = new HashSet<>(collectGalleryAssetIds(oldGallery));
        Set<String> newAssetIds = new HashSet<>(collectGalleryAssetIds(newGallery));

        // 新增的 assetId → 建立引用
        List<String> toLink = newAssetIds.stream()
                .filter(id -> !oldAssetIds.contains(id))
                .collect(Collectors.toList());
        // Link 失敗往外拋 → 交易回滾（#2222）：吞掉會 commit 出「gallery 指著、卻沒有引用」的列。
        if (!toLink.isEmpty()) {
            mediaGrpcClient.linkImages(toLink, AssetReferenceType.PRODUCT.value(), productId.toString());
        }

        // 移除的 assetId → 解除引用
        List<String> toUnlink = oldAssetIds.stream()
                .filter(id -> !newAssetIds.contains(id))
                .collect(Collectors.toList());
        if (!toUnlink.isEmpty()) {
            try {
                mediaGrpcClient.unlinkImages(toUnlink, AssetReferenceType.PRODUCT.value(), productId.toString());
            } catch (Exception e) {
                log.error("galleryUnlinkFailed", "商品圖庫引用解除失敗，product_id=" + productId + "，error=" + e.getMessage(), e);
            }
        }
    }

    /**
     * 比對新舊商品描述圖片，建立/解除 Media 引用關係
     * 新增的 assetId → LinkImages，移除的 assetId → UnlinkImages
     * 在業務資料落盤後呼叫
     */
    private void linkAndUnlinkDescriptionImages(
            Long productId,
            ProductEntity.ProductDescription oldDescription,
            ProductEntity.ProductDescription newDescription
    ) {
        Set<String> oldAssetIds = new HashSet<>(collectDescriptionAssetIds(oldDescription));
        Set<String> newAssetIds = new HashSet<>(collectDescriptionAssetIds(newDescription));

        // 新增的 assetId → 建立引用
        List<String> toLink = newAssetIds.stream()
                .filter(id -> !oldAssetIds.contains(id))
                .collect(Collectors.toList());
        // Link 失敗往外拋 → 交易回滾（#2222），理由同 gallery。
        if (!toLink.isEmpty()) {
            mediaGrpcClient.linkImages(toLink, AssetReferenceType.PRODUCT_DESC.value(), productId.toString());
        }

        // 移除的 assetId → 解除引用
        List<String> toUnlink = oldAssetIds.stream()
                .filter(id -> !newAssetIds.contains(id))
                .collect(Collectors.toList());
        if (!toUnlink.isEmpty()) {
            try {
                mediaGrpcClient.unlinkImages(toUnlink, AssetReferenceType.PRODUCT_DESC.value(), productId.toString());
            } catch (Exception e) {
                log.error("descriptionUnlinkFailed", "描述圖片引用解除失敗，product_id=" + productId + "，error=" + e.getMessage(), e);
            }
        }
    }

    /**
     * 商品刪除時釋放展示圖與描述圖的 Media 引用關係（#1489）。
     * <p>
     * 依 (ref_type, ref_id) 批次釋放 PRODUCT / PRODUCT_DESC 全部引用；釋放後資產若無其他引用
     * 即成孤兒，由 media-service GC 回收 OSS。best-effort：釋放失敗僅記錄，不回滾商品刪除
     * （與 create/update 的 link/unlink 一致，避免 media 暫時性故障擋住主流程）。
     */
    private void releaseProductMediaReferences(Long productId) {
        List<String> refIds = List.of(productId.toString());
        try {
            mediaGrpcClient.releaseReferences(AssetReferenceType.PRODUCT.value(), refIds);
        } catch (Exception e) {
            log.error("productMediaReleaseFailed", "商品展示圖引用釋放失敗（商品已刪除），product_id=" + productId + "，error=" + e.getMessage(), e);
        }
        try {
            mediaGrpcClient.releaseReferences(AssetReferenceType.PRODUCT_DESC.value(), refIds);
        } catch (Exception e) {
            log.error("descriptionMediaReleaseFailed", "商品描述圖引用釋放失敗（商品已刪除），product_id=" + productId + "，error=" + e.getMessage(), e);
        }
    }

    /**
     * 從 gallery 中收集所有有效的 assetId（委派共用元件，避免重複實作 docs/開發規範/Java 微服務開發規範.md §15）
     */
    private List<String> collectGalleryAssetIds(List<ProductEntity.GalleryItem> gallery) {
        return mediaReferenceLinker.collectGalleryAssetIds(gallery);
    }

    /**
     * 從 description 的 image blocks 中收集所有有效的 assetId（委派共用元件，避免重複實作 docs/開發規範/Java 微服務開發規範.md §15）
     */
    private List<String> collectDescriptionAssetIds(ProductEntity.ProductDescription description) {
        return mediaReferenceLinker.collectDescriptionAssetIds(description);
    }

    // ==================== 輕量查詢（Cart Service 呼叫） ====================

    /**
     * 根據 SKU ID 查詢 SKU（Cart AddItem 驗證用）
     */
    public Optional<ProductSkuEntity> findSkuById(Long skuId) {
        return productSkuRepository.findById(skuId);
    }

    /**
     * 批次查詢 SKU（購物車列表即時資訊用）
     * 回傳包含 shippingMethod JSONB 和 stockQty
     */
    public List<ProductSkuEntity> findSkusByIds(List<Long> skuIds) {
        if (skuIds == null || skuIds.isEmpty()) {
            return List.of();
        }
        return productSkuRepository.selectBatchIds(skuIds);
    }

    /**
     * 購物車批次查詢（02 §5.5.3 BatchGetSkuCartInfo 用）。
     * <p>
     * 單階段 LEFT JOIN 一次拿 SKU + Product 必要欄位（單一 PG snapshot），
     * 取代既有兩階段「findSkusByIds + findProductsByIds」查詢，避免並行
     * UpdateProductStatus / DeductStock 期間 snapshot 漂移造成 cart 假 DELISTED（#1133）。
     */
    public List<io.github.will413028.hawkr.product.entity.projection.SkuCartProjection> findSkuCartInfoByIds(List<Long> skuIds) {
        if (skuIds == null || skuIds.isEmpty()) {
            return List.of();
        }
        return productSkuRepository.findCartInfoBySkuIds(skuIds);
    }

    /**
     * 根據商品 ID 查詢商品（不含權限檢查，內部服務呼叫用）
     */
    public Optional<ProductEntity> findProductById(Long productId) {
        return productRepository.findById(productId);
    }

    /**
     * 批次查詢商品 Entity（不含權限檢查，內部服務呼叫用）
     */
    public List<ProductEntity> findProductsByIds(List<Long> productIds) {
        if (productIds == null || productIds.isEmpty()) {
            return List.of();
        }
        return productRepository.selectBatchIds(productIds);
    }

    /**
     * 批次查詢商品（一律排除軟刪除；ACTIVE 過濾由 includeInactive 決定）。
     * 詳見 02 §5.5.1。
     *
     * @param productIds      商品 ID 列表
     * @param includeInactive false=僅 ACTIVE（瀏覽紀錄路徑）；true=含 DRAFT/PENDING_REVIEW/INACTIVE（favorites enrichment 路徑）
     */
    public List<ProductListItemDTO> getProductsByIds(List<Long> productIds, boolean includeInactive) {
        if (productIds == null || productIds.isEmpty()) {
            return List.of();
        }

        List<ProductEntity> entities = productRepository.selectBatchIds(productIds);

        // includeInactive=false 是買家可見的路徑：店家不可曝光（含沒有 store_exposure 列）的商品不回
        Set<Long> exposableStores = Set.of();
        if (!includeInactive) {
            Set<Long> storeIds = entities.stream().map(ProductEntity::getStoreId)
                    .filter(Objects::nonNull).collect(Collectors.toSet());
            if (!storeIds.isEmpty()) {
                exposableStores = new HashSet<>(storeExposureRepository.findExposableStoreIds(storeIds));
            }
        }
        final Set<Long> visibleStores = exposableStores;

        return entities.stream()
                .filter(e -> e.getDeletedAt() == null)
                .filter(e -> includeInactive
                        || (ProductStatus.ACTIVE.value().equals(e.getStatus()) && visibleStores.contains(e.getStoreId())))
                .map(productEntityConverter::toListItemDTO)
                .collect(Collectors.toList());
    }

    /**
     * 軟刪除商品（賣家用）
     * 僅 DRAFT(4001) 或 INACTIVE(4003) 狀態的商品可以刪除
     *
     * @param productId 商品 ID（TSID 數字字串）
     * @param storeId 操作者 store 代理鍵
     * @throws ProductNotFoundException 商品不存在
     * @throws ProductAccessDeniedException 非商品擁有者
     * @throws IllegalStateException 商品狀態不允許刪除
     */
    @Transactional
    public void softDeleteProduct(String productId, long storeId) {
        Long id = Long.parseLong(productId);

        // 1. 查詢商品
        ProductEntity entity = productRepository.findById(id)
                .orElseThrow(() -> new ProductNotFoundException(productId));

        // 2. 檢查已刪除
        if (entity.getDeletedAt() != null) {
            throw new ProductNotFoundException(productId);
        }

        // 3. 驗證擁有者
        if (!entity.getStoreId().equals(storeId)) {
            throw new ProductAccessDeniedException(productId, "非商品擁有者");
        }

        boolean wasActive = ProductStatus.ACTIVE.value().equals(entity.getStatus());

        // 3.2 連帶撤回待審的認證申請（#2820）：softDeleteProduct 原本完全不觸及
        //     product_certification_application，而 19 §8.3 明令禁止掃 PENDING 的排程
        //     ⇒ SUBMITTED 的申請會永久留在 ADM 待審佇列，且沒有任何路徑會收掉。
        //     只收 SUBMITTED，APPROVED 刻意保留（理由見該方法 javadoc）。
        //     跨 bean 呼叫，本方法的 @Transactional 會傳播過去（§11.1 禁的是 self-invocation）。
        certificationApplicationService.withdrawSubmittedOnProductDeleted(id);

        // 4. 軟刪除：公開生命週期會由 deleted_at 推導為 REMOVED，不另寫第二套狀態。
        productRepository.deleteById(id);

        // 5. 釋放展示圖 / 描述圖的 Media 引用（#1489）：刪除後不再引用，讓 GC 回收孤兒資產。
        //    與 create/update 的 link/unlink 一致，緊接業務寫入呼叫、best-effort（失敗不擋刪除）。
        releaseProductMediaReferences(id);

        // 6. 清除快取
        cacheService.evictProductDetail(productId);
        cacheService.evictProductListForWrite(storeId);

        // 7. 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）
        // 刪除前為 ACTIVE（在公開貨架上）→ 賣家公開商品集合變動，共發 seller:<id>；
        // 非 ACTIVE 的刪除不改公開集合，只需 product:<id> 讓 PDP 失效。
        if (wasActive) {
            revalidateProductAndSeller(id, storeId);
        } else {
            revalidateProduct(id);
        }

        log.info("ProductService.softDeleteProduct.success",
                "商品軟刪除成功，product_id=" + productId,
                "store_id", String.valueOf(storeId));
    }

    /**
     * 公開商品產地聚合（02 §5.13）。
     *
     * @param categoryId 分類 ID（"0" / null / 空字串 = 全部）
     * @param limit 上限（service clamp 至 1~100）
     */
    public List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow>
            getPublicOriginsSummary(Long categoryId, int limit) {
        // limit clamp
        int effectiveLimit = limit;
        if (effectiveLimit <= 0) effectiveLimit = 50;
        if (effectiveLimit > 100) effectiveLimit = 100;

        // 分類層級偵測（§5.3.1 同邏輯）
        int categoryLevel = 0;
        Long categoryFilter = categoryId;
        if (categoryFilter != null && categoryFilter > 0) {
            categoryLevel = categoryService.detectCategoryLevel(categoryFilter);
            if (categoryLevel == -1) {
                throw new IllegalArgumentException("分類 ID " + categoryFilter + " 不存在");
            }
        } else {
            categoryFilter = null;
        }

        // cache key
        String cacheKey = cacheService.generateOriginsSummaryCacheKey(categoryFilter, effectiveLimit);
        Optional<List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow>> cached =
                cacheService.getOriginsSummary(cacheKey);
        if (cached.isPresent()) {
            log.info("originsSummary.cacheHit", "產地聚合快取命中", "cache_key", cacheKey);
            return cached.get();
        }

        List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow> rows =
                productRepository.findPublicOriginsSummary(categoryFilter, categoryLevel, effectiveLimit);

        cacheService.cacheOriginsSummary(cacheKey, rows);
        log.info("originsSummary.success", "查詢產地聚合，count=" + rows.size());
        return rows;
    }

    public List<io.github.will413028.hawkr.product.domain.ProductSellerCitySummaryPolicy.Summary>
            getPublicSellerCitiesSummary() {
        var cities = sellerGrpcClient.listPublicStoresByCity();
        Map<Long, Long> productCounts = productRepository.findPublicProductCountsByStore().stream()
                .filter(row -> row.getStoreId() != null)
                .collect(Collectors.toMap(
                        io.github.will413028.hawkr.product.repository.ProductRepository.StoreProductCountRow::getStoreId,
                        row -> row.getProductCount() != null ? row.getProductCount() : 0L));
        return io.github.will413028.hawkr.product.domain.ProductSellerCitySummaryPolicy.aggregate(cities, productCounts);
    }

    /**
     * 回傳「至少有一件公開上架商品」的 store_id 全集（02 §5.13c）。
     *
     * <p>與 {@link #getPublicSellerCitiesSummary()} 共用同一句聚合
     * （{@code findPublicProductCountsByStore}，deleted_at IS NULL AND seller_deleted_at IS NULL
     * AND status = '4004' GROUP BY store_id），因此「什麼叫公開上架」在兩條路徑上
     * 只有一份定義；GROUP BY 本身即保證每一列的 count &gt;= 1，這裡不再另設門檻。
     *
     * <p>呼叫端是 seller-service 的推薦賣家 / marketplace rail，用來把「0 件在架商品」
     * 的店排除在推薦名單外。
     */
    public List<Long> listStoreIDsWithPublicProducts() {
        return productRepository.findPublicProductCountsByStore().stream()
                .map(io.github.will413028.hawkr.product.repository.ProductRepository.StoreProductCountRow::getStoreId)
                .filter(Objects::nonNull)
                .toList();
    }

    /**
     * 從 Seller Service 建構商家資訊 DTO（list 與 detail 共用 SellerInfoAssembler）
     * 失敗時降級為僅含 storeId 的基本資訊
     */
    private SellerInfoDTO buildSellerInfo(Long storeId) {
        if (storeId == null) {
            return sellerInfoAssembler.from(null, null);
        }
        SellerMeta meta = sellerGrpcClient.getSellerMeta(storeId);
        return sellerInfoAssembler.from(storeId, meta);
    }

    // ==================== Admin 分類管理（搭配 CategoryService 軟刪除/批次更換） ====================

    /**
     * 是否有商品掛在指定分類下（依層級用對應反正規化欄位查）。
     * 給 {@link CategoryService.ProductExistsChecker} 注入用。
     */
    public boolean hasProductsInCategory(Long categoryId, int level) {
        if (categoryId == null || level < 1 || level > 3) {
            return false;
        }
        return productRepository.existsInCategoryLevel(categoryId, level);
    }

    /**
     * 取分類底下所有商品（admin 用，含分頁）。
     */
    public ProductListResultDTO listProductsByCategory(Long categoryId, int page, int pageSize) {
        if (categoryId == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "category_id 不可為空");
        }
        int level = categoryService.detectCategoryLevel(categoryId);
        if (level < 1) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND);
        }

        ProductQueryDTO query = new ProductQueryDTO();
        query.setCategoryIds(java.util.List.of(categoryId));
        query.setPage(page);
        query.setPageSize(pageSize);
        return getProductList(query);
    }

    /**
     * 批次更換商品分類（admin 用）。<strong>強制連動更新 lv1/lv2 三欄位</strong>。
     *
     * @param productIds 商品 ID 列表（與 sourceCategoryId 擇一）
     * @param sourceCategoryId 來源分類 ID（限 L3，與 productIds 擇一）
     * @param targetCategoryId 目標分類 ID（限 L3，必填）
     * @param updatedBy 更新者
     * @return 受影響商品筆數
     */
    @Transactional
    public int batchChangeProductCategory(
            List<Long> productIds,
            Long sourceCategoryId,
            Long targetCategoryId,
            Long updatedBy) {

        if (targetCategoryId == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "target_category_id 不可為空");
        }
        boolean hasIds = productIds != null && !productIds.isEmpty();
        boolean hasSource = sourceCategoryId != null;
        if (hasIds == hasSource) {
            throw new ProductBusinessException(
                    GrpcResponseCode.INVALID_PARAM,
                    "product_ids 與 source_category_id 擇一傳入");
        }

        int targetLevel = categoryService.detectCategoryLevel(targetCategoryId);
        if (targetLevel == -1) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND, "target_category_id 不存在");
        }
        if (targetLevel != 3) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_LEVEL_INVALID, "target_category_id 必須為第三層");
        }
        if (!categoryService.isActive(targetCategoryId)) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND, "target_category_id 已被軟刪除");
        }

        Long lv2Id = categoryService.getParentId(targetCategoryId);
        Long lv1Id = lv2Id != null ? categoryService.getLv1Id(lv2Id) : null;
        if (lv2Id == null || lv1Id == null) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND, "目標分類層級不完整");
        }

        List<Long> ids;
        if (hasIds) {
            ids = productIds;
        } else {
            int sourceLevel = categoryService.detectCategoryLevel(sourceCategoryId);
            if (sourceLevel == -1) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND, "source_category_id 不存在");
            }
            if (sourceLevel != 3) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_LEVEL_INVALID, "source_category_id 必須為第三層");
            }
            ids = productRepository.findIdsByCategoryLevel(sourceCategoryId, 3);
        }
        if (ids.isEmpty()) {
            return 0;
        }

        int affected = productRepository.batchUpdateCategoryHierarchy(
                ids, targetCategoryId, lv2Id, lv1Id, updatedBy);

        // Cascade reflow（02 §5.11.4）：同事務內重算受影響 product 的 cat_*_name + searchable_text
        productCascadeService.reflow(ids, "BATCH_CHANGE_CATEGORY");

        cacheService.evictAllProductLists();
        for (Long id : ids) {
            cacheService.evictProductDetail(String.valueOf(id));
        }

        // frontend cache bust（§9.3）：改分類影響 cat_*_name / searchable_text，顯示於 PDP 與
        // storefront 商品卡；不改變任何賣家的公開商品集合 → 只發 product:<id>×N，
        // 列表面由 web expansion（product:<id> → products:list）涵蓋。
        productRevalidationService.revalidateProducts(ids);

        log.info("batchChangeProductCategory.success",
                "affected=" + affected + "，target=" + targetCategoryId + "（lv2=" + lv2Id + "，lv1=" + lv1Id + "）");
        return affected;
    }
}
