package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.Optional;

import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #2820：商品軟刪必須連帶撤回該商品**待審**的認證申請。
 *
 * <p>修復前 {@code softDeleteProduct} 的七個步驟完全不觸及
 * {@code product_certification_application}，而 19 §8.3 明令禁止掃 PENDING 的排程
 * ⇒ {@code SUBMITTED} 的申請永久留在 ADM 待審佇列，沒有任何路徑會收掉。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceCertificationCascadeTest {

    @Mock ProductRepository productRepository;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductService productService;

    private static final long STORE_ID = 2639226113L;
    private static final long PRODUCT_ID = 876667630441496665L;

    private ProductEntity liveProduct() {
        ProductEntity e = new ProductEntity();
        e.setId(PRODUCT_ID);
        e.setStoreId(STORE_ID);
        e.setDeletedAt(null);
        // 非 ACTIVE：略過 hashtag 與共發 seller tag 的 revalidate 分支，聚焦本測試的關注點
        e.setStatus(ProductStatus.INACTIVE.value());
        return e;
    }

    @Test
    @DisplayName("softDeleteProduct 連帶撤回該商品待審的認證申請")
    void softDeleteWithdrawsSubmittedCertificationApplications() {
        when(productRepository.findById(PRODUCT_ID)).thenReturn(Optional.of(liveProduct()));

        productService.softDeleteProduct(String.valueOf(PRODUCT_ID), STORE_ID);

        verify(certificationApplicationService).withdrawSubmittedOnProductDeleted(PRODUCT_ID);
    }

    @Test
    @DisplayName("撤回發生在 deleteById 之前 —— 兩者必須在同一事務內，且鎖住的是尚未被刪的列")
    void withdrawHappensBeforeSoftDelete() {
        when(productRepository.findById(PRODUCT_ID)).thenReturn(Optional.of(liveProduct()));

        productService.softDeleteProduct(String.valueOf(PRODUCT_ID), STORE_ID);

        var ordered = inOrder(certificationApplicationService, productRepository);
        ordered.verify(certificationApplicationService).withdrawSubmittedOnProductDeleted(PRODUCT_ID);
        ordered.verify(productRepository).deleteById(PRODUCT_ID);
    }

    @Test
    @DisplayName("非擁有者刪除被擋下時，不得動到任何認證申請")
    void nonOwnerDeleteDoesNotTouchCertifications() {
        when(productRepository.findById(PRODUCT_ID)).thenReturn(Optional.of(liveProduct()));

        try {
            productService.softDeleteProduct(String.valueOf(PRODUCT_ID), STORE_ID + 1);
        } catch (RuntimeException expected) {
            // ProductAccessDeniedException
        }

        verify(certificationApplicationService, org.mockito.Mockito.never())
                .withdrawSubmittedOnProductDeleted(org.mockito.ArgumentMatchers.anyLong());
    }

    @Test
    @DisplayName("已刪除的商品重複刪除被擋下時，不得重複撤回（冪等性靠前置守衛而非下游）")
    void alreadyDeletedProductDoesNotTouchCertifications() {
        ProductEntity deleted = liveProduct();
        deleted.setDeletedAt(java.time.LocalDateTime.of(2026, 8, 1, 0, 0));
        when(productRepository.findById(PRODUCT_ID)).thenReturn(Optional.of(deleted));

        try {
            productService.softDeleteProduct(String.valueOf(PRODUCT_ID), STORE_ID);
        } catch (RuntimeException expected) {
            // ProductNotFoundException
        }

        verify(certificationApplicationService, org.mockito.Mockito.never())
                .withdrawSubmittedOnProductDeleted(org.mockito.ArgumentMatchers.anyLong());
    }
}
