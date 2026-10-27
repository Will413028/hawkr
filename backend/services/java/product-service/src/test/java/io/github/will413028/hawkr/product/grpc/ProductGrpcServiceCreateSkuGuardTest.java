package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.DelegationCapability;
import io.github.will413028.hawkr.common.grpc.DelegationContext;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.dto.request.CreateProductDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.ProductService;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 「商品至少一個 SKU」的 gRPC 收斂點（{@code createProduct} 的 skus_count 守衛）。
 *
 * <p><b>它守的是什麼</b>：商品的價格 / 庫存 / 材積全部掛在 SKU 上，零 SKU 商品建出來是
 * 「上架但不可購買」的破損狀態——{@code product.price_min} 為 NULL、{@code stock_qty} 為 0，
 * 而 {@code product_sku.price} 是 {@code BIGINT NOT NULL}，因此
 * <b>{@code price_min IS NULL} ⟺ 零 SKU</b>。整條不變式的維繫靠三件事：本守衛（建立側）、
 * {@code ProductService.updateSkus} 的兩個分支（更新側，見
 * {@code ProductServiceUpdateSkusEmptyListTest}）、以及 Gateway binding 的 {@code min=1}。
 *
 * <p><b>為什麼要有這支測試</b>：守衛整段刪掉之後 maven 全綠——沒有任何東西在守它。而它一旦
 * 消失不會有錯誤訊號：RPC 回 code 0、商品建立成功、只是永遠賣不出去。
 *
 * <p><b>為什麼要斷言「沒有呼叫 productService」</b>：只斷言回傳碼的話，「先建再擋」
 * （建完商品才回 INVALID_PARAM）這種錯誤實作照樣會通過——那會留下一筆孤兒商品。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductGrpcServiceCreateSkuGuardTest {

    private static final long STORE_ID = 2685914682L;

    @Mock private ProductService productService;
    @Mock private ProductProtoConverter protoConverter;
    @Mock private io.github.will413028.hawkr.product.service.ProductCacheService cacheService;
    @Mock private io.github.will413028.hawkr.product.service.CategoryCacheService categoryCacheService;
    @Mock private org.springframework.data.redis.core.StringRedisTemplate redisTemplate;
    @Mock private io.github.will413028.hawkr.product.service.StockService stockService;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient shippingGrpcClient;
    @Mock private ProductSkuRepository skuRepository;
    @Mock private io.github.will413028.hawkr.product.service.GovCertificationService govCertificationService;
    @Mock private io.github.will413028.hawkr.product.service.HashtagService hashtagService;
    @Mock private io.github.will413028.hawkr.product.service.PopularHashtagService popularHashtagService;
    @Mock private io.github.will413028.hawkr.product.service.ProductImportService productImportService;
    @Mock private io.github.will413028.hawkr.product.service.validator.ProductPublishValidator productPublishValidator;
    @Mock private io.github.will413028.hawkr.product.repository.ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService;
    @Mock io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter;

    private Context previousContext;

    /**
     * 賣家本人路徑要通過 requireAccountType(SELLER)，該檢查讀 gRPC Context。
     *
     * <p>同時把「守衛之後」的整條 happy path 都 stub 齊——這是刻意的：守衛被拿掉時，零 SKU 的
     * 請求必須一路跑到底並回 code 0，紅字才會直指真正的後果（商品被建出來了）。少 stub 任何一段，
     * 拿掉守衛會炸成 NPE / 500，測試雖然照樣轉紅，但紅的理由是 mock 沒接好，不是不變式被破壞。
     */
    @BeforeEach
    void attachSellerContext() {
        previousContext = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, STORE_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.SELLER.value())
                .attach();

        ProductDetailDTO created = ProductDetailDTO.builder().id("7495464541774413824").name("蓮霧").build();
        when(protoConverter.toCreateDTO(any())).thenReturn(new CreateProductDTO());
        // handler 改走 createProductWithWarnings()（19 §9.5 需要 certificationWarnings[]）
        when(productService.createProductWithWarnings(any()))
                .thenReturn(io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome.of(created, java.util.List.of()));
        when(protoConverter.toProductDetailProto(created)).thenReturn(ProductDetail.newBuilder().build());
    }

    @AfterEach
    void detachSellerContext() {
        Context.current().detach(previousContext);
    }

    private ProductGrpcService buildService() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    private static CreateProductRequest.Builder validRequestWithoutSkus() {
        return CreateProductRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setIdempotencyKey("idem-2701445588392001")
                .setName("蓮霧");
    }

    private static ProductSku aSku() {
        return ProductSku.newBuilder()
                .setSkuName("大顆裝 1kg")
                .setPrice("300")
                .setStockQty("10")
                .build();
    }

    private CreateProductResponse call(CreateProductRequest request) {
        @SuppressWarnings("unchecked")
        StreamObserver<CreateProductResponse> observer = mock(StreamObserver.class);
        buildService().createProduct(request, observer);

        ArgumentCaptor<CreateProductResponse> captor = ArgumentCaptor.forClass(CreateProductResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        return captor.getValue();
    }

    @Test
    @DisplayName("零 SKU → INVALID_PARAM，且商品絕不能已經被建出來")
    void zeroSkusIsRejectedBeforeAnyWrite() {
        CreateProductResponse response = call(validRequestWithoutSkus().build());

        // 這一條先斷言：它才是不變式本身。若寫成「先建再擋」（建完商品才回 INVALID_PARAM），
        // 下面的 code 斷言照樣會通過，但 DB 已經留下一筆零 SKU 的孤兒商品。
        verify(productService, never()).createProductWithWarnings(any());

        assertEquals(GrpcResponseCode.INVALID_PARAM.code(), response.getCode());
        assertTrue(response.getMessage().contains("SKU"),
                "訊息要指出是 SKU 這一條，否則賣家看到的是無從下手的「參數錯誤」");
    }

    @Test
    @DisplayName("代行路徑走同一個守衛（收斂點不是只擋賣家本人）")
    void zeroSkusIsRejectedOnDelegatedPathToo() {
        CreateProductResponse response = call(validRequestWithoutSkus()
                .setDelegation(DelegationContext.newBuilder()
                        .setSessionId("2701445588392001")
                        .setOperatorAdminId("42")
                        .setCapability(DelegationCapability.PRODUCTS_WRITE)
                        .setAuditLogId("7495464541774413824")
                        .build())
                .build());

        verify(productService, never()).createProductWithWarnings(any());
        assertEquals(GrpcResponseCode.INVALID_PARAM.code(), response.getCode());
    }

    @Test
    @DisplayName("控制組：帶一個 SKU 時確實建得成（證明上面兩測不是被別的必填檢查擋掉）")
    void oneSkuPassesTheGuard() {
        CreateProductResponse response = call(validRequestWithoutSkus().addSkus(aSku()).build());

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        verify(productService).createProductWithWarnings(any());
    }
}
