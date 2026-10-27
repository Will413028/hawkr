package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Rescue 退款失敗路徑測試
 *
 * 對應審計報告 §5 F9：之前 commit 的 createRescueRefund 失敗 fallback 把
 * "RESCUE_EXEC_FAIL"（13 字元）寫入 provider_status（VARCHAR(4)），
 * 導致 PostgreSQL "value too long" crash。
 *
 * 鎖死規則：rescue 退款的 PayUni 呼叫拋例外時：
 * 1. provider_status 必須為 null（PayUni 沒回真實狀態碼）
 * 2. 完整錯誤訊息必須塞進 provider_message
 * 3. 不能寫入任何超過 4 字元的字串到 provider_status
 *
 * 與 RefundCloseStatusRoutingTest 不同：那邊測「正常分流」，這邊測「失敗 fallback 是否守住欄位語意」
 */
@ExtendWith(MockitoExtension.class)
class RescueRefundFailurePathTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentEventPublisher eventPublisher;
    @Mock private RefundDomainService domainService;
    @Mock private PaymentClient paymentClient;

    private RefundApplicationService service;
    private PaymentTransaction creditCardExpired;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(901, 0);
        } catch (Exception ignored) {
            // already initialized
        }
    }

    @BeforeEach
    void setUp() {
        service = new RefundApplicationService(
                transactionRepository, refundRepository,
                eventPublisher, domainService, paymentClient
        );

        // rescue 路徑專用：信用卡 + EXPIRED（不是 PAID）
        creditCardExpired = PaymentTransaction.builder()
                .id(1L)
                .tradeNo("RESCUE_TRD_001")
                .orderId("RESCUE_ORD_001")
                .merId("HAWK9")
                .paymentType(PaymentMethod.CREDIT_CARD)
                .amount(500L)
                .status(PaymentTransactionStatus.EXPIRED)
                .providerTradeNo("UNI_99999")
                .version(0)
                .build();

        // 預設：idempotency key 不命中、找得到母交易、無進行中退款
        when(refundRepository.findByIdempotencyKey(eq("rescue:RESCUE_TRD_001")))
                .thenReturn(Optional.empty());
        when(transactionRepository.findCanonicalByTradeNo(eq("RESCUE_TRD_001")))
                .thenReturn(Optional.of(creditCardExpired));
        when(refundRepository.hasProcessingRefund(eq("RESCUE_TRD_001"))).thenReturn(false);
    }

    @Test
    @DisplayName("信用卡 rescue：handleCreditCardRefund 拋例外時 provider_status 必須為 null")
    void rescue_creditCardThrows_providerStatusMustBeNull() {
        // 模擬 PayUni 查詢 API 拋出例外（這是 handleCreditCardRefund 內第一步）
        when(paymentClient.queryTransactionByProviderTradeNo(eq("HAWK9"), eq("UNI_99999")))
                .thenThrow(new RuntimeException("connection refused"));

        var result = service.createRescueRefund(
                "RESCUE_TRD_001",
                500L,
                "RESCUE_RECONCILE_AFTER_EXPIRED",
                "rescue:RESCUE_TRD_001"
        );

        assertEquals(RefundStatus.FAILED.value(), result.finalStatus());
        assertFalse(result.requiresManual());
        assertFalse(result.succeeded());

        // 驗證 updateStatusToFailed 呼叫的參數
        ArgumentCaptor<String> providerStatusCap = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> providerMessageCap = ArgumentCaptor.forClass(String.class);

        verify(refundRepository).updateStatusToFailed(
                anyString(),
                providerStatusCap.capture(),
                providerMessageCap.capture(),
                anyInt()
        );

        // 核心斷言：provider_status 必須是 null（不能塞 RX、RESCUE_EXEC_FAIL、X 等任何字串）
        assertNull(providerStatusCap.getValue(),
                "rescue 失敗 fallback 不可寫入任何字串到 provider_status，必須為 null");

        // 完整錯誤訊息必須塞 provider_message
        String message = providerMessageCap.getValue();
        assertNotNull(message);
        assertTrue(message.contains("RESCUE_EXEC_FAIL"),
                "provider_message 必須含 RESCUE_EXEC_FAIL 標記，便於 audit");
        assertTrue(message.contains("connection refused"),
                "provider_message 必須含原始錯誤原因");
    }

    @Test
    @DisplayName("信用卡 rescue：寫入 provider_status 的字串不可超過 4 字元（VARCHAR(4) 防護）")
    void rescue_providerStatusLengthMustNotExceedColumnLimit() {
        when(paymentClient.queryTransactionByProviderTradeNo(eq("HAWK9"), eq("UNI_99999")))
                .thenThrow(new RuntimeException("any failure"));

        service.createRescueRefund(
                "RESCUE_TRD_001",
                500L,
                "reason",
                "rescue:RESCUE_TRD_001"
        );

        ArgumentCaptor<String> providerStatusCap = ArgumentCaptor.forClass(String.class);
        verify(refundRepository).updateStatusToFailed(
                anyString(),
                providerStatusCap.capture(),
                anyString(),
                anyInt()
        );

        String providerStatus = providerStatusCap.getValue();
        if (providerStatus != null) {
            assertTrue(providerStatus.length() <= 4,
                    "provider_status 寫入長度不可超過 DB 欄位限制 VARCHAR(4)，實際=" + providerStatus.length());
        }
    }

    @Test
    @DisplayName("信用卡 rescue：handleCreditCardRefund 回應失敗（PayUni 真有回應但業務失敗）走 handleRefundFailure")
    void rescue_creditCardBusinessFailure_goesThroughHandleRefundFailure() {
        // PayUni 查 CloseStatus 成功，但回未請款狀態，handleCreditCardRefund 會拋 PaymentBusinessException
        when(paymentClient.queryTransactionByProviderTradeNo(eq("HAWK9"), eq("UNI_99999")))
                .thenReturn(new PayUniApiResponse(
                        "SUCCESS",
                        "查詢成功",
                        Map.of("Result[0][" + PayUniConstants.FIELD_CLOSE_STATUS + "]",
                               PayUniConstants.CLOSE_STATUS_NOT_APPLIED)
                ));

        var result = service.createRescueRefund(
                "RESCUE_TRD_001",
                500L,
                "reason",
                "rescue:RESCUE_TRD_001"
        );

        // 業務例外被 catch → 走 fallback path → provider_status=null
        assertEquals(RefundStatus.FAILED.value(), result.finalStatus());

        ArgumentCaptor<String> providerStatusCap = ArgumentCaptor.forClass(String.class);
        verify(refundRepository).updateStatusToFailed(
                anyString(),
                providerStatusCap.capture(),
                anyString(),
                anyInt()
        );
        assertNull(providerStatusCap.getValue());
    }
}
