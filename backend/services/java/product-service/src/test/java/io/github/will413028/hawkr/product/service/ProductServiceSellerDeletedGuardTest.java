package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.exception.ProductNotFoundException;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.when;

/**
 * 賣家 PDP / admin 狀態變更 對 seller_deleted flagged 商品的 guard（Part A / doc 44 §11）。
 * guard 皆在 findById 之後即拋，故只需 mock ProductRepository。
 */
@ExtendWith(MockitoExtension.class)
class ProductServiceSellerDeletedGuardTest {

    @Mock ProductRepository productRepository;
    @InjectMocks ProductService productService;

    private static final long OWNER = 2619961976L;
    private static final long PID = 1001L;

    private ProductEntity flagged(String status) {
        ProductEntity e = new ProductEntity();
        e.setId(PID);
        e.setCreatedBy(OWNER);
        e.setStoreId(OWNER);
        e.setStatus(status);
        e.setSellerDeletedAt(LocalDateTime.now());
        return e;
    }

    @Test
    @DisplayName("getProduct：flagged + owner + 非 ACTIVE → NotFound（對復活賣家隱形）")
    void getProductFlaggedOwnerInactiveThrowsNotFound() {
        when(productRepository.findById(PID)).thenReturn(Optional.of(flagged(ProductStatus.INACTIVE.value())));
        assertThrows(ProductNotFoundException.class,
                () -> productService.getProduct(String.valueOf(PID), OWNER));
    }

    @Test
    @DisplayName("admin updateProductStatus：flagged 商品目標 ACTIVE → 拋 ProductBusinessException（禁復活）")
    void adminReactivateFlaggedThrows() {
        when(productRepository.findById(PID)).thenReturn(Optional.of(flagged(ProductStatus.INACTIVE.value())));
        assertThrows(ProductBusinessException.class,
                () -> productService.updateProductStatus(String.valueOf(PID), ProductStatus.ACTIVE.value()));
    }
}
