package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.GrantResponseRules;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payuni.http.PayUniApiException;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * §5.6 撥款去重硬化 A（收緊重試）+ B（grant_query 對帳）行為測試（Mockito）。
 *
 * <p>驗證分類 / 重試次數 / 對帳分支的控制流；哨兵值「真的 commit 存活（不被 rollback）」由
 * {@code GrantSentinelSurvivalIntegrationTest}（真實 Spring transaction）另行把關。
 */
@ExtendWith(MockitoExtension.class)
class GrantDedupHardeningTest {

    @Mock private PaymentTransactionRepository transactionRepository;
    @Mock private io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository refundRepository;
    @Mock private PaymentOutboxWriter outboxWriter;
    @Mock private PaymentDomainService domainService;
    @Mock private PaymentClient paymentClient;
    @Mock private PayUniConfig payUniConfig;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private PaymentLogMapper paymentLogMapper;
    @Mock private AtmReactivationService atmReactivationService;
    @Mock private PaymentCloseService paymentCloseService;

    private PaymentApplicationService service;

    @BeforeEach
    void setUp() {
        service = new PaymentApplicationService(
                transactionRepository, refundRepository, outboxWriter, domainService, paymentClient,
                payUniConfig, orderGrpcClient, paymentLogMapper,
                atmReactivationService, paymentCloseService);
        lenient().when(payUniConfig.getAgentId()).thenReturn("HAWK");
        // 預設無已完成退款（§5.6.1 接管主判準）；接管案例各測試自行覆寫
        lenient().when(refundRepository.hasSucceededRefund(anyString())).thenReturn(false);
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
                .updatedAt(DateTimeUtils.nowUtc())
                .version(0);
    }

    private static PayUniApiResponse success() {
        return new PayUniApiResponse("SUCCESS", "撥款成功", Map.of());
    }

    private static PayUniApiResponse status(String code, String msg) {
        return new PayUniApiResponse(code, msg, Map.of());
    }

    private void stubLocked(PaymentTransaction tx) {
        when(transactionRepository.findActiveByOrderIdForUpdate("O123")).thenReturn(Optional.of(tx));
    }

    // ==================== A：收緊撥款重試 ====================

