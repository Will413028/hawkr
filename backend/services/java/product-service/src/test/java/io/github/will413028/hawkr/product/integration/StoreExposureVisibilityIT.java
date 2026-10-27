package io.github.will413028.hawkr.product.integration;

import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.domain.ProductPublicationPolicy;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.repository.ProductHashtagRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.repository.StoreExposureRepository;
import io.github.will413028.hawkr.product.service.ProductService;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 買家讀取面依 {@code store_exposure} 過濾（真實 PostgreSQL）：
 * 店 A 可曝光、店 B 不可曝光、店 C 沒有 store_exposure 列（等同不可曝光）。
 */
class StoreExposureVisibilityIT extends AbstractProductPostgresIT {

    private static final long STORE_A = 1001L;
    private static final long STORE_B = 1002L;
    private static final long STORE_C = 1003L;

    private static final long A_ACTIVE_1 = 5001L;
    private static final long A_ACTIVE_2 = 5002L;
    private static final long A_INACTIVE = 5003L;
    private static final long B_ACTIVE = 5101L;
    private static final long C_ACTIVE = 5201L;

    @Autowired ProductRepository productRepository;
    @Autowired ProductSkuRepository productSkuRepository;
    @Autowired StoreExposureRepository storeExposureRepository;
    @Autowired ProductHashtagRepository productHashtagRepository;
    @Autowired io.github.will413028.hawkr.product.repository.HashtagRepository hashtagRepository;
    @Autowired JdbcTemplate jdbc;

    private long categoryId;

    @BeforeEach
    void seed() {
        jdbc.update("DELETE FROM product_hashtag");
        jdbc.update("DELETE FROM hashtag");
        jdbc.update("DELETE FROM product_sku");
        jdbc.update("DELETE FROM product");
        jdbc.update("DELETE FROM store_exposure");
        jdbc.update("DELETE FROM category");

        long lv1 = insertCategory("生鮮", null);
        long lv2 = insertCategory("蔬菜", lv1);
        categoryId = insertCategory("葉菜", lv2);
        categoryLevels = new long[] {lv1, lv2, categoryId};

        insertProduct(A_ACTIVE_1, STORE_A, "4004", "台灣");
        insertProduct(A_ACTIVE_2, STORE_A, "4004", "台灣");
        insertProduct(A_INACTIVE, STORE_A, "4003", "日本");
        insertProduct(B_ACTIVE, STORE_B, "4004", "台灣");
        insertProduct(C_ACTIVE, STORE_C, "4004", "日本");

        storeExposureRepository.upsertIfNewer(STORE_A, true, 1L);
        storeExposureRepository.upsertIfNewer(STORE_B, false, 1L);
    }

    private long[] categoryLevels;

    private long insertCategory(String name, Long parentId) {
        return jdbc.queryForObject(
                "INSERT INTO category (name, parent_id) VALUES (?, ?) RETURNING id", Long.class, name, parentId);
    }

    private void insertProduct(long id, long storeId, String status, String origin) {
        jdbc.update("""
                INSERT INTO product (id, name, status, category, category_lv2, category_lv1, origin,
                                     store_id, created_by, stock_qty)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 5)
                """, id, "商品" + id, status, categoryLevels[2], categoryLevels[1], categoryLevels[0],
                origin, storeId, storeId);
    }

    private IPage<ProductEntity> list(int pageNo, int pageSize, boolean requireExposableStore) {
        return productRepository.findPageByCondition(
                new Page<>(pageNo, pageSize), null, null, null, null, "4004", null, null,
                false, 0, null, null, null, null, null, null, null, false, requireExposableStore);
    }

    private static List<Long> ids(IPage<ProductEntity> page) {
        return page.getRecords().stream().map(ProductEntity::getId).sorted().collect(Collectors.toList());
    }

    @Test
    @DisplayName("公開列表：只含可曝光店的 ACTIVE 商品，總數與分頁都以過濾後為準；沒有 store_exposure 列的店同樣隱藏")
    void publicListHidesNonExposableStores() {
        IPage<ProductEntity> all = list(1, 20, true);
        assertThat(ids(all)).containsExactly(A_ACTIVE_1, A_ACTIVE_2);
        assertThat(all.getTotal()).isEqualTo(2L);

        IPage<ProductEntity> firstPage = list(1, 1, true);
        assertThat(firstPage.getRecords()).hasSize(1);
        assertThat(firstPage.getTotal()).isEqualTo(2L);
        IPage<ProductEntity> secondPage = list(2, 1, true);
        assertThat(secondPage.getRecords()).hasSize(1);
        assertThat(secondPage.getRecords().get(0).getId()).isNotEqualTo(firstPage.getRecords().get(0).getId());
    }

    @Test
    @DisplayName("賣家看自己商品／admin 視角（不要求可曝光）：不可曝光的店的商品仍在列表與總數內")
    void ownerViewIsNotFilteredByExposure() {
        IPage<ProductEntity> all = list(1, 20, false);
        assertThat(ids(all)).containsExactly(A_ACTIVE_1, A_ACTIVE_2, B_ACTIVE, C_ACTIVE);
        assertThat(all.getTotal()).isEqualTo(4L);
    }

