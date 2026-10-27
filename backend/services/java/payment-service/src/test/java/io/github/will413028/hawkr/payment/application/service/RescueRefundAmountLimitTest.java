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
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Rescue 退款金額上限檢查測試（issue #1329 項3：應用層縱深防禦，與 createRefund L2 對齊）。
 *
 * <p>目的：驗證 {@code createRescueRefund} 在建立退款 row <b>之前</b> 就以應用層金額上限檢查拒絕超額，
 * 不依賴 #1328 的 L3 DB trigger 兜底。刻意使用<b>真實</b> {@link RefundDomainService}（非 mock），
 * 這樣「超額被拒 / 正常放行」是真的走 {@code validateRefundAmount} 的判定，而非測試自己 stub 的假結果。</p>
 *
 * <p>累計語意與 createRefund L2 一致：用 {@code sumRefundedAmountByTradeNo}（含在途 '0','1','2'、不排除本筆），
 * 因 rescue 本筆此時尚未 save。</p>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class RescueRefundAmountLimitTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private RefundTransactionRepository refundRepository;
    @Mock private PaymentEventPublisher eventPublisher;
    @Mock private PaymentClient paymentClient;

    // 刻意用真實 domain service：金額檢查要走真正的業務邏輯，不放水成 mock stub
    private final RefundDomainService domainService = new RefundDomainService();

    private RefundApplicationService service;
    private PaymentTransaction creditCardExpired;

    private static final String TRADE_NO = "RESCUE_TRD_AMT";
    private static final String IDEM_KEY = "rescue:RESCUE_TRD_AMT";
    private static final long TRANSACTION_AMOUNT = 500L;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(902, 0);
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

        creditCardExpired = PaymentTransaction.builder()
                .id(1L)
                .tradeNo(TRADE_NO)
                .orderId("RESCUE_ORD_AMT")
                .merId("HAWK9")
                .paymentType(PaymentMethod.CREDIT_CARD)
                .amount(TRANSACTION_AMOUNT)
                .status(PaymentTransactionStatus.EXPIRED)
                .providerTradeNo("UNI_AMT")
                .version(0)
                .build();

        // 共通：idempotency 不命中、找得到母交易、無進行中退款
        when(refundRepository.findByIdempotencyKey(eq(IDEM_KEY))).thenReturn(Optional.empty());
        when(transactionRepository.findCanonicalByTradeNo(eq(TRADE_NO)))
                .thenReturn(Optional.of(creditCardExpired));
        when(refundRepository.hasProcessingRefund(eq(TRADE_NO))).thenReturn(false);
    }

    @Test
    @DisplayName("超額（已退 400 + 本次 200 > 交易 500）→ 應用層拒絕，且退款 row 從未建立（不靠 L3）")
    void rescue_overLimit_rejectedAtApplicationLayerBeforeSave() {
        // 已退 400（在途+成功累計），交易額 500 → 可退餘額 100；本次要退 200 → 超額
        when(refundRepository.sumRefundedAmountByTradeNo(eq(TRADE_NO))).thenReturn(400L);

        IllegalArgumentException ex = assertThrows(
                IllegalArgumentException.class,
                () -> service.createRescueRefund(TRADE_NO, 200L, "reason", IDEM_KEY),
                "超額 rescue 退款必須在應用層被拒");

        assertTrue(ex.getMessage().contains("退款金額超過可退款餘額"),
                "應為既有金額超額訊息，實際=" + ex.getMessage());

        // 核心：被拒發生在建立 row 之前 —— 完全不依賴 L3 DB trigger
        verify(refundRepository, never()).save(any(RefundTransaction.class));
        // 也不應觸發任何 PayUni 呼叫
        verifyNoInteractions(paymentClient);
    }

    @Test
    @DisplayName("剛好等於餘額（已退 400 + 本次 100 = 交易 500）→ 放行，正常建立並完成退款")
    void rescue_exactlyAtRemainingBalance_allowed() {
        when(refundRepository.sumRefundedAmountByTradeNo(eq(TRADE_NO))).thenReturn(400L);

        // 放行後走信用卡同步退款：查詢請款狀態成功且已請款成功 → refund API 成功
        when(paymentClient.queryTransactionByProviderTradeNo(eq("HAWK9"), eq("UNI_AMT")))
                .thenReturn(new PayUniApiResponse(
                        "SUCCESS", "查詢成功",
                        Map.of("Result[0][" + PayUniConstants.FIELD_CLOSE_STATUS + "]",
                               PayUniConstants.CLOSE_STATUS_SUCCESS)));
        when(paymentClient.refund(eq("HAWK9"), eq("UNI_AMT"), anyString(), eq(100L)))
                .thenReturn(new PayUniApiResponse("SUCCESS", "退款成功", Map.of()));
        // updateStatusToSucceeded 回 1 → 進 success 分支
        when(refundRepository.updateStatusToSucceeded(
                anyString(), any(), any(), any(), any(), any(), anyInt())).thenReturn(1);

        var result = service.createRescueRefund(TRADE_NO, 100L, "reason", IDEM_KEY);

        assertEquals(RefundStatus.SUCCEEDED.value(), result.finalStatus());
        assertTrue(result.succeeded());
        assertFalse(result.requiresManual());

        // 放行必然有建立退款 row
        verify(refundRepository, times(1)).save(any(RefundTransaction.class));
    }

    @Test
    @DisplayName("尚無任何退款（已退 0 + 本次 500 = 交易 500）→ 放行")
    void rescue_firstFullRefund_allowed() {
        when(refundRepository.sumRefundedAmountByTradeNo(eq(TRADE_NO))).thenReturn(0L);

        when(paymentClient.queryTransactionByProviderTradeNo(eq("HAWK9"), eq("UNI_AMT")))
                .thenReturn(new PayUniApiResponse(
                        "SUCCESS", "查詢成功",
                        Map.of("Result[0][" + PayUniConstants.FIELD_CLOSE_STATUS + "]",
                               PayUniConstants.CLOSE_STATUS_SUCCESS)));
        when(paymentClient.refund(eq("HAWK9"), eq("UNI_AMT"), anyString(), eq(500L)))
                .thenReturn(new PayUniApiResponse("SUCCESS", "退款成功", Map.of()));
        when(refundRepository.updateStatusToSucceeded(
                anyString(), any(), any(), any(), any(), any(), anyInt())).thenReturn(1);

        var result = service.createRescueRefund(TRADE_NO, 500L, "reason", IDEM_KEY);

        assertEquals(RefundStatus.SUCCEEDED.value(), result.finalStatus());
        verify(refundRepository, times(1)).save(any(RefundTransaction.class));
    }
}
