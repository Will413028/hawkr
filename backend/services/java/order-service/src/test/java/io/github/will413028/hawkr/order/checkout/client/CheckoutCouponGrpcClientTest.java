package io.github.will413028.hawkr.order.checkout.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.coupon.grpc.ConsumeCouponRequest;
import io.github.will413028.hawkr.coupon.grpc.ConsumeCouponResponse;
import io.github.will413028.hawkr.coupon.grpc.CouponServiceGrpc;
import io.github.will413028.hawkr.coupon.grpc.ReleaseCouponRequest;
import io.github.will413028.hawkr.coupon.grpc.ReleaseCouponResponse;
import io.github.will413028.hawkr.coupon.grpc.ReserveCouponRequest;
import io.github.will413028.hawkr.coupon.grpc.ReserveCouponResponse;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.concurrent.TimeUnit;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * D10：鎖券帶店家與訂單金額讓 coupon 端重驗；核銷失敗一律拋出，worker 才有機會重試。
 */
class CheckoutCouponGrpcClientTest {

    private CouponServiceGrpc.CouponServiceBlockingStub stub;
    private CheckoutCouponGrpcClient client;

    @BeforeEach
    void setUp() {
        stub = mock(CouponServiceGrpc.CouponServiceBlockingStub.class);
        when(stub.withDeadlineAfter(anyLong(), any(TimeUnit.class))).thenReturn(stub);
        GrpcStubFactory factory = mock(GrpcStubFactory.class);
        when(factory.blocking(eq("coupon"), any())).thenReturn(stub);
        client = new CheckoutCouponGrpcClient(factory);
    }

    @Test
    @DisplayName("reserveCoupon 把 store_id 與 order_amount 一起送給 coupon")
    void reserveSendsStoreAndAmount() {
        when(stub.reserveCoupon(any(ReserveCouponRequest.class)))
                .thenReturn(ReserveCouponResponse.newBuilder().setSuccess(true).setInstanceId("V1").build());

        String instanceId = client.reserveCoupon("TPL", 77L, "ORD-1", 501L, 1000L);

        ArgumentCaptor<ReserveCouponRequest> sent = ArgumentCaptor.forClass(ReserveCouponRequest.class);
        verify(stub).reserveCoupon(sent.capture());
        assertThat(instanceId).isEqualTo("V1");
        assertThat(sent.getValue().getTemplateId()).isEqualTo("TPL");
        assertThat(sent.getValue().getBuyerId()).isEqualTo(77L);
        assertThat(sent.getValue().getOrderNo()).isEqualTo("ORD-1");
        assertThat(sent.getValue().getStoreId()).isEqualTo("501");
        assertThat(sent.getValue().getOrderAmount()).isEqualTo("1000");
    }

    @Test
    @DisplayName("consumeCoupon 成功 → 不拋")
    void consumeSuccessDoesNotThrow() {
        when(stub.consumeCoupon(any(ConsumeCouponRequest.class)))
                .thenReturn(ConsumeCouponResponse.newBuilder().setSuccess(true).build());

        client.consumeCoupon("V1", 77L, "ORD-1");

        ArgumentCaptor<ConsumeCouponRequest> sent = ArgumentCaptor.forClass(ConsumeCouponRequest.class);
        verify(stub).consumeCoupon(sent.capture());
        assertThat(sent.getValue().getOrderNo()).isEqualTo("ORD-1");
    }

    @Test
    @DisplayName("consumeCoupon 的 gRPC 失敗 → 原樣拋出，不吞")
    void consumeGrpcFailureIsRethrown() {
        when(stub.consumeCoupon(any(ConsumeCouponRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThatThrownBy(() -> client.consumeCoupon("V1", 77L, "ORD-1"))
                .isInstanceOf(StatusRuntimeException.class);
    }

    @Test
    @DisplayName("consumeCoupon 回 success=false → 拋出，不當成完成")
    void consumeBusinessFailureIsThrown() {
        when(stub.consumeCoupon(any(ConsumeCouponRequest.class)))
                .thenReturn(ConsumeCouponResponse.newBuilder().setSuccess(false).setCode(7009).setMessage("消耗失敗").build());

        assertThatThrownBy(() -> client.consumeCoupon("V1", 77L, "ORD-1"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("ORD-1");
    }

    @Test
    @DisplayName("consumeCoupon 回 7009 → 拋專用例外，讓 worker 判斷訂單是否已取消")
    void consumeNoVoucherIsATypedException() {
        when(stub.consumeCoupon(any(ConsumeCouponRequest.class)))
                .thenReturn(ConsumeCouponResponse.newBuilder().setSuccess(false).setCode(7009).build());

        assertThatThrownBy(() -> client.consumeCoupon("V1", 77L, "ORD-1"))
                .isInstanceOf(CouponNoVoucherForOrderException.class);
    }

    @Test
    @DisplayName("releaseCouponByOrderNo 已知券 id → order_no 與 instance_id 都送（舊 coupon-service 要讀 instance_id）")
    void releaseSendsInstanceIdWhenKnown() {
        when(stub.releaseCoupon(any(ReleaseCouponRequest.class)))
                .thenReturn(ReleaseCouponResponse.newBuilder().setSuccess(true).build());

        client.releaseCouponByOrderNo("ORD-1", "V1");

        ArgumentCaptor<ReleaseCouponRequest> sent = ArgumentCaptor.forClass(ReleaseCouponRequest.class);
        verify(stub).releaseCoupon(sent.capture());
        assertThat(sent.getValue().getOrderNo()).isEqualTo("ORD-1");
        assertThat(sent.getValue().getInstanceId()).isEqualTo("V1");
    }

    @Test
    @DisplayName("releaseCouponByOrderNo 不知道券 id → 只送 order_no")
    void releaseOmitsInstanceIdWhenUnknown() {
        when(stub.releaseCoupon(any(ReleaseCouponRequest.class)))
                .thenReturn(ReleaseCouponResponse.newBuilder().setSuccess(true).build());

        client.releaseCouponByOrderNo("ORD-1", null);

        ArgumentCaptor<ReleaseCouponRequest> sent = ArgumentCaptor.forClass(ReleaseCouponRequest.class);
        verify(stub).releaseCoupon(sent.capture());
        assertThat(sent.getValue().getOrderNo()).isEqualTo("ORD-1");
        assertThat(sent.getValue().getInstanceId()).isEmpty();
    }
}
