package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.AvailableOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ResolvedItem;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.SkuShippingOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.catchThrowableOfType;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * {@link UberOrderEligibilityGuard} 在<b>結帳鏈</b>上真的被接上了。
 *
 * <h2>為什麼這一支與 {@code UberOrderEligibilityGuardTest} 是兩件事</h2>
 * 那一支證明「閘門自己會拒絕」，本支證明「<b>有人去問它</b>」。
 * 兩者缺一，都會產生一個寫得很好但零呼叫端的 guard——
 * 而那正是 {@code can_accept_orders} 踩過的形狀（抽象做了卻全 repo 零消費，46 §11）。
 *
 * <h2>🔴 兩個接點各自擋不同的東西</h2>
 * <ul>
 *   <li>{@code validate}：Uber 不進 {@code availableOptions}（顯示層；買家看不到這個選項）</li>
 *   <li>{@code lookupSingleOption}：買家<b>指名</b> 3003 時，<b>拿不出報價 context 就拋 9503</b>
 *       （shippingFee 是這一格；preview／confirm 帶得出 context，改由 shipping 裁決）</li>
 * </ul>
 * 🔴 <b>只做前者的話</b>：買家在 App 上看不到 Uber，但直接打 API 帶 {@code shipping_method=3003}
 * 照樣能過 preview 與 confirm ⇒ 錢收了、shipment 建不出來。前者是 UX，後者才是防線。
 */
class UberCheckoutWiringTest {

    /** 硬寫字面值，不從 enum 取。 */
    private static final String UBER = "3003";
    private static final String TAQBIN = "3001";
    private static final String AMBIENT = "3201";

    private ShippingFeeCacheClient feeCache;
    private CheckoutValidator validator;

    @BeforeEach
    void setUp() {
        feeCache = mock(ShippingFeeCacheClient.class);
        // 對**每一種** method 都給得出 config 與費率：讓「Uber 被排除」只可能來自閘門，
        // 不可能來自「剛好查不到資料」——今天 repo 上 Uber 被跳過的真正原因正是後者
        // （shipping_method_configs 沒有 3003 列），若不 stub 成可用，本測試對閘門零鑑別力。
        when(feeCache.getConfig(anyString(), anyString()))
                .thenReturn(new ShippingFeeCacheClient.ConfigEntry(
                        "any", "any", 200, 150, 20000L, 2, 7));
        when(feeCache.isWithinLimit(any(), anyInt(), anyInt(), anyLong())).thenReturn(true);
        when(feeCache.lookupFee(anyString(), anyString(), anyString(), anyInt())).thenReturn(120L);

        validator = new CheckoutValidator(feeCache, new UberOrderEligibilityGuard());
    }

    private ResolvedItem itemSupporting(String... methodCodes) {
        List<SkuShippingOption> options = java.util.Arrays.stream(methodCodes)
                .map(m -> new SkuShippingOption(m, AMBIENT, 0L, "3101")) // BUYER 付運費，避開免運分支
                .toList();
        return new ResolvedItem(
                1L, "sku-1", 1, 500L, 500L, 99L,
                10, 10, 10, 1000L, 2683382637L, "p-1", "product-1",
                options, "img", true);
    }

    // ===== 接點 A：validate 的選項列表（顯示層）=====

    @Test
    @DisplayName("🔴 validate：3003 不進 availableOptions（即使 config 與費率都查得到）")
    void validateExcludesUberFromAvailableOptions() {
        ValidationResult result = validator.validate(
                List.of(itemSupporting(UBER, TAQBIN)), ShippingRoute.DOMESTIC);

        assertThat(result.valid()).isTrue();
        assertThat(result.availableOptions())
                .extracting(o -> o.method().value())
                .doesNotContain(UBER);
    }

    @Test
    @DisplayName("🔴 陰性：3001 仍在 availableOptions——閘門不得誤傷其他配送方式")
    void validateKeepsNonUberOptions() {
        ValidationResult result = validator.validate(
                List.of(itemSupporting(UBER, TAQBIN)), ShippingRoute.DOMESTIC);

        assertThat(result.availableOptions())
                .extracting(o -> o.method().value())
                .contains(TAQBIN);
    }

    @Test
    @DisplayName("只支援 3003 的商品：availableOptions 空，回 ALL_SHIPPING_EXCEEDED 而不是 500")
    void validateWithOnlyUberYieldsNoOptions() {
        ValidationResult result = validator.validate(
                List.of(itemSupporting(UBER)), ShippingRoute.DOMESTIC);

        assertThat(result.valid()).isFalse();
        assertThat(result.availableOptions()).isEmpty();
        assertThat(result.errors())
                .extracting(CheckoutValidator.ValidationError::errorCode)
                .contains("ALL_SHIPPING_EXCEEDED");
    }

    // ===== 接點 B：lookupSingleOption（買家指名該方式）=====

    private static UberOrderEligibilityGuard.Context methodOnly(ShippingMethod method) {
        return UberOrderEligibilityGuard.Context.methodOnly(method.value());
    }