    @Test
    @DisplayName("A1 首撥 timeout（PayUniApiException）→ 不穿透、不重試、標 PENDING_CONFIRM（updateGrantFailure）")
    void firstAttempt_timeoutException_pendingConfirm_noRetry() {
        stubLocked(paidTx().build());   // grantStatus=null → 首撥，不對帳
        when(paymentClient.grant(eq("HAWK"), eq("HAWK9"), eq("UNI_123"), eq(500L)))
                .thenThrow(new PayUniApiException("HTTP_ERROR", "timeout", 504));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());
        verify(paymentClient, times(1)).grant(any(), any(), any(), anyLong());   // 模稜兩可 → 不重試
        verify(transactionRepository).updateGrantFailure(eq(1L), eq(PayUniConstants.PENDING_CONFIRM), any(), any());
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());
        // 延遲查詢：模稜兩可（無可信拒絕碼）不查退款判準
        verify(refundRepository, never()).hasSucceededRefund(anyString());
    }

    @Test
    @DisplayName("A2 首撥 HASH_INVALID（不可信回應）→ 不重試、標 PENDING_CONFIRM")
    void firstAttempt_hashInvalid_pendingConfirm() {
        stubLocked(paidTx().build());
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(status("HASH_INVALID", "簽章驗證失敗"));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());
        verify(paymentClient, times(1)).grant(any(), any(), any(), anyLong());
        verify(transactionRepository).updateGrantFailure(eq(1L), eq(PayUniConstants.PENDING_CONFIRM), any(), any());
        verify(refundRepository, never()).hasSucceededRefund(anyString());   // 延遲查詢：不可信回應不查
    }

    @Test
    @DisplayName("A3 首撥 明確拒絕碼 → retry 一次（共 2 次）→ 仍失敗 → 寫 PayUni 錯誤碼")
    void firstAttempt_rejectCode_retriesOnce_writesErrorCode() {
        stubLocked(paidTx().build());
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(status("GRANT99999", "餘額不足"));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals("GRANT99999", result.grantStatus());
        verify(paymentClient, times(2)).grant(any(), any(), any(), anyLong());   // 明確未發動 → retry 一次
        verify(transactionRepository).updateGrantFailure(eq(1L), eq("GRANT99999"), any(), any());
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());
    }

    @Test
    @DisplayName("A4 首撥 明確拒絕碼 → retry 成功 → 寫 SUCCESS（granted_at）")
    void firstAttempt_rejectThenRetrySuccess_writesSuccess() {
        stubLocked(paidTx().build());
        when(paymentClient.grant(any(), any(), any(), anyLong()))
                .thenReturn(status("GRANT99999", "暫時失敗"))
                .thenReturn(success());

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());
        assertEquals(GrantResponseRules.GRANT_OUTCOME_GRANTED, result.grantOutcome());
        verify(paymentClient, times(2)).grant(any(), any(), any(), anyLong());
        verify(transactionRepository).updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
        verify(transactionRepository, never()).updateGrantFailure(any(), any(), any(), any());
    }

    // ==================== §5.6.1：退款接管（PROVIDER_TOOK_OVER）====================

    @Test
    @DisplayName("A5 拒絕碼 + 存在已完成退款 → 正常回應 grant_outcome=PROVIDER_TOOK_OVER：不重試、grant_status 落 PayUni 原碼")
    void rejectWithCompletedRefund_providerTookOver_noRetry_writesOriginalCode() {
        stubLocked(paidTx().build());
        when(refundRepository.hasSucceededRefund("O123001")).thenReturn(true);
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(status("GRANT30023", "退款交易不受理補發"));

        GrantPaymentResult result = service.grantPayment("O123");

        // 結構化訊號：正常結果（非失敗），adapter 據此回 grant_outcome=PROVIDER_TOOK_OVER 而非 throw
        assertEquals(GrantResponseRules.GRANT_OUTCOME_PROVIDER_TOOK_OVER, result.grantOutcome());
        assertEquals("GRANT30023", result.grantStatus());   // 原碼原樣落地，非 PENDING_CONFIRM 哨兵
        verify(paymentClient, times(1)).grant(any(), any(), any(), anyLong());   // 接管 → 不重試
        verify(refundRepository, times(1)).hasSucceededRefund("O123001");        // 延遲查詢：僅拒絕碼觸發一次
        verify(transactionRepository).updateGrantFailure(eq(1L), eq("GRANT30023"), any(), any());
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());   // granted_at 保持 NULL
    }

    @Test
    @DisplayName("A6 GRANT30023 但無已完成退款（請款批次未結時間性場景）→ 照既有明確未發動路徑（retry 一次、grantOutcome=null）")
    void grant30023WithoutRefund_definitelyNotSent_retriesOnce() {
        stubLocked(paidTx().build());
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(status("GRANT30023", "請款批次未結"));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals("GRANT30023", result.grantStatus());
        assertNull(result.grantOutcome());   // 失敗結果 → adapter throw gRPC error（既有路徑）
        verify(paymentClient, times(2)).grant(any(), any(), any(), anyLong());   // 不收斂 → 照 retry 一次
        verify(transactionRepository).updateGrantFailure(eq(1L), eq("GRANT30023"), any(), any());
    }

    // ==================== B：grant_query 對帳去重 ====================

    private static PayUniApiResponse gqHit(String tradeNo) {
        return new PayUniApiResponse("SUCCESS", "ok", Map.of(
                "Result[0][TradeNo]", tradeNo,
                "Result[0][Type]", "1",
                "Result[0][FundsAmt]", "495",
                "Result[0][ExecuteAmt]", "500",
                "PageInfo[Total]", "1"));
    }

    private static PayUniApiResponse gqNoMatchTwoPages() {
        return new PayUniApiResponse("SUCCESS", "ok", Map.of(
                "Result[0][TradeNo]", "SOMEONE_ELSE",
                "Result[0][Type]", "1",
                "PageInfo[Total]", "2"));
    }

    /** 真實 sandbox：該查詢日查無符合撥款資料 → 外層 Status=QUERY03002（有效空頁，非失敗）。 */
    private static PayUniApiResponse gqNoData() {
        return status(GrantResponseRules.GRANT_QUERY_NO_DATA, "查無符合撥款資料");
    }

    /** grant_query 查詢窗超限 → 外層 Status=QUERY02004（未查完，非有效空結果）。 */
    private static PayUniApiResponse gqOutOfWindow() {
        return status(GrantResponseRules.GRANT_QUERY_OUT_OF_WINDOW, "查詢日期須為30天內");
    }

    @Test
    @DisplayName("B1 非首撥 + grant_query 命中已入帳 → 標 SUCCESS、絕不重打 Grant")
    void retry_reconcileHit_marksSuccess_noGrant() {
        // grantStatus=PENDING_CONFIRM（非首撥），updated_at=昨日 → today>attemptDay，可對帳
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))
                .build();
        stubLocked(tx);
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt())).thenReturn(gqHit("UNI_123"));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());
        verify(paymentClient, atLeastOnce()).grantQuery(eq("HAWK9"), anyString(), anyInt());
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());   // 命中已入帳 → 不重發
        verify(transactionRepository).updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
    }

    @Test
    @DisplayName("B2 非首撥 + 全區間逐日 QUERY03002（真的從未撥）查完 → 確認未撥 → 安全重發 Grant")
    void retry_allDaysNoData_confirmedNotGranted_reGrants() {
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))   // attemptDay=昨日 → 逐日 [昨日, 今日]=2 天
                .build();
        stubLocked(tx);
        // 真實 sandbox：每日皆 QUERY03002（查無撥款資料，有效空頁）→ 續掃至全區間查完仍查無
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt())).thenReturn(gqNoData());
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(success());

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());
        // QUERY03002 空日不分頁，每日僅 1 次 → 2 天共 2 次 grant_query（舊 code 會在第 1 天誤判 INCONCLUSIVE 提早返回）
        verify(paymentClient, times(2)).grantQuery(eq("HAWK9"), anyString(), anyInt());
        verify(paymentClient, atLeastOnce()).grant(any(), any(), any(), anyLong());
        verify(transactionRepository).updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
    }

    @Test
    @DisplayName("B2b 非首撥 + 某日 SUCCESS 多分頁（他人撥款）無命中 + 其餘日 QUERY03002 → 查完查無 → 安全重發")
    void retry_successMultiPageNoMatch_thenNoData_confirmedNotGranted_reGrants() {
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))
                .build();
        stubLocked(tx);
        // 逐日 [昨日, 今日]，依對帳迭代序（昨日 page1、page2；今日 page1）連續回傳，避開跨午夜日期匹配脆弱性
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt()))
                .thenReturn(gqNoMatchTwoPages())   // 昨日 page1（SUCCESS，他人撥款 SOMEONE_ELSE，PageInfo Total=2）
                .thenReturn(gqNoMatchTwoPages())   // 昨日 page2（遍歷全分頁仍無命中）
                .thenReturn(gqNoData());           // 今日 page1（QUERY03002 有效空日）
        when(paymentClient.grant(any(), any(), any(), anyLong())).thenReturn(success());

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());
        // SUCCESS 日遍歷全分頁(2) + 空日(1) = 3 次；確認未撥 → 安全重發
        verify(paymentClient, times(3)).grantQuery(eq("HAWK9"), anyString(), anyInt());
        verify(paymentClient, atLeastOnce()).grant(any(), any(), any(), anyLong());
        verify(transactionRepository).updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
    }

    @Test
    @DisplayName("B4 非首撥 + 隔日入帳：嘗試日 QUERY03002 空、隔日 SUCCESS 命中該 TradeNo → SUCCESS 短路（不誤判 INCONCLUSIVE）")
    void retry_noDataThenHitNextDay_marksSuccess_noGrant() {
        // 撥款隔日入帳：attemptDay(昨日) 當天對本交易恆空(QUERY03002)，隔日(今日) 才入帳命中。
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))   // 逐日 [昨日, 今日]
                .build();
        stubLocked(tx);
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt()))
                .thenReturn(gqNoData())            // 昨日(attemptDay)：查無撥款資料（有效空日，須續掃）
                .thenReturn(gqHit("UNI_123"));     // 今日：命中已入帳

        GrantPaymentResult result = service.grantPayment("O123");

        // 舊 code：昨日 QUERY03002 被當查詢失敗 → 第 1 天即 INCONCLUSIVE → 永遠到不了入帳日 → PENDING_CONFIRM（此斷言會紅）
        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());
        verify(paymentClient, times(2)).grantQuery(eq("HAWK9"), anyString(), anyInt());   // 空日續掃到入帳日
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());             // 命中已入帳 → 不重發
        verify(transactionRepository).updateGrantResult(eq(1L), any(), eq(PayUniConstants.RESPONSE_SUCCESS), any());
    }

    @Test
    @DisplayName("B5 非首撥 + grant_query 回 QUERY02004（超 30 天窗）→ 未查完 → INCONCLUSIVE：不盲目重發")
    void retry_queryOutOfWindow_inconclusive_noGrant() {
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))
                .build();
        stubLocked(tx);
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt())).thenReturn(gqOutOfWindow());

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());               // 維持，不重發
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());              // 未查完 → 絕不重發
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());
    }

    @Test
    @DisplayName("B3 非首撥 + 落在入帳前（today<=attemptDay）→ INCONCLUSIVE：不查、不重發、維持非成功")
    void retry_beforePosting_inconclusive_noGrant_noQuery() {
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc())   // 今日嘗試 → today==attemptDay → 落在入帳前
                .build();
        stubLocked(tx);

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());   // 維持
        verify(paymentClient, never()).grantQuery(any(), any(), anyInt());
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());
        verify(transactionRepository, never()).updateGrantResult(any(), any(), any(), any());
    }

    @Test
    @DisplayName("B3b 非首撥 + grant_query 查詢失敗（未查完）→ INCONCLUSIVE：不盲目重發")
    void retry_queryFails_inconclusive_noGrant() {
        PaymentTransaction tx = paidTx()
                .grantStatus(PayUniConstants.PENDING_CONFIRM)
                .updatedAt(DateTimeUtils.nowUtc().minusDays(1))
                .build();
        stubLocked(tx);
        when(paymentClient.grantQuery(eq("HAWK9"), anyString(), anyInt()))
                .thenThrow(new PayUniApiException("HTTP_ERROR", "timeout", 504));

        GrantPaymentResult result = service.grantPayment("O123");

        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());
        verify(paymentClient, never()).grant(any(), any(), any(), anyLong());   // 未查完 → 絕不重發
    }
}
