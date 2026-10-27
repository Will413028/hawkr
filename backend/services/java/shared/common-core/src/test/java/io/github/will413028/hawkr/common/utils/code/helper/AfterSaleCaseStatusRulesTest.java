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

    private static final String NONE = AfterSaleAppealState.NONE.value();       // "0"
    private static final String APPEALED = AfterSaleAppealState.APPEALED.value(); // "10"

    // ==================== isActive ====================

    @Test
    void activeStatusesWhenNoAppeal() {
        // active = NOT IN {33,50,60,70,71}
        AfterSaleCaseStatus[] active = {
                AfterSaleCaseStatus.PENDING,            // 0
                AfterSaleCaseStatus.RETURN_PENDING,     // 20
                AfterSaleCaseStatus.RETURN_SHIPPING,    // 25
                AfterSaleCaseStatus.RETURN_AT_STORE,    // 26
                AfterSaleCaseStatus.RETURN_DELIVERED,   // 27
                AfterSaleCaseStatus.RETURN_FAILED,      // 28
                AfterSaleCaseStatus.RETURN_RECEIVED,    // 30
                AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT,    // 31
                AfterSaleCaseStatus.AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT, // 32
                AfterSaleCaseStatus.REFUND_PROCESSING,  // 40
        };
        for (AfterSaleCaseStatus s : active) {
            assertTrue(AfterSaleCaseStatusRules.isActive(s, NONE),
                    s.name() + "(" + s.value() + ") 應為 active");
        }
    }

    @Test
    void terminalStatusesAreNotActiveWhenNoAppeal() {
        // 33 / 50 / 60 / 70 無申訴時非 active
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CLOSED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.REJECTED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CANCELLED, NONE));
    }

    @Test
    void autoCompleteRefundIntermediateStatesAreActive() {
        // ★ issue #1188 核心糾正：31/32 為退款中間態，必須 active（否則退款進行中誤恢復撥款倒數）
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT, NONE));
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT, NONE));
    }

    @Test
    void appealReactivatesRejectedAndReturnPendingTimeout() {
        // 60 / 33 申訴中 → active（Admin 裁決前案件凍結）
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.REJECTED, APPEALED));
        assertTrue(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, APPEALED));
        // 50 / 70 即使帶 appeal 仍非 active（無申訴重啟語意）
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CLOSED, APPEALED));
        assertFalse(AfterSaleCaseStatusRules.isActive(AfterSaleCaseStatus.CANCELLED, APPEALED));
    }

    @Test
    void nullStatusIsNotActive() {
        assertFalse(AfterSaleCaseStatusRules.isActive(null, NONE));
    }

    // ==================== isTerminalOverrideToCompleted ====================

    @Test
    void terminalOverrideIsClosedAndReturnPendingTimeoutNoAppeal() {
        assertTrue(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.CLOSED, NONE));
        assertTrue(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, NONE));
    }

    @Test
    void returnPendingTimeoutUnderAppealIsNotOverride() {
        // 33 申訴中屬 active，不是 terminal-override
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT, APPEALED));
    }

    @Test
    void rejectedCancelledAndActiveAreNotOverride() {
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.REJECTED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.CANCELLED, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.PENDING, NONE));
        assertFalse(AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(AfterSaleCaseStatus.AUTO_COMPLETED_STORE_PICKUP_TIMEOUT, NONE)); // 31 active
    }

    // ==================== overridesOrderLifecycle ====================

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

    // ==================== active 與 terminal-override 互斥（驗收 #7：四桶不重疊基礎）====================

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
