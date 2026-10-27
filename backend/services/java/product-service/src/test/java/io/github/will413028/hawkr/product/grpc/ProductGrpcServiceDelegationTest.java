package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.DelegationCapability;
import io.github.will413028.hawkr.common.grpc.DelegationContext;
import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.dto.response.ProductSnapshotDTO;
import io.github.will413028.hawkr.product.exception.ProductNotFoundException;
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
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 代行商品讀寫（56 §6.2.4 / §7.1.2 Q4 / §7.2.8）。
 *
 * <p>兩件漏掉不會有任何錯誤訊號的事：代行讀取沒做 Q4 歸屬斷言（跨店讀取成功且不落稽核）、
 * 批次用單一交易或全批假成功（一項版本衝突把其他項一起吃掉）。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
// #2744：DelegationContextValidator 自身的拒絕（S1a–S5）一律改發 9289
// DELEGATION_INTERNAL_CONTRACT_MISMATCH，不再分散在 9260／9262。
// DelegationContext 由 Gateway 組裝，操作者碰不到 —— 它們全是「我方組錯」，
// 而 9260 說「你填錯了」、9262 說「你沒這個權限」，兩者都把人送去做一件
// 不會改變任何事的工作；9262 還會讓稽核列指控一位沒做錯事的管理員。
class ProductGrpcServiceDelegationTest {

    private static final long STORE_ID = 2685914682L;

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

    private ProductGrpcService buildService() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    private static DelegationContext.Builder delegation(String capability) {
        return DelegationContext.newBuilder()
                .setSessionId("2701445588392001")
                .setOperatorAdminId("42")
                .setCapability(capability);
    }

    @Test
    @DisplayName("代行讀取單筆走 requireOwnership 分支（Q4），賣家本人路徑不走")
    void getProduct_delegated_asksForOwnershipCheck() {
        ProductDetailDTO result = ProductDetailDTO.builder().id("123").storeId(STORE_ID).version(3L).build();
        when(productService.getProduct(eq("123"), eq(STORE_ID), eq(true))).thenReturn(result);
        when(protoConverter.toProductDetailProto(result)).thenReturn(ProductDetail.newBuilder().build());

        @SuppressWarnings("unchecked")
        StreamObserver<GetProductResponse> observer = mock(StreamObserver.class);
        buildService().getProduct(GetProductRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setDelegation(delegation(DelegationCapability.PRODUCTS_READ).build())
                .build(), observer);

        verify(productService).getProduct("123", STORE_ID, true);
    }

    @Test
    @DisplayName("Q4 不符 ⇒ 9275（跨店讀取必須被擋，且不是靜默 200）")
    void getProduct_delegated_notOwned_returns9275() {
        when(productService.getProduct(eq("123"), eq(STORE_ID), eq(true)))
                .thenThrow(new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_NOT_OWNED));

        @SuppressWarnings("unchecked")
        StreamObserver<GetProductResponse> observer = mock(StreamObserver.class);
        buildService().getProduct(GetProductRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setDelegation(delegation(DelegationCapability.PRODUCTS_READ).build())
                .build(), observer);

