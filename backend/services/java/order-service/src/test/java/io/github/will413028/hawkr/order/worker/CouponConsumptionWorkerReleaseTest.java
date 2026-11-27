package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.checkout.client.CouponNoVoucherForOrderException;
import io.github.will413028.hawkr.order.entity.CouponConsumptionTaskEntity;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.contains;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 釋券任務（D3）：RELEASE 任務由 worker 以 order_no 呼叫 coupon，失敗時依既有的退避重試，不吞例外、不標完成。
 */
class CouponConsumptionWorkerReleaseTest {

    private static CouponConsumptionTaskEntity task(String type) {
        CouponConsumptionTaskEntity t = new CouponConsumptionTaskEntity();
        t.setTaskId(3003L);
        t.setOrderNo("ORD-R");
        t.setTaskType(type);
        t.setRetryCount(2);
        t.setMaxRetryCount(10);
        return t;
    }

    private final CouponConsumptionTaskRepository repo = mock(CouponConsumptionTaskRepository.class);
    private final CheckoutCouponGrpcClient coupon = mock(CheckoutCouponGrpcClient.class);
    private final OrderRepository orderRepository = mock(OrderRepository.class);

    private void run(CouponConsumptionTaskEntity t) {
        when(repo.findRetryable(anyInt())).thenReturn(List.of(t));
        when(repo.claimTask(anyLong(), any())).thenReturn(1);
        new CouponConsumptionWorker(repo, coupon, orderRepository).process();
    }

    @Test
    @DisplayName("RELEASE 任務 → 以 order_no 釋券（不核銷），成功標完成")
    void releaseTaskReleasesByOrderNo() {
        run(task(OrderConstants.COUPON_TASK_TYPE_RELEASE));

        verify(coupon).releaseCouponByOrderNo("ORD-R", null);
        verify(coupon, never()).consumeCoupon(anyString(), anyLong(), anyString());
        verify(repo).markDone(3003L);
    }

    @Test
    @DisplayName("RELEASE 失敗 → 不標完成，走 updateFailed 排下一次重試（退避）")
    void releaseFailureIsRetriedNotMarkedDone() {
        doThrow(new IllegalStateException("coupon down")).when(coupon).releaseCouponByOrderNo("ORD-R", null);

        run(task(OrderConstants.COUPON_TASK_TYPE_RELEASE));

        verify(repo, never()).markDone(anyLong());
        verify(repo).updateFailed(eq(3003L), eq(OrderConstants.TASK_STATUS_FAILED), contains("coupon down"), eq(240));
    }

    @Test
    @DisplayName("CONSUME 失敗（coupon 不可用）→ 不標完成，走 updateFailed 排下一次重試（退避）")
    void consumeFailureIsRetriedNotMarkedDone() {
        CouponConsumptionTaskEntity t = task(OrderConstants.COUPON_TASK_TYPE_CONSUME);
        t.setCouponInstanceId("CI-9");
        t.setBuyerId(3001L);
        doThrow(new IllegalStateException("coupon down")).when(coupon).consumeCoupon("CI-9", 3001L, "ORD-R");

        run(t);

        verify(repo, never()).markDone(anyLong());
        verify(repo).updateFailed(eq(3003L), eq(OrderConstants.TASK_STATUS_FAILED), contains("coupon down"), eq(240));
    }

    @Test
    @DisplayName("超過最大重試次數的 RELEASE 任務 → 轉人工，不再呼叫 coupon")
    void exhaustedReleaseTaskGoesToManualIntervention() {
        CouponConsumptionTaskEntity t = task(OrderConstants.COUPON_TASK_TYPE_RELEASE);
        t.setRetryCount(10);

        run(t);

        verify(repo).markManualIntervention(3003L);
        verify(coupon, never()).releaseCouponByOrderNo(anyString(), any());
    }

    @Test
    @DisplayName("不認得的任務型別 → 不核銷也不釋券，記失敗等重試上限後轉人工")
    void unknownTaskTypeDoesNothingToTheCoupon() {
        run(task("WHATEVER"));

        verify(coupon, never()).releaseCouponByOrderNo(anyString(), any());
        verify(coupon, never()).consumeCoupon(anyString(), anyLong(), anyString());
        verify(repo, never()).markDone(anyLong());
        verify(repo).updateFailed(eq(3003L), eq(OrderConstants.TASK_STATUS_FAILED), anyString(), anyInt());
    }

    @Test
    @DisplayName("RELEASE 任務帶券 id → 一併送給 coupon")
    void releaseTaskPassesTheInstanceId() {
        CouponConsumptionTaskEntity t = task(OrderConstants.COUPON_TASK_TYPE_RELEASE);
        t.setCouponInstanceId("CI-7");

        run(t);

        verify(coupon).releaseCouponByOrderNo("ORD-R", "CI-7");
    }

    private CouponConsumptionTaskEntity consumeTask() {
        CouponConsumptionTaskEntity t = task(OrderConstants.COUPON_TASK_TYPE_CONSUME);
        t.setCouponInstanceId("CI-9");
        t.setBuyerId(3001L);
        return t;
    }

    private OrderEntity orderWithStatus(OrderStatus status) {
        OrderEntity o = new OrderEntity();
        o.setOrderNo("ORD-R");
        o.setStatus(status.value());
        return o;
    }

    @Test
    @DisplayName("CONSUME 回 7009 且訂單已取消 → 終態，任務標完成，不重試")
    void consumeNoVoucherOnCancelledOrderIsTerminal() {
        doThrow(new CouponNoVoucherForOrderException("7009")).when(coupon).consumeCoupon("CI-9", 3001L, "ORD-R");
        when(orderRepository.findByOrderNo("ORD-R")).thenReturn(orderWithStatus(OrderStatus.CANCELLED));

        run(consumeTask());

        verify(repo).markDone(3003L);
        verify(repo, never()).updateFailed(anyLong(), anyString(), anyString(), anyInt());
    }

    @Test
    @DisplayName("CONSUME 回 7009 但訂單沒取消 → 券遺失，照樣失敗重試，不標完成")
    void consumeNoVoucherOnLiveOrderKeepsFailing() {
        doThrow(new CouponNoVoucherForOrderException("7009")).when(coupon).consumeCoupon("CI-9", 3001L, "ORD-R");
        when(orderRepository.findByOrderNo("ORD-R")).thenReturn(orderWithStatus(OrderStatus.PAID));

        run(consumeTask());

        verify(repo, never()).markDone(anyLong());
        verify(repo).updateFailed(eq(3003L), eq(OrderConstants.TASK_STATUS_FAILED), anyString(), anyInt());
    }
}
