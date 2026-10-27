package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.dto.request.CreateProductDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * 賣家停權 ACTIVE transition backstop 測試（02 §5.10 步驟 5b / 46 §6.2 §11，首次接線）。
 *
 * <p>{@code UpdateSellerProductStatus} 與 {@code CreateProduct} 共用同一條私有規則
 * {@code requireSellerCanManageProductsForActivation}，故規則本身（false/true/查詢失敗）
 * 只需驗一次；另補 {@code updateSellerProductStatus} 的接線測試，確認：
 * (a) 目標 ACTIVE 時真的呼叫了 backstop 且會在 {@code updateById} 前擋下、
 * (b) 目標 INACTIVE（收縮型操作）不消費 SellerMeta（設計書明文「不需擋」）。
 */
@ExtendWith(MockitoExtension.class)
class ProductServiceSellerSuspensionActivationTest {

    /**
     * store 代理鍵（{@code store_id}），非 JWT {@code account_id}：本測試涵蓋的三條生產路徑
     * （{@code requireSellerCanManageProductsForActivation} / {@code updateSellerProductStatus} /
     * {@code createProduct}）參數一律命名 {@code storeId}。同一個值也餵給 {@code setCreatedBy} 僅是
     * audit 寫入；本服務 {@code product.store_id} 才是 owner contract（見 V8/V9 migration），
     * 不是兩個 id 空間混用。
     */
    private static final long STORE_ID = 200L;
    private static final long PRODUCT_ID = 1001L;

    // ==================== 規則本身（package-visible test seam） ====================

    @Nested
    class RuleOnly {
        @Mock SellerGrpcClient sellerGrpcClient;
        @Mock ProductRepository productRepository;
        @InjectMocks ProductService productService;

