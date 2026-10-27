package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.aftersales.grpc.AfterSalesServiceGrpc;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldRequest;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHold;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient.PayoutHoldReason;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * backend#3262 P1-a：{@link AfterSalesGrpcClient#getPayoutHold} 的 <b>fail-closed 不變式</b>。
 *
 * <p><b>為什麼這支測試必須存在</b>（獨立 QA 的 FAIL 判定）：把 client 的 catch 分支從
 * {@code PayoutHold(true, …, LOOKUP_FAILED)} 改成 {@code PayoutHold(false, …, NONE)}
 * （＝aftersales 掛掉就照撥），order-service 全部 243 支測試<b>沒有一支轉紅</b>。
 * 根因是既有的 {@code GrantPaymentWorkerPayoutHoldTest} 用 Mockito 把
 * {@code AfterSalesGrpcClient} 整個換掉、直接餵 {@code LOOKUP_FAILED} 進去
 * ——那等於<b>假設</b>了 client 的行為，真正實作 fail-closed 的三個分支
 * （catch / {@code code != 0} / {@code toReason} 的 default）從來沒有被執行過。
 *
 * <p>本檔把 mock 的位置<b>下移一層</b>：mock 的是 gRPC blocking stub，
 * 跑的是<b>真的</b> {@link AfterSalesGrpcClient}。例外一律用 gRPC 自己的
 * {@link StatusRuntimeException} ＋ 真實 {@link Status} 碼，回應一律用真實的
 * generated proto 物件——沒有任何手寫的形狀假設。
 *
 * <p><b>不對稱的理由</b>（為什麼方向只能是 hold）：撥錯款<b>追不回來</b>
 * （全 repo 無賣家扣回機制），撥晚了只是等。所以「查不出來」必須等同「不准撥」。
 */
class AfterSalesGrpcClientPayoutHoldFailClosedTest {

    private static final String ORDER_NO = "O3262FC";

    private AfterSalesServiceGrpc.AfterSalesServiceBlockingStub stub;
    private AfterSalesGrpcClient client;

    @BeforeEach
    void setUp() {
        stub = mock(AfterSalesServiceGrpc.AfterSalesServiceBlockingStub.class);
        // withDeadlineAfter 回自己：讓真實 client 的「每次呼叫套 deadline」寫法照原樣跑完
        when(stub.withDeadlineAfter(anyLong(), any(TimeUnit.class))).thenReturn(stub);

        GrpcStubFactory factory = mock(GrpcStubFactory.class);
        when(factory.blocking(eq("aftersales"), any())).thenReturn(stub);

        client = new AfterSalesGrpcClient(factory);
    }

