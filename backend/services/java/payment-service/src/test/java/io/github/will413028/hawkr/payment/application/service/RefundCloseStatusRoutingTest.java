package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.application.dto.CreateRefundRequest;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 信用卡退款 CloseStatus 分流邏輯測試
 *
 * 驗證 RefundApplicationService.createRefund() 中信用卡分支：
 * 依 PayUni 查詢 API 回傳的 CloseStatus 決定走取消授權或退款
 */
@ExtendWith(MockitoExtension.class)
class RefundCloseStatusRoutingTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentEventPublisher eventPublisher;
    @Mock private RefundDomainService domainService;
    @Mock private PaymentClient paymentClient;

    private RefundApplicationService service;
    private PaymentTransaction creditCardTransaction;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(900, 0);
        } catch (Exception ignored) {
            // 已初始化則忽略
        }
    }

    @BeforeEach
    void setUp() {
        service = new RefundApplicationService(
                transactionRepository, refundRepository,
                eventPublisher, domainService, paymentClient
        );

        creditCardTransaction = PaymentTransaction.builder()
                .id(1L)
                .tradeNo("TRD_001")
                .orderId("ORD_001")
                .merId("HAWK9")
                .paymentType(PaymentMethod.CREDIT_CARD)
                .amount(500L)
                .status(PaymentTransactionStatus.PAID)
                .providerTradeNo("UNI_12345")
                .version(0)
                .build();
    }

    private void setupCommonMocks() {
        // §5.5.5 L1：createRefund 改用 FOR UPDATE 版本讀原交易
        when(transactionRepository.findCanonicalByTradeNoForUpdate("TRD_001"))
                .thenReturn(Optional.of(creditCardTransaction));
        when(domainService.canRequestRefund(any())).thenReturn(true);
        when(refundRepository.hasProcessingRefund("TRD_001")).thenReturn(false);
        when(refundRepository.sumRefundedAmountByTradeNo("TRD_001")).thenReturn(0L);
        when(domainService.validateRefundAmount(anyLong(), anyLong(), anyLong()))
                .thenReturn(Optional.empty());
        when(refundRepository.save(any())).thenAnswer(invocation -> invocation.getArgument(0));
    }

    private PayUniApiResponse mockQueryResponse(String closeStatus) {
        return new PayUniApiResponse("SUCCESS", "查詢成功", Map.of(
                "Result[0][" + PayUniConstants.FIELD_CLOSE_STATUS + "]", closeStatus,
                "Result[0][TradeStatus]", "1"
        ));
    }

    private PayUniApiResponse mockSuccessResponse() {
        return new PayUniApiResponse("SUCCESS", "處理成功", Map.of(
                "TradeNo", "UNI_12345",
                "TradeStatus", "1"
        ));
    }

    private void mockRefundCompletion() {
        // updateStatusToSucceeded(refundNo, providerRefundNo, providerStatus, providerMessage, bankDate, completedAt, version)
        when(refundRepository.updateStatusToSucceeded(any(), any(), any(), any(), any(), any(), anyInt()))
                .thenReturn(1);
        when(refundRepository.findByRefundNo(any())).thenReturn(Optional.of(
                RefundTransaction.create(1L, "TRD_001", "ORD_001", 500L, "測試")));
    }

    // ========== CloseStatus=1 → 取消授權 ==========

    @Test
    @DisplayName("CloseStatus=1（請款申請中）→ 呼叫 cancelAuth 取消授權")
    void closeStatus1_shouldCallCancelAuth() {
        setupCommonMocks();
        mockRefundCompletion();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(mockQueryResponse("1"));
        when(paymentClient.cancelAuthorization("HAWK9", "UNI_12345"))
                .thenReturn(mockSuccessResponse());

        service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試退款"));

        verify(paymentClient).cancelAuthorization("HAWK9", "UNI_12345");
        verify(paymentClient, never()).refund(any(), any(), any(), anyLong());
    }

    // ========== CloseStatus=2 → 退款 ==========

    @Test
    @DisplayName("CloseStatus=2（請款成功）→ 呼叫 refund 退款")
    void closeStatus2_shouldCallRefund() {
        setupCommonMocks();
        mockRefundCompletion();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(mockQueryResponse("2"));
        when(paymentClient.refund(eq("HAWK9"), eq("UNI_12345"), any(), eq(500L)))
                .thenReturn(mockSuccessResponse());

        service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試退款"));

        verify(paymentClient).refund(eq("HAWK9"), eq("UNI_12345"), any(), eq(500L));
        verify(paymentClient, never()).cancelAuthorization(any(), any());
    }

    // ========== CloseStatus=7 → 退款 ==========

    @Test
    @DisplayName("CloseStatus=7（請款處理中）→ 呼叫 refund 退款")
    void closeStatus7_shouldCallRefund() {
        setupCommonMocks();
        mockRefundCompletion();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(mockQueryResponse("7"));
        when(paymentClient.refund(eq("HAWK9"), eq("UNI_12345"), any(), eq(500L)))
                .thenReturn(mockSuccessResponse());

        service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試退款"));

        verify(paymentClient).refund(eq("HAWK9"), eq("UNI_12345"), any(), eq(500L));
        verify(paymentClient, never()).cancelAuthorization(any(), any());
    }

    // ========== CloseStatus=3 → 異常 ==========

    @Test
    @DisplayName("CloseStatus=3（請款取消）→ 拋 IllegalStateException")
    void closeStatus3_shouldThrowException() {
        setupCommonMocks();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(mockQueryResponse("3"));

        var ex = assertThrows(PaymentBusinessException.class,
                () -> service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試")));
        assertEquals(GrpcResponseCode.CLOSE_STATUS_CANCELED.code(), ex.code());

        verify(paymentClient, never()).refund(any(), any(), any(), anyLong());
        verify(paymentClient, never()).cancelAuthorization(any(), any());
    }

    // ========== CloseStatus=9 → 異常 ==========

    @Test
    @DisplayName("CloseStatus=9（未申請）→ 拋 IllegalStateException")
    void closeStatus9_shouldThrowException() {
        setupCommonMocks();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(mockQueryResponse("9"));

        var ex = assertThrows(PaymentBusinessException.class,
                () -> service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試")));
        assertEquals(GrpcResponseCode.CLOSE_STATUS_NOT_APPLIED.code(), ex.code());

        verify(paymentClient, never()).refund(any(), any(), any(), anyLong());
        verify(paymentClient, never()).cancelAuthorization(any(), any());
    }

    // ========== 查詢失敗 ==========

    @Test
    @DisplayName("查詢 API 失敗 → 拋 IllegalStateException")
    void queryFailed_shouldThrowException() {
        setupCommonMocks();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(new PayUniApiResponse("QUERY_FAIL", "查詢失敗", null));

        var ex = assertThrows(PaymentBusinessException.class,
                () -> service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試")));
        assertEquals(GrpcResponseCode.QUERY_TRANSACTION_FAILED.code(), ex.code());
    }

    // ========== CloseStatus 為 null ==========

    @Test
    @DisplayName("CloseStatus 為 null → 拋 IllegalStateException")
    void closeStatusNull_shouldThrowException() {
        setupCommonMocks();

        when(paymentClient.queryTransactionByProviderTradeNo("HAWK9", "UNI_12345"))
                .thenReturn(new PayUniApiResponse("SUCCESS", "查詢成功", Map.of()));

        var ex = assertThrows(PaymentBusinessException.class,
                () -> service.createRefund(new CreateRefundRequest("TRD_001", 500L, "測試")));
        assertEquals(GrpcResponseCode.CLOSE_STATUS_UNAVAILABLE.code(), ex.code());
    }
}
