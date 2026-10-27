package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.mq.dto.PayoutCompletedPayload;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentResult;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
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
 * GrantPaymentWorker 任務終態分流（03 §3.11.2 / §8.8）。
 *
 * <p>收斂唯一依據 = gRPC 回應的結構化 {@code grant_outcome}：
 * PROVIDER_TOOK_OVER → PROVIDER_AUTO（不發 payout 通知）；GRANTED → DONE；
 * 業務錯誤 / 例外 → 既有 MANUAL_INTERVENTION / retry 路徑。
 */
@ExtendWith(MockitoExtension.class)
class GrantPaymentWorkerTest {

    @Mock private GrantPaymentTaskRepository grantPaymentTaskRepository;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private OrderRepository orderRepository;
    @Mock private OrderEventPublisher orderEventPublisher;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient afterSalesGrpcClient;

    private GrantPaymentWorker worker;

    @BeforeEach
    void setUp() {
        worker = new GrantPaymentWorker(grantPaymentTaskRepository, paymentGrpcClient,
                orderRepository, orderEventPublisher, sellerGrpcClient,
                afterSalesGrpcClient, new io.github.will413028.hawkr.order.config.OrderConfig(),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    private GrantPaymentTaskEntity task() {
        GrantPaymentTaskEntity t = new GrantPaymentTaskEntity();
        t.setTaskId(1L);
        t.setOrderNo("O123");
        t.setRetryCount(0);
        t.setMaxRetryCount(10);
        return t;
    }

    private void stubOneClaimedTask() {
        when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(grantPaymentTaskRepository.claimTask(eq(1L), any())).thenReturn(1);
        // backend#3262：本檔驗的是「已放行之後」的終態分流，撥款資格閘門一律放行。
        // 閘門本身的行為由 GrantPaymentWorkerPayoutHoldTest 覆蓋。
        when(afterSalesGrpcClient.getPayoutHold("O123")).thenReturn(
                new io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHold(
                        false, null,
                        io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHoldReason.NONE, null));
    }

    @Test
    @DisplayName("grant_outcome=PROVIDER_TOOK_OVER → task 收斂 PROVIDER_AUTO：不 DONE、不發 payout、不進人工介入")
    void providerTookOver_convergesToProviderAuto() {
        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome(OrderConstants.GRANT_OUTCOME_PROVIDER_TOOK_OVER)
                .build());
        when(grantPaymentTaskRepository.markGrantTaskProviderAuto(eq(1L), anyString())).thenReturn(1);

        worker.execute();

        verify(grantPaymentTaskRepository).markGrantTaskProviderAuto(eq(1L), contains("PROVIDER_TOOK_OVER"));
        verify(grantPaymentTaskRepository, never()).markGrantTaskDone(anyLong());
        verify(grantPaymentTaskRepository, never()).markGrantTaskManualIntervention(anyLong(), anyString());
        verify(orderEventPublisher, never()).publishPayoutCompleted(any());
    }

    @Test
    @DisplayName("grant_outcome=GRANTED → task DONE + 發 payout.completed 通知")
    void granted_marksDoneAndPublishesPayout() {
        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome("GRANTED")
                .setData(GrantPaymentResult.newBuilder()
                        .setTradeNo("O123001").setAmount("500").setGrantedAt("123").build())
                .build());
        when(grantPaymentTaskRepository.markGrantTaskDone(1L)).thenReturn(1);
        when(orderRepository.findStoreIdByOrderNo("O123")).thenReturn(9L);

        worker.execute();

        verify(grantPaymentTaskRepository).markGrantTaskDone(1L);
        verify(grantPaymentTaskRepository, never()).markGrantTaskProviderAuto(anyLong(), anyString());
        verify(orderEventPublisher).publishPayoutCompleted(any());
    }

