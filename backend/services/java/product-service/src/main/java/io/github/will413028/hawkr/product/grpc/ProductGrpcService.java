package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.DelegationCapability;
import io.github.will413028.hawkr.common.grpc.DelegationContext;
import io.github.will413028.hawkr.common.grpc.DelegationContextValidator;
import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.grpc.GrpcServiceHelper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.redis.RedisPatternEvictor;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.code.StockOperationStatus;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingArea;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.common.utils.shipping.ShippingAreaResolver;
import io.github.will413028.hawkr.common.utils.shipping.WeightConverter;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.product.domain.ProductPublicationPolicy;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.dto.request.*;
import io.github.will413028.hawkr.product.dto.response.*;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.product.exception.IdempotencyConflictException;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.exception.ProductAccessDeniedException;
import io.github.will413028.hawkr.product.exception.ProductNotFoundException;
import io.github.will413028.hawkr.product.exception.ProductPublicNotFoundException;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.entity.projection.SkuCartProjection;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.github.will413028.hawkr.product.service.ProductService;
import io.github.will413028.hawkr.product.service.CategoryCacheService;
import io.github.will413028.hawkr.product.service.ProductCacheService;
import io.github.will413028.hawkr.product.service.StockService;
import io.github.will413028.hawkr.product.service.GovCertificationService;
import io.github.will413028.hawkr.product.service.ProductCertificationApplicationService;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.product.domain.CertificationTypeRules;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;

import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Product Service gRPC Handler
 * 實作商品 CRUD、快取管理、庫存操作等 RPC
 */
