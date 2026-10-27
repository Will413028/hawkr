package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductAccessDeniedException;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * ② re-key：擁有權判定改讀 store_id（store 代理鍵，D7 Stage 2 更名），created_by 降級 audit。
 * 每個 site 正反成對，確保不是仍在讀 created_by。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceOwnerRekeyTest {

    @Mock ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductEntityConverter productEntityConverter;
    @Mock ProductCacheService cacheService;
    @Mock HashtagService hashtagService;
    @Mock SellerGrpcClient sellerGrpcClient;
    @Mock SellerInfoAssembler sellerInfoAssembler;
    @Mock ProductRevalidationService productRevalidationService;
    // #2820：softDeleteProduct 會連帶撤回待審認證申請 —— 不 mock 會 NPE
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductService productService;

    private static final long OWNER_SURROGATE = 7412345678901234567L;
    private static final long OLD_LOGIN_ID = 2619961976L;
    private static final long PID = 1001L;

    private ProductEntity product(String status) {
        ProductEntity e = new ProductEntity();
        e.setId(PID);
        e.setStatus(status);
        e.setStoreId(OWNER_SURROGATE);
        e.setCreatedBy(OLD_LOGIN_ID); // audit 值刻意 ≠ owner
        return e;
    }

    // ==== getProduct（ProductService:584 isOwner） ====

    @Test
    @DisplayName("getProduct：INACTIVE + 請求者 == store_id（≠ created_by）→ 視為 owner，可見")
    void getProductOwnerByStoreId() {
        ProductEntity e = product(ProductStatus.INACTIVE.value());
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(e, List.of(), false))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());

        assertNotNull(productService.getProduct(String.valueOf(PID), OWNER_SURROGATE));
    }

    @Test
    @DisplayName("getProduct：INACTIVE + 請求者 == created_by（≠ store_id）→ 非 owner，拒絕")
    void getProductCreatedByIsNotOwnership() {
        when(productRepository.findById(PID))
                .thenReturn(Optional.of(product(ProductStatus.INACTIVE.value())));

        assertThrows(ProductAccessDeniedException.class,
                () -> productService.getProduct(String.valueOf(PID), OLD_LOGIN_ID));
    }

    // ==== softDeleteProduct（ProductService:2242 擁有者驗證） ====

    @Test
    @DisplayName("softDelete：請求者 == store_id → 通過擁有者驗證並軟刪")
    void softDeleteOwnerByStoreId() {
        when(productRepository.findById(PID))
                .thenReturn(Optional.of(product(ProductStatus.DRAFT.value())));

        productService.softDeleteProduct(String.valueOf(PID), OWNER_SURROGATE);

        verify(productRepository).deleteById(PID);
    }

    @Test
    @DisplayName("softDelete：請求者 == created_by（≠ store_id）→ 拒絕")
    void softDeleteCreatedByIsNotOwnership() {
        when(productRepository.findById(PID))
                .thenReturn(Optional.of(product(ProductStatus.DRAFT.value())));

        assertThrows(ProductAccessDeniedException.class,
                () -> productService.softDeleteProduct(String.valueOf(PID), OLD_LOGIN_ID));
        verify(productRepository, never()).deleteById(anyLong());
    }
}
