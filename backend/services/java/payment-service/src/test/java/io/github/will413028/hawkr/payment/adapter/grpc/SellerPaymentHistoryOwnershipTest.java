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
import io.github.will413028.hawkr.payment.grpc.GetPaymentRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentResponse;
import io.github.will413028.hawkr.payment.grpc.GetSellerPaymentHistoryRequest;
import io.github.will413028.hawkr.payment.grpc.GetSellerPaymentHistoryResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
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

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #2501 賣家視角付款歷程的 store 歸屬與遮蔽契約。
 *
 * <p>本 bug class 的根因是「賣家分支只做 account-type gate、沒有 store 歸屬比對」。這裡把賣家歸屬
 * 軸釘死：caller store（Gateway 解出、由 request.store_id 承載）必須等於 canonical order 反解的
 * store_id，跨店與查無訂單一律回同形 NOT_FOUND（防枚舉）；賣家視角回應遮蔽買家 PII。
 *
 * <p>fixture 的 caller store 與他店 store 刻意是不同數字（非對稱 ID 原則，見
 * [[identity-resolution-consistency]] Rule 4），讓「有沒有真的比對 store」有鑑別力。
 *
 * <p>單元測試環境無 gRPC Context → {@code isInternalCall()} 為 false，模擬 gateway 帶 JWT 呼叫。
 */
@ExtendWith(MockitoExtension.class)
class SellerPaymentHistoryOwnershipTest {

    private static final long CALLER_STORE_ID = 30003001L;   // Gateway 為賣家解出的現任 store_id
    private static final long OTHER_STORE_ID = 30009999L;    // 他店 store_id（刻意不同數字）
    private static final String ORDER_ID = "O488996314";
    private static final String BUYER_ID = "7489534863566774901";
    private static final String ATM_ACCOUNT = "8220100000447103";
    private static final String PROVIDER_TRADE_NO = "1788600898409376524";

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

    /** 一筆含買家 PII（buyer_id / ATM 虛擬帳號 / provider trade ref）的付款 row。 */
    private static PaymentDetailResponse rowWithBuyerPii() {
        return new PaymentDetailResponse(
                ORDER_ID + "001", ORDER_ID, "HAWK2662192214", BUYER_ID,
                PaymentMethod.ATM, 1055L, "測試商品", PaymentTransactionStatus.PAID,
                PROVIDER_TRADE_NO, "1", "付款成功", "822", ATM_ACCOUNT, null,
                null, null, null, null, null,
                false, null, null, null, null);
    }

    private GetSellerPaymentHistoryResponse callUnder(Context ctx, GetSellerPaymentHistoryRequest request) {
        PaymentGrpcService service = newService();
        @SuppressWarnings("unchecked")
        StreamObserver<GetSellerPaymentHistoryResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetSellerPaymentHistoryResponse> captor =
                ArgumentCaptor.forClass(GetSellerPaymentHistoryResponse.class);
        Context previous = ctx.attach();
        try {
            service.getSellerPaymentHistory(request, observer);
        } finally {
            ctx.detach(previous);
        }
        verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    private static GetSellerPaymentHistoryRequest req(long storeId) {
        return GetSellerPaymentHistoryRequest.newBuilder()
                .setOrderId(ORDER_ID).setStoreId(storeId).build();
    }

    private static Context sellerCtx(String accountTypeCode) {
        return Context.current().withValue(GrpcAuthContext.ACCOUNT_TYPE, accountTypeCode);
    }

    @Test
    @DisplayName("賣家查自己店家的訂單 → 200，且遮蔽 buyerId / atmVirtualAccount / providerTradeNo")
    void sellerSameStore_succeedsAndRedactsBuyerPii() {
        when(paymentService.resolveOrderStoreId(ORDER_ID)).thenReturn(CALLER_STORE_ID);
        when(paymentService.getPaymentHistory(ORDER_ID)).thenReturn(List.of(rowWithBuyerPii()));

        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("2"), req(CALLER_STORE_ID));

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertEquals(1, response.getRecordsCount());
        PaymentDetail row = response.getRecords(0);
        // 遮蔽：買家 PII 一律清空
        assertEquals("", row.getBuyerId(), "賣家視角不得看到買家 buyer_id");
        assertEquals("", row.getAtmVirtualAccount(), "賣家視角不得看到買家 ATM 虛擬帳號");
        assertEquals("", row.getProviderTradeNo(), "賣家視角不得看到 provider trade reference");
        // 賣家對帳需要的欄位保留
        assertEquals("1055", row.getAmount());
        assertEquals(PaymentTransactionStatus.PAID.value(), row.getStatus());
        assertEquals(PaymentMethod.ATM.value(), row.getPaymentType());
    }

