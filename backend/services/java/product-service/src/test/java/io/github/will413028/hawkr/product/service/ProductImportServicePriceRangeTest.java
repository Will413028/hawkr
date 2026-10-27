package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.junit.jupiter.MockitoExtension;

import java.lang.reflect.Method;
import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * Import 轉正路徑的價格區間彙總（{@code ProductImportService.updatePriceRange}）。
 *
 * <p>它與 {@link ProductService} 那支是**兩份各自獨立的實作**（02 §9 要求 import 與既有
 * CreateProduct 嚴格分離、不共用 RPC、不走 flag 分流），但寫的是同一張 product 表的同六個欄位。
 * 兩份都對空 SKU 集合 {@code setXxx(null)}，而只有 {@link ProductServicePriceRangeClearTest}
 * 那份有測試 ⇒ import 這份的空集合分支可以整段消失而編譯過、全套測試綠。
 *
 * <p>所以本測有兩個錨：
 * <ul>
 *   <li>{@link #emptySkuSetClearsAllSixPriceFields()} — 這份自己的空集合分支還在</li>
 *   <li>{@link #bothImplementationsAgreeOnTheSameSkuSet()} — 兩份沒有分岔</li>
 * </ul>
 * null 寫不寫得進 DB 由 {@link ProductEntity} 上的 {@code updateStrategy = IGNORED} 決定，
 * 那是兩條路徑共用的一份設定，已由 {@link ProductServicePriceRangeClearTest} 守，此處不重複。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductImportServicePriceRangeTest {

    /** updatePriceRange 是純計算的 private method，不碰任何依賴。 */
    @InjectMocks ProductImportService productImportService;
    @InjectMocks ProductService productService;

    private void invokeImport(ProductEntity entity, List<ProductSkuEntity> skus) throws Exception {
        Method m = ProductImportService.class.getDeclaredMethod(
                "updatePriceRange", ProductEntity.class, List.class);
        m.setAccessible(true);
        m.invoke(productImportService, entity, skus);
    }

    private void invokeSelfServe(ProductEntity entity, List<ProductSkuEntity> skus) throws Exception {
        Method m = ProductService.class.getDeclaredMethod(
                "updatePriceRange", ProductEntity.class, List.class);
        m.setAccessible(true);
        m.invoke(productService, entity, skus);
    }

    private ProductSkuEntity sku(Long price, Long salePrice) {
        ProductSkuEntity e = new ProductSkuEntity();
        e.setPrice(price);
        e.setSalePrice(salePrice);
        return e;
    }

    /** 先塞入「舊價格」，模擬 import 覆寫既有商品時 entity 上殘留的前一版價格。 */
    private ProductEntity entityWithStalePrices() {
        ProductEntity e = new ProductEntity();
        e.setPriceMin(100L);
        e.setPriceMax(500L);
        e.setListPriceMin(120L);
        e.setListPriceMax(600L);
        e.setSalePriceMin(100L);
        e.setSalePriceMax(500L);
        return e;
    }

    private List<Long> sixFields(ProductEntity e) {
        return Arrays.asList(e.getPriceMin(), e.getPriceMax(),
                e.getListPriceMin(), e.getListPriceMax(),
                e.getSalePriceMin(), e.getSalePriceMax());
    }

    @Test
    @DisplayName("SKU 集合為空 → 六個價格欄位全部算成 null（舊價格不得留在 entity 上）")
    void emptySkuSetClearsAllSixPriceFields() throws Exception {
        ProductEntity entity = entityWithStalePrices();

        invokeImport(entity, List.of());

        assertNull(entity.getPriceMin(), "priceMin");
        assertNull(entity.getPriceMax(), "priceMax");
        assertNull(entity.getListPriceMin(), "listPriceMin");
        assertNull(entity.getListPriceMax(), "listPriceMax");
        assertNull(entity.getSalePriceMin(), "salePriceMin");
        assertNull(entity.getSalePriceMax(), "salePriceMax");
    }

    @Test
    @DisplayName("SKU 集合為 null（同樣是「沒有 SKU」）→ 六欄同樣清空")
    void nullSkuSetClearsAllSixPriceFields() throws Exception {
        ProductEntity entity = entityWithStalePrices();

        invokeImport(entity, null);

        assertNull(entity.getPriceMin());
        assertNull(entity.getListPriceMin());
        assertNull(entity.getSalePriceMin());
    }

    @Test
    @DisplayName("控制組：SKU 集合非空時照常算出區間（證明上面的 assertNull 不是 vacuously true）")
    void nonEmptySkuSetStillComputesRange() throws Exception {
        ProductEntity entity = new ProductEntity();

        // effective = salePrice ?? price → {80, 300}；list = price → {100, 300}；sale = {80}
        invokeImport(entity, List.of(sku(100L, 80L), sku(300L, null)));

        assertEquals(80L, entity.getPriceMin());
        assertEquals(300L, entity.getPriceMax());
        assertEquals(100L, entity.getListPriceMin());
        assertEquals(300L, entity.getListPriceMax());
        assertEquals(80L, entity.getSalePriceMin());
        assertEquals(80L, entity.getSalePriceMax());
    }

    @Test
    @DisplayName("兩份獨立實作對同一組 SKU 必須算出完全相同的六欄，否則同一商品的價格會隨建立路徑而異")
    void bothImplementationsAgreeOnTheSameSkuSet() throws Exception {
        List<List<ProductSkuEntity>> cases = List.of(
                List.of(),                                          // 無 SKU
                List.of(sku(100L, 80L), sku(300L, null)),           // 部分有特價
                List.of(sku(200L, null), sku(150L, null)),          // 全無特價
                List.of(sku(500L, 450L), sku(500L, 400L)));         // 全有特價

        for (List<ProductSkuEntity> skus : cases) {
            ProductEntity viaImport = entityWithStalePrices();
            ProductEntity viaSelfServe = entityWithStalePrices();

            invokeImport(viaImport, skus);
            invokeSelfServe(viaSelfServe, skus);

            assertEquals(sixFields(viaSelfServe), sixFields(viaImport),
                    "import 路徑與賣家自助路徑對 " + skus.size() + " 個 SKU 算出不同的價格區間；"
                            + "兩份是獨立實作、寫的卻是同一張表的同六個欄位，分岔後同一個商品的售價"
                            + "會取決於它是被匯入的還是手動建立的");
        }
    }
}
