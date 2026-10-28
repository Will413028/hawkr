package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.FulfillmentMode;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceModel;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 收款主體的契約守護：**快照欄 vs 現金流向**。
 *
 * <h2>🔴 這支測試是那個不一致唯一的偵測</h2>
 * 沒有它，下面這個失效在資料上完全看不出來：
 * {@code orders.collection_model} 被正確寫成「平台代理商戶收款」（快照欄、有 DB CHECK 守著），
 * 帳本也據此記「平台收到買家款」——<b>而錢實際上還在賣家的 MerID</b>。
 * 訂單成功、付款成功、帳本平衡、對帳查詢也查得到（只是查到的是賣家子商店的交易）。
 * 兩邊各自都「對」，只有把它們並排比對才看得出來。
 *
 * <p>⚠️ 本測試守的是<b>決策點</b>（{@code CheckoutService.resolvePaymentMerId}）。
 * 下游 {@code payment_transactions.mer_id} 是從 {@code orders.mer_id} 單路徑複製而來，
 * 那一段由 payment-service 自己的測試守；本測試不重複斷言它，但那條複製鏈斷掉的話，
 * 本測試不會發現——這一點寫出來，免得把它當成端到端的保證。
 */
@DisplayName("收款主體的政策契約")
class PaymentMerIdPolicyContractTest {

    private static final String SELLER_MER_ID = "HAWK26001234";
    private static final String AGENT_MER_ID = "HAWKAGENT";

