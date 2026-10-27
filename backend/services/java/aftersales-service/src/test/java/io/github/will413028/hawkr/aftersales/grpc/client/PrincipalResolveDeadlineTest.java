package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.github.will413028.hawkr.seller.grpc.SellerBatchResponse;
import io.github.will413028.hawkr.seller.grpc.SellerInfo;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>P1-C</b>：身分反解的兩支 gRPC 必須有 deadline。
 *
 * <p><b>這一支測試在防什麼</b>：P1-2 把身分反解從 tx 內搬到 tx 外，解掉了「無界呼叫持有 row lock」，
 * 但沒有解掉「無界呼叫」本身——它換成了「無界呼叫佔住排程執行緒」。aftersales <b>沒有設定</b>
 * {@code spring.task.scheduling.pool.size} ⇒ Spring Boot 預設單一排程執行緒 ⇒ seller／buyer service
 * 若是「掛住」而非「失敗」，整個 aftersales 的<b>所有</b>排程一起停擺且永不恢復。
 * P1-1 用來論證 {@code createRefund} 需要 deadline 的理由逐字適用於這兩支。
 *
 * <p><b>三組斷言</b>：
 * <ol>
 *   <li><b>結構</b>（真正的鑑別點）：RPC 是發在<b>帶 deadline 的 stub</b> 上。
 *       把 {@code withDeadlineAfter} 拿掉 ⇒ RPC 落在原始 stub ⇒ 本組轉紅。</li>
 *   <li><b>數值邊界</b>：deadline 落在推導出的區間內。任一端被單獨調整即轉紅。</li>
 *   <li><b>失敗語意</b>：{@code DEADLINE_EXCEEDED} 觸發後，strict 路徑拋、best-effort 路徑降級。</li>
 * </ol>
 */
class PrincipalResolveDeadlineTest {

    private static final long STORE_ID = 7001L;
    private static final long BUYER_ID = 9001L;

    private SellerServiceGrpc.SellerServiceBlockingStub rawSellerStub;
    private SellerServiceGrpc.SellerServiceBlockingStub deadlineSellerStub;
    private BuyerServiceGrpc.BuyerServiceBlockingStub rawBuyerStub;
    private BuyerServiceGrpc.BuyerServiceBlockingStub deadlineBuyerStub;
    /** issue #3332：{@code getSellerBatch} 走的是另一個常數（{@code INTERNAL_READ_DEADLINE_SEC}）。 */
    private SellerServiceGrpc.SellerServiceBlockingStub readDeadlineSellerStub;

    private SellerGrpcClient sellerGrpcClient;
    private BuyerGrpcClient buyerGrpcClient;

    @BeforeEach
    @SuppressWarnings("unchecked")
    void setUp() {
        rawSellerStub = mock(SellerServiceGrpc.SellerServiceBlockingStub.class);
        deadlineSellerStub = mock(SellerServiceGrpc.SellerServiceBlockingStub.class);
        rawBuyerStub = mock(BuyerServiceGrpc.BuyerServiceBlockingStub.class);
        deadlineBuyerStub = mock(BuyerServiceGrpc.BuyerServiceBlockingStub.class);
        readDeadlineSellerStub = mock(SellerServiceGrpc.SellerServiceBlockingStub.class);

        // 只有「帶對 deadline」的那次 withDeadlineAfter 才會拿到 deadlineStub；
        // 其餘（含完全不呼叫）都會停在 rawStub 上 ⇒ 下面的 verify 轉紅。
        when(rawSellerStub.withDeadlineAfter(
                SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS))
                .thenReturn(deadlineSellerStub);
        when(rawBuyerStub.withDeadlineAfter(
                BuyerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS))
                .thenReturn(deadlineBuyerStub);
        when(rawSellerStub.withDeadlineAfter(
                SellerGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS))
                .thenReturn(readDeadlineSellerStub);

        GrpcStubFactory sellerFactory = mock(GrpcStubFactory.class);
        when(sellerFactory.blocking(any(), any())).thenReturn(rawSellerStub);
        GrpcStubFactory buyerFactory = mock(GrpcStubFactory.class);
        when(buyerFactory.blocking(any(), any())).thenReturn(rawBuyerStub);

        sellerGrpcClient = new SellerGrpcClient(sellerFactory);
        buyerGrpcClient = new BuyerGrpcClient(buyerFactory);
    }

    // ==================== 1. 結構：RPC 必須發在帶 deadline 的 stub 上 ====================

