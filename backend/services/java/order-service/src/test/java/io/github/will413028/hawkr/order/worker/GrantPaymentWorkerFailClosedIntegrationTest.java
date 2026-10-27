package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.aftersales.grpc.AfterSalesServiceGrpc;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldRequest;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.time.LocalDateTime;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.contains;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * backend#3262 P1-a：aftersales 反查失敗時，<b>整條 worker 路徑</b>的資金安全不變式。
 *
 * <p>與 {@link GrantPaymentWorkerPayoutHoldTest} 的差別，也是這支存在的唯一理由：
 * 那一支把 {@link AfterSalesGrpcClient} 整個 mock 掉、直接餵 {@code LOOKUP_FAILED} 進 worker，
 * 於是 client 的 fail-closed 實作被<b>假設</b>而非被<b>執行</b>——獨立 QA 把 catch 分支改成
 * fail-open，order-service 243 支測試全綠。
 *
 * <p>本檔用<b>真的</b> {@link AfterSalesGrpcClient}（mock 下移到 gRPC blocking stub），
 * 於是「client 判定 fail-closed」與「worker 據此不撥款」被綁在同一條斷言鏈上：
 * 任一端被改成 fail-open，這裡就轉紅。
 */
class GrantPaymentWorkerFailClosedIntegrationTest {

    private static final String ORDER_NO = "O3262FCW";
    private static final long TASK_ID = 4242L;

    private AfterSalesServiceGrpc.AfterSalesServiceBlockingStub stub;
    private GrantPaymentTaskRepository repo;
    private PaymentGrpcClient paymentGrpcClient;
    private OrderConfig orderConfig;
    private MeterRegistry meterRegistry;
    private GrantPaymentWorker worker;

    @BeforeEach
    void setUp() {
        stub = mock(AfterSalesServiceGrpc.AfterSalesServiceBlockingStub.class);
        when(stub.withDeadlineAfter(anyLong(), any(TimeUnit.class))).thenReturn(stub);
        GrpcStubFactory factory = mock(GrpcStubFactory.class);
        when(factory.blocking(eq("aftersales"), any())).thenReturn(stub);

        repo = mock(GrantPaymentTaskRepository.class);
        paymentGrpcClient = mock(PaymentGrpcClient.class);
        orderConfig = new OrderConfig();
        meterRegistry = new SimpleMeterRegistry();

        worker = new GrantPaymentWorker(repo, paymentGrpcClient,
                mock(OrderRepository.class), mock(OrderEventPublisher.class),
                mock(SellerGrpcClient.class),
                new AfterSalesGrpcClient(factory),      // ← 真的 client，不是 mock
                orderConfig, meterRegistry);
    }

    private GrantPaymentTaskEntity task() {
        GrantPaymentTaskEntity t = new GrantPaymentTaskEntity();
        t.setTaskId(TASK_ID);
        t.setOrderNo(ORDER_NO);
        t.setRetryCount(0);
        t.setMaxRetryCount(10);
        return t;
    }

    private void givenClaimedTask() {
        when(repo.findPendingTasks(anyInt())).thenReturn(List.of(task()));
        when(repo.claimTask(eq(TASK_ID), any())).thenReturn(1);
        when(repo.deferForPayoutHold(anyLong(), any(), anyString())).thenReturn(1);
    }

    /**
     * §7 的完整不變式組：撥款 0 次、任務不成功、不消耗業務重試額度、
     * next_retry_time 正確推遲，且下一次仍會重新查資格。
     */
    private void assertNoGrantAndProperlyDeferred(String situation) {
        // (1) 一次 Payment Grant 都不能發出——這是資損防線本體
        verify(paymentGrpcClient, never()).grantPayment(anyString());

        // (2) task 不得變成成功／終態
        verify(repo, never()).markGrantTaskDone(anyLong());
        verify(repo, never()).markGrantTaskProviderAuto(anyLong(), anyString());
        verify(repo, never()).markGrantTaskManualIntervention(anyLong(), anyString());

        // (3) 不得消耗業務重試額度（依賴故障不是撥款失敗；否則 aftersales 抖幾次就把
        //     任務推進 MANUAL_INTERVENTION，變成要人工救的假故障）
        verify(repo, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
        verify(repo, never()).markGrantTaskMaxRetryReached(anyLong(), anyInt(), anyString());

        // (4) next_retry_time 推遲到「無時間界 hold」的重問間隔（查不到 ⇒ 沒有可信時間界）
        ArgumentCaptor<LocalDateTime> next = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).deferForPayoutHold(eq(TASK_ID), next.capture(), contains("LOOKUP_FAILED"));
        long secs = ChronoUnit.SECONDS.between(DateTimeUtils.nowUtc(), next.getValue());
        assertTrue(Math.abs(secs - orderConfig.getPayoutHoldRecheckSeconds()) <= 5,
                situation + "：應推遲約 " + orderConfig.getPayoutHoldRecheckSeconds()
                        + " 秒（實際 " + secs + "）");

        // (5) 觀測面：hold 有記帳，運維看得到卡在哪一類
        assertEquals(1.0, meterRegistry.find("hawkr_grant_payout_held")
                .tag("reason", "LOOKUP_FAILED").counter().count());
    }

