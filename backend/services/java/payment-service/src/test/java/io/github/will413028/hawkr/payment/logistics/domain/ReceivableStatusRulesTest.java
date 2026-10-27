package io.github.will413028.hawkr.payment.logistics.domain;

import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.CLOSED;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.FAILED;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.PENDING;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.SETTLED;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.SUBMITTED;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.UNCONFIRMED;
import static io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus.WRITTEN_OFF;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;

import org.junit.jupiter.api.Test;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;

/**
 * 驗 70 §3.2 的應收狀態機。
 * <p>
 * 🔴 <b>每一條 allowed-from 都硬寫字面值</b>，不從 {@code ReceivableStatusRules} 反推：
 * 期望值取自被測物的話，拿掉一個起點測試照樣全綠
 * （03-delegation-templates.md §0.3 第二種形態）。
 */
class ReceivableStatusRulesTest {

    // -------------------------------------------------- allowed-from 逐條釘死

    @Test
    void submittedAcceptsPendingAndFailed() {
        // ⚠️ 含 FAILED：退避重送成功的目標就是 SUBMITTED，漏了它會讓每一次
        //    重送成功的 CAS 都 rows=0，而呼叫端多半把 rows=0 當成「別人先做過了」。
        assertEquals(List.of(FAILED, PENDING), ReceivableStatusRules.allowedFrom(SUBMITTED));
    }

    @Test
    void settledAcceptsSubmittedAndUnconfirmed() {
        // ⚠️ 含 UNCONFIRMED：那是 UNCONFIRMED 三條出邊裡**最可能**的一條
        //    （ADM 到 PayUni 後台核對後確認「其實收到了」）。
        //    少了它，查證為真的那條路沒有邊、只剩認賠。
        assertEquals(List.of(SUBMITTED, UNCONFIRMED), ReceivableStatusRules.allowedFrom(SETTLED));
    }

    @Test
    void unconfirmedAcceptsOnlySubmitted() {
        assertEquals(List.of(SUBMITTED), ReceivableStatusRules.allowedFrom(UNCONFIRMED));
    }

    @Test
    void failedAcceptsOnlyPending() {
        // 🔴 FAILED 的語意是「送出當下回 Status ≠ SUCCESS」⇒ **只會從 PENDING 來**。
        //    原稿的 allowed-from 含 SUBMITTED，那是一條沒有執行者的邊。
        assertEquals(List.of(PENDING), ReceivableStatusRules.allowedFrom(FAILED));
        assertFalse(ReceivableStatusRules.canTransition(SUBMITTED, FAILED),
                "SUBMITTED → FAILED 是沒有執行者的邊，不得存在");
    }

    @Test
    void pendingAcceptsOnlyUnconfirmed() {
        // ADM 確認 PayUni 未受理 ⇒ 退回 PENDING 開新 attempt ＋ 新 trade_no。
        assertEquals(List.of(UNCONFIRMED), ReceivableStatusRules.allowedFrom(PENDING));
    }

    @Test
    void closedAcceptsOnlySettled() {
        assertEquals(List.of(SETTLED), ReceivableStatusRules.allowedFrom(CLOSED));
    }

    @Test
    void writtenOffAcceptsUnconfirmedAndFailed() {
        assertEquals(List.of(FAILED, UNCONFIRMED), ReceivableStatusRules.allowedFrom(WRITTEN_OFF));
    }

    // ------------------------------------------------------------ 字串形式

    @Test
    void allowedFromValuesFeedsTheCasArrayDirectly() {
        // 🔴 呼叫端一律用它組 CAS 的 allowed_from，不要在 SQL 裡各寫一份。
        //    排序是契約的一部分：它讓「repository 實際送出的陣列」與本表可以逐位元比對。
        assertEquals(List.of("FAILED", "PENDING"), ReceivableStatusRules.allowedFromValues(SUBMITTED));
        assertEquals(List.of("SUBMITTED", "UNCONFIRMED"), ReceivableStatusRules.allowedFromValues(SETTLED));
    }

    // ------------------------------------------------------------ 終態判定

    @Test
    void settledIsNotTerminal() {
        // 🔴 本檔最容易被讀錯的一格：收到錢不等於這筆帳結清了，還要等 provider 帳單
        //    確定 actual_cost 才能 true-up。把它當終態 ⇒ actual > estimated 的差額
        //    永遠不會被收，而帳面上看起來一切正常。
        assertFalse(ReceivableStatusRules.isTerminal(SETTLED));
    }

