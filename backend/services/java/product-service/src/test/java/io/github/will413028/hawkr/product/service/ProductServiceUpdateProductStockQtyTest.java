package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
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

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.Mockito.when;

/**
 * UpdateProduct 對 {@code product.stock_qty} 的寫入條件。
 *
 * <p>{@code product.stock_qty} 是 SKU 的反正規化加總，真實來源在 {@code product_sku}。它有兩條
 * 寫入者，而兩條看不見彼此：
 * <ul>
 *   <li>庫存異動（deductStock / restoreStock）走 {@code reaggregateStockFromSku}，該 SQL
 *       <b>刻意不 bump {@code product.version}</b>（56 §5.4 F41 第 11 列，登記在
 *       {@code scripts/check-version-bump-coverage.py} 的豁免清單裡）——反正規化計數欄若參與
 *       樂觀鎖，代行編輯在高流量商品上幾乎必然 409。</li>
 *   <li>UpdateProduct 走 {@code updateById}，version CAS 由攔截器加述詞。</li>
 * </ul>
 * 於是 UpdateProduct 的 CAS 對「表單送出期間發生的庫存異動」<b>完全失明</b>：交易開頭讀到的
 * SKU 快照加總會在步驟 6 原樣蓋回去，而 CAS 照樣 rows=1。
 *
 * <p>差別在請求有沒有帶 skus：
 * <ul>
 *   <li><b>帶了</b> ⇒ 賣家正在改 SKU，{@code updateSkus} 逐筆 CAS（#2545）過，加總是本次意圖的
 *       一部分，必須寫。</li>
 *   <li><b>沒帶</b>（改名稱 / 描述 / 分類的純商品編輯）⇒ {@code updateSkus} 是 no-op，這條路根本
 *       不打算改庫存，卻會把舊快照寫回去。正解是讓該欄不進 UPDATE。</li>
 * </ul>
 *
 * <p>危害不是超賣——結帳的 {@code deductStock} 帶 {@code stock_qty >= quantity} 述詞，SKU 才是
 * 真實來源。壞的是 {@code ProductPublicationPolicy.isPurchasable} 的判準之一被寫回舊值：商品
 * 必須是 DRAFT / INACTIVE 才編輯得動（§2.8.5），所以被覆寫的通常是編輯期間的退款歸還庫存，
 * 賣家重新上架後商品有貨卻顯示售完，直到下一次任何庫存異動把它重算回來。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceUpdateProductStockQtyTest {

    @Mock ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductEntityConverter productEntityConverter;
    @Mock ProductSkuConverter productSkuConverter;
    @Mock IdempotencyService idempotencyService;
    @Mock ProductCacheService cacheService;
    @Mock CategoryService categoryService;
    @Mock HashtagService hashtagService;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock ProductMediaReferenceLinker mediaReferenceLinker;
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductService productService;

    private static final long PID = 1001L;
    private static final long STORE = 2685914682L;
    /** 交易開頭 findByProductId 讀到的 SKU 庫存（＝可能已經過期的那個快照）。 */
    private static final int STOCK_IN_DB = 30;
    /** 賣家在請求裡送來的 SKU 庫存（只有「有帶 skus」的路徑才會用到）。 */
    private static final int STOCK_IN_REQUEST = 7;

    /**
     * updateById 當下 entity 帶的 stock_qty。
     *
     * <p>不能用 {@code ArgumentCaptor}：它捕捉的是<b>參考</b>，而 updateProduct 在落盤之後會把
     * 同一個 entity 的 stockQty 補回去供回應 DTO 使用 ⇒ 事後讀 captor 只會讀到補回後的值，
     * 「有沒有進 UPDATE」這件事就分辨不出來了。要驗送去 DB 的是什麼，只能在 stub 內當場記下來。
     */
    private final List<Long> stockSeenByUpdate = new ArrayList<>();

    private ProductEntity persistedProduct() {
        ProductEntity e = new ProductEntity();
        e.setId(PID);
        e.setStoreId(STORE);
        e.setStatus(ProductStatus.INACTIVE.value());
        e.setName("有機小松菜");
        // 三層分類必須有值：enrichSearchableFields 會拿它們查 categoryService 的名稱快照
        e.setCategoryLv1(11L);
        e.setCategoryLv2(12L);
        e.setCategory(13L);
        e.setStockQty((long) STOCK_IN_DB);
        e.setVersion(3L);
        return e;
    }

    private ProductSkuEntity persistedSku() {
        ProductSkuEntity s = new ProductSkuEntity();
        s.setId(101L);
        s.setProductId(PID);
        s.setSkuName("300g");
        s.setPrice(100L);
        s.setStockQty(STOCK_IN_DB);
        s.setVersion(5L);
        return s;
    }

    private void wireHappyPath() {
        when(productRepository.findByIdAndStoreId(PID, STORE))
                .thenReturn(Optional.of(persistedProduct()));
        when(productSkuRepository.findByProductId(PID))
                .thenReturn(new ArrayList<>(List.of(persistedSku())));
        when(productSkuRepository.updateById(any())).thenReturn(1);
        when(productEntityConverter.toDetailDTO(any(), any(), anyBoolean())).thenReturn(new ProductDetailDTO());
        when(categoryService.getCategoryNames(any()))
                .thenReturn(new java.util.HashMap<>(java.util.Map.of(11L, "蔬菜", 12L, "葉菜", 13L, "小松菜")));
        when(productRepository.updateById(any())).thenAnswer(inv -> {
            stockSeenByUpdate.add(((ProductEntity) inv.getArgument(0)).getStockQty());
            return 1;
        });
        // converter 是 mock（預設 no-op），但「帶 skus」那條路要真的把請求庫存套進 entity，
        // 否則兩條路徑的加總都會是 DB 快照值，測試分辨不出差別。
        org.mockito.Mockito.doAnswer(inv -> {
            ProductSkuEntity target = inv.getArgument(0);
            ProductSkuDTO source = inv.getArgument(1);
            if (source.getStockQty() != null) {
                target.setStockQty(source.getStockQty());
            }
            return null;
        }).when(productSkuConverter).updateEntity(any(), any());
    }

    private UpdateProductDTO dto(List<ProductSkuDTO> skus) {
        UpdateProductDTO dto = new UpdateProductDTO();
        dto.setIdempotencyKey("idem-" + System.nanoTime());
        dto.setProductId(String.valueOf(PID));
        dto.setStoreId(STORE);
        dto.setName("有機小松菜（改名）");
        dto.setSkus(skus);
        return dto;
    }

    /** 帶完整必填欄位的 SKU（重量與三個材積是必填且必須 > 0，見 requirePositive）。 */
    private ProductSkuDTO requestSku() {
        return ProductSkuDTO.builder()
                .id("101")
                .skuName("300g")
                .price(100L)
                .stockQty(STOCK_IN_REQUEST)
                .weight(BigDecimal.ONE)
                .lengthCm(BigDecimal.ONE)
                .widthCm(BigDecimal.ONE)
                .heightCm(BigDecimal.ONE)
                .build();
    }

    @Test
    @DisplayName("純商品編輯（未帶 skus）→ stock_qty 不得進 UPDATE，否則會蓋掉編輯期間的庫存異動")
    void productOnlyEditDoesNotWriteStockQty() {
        wireHappyPath();

        productService.updateProduct(dto(null));

        assertEquals(1, stockSeenByUpdate.size(), "應該只有一次 product updateById");
        assertNull(stockSeenByUpdate.get(0),
                "未帶 skus 的請求根本不打算改庫存，卻把交易開頭讀到的 SKU 快照加總寫了回去。"
                        + "庫存異動走 reaggregateStockFromSku 且刻意不 bump product.version"
                        + "（F41 第 11 列）⇒ 本次的 version CAS 對那筆異動完全失明、照樣 rows=1 ⇒ "
                        + "異動被靜默覆寫。stockQty 沒有 @TableField，設成 null 才會被 MyBatis-Plus "
                        + "預設的 NOT_NULL 策略排除出 UPDATE");
    }

    @Test
    @DisplayName("純商品編輯的回應仍要帶既有庫存（不得因為不寫 DB 就把回應裡的 stock 變成 null）")
    void productOnlyEditStillReportsStockInResponse() {
        wireHappyPath();

        productService.updateProduct(dto(null));

        // 落盤後補回：DB 沒動，但回應 DTO 讀的是 entity，不補回前端會看到「庫存消失」
        org.mockito.ArgumentCaptor<ProductEntity> forDto =
                org.mockito.ArgumentCaptor.forClass(ProductEntity.class);
        org.mockito.Mockito.verify(productEntityConverter).toDetailDTO(forDto.capture(), any(), anyBoolean());
        assertEquals((long) STOCK_IN_DB, forDto.getValue().getStockQty(),
                "回應用的 entity 在 updateById 之後必須把加總補回來");
    }

    @Test
    @DisplayName("有帶 skus → stock_qty 照常寫入（本次修改不得把 SKU 編輯路徑的庫存維護一起關掉）")
    void skuEditStillWritesAggregatedStockQty() {
        wireHappyPath();

        productService.updateProduct(dto(List.of(requestSku())));

        assertEquals(1, stockSeenByUpdate.size(), "應該只有一次 product updateById");
        assertEquals((long) STOCK_IN_REQUEST, stockSeenByUpdate.get(0),
                "賣家確實送了 SKU（且已逐筆過 #2545 的 SKU 層 CAS），加總是本次意圖的一部分，必須落盤；"
                        + "寫成 " + STOCK_IN_DB + " 表示讀到的是 DB 快照而非請求值，寫成 null 表示整條 "
                        + "cascade 被關掉");
    }
}
