package io.github.will413028.hawkr.order.checkout;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@link OrderPolicyResolver} 的專屬單元測試。
 *
 * <h2>🔴 期望值一律硬寫字面值</h2>
 * 不用 {@code FulfillmentMode.SELLER_MANAGED.value()}、不用 {@code OrderPolicyResolver.POLICY_VERSION}。
 * 那些常數<b>會跟著被測物一起變</b>：改了 enum 的 value、改了 POLICY_VERSION，測試照樣綠，
 * 等於什麼都沒驗。政策快照是「事後用同一組輸入重算出同一個數字」的依據，它的值<b>就是</b>契約本身。
 */
class OrderPolicyResolverTest {

    private final OrderPolicyResolver resolver = new OrderPolicyResolver();

    @Test
    @DisplayName("履約一律 SELLER_MANAGED（賣家自寄，與 DB chk_orders_fulfillment_mode_consistent 相容）")
    void fulfillmentIsAlwaysSellerManaged() {
        assertThat(resolver.resolve().fulfillmentMode()).isEqualTo("SELLER_MANAGED");
    }

    @Test
    @DisplayName("其餘四欄：商流／收款／發票／政策版本恆定")
    void otherColumnsAreConstant() {
        OrderPolicySnapshot snapshot = resolver.resolve();

        assertThat(snapshot.commerceModel()).isEqualTo("SELLER_DIRECT");
        assertThat(snapshot.collectionModel()).isEqualTo("SELLER_MERCHANT");
        assertThat(snapshot.invoiceModel()).isEqualTo("SELLER_TO_BUYER");
        assertThat(snapshot.policyVersion()).isEqualTo("v1-foundation");
    }

    @Test
    @DisplayName("重複推導必須得到相等的快照（政策可重算，是事後對帳的前提）")
    void resolveIsDeterministic() {
        assertThat(resolver.resolve()).isEqualTo(resolver.resolve());
    }
}
