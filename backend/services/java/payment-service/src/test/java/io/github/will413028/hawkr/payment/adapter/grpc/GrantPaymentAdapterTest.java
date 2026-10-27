package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;
import io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.GrantResponseRules;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentRequest;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * §5.6 P0-3a / §5.6.1：gRPC adapter 依內層回傳的結構化 grantOutcome 決定成敗。
 *
 * <p>write-then-return 讓內層 grantPayment 失敗時正常 return（不 throw），adapter 在 @Transactional 外
 * 依 grantOutcome：GRANTED / PROVIDER_TOOK_OVER → 正常回應（帶 grant_outcome）；null（PENDING_CONFIRM /
 * PayUni 錯誤碼）→ throw gRPC error（維持 Order task「收 gRPC error → 重試」語意）。
 */
@ExtendWith(MockitoExtension.class)
class GrantPaymentAdapterTest {

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

    private GrantPaymentResponse invoke(GrantPaymentResult stubbed) {
        PaymentGrpcService service = newService();
        when(paymentService.grantPayment("O123")).thenReturn(stubbed);

        @SuppressWarnings("unchecked")
        StreamObserver<GrantPaymentResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GrantPaymentResponse> captor = ArgumentCaptor.forClass(GrantPaymentResponse.class);

        service.grantPayment(GrantPaymentRequest.newBuilder().setOrderNo("O123").build(), observer);

        org.mockito.Mockito.verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("內層回 PENDING_CONFIRM（grantOutcome=null）→ adapter throw gRPC error（非 SUCCESS code）")
    void pendingConfirm_throwsGrpcError() {
        GrantPaymentResult pending = new GrantPaymentResult(
                "O123001", "O123", "UNI_123", 500L, null,
                PayUniConstants.PENDING_CONFIRM, "撥款回應模稜兩可", null);

        GrantPaymentResponse resp = invoke(pending);

        assertEquals(GrpcResponseCode.GRANT_FAILED.code(), resp.getCode(), "PENDING_CONFIRM 須回 GRANT_FAILED（Order 收 gRPC error → 重試）");
        assertNotEquals(GrpcResponseCode.SUCCESS.code(), resp.getCode());
    }

    @Test
    @DisplayName("內層回 PayUni 錯誤碼（grantOutcome=null）→ adapter throw gRPC error")
    void errorCode_throwsGrpcError() {
        GrantPaymentResult err = new GrantPaymentResult(
                "O123001", "O123", "UNI_123", 500L, null,
                "GRANT99999", "餘額不足", null);

        GrantPaymentResponse resp = invoke(err);

        assertEquals(GrpcResponseCode.GRANT_FAILED.code(), resp.getCode());
    }

    @Test
    @DisplayName("內層回 SUCCESS（grantOutcome=GRANTED）→ adapter 回 gRPC success 帶 grant_outcome")
    void success_returnsGrpcSuccess() {
        GrantPaymentResult ok = new GrantPaymentResult(
                "O123001", "O123", "UNI_123", 500L, LocalDateTime.now(),
                PayUniConstants.RESPONSE_SUCCESS, "撥款指示成功",
                GrantResponseRules.GRANT_OUTCOME_GRANTED);

        GrantPaymentResponse resp = invoke(ok);

        assertEquals(GrpcResponseCode.SUCCESS.code(), resp.getCode());
        assertEquals(GrantResponseRules.GRANT_OUTCOME_GRANTED, resp.getGrantOutcome());
    }

    @Test
    @DisplayName("內層回 PROVIDER_TOOK_OVER → adapter 回正常回應（非錯誤）帶 grant_outcome、granted_at 空字串")
    void providerTookOver_returnsNormalResponseWithOutcome() {
        GrantPaymentResult tookOver = new GrantPaymentResult(
                "O123001", "O123", "UNI_123", 500L, null,
                "GRANT30023", "PayUni 已接管撥款（退款交易）",
                GrantResponseRules.GRANT_OUTCOME_PROVIDER_TOOK_OVER);

        GrantPaymentResponse resp = invoke(tookOver);

        assertEquals(GrpcResponseCode.SUCCESS.code(), resp.getCode(), "接管為預期收斂結果，不得回 gRPC error");
        assertEquals(GrantResponseRules.GRANT_OUTCOME_PROVIDER_TOOK_OVER, resp.getGrantOutcome());
        assertEquals("", resp.getData().getGrantedAt(), "granted_at 刻意保持 NULL → wire 上為空字串");
    }
}