    @Test
    @DisplayName("SellerGrpcClient.getSellerProfiles：結果以 store_id 建索引，不以 account_id 建索引")
    void sellerProfiles_areKeyedByStoreId() {
        when(readDeadlineSellerStub.getSellerBatch(any()))
                .thenReturn(SellerBatchResponse.newBuilder()
                        .addData(SellerInfo.newBuilder()
                                .setAccountId(42L)
                                .setStoreId(String.valueOf(STORE_ID))
                                .setName("鮮魚舖")
                                .build())
                        .build());

        var result = sellerGrpcClient.getSellerProfiles(List.of(STORE_ID));

        assertTrue(result.containsKey(STORE_ID), "結果應以 store_id 建索引");
        assertTrue(!result.containsKey(42L), "不得再以 principal account_id 建索引");

        // issue #3332：RPC 必須發在帶 deadline 的 stub 上。拿掉 withDeadlineAfter ⇒
        // getSellerBatch 落在 rawSellerStub（未 stub，回 null）⇒ catch 降級成空 map
        // ⇒ 上面兩行先轉紅；這一行則直接指出成因。
        verify(rawSellerStub).withDeadlineAfter(
                SellerGrpcClient.INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS);
        verify(rawSellerStub, org.mockito.Mockito.never()).getSellerBatch(any());
    }

    @Test
    @DisplayName("SellerGrpcClient.resolveAccountIdForStore：unresolved_ids 的 store 回 0L，不回吐 store_id")
    void sellerResolve_unresolvedStoreYieldsZeroNotTheStoreId() {
        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenReturn(io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse.newBuilder()
                        .addUnresolvedIds(STORE_ID)
                        .build());

        assertEquals(0L, sellerGrpcClient.resolveAccountIdForStore(STORE_ID));
    }

    @Test
    @DisplayName("SellerGrpcClient.resolveAccountIdForStore：RPC 發在帶 deadline 的 stub 上")
    void sellerResolve_issuesRpcOnDeadlineBearingStub() {
        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenReturn(io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse.newBuilder()
                        .putPrincipals(STORE_ID, 42L)
                        .build());

        assertEquals(42L, sellerGrpcClient.resolveAccountIdForStore(STORE_ID));

        verify(rawSellerStub).withDeadlineAfter(
                SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS);
        verify(deadlineSellerStub).resolvePrincipals(any());
        // 拿掉 withDeadlineAfter ⇒ RPC 打在 rawSellerStub 上 ⇒ 這一行轉紅
        verify(rawSellerStub, org.mockito.Mockito.never()).resolvePrincipals(any());
    }

    @Test
    @DisplayName("BuyerGrpcClient.resolvePrincipals：RPC 發在帶 deadline 的 stub 上")
    void buyerResolve_issuesRpcOnDeadlineBearingStub() {
        when(deadlineBuyerStub.resolvePrincipals(any()))
                .thenReturn(io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsResponse.newBuilder()
                        .setCode(0)
                        .putPrincipals(BUYER_ID, 55L)
                        .build());

        assertEquals(55L, buyerGrpcClient.resolvePrincipals(List.of(BUYER_ID))
                .principals().get(BUYER_ID));

        verify(rawBuyerStub).withDeadlineAfter(
                BuyerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS);
        verify(deadlineBuyerStub).resolvePrincipals(any());
        verify(rawBuyerStub, org.mockito.Mockito.never()).resolvePrincipals(any());
    }

    @Test
    @DisplayName("deadline 以「每次呼叫」為起點：呼叫兩次就要換算兩次，不得快取成絕對時間")
    void deadlineIsAppliedPerCall_notCachedInConstructor() {
        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenReturn(io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse.newBuilder()
                        .putPrincipals(STORE_ID, 42L)
                        .build());

        sellerGrpcClient.resolveAccountIdForStore(STORE_ID);
        sellerGrpcClient.resolveAccountIdForStore(STORE_ID);

        // withDeadlineAfter 是以「呼叫當下」換算成絕對 deadline。若被挪進建構子（只換算一次），
        // 服務啟動 N 秒後所有呼叫一律 DEADLINE_EXCEEDED —— 這一行是它的機械防線。
        verify(rawSellerStub, org.mockito.Mockito.times(2)).withDeadlineAfter(
                SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS);
    }

    // ==================== 2. 數值邊界 ====================

    /** 那兩支排程（AutoConfirmReceiptScheduler／StorePickupTimeoutScheduler）的 BATCH_LIMIT。 */
    private static final int SCHEDULER_BATCH_LIMIT = 200;
    /** 一筆 case 的一次身分反解 = seller + buyer 兩支 RPC。 */
    private static final int RPCS_PER_CASE = 2;
    /**
     * 單一排程執行緒可接受的最壞單 tick 佔用（秒）。
     * 取 2000 秒＝本服務目前已被接受的同量級上限（{@code RefundTaskWorker} 的
     * {@code LIMIT 10 × CREATE_REFUND_DEADLINE_SEC 100s} = 1000 秒的兩倍）。
     */
    private static final long MAX_TICK_OCCUPANCY_SEC = 2000L;

    @Test
    @DisplayName("反解 deadline 必須 > 0 且遠小於 createRefund 的 100 秒（內部查表 ≠ 第三方往返）")
    void principalResolveDeadline_isOrderOfMagnitudeBelowCreateRefundDeadline() {
        long d = SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC;
        assertTrue(d > 0, "身分反解必須有 deadline（0/負值＝無界，正是 P1-C 要修的東西）");
        assertTrue(d * 10 <= PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC,
                "反解(" + d + "s) 必須至少比 CreateRefund("
                        + PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC
                        + "s) 小一個數量級：後者把 PayUni 的 90 秒最壞往返夾在中間，"
                        + "前者只是 seller/buyer 的一次 PK 查表，兩者不同量級");
    }

