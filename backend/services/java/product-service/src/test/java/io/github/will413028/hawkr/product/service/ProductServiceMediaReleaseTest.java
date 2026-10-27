package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;
import java.util.Optional;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #1489：商品軟刪除必須釋放 PRODUCT / PRODUCT_DESC 的 Media 引用，
 * 否則 asset_reference row 永久殘留 → media-service GC 永不回收 → 孤兒媒體累積。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServiceMediaReleaseTest {

    @Mock ProductRepository productRepository;
    @Mock MediaGrpcClient mediaGrpcClient;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService productRevalidationService;
    // #2820：softDeleteProduct 會連帶撤回待審認證申請 —— 不 mock 會 NPE
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductService productService;

    @Test
    @DisplayName("softDeleteProduct 釋放 PRODUCT(7101) 與 PRODUCT_DESC(7102) 引用，ref_id=商品 id")
    void softDeleteReleasesMediaReferences() {
        long accountId = 987L;
        long productId = 123456789L;

        ProductEntity entity = new ProductEntity();
        entity.setId(productId);
        entity.setStoreId(accountId);
        entity.setDeletedAt(null);
        entity.setStatus(ProductStatus.INACTIVE.value()); // 非 ACTIVE：略過 hashtag 與共發 seller tag 的 revalidate 分支

        when(productRepository.findById(productId)).thenReturn(Optional.of(entity));

        productService.softDeleteProduct(String.valueOf(productId), accountId);

        List<String> expectedRefIds = List.of(String.valueOf(productId));
        verify(mediaGrpcClient).releaseReferences(AssetReferenceType.PRODUCT.value(), expectedRefIds);
        verify(mediaGrpcClient).releaseReferences(AssetReferenceType.PRODUCT_DESC.value(), expectedRefIds);
    }
}
