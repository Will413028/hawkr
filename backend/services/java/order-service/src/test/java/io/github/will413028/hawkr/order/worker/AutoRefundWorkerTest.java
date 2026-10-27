package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.AutoRefundTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.repository.AutoRefundTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * #1150 同類：AutoRefundWorker 在 CreateRefund envelope-code 失敗時不得標 task DONE。
 *
 * <p>原 bug：PaymentGrpcClient.createRefund 不檢查 envelope code，PayUni 拒退（code=500）被當成功，
 * task 標 DONE 不再重試 → 買家錢未退。修復後 wrapper 在 code≠0 時拋 IllegalStateException，
 * worker 既有 catch 走 updateFailed → 重試 → MANUAL_INTERVENTION，不標 DONE。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AutoRefundWorkerTest {

    @Mock private AutoRefundTaskRepository autoRefundTaskRepository;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private OrderRepository orderRepository;
    @Mock private AutoRefundTxHandler autoRefundTxHandler;

    @InjectMocks private AutoRefundWorker worker;

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(300, 0); } catch (IllegalStateException ignored) { }
    }

    private AutoRefundTaskEntity nonAtmPendingTask() {
        AutoRefundTaskEntity t = new AutoRefundTaskEntity();
        t.setTaskId(123456789L);
        t.setOrderNo("O400280260");
        t.setTransactionId("T123");
        t.setAmount(500L);
        t.setPaymentMethod(PaymentMethod.CREDIT_CARD.value());  // 非 ATM → 走 createRefund
        t.setStatus(OrderConstants.TASK_STATUS_PENDING);
        t.setRetryCount(0);
        t.setMaxRetryCount(3);
        return t;
    }

    @Test
    void createRefundEnvelopeFailed_doesNotMarkDone_goesToUpdateFailed() {
        AutoRefundTaskEntity task = nonAtmPendingTask();
        when(autoRefundTaskRepository.findRetryable(anyInt())).thenReturn(List.of(task));
        when(autoRefundTaskRepository.claimTask(anyLong(), any())).thenReturn(1);
        // wrapper 在 envelope code≠0 時拋 IllegalStateException（修復後行為）
        when(paymentGrpcClient.createRefund(anyString(), anyLong(), anyString()))
                .thenThrow(new IllegalStateException("createRefund 失敗: code=500, message=退款失敗: PayUni 拒退"));

        worker.process();

        // 關鍵不變式：失敗不得標 DONE、不得進完成事務（markDone + financial_log 已抽 AutoRefundTxHandler）
        verify(autoRefundTaskRepository, never()).markDoneByStatus(anyLong(), any());
        verify(autoRefundTxHandler, never()).completeAutoRefund(any(), any());
        // 應走失敗重試路徑
        verify(autoRefundTaskRepository, times(1)).updateFailed(eq(123456789L), any(), any(), anyInt());
    }

    /**
     * issue #2294 C2 鑑別力測試：claim 回 0（另一個 pod 持有租約）⇒ <b>一個副作用都不做</b>。
     *
     * <p>⚠️ 這一支非有不可：Mockito 對 {@code int} 的預設回傳是 <b>0</b>，
     * 所以「忘記 stub claim」與「claim 真的回 0」在測試裡長得一模一樣——
     * 若只有上面那支 happy-path 測試，把 claim 閘門整個拿掉也不會有任何測試轉紅
     * （閘門一律成立、被斷言的副作用一次都不發生 ⇒ never() 反而更容易通過）。
     * 本測試斷言的是<b>相反方向</b>：claim 失敗時 gRPC 與後續狀態寫入都不得發生。
     */
    @Test
    void claimLost_doesNotCallGrpcOrWriteAnyStatus() {
        AutoRefundTaskEntity task = nonAtmPendingTask();
        when(autoRefundTaskRepository.findRetryable(anyInt())).thenReturn(List.of(task));
        when(autoRefundTaskRepository.claimTask(anyLong(), any())).thenReturn(0);

        worker.process();

        verifyNoInteractions(paymentGrpcClient);
        verify(autoRefundTxHandler, never()).completeAutoRefund(any(), any());
        verify(autoRefundTaskRepository, never()).markDoneByStatus(anyLong(), any());
        verify(autoRefundTaskRepository, never()).updateFailed(anyLong(), any(), any(), anyInt());
        verify(autoRefundTaskRepository, never()).markManualIntervention(anyLong());
    }

    /**
     * issue #2294 C2：claim 必須帶「未來時間」的租約（lease），不是永久旗標。
     *
     * <p>斷言租約到期時間 = 呼叫當下 + {@link AutoRefundWorker#CLAIM_LEASE_SEC}（容許執行耗時），
     * 並機械守住不變式 <b>租約長度 &gt; createRefund 的 deadline</b>
     * （《多副本排程搶佔規範》§三第 5 步：租約必須大於單筆工作的最壞耗時）。
     */
    @Test
    void claimUsesFutureLease_longerThanCreateRefundDeadline() {
        org.junit.jupiter.api.Assertions.assertTrue(
                AutoRefundWorker.CLAIM_LEASE_SEC > PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC,
                "租約長度必須大於 createRefund 的 deadline，否則原持有者還在跑租約就到期，搶佔等於沒有");

        AutoRefundTaskEntity task = nonAtmPendingTask();
        when(autoRefundTaskRepository.findRetryable(anyInt())).thenReturn(List.of(task));
        when(autoRefundTaskRepository.claimTask(anyLong(), any())).thenReturn(0);

        java.time.LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        worker.process();
        java.time.LocalDateTime after = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();

        org.mockito.ArgumentCaptor<java.time.LocalDateTime> captor =
                org.mockito.ArgumentCaptor.forClass(java.time.LocalDateTime.class);
        verify(autoRefundTaskRepository).claimTask(eq(123456789L), captor.capture());
        java.time.LocalDateTime lease = captor.getValue();
        org.junit.jupiter.api.Assertions.assertFalse(
                lease.isBefore(before.plusSeconds(AutoRefundWorker.CLAIM_LEASE_SEC)),
                "租約不得早於 now + CLAIM_LEASE_SEC");
        org.junit.jupiter.api.Assertions.assertFalse(
                lease.isAfter(after.plusSeconds(AutoRefundWorker.CLAIM_LEASE_SEC)),
                "租約不得晚於 now + CLAIM_LEASE_SEC");
    }
}
