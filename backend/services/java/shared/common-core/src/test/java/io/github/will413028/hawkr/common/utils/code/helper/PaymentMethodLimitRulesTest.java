package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * §2A.2 付款方式金額範圍 helper 邊界測試。
 * 規則：
 *   CREDIT_CARD / APPLE_PAY / GOOGLE_PAY: 1~199,999
 *   ZERO_AMOUNT:                           不適用
 */
class PaymentMethodLimitRulesTest {

    @Test
    void orderMaxAmountIsLargestLimit() {
        assertEquals(199_999L, PaymentMethodLimitRules.ORDER_MAX_AMOUNT);
    }

    @Test
    void creditCardLowerBound() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.CREDIT_CARD, 1L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.CREDIT_CARD, 0L));
    }

    @Test
    void creditCardUpperBound() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.CREDIT_CARD, 199_999L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.CREDIT_CARD, 200_000L));
    }

    @Test
    void applePayShareCreditCardRange() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.APPLE_PAY, 1L));
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.APPLE_PAY, 199_999L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.APPLE_PAY, 200_000L));
    }

    @Test
    void googlePayShareCreditCardRange() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.GOOGLE_PAY, 1L));
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.GOOGLE_PAY, 199_999L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.GOOGLE_PAY, 200_000L));
    }

    @Test
    void zeroAmountHasNoRange() {
        assertThrows(IllegalArgumentException.class,
                () -> PaymentMethodLimitRules.limits(PaymentMethod.ZERO_AMOUNT));
    }

    @Test
    void validateOrThrowAttachesParameters() {
        PaymentMethodLimitRules.OutOfRangeException ex = assertThrows(
                PaymentMethodLimitRules.OutOfRangeException.class,
                () -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.CREDIT_CARD, 200_000L));
        assertEquals(PaymentMethod.CREDIT_CARD, ex.method());
        assertEquals("信用卡", ex.label());
        assertEquals(200_000L, ex.amount());
        assertEquals(1L, ex.min());
        assertEquals(199_999L, ex.max());
    }

    @Test
    void validateOrThrowPassesWithinRange() {
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.CREDIT_CARD, 1L));
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.CREDIT_CARD, 199_999L));
    }

    // ===== resolveAvailability（前端 gating 用，§3.4.1）=====

    @Test
    void resolveAvailabilityZeroAmountReturnsEmpty() {
        // 零元 → 空清單（前端「無需付款」）
        assertTrue(PaymentMethodLimitRules.resolveAvailability(0L).isEmpty());
    }

    @Test
    void resolveAvailabilityReturnsThreeCandidatesInFixedOrder() {
        var list = PaymentMethodLimitRules.resolveAvailability(100L);
        assertEquals(3, list.size());
        assertEquals(PaymentMethod.CREDIT_CARD, list.get(0).method());
        assertEquals(PaymentMethod.APPLE_PAY, list.get(1).method());
        assertEquals(PaymentMethod.GOOGLE_PAY, list.get(2).method());
    }

    @Test
    void resolveAvailabilityAllEligibleWithinRange() {
        for (long amt : new long[]{1L, 1000L, 199_999L}) {
            var list = PaymentMethodLimitRules.resolveAvailability(amt);
            for (var a : list) {
                assertTrue(a.eligible(), "amount=" + amt + " method=" + a.method());
                assertNull(a.ineligibleReason());
            }
        }
    }

    @Test
    void resolveAvailabilityAboveOrderMaxAllIneligible() {
        // >=200,000：全部 above_max（信用卡上限 = 訂單上限 = 199,999）
        var list = PaymentMethodLimitRules.resolveAvailability(200_000L);
        for (var a : list) {
            assertFalse(a.eligible(), "method=" + a.method());
            assertEquals(io.github.will413028.hawkr.common.utils.code.PaymentMethodIneligibleReason.AMOUNT_ABOVE_MAX, a.ineligibleReason());
        }
    }
}
