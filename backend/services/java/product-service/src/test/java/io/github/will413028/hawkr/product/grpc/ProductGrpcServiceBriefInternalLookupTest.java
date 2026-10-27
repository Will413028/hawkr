package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.service.ProductService;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.anyCollection;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code GetProductsBriefInternal} **必須**用不濾可見性的查詢（#2819）。
 *
 * <p>為什麼需要這一支：唯一的跨服務消費端 product-import-service 用它顯示「正式商品現在
 * 什麼狀態」。用帶 {@code deleted_at IS NULL} 的 {@code findBriefByIds}，已軟刪商品不在
 * 結果集 ⇒ 消費端的 map miss ⇒ 與「RPC 整支失敗」壓成同一個空字串，賣家無從分辨，
 * 而「開啟正式商品」按鈕照樣顯示、點下去是 404 / 3023「商品已永久移除」。
 *
 * <p>⚠️ 只驗回傳值不夠：把查詢改回 {@code findBriefByIds} 之後，**正常商品的路徑照樣全綠**
 * ——差別只在已軟刪商品不出現。所以這裡同時守「呼叫了哪一支查詢」與「已軟刪商品的
 * publication_state」。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductGrpcServiceBriefInternalLookupTest {

    @Mock private ProductService productService;
    @Mock private ProductProtoConverter protoConverter;
    @Mock private io.github.will413028.hawkr.product.service.ProductCacheService cacheService;
    @Mock private io.github.will413028.hawkr.product.service.CategoryCacheService categoryCacheService;
    @Mock private org.springframework.data.redis.core.StringRedisTemplate redisTemplate;
    @Mock private io.github.will413028.hawkr.product.service.StockService stockService;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient shippingGrpcClient;
    @Mock private io.github.will413028.hawkr.product.repository.ProductSkuRepository skuRepository;
    @Mock private io.github.will413028.hawkr.product.service.GovCertificationService govCertificationService;
    @Mock private io.github.will413028.hawkr.product.service.HashtagService hashtagService;
    @Mock private io.github.will413028.hawkr.product.service.PopularHashtagService popularHashtagService;
    @Mock private io.github.will413028.hawkr.product.service.ProductImportService productImportService;
    @Mock private io.github.will413028.hawkr.product.service.validator.ProductPublishValidator productPublishValidator;
    @Mock private io.github.will413028.hawkr.product.repository.ProductRepository productRepository;
    @Mock private io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService;
    @Mock private io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter;

    @Mock private StreamObserver<GetProductsBriefInternalResponse> observer;

    private static final long DELETED_PRODUCT_ID = 873101296545857928L;
    private static final long LIVE_PRODUCT_ID = 825045450150940300L;

    private ProductGrpcService service() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    @Test
    @DisplayName("已軟刪商品必須回得出來並帶 publication_state=REMOVED，且不得改用 findBriefByIds")
    void softDeletedProductIsReturnedAsRemoved() {
        when(productRepository.findPublicationBriefByIdsIncludingDeleted(anyCollection()))
                .thenReturn(List.of(new ProductPublicationBriefProjection(
                        DELETED_PRODUCT_ID, "已刪除的商品", ProductStatus.INACTIVE.value(),
                        LocalDateTime.of(2026, 6, 1, 0, 0),   // 曾公開
                        LocalDateTime.of(2026, 8, 1, 0, 0),   // 已軟刪
                        null, true)));

        service().getProductsBriefInternal(
                GetProductsBriefInternalRequest.newBuilder()
                        .addProductIds(String.valueOf(DELETED_PRODUCT_ID))
                        .setCallerStoreId(2639226113L)
                        .build(),
                observer);

        ArgumentCaptor<GetProductsBriefInternalResponse> captor =
                ArgumentCaptor.forClass(GetProductsBriefInternalResponse.class);
        verify(observer).onNext(captor.capture());
        GetProductsBriefInternalResponse resp = captor.getValue();

        assertEquals(1, resp.getRecordsCount(), "已軟刪商品必須出現在結果集，否則消費端無從分辨「已移除」與「查不到」");
        ProductBriefInternal rec = resp.getRecords(0);
        assertEquals(ProductPublicationState.REMOVED.value(), rec.getPublicationState());
        assertNotEquals("", rec.getStatusLabel(),
                "status 仍要是合法的 ProductStatus —— 把 4014 塞進 status 會讓 isValidValue 判 false、label 變空");
        assertEquals("已刪除的商品", rec.getName());

        verify(productRepository, never()).findBriefByIds(anyCollection());
    }

    @Test
    @DisplayName("在架商品的 publication_state 是 PUBLIC，不是 REMOVED")
    void livePublishedProductIsPublic() {
        when(productRepository.findPublicationBriefByIdsIncludingDeleted(anyCollection()))
                .thenReturn(List.of(new ProductPublicationBriefProjection(
                        LIVE_PRODUCT_ID, "正常商品", ProductStatus.ACTIVE.value(),
                        LocalDateTime.of(2026, 6, 1, 0, 0), null, null, true)));

        service().getProductsBriefInternal(
                GetProductsBriefInternalRequest.newBuilder()
                        .addProductIds(String.valueOf(LIVE_PRODUCT_ID))
                        .setCallerStoreId(2639226113L)
                        .build(),
                observer);

        ArgumentCaptor<GetProductsBriefInternalResponse> captor =
                ArgumentCaptor.forClass(GetProductsBriefInternalResponse.class);
        verify(observer).onNext(captor.capture());
        ProductBriefInternal rec = captor.getValue().getRecords(0);

        assertEquals(ProductPublicationState.PUBLIC.value(), rec.getPublicationState());
        assertEquals(ProductStatus.ACTIVE.value(), rec.getStatus());
        assertEquals(ProductStatus.ACTIVE.label(), rec.getStatusLabel());
    }

    @Test
    @DisplayName("賣家帳號刪除也算 REMOVED（deletedAt 與 sellerDeletedAt 是兩個獨立維度）")
    void sellerDeletedAlsoCountsAsRemoved() {
        when(productRepository.findPublicationBriefByIdsIncludingDeleted(anyCollection()))
                .thenReturn(List.of(new ProductPublicationBriefProjection(
                        LIVE_PRODUCT_ID, "賣家已刪帳號的商品", ProductStatus.ACTIVE.value(),
                        LocalDateTime.of(2026, 6, 1, 0, 0), null,
                        LocalDateTime.of(2026, 8, 1, 0, 0), true)));

        service().getProductsBriefInternal(
                GetProductsBriefInternalRequest.newBuilder()
                        .addProductIds(String.valueOf(LIVE_PRODUCT_ID))
                        .setCallerStoreId(2639226113L)
                        .build(),
                observer);

        ArgumentCaptor<GetProductsBriefInternalResponse> captor =
                ArgumentCaptor.forClass(GetProductsBriefInternalResponse.class);
        verify(observer).onNext(captor.capture());

        assertEquals(ProductPublicationState.REMOVED.value(),
                captor.getValue().getRecords(0).getPublicationState());
    }
}
