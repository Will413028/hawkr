package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.application.dto.PaymentDetailResponse;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;
import io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.grpc.GetPaymentHistoryRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentHistoryResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentResponse;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Context;
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
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 釘住 payment-service 買家歸屬檢查的跨 repo 契約：x-account-id 在這條路徑上承載的是
 * <b>已解析的現任 buyer_id</b>，不是 raw account_id（見
 * {@code PaymentGrpcService.requireCallerBuyerId} 的 javadoc）。
 *
 * <p>fixture 的 raw account_id 與 buyer_id 刻意是不同數字——同值 fixture 無法區分
 * 「檢查真的通過」與「剛好相等」，本 bug class 過去多輪稽核都是這樣漏掉的。
 *
 * <p>單元測試環境無 gRPC Context → {@code INTERNAL_CALL.get()} 為 null →
 * {@code isInternalCall()} 為 false，恰好模擬「gateway 帶 JWT 呼叫」情境；需要模擬
 * 內部服務轉呼叫時再顯式塞 {@code INTERNAL_CALL}。
 */
@ExtendWith(MockitoExtension.class)
class PaymentOwnershipBuyerIdContractTest {

    private static final long RAW_ACCOUNT_ID = 651978000L;            // JWT 的 account_id
    private static final long RESOLVED_BUYER_ID = 7489534863566774901L; // 已解析的現任 buyer_id
    private static final String TRADE_NO = "O488996314001";
    private static final String ORDER_ID = "O488996314";

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

    /** 一筆屬於 RESOLVED_BUYER_ID 的交易；只填 toPaymentDetail 會無條件讀的欄位，其餘留 null。 */
    private static PaymentDetailResponse txnOwnedByResolvedBuyer() {
        return new PaymentDetailResponse(
                TRADE_NO, "O488996314", "HAWK2662192214", String.valueOf(RESOLVED_BUYER_ID),
                PaymentMethod.ATM, 1276L, "測試商品", PaymentTransactionStatus.INIT,
                null, null, null, null, null, null,
                null, null, null, null, null,
                false, null, null, null, null);
    }

    private GetPaymentResponse callGetPaymentUnder(Context ctx) {
        PaymentGrpcService service = newService();
        GetPaymentRequest request = GetPaymentRequest.newBuilder().setTradeNo(TRADE_NO).build();

        @SuppressWarnings("unchecked")
        StreamObserver<GetPaymentResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetPaymentResponse> captor = ArgumentCaptor.forClass(GetPaymentResponse.class);

        Context previous = ctx.attach();
        try {
            service.getPayment(request, observer);
        } finally {
            ctx.detach(previous);
        }

        verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    private GetPaymentHistoryResponse callGetPaymentHistoryUnder(Context ctx) {
        PaymentGrpcService service = newService();
        GetPaymentHistoryRequest request = GetPaymentHistoryRequest.newBuilder().setOrderId(ORDER_ID).build();

        @SuppressWarnings("unchecked")
        StreamObserver<GetPaymentHistoryResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetPaymentHistoryResponse> captor = ArgumentCaptor.forClass(GetPaymentHistoryResponse.class);

        Context previous = ctx.attach();
        try {
            service.getPaymentHistory(request, observer);
        } finally {
            ctx.detach(previous);
        }

        verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("帶已解析 buyer_id 的買家呼叫 → 200，回傳交易")
    void buyerCallerWithResolvedBuyerId_getsTransaction() {
        when(paymentService.getPaymentDetail(TRADE_NO)).thenReturn(Optional.of(txnOwnedByResolvedBuyer()));

        GetPaymentResponse response = callGetPaymentUnder(Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, RESOLVED_BUYER_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, "1"));

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertEquals(String.valueOf(RESOLVED_BUYER_ID), response.getData().getBuyerId());
    }

    @Test
    @DisplayName("帶 raw account_id 的買家呼叫 → NOT_FOUND（與『交易不存在』無法區分）")
    void buyerCallerWithRawAccountId_isDeniedAsNotFound() {
        when(paymentService.getPaymentDetail(TRADE_NO)).thenReturn(Optional.of(txnOwnedByResolvedBuyer()));

        GetPaymentResponse response = callGetPaymentUnder(Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, RAW_ACCOUNT_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, "1"));

        assertEquals(GrpcResponseCode.NOT_FOUND.code(), response.getCode(),
                "raw account_id 與 payment_transactions.buyer_id 永遠沒有數值關係，必須被拒；"
                        + "這個回應刻意與『交易不存在』一致（防枚舉），所以呼叫端接錯線時只會靜默降級——"
                        + "server 端的 buyerMismatch WARN log 是唯一線索");
        assertEquals("交易不存在", response.getMessage());
    }

    @Test
    @DisplayName("內部呼叫傳播了買家 account-type 卻沒帶 buyer_id → UNAUTHENTICATED（fail loud，不塌成 NOT_FOUND）")
    void buyerScopedInternalCallWithoutBuyerId_failsLoud() {
        GetPaymentResponse response = callGetPaymentUnder(Context.current()
                .withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, "1"));

        assertEquals(GrpcResponseCode.UNAUTHORIZED.code(), response.getCode(),
                "契約違反（呼叫端宣告買家角色卻沒傳播 buyer_id）必須與『查無此交易』區分開，"
                        + "否則接線錯誤永遠只能在 production 被發現");
        verify(paymentService, never()).getPaymentDetail(any());
    }

    @Test
    @DisplayName("getPaymentHistory：內部呼叫傳播了買家 account-type 卻沒帶 buyer_id → UNAUTHENTICATED（fail loud，同一份 guard 邏輯複製在 verifyPaymentOwnershipByOrderId）")
    void buyerScopedInternalCallWithoutBuyerId_getPaymentHistory_failsLoud() {
        GetPaymentHistoryResponse response = callGetPaymentHistoryUnder(Context.current()
                .withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, "1"));

        assertEquals(GrpcResponseCode.UNAUTHORIZED.code(), response.getCode(),
                "verifyPaymentOwnershipByOrderId 是 verifyPaymentOwnership 的複製品，同樣的契約違反"
                        + "（呼叫端宣告買家角色卻沒傳播 buyer_id）必須同樣 fail loud，不能塌成『訂單無付款記錄』");
        verify(paymentService, never()).getPaymentHistory(any());
    }

    @Test
    @DisplayName("SELLER_AND_BUYER（code 3）也走買家歸屬檢查，raw account_id 一樣被拒")
    void sellerAndBuyerCallerWithRawAccountId_isDeniedAsNotFound() {
        when(paymentService.getPaymentDetail(TRADE_NO)).thenReturn(Optional.of(txnOwnedByResolvedBuyer()));

        GetPaymentResponse response = callGetPaymentUnder(Context.current()
                .withValue(GrpcAuthContext.ACCOUNT_ID, RAW_ACCOUNT_ID)
                .withValue(GrpcAuthContext.ACCOUNT_TYPE, "3"));

        assertEquals(GrpcResponseCode.NOT_FOUND.code(), response.getCode(),
                "AccountTypeRules.covers(SELLER_AND_BUYER, BUYER) 為 true，這個角色同樣受買家歸屬約束");
    }
}