    @Test
    @DisplayName("SELLER_AND_BUYER（code 3）賣家視角查自己店家 → 200（covers SELLER）")
    void dualRoleSeller_sameStore_succeeds() {
        when(paymentService.resolveOrderStoreId(ORDER_ID)).thenReturn(CALLER_STORE_ID);
        when(paymentService.getPaymentHistory(ORDER_ID)).thenReturn(List.of(rowWithBuyerPii()));

        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("3"), req(CALLER_STORE_ID));

        assertEquals(GrpcResponseCode.SUCCESS.code(), response.getCode());
        assertEquals(1, response.getRecordsCount());
    }

    @Test
    @DisplayName("賣家查他店訂單 → NOT_FOUND（與訂單不存在無法區分），不回傳任何 row")
    void sellerCrossStore_isDeniedAsNotFound() {
        // 訂單屬他店：canonical order 反解出 OTHER_STORE_ID，caller 是 CALLER_STORE_ID
        when(paymentService.resolveOrderStoreId(ORDER_ID)).thenReturn(OTHER_STORE_ID);

        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("2"), req(CALLER_STORE_ID));

        assertEquals(GrpcResponseCode.NOT_FOUND.code(), response.getCode(),
                "caller store 與 order store 不符必須被拒；回應刻意與『訂單無付款記錄』一致（防枚舉）");
        assertEquals("訂單無付款記錄", response.getMessage());
        // 絕不可讀到他店付款歷程
        verify(paymentService, never()).getPaymentHistory(any());
    }

    @Test
    @DisplayName("賣家查不存在的訂單（resolveOrderStoreId=0）→ NOT_FOUND（與跨店同形）")
    void sellerOrderNotFound_isNotFound() {
        when(paymentService.resolveOrderStoreId(ORDER_ID)).thenReturn(0L);

        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("2"), req(CALLER_STORE_ID));

        assertEquals(GrpcResponseCode.NOT_FOUND.code(), response.getCode());
        assertEquals("訂單無付款記錄", response.getMessage());
        verify(paymentService, never()).getPaymentHistory(any());
    }

    @Test
    @DisplayName("賣家視角呼叫但未帶 store_id（Gateway 契約違反）→ UNAUTHORIZED（fail loud，不塌成 NOT_FOUND）")
    void sellerMissingStoreId_failsLoud() {
        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("2"), req(0L));

        assertEquals(GrpcResponseCode.UNAUTHORIZED.code(), response.getCode(),
                "契約違反（賣家視角卻沒帶 Gateway 解出的 store_id）必須 fail loud，"
                        + "不能與『訂單無付款記錄』無法區分——否則 Gateway 接線錯誤只能在 production 被發現");
        verify(paymentService, never()).resolveOrderStoreId(any());
        verify(paymentService, never()).getPaymentHistory(any());
    }

    @Test
    @DisplayName("純買家（code 1）呼叫賣家視角端點 → PERMISSION_DENIED")
    void pureBuyer_isDenied() {
        GetSellerPaymentHistoryResponse response = callUnder(sellerCtx("1"), req(CALLER_STORE_ID));

        assertEquals(GrpcResponseCode.PERMISSION_DENIED.code(), response.getCode());
        verify(paymentService, never()).resolveOrderStoreId(any());
    }

    @Test
    @DisplayName("#2501 收斂：純賣家（code 2）呼叫 buyer-only 的 getPayment → PERMISSION_DENIED（不再放行）")
    void sellerCannotAccessBuyerOnlyGetPayment() {
        PaymentGrpcService service = newService();
        GetPaymentRequest request = GetPaymentRequest.newBuilder().setTradeNo(ORDER_ID + "001").build();
        @SuppressWarnings("unchecked")
        StreamObserver<GetPaymentResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetPaymentResponse> captor = ArgumentCaptor.forClass(GetPaymentResponse.class);

        Context ctx = sellerCtx("2");
        Context previous = ctx.attach();
        try {
            service.getPayment(request, observer);
        } finally {
            ctx.detach(previous);
        }

        verify(observer).onNext(captor.capture());
        assertEquals(GrpcResponseCode.PERMISSION_DENIED.code(), captor.getValue().getCode(),
                "getPayment 已收斂為 buyer-only；純賣家不得再讀單筆付款（含買家 PII）");
        verify(paymentService, never()).getPaymentDetail(any());
        assertTrue(true);
    }
}
