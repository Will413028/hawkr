package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
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
 * issue #1356 — ATM 退款受理 Notify 找不到 active 退款的處置（05 §5.5.4.3）。
 *
 * 驗證 handleAtmRefundAccepted 回空分支（不再靜默 warn，改 findLatest 判定）：
 * - active present → markAtmRefundAccepted（既有分支，不受本次修改影響）
 * - 無 active + 最新 SUCCEEDED → 終態冪等略過，不寫 mark、不 throw
 * - 無 active + 最新 FAILED    → 終態冪等略過，不寫 mark、不 throw
 * - 無 active + 查無 row       → 孤兒受理，不寫 mark、不 throw（記 ERROR）
 * - 無 active + 最新 REQUESTED → 非預期，不寫 mark、不 throw（記 ERROR）
 *
 * 通則：回空的任何情境皆不呼叫 markAtmRefundAccepted、皆不 throw（受理里程碑不可重試，
 * 避免 consumer requeue 造成 infinite-loop）。
 */
@ExtendWith(MockitoExtension.class)
class AtmRefundAcceptedNoActiveTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentEventPublisher eventPublisher;
    @Mock private RefundDomainService domainService;
    @Mock private PaymentClient paymentClient;

    private RefundApplicationService service;

    private static final String TRADE_NO = "TRD_ATM_ACC_1";
    private static final String ESTIMATED = "2026-06-20";

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(902, 0);
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
    @DisplayName("active present → markAtmRefundAccepted（既有分支不受影響）")
    void active_marksAccepted() {
        RefundTransaction refund = refundWithStatus(RefundStatus.REQUESTED);
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.of(refund));
        when(refundRepository.markAtmRefundAccepted(eq("RFD_1"), eq("SUCCESS"), any(), any()))
                .thenReturn(1);

        service.handleAtmRefundAccepted(TRADE_NO, ESTIMATED, "ACCEPTED");

        verify(refundRepository).markAtmRefundAccepted(eq("RFD_1"), eq("SUCCESS"), any(), any());
        verify(refundRepository, never()).findLatestByTradeNo(any());
    }

    @Test
    @DisplayName("無 active + 最新 SUCCEEDED → 終態冪等略過，不寫 mark、不 throw")
    void noActive_latestSucceeded_idempotentNoMark() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.SUCCEEDED)));

        service.handleAtmRefundAccepted(TRADE_NO, ESTIMATED, "ACCEPTED");

        verify(refundRepository, never()).markAtmRefundAccepted(any(), any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 最新 FAILED → 終態冪等略過，不寫 mark、不 throw")
    void noActive_latestFailed_idempotentNoMark() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.FAILED)));

        service.handleAtmRefundAccepted(TRADE_NO, ESTIMATED, "ACCEPTED");

        verify(refundRepository, never()).markAtmRefundAccepted(any(), any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 查無 row → 孤兒受理，不寫 mark、不 throw")
    void noActive_noRow_orphanNoMark() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO)).thenReturn(Optional.empty());

        service.handleAtmRefundAccepted(TRADE_NO, ESTIMATED, "ACCEPTED");

        verify(refundRepository, never()).markAtmRefundAccepted(any(), any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }

    @Test
    @DisplayName("無 active + 最新 REQUESTED（非預期矛盾）→ 不寫 mark、不 throw")
    void noActive_latestRequested_unexpectedNoMark() {
        when(refundRepository.findActiveByTradeNo(TRADE_NO)).thenReturn(Optional.empty());
        when(refundRepository.findLatestByTradeNo(TRADE_NO))
                .thenReturn(Optional.of(refundWithStatus(RefundStatus.REQUESTED)));

        service.handleAtmRefundAccepted(TRADE_NO, ESTIMATED, "ACCEPTED");

        verify(refundRepository, never()).markAtmRefundAccepted(any(), any(), any(), any());
        verify(eventPublisher, never()).publish(any());
    }
}
