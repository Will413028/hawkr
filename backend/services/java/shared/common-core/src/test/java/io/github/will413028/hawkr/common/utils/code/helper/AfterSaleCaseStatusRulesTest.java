package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * issue #1188 驗收標準 #7：售後 active / terminal canonical 判定，涵蓋全 14 個 status。
 * <p>
 * 重點糾正：AUTO_COMPLETED_STORE_PICKUP_TIMEOUT(31) / AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT(32)
 * 為退款中間態（→40→50），屬 active，非終態。
 */
class AfterSaleCaseStatusRulesTest {

    private static final String NONE = AfterSaleAppealState.NONE.value();
    private static final String APPEALED = AfterSaleAppealState.APPEALED.value();


    @Test
    void activeStatusesWhenNoAppeal() {
        AfterSaleCaseStatus[] active = {
                AfterSaleCaseStatus.PENDING,
                AfterSaleCaseStatus.RETURN_PENDING,
                AfterSaleCaseStatus.RETURN_SHIPPING,
                AfterSaleCaseStatus.RETURN_AT_STORE,
                AfterSaleCaseStatus.RETURN_DELIVERED,
                AfterSaleCaseStatus.RETURN_FAILED,
                AfterSaleCaseStatus.RETURN_RECEIVED,
                AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT,
                AfterSaleCaseStatus.AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT,
                AfterSaleCaseStatus.REFUND_PROCESSING,
        };
        for (AfterSaleCaseStatus s : active) {
            assertTrue(AfterSaleCaseStatusRules.isActive(s, NONE),
                    s.name() + "(" + s.value() + ") 應為 active");
        }
    }

    @Test
    void terminalStatusesAreNotActiveWhenNoAppeal() {
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CLOSED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.REJECTED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CANCELLED, NONE));
    }

    @Test
    void autoCompleteRefundIntermediateStatesAreActive() {
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT, NONE));
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT, NONE));
    }

    @Test
    void appealReactivatesRejectedAndReturnPendingTimeout() {
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.REJECTED, APPEALED));
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, APPEALED));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CLOSED, APPEALED));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CANCELLED, APPEALED));
    }

    @Test
    void nullStatusIsNotActive() {
        assertFalse(AfterSaleCaseStatusRules.isActive(null, NONE));
    }


    @Test
    void terminalOverrideIsClosedAndReturnPendingTimeoutNoAppeal() {
        assertTrue(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.CLOSED, NONE));
        assertTrue(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, NONE));
    }

    @Test
    void returnPendingTimeoutUnderAppealIsNotOverride() {
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, APPEALED));
    }

    @Test
    void rejectedCancelledAndActiveAreNotOverride() {
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.REJECTED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.CANCELLED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.PENDING, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT, NONE));
    }


    @Test
    void rejectedAndCancelledDoNotOverrideLifecycle() {
        assertFalse(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.REJECTED));
        assertFalse(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.CANCELLED));
    }

    @Test
    void otherStatusesOverrideLifecycle() {
        assertTrue(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.PENDING));
        assertTrue(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.CLOSED));
        assertTrue(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT));
        assertTrue(AfterSaleCaseStatusRules.overridesOrderLifecycle(AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT));
    }


    @Test
    void activeAndTerminalOverrideAreMutuallyExclusiveAcrossAllStatuses() {
        for (AfterSaleCaseStatus s : AfterSaleCaseStatus.values()) {
            for (String appeal : new String[]{NONE, APPEALED}) {
                boolean active = AfterSaleCaseStatusRules.isActive(s, appeal);
                boolean override = AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(s, appeal);
                assertFalse(active && override,
                        s.name() + "(" + s.value() + ", appeal=" + appeal + ") 不可同時 active 與 terminal-override");
            }
        }
    }
}
