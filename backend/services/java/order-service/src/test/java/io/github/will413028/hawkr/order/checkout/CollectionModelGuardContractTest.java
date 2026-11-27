package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.FulfillmentMode;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceModel;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 收款模式守門的契約守護：只有賣家收款（SELLER_MERCHANT）放行，其餘 fail-closed。
 *
 * <p>沒有它，下面這個失效在資料上完全看不出來：{@code orders.collection_model} 被寫成
 * 一個沒有收款路徑的值，訂單、付款、帳本三邊卻都正常。本測試守的是決策點
 * （{@code CheckoutService.requireSupportedCollectionModel}）。
 */
@DisplayName("收款模式守門的政策契約")
class CollectionModelGuardContractTest {

    /**
     * 直接呼叫 private 的守門方法。
     *
     * <p>用反射而不是把方法改成 package-private：可見性是 production 的設計決定，
     * 不該為了測試而放寬。找不到方法時<b>丟 AssertionError 不是 skip</b>——
     * 方法被改名的那天，skip 會讓這道守護靜默消失。
     */
    private static void invokeGuard(OrderPolicySnapshot policy) throws Exception {
        CheckoutService service = newServiceWithoutDependencies();

        Method m;
        try {
            m = CheckoutService.class.getDeclaredMethod(
                    "requireSupportedCollectionModel", OrderPolicySnapshot.class);
        } catch (NoSuchMethodException e) {
            throw new AssertionError(
                    "找不到 requireSupportedCollectionModel(OrderPolicySnapshot)——"
                            + "它被改名、改簽章或搬走了？本測試對『目標不存在』一律 FAIL，"
                            + "因為 skip 會讓收款模式的守護在目標消失的那一刻靜默失效", e);
        }
        m.setAccessible(true);
        try {
            m.invoke(service, policy);
        } catch (InvocationTargetException e) {
            if (e.getCause() instanceof RuntimeException re) {
                throw re;
            }
            throw e;
        }
    }

    /** 只為了呼叫一個純決策方法，不需要任何依賴——全部給 null。 */
    private static CheckoutService newServiceWithoutDependencies() throws Exception {
        var ctor = CheckoutService.class.getDeclaredConstructors()[0];
        ctor.setAccessible(true);
        Object[] args = new Object[ctor.getParameterCount()];
        return (CheckoutService) ctor.newInstance(args);
    }

    private static OrderPolicySnapshot policyWith(String collectionModel) {
        return new OrderPolicySnapshot(
                CommerceModel.SELLER_DIRECT.value(), collectionModel,
                FulfillmentMode.PLATFORM_LOGISTICS.value(), InvoiceModel.SELLER_TO_BUYER.value(),
                OrderPolicyResolver.POLICY_VERSION);
    }

    @Nested
    @DisplayName("collection_model → 是否放行")
    class CollectionModelGuard {

        @Test
        @DisplayName("賣家收款 → 放行（可否接單由 can_accept_orders 擋）")
        void sellerMerchantPasses() {
            assertThatCode(() -> invokeGuard(policyWith(CollectionModel.SELLER_MERCHANT.value())))
                    .doesNotThrowAnyException();
        }

        @Test
        @DisplayName("平台代收沒有收款路徑 → 結帳當下 fail-closed")
        void platformCollectionFailsClosed() {
            assertThatThrownBy(() -> invokeGuard(policyWith(CollectionModel.PLATFORM_AGENT_MERCHANT.value())))
                    .isInstanceOf(OrderBusinessException.class)
                    .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                            .as("必須回 PLATFORM_COLLECTION_NOT_CONFIGURED，而不是通用參數錯誤——"
                                    + "維運看到的碼決定他去查哪裡")
                            .isEqualTo(GrpcResponseCode.PLATFORM_COLLECTION_NOT_CONFIGURED.code()));
        }

