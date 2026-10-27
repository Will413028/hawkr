package io.github.will413028.hawkr.product.service;

import com.baomidou.mybatisplus.annotation.FieldStrategy;
import com.baomidou.mybatisplus.annotation.TableField;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.junit.jupiter.MockitoExtension;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * 商品沒有任何 SKU 時，六個價格冗餘欄位必須真的被清成 NULL（#2546）。
 *
 * <p>缺陷形狀：{@code updatePriceRange} 對空集合已經 {@code setPriceMin(null)}，但
 * MyBatis-Plus 預設的 {@code NOT_NULL} 更新策略會把 null 欄位整個排除在 UPDATE 的 SET 之外
 * ⇒ 舊價格原封不動留在 DB。對照組是同一批算出來的 {@code stock_qty}：
 * {@code aggregateStock(∅)} 回原生 {@code long} 0，**寫得進去**。
 * 於是同一個「這個商品沒有 SKU 了」的事實，在庫存上生效、在價格上失效 ——
 * 商品對外顯示「售完」卻仍掛著舊價格。
 *
 * <p>所以本測試有兩個錨，缺一不可：
 * <ul>
 *   <li>{@link #emptySkuSetClearsAllSixPriceFields()} — 算得出 null</li>
 *   <li>{@link #allSixPriceFieldsCarryIgnoredUpdateStrategy()} — null 寫得進 DB</li>
 * </ul>
 * 只驗前者會在「有人移除 {@code @TableField(updateStrategy = IGNORED)}」時保持綠燈。
 */
@ExtendWith(MockitoExtension.class)
@org.mockito.junit.jupiter.MockitoSettings(strictness = org.mockito.quality.Strictness.LENIENT)
class ProductServicePriceRangeClearTest {

    @InjectMocks ProductService productService;

    /** updatePriceRange 是純計算的 private method，不碰任何依賴。 */
    private void invoke(ProductEntity entity, List<ProductSkuEntity> skus) throws Exception {
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

    /** 先塞入「舊價格」，模擬 DB 讀出來、SKU 已全數移除的 entity。 */
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

    @Test
    @DisplayName("SKU 集合為空 → 六個價格欄位全部算成 null（舊價格不得留在 entity 上）")
    void emptySkuSetClearsAllSixPriceFields() throws Exception {
        ProductEntity entity = entityWithStalePrices();

        invoke(entity, List.of());

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

        invoke(entity, null);

        assertNull(entity.getPriceMin());
        assertNull(entity.getListPriceMin());
        assertNull(entity.getSalePriceMin());
    }

    @Test
    @DisplayName("六個價格欄位都必須帶 updateStrategy = IGNORED，null 才寫得進 DB（#2546 真正的修復點）")
    void allSixPriceFieldsCarryIgnoredUpdateStrategy() throws Exception {
        for (String name : List.of("priceMin", "priceMax",
                "listPriceMin", "listPriceMax", "salePriceMin", "salePriceMax")) {
            Field f = ProductEntity.class.getDeclaredField(name);
            TableField ann = f.getAnnotation(TableField.class);
            assertNotNull(ann, name + " 缺少 @TableField，會落回預設 NOT_NULL 策略 ⇒ null 不寫進 DB");
            assertEquals(FieldStrategy.IGNORED, ann.updateStrategy(),
                    name + " 的 updateStrategy 必須是 IGNORED；"
                            + "預設 NOT_NULL 會讓 updatePriceRange 算出的 null 被整個排除在 UPDATE 之外，"
                            + "0 SKU 的商品會繼續對外宣稱舊價格（#2546）");
        }
    }

    @Test
    @DisplayName("控制組：SKU 集合非空時照常算出區間（證明上面的 assertNull 不是 vacuously true）")
    void nonEmptySkuSetStillComputesRange() throws Exception {
        ProductEntity entity = new ProductEntity();

        // effective = salePrice ?? price → {80, 300}；list = price → {100, 300}；sale = {80}
        invoke(entity, List.of(sku(100L, 80L), sku(300L, null)));

        assertEquals(80L, entity.getPriceMin());
        assertEquals(300L, entity.getPriceMax());
        assertEquals(100L, entity.getListPriceMin());
        assertEquals(300L, entity.getListPriceMax());
        assertEquals(80L, entity.getSalePriceMin());
        assertEquals(80L, entity.getSalePriceMax());
    }
}