    @Test
    @DisplayName("店家由可曝光翻成不可曝光後，列表與總數立即不含它的商品")
    void exposureFlipChangesListImmediately() {
        storeExposureRepository.upsertIfNewer(STORE_A, false, 2L);
        assertThat(list(1, 20, true).getRecords()).isEmpty();
        assertThat(list(1, 20, true).getTotal()).isZero();

        storeExposureRepository.upsertIfNewer(STORE_B, true, 2L);
        assertThat(ids(list(1, 20, true))).containsExactly(B_ACTIVE);
    }

    @Test
    @DisplayName("產地摘要與店家公開商品數只計可曝光店的 ACTIVE 商品")
    void summariesCountOnlyExposableStores() {
        List<ProductRepository.OriginSummaryRow> origins = productRepository.findPublicOriginsSummary(null, 0, 50);
        assertThat(origins).extracting(ProductRepository.OriginSummaryRow::getOrigin).containsExactly("台灣");
        assertThat(origins.get(0).getProductCount()).isEqualTo("2");

        List<ProductRepository.StoreProductCountRow> counts = productRepository.findPublicProductCountsByStore();
        assertThat(counts).hasSize(1);
        assertThat(counts.get(0).getStoreId()).isEqualTo(STORE_A);
        assertThat(counts.get(0).getProductCount()).isEqualTo(2L);
    }

    @Test
    @DisplayName("熱門 hashtag 只計可曝光店的 ACTIVE 商品")
    void popularHashtagsCountOnlyExposableStores() {
        long hashtagId = 9001L;
        jdbc.update("INSERT INTO hashtag (id, name) VALUES (?, '有機')", hashtagId);
        for (long productId : new long[] {A_ACTIVE_1, B_ACTIVE, C_ACTIVE}) {
            jdbc.update("INSERT INTO product_hashtag (product_id, hashtag_id, display_name) VALUES (?, ?, '有機')",
                    productId, hashtagId);
        }

        List<ProductHashtagRepository.PopularHashtagRow> rows = productHashtagRepository.selectPopularInRange(
                LocalDateTime.now().minusDays(1), LocalDateTime.now().plusDays(1), "4004", 10);

        assertThat(rows).hasSize(1);
        assertThat(rows.get(0).getCount()).isEqualTo("1");
    }

    @Test
    @DisplayName("brief 一次查詢帶回 storeExposable，Policy 對不可曝光或沒有列的店回 TEMPORARILY_UNAVAILABLE")
    void briefCarriesExposureIntoPolicy() {
        List<ProductPublicationBriefProjection> briefs = productRepository.findPublicationBriefByIdsIncludingDeleted(
                List.of(A_ACTIVE_1, B_ACTIVE, C_ACTIVE));
        Map<Long, ProductPublicationState> states = briefs.stream().collect(Collectors.toMap(
                ProductPublicationBriefProjection::id,
                b -> ProductPublicationPolicy.deriveState(
                        b.firstPublishedAt(), b.status(), b.deletedAt(), b.sellerDeletedAt(), b.storeExposable())));

        assertThat(states).containsEntry(A_ACTIVE_1, ProductPublicationState.PUBLIC)
                .containsEntry(B_ACTIVE, ProductPublicationState.TEMPORARILY_UNAVAILABLE)
                .containsEntry(C_ACTIVE, ProductPublicationState.TEMPORARILY_UNAVAILABLE);
    }

    @Test
    @DisplayName("公開詳情：不可曝光的店（含沒有列）回 TEMPORARILY_UNAVAILABLE 且不可購買；可曝光的店 PUBLIC 且可購買")
    void publicDetailReflectsExposure() {
        ProductService service = productService();

        ProductDetailDTO exposable = service.getPublicProduct(String.valueOf(A_ACTIVE_1));
        assertThat(exposable.getPublicationState()).isEqualTo(ProductPublicationState.PUBLIC.value());
        assertThat(exposable.getPurchasable()).isTrue();

        for (long hidden : new long[] {B_ACTIVE, C_ACTIVE}) {
            ProductDetailDTO dto = service.getPublicProduct(String.valueOf(hidden));
            assertThat(dto.getPublicationState()).isEqualTo(ProductPublicationState.TEMPORARILY_UNAVAILABLE.value());
            assertThat(dto.getPurchasable()).isFalse();
        }

        assertThatThrownBy(() -> service.getPublicProduct("999999"))
                .isInstanceOf(io.github.will413028.hawkr.product.exception.ProductPublicNotFoundException.class);
    }

