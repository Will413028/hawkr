package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.order.TestIds;
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

    private static final String METHOD = "3001";
    private static final String TEMP = "3201";
    private static final String SELLER = ShippingFeePayer.SELLER.value();
    private static final String BUYER = ShippingFeePayer.BUYER.value();

    private final CheckoutValidator validator = new CheckoutValidator(null, new UberOrderEligibilityGuard());

    private ResolvedItem item(long skuId, long unitPrice, int qty, long freeThreshold, String feePayer) {
        return new ResolvedItem(
                TestIds.id(skuId), "sku-" + skuId, qty, unitPrice, unitPrice, 999L,
                10, 10, 10, 1000L, 2683382637L, TestIds.id(skuId + 1000), "product-" + skuId,
                List.of(new SkuShippingOption(METHOD, TEMP, freeThreshold, feePayer)),
                "img", true);
    }


    @Test
    void sellerPaysWithoutThreshold_unconditionallyFree() {
        var items = List.of(item(1L, 50, 1, 0L, SELLER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
        assertThat(fsr.feePayer()).isEqualTo(SELLER);
        assertThat(fsr.lowestThreshold()).isZero();
    }

    @Test
    void sellerPaysWithoutThreshold_stillFreeAfterFullCoupon() {
        var items = List.of(item(1L, 50, 1, 0L, SELLER));
        Map<java.util.UUID, Long> discount = Map.of(TestIds.id(1L), 50L);
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, discount);
        assertThat(fsr.eligible()).isTrue();
    }


    @Test
    void sellerPaysWithThreshold_eligibleWhenSubtotalMeetsThreshold() {
        var items = List.of(item(1L, 1500, 1, 1000L, SELLER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
        assertThat(fsr.lowestThreshold()).isEqualTo(1000L);
    }

    @Test
    void sellerPaysWithThreshold_flipsOffAfterDiscountBelowThreshold() {
        var items = List.of(item(1L, 1500, 1, 1000L, SELLER));
        Map<java.util.UUID, Long> discount = Map.of(TestIds.id(1L), 600L);
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, discount);
        assertThat(fsr.eligible()).isFalse();
    }


    @Test
    void buyerPays_neverFree() {
        var items = List.of(item(1L, 50, 1, 0L, BUYER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isFalse();
        assertThat(fsr.feePayer()).isEqualTo(BUYER);
    }


    @Test
    void mixedItems_anyUnconditionalSellerMakesWholeOrderFree() {
        var items = List.of(
                item(1L, 50, 1, 0L, SELLER),
                item(2L, 30, 1, 0L, BUYER));
        FreeShippingResult fsr = validator.checkFreeShipping(items, METHOD, TEMP, null);
        assertThat(fsr.eligible()).isTrue();
    }
}
