package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.MybatisConfiguration;
import com.baomidou.mybatisplus.core.metadata.TableInfoHelper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.payment.grpc.GetRefundResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
import org.apache.ibatis.builder.MapperBuilderAssistant;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #1288 — AtmRefundReconciler（09 §5.4.3.1）：REF02037 對帳 + revert 前 payment 終態守衛。
 */
class AtmRefundReconcilerTest {

    private final PaymentGrpcClient paymentGrpcClient = mock(PaymentGrpcClient.class);
    private final RefundTaskMapper refundTaskMapper = mock(RefundTaskMapper.class);
    private final AfterSaleCaseMapper caseMapper = mock(AfterSaleCaseMapper.class);
    private final SellerCancelRequestMapper cancelMapper = mock(SellerCancelRequestMapper.class);
    private final io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService auditService =
            mock(io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService.class);
    private final AfterSalesEventPublisherImpl eventPublisher = mock(AfterSalesEventPublisherImpl.class);

    private AtmRefundReconciler reconciler;

    /**
     * issue #1626：{@code publishRefundCompleted(caseNo, orderNo, storeId, buyerId, ...)} 的
     * storeId / buyerId 是相鄰的兩個 {@code Long} 參數，寫反了照樣編譯過。這兩個常數刻意取
     * 不同值（且不等於 refundAmount 680 / taskId 9001），下面 verify 用 eq() 綁死位置，
     * 讓任何對調都會 fail 而非默默通過。
     */
    private static final long CASE_STORE_ID = 7001L;
    private static final long CASE_BUYER_ID = 4201L;

    @BeforeAll
    static void initLambdaCache() {
        MybatisConfiguration config = new MybatisConfiguration();
        MapperBuilderAssistant assistant = new MapperBuilderAssistant(config, "");
        TableInfoHelper.initTableInfo(assistant, RefundTaskEntity.class);
    }

    @BeforeEach
    void setUp() {
        reconciler = new AtmRefundReconciler(paymentGrpcClient, refundTaskMapper, caseMapper,
                cancelMapper, auditService, eventPublisher);
    }

    private RefundTaskEntity task(String status) {
        RefundTaskEntity t = new RefundTaskEntity();
        t.setId(9001L);
        t.setCaseNo("C0001");
        t.setOrderNo("O0001");
        t.setExternalRefId("RFD_1");
        t.setRefundOutcome("FULL_REFUND");
        t.setRefundAmount(680L);
        t.setPaymentMethod(PaymentMethod.ATM.value());
        t.setStatus(status);
        return t;
    }

    private GetRefundResponse refundResp(String refundStatusValue) {
        return GetRefundResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setData(RefundDetail.newBuilder().setStatus(refundStatusValue).build())
                .build();
    }

    // ---------- 缺陷 C：REF02037 對帳 ----------

    @Test
    @DisplayName("REF02037 + payment SUCCEEDED → 推進 DONE，不 revert")
    void noBalance_paymentSucceeded_advancesToDone() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenReturn(refundResp(RefundStatus.SUCCEEDED.value()));
        when(refundTaskMapper.advanceAnyAwaitingToDone(eq(9001L), any(), any())).thenReturn(1);
        when(caseMapper.closeIfNotClosed(eq("C0001"), any())).thenReturn(1);
        when(caseMapper.findStoreIdByCaseNo("C0001")).thenReturn(CASE_STORE_ID);
        when(caseMapper.findBuyerIdByCaseNo("C0001")).thenReturn(CASE_BUYER_ID);

        // P1-2：狀態查詢（gRPC）先於 tx 外執行，再把結果帶入寫入方法
        var queried = reconciler.queryPaymentRefundState(t);
        var state = reconciler.reconcileNoBalance(queried, t, "REF02037", "無足夠金額可退款");

