package io.github.will413028.hawkr.product.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.product.converter.ProductProtoConverter;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
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

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.anyCollection;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * ADM 認證審核補商品名時，**必須**用不濾可見性的查詢（#2818）。
 *
 * <p>為什麼需要這一支而不是只有 converter 的測試：converter 測的是「拿到 brief 之後
 * 映射對不對」，把 {@code loadProductBriefsForAdmin} 改回 {@code findBriefByIds}
 * 那些測試**照樣全綠**——而那正是這個 bug 的本體。這裡守的是「呼叫了哪一支查詢」。
 *
 * <p>認證申請結構上會指向已軟刪商品（§15.3 migration 不濾 deleted_at、賣家送件後可刪商品、
 * {@code updateCertificationProjection} 刻意不濾以便 ADM 對已下架商品撤銷標章），
 * 用帶 {@code deleted_at IS NULL} 的 {@code findBriefByIds} 會讓六支 ADM 端點回空商品名。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductGrpcServiceAdminCertificationProductLookupTest {

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

    @Mock private StreamObserver<ProductCertificationApplicationAdminResponse> responseObserver;
    @Mock private StreamObserver<ListProductCertificationApplicationsForAdminResponse> listObserver;

    private static final long PRODUCT_ID = 843856384063471755L;
    private static final long APPLICATION_ID = 843856384063471761L;

    private ProductGrpcService service() {
        return new ProductGrpcService(
                productService, protoConverter, cacheService, categoryCacheService,
                redisTemplate, stockService, sellerGrpcClient, shippingGrpcClient,
                skuRepository, govCertificationService, hashtagService, popularHashtagService,
                productImportService, productPublishValidator, productRepository,
                certificationApplicationService, certificationConverter);
    }

    private static ProductCertificationApplicationEntity application() {
        ProductCertificationApplicationEntity e = new ProductCertificationApplicationEntity();
        e.setId(APPLICATION_ID);
        e.setProductId(PRODUCT_ID);
        e.setStoreId(2685914682L);
        e.setCertificationType(3703);
        e.setCertificateNo("010105");
        e.setStatus("SUBMITTED");
        e.setSource("LEGACY_MIGRATION");
        e.setRequiresManualReview(false);
        e.setGovVerifyState("PENDING");
        e.setVersion(0L);
        e.setSubmittedAt(LocalDateTime.of(2026, 8, 27, 4, 12, 5));
        return e;
    }

    @Test
    @DisplayName("ADM 詳情補商品名走「含已軟刪」的查詢，且不得改用帶 deleted_at IS NULL 的 findBriefByIds")
    void adminDetailUsesIncludingDeletedLookup() {
        Context previous = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 2701338845L)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.ADMIN.value())
                .attach();
        try {
            ProductCertificationApplicationEntity app = application();
            when(certificationApplicationService.getForAdmin(APPLICATION_ID)).thenReturn(app);
            when(certificationApplicationService.findCurrentApproved(any())).thenReturn(Optional.empty());
            ProductPublicationBriefProjection brief = new ProductPublicationBriefProjection(
                    PRODUCT_ID, "有機小松菜", "4004",
                    LocalDateTime.of(2026, 8, 1, 0, 0), LocalDateTime.of(2026, 8, 20, 0, 0), null, true);
            when(productRepository.findPublicationBriefByIdsIncludingDeleted(anyCollection()))
                    .thenReturn(List.of(brief));
            // converter 要回真的 proto——回 mock 的 null 會讓 RPC 在補完商品名之後才炸，
            // 那樣 verify 仍會過，但測到的是一條崩掉的路徑。
            when(certificationConverter.toAdminProto(any(), any(), any()))
                    .thenReturn(ProductCertificationApplication.newBuilder()
                            .setId(String.valueOf(APPLICATION_ID))
                            .setProductId(String.valueOf(PRODUCT_ID))
                            .setProductName("有機小松菜")
                            .setProductPublicationState("4014")
                            .build());

            service().getProductCertificationApplicationForAdmin(
                    GetProductCertificationApplicationForAdminRequest.newBuilder()
                            .setApplicationId(String.valueOf(APPLICATION_ID))
                            .setAccountId(2701338845L)
                            .build(),
                    responseObserver);

            verify(productRepository).findPublicationBriefByIdsIncludingDeleted(anyCollection());
            verify(productRepository, never()).findBriefByIds(anyCollection());

            // 控制組：RPC 必須真的成功走完，否則上面的 verify 可能只是「炸掉之前剛好呼叫過」。
            ArgumentCaptor<ProductCertificationApplicationAdminResponse> captor =
                    ArgumentCaptor.forClass(ProductCertificationApplicationAdminResponse.class);
            verify(responseObserver).onNext(captor.capture());
            verify(responseObserver).onCompleted();
            assertEquals(0, captor.getValue().getCode(), "RPC 必須成功；非 0 代表走進了錯誤分支");
            // 上游確實把 brief 交給了 converter（而不是傳 null 進去）。
            verify(certificationConverter).toAdminProto(any(), eq(brief), any());
        } finally {
            Context.current().detach(previous);
        }
    }

    @Test
    @DisplayName("ADM 佇列列表也走「含已軟刪」的查詢——它是 #2818 最顯眼的 surface，且是另一個獨立 call site")
    void adminQueueListAlsoUsesIncludingDeletedLookup() {
        Context previous = Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, 2701338845L)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, AccountType.ADMIN.value())
                .attach();
        try {
            ProductCertificationApplicationEntity app = application();
            when(certificationApplicationService.listForAdmin(
                    any(), any(), any(), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(new io.github.will413028.hawkr.product.service.ProductCertificationApplicationService
                            .Paged<>(List.of(app), 1L));
            when(productRepository.findPublicationBriefByIdsIncludingDeleted(anyCollection()))
                    .thenReturn(List.of(new ProductPublicationBriefProjection(
                            PRODUCT_ID, "有機小松菜", "4004",
                            LocalDateTime.of(2026, 8, 1, 0, 0), LocalDateTime.of(2026, 8, 20, 0, 0), null, true)));
            when(certificationConverter.toAdminProto(any(), any(), any()))
                    .thenReturn(ProductCertificationApplication.newBuilder()
                            .setId(String.valueOf(APPLICATION_ID))
                            .setProductName("有機小松菜")
                            .setProductPublicationState("4014")
                            .build());

            service().listProductCertificationApplicationsForAdmin(
                    ListProductCertificationApplicationsForAdminRequest.newBuilder()
                            .setAccountId(2701338845L).setPage("1").setPageSize("20").build(),
                    listObserver);

            verify(productRepository).findPublicationBriefByIdsIncludingDeleted(anyCollection());
            verify(productRepository, never()).findBriefByIds(anyCollection());

            ArgumentCaptor<ListProductCertificationApplicationsForAdminResponse> captor =
                    ArgumentCaptor.forClass(ListProductCertificationApplicationsForAdminResponse.class);
            verify(listObserver).onNext(captor.capture());
            verify(listObserver).onCompleted();
            assertEquals(0, captor.getValue().getCode(), "RPC 必須成功；非 0 代表走進了錯誤分支");
        } finally {
            Context.current().detach(previous);
        }
    }
}