    @Test
    @DisplayName("反解 deadline 必須 ≤ 排程執行緒可容忍上限（BATCH_LIMIT × 2 支 × deadline）")
    void principalResolveDeadline_keepsSchedulerTickBounded() {
        long d = SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC;
        long worstTick = (long) SCHEDULER_BATCH_LIMIT * RPCS_PER_CASE * d;
        assertTrue(worstTick <= MAX_TICK_OCCUPANCY_SEC,
                "最壞單 tick 佔用 " + worstTick + "s（" + SCHEDULER_BATCH_LIMIT + " 筆 × "
                        + RPCS_PER_CASE + " 支 × " + d + "s）超過單一排程執行緒的可容忍上限 "
                        + MAX_TICK_OCCUPANCY_SEC + "s。aftersales 未設定 "
                        + "spring.task.scheduling.pool.size ⇒ 只有一條排程執行緒，"
                        + "調大這個 deadline 等於按比例拉長所有其他排程的停擺時間");
    }

    @Test
    @DisplayName("下界：不得低於本 repo 既有先例（cart-service 2s，且該處還有一次重試）")
    void principalResolveDeadline_notBelowExistingPrecedent() {
        // cart-service SellerGrpcClient.CALL_DEADLINE_SECONDS = 2s，配 MAX_ATTEMPTS=2 ⇒ 實質 4s。
        // 本呼叫點沒有重試，一次就得涵蓋 channel 重建 + GC 停頓 + 一次慢查詢，
        // 調到 2s 以下會開始製造假的 DEADLINE_EXCEEDED（strict 路徑＝整筆延後一個 10 分鐘 tick）。
        assertTrue(SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC >= 2L,
                "反解 deadline 不得低於 2 秒");
    }

    // ==================== 3. 失敗語意：DEADLINE_EXCEEDED 之後怎麼走 ====================

    @Test
    @DisplayName("seller 逾時：client 內部吸收回 null（不拋），永遠只有這一種 failure mode")
    void sellerDeadlineExceeded_degradesToNullWithoutThrowing() {
        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));

        assertNull(sellerGrpcClient.resolveAccountIdForStore(STORE_ID));
    }

    @Test
    @DisplayName("buyer 逾時：client 原樣往上拋 StatusRuntimeException，降級與否由呼叫端決定")
    void buyerDeadlineExceeded_propagates() {
        when(deadlineBuyerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));

        StatusRuntimeException e = assertThrows(StatusRuntimeException.class,
                () -> buyerGrpcClient.resolvePrincipals(List.of(BUYER_ID)));
        assertEquals(Status.Code.DEADLINE_EXCEEDED, e.getStatus().getCode());
    }

    @Test
    @DisplayName("strict（排程）：seller 側逾時 ⇒ PrincipalResolutionFailedException ⇒ 本筆本輪跳過、下一 tick 重撈")
    void strictPath_sellerDeadlineExceeded_throwsAndSkipsCase() {
        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(
                mock(ObsRabbitPublisher.class), sellerGrpcClient, buyerGrpcClient);

        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));

        assertThrows(PrincipalResolutionFailedException.class,
                () -> publisher.resolveEventPrincipals(STORE_ID, BUYER_ID));
    }

    @Test
    @DisplayName("strict（排程）：buyer 側逾時 ⇒ 同樣拋，不得退回 buyer_id 自身把事件照發")
    void strictPath_buyerDeadlineExceeded_throwsAndSkipsCase() {
        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(
                mock(ObsRabbitPublisher.class), sellerGrpcClient, buyerGrpcClient);

        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenReturn(io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse.newBuilder()
                        .putPrincipals(STORE_ID, 42L)
                        .build());
        when(deadlineBuyerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));

        assertThrows(PrincipalResolutionFailedException.class,
                () -> publisher.resolveEventPrincipals(STORE_ID, BUYER_ID));
    }

    @Test
    @DisplayName("best-effort（API）：兩側逾時 ⇒ 降級成 (null, 0L)，永不拋（API 沒有下一個 tick）")
    void bestEffortPath_deadlineExceeded_degrades() {
        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(
                mock(ObsRabbitPublisher.class), sellerGrpcClient, buyerGrpcClient);

        when(deadlineSellerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));
        when(deadlineBuyerStub.resolvePrincipals(any()))
                .thenThrow(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));

        EventPrincipals p = publisher.resolveEventPrincipalsBestEffort(STORE_ID, BUYER_ID);

        assertNull(p.sellerAccountId(), "seller 反解失敗 ⇒ null ⇒ 呼叫端跳過 realtime 推播");
        assertEquals(0L, p.notifyBuyerAccountId(), "buyer 反解失敗 ⇒ 0L（既有降級語意，不變）");
    }
}