    private void givenRpcThrows(Throwable t) {
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class))).thenThrow(t);
    }

    private void givenRpcReturns(GetOrderPayoutHoldResponse resp) {
        when(stub.getOrderPayoutHold(any(GetOrderPayoutHoldRequest.class))).thenReturn(resp);
    }

    /** 每一種失敗形態的共同斷言：擋、無時間界、原因誠實標成查不到。 */
    private void assertFailedClosed(PayoutHold hold, String situation) {
        assertTrue(hold.hold(), situation + " ⇒ 必須 fail-closed（撥錯款追不回來）");
        assertEquals(PayoutHoldReason.LOOKUP_FAILED, hold.reason(),
                situation + " ⇒ 原因必須誠實標成 LOOKUP_FAILED，不得偽裝成 NONE/ACTIVE_CASE");
        assertNull(hold.holdUntil(),
                situation + " ⇒ 查不到就沒有可信的時間界，必須無時間界（呼叫端定期重問）");
    }

    // ==================== §7 逐項：五種 lookup 失敗 ====================

    @Test
    @DisplayName("AfterSales gRPC timeout（DEADLINE_EXCEEDED）⇒ hold，不得撥款")
    void deadlineExceeded_failsClosed() {
        givenRpcThrows(new StatusRuntimeException(Status.DEADLINE_EXCEEDED));
        assertFailedClosed(client.getPayoutHold(ORDER_NO), "gRPC 逾時");
    }

    @Test
    @DisplayName("connection refused（UNAVAILABLE）⇒ hold，不得撥款")
    void connectionRefused_failsClosed() {
        givenRpcThrows(new StatusRuntimeException(
                Status.UNAVAILABLE.withDescription("io exception: Connection refused")));
        assertFailedClosed(client.getPayoutHold(ORDER_NO), "aftersales 連不上");
    }

    @Test
    @DisplayName("非預期 gRPC error（INTERNAL / PERMISSION_DENIED …）⇒ hold，不得撥款")
    void unexpectedGrpcError_failsClosed() {
        for (Status s : new Status[]{Status.INTERNAL, Status.PERMISSION_DENIED,
                Status.UNIMPLEMENTED, Status.RESOURCE_EXHAUSTED, Status.CANCELLED}) {
            setUp();
            givenRpcThrows(new StatusRuntimeException(s));
            assertFailedClosed(client.getPayoutHold(ORDER_NO), "gRPC " + s.getCode());
        }
    }

    @Test
    @DisplayName("非 gRPC 的執行期例外（NPE 等）也要被接住 ⇒ hold，不得逃出去變成撥款失敗")
    void nonGrpcRuntimeException_failsClosed() {
        givenRpcThrows(new IllegalStateException("channel already shutdown"));
        assertFailedClosed(client.getPayoutHold(ORDER_NO), "非 gRPC 例外");
    }

    @Test
    @DisplayName("回非 0 業務碼（server 端 catch 到例外時就是這條）⇒ hold，不得撥款")
    void nonZeroBizCode_failsClosed() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                .setMessage("撥款資格反查失敗")
                .build());   // 注意：hold 預設 false，正是 fail-open 最容易溜過去的形狀
        assertFailedClosed(client.getPayoutHold(ORDER_NO), "非 0 業務碼");
    }

    @Test
    @DisplayName("回應格式壞掉（hold_until 不是 ISO 8601）⇒ hold，不得因解析失敗而放行")
    void malformedHoldUntil_failsClosed() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0)
                .setHold(true)
                .setHoldUntil("not-a-timestamp")
                .setReason(GetOrderPayoutHoldResponse.Reason.APPEAL_WINDOW_OPEN)
                .build());
        assertFailedClosed(client.getPayoutHold(ORDER_NO), "hold_until 解析失敗");
    }

    @Test
    @DisplayName("未知 reason enum（對端新增碼值 / 版本不一致）＋ hold=true ⇒ 無時間界擋住")
    void unknownReasonEnum_holdsIndefinitely() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0)
                .setHold(true)
                .setReasonValue(9999)          // UNRECOGNIZED
                .setCaseNo("ASX")
                .build());
        PayoutHold hold = client.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold(), "未知 reason 不得放行");
        assertEquals(PayoutHoldReason.ACTIVE_CASE, hold.reason(),
                "未知碼值一律歸為無時間界 hold（deny-by-default）");
        assertNull(hold.holdUntil());
    }

    @Test
    @DisplayName("REASON_UNSPECIFIED（proto 預設值，＝對端根本沒設）＋ hold=true ⇒ 無時間界擋住")
    void unspecifiedReason_holdsIndefinitely() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0).setHold(true).build());
        PayoutHold hold = client.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(PayoutHoldReason.ACTIVE_CASE, hold.reason());
    }

    // ==================== 對照組：證明上面那些斷言不是恆真 ====================

    @Test
    @DisplayName("對照：正常回 hold=false ⇒ 必須真的放行（否則賣家款項永久卡死）")
    void healthyRelease_isNotBlocked() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0).setHold(false)
                .setReason(GetOrderPayoutHoldResponse.Reason.NONE)
                .build());
        PayoutHold hold = client.getPayoutHold(ORDER_NO);
        assertFalse(hold.hold(), "健康路徑必須放行——本檔的 fail-closed 斷言才有鑑別力");
        assertEquals(PayoutHoldReason.NONE, hold.reason());
    }

    @Test
    @DisplayName("對照：有時間界的 hold 必須把 hold_until 帶回來（推遲時點的唯一來源）")
    void boundedHold_carriesHoldUntil() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0).setHold(true)
                .setHoldUntil("2026-09-12T04:27:44Z")
                .setReason(GetOrderPayoutHoldResponse.Reason.APPEAL_WINDOW_OPEN)
                .setCaseNo("AS1")
                .build());
        PayoutHold hold = client.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(PayoutHoldReason.APPEAL_WINDOW_OPEN, hold.reason());
        assertEquals("AS1", hold.caseNo());
        assertEquals(2026, hold.holdUntil().getYear());
        assertEquals(9, hold.holdUntil().getMonthValue());
        assertEquals(12, hold.holdUntil().getDayOfMonth());
        assertEquals(4, hold.holdUntil().getHour());
        assertEquals(27, hold.holdUntil().getMinute());
    }

    @Test
    @DisplayName("P0 新碼值：APPEAL_IN_FLIGHT 必須映射成無時間界 hold，不得掉進 default 被誤標")
    void appealInFlightReason_mapped() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder()
                .setCode(0).setHold(true)
                .setReason(GetOrderPayoutHoldResponse.Reason.APPEAL_IN_FLIGHT)
                .setCaseNo("AS1")
                .build());
        PayoutHold hold = client.getPayoutHold(ORDER_NO);
        assertTrue(hold.hold());
        assertEquals(PayoutHoldReason.APPEAL_IN_FLIGHT, hold.reason());
        assertNull(hold.holdUntil(), "在途申訴無可預期結束時點");
    }

    @Test
    @DisplayName("反查必須真的帶 deadline 出去——沒有 deadline 就沒有租約下界")
    void deadlineIsActuallyApplied() {
        givenRpcReturns(GetOrderPayoutHoldResponse.newBuilder().setCode(0).setHold(false).build());
        client.getPayoutHold(ORDER_NO);
        org.mockito.Mockito.verify(stub).withDeadlineAfter(
                eq(AfterSalesGrpcClient.PAYOUT_HOLD_DEADLINE_SEC), eq(TimeUnit.SECONDS));
    }
}
