package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 B4：逾期關單改走 <b>例外 A（同 tx outbox）</b>。
 *
 * <p>缺陷：{@code updateToExpired} 是永久狀態轉換（{@code '0'→'3'} 終態），
 * 而 {@code payment.expired} 原本在其後以 {@code PaymentEventPublisher} 直發，
 * 例外被<b>同一方法內</b>的 per-item catch 吞掉 ⇒ 不標 rollback-only ⇒
 * 狀態照樣 commit、事件永久遺失、無告警無重試。
 *
 * <p>修法：事件改在 {@link ExpiredTransactionCloseService#closeExpiredInTx} 的短 tx 內
 * 寫進 {@code outbox_events}，且事務邊界從「整批一個 tx」改成「逐筆跨 bean tx」，
 * 讓「enqueue 失敗 ⇒ 該筆 rollback ⇒ 下一 tick 重來」真的成立。
 */
class ExpiredTransactionOutboxTest {

    private static PaymentTransaction expired(String tradeNo) {
        return PaymentTransaction.builder()
                .id(1L)
                .tradeNo(tradeNo)
                .orderId("O" + tradeNo)
                .merId("HAWK9")
                .buyerId("2001")
                .paymentType(PaymentMethod.ATM)
                .amount(100)
                .status(PaymentTransactionStatus.INIT)
                .createdAt(LocalDateTime.now().minusHours(2))
                .updatedAt(LocalDateTime.now().minusHours(2))
                .expireAt(LocalDateTime.now().minusHours(1))
                .version(0)
                .build();
    }

    @Test
    @DisplayName("CAS 成功 → payment.expired 走 outbox（不再事務外直發），且 CAS 嚴格先於 enqueue")
    void casWon_enqueuesToOutboxAfterCas() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PaymentDomainService domainService = mock(PaymentDomainService.class);
        PaymentOutboxWriter writer = mock(PaymentOutboxWriter.class);
        PaymentTransaction tx = expired("T_EXP_OK");

        when(repo.updateToExpired(eq("T_EXP_OK"), anyInt())).thenReturn(1);
        PaymentDomainEvent.PaymentExpired event = new PaymentDomainEvent.PaymentExpired(
                "T_EXP_OK", "OT_EXP_OK", "2001", 100L, tx.expireAt(), LocalDateTime.now());
        when(domainService.createPaymentExpiredEvent(tx)).thenReturn(event);

        ExpiredTransactionCloseService close =
                new ExpiredTransactionCloseService(repo, domainService, writer);

        assertTrue(close.closeExpiredInTx(tx));

        InOrder order = inOrder(repo, writer);
        order.verify(repo).updateToExpired(eq("T_EXP_OK"), anyInt());
        ArgumentCaptor<PaymentDomainEvent> captor = ArgumentCaptor.forClass(PaymentDomainEvent.class);
        order.verify(writer).enqueueAndDispatch(captor.capture());
        assertEquals("payment.expired", captor.getValue().eventType().value(),
                "outbox 的 routing key 直接取 eventType value，必須是 payment.expired");
    }

    /**
     * ⚠️ 鑑別點：Mockito 對 {@code int} 的預設回傳是 <b>0</b>，
     * 「忘記 stub CAS」與「CAS 真的回 0」長得一樣。本測試斷言相反方向——
     * CAS 回 0（另一個 pod 已關單）時<b>一筆 outbox 都不得寫</b>，否則兩個 pod 各發一次
     * {@code payment.expired}。
     */
    @Test
    @DisplayName("CAS 回 0（另一個 pod 已關單）→ 一筆 outbox 都不寫")
    void casLost_writesNoOutbox() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PaymentDomainService domainService = mock(PaymentDomainService.class);
        PaymentOutboxWriter writer = mock(PaymentOutboxWriter.class);
        PaymentTransaction tx = expired("T_EXP_LOST");

        when(repo.updateToExpired(eq("T_EXP_LOST"), anyInt())).thenReturn(0);

        ExpiredTransactionCloseService close =
                new ExpiredTransactionCloseService(repo, domainService, writer);

        org.junit.jupiter.api.Assertions.assertFalse(close.closeExpiredInTx(tx));

        verifyNoInteractions(writer);
        verify(domainService, never()).createPaymentExpiredEvent(any());
    }

    /**
     * 例外 A 的原子性守門：outbox 寫入失敗必須<b>傳播出 {@code @Transactional} 方法</b>，
     * 否則 Spring proxy 不會 rollback ⇒ 回到「狀態已轉、事件遺失」的原缺陷。
     */
    @Test
    @DisplayName("outbox enqueue 失敗必須傳播（讓該筆 tx rollback），不得被吞成 true")
    void enqueueFailure_propagatesOutOfTxMethod() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PaymentDomainService domainService = mock(PaymentDomainService.class);
        PaymentOutboxWriter writer = mock(PaymentOutboxWriter.class);
        PaymentTransaction tx = expired("T_EXP_THROW");

        when(repo.updateToExpired(eq("T_EXP_THROW"), anyInt())).thenReturn(1);
        when(domainService.createPaymentExpiredEvent(tx)).thenReturn(
                new PaymentDomainEvent.PaymentExpired("T_EXP_THROW", "OT_EXP_THROW", "2001",
                        100L, tx.expireAt(), LocalDateTime.now()));
        doThrow(new IllegalStateException("payload 序列化失敗"))
                .when(writer).enqueueAndDispatch(any());

        ExpiredTransactionCloseService close =
                new ExpiredTransactionCloseService(repo, domainService, writer);

        assertThrows(IllegalStateException.class, () -> close.closeExpiredInTx(tx));
    }

    /**
     * 排程層：{@code canMarkAsExpired} 為 false 時不得進入短 tx（連 CAS 都不做）。
     */
    @Test
    @DisplayName("domain 判定不可標逾期 → 不進短 tx、不寫 outbox")
    void notExpirable_skipsTx() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PaymentDomainService domainService = mock(PaymentDomainService.class);
        ExpiredTransactionCloseService close = mock(ExpiredTransactionCloseService.class);
        PaymentTransaction tx = expired("T_EXP_SKIP");

        when(repo.findExpiredTransactions(anyInt())).thenReturn(List.of(tx));
        when(domainService.canMarkAsExpired(tx)).thenReturn(false);

        new ExpiredTransactionScheduler(repo, domainService, close).processExpiredTransactions();

        verifyNoInteractions(close);
    }
}
