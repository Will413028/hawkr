package io.github.will413028.hawkr.product.service;

import com.baomidou.mybatisplus.annotation.Version;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.dto.request.CreateProductDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.lang.reflect.Field;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code ProductEntity} 加上 {@code @Version} 之後，{@code updateById} 由 last-write-wins
 * 變成 CAS：版本不符只回 0 列、<b>不拋例外</b>。丟棄回傳值的呼叫端會回 200 而資料沒寫進去
 * （56 §6.6.3 (c) 明列的反面寫法）。
 *
 * <p>每個測試都以「讀出 entity 到 updateById 之間有第三方改同一列」為模型——mock 讓
 * {@code updateById} 回 0，這正是併發寫入者已 commit 時 CAS 的實際回傳。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceOptimisticLockGuardTest {

    @Mock ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductEntityConverter productEntityConverter;
    @Mock IdempotencyService idempotencyService;
    @Mock ProductCacheService cacheService;
    @Mock CategoryService categoryService;
    @Mock HashtagService hashtagService;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock ProductMediaReferenceLinker mediaReferenceLinker;
    @InjectMocks ProductService productService;

    private static final long PID = 1001L;
    private static final long STORE = 2685914682L;
    private static final long LV1 = 11L;
    private static final long LV2 = 12L;
    private static final long LV3 = 13L;

    private ProductEntity persisted(String status, long version) {
        ProductEntity e = new ProductEntity();
        e.setId(PID);
        e.setStatus(status);
        e.setStoreId(STORE);
        e.setName("有機小松菜");
        e.setCategory(LV3);
        e.setCategoryLv2(LV2);
        e.setCategoryLv1(LV1);
        e.setVersion(version);
        return e;
    }

    // ==================== 攔截器開關 ====================

    @Test
    @DisplayName("ProductEntity.version 必須帶 @Version —— 少了它，本檔每一測都只是在驗 mock 自己的回傳值")
    void productVersionFieldCarriesVersionAnnotation() throws Exception {
        Field version = ProductEntity.class.getDeclaredField("version");

        assertNotNull(version.getAnnotation(Version.class),
                "ProductEntity.version 缺少 @Version。OptimisticLockerInnerInterceptor 只認這個註解，"
                        + "沒有它 updateById 不會補上 AND version = #{et.version} 述詞 ⇒ 回到 last-write-wins，"
                        + "代行者與賣家同時編輯時後寫的靜默覆蓋前寫的，永遠不會出現 9266。"
                        + "本檔其餘測試的 rows=0 全部由 mock 直接 thenReturn(0)，"
                        + "分辨不出真實述詞在不在 ⇒ 拿掉註解後那些測試照樣全綠。");
    }

    // ==================== admin：UpdateProductStatusForAdmin ====================

    @Test
    @DisplayName("admin 改狀態：CAS 回 0 列 ⇒ 拋 9266，不回成功")
    void updateProductStatus_casMiss_throwsVersionConflict() {
        // fixture 刻意補滿到「成功回應」所需的每一個相依：拿掉守衛時這條路徑要一路走到回傳 DTO
        // （那正是漏洞的形狀），而不是半途撞上未 stub 的相依拋 NPE——後者會讓「轉紅」變成假訊號
        ProductEntity e = persisted(ProductStatus.ACTIVE.value(), 7L);
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productRepository.updateById(any())).thenReturn(0);
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(any(), any(), anyBoolean()))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());
        when(categoryService.getCategoryName(LV3)).thenReturn("葉菜類");

        ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                () -> productService.updateProductStatus(String.valueOf(PID), ProductStatus.INACTIVE.value()));

        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT.code(), ex.code());
        // CAS 未命中即中止：快取失效不得發生
        verify(cacheService, never()).evictProductDetail(anyString());
    }

    @Test
    @DisplayName("admin 改狀態：CAS 命中 ⇒ 回應帶的 version 是 +1 後的值（不可回舊值讓下一次寫必然衝突）")
    void updateProductStatus_casHit_returnsBumpedVersion() {
        ProductEntity e = persisted(ProductStatus.ACTIVE.value(), 7L);
        when(productRepository.findById(PID)).thenReturn(Optional.of(e));
        when(productRepository.updateById(any())).thenReturn(1);
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(any(), any(), anyBoolean()))
                .thenAnswer(inv -> ProductDetailDTO.builder()
                        .id(String.valueOf(PID))
                        .version(((ProductEntity) inv.getArgument(0)).getVersion())
                        .build());
        when(categoryService.getCategoryName(LV3)).thenReturn("葉菜類");

        ProductDetailDTO result = productService.updateProductStatus(
                String.valueOf(PID), ProductStatus.INACTIVE.value());

        assertEquals(8L, result.getVersion());
    }

    // ==================== cascade reflow（分類改名 / 批次換分類） ====================

    @Test
    @DisplayName("reflow：第一次 CAS 未命中 ⇒ 重讀重算後再寫一次，成功即計入")
    void reflow_casMissThenRetry_succeeds() {
        when(productRepository.findById(PID))
                .thenReturn(Optional.of(persisted(ProductStatus.ACTIVE.value(), 7L)))
                .thenReturn(Optional.of(persisted(ProductStatus.ACTIVE.value(), 8L)));
        when(productRepository.updateById(any())).thenReturn(0).thenReturn(1);
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(categoryService.getCategoryNames(any())).thenReturn(Map.of(LV1, "蔬菜", LV2, "葉菜", LV3, "小松菜"));

        assertEquals(1, productService.reflowSearchableForProductIds(List.of(PID)));

        // 重讀是重點：拿的是併發寫入者留下的當前列，不是第一次讀到的舊快照
        verify(productRepository, times(2)).findById(PID);
        verify(productRepository, times(2)).updateById(any());
    }

    @Test
    @DisplayName("reflow：重試後仍未命中 ⇒ 拋 9266，不靜默把 searchable_text 停在舊分類名")
    void reflow_casMissTwice_throwsVersionConflict() {
        when(productRepository.findById(PID))
                .thenReturn(Optional.of(persisted(ProductStatus.ACTIVE.value(), 7L)));
        when(productRepository.updateById(any())).thenReturn(0);
        when(hashtagService.findHashtagsByProductId(PID)).thenReturn(List.of());
        when(productSkuRepository.findByProductId(PID)).thenReturn(List.of());
        when(categoryService.getCategoryNames(any())).thenReturn(Map.of(LV1, "蔬菜", LV2, "葉菜", LV3, "小松菜"));

        ProductBusinessException ex = assertThrows(ProductBusinessException.class,
                () -> productService.reflowSearchableForProductIds(List.of(PID)));

        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_VERSION_CONFLICT.code(), ex.code());
        verify(productRepository, times(2)).updateById(any());
    }

    @Test
    @DisplayName("reflow：id 不存在／已軟刪除仍是跳過，不算失敗也不拋")
    void reflow_missingProduct_isSkipped() {
        when(productRepository.findById(PID)).thenReturn(Optional.empty());

        assertEquals(0, productService.reflowSearchableForProductIds(List.of(PID)));
        verify(productRepository, never()).updateById(any());
    }

    // ==================== 建立路徑：INSERT 後的補寫 ====================

    @Test
    @DisplayName("建立商品：補寫影響 0 列 ⇒ 拋 IllegalStateException 並標冪等 FAILED，不回 CREATED")
    void createProduct_postInsertUpdateAffectsNoRow_throws() {
        CreateProductDTO dto = new CreateProductDTO();
        dto.setIdempotencyKey("idem-key-1");
        dto.setStoreId(STORE);
        dto.setName("有機小松菜");
        dto.setCategoryId(LV3);
        dto.setStatus(ProductStatus.DRAFT.value());

        ProductEntity entity = persisted(ProductStatus.DRAFT.value(), 0L);
        entity.setVersion(null);   // INSERT 前後記憶體中都是 null，樂觀鎖攔截器因此整段略過
        when(idempotencyService.calculateHash(dto)).thenReturn("hash");
        when(idempotencyService.acquireLock(anyString(), eq(STORE), anyString(), anyString(), any()))
                .thenReturn(Optional.empty());
        when(productEntityConverter.toEntity(dto)).thenReturn(entity);
        when(categoryService.getParentId(LV3)).thenReturn(LV2);
        when(categoryService.getLv1Id(LV2)).thenReturn(LV1);
        when(categoryService.getCategoryNames(any())).thenReturn(Map.of(LV1, "蔬菜", LV2, "葉菜", LV3, "小松菜"));
        when(hashtagService.upsertHashtags(any())).thenReturn(List.of());
        when(productRepository.updateById(any())).thenReturn(0);
        // 補滿到「成功回應」為止：拿掉守衛時必須是回傳 DTO（漏洞形狀），不是半途 NPE（假訊號）
        when(hashtagService.findHashtagsByProductId(any())).thenReturn(List.of());
        when(productEntityConverter.toDetailDTO(any(), any(), anyBoolean()))
                .thenReturn(ProductDetailDTO.builder().id(String.valueOf(PID)).build());

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> productService.createProduct(dto));
        // 只判型別會拿到假綠（這條路徑上還有別的 IllegalStateException 來源）
        org.junit.jupiter.api.Assertions.assertTrue(
                ex.getMessage().contains("商品建立後補寫影響 0 列"), ex.getMessage());

        verify(idempotencyService).markFailed("idem-key-1", STORE);
        verify(idempotencyService, never()).markSucceeded(anyString(), any(), any());
    }
}
