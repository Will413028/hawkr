package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.StoreExposureRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class StoreExposureServiceTest {

    private static final long STORE_ID = 123L;

    @Mock StoreExposureRepository storeExposureRepository;
    @Mock ProductRepository productRepository;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService productRevalidationService;
    @InjectMocks StoreExposureService service;

    @Test
    @DisplayName("有改到列：清這家店每個商品的詳情快取、列表快取，並通知前端快取失效")
    void appliedEvictsCaches() {
        List<Long> ids = List.of(11L, 12L);
        when(storeExposureRepository.upsertIfNewer(STORE_ID, true, 7L)).thenReturn(1);
        when(productRepository.findProductIdsByStoreId(STORE_ID)).thenReturn(ids);

        assertTrue(service.apply(STORE_ID, true, 7L));

        verify(cacheService).evictProductDetail("11");
        verify(cacheService).evictProductDetail("12");
        verify(cacheService).evictProductListForWrite(STORE_ID);
        verify(productRevalidationService).revalidateProductsWithSellers(ids);
    }

    @Test
    @DisplayName("同一版本重送（上一次寫入已提交、快取清除失敗）：仍清快取，補上沒做完的清除")
    void redeliveredSameVersionStillEvicts() {
        List<Long> ids = List.of(11L);
        when(storeExposureRepository.upsertIfNewer(STORE_ID, true, 7L)).thenReturn(1, 0);
        when(storeExposureRepository.findVersion(STORE_ID)).thenReturn(7L);
        when(productRepository.findProductIdsByStoreId(STORE_ID))
                .thenThrow(new IllegalStateException("transient"))
                .thenReturn(ids);

        assertThrows(IllegalStateException.class, () -> service.apply(STORE_ID, true, 7L));
        verify(cacheService, never()).evictProductDetail(anyString());

        assertTrue(service.apply(STORE_ID, true, 7L));

        verify(cacheService).evictProductDetail("11");
        verify(cacheService, times(1)).evictProductListForWrite(STORE_ID);
        verify(productRevalidationService).revalidateProductsWithSellers(ids);
    }

    @Test
    @DisplayName("舊版本（沒改到列）：不清任何快取")
    void staleDoesNotEvict() {
        when(storeExposureRepository.upsertIfNewer(STORE_ID, true, 7L)).thenReturn(0);
        when(storeExposureRepository.findVersion(STORE_ID)).thenReturn(9L);

        assertFalse(service.apply(STORE_ID, true, 7L));

        verify(productRepository, never()).findProductIdsByStoreId(anyLong());
        verify(cacheService, never()).evictProductDetail(anyString());
        verify(cacheService, never()).evictProductListForWrite(anyLong());
        verify(productRevalidationService, never()).revalidateProductsWithSellers(anyList());
    }
}