@GrpcService
public class ProductGrpcService extends ProductServiceGrpc.ProductServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(ProductGrpcService.class);
    private static final String SKU_IDS_REQUIRED_MESSAGE = "sku_ids 不可為空";

    /** 代行批次的單次上限（56 §6.1.8；Gateway binding tag 同值，此處為收斂點的第二道） */
    private static final int BATCH_MAX_ITEMS = 50;
    private static final ObjectMapper SHIPPING_METHOD_MAPPER = new ObjectMapper();
    private static final TypeReference<List<ProductSkuEntity.ShippingMethodItem>> SHIPPING_METHOD_TYPE_REF = new TypeReference<>() {};

    private final ProductService productService;
    private final ProductProtoConverter protoConverter;
    private final ProductCacheService cacheService;
    private final CategoryCacheService categoryCacheService;
    private final StringRedisTemplate redisTemplate;
    private final StockService stockService;
    private final SellerGrpcClient sellerGrpcClient;
    private final io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient shippingGrpcClient;
    private final ProductSkuRepository skuRepository;
    private final GovCertificationService govCertificationService;
    private final io.github.will413028.hawkr.product.service.HashtagService hashtagService;
    private final io.github.will413028.hawkr.product.service.PopularHashtagService popularHashtagService;
    private final io.github.will413028.hawkr.product.service.ProductImportService productImportService;
    private final io.github.will413028.hawkr.product.service.validator.ProductPublishValidator productPublishValidator;
    private final io.github.will413028.hawkr.product.repository.ProductRepository productRepository;
    private final io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService;
    private final io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter;

    public ProductGrpcService(
            ProductService productService,
            ProductProtoConverter protoConverter,
            ProductCacheService cacheService,
            CategoryCacheService categoryCacheService,
            StringRedisTemplate redisTemplate,
            StockService stockService,
            SellerGrpcClient sellerGrpcClient,
            io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient shippingGrpcClient,
            ProductSkuRepository skuRepository,
            GovCertificationService govCertificationService,
            io.github.will413028.hawkr.product.service.HashtagService hashtagService,
            io.github.will413028.hawkr.product.service.PopularHashtagService popularHashtagService,
            io.github.will413028.hawkr.product.service.ProductImportService productImportService,
            io.github.will413028.hawkr.product.service.validator.ProductPublishValidator productPublishValidator,
            io.github.will413028.hawkr.product.repository.ProductRepository productRepository,
            io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService,
            io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter
    ) {
        this.productService = productService;
        this.protoConverter = protoConverter;
        this.cacheService = cacheService;
        this.categoryCacheService = categoryCacheService;
        this.redisTemplate = redisTemplate;
        this.stockService = stockService;
        this.sellerGrpcClient = sellerGrpcClient;
        this.shippingGrpcClient = shippingGrpcClient;
        this.skuRepository = skuRepository;
        this.govCertificationService = govCertificationService;
        this.hashtagService = hashtagService;
        this.popularHashtagService = popularHashtagService;
        this.productImportService = productImportService;
        this.productPublishValidator = productPublishValidator;
        this.productRepository = productRepository;
        this.certificationApplicationService = certificationApplicationService;
        this.certificationConverter = certificationConverter;
    }

    /**
     * 建立商品
     */
    @Override
    public void createProduct(CreateProductRequest request, StreamObserver<CreateProductResponse> responseObserver) {
        try {
            // 授權：賣家本人或代行（products.write）
            GrpcAuthContext.requireSellerOrDelegated(delegationOrNull(request.hasDelegation(), request.getDelegation()),
                    request.getStoreId(), DelegationCapability.PRODUCTS_WRITE, request.getIdempotencyKey());

            // 驗證必填欄位
            if (request.getIdempotencyKey().isEmpty()) {
                responseObserver.onNext(CreateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("idempotency_key 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (request.getName().isEmpty()) {
                responseObserver.onNext(CreateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("商品名稱為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 至少一個 SKU：商品的價格 / 庫存 / 材積全部掛在 SKU 上，零 SKU 商品建出來就是
            // 上架但不可購買（priceMin=0、stock=0）。此檢查放在收斂點（gRPC 入口）而非只在
            // Gateway，避免「賭 Gateway 是唯一入口」。
            if (request.getSkusCount() == 0) {
                responseObserver.onNext(CreateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("至少需要一個商品規格 (SKU)")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換 Proto → DTO
            CreateProductDTO dto = protoConverter.toCreateDTO(request);

            // 呼叫 Service
            ProductWriteOutcome outcome = productService.createProductWithWarnings(dto);
            ProductDetailDTO result = outcome.detail();

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            // 19 號設計書 §9.5：本次未被採用的認證。product-service 只填 CONCURRENT_CONFLICT
            // 與 gRPC 直呼時被第二道防線剔除的非法輸入；Gateway 把自己 filter 掉的併進同一陣列。
            // ⚠️ 無警告時回空陣列（proto3 repeated 天然為空），不得省略欄位——省略會讓前端
            // 分不出「後端沒實作」與「這次沒有警告」。
            CreateProductResponse.Builder responseBuilder = CreateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("商品建立成功")
                    .setData(productDetail);
            addCertificationWarnings(outcome, responseBuilder::addCertificationWarnings);
            responseObserver.onNext(responseBuilder.build());

            log.info("ProductGrpcService.createProduct.success", "商品建立成功，name=" + result.getName(), "product_id", result.getId());

        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.createProduct.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(CreateProductResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (IdempotencyConflictException e) {
            log.warn("ProductGrpcService.createProduct.conflict", "冪等衝突: " + e.getMessage(), "key", request.getIdempotencyKey());
            responseObserver.onNext(CreateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (ProductBusinessException e) {
            // 業務碼透傳：hashtag/description 校驗（3014-3018）、category 不存在等帶有專屬 GrpcResponseCode 直接回
            // INTERNAL_ERROR 視為 Media client 失敗，加「圖片處理失敗:」前綴並降為 SERVICE_UNAVAILABLE
            int code = e.code();
            String msg = e.getMessage();
            if (e.responseCode() == GrpcResponseCode.INTERNAL_ERROR) {
                code = GrpcResponseCode.SERVICE_UNAVAILABLE.code();
                msg = "圖片處理失敗: " + msg;
                log.error("ProductGrpcService.createProduct.mediaFailed", "Media Service 呼叫失敗: " + e.getMessage());
            } else {
                log.warn("ProductGrpcService.createProduct.businessFailed", "code=" + code + ", msg=" + msg);
            }
            responseObserver.onNext(CreateProductResponse.newBuilder()
                    .setCode(code)
                    .setMessage(msg)
                    .build());
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.createProduct.invalidParam", "參數錯誤: " + e.getMessage());
            responseObserver.onNext(CreateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CreateProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateProductResponse.newBuilder(), e, "ProductGrpcService", "createProduct");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 更新商品
     */
    @Override
    public void updateProduct(UpdateProductRequest request, StreamObserver<UpdateProductResponse> responseObserver) {
        try {
            // 授權：賣家本人或代行（products.write）
            GrpcAuthContext.requireSellerOrDelegated(delegationOrNull(request.hasDelegation(), request.getDelegation()),
                    request.getStoreId(), DelegationCapability.PRODUCTS_WRITE, request.getIdempotencyKey());

            // 驗證必填欄位
            if (request.getIdempotencyKey().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("idempotency_key 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換 Proto → DTO
            UpdateProductDTO dto = protoConverter.toUpdateDTO(request);

            // 呼叫 Service
            ProductWriteOutcome outcome = productService.updateProductWithWarnings(dto);
            ProductDetailDTO result = outcome.detail();

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            // 19 號設計書 §9.5，同 CreateProductResponse。⚠️ 這是兩個不同的 message，
            // 各補一次；漏掉任一個，對應端點的那個欄位就會恆空且無任何訊號。
            UpdateProductResponse.Builder responseBuilder = UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("商品更新成功")
                    .setData(productDetail);
            addCertificationWarnings(outcome, responseBuilder::addCertificationWarnings);
            responseObserver.onNext(responseBuilder.build());

            log.info("ProductGrpcService.updateProduct.success", "商品更新成功", "product_id", result.getId());

        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.updateProduct.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (ProductNotFoundException e) {
            log.warn("ProductGrpcService.updateProduct.notFound", "商品不存在", "product_id", request.getProductId());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (IdempotencyConflictException e) {
            log.warn("ProductGrpcService.updateProduct.conflict", "冪等衝突: " + e.getMessage(), "key", request.getIdempotencyKey());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (ProductBusinessException e) {
            // 用 exception 自帶的 GrpcResponseCode 與 message（涵蓋 Media 失敗、編輯狀態守門 §2.8.5 等）
            log.warn("ProductGrpcService.updateProduct.businessException",
                    "業務例外 code=" + e.code() + " msg=" + e.getMessage());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.updateProduct.invalidParam", "參數錯誤: " + e.getMessage());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UpdateProductResponse.newBuilder(), e, "ProductGrpcService", "updateProduct");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 取得商品詳情
     *
     * 權限規則：
     * - ACTIVE (4004) 狀態：任何人都可以查看
     * - DRAFT (4001) / PENDING_REVIEW (4002) 狀態：只有 owner（storeId == product.store_id）可以查看
     */
    @Override
    public void getProduct(GetProductRequest request, StreamObserver<GetProductResponse> responseObserver) {
        try {
            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(GetProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 取得 store_id：一律讀 request 欄位（gateway 已解析重開店後的
            // store surrogate id），不可用 GrpcAuthContext——那是請求進入時的
            // 原始 JWT accountId，重開過店的賣家會跟 entity.getStoreId() 對不上
            // （對齊 getProductList / deleteProduct 等既有擁有權判斷慣例）
            long storeId = request.getStoreId();

            // 代行判定（products.read）。本 RPC 同時服務公開頁與匿名呼叫，所以不得整支收斂成
            // 賣家專屬——判定以 delegation 欄位是否存在做分支；未帶即維持現狀不動。
            DelegationContext delegation = delegationOrNull(request.hasDelegation(), request.getDelegation());
            if (delegation != null) {
                GrpcAuthContext.requireSellerOrDelegated(delegation, storeId, DelegationCapability.PRODUCTS_READ);
            }

            // 呼叫 Service（含權限檢查；代行路徑另加 Q4 歸屬斷言）
            ProductDetailDTO result = delegation != null
                    ? productService.getProduct(request.getProductId(), storeId, true)
                    : productService.getProduct(request.getProductId(), storeId);

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            // 自買標記（§2.2.1）：請求者的 storeId 為商品 owner 時為 true，匿名固定 false
            boolean selfOwned = storeId > 0 && Long.valueOf(storeId).equals(result.getStoreId());
            productDetail = productDetail.toBuilder().setSelfOwned(selfOwned).build();

            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功")
                    .setData(productDetail)
                    .build());

            log.info("ProductGrpcService.getProduct.success", "查詢商品成功", "product_id", request.getProductId());

        } catch (ProductNotFoundException e) {
            log.warn("ProductGrpcService.getProduct.notFound", "商品不存在", "product_id", request.getProductId());
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.getProduct.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (ProductAccessDeniedException e) {
            log.warn("ProductGrpcService.getProduct.accessDenied", "無權存取商品: " + e.getMessage());
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())  // 回傳 404 而非 403，避免洩漏商品存在
                    .setMessage("商品不存在")
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductResponse.newBuilder(), e, "ProductGrpcService", "getProduct");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 公開商品詳情（依 publication contract，支援 Redis 快取）
     *
     * 供買家或匿名使用者瀏覽，不需要登入識別；曾公開後暫時下架仍可查看。
     * 內部使用 Cache-Aside 模式，先查 Redis 快取，未命中再查 DB。
     */
    @Override
    public void getPublicProduct(GetPublicProductRequest request, StreamObserver<GetProductResponse> responseObserver) {
        try {
            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(GetProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service（含 Redis 快取，依 publication contract 判斷可見性）
            ProductDetailDTO result = productService.getPublicProduct(request.getProductId());

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功")
                    .setData(productDetail)
                    .build());

            log.info("ProductGrpcService.getPublicProduct.success", "公開查詢商品成功", "product_id", request.getProductId());

        } catch (ProductPublicNotFoundException e) {
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(e.responseCode().code())
                    .setMessage(e.responseCode().message())
                    .build());
        } catch (ProductNotFoundException e) {
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductResponse.newBuilder(), e, "ProductGrpcService", "getPublicProduct");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 取得商品列表（禁止 join product_sku）
     */
    @Override
    public void getProductList(GetProductListRequest request, StreamObserver<GetProductListResponse> responseObserver) {
        try {
            // 代行判定（products.read）。owner_store_id 為零會讓下方 isPublic 分支把整份查詢
            // 轉成公開列表 ⇒ 代行者拿到的是全站 ACTIVE 商品，故此處必須先擋（S4 同時也擋一次）。
            DelegationContext delegation = delegationOrNull(request.hasDelegation(), request.getDelegation());
            if (delegation != null) {
                GrpcAuthContext.requireSellerOrDelegated(delegation, request.getOwnerStoreId(),
                        DelegationCapability.PRODUCTS_READ);
            }

            // 轉換 Proto → DTO
            ProductQueryDTO query = protoConverter.toQueryDTO(request);

            // ownerStoreId == 0（無擁有者篩選）= 公開 API → 強制 ACTIVE + 填充商家資訊
            boolean isPublic = request.getOwnerStoreId() <= 0;
            if (isPublic) {
                query.setStatus(ProductStatus.ACTIVE.value());
                query.setPublicApi(true);
            }

            // 呼叫 Service
            ProductListResultDTO result = productService.getProductList(query);

            // 組建回應
            GetProductListResponse.Builder responseBuilder = GetProductListResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功")
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getPage()))
                    .setPageSize(String.valueOf(result.getPageSize()));

            // 轉換商品列表
            for (ProductListItemDTO item : result.getProducts()) {
                ProductListItem protoItem = protoConverter.toProductListItemProto(item);
                responseBuilder.addProducts(protoItem);
            }

            responseObserver.onNext(responseBuilder.build());

            log.info("ProductGrpcService.getProductList.success", "查詢商品列表成功，total=" + result.getTotal() + "，page=" + result.getPage());

        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.getProductList.delegationDenied", "代行判定失敗 code=" + e.code());
            responseObserver.onNext(GetProductListResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (io.github.will413028.hawkr.product.grpc.client.BuyerGrpcClient.BuyerServiceUnavailableException e) {
            // followed_product_only / followed_seller_only filter 失敗（buyer service 不可用）→ 503 SERVICE_UNAVAILABLE，由 Gateway 翻成 HTTP 503
            log.warn("ProductGrpcService.getProductList.buyerUnavailable",
                    "buyer service 不可用，followed filter 失敗: " + e.getMessage());
            GetProductListResponse resp = GetProductListResponse.newBuilder()
                    .setCode(GrpcResponseCode.SERVICE_UNAVAILABLE.code())
                    .setMessage("buyer service unavailable")
                    .build();
            responseObserver.onNext(resp);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductListResponse.newBuilder(), e, "ProductGrpcService", "getProductList");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 管理員取得商品詳情（不檢查擁有者，不限狀態）
     */
    @Override
    public void getProductForAdmin(GetProductForAdminRequest request,
            StreamObserver<GetProductResponse> responseObserver) {
        try {
            // 授權：僅 ADMIN 可使用此 RPC
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(GetProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service（不檢查擁有者，不限狀態）
            ProductDetailDTO result = productService.getProductForAdmin(request.getProductId());

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功")
                    .setData(productDetail)
                    .build());

            log.info("ProductGrpcService.getProductForAdmin.success", "管理員查詢商品成功", "product_id", request.getProductId());

        } catch (ProductNotFoundException e) {
            log.warn("ProductGrpcService.getProductForAdmin.notFound", "商品不存在", "product_id", request.getProductId());
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductResponse.newBuilder(), e, "ProductGrpcService", "getProductForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 管理員更新商品狀態（不檢查擁有者）
     */
    @Override
    public void updateProductStatus(UpdateProductStatusRequest request,
            StreamObserver<UpdateProductResponse> responseObserver) {
        try {
            // 授權：僅 ADMIN 可更新商品狀態
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (request.getStatus().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("status 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            ProductDetailDTO result = productService.updateProductStatus(
                    request.getProductId(), request.getStatus());

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("商品狀態更新成功")
                    .setData(productDetail)
                    .build());

            log.info("ProductGrpcService.updateProductStatus.success", "管理員更新商品狀態成功", "product_id", request.getProductId());

        } catch (ProductNotFoundException e) {
            log.warn("ProductGrpcService.updateProductStatus.notFound", "商品不存在", "product_id", request.getProductId());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.updateProductStatus.invalidParam", "參數錯誤: " + e.getMessage());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UpdateProductResponse.newBuilder(), e, "ProductGrpcService", "updateProductStatus");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 賣家更新商品狀態（含擁有者驗證 + 狀態轉換驗證）
     */
    @Override
    public void updateSellerProductStatus(UpdateSellerProductStatusRequest request,
            StreamObserver<UpdateProductResponse> responseObserver) {
        try {
            // 授權：賣家本人或代行（products.publish）
            GrpcAuthContext.requireSellerOrDelegated(delegationOrNull(request.hasDelegation(), request.getDelegation()),
                    request.getStoreId(), DelegationCapability.PRODUCTS_PUBLISH, request.getIdempotencyKey());

            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (request.getStatus().isEmpty()) {
                responseObserver.onNext(UpdateProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("status 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            ProductDetailDTO result = productService.updateSellerProductStatus(
                    request.getProductId(), request.getStoreId(), request.getStatus(),
                    request.getExpectedVersion());

            // 轉換 DTO → Proto
            ProductDetail productDetail = protoConverter.toProductDetailProto(result);

            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("商品狀態更新成功")
                    .setData(productDetail)
                    .build());

            log.info("ProductGrpcService.updateSellerProductStatus.success",
                    "賣家更新商品狀態成功，product_id=" + request.getProductId()
                            + "，store_id=" + request.getStoreId());

        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.updateSellerProductStatus.delegationDenied",
                    "代行判定失敗 code=" + e.code());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (ProductNotFoundException e) {
            log.warn("ProductGrpcService.updateSellerProductStatus.notFound",
                    "商品不存在", "product_id", request.getProductId());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.updateSellerProductStatus.invalidParam",
                    "參數錯誤: " + e.getMessage());
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(UpdateProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UpdateProductResponse.newBuilder(), e,
                    "ProductGrpcService", "updateSellerProductStatus");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 批次取 before 快照與當前 version（代行批次寫入的第一跳，56 §7.2.8 步驟 ②）。
     *
     * 唯讀，宣告 capability 為 products.read —— 這一跳在 RecordAuditIntentBatch 之前送出，
     * audit_log_id 必然為空；寫成 products.publish 會被 S1b 打回 9260，批次下架完全不可用。
     */
    @Override
    public void batchGetSellerProductsForDelegation(BatchGetSellerProductsForDelegationRequest request,
            StreamObserver<BatchGetSellerProductsForDelegationResponse> responseObserver) {
        try {
            GrpcAuthContext.requireSellerOrDelegated(delegationOrNull(request.hasDelegation(), request.getDelegation()),
                    request.getStoreId(), DelegationCapability.PRODUCTS_READ);

            String shapeError = validateBatchProductIds(request.getProductIdsList());
            if (shapeError != null) {
                responseObserver.onNext(BatchGetSellerProductsForDelegationResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(shapeError)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            BatchGetSellerProductsForDelegationResponse.Builder builder =
                    BatchGetSellerProductsForDelegationResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("查詢成功");
            // withDetail=true：稽核的 before_snapshot 取自這一份 detail。傳 false 的話
            // 批次下架的 before_snapshot 只會有 status 一個鍵，而單筆版存的是完整 12 欄。
            for (ProductSnapshotDTO snapshot : productService.getProductSnapshotsForDelegation(
                    request.getStoreId(), request.getProductIdsList(), true)) {
                BatchProductSnapshot.Builder item = BatchProductSnapshot.newBuilder()
                        .setProductId(snapshot.getProductId())
                        .setFound(snapshot.isFound())
                        .setVersion(snapshot.getVersion() != null ? String.valueOf(snapshot.getVersion()) : "0")
                        .setStatus(snapshot.getStatus() != null ? snapshot.getStatus() : "")
                        .setName(snapshot.getName() != null ? snapshot.getName() : "");
                if (snapshot.getDetail() != null) {
                    item.setDetail(protoConverter.toProductDetailProto(snapshot.getDetail()));
                }
                builder.addResults(item.build());
            }
            responseObserver.onNext(builder.build());

            log.info("ProductGrpcService.batchGetSellerProductsForDelegation.success",
                    "批次快照查詢成功，count=" + request.getProductIdsCount()
                            + "，store_id=" + request.getStoreId());
        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.batchGetSellerProductsForDelegation.delegationDenied",
                    "代行判定失敗 code=" + e.code());
            responseObserver.onNext(BatchGetSellerProductsForDelegationResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(BatchGetSellerProductsForDelegationResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchGetSellerProductsForDelegationResponse.newBuilder(), e,
                    "ProductGrpcService", "batchGetSellerProductsForDelegation");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 賣家批次更新商品狀態（56 §7.2.8 步驟 ④）。
     *
     * 逐項獨立成敗：逐項 try-catch、逐項一次交易、逐項 version CAS，例外一律轉成 per-item result
     * 而非外拋；頂層 code 恆為 0（部分失敗以 data.failed 呈現），因為前端對非零頂層 code 一律整包
     * 判失敗並丟棄 results ⇒ 逐項的 auditLogId 會連同結果一起消失。
     */
    @Override
    public void batchUpdateSellerProductStatus(BatchUpdateSellerProductStatusRequest request,
            StreamObserver<BatchUpdateSellerProductStatusResponse> responseObserver) {
        try {
            DelegationContext delegation = delegationOrNull(request.hasDelegation(), request.getDelegation());
            DelegationContextValidator.validateBatch(delegation, request.getStoreId(),
                    DelegationCapability.PRODUCTS_PUBLISH, request.getIdempotencyKey(),
                    request.getItemsList().stream().map(BatchProductStatusItem::getAuditLogId).toList());
            if (delegation == null) {
                GrpcAuthContext.requireAccountType(AccountType.SELLER);
            }

            String shapeError = validateBatchProductIds(
                    request.getItemsList().stream().map(BatchProductStatusItem::getProductId).toList());
            if (shapeError == null && request.getStatus().isEmpty()) {
                shapeError = "status 為必填欄位";
            }
            if (shapeError != null) {
                responseObserver.onNext(BatchUpdateSellerProductStatusResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(shapeError)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            int succeeded = 0;
            BatchUpdateSellerProductStatusResponse.Builder builder =
                    BatchUpdateSellerProductStatusResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message());
            for (BatchProductStatusItem item : request.getItemsList()) {
                BatchProductStatusResult result = applyBatchStatusItem(request, item);
                if (result.getOk()) {
                    succeeded++;
                }
                builder.addResults(result);
            }
            int requested = request.getItemsCount();
            responseObserver.onNext(builder
                    .setRequested(String.valueOf(requested))
                    .setSucceeded(String.valueOf(succeeded))
                    .setFailed(String.valueOf(requested - succeeded))
                    .build());

            log.info("ProductGrpcService.batchUpdateSellerProductStatus.success",
                    "批次狀態更新完成，requested=" + requested + "，succeeded=" + succeeded
                            + "，store_id=" + request.getStoreId());
        } catch (DelegationDeniedException e) {
            log.warn("ProductGrpcService.batchUpdateSellerProductStatus.delegationDenied",
                    "代行判定失敗 code=" + e.code());
            responseObserver.onNext(BatchUpdateSellerProductStatusResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(BatchUpdateSellerProductStatusResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchUpdateSellerProductStatusResponse.newBuilder(), e,
                    "ProductGrpcService", "batchUpdateSellerProductStatus");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 逐項執行一筆狀態更新，任何例外都轉成 per-item result（不外拋，否則整批一起死）。
     * 失敗項回現行版本，讓前端能直接重試。
     */
    private BatchProductStatusResult applyBatchStatusItem(BatchUpdateSellerProductStatusRequest request,
            BatchProductStatusItem item) {
        BatchProductStatusResult.Builder result = BatchProductStatusResult.newBuilder()
                .setProductId(item.getProductId())
                .setAuditLogId(item.getAuditLogId());   // 請求帶進來的值原樣回音，非服務端產生
        try {
            ProductDetailDTO updated = productService.updateSellerProductStatus(
                    item.getProductId(), request.getStoreId(), request.getStatus(), item.getExpectedVersion());
            return result.setOk(true)
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setVersion(updated.getVersion() != null ? String.valueOf(updated.getVersion()) : "0")
                    .build();
        } catch (Exception e) {
            int code;
            if (e instanceof DelegationDeniedException dde) {
                code = dde.code();
            } else if (e instanceof ProductNotFoundException) {
                code = GrpcResponseCode.NOT_FOUND.code();
            } else if (e instanceof ProductBusinessException pbe) {
                code = pbe.code();
            } else if (e instanceof IllegalArgumentException) {
                code = GrpcResponseCode.INVALID_PARAM.code();
            } else {
                code = GrpcResponseCode.INTERNAL_ERROR.code();
                log.error("ProductGrpcService.batchUpdateSellerProductStatus.itemFailed",
                        "批次項目失敗，product_id=" + item.getProductId(), e);
            }
            return result.setOk(false)
                    .setCode(code)
                    .setMessage(e.getMessage() != null ? e.getMessage() : "")
                    .setVersion(currentVersionOf(request.getStoreId(), item.getProductId()))
                    .build();
        }
    }

    /** 失敗項的現行版本；查不到（不存在／不屬本店）回 "0"。 */
    private String currentVersionOf(long storeId, String productId) {
        try {
            List<ProductSnapshotDTO> snapshots =
                    productService.getProductSnapshotsForDelegation(storeId, List.of(productId));
            ProductSnapshotDTO snapshot = snapshots.isEmpty() ? null : snapshots.get(0);
            return snapshot != null && snapshot.getVersion() != null
                    ? String.valueOf(snapshot.getVersion()) : "0";
        } catch (Exception e) {
            return "0";
        }
    }

    /** 批次共通形狀檢查：1–50 筆、id 非空且不重複。回 null 表示通過。 */
    private String validateBatchProductIds(List<String> productIds) {
        if (productIds.isEmpty()) {
            return "至少需要一筆商品";
        }
        if (productIds.size() > BATCH_MAX_ITEMS) {
            return "單次批次上限 " + BATCH_MAX_ITEMS + " 筆";
        }
        Set<String> seen = new java.util.HashSet<>();
        for (String productId : productIds) {
            if (productId.isEmpty()) {
                return "product_id 為必填欄位";
            }
            if (!seen.add(productId)) {
                return "product_id 不可重複: " + productId;
            }
        }
        return null;
    }

    /** proto3 message 欄位天生有 presence：未帶 = 賣家本人操作，帶了 = 代行操作。 */
    private static DelegationContext delegationOrNull(boolean present, DelegationContext delegation) {
        return present ? delegation : null;
    }

    /**
     * 清除商品快取
     */
    @Override
    public void purgeProductCache(PurgeProductCacheRequest request, StreamObserver<PurgeProductCacheResponse> responseObserver) {
        try {
            // 授權：僅 ADMIN 可清除快取
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            String scope = request.getScope();

            // 驗證 scope 參數
            if (scope == null || scope.isEmpty()) {
                responseObserver.onNext(PurgeProductCacheResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("scope 為必填欄位（all/detail/list）")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            int purgedKeys = 0;
            String description;

            switch (scope.toLowerCase()) {
                case "all" -> {
                    // 清除所有商品快取（Redis 失聯時 best-effort 略過，不 500，02 §2.7.3）
                    purgedKeys += safePurgeByPattern("prod:detail:*");
                    purgedKeys += safePurgeByPattern("prod:list:*");

                    // 同時清除分類快取
                    categoryCacheService.evict();
                    purgedKeys++;

                    description = "已清除所有商品快取（詳情 + 列表 + 分類）";
                    log.info("ProductGrpcService.purgeProductCache.success", "清除所有快取成功", "purged_keys", String.valueOf(purgedKeys));
                }
                case "detail" -> {
                    // 清除特定商品詳情快取
                    String productId = request.getProductId();
                    if (productId == null || productId.isEmpty()) {
                        responseObserver.onNext(PurgeProductCacheResponse.newBuilder()
                                .setCode(GrpcResponseCode.INVALID_PARAM.code())
                                .setMessage("當 scope=detail 時，product_id 為必填欄位")
                                .build());
                        responseObserver.onCompleted();
                        return;
                    }

                    cacheService.evictProductDetail(productId);
                    purgedKeys = 1;
                    description = "已清除商品詳情快取：" + productId;
                    log.info("ProductGrpcService.purgeProductCache.success", "清除商品詳情快取成功", "product_id", productId);
                }
                case "list" -> {
                    // 清除列表快取
                    Long storeId = request.getStoreId();
                    if (storeId > 0) {
                        // 清除特定賣家的列表快取（Redis 失聯時 best-effort 略過，不 500）
                        purgedKeys = safePurgeByPattern("prod:list:" + storeId + ":*");
                        description = "已清除賣家 " + storeId + " 的列表快取";
                        log.info("ProductGrpcService.purgeProductCache.success", "清除列表快取成功", "store_id", String.valueOf(storeId));
                    } else {
                        // 清除所有列表快取
                        cacheService.evictAllProductLists();
                        purgedKeys = safePurgeByPattern("prod:list:*");
                        description = "已清除所有商品列表快取";
                        log.info("ProductGrpcService.purgeProductCache.success", "清除所有列表快取成功");
                    }
                }
                case "category" -> {
                    // 清除分類快取
                    categoryCacheService.evict();
                    purgedKeys = 1;
                    description = "已清除分類快取";
                    log.info("ProductGrpcService.purgeProductCache.success", "清除分類快取成功");
                }
                default -> {
                    responseObserver.onNext(PurgeProductCacheResponse.newBuilder()
                            .setCode(GrpcResponseCode.INVALID_PARAM.code())
                            .setMessage("無效的 scope 值，必須為 all/detail/list/category")
                            .build());
                    responseObserver.onCompleted();
                    return;
                }
            }

            // 建立回應
            PurgeCacheResult result = PurgeCacheResult.newBuilder()
                    .setScope(scope)
                    .setPurgedKeys(String.valueOf(purgedKeys))
                    .setDescription(description)
                    .build();

            responseObserver.onNext(PurgeProductCacheResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("快取清除成功")
                    .setData(result)
                    .build());

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(PurgeProductCacheResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, PurgeProductCacheResponse.newBuilder(), e, "ProductGrpcService", "purgeProductCache");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * best-effort 依 pattern 清除快取 key，回傳實際清除筆數；
     * Redis 失聯時 log.warn 回 0，不讓 PurgeProductCache 因 redis 失聯回 500（02 §2.7.3）。
     */
    private int safePurgeByPattern(String pattern) {
        try {
            // 非阻塞 SCAN + UNLINK 取代阻塞式 KEYS + DEL（清除範圍不變，見 RedisPatternEvictor）
            return (int) RedisPatternEvictor.scanUnlink(redisTemplate, pattern);
        } catch (DataAccessException e) {
            log.warn("ProductGrpcService.purgeProductCache.unavailable", "Redis 失聯，清除快取略過: " + e.getMessage(), "pattern", pattern);
            return 0;
        }
    }

    // ========== 庫存操作 ==========

    /**
     * 扣減庫存（Order Service 呼叫，冪等）
     */
    @Override
    public void deductStock(DeductStockRequest request, StreamObserver<StockOperationResponse> responseObserver) {
        try {
            // 驗證必填欄位
            if (request.getStockTxId().isEmpty()) {
                responseObserver.onNext(StockOperationResponse.newBuilder()
                        .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                        .setMessage("stock_tx_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getItemsList().isEmpty()) {
                responseObserver.onNext(StockOperationResponse.newBuilder()
                        .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                        .setMessage("items 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換 Proto → Service 參數
            List<StockService.StockItemParam> items = request.getItemsList().stream()
                    .map(item -> new StockService.StockItemParam(
                            parseLong(item.getSkuId()),
                            parseInt(item.getQuantity())))
                    .toList();

            // 呼叫 Service
            StockService.StockResult result = stockService.deductStock(request.getStockTxId(), items);

            // 映射結果
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(mapStockStatus(result))
                    .setMessage(result == StockService.StockResult.SUCCESS ? "庫存扣減成功" : "重複請求，已忽略")
                    .build());

            // 快取失效已收斂至 StockService afterCommit 單一入口（#1455），此處不再二次清除

            log.info("ProductGrpcService.deductStock.success", "庫存扣減完成",
                    "stockTxId", request.getStockTxId());

        } catch (StockService.InsufficientStockException e) {
            log.warn("ProductGrpcService.deductStock.insufficient", e.getMessage(),
                    "skuId", String.valueOf(e.getSkuId()));

            // 查詢實際剩餘庫存，填入結構化回傳
            int availableQty = 0;
            try {
                var skuOpt = skuRepository.findById(e.getSkuId());
                if (skuOpt.isPresent()) {
                    availableQty = skuOpt.get().getStockQty() != null ? skuOpt.get().getStockQty() : 0;
                }
            } catch (Exception ex) {
                log.warn("ProductGrpcService.deductStock.queryStockFailed",
                        "查詢剩餘庫存失敗: " + ex.getMessage());
            }

            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(StockOperationStatus.INSUFFICIENT_STOCK.value())
                    .setMessage(e.getMessage())
                    .addInsufficientItems(InsufficientItem.newBuilder()
                            .setSkuId(String.valueOf(e.getSkuId()))
                            .setRequestedQty(String.valueOf(e.getRequestedQuantity()))
                            .setAvailableQty(String.valueOf(availableQty))
                            .build())
                    .build());
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.deductStock.invalidParam", "參數格式錯誤: " + e.getMessage());
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
        } catch (Exception e) {
            log.error("ProductGrpcService.deductStock.failed", "庫存扣減失敗", e);
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                    .setMessage(GrpcResponseCode.INTERNAL_ERROR.message())
                    .build());
        }
        responseObserver.onCompleted();
    }

    /**
     * 歸還庫存（取消/補償，冪等）
     */
    @Override
    public void restoreStock(RestoreStockRequest request, StreamObserver<StockOperationResponse> responseObserver) {
        try {
            // 驗證必填欄位
            if (request.getStockTxId().isEmpty()) {
                responseObserver.onNext(StockOperationResponse.newBuilder()
                        .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                        .setMessage("stock_tx_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getItemsList().isEmpty()) {
                responseObserver.onNext(StockOperationResponse.newBuilder()
                        .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                        .setMessage("items 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換 Proto → Service 參數
            List<StockService.StockItemParam> items = request.getItemsList().stream()
                    .map(item -> new StockService.StockItemParam(
                            parseLong(item.getSkuId()),
                            parseInt(item.getQuantity())))
                    .toList();

            // 呼叫 Service
            StockService.StockResult result = stockService.restoreStock(request.getStockTxId(), items);

            // 映射結果
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(mapStockStatus(result))
                    .setMessage(result == StockService.StockResult.SUCCESS ? "庫存歸還成功" : "重複請求，已忽略")
                    .build());

            // 快取失效已收斂至 StockService afterCommit 單一入口（#1455），此處不再二次清除

            log.info("ProductGrpcService.restoreStock.success", "庫存歸還完成",
                    "stockTxId", request.getStockTxId());

        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.restoreStock.invalidParam", "參數格式錯誤: " + e.getMessage());
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
        } catch (Exception e) {
            log.error("ProductGrpcService.restoreStock.failed", "庫存歸還失敗", e);
            responseObserver.onNext(StockOperationResponse.newBuilder()
                    .setStatus(StockOperationStatus.SYSTEM_ERROR.value())
                    .setMessage(GrpcResponseCode.INTERNAL_ERROR.message())
                    .build());
        }
        responseObserver.onCompleted();
    }

    /**
     * 批量查詢庫存（Order Service 呼叫）
     */
    @Override
    public void getStock(GetStockRequest request, StreamObserver<GetStockResponse> responseObserver) {
        try {
            // 驗證必填欄位
            if (request.getSkuIdsList().isEmpty()) {
                responseObserver.onNext(GetStockResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(SKU_IDS_REQUIRED_MESSAGE)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換 Proto string → Long
            List<Long> skuIds = request.getSkuIdsList().stream()
                    .map(this::parseLong)
                    .toList();

            // 呼叫 Service
            Map<Long, Integer> stockMap = stockService.getStock(skuIds);

            // 組建回應
            GetStockResponse.Builder responseBuilder = GetStockResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");

            for (Map.Entry<Long, Integer> entry : stockMap.entrySet()) {
                responseBuilder.addItems(StockInfo.newBuilder()
                        .setSkuId(formatLongOrZero(entry.getKey()))
                        .setStockQty(formatInt(entry.getValue()))
                        .build());
            }

            responseObserver.onNext(responseBuilder.build());

            log.info("ProductGrpcService.getStock.success", "庫存查詢成功",
                    "count", String.valueOf(stockMap.size()));

        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.getStock.invalidParam", "參數格式錯誤: " + e.getMessage());
            responseObserver.onNext(GetStockResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetStockResponse.newBuilder(), e, "ProductGrpcService", "getStock");
            return;
        }
        responseObserver.onCompleted();
    }

    // ========== 型別轉換工具方法（委派至 ProtoConversionUtils） ==========

    private Long parseLong(String value) {
        return ProtoConversionUtils.parseLongRequired(value, "數字欄位");
    }

    private int parseInt(String value) {
        return ProtoConversionUtils.parseIntRequired(value, "數字欄位");
    }

    /**
     * Long → Proto string，null 一律成 {@code "0"}（委派 {@link ProtoConversionUtils#formatAmount}）。
     *
     * <p>名字刻意不叫 {@code formatLong}：{@link ProtoConversionUtils#formatLong} 存在且對 null
     * 回的是<b>空字串</b>，而本檔兩支都在用（下方 GetProductSimpleBatch / GetSkuInfoBatch 走
     * {@code ProtoConversionUtils.formatLong}）。同名反義的私有方法會讓讀者以為兩處行為一致。
     */
    private String formatLongOrZero(Long value) {
        return ProtoConversionUtils.formatAmount(value);
    }

    /**
     * 解析尺寸字串為整數 cm（支援小數，四捨五入，必須 > 0）
     */
    private int parseDimensionCm(String value, String fieldName) {
        return InputValidator.requirePositiveCm(value, fieldName);
    }

    private String formatInt(Integer value) {
        return value == null ? "0" : ProtoConversionUtils.formatInt(value);
    }

    private String mapStockStatus(StockService.StockResult result) {
        return switch (result) {
            case SUCCESS -> StockOperationStatus.SUCCESS.value();
            case INSUFFICIENT_STOCK -> StockOperationStatus.INSUFFICIENT_STOCK.value();
            case ALREADY_APPLIED -> StockOperationStatus.ALREADY_APPLIED.value();
            case NOOP_NOT_FOUND -> StockOperationStatus.NOOP_NOT_FOUND.value();
            case SYSTEM_ERROR -> StockOperationStatus.SYSTEM_ERROR.value();
        };
    }

    // ==================== CalculateShippingOptions（運費計算） ====================

    /**
     * 計算可用配送選項與運費（依溫層分組）
     * 賣家建立商品時使用，根據包裝尺寸和重量計算可用的配送方式與運費
     */
    @Override
    public void calculateShippingOptions(CalculateShippingOptionsRequest request,
                                          StreamObserver<CalculateShippingOptionsResponse> responseObserver) {
        try {
            // 授權：僅 SELLER 可使用
            GrpcAuthContext.requireAccountType(AccountType.SELLER);

            // 驗證並解析必填欄位（支援小數字串，四捨五入為整數 cm）
            int lengthCm = parseDimensionCm(request.getLength(), "length");
            int widthCm = parseDimensionCm(request.getWidth(), "width");
            int heightCm = parseDimensionCm(request.getHeight(), "height");

            // 解析重量單位
            String weightUnitStr = request.getWeightUnit();
            if (weightUnitStr == null || weightUnitStr.isEmpty()) {
                responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("weight_unit 為必填欄位（3301=KG/3302=CATTY/3303=GRAM）")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (!WeightUnit.isValidValue(weightUnitStr)) {
                responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("無效的重量單位: " + weightUnitStr + "，允許值: 3301(KG), 3302(CATTY), 3303(GRAM)")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            WeightUnit weightUnit = WeightUnit.fromValue(weightUnitStr);

            // 轉換重量為公克
            long weightGrams = WeightConverter.convertToGrams(request.getWeight(), weightUnit);

            // 透過 Seller Service 取得賣家 zip_code → 推導出貨地區
            long storeId = request.getStoreId();
            SellerMeta sellerMeta = sellerGrpcClient.getSellerMeta(storeId);
            // TODO: 待 seller.proto SellerMeta 新增 zip_code 欄位後，改用 sellerMeta.getZipCode()
            String zipCode = (sellerMeta != null) ? "" : "";
            ShippingArea senderArea = ShippingAreaResolver.resolve(zipCode);
            // 收件地區固定為本島（Phase 1 僅支援寄送至台灣本島）
            ShippingArea receiverArea = ShippingArea.MAIN_ISLAND;

            // 透過 ShippingRoute 推導 DeliveryZoneKey value（與 ShippingFeeCacheClient.toDeliveryZone 對應）
            io.github.will413028.hawkr.common.utils.code.ShippingRoute route =
                    io.github.will413028.hawkr.common.utils.code.helper.ShippingRouteRules.resolve(senderArea, receiverArea);
            String deliveryZone = switch (route) {
                case DOMESTIC -> io.github.will413028.hawkr.common.utils.code.DeliveryZoneKey.DOMESTIC.value();
                case TO_OFFSHORE, FROM_OFFSHORE -> io.github.will413028.hawkr.common.utils.code.DeliveryZoneKey.OFFSHORE.value();
                case OFFSHORE_TO_OFFSHORE -> io.github.will413028.hawkr.common.utils.code.DeliveryZoneKey.INTER_ISLAND.value();
            };

            // 呼叫 Shipping Service 計算配送選項（傳入 storeId，只回傳賣家已開通的配送方式）
            io.github.will413028.hawkr.shipping.grpc.CalculateShippingOptionsResponse shippingResp =
                    shippingGrpcClient.calculateShippingOptions(
                            String.valueOf(lengthCm), String.valueOf(widthCm), String.valueOf(heightCm),
                            String.valueOf(weightGrams), deliveryZone, storeId);

            // shipping-service 失敗時會拋 StatusRuntimeException，由 catch-all 處理；
            // 此處只需檢查業務結果是否為空
            // 檢查是否有可用選項；無選項時透傳 shipping-service 回報的真正原因碼 + 訊息
            // （SELLER_NOT_ONBOARDED 未開通 / SHIPPING_ALL_METHODS_EXCEED_LIMITS 超尺寸重量），
            // 避免一律誤指「商品尺寸超過限制」。
            if (shippingResp.getTemperatureGroupsList().isEmpty()) {
                int reasonCode = shippingResp.getCode();
                String reasonMsg = shippingResp.getMessage();
                // shipping 未回報具體原因時（版本 skew）的保底
                if (reasonCode == GrpcResponseCode.SUCCESS.code()) {
                    reasonCode = GrpcResponseCode.SHIPPING_ALL_METHODS_EXCEED_LIMITS.code();
                }
                if (reasonMsg == null || reasonMsg.isBlank()
                        || GrpcResponseCode.SUCCESS.message().equals(reasonMsg)) {
                    reasonMsg = GrpcResponseCode.SHIPPING_ALL_METHODS_EXCEED_LIMITS.message();
                }
                responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                        .setCode(reasonCode)
                        .setMessage(reasonMsg)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 計算材積 L×W×H÷27000
            String volumeSize = String.format("%.2f", (double)(lengthCm * widthCm * heightCm) / 27000.0);

            // 將 Shipping Service 回應映射到 Product Service 的 Proto 格式
            ShippingCalculationData.Builder dataBuilder = ShippingCalculationData.newBuilder()
                    .setVolumeSize(volumeSize)
                    .setWeightGrams(shippingResp.getWeightGrams())
                    .setDimensionSum(shippingResp.getDimensionSum())
                    .setLongestSide(shippingResp.getLongestSide());

            // 收集可配送溫層
            for (io.github.will413028.hawkr.shipping.grpc.ShippingOptionGroup group : shippingResp.getTemperatureGroupsList()) {
                dataBuilder.addAvailableTemperatures(AvailableTemperatureProto.newBuilder()
                        .setTemperature(group.getTemperature())
                        .setDisplayName(group.getDisplayName())
                        .build());

                // 攤平運送方式：每個 method × temperature 組合一筆
                for (io.github.will413028.hawkr.shipping.grpc.ShippingOptionDetail detail : group.getMethodsList()) {
                    dataBuilder.addShippingOptions(ShippingOptionProto.newBuilder()
                            .setShippingMethod(detail.getShippingMethod())
                            .setDisplayName(detail.getDisplayName())
                            .setTemperature(group.getTemperature())
                            .setFee(detail.getFee())
                            .setEstimatedDays(detail.getEstimatedDays())
                            .setLimits(ShippingLimitsProto.newBuilder()
                                    .setMaxDimensionSum(detail.getMaxDimensionSum())
                                    .setMaxLongestSide(detail.getMaxLongestSide())
                                    .setMaxWeightGrams(detail.getMaxWeightGrams())
                                    .build())
                            .build());
                }
            }

            responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());

            int tempCount = shippingResp.getTemperatureGroupsCount();
            log.info("ProductGrpcService.calculateShippingOptions.success",
                    "運費計算成功，可用溫層數=" + tempCount,
                    "volumeSize", volumeSize);

        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.calculateShippingOptions.invalidParam", "參數錯誤: " + e.getMessage());
            responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CalculateShippingOptionsResponse.newBuilder(), e, "ProductGrpcService", "calculateShippingOptions");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== GetProductSimple（Cart Service 呼叫） ====================

    @Override
    public void getProductSimple(GetProductSimpleRequest request,
                                  StreamObserver<GetProductSimpleResponse> responseObserver) {
        try {
            String skuIdStr = request.getSkuId();
            if (skuIdStr == null || skuIdStr.isEmpty()) {
                responseObserver.onNext(GetProductSimpleResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("sku_id 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            Long skuId = parseLong(skuIdStr);

            // 查詢 product_sku
            var skuOpt = productService.findSkuById(skuId);
            if (skuOpt.isEmpty()) {
                responseObserver.onNext(GetProductSimpleResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("SKU 不存在")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            var sku = skuOpt.get();

            // 查詢 product（取 name、status、store_id）
            var productOpt = productService.findProductById(sku.getProductId());
            if (productOpt.isEmpty()) {
                responseObserver.onNext(GetProductSimpleResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("商品不存在")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            var product = productOpt.get();

            // is_active = product.status == ACTIVE 且未軟刪除
            boolean isActive = ProductStatus.ACTIVE.value().equals(product.getStatus()) && product.getDeletedAt() == null;

            // 取得商品主圖 objectKey（gallery[0]）
            String imageObjectKey = "";
            if (product.getGallery() != null && !product.getGallery().isEmpty()) {
                var firstImage = product.getGallery().get(0);
                if (firstImage.getObjectKey() != null) {
                    imageObjectKey = firstImage.getObjectKey();
                }
            }

            // 02 設計書 §2.8.6 / §4.2.2：實際成交單價 = salePrice ?? price，list_price = price（原價）
            Long effectivePrice = sku.getSalePrice() != null ? sku.getSalePrice() : sku.getPrice();
            var info = ProductSimpleInfo.newBuilder()
                    .setSkuId(skuIdStr)
                    .setStoreId(formatLongOrZero(product.getStoreId()))
                    .setIsActive(isActive)
                    .setName(product.getName())
                    .setPrice(formatLongOrZero(effectivePrice))         // 實際成交單價（特價優先）
                    .setListPrice(formatLongOrZero(sku.getPrice()))     // 原價快照（永遠 = SKU.price）
                    .setMaxLimit(formatInt(sku.getStockQty()))
                    .setProductId(formatLongOrZero(product.getId()))
                    .setSkuName(sku.getSkuName() != null ? sku.getSkuName() : "")
                    .setImageObjectKey(imageObjectKey)
                    .build();

            responseObserver.onNext(GetProductSimpleResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(info)
                    .build());
            responseObserver.onCompleted();

            log.info("ProductGrpcService.getProductSimple.success", "查詢商品簡化資訊成功",
                    "skuId", skuIdStr);

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductSimpleResponse.newBuilder(), e, "ProductGrpcService", "getProductSimple");
        }
    }

    // ==================== BatchGetSkuCartInfo（Cart Service GetCart 呼叫） ====================
    // 02 設計書 §5.5.3：單階段 LEFT JOIN 一次拿 SKU + Product 一個 PG snapshot，
    // 取代既有兩階段「findSkusByIds + findProductsByIds」查詢，避免並行
    // UpdateProductStatus / DeductStock 期間 snapshot 漂移造成 cart 假 DELISTED（#1133）。
    // Product 找不到（projection.productStatus == null）= product orphan，從 response 排除
    // 該 SKU（caller 自動走 LOOKUP_FAILED 分支），禁止 silent collapse 成 is_active=false。

    @Override
    public void batchGetSkuCartInfo(BatchGetSkuCartInfoRequest request,
                                     StreamObserver<BatchGetSkuCartInfoResponse> responseObserver) {
        try {
            List<String> rawIds = request.getSkuIdsList();

            if (rawIds.isEmpty()) {
                responseObserver.onNext(BatchGetSkuCartInfoResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("查詢成功")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (rawIds.size() > 200) {
                responseObserver.onNext(BatchGetSkuCartInfoResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("SKU ID 數量不可超過 200")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 解析 ID
            List<Long> skuIds = new java.util.ArrayList<>();
            for (String idStr : rawIds) {
                Long id = ProtoConversionUtils.parseLongOrNull(idStr);
                if (id != null) {
                    skuIds.add(id);
                }
            }

            // 單階段 LEFT JOIN 查詢 SKU + Product 必要欄位（02 §5.5.3）
            List<SkuCartProjection> projections = productService.findSkuCartInfoByIds(skuIds);

            // 組裝結果
            BatchGetSkuCartInfoResponse.Builder responseBuilder = BatchGetSkuCartInfoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");

            int includedCount = 0;
            int orphanCount = 0;
            for (SkuCartProjection p : projections) {
                // product orphan（sku.product_id 指向不存在 / 已硬刪 product）→ 從 response 排除（caller 走 LOOKUP_FAILED）
                if (p.productStatus() == null) {
                    orphanCount++;
                    log.warn("productOrphan",
                            String.format("SKU 對應的 product 查無，已從 response 排除 skuId=%d productId=%d",
                                    p.skuId(), p.productId()),
                            "skuId", String.valueOf(p.skuId()));
                    continue;
                }

                boolean isActive = ProductStatus.ACTIVE.value().equals(p.productStatus())
                        && p.productDeletedAt() == null;
                int stockQty = p.stockQty() != null ? p.stockQty() : 0;

                // 不可購買時補診斷 log（含 productStatus / deletedAt / stockQty），供事後對齊
                if (!isActive || stockQty <= 0) {
                    log.info("unpurchasable",
                            String.format("SKU 即時判定為不可購買 skuId=%d productId=%d status=%s deletedAt=%s stockQty=%d",
                                    p.skuId(), p.productId(), p.productStatus(),
                                    String.valueOf(p.productDeletedAt()), stockQty),
                            "skuId", String.valueOf(p.skuId()));
                }

                // 提取配送溫層（從 shipping_method JSONB 去重，保留 JSONB 出現順序）
                Set<String> temps = parseDeliveryTemps(p.shippingMethodJson(), p.skuId());

                // 02 設計書 §4.2.2：price = effective price（特價優先），list_price = 原價
                Long effectivePrice = p.salePrice() != null ? p.salePrice() : p.price();
                SkuCartInfo info = SkuCartInfo.newBuilder()
                        .setStockQty(ProtoConversionUtils.formatInt(stockQty))
                        .addAllDeliveryTemps(temps)
                        .setPrice(ProtoConversionUtils.formatLong(effectivePrice))
                        .setListPrice(ProtoConversionUtils.formatLong(p.price()))
                        .setIsActive(isActive)
                        .build();

                responseBuilder.putSkus(ProtoConversionUtils.formatLong(p.skuId()), info);
                includedCount++;
            }

            responseObserver.onNext(responseBuilder.build());
            responseObserver.onCompleted();

            log.info("success",
                    String.format("批次查詢 SKU 購物車資訊成功 requested=%d included=%d orphan=%d",
                            skuIds.size(), includedCount, orphanCount));

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchGetSkuCartInfoResponse.newBuilder(), e, "ProductGrpcService", "batchGetSkuCartInfo");
        }
    }

    /**
     * 解析 shipping_method JSONB 為配送溫層集合（保留首次出現順序）。
     * 解析失敗時 log warn 並回空集合（不擋 cart 顯示）。
     */
    private Set<String> parseDeliveryTemps(String shippingMethodJson, Long skuId) {
        if (shippingMethodJson == null || shippingMethodJson.isEmpty() || "null".equals(shippingMethodJson)) {
            return java.util.Collections.emptySet();
        }
        try {
            List<ProductSkuEntity.ShippingMethodItem> items = SHIPPING_METHOD_MAPPER.readValue(
                    shippingMethodJson, SHIPPING_METHOD_TYPE_REF);
            Set<String> temps = new java.util.LinkedHashSet<>();
            for (ProductSkuEntity.ShippingMethodItem item : items) {
                if (item.getTemperature() != null && !item.getTemperature().isEmpty()) {
                    temps.add(item.getTemperature());
                }
            }
            return temps;
        } catch (Exception e) {
            log.warn("shippingMethodParseFailed",
                    String.format("shipping_method JSON 解析失敗 skuId=%d error=%s",
                            skuId, e.getMessage()),
                    "skuId", String.valueOf(skuId));
            return java.util.Collections.emptySet();
        }
    }

    // ==================== BatchGetProductForCheckout（Order Service Checkout 呼叫） ====================
    // 02 設計書 §5.5.2：結帳鏈路批次商品查詢，接受 inactive，回完整 weight/dimension/shipping_methods。
    // 取代 GetPublicProduct 在 checkout flow 內的呼叫（後者對 inactive throw 致 unavailable_items 永遠空）。

    @Override
    public void batchGetProductForCheckout(BatchGetProductForCheckoutRequest request,
                                            StreamObserver<BatchGetProductForCheckoutResponse> responseObserver) {
        try {
            List<String> rawIds = request.getSkuIdsList();

            if (rawIds.isEmpty()) {
                responseObserver.onNext(BatchGetProductForCheckoutResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (rawIds.size() > 100) {
                responseObserver.onNext(BatchGetProductForCheckoutResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 解析 SKU ID
            List<Long> skuIds = new java.util.ArrayList<>();
            for (String idStr : rawIds) {
                Long id = ProtoConversionUtils.parseLongOrNull(idStr);
                if (id != null) {
                    skuIds.add(id);
                }
            }

            // 批次查 SKU（不過濾 status / deleted_at）
            List<ProductSkuEntity> skus = productService.findSkusByIds(skuIds);

            // 批次查對應 Product（不過濾 status / deleted_at）
            Set<Long> productIds = new java.util.HashSet<>();
            for (ProductSkuEntity sku : skus) {
                productIds.add(sku.getProductId());
            }
            Map<Long, ProductEntity> productMap = new java.util.HashMap<>();
            if (!productIds.isEmpty()) {
                var products = productService.findProductsByIds(new java.util.ArrayList<>(productIds));
                for (var p : products) {
                    productMap.put(p.getId(), p);
                }
            }

            BatchGetProductForCheckoutResponse.Builder responseBuilder = BatchGetProductForCheckoutResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code());

            for (ProductSkuEntity sku : skus) {
                ProductEntity product = productMap.get(sku.getProductId());
                if (product == null) {
                    // SKU 存在但對應 product 缺失 → key 缺席（caller 視為 LOOKUP_FAILED）
                    continue;
                }

                boolean isActive = ProductStatus.ACTIVE.value().equals(product.getStatus())
                        && product.getDeletedAt() == null;

                // price = sale_price 優先，否則 list price
                Long effectivePrice = sku.getSalePrice() != null ? sku.getSalePrice() : sku.getPrice();

                // image_object_key：取 product.gallery[0].objectKey
                String imageObjectKey = "";
                if (product.getGallery() != null && !product.getGallery().isEmpty()) {
                    var first = product.getGallery().get(0);
                    if (first != null && first.getObjectKey() != null) {
                        imageObjectKey = first.getObjectKey();
                    }
                }

                // 將 SKU shipping_methods JSONB 轉成 proto ShippingMethod；同時取 temperature
                ProductForCheckoutInfo.Builder infoBuilder = ProductForCheckoutInfo.newBuilder()
                        .setSkuId(ProtoConversionUtils.formatLong(sku.getId()))
                        .setProductId(ProtoConversionUtils.formatLong(product.getId()))
                        .setStoreId(ProtoConversionUtils.formatLong(product.getStoreId()))
                        .setIsActive(isActive)
                        .setPrice(ProtoConversionUtils.formatLong(effectivePrice))
                        .setListPrice(ProtoConversionUtils.formatLong(sku.getPrice()))
                        .setSalePrice(sku.getSalePrice() != null ? ProtoConversionUtils.formatLong(sku.getSalePrice()) : "")
                        .setStockQty(ProtoConversionUtils.formatInt(sku.getStockQty()))
                        .setSkuName(sku.getSkuName() != null ? sku.getSkuName() : "")
                        .setProductName(product.getName() != null ? product.getName() : "")
                        .setImageObjectKey(imageObjectKey)
                        .setWeight(sku.getWeight() != null ? sku.getWeight().toPlainString() : "0")
                        .setUnit(sku.getUnit() != null ? sku.getUnit() : "")
                        .setLengthCm(sku.getLengthCm() != null ? sku.getLengthCm().toPlainString() : "0")
                        .setWidthCm(sku.getWidthCm() != null ? sku.getWidthCm().toPlainString() : "0")
                        .setHeightCm(sku.getHeightCm() != null ? sku.getHeightCm().toPlainString() : "0");

                String temperature = "";
                if (sku.getShippingMethod() != null) {
                    for (var item : sku.getShippingMethod()) {
                        if (item == null) continue;
                        infoBuilder.addShippingMethods(ShippingMethod.newBuilder()
                                .setType(item.getType() != null ? item.getType() : "")
                                .setTemperature(item.getTemperature() != null ? item.getTemperature() : "")
                                .setFreeThreshold(item.getFreeThreshold() != null ? item.getFreeThreshold() : "")
                                .setFeePayer(item.getFeePayer() != null ? item.getFeePayer() : "")
                                .build());
                        if (temperature.isEmpty() && item.getTemperature() != null && !item.getTemperature().isEmpty()) {
                            temperature = item.getTemperature();
                        }
                    }
                }
                infoBuilder.setTemperature(temperature);

                responseBuilder.putData(ProtoConversionUtils.formatLong(sku.getId()), infoBuilder.build());
            }

            responseObserver.onNext(responseBuilder.build());
            responseObserver.onCompleted();

            log.info("ProductGrpcService.batchGetProductForCheckout.success",
                    "結帳鏈路批次查詢成功，requested=" + skuIds.size() + " returned=" + responseBuilder.getDataCount());

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchGetProductForCheckoutResponse.newBuilder(), e,
                    "ProductGrpcService", "batchGetProductForCheckout");
        }
    }

    // ==================== GetProductsByIDs（Buyer Service 瀏覽紀錄呼叫） ====================

    @Override
    public void getProductsByIDs(GetProductsByIDsRequest request,
                                  StreamObserver<GetProductsByIDsResponse> responseObserver) {
        try {
            List<String> rawIds = request.getProductIdsList();

            // 空列表 → 直接回傳空結果
            if (rawIds.isEmpty()) {
                responseObserver.onNext(GetProductsByIDsResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("查詢成功")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 超過上限 → 回傳錯誤
            if (rawIds.size() > 100) {
                responseObserver.onNext(GetProductsByIDsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("商品 ID 數量不可超過 100")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 解析 ID，跳過無效格式
            List<Long> productIds = new java.util.ArrayList<>();
            for (String idStr : rawIds) {
                try {
                    productIds.add(Long.parseLong(idStr));
                } catch (NumberFormatException ignored) {
                    // 跳過無效 ID
                }
            }

            // 批次查詢；includeInactive=false（預設）僅回 ACTIVE，true 含 inactive（02 §5.5.1）
            boolean includeInactive = request.getIncludeInactive();
            log.info("ProductGrpcService.getProductsByIDs.start",
                    "開始批次查詢商品，product_ids=" + productIds + "，include_inactive=" + includeInactive);
            List<ProductListItemDTO> items = productService.getProductsByIds(productIds, includeInactive);

            // 組建回應
            GetProductsByIDsResponse.Builder responseBuilder = GetProductsByIDsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");

            for (ProductListItemDTO item : items) {
                responseBuilder.addProducts(protoConverter.toProductListItemProto(item));
            }

            responseObserver.onNext(responseBuilder.build());
            responseObserver.onCompleted();

            log.info("ProductGrpcService.getProductsByIDs.success",
                    "批次查詢商品成功，requested=" + rawIds.size() + "，returned=" + items.size());

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetProductsByIDsResponse.newBuilder(), e, "ProductGrpcService", "getProductsByIDs");
        }
    }

    /**
     * 取得相關商品（同分類、排除自身、僅 ACTIVE、上限 8 筆）
     */
    @Override
    public void getRelatedProducts(GetRelatedProductsRequest request,
            StreamObserver<GetRelatedProductsResponse> responseObserver) {
        try {
            String productId = request.getProductId();
            if (productId.isEmpty()) {
                responseObserver.onNext(GetRelatedProductsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 解析上限（預設 8）
            int limit = 8;
            if (!request.getLimit().isEmpty()) {
                try {
                    int parsed = Integer.parseInt(request.getLimit());
                    if (parsed > 0 && parsed <= 20) {
                        limit = parsed;
                    }
                } catch (NumberFormatException ignored) {
                    // 使用預設值
                }
            }

            // 取得商品的分類 ID
            ProductDetailDTO product;
            try {
                product = productService.getPublicProduct(productId);
            } catch (ProductNotFoundException e) {
                // 商品不存在，回傳空列表
                responseObserver.onNext(GetRelatedProductsResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("查詢成功")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 查詢同分類的 ACTIVE 商品（多取一筆以便排除自身後仍有足夠數量）
            ProductQueryDTO query = ProductQueryDTO.builder()
                    .categoryIds(product.getCategoryId() != null
                            ? java.util.List.of(product.getCategoryId())
                            : null)
                    .status(ProductStatus.ACTIVE.value())
                    .page(1)
                    .pageSize(limit + 1)
                    .publicApi(true) // 02 §2.7.2：公開讀路徑，套 onboarding filter
                    .build();

            ProductListResultDTO result = productService.getProductList(query);

            // 轉換並排除自身
            GetRelatedProductsResponse.Builder responseBuilder = GetRelatedProductsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("查詢成功");

            int count = 0;
            for (ProductListItemDTO item : result.getProducts()) {
                if (item.getId().equals(productId)) {
                    continue;
                }
                responseBuilder.addProducts(protoConverter.toProductListItemProto(item));
                count++;
                if (count >= limit) {
                    break;
                }
            }

            responseObserver.onNext(responseBuilder.build());
            log.info("ProductGrpcService.getRelatedProducts.success",
                    "查詢相關商品成功，product_id=" + productId + "，returned=" + count);

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetRelatedProductsResponse.newBuilder(), e, "ProductGrpcService", "getRelatedProducts");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 軟刪除商品（賣家用）
     * 僅 DRAFT 或 INACTIVE 狀態可刪除
     */
    @Override
    public void deleteProduct(DeleteProductRequest request, StreamObserver<DeleteProductResponse> responseObserver) {
        try {
            // 授權：僅 SELLER 可刪除商品
            GrpcAuthContext.requireAccountType(AccountType.SELLER);

            // 驗證必填欄位
            if (request.getProductId().isEmpty()) {
                responseObserver.onNext(DeleteProductResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            productService.softDeleteProduct(request.getProductId(), request.getStoreId());

            responseObserver.onNext(DeleteProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("商品刪除成功")
                    .build());

            log.info("ProductGrpcService.deleteProduct.success", "商品軟刪除成功",
                    "product_id", request.getProductId());

        } catch (ProductNotFoundException e) {
            responseObserver.onNext(DeleteProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (ProductAccessDeniedException e) {
            // 越權防枚舉（API 設計規範 §14.1）：回應與「商品不存在」一致，對齊 getProduct
            responseObserver.onNext(DeleteProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.NOT_FOUND.code())
                    .setMessage("商品不存在")
                    .build());
        } catch (IllegalStateException e) {
            responseObserver.onNext(DeleteProductResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(DeleteProductResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, DeleteProductResponse.newBuilder(), e, "ProductGrpcService", "deleteProduct");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 驗證認證編號是否存在於政府開放資料平台
     * 僅限 Seller (account_type: 2, 3)
     */
    @Override
    public void verifyCertification(VerifyCertificationRequest request,
                                    StreamObserver<VerifyCertificationResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);

            String certNo = request.getCertNo();
            String type = request.getType();

            // 驗證參數
            if (certNo == null || certNo.isBlank()) {
                responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                        .setCode(GrpcResponseCode.CERT_NO_REQUIRED.code())
                        .setMessage(GrpcResponseCode.CERT_NO_REQUIRED.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (!CertificationType.isValidValue(type)) {
                log.warn("verifyCertification.invalidType", "收到無效的認證類型", "type", type);
                responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                        .setCode(GrpcResponseCode.CERT_TYPE_INVALID.code())
                        .setMessage(GrpcResponseCode.CERT_TYPE_INVALID.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            CertificationType certType = CertificationType.fromValue(type);

            // 第二類認證暫不支援
            if (!CertificationTypeRules.isVerifiable(certType)) {
                responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                        .setCode(GrpcResponseCode.CERT_TYPE_NOT_VERIFIABLE.code())
                        .setMessage(GrpcResponseCode.CERT_TYPE_NOT_VERIFIABLE.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 字元白名單，規則與 gateway product.go 的 certNoPattern 同一組。
            // certNo 會被 GovCertificationService 原字串接進政府 API 的查詢字串，而該 URL
            // 尾端帶 api_key；放行 URI 保留字元等於讓呼叫端隨手把金鑰逼進例外訊息。
            // 這裡不把 certNo 本身寫進 log——值由呼叫端控制。
            //
            // ⚠️ 必須擺在「類型可線上驗證」判斷之後：HACCP/ISO/GMP 的 certNo 永遠不會被送進
            // 政府 API，白名單對它們沒有任何安全效益，卻會把 `ISO 22000:2018` 這種合法編號
            // 從 501（暫不支援線上驗證）改判成 400，等於無故改掉對外契約。
            // 真正的安全邊界在 GovCertificationService.verify() 開頭的同一道檢查，這裡只是
            // 可驗證類型的提前退場。
            if (!GovCertificationService.isValidCertNo(certNo)) {
                log.warn("verifyCertification.invalidCertNo", "認證編號含不合法字元或超出長度上限");
                responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("認證編號格式不正確")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // §9.4：回應由 {exists} 擴充為結構化。
            // 🔴 exists 欄位必須保留（= found && validity == VALID），漏了就等於直接對舊 App
            // 做了破壞性變更；新前端一律讀 result.validity——「已失效」會讓 exists=false，
            // 塌成「查無此編號」會給賣家錯誤的文案（§8.6）。
            GovCertificationService.GovVerifyResult verifyResult =
                    govCertificationService.verifyDetailed(certNo, certType);

            responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setExists(verifyResult.exists())
                    .setResult(toGovVerifyResultProto(verifyResult))
                    .build());

        } catch (GovCertificationService.GovApiUnavailableException e) {
            responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                    .setCode(GrpcResponseCode.GOV_CERT_API_UNAVAILABLE.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (GovCertificationService.GovApiResponseException e) {
            responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                    .setCode(GrpcResponseCode.GOV_CERT_API_RESPONSE_INVALID.code())
                    .setMessage(e.getMessage())
                    .build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(VerifyCertificationResponse.newBuilder()
                    .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                            ? GrpcResponseCode.UNAUTHORIZED.code()
                            : GrpcResponseCode.PERMISSION_DENIED.code())
                    .setMessage(e.getStatus().getDescription())
                    .build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, VerifyCertificationResponse.newBuilder(), e, "ProductGrpcService", "verifyCertification");
            return;
        }
        responseObserver.onCompleted();
    }

    // ==================== Admin 分類管理 ====================

    /**
     * 取分類底下所有商品（admin 用，含分頁，依層級判斷）
     */
    @Override
    public void listProductsByCategoryForAdmin(ListProductsByCategoryForAdminRequest request,
                                               StreamObserver<ListProductsByCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            long categoryId = ProtoConversionUtils.parseLongOrDefault(request.getCategoryId(), 0L);
            int page = (int) ProtoConversionUtils.parseLongOrDefault(request.getPage(), 1L);
            int pageSize = (int) ProtoConversionUtils.parseLongOrDefault(request.getPageSize(), 20L);

            ProductListResultDTO result = productService.listProductsByCategory(categoryId, page, pageSize);

            ListProductsByCategoryForAdminResponse.Builder builder = ListProductsByCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getPage()))
                    .setPageSize(String.valueOf(result.getPageSize()));

            for (ProductListItemDTO item : result.getProducts()) {
                builder.addProducts(protoConverter.toProductListItemProto(item));
            }
            responseObserver.onNext(builder.build());

            log.info("listProductsByCategoryForAdmin.success",
                    "category_id=" + categoryId + "，total=" + result.getTotal());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListProductsByCategoryForAdminResponse.newBuilder(), e, "ProductGrpcService", "listProductsByCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 批次更換商品分類（admin 用，限 L3→L3，連動 lv1/lv2）
     */
    @Override
    public void batchChangeProductCategoryForAdmin(BatchChangeProductCategoryForAdminRequest request,
                                                   StreamObserver<BatchChangeProductCategoryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            java.util.List<Long> productIds = request.getProductIdsList().stream()
                    .map(Long::parseLong)
                    .toList();
            Long sourceCategoryId = request.getSourceCategoryId().isEmpty()
                    ? null
                    : Long.parseLong(request.getSourceCategoryId());
            Long targetCategoryId = request.getTargetCategoryId().isEmpty()
                    ? null
                    : Long.parseLong(request.getTargetCategoryId());

            int affected = productService.batchChangeProductCategory(
                    productIds, sourceCategoryId, targetCategoryId, request.getUpdatedBy());

            responseObserver.onNext(BatchChangeProductCategoryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setAffectedCount(String.valueOf(affected))
                    .build());

            log.info("batchChangeProductCategoryForAdmin.success",
                    "affected=" + affected + "，target=" + targetCategoryId);
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchChangeProductCategoryForAdminResponse.newBuilder(), e, "ProductGrpcService", "batchChangeProductCategoryForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * Hashtag 自動補全（02 §5.12）。
     * Public/Seller 皆可呼叫；空 prefix 直接回空陣列。
     */
    @Override
    public void searchHashtags(io.github.will413028.hawkr.product.grpc.SearchHashtagsRequest request,
                               StreamObserver<io.github.will413028.hawkr.product.grpc.SearchHashtagsResponse> responseObserver) {
        try {
            io.github.will413028.hawkr.product.service.HashtagService.SearchMode mode =
                    request.getMode() == io.github.will413028.hawkr.product.grpc.HashtagSearchMode.HASHTAG_SEARCH_CONTAIN
                            ? io.github.will413028.hawkr.product.service.HashtagService.SearchMode.CONTAIN
                            : io.github.will413028.hawkr.product.service.HashtagService.SearchMode.PREFIX;

            java.util.List<io.github.will413028.hawkr.product.service.HashtagService.HashtagDisplayItem> items =
                    hashtagService.searchHashtags(request.getPrefix(), mode, request.getLimit());

            io.github.will413028.hawkr.product.grpc.SearchHashtagsResponse.Builder respBuilder =
                    io.github.will413028.hawkr.product.grpc.SearchHashtagsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success");

            for (io.github.will413028.hawkr.product.service.HashtagService.HashtagDisplayItem h : items) {
                respBuilder.addItems(io.github.will413028.hawkr.product.grpc.HashtagItem.newBuilder()
                        .setName(h.canonicalName() != null ? h.canonicalName() : "")
                        .setDisplayName(h.displayName() != null ? h.displayName() : "")
                        .setProductCount(h.productCount())
                        .build());
            }

            responseObserver.onNext(respBuilder.build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.product.grpc.SearchHashtagsResponse.newBuilder(), e,
                    "ProductGrpcService", "searchHashtags");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 區間熱門 Hashtag 查詢（33-商品 Hashtag 模組詳細設計書 §5.2）。
     * 匿名公開 API；依日期區間回傳被最多 ACTIVE 商品掛上的 hashtag 排行。
     */
    @Override
    public void getPopularHashtags(io.github.will413028.hawkr.product.grpc.GetPopularHashtagsRequest request,
                                   StreamObserver<io.github.will413028.hawkr.product.grpc.GetPopularHashtagsResponse> responseObserver) {
        try {
            java.util.List<io.github.will413028.hawkr.product.dto.response.PopularHashtagItemDTO> records =
                    popularHashtagService.getPopularHashtags(
                            request.getStartFrom(),
                            request.getEndTo(),
                            request.getLimit());

            io.github.will413028.hawkr.product.grpc.GetPopularHashtagsResponse.Builder respBuilder =
                    io.github.will413028.hawkr.product.grpc.GetPopularHashtagsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success");

            for (io.github.will413028.hawkr.product.dto.response.PopularHashtagItemDTO r : records) {
                respBuilder.addRecords(io.github.will413028.hawkr.product.grpc.PopularHashtagItem.newBuilder()
                        .setHashtag(r.getHashtag() != null ? r.getHashtag() : "")
                        .setCount(r.getCount() != null ? r.getCount() : "0")
                        .build());
            }

            responseObserver.onNext(respBuilder.build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.product.grpc.GetPopularHashtagsResponse.newBuilder(), e,
                    "ProductGrpcService", "getPopularHashtags");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 公開商品產地聚合（02 §5.13）
     */
    @Override
    public void getPublicOriginsSummary(
            io.github.will413028.hawkr.product.grpc.GetPublicOriginsSummaryRequest request,
            StreamObserver<io.github.will413028.hawkr.product.grpc.GetPublicOriginsSummaryResponse> responseObserver) {
        try {
            // category_id: "0" 或空字串 = 全部
            Long categoryId = null;
            String rawCat = request.getCategoryId();
            if (rawCat != null && !rawCat.isEmpty() && !"0".equals(rawCat)) {
                try {
                    long parsed = Long.parseLong(rawCat);
                    if (parsed > 0) categoryId = parsed;
                } catch (NumberFormatException nfe) {
                    throw new IllegalArgumentException("category_id 必須為數字字串");
                }
            }

            List<io.github.will413028.hawkr.product.repository.ProductRepository.OriginSummaryRow> rows =
                    productService.getPublicOriginsSummary(categoryId, request.getLimit());

            io.github.will413028.hawkr.product.grpc.GetPublicOriginsSummaryResponse.Builder respBuilder =
                    io.github.will413028.hawkr.product.grpc.GetPublicOriginsSummaryResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success");

            for (var row : rows) {
                respBuilder.addRecords(io.github.will413028.hawkr.product.grpc.OriginSummaryItem.newBuilder()
                        .setOrigin(row.getOrigin() != null ? row.getOrigin() : "")
                        .setProductCount(row.getProductCount() != null ? row.getProductCount() : "0")
                        .build());
            }

            responseObserver.onNext(respBuilder.build());
            log.info("ProductGrpcService.getPublicOriginsSummary.success",
                    "查詢產地聚合成功，count=" + rows.size());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.product.grpc.GetPublicOriginsSummaryResponse.newBuilder(), e,
                    "ProductGrpcService", "getPublicOriginsSummary");
            return;
        }
        responseObserver.onCompleted();
    }

    @Override
    public void getPublicSellerCitiesSummary(
            io.github.will413028.hawkr.product.grpc.GetPublicSellerCitiesSummaryRequest request,
            StreamObserver<io.github.will413028.hawkr.product.grpc.GetPublicSellerCitiesSummaryResponse> responseObserver) {
        try {
            var rows = productService.getPublicSellerCitiesSummary();
            var response = io.github.will413028.hawkr.product.grpc.GetPublicSellerCitiesSummaryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success");
            for (var row : rows) {
                response.addRecords(io.github.will413028.hawkr.product.grpc.SellerCitySummaryItem.newBuilder()
                        .setCityCode(row.cityCode())
                        .setCityName(row.cityName())
                        .setProductCount(Long.toString(row.productCount()))
                        .setSellerCount(Long.toString(row.sellerCount()))
                        .addAllPreviewStoreIds(row.previewStoreIds())
                        .build());
            }
            responseObserver.onNext(response.build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.product.grpc.GetPublicSellerCitiesSummaryResponse.newBuilder(), e,
                    "ProductGrpcService", "getPublicSellerCitiesSummary");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 有公開上架商品的 store_id 全集（02 §5.13c）——內部 RPC，seller-service 用來把
     * 空店排除在推薦賣家 / marketplace rail 之外。
     *
     * <p>失敗一律走 GrpcErrorHandler 回非零 code；呼叫端據此 fail-open（不過濾），
     * 所以這裡**不得**把錯誤降級成空集合——空集合的語意是「全平台沒有任何公開商品」。
     */
    @Override
    public void listStoreIDsWithPublicProductsInternal(
            io.github.will413028.hawkr.product.grpc.ListStoreIDsWithPublicProductsInternalRequest request,
            StreamObserver<io.github.will413028.hawkr.product.grpc.ListStoreIDsWithPublicProductsInternalResponse> responseObserver) {
        try {
            List<Long> storeIds = productService.listStoreIDsWithPublicProducts();
            var response = io.github.will413028.hawkr.product.grpc.ListStoreIDsWithPublicProductsInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success");
            for (Long storeId : storeIds) {
                response.addStoreIds(Long.toString(storeId));
            }
            responseObserver.onNext(response.build());
            log.info("ProductGrpcService.listStoreIDsWithPublicProductsInternal.success",
                    "查詢有公開商品的店家成功，count=" + storeIds.size());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.product.grpc.ListStoreIDsWithPublicProductsInternalResponse.newBuilder(), e,
                    "ProductGrpcService", "listStoreIDsWithPublicProductsInternal");
            return;
        }
        responseObserver.onCompleted();
    }

    // ============================================================================
    // Product Import 整合（02 號設計書 §9.1 / §9.2 / §9.5）
    //
    // PR1 階段：純 stub（拋 UNIMPLEMENTED），proto 契約凍結但無行為改動。
    // 完整實作於 PR2，與 product-import-service 本體一起上線。
    // ============================================================================

    /** 02 §9.1 — 驗證 category_id 為合法葉節點並回傳 cat_l1/l2 反正規化資料（內部 RPC，免身份驗證）。 */
    @Override
    public void validateLeafCategoryInternal(
            io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalRequest request,
            io.grpc.stub.StreamObserver<io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse> responseObserver) {
        try {
            io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse response =
                    productImportService.validateLeafCategory(request.getCategoryId());
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (ProductBusinessException e) {
            log.warn("ProductGrpcService.validateLeafCategoryInternal.businessFailed",
                    "category_id=" + request.getCategoryId() + "，error=" + e.getMessage());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .setIsValid(false)
                    .setInvalidReason("NOT_FOUND")
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                    responseObserver,
                    io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse.newBuilder(),
                    e, "ProductGrpcService", "validateLeafCategoryInternal");
        }
    }

    /** 02 §9.2 — 由 Product Import Service 在 commit 階段呼叫，將暫存 item 轉正為 product + product_sku（內部 RPC，免身份驗證）。 */
    @Override
    public void createProductFromImportInternal(
            io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalRequest request,
            io.grpc.stub.StreamObserver<io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse> responseObserver) {
        try {
            io.github.will413028.hawkr.product.service.ProductImportService.CreateProductFromImportInternalResponseDTO result =
                    productImportService.createProductFromImport(request);
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setProductId(result.getProductId())
                    .setIdempotencyStatus(result.getIdempotencyStatus())
                    .build());
            responseObserver.onCompleted();
            log.info("ProductGrpcService.createProductFromImportInternal.success",
                    "Import 商品轉正成功，product_id=" + result.getProductId()
                            + "，import_item_id=" + request.getImportItemId()
                            + "，status=" + result.getIdempotencyStatus());
        } catch (IdempotencyConflictException e) {
            log.warn("ProductGrpcService.createProductFromImportInternal.conflict",
                    "冪等衝突: " + e.getMessage(), "key", request.getIdempotencyKey());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (ProductBusinessException e) {
            log.warn("ProductGrpcService.createProductFromImportInternal.businessFailed",
                    "code=" + e.code() + "，msg=" + e.getMessage()
                            + "，import_item_id=" + request.getImportItemId());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.warn("ProductGrpcService.createProductFromImportInternal.invalidParam",
                    "參數錯誤: " + e.getMessage());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                    responseObserver,
                    io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse.newBuilder(),
                    e, "ProductGrpcService", "createProductFromImportInternal");
        }
    }

    /** 02 §9.5 — 批次偵測商品名稱是否與賣家既有商品重複（內部 RPC，免身份驗證）。 */
    @Override
    public void batchCheckSellerProductNames(
            io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesRequest request,
            io.grpc.stub.StreamObserver<io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse> responseObserver) {
        try {
            List<io.github.will413028.hawkr.product.entity.projection.SellerProductNameProjection> existing =
                    productImportService.batchCheckSellerProductNames(
                            request.getStoreId(),
                            request.getNamesList());

            io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse.Builder builder =
                    io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success");
            for (io.github.will413028.hawkr.product.entity.projection.SellerProductNameProjection row : existing) {
                builder.addExisting(io.github.will413028.hawkr.product.grpc.ExistingProductByName.newBuilder()
                        .setName(row.name() != null ? row.name() : "")
                        .setProductId(String.valueOf(row.id()))
                        .setStatus(row.status() != null ? row.status() : "")
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
            log.info("ProductGrpcService.batchCheckSellerProductNames.success",
                    "store_id=" + request.getStoreId()
                            + "，requested=" + request.getNamesCount()
                            + "，matched=" + existing.size());
        } catch (ProductBusinessException e) {
            log.warn("ProductGrpcService.batchCheckSellerProductNames.businessFailed",
                    "code=" + e.code() + "，msg=" + e.getMessage());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                    responseObserver,
                    io.github.will413028.hawkr.product.grpc.BatchCheckSellerProductNamesResponse.newBuilder(),
                    e, "ProductGrpcService", "batchCheckSellerProductNames");
        }
    }

    // ============================================================================
    // 40 號設計書 §4.3a / §4.3b — Product Publish Validator + Brief Lookup
    // ============================================================================

    /** 40 §4.3a — 商品 draft 統一驗證（CSV_PARSE / IMPORT_UPDATE / IMPORT_COMMIT / PUBLISH 四時機點共用）。 */
    @Override
    public void validateProductDraft(
            io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest request,
            io.grpc.stub.StreamObserver<io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse> responseObserver) {
        try {
            io.github.will413028.hawkr.product.service.validator.ProductPublishValidator.ValidationResult result =
                    productPublishValidator.validate(
                            request.getStoreId(),
                            request.getDraft(),
                            request.getContext());

            io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse.Builder builder =
                    io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success")
                            .setHasBlockingError(result.hasBlockingError());
            for (io.github.will413028.hawkr.product.service.validator.ProductPublishValidator.Violation v : result.violations()) {
                builder.addViolations(io.github.will413028.hawkr.product.grpc.DraftViolation.newBuilder()
                        .setField(v.field() == null ? "" : v.field())
                        .setCode(v.code() == null ? "" : v.code())
                        .setSeverity(v.severity() == null ? "" : v.severity())
                        .setMessage(v.message() == null ? "" : v.message())
                        .setRawValue(v.rawValue() == null ? "" : v.rawValue())
                        .build());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
            log.info("ProductGrpcService.validateProductDraft.success",
                    "store_id=" + request.getStoreId()
                            + "，context=" + request.getContext().name()
                            + "，violations=" + result.violations().size()
                            + "，blocking=" + result.hasBlockingError());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                    responseObserver,
                    io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse.newBuilder(),
                    e, "ProductGrpcService", "validateProductDraft");
        }
    }

    /** 40 §4.3b — 批次取商品輕量資訊（id/name/status），上限 200，查不到的 id 不出現。 */
    @Override
    public void getProductsBriefInternal(
            io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalRequest request,
            io.grpc.stub.StreamObserver<io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse> responseObserver) {
        try {
            List<String> rawIds = request.getProductIdsList();
            if (rawIds.size() > 200) {
                throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM,
                        "product_ids 上限為 200，當前=" + rawIds.size());
            }
            List<Long> ids = new java.util.ArrayList<>();
            for (String s : rawIds) {
                if (s == null || s.isBlank()) continue;
                try {
                    ids.add(Long.parseLong(s.trim()));
                } catch (NumberFormatException e) {
                    // 跳過格式錯誤的 id（per §4.3b：查不到的 id 不出現）
                }
            }

            io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse.Builder builder =
                    io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage("success");
            if (!ids.isEmpty()) {
                // #2819：刻意用**不濾 deleted_at** 的查詢。用 findBriefByIds 的話，已軟刪的
                // 商品不在結果集 ⇒ 呼叫端的 map miss ⇒ 與「RPC 整支失敗」壓成同一個空字串，
                // 而賣家的匯入紀錄無從區分（連結還會指向 404 / 3023 已永久移除的商品頁）。
                // 「已移除」由 publication_state 表達，不塞進 status（那個 enum 沒有這個值）。
                List<io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection> rows =
                        productRepository.findPublicationBriefByIdsIncludingDeleted(ids);
                for (io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection row : rows) {
                    String statusValue = row.status() == null ? "" : row.status();
                    String statusLabel = "";
                    if (ProductStatus.isValidValue(statusValue)) {
                        statusLabel = ProductStatus.fromValue(statusValue).label();
                    }
                    builder.addRecords(io.github.will413028.hawkr.product.grpc.ProductBriefInternal.newBuilder()
                            .setProductId(String.valueOf(row.id()))
                            .setName(row.name() == null ? "" : row.name())
                            .setStatus(statusValue)
                            .setStatusLabel(statusLabel)
                            .setPublicationState(ProductPublicationPolicy.deriveState(
                                    row.firstPublishedAt(), statusValue,
                                    row.deletedAt(), row.sellerDeletedAt(), row.storeExposable()).value())
                            .build());
                }
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
            log.info("ProductGrpcService.getProductsBriefInternal.success",
                    "caller_store_id=" + request.getCallerStoreId()
                            + "，requested=" + rawIds.size()
                            + "，returned=" + builder.getRecordsCount());
        } catch (ProductBusinessException e) {
            log.warn("ProductGrpcService.getProductsBriefInternal.businessFailed",
                    "code=" + e.code() + "，msg=" + e.getMessage());
            responseObserver.onNext(io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                    responseObserver,
                    io.github.will413028.hawkr.product.grpc.GetProductsBriefInternalResponse.newBuilder(),
                    e, "ProductGrpcService", "getProductsBriefInternal");
        }
    }

    // ==================== 農產品認證申請與審核（19 號設計書 §9 / §10.1） ====================
    //
    // 角色分離（§5.1 gRPC 設計）：賣家 3 支、Admin 7 支，各自獨立 RPC，
    // 不以 flag 分流角色；賣家版與 Admin 版連 Entity→Proto 的轉換方法都是分開的兩支
    // （toSellerProto / toAdminProto），gov_verify_payload 與 internal_note
    // 在賣家路徑上是「結構上拿不到」，不是靠記得別填。

    /**
     * 賣家送出單筆認證申請（§9.1）。
     *
     * <p>與商品寫入路徑的差別：單筆明確動作 ⇒ 併發衝突回 typed 409（{@code 3029}），
     * 而不是靜默跳過落進 {@code certificationWarnings[]}。
     */
    @Override
    public void submitProductCertification(SubmitProductCertificationRequest request,
                                           StreamObserver<ProductCertificationApplicationResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            Long productId = ProtoConversionUtils.parseLongOrNull(request.getProductId());
            if (productId == null) {
                responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位且必須為數字字串")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app =
                    certificationApplicationService.submitForSeller(
                            productId, request.getStoreId(), request.getAccountId(),
                            request.getCertificationType(), request.getCertificateNo(),
                            request.getIdempotencyKey());

            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(certificationConverter.toSellerProto(app))
                    .build());
        } catch (IdempotencyConflictException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code()).setMessage(e.getMessage()).build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ProductCertificationApplicationResponse.newBuilder(), e,
                    "ProductGrpcService", "submitProductCertification");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 賣家查自己商品的認證申請（§9.1）。
     *
     * <p>沒有這支端點，賣家 web 的狀態徽章就沒有資料來源 ——
     * 商品詳情回的 {@code certifications} 已是 approved-only 投影，<b>賣家看不到自己的待審</b>。
     */
    @Override
    public void listProductCertificationApplications(
            ListProductCertificationApplicationsRequest request,
            StreamObserver<ListProductCertificationApplicationsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            Long productId = ProtoConversionUtils.parseLongOrNull(request.getProductId());
            if (productId == null) {
                responseObserver.onNext(ListProductCertificationApplicationsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 為必填欄位且必須為數字字串")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            int page = parsePage(request.getPage());
            int pageSize = parsePageSize(request.getPageSize());

            ProductCertificationApplicationService.Paged<
                    io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity> paged =
                    certificationApplicationService.listForSeller(
                            productId, request.getStoreId(),
                            emptyToNullString(request.getStatus()), page, pageSize);

            ListProductCertificationApplicationsResponse.Builder builder =
                    ListProductCertificationApplicationsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message())
                            .setTotal(String.valueOf(paged.total()))
                            .setPage(String.valueOf(page))
                            .setPageSize(String.valueOf(pageSize));
            paged.records().forEach(app -> builder.addRecords(certificationConverter.toSellerProto(app)));
            responseObserver.onNext(builder.build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ListProductCertificationApplicationsResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ListProductCertificationApplicationsResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListProductCertificationApplicationsResponse.newBuilder(), e,
                    "ProductGrpcService", "listProductCertificationApplications");
            return;
        }
        responseObserver.onCompleted();
    }

    /** 賣家撤回申請（§9.1）。取消勾選走這支，不再靠全量取代把它移除。 */
    @Override
    public void withdrawProductCertification(WithdrawProductCertificationRequest request,
                                             StreamObserver<ProductCertificationApplicationResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            Long productId = ProtoConversionUtils.parseLongOrNull(request.getProductId());
            Long applicationId = ProtoConversionUtils.parseLongOrNull(request.getApplicationId());
            if (productId == null || applicationId == null) {
                responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("product_id 與 application_id 為必填欄位且必須為數字字串")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app =
                    certificationApplicationService.withdrawForSeller(
                            productId, applicationId, request.getStoreId(), request.getAccountId(),
                            ProtoConversionUtils.parseLongOrNull(request.getExpectedVersion()),
                            request.getIdempotencyKey());

            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(certificationConverter.toSellerProto(app))
                    .build());
        } catch (IdempotencyConflictException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code()).setMessage(e.getMessage()).build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ProductCertificationApplicationResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ProductCertificationApplicationResponse.newBuilder(), e,
                    "ProductGrpcService", "withdrawProductCertification");
            return;
        }
        responseObserver.onCompleted();
    }

    /** ADM 審核佇列（§9.2）。 */
    @Override
    public void listProductCertificationApplicationsForAdmin(
            ListProductCertificationApplicationsForAdminRequest request,
            StreamObserver<ListProductCertificationApplicationsForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            int page = parsePage(request.getPage());
            int pageSize = parsePageSize(request.getPageSize());

            ProductCertificationApplicationService.Paged<
                    io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity> paged =
                    certificationApplicationService.listForAdmin(
                            emptyToNullString(request.getStatus()),
                            ProtoConversionUtils.parseIntOrNull(request.getCertificationType()),
                            ProtoConversionUtils.parseLongOrNull(request.getStoreId()),
                            emptyToNullString(request.getKeyword()),
                            ProtoConversionUtils.parseIso8601OrNull(request.getSubmittedFrom()),
                            ProtoConversionUtils.parseIso8601OrNull(request.getSubmittedTo()),
                            page, pageSize);

            ListProductCertificationApplicationsForAdminResponse.Builder builder =
                    ListProductCertificationApplicationsForAdminResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message())
                            .setTotal(String.valueOf(paged.total()))
                            .setPage(String.valueOf(page))
                            .setPageSize(String.valueOf(pageSize));
            // 商品名與公開狀態由 Service 另行補齊（Repository 不把它 JOIN 進 Entity，partial projection 禁令）
            Map<Long, ProductPublicationBriefProjection> productBriefs = loadProductBriefsForAdmin(paged.records());
            for (io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app : paged.records()) {
                builder.addRecords(certificationConverter.toAdminProto(
                        app, productBriefs.get(app.getProductId()), null));
            }
            responseObserver.onNext(builder.build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ListProductCertificationApplicationsForAdminResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ListProductCertificationApplicationsForAdminResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListProductCertificationApplicationsForAdminResponse.newBuilder(), e,
                    "ProductGrpcService", "listProductCertificationApplicationsForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /** ADM 申請詳情（§9.2）：比賣家版多 {@code govVerifyPayload}、{@code internalNote}、{@code currentApproved}。 */
    @Override
    public void getProductCertificationApplicationForAdmin(
            GetProductCertificationApplicationForAdminRequest request,
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long applicationId = requireApplicationId(request.getApplicationId());
            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app =
                    certificationApplicationService.getForAdmin(applicationId);
            respondAdminApplication(responseObserver, app);
            return;
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ProductCertificationApplicationAdminResponse.newBuilder(), e,
                    "ProductGrpcService", "getProductCertificationApplicationForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /** ADM 稽核歷程（§9.2）。 */
    @Override
    public void listProductCertificationAuditLogsForAdmin(
            ListProductCertificationAuditLogsForAdminRequest request,
            StreamObserver<ListProductCertificationAuditLogsForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long applicationId = requireApplicationId(request.getApplicationId());
            int page = parsePage(request.getPage());
            int pageSize = parsePageSize(request.getPageSize());

            ProductCertificationApplicationService.Paged<
                    io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity> paged =
                    certificationApplicationService.listAuditLogs(applicationId, page, pageSize);

            ListProductCertificationAuditLogsForAdminResponse.Builder builder =
                    ListProductCertificationAuditLogsForAdminResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message())
                            .setTotal(String.valueOf(paged.total()))
                            .setPage(String.valueOf(page))
                            .setPageSize(String.valueOf(pageSize));
            paged.records().forEach(event -> builder.addRecords(certificationConverter.toAdminProto(event)));
            responseObserver.onNext(builder.build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ListProductCertificationAuditLogsForAdminResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ListProductCertificationAuditLogsForAdminResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListProductCertificationAuditLogsForAdminResponse.newBuilder(), e,
                    "ProductGrpcService", "listProductCertificationAuditLogsForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * ADM 核准（§9.2）。
     *
     * <p>reviewer 身分一律取 auth context（{@code request.getAccountId()} 由 Gateway 注入），
     * <b>不得抄 KYC 那個 {@code rid=1, rname="Admin"} fallback</b>（既有瑕疵）。
     */
    @Override
    public void approveProductCertificationForAdmin(
            ReviewProductCertificationForAdminRequest request,
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver) {
        reviewForAdmin(request, responseObserver, "approveProductCertificationForAdmin",
                (applicationId, expectedVersion) -> certificationApplicationService.approveForAdmin(
                        applicationId, expectedVersion, request.getAccountId(),
                        emptyToNullString(request.getPublicReason()),
                        emptyToNullString(request.getInternalNote()),
                        request.getIdempotencyKey()));
    }

    /** ADM 駁回（§9.2）：{@code reasonCode} 必填（缺 → {@code 3028}）。 */
    @Override
    public void rejectProductCertificationForAdmin(
            ReviewProductCertificationForAdminRequest request,
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver) {
        reviewForAdmin(request, responseObserver, "rejectProductCertificationForAdmin",
                (applicationId, expectedVersion) -> certificationApplicationService.rejectForAdmin(
                        applicationId, expectedVersion, request.getAccountId(),
                        emptyToNullString(request.getReasonCode()),
                        emptyToNullString(request.getPublicReason()),
                        emptyToNullString(request.getInternalNote()),
                        request.getIdempotencyKey()));
    }

    /** ADM 撤銷（§9.2）：已公開標章的唯一治理手段；{@code reasonCode} 必填。 */
    @Override
    public void revokeProductCertificationForAdmin(
            ReviewProductCertificationForAdminRequest request,
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver) {
        reviewForAdmin(request, responseObserver, "revokeProductCertificationForAdmin",
                (applicationId, expectedVersion) -> certificationApplicationService.revokeForAdmin(
                        applicationId, expectedVersion, request.getAccountId(),
                        emptyToNullString(request.getReasonCode()),
                        emptyToNullString(request.getPublicReason()),
                        emptyToNullString(request.getInternalNote()),
                        request.getIdempotencyKey()));
    }

    /**
     * ADM 重新查驗（§9.3）：同步跑一次背景查驗邏輯，<b>只重新查驗、不改 {@code status}</b>。
     *
     * <p>它也是 §15.3 migration 產生的 legacy 列唯一的查驗觸發方式
     * （那些列沒有 MQ 事件，而 §8.3 明令禁止掃 {@code PENDING} 的排程）。
     */
    @Override
    public void reverifyProductCertificationForAdmin(
            ReverifyProductCertificationForAdminRequest request,
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long applicationId = requireApplicationId(request.getApplicationId());
            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app =
                    certificationApplicationService.reverifyForAdmin(
                            applicationId, request.getAccountId(), request.getIdempotencyKey());
            respondAdminApplication(responseObserver, app);
            return;
        } catch (IdempotencyConflictException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code()).setMessage(e.getMessage()).build());
        } catch (ProductBusinessException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ProductCertificationApplicationAdminResponse.newBuilder(), e,
                    "ProductGrpcService", "reverifyProductCertificationForAdmin");
            return;
        }
        responseObserver.onCompleted();
    }

    // ---------- 認證 RPC 的共用 helper ----------

    /** approve / reject / revoke 的共用外殼（動作由 RPC 名稱區分，不用 flag 分流角色或動作）。 */
    @FunctionalInterface
    private interface AdminReviewAction {
        io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity apply(
                Long applicationId, Long expectedVersion);
    }

    private void reviewForAdmin(ReviewProductCertificationForAdminRequest request,
                                StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver,
                                String event,
                                AdminReviewAction action) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Long applicationId = requireApplicationId(request.getApplicationId());
            Long expectedVersion = ProtoConversionUtils.parseLongOrNull(request.getExpectedVersion());
            if (expectedVersion == null) {
                // 缺 expectedVersion ＝ 沒有樂觀鎖保護，會靜默覆蓋掉別人的裁決 ⇒ 直接擋
                responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("expected_version 為必填欄位且必須為數字字串")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app =
                    action.apply(applicationId, expectedVersion);
            respondAdminApplication(responseObserver, app);
            return;
        } catch (IdempotencyConflictException e) {
            log.warn("ProductGrpcService." + event + ".idempotencyConflict", e.getMessage());
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.CONFLICT.code()).setMessage(e.getMessage()).build());
            responseObserver.onCompleted();
            return;
        } catch (ProductBusinessException e) {
            log.warn("ProductGrpcService." + event + ".businessFailed",
                    "code=" + e.code() + "，msg=" + e.getMessage());
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(e.code()).setMessage(e.getMessage()).build());
        } catch (IllegalArgumentException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code()).setMessage(e.getMessage()).build());
        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                    .setCode(authFailureCode(e)).setMessage(e.getStatus().getDescription()).build());
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ProductCertificationApplicationAdminResponse.newBuilder(), e,
                    "ProductGrpcService", event);
            return;
        }
        responseObserver.onCompleted();
    }

    /**
     * 回一筆 Admin 版申請詳情。
     *
     * <p>{@code currentApproved} 不是裝飾：核准這一筆會讓它被 SUPERSEDE（§6.6 ①），
     * ADM 必須先看得到「我核准之後，哪一張標章會被換掉」。
     */
    private void respondAdminApplication(
            StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver,
            io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity app) {
        ProductPublicationBriefProjection productBrief =
                loadProductBriefsForAdmin(List.of(app)).get(app.getProductId());
        io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity currentApproved =
                certificationApplicationService.findCurrentApproved(app).orElse(null);
        responseObserver.onNext(ProductCertificationApplicationAdminResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage(GrpcResponseCode.SUCCESS.message())
                .setData(certificationConverter.toAdminProto(app, productBrief, currentApproved))
                .build());
        responseObserver.onCompleted();
    }

    /**
     * ADM 認證審核路徑補齊商品資訊。
     *
     * <p>🔴 這裡**必須**用 {@code findPublicationBriefByIdsIncludingDeleted} 而不是
     * {@code findBriefByIds}：後者帶 {@code WHERE deleted_at IS NULL}，而認證申請
     * <b>結構上會指向已軟刪的商品</b>——V18 legacy migration 的 INSERT 不濾 deleted_at、
     * 賣家送件後可以刪掉自己的商品，而 {@code updateCertificationProjection} 也刻意不濾
     * （ADM 要能對已下架商品撤銷標章）。用公開視角的查詢會讓 ADM 拿到空白商品名（#2818），
     * 而同一支端點的 keyword 搜尋（{@code findForAdmin} 的 JOIN product）不濾 deleted_at
     * ⇒ 同一個請求裡兩個 SQL 對「這個商品存不存在」給出相反答案。
     *
     * <p>同服務的 MQ 路徑（{@code publishDecidedAfterCommit} → {@code findByIdIncludingDeleted}）
     * 一直是對的，只有 gRPC 路徑沒跟上。
     */
    private Map<Long, ProductPublicationBriefProjection> loadProductBriefsForAdmin(
            List<io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity> apps) {
        if (apps.isEmpty()) {
            return Map.of();
        }
        Set<Long> ids = apps.stream()
                .map(io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity::getProductId)
                .collect(java.util.stream.Collectors.toSet());
        Map<Long, ProductPublicationBriefProjection> briefs = new java.util.HashMap<>();
        for (ProductPublicationBriefProjection brief
                : productRepository.findPublicationBriefByIdsIncludingDeleted(ids)) {
            briefs.put(brief.id(), brief);
        }
        return briefs;
    }

    private Long requireApplicationId(String raw) {
        Long applicationId = ProtoConversionUtils.parseLongOrNull(raw);
        if (applicationId == null) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND);
        }
        return applicationId;
    }

    private static int parsePage(String raw) {
        int page = ProtoConversionUtils.parseIntOrDefault(raw, 1);
        return page < 1 ? 1 : page;
    }

    private static int parsePageSize(String raw) {
        int pageSize = ProtoConversionUtils.parseIntOrDefault(raw, 20);
        if (pageSize < 1) {
            return 20;
        }
        return Math.min(pageSize, 100);
    }

    private static String emptyToNullString(String value) {
        return value == null || value.isBlank() ? null : value;
    }

    /**
     * 把 service 層的認證警告攤進 response builder（19 §9.5）。
     *
     * <p>兩個 response message 各有自己的 {@code addCertificationWarnings}，用 Consumer 收斂
     * 成一份實作，避免兩邊漂移（漏掉任一邊的症狀是該端點的欄位恆空且無訊號）。
     */
    private static void addCertificationWarnings(
            ProductWriteOutcome outcome,
            java.util.function.Consumer<CertificationWarning> sink) {
        for (io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO warning
                : outcome.certificationWarnings()) {
            sink.accept(CertificationWarning.newBuilder()
                    .setCertificationType(warning.certificationType())
                    .setCertificateNo(warning.certificateNo())
                    .setReason(warning.reason())
                    .setMessage(warning.message())
                    .build());
        }
    }

    /** 服務層查驗結果 record → proto（{@code matchedCount} 對外是數字字串，§10.3）。 */
    private static GovVerifyResult toGovVerifyResultProto(GovCertificationService.GovVerifyResult r) {
        GovVerifyResult.Builder builder = GovVerifyResult.newBuilder()
                .setCertType(nullSafe(r.certType()))
                .setCertNo(nullSafe(r.certNo()))
                .setQueriedAt(nullSafe(r.queriedAt()))
                .setFound(r.found())
                .setValidity(r.validity() == null ? "" : r.validity().name())
                .setStatusRaw(nullSafe(r.statusRaw()))
                .setExpiresOn(nullSafe(r.expiresOn()))
                .setHolderName(nullSafe(r.holderName()))
                .setHolderAddress(nullSafe(r.holderAddress()))
                .setIssuerName(nullSafe(r.issuerName()))
                .setProductSummary(nullSafe(r.productSummary()))
                .setMatchedCount(String.valueOf(r.matchedCount()));
        if (r.rawExcerpt() != null) {
            builder.setRawExcerptJson(r.rawExcerpt().toString());
        }
        return builder.build();
    }

    private static String nullSafe(String value) {
        return value == null ? "" : value;
    }

    private static int authFailureCode(StatusRuntimeException e) {
        return e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                ? GrpcResponseCode.UNAUTHORIZED.code()
                : GrpcResponseCode.PERMISSION_DENIED.code();
    }
}