    @Test
    @DisplayName("hashtag 自動完成：數量在讀取時只計可曝光店的 ACTIVE 商品，不用預先計算的 product_count")
    void hashtagAutocompleteCountsOnlyExposableStores() {
        jdbc.update("INSERT INTO hashtag (id, name) VALUES (9101, '有機蔬菜')");
        jdbc.update("INSERT INTO hashtag (id, name) VALUES (9102, '有機隱藏')");
        for (long productId : new long[] {A_ACTIVE_1, A_INACTIVE, B_ACTIVE, C_ACTIVE}) {
            jdbc.update("INSERT INTO product_hashtag (product_id, hashtag_id, display_name) VALUES (?, 9101, '有機蔬菜')",
                    productId);
        }
        // 9102 只掛在不可曝光的店：公開路徑不得出現
        for (long productId : new long[] {B_ACTIVE, C_ACTIVE}) {
            jdbc.update("INSERT INTO product_hashtag (product_id, hashtag_id, display_name) VALUES (?, 9102, '有機隱藏')",
                    productId);
        }

        for (List<io.github.will413028.hawkr.product.repository.HashtagRepository.HashtagSuggestion> rows : List.of(
                hashtagRepository.searchByPrefix("有機", 10), hashtagRepository.searchByContain("有機", 10))) {
            assertThat(rows).extracting(io.github.will413028.hawkr.product.repository.HashtagRepository.HashtagSuggestion::getName)
                    .containsExactly("有機蔬菜");
            assertThat(rows.get(0).getProductCount()).isEqualTo(1);
        }
    }

    @Test
    @DisplayName("商品詳情／列表的 hashtag 數量在讀取時計算：不含不可曝光店與非 ACTIVE 的商品，沒有符合商品時為 0")
    void hashtagItemCountExcludesNonExposableStores() {
        jdbc.update("INSERT INTO hashtag (id, name) VALUES (9201, '產地直送')");
        jdbc.update("INSERT INTO hashtag (id, name) VALUES (9202, '店家隱藏')");
        for (long productId : new long[] {A_ACTIVE_1, A_ACTIVE_2, A_INACTIVE, B_ACTIVE, C_ACTIVE}) {
            jdbc.update("INSERT INTO product_hashtag (product_id, hashtag_id, display_name) VALUES (?, 9201, '產地直送')",
                    productId);
        }
        jdbc.update("INSERT INTO product_hashtag (product_id, hashtag_id, display_name) VALUES (?, 9202, '店家隱藏')",
                B_ACTIVE);

        io.github.will413028.hawkr.product.service.HashtagService service =
                new io.github.will413028.hawkr.product.service.HashtagService(hashtagRepository, productHashtagRepository);

        // 可曝光店 A 的 ACTIVE 商品 2 個；B、C 不可曝光、A_INACTIVE 非 ACTIVE 都不計
        assertThat(service.findHashtagsByProductId(A_ACTIVE_1))
                .extracting(io.github.will413028.hawkr.product.service.HashtagService.HashtagDisplayItem::productCount)
                .containsExactly("2");
        assertThat(service.findHashtagsByProductId(B_ACTIVE))
                .extracting(io.github.will413028.hawkr.product.service.HashtagService.HashtagDisplayItem::canonicalName,
                        io.github.will413028.hawkr.product.service.HashtagService.HashtagDisplayItem::productCount)
                .containsExactlyInAnyOrder(
                        org.assertj.core.groups.Tuple.tuple("產地直送", "2"),
                        org.assertj.core.groups.Tuple.tuple("店家隱藏", "0"));
    }

    @Test
    @DisplayName("getProductsByIds：includeInactive=false 只回可曝光店的 ACTIVE 商品；true 不受影響")
    void getProductsByIdsHidesNonExposableStoresForBuyers() {
        ProductService service = productService();
        List<Long> all = List.of(A_ACTIVE_1, A_INACTIVE, B_ACTIVE, C_ACTIVE);

        assertThat(service.getProductsByIds(all, false))
                .extracting(io.github.will413028.hawkr.product.dto.response.ProductListItemDTO::getId)
                .containsExactly(String.valueOf(A_ACTIVE_1));
        assertThat(service.getProductsByIds(all, true)).hasSize(4);
    }

    private ProductService productService() {
        ProductEntityConverter converter = new ProductEntityConverter(
                Mockito.mock(ProductSkuConverter.class), new ObjectMapper());
        return new ProductService(
                productRepository,
                productSkuRepository,
                converter,
                Mockito.mock(ProductSkuConverter.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.IdempotencyService.class),
                Mockito.mock(io.github.will413028.hawkr.product.grpc.client.MediaGrpcClient.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.ProductCacheService.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.CategoryService.class),
                Mockito.mock(io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient.class),
                Mockito.mock(io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.HashtagService.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.ProductCascadeService.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.SellerInfoAssembler.class),
                Mockito.mock(io.github.will413028.hawkr.product.grpc.client.BuyerGrpcClient.class),
                Mockito.mock(io.github.will413028.hawkr.product.mq.ProductEventPublisher.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.ProductRevalidationService.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker.class),
                Mockito.mock(io.github.will413028.hawkr.product.service.ProductCertificationApplicationService.class),
                storeExposureRepository);
    }
}