        assertThat(state).isEqualTo(AtmRefundReconciler.PaymentRefundState.SUCCEEDED);
        verify(refundTaskMapper).advanceAnyAwaitingToDone(eq(9001L), any(), any());
        // issue #1626：storeId / buyerId 位置綁死（相鄰同型參數，對調不會編譯失敗）
        verify(eventPublisher).publishRefundCompleted(
                eq("C0001"), eq("O0001"), eq(CASE_STORE_ID), eq(CASE_BUYER_ID),
                eq(AfterSalePlatformDecisionOutcome.FULL_REFUND), eq(680L),
                eq(PaymentMethod.ATM.value()), eq("RFD_1"));
        verify(refundTaskMapper, never()).revertFromAnyAwaitingToAwaitingPage(any(), any(), any());
        verify(refundTaskMapper, never()).markManualIntervention(any(), any(), any());
    }

    @Test
    @DisplayName("REF02037 + payment PROCESSING → 推進 AWAITING_WEBHOOK，不 MANUAL")
    void noBalance_paymentProcessing_advancesToAwaitingWebhook() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenReturn(refundResp(RefundStatus.PROCESSING.value()));

        var queried = reconciler.queryPaymentRefundState(t);
        var state = reconciler.reconcileNoBalance(queried, t, "REF02037", "無足夠金額可退款");

        assertThat(state).isEqualTo(AtmRefundReconciler.PaymentRefundState.PROCESSING);
        verify(refundTaskMapper).advanceAwaitingPageToAwaitingWebhook(eq(9001L), any(), any());
        verify(refundTaskMapper, never()).markManualIntervention(any(), any(), any());
        verify(refundTaskMapper, never()).advanceAnyAwaitingToDone(any(), any(), any());
    }

    @Test
    @DisplayName("REF02037 + 無法判定（payment REQUESTED）→ MANUAL_INTERVENTION，不 revert")
    void noBalance_undetermined_marksManualIntervention() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenReturn(refundResp(RefundStatus.REQUESTED.value()));
        when(refundTaskMapper.markManualIntervention(eq(9001L), any(), any())).thenReturn(1);

        var queried = reconciler.queryPaymentRefundState(t);
        var state = reconciler.reconcileNoBalance(queried, t, "REF02037", "無足夠金額可退款");

        assertThat(state).isEqualTo(AtmRefundReconciler.PaymentRefundState.UNDETERMINED);
        verify(refundTaskMapper).markManualIntervention(eq(9001L), any(), any());
        verify(refundTaskMapper, never()).revertFromAnyAwaitingToAwaitingPage(any(), any(), any());
        verify(refundTaskMapper, never()).advanceAnyAwaitingToDone(any(), any(), any());
    }

    @Test
    @DisplayName("REF02037 + payment 查詢拋例外 → UNDETERMINED（保守，不臆造已退）")
    void noBalance_grpcThrows_undetermined() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenThrow(new RuntimeException("grpc down"));
        when(refundTaskMapper.markManualIntervention(eq(9001L), any(), any())).thenReturn(1);

        var queried = reconciler.queryPaymentRefundState(t);
        var state = reconciler.reconcileNoBalance(queried, t, "REF02037", "無足夠金額可退款");

        assertThat(state).isEqualTo(AtmRefundReconciler.PaymentRefundState.UNDETERMINED);
        verify(refundTaskMapper).markManualIntervention(eq(9001L), any(), any());
    }

    // ---------- 缺陷 B：revert 前 payment 終態守衛 ----------

    @Test
    @DisplayName("一般失敗 + payment SUCCEEDED → 守衛命中，推進完成回傳 true（caller 不 revert）")
    void guard_paymentSucceeded_returnsTrueAndAdvances() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenReturn(refundResp(RefundStatus.SUCCEEDED.value()));
        when(refundTaskMapper.advanceAnyAwaitingToDone(eq(9001L), any(), any())).thenReturn(1);
        when(caseMapper.closeIfNotClosed(eq("C0001"), any())).thenReturn(1);
        when(caseMapper.findStoreIdByCaseNo("C0001")).thenReturn(CASE_STORE_ID);
        when(caseMapper.findBuyerIdByCaseNo("C0001")).thenReturn(CASE_BUYER_ID);

        var queried = reconciler.queryPaymentRefundState(t);
        boolean advanced = reconciler.advanceIfPaymentSucceeded(queried, t, "BANK_REJECT", "退件");

        assertThat(advanced).isTrue();
        verify(refundTaskMapper).advanceAnyAwaitingToDone(eq(9001L), any(), any());
        // issue #1626：storeId / buyerId 位置綁死（相鄰同型參數，對調不會編譯失敗）
        verify(eventPublisher).publishRefundCompleted(
                eq("C0001"), eq("O0001"), eq(CASE_STORE_ID), eq(CASE_BUYER_ID),
                eq(AfterSalePlatformDecisionOutcome.FULL_REFUND), eq(680L),
                eq(PaymentMethod.ATM.value()), eq("RFD_1"));
    }

    @Test
    @DisplayName("一般失敗 + payment FAILED → 守衛未命中回傳 false（caller 照常 revert）")
    void guard_paymentFailed_returnsFalse() {
        RefundTaskEntity t = task(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        when(paymentGrpcClient.getRefund("RFD_1")).thenReturn(refundResp(RefundStatus.FAILED.value()));

        var queried = reconciler.queryPaymentRefundState(t);
        boolean advanced = reconciler.advanceIfPaymentSucceeded(queried, t, "BANK_REJECT", "退件");

        assertThat(advanced).isFalse();
        verify(refundTaskMapper, never()).advanceAnyAwaitingToDone(any(), any(), any());
        // never() 這裡刻意維持 any()：要斷言「完全沒發過這個事件」，any() 比 eq(具體值) 嚴格
        // （eq 只會證明「沒發過剛好長這樣的那一次」，換個參數值發出去就抓不到）
        verify(eventPublisher, never()).publishRefundCompleted(any(), any(), any(), any(), any(), any(), any(), any());
    }
}
