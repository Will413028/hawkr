package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.cache.RevalidateClient;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anySet;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * ProductRevalidationService tag 組成測試（#1425；M3 收斂後 = 53 號設計書 §4.4 step3）。
 *
 * <p>核心不變式：
 * <ol>
 *   <li>producer <b>只發 entity tag</b>——任何變體都不得出現 legacy {@code home} 或
 *       domain-collection（{@code products:list} 等，那是 web expansion 的職責）。</li>
 *   <li>共發 {@code seller:<id>} 僅限「改變賣家公開商品集合」的變體；欄位編輯 / 庫存 /
 *       改分類只發 {@code product:<id>}。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
class ProductRevalidationServiceTest {

    @Mock RevalidateClient revalidateClient;
    @Mock ProductRepository productRepository;
    @Mock SellerGrpcClient sellerGrpcClient;

    ProductRevalidationService service;

    @BeforeEach
    void setUp() {
        service = new ProductRevalidationService(revalidateClient, productRepository, sellerGrpcClient);
    }

    /** 讓 seller-service 對這些 store_id 回一個 store_code = store_id + 900000000 的 meta。 */
    private void stubStoreCodes(Long... storeIds) {
        Map<Long, SellerMeta> metas = new LinkedHashMap<>();
        for (Long id : storeIds) {
            metas.put(id, SellerMeta.newBuilder()
                    .setStoreId(String.valueOf(id))
                    .setStoreCode(String.valueOf(id + 900000000L))
                    .build());
        }
        when(sellerGrpcClient.batchGetSellerMetas(anySet())).thenReturn(metas);
    }

    @SuppressWarnings("unchecked")
    private List<String> captureTags() {
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(revalidateClient).bustAfterCommit(captor.capture());
        return captor.getValue();
    }

    // ---- 商品集合不變：只發 product entity tag ----

    @Test
    @DisplayName("欄位編輯：revalidateProduct 只含 product，不含 seller / home")
    void fieldEditEmitsProductOnly() {
        service.revalidateProduct(1001L);
        assertThat(captureTags()).containsExactly("product:1001");
    }

    @Test
    @DisplayName("庫存 / 改分類批次：revalidateProducts 只含每筆 product（去重），不反查賣家")
    void batchFieldEditEmitsProductsOnly() {
        service.revalidateProducts(List.of(1001L, 1002L, 1001L));

        assertThat(captureTags()).containsExactly("product:1001", "product:1002");
        verifyNoInteractions(productRepository);
    }

    @Test
    @DisplayName("空批次：不發任何 revalidate 請求")
    void emptyBatchIsNoop() {
        service.revalidateProducts(List.of());
        verifyNoInteractions(revalidateClient);
    }

    // ---- 商品集合變動 / 賣家實體變動：共發 seller tag ----

    @Test
    @DisplayName("建立即上架 / 上下架跨 ACTIVE / 刪除前為 ACTIVE：revalidateProductAndSeller 含 product + seller")
    void productSetChangeEmitsProductAndSeller() {
        stubStoreCodes(2001L);
        service.revalidateProductAndSeller(1001L, 2001L);
        assertThat(captureTags())
                .containsExactlyInAnyOrder("product:1001", "seller:2001", "seller:900002001");
    }

    @Test
    @DisplayName("新建 DRAFT：revalidateSeller 只含 seller")
    void draftCreateEmitsSellerOnly() {
        stubStoreCodes(2001L);
        service.revalidateSeller(2001L);
        assertThat(captureTags()).containsExactlyInAnyOrder("seller:2001", "seller:900002001");
    }

    @Test
    @DisplayName("賣家批次下架：revalidateProductsWithSellers 含每筆 product + 反查去重後的 seller")
    void batchOffShelfEmitsProductsAndSellers() {
        when(productRepository.findProductSellerRefs(List.of(1001L, 1002L)))
                .thenReturn(List.of(new ProductSellerRef(1001L, 2001L), new ProductSellerRef(1002L, 2001L)));

        stubStoreCodes(2001L);

        service.revalidateProductsWithSellers(List.of(1001L, 1002L));

        assertThat(captureTags())
                .containsExactlyInAnyOrder(
                        "product:1001", "product:1002", "seller:2001", "seller:900002001");
    }

    // ---- 賣家 tag 必須同時涵蓋店面頁與 PDP 兩把 cache key ----

    /**
     * web 以 URL 路徑片段當 cache key，而同一個賣家有兩個片段各自活著：店面頁被 308 收斂到
     * {@code /sellers/{store_code}}，PDP 的賣家卡片則走商品 payload 的 {@code seller.storeId}
     * （{@code features/product/SellerIntroAsync.tsx}，不經 308）。
     *
     * <p>本測試釘的是一個真的壞過的行為：只發 store_id 時，店面頁的失效永遠打不中，而
     * {@code /api/revalidate} 照樣回 200，所以沒有人會發現——賣家上下架商品後，店面頁要等
     * 1800s TTL 才換內容。
     */
    @Test
    @DisplayName("賣家 tag 同時含 store_id（PDP）與 store_code（店面頁），兩者皆不可少")
    void sellerTagsCoverBothStorefrontAndPdpCacheKeys() {
        stubStoreCodes(2001L);

        service.revalidateSeller(2001L);

        assertThat(captureTags())
                .as("store_code 缺席 = 店面頁失效靜默打不中；store_id 缺席 = PDP 賣家卡片變舊")
                .contains("seller:900002001", "seller:2001");
    }

