package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;
import io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.grpc.GetPaymentFactsInternalRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentFactsInternalResponse;
import io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Context;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.LocalDateTime;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code GetPaymentFactsInternal}（58 號 §4.1「05 支付」）的契約測試。
 *
 * <p>守三件事，每一件失效時都<b>沒有任何 runtime 訊號</b>：
 * <ol>
 *   <li>{@code provider_paid_at} 的三態：有值／DB 為 NULL（{@code hasProviderPaidAt()==false}）／查無交易。
 *       NULL 若被寫成空字串，呼叫端無法與「本來就沒有」區分，發票日期會靜默退回我方收單時間。</li>
 *   <li>查無 canonical 交易必須回<b>非 0 業務碼且不帶 data</b>，不得回 code=0 ＋ 空 data。</li>
 *   <li>{@code pay_time} 與 {@code provider_paid_at} 是兩個語意完全不同的時間，接錯不會報錯
 *       ⇒ 兩者以不同值斷言，防止欄位對調。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
class PaymentFactsInternalTest {

    private static final String ORDER_NO = "O382716459";
    private static final String TRADE_NO = "O382716459001";

    /** PayUni 回報的真實付款時間（卡片 AuthDay+AuthTime） */
    private static final LocalDateTime PROVIDER_PAID_AT = LocalDateTime.of(2026, 8, 20, 3, 4, 5);
    /** 我方 webhook 寫入當下的 nowUtc — 刻意與上者不同，欄位對調時測試才會轉紅 */
    private static final LocalDateTime PAY_TIME = LocalDateTime.of(2026, 8, 20, 6, 7, 8);

    private static final String PROVIDER_PAID_AT_ISO = "2026-08-20T03:04:05Z";
    private static final String PAY_TIME_ISO = "2026-08-20T06:07:08Z";

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

    private PaymentTransaction card(LocalDateTime providerPaidAt) {
        return PaymentTransaction.builder()
                .tradeNo(TRADE_NO)
                .orderId(ORDER_NO)
                .paymentType(PaymentMethod.CREDIT_CARD)
                .amount(1280L)
                .status(PaymentTransactionStatus.PAID)
                .payTime(PAY_TIME)
                .providerPaidAt(providerPaidAt)
                .isDuplicate(false)
                .build();
    }

    private PaymentTransaction atmWithoutProviderPaidAt() {
        return PaymentTransaction.builder()
                .tradeNo(TRADE_NO)
                .orderId(ORDER_NO)
                .paymentType(PaymentMethod.ATM)
                .amount(990L)
                .status(PaymentTransactionStatus.PAID)
                .payTime(PAY_TIME)
                .providerPaidAt(null)   // ATM webhook 常無 PaymentDay — 設計上的合法值
                .isDuplicate(false)
                .build();
    }

    /** 以「內部呼叫」語境執行 RPC 並取回 onNext 的回應。 */
    private GetPaymentFactsInternalResponse callAsInternal() {
        PaymentGrpcService service = newService();
        @SuppressWarnings("unchecked")
        StreamObserver<GetPaymentFactsInternalResponse> observer = mock(StreamObserver.class);
        Context ctx = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
        Context previous = ctx.attach();
        try {
            service.getPaymentFactsInternal(
                    GetPaymentFactsInternalRequest.newBuilder().setOrderNo(ORDER_NO).build(),
                    observer);
        } finally {
            ctx.detach(previous);
        }
        ArgumentCaptor<GetPaymentFactsInternalResponse> captor =
                ArgumentCaptor.forClass(GetPaymentFactsInternalResponse.class);
        verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("① 卡片交易有 provider_paid_at → hasProviderPaidAt()==true 且值正確")
    void cardWithProviderPaidAt_isSet() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.of(card(PROVIDER_PAID_AT)));

