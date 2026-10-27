package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Optional;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * issue #1288 — ATM 退款 webhook finalize 語意（05 §5.5.4.1）。
 *
 * 驗證 handleAtmRefundWebhook 五分支：
 * - active + RefundStatus=1 → finalizeAtmRefundSucceeded + 發 succeeded
 * - active + RefundStatus=2 → finalizeAtmRefundFailed + 發 failed
 * - 無 active + 最新 SUCCEEDED → 冪等，不 finalize、不重發
 * - 無 active + 最新 FAILED + RefundStatus=1 → 衝突校正走 correctFailedToSucceeded（WHERE status='3'）+ 補發
 * - 無 active + 查無 row → 孤兒，不 finalize、不發事件
 */
@ExtendWith(MockitoExtension.class)
class AtmRefundWebhookFinalizeTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentEventPublisher eventPublisher;
    @Mock private RefundDomainService domainService;
    @Mock private PaymentClient paymentClient;

    private RefundApplicationService service;

    private static final String TRADE_NO = "TRD_ATM_1";

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(901, 0);
        } catch (Exception ignored) {
            // 已初始化則忽略
        }
    }

    @BeforeEach
    void setUp() {
        service = new RefundApplicationService(
                transactionRepository, refundRepository,
                eventPublisher, domainService, paymentClient);
    }

    private RefundTransaction refundWithStatus(RefundStatus status) {
        return RefundTransaction.builder()
                .id(1L)
                .refundNo("RFD_1")
                .tradeNo(TRADE_NO)
                .orderId("ORD_1")
                .amount(500L)
                .status(status)
                .version(0)
                .build();
    }

    @Test
    @DisplayName("active + RefundStatus=1 → finalizeAtmRefundSucceeded + 發 succeeded")
    void activeSuccess_finalizesSucceededAndPublishes() {
        RefundTransaction refund = refundWithStatus(RefundStatus.PROCESSING);
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.of(refund));
        when(refundRepository.finalizeAtmRefundSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any()))
                .thenReturn(1);
        when(domainService.createRefundSucceededEvent(any(), any(), any(), any(), any(), any()))
                .thenReturn(mock(PaymentDomainEvent.RefundSucceeded.class));

        service.handleAtmRefundWebhook(TRADE_NO, "1", "2026-06-20", "OK");

        verify(refundRepository).finalizeAtmRefundSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundFailed(any(), any(), any());
        verify(eventPublisher).publish(any(PaymentDomainEvent.RefundSucceeded.class));
    }

    @Test
    @DisplayName("active + RefundStatus=2 → finalizeAtmRefundFailed + 發 failed")
    void activeFail_finalizesFailedAndPublishes() {
        RefundTransaction refund = refundWithStatus(RefundStatus.PROCESSING);
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.of(refund));
        when(refundRepository.finalizeAtmRefundFailed(eq("RFD_1"), eq("2"), any())).thenReturn(1);
        when(domainService.createRefundFailedEvent(any(), any(), any()))
                .thenReturn(mock(PaymentDomainEvent.RefundFailed.class));

        service.handleAtmRefundWebhook(TRADE_NO, "2", "-", "BANK_REJECT");

        verify(refundRepository).finalizeAtmRefundFailed(eq("RFD_1"), eq("2"), any());
        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(eventPublisher).publish(any(PaymentDomainEvent.RefundFailed.class));
    }

    @Test
    @DisplayName("無 active + 最新 SUCCEEDED → 冪等，不 finalize、不重發")
    void noActive_latestSucceeded_idempotentNoFinalizeNoPublish() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.SUCCEEDED)));

        service.handleAtmRefundWebhook(TRADE_NO, "1", "2026-06-20", "OK");

        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundFailed(any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 最新 FAILED + RefundStatus=1 → 衝突校正走 correctFailedToSucceeded（非 finalize）+ 補發")
    void noActive_latestFailed_conflict_correctsAndPublishes() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.FAILED)));
        // 校正路徑必須走 correctFailedToSucceeded（WHERE status='3'）；finalizeAtmRefundSucceeded
        // 的 WHERE status IN('0','1') 對 FAILED row 命中 0 rows，會靜默丟失成功結果（P0）。
        when(refundRepository.correctFailedToSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any()))
                .thenReturn(1);
        when(domainService.createRefundSucceededEvent(any(), any(), any(), any(), any(), any()))
                .thenReturn(mock(PaymentDomainEvent.RefundSucceeded.class));

        service.handleAtmRefundWebhook(TRADE_NO, "1", "2026-06-20", "OK");

        // 斷言呼叫的是正確 mapper 方法（correctFailedToSucceeded），且未誤用 finalizeAtmRefundSucceeded
        verify(refundRepository).correctFailedToSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(eventPublisher).publish(any(PaymentDomainEvent.RefundSucceeded.class));
    }

    @Test
    @DisplayName("無 active + 最新 FAILED + RefundStatus=1 但校正 0 rows（已被別的路徑校正）→ 不重發")
    void noActive_latestFailed_conflict_correctNoop_doesNotPublish() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.FAILED)));
        // correctFailedToSucceeded 回 0：WHERE status='3' 未命中（已被 sync 或前一次 webhook 校正為 '2'）→ 冪等不重發
        when(refundRepository.correctFailedToSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any()))
                .thenReturn(0);

        service.handleAtmRefundWebhook(TRADE_NO, "1", "2026-06-20", "OK");

        verify(refundRepository).correctFailedToSucceeded(eq("RFD_1"), any(), eq("1"), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 最新 FAILED + RefundStatus=2 → 冪等，不動作")
    void noActive_latestFailed_bothFailed_idempotent() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.FAILED)));

        service.handleAtmRefundWebhook(TRADE_NO, "2", "-", "BANK_REJECT");

        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundFailed(any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 查無 row → 孤兒，不 finalize、不發事件")
    void noActive_noRow_orphanNoAction() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO)).thenReturn(Optional.empty());

        service.handleAtmRefundWebhook(TRADE_NO, "1", "2026-06-20", "OK");

        verify(refundRepository, never()).finalizeAtmRefundSucceeded(any(), any(), any(), any(), any(), any());
        verify(refundRepository, never()).finalizeAtmRefundFailed(any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }
}