        /**
         * {@code PROVIDER_COLLECTED} 與 {@code EXTERNAL_PREPAID} 是
         * {@code chk_orders_collection_model} 的合法值，但尚無寫入路徑；
         * 它們出現時守門不得 fall-through 成放行。
         */
        @Test
        @DisplayName("未知／未啟用／空白的 collection_model 一律 fail-closed")
        void unknownCollectionModelFailsClosed() {
            String[] denied = {
                    CollectionModel.PROVIDER_COLLECTED.value(),
                    CollectionModel.EXTERNAL_PREPAID.value(),
                    "",
                    "  ",
                    "SOMETHING_NEW",
            };
            for (String model : denied) {
                assertThatThrownBy(() -> invokeGuard(policyWith(model)))
                        .as("collection_model='" + model + "' 必須被擋下")
                        .isInstanceOf(OrderBusinessException.class)
                        .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                                .isEqualTo(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN.code()));
            }
            assertThatThrownBy(() -> invokeGuard(policyWith(null)))
                    .as("collection_model=null 必須被擋下")
                    .isInstanceOf(OrderBusinessException.class)
                    .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                            .isEqualTo(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN.code()));
        }

        /** 收款主體不能由呼叫端指定：守門方法不得吃 boolean（consolidated 透傳的形狀）。 */
        @Test
        @DisplayName("守門方法的簽章不得吃 boolean")
        void guardDoesNotTakeBooleanFromRequest() {
            boolean found = false;
            for (Method m : CheckoutService.class.getDeclaredMethods()) {
                if (!m.getName().equals("requireSupportedCollectionModel")) {
                    continue;
                }
                found = true;
                for (Class<?> p : m.getParameterTypes()) {
                    assertThat(p)
                            .as("requireSupportedCollectionModel 出現 boolean 參數——收款主體又變成由呼叫端傳入的旗標了")
                            .isNotEqualTo(boolean.class)
                            .isNotEqualTo(Boolean.class);
                }
            }
            assertThat(found).as("找不到 requireSupportedCollectionModel").isTrue();
        }
    }

    @Nested
    @DisplayName("政策推導：本波受託代銷刻意未啟用")
    class PolicyResolution {

        private final OrderPolicyResolver resolver = new OrderPolicyResolver();

        @Test
        @DisplayName("一律推導出賣家直營（費率未定案 ＋ 進項發票軸未落地 ⇒ fail-closed）")
        void alwaysSellerDirectInThisWave() {
            for (ShippingMethod m : ShippingMethod.values()) {
                OrderPolicySnapshot p = resolver.resolve(m.value());
                assertThat(p.commerceModel()).isEqualTo(CommerceModel.SELLER_DIRECT.value());
                assertThat(p.collectionModel()).isEqualTo(CollectionModel.SELLER_MERCHANT.value());
                assertThat(p.invoiceModel()).isEqualTo(InvoiceModel.SELLER_TO_BUYER.value());
            }
        }

        /**
         * 政策矩陣的雙條件式在 DB 是 {@code chk_orders_policy_matrix}。
         * 這裡驗的是「推導出來的組合本來就滿足它」——讓錯誤在結帳前就被發現，
         * 而不是等 INSERT 撞 CHECK。
         */
        @Test
        @DisplayName("推導結果必須滿足 DB 的政策矩陣雙條件式")
        void resolvedPolicySatisfiesDbMatrix() {
            for (ShippingMethod m : ShippingMethod.values()) {
                OrderPolicySnapshot p = resolver.resolve(m.value());
                boolean consignment = CommerceModel.PLATFORM_CONSIGNMENT.value().equals(p.commerceModel());
                boolean platformCollected = CollectionModel.PLATFORM_AGENT_MERCHANT.value().equals(p.collectionModel());
                assertThat(consignment).isEqualTo(platformCollected);

                boolean platformInvoice = InvoiceModel.PLATFORM_TO_BUYER.value().equals(p.invoiceModel());
                assertThat(platformInvoice).isEqualTo(platformCollected);
            }
        }

        @Test
        @DisplayName("商家自送必然是賣家履約（與 chk_orders_fulfillment_mode_consistent 同一條規則）")
        void merchantDeliveryIsSellerManaged() {
            assertThat(resolver.resolve(ShippingMethod.MERCHANT_DELIVERY.value()).fulfillmentMode())
                    .isEqualTo(FulfillmentMode.SELLER_MANAGED.value());
        }

        @Test
        @DisplayName("未知配送方式 fail-closed：不得 default 成平台履約")
        void unknownShippingMethodFailsClosed() {
            for (String bogus : new String[]{"3099", "", "  ", "TAQBIN", "9999"}) {
                assertThatThrownBy(() -> resolver.resolve(bogus))
                        .as("配送方式 '" + bogus + "' 應被擋下")
                        .isInstanceOf(OrderBusinessException.class)
                        .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                                .isEqualTo(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN.code()));
            }
            assertThatThrownBy(() -> resolver.resolve(null))
                    .as("null 也要回業務碼，不能是 NPE（NPE 會被翻成 500，指不到真正的原因）")
                    .isInstanceOf(OrderBusinessException.class)
                    .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                            .isEqualTo(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN.code()));
        }

        @Test
        @DisplayName("平台物流通道推導為平台履約")
        void platformChannelsAreePlatformLogistics() {
            for (ShippingMethod m : new ShippingMethod[]{
                    ShippingMethod.TAQBIN, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
                    ShippingMethod.UBER_INSTANT}) {
                assertThat(resolver.resolve(m.value()).fulfillmentMode())
                        .as("配送方式 " + m.value())
                        .isEqualTo(FulfillmentMode.PLATFORM_LOGISTICS.value());
            }
        }
    }
}
