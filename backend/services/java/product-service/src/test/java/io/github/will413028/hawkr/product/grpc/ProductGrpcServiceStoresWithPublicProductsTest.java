package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
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

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.Mockito.when;

/**
 * {@code ListStoreIDsWithPublicProductsInternal}：回「至少有一件公開上架商品」的 store_id 全集。
 *
 * <p>消費端 seller-service 用它把 0 件在架商品的店排除在推薦賣家 / marketplace rail 之外，
 * 並且在 RPC 失敗時 fail-open（不過濾）。因此**空集合與失敗必須可分辨**：
 * 空集合＝「全平台沒有任何公開商品」（正常會把所有店濾掉），失敗＝非零 code（呼叫端不過濾）。
 * 把例外降級成 code=0 + 空 store_ids 會讓推薦欄整個消失，而且沒有任何訊號。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductGrpcServiceStoresWithPublicProductsTest {

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

    @Mock private StreamObserver<ListStoreIDsWithPublicProductsInternalResponse> observer;

    private ProductGrpcService service() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    private ListStoreIDsWithPublicProductsInternalResponse capture() {
        ArgumentCaptor<ListStoreIDsWithPublicProductsInternalResponse> captor =
                ArgumentCaptor.forClass(ListStoreIDsWithPublicProductsInternalResponse.class);
        org.mockito.Mockito.verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("回傳的 store_id 以數字字串輸出，順序與來源一致")
    void returnsStoreIdsAsNumericStrings() {
        when(productService.listStoreIDsWithPublicProducts())
                .thenReturn(List.of(2685914682L, 2632049831L));

        service().listStoreIDsWithPublicProductsInternal(
                ListStoreIDsWithPublicProductsInternalRequest.getDefaultInstance(), observer);

        var resp = capture();
        assertEquals(0, resp.getCode());
        assertEquals(List.of("2685914682", "2632049831"), resp.getStoreIdsList());
    }

    @Test
    @DisplayName("平台上沒有任何公開商品時回空集合且 code=0（與失敗可分辨）")
    void emptySetIsASuccessfulAnswer() {
        when(productService.listStoreIDsWithPublicProducts()).thenReturn(List.of());

        service().listStoreIDsWithPublicProductsInternal(
                ListStoreIDsWithPublicProductsInternalRequest.getDefaultInstance(), observer);

        var resp = capture();
        assertEquals(0, resp.getCode());
        assertEquals(0, resp.getStoreIdsCount());
    }

    @Test
    @DisplayName("查詢失敗必須回非零 code，不得降級成 code=0 的空集合")
    void failureMustNotDegradeToEmptySet() {
        when(productService.listStoreIDsWithPublicProducts())
                .thenThrow(new RuntimeException("db down"));

        service().listStoreIDsWithPublicProductsInternal(
                ListStoreIDsWithPublicProductsInternalRequest.getDefaultInstance(), observer);

        var resp = capture();
        assertNotEquals(0, resp.getCode(),
                "失敗與空集合同形 ⇒ 呼叫端會把每一家店都當成沒有商品而全部濾掉，推薦欄直接消失");
    }
}
