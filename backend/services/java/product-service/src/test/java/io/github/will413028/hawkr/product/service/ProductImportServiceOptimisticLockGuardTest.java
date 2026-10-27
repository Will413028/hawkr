package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.CommonStatus;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalRequest;
import io.github.will413028.hawkr.product.grpc.ProductSkuImport;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker;
import io.github.will413028.hawkr.product.service.validator.ProductPublishValidator;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Import 轉正的「INSERT 後補寫」落盤判定（56 §6.6.3 (c) 的反面寫法）。
 *
 * <p>該 {@code updateById} 寫的是 gallery / description / 價格區間 / searchable_text 等
 * 全部反正規化欄位；丟棄回傳值就會回 {@code CREATED} 而這些欄位一個都沒落盤
 * ——商品破圖且搜尋查不到，且 caller 端把 item 標成成功、不會重試。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductImportServiceOptimisticLockGuardTest {

    @Mock CategoryService categoryService;
    @Mock CategoryCacheService categoryCacheService;
    @Mock IdempotencyService idempotencyService;
    @Mock ProductRepository productRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductImportAssetProcessor importAssetProcessor;
    @Mock HashtagService hashtagService;
    @Mock ProductCacheService cacheService;
    @Mock ProductPublishValidator productPublishValidator;
    @Mock ProductMediaReferenceLinker mediaReferenceLinker;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductImportService productImportService;

    private static final long STORE = 2685914682L;
    private static final long LV1 = 11L;
    private static final long LV2 = 12L;
    private static final long LV3 = 13L;

    @org.junit.jupiter.api.BeforeAll
    static void initTsid() {
        // product 的 node-id-base（§7.2）；未初始化時 TsidUtil.generate() 自己會拋
        // IllegalStateException，會讓下面的 assertThrows 拿到假綠。
        try { io.github.will413028.hawkr.common.utils.TsidUtil.init(200, 0); } catch (IllegalStateException ignored) { /* 已初始化 */ }
    }

    private CreateProductFromImportInternalRequest request() {
        return CreateProductFromImportInternalRequest.newBuilder()
                .setIdempotencyKey("import-key-1")
                .setStoreId(STORE)
                .setName("有機小松菜")
                .setCategoryId(String.valueOf(LV3))
                .setTargetStatus(ProductStatus.ACTIVE.code())
                .addSkus(ProductSkuImport.newBuilder()
                        .setSkuName("一把")
                        .setUnit(WeightUnit.KG.value())
                        .setPrice("120")
                        .setStockQty("10")
                        .setWeight("0.3")
                        .setLengthCm("10")
                        .setWidthCm("10")
                        .setHeightCm("10")
                        .build())
                .build();
    }

    private void stubHappyPathUpToPersist() {
        CategoryEntity leaf = new CategoryEntity();
        leaf.setId(LV3);
        leaf.setStatus(CommonStatus.ACTIVE.value());
        leaf.setParentId(LV2);

        when(idempotencyService.calculateHash(any(byte[].class))).thenReturn("hash");
        when(idempotencyService.acquireLock(anyString(), anyLong(), anyString(), anyString(), any()))
                .thenReturn(Optional.empty());
        when(productPublishValidator.validate(anyLong(), any(), any()))
                .thenReturn(new ProductPublishValidator.ValidationResult(List.of()));
        when(categoryCacheService.findById(LV3)).thenReturn(Optional.of(leaf));
        when(categoryService.detectCategoryLevel(LV3)).thenReturn(3);
        when(categoryService.getParentId(LV2)).thenReturn(LV1);
        when(categoryService.getCategoryNames(any()))
                .thenReturn(Map.of(LV1, "蔬菜", LV2, "葉菜", LV3, "小松菜"));
        when(importAssetProcessor.processGallery(anyLong(), any(), anyLong())).thenReturn(List.of());
        when(hashtagService.upsertHashtags(any())).thenReturn(List.of());
    }

    @Test
    @DisplayName("轉正補寫影響 0 列 ⇒ 拋 IllegalStateException 並標冪等 FAILED，不回 CREATED")
    void createProductFromImport_postInsertUpdateAffectsNoRow_throws() {
        stubHappyPathUpToPersist();
        when(productRepository.updateById(any())).thenReturn(0);

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> productImportService.createProductFromImport(request()));
        // 訊息要斷言：同型別的例外在這條路徑上還有別的來源（如 TsidUtil 未初始化），只判型別會拿到假綠
        org.junit.jupiter.api.Assertions.assertTrue(ex.getMessage().contains("轉正補寫影響 0 列"), ex.getMessage());

        // 補寫沒成功就不得建立 media 引用、不得標成功（否則 item 被記成已上架，永不重試）
        verify(mediaReferenceLinker, never()).linkGallery(anyLong(), any());
        verify(idempotencyService, never()).markSucceeded(anyString(), anyLong(), any());
        verify(idempotencyService).markFailed("import-key-1", STORE);
    }

    @Test
    @DisplayName("對照組：補寫影響 1 列 ⇒ 正常走完並標冪等成功")
    void createProductFromImport_postInsertUpdateAffectsOneRow_succeeds() {
        stubHappyPathUpToPersist();
        when(productRepository.updateById(any())).thenReturn(1);

        productImportService.createProductFromImport(request());

        verify(mediaReferenceLinker).linkGallery(anyLong(), any());
        verify(idempotencyService).markSucceeded(anyString(), anyLong(), any());
        verify(idempotencyService, never()).markFailed(anyString(), anyLong());
    }
}