        ArgumentCaptor<GetProductResponse> captor = ArgumentCaptor.forClass(GetProductResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9275, captor.getValue().getCode());
    }

    @Test
    @DisplayName("代行列表 owner_store_id 為零 ⇒ 9260（否則會落進公開列表分支拿到全站商品）")
    void getProductList_delegated_withoutOwnerStoreId_returns9289() {
        @SuppressWarnings("unchecked")
        StreamObserver<GetProductListResponse> observer = mock(StreamObserver.class);
        buildService().getProductList(GetProductListRequest.newBuilder()
                .setDelegation(delegation(DelegationCapability.PRODUCTS_READ).build())
                .build(), observer);

        ArgumentCaptor<GetProductListResponse> captor = ArgumentCaptor.forClass(GetProductListResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9289, captor.getValue().getCode());
    }

    @Test
    @DisplayName("代行讀取宣告 products.read；帶 products.publish 會被 S3 擋（批次第一跳的形狀）")
    void batchSnapshot_declaresProductsRead() {
        ProductDetailDTO detailDTO = ProductDetailDTO.builder().id("1").name("A").build();
        when(productService.getProductSnapshotsForDelegation(eq(STORE_ID), any(), eq(true)))
                .thenReturn(List.of(
                        ProductSnapshotDTO.builder().productId("1").found(true).version(4L)
                                .status("4004").name("A").detail(detailDTO).build(),
                        ProductSnapshotDTO.builder().productId("2").found(false).build()));
        when(protoConverter.toProductDetailProto(detailDTO))
                .thenReturn(ProductDetail.newBuilder().setId("1").setName("A").setOrigin("屏東").build());

        @SuppressWarnings("unchecked")
        StreamObserver<BatchGetSellerProductsForDelegationResponse> observer = mock(StreamObserver.class);
        // audit_log_id 刻意留空：這一跳在 RecordAuditIntentBatch 之前，S1b 對 .read 不驗
        buildService().batchGetSellerProductsForDelegation(
                BatchGetSellerProductsForDelegationRequest.newBuilder()
                        .setStoreId(STORE_ID)
                        .addProductIds("1").addProductIds("2")
                        .setDelegation(delegation(DelegationCapability.PRODUCTS_READ).build())
                        .build(), observer);

        ArgumentCaptor<BatchGetSellerProductsForDelegationResponse> captor =
                ArgumentCaptor.forClass(BatchGetSellerProductsForDelegationResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(0, captor.getValue().getCode());
        assertEquals(2, captor.getValue().getResultsCount());
        assertTrue(captor.getValue().getResults(0).getFound());
        assertEquals("4", captor.getValue().getResults(0).getVersion());
        assertFalse(captor.getValue().getResults(1).getFound());

        // withDetail 必須是 true，且 detail 必須真的被塞進回應。
        //
        // 這一跳是稽核 before_snapshot 的唯一素材來源：傳 false（或忘記 setDetail）時
        // Gateway 會退回「只有 status」的舊形狀，於是批次下架的 before_snapshot 只有一個鍵，
        // 而同一個操作的單筆版本存的是完整 12 欄（56 §7.2.8 讀法 8）。
        // 那個退化**不會有任何錯誤訊號**：RPC 回 code 0、逐項結果齊全、批次照常執行成功。
        verify(productService).getProductSnapshotsForDelegation(eq(STORE_ID), any(), eq(true));
        assertTrue(captor.getValue().getResults(0).hasDetail(),
                "found=true 的項必須帶 detail —— 它是 before_snapshot 的唯一素材來源");
        assertEquals("屏東", captor.getValue().getResults(0).getDetail().getOrigin());
        assertFalse(captor.getValue().getResults(1).hasDetail(),
                "found=false 的項不得帶 detail");
    }

    @Test
    @DisplayName("批次下架：逐項獨立成敗，頂層 code 恆為 0，audit_log_id 原樣回音")
    void batchUpdate_partialFailure_keepsTopLevelCodeZero() {
        when(productService.updateSellerProductStatus(eq("1"), eq(STORE_ID), anyString(), anyString()))
                .thenReturn(ProductDetailDTO.builder().id("1").version(5L).build());
        when(productService.updateSellerProductStatus(eq("2"), eq(STORE_ID), anyString(), anyString()))
                .thenThrow(new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT));
        when(productService.updateSellerProductStatus(eq("3"), eq(STORE_ID), anyString(), anyString()))
                .thenThrow(new ProductNotFoundException("3"));
        when(productService.getProductSnapshotsForDelegation(anyLong(), any()))
                .thenReturn(List.of(ProductSnapshotDTO.builder().productId("2").found(true).version(6L).build()));

        BatchUpdateSellerProductStatusRequest request = BatchUpdateSellerProductStatusRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .addItems(item("1", "4", "a1"))
                .addItems(item("2", "5", "a2"))
                .addItems(item("3", "9", "a3"))
                .setDelegation(delegation(DelegationCapability.PRODUCTS_PUBLISH).build())
                .build();

        @SuppressWarnings("unchecked")
        StreamObserver<BatchUpdateSellerProductStatusResponse> observer = mock(StreamObserver.class);
        buildService().batchUpdateSellerProductStatus(request, observer);

        ArgumentCaptor<BatchUpdateSellerProductStatusResponse> captor =
                ArgumentCaptor.forClass(BatchUpdateSellerProductStatusResponse.class);
        verify(observer).onNext(captor.capture());
        BatchUpdateSellerProductStatusResponse response = captor.getValue();

        assertEquals(0, response.getCode(), "部分失敗仍須回頂層 0，否則前端整包丟棄 results");
        assertEquals("3", response.getRequested());
        assertEquals("1", response.getSucceeded());
        assertEquals("2", response.getFailed());

        assertTrue(response.getResults(0).getOk());
        assertEquals("5", response.getResults(0).getVersion());
        assertEquals("a1", response.getResults(0).getAuditLogId());

        assertFalse(response.getResults(1).getOk());
        assertEquals(9266, response.getResults(1).getCode());
        assertEquals("6", response.getResults(1).getVersion(), "失敗項回現行版本供前端直接重試");
        assertEquals("a2", response.getResults(1).getAuditLogId());

        assertFalse(response.getResults(2).getOk());
        assertEquals(404, response.getResults(2).getCode());
        assertEquals("a3", response.getResults(2).getAuditLogId());
    }

    @Test
    @DisplayName("批次任一項缺 audit_log_id ⇒ 整批 9260（S1b 在批次落在逐項上）")
    void batchUpdate_itemWithoutAuditLogId_rejectsWholeBatch() {
        @SuppressWarnings("unchecked")
        StreamObserver<BatchUpdateSellerProductStatusResponse> observer = mock(StreamObserver.class);
        buildService().batchUpdateSellerProductStatus(BatchUpdateSellerProductStatusRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .addItems(item("1", "4", "a1"))
                .addItems(item("2", "5", ""))
                .setDelegation(delegation(DelegationCapability.PRODUCTS_PUBLISH).build())
                .build(), observer);

        ArgumentCaptor<BatchUpdateSellerProductStatusResponse> captor =
                ArgumentCaptor.forClass(BatchUpdateSellerProductStatusResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9289, captor.getValue().getCode());
        assertEquals(0, captor.getValue().getResultsCount());
    }

    @Test
    @DisplayName("批次 product_id 重複 ⇒ 整批拒，一項都不執行")
    void batchUpdate_duplicateProductId_rejected() {
        @SuppressWarnings("unchecked")
        StreamObserver<BatchUpdateSellerProductStatusResponse> observer = mock(StreamObserver.class);
        buildService().batchUpdateSellerProductStatus(BatchUpdateSellerProductStatusRequest.newBuilder()
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .addItems(item("1", "4", "a1"))
                .addItems(item("1", "4", "a2"))
                .setDelegation(delegation(DelegationCapability.PRODUCTS_PUBLISH).build())
                .build(), observer);

        ArgumentCaptor<BatchUpdateSellerProductStatusResponse> captor =
                ArgumentCaptor.forClass(BatchUpdateSellerProductStatusResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(GrpcResponseCode.INVALID_PARAM.code(), captor.getValue().getCode());
        verify(productService, org.mockito.Mockito.never())
                .updateSellerProductStatus(anyString(), anyLong(), anyString(), anyString());
    }

    @Test
    @DisplayName("寫入路徑 capability 不符 ⇒ 9262，且不進 Service")
    void updateSellerProductStatus_wrongCapability_denied() {
        @SuppressWarnings("unchecked")
        StreamObserver<UpdateProductResponse> observer = mock(StreamObserver.class);
        buildService().updateSellerProductStatus(UpdateSellerProductStatusRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .setDelegation(delegation(DelegationCapability.PRODUCTS_WRITE)
                        .setAuditLogId("2701445588392099").build())
                .build(), observer);

        ArgumentCaptor<UpdateProductResponse> captor = ArgumentCaptor.forClass(UpdateProductResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9289, captor.getValue().getCode());
        verify(productService, org.mockito.Mockito.never())
                .updateSellerProductStatus(anyString(), anyLong(), anyString(), anyString());
    }

    @Test
    @DisplayName("寫入路徑缺 audit_log_id ⇒ 9260（S1b 對 .publish 必驗）")
    void updateSellerProductStatus_withoutAuditLogId_denied() {
        @SuppressWarnings("unchecked")
        StreamObserver<UpdateProductResponse> observer = mock(StreamObserver.class);
        buildService().updateSellerProductStatus(UpdateSellerProductStatusRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .setDelegation(delegation(DelegationCapability.PRODUCTS_PUBLISH).build())
                .build(), observer);

        ArgumentCaptor<UpdateProductResponse> captor = ArgumentCaptor.forClass(UpdateProductResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9289, captor.getValue().getCode());
    }

    @Test
    @DisplayName("樂觀鎖衝突 ⇒ 9266（MyBatis-Plus 回 0 列不拋例外，不判就是 200 但沒寫）")
    void updateSellerProductStatus_versionConflict_returns9266() {
        when(productService.updateSellerProductStatus(anyString(), anyLong(), anyString(), anyString()))
                .thenThrow(new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT));

        @SuppressWarnings("unchecked")
        StreamObserver<UpdateProductResponse> observer = mock(StreamObserver.class);
        buildService().updateSellerProductStatus(UpdateSellerProductStatusRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setStatus("4003")
                .setExpectedVersion("4")
                .setDelegation(delegation(DelegationCapability.PRODUCTS_PUBLISH)
                        .setAuditLogId("2701445588392099").build())
                .build(), observer);

        ArgumentCaptor<UpdateProductResponse> captor = ArgumentCaptor.forClass(UpdateProductResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9266, captor.getValue().getCode());
    }

    @Test
    @DisplayName("request 與代行語境的冪等鍵不同 ⇒ 9260（S5）")
    void updateProduct_idempotencyKeyMismatch_denied() {
        @SuppressWarnings("unchecked")
        StreamObserver<UpdateProductResponse> observer = mock(StreamObserver.class);
        buildService().updateProduct(UpdateProductRequest.newBuilder()
                .setProductId("123")
                .setStoreId(STORE_ID)
                .setIdempotencyKey("ik_a")
                .setDelegation(delegation(DelegationCapability.PRODUCTS_WRITE)
                        .setAuditLogId("2701445588392099")
                        .setIdempotencyKey("ik_b").build())
                .build(), observer);

        ArgumentCaptor<UpdateProductResponse> captor = ArgumentCaptor.forClass(UpdateProductResponse.class);
        verify(observer).onNext(captor.capture());
        assertEquals(9289, captor.getValue().getCode());
    }

    private static BatchProductStatusItem item(String productId, String expectedVersion, String auditLogId) {
        return BatchProductStatusItem.newBuilder()
                .setProductId(productId)
                .setExpectedVersion(expectedVersion)
                .setAuditLogId(auditLogId)
                .build();
    }
}
