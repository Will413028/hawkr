package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.SellerCancelRequestEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesResponse;
import io.github.will413028.hawkr.order.grpc.OrderAfterSalesView;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Plan C Phase 3：{@link SellerCancelService#cancelShipment} 的 account_id vs
 * store_id 判別測試 — 這支比對的是 order-service 回傳的 store_id（非本地
 * entity），呼叫端（gRPC handler）過去傳入未解析的登入 account_id 時必須被拒絕。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class SellerCancelServiceTest {

    @Mock private SellerCancelRequestMapper cancelMapper;
    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesProperties properties;
    @Mock private AfterSalesEventPublisher afterSalesEventPublisher;

    @InjectMocks private SellerCancelService service;

    private static final long ORDER_STORE_ID = 7001L;
    /** 刻意與 ORDER_STORE_ID / 退款金額 680 都不同，讓參數錯位一定會 fail */
    private static final long ORDER_BUYER_ID = 4201L;

    private OrderAfterSalesView.Builder orderViewBuilder() {
        return OrderAfterSalesView.newBuilder()
                .setStoreId(String.valueOf(ORDER_STORE_ID))
                .setStatus(OrderStatus.PAID.value());
    }

    /**
     * cancelShipment 走到建立 cancel/refund task 時會呼叫 {@code TsidUtil.generate()}，
     * 未初始化會拋 IllegalStateException。TsidUtil 是 JVM 靜態單例、重複 init 會拋，
     * 而 surefire 預設 reuseForks 讓整個 module 的測試共用同一個 JVM——所以吞掉
     * 「已初始化」例外（比照 shipping-service BuyerStorePickupServiceTest 的作法）。
     * node-id-base 700 對齊 application.yml。
     */
    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(700, 0);
        } catch (IllegalStateException alreadyInitialized) {
            // 同一個 JVM 內已由其他測試初始化，忽略
        }
    }

    @BeforeEach
    void setup() {
        when(orderGrpcClient.getOrderForAfterSales("O001")).thenReturn(
                GetOrderForAfterSalesResponse.newBuilder()
                        .setCode(0)
                        .setData(orderViewBuilder().build())
                        .build());
    }

    @Test
    void cancelShipment_callerIdDiffersFromOrderStoreId_throwsNotOwner() {
        AfterSalesBusinessException ex = assertThrows(
                AfterSalesBusinessException.class,
                () -> service.cancelShipment(42L, "O001", "9202", null));
        assertEquals(GrpcResponseCode.AFTERSALE_NOT_OWNER.code(), ex.code());

        verify(cancelMapper, never()).insert(any());
    }

    @Test
    void cancelShipment_callerIdMatchesOrderStoreId_proceedsPastOwnershipCheck() {
        when(cancelMapper.selectCount(any())).thenReturn(0L);

        // Downstream order fields (buyerId, totalActualAmount, etc.) are
        // intentionally left unstubbed on the order view, so the flow throws
        // past the ownership check without needing to stub the entire
        // downstream flow. The assertThrows/isNotOwnerRejection check alone
        // only proves the exception wasn't a NOT_OWNER rejection — it does NOT
        // by itself prove the ownership check was reached (e.g. a regression
        // in the earlier reason-code validation at cancelShipment's top would
        // also throw a non-NOT_OWNER RuntimeException and pass that check
        // vacuously). The verify() below closes that gap: cancelMapper
        // .selectCount(...) is only invoked after both the ownership check
        // (store_id match) and the status check succeed, so observing exactly
        // one call to it proves execution actually passed the ownership check
        // for a matching id, not merely that some non-NOT_OWNER exception was
        // thrown.
        RuntimeException ex = assertThrows(
                RuntimeException.class,
                () -> service.cancelShipment(ORDER_STORE_ID, "O001", "9202", null));
        boolean isNotOwnerRejection = ex instanceof AfterSalesBusinessException businessEx
                && businessEx.code() == GrpcResponseCode.AFTERSALE_NOT_OWNER.code();
        assertFalse(isNotOwnerRejection);
        verify(cancelMapper).selectCount(any());
    }

    /**
     * issue #1626：cancel 流程的 {@code publishAwaitingBuyerRefundPage(orderNo, caseNo, refundTaskId, buyerId)}
     * 是這次修改裡最脆弱的一條路徑——{@code caseNo} 傳 null（取消沒有售後案件），{@code buyerId} 來自
     * method 內區域變數 {@code cancelEntity}，而非參數或 entity 欄位；先前完全沒有測試走到這裡
     * （設計書 §7 Followup 點名要補）。這支測試把整條 ATM 分支跑到底，用 captor 綁死第 4 個
     * 參數 = order view 的 buyer_id（{@link #ORDER_BUYER_ID}），值刻意與 storeId / 退款金額 /
     * refundTaskId 都不同，避免任何錯位參數矇混過關。
     */
    @Test
    @DisplayName("ATM 已付款取消 → publishAwaitingBuyerRefundPage 帶 caseNo=null + cancelEntity.buyerId")
    void cancelShipment_atmPaid_publishesAwaitingBuyerRefundPageWithBuyerId() {
        when(orderGrpcClient.getOrderForAfterSales("O001")).thenReturn(
                GetOrderForAfterSalesResponse.newBuilder()
                        .setCode(0)
                        .setData(orderViewBuilder()
                                .setBuyerId(String.valueOf(ORDER_BUYER_ID))
                                .setTotalActualAmount("680")
                                .setPaymentMethod(PaymentMethod.ATM.value())
                                .setTransactionId("TXN1")
                                .setMerId("MER1")
                                .build())
                        .build());
        when(cancelMapper.selectCount(any())).thenReturn(0L);
        when(paymentGrpcClient.createRefund(any(), any(), any(), any())).thenReturn(
                CreateRefundResponse.newBuilder()
                        .setCode(0)
                        .setData(RefundDetail.newBuilder().setRefundNo("RFD_1").build())
                        .build());
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());

        service.cancelShipment(ORDER_STORE_ID, "O001", "9202", null);

        // 先證明真的走到 ATM 分支（非 ATM 只會 insert task、不會預建 payment refund）
        verify(paymentGrpcClient).createRefund(any(), any(), any(), any());
        verify(cancelMapper).markRefundProcessing(anyLong());

        ArgumentCaptor<String> orderNoCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> caseNoCaptor = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<Long> refundTaskIdCaptor = ArgumentCaptor.forClass(Long.class);
        ArgumentCaptor<Long> buyerIdCaptor = ArgumentCaptor.forClass(Long.class);
        verify(afterSalesEventPublisher).publishAwaitingBuyerRefundPage(
                orderNoCaptor.capture(), caseNoCaptor.capture(),
                refundTaskIdCaptor.capture(), buyerIdCaptor.capture());

        assertEquals("O001", orderNoCaptor.getValue());
        assertNull(caseNoCaptor.getValue(), "取消流程沒有售後案件，caseNo 必須是 null");
        assertNotNull(refundTaskIdCaptor.getValue(), "refundTaskId 應為建立的 refund task id");
        assertEquals(ORDER_BUYER_ID, buyerIdCaptor.getValue(),
                "第 4 個參數必須是 cancelEntity.buyerId（= order view 的 buyer_id），不可與 storeId/refundTaskId 錯位");
        assertNotEquals(ORDER_STORE_ID, buyerIdCaptor.getValue());
    }

    /** 非 ATM 已付款取消：不預建 payment refund、也不發 awaiting_buyer_page（對照組） */
    @Test
    @DisplayName("非 ATM 已付款取消 → 不發 publishAwaitingBuyerRefundPage")
    void cancelShipment_nonAtmPaid_doesNotPublishAwaitingBuyerRefundPage() {
        when(orderGrpcClient.getOrderForAfterSales("O001")).thenReturn(
                GetOrderForAfterSalesResponse.newBuilder()
                        .setCode(0)
                        .setData(orderViewBuilder()
                                .setBuyerId(String.valueOf(ORDER_BUYER_ID))
                                .setTotalActualAmount("680")
                                .setPaymentMethod(PaymentMethod.CREDIT_CARD.value())
                                .setTransactionId("TXN1")
                                .setMerId("MER1")
                                .build())
                        .build());
        when(cancelMapper.selectCount(any())).thenReturn(0L);
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());

        service.cancelShipment(ORDER_STORE_ID, "O001", "9202", null);

        verify(paymentGrpcClient, never()).createRefund(any(), any(), any(), any());
        verify(afterSalesEventPublisher, never())
                .publishAwaitingBuyerRefundPage(any(), any(), any(), any());
    }
}
