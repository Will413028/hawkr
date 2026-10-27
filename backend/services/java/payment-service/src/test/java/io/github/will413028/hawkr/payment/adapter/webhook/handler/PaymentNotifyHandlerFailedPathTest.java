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
 * issue #1288 §17.7 — PaymentNotifyHandler.handleFailed 版本衝突健壯化。
 *
 * 對稱 handlePaid：updateToFailed 回 0（version 衝突）時 reload 判斷後
 * 重試 / 冪等 / 拋重投，禁止靜默回 success 把失敗結果丟掉（否則 payment.failed
 * 不發、order 卡 PAYMENT_PENDING）。
 *
 * 透過 public handle() 餵「付款失敗」payload（TradeStatus=2）驅動 handleFailed。
 */
@ExtendWith(MockitoExtension.class)
class PaymentNotifyHandlerFailedPathTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private PaymentDomainService domainService;
    @Mock private DuplicatePaymentService duplicatePaymentService;
    @Mock private PaymentOutboxWriter outboxWriter;

    private PaymentNotifyHandler handler;

    private static final String TRADE_NO = "O123456789001";
    private static final String PROVIDER_TRADE_NO = "UNI_FAIL_1";
    private static final Long ROW_ID = 42L;

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
        handler = new PaymentNotifyHandler(transactionRepository, domainService, duplicatePaymentService, outboxWriter);
    }

    private PaymentTransaction txWithStatus(PaymentTransactionStatus status, int version) {
        return PaymentTransaction.builder()
                .id(ROW_ID)
                .tradeNo(TRADE_NO)
                .orderId("O123456789")
                .merId("HAWK9")
                .buyerId("2608240571")
                .paymentType(PaymentMethod.ATM)
                .amount(500L)
                .status(status)
                .version(version)
                .build();
    }

    /** 建構「付款失敗」webhook payload（TradeStatus=2，無時間欄位 → 不觸發 updateProviderTimes）。 */
    private Map<String, String> failedPayload() {
        Map<String, String> data = new HashMap<>();
        data.put(PayUniConstants.FIELD_MER_TRADE_NO, TRADE_NO);
        data.put(PayUniConstants.FIELD_TRADE_NO, PROVIDER_TRADE_NO);
        data.put(PayUniConstants.FIELD_TRADE_STATUS, "2");
        data.put(PayUniConstants.FIELD_MESSAGE, "CARD_DECLINED");
        return data;
    }

    /** handle() 進 handleFailed 前的共用 stub：lookup 到 row + parse 出 FAILED。 */
    private void stubLookupAndParse(PaymentTransaction found) {
        when(transactionRepository.findByProviderTradeNo(PROVIDER_TRADE_NO))
                .thenReturn(Optional.of(found));
        when(domainService.parseProviderStatus("2"))
                .thenReturn(Optional.of(PaymentTransactionStatus.FAILED));
    }

    @Test
    @DisplayName("a) updateToFailed 第一次成功 → 事務內 outbox 寫入 payment.failed")
    void firstUpdateSucceeds_publishesFailed() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.INIT, 0);
        stubLookupAndParse(tx);
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0))
                .thenReturn(1);
        PaymentDomainEvent.PaymentFailed event = new PaymentDomainEvent.PaymentFailed(
                TRADE_NO, "O123456789", "2608240571", 500L, "CARD_DECLINED", null);
        when(domainService.createPaymentFailedEvent(tx, "CARD_DECLINED")).thenReturn(event);

        WebhookHandleResult result = handler.handle(failedPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        // §5.10.1 收斂點 A：事件經 outbox writer 在事務內寫入（取代原事務外直發 mqEvent）
        verify(outboxWriter).enqueueAndDispatch(event);
        assertEquals("O123456789001", event.aggregateId());
        verify(transactionRepository).updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0);
        verify(transactionRepository, never()).findById(any());
        verify(domainService).createPaymentFailedEvent(tx, "CARD_DECLINED");
    }

    @Test
    @DisplayName("b) updated=0 → reload 為 FAILED → 冪等 success、不發第二次事件")
    void conflictReloadFailed_idempotentNoPublish() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.INIT, 0);
        stubLookupAndParse(tx);
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0))
                .thenReturn(0);
        when(transactionRepository.findById(ROW_ID))
                .thenReturn(Optional.of(txWithStatus(PaymentTransactionStatus.FAILED, 1)));

        WebhookHandleResult result = handler.handle(failedPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter, never()).enqueueAndDispatch(any());  // 冪等分支不得重寫 payment.failed
        verify(transactionRepository, times(1))
                .updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0);
        verify(domainService, never()).createPaymentFailedEvent(any(), any());
    }

    @Test
    @DisplayName("c) updated=0 → reload 為 PAID → log 衝突、回 success、不覆蓋、不發 failed 事件")
    void conflictReloadPaid_noOverwriteNoPublish() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.INIT, 0);
        stubLookupAndParse(tx);
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0))
                .thenReturn(0);
        when(transactionRepository.findById(ROW_ID))
                .thenReturn(Optional.of(txWithStatus(PaymentTransactionStatus.PAID, 1)));

        WebhookHandleResult result = handler.handle(failedPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter, never()).enqueueAndDispatch(any());  // 已 PAID 時不得寫 payment.failed（否則覆蓋成功結果）
        // 只呼叫過一次（第一次 version=0），不得用 reloaded.version 重推 FAILED 蓋掉 PAID
        verify(transactionRepository, times(1)).updateToFailed(any(), any(), any(), any(), anyInt());
        verify(domainService, never()).createPaymentFailedEvent(any(), any());
    }

    @Test
    @DisplayName("d) updated=0 → reload 仍 INIT → 用新 version 重試成功 → 發 payment.failed")
    void conflictReloadInit_retrySucceedsPublishes() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.INIT, 0);
        PaymentTransaction reloaded = txWithStatus(PaymentTransactionStatus.INIT, 3);
        stubLookupAndParse(tx);
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0))
                .thenReturn(0);
        when(transactionRepository.findById(ROW_ID)).thenReturn(Optional.of(reloaded));
        // 重試用 reloaded.version()=3 命中
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 3))
                .thenReturn(1);
        PaymentDomainEvent.PaymentFailed event = new PaymentDomainEvent.PaymentFailed(
                TRADE_NO, "O123456789", "2608240571", 500L, "CARD_DECLINED", null);
        when(domainService.createPaymentFailedEvent(reloaded, "CARD_DECLINED")).thenReturn(event);

        WebhookHandleResult result = handler.handle(failedPayload());

        assertEquals(WebhookProcessStatus.SUCCESS.value(), result.processStatus());
        verify(outboxWriter).enqueueAndDispatch(event);  // reload 重試成功應寫 payment.failed
        verify(transactionRepository).updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 3);
        // 事件用 reloaded（新 version）建構，非陳舊 tx
        verify(domainService).createPaymentFailedEvent(reloaded, "CARD_DECLINED");
        verify(domainService, never()).createPaymentFailedEvent(tx, "CARD_DECLINED");
    }

    @Test
    @DisplayName("e) updated=0 → reload 仍 INIT → 重試又 0 → 拋 WebhookRetryableException")
    void conflictReloadInit_retryExhausted_throws() {
        PaymentTransaction tx = txWithStatus(PaymentTransactionStatus.INIT, 0);
        PaymentTransaction reloaded = txWithStatus(PaymentTransactionStatus.INIT, 3);
        stubLookupAndParse(tx);
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0))
                .thenReturn(0);
        when(transactionRepository.findById(ROW_ID)).thenReturn(Optional.of(reloaded));
        when(transactionRepository.updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 3))
                .thenReturn(0);

        assertThrows(WebhookRetryableException.class, () -> handler.handle(failedPayload()));

        // 兩次 updateToFailed 都嘗試過（version 0 與 3），不吞失敗結果
        verify(transactionRepository).updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 0);
        verify(transactionRepository).updateToFailed(TRADE_NO, "2", "CARD_DECLINED", PROVIDER_TRADE_NO, 3);
        verify(domainService, never()).createPaymentFailedEvent(any(), any());
        verify(outboxWriter, never()).enqueueAndDispatch(any());
    }
}
