package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * 取消路徑的釋券（D3）：先寫任務再當場嘗試；當場失敗不拋出、不標完成，留給 worker 重試。
 */
class CouponReleaseServiceTest {

    private final CouponConsumptionTaskRepository repo = mock(CouponConsumptionTaskRepository.class);
    private final CheckoutCouponGrpcClient coupon = mock(CheckoutCouponGrpcClient.class);
    private final CouponReleaseService service = new CouponReleaseService(repo, coupon);

    @Test
    @DisplayName("當場釋放成功 → 任務先寫入、再呼叫 coupon（只帶 order_no）、最後標完成")
    void successEnqueuesThenReleasesThenMarksDone() {
        service.release("ORD-1");

        InOrder order = inOrder(repo, coupon);
        order.verify(repo).enqueueRelease(anyLong(), eq("ORD-1"));
        order.verify(coupon).releaseCouponByOrderNo("ORD-1");
        order.verify(repo).markReleaseDone("ORD-1");
    }

    @Test
    @DisplayName("當場釋放失敗 → 不拋出、不標完成（任務留著給 worker 重試）")
    void failureLeavesTheTaskForTheWorker() {
        doThrow(new IllegalStateException("coupon down")).when(coupon).releaseCouponByOrderNo("ORD-2");

        assertDoesNotThrow(() -> service.release("ORD-2"));

        verify(repo).enqueueRelease(anyLong(), eq("ORD-2"));
        verify(repo, never()).markReleaseDone(anyString());
    }

    @Test
    @DisplayName("寫任務失敗 → 仍當場嘗試釋放，不拋出")
    void enqueueFailureStillTriesToRelease() {
        doThrow(new RuntimeException("db down")).when(repo).enqueueRelease(anyLong(), anyString());

        assertDoesNotThrow(() -> service.release("ORD-3"));

        verify(coupon).releaseCouponByOrderNo("ORD-3");
    }
}