    @Test
    void onlyClosedAndWrittenOffAreTerminal() {
        assertTrue(ReceivableStatusRules.isTerminal(CLOSED));
        assertTrue(ReceivableStatusRules.isTerminal(WRITTEN_OFF));
        assertFalse(ReceivableStatusRules.isTerminal(PENDING));
        assertFalse(ReceivableStatusRules.isTerminal(SUBMITTED));
        assertFalse(ReceivableStatusRules.isTerminal(UNCONFIRMED));
        assertFalse(ReceivableStatusRules.isTerminal(FAILED));
    }

    @Test
    void terminalStatesHaveNoOutboundEdges() {
        // 終態沒有出邊：兩個終態都不得出現在任何一條 allowed-from 裡（CLOSED），
        // 或只作為終點（WRITTEN_OFF）。這是「終態真的終了」的機械形式。
        for (LogisticsReceivableStatus to : LogisticsReceivableStatus.values()) {
            List<LogisticsReceivableStatus> from = ReceivableStatusRules.allowedFrom(to);
            assertFalse(from.contains(CLOSED), "CLOSED 是終態，不得是 " + to.value() + " 的來源");
            assertFalse(from.contains(WRITTEN_OFF), "WRITTEN_OFF 是終態，不得是 " + to.value() + " 的來源");
        }
    }

    // ----------------------------------------------------------- canTransition

    @Test
    void canTransitionMatchesTheTable() {
        assertTrue(ReceivableStatusRules.canTransition(PENDING, SUBMITTED));
        assertTrue(ReceivableStatusRules.canTransition(FAILED, SUBMITTED));
        assertTrue(ReceivableStatusRules.canTransition(UNCONFIRMED, SETTLED));
        assertTrue(ReceivableStatusRules.canTransition(SETTLED, CLOSED));
        assertTrue(ReceivableStatusRules.canTransition(UNCONFIRMED, PENDING));
    }

    @Test
    void illegalEdgesAreRejected() {
        // 這一組是「陰性驗證」：不該存在的邊不存在，才是 allowed-from 的實質內容。
        assertFalse(ReceivableStatusRules.canTransition(PENDING, SETTLED), "不得跳過送出直接結案");
        assertFalse(ReceivableStatusRules.canTransition(PENDING, CLOSED));
        assertFalse(ReceivableStatusRules.canTransition(SUBMITTED, WRITTEN_OFF), "送出中不得直接核銷");
        assertFalse(ReceivableStatusRules.canTransition(SETTLED, WRITTEN_OFF));
        assertFalse(ReceivableStatusRules.canTransition(CLOSED, SETTLED));
        assertFalse(ReceivableStatusRules.canTransition(SUBMITTED, SUBMITTED), "自我轉換不是合法邊");
    }

    @Test
    void nullIsNotATransition() {
        assertFalse(ReceivableStatusRules.canTransition(null, SUBMITTED));
        assertFalse(ReceivableStatusRules.canTransition(PENDING, null));
        assertThrows(IllegalArgumentException.class, () -> ReceivableStatusRules.allowedFrom(null));
    }

    // -------------------------------------------------------------- 窮舉保證

    @Test
    void everyCodeMetaStatusHasRegisteredInboundEdges() {
        // 🔴 CodeMeta 新增一個狀態卻沒在這裡補邊 ⇒ 那個狀態的 CAS 永遠 rows=0
        //    且完全沒有訊號。本測試讓它在 build 時就現形，而不是在 prod 靜默卡死。
        assertEquals(LogisticsReceivableStatus.values().length,
                ReceivableStatusRules.statusesWithRegisteredInboundEdges().size(),
                "CodeMeta 的每個狀態都必須登記入邊");
        for (LogisticsReceivableStatus s : LogisticsReceivableStatus.values()) {
            assertFalse(ReceivableStatusRules.allowedFrom(s).isEmpty(),
                    s.value() + " 沒有任何入邊 —— 那是一個沒有人推得進來的狀態");
        }
    }

    @Test
    void statusValueDomainMatchesTheDbCheckConstraint() {
        // 對照 V9 的 chk_logrecv_status。兩份漂移的症狀是 23514，
        // 而它會在寫入當下才炸、看起來像「應用層 500」。
        assertEquals(
                List.of("PENDING", "SUBMITTED", "SETTLED", "UNCONFIRMED", "FAILED", "CLOSED", "WRITTEN_OFF"),
                java.util.Arrays.stream(LogisticsReceivableStatus.values())
                        .map(LogisticsReceivableStatus::value)
                        .toList());
    }
}
