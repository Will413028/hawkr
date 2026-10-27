package io.github.will413028.hawkr.product.service;

import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.product.dto.request.ProductQueryDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.when;

/**
 * getProductList 的 seller_deleted exclude 述詞連線測試（Part A / doc 44 §11）。
 * 只驗傳給 findPageByCondition 的 excludeSellerDeleted：賣家自己列表 true、公開篩選 false。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServiceListGuardTest {

    @Mock ProductRepository productRepository;
    @Mock ProductCacheService cacheService;
    @Mock io.github.will413028.hawkr.product.converter.ProductEntityConverter productEntityConverter;
    @Mock HashtagService hashtagService;
    @InjectMocks ProductService productService;

    private ArgumentCaptor<Boolean> runAndCapture(ProductQueryDTO query) {
        when(cacheService.generateListCacheKey(any())).thenReturn("k");
        when(cacheService.getProductList(anyString())).thenReturn(Optional.empty());
        IPage<ProductEntity> empty = new Page<>(1, 20);
        ArgumentCaptor<Boolean> cap = ArgumentCaptor.forClass(Boolean.class);
        when(productRepository.findPageByCondition(
                any(), any(), any(), any(), any(), any(), any(), any(),
                anyBoolean(), anyInt(), any(), any(), any(), any(), any(), any(),
                any(), cap.capture(), exposureCap.capture())).thenReturn(empty);
        productService.getProductList(query);
        return cap;
    }

    private final ArgumentCaptor<Boolean> exposureCap = ArgumentCaptor.forClass(Boolean.class);

    @Test
    @DisplayName("賣家看自己的商品（ownerStoreId 設、storeId 空）→ excludeSellerDeleted=true")
    void sellerOwnListExcludes() {
        ProductQueryDTO q = new ProductQueryDTO();
        q.setOwnerStoreId(2619961976L);
        q.setPage(1);
        q.setPageSize(20);
        assertEquals(Boolean.TRUE, runAndCapture(q).getValue());
    }

    @Test
    @DisplayName("公開篩選某賣家（storeId 設）→ excludeSellerDeleted=false（flagged 已 INACTIVE，靠 status 過濾）")
    void publicSellerFilterDoesNotExclude() {
        ProductQueryDTO q = new ProductQueryDTO();
        q.setStoreId(2619961976L);
        q.setPage(1);
        q.setPageSize(20);
        assertEquals(Boolean.FALSE, runAndCapture(q).getValue());
    }

    @Test
    @DisplayName("公開視角（publicApi）→ requireExposableStore=true；賣家看自己、admin → false")
    void exposableStoreFilterAppliesToPublicViewOnly() {
        ProductQueryDTO pub = new ProductQueryDTO();
        pub.setPublicApi(true);
        pub.setPage(1);
        pub.setPageSize(20);
        runAndCapture(pub);
        assertEquals(Boolean.TRUE, exposureCap.getValue());

        ProductQueryDTO seller = new ProductQueryDTO();
        seller.setOwnerStoreId(2619961976L);
        seller.setPage(1);
        seller.setPageSize(20);
        runAndCapture(seller);
        assertEquals(Boolean.FALSE, exposureCap.getValue());

        ProductQueryDTO admin = new ProductQueryDTO();
        admin.setPage(1);
        admin.setPageSize(20);
        runAndCapture(admin);
        assertEquals(Boolean.FALSE, exposureCap.getValue());
    }
}
