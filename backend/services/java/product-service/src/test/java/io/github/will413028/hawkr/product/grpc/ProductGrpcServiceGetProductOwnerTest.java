package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.service.ProductService;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
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
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 重開店 regression（2026-07-15）：getProduct 的擁有權判斷必須讀
 * request.store_id（gateway 已解析的 store surrogate id），不可讀
 * GrpcAuthContext（請求進入時的原始 JWT accountId）——兩者在賣家重開過店時
 * 會分岔，讀錯會讓賣家看不到自己非上架中的商品。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductGrpcServiceGetProductOwnerTest {

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
    @Mock io.github.will413028.hawkr.product.service.ProductCertificationApplicationService certificationApplicationService;
    @Mock io.github.will413028.hawkr.product.converter.ProductCertificationApplicationConverter certificationConverter;

    @Mock private StreamObserver<GetProductResponse> responseObserver;

    @Test
    @DisplayName("擁有權判斷讀 request.store_id（重開店 surrogate），忽略 GrpcAuthContext 的原始 JWT accountId")
    void getProduct_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 9999L;       // 請求進入時的原始 JWT accountId（重開店前的舊值）
        long resolvedSurrogateId = 5001L;     // gateway 已解析的現任 store surrogate id

        Context ctxWithStaleJwtId = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctxWithStaleJwtId.attach();

        ProductGrpcService service = new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);

        ProductDetailDTO result = ProductDetailDTO.builder().id("123").storeId(resolvedSurrogateId).build();
        when(productService.getProduct(eq("123"), eq(resolvedSurrogateId))).thenReturn(result);
        when(protoConverter.toProductDetailProto(result)).thenReturn(ProductDetail.newBuilder().build());

        GetProductRequest request = GetProductRequest.newBuilder()
                .setProductId("123")
                .setStoreId(resolvedSurrogateId)
                .build();

        try {
            service.getProduct(request, responseObserver);
        } finally {
            ctxWithStaleJwtId.detach(previous);
        }

        // 若回歸讀 GrpcAuthContext（9999），這個 verify 會失敗
        verify(productService).getProduct("123", resolvedSurrogateId);

        ArgumentCaptor<GetProductResponse> captor = ArgumentCaptor.forClass(GetProductResponse.class);
        verify(responseObserver).onNext(captor.capture());
        assertEquals(0, captor.getValue().getCode());
        assertTrue(captor.getValue().getData().getSelfOwned());
    }
}
