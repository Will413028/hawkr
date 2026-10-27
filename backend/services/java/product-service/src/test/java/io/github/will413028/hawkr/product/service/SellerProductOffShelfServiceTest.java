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

    private static final String ACTIVE = ProductStatus.ACTIVE.value();     // 4004
    private static final String INACTIVE = ProductStatus.INACTIVE.value(); // 4003
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

        // 1. 批次 ACTIVE → INACTIVE，updated_by = 該賣家 storeId
        verify(productRepository).batchUpdateStatusByIds(activeIds, INACTIVE, STORE_ID);

        // 2. 每筆 hashtag -1 + 詳情快取失效
        for (Long id : activeIds) {
            verify(cacheService).evictProductDetail(String.valueOf(id));
        }

        // 3. 列表快取：寫入路徑統一走 evictProductListForWrite（內部清賣家私有 bucket + 公開 bucket 0）
        verify(cacheService).evictProductListForWrite(STORE_ID);

        // 4. frontend cache bust：賣家公開商品集合整批變動 → 共發 seller entity tag（§9.3）
        verify(productRevalidationService).revalidateProductsWithSellers(activeIds);
        verify(productRevalidationService, never()).revalidateProducts(anyList());

        // 蓋章：全狀態商品標記 seller_deleted_at
        verify(productRepository).stampSellerDeletedByMerchant(STORE_ID);
    }

    @Test
    @DisplayName("無 ACTIVE 商品：不下架/不動 hashtag/不清商品詳情快取，回 0；但仍無條件蓋章 seller_deleted_at 且 bust 列表快取（含只剩 INACTIVE/DRAFT 的賣家）")
    void stampsEvenWhenNoActiveProducts() {
        when(productRepository.findProductIdsBySellerAndStatus(STORE_ID, ACTIVE))
                .thenReturn(List.of());

        int count = service.offShelfAllByMerchant(STORE_ID);

        assertEquals(0, count);
        // 蓋章必須發生（early-return 之前）
        verify(productRepository).stampSellerDeletedByMerchant(STORE_ID);
        // 蓋章改動了 seller_deleted_at，賣家私有列表查詢結果變了 → 仍需 bust 列表快取
        verify(cacheService).evictProductListForWrite(STORE_ID);
        // 其餘下架副作用不發生
        verify(productRepository, never()).batchUpdateStatusByIds(anyList(), anyString(), anyLong());
        verify(cacheService, never()).evictProductDetail(anyString());
    }
}