        @Test
        @DisplayName("停權賣家（can_manage_products=false）→ 拋 ProductBusinessException 9240")
        void suspendedSeller_throws9240() {
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(false).build());

            ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                    () -> productService.requireSellerCanManageProductsForActivationForTest(STORE_ID));
            assertEquals(9240, ex.code());
        }

        @Test
        @DisplayName("正常賣家（can_manage_products=true）→ 放行不拋例外")
        void activeSeller_allowed() {
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(true).build());

            assertDoesNotThrow(() ->
                    productService.requireSellerCanManageProductsForActivationForTest(STORE_ID));
        }

        @Test
        @DisplayName("可以管理商品但還不可接單（KYC 或 PayUni 未核准）→ 放行：上架閘只讀 can_manage_products")
        void canManageButNotAcceptOrders_allowed() {
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(true).setCanAcceptOrders(false).build());

            assertDoesNotThrow(() ->
                    productService.requireSellerCanManageProductsForActivationForTest(STORE_ID));
        }

        @Test
        @DisplayName("可接單但不能管理商品（等待刪除或停權）→ 拋 9240：不以 can_accept_orders 放行")
        void canAcceptOrdersButNotManageProducts_throws9240() {
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(false).setCanAcceptOrders(true).build());

            ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                    () -> productService.requireSellerCanManageProductsForActivationForTest(STORE_ID));
            assertEquals(9240, ex.code());
        }

        @Test
        @DisplayName("SellerMeta 查詢失敗（gRPC 例外）→ fail-closed 拋 ProductBusinessException 503，不放行")
        void sellerMetaQueryFails_failClosed503() {
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

            ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                    () -> productService.requireSellerCanManageProductsForActivationForTest(STORE_ID));
            assertEquals(503, ex.code());
        }
    }

    // ==================== updateSellerProductStatus 接線 ====================

    @Nested
    class UpdateSellerProductStatusWiring {
        @Mock ProductRepository productRepository;
        @Mock SellerGrpcClient sellerGrpcClient;
        @InjectMocks ProductService productService;

        private ProductEntity entity(String status) {
            ProductEntity e = new ProductEntity();
            e.setId(PRODUCT_ID);
            e.setCreatedBy(STORE_ID);
            e.setStoreId(STORE_ID);
            e.setStatus(status);
            return e;
        }

        @Test
        @DisplayName("DRAFT→ACTIVE 且賣家停權 → 擋下，updateById 未被呼叫（backstop 在更新前生效）")
        void draftToActive_suspendedSeller_blockedBeforeUpdate() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID))
                    .thenReturn(Optional.of(entity(ProductStatus.DRAFT.value())));
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(false).build());

            assertThrows(ProductBusinessException.class, () -> productService.updateSellerProductStatus(
                    String.valueOf(PRODUCT_ID), STORE_ID, ProductStatus.ACTIVE.value()));

            verify(productRepository, never()).updateById(any());
        }

        @Test
        @DisplayName("ACTIVE→INACTIVE（下架，收縮型操作）→ 不查 SellerMeta（02 §5.10 步驟 5b 明文不擋）")
        void activeToInactive_doesNotConsultSellerService() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID))
                    .thenReturn(Optional.of(entity(ProductStatus.ACTIVE.value())));

            // INACTIVE 下架路徑走到 productRepository.updateById 之後還有 hashtag/cache/SKU 查詢等
            // 未 mock 的協作者；本測試只關心「backstop 不消費 sellerGrpcClient」，
            // 用 verifyNoInteractions 斷言，故不需要在意後續是否因未 mock 協作者拋例外 —
            // 若真的拋了，代表已經跑過了「不呼叫 backstop」這段，斷言仍成立。
            try {
                productService.updateSellerProductStatus(
                        String.valueOf(PRODUCT_ID), STORE_ID, ProductStatus.INACTIVE.value());
            } catch (Exception ignored) {
                // 忽略後續未 mock 協作者導致的例外，本測試只關心 sellerGrpcClient 互動
            }

            verifyNoInteractions(sellerGrpcClient);
        }
    }

    // ==================== createProduct 接線（缺口#3：#1226「同 helper 覆蓋」風險） ====================

    /**
     * {@code createProduct} 與 {@code updateSellerProductStatus} 共用同一條私有規則，但落地路徑
     * 完全不同（insert 而非 updateById）——「規則本身已測」不代表「這條呼叫路徑真的接上了」。
     * 驗證：以 ACTIVE 落地（converter 未指定 status 時的預設值，見 ProductService.java:226-231）
     * 且賣家停權時，backstop 必須在 {@code productRepository.insert} 之前擋下，DB 完全無寫入。
     */
    @Nested
    class CreateProductWiring {
        @Mock ProductRepository productRepository;
        @Mock SellerGrpcClient sellerGrpcClient;
        @Mock IdempotencyService idempotencyService;
        @Mock ProductEntityConverter productEntityConverter;
        @InjectMocks ProductService productService;

        private CreateProductDTO dto() {
            CreateProductDTO dto = new CreateProductDTO();
            dto.setIdempotencyKey("idem-key-1");
            dto.setStoreId(STORE_ID);
            // description/hashtags 留空以繞過與本測試無關的 fail-fast 驗證
            // （HashtagInputValidator.requireHashtagSet(null) 直接回傳空集合、
            // validateDescriptionDtoTextBlocks(null) 直接 return）。
            return dto;
        }

        @Test
        @DisplayName("CreateProduct 以 ACTIVE 落地且賣家停權 → backstop 在 insert 前擋下，DB 無寫入")
        void createProduct_activeStatus_suspendedSeller_blockedBeforeInsert() {
            CreateProductDTO dto = dto();
            when(idempotencyService.calculateHash(any())).thenReturn("hash-1");
            when(idempotencyService.acquireLock(anyString(), anyLong(), anyString(), anyString(), any()))
                    .thenReturn(Optional.empty());
            ProductEntity entityToInsert = new ProductEntity();
            entityToInsert.setStatus(ProductStatus.ACTIVE.value());
            when(productEntityConverter.toEntity(dto)).thenReturn(entityToInsert);
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(false).build());

            ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                    () -> productService.createProduct(dto));

            assertEquals(9240, ex.code());
            verify(productRepository, never()).insert(any());
        }

        @Test
        @DisplayName("CreateProduct 以 ACTIVE 落地但賣家正常 → 呼叫 backstop 放行，繼續走到 insert 前的下一步")
        void createProduct_activeStatus_activeSeller_backstopPassesThrough() {
            CreateProductDTO dto = dto();
            when(idempotencyService.calculateHash(any())).thenReturn("hash-1");
            when(idempotencyService.acquireLock(anyString(), anyLong(), anyString(), anyString(), any()))
                    .thenReturn(Optional.empty());
            ProductEntity entityToInsert = new ProductEntity();
            entityToInsert.setStatus(ProductStatus.ACTIVE.value());
            when(productEntityConverter.toEntity(dto)).thenReturn(entityToInsert);
            when(sellerGrpcClient.getSellerMetaOrThrow(STORE_ID))
                    .thenReturn(SellerMeta.newBuilder().setCanManageProducts(true).build());

            // categoryService 等後續協作者未 mock，會在 backstop 通過後的下一步拋例外——
            // 這正是我們要的訊號：backstop 沒有擋下，代表接線放行成立。
            // （若 backstop 誤判擋下，會拋 ProductBusinessException 9240 而非其他例外，斷言即失敗。）
            Exception ex = assertThrows(Exception.class, () -> productService.createProduct(dto));
            org.junit.jupiter.api.Assertions.assertFalse(
                    ex instanceof ProductBusinessException pbe && pbe.code() == 9240,
                    "backstop 應放行正常賣家，不應拋 9240，實際: " + ex);

            verify(sellerGrpcClient).getSellerMetaOrThrow(STORE_ID);
        }
    }
}
