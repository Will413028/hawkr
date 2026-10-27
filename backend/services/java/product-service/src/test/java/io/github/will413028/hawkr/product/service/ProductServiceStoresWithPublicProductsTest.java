package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code listStoreIDsWithPublicProducts}（02 §5.13c）必須沿用
 * {@code findPublicProductCountsByStore} 這句既有聚合。
 *
 * <p>為什麼要把「呼叫了哪一支查詢」也守住：另寫一句 SQL 也能讓回傳值測試全綠，
 * 但「什麼叫公開上架」（{@code deleted_at IS NULL AND seller_deleted_at IS NULL AND status='4004'}）
 * 就會出現第二份定義，之後兩邊各自漂移 —— 城市聚合與推薦賣家 filter 會對同一家店
 * 給出相反答案，而且不會有任何測試變紅。
 */
@ExtendWith(MockitoExtension.class)
class ProductServiceStoresWithPublicProductsTest {

    @Mock ProductRepository productRepository;
    @InjectMocks ProductService productService;

    private static ProductRepository.StoreProductCountRow row(Long storeId, Long count) {
        var r = new ProductRepository.StoreProductCountRow();
        r.setStoreId(storeId);
        r.setProductCount(count);
        return r;
    }

    @Test
    @DisplayName("沿用 findPublicProductCountsByStore 的結果，只取 store_id")
    void reusesSharedPublicProductAggregate() {
        when(productRepository.findPublicProductCountsByStore()).thenReturn(List.of(
                row(2685914682L, 12L),
                row(2632049831L, 1L)));

        assertEquals(List.of(2685914682L, 2632049831L), productService.listStoreIDsWithPublicProducts());
        verify(productRepository).findPublicProductCountsByStore();
    }

    @Test
    @DisplayName("store_id 為 null 的列直接丟棄，不得混進 filter 集合")
    void dropsNullStoreIds() {
        when(productRepository.findPublicProductCountsByStore()).thenReturn(java.util.Arrays.asList(
                row(null, 3L),
                row(2685914682L, 4L)));

        assertEquals(List.of(2685914682L), productService.listStoreIDsWithPublicProducts());
    }

    @Test
    @DisplayName("平台上沒有任何公開商品時回空集合（不是 null）")
    void emptyAggregateYieldsEmptyList() {
        when(productRepository.findPublicProductCountsByStore()).thenReturn(List.of());

        assertEquals(List.of(), productService.listStoreIDsWithPublicProducts());
    }
}