    @Test
    @DisplayName("🔴 lookupSingleOption(3003) 且無報價 context：拋 9503，而不是回 null 讓 caller 塌縮成 4008")
    void lookupSingleOptionRejectsUberWithoutProbe() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> validator.lookupSingleOption(
                        List.of(itemSupporting(UBER)),
                        ShippingMethod.UBER_INSTANT, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                        methodOnly(ShippingMethod.UBER_INSTANT)),
                OrderBusinessException.class);

        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9503);
    }

    @Test
    @DisplayName("🔴 lookupSingleOption(3003) 帶報價 context 且 shipping 判定可用：回報價運費，**不查費率表**")
    void lookupSingleOptionUsesQuoteFeeForUber() {
        // 費率表 stub 的是 120——若 Uber 誤走費率表，下面的 250 就會變成 120。
        CheckoutValidator withQuote = new CheckoutValidator(
                feeCache, new UberOrderEligibilityGuard(
                        probe -> UberOrderEligibilityGuard.Assessment.eligible(
                                "dqt_wired", "250", "TWD", "2026-09-12T05:00:00Z")));

        AvailableOption option = withQuote.lookupSingleOption(
                List.of(itemSupporting(UBER)),
                ShippingMethod.UBER_INSTANT, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                UberOrderEligibilityGuard.Context.of(UBER, uberProbe("")));

        assertThat(option).isNotNull();
        assertThat(option.method().value()).isEqualTo(UBER);
        assertThat(option.rawFee()).isEqualTo(250L);
        assertThat(option.fee()).isEqualTo(250L);
        assertThat(option.freeShippingEligible()).isFalse();
        // confirm 要拿它去 CAS 成 CHECKOUT_ACCEPTED；空字串＝建單時找不到報價。
        assertThat(option.uberQuoteId()).isEqualTo("dqt_wired");
    }

    @Test
    @DisplayName("🔴 帶報價 context 但 shipping 判定不可用：仍是 9503，不得掉回費率表出價")
    void lookupSingleOptionStillRejectsWhenProviderSaysNo() {
        CheckoutValidator withQuote = new CheckoutValidator(
                feeCache, new UberOrderEligibilityGuard(
                        probe -> UberOrderEligibilityGuard.Assessment.unavailable(
                                UberOrderEligibilityGuard.ProviderReason.ADDRESS_UNPARSEABLE)));

        OrderBusinessException ex = catchThrowableOfType(
                () -> withQuote.lookupSingleOption(
                        List.of(itemSupporting(UBER)),
                        ShippingMethod.UBER_INSTANT, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                        UberOrderEligibilityGuard.Context.of(UBER, uberProbe(""))),
                OrderBusinessException.class);

        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9503);
    }

    @Test
    @DisplayName("🔴 context 的配送方式與本次查找不一致 ⇒ 直接拒絕，不得拿 A 的 context 放行 B")
    void lookupSingleOptionRejectsMismatchedContext() {
        assertThatCode(() -> validator.lookupSingleOption(
                List.of(itemSupporting(UBER)),
                ShippingMethod.UBER_INSTANT, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                methodOnly(ShippingMethod.TAQBIN)))
                .isInstanceOf(IllegalArgumentException.class);
    }

    @Test
    @DisplayName("🔴 陰性：lookupSingleOption(3001) 正常回傳運費選項，未被閘門誤擋")
    void lookupSingleOptionAllowsNonUber() {
        AvailableOption option = validator.lookupSingleOption(
                List.of(itemSupporting(TAQBIN)),
                ShippingMethod.TAQBIN, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                methodOnly(ShippingMethod.TAQBIN));

        assertThat(option).isNotNull();
        assertThat(option.method().value()).isEqualTo(TAQBIN);
        assertThat(option.rawFee()).isEqualTo(120L);
        assertThat(option.uberQuoteId()).isEmpty();
    }

    @Test
    @DisplayName("🔴 陰性：其餘三個平台物流碼（3002／3004／3005）都不被閘門擋")
    void lookupSingleOptionAllowsEveryOtherMethod() {
        for (ShippingMethod method : ShippingMethod.values()) {
            if (ShippingMethod.UBER_INSTANT == method) {
                continue;
            }
            assertThatCode(() -> validator.lookupSingleOption(
                    List.of(itemSupporting(method.value())),
                    method, ShippingTemperature.AMBIENT, ShippingRoute.DOMESTIC,
                    methodOnly(method)))
                    .as("配送方式 %s 不應被 Uber 閘門攔截", method.value())
                    .doesNotThrowAnyException();
        }
    }

    private static UberOrderEligibilityGuard.Probe uberProbe(String quoteId) {
        return UberOrderEligibilityGuard.Probe.forCheckout(
                1L, 2683382637L, AMBIENT, "王小明", "0912345678",
                "臺北市", "大安區", "測試路1號",
                List.of(new UberOrderEligibilityGuard.Item(
                        1L, "sku-1", 1, 10, 10, 10, 1000L, 500L)),
                quoteId);
    }
}
