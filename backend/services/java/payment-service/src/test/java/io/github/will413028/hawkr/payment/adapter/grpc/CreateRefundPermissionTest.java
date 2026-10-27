package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;
import io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.grpc.CreateRefundRequest;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.transaction.support.TransactionTemplate;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * P0 退款漏洞修復（05 §5.5.3）：createRefund 收斂為 internal-only。
 *
 * 驗證外部（非 internal service）呼叫 createRefund 一律 PERMISSION_DENIED，
 * 且不進入 RefundApplicationService.createRefund（買家無法繞過售後審核自助退款）。
 *
 * 單元測試環境無 gRPC Context → INTERNAL_CALL.get()=null → isInternalCall()=false，
 * 恰好模擬「外部帶 JWT 前端直呼」情境。
 */
@ExtendWith(MockitoExtension.class)
class CreateRefundPermissionTest {

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

    @Test
    @DisplayName("外部（非 internal）呼叫 createRefund → PERMISSION_DENIED 且不進退款流程")
    void externalCall_isRejectedWithPermissionDenied() {
        PaymentGrpcService service = newService();

        CreateRefundRequest request = CreateRefundRequest.newBuilder()
                .setTradeNo("TRD_001")
                .setAmount("100")
                .setReason("測試")
                .setRefundNo("case-1")
                .build();

        @SuppressWarnings("unchecked")
        StreamObserver<CreateRefundResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<CreateRefundResponse> captor = ArgumentCaptor.forClass(CreateRefundResponse.class);

        service.createRefund(request, observer);

        // 核心不變式：退款流程完全未被觸及
        verify(refundService, never()).createRefund(any());

        // 回應為 PERMISSION_DENIED（Gateway 塌縮為 HTTP 403）
        verify(observer).onNext(captor.capture());
        assertEquals(GrpcResponseCode.PERMISSION_DENIED.code(), captor.getValue().getCode());
    }
}
