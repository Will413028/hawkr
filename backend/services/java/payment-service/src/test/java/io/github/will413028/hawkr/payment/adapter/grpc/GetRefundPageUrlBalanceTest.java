package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.application.dto.PaymentDetailResponse;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;
import io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.grpc.GetRefundPageUrlRequest;
import io.github.will413028.hawkr.payment.grpc.GetRefundPageUrlResponse;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.payment.model.UppFormData;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * getRefundPageUrl 可退餘額校驗（§5.5.5 L2「排除本筆」回歸修復）。
 *
 * REF-02 修復把 sumRefundedAmount 改含在途('0','1','2') 後，ATM 退款流程「先預建一筆 REQUESTED('0')
 * refund row」使本筆已存在於表中。若沿用不排除版，getRefundPageUrl 會把本筆自己算進已退累計，
 * 全額 / 大額 ATM 退款 remaining=0 → 誤報 REFUND_AMOUNT_EXCEEDED → 買家永遠拿不到退款頁。
 *
 * 本測試驗證：改用「排除本筆活躍退款」的累計後，ATM 全額退款可正常取得頁面 URL；
 * 同時防線仍在（其他退款把餘額用盡時仍擋）。
 */
@ExtendWith(MockitoExtension.class)
class GetRefundPageUrlBalanceTest {

    @Mock private PaymentApplicationService paymentService;
    @Mock private PaymentCloseService paymentCloseService;
    @Mock private PaymentPageService pageService;
    @Mock private RefundApplicationService refundService;
    @Mock private PayUniCryptoService cryptoService;
    @Mock private PaymentNotifyHandler notifyHandler;
    @Mock private PaymentWebhookMapper webhookMapper;
    @Mock private TransactionTemplate transactionTemplate;
    @Mock private ObjectMapper objectMapper;
    @Mock private PayUniAuditLogger auditLogger;
    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentAdminQueryService adminQueryService;
    @Mock private PaymentLogMapper paymentLogMapper;
    @Mock private ApplePayMerchantValidator applePayMerchantValidator;

    private PaymentGrpcService newService() {
        return new PaymentGrpcService(
                paymentService, paymentCloseService, pageService, refundService,
                cryptoService, notifyHandler, webhookMapper,
                transactionTemplate, objectMapper, auditLogger, transactionRepository,
                refundRepository, adminQueryService, paymentLogMapper, applePayMerchantValidator,
                mock(io.github.will413028.hawkr.payment.agentfee.AgentFeeService.class));
    }

    private PaymentDetailResponse paidAtm(String tradeNo, long amount) {
        return PaymentDetailResponse.from(PaymentTransaction.builder()
                .tradeNo(tradeNo)
                .orderId("O100")
                .merId("HAWK9")
                .paymentType(PaymentMethod.ATM)
                .amount(amount)
                .status(PaymentTransactionStatus.PAID)
                .providerTradeNo("UNI_1")
                .build());
    }

    private RefundTransaction activeRefund(String refundNo, String tradeNo, long amount) {
        return RefundTransaction.builder()
                .refundNo(refundNo)
                .tradeNo(tradeNo)
                .orderId("O100")
                .amount(amount)
                .status(RefundStatus.REQUESTED)
                .build();
    }

    @Test
    @DisplayName("ATM 全額退款：預建 REQUESTED 本筆存在 → 排除本筆 → 成功取得退款頁 URL（不誤擋）")
    void atmFullRefund_prebuiltRow_getsPageUrl() {
        PaymentGrpcService service = newService();
        String tradeNo = "O1001";

        when(paymentService.getPaymentDetail(tradeNo)).thenReturn(Optional.of(paidAtm(tradeNo, 1_000L)));
        // 本筆 = 預建的全額 REQUESTED 退款
        when(refundRepository.findActiveByTradeNo(tradeNo))
                .thenReturn(Optional.of(activeRefund("RFD_ACTIVE", tradeNo, 1_000L)));
        // 排除本筆 → 其他退款已用額度 = 0
        when(refundRepository.sumRefundedAmountByTradeNoExcluding(tradeNo, "RFD_ACTIVE")).thenReturn(0L);
        when(paymentService.buildRefundPageFormData(eq("HAWK9"), eq("UNI_1"), eq(1_000L), eq("https://return")))
                .thenReturn(mock(UppFormData.class));
        when(pageService.buildRedirectUrl(any())).thenReturn("https://payuni/refundpage");

        GetRefundPageUrlRequest request = GetRefundPageUrlRequest.newBuilder()
                .setTradeNo(tradeNo).setAmount("1000").setReturnUrl("https://return").build();

        @SuppressWarnings("unchecked")
        StreamObserver<GetRefundPageUrlResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetRefundPageUrlResponse> captor = ArgumentCaptor.forClass(GetRefundPageUrlResponse.class);

        service.getRefundPageUrl(request, observer);

        verify(observer).onNext(captor.capture());
        GetRefundPageUrlResponse resp = captor.getValue();
        assertEquals(GrpcResponseCode.SUCCESS.code(), resp.getCode(),
                "全額 ATM 退款不應被誤報 REFUND_AMOUNT_EXCEEDED");
        assertEquals("https://payuni/refundpage", resp.getData().getRedirectUrl());
        // 確認確實走「排除本筆」查詢
        verify(refundRepository).sumRefundedAmountByTradeNoExcluding(tradeNo, "RFD_ACTIVE");
    }

    @Test
    @DisplayName("防線仍在：其他退款已用盡餘額時，超額請求仍回 REFUND_AMOUNT_EXCEEDED")
    void guardStillBlocksWhenOtherRefundsExhaustBalance() {
        PaymentGrpcService service = newService();
        String tradeNo = "O1002";

        when(paymentService.getPaymentDetail(tradeNo)).thenReturn(Optional.of(paidAtm(tradeNo, 1_000L)));
        when(refundRepository.findActiveByTradeNo(tradeNo))
                .thenReturn(Optional.of(activeRefund("RFD_ACTIVE", tradeNo, 700L)));
        // 排除本筆後，其他退款已用 400 → remaining = 1000-400 = 600 < 請求 700
        when(refundRepository.sumRefundedAmountByTradeNoExcluding(tradeNo, "RFD_ACTIVE")).thenReturn(400L);

        GetRefundPageUrlRequest request = GetRefundPageUrlRequest.newBuilder()
                .setTradeNo(tradeNo).setAmount("700").setReturnUrl("https://return").build();

        @SuppressWarnings("unchecked")
        StreamObserver<GetRefundPageUrlResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetRefundPageUrlResponse> captor = ArgumentCaptor.forClass(GetRefundPageUrlResponse.class);

        service.getRefundPageUrl(request, observer);

        verify(observer).onNext(captor.capture());
        assertEquals(GrpcResponseCode.REFUND_AMOUNT_EXCEEDED.code(), captor.getValue().getCode());
        // 超額 → 未進入組頁面 URL
        verify(paymentService, never()).buildRefundPageFormData(any(), any(), anyLong(), any());
    }
}
