package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.mq.PaymentEventHandler;
import io.github.will413028.hawkr.payment.grpc.GetActivePaymentStatusByOrderNoResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
import com.fasterxml.jackson.databind.JsonNode;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * PENDING_PAYMENT 訂單系統批次取消共用編排測試（03 §5.1a.2 / §8.5）。
 *
 * 覆蓋 gate 各分支：PAID 收斂跳過、確定非 PAID 照舊取消、付款真相未知（RPC 失敗／非成功碼）
 * fail-safe 保守跳過（MQ-01），以及 CAS 0 rows 已被他流程處理。此防護於付款逾時與賣家停權批次
 * 取消兩路徑共用（cancelReason 只是透傳參數，不影響防護本身）。
 */
@ExtendWith(MockitoExtension.class)
class OrderExpiryCancellationServiceTest {

    @Mock PaymentGrpcClient paymentGrpcClient;
    @Mock PaymentEventHandler paymentEventHandler;
    @Mock CheckoutCouponGrpcClient checkoutCouponGrpcClient;

    private static final String ORDER_NO = "O400280260";

    private OrderExpiryCancellationService newService() {
        return new OrderExpiryCancellationService(
                paymentGrpcClient, paymentEventHandler, checkoutCouponGrpcClient);
    }

    @Test
    @DisplayName("取消前反查已付款（PAID）→ 收斂為 PAID，跳過取消，不呼叫 cancelExpiredOrderInTx")
    void skipsCancelWhenAlreadyPaid() {
        PaymentDetail detail = PaymentDetail.newBuilder()
                .setTradeNo("TRADE001")
                .setOrderId(ORDER_NO)
                .setAmount("1000")
                .setPayTime("")
                .setPaymentType("CREDIT_CARD")
                .setStatus("1") // PaymentTransactionStatus.PAID
                .build();
        GetActivePaymentStatusByOrderNoResponse response = GetActivePaymentStatusByOrderNoResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setData(detail)
                .build();
        when(paymentGrpcClient.getActivePaymentStatusByOrderNo(ORDER_NO)).thenReturn(response);

        boolean cancelled = newService().cancelPendingOrder(ORDER_NO, OrderCancelReason.SELLER_SUSPENDED.value());

        assertFalse(cancelled);
        verify(paymentEventHandler, never()).cancelExpiredOrderInTx(anyString(), anyString());
        ArgumentCaptor<JsonNode> payloadCaptor = ArgumentCaptor.forClass(JsonNode.class);
        verify(paymentEventHandler).handlePaymentSuccess(payloadCaptor.capture(), anyString(), anyString());
        assertEquals("TRADE001", payloadCaptor.getValue().path("trade_no").asText());
    }

    @Test
    @DisplayName("非已付款（查無交易）→ 照舊走 CAS 取消（庫存歸還已於 cancelExpiredOrderInTx 內落 outbox）+ best-effort 釋券")
    void cancelsWhenNotPaid() {
        GetActivePaymentStatusByOrderNoResponse response = GetActivePaymentStatusByOrderNoResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setData(PaymentDetail.newBuilder().setStatus("").build())
                .build();
        when(paymentGrpcClient.getActivePaymentStatusByOrderNo(ORDER_NO)).thenReturn(response);

        PaymentEventHandler.CancelInfo info =
                new PaymentEventHandler.CancelInfo(ORDER_NO, "checkout-1", 1001L, "coupon-1");
        when(paymentEventHandler.cancelExpiredOrderInTx(ORDER_NO, OrderCancelReason.SELLER_SUSPENDED.value()))
                .thenReturn(info);

        boolean cancelled = newService().cancelPendingOrder(ORDER_NO, OrderCancelReason.SELLER_SUSPENDED.value());

        assertTrue(cancelled);
        verify(paymentEventHandler).cancelExpiredOrderInTx(ORDER_NO, OrderCancelReason.SELLER_SUSPENDED.value());
        // 庫存歸還已改為 cancelExpiredOrderInTx 事務內落 order.cancelled outbox（#1368），此處只驗 best-effort 釋券
        verify(checkoutCouponGrpcClient).releaseCoupon("coupon-1", ORDER_NO);
    }

    @Test
    @DisplayName("gRPC 反查失敗（fail-safe，MQ-01）→ 付款真相未知，保守跳過取消不誤取消已付款單，待下輪重掃")
    void skipsCancelWhenGateRpcFails() {
        when(paymentGrpcClient.getActivePaymentStatusByOrderNo(ORDER_NO))
                .thenThrow(new RuntimeException("payment-service unavailable"));

        boolean cancelled = newService().cancelPendingOrder(ORDER_NO, OrderCancelReason.PAYMENT_TIMEOUT.value());

        assertFalse(cancelled);
        // fail-safe：gate 無法判定付款真相 → 不進 CAS 取消，交由下輪 PaymentDeadlineScheduler 重掃自癒
        verify(paymentEventHandler, never()).cancelExpiredOrderInTx(anyString(), anyString());
        verify(checkoutCouponGrpcClient, never()).releaseCoupon(anyString(), anyString());
    }

    @Test
    @DisplayName("反查回非成功碼（fail-safe，MQ-01）→ payment 側查詢自身失敗，付款真相未知同 RPC 失敗保守跳過取消")
    void skipsCancelWhenGateReturnsNonSuccess() {
        GetActivePaymentStatusByOrderNoResponse response = GetActivePaymentStatusByOrderNoResponse.newBuilder()
                .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                .build();
        when(paymentGrpcClient.getActivePaymentStatusByOrderNo(ORDER_NO)).thenReturn(response);

        boolean cancelled = newService().cancelPendingOrder(ORDER_NO, OrderCancelReason.PAYMENT_TIMEOUT.value());

        assertFalse(cancelled);
        verify(paymentEventHandler, never()).cancelExpiredOrderInTx(anyString(), anyString());
        verify(checkoutCouponGrpcClient, never()).releaseCoupon(anyString(), anyString());
    }

    @Test
    @DisplayName("已被其他流程處理（CAS 0 rows，cancelExpiredOrderInTx 回 null）→ 回 false，不做補償")
    void returnsFalseWhenAlreadyHandledByOtherFlow() {
        GetActivePaymentStatusByOrderNoResponse response = GetActivePaymentStatusByOrderNoResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setData(PaymentDetail.newBuilder().setStatus("").build())
                .build();
        when(paymentGrpcClient.getActivePaymentStatusByOrderNo(ORDER_NO)).thenReturn(response);
        when(paymentEventHandler.cancelExpiredOrderInTx(eq(ORDER_NO), any())).thenReturn(null);

        boolean cancelled = newService().cancelPendingOrder(ORDER_NO, OrderCancelReason.SELLER_SUSPENDED.value());

        assertFalse(cancelled);
        // CAS 0 rows → cancelExpiredOrderInTx 內既不改狀態也不落 outbox，且不做 TX 外釋券補償
        verify(checkoutCouponGrpcClient, never()).releaseCoupon(anyString(), anyString());
    }
}
