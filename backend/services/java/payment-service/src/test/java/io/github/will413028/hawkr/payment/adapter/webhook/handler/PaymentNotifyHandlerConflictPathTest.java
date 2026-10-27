package io.github.will413028.hawkr.payment.adapter.webhook.handler;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;
import io.github.will413028.hawkr.payment.application.service.DuplicatePaymentService;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * issue #1356 §17.7 稽核附帶 — PaymentNotifyHandler.handleConflict 版本衝突健壯化。
 *
 * 對稱 handlePaid/handleFailed（§2A.6.13）：updateToConflict 回 0（version 衝突）時
 * reload 判斷後重試 / 冪等 / 拋重投，禁止靜默回 success 把衝突結果丟掉（否則
 * payment.conflict 不發、狀態不收斂，需手動刪 payment_webhooks 才能重投）。
 *
 * 透過 public handle() 餵「直付終態 PAID + webhook 回 FAILED（isConflict）」payload
 * 驅動 handleConflict。
 */
@ExtendWith(MockitoExtension.class)
class PaymentNotifyHandlerConflictPathTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private PaymentDomainService domainService;
    @Mock private DuplicatePaymentService duplicatePaymentService;
    @Mock private PaymentOutboxWriter outboxWriter;

    private PaymentNotifyHandler handler;

    private static final String TRADE_NO = "O987654321001";
    private static final String PROVIDER_TRADE_NO = "UNI_CONFLICT_1";
    private static final Long ROW_ID = 77L;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(903, 0);
        } catch (Exception ignored) {
            // 已初始化則忽略
        }
    }

    @BeforeEach
    void setUp() {
        handler = new PaymentNotifyHandler(transactionRepository, domainService, duplicatePaymentService, outboxWriter);
    }

    /** 直付（APPLE_PAY）終態 row，isDirectPayment=true 且 isTerminal 觸發衝突檢查。 */
    private PaymentTransaction txWithStatus(PaymentTransactionStatus status, int version) {
        return PaymentTransaction.builder()
                .id(ROW_ID)
                .tradeNo(TRADE_NO)
                .orderId("O987654321")
                .merId("HAWK9")
                .buyerId("2608240571")
                .paymentType(PaymentMethod.APPLE_PAY)
                .amount(500L)
                .status(status)
                .version(version)
                .build();
    }

    /** webhook 回 FAILED（TradeStatus=2），與 DB PAID 衝突；無時間欄位 → 不觸發 updateProviderTimes。 */
    private Map<String, String> conflictPayload() {
        Map<String, String> data = new HashMap<>();
        data.put(PayUniConstants.FIELD_MER_TRADE_NO, TRADE_NO);
        data.put(PayUniConstants.FIELD_TRADE_NO, PROVIDER_TRADE_NO);
        data.put(PayUniConstants.FIELD_TRADE_STATUS, "2");
        data.put(PayUniConstants.FIELD_MESSAGE, "WEBHOOK_SAYS_FAILED");
        return data;
    }

    /** handle() 進 handleConflict 前的共用 stub：lookup 到 PAID 直付 row + webhook 解析 FAILED + 判定衝突。 */
    private void stubLookupAndConflict(PaymentTransaction found) {
        when(transactionRepository.findByProviderTradeNo(PROVIDER_TRADE_NO))
                .thenReturn(Optional.of(found));
        when(domainService.parseProviderStatus("2"))
                .thenReturn(Optional.of(PaymentTransactionStatus.FAILED));
        when(domainService.isConflict(PaymentTransactionStatus.PAID, PaymentTransactionStatus.FAILED))
                .thenReturn(true);
    }

    private void stubConflictEvent(PaymentTransaction eventSource) {
        PaymentDomainEvent.PaymentConflict event = mock(PaymentDomainEvent.PaymentConflict.class);
        when(domainService.createPaymentConflictEvent(eq(eventSource), any(), any())).thenReturn(event);
    }

    @Test
    @DisplayName("a) updateToConflict 第一次成功 → 事務內 outbox 寫入 payment.conflict")
    void firstUpdateSucceeds_publishesConflict() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(1);
        stubConflictEvent(tx);

        WebhookHandleResult result = handler.handle(conflictPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter).enqueueAndDispatch(any(PaymentDomainEvent.PaymentConflict.class));
        verify(transactionRepository).updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0);
        verify(transactionRepository, never()).findById(any());
    }

    @Test
    @DisplayName("b) updated=0 → reload 為 CONFLICT → 冪等 success、不重發")
    void conflictReloadConflict_idempotentNoPublish() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(0);
        when(transactionRepository.findById(ROW_ID))
                .thenReturn(Optional.of(txWithStatus(PaymentTransactionStatus.CONFLICT, 1)));

        WebhookHandleResult result = handler.handle(conflictPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter, never()).enqueueAndDispatch(any());
        verify(transactionRepository, times(1)).updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0);
        verify(domainService, never()).createPaymentConflictEvent(any(), any(), any());
    }

    @Test
    @DisplayName("c) updated=0 → reload 仍 PAID → 新 version 重試成功 → 發 payment.conflict")
    void conflictReloadPaid_retrySucceeds_publishes() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        PaymentTransaction reloaded = txWithStatus(PaymentTransactionStatus.PAID, 1);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(0);
        when(transactionRepository.findById(ROW_ID)).thenReturn(Optional.of(reloaded));
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 1)).thenReturn(1);
        stubConflictEvent(reloaded);

        WebhookHandleResult result = handler.handle(conflictPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(transactionRepository).updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 1);
        verify(outboxWriter).enqueueAndDispatch(any(PaymentDomainEvent.PaymentConflict.class));
        // 事件用 reloaded（新 version）為來源，非原 stale transaction
        verify(domainService).createPaymentConflictEvent(eq(reloaded), any(), any());
    }

    @Test
    @DisplayName("d) updated=0 → reload 仍 PAID → 重試仍 0 → 拋 WebhookRetryableException（不吞）")
    void conflictReloadPaid_retryExhausted_throwsRetryable() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        PaymentTransaction reloaded = txWithStatus(PaymentTransactionStatus.PAID, 1);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(0);
        when(transactionRepository.findById(ROW_ID)).thenReturn(Optional.of(reloaded));
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 1)).thenReturn(0);

        assertThrows(WebhookRetryableException.class, () -> handler.handle(conflictPayload()));
        verify(outboxWriter, never()).enqueueAndDispatch(any());
    }

    @Test
    @DisplayName("e) updated=0 → reload 為其他終態（EXPIRED）→ stateConflict 不覆蓋、不發事件")
    void conflictReloadOtherTerminal_stateConflictNoPublish() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(0);
        when(transactionRepository.findById(ROW_ID))
                .thenReturn(Optional.of(txWithStatus(PaymentTransactionStatus.EXPIRED, 1)));

        WebhookHandleResult result = handler.handle(conflictPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter, never()).enqueueAndDispatch(any());
        verify(transactionRepository, times(1)).updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0);
        verify(domainService, never()).createPaymentConflictEvent(any(), any(), any());
    }

    @Test
    @DisplayName("f) updated=0 → reload 找不到 row → 拋 WebhookRetryableException")
    void conflictReloadMissing_throwsRetryable() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.PAID, 0);
        stubLookupAndConflict(tx);
        when(transactionRepository.updateToConflict(TRADE_NO, "2", "WEBHOOK_SAYS_FAILED", 0)).thenReturn(0);
        when(transactionRepository.findById(ROW_ID)).thenReturn(Optional.empty());

        assertThrows(WebhookRetryableException.class, () -> handler.handle(conflictPayload()));
        verify(outboxWriter, never()).enqueueAndDispatch(any());
    }
}
