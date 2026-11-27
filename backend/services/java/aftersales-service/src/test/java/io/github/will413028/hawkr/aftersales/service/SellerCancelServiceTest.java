package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.entity.SellerCancelRequestEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesResponse;
import io.github.will413028.hawkr.order.grpc.OrderAfterSalesView;
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

        RuntimeException ex = assertThrows(
                RuntimeException.class,
                () -> service.cancelShipment(ORDER_STORE_ID, "O001", "9202", null));
        boolean isNotOwnerRejection = ex instanceof AfterSalesBusinessException businessEx
                && businessEx.code() == GrpcResponseCode.AFTERSALE_NOT_OWNER.code();
        assertFalse(isNotOwnerRejection);
        verify(cancelMapper).selectCount(any());
    }

    /** 已付款取消：建一筆 PENDING 退款任務交給 RefundTaskWorker，不在取消流程內直接呼叫 payment。 */
    @Test
    @DisplayName("已付款取消 → 建 PENDING 退款任務並標 REFUND_PROCESSING，不直接呼叫 payment")
    void cancelShipment_paid_insertsPendingRefundTask() {
        when(orderGrpcClient.getOrderForAfterSales("O001")).thenReturn(
                GetOrderForAfterSalesResponse.newBuilder()
                        .setCode(0)
                        .setData(orderViewBuilder()
                                .setBuyerId(String.valueOf(ORDER_BUYER_ID))
                                .setTotalActualAmount("680")
                                .setPaymentMethod(PaymentMethod.CREDIT_CARD.value())
                                .setTransactionId("TXN1")
                                .build())
                        .build());
        when(cancelMapper.selectCount(any())).thenReturn(0L);
        when(properties.getRefund()).thenReturn(new AfterSalesProperties.RefundConfig());

        service.cancelShipment(ORDER_STORE_ID, "O001", "9202", null);

        ArgumentCaptor<RefundTaskEntity> taskCaptor = ArgumentCaptor.forClass(RefundTaskEntity.class);
        verify(refundTaskMapper).insert(taskCaptor.capture());
        RefundTaskEntity task = taskCaptor.getValue();
        assertEquals("O001", task.getOrderNo());
        assertEquals("TXN1", task.getTransactionId());
        assertEquals(680L, task.getRefundAmount());
        assertEquals(WorkerTaskStatus.PENDING.value(), task.getStatus());
        verify(cancelMapper).markRefundProcessing(anyLong());
    }
}