    /**
     * 直接呼叫 private 的決策方法。
     *
     * <p>用反射而不是把方法改成 package-private：可見性是 production 的設計決定，
     * 不該為了測試而放寬。而找不到方法時<b>丟 AssertionError 不是 skip</b>——
     * 方法被改名的那天，skip 會讓這道守護靜默消失。
     */
    private static String invokeResolve(String agentMerId, OrderPolicySnapshot policy) throws Exception {
        CheckoutService service = newServiceWithoutDependencies();
        Field agentField = CheckoutService.class.getDeclaredField("agentMerId");
        agentField.setAccessible(true);
        agentField.set(service, agentMerId);

        Method m;
        try {
            m = CheckoutService.class.getDeclaredMethod(
                    "resolvePaymentMerId", OrderPolicySnapshot.class, SellerMeta.class);
        } catch (NoSuchMethodException e) {
            throw new AssertionError(
                    "找不到 resolvePaymentMerId(OrderPolicySnapshot, SellerMeta)——"
                            + "它被改名、改簽章或搬走了？本測試對『目標不存在』一律 FAIL，"
                            + "因為 skip 會讓收款主體的守護在目標消失的那一刻靜默失效", e);
        }
        m.setAccessible(true);
        SellerMeta meta = SellerMeta.newBuilder().setPayuniMerId(SELLER_MER_ID).build();
        try {
            return (String) m.invoke(service, policy, meta);
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
    @DisplayName("collection_model → 實際收款的 MerID")
    class MerIdResolution {

        @Test
        @DisplayName("賣家子商店收款 → 賣家自己的 MerID")
        void sellerMerchantUsesSellerMerId() throws Exception {
            assertThat(invokeResolve(AGENT_MER_ID, policyWith(CollectionModel.SELLER_MERCHANT.value())))
                    .isEqualTo(SELLER_MER_ID);
        }

        @Test
        @DisplayName("平台代理商戶收款 → 代理商 MerID")
        void platformAgentUsesAgentMerId() throws Exception {
            assertThat(invokeResolve(AGENT_MER_ID, policyWith(CollectionModel.PLATFORM_AGENT_MERCHANT.value())))
                    .isEqualTo(AGENT_MER_ID);
        }

        /**
         * 🔴 <b>兩個候選值必須不同</b>，否則「填錯也會通過」——
         * 用同一個字串當賣家與平台的 MerID，這支測試會在決策邏輯完全反了的情況下照樣全綠。
         */
        @Test
        @DisplayName("兩種收款模式必須解析出不同的 MerID（測試自身的鑑別力）")
        void twoModelsResolveDifferently() throws Exception {
            assertThat(invokeResolve(AGENT_MER_ID, policyWith(CollectionModel.SELLER_MERCHANT.value())))
                    .isNotEqualTo(invokeResolve(AGENT_MER_ID, policyWith(CollectionModel.PLATFORM_AGENT_MERCHANT.value())));
        }

        @Test
        @DisplayName("平台代收但代理商 MerID 未設定 → 結帳當下 fail-closed")
        void platformCollectionWithoutAgentMerIdFailsClosed() {
            assertThatThrownBy(() -> invokeResolve("", policyWith(CollectionModel.PLATFORM_AGENT_MERCHANT.value())))
                    .isInstanceOf(OrderBusinessException.class)
                    .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                            .as("必須回 PLATFORM_COLLECTION_NOT_CONFIGURED，而不是通用參數錯誤——"
                                    + "維運看到的碼決定他去查哪裡")
                            .isEqualTo(GrpcResponseCode.PLATFORM_COLLECTION_NOT_CONFIGURED.code()));
        }

        /**
         * 🔴 <b>BLOCKER 6 的核心</b>：這四個輸入以前全部靜默解析成賣家 MerID。
         *
         * <p>{@code PROVIDER_COLLECTED} 與 {@code EXTERNAL_PREPAID} 是
         * {@code chk_orders_collection_model} 的<b>合法值</b>（碼表逐字：值域內但尚無寫入路徑）。
         * 「今天 resolver 不會產出它們」是<b>呼叫端保證</b>，不是決策點保證——
         * 68／69 落地時 resolver 會開始產出，而那一天這裡若還是 fall-through，
         * 症狀是<b>錢進賣家子商店而快照欄說是 provider 代收</b>，且訂單、付款、帳本三邊都正常。
         */
        @Test
        @DisplayName("未知／未啟用／空白的 collection_model 一律 fail-closed，不得落到賣家 MerID")
        void unknownCollectionModelFailsClosed() {
            String[] denied = {
                    CollectionModel.PROVIDER_COLLECTED.value(),
                    CollectionModel.EXTERNAL_PREPAID.value(),
                    "",
                    "SOMETHING_NEW",
            };
            for (String model : denied) {
                assertThatThrownBy(() -> invokeResolve(AGENT_MER_ID, policyWith(model)))
                        .as("collection_model='" + model + "' 必須被擋下")
                        .isInstanceOf(OrderBusinessException.class)
                        .satisfies(e -> assertThat(((OrderBusinessException) e).code())
                                .isEqualTo(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN.code()));
            }
            assertThatThrownBy(() -> invokeResolve(AGENT_MER_ID, policyWith(null)))
                    .as("collection_model=null 必須被擋下")
                    .isInstanceOf(OrderBusinessException.class);
        }

        /**
         * 🔴 這一條守的是需求書列為「不可妥協」的那件事：<b>收款主體不能由呼叫端指定</b>。
         * 改造前，這個決策讀的是 {@code CheckoutPreviewRequest.consolidated}——一個買家 request 的布林。
         */
        @Test
        @DisplayName("決策方法的簽章不得再吃 boolean（那是 consolidated 透傳的形狀）")
        void resolveDoesNotTakeBooleanFromRequest() {
            for (Method m : CheckoutService.class.getDeclaredMethods()) {
                if (!m.getName().equals("resolvePaymentMerId")) {
                    continue;
                }
                for (Class<?> p : m.getParameterTypes()) {
                    assertThat(p)
                            .as("resolvePaymentMerId 出現 boolean 參數——收款主體又變成由呼叫端傳入的旗標了")
                            .isNotEqualTo(boolean.class)
                            .isNotEqualTo(Boolean.class);
                }
            }
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
            // 🔴 BLOCKER 6：「Missing / unknown / inconsistent → BLOCK」。
            //    舊版對任何非 3005 的輸入一律回 PLATFORM_LOGISTICS，包含打錯的字串與未來新增的碼。
            //    這一欄會成為撥款與開票的法源，猜錯不會有任何東西發現。
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
