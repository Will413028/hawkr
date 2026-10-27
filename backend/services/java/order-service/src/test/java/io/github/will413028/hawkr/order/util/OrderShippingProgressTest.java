package io.github.will413028.hawkr.order.util;

import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 物流推進守門規則單元測試（issue #1644）。
 *
 * <p>驗證 {@link OrderShippingProgress} 的「單調前進 + 異常恢復」合法前置集合展開值，
 * 涵蓋：起點無前置、待出貨杜絕 35→20 倒退、異常恢復邊、終端態保護、異常態自身回空。
 */
class OrderShippingProgressTest {

    @Test
    @DisplayName("allowedFromFor(PAID) = ∅（起點無更早主線態，且非異常恢復目標）")
    void paidHasNoPredecessor() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.PAID.value())).isEmpty();
    }

    @Test
    @DisplayName("allowedFromFor(PENDING_SHIPMENT) = {PAID}（不含 35，杜絕 35→20 倒退）")
    void pendingShipmentExcludesAbnormal() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.PENDING_SHIPMENT.value()))
                .containsExactlyInAnyOrder(OrderStatus.PAID.value())
                .doesNotContain(OrderStatus.SHIPPING_ABNORMAL.value());
    }

    @Test
    @DisplayName("allowedFromFor(IN_TRANSIT) = {PAID, PENDING_SHIPMENT, SHIPPING_ABNORMAL}")
    void inTransitIncludesRecovery() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.IN_TRANSIT.value()))
                .containsExactlyInAnyOrder(
                        OrderStatus.PAID.value(),
                        OrderStatus.PENDING_SHIPMENT.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value());
    }

    @Test
    @DisplayName("allowedFromFor(WAITING_PICKUP) = {PAID, PENDING_SHIPMENT, IN_TRANSIT, SHIPPING_ABNORMAL}")
    void waitingPickupIncludesRecovery() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.WAITING_PICKUP.value()))
                .containsExactlyInAnyOrder(
                        OrderStatus.PAID.value(),
                        OrderStatus.PENDING_SHIPMENT.value(),
                        OrderStatus.IN_TRANSIT.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value());
    }

    @Test
    @DisplayName("allowedFromFor(DELIVERED) = {PAID, PENDING_SHIPMENT, IN_TRANSIT, WAITING_PICKUP, SHIPPING_ABNORMAL}")
    void deliveredIncludesAllInTransitAndRecovery() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.DELIVERED.value()))
                .containsExactlyInAnyOrder(
                        OrderStatus.PAID.value(),
                        OrderStatus.PENDING_SHIPMENT.value(),
                        OrderStatus.IN_TRANSIT.value(),
                        OrderStatus.WAITING_PICKUP.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value());
    }

    @Test
    @DisplayName("allowedFromFor(SHIPPING_ABNORMAL) = ∅（35 非主線態，回空）")
    void abnormalTargetReturnsEmpty() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.SHIPPING_ABNORMAL.value())).isEmpty();
    }

    @Test
    @DisplayName("allowedFromFor(COMPLETED) = ∅（終端態不在序位表，物流事件推不動）")
    void terminalStateReturnsEmpty() {
        assertThat(OrderShippingProgress.allowedFromFor(OrderStatus.COMPLETED.value())).isEmpty();
    }

    @Test
    @DisplayName("allowedFromForAbnormal() = {PAID, PENDING_SHIPMENT, IN_TRANSIT, WAITING_PICKUP}（不含 40/35）")
    void abnormalAllowedFromIsInTransitMainline() {
        assertThat(OrderShippingProgress.allowedFromForAbnormal())
                .containsExactlyInAnyOrder(
                        OrderStatus.PAID.value(),
                        OrderStatus.PENDING_SHIPMENT.value(),
                        OrderStatus.IN_TRANSIT.value(),
                        OrderStatus.WAITING_PICKUP.value())
                .doesNotContain(
                        OrderStatus.DELIVERED.value(),
                        OrderStatus.SHIPPING_ABNORMAL.value());
    }
}