    @Test
    @DisplayName("seller-service 解析不到 store_code：fail-soft 退回只發 store_id，不整批放棄")
    void unresolvableStoreCodeDegradesToStoreIdOnly() {
        when(sellerGrpcClient.batchGetSellerMetas(anySet())).thenReturn(Map.of());

        service.revalidateProductAndSeller(1001L, 2001L);

        assertThat(captureTags()).containsExactlyInAnyOrder("product:1001", "seller:2001");
    }

    // ---- 分類：分類樹 surface（#1919）----

    @Test
    @DisplayName("分類新增 / 還原：revalidateCategory 只含 category")
    void categoryCreateOrRestoreEmitsCategoryOnly() {
        service.revalidateCategory(9001L);
        assertThat(captureTags()).containsExactly("category:9001");
    }

    @Test
    @DisplayName("批次軟刪除：revalidateCategories 每筆 category 去重，單次 POST")
    void categoryBatchDeleteEmitsDistinctCategories() {
        service.revalidateCategories(List.of(9001L, 9002L, 9001L));

        assertThat(captureTags()).containsExactly("category:9001", "category:9002");
        // 前置驗證保證分類底下無商品，故不該反查商品
        verifyNoInteractions(productRepository);
    }

    @Test
    @DisplayName("空批次 / null 分類：不發任何 revalidate 請求")
    void emptyCategoryBatchIsNoop() {
        service.revalidateCategories(List.of());
        service.revalidateCategory(null);
        verifyNoInteractions(revalidateClient);
    }

    /**
     * 改名是同一個 mutation 動到兩個 surface（分類樹 + 每筆商品的 cat_*_name），
     * 所以必須合成單次 POST——拆成兩次違反 §9.8「每 mutation 一次 POST」。
     */
    @Test
    @DisplayName("分類改名：category + 受影響 product 合成單次 POST，不共發 seller")
    void categoryRenameEmitsCategoryAndProductsInOnePost() {
        service.revalidateCategoryWithProducts(9001L, List.of(1001L, 1002L, 1001L));

        assertThat(captureTags())
                .containsExactly("category:9001", "product:1001", "product:1002");
        verify(revalidateClient, org.mockito.Mockito.times(1))
                .bustAfterCommit(org.mockito.ArgumentMatchers.anyList());
        verifyNoInteractions(sellerGrpcClient);
    }

    @Test
    @DisplayName("分類改名但底下無商品：只發 category，不發空的 product tag")
    void categoryRenameWithNoProductsEmitsCategoryOnly() {
        service.revalidateCategoryWithProducts(9001L, List.of());
        assertThat(captureTags()).containsExactly("category:9001");
    }

    /**
     * backend 只發 entity tag；{@code categories} 是 domain-collection，由 web
     * {@code /api/revalidate} 從 {@code category:<id>} 展開（§9.8）。producer 直接發它
     * 會繞過信任邊界的展開邏輯。
     */
    @Test
    @DisplayName("分類變體不得直接發 domain-collection tag categories")
    void categoryVariantsNeverEmitDomainCollectionTag() {
        service.revalidateCategory(9001L);
        service.revalidateCategories(List.of(9002L));
        service.revalidateCategoryWithProducts(9003L, List.of(1001L));

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(revalidateClient, org.mockito.Mockito.times(3)).bustAfterCommit(captor.capture());
        assertThat(captor.getAllValues())
                .allSatisfy(tags -> assertThat(tags).doesNotContain("categories", "home"));
    }

    // ---- 全變體共同約束：不得再發 legacy home ----

    @Test
    @DisplayName("所有變體都不再發 legacy home tag")
    void noVariantEmitsHomeTag() {
        when(productRepository.findProductSellerRefs(List.of(1001L)))
                .thenReturn(List.of(new ProductSellerRef(1001L, 2001L)));

        service.revalidateProduct(1001L);
        service.revalidateProducts(List.of(1001L));
        service.revalidateProductAndSeller(1001L, 2001L);
        service.revalidateSeller(2001L);
        service.revalidateProductsWithSellers(List.of(1001L));

        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<String>> captor = ArgumentCaptor.forClass(List.class);
        verify(revalidateClient, org.mockito.Mockito.times(5)).bustAfterCommit(captor.capture());
        assertThat(captor.getAllValues()).allSatisfy(tags -> assertThat(tags).doesNotContain("home"));
    }
}
