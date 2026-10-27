package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.CommonStatus;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.repository.CategoryRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;
import java.util.Optional;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyCollection;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * CategoryService → ProductRevalidationService 接線測試（#1919）。
 *
 * <p>釘的是「producer 有沒有真的被呼叫」，而不是 tag 怎麼組（那在
 * {@link ProductRevalidationServiceTest}）。這一層值得單獨釘，是因為缺 producer 的
 * under-invalidation <b>沒有任何失敗訊號</b>：分類樹的 web 快取 TTL 是 21600 秒，
 * 漏發只表現為「改名後最久 6 小時才反映」，不會有 error、不會有告警。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CategoryServiceRevalidateTest {

    @Mock CategoryRepository categoryRepository;
    @Mock CategoryCacheService cacheService;
    @Mock ProductCascadeService productCascadeService;
    @Mock ProductRevalidationService productRevalidationService;

    CategoryService service;

    @BeforeEach
    void setUp() {
        service = new CategoryService(categoryRepository, cacheService,
                productCascadeService, productRevalidationService);
    }

    private static CategoryEntity active(Long id, String name, Long parentId) {
        CategoryEntity e = new CategoryEntity();
        e.setId(id);
        e.setName(name);
        e.setParentId(parentId);
        e.setStatus(CommonStatus.ACTIVE.value());
        return e;
    }

    // ---- 新增 ----

    @Test
    @DisplayName("新增分類：發 category:<新 id>（新分類進入分類樹）")
    void createCategoryRevalidatesNewCategory() {
        when(categoryRepository.findByNameAndParentId(eq("葉菜類"), any())).thenReturn(Optional.empty());
        when(categoryRepository.insert(any(CategoryEntity.class))).thenAnswer(inv -> {
            inv.getArgument(0, CategoryEntity.class).setId(9001L);
            return 1;
        });

        service.createCategory(null, "葉菜類", 42L);

        verify(productRevalidationService).revalidateCategory(9001L);
    }

    /**
     * 同名同父已有 ACTIVE 分類時是「透明回傳既有分類」，一個位元組都沒改——
     * 此時 bust 就是純粹的 over-invalidation。
     */
    @Test
    @DisplayName("自動套用既有 ACTIVE 分類：什麼都沒改，不得 bust")
    void createCategoryAutoApplyDoesNotRevalidate() {
        when(categoryRepository.findByNameAndParentId(eq("葉菜類"), any()))
                .thenReturn(Optional.of(active(9001L, "葉菜類", null)));

        service.createCategory(null, "葉菜類", 42L);

        verifyNoInteractions(productRevalidationService);
        verify(categoryRepository, never()).insert(any(CategoryEntity.class));
    }

    // ---- 改名 ----

    @Test
    @DisplayName("分類改名：category + cascade 受影響 product 走單次合併 POST")
    void updateCategoryNameRevalidatesCategoryWithProducts() {
        CategoryEntity entity = active(9001L, "舊名", null);
        when(categoryRepository.findById(9001L)).thenReturn(Optional.of(entity));
        when(categoryRepository.findByNameAndParentId(eq("新名"), any())).thenReturn(Optional.empty());
        when(cacheService.findById(9001L)).thenReturn(Optional.of(entity));
        when(productCascadeService.findProductIdsByCategoryLevel(9001L, 1))
                .thenReturn(List.of(1001L, 1002L));

        service.updateCategoryName(9001L, "新名", 42L);

        verify(productRevalidationService)
                .revalidateCategoryWithProducts(9001L, List.of(1001L, 1002L));
        // 分別發兩次 = 違反「每 mutation 一次 POST」
        verify(productRevalidationService, never()).revalidateProducts(anyCollection());
        verify(productRevalidationService, never()).revalidateCategory(anyLong());
    }

    @Test
    @DisplayName("分類改名但底下無商品：仍要發 category（分類樹本身變了）")
    void updateCategoryNameWithNoProductsStillRevalidatesCategory() {
        CategoryEntity entity = active(9001L, "舊名", null);
        when(categoryRepository.findById(9001L)).thenReturn(Optional.of(entity));
        when(categoryRepository.findByNameAndParentId(eq("新名"), any())).thenReturn(Optional.empty());
        when(cacheService.findById(9001L)).thenReturn(Optional.of(entity));
        when(productCascadeService.findProductIdsByCategoryLevel(9001L, 1)).thenReturn(List.of());

        service.updateCategoryName(9001L, "新名", 42L);

        verify(productRevalidationService).revalidateCategoryWithProducts(9001L, List.of());
    }

    /**
     * 名稱沒變時方法提早 return，分類樹的內容一模一樣。
     */
    @Test
    @DisplayName("改名成相同名稱：no-op，不得 bust")
    void updateCategoryNameToSameNameDoesNotRevalidate() {
        when(categoryRepository.findById(9001L)).thenReturn(Optional.of(active(9001L, "同名", null)));

        service.updateCategoryName(9001L, "同名", 42L);

        verifyNoInteractions(productRevalidationService);
    }

    // ---- 軟刪除 / 還原 ----

    @Test
    @DisplayName("批次軟刪除：發被刪的每個 category（分類離開分類樹）")
    void softDeleteRevalidatesDeletedCategories() {
        CategoryEntity a = active(9001L, "A", null);
        CategoryEntity b = active(9002L, "B", null);
        when(categoryRepository.findById(9001L)).thenReturn(Optional.of(a));
        when(categoryRepository.findById(9002L)).thenReturn(Optional.of(b));
        when(cacheService.findById(9001L)).thenReturn(Optional.of(a));
        when(cacheService.findById(9002L)).thenReturn(Optional.of(b));
        when(categoryRepository.hasActiveChildren(anyLong())).thenReturn(false);
        when(categoryRepository.softDelete(anyLong(), any(), any())).thenReturn(1);

        service.softDelete(List.of(9001L, 9002L), 42L, (id, level) -> false);

        verify(productRevalidationService).revalidateCategories(List.of(9001L, 9002L));
    }

    @Test
    @DisplayName("還原分類：發 category:<id>（分類回到分類樹）")
    void restoreRevalidatesCategory() {
        CategoryEntity deleted = active(9001L, "A", null);
        deleted.setStatus(CommonStatus.INACTIVE.value());
        when(categoryRepository.findById(9001L)).thenReturn(Optional.of(deleted));
        when(categoryRepository.findByNameAndParentId(eq("A"), any())).thenReturn(Optional.empty());
        when(categoryRepository.restore(anyLong(), any(), any())).thenReturn(1);

        service.restore(9001L, 42L);

        verify(productRevalidationService).revalidateCategory(9001L);
    }
}
