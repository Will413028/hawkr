package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;

/**
 * generic {@code enqueue} 的 transaction boundary 閘門（WP1-R1 主線 blocker）。
 *
 * <p><b>要守住的不變式</b>：transactional outbox 的存在意義是
 * 「domain transition ＋ outbox INSERT 在**同一個交易**」——業務 rollback 時事件必須一起消失。
 * 少了機械閘門，未來的 caller（N16／N17）忘記開交易時，outbox 列會**獨立落庫**，
 * 而那個失敗完全靜默：insert 成功、scheduler 照常撈到、事件照常發出去，
 * 但它描述的那件事從來沒有真的發生過。
 *
 * <p>🔴 <b>本檔最重要的一支是 {@link #t2_fakeSynchronizationIsNotEnough()}</b>：
 * {@code isSynchronizationActive()} 與 {@code isActualTransactionActive()} 是**兩個不同的**
 * ThreadLocal。前者只表示「這條 thread 上有 synchronization 機制」，可以在沒有任何實際 DB 交易的
 * 情況下為 true。拿前者當閘門會放行真正危險的那一種情形，而且**其他每一支測試都看不出差別**
 * ——只有 T2 分得出來。
 */
class PaymentOutboxWriterTransactionGuardTest {

    private final OutboxEventMapper mapper = mock(OutboxEventMapper.class);
    private final PaymentOutboxPublisher publisher = mock(PaymentOutboxPublisher.class);
    private final SimpleMeterRegistry registry = new SimpleMeterRegistry();
    private final PaymentOutboxWriter writer =
            new PaymentOutboxWriter(mapper, new ObjectMapper(), publisher, registry);

    @AfterEach
    void cleanUp() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.clearSynchronization();
        }
        TransactionSynchronizationManager.setActualTransactionActive(false);
    }

    // ───────────────── T1：完全沒有交易、也沒有 synchronization ─────────────────

    @Test
    @DisplayName("T1｜無交易無 synchronization → throw IllegalStateException，mapper 零互動")
    void t1_noTransactionAtAll() {
        assertFalse(TransactionSynchronizationManager.isActualTransactionActive(),
                "前置條件：本測試必須在沒有交易的情況下開始");

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> writer.enqueue("payment.events", "payment.success", "payment.success", "{}"));

        assertTrue(ex.getMessage().contains("requires an active transaction"),
                "訊息要說得出是交易的問題，實際值=" + ex.getMessage());
        // 閘門必須在**任何副作用之前**：連 insert 都不能發生
        verifyNoInteractions(mapper);
        verifyNoInteractions(publisher);
    }

    // ───────────────── T2（mandatory）：fake synchronization 不算交易 ─────────────────

    /**
     * 🔴 這一支是本輪 PM blocker 的本體。
     *
     * <p>只呼叫 {@code initSynchronization()} ⇒ {@code isSynchronizationActive()} 為 <b>true</b>、
     * 而 {@code isActualTransactionActive()} 仍為 <b>false</b>。
     * 閘門若寫成 {@code isSynchronizationActive()}，這一支會通過 —— 而它代表的正是
     * 「有人以為自己在交易裡，其實沒有」的那種情形。
     */
    @Test
    @DisplayName("T2｜只有 synchronization、沒有實際交易 → 仍然 throw，mapper 零互動")
    void t2_fakeSynchronizationIsNotEnough() {
        TransactionSynchronizationManager.initSynchronization();

        // 先把「兩者確實不同」這件事本身斷言出來，否則本測試只是在重複 T1
        assertTrue(TransactionSynchronizationManager.isSynchronizationActive(),
                "前置條件：synchronization 應為 active");
        assertFalse(TransactionSynchronizationManager.isActualTransactionActive(),
                "🔴 前置條件：initSynchronization() 不得讓 isActualTransactionActive() 變 true"
                        + "——若這裡失敗，代表本測試已無鑑別力，兩個述詞在此環境下不可分辨");

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> writer.enqueue("payment.events", "payment.success", "payment.success", "{}"));

        assertTrue(ex.getMessage().contains("requires an active transaction"), ex.getMessage());
        verifyNoInteractions(mapper);
        verifyNoInteractions(publisher);
    }

    // ───────────────── payment wrapper 走同一條閘門 ─────────────────

    /**
     * {@code enqueueAndDispatch} 內部走 generic {@code enqueue} ⇒ 交易外呼叫時，
     * 它也必須在 insert **之前** fail-fast，而不是先寫了一列才發現。
     */
    @Test
    @DisplayName("payment wrapper 在交易外呼叫 → 同樣 fail-fast，且業務 metric 不得被計數")
    void paymentWrapperAlsoFailsFastOutsideTransaction() {
        LocalDateTime now = LocalDateTime.of(2026, 9, 16, 12, 0, 0);

        assertThrows(IllegalStateException.class,
                () -> writer.enqueueAndDispatch(new PaymentDomainEvent.PaymentSucceeded(
                        "T1", "O1", "B1", 100L, "P1", "1", now, now)));

        verifyNoInteractions(mapper);
        verifyNoInteractions(publisher);
        // 事件沒有落地 ⇒ 業務 metric 也不該動（它代表「已提交的付款結果」）
        assertEquals(0.0, registry.counter("hawkr_payment").count(),
                "交易外被擋下的事件不得計入付款終態 metric");
    }

    // ───────────────── 對照組：閘門不是恆擋 ─────────────────

    /**
     * 沒有這一支，把閘門寫成 {@code throw new IllegalStateException()}（無條件）
     * 也會讓 T1／T2 全綠。
     *
     * <p>⚠️ 這裡用 {@code setActualTransactionActive(true)} 模擬交易旗標，
     * 只證明「閘門會放行」。**真實交易語意（rollback 時 outbox 列一起消失）
     * 由 {@code OutboxEnqueueTransactionIntegrationTest} 以真 DB 證明**，不在本檔宣稱。
     */
    @Test
    @DisplayName("對照組｜actualTransactionActive=true → 放行並 insert（證明閘門不是恆擋）")
    void controlGroup_realTransactionFlagIsAccepted() {
        TransactionSynchronizationManager.initSynchronization();
        TransactionSynchronizationManager.setActualTransactionActive(true);

        var entity = writer.enqueue("hawkr.shipping", "fulfillment.responsibility.finalized",
                "fulfillment.responsibility.finalized", "{}");

        assertEquals("hawkr.shipping", entity.getExchange());
        org.mockito.Mockito.verify(mapper).insert(entity);
    }
}
