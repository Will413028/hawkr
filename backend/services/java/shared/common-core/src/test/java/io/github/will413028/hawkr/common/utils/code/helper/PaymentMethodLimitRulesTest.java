package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * §2A.2 付款方式金額範圍 helper 邊界測試。
 * 規則：
 *   CREDIT_CARD / APPLE_PAY / GOOGLE_PAY: 1~199,999
 *   ATM:                                   15~49,999
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
    void atmLowerBound() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.ATM, 15L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.ATM, 14L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.ATM, 1L));
    }

    @Test
    void atmUpperBound() {
        assertTrue(PaymentMethodLimitRules.isWithinRange(PaymentMethod.ATM, 49_999L));
        assertFalse(PaymentMethodLimitRules.isWithinRange(PaymentMethod.ATM, 50_000L));
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
                () -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.ATM, 14L));
        assertEquals(PaymentMethod.ATM, ex.method());
        assertEquals("ATM 轉帳", ex.label());
        assertEquals(14L, ex.amount());
        assertEquals(15L, ex.min());
        assertEquals(49_999L, ex.max());
    }

    @Test
    void validateOrThrowPassesWithinRange() {
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.CREDIT_CARD, 1L));
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.CREDIT_CARD, 199_999L));
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.ATM, 15L));
        assertDoesNotThrow(() -> PaymentMethodLimitRules.validateOrThrow(PaymentMethod.ATM, 49_999L));
    }

    // ===== resolveAvailability（前端 gating 用，§3.4.1）=====

    private static PaymentMethodLimitRules.Availability find(java.util.List<PaymentMethodLimitRules.Availability> list, PaymentMethod m) {
        return list.stream().filter(a -> a.method() == m).findFirst().orElseThrow();
    }

    @Test
    void resolveAvailabilityZeroAmountReturnsEmpty() {
        // 零元 → 空清單（前端「無需付款」）
        assertTrue(PaymentMethodLimitRules.resolveAvailability(0L).isEmpty());
    }

    @Test
    void resolveAvailabilityReturnsFourCandidatesInFixedOrder() {
        var list = PaymentMethodLimitRules.resolveAvailability(100L);
        assertEquals(4, list.size());
        assertEquals(PaymentMethod.CREDIT_CARD, list.get(0).method());
        assertEquals(PaymentMethod.APPLE_PAY, list.get(1).method());
        assertEquals(PaymentMethod.GOOGLE_PAY, list.get(2).method());
        assertEquals(PaymentMethod.ATM, list.get(3).method());
    }

    @Test
    void resolveAvailabilityBelowAtmMin() {
        // 1~14：信用卡/Apple/Google eligible；ATM below_min
        for (long amt : new long[]{1L, 14L}) {
            var list = PaymentMethodLimitRules.resolveAvailability(amt);
            assertTrue(find(list, PaymentMethod.CREDIT_CARD).eligible());
            assertTrue(find(list, PaymentMethod.APPLE_PAY).eligible());
            assertTrue(find(list, PaymentMethod.GOOGLE_PAY).eligible());
            var atm = find(list, PaymentMethod.ATM);
            assertFalse(atm.eligible());
            assertEquals(io.github.will413028.hawkr.common.utils.code.PaymentMethodIneligibleReason.AMOUNT_BELOW_MIN, atm.ineligibleReason());
        }
    }

    @Test
    void resolveAvailabilityAllEligibleInAtmRange() {
        // 15~49,999：全部 eligible（含端點）
        for (long amt : new long[]{15L, 49_999L, 1000L}) {
            var list = PaymentMethodLimitRules.resolveAvailability(amt);
            for (var a : list) {
                assertTrue(a.eligible(), "amount=" + amt + " method=" + a.method());
                assertNull(a.ineligibleReason());
            }
        }
    }

    @Test
    void resolveAvailabilityAboveAtmMax() {
        // 50,000~199,999：信用卡/Apple/Google eligible；ATM above_max
        for (long amt : new long[]{50_000L, 199_999L}) {
            var list = PaymentMethodLimitRules.resolveAvailability(amt);
            assertTrue(find(list, PaymentMethod.CREDIT_CARD).eligible());
            assertTrue(find(list, PaymentMethod.APPLE_PAY).eligible());
            assertTrue(find(list, PaymentMethod.GOOGLE_PAY).eligible());
            var atm = find(list, PaymentMethod.ATM);
            assertFalse(atm.eligible());
            assertEquals(io.github.will413028.hawkr.common.utils.code.PaymentMethodIneligibleReason.AMOUNT_ABOVE_MAX, atm.ineligibleReason());
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