    @Test
    @DisplayName("AfterSales gRPC timeout ⇒ Payment Grant = 0 calls")
    void timeout_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));
        worker.execute();
        assertNoGrantAndProperlyDeferred("gRPC 逾時");
    }

    @Test
    @DisplayName("connection refused ⇒ Payment Grant = 0 calls")
    void connectionRefused_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenThrow(new StatusRuntimeException(
                        Status.UNAVAILABLE.withDescription("io exception: Connection refused")));
        worker.execute();
        assertNoGrantAndProperlyDeferred("連線被拒");
    }

    @Test
    @DisplayName("非預期 gRPC error ⇒ Payment Grant = 0 calls")
    void unexpectedError_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.INTERNAL));
        worker.execute();
        assertNoGrantAndProperlyDeferred("非預期 gRPC error");
    }

    @Test
    @DisplayName("回應格式壞掉（hold_until 非 ISO 8601）⇒ Payment Grant = 0 calls")
    void malformedResponse_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenReturn(GetOrderPayoutHoldResponse.newBuilder()
                        .setCode(0).setHold(true).setHoldUntil("garbage")
                        .setReason(GetOrderPayoutHoldResponse.Reason.APPEAL_WINDOW_OPEN).build());
        worker.execute();
        assertNoGrantAndProperlyDeferred("回應格式壞掉");
    }

    @Test
    @DisplayName("非 0 業務碼（hold 欄位為 proto 預設 false）⇒ Payment Grant = 0 calls")
    void nonZeroCode_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenReturn(GetOrderPayoutHoldResponse.newBuilder()
                        .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                        .setMessage("撥款資格反查失敗").build());
        worker.execute();
        assertNoGrantAndProperlyDeferred("非 0 業務碼");
    }

    @Test
    @DisplayName("未知 reason enum ＋ hold=true ⇒ Payment Grant = 0 calls（無時間界推遲）")
    void unknownReason_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenReturn(GetOrderPayoutHoldResponse.newBuilder()
                        .setCode(0).setHold(true).setReasonValue(9999).setCaseNo("ASX").build());
        worker.execute();

        verify(paymentGrpcClient, never()).grantPayment(anyString());
        verify(repo, never()).markGrantTaskDone(anyLong());
        verify(repo, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
        ArgumentCaptor<LocalDateTime> next = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).deferForPayoutHold(eq(TASK_ID), next.capture(), contains("ACTIVE_CASE"));
        long secs = ChronoUnit.SECONDS.between(DateTimeUtils.nowUtc(), next.getValue());
        assertTrue(Math.abs(secs - orderConfig.getPayoutHoldRecheckSeconds()) <= 5);
    }

    @Test
    @DisplayName("P0：APPEAL_IN_FLIGHT（閘門偵測到在途申訴）⇒ Payment Grant = 0 calls")
    void appealInFlight_noGrant() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenReturn(GetOrderPayoutHoldResponse.newBuilder()
                        .setCode(0).setHold(true)
                        .setReason(GetOrderPayoutHoldResponse.Reason.APPEAL_IN_FLIGHT)
                        .setCaseNo("AS1").build());
        worker.execute();

        verify(paymentGrpcClient, never()).grantPayment(anyString());
        verify(repo).deferForPayoutHold(eq(TASK_ID), any(), contains("APPEAL_IN_FLIGHT"));
        verify(repo, never()).markGrantTaskRetryScheduled(anyLong(), anyInt(), any(), anyString());
    }

    @Test
    @DisplayName("下一個 tick 必須重新查資格，不得沿用上一次的判定")
    void nextTickReQueriesEligibility() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        worker.execute();
        worker.execute();

        verify(stub, times(2)).getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class));
        verify(paymentGrpcClient, never()).grantPayment(anyString());
    }

    @Test
    @DisplayName("對照：aftersales 健康且回 hold=false ⇒ 撥款必須真的發動（斷言有鑑別力）")
    void healthyLookup_actuallyGrants() {
        givenClaimedTask();
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class)))
                .thenReturn(GetOrderPayoutHoldResponse.newBuilder()
                        .setCode(0).setHold(false)
                        .setReason(GetOrderPayoutHoldResponse.Reason.NONE).build());
        when(paymentGrpcClient.grantPayment(ORDER_NO))
                .thenReturn(io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse.newBuilder()
                        .setCode(0).build());
        when(repo.markGrantTaskDone(TASK_ID)).thenReturn(1);

        worker.execute();

        verify(paymentGrpcClient).grantPayment(ORDER_NO);
        verify(repo, never()).deferForPayoutHold(anyLong(), any(), anyString());
    }
}
