package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.dto.request.ProductImageDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.grpc.client.MediaGrpcClient;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

/**
 * processImageUpdate 的 gallery 組裝順序（#1424）：最終順序必須完全等於前端送出的 newImages 陣列順序，
 * 既有圖（assetId）與新圖（stagedToken）可交錯混用，不得被切成「既有群組 → 新增群組」兩段式重排。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServiceImageUpdateOrderTest {

    @Mock MediaGrpcClient mediaGrpcClient;
    @InjectMocks ProductService productService;

    @SuppressWarnings("unchecked")
    private List<ProductEntity.GalleryItem> invoke(
            List<ProductEntity.GalleryItem> existing, List<ProductImageDTO> newImages) throws Exception {
        Method m = ProductService.class.getDeclaredMethod(
                "processImageUpdate", Long.class, List.class, List.class);
        m.setAccessible(true);
        return (List<ProductEntity.GalleryItem>) m.invoke(productService, 1L, existing, newImages);
    }

    private ProductEntity.GalleryItem existingItem(String assetId) {
        ProductEntity.GalleryItem g = new ProductEntity.GalleryItem();
        g.setAssetId(assetId);
        g.setObjectKey("obj-" + assetId);
        return g;
    }

    @Test
    @DisplayName("新圖拖曳插入既有圖之間 [A, 新, B] → 落地順序 = [A, 新, B]，不被重排到最後")
    void interleavedOrderPreserved() throws Exception {
        List<ProductEntity.GalleryItem> existing = List.of(existingItem("a-A"), existingItem("a-B"));

        List<ProductImageDTO> newImages = new ArrayList<>();
        newImages.add(ProductImageDTO.builder().assetId("a-A").build());
        newImages.add(ProductImageDTO.builder().stagedToken("tok-N").build());
        newImages.add(ProductImageDTO.builder().assetId("a-B").build());

        when(mediaGrpcClient.finalizeImages(any(), anyString()))
                .thenReturn(List.of(new MediaGrpcClient.FinalizedImage("tok-N", "a-N", "obj-N")));

        List<ProductEntity.GalleryItem> result = invoke(existing, newImages);

        assertEquals(List.of("a-A", "a-N", "a-B"),
                result.stream().map(ProductEntity.GalleryItem::getAssetId).toList(),
                "交錯順序必須逐項保序，新圖不得被排到既有圖之後");
    }

    @Test
    @DisplayName("純重排既有圖 [B, A] → [B, A]（regression：不呼叫 finalizeImages）")
    void pureReorderPreserved() throws Exception {
        List<ProductEntity.GalleryItem> existing = List.of(existingItem("a-A"), existingItem("a-B"));

        List<ProductImageDTO> newImages = new ArrayList<>();
        newImages.add(ProductImageDTO.builder().assetId("a-B").build());
        newImages.add(ProductImageDTO.builder().assetId("a-A").build());

        List<ProductEntity.GalleryItem> result = invoke(existing, newImages);

        assertEquals(List.of("a-B", "a-A"),
                result.stream().map(ProductEntity.GalleryItem::getAssetId).toList());
    }
}
