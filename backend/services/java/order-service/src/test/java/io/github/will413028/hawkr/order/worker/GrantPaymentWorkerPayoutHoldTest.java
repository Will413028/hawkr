package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHold;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHoldReason;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.contains;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * backend#3262 撥款資格閘門在 {@link GrantPaymentWorker} 的行為。
 *
 * <p>對應 issue 的 Regression Case：
 * <ul>
 *   <li><b>Case C</b>（確認收貨 race）：hold 成立時<b>一次 payment 都不能打</b>
 *       ——撥款是不可逆的外部副作用，「先撥了再說」沒有回頭路</li>
 *   <li><b>Case E</b>：hold 到期後正常撥款，不得永久 block</li>
 *   <li><b>Case F</b>（duplicate / retry）：租約沒搶到不查也不撥；推遲 CAS 回 0 時冪等 skip</li>
 *   <li><b>Case G</b>（crash / restart）：閘門在 claim 之後、payment 之前，
 *       任何重跑都必須重新過閘（本檔以「每次 execute 都呼叫 getPayoutHold」斷言）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class GrantPaymentWorkerPayoutHoldTest {

    private static final String ORDER_NO = "O3262";
    private static final long TASK_ID = 42L;

    @Mock private GrantPaymentTaskRepository grantPaymentTaskRepository;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private OrderRepository orderRepository;
    @Mock private OrderEventPublisher orderEventPublisher;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private AfterSalesGrpcClient afterSalesGrpcClient;

    private OrderConfig orderConfig;
    private MeterRegistry meterRegistry;
    private GrantPaymentWorker worker;

    @BeforeEach
    void setUp() {
        orderConfig = new OrderConfig();
        meterRegistry = new SimpleMeterRegistry();
        worker = new GrantPaymentWorker(grantPaymentTaskRepository, paymentGrpcClient,
                orderRepository, orderEventPublisher, sellerGrpcClient,
                afterSalesGrpcClient, orderConfig, meterRegistry);
    }

    private GrantPaymentTaskEntity task() {
        GrantPaymentTaskEntity t = new GrantPaymentTaskEntity();
        t.setTaskId(TASK_ID);
        t.setOrderNo(ORDER_NO);
        t.setRetryCount(0);
        t.setMaxRetryCount(10);
        return t;
    }

    private void stubClaimed() {
        when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(grantPaymentTaskRepository.claimTask(eq(TASK_ID), any())).thenReturn(1);
    }

    private double heldCount(String reason) {
        var counter = meterRegistry.find("hawkr_grant_payout_held").tag("reason", reason).counter();
        return counter == null ? 0d : counter.count();
    }

    // ==================== 租約下界不變式（§29 §三 第 5 步）====================

    @Test
    @DisplayName("租約必須涵蓋『兩個』外部呼叫的 deadline 總和——閘門加進租約窗口後這條才是完整的")
    void leaseCoversBothExternalCallDeadlines() {
        long worstCase = AfterSalesGrpcClient.PAYOUT_HOLD_DEADLINE_SEC
                + PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC;
        assertTrue(GrantPaymentWorker.CLAIM_LEASE_SEC > worstCase,
                "CLAIM_LEASE_SEC(" + GrantPaymentWorker.CLAIM_LEASE_SEC + ") 必須 > "
                        + "PAYOUT_HOLD_DEADLINE_SEC + PAYUNI_BACKED_DEADLINE_SEC(" + worstCase + ")。"
                        + "租約短於單筆最壞耗時 ⇒ 持有者還在打第三方時租約就被別的 pod 合法搶走 ⇒ 重複撥款。");
    }

    @Test
    @DisplayName("撥款資格反查必須帶 deadline：沒有 deadline 就沒有上界，租約下界的推導直接失效")
    void payoutHoldLookupHasBoundedDeadline() {
        assertTrue(AfterSalesGrpcClient.PAYOUT_HOLD_DEADLINE_SEC > 0,
                "PAYOUT_HOLD_DEADLINE_SEC 必須為正數（0 或負數 = 無界）");
        assertTrue(AfterSalesGrpcClient.PAYOUT_HOLD_DEADLINE_SEC
                        < PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC,
                "反查只做一次索引查詢 ＋ 記憶體分類，deadline 不該與同步打第三方的撥款同量級；"
                        + "調大它之前先重算租約下界");
    }

    // ==================== Case C：hold ⇒ 一次 payment 都不打 ====================

    @Test
    @DisplayName("Case C：申訴窗未關 ⇒ 不打 payment，推遲到 hold_until，且不動 retry_count")
    void appealWindowOpen_deferredWithoutGranting() {
        stubClaimed();
        LocalDateTime holdUntil = DateTimeUtils.nowUtc().plusHours(71);
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(true, holdUntil, PayoutHoldReason.APPEAL_WINDOW_OPEN, "AS1"));
        when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(1);

        worker.execute();

        verifyNoInteractions(paymentGrpcClient);
        ArgumentCaptor<LocalDateTime> at = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<String> reason = ArgumentCaptor.forClass(String.class);
        verify(grantPaymentTaskRepository).deferForPayoutHold(eq(TASK_ID), at.capture(), reason.capture());
        assertEquals(holdUntil, at.getValue(), "有時間界的 hold 必須推到 hold_until，不是推 recheck 間隔");
        assertTrue(reason.getValue().contains("APPEAL_WINDOW_OPEN"));
        assertTrue(reason.getValue().contains("AS1"));

        // 不是失敗 ⇒ 不得走任何 retry / 人工介入路徑
        verify(grantPaymentTaskRepository, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskManualIntervention(anyLong(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskMaxRetryReached(anyLong(), anyInt(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskDone(anyLong());
        assertEquals(1d, heldCount("APPEAL_WINDOW_OPEN"));
    }

    @Test
    @DisplayName("無時間界 hold（申訴審理中）⇒ 推 payout-hold-recheck-seconds，不打 payment")
    void activeCase_deferredByRecheckInterval() {
        stubClaimed();
        orderConfig.setPayoutHoldRecheckSeconds(600);
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(true, null, PayoutHoldReason.ACTIVE_CASE, "AS2"));
        when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(1);

        LocalDateTime before = DateTimeUtils.nowUtc();
        worker.execute();

        verifyNoInteractions(paymentGrpcClient);
        ArgumentCaptor<LocalDateTime> at = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(grantPaymentTaskRepository).deferForPayoutHold(eq(TASK_ID), at.capture(), contains("ACTIVE_CASE"));
        assertTrue(!at.getValue().isBefore(before.plusSeconds(600)),
                "無時間界 hold 必須至少推 recheck 間隔，否則會每 30 秒打一次跨服務呼叫");
        assertEquals(1d, heldCount("ACTIVE_CASE"));
    }

    @Test
    @DisplayName("aftersales 反查失敗（client fail-closed 回 LOOKUP_FAILED）⇒ 不撥款、不消耗重試額度")
    void lookupFailed_failsClosedWithoutBurningRetries() {
        stubClaimed();
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(true, null, PayoutHoldReason.LOOKUP_FAILED, null));
        when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(1);

        worker.execute();

        verifyNoInteractions(paymentGrpcClient);
        verify(grantPaymentTaskRepository).deferForPayoutHold(eq(TASK_ID), any(), contains("LOOKUP_FAILED"));
        verify(grantPaymentTaskRepository, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskManualIntervention(anyLong(), anyString());
        assertEquals(1d, heldCount("LOOKUP_FAILED"));
    }

    @Test
    @DisplayName("Case D：AutoComplete 路徑（confirm_type=AUTO）與買家確認收貨（MANUAL）過同一道閘門")
    void bothCompletionPathsPassThroughTheSameGate() {
        for (String confirmType : List.of(
                io.github.will413028.hawkr.order.constant.OrderConstants.CONFIRM_TYPE_MANUAL,
                io.github.will413028.hawkr.order.constant.OrderConstants.CONFIRM_TYPE_AUTO)) {
            org.mockito.Mockito.reset(grantPaymentTaskRepository, afterSalesGrpcClient, paymentGrpcClient);
            GrantPaymentTaskEntity t = task();
            t.setConfirmType(confirmType);
            when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(t));
            when(grantPaymentTaskRepository.claimTask(eq(TASK_ID), any())).thenReturn(1);
            when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                    new PayoutHold(true, DateTimeUtils.nowUtc().plusHours(71),
                            PayoutHoldReason.APPEAL_WINDOW_OPEN, "AS1"));
            when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(1);

            worker.execute();

            verify(afterSalesGrpcClient).getPayoutHold(ORDER_NO);
            verify(grantPaymentTaskRepository).deferForPayoutHold(eq(TASK_ID), any(), anyString());
            verify(paymentGrpcClient, never()).grantPayment(anyString());
        }
    }

    // ==================== Case E：窗關了就撥 ====================

    @Test
    @DisplayName("Case E：hold 解除 ⇒ 正常撥款（Hotfix 不得讓賣家款項永久卡住）")
    void released_grantsNormally() {
        stubClaimed();
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(false, null, PayoutHoldReason.NONE, null));
        when(paymentGrpcClient.grantPayment(ORDER_NO)).thenReturn(
                GrantPaymentResponse.newBuilder().setCode(0).setMessage("success").build());
        when(grantPaymentTaskRepository.markGrantTaskDone(TASK_ID)).thenReturn(1);

        worker.execute();

        verify(paymentGrpcClient).grantPayment(ORDER_NO);
        verify(grantPaymentTaskRepository).markGrantTaskDone(TASK_ID);
        verify(grantPaymentTaskRepository, never()).deferForPayoutHold(anyLong(), any(), anyString());
    }

    // ==================== Case F：重複取得 / 推遲競態 ====================

    @Test
    @DisplayName("Case F：租約沒搶到（claim=0）⇒ 連反查都不做，一個副作用都沒有")
    void claimLost_noLookupNoGrant() {
        when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(grantPaymentTaskRepository.claimTask(eq(TASK_ID), any())).thenReturn(0);

        worker.execute();

        verifyNoInteractions(afterSalesGrpcClient);
        verifyNoInteractions(paymentGrpcClient);
        verify(grantPaymentTaskRepository, never()).deferForPayoutHold(anyLong(), any(), anyString());
    }

    @Test
    @DisplayName("Case F：推遲 CAS 回 0（租約已易主 / admin 剛重發）⇒ 冪等 skip，不重複記帳、不撥款")
    void deferCasLost_idempotentSkip() {
        stubClaimed();
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(true, DateTimeUtils.nowUtc().plusHours(1), PayoutHoldReason.APPEAL_WINDOW_OPEN, "AS1"));
        when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(0);

        worker.execute();

        verifyNoInteractions(paymentGrpcClient);
        assertEquals(0d, heldCount("APPEAL_WINDOW_OPEN"), "CAS 沒生效就不該記一次 hold");
    }

    // ==================== Case G：重跑必須重新過閘 ====================

    @Test
    @DisplayName("Case G：同一筆任務被重複處理（crash 後重跑 / 租約到期）⇒ 每一次都重新反查資格")
    void everyRunReChecksEligibility() {
        stubClaimed();
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(true, DateTimeUtils.nowUtc().plusHours(1), PayoutHoldReason.APPEAL_WINDOW_OPEN, "AS1"));
        when(grantPaymentTaskRepository.deferForPayoutHold(eq(TASK_ID), any(), anyString())).thenReturn(1);

        worker.execute();
        worker.execute();
        worker.execute();

        // 閘門在執行點：資格不是建立時算一次就算數的，每次要撥款前都得重問
        verify(afterSalesGrpcClient, org.mockito.Mockito.times(3)).getPayoutHold(ORDER_NO);
        verifyNoInteractions(paymentGrpcClient);
    }

    @Test
    @DisplayName("閘門在 claim 之後、payment 之前：先 claim → 再反查 → 才可能撥款")
    void gateSitsBetweenClaimAndPayment() {
        stubClaimed();
        when(afterSalesGrpcClient.getPayoutHold(ORDER_NO)).thenReturn(
                new PayoutHold(false, null, PayoutHoldReason.NONE, null));
        when(paymentGrpcClient.grantPayment(ORDER_NO)).thenReturn(
                GrantPaymentResponse.newBuilder().setCode(0).setMessage("success").build());
        when(grantPaymentTaskRepository.markGrantTaskDone(TASK_ID)).thenReturn(1);

        worker.execute();

        var inOrder = org.mockito.Mockito.inOrder(grantPaymentTaskRepository, afterSalesGrpcClient, paymentGrpcClient);
        inOrder.verify(grantPaymentTaskRepository).claimTask(eq(TASK_ID), any());
        inOrder.verify(afterSalesGrpcClient).getPayoutHold(ORDER_NO);
        inOrder.verify(paymentGrpcClient).grantPayment(ORDER_NO);
    }
}
