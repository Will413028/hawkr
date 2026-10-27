package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.FreeShippingResult;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ResolvedItem;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.SkuShippingOption;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 免運判定單元測試（issue #1147）。
 * 對應設計書 20 §4.7：無條件免運（§4.7.0-A）+ 門檻免運折扣後判定（§4.7.0-B）。
 *
 * checkFreeShipping 不使用 ShippingFeeCacheClient，故以 null 建構即可。
 */
class CheckoutValidatorFreeShippingTest {

    private static final String METHOD = "3001";   // TAQBIN value
    private static final String TEMP = "3201";     // AMBIENT value
    private static final String SELLER = ShippingFeePayer.SELLER.value(); // "3102"
    private static final String BUYER = ShippingFeePayer.BUYER.value();   // "3101"

    private final CheckoutValidator validator = new CheckoutValidator(null, new UberOrderEligibilityGuard());

    private ResolvedItem item(long skuId, long unitPrice, int qty, long freeThreshold, String feePayer) {
        return new ResolvedItem(
                skuId, "sku-" + skuId, qty, unitPrice, unitPrice, 999L,
                10, 10, 10, 1000L, 2683382637L, "p-" + skuId, "product-" + skuId,
                List.of(new SkuShippingOption(METHOD, TEMP, freeThreshold, feePayer)),
                "img", true);
    }

    // ===== §4.7.0-A 無條件免運 =====

    @Test
    void sellerPaysWithoutThreshold_unconditionallyFree() {
        // feePayer=SELLER + freeThreshold=0 → 無條件免運（tentative，無折扣）
        var items = List.of(item(1L, 50, 1, 0L, SELLER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
        assertThat(fsr.feePayer()).isEqualTo(SELLER);
        assertThat(fsr.lowestThreshold()).isZero();
    }

    @Test
    void sellerPaysWithoutThreshold_stillFreeAfterFullCoupon() {
        // feePayer=SELLER + freeThreshold=0 + 全額券折到 0 → 仍免運（final，折扣後不翻轉）
        var items = List.of(item(1L, 50, 1, 0L, SELLER));
        Map<Long, Long> discount = Map.of(1L, 50L); // 商品折到 0
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, discount);
        assertThat(fsr.eligible()).isTrue(); // issue #1147 核心：0 元訂單路徑不被運費阻斷
    }

    // ===== §4.7.0-B 門檻免運（折扣後判定，維持不變）=====

    @Test
    void sellerPaysWithThreshold_eligibleWhenSubtotalMeetsThreshold() {
        var items = List.of(item(1L, 1500, 1, 1000L, SELLER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
        assertThat(fsr.lowestThreshold()).isEqualTo(1000L);
    }

    @Test
    void sellerPaysWithThreshold_flipsOffAfterDiscountBelowThreshold() {
        // 1500 − 600 = 900 < 1000 → 折扣後失去免運（§4.7.0-B 維持）
        var items = List.of(item(1L, 1500, 1, 1000L, SELLER));
        Map<Long, Long> discount = Map.of(1L, 600L);
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, discount);
        assertThat(fsr.eligible()).isFalse();
    }

    // ===== feePayer=BUYER 不免運 =====

    @Test
    void buyerPays_neverFree() {
        var items = List.of(item(1L, 50, 1, 0L, BUYER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isFalse();
        assertThat(fsr.feePayer()).isEqualTo(BUYER);
    }

    // ===== 多商品：任一無條件免運 → 整筆免運 =====

    @Test
    void mixedItems_anyUnconditionalSellerMakesWholeOrderFree() {
        var items = List.of(
                item(1L, 50, 1, 0L, SELLER),      // 無條件免運
                item(2L, 30, 1, 0L, BUYER));      // 買家負擔
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
    }
}
