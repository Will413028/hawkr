package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.assertj.core.api.Assertions.catchThrowableOfType;

/**
 * {@link OrderPolicyResolver} 的專屬單元測試（W0-4）。
 *
 * <h2>為什麼需要這一支——既有覆蓋長什麼樣</h2>
 * 在本檔之前，{@code OrderPolicyResolver} 沒有任何專屬測試，只被四支契約／持久化測試
 * <b>順帶</b>經過（{@code OrderPolicySnapshotChainContractTest}／{@code PaymentMerIdPolicyContractTest}／
 * {@code OrderPolicyPersistenceOnCreationPathsTest}／{@code OrderPolicySnapshotPersistenceIT}），
 * 而那四支都是<b>拿 resolver 當工具</b>去驗別的東西（鏈路、merId、entity 寫入、DB 落地）。
 * 結果是：resolver 自己的<b>值域完整性</b>（每個 {@code shipping_method} 各算出什麼）從來沒有被斷言過，
 * 而那正是 {@code fulfillment_mode} 這個<b>會寫進訂單快照、成為撥款與開票法源</b>的欄位的唯一決定點。
 *
 * <h2>🔴 期望值一律硬寫字面值</h2>
 * 不用 {@code FulfillmentMode.PLATFORM_LOGISTICS.value()}、不用 {@code OrderPolicyResolver.POLICY_VERSION}。
 * 那些常數<b>會跟著被測物一起變</b>：改了 enum 的 value、改了 POLICY_VERSION，測試照樣綠，
 * 等於什麼都沒驗（見 {@code 03-delegation-templates.md} §0.3 第二種形態）。
 * 政策快照是「事後用同一組輸入重算出同一個數字」的依據，它的值<b>就是</b>契約本身。
 */
class OrderPolicyResolverTest {

    private final OrderPolicyResolver resolver = new OrderPolicyResolver();

    // ===== 履約模式：逐一列舉 ShippingMethod 的全部五個值 =====

    @ParameterizedTest(name = "shipping_method={0} → PLATFORM_LOGISTICS")
    @ValueSource(strings = {"3001", "3002", "3003", "3004"})
    @DisplayName("平台物流白名單四碼（黑貓／7-11店到店／Uber／7-11大宗寄倉）→ PLATFORM_LOGISTICS")
    void platformLogisticsMethods(String shippingMethod) {
        assertThat(resolver.resolve(shippingMethod).fulfillmentMode()).isEqualTo("PLATFORM_LOGISTICS");
    }

    @Test
    @DisplayName("3005 商家自行配送 → SELLER_MANAGED（與 DB chk_orders_fulfillment_mode_consistent 同一條規則）")
    void merchantDeliveryIsSellerManaged() {
        assertThat(resolver.resolve("3005").fulfillmentMode()).isEqualTo("SELLER_MANAGED");
    }

    // ===== 值域外：fail-closed，不得猜成平台履約 =====

    @Test
    @DisplayName("null → 拋 9504，不是 NPE（Set.of(...).contains(null) 會 NPE，而 NPE 被上層翻成 500，維運看到的碼指不到原因）")
    void nullShippingMethodThrows9504() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> resolver.resolve(null), OrderBusinessException.class);
        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9504);
    }

    @ParameterizedTest(name = "值域外 shipping_method={0} → 拋 9504")
    @ValueSource(strings = {"3006", "9999", "", "3001 ", "TAQBIN", "3003.0"})
    @DisplayName("值域外一律 fail-closed 拋 9504——「看不懂的配送方式」不得被記成平台履約")
    void unknownShippingMethodThrows9504(String shippingMethod) {
        OrderBusinessException ex = catchThrowableOfType(
                () -> resolver.resolve(shippingMethod), OrderBusinessException.class);
        assertThat(ex).isNotNull();
        assertThat(ex.code()).isEqualTo(9504);
    }

    @Test
    @DisplayName("enum 的 code 名不是 value——用 code 名當輸入必須被拒，否則兩種表示法會同時成立")
    void enumCodeNameIsNotAcceptedAsValue() {
        assertThatThrownBy(() -> resolver.resolve("UBER_INSTANT"))
                .isInstanceOf(OrderBusinessException.class);
    }

    // ===== 其餘五欄：本波恆定值 =====

    @ParameterizedTest(name = "shipping_method={0} 的其餘五欄")
    @ValueSource(strings = {"3001", "3002", "3003", "3004", "3005"})
    @DisplayName("商流／收款／發票／政策版本／代銷附約五欄與配送方式無關，本波恆定")
    void nonFulfillmentColumnsAreConstantThisWave(String shippingMethod) {
        OrderPolicySnapshot snapshot = resolver.resolve(shippingMethod);

        assertThat(snapshot.commerceModel()).isEqualTo("SELLER_DIRECT");
        assertThat(snapshot.collectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(snapshot.invoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(snapshot.policyVersion()).isEqualTo("v1-foundation");
    }

    @Test
    @DisplayName("同樣的輸入必須算出相等的快照（政策可重算，是事後對帳的前提）")
    void resolveIsDeterministic() {
        assertThat(resolver.resolve("3002")).isEqualTo(resolver.resolve("3002"));
    }

    // ===== 🔴 陰性：resolver 不管 Uber 能不能用 =====

    @Test
    @DisplayName("resolver 對 3003 仍回 PLATFORM_LOGISTICS——「值域已定義」與「已啟用」是兩件事，"
            + "擋 Uber 是 UberOrderEligibilityGuard 的責任，不是把 3003 從白名單拿掉")
    void resolverStillClassifiesUberAsPlatformLogistics() {
        // 這一條看起來像在保護一個 bug，實際上守的是「兩個決定點不重疊」：
        // 若有人為了擋 Uber 而把 3003 從 PLATFORM_LOGISTICS_METHODS 拿掉，
        // resolver 會對 3003 拋 9504「無法判定履約模式」——那個碼的語意是「我方配置讀不到」(424)，
        // 與真正的原因「此配送服務尚未開放」(9503/403) 完全不同，
        // 而且會連帶讓**未來啟用 Uber 時**的政策推導變成需要改兩個地方。
        assertThat(resolver.resolve("3003").fulfillmentMode()).isEqualTo("PLATFORM_LOGISTICS");
    }
}