        GetPaymentFactsInternalResponse response = callAsInternal();

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertTrue(response.hasData(), "成功回應必須帶 data");
        assertTrue(response.getData().hasProviderPaidAt(),
                "DB 有值時必須 set，否則呼叫端會誤判成『payment 端本來就沒有』並改用我方收單時間");
        assertEquals(PROVIDER_PAID_AT_ISO, response.getData().getProviderPaidAt());
    }

    @Test
    @DisplayName("② ATM 交易 provider_paid_at 為 NULL → hasProviderPaidAt()==false（不是空字串）")
    void atmWithNullProviderPaidAt_isUnset() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.of(atmWithoutProviderPaidAt()));

        GetPaymentFactsInternalResponse response = callAsInternal();

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertTrue(response.hasData());
        assertFalse(response.getData().hasProviderPaidAt(),
                "DB 為 NULL 時必須完全不 set。set 成空字串會讓 optional 的三態塌成兩態，"
                        + "『拿不到』與『本來就沒有』在 wire 上分不開，invoice_date 靜默取錯且無訊號");
        // 但 pay_time 仍必須有值（PAID 交易的契約）
        assertEquals(PAY_TIME_ISO, response.getData().getPayTime());
    }

    @Test
    @DisplayName("③ 查不到訂單的 canonical 交易 → 非 0 業務碼且不帶 data（不得回 code=0 ＋ 空 data）")
    void noCanonicalTransaction_returnsBusinessErrorWithoutData() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.empty());

        GetPaymentFactsInternalResponse response = callAsInternal();

        assertEquals(GrpcResponseCode.TRANSACTION_NOT_FOUND_FOR_ORDER.code(), response.getCode(),
                "呼叫端唯一安全的行為是 fail-loud（order.paid 重試三次進 DLQ）");
        assertFalse(response.hasData(),
                "空 data ＋ code=0 會讓發票日期退回本地時鐘、金額對帳失去基準");
    }

    @Test
    @DisplayName("④ pay_time 與 provider_paid_at 分屬不同欄位（防對調：兩者語意不同，接錯無任何訊號）")
    void payTimeAndProviderPaidAt_areNotSwapped() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.of(card(PROVIDER_PAID_AT)));

        GetPaymentFactsInternalResponse response = callAsInternal();

        assertEquals(PROVIDER_PAID_AT_ISO, response.getData().getProviderPaidAt(),
                "provider_paid_at ＝ PayUni 回報的真實付款時間");
        assertEquals(PAY_TIME_ISO, response.getData().getPayTime(),
                "pay_time ＝ 我方 webhook 寫入當下的 nowUtc");
    }

    @Test
    @DisplayName("⑤ 其餘欄位對帳：order_no / trade_no / trade_amt / payment_type / is_duplicate")
    void remainingFieldsMapCorrectly() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.of(card(PROVIDER_PAID_AT)));

        GetPaymentFactsInternalResponse response = callAsInternal();

        assertEquals(ORDER_NO, response.getData().getOrderNo());
        assertEquals(TRADE_NO, response.getData().getTradeNo());
        assertEquals("1280", response.getData().getTradeAmt(), "金額為元整數的數字字串");
        assertEquals(PaymentMethod.CREDIT_CARD.value(), response.getData().getPaymentType());
        assertFalse(response.getData().getIsDuplicate(),
                "本 RPC 只回 is_duplicate=FALSE 的 canonical 交易；回 true 代表選錯 row");
    }

    @Test
    @DisplayName("⑥ 非內部呼叫 → PERMISSION_DENIED，且不查任何資料")
    void nonInternalCall_isDenied() {
        PaymentGrpcService service = newService();
        @SuppressWarnings("unchecked")
        StreamObserver<GetPaymentFactsInternalResponse> observer = mock(StreamObserver.class);

        // 單元測試無 gRPC Context → INTERNAL_CALL 為 null → isInternalCall()=false
        service.getPaymentFactsInternal(
                GetPaymentFactsInternalRequest.newBuilder().setOrderNo(ORDER_NO).build(),
                observer);

        ArgumentCaptor<Throwable> captor = ArgumentCaptor.forClass(Throwable.class);
        verify(observer).onError(captor.capture());
        assertTrue(captor.getValue() instanceof StatusRuntimeException);
        assertEquals(io.grpc.Status.Code.PERMISSION_DENIED,
                ((StatusRuntimeException) captor.getValue()).getStatus().getCode());
        verify(observer, never()).onNext(any());
        verify(transactionRepository, never()).findCanonicalPaidByOrderId(any());
    }

    @Test
    @DisplayName("⑦ 只讀不寫：本 RPC 不得觸發任何寫入路徑")
    void readOnly_neverWrites() {
        when(transactionRepository.findCanonicalPaidByOrderId(ORDER_NO))
                .thenReturn(Optional.of(card(PROVIDER_PAID_AT)));

        callAsInternal();

        verify(transactionRepository, never()).save(any());
        verify(transactionRepository, never()).updateProviderTimes(any(), any(), any());
        verify(transactionTemplate, never()).execute(any());
    }
}
