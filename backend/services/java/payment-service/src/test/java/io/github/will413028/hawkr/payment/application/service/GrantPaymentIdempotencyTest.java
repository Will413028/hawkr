package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * P0 撥款重撥修復（05 §5.6）：grantPayment 撥款前冪等短路。
 *
 * 驗證鎖定交易後若 grant_status 已為 SUCCESS，直接回傳既有結果、不再呼叫 PayUni Grant；
 * 尚未撥款（grant_status=null）時才實際打 PayUni。防時序型雙重撥款。
 */
@ExtendWith(MockitoExtension.class)
class GrantPaymentIdempotencyTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private PaymentOutboxWriter outboxWriter;
    @Mock private PaymentDomainService domainService;
    @Mock private PaymentClient paymentClient;
    @Mock private PayUniConfig payUniConfig;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private PaymentLogMapper paymentLogMapper;
    @Mock private AtmReactivationService atmReactivationService;
    @Mock private PaymentCloseService paymentCloseService;
    @Mock private io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository refundRepository;

    private PaymentApplicationService service;

    @BeforeEach
    void setUp() {
        service = new PaymentApplicationService(
                transactionRepository, refundRepository, outboxWriter, domainService, paymentClient,
                payUniConfig, orderGrpcClient, paymentLogMapper,
                atmReactivationService, paymentCloseService);
    }

    private PaymentTransaction.PaymentTransactionBuilder paidTx() {
        return PaymentTransaction.builder()
                .id(1L)
                .tradeNo("O123001")
                .orderId("O123")
                .merId("HAWK9")
                .paymentType(PaymentMethod.CREDIT_CARD)
                .amount(500L)
                .status(PaymentTransactionStatus.PAID)
                .providerTradeNo("UNI_123")
                .version(0);
    }

    @Test
    @DisplayName("grant_status==SUCCESS → 冪等短路，回既有結果、不打 PayUni")
    void alreadyGranted_shortCircuits_noPayUniCall() {
        LocalDateTime grantedAt = DateTimeUtils.nowUtc();
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.RESPONSE_SUCCESS)
                .grantedAt(grantedAt)
                .build();
        when(transactionRepository.findActiveByOrderIdForUpdate("O123"))
                .thenReturn(Optional.of(tx));

        GrantPaymentResult result = service.grantPayment("O123");

        // 回傳既有撥款結果
        assertEquals("O123001", result.tradeNo());
        assertEquals(grantedAt, result.grantedAt());

        // 不再打 PayUni，也不重寫 grant 結果 / 不寫 log
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());
        verify(paymentLogMapper, never()).insert(any());
    }

    @Test
    @DisplayName("grant_status==null（未撥款）→ 實際呼叫 PayUni Grant 並寫入 SUCCESS")
    void notYetGranted_callsPayUni() {
        PaymentTransaction tx = paidTx().build();   // grantStatus 為 null
        when(transactionRepository.findActiveByOrderIdForUpdate("O123"))
                .thenReturn(Optional.of(tx));
        lenient().when(payUniConfig.getAgentId()).thenReturn("HAWK");
        when(paymentClient.grant(eq("HAWK"), eq("HAWK9"), eq("UNI_123"), eq(500L)))
                .thenReturn(new PayUniApiResponse("SUCCESS", "撥款成功", Map.of()));

        service.grantPayment("O123");

        // 未撥款 → 短路不成立 → 實際打 PayUni 並寫入成功結果
        verify(paymentClient, atLeastOnce()).grant(eq("HAWK"), eq("HAWK9"), eq("UNI_123"), eq(500L));
        verify(transactionRepository)
                .updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
    }
}