    @Test
    @DisplayName("grant_outcome=GRANTED → PayoutCompletedPayload 帶解析後 account_id（非原始 store_id，discriminating）")
    void granted_resolvesStoreIdToAccountId_inPayoutPayload() {
        long storeId = 7001L;
        long resolvedAccountId = 42L;

        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome("GRANTED")
                .setData(GrantPaymentResult.newBuilder()
                        .setTradeNo("O123001").setAmount("500").setGrantedAt("123").build())
                .build());
        when(grantPaymentTaskRepository.markGrantTaskDone(1L)).thenReturn(1);
        when(orderRepository.findStoreIdByOrderNo("O123")).thenReturn(storeId);
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId))).thenReturn(PrincipalResolution.allResolved(Map.of(storeId, resolvedAccountId)));

        worker.execute();

        ArgumentCaptor<PayoutCompletedPayload> captor = ArgumentCaptor.forClass(PayoutCompletedPayload.class);
        verify(orderEventPublisher).publishPayoutCompleted(captor.capture());
        assertEquals(String.valueOf(resolvedAccountId), captor.getValue().notifySellerAccountId(),
                "PayoutCompletedPayload.notifySellerAccountId must be the resolved login account_id (42), "
                        + "not store_id (7001)");
        assertEquals(String.valueOf(storeId), captor.getValue().storeId(),
                "PayoutCompletedPayload.storeId must be the raw store_id (7001), independent of the "
                        + "resolved account_id");
    }

    @Test
    @DisplayName("grant_outcome=GRANTED + resolvePrincipals 拋例外 → payout 通知 fail-closed 帶 0，但 task 仍正常標記 DONE（不中斷撥款業務）")
    void granted_resolvePrincipalsThrows_failsClosedToZero_stillMarksDone() {
        long storeId = 7001L;

        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome("GRANTED")
                .setData(GrantPaymentResult.newBuilder()
                        .setTradeNo("O123001").setAmount("500").setGrantedAt("123").build())
                .build());
        when(grantPaymentTaskRepository.markGrantTaskDone(1L)).thenReturn(1);
        when(orderRepository.findStoreIdByOrderNo("O123")).thenReturn(storeId);
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId))).thenThrow(new RuntimeException("UNAVAILABLE"));

        assertDoesNotThrow(() -> worker.execute(),
                "resolvePrincipals 失敗只能降級通知欄位，不可讓例外逸出 processTask 影響撥款 task 的 DONE 狀態");

        verify(grantPaymentTaskRepository).markGrantTaskDone(1L);

        ArgumentCaptor<PayoutCompletedPayload> captor = ArgumentCaptor.forClass(PayoutCompletedPayload.class);
        verify(orderEventPublisher).publishPayoutCompleted(captor.capture());
        assertEquals("0", captor.getValue().notifySellerAccountId(),
                "resolvePrincipals 失敗時必須 fail-closed 帶 0（絕不可誤用原始 store_id 7001 或洩漏例外）");
        assertEquals(String.valueOf(storeId), captor.getValue().storeId(),
                "storeId 不依賴 resolvePrincipals 是否成功，resolvePrincipals 失敗不得連帶讓 storeId 降級");
    }

    @Test
    @DisplayName("grant_outcome=GRANTED + store_id 查無存活賣家（unresolved_ids）→ payout 通知帶 0，不回吐 store_id 冒充 account_id")
    void granted_unresolvedStore_isNotEchoedAsAccount() {
        long storeId = 7001L;

        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome("GRANTED")
                .setData(GrantPaymentResult.newBuilder()
                        .setTradeNo("O123001").setAmount("500").setGrantedAt("123").build())
                .build());
        when(grantPaymentTaskRepository.markGrantTaskDone(1L)).thenReturn(1);
        when(orderRepository.findStoreIdByOrderNo("O123")).thenReturn(storeId);
        // 舊 seller-service 會把輸入回填進 principals；unresolved_ids 才是真相。
        when(sellerGrpcClient.resolvePrincipals(List.of(storeId)))
                .thenReturn(PrincipalResolution.of(Map.of(storeId, storeId), List.of(storeId)));

        worker.execute();

        ArgumentCaptor<PayoutCompletedPayload> captor = ArgumentCaptor.forClass(PayoutCompletedPayload.class);
        verify(orderEventPublisher).publishPayoutCompleted(captor.capture());
        assertEquals("0", captor.getValue().notifySellerAccountId());
        assertEquals(String.valueOf(storeId), captor.getValue().storeId());
    }

    @Test
    @DisplayName("grant_outcome=GRANTED + 訂單查無 store_id（storeId 為 null）→ PayoutCompletedPayload.storeId 帶空字串，task 仍正常標記 DONE")
    void granted_storeIdNull_storeIdEmitsEmptyString_stillMarksDone() {
        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setGrantOutcome("GRANTED")
                .setData(GrantPaymentResult.newBuilder()
                        .setTradeNo("O123001").setAmount("500").setGrantedAt("123").build())
                .build());
        when(grantPaymentTaskRepository.markGrantTaskDone(1L)).thenReturn(1);
        when(orderRepository.findStoreIdByOrderNo("O123")).thenReturn(null);

        assertDoesNotThrow(() -> worker.execute(),
                "storeId 為 null（訂單查無）不可讓例外逸出 processTask，撥款 task 仍必須標記 DONE");

        verify(grantPaymentTaskRepository).markGrantTaskDone(1L);

        ArgumentCaptor<PayoutCompletedPayload> captor = ArgumentCaptor.forClass(PayoutCompletedPayload.class);
        verify(orderEventPublisher).publishPayoutCompleted(captor.capture());
        assertEquals("", captor.getValue().storeId(),
                "storeId 為 null 時 PayoutCompletedPayload.storeId 必須是空字串，不可拋出 NPE 或誤帶 \"null\" 字面值");
    }

    @Test
    @DisplayName("code!=0（業務錯誤）→ 既有路徑 MANUAL_INTERVENTION（不收斂 PROVIDER_AUTO）")
    void bizError_manualIntervention_unchanged() {
        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenReturn(GrantPaymentResponse.newBuilder()
                .setCode(5000)
                .setMessage("撥款未成功")
                .build());
        when(grantPaymentTaskRepository.markGrantTaskManualIntervention(eq(1L), anyString())).thenReturn(1);

        worker.execute();

        verify(grantPaymentTaskRepository).markGrantTaskManualIntervention(eq(1L), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskProviderAuto(anyLong(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskDone(anyLong());
    }

    @Test
    @DisplayName("gRPC 例外 → 既有 retry 排程路徑（FAILED + next_retry_time）")
    void exception_schedulesRetry_unchanged() {
        stubOneClaimedTask();
        when(paymentGrpcClient.grantPayment("O123")).thenThrow(new RuntimeException("UNAVAILABLE"));
        when(grantPaymentTaskRepository.markGrantTaskRetryScheduled(eq(1L), eq(1), any(), anyString()))
                .thenReturn(1);

        worker.execute();

        verify(grantPaymentTaskRepository).markGrantTaskRetryScheduled(eq(1L), eq(1), any(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskProviderAuto(anyLong(), anyString());
    }

    /**
     * issue #2294 C4 鑑別力測試：claim 回 0（另一個 pod 持有租約）⇒ <b>一個副作用都不做</b>。
     *
     * <p>⚠️ 這一支非有不可：Mockito 對 {@code int} 的預設回傳是 <b>0</b>，
     * 所以「忘記 stub claim」與「claim 真的回 0」在測試裡長得一模一樣——
     * 若沒有本測試，把 claim 閘門整個拿掉不會有任何測試轉紅。
     * 本測試斷言的是<b>相反方向</b>：claim 失敗時 gRPC 與所有狀態寫入都不得發生。
     */
    @Test
    @DisplayName("claim 回 0（另一個 pod 持有租約）→ 不打 gRPC、不寫任何狀態、不發 payout")
    void claimLost_doesNothing() {
        when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(grantPaymentTaskRepository.claimTask(eq(1L), any())).thenReturn(0);

        worker.execute();

        verifyNoInteractions(paymentGrpcClient);
        verify(grantPaymentTaskRepository, never()).markGrantTaskDone(anyLong());
        verify(grantPaymentTaskRepository, never()).markGrantTaskProviderAuto(anyLong(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskManualIntervention(anyLong(), anyString());
        verify(grantPaymentTaskRepository, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
        verify(orderEventPublisher, never()).publishPayoutCompleted(any());
    }

    /**
     * issue #2294 C4：claim 必須帶「未來時間」的租約（lease），不是永久旗標。
     *
     * <p>並機械守住不變式 <b>租約長度 &gt; grantPayment 的 deadline</b>
     * （《多副本排程搶佔規範》§三第 5 步）。
     */
    @Test
    @DisplayName("claim 帶未來租約時間，且租約長度 > grantPayment deadline")
    void claimUsesFutureLease_longerThanGrantDeadline() {
        org.junit.jupiter.api.Assertions.assertTrue(
                GrantPaymentWorker.CLAIM_LEASE_SEC
                        > io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC,
                "租約長度必須大於 grantPayment 的 deadline，否則原持有者還在跑租約就到期，搶佔等於沒有");

        when(grantPaymentTaskRepository.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(grantPaymentTaskRepository.claimTask(eq(1L), any())).thenReturn(0);

        java.time.LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        worker.execute();
        java.time.LocalDateTime after = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();

        org.mockito.ArgumentCaptor<java.time.LocalDateTime> captor =
                org.mockito.ArgumentCaptor.forClass(java.time.LocalDateTime.class);
        verify(grantPaymentTaskRepository).claimTask(eq(1L), captor.capture());
        java.time.LocalDateTime lease = captor.getValue();
        org.junit.jupiter.api.Assertions.assertFalse(
                lease.isBefore(before.plusSeconds(GrantPaymentWorker.CLAIM_LEASE_SEC)),
                "租約不得早於 now + CLAIM_LEASE_SEC");
        org.junit.jupiter.api.Assertions.assertFalse(
                lease.isAfter(after.plusSeconds(GrantPaymentWorker.CLAIM_LEASE_SEC)),
                "租約不得晚於 now + CLAIM_LEASE_SEC");
    }
}
