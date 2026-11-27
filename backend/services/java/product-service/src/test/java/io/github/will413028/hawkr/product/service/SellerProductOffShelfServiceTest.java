package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

/**
 * 賣家刪除批次下架核心邏輯測試。
 *
 * 對應 bug：賣家帳號刪除後商品仍停在 ACTIVE，導致公開面向洩漏已刪除賣家的商品。
 * 修法為消費 seller.deleted 事件把該賣家 ACTIVE 商品下架（ACTIVE→INACTIVE），
 * 連帶維護 hashtag 計數與快取失效，並具天然冪等。
 */
@ExtendWith(MockitoExtension.class)
class SellerProductOffShelfServiceTest {

    @Mock ProductRepository productRepository;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService productRevalidationService;

    SellerProductOffShelfService service;

    private static final String ACTIVE = ProductStatus.ACTIVE.value();
    private static final String INACTIVE = ProductStatus.INACTIVE.value();
    private static final long STORE_ID = 7412345678901234567L;

    @BeforeEach
    void setUp() {
        service = new SellerProductOffShelfService(productRepository, cacheService,
                productRevalidationService);
    }

    @Test
    @DisplayName("有 ACTIVE 商品：批次下架為 INACTIVE，每筆扣 hashtag、清詳情快取，並清賣家+公開列表快取")
    void offShelfsActiveProductsWithSideEffects() {
        List<Long> activeIds = List.of(1001L, 1002L, 1003L);
        when(productRepository.findProductIdsBySellerAndStatus(STORE_ID, ACTIVE)).thenReturn(activeIds);

        int count = service.offShelfAllByMerchant(STORE_ID);

        assertEquals(3, count);

        verify(productRepository).batchUpdateStatusByIds(activeIds, INACTIVE, STORE_ID);

        for (Long id : activeIds) {
            verify(cacheService).evictProductDetail(String.valueOf(id));
        }

        verify(cacheService).evictProductListForWrite(STORE_ID);

        verify(productRevalidationService).revalidateProductsWithSellers(activeIds);
        verify(productRevalidationService, never()).revalidateProducts(anyList());

        verify(productRepository).stampSellerDeletedByMerchant(STORE_ID);
    }

    @Test
    @DisplayName("無 ACTIVE 商品：不下架/不動 hashtag/不清商品詳情快取，回 0；但仍無條件蓋章 seller_deleted_at 且 bust 列表快取（含只剩 INACTIVE/DRAFT 的賣家）")
    void stampsEvenWhenNoActiveProducts() {
        when(productRepository.findProductIdsBySellerAndStatus(STORE_ID, ACTIVE))
                .thenReturn(List.of());

        int count = service.offShelfAllByMerchant(STORE_ID);

        assertEquals(0, count);
        verify(productRepository).stampSellerDeletedByMerchant(STORE_ID);
        verify(cacheService).evictProductListForWrite(STORE_ID);
        verify(productRepository, never()).batchUpdateStatusByIds(anyList(), anyString(), anyLong());
        verify(cacheService, never()).evictProductDetail(anyString());
    }
}
