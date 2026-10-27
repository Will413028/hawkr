package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.grpc.DelegationDeniedException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.dto.response.ProductSnapshotDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
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

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.when;

/**
 * 代行在 ProductService 這一層的兩道守門（56 §7.1.2 Q4、§6.6.3 樂觀鎖）。
 *
 * <p>兩者共同點：漏掉都不會有錯誤訊號——Q4 漏掉是「跨店讀取成功且不落稽核」，
 * 樂觀鎖 rows=0 漏判是「回 200 但資料沒寫」。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceDelegationGuardTest {

    @Mock ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductEntityConverter productEntityConverter;
    @Mock ProductCacheService cacheService;
    @Mock HashtagService hashtagService;
    @Mock SellerGrpcClient sellerGrpcClient;
    @Mock SellerInfoAssembler sellerInfoAssembler;
    @Mock ProductRevalidationService productRevalidationService;
    @InjectMocks ProductService productService;

    private static final long OWNER_STORE = 2685914682L;
    private static final long OTHER_STORE = 2632049831L;
    private static final long PID = 1001L;

    private ProductEntity activeProduct() {
        ProductEntity e = new ProductEntity();
        e.setId(PID);
        e.setStatus(ProductStatus.ACTIVE.value());
        e.setStoreId(OWNER_STORE);
        e.setName("有機小松菜");
        e.setVersion(4L);
        return e;
    }

    @Test
    @DisplayName("Q4：ACTIVE 商品跨店代行讀取 ⇒ 9275（既有述詞對 ACTIVE 一律放行，靠這一步擋）")
    void getProduct_delegatedCrossStore_denied() {
        // fixture 刻意完整：拿掉歸屬斷言時這條路徑會一路走到成功回應（那正是漏洞的形狀），
        // 而不是在半途撞上未 stub 的相依而拋 NPE——後者會讓「轉紅」變成一個假訊號
        ProductEntity e = activeProduct();
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(e, List.of(), false))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());

        DelegationDeniedException ex = assertThrows(DelegationDeniedException.class,
                () -> productService.getProduct(String.valueOf(PID), OTHER_STORE, true));
        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_NOT_OWNED.code(), ex.code());
    }

    @Test
    @DisplayName("鑑別力對照：同一筆 ACTIVE 商品在不做歸屬斷言時跨店讀得到")
    void getProduct_withoutOwnershipCheck_crossStoreSucceeds() {
        ProductEntity e = activeProduct();
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(e, List.of(), false))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());

        assertNotNull(productService.getProduct(String.valueOf(PID), OTHER_STORE));
    }

    @Test
    @DisplayName("代行本店讀取照常通過")
    void getProduct_delegatedOwnStore_allowed() {
        ProductEntity e = activeProduct();
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(e, List.of(), false))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());

        assertNotNull(productService.getProduct(String.valueOf(PID), OWNER_STORE, true));
    }

    @Test
    @DisplayName("expectedVersion 與現況不符 ⇒ 9266，且不進到狀態轉換")
    void updateSellerProductStatus_staleExpectedVersion_denied() {
        when(productRepository.findByIdAndStoreId(PID, OWNER_STORE)).thenReturn(Optional.of(activeProduct()));

        DelegationDeniedException ex = assertThrows(DelegationDeniedException.class,
                () -> productService.updateSellerProductStatus(
                        String.valueOf(PID), OWNER_STORE, ProductStatus.INACTIVE.value(), "3"));
        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT.code(), ex.code());
    }

    @Test
    @DisplayName("CAS 回 0 列 ⇒ 9266（MyBatis-Plus 版本不符只回 0，不拋例外）")
    void updateSellerProductStatus_casMiss_denied() {
        when(productRepository.findByIdAndStoreId(PID, OWNER_STORE)).thenReturn(Optional.of(activeProduct()));
        when(productRepository.updateById(org.mockito.ArgumentMatchers.any())).thenReturn(0);

        DelegationDeniedException ex = assertThrows(DelegationDeniedException.class,
                () -> productService.updateSellerProductStatus(
                        String.valueOf(PID), OWNER_STORE, ProductStatus.INACTIVE.value(), "4"));
        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT.code(), ex.code());
    }

    @Test
    @DisplayName("批次快照：查不到與不屬本店都回 found=false，不區分（避免探測）")
    void batchSnapshots_notFoundAndNotOwnedAreIndistinguishable() {
        when(productRepository.findByIdAndStoreId(PID, OWNER_STORE)).thenReturn(Optional.of(activeProduct()));
        when(productRepository.findByIdAndStoreId(2002L, OWNER_STORE)).thenReturn(Optional.empty());

        List<ProductSnapshotDTO> snapshots = productService.getProductSnapshotsForDelegation(
                OWNER_STORE, List.of(String.valueOf(PID), "2002", "not-a-number"));

        assertEquals(3, snapshots.size(), "與 product_ids 同序同長度");
        assertTrue(snapshots.get(0).isFound());
        assertEquals(4L, snapshots.get(0).getVersion());
        assertEquals("有機小松菜", snapshots.get(0).getName());
        assertFalse(snapshots.get(1).isFound());
        assertFalse(snapshots.get(2).isFound());
    }
}
